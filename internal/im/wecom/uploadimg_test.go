package wecom

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/im"
)

func newUploadimgTestServer(t *testing.T, tokenCalls, uploadCalls *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			tokenCalls.Add(1)
			require.Equal(t, "corp-1", r.URL.Query().Get("corpid"))
			require.Equal(t, "secret-1", r.URL.Query().Get("corpsecret"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errcode": 0, "access_token": "tok-1", "expires_in": 7200,
			})
		case "/cgi-bin/media/uploadimg":
			uploadCalls.Add(1)
			require.Equal(t, "tok-1", r.URL.Query().Get("access_token"))
			require.NoError(t, r.ParseMultipartForm(16<<20))
			_, header, err := r.FormFile("media")
			require.NoError(t, err)
			require.Greater(t, header.Size, int64(0))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errcode": 0, "url": "https://wework.qpic.cn/wwpic/mock-img",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func testUploader(baseURL string) *ImgUploader {
	return NewImgUploaderFromConfig(map[string]any{
		"corp_id":      "corp-1",
		"corp_secret":  "secret-1",
		"api_base_url": baseURL,
	})
}

func TestImgUploaderUploadsAndCaches(t *testing.T) {
	var tokenCalls, uploadCalls atomic.Int32
	server := newUploadimgTestServer(t, &tokenCalls, &uploadCalls)
	defer server.Close()

	u := testUploader(server.URL)
	require.NotNil(t, u)

	img := im.ReplyImage{Data: []byte("jpeg-bytes"), MD5: "abc123"}
	url1, err := u.UploadForURL(context.Background(), img)
	require.NoError(t, err)
	assert.Equal(t, "https://wework.qpic.cn/wwpic/mock-img", url1)

	// Same MD5 → cache hit, no second upload; token fetched once.
	url2, err := u.UploadForURL(context.Background(), img)
	require.NoError(t, err)
	assert.Equal(t, url1, url2)
	assert.Equal(t, int32(1), tokenCalls.Load())
	assert.Equal(t, int32(1), uploadCalls.Load())
}

func TestImgUploaderRefreshesExpiredToken(t *testing.T) {
	var uploads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errcode": 0, "access_token": "tok-1", "expires_in": 7200,
			})
		case "/cgi-bin/media/uploadimg":
			if uploads.Add(1) == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 42001, "errmsg": "access_token expired"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "url": "https://wework.qpic.cn/ok"})
		}
	}))
	defer server.Close()

	u := testUploader(server.URL)
	url, err := u.UploadForURL(context.Background(), im.ReplyImage{Data: []byte("x"), MD5: "m1"})
	require.NoError(t, err)
	assert.Equal(t, "https://wework.qpic.cn/ok", url)
	assert.Equal(t, int32(2), uploads.Load(), "retries once after token expiry")
}

func TestImgUploaderUnconfiguredReturnsNil(t *testing.T) {
	assert.Nil(t, NewImgUploaderFromConfig(map[string]any{}))
	assert.Nil(t, NewImgUploaderFromConfig(map[string]any{"corp_id": "only-id"}))
}

func TestImgUploaderSurfacesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/gettoken" {
			_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "access_token": "tok", "expires_in": 7200})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 301002, "errmsg": "no privilege"})
	}))
	defer server.Close()

	u := testUploader(server.URL)
	_, err := u.UploadForURL(context.Background(), im.ReplyImage{Data: []byte("x"), MD5: "m2"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "301002")
}
