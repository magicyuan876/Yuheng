package file

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/magicyuan876/yuheng/internal/utils"
)

// s3FileService is the S3-compatible file service implementation.
type s3FileService struct {
	client     *s3.Client
	bucketName string
	pathPrefix string
}

// S3Options is everything needed to reach one S3-compatible bucket.
type S3Options struct {
	// Endpoint is empty for AWS S3 itself. A value without a scheme gets one
	// from UseSSL so operators can write "rustfs:9000" the way MinIO clients do.
	Endpoint   string
	Region     string
	AccessKey  string
	SecretKey  string
	BucketName string
	PathPrefix string
	UseSSL     bool
	// AddressingStyle is "", "auto", "path" or "virtual"; see resolveS3PathStyle.
	AddressingStyle string
}

// resolveS3PathStyle maps an endpoint and a configured addressing style to the
// SDK's UsePathStyle switch.
//
// "auto" mirrors what each family of services needs: AWS (or no endpoint at
// all, which the SDK resolves to AWS) works best virtual-hosted, while any
// other custom endpoint, such as MinIO or RustFS, is normally reached by an
// IP or a single hostname with no wildcard DNS and therefore needs path-style.
// Providers that reject path-style (Aliyun OSS, Tencent COS, Volcengine TOS,
// Huawei OBS) must be configured with "virtual" explicitly.
func resolveS3PathStyle(endpoint, style string) (bool, error) {
	if err := types.ValidateS3AddressingStyle(style); err != nil {
		return false, err
	}
	switch style {
	case types.S3AddressingPath:
		return true, nil
	case types.S3AddressingVirtual:
		return false, nil
	default:
		return endpoint != "" && !strings.Contains(endpoint, "amazonaws.com"), nil
	}
}

// S3EndpointURL returns the endpoint with an explicit scheme, or "" for AWS.
func S3EndpointURL(endpoint string, useSSL bool) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" || strings.Contains(endpoint, "://") {
		return endpoint
	}
	if useSSL {
		return "https://" + endpoint
	}
	return "http://" + endpoint
}

// newS3Client creates a bare s3FileService with just the SDK client initialised.
func newS3Client(opts S3Options) (*s3FileService, error) {
	endpoint := S3EndpointURL(opts.Endpoint, opts.UseSSL)
	if err := utils.ValidateURLForSSRF(endpoint); err != nil {
		return nil, fmt.Errorf("unsafe S3 endpoint: %w", err)
	}
	usePathStyle, err := resolveS3PathStyle(endpoint, opts.AddressingStyle)
	if err != nil {
		return nil, err
	}

	// With no explicit AK/SK, keep the AWS default credential chain intact. This
	// supports IAM roles for EC2/ECS/EKS (IRSA), web identity, shared config, and
	// environment credentials without persisting long-lived keys in Yuheng.
	loadOptions := []func(*config.LoadOptions) error{config.WithRegion(opts.Region)}
	if opts.AccessKey != "" || opts.SecretKey != "" {
		if opts.AccessKey == "" || opts.SecretKey == "" {
			return nil, fmt.Errorf("S3 access key and secret key must be provided together")
		}
		loadOptions = append(loadOptions, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(opts.AccessKey, opts.SecretKey, ""),
		))
	}
	cfg, err := config.LoadDefaultConfig(context.Background(), loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	httpClient := utils.NewSSRFSafeHTTPClient(utils.DefaultSSRFSafeHTTPClientConfig())
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = usePathStyle
		o.HTTPClient = httpClient
	})

	// Normalize pathPrefix: ensure it ends with '/' if not empty
	pathPrefix := opts.PathPrefix
	if pathPrefix != "" && !strings.HasSuffix(pathPrefix, "/") {
		pathPrefix += "/"
	}

	return &s3FileService{
		client:     client,
		bucketName: opts.BucketName,
		pathPrefix: pathPrefix,
	}, nil
}

// NewS3FileService creates an S3-compatible file service.
// It verifies that the bucket exists and creates it if missing.
func NewS3FileService(opts S3Options) (interfaces.FileService, error) {
	svc, err := newS3Client(opts)
	if err != nil {
		return nil, err
	}
	exists, err := svc.bucketExists(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket: %w", err)
	}
	if !exists {
		if err = svc.createBucket(context.Background()); err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}
	return svc, nil
}

// bucketExists checks if the bucket exists
func (s *s3FileService) bucketExists(ctx context.Context) (bool, error) {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucketName),
	})
	if err != nil {
		// Check if the error is a NotFound error
		var notFound *s3types.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// createBucket creates a new bucket
func (s *s3FileService) createBucket(ctx context.Context) error {
	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.bucketName),
	})
	return err
}

// CheckConnectivity verifies S3 is reachable and, if a bucket is configured,
// that the bucket exists. This is a read-only probe — it never creates a bucket.
func (s *s3FileService) CheckConnectivity(ctx context.Context) error {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if s.bucketName != "" {
		exists, err := s.bucketExists(checkCtx)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("bucket %q does not exist", s.bucketName)
		}
		return nil
	}

	// List buckets to verify connectivity
	_, err := s.client.ListBuckets(checkCtx, &s3.ListBucketsInput{})
	return err
}

