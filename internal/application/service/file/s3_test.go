package file

import (
	"context"
	"testing"
)

func TestNewS3Client_Credentials(t *testing.T) {
	t.Run("static credentials remain supported", func(t *testing.T) {
		svc, err := newS3Client(S3Options{
			AccessKey: "static-ak", SecretKey: "static-sk", BucketName: "bucket", Region: "us-east-1",
		})
		if err != nil {
			t.Fatalf("newS3Client() error = %v", err)
		}
		got, err := svc.client.Options().Credentials.Retrieve(context.Background())
		if err != nil {
			t.Fatalf("Retrieve() error = %v", err)
		}
		if got.AccessKeyID != "static-ak" || got.SecretAccessKey != "static-sk" {
			t.Fatalf("unexpected credentials: access key %q", got.AccessKeyID)
		}
	})

	t.Run("empty keys use the AWS default credential chain", func(t *testing.T) {
		t.Setenv("AWS_ACCESS_KEY_ID", "role-ak")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "role-sk")
		svc, err := newS3Client(S3Options{BucketName: "bucket", Region: "us-east-1"})
		if err != nil {
			t.Fatalf("newS3Client() error = %v", err)
		}
		got, err := svc.client.Options().Credentials.Retrieve(context.Background())
		if err != nil {
			t.Fatalf("Retrieve() error = %v", err)
		}
		if got.AccessKeyID != "role-ak" || got.SecretAccessKey != "role-sk" {
			t.Fatalf("default credential chain returned access key %q", got.AccessKeyID)
		}
	})

	t.Run("partial static credentials are rejected", func(t *testing.T) {
		_, err := newS3Client(S3Options{AccessKey: "only-ak", BucketName: "bucket", Region: "us-east-1"})
		if err == nil {
			t.Fatal("newS3Client() expected an error")
		}
	})
}

func TestResolveS3PathStyle(t *testing.T) {
	const (
		awsEndpoint   = "https://s3.us-east-1.amazonaws.com"
		awsChina      = "https://s3.cn-north-1.amazonaws.com.cn"
		customHTTPS   = "https://storage.internal:9000"
		rustfs        = "http://rustfs:9000"
		aliyunOSS     = "https://oss-cn-hangzhou.aliyuncs.com"
		noEndpointAWS = ""
	)
	tests := []struct {
		name     string
		endpoint string
		style    string
		want     bool
		wantErr  bool
	}{
		{name: "auto with AWS regional endpoint", endpoint: awsEndpoint, style: "auto", want: false},
		{name: "auto with AWS China endpoint", endpoint: awsChina, style: "auto", want: false},
		{name: "auto with custom endpoint is path-style", endpoint: rustfs, style: "auto", want: true},
		{name: "empty style is auto for a custom endpoint", endpoint: customHTTPS, style: "", want: true},
		{name: "empty style is auto for AWS", endpoint: awsEndpoint, style: "", want: false},
		{name: "auto with empty endpoint keeps the AWS default", endpoint: noEndpointAWS, style: "auto", want: false},
		{name: "path wins over an AWS endpoint", endpoint: awsEndpoint, style: "path", want: true},
		{name: "path with empty endpoint", endpoint: noEndpointAWS, style: "path", want: true},
		{name: "virtual wins on a custom endpoint", endpoint: aliyunOSS, style: "virtual", want: false},
		{name: "unknown style is rejected", endpoint: rustfs, style: "bucket-first", wantErr: true},
		{name: "style is case sensitive", endpoint: rustfs, style: "Path", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveS3PathStyle(tt.endpoint, tt.style)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveS3PathStyle() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("resolveS3PathStyle(%q, %q) = %v, want %v", tt.endpoint, tt.style, got, tt.want)
			}
		})
	}
}

func TestS3EndpointURL(t *testing.T) {
	tests := []struct {
		endpoint string
		useSSL   bool
		want     string
	}{
		{"", true, ""},
		{"  ", false, ""},
		{"rustfs:9000", false, "http://rustfs:9000"},
		{"rustfs:9000", true, "https://rustfs:9000"},
		{"http://rustfs:9000", true, "http://rustfs:9000"},
		{"https://oss-cn-hangzhou.aliyuncs.com", false, "https://oss-cn-hangzhou.aliyuncs.com"},
	}
	for _, tt := range tests {
		if got := S3EndpointURL(tt.endpoint, tt.useSSL); got != tt.want {
			t.Errorf("S3EndpointURL(%q, %v) = %q, want %q", tt.endpoint, tt.useSSL, got, tt.want)
		}
	}
}

// The client is built without an endpoint here because a custom endpoint is
// SSRF-checked through DNS at construction, which unit tests must not need.
// The endpoint-dependent mapping is covered by TestResolveS3PathStyle.
func TestNewS3Client_AppliesAddressingStyle(t *testing.T) {
	base := S3Options{Region: "us-east-1", BucketName: "bucket", AccessKey: "ak", SecretKey: "sk"}
	tests := []struct {
		style string
		want  bool
	}{
		{"", false},
		{"auto", false},
		{"virtual", false},
		{"path", true},
	}
	for _, tt := range tests {
		t.Run("style "+tt.style, func(t *testing.T) {
			opts := base
			opts.AddressingStyle = tt.style
			svc, err := newS3Client(opts)
			if err != nil {
				t.Fatalf("newS3Client() error = %v", err)
			}
			if got := svc.client.Options().UsePathStyle; got != tt.want {
				t.Errorf("UsePathStyle = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("invalid style is rejected", func(t *testing.T) {
		opts := base
		opts.AddressingStyle = "sideways"
		if _, err := newS3Client(opts); err == nil {
			t.Fatal("expected an error for an unknown addressing style")
		}
	})
}

func TestParseS3FilePath(t *testing.T) {
	svc := &s3FileService{bucketName: "test-bucket"}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "valid s3 path",
			input: "s3://test-bucket/123/exports/abc.png",
			want:  "123/exports/abc.png",
		},
		{
			name:    "wrong bucket",
			input:   "s3://other-bucket/key",
			wantErr: true,
		},
		{
			name:    "not s3 scheme",
			input:   "local://test-bucket/key",
			wantErr: true,
		},
		{
			name:    "missing object key",
			input:   "s3://test-bucket/",
			wantErr: true,
		},
		{
			name:    "empty path",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.parseS3FilePath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseS3FilePath(%q) expected error, got %q", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("parseS3FilePath(%q) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("parseS3FilePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
