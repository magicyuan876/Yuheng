package service

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// routedStorage keeps one in-memory backend per id and, like the real file
// store, reads a reference back from whichever backend wrote it — never from
// the space's current binding.
type routedStorage struct {
	backends map[string]*memFiles
}

func (r routedStorage) Writer(_ context.Context, id string) (interfaces.FileService, error) {
	files, ok := r.backends[id]
	if !ok {
		return nil, fmt.Errorf("no backend %q", id)
	}
	return files, nil
}

func (r routedStorage) ForTenantDefault(ctx context.Context, _ uint64) (interfaces.FileService, error) {
	return r.Writer(ctx, "default")
}

func (r routedStorage) holder(ref string) (*memFiles, error) {
	for _, files := range r.backends {
		files.mu.Lock()
		_, ok := files.objects[ref]
		files.mu.Unlock()
		if ok {
			return files, nil
		}
	}
	return nil, types.ErrResourceNotFound
}

func (r routedStorage) Open(ctx context.Context, ref string) (io.ReadCloser, *types.StoredResource, error) {
	files, err := r.holder(ref)
	if err != nil {
		return nil, nil, err
	}
	body, err := files.GetFile(ctx, ref)
	return body, &types.StoredResource{PhysicalPath: ref}, err
}

func (r routedStorage) URL(context.Context, string, time.Duration) (string, bool, error) {
	return "", false, nil
}

func (r routedStorage) Delete(ctx context.Context, ref string) error {
	files, err := r.holder(ref)
	if err != nil {
		return err
	}
	return files.DeleteFile(ctx, ref)
}

// Rebinding a space, or changing the workspace default, moves only where new
// attachments go; every existing attachment keeps resolving — and can still be
// released — from the backend that holds it (B3).
func TestAttachmentsSurviveRebindingTheSpace(t *testing.T) {
	first, second, workspaceDefault := newMemFiles(), newMemFiles(), newMemFiles()
	storage := routedStorage{backends: map[string]*memFiles{
		"first": first, "second": second, "default": workspaceDefault,
	}}
	e := &attachEnv{pageEnv: newPageEnvWith(t, func(d *Deps) {
		d.Storage = storage
		d.Tenants = &fakeTenants{}
	}), files: first, tenants: &fakeTenants{}}
	page := e.create(t, e.alice, nil, "Page")

	// Unbound, the space writes to the workspace default.
	onDefault, err := e.put(t, e.alice, "default.png", testPNG(t, 4, 4, 1), page.ID)
	require.NoError(t, err)
	require.Equal(t, 1, workspaceDefault.count())

	bound := "first"
	e.space.StorageBackendID = &bound
	onFirst, err := e.put(t, e.alice, "first.png", testPNG(t, 4, 4, 2), page.ID)
	require.NoError(t, err)
	require.Equal(t, 1, first.count())

	rebound := "second"
	e.space.StorageBackendID = &rebound
	onSecond, err := e.put(t, e.alice, "second.png", testPNG(t, 4, 4, 3), page.ID)
	require.NoError(t, err)
	require.Equal(t, 1, second.count())

	for _, view := range []*AttachmentView{onDefault, onFirst, onSecond} {
		res, err := e.svc.Files.Fetch(ctx(), e.alice, view.ID, 0)
		require.NoError(t, err, view.FileName)
		require.NotEmpty(t, readAll(t, res), view.FileName)
		// A variant is rendered from the original, read the same way.
		res, err = e.svc.Files.Fetch(ctx(), e.alice, view.ID, 320)
		require.NoError(t, err, view.FileName)
		require.NotEmpty(t, readAll(t, res), view.FileName)
	}

	require.NoError(t, e.svc.Files.Delete(ctx(), e.alice, onFirst.ID))
	require.Equal(t, 0, first.count(), "the object is released from the backend that held it")
	require.Equal(t, 1, second.count())
}
