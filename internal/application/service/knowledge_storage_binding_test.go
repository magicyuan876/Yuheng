package service

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"strings"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// bindingRecorder is a FileStore that records which backend each write was
// sent to; every writer it hands out records into the same log.
type bindingRecorder struct {
	writes []string // backend id per saved object
}

func (r *bindingRecorder) Writer(_ context.Context, backendID string) (interfaces.FileService, error) {
	if backendID == "" {
		return nil, errors.New("no backend named")
	}
	return &recordingWriter{log: r, backendID: backendID}, nil
}

func (r *bindingRecorder) ForKnowledgeBase(
	ctx context.Context, kb *types.KnowledgeBase,
) (interfaces.FileService, error) {
	if kb == nil || kb.StorageBackendID == nil {
		return nil, errors.New("unbound knowledge base")
	}
	return r.Writer(ctx, *kb.StorageBackendID)
}

func (r *bindingRecorder) ForTenantDefault(ctx context.Context, _ uint64) (interfaces.FileService, error) {
	return r.Writer(ctx, "workspace-default")
}

func (r *bindingRecorder) Open(context.Context, string) (io.ReadCloser, *types.StoredResource, error) {
	return nil, nil, errors.New("not used")
}

func (r *bindingRecorder) URL(context.Context, string, time.Duration) (string, bool, error) {
	return "", false, nil
}

func (r *bindingRecorder) Delete(context.Context, string) error { return nil }

func (r *bindingRecorder) LocalPath(context.Context, string) (string, bool) { return "", false }

type recordingWriter struct {
	log       *bindingRecorder
	backendID string
}

func (w *recordingWriter) CheckConnectivity(context.Context) error { return nil }

func (w *recordingWriter) SaveFile(context.Context, *multipart.FileHeader, uint64, string) (string, error) {
	w.log.writes = append(w.log.writes, w.backendID)
	return "resource://AbCdEfGhIjKlMnOpQrStUv", nil
}

func (w *recordingWriter) SaveBytes(context.Context, []byte, uint64, string, bool) (string, error) {
	w.log.writes = append(w.log.writes, w.backendID)
	return "resource://AbCdEfGhIjKlMnOpQrStUv", nil
}

func (w *recordingWriter) GetFile(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("not used")
}

func (w *recordingWriter) GetFileURL(_ context.Context, ref string) (string, error) { return ref, nil }

func (w *recordingWriter) DeleteFile(context.Context, string) error { return nil }

func (w *recordingWriter) CopyFile(context.Context, string, uint64, string) (string, error) {
	return "", errors.New("not used")
}

// An upload lands on the knowledge base's bound backend, never on a
// process-wide default.
func TestKnowledgeUploadWritesToTheKnowledgeBaseBackend(t *testing.T) {
	backendID := "kb-backend"
	files := &bindingRecorder{}
	kb := &types.KnowledgeBase{ID: "kb-1", StorageBackendID: &backendID}
	svc := &knowledgeService{
		repo:      &createKnowledgeFileRepoStub{},
		kbService: &createKnowledgeFileKBServiceStub{kb: kb},
		files:     files,
		task:      &createKnowledgeTaskEnqueuerStub{},
	}
	_, err := svc.CreateKnowledgeFromFile(newCreateKnowledgeFileContext(), "kb-1",
		newMultipartFileHeader(t, "doc.txt", "hello"), nil, nil, "", nil, "", nil)
	require.NoError(t, err)
	require.Equal(t, []string{backendID}, files.writes)
}

// A knowledge base with no backend has nowhere to write: the upload is
// refused up front instead of being saved somewhere unrelated.
func TestKnowledgeUploadRefusesAnUnboundKnowledgeBase(t *testing.T) {
	files := &bindingRecorder{}
	repo := &createKnowledgeFileRepoStub{}
	svc := &knowledgeService{
		repo:      repo,
		kbService: &createKnowledgeFileKBServiceStub{kb: &types.KnowledgeBase{ID: "kb-1"}},
		files:     files,
	}
	_, err := svc.CreateKnowledgeFromFile(newCreateKnowledgeFileContext(), "kb-1",
		newMultipartFileHeader(t, "doc.txt", "hello"), nil, nil, "", nil, "", nil)
	require.Error(t, err)
	require.Empty(t, files.writes)
	require.Zero(t, repo.createCalls)
}

type faqCSVKBServiceStub struct {
	interfaces.KnowledgeBaseService
	kb *types.KnowledgeBase
}

func (s *faqCSVKBServiceStub) GetKnowledgeBaseByIDOnly(context.Context, string) (*types.KnowledgeBase, error) {
	return s.kb, nil
}

// The failed-entries CSV of an FAQ import is written to the FAQ knowledge
// base's own backend (B9: it used to go to the deployment's environment
// storage whatever the knowledge base was bound to).
func TestFAQFailedEntriesCSVWritesToTheKnowledgeBaseBackend(t *testing.T) {
	backendID := "faq-backend"
	files := &bindingRecorder{}
	svc := &knowledgeService{
		kbService: &faqCSVKBServiceStub{kb: &types.KnowledgeBase{ID: "kb-faq", StorageBackendID: &backendID}},
		files:     files,
	}
	link, err := svc.generateFailedEntriesCSV(context.Background(), 1, "kb-faq", "task-1",
		[]types.FAQFailedEntry{{Reason: "duplicate", StandardQuestion: "q"}})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(link, types.ResourceScheme), link)
	require.Equal(t, []string{backendID}, files.writes)
}
