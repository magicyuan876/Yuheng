package router

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// The docs module's API contract: every route it registers is either
// documented in the generated Swagger spec or is a known placeholder.
//
// This exists because of a failure that was silent in both directions.
// `swag init` excludes its own OUTPUT directory by name, so scanning "./"
// skipped every package under internal/docs — the whole module was missing
// from the published spec with no error anywhere, for four work packages.
// The Makefile now names internal/docs/handler explicitly; this test is what
// notices if that argument is ever dropped again, or if somebody adds a route
// and forgets the annotation.
//
// The placeholder list is the other half. A route left at 501 is a promise
// about work not yet done, and listing them here means finishing that work
// forces somebody to come back and delete the entry.

// docsPlaceholderRoutes are the endpoints still answering 501, each with the
// work package that will implement it. Deleting an entry is part of doing it.
var docsPlaceholderRoutes = map[string]string{}

// swaggerSpec is the generated contract, read from the repository root.
type swaggerSpec struct {
	Paths map[string]map[string]json.RawMessage `json:"paths"`
}

func loadSwaggerSpec(t *testing.T) swaggerSpec {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate this test file")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	raw, err := os.ReadFile(filepath.Join(root, "docs", "swagger.json"))
	if err != nil {
		t.Fatalf("read swagger.json: %v", err)
	}
	var spec swaggerSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse swagger.json: %v", err)
	}
	return spec
}

// specKey turns a gin route into the way Swagger spells it:
// /api/v1/docs/pages/:pid -> /docs/pages/{pid}, since the spec's basePath is
// /api/v1.
func specKey(path string) string {
	trimmed := strings.TrimPrefix(path, "/api/v1")
	parts := strings.Split(trimmed, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + strings.TrimPrefix(part, ":") + "}"
		}
	}
	return strings.Join(parts, "/")
}

func docsRouteEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterDocsRoutes(engine.Group("/api/v1"), newDocsTestModule(), &rbacGuards{})
	return engine
}

// isPlaceholder reports whether a route's final handler is the 501 stub.
func isPlaceholder(handlerName string) bool {
	want := runtime.FuncForPC(
		reflect.ValueOf(dochandlerNotImplemented).Pointer()).Name()
	return handlerName == want
}

func TestEveryImplementedDocsRouteIsInTheSwaggerSpec(t *testing.T) {
	spec := loadSwaggerSpec(t)
	engine := docsRouteEngine()

	var missing []string
	for _, route := range engine.Routes() {
		key := route.Method + " " + route.Path
		if _, known := docsPlaceholderRoutes[key]; known {
			continue
		}
		if isPlaceholder(route.Handler) {
			t.Errorf("%s answers 501 but is not listed in docsPlaceholderRoutes; "+
				"add it with the work package that will implement it", key)
			continue
		}
		ops, ok := spec.Paths[specKey(route.Path)]
		if !ok {
			missing = append(missing, key)
			continue
		}
		if _, ok := ops[strings.ToLower(route.Method)]; !ok {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("%d implemented routes are missing from docs/swagger.json:\n  %s\n\n"+
			"Regenerate with `make docs`. If the whole module is missing, check that the "+
			"swag command still passes -d ./,./internal/docs/handler — swag excludes its "+
			"output directory by name and silently skips internal/docs without it.",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// The other direction: a listed placeholder that quietly started working
// would leave a stale promise in the list above.
func TestListedPlaceholdersAreStillPlaceholders(t *testing.T) {
	engine := docsRouteEngine()
	registered := map[string]string{}
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = route.Handler
	}

	for key, owner := range docsPlaceholderRoutes {
		handler, ok := registered[key]
		if !ok {
			t.Errorf("%s is listed as a placeholder for %s but is not registered at all; "+
				"remove the entry", key, owner)
			continue
		}
		if !isPlaceholder(handler) {
			t.Errorf("%s is implemented now — remove it from docsPlaceholderRoutes (%s)", key, owner)
		}
	}
}

// A sanity bound: if the spec ever loses the module wholesale, this says so
// in one line rather than as ninety separate failures.
func TestTheSpecDocumentsTheDocsModule(t *testing.T) {
	spec := loadSwaggerSpec(t)
	count := 0
	for path := range spec.Paths {
		if strings.HasPrefix(path, "/docs/") {
			count++
		}
	}
	if count < 50 {
		t.Fatalf("docs/swagger.json documents only %d /docs/ paths; the online-documents "+
			"module is missing from the generated spec. See the note on the docs target "+
			"in the Makefile.", count)
	}
}
