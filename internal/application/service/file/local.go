package file

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	secutils "github.com/magicyuan876/yuheng/internal/utils"
)

// localFileService implements the FileService interface for local file system storage
type localFileService struct {
	baseDir string // Base directory for file storage
}

const localScheme = "local://"

// CheckConnectivity verifies the local storage directory exists and is accessible.
func (s *localFileService) CheckConnectivity(ctx context.Context) error {
	info, err := os.Stat(s.baseDir)
	if err != nil {
		return fmt.Errorf("storage directory not accessible: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("storage path is not a directory: %s", s.baseDir)
	}
	return nil
}

// NewLocalFileService creates a new local file service instance rooted at
// baseDir.
func NewLocalFileService(baseDir string) interfaces.FileService {
	return &localFileService{baseDir: baseDir}
}

// SaveFile stores an uploaded file to the local file system
// The file is stored in a directory structure: baseDir/tenantID/knowledgeID/filename
// Returns the full file path or an error if saving fails
func (s *localFileService) SaveFile(ctx context.Context,
	file *multipart.FileHeader, tenantID uint64, knowledgeID string,
) (string, error) {
	logger.Info(ctx, "Starting to save file locally")
	logger.Infof(ctx, "File information: name=%s, size=%d, tenant ID=%d, knowledge ID=%s",
		file.Filename, file.Size, tenantID, knowledgeID)

	// Create storage directory with tenant and knowledge ID
	dir := filepath.Join(s.baseDir, fmt.Sprintf("%d", tenantID), knowledgeID)
	if _, err := secutils.SafePathUnderBase(s.baseDir, dir); err != nil {
		logger.Errorf(ctx, "Path traversal denied for SaveFile dir: %v", err)
		return "", fmt.Errorf("invalid path: %w", err)
	}
	logger.Infof(ctx, "Creating directory: %s", dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.Errorf(ctx, "Failed to create directory: %v", err)
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate unique filename using timestamp
	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	filePath := filepath.Join(dir, filename)
	logger.Infof(ctx, "Generated file path: %s", filePath)

	// Open source file for reading
	logger.Info(ctx, "Opening source file")
	src, err := file.Open()
	if err != nil {
		logger.Errorf(ctx, "Failed to open source file: %v", err)
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	// Create destination file for writing
	logger.Info(ctx, "Creating destination file")
	dst, err := os.Create(filePath)
	if err != nil {
		logger.Errorf(ctx, "Failed to create destination file: %v", err)
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy content from source to destination
	logger.Info(ctx, "Copying file content")
	if _, err := io.Copy(dst, src); err != nil {
		logger.Errorf(ctx, "Failed to copy file content: %v", err)
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	logger.Infof(ctx, "File saved successfully: %s", filePath)
	// Return the driver-native locator: local://{relative_path}
	relPath, _ := filepath.Rel(s.baseDir, filePath)
	return localScheme + filepath.ToSlash(relPath), nil
}

// GetFile retrieves a file from the local file system by its local:// path.
// Returns a ReadCloser for reading the file content.
// 路径必须在 baseDir 下，防止路径遍历（如 ../../）
func (s *localFileService) GetFile(ctx context.Context, filePath string) (io.ReadCloser, error) {
	logger.Infof(ctx, "Getting file: %s", filePath)

	resolved, err := s.resolvePath(filePath)
	if err != nil {
		logger.Errorf(ctx, "Path traversal denied for GetFile: %v", err)
		return nil, fmt.Errorf("invalid file path: %w", err)
	}

	file, err := os.Open(resolved)
	if err != nil {
		// baseDir/resolved are logged so a storage base-dir mismatch (e.g.
		// writer and reader started with different LOCAL_STORAGE_BASE_DIR)
		// is immediately visible instead of just "no such file or directory".
		logger.Errorf(ctx, "Failed to open file: baseDir=%s resolvedPath=%s err=%v", s.baseDir, resolved, err)
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	logger.Info(ctx, "File opened successfully")
	return file, nil
}

// DeleteFile removes a file from the local file system
// Returns an error if deletion fails
// 路径必须在 baseDir 下，防止路径遍历（如 ../../）
func (s *localFileService) DeleteFile(ctx context.Context, filePath string) error {
	logger.Infof(ctx, "Deleting file: %s", filePath)

	resolved, err := s.resolvePath(filePath)
	if err != nil {
		logger.Errorf(ctx, "Path traversal denied for DeleteFile: %v", err)
		return fmt.Errorf("invalid file path: %w", err)
	}

	err = os.Remove(resolved)
	if err != nil {
		logger.Errorf(ctx, "Failed to delete file: %v", err)
		return fmt.Errorf("failed to delete file: %w", err)
	}

	logger.Info(ctx, "File deleted successfully")
	return nil
}

// CopyFile copies an existing local object to a new knowledge-owned object.
// The destination uses the same layout as SaveFile (baseDir/{tenantID}/{knowledgeID}/{unique}{ext}),
// and the copy is a real byte-for-byte copy (no hardlink) so deleting the source
// never affects it. Returns ErrCrossBackendCopy when srcPath is not a local path.
func (s *localFileService) CopyFile(ctx context.Context,
	srcPath string, tenantID uint64, knowledgeID string,
) (string, error) {
	// A provider scheme other than local:// (e.g. s3://) means a
	// cross-backend copy, which this service does not support.
	if i := strings.Index(srcPath, "://"); i >= 0 && srcPath[:i+3] != localScheme {
		return "", fmt.Errorf("local file service cannot copy %q: %w", srcPath, ErrCrossBackendCopy)
	}

	// Validate and resolve the source path under baseDir (same guard as GetFile).
	srcResolved, err := s.resolvePath(srcPath)
	if err != nil {
		logger.Errorf(ctx, "Path traversal denied for CopyFile src: %v", err)
		return "", fmt.Errorf("invalid source path: %w", err)
	}

	// Build destination path with the knowledge-owned layout.
	dir := filepath.Join(s.baseDir, fmt.Sprintf("%d", tenantID), knowledgeID)
	if _, err := secutils.SafePathUnderBase(s.baseDir, dir); err != nil {
		logger.Errorf(ctx, "Path traversal denied for CopyFile dir: %v", err)
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	ext := filepath.Ext(srcPath)
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dstPath := filepath.Join(dir, filename)

	src, err := os.Open(srcResolved)
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	relPath, _ := filepath.Rel(s.baseDir, dstPath)
	newPath := localScheme + filepath.ToSlash(relPath)
	logger.Infof(ctx, "Copied local file %s to %s", srcPath, newPath)
	return newPath, nil
}

// SaveBytes saves bytes data to a file and returns the file path
// temp parameter is ignored for local storage (no auto-expiration support)
// fileName 仅允许安全文件名，禁止路径遍历（如 ../../）
func (s *localFileService) SaveBytes(ctx context.Context, data []byte, tenantID uint64, fileName string, temp bool) (string, error) {
	logger.Infof(ctx, "Saving bytes data: fileName=%s, size=%d, tenantID=%d, temp=%v", fileName, len(data), tenantID, temp)

	safeName, err := secutils.SafeFileName(fileName)
	if err != nil {
		logger.Errorf(ctx, "Invalid fileName for SaveBytes: %v", err)
		return "", fmt.Errorf("invalid file name: %w", err)
	}

	// Create storage directory with tenant ID
	dir := filepath.Join(s.baseDir, fmt.Sprintf("%d", tenantID), "exports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logger.Errorf(ctx, "Failed to create directory: %v", err)
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate unique filename using timestamp
	ext := filepath.Ext(safeName)
	baseName := safeName[:len(safeName)-len(ext)]
	uniqueFileName := fmt.Sprintf("%s_%d%s", baseName, time.Now().UnixNano(), ext)
	filePath := filepath.Join(dir, uniqueFileName)

	// Write data to file
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		logger.Errorf(ctx, "Failed to write file: %v", err)
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	logger.Infof(ctx, "Bytes data saved successfully: %s", filePath)
	relPath, _ := filepath.Rel(s.baseDir, filePath)
	return localScheme + filepath.ToSlash(relPath), nil
}

// GetFileURL has no URL to offer: a directory on this server's disk is not
// addressable from outside it. It returns the local:// locator unchanged, and
// public links are resource grants that FileStore issues on this server's
// own address instead.
func (s *localFileService) GetFileURL(ctx context.Context, filePath string) (string, error) {
	if !strings.HasPrefix(filePath, localScheme) {
		return "", fmt.Errorf("not a %s path: %q", localScheme, filePath)
	}
	return filePath, nil
}

// resolvePath maps a local://{relative_path} storage path onto the file
// system, refusing anything that is not a local:// path or that would escape
// baseDir (e.g. local://../../etc/passwd).
func (s *localFileService) resolvePath(filePath string) (string, error) {
	relPath, ok := strings.CutPrefix(filePath, localScheme)
	if !ok {
		return "", fmt.Errorf("not a %s path: %q", localScheme, filePath)
	}
	return secutils.SafePathUnderBase(s.baseDir, filepath.Join(s.baseDir, filepath.FromSlash(relPath)))
}
