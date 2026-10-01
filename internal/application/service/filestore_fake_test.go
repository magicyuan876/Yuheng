package service

import (
	"context"
	"io"
	"time"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// singleServiceStore is a FileStore over one FileService stub, for tests of
// code that reads through the store and writes through a writer it obtained
// from it. Every reference resolves to the stub, which is what a deployment
// with one backend looks like.
type singleServiceStore struct{ svc interfaces.FileService }

func storeOver(svc interfaces.FileService) interfaces.FileStore { return singleServiceStore{svc: svc} }

func (s singleServiceStore) Writer(context.Context, string) (interfaces.FileService, error) {
	return s.svc, nil
}

func (s singleServiceStore) ForKnowledgeBase(context.Context, *types.KnowledgeBase) (interfaces.FileService, error) {
	return s.svc, nil
}

func (s singleServiceStore) ForTenantDefault(context.Context, uint64) (interfaces.FileService, error) {
	return s.svc, nil
}

func (s singleServiceStore) Open(ctx context.Context, ref string) (io.ReadCloser, *types.StoredResource, error) {
	reader, err := s.svc.GetFile(ctx, ref)
	if err != nil {
		return nil, nil, err
	}
	return reader, &types.StoredResource{PhysicalPath: ref}, nil
}

func (s singleServiceStore) URL(ctx context.Context, ref string, _ time.Duration) (string, bool, error) {
	url, err := s.svc.GetFileURL(ctx, ref)
	return url, err == nil && url != ref, err
}

func (s singleServiceStore) Delete(ctx context.Context, ref string) error {
	return s.svc.DeleteFile(ctx, ref)
}

func (s singleServiceStore) LocalPath(context.Context, string) (string, bool) { return "", false }
