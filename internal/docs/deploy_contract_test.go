package docs_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// The deployment contract of the docs module.
//
// Deployment files are the part of a feature nobody runs in CI and everybody
// edits: a profile renamed, a service renamed, a limit changed on one side.
// Each of the facts below is one that breaks the module silently — the app
// comes up, the page loads, and only the thing being tested is wrong — so
// each is asserted here rather than left to be discovered in production.
//
// These are contract tests over text, not a deployment. They cannot tell you
// the cluster works; they can tell you the files still say what the code
// assumes they say.

// repoRoot walks up from this file to the directory holding go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "cannot locate this test file")
	dir := filepath.Dir(thisFile)
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("cannot find the repository root")
	return ""
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(rel)))
	require.NoError(t, err, "read %s", rel)
	return string(body)
}

// composeService is one service, left untyped.
//
// Compose accepts two shapes for several of these keys -- environment as a
// list or a map, build as a string or a map -- and this file reads services
// written in both, so the accessors below do the narrowing rather than a
// struct that would fail to parse half the file.
type composeService map[string]any

func composeFile(t *testing.T, rel string) map[string]composeService {
	t.Helper()
	var doc struct {
		Services map[string]composeService `yaml:"services"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(readRepoFile(t, rel)), &doc), "parse %s", rel)
	return doc.Services
}

// profiles lists the Compose profiles a service belongs to.
func (s composeService) profiles() []string {
	raw, _ := s["profiles"].([]any)
	out := make([]string, 0, len(raw))
	for _, entry := range raw {
		if name, ok := entry.(string); ok {
			out = append(out, name)
		}
	}
	return out
}

// asMap narrows a nested mapping.
//
// It has to accept composeService as well as map[string]any: yaml.v3 decodes
// a nested mapping into the NAMED map type it is decoding into, so a bare
// `case map[string]any` in a type switch silently never matches here and
// every accessor quietly returns nothing.
func asMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	case composeService:
		return m
	}
	return nil
}

// env finds one environment entry by its variable name, in either shape.
func (s composeService) env(name string) (string, bool) {
	if block := asMap(s["environment"]); block != nil {
		value, ok := block[name]
		return toString(value), ok
	}
	switch block := s["environment"].(type) {
	case []any:
		for _, entry := range block {
			line, ok := entry.(string)
			if !ok {
				continue
			}
			if key, value, ok := strings.Cut(line, "="); ok && key == name {
				return value, true
			}
		}
	}
	return "", false
}

// build reads one key of the build section, which may also be a bare string
// naming the context.
func (s composeService) build(key string) string {
	if context, ok := s["build"].(string); ok {
		if key == "context" {
			return context
		}
		return ""
	}
	return toString(asMap(s["build"])[key])
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// The collaboration service is an optional upgrade to the docs module, not a
// prerequisite: without it the editor falls back to exclusive editing. It
// must therefore stay behind a profile, or a plain `docker compose up` would
// start a service most deployments do not want and would fail its healthcheck
// for want of a shared secret.
func TestTheCollabServiceIsNotStartedByAPlainComposeUp(t *testing.T) {
	for _, file := range []string{"docker-compose.yml", "docker-compose.dev.yml"} {
		services := composeFile(t, file)
		collab, ok := services["collab"]
		require.True(t, ok, "%s has no collab service", file)
		assert.Contains(t, collab.profiles(), "docs",
			"%s: collab must stay behind the docs profile", file)
	}
}

// The service name is load-bearing: the app addresses the collaboration
// service by it, and so does the frontend's proxy.
func TestTheAppCallsBackToTheCollabServiceByName(t *testing.T) {
	app := composeFile(t, "docker-compose.yml")["app"]
	value, ok := app.env("YUHENG_COLLAB_INTERNAL_URL")
	require.True(t, ok, "the app has no YUHENG_COLLAB_INTERNAL_URL")
	assert.Contains(t, value, "collab:1234",
		"the callback must reach the collab service directly, not through a proxy")
}

// Two different ceilings means one side accepting an update the other refuses
// to store, which surfaces as edits that vanish on reload.
func TestBothSidesReadTheSameDocumentSizeLimit(t *testing.T) {
	collab := composeFile(t, "docker-compose.yml")["collab"]
	value, ok := collab.env("COLLAB_MAX_YDOC_BYTES")
	require.True(t, ok, "the collab service has no COLLAB_MAX_YDOC_BYTES")
	assert.Contains(t, value, "YUHENG_DOCS_MAX_YDOC_BYTES",
		"the collab limit must come from the same variable the app reads")
}

// A required-variable reference (${VAR:?message}) is evaluated when Compose
// parses the file, not when the service starts, so one here would break
// `docker compose up` for everybody who never enables the docs profile.
func TestTheComposeFileDoesNotRequireTheCollabSecretFromEverybody(t *testing.T) {
	body := readRepoFile(t, "docker-compose.yml")
	assert.NotContains(t, body, "${YUHENG_COLLAB_SHARED_SECRET:?",
		"the secret is validated by the collab service itself; a required-variable "+
			"reference here would fail parsing for deployments that never use it")
}

// The build context is the repository root rather than collab/, because the
// service bundles the shared document schema from packages/docs-schema.
func TestTheCollabImageIsBuiltFromTheRepositoryRoot(t *testing.T) {
	for _, file := range []string{"docker-compose.yml", "docker-compose.dev.yml"} {
		collab := composeFile(t, file)["collab"]
		assert.Equal(t, ".", collab.build("context"), "%s", file)
		assert.Equal(t, "collab/Dockerfile", collab.build("dockerfile"), "%s", file)
	}
}

// ---- the frontend proxy ---------------------------------------------------

// A WebSocket that reaches nginx without these two headers is never upgraded,
// and the editor sits at "connecting" with no error anywhere.
func TestTheFrontendProxyUpgradesTheCollabConnection(t *testing.T) {
	conf := readRepoFile(t, "frontend/nginx.conf")
	start := strings.Index(conf, "location ^~ /collab")
	require.Positive(t, start, "nginx.conf has no /collab location")
	block := conf[start:]
	if end := strings.Index(block, "\n    location "); end > 0 {
		block = block[:end]
	}
	assert.Contains(t, block, "proxy_set_header Upgrade $http_upgrade")
	assert.Contains(t, block, `proxy_set_header Connection "upgrade"`)
	// An editing session holds one connection open for as long as the document
	// is on screen; the default 60s read timeout would cut it repeatedly.
	assert.Contains(t, block, "proxy_read_timeout 3600s")
}

// envsubst only substitutes the variables it is told to, so a variable added
// to the template and not to the list is emitted literally and nginx fails to
// start with a name-resolution error naming "${COLLAB_HOST}".
func TestTheProxyVariablesAreSubstitutedAtStartup(t *testing.T) {
	entrypoint := readRepoFile(t, "frontend/docker-entrypoint.sh")
	for _, name := range []string{"COLLAB_HOST", "COLLAB_PORT"} {
		assert.Contains(t, entrypoint, "export "+name+"=",
			"%s has no default in the entrypoint", name)
		assert.Contains(t, entrypoint, "${"+name+"}",
			"%s is missing from the envsubst variable list", name)
	}
}

// ---- the Helm chart -------------------------------------------------------

// The chart's two switches, and the direction of the dependency between them.
func TestTheChartKeepsDocsAndCollabSeparate(t *testing.T) {
	var values struct {
		Docs struct {
			Enabled      bool `yaml:"enabled"`
			MaxYDocBytes int  `yaml:"maxYDocBytes"`
		} `yaml:"docs"`
		Collab struct {
			Enabled      bool `yaml:"enabled"`
			ReplicaCount int  `yaml:"replicaCount"`
			Redis        struct {
				Enabled bool `yaml:"enabled"`
			} `yaml:"redis"`
		} `yaml:"collab"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(readRepoFile(t, "helm/values.yaml")), &values))

	assert.False(t, values.Docs.Enabled, "the docs module is off by default")
	assert.False(t, values.Collab.Enabled, "collaboration is off by default")
	assert.Positive(t, values.Docs.MaxYDocBytes)
	// A second replica without Redis sees a different copy of every document,
	// so the default must not be one that is wrong on its own.
	assert.Equal(t, 1, values.Collab.ReplicaCount)
	assert.False(t, values.Collab.Redis.Enabled)

	// Enabling collaboration without the module would deploy a service with
	// nothing to serve; the chart refuses rather than rendering it.
	template := readRepoFile(t, "helm/templates/collab.yaml")
	assert.Contains(t, template, "fail",
		"collab.yaml must refuse to render when docs.enabled is false")
	assert.Contains(t, template, ".Values.docs.enabled")

	// Kubernetes expands $(VAR) only against variables declared earlier in the
	// list, so the password has to come before the URL that interpolates it.
	if pwd, url := strings.Index(template, "name: REDIS_PASSWORD"),
		strings.Index(template, "name: COLLAB_REDIS_URL"); pwd > 0 && url > 0 {
		assert.Less(t, pwd, url,
			"REDIS_PASSWORD must be declared before COLLAB_REDIS_URL, or the URL "+
				"carries the literal string $(REDIS_PASSWORD)")
	}
}

// The app and the collaboration service must read the same limit here too.
func TestTheChartGivesBothSidesTheSameDocumentSizeLimit(t *testing.T) {
	assert.Contains(t, readRepoFile(t, "helm/templates/collab.yaml"),
		".Values.docs.maxYDocBytes",
		"the collab deployment must take its limit from the docs values")
	assert.Contains(t, readRepoFile(t, "helm/templates/app.yaml"),
		".Values.docs.maxYDocBytes",
		"the app must take its limit from the same place")
}
