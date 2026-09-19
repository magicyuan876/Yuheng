package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func dispositionFor(t *testing.T, fileName string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	serveDownload(c, fileName, "text/markdown", []byte("body"))
	return rec.Header().Get("Content-Disposition")
}

func TestAnASCIINameIsSentPlainly(t *testing.T) {
	got := dispositionFor(t, "Onboarding.md")
	assert.Contains(t, got, `filename="Onboarding.md"`)
	assert.Contains(t, got, "filename*=UTF-8''Onboarding.md")
}

// A Chinese title must survive, which the bare filename= parameter cannot do.
func TestAChineseNameIsSentEncodedWithAnASCIIFallback(t *testing.T) {
	got := dispositionFor(t, "存储配额说明.md")
	assert.Contains(t, got, "filename*=UTF-8''%E5%AD%98%E5%82%A8%E9%85%8D%E9%A2%9D%E8%AF%B4%E6%98%8E.md")
	// The fallback carries no raw bytes that would arrive as mojibake.
	assert.Contains(t, got, `filename="______.md"`)
}

// A quote in the file name would end the parameter early and let the rest of
// the name be read as further parameters.
func TestAQuotedNameCannotBreakOutOfTheHeader(t *testing.T) {
	got := dispositionFor(t, `a"; download="evil.sh`)
	assert.NotContains(t, got, `download="evil.sh"`)
	assert.Contains(t, got, `filename="a_`)
}

func TestANamelessDownloadStillHasAName(t *testing.T) {
	assert.Contains(t, dispositionFor(t, ""), `filename="download"`)
	assert.Contains(t, dispositionFor(t, "🎉"), `filename="download"`)
}

// An exported HTML file is somebody's own markup; sniffing it would be a way
// to run it on this origin.
func TestADownloadIsNeverSniffed(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	serveDownload(c, "page.html", "text/html; charset=utf-8", []byte("<b>hi</b>"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
}
