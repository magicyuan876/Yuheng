package file

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// backendScopedFileService makes the storage instance part of every newly
// persisted path while delegating actual I/O to the existing provider driver.
type backendScopedFileService struct {
	backendID string
	inner     interfaces.FileService
}

func NewBackendScopedFileService(backendID string, inner interfaces.FileService) interfaces.FileService {
	return &backendScopedFileService{backendID: backendID, inner: inner}
}

func (s *backendScopedFileService) unwrap(path string) (string, error) {
	id, inner, ok := types.ParseStorageBackendPath(path)
	if !ok {
		return path, nil
	}
	if id != s.backendID {
		return "", fmt.Errorf("storage backend mismatch")
	}
	return inner, nil
}

func (s *backendScopedFileService) wrap(path string) string {
	return types.BuildStorageBackendPath(s.backendID, path)
}

func (s *backendScopedFileService) CheckConnectivity(ctx context.Context) error {
	return s.inner.CheckConnectivity(ctx)
}

func (s *backendScopedFileService) SaveFile(ctx context.Context, f *multipart.FileHeader, tenantID uint64, knowledgeID string) (string, error) {
	p, err := s.inner.SaveFile(ctx, f, tenantID, knowledgeID)
	if err != nil {
		return "", err
	}
	return s.wrap(p), nil
}

func (s *backendScopedFileService) SaveBytes(ctx context.Context, data []byte, tenantID uint64, name string, temp bool) (string, error) {
	p, err := s.inner.SaveBytes(ctx, data, tenantID, name, temp)
	if err != nil {
		return "", err
	}
	return s.wrap(p), nil
}

func (s *backendScopedFileService) GetFile(ctx context.Context, path string) (io.ReadCloser, error) {
	p, err := s.unwrap(path)
	if err != nil {
		return nil, err
	}
	return s.inner.GetFile(ctx, p)
}

func (s *backendScopedFileService) GetFileURL(ctx context.Context, path string) (string, error) {
	p, err := s.unwrap(path)
	if err != nil {
		return "", err
	}
	result, err := s.inner.GetFileURL(ctx, p)
	if err != nil {
		return "", err
	}
	if result == p {
		return s.wrap(p), nil
	}
	return result, nil
}

func (s *backendScopedFileService) DeleteFile(ctx context.Context, path string) error {
	p, err := s.unwrap(path)
	if err != nil {
		return err
	}
	return s.inner.DeleteFile(ctx, p)
}

func (s *backendScopedFileService) CopyFile(ctx context.Context, path string, tenantID uint64, knowledgeID string) (string, error) {
	p, err := s.unwrap(path)
	if err != nil {
		return "", err
	}
	result, err := s.inner.CopyFile(ctx, p, tenantID, knowledgeID)
	if err != nil {
		return "", err
	}
	return s.wrap(result), nil
}
