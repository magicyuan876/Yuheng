package container

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/docs"
	"github.com/magicyuan876/yuheng/internal/router"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// TestServerStartsWithItsOwnDefaults boots the whole server with the
// configuration the shipped config and .env.example describe, in both
// deployment shapes, and checks that it comes up healthy.
//
// It would have caught the release-blocking bug where config validation
// rejected the registration mode "auto" that the server itself defaults to:
// the process died in LoadConfig before serving anything, and no test loaded
// the real config file through the real validation.
func TestServerStartsWithItsOwnDefaults(t *testing.T) {
	shapes := []struct {
		name  string
		redis bool
	}{
		{"single process, no Redis", false},
		{"distributed, Redis", true},
	}
	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			s := bootServer(t, bootOptions{redis: shape.redis})

			// The probes an orchestrator uses. /ready also proves the database
			// answers and the migration state is clean.
			for _, path := range []string{"/health", "/ready"} {
				if got := s.do(request{method: http.MethodGet, path: path}); got.Code != http.StatusOK {
					t.Errorf("GET %s = %d, want 200: %s", path, got.Code, got.Body)
				}
			}

			// The default registration mode is one config validation accepts.
			must1(t, s.DI.Invoke(func(cfg *config.Config) {
				if mode := cfg.Auth.RegistrationMode; mode != config.AuthRegistrationModeAuto {
					t.Errorf("registration mode = %q, want the default %q", mode, config.AuthRegistrationModeAuto)
				}
			}))

			// Redis is the one optional dependency whose presence depends on
			// the shape: absent (nil) exactly when REDIS_ADDR is unset.
			must1(t, s.DI.Invoke(func(rc *redis.Client) {
				if shape.redis && rc == nil {
					t.Error("REDIS_ADDR is set but the container has no Redis client")
				}
				if !shape.redis && rc != nil {
					t.Error("REDIS_ADDR is unset but the container built a Redis client")
				}
			}))

			// The docs module. Its dependencies are all dig-optional, so a
			// missing binding does not fail the graph; it silently turns
			// features off. Degraded lists what arrived nil.
			must1(t, s.DI.Invoke(func(m *docs.Module) {
				if m == nil || !m.Enabled {
					t.Fatal("docs module is not enabled although YUHENG_DOCS_ENABLED=true")
				}
				if len(m.Degraded) != 0 {
					t.Errorf("docs module runs without %v: those features are silently off", m.Degraded)
				}
				if m.Services == nil || m.Handler == nil || m.Indexer == nil {
					t.Error("docs module was assembled without its services, handler or indexer")
				}
			}))

			// The docs routes exist only when the module is enabled; they must
			// be behind authentication.
			got := s.do(request{method: http.MethodGet, path: "/api/v1/docs/spaces"})
			if got.Code != http.StatusUnauthorized {
				t.Errorf("GET /api/v1/docs/spaces without credentials = %d, want 401", got.Code)
			}
			assertRoutePresent(t, s.Engine, http.MethodGet, "/api/v1/docs/spaces")

			// Knowledge health: its routes are mounted, the page-level view
			// is part of the docs module, and without Redis the check task
			// has a handler — otherwise every enqueue after indexing would
			// fail with "no handler registered" and nothing would be found.
			assertRoutePresent(t, s.Engine, http.MethodGet, "/api/v1/knowledge-bases/:id/findings")
			assertRoutePresent(t, s.Engine, http.MethodGet, "/api/v1/docs/pages/:pid/findings")
			if !shape.redis {
				must1(t, s.DI.Invoke(func(e *router.SyncTaskExecutor) {
					if !e.Handles(types.TypeKnowledgeFindings) {
						t.Error("the Redis-less executor has no handler for the knowledge-health check")
					}
				}))
			}

			// Anything else the router needs must resolve too; this is the
			// list of interfaces the docs bridge and the knowledge mirror
			// depend on, resolved from the same container.
			must1(t, s.DI.Invoke(func(
				_ interfaces.KnowledgeService,
				_ interfaces.StorageBackendResolver,
				_ interfaces.TenantRepository,
				_ interfaces.TaskEnqueuer,
			) {
			}))
		})
	}
}

