package container

// The tests in this file boot the real server: BuildContainer, the real
// router, a real database. Nothing here is a fake.
//
// Why this exists: every other test in the repository builds the pieces it
// needs by hand, so nothing ever started the dependency-injection graph the way
// main() does, and nothing ran the real authentication middleware in front of
// the real handlers. Two defects shipped through exactly that gap: the server
// could not start with its own default configuration, and mirrored documents
// were written without a tenant. Unit tests of the parts could not see either.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"go.uber.org/dig"
	"gorm.io/gorm"

	"github.com/magicyuan876/yuheng/internal/docs"
	"github.com/magicyuan876/yuheng/internal/testutil/pgtest"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// testJWTSecret and testAESKey are valid values for the two secrets the server
// refuses (or warns about) when they look like the shipped examples. They are
// fixed so that a test can mint tokens the server will accept.
const (
	testJWTSecret = "e2e-jwt-secret-0123456789abcdef0123456789abcdef"
	testAESKey    = "0123456789abcdef0123456789abcdef" // exactly 32 bytes
)

// bootOptions selects the deployment shape a test starts.
type bootOptions struct {
	// redis starts an in-process Redis (miniredis) and points REDIS_ADDR at it,
	// which switches the server from its single-process mode (inline task
	// executor, in-memory caches) to the distributed one (asynq, Redis bus).
	redis bool
	// env is applied last, so a test can change any setting.
	env map[string]string
}

// server is a booted application.
type server struct {
	t      *testing.T
	Engine *gin.Engine
	DI     *dig.Container
	DB     *gorm.DB
	Redis  *miniredis.Miniredis // nil unless bootOptions.redis
}

// repoRoot finds the repository root from this file's location, so the test
// does not depend on the directory `go test` happens to run in.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the source file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// bootServer starts the dependency graph the way cmd/server/main.go does, up to
// (not including) binding a socket: config from ./config, PostgreSQL from
// pgtest, local file storage in a temp dir, the built-in PostgreSQL retrieval
// engine, the docs module on.
func bootServer(t *testing.T, opts bootOptions) *server {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// config.LoadConfig reads ./config/config.yaml relative to the working
	// directory and offers no override, so run from the repository root. The
	// tests using this helper must therefore not run in parallel.
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoRoot(t)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })

	db, dsn := pgtest.NewURL(t)
	pointEnvAt(t, dsn)
	t.Setenv("RETRIEVE_DRIVER", "postgres")
	t.Setenv("STORAGE_TYPE", "local")
	t.Setenv("LOCAL_STORAGE_BASE_DIR", t.TempDir())
	t.Setenv("JWT_SECRET", testJWTSecret)
	t.Setenv("SYSTEM_AES_KEY", testAESKey)
	t.Setenv("YUHENG_DOCS_ENABLED", "true")
	t.Setenv("YUHENG_COLLAB_URL", "")
	// Nothing in these tests may reach a real docreader or the network.
	t.Setenv("DOCREADER_ADDR", "")
	t.Setenv("YUHENG_AUTH_REGISTRATION_MODE", "")
	t.Setenv("DISABLE_REGISTRATION", "")
	t.Setenv("REDIS_ADDR", "")

	var mr *miniredis.Miniredis
	if opts.redis {
		mr = miniredis.RunT(t)
		t.Setenv("REDIS_ADDR", mr.Addr())
	}
	for k, v := range opts.env {
		t.Setenv(k, v)
	}

	di := dig.New()
	// BuildContainer panics on a wiring or configuration error, as it does in
	// main(). Report that as a test failure rather than killing the whole test
	// binary, so the message reaches the person reading it.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("the server cannot start: %v", r)
			}
		}()
		BuildContainer(di)
	}()

	s := &server{t: t, DI: di, DB: db, Redis: mr}
	must1(t, di.Invoke(func(e *gin.Engine) { s.Engine = e }))
	t.Cleanup(func() {
		// Stop the docs module's background loops before the database goes.
		_ = di.Invoke(func(m *docs.Module) { _ = m.Close() })
		_ = di.Invoke(func(c interfaces.ResourceCleaner) { _ = c.Cleanup(t.Context()) })
	})
	return s
}

func must1(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%v", err)
	}
}

// response is what a request through the router produced.
type response struct {
	Code   int
	Header http.Header
	Body   []byte
}

// JSON decodes the body into a map for field assertions.
func (r response) JSON(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		t.Fatalf("response is not a JSON object (status %d): %v\n%s", r.Code, err, r.Body)
	}
	return m
}

// request describes one call through the router.
type request struct {
	method, path string
	body         any
	bearer       string
	apiKey       string
	tenantHeader string
	// ip is the client address. The credential endpoints have a per-IP budget
	// per minute, so a test that needs many attempts uses an address of its own.
	ip string
}

// do sends the request through the real router.
func (s *server) do(r request) response {
	s.t.Helper()
	var rd *bytes.Reader
	if r.body != nil {
		b, err := json.Marshal(r.body)
		if err != nil {
			s.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(r.method, r.path, rd)
	if r.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if r.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+r.bearer)
	}
	if r.apiKey != "" {
		req.Header.Set("X-API-Key", r.apiKey)
	}
	if r.tenantHeader != "" {
		req.Header.Set("X-Tenant-ID", r.tenantHeader)
	}
	ip := r.ip
	if ip == "" {
		ip = "203.0.113.1"
	}
	// A public address: 127.0.0.0/8 and the private ranges are trusted proxies,
	// and would let the header-derived address differ from the peer.
	req.RemoteAddr = fmt.Sprintf("%s:40000", ip)
	w := httptest.NewRecorder()
	s.Engine.ServeHTTP(w, req)
	return response{Code: w.Code, Header: w.Header(), Body: w.Body.Bytes()}
}