// CheckS3Connectivity tests S3 connectivity using the provided settings.
// It creates a temporary service instance internally and delegates to CheckConnectivity.
func CheckS3Connectivity(ctx context.Context, opts S3Options) error {
	opts.PathPrefix = ""
	svc, err := newS3Client(opts)
	if err != nil {
		return err
	}
	return svc.CheckConnectivity(ctx)
}

// parseS3FilePath extracts the object name from a provider scheme: s3://{bucket}/{objectKey}
func (s *s3FileService) parseS3FilePath(filePath string) (string, error) {
	// Provider scheme format: s3://{bucket}/{objectKey}
	const prefix = "s3://"
	if !strings.HasPrefix(filePath, prefix) {
		return "", fmt.Errorf("invalid S3 file path: %s", filePath)
	}
	rest := strings.TrimPrefix(filePath, prefix)
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid S3 file path: %s", filePath)
	}
	if parts[0] != s.bucketName {
		return "", fmt.Errorf("bucket mismatch in path: got %s, want %s", parts[0], s.bucketName)
	}
	if err := utils.SafeObjectKey(parts[1]); err != nil {
		return "", fmt.Errorf("invalid file path: %w", err)
	}
	return parts[1], nil
}

// SaveFile saves a file to S3
func (s *s3FileService) SaveFile(ctx context.Context,
	file *multipart.FileHeader, tenantID uint64, knowledgeID string,
) (string, error) {
	// Generate object name
	ext := filepath.Ext(file.Filename)
	objectName := fmt.Sprintf("%s%d/%s/%s%s", s.pathPrefix, tenantID, knowledgeID, uuid.New().String(), ext)

	// Open file
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	// Determine content type
	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = utils.GetContentTypeByExt(ext)
	}

	// Upload file to S3
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucketName),
		Key:           aws.String(objectName),
		Body:          src,
		ContentLength: aws.Int64(file.Size),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file to S3: %w", err)
	}

	return fmt.Sprintf("s3://%s/%s", s.bucketName, objectName), nil
}

// GetFile gets a file from S3
func (s *s3FileService) GetFile(ctx context.Context, filePath string) (io.ReadCloser, error) {
	objectName, err := s.parseS3FilePath(filePath)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(objectName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file from S3: %w", err)
	}

	return resp.Body, nil
}

// DeleteFile deletes a file
func (s *s3FileService) DeleteFile(ctx context.Context, filePath string) error {
	objectName, err := s.parseS3FilePath(filePath)
	if err != nil {
		return err
	}

	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(objectName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// CopyFile copies an existing S3 object to a new knowledge-owned object using a
// server-side CopyObject (no data leaves S3). The destination uses the same
// layout as SaveFile. Returns ErrCrossBackendCopy when srcPath is not an s3:// path.
func (s *s3FileService) CopyFile(ctx context.Context,
	srcPath string, tenantID uint64, knowledgeID string,
) (string, error) {
	srcKey, err := s.parseS3FilePath(srcPath)
	if err != nil {
		return "", fmt.Errorf("s3 copy rejected source %q: %w", srcPath, ErrCrossBackendCopy)
	}

	ext := filepath.Ext(srcPath)
	destKey := fmt.Sprintf("%s%d/%s/%s%s", s.pathPrefix, tenantID, knowledgeID, uuid.New().String(), ext)

	// CopySource is "bucket/key"; the '/' separators must NOT be percent-encoded
	// (url.PathEscape would turn them into %2F and break the bucket/key split).
	// srcKey is already validated by parseS3FilePath -> SafeObjectKey.
	_, err = s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucketName),
		CopySource: aws.String(s.bucketName + "/" + srcKey),
		Key:        aws.String(destKey),
	})
	if err != nil {
		return "", fmt.Errorf("failed to copy file in S3: %w", err)
	}

	newPath := fmt.Sprintf("s3://%s/%s", s.bucketName, destKey)
	logger.Infof(ctx, "Copied S3 object %s to %s", srcPath, newPath)
	return newPath, nil
}

// SaveBytes saves bytes data to S3 and returns the file path
// temp parameter is ignored for S3 (no auto-expiration support in this implementation)
func (s *s3FileService) SaveBytes(ctx context.Context, data []byte, tenantID uint64, fileName string, temp bool) (string, error) {
	safeName, err := utils.SafeFileName(fileName)
	if err != nil {
		return "", fmt.Errorf("invalid file name: %w", err)
	}
	ext := filepath.Ext(safeName)
	objectName := fmt.Sprintf("%s%d/exports/%s%s", s.pathPrefix, tenantID, uuid.New().String(), ext)

	// Upload bytes to S3
	reader := bytes.NewReader(data)
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucketName),
		Key:           aws.String(objectName),
		Body:          reader,
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   aws.String(utils.GetContentTypeByExt(ext)),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload bytes to S3: %w", err)
	}

	return fmt.Sprintf("s3://%s/%s", s.bucketName, objectName), nil
}

// GetFileURL returns a presigned download URL for the file
func (s *s3FileService) GetFileURL(ctx context.Context, filePath string) (string, error) {
	objectName, err := s.parseS3FilePath(filePath)
	if err != nil {
		return "", err
	}

	// Create presign client
	presignClient := s3.NewPresignClient(s.client)

	// Generate presigned URL
	presignedReq, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(objectName),
	}, s3.WithPresignExpires(24*time.Hour))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedReq.URL, nil
}