func assertRoutePresent(t *testing.T, e *gin.Engine, method, path string) {
	t.Helper()
	for _, r := range e.Routes() {
		if r.Method == method && r.Path == path {
			return
		}
	}
	t.Errorf("route %s %s is not registered", method, path)
}

// knownOptionalDependencies is every `optional:"true"` field in non-test code,
// and what stands behind it. A dig-optional dependency that has no provider is
// handed over as nil and nothing fails, so each one must either be asserted
// present by a start-up test or be listed here with the reason that absence is
// fine. Adding a new optional dependency fails TestOptionalDependencies until
// someone decides which it is.
var knownOptionalDependencies = map[string]string{
	// Asserted through docs.Module.Degraded (must be empty) in
	// TestServerStartsWithItsOwnDefaults: every one of these has a provider in
	// BuildContainer.
	"docs.Params.UserService":          "asserted via Module.Degraded",
	"docs.Params.Audit":                "asserted via Module.Degraded",
	"docs.Params.KnowledgeBases":       "asserted via Module.Degraded",
	"docs.Params.StorageBackends":      "asserted via Module.Degraded",
	"docs.Params.StorageResolver":      "asserted via Module.Degraded",
	"docs.Params.Tenants":              "asserted via Module.Degraded",
	"docs.Params.Favourites":           "asserted via Module.Degraded",
	"docs.Params.KnowledgeBaseService": "asserted via Module.Degraded",
	"docs.Params.ModelService":         "asserted via Module.Degraded",
	"docs.Params.KnowledgeService":     "asserted via Module.Degraded",
	"docs.Params.Findings":             "asserted via Module.Degraded",
	"docs.Params.Stewardship":          "asserted via Module.Degraded",
	"docs.Params.Retirers":             "asserted via Module.Degraded",
	// Legitimately absent: single-process mode has no Redis. The container
	// provides a nil client then, and the test above asserts nil-iff-unset.
	"docs.Params.Redis": "absent by design without REDIS_ADDR; asserted in both shapes",
	// The router takes the module optionally so tests can build a router
	// without one; the container always provides it (enabled or not) and the
	// test above resolves it and requires it to be enabled.
	"router.RouterParams.DocsModule": "always provided by the container; asserted enabled",
}

// TestOptionalDependencies keeps knownOptionalDependencies honest: the
// declared set must match the source exactly, in both directions.
func TestOptionalDependencies(t *testing.T) {
	root := repoRoot(t)
	found := map[string]bool{}
	fset := token.NewFileSet()
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(f, func(n ast.Node) bool {
				ts, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					return true
				}
				for _, field := range st.Fields.List {
					if field.Tag == nil || !strings.Contains(field.Tag.Value, `optional:"true"`) {
						continue
					}
					for _, name := range field.Names {
						found[f.Name.Name+"."+ts.Name.Name+"."+name.Name] = true
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	var unexpected, stale []string
	for k := range found {
		if _, ok := knownOptionalDependencies[k]; !ok {
			unexpected = append(unexpected, k)
		}
	}
	for k := range knownOptionalDependencies {
		if !found[k] {
			stale = append(stale, k)
		}
	}
	sort.Strings(unexpected)
	sort.Strings(stale)
	if len(unexpected) > 0 {
		t.Errorf("new dig optional:\"true\" dependencies %v: a missing provider degrades silently. "+
			"Assert them in TestServerStartsWithItsOwnDefaults or list them in knownOptionalDependencies "+
			"with the reason that absence is acceptable", unexpected)
	}
	if len(stale) > 0 {
		t.Errorf("knownOptionalDependencies lists fields that no longer exist: %v", stale)
	}
}
