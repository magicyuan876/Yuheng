package im

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec // asserting the protocol checksum
	"encoding/hex"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/storageurl"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// replyImageFakeFileService serves canned bytes per reference.
type replyImageFakeFileService struct {
	interfaces.FileService
	files map[string][]byte
}

func (f *replyImageFakeFileService) GetFile(_ context.Context, path string) (io.ReadCloser, error) {
	data, ok := f.files[path]
	if !ok {
		return nil, assert.AnError
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))))
	return buf.Bytes()
}

func replyImageResolver(files map[string][]byte) *storageurl.FileServiceResolver {
	svc := &replyImageFakeFileService{files: files}
	tenant := &types.Tenant{}
	return storageurl.NewFileServiceResolver(tenant, svc)
}

func TestExtractReplyImagesInlinesStorageImages(t *testing.T) {
	img := pngBytes(t)
	base := t.TempDir()
	t.Setenv("LOCAL_STORAGE_BASE_DIR", base)
	require.NoError(t, os.MkdirAll(filepath.Join(base, "10000", "k1"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(base, "10000", "k1", "frame.png"), img, 0o644))
	resolver := replyImageResolver(nil)

	content := "开头文本\n\n![帧](local://10000/k1/frame.png)\n\n结尾文本"
	out, images := extractReplyImages(context.Background(), content, resolver)

	require.Len(t, images, 1)
	// Output is the recompressed JPEG, not the stored bytes.
	decoded, format, derr := image.Decode(bytes.NewReader(images[0].Data))
	require.NoError(t, derr)
	assert.Equal(t, "jpeg", format)
	assert.Greater(t, decoded.Bounds().Dx(), 0)
	assert.LessOrEqual(t, len(images[0].Data), replyImageTargetBytes())
	sum := md5.Sum(images[0].Data) //nolint:gosec
	assert.Equal(t, hex.EncodeToString(sum[:]), images[0].MD5)

	assert.NotContains(t, out, "local://10000/k1/frame.png")
	assert.Contains(t, out, "【图1】")
	assert.Contains(t, out, "开头文本")
	assert.Contains(t, out, "结尾文本")
}

func TestExtractReplyImagesKeepsFailedRefsForURLFallback(t *testing.T) {
	resolver := replyImageResolver(map[string][]byte{}) // nothing resolvable

	content := "![图](local://missing/a.png) 后文"
	out, images := extractReplyImages(context.Background(), content, resolver)

	assert.Empty(t, images)
	assert.Contains(t, out, "local://missing/a.png",
		"unresolvable refs keep markdown so URL rewriting can still apply")
}

func TestExtractReplyImagesRejectsNonImageBytes(t *testing.T) {
	resolver := replyImageResolver(map[string][]byte{
		"local://10000/k1/doc.pdf.png": []byte("%PDF-1.7 not an image"),
	})

	content := "![x](local://10000/k1/doc.pdf.png)"
	out, images := extractReplyImages(context.Background(), content, resolver)
	assert.Empty(t, images)
	assert.Contains(t, out, "local://10000/k1/doc.pdf.png")
}

func TestExtractReplyImagesCapsCount(t *testing.T) {
	img := pngBytes(t)
	base := t.TempDir()
	t.Setenv("LOCAL_STORAGE_BASE_DIR", base)
	require.NoError(t, os.MkdirAll(filepath.Join(base, "k1"), 0o755))
	var sb strings.Builder
	for i := 0; i < maxReplyImages+3; i++ {
		name := "f" + strings.Repeat("x", i+1) + ".png"
		require.NoError(t, os.WriteFile(filepath.Join(base, "k1", name), img, 0o644))
		sb.WriteString("![f](local://k1/" + name + ")\n")
	}
	resolver := replyImageResolver(nil)

	out, images := extractReplyImages(context.Background(), sb.String(), resolver)
	assert.Len(t, images, maxReplyImages)
	// Overflow images keep their markdown for the URL fallback.
	assert.Contains(t, out, "local://k1/f"+strings.Repeat("x", maxReplyImages+1))
}

func TestInlineReplyImagesAsURLsEmbedsCDNLinks(t *testing.T) {
	img := pngBytes(t)
	base := t.TempDir()
	t.Setenv("LOCAL_STORAGE_BASE_DIR", base)
	require.NoError(t, os.MkdirAll(filepath.Join(base, "k1"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(base, "k1", "a.png"), img, 0o644))
	resolver := replyImageResolver(nil)

	uploaderAny := &anyURLUploader{url: "https://wework.qpic.cn/wwpic/xyz"}

	content := "前文\n\n![图](local://k1/a.png)\n\n后文"
	out, replaced := inlineReplyImagesAsURLs(context.Background(), content, resolver, uploaderAny)
	assert.Equal(t, 1, replaced)
	assert.Contains(t, out, "![图](https://wework.qpic.cn/wwpic/xyz)")
	assert.NotContains(t, out, "local://k1/a.png")
}

type anyURLUploader struct{ url string }

func (a *anyURLUploader) ImageURLUploadAvailable() bool { return true }
func (a *anyURLUploader) UploadImageForURL(context.Context, ReplyImage) (string, error) {
	return a.url, nil
}

type failingURLUploader struct{}

func (f *failingURLUploader) ImageURLUploadAvailable() bool { return true }
func (f *failingURLUploader) UploadImageForURL(context.Context, ReplyImage) (string, error) {
	return "", assert.AnError
}

func TestInlineReplyImagesAsURLsKeepsMarkdownOnUploadFailure(t *testing.T) {
	img := pngBytes(t)
	base := t.TempDir()
	t.Setenv("LOCAL_STORAGE_BASE_DIR", base)
	require.NoError(t, os.MkdirAll(filepath.Join(base, "k1"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(base, "k1", "a.png"), img, 0o644))
	resolver := replyImageResolver(nil)

	content := "![图](local://k1/a.png)"
	out, replaced := inlineReplyImagesAsURLs(context.Background(), content, resolver, &failingURLUploader{})
	assert.Equal(t, 0, replaced)
	assert.Contains(t, out, "local://k1/a.png", "failed uploads keep markdown for URL rewrite fallback")
}

func TestExtractReplyImagesHandlesResourceRefs(t *testing.T) {
	img := pngBytes(t)
	resolver := replyImageResolver(map[string][]byte{
		"resource://AAAAAAAAAAAAAAAAAAAAAA": img,
	})

	content := "![帧](resource://AAAAAAAAAAAAAAAAAAAAAA)"
	out, images := extractReplyImages(context.Background(), content, resolver)
	require.Len(t, images, 1)
	assert.Contains(t, out, "【图1】")
}
