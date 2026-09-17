package types

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withPlatformDefault installs a platform layer for the duration of one test
// and restores the previous state. The hook is process-wide, so leaking one
// would silently change every later test in the package.
func withPlatformDefault(t *testing.T, cfg *ParserEngineConfig) {
	t.Helper()
	RegisterPlatformParserEngineDefault(func(context.Context) *ParserEngineConfig { return cfg })
	t.Cleanup(func() { RegisterPlatformParserEngineDefault(nil) })
}

// Nothing registered is the state of every deployment that has not configured
// a platform default, and of every unit test. It must resolve to exactly the
// old two-layer behaviour rather than, say, an empty non-nil map — callers
// pass this straight to DocReader, where an empty override map and a nil one
// are not equivalent for every engine.
func TestResolveParserEngineOverrides_NoPlatformLayer(t *testing.T) {
	tenant := &Tenant{ParserEngineConfig: &ParserEngineConfig{MinerUEndpoint: "http://own:8000"}}

	got := ResolveParserEngineOverrides(context.Background(), tenant)
	assert.Equal(t, map[string]string{"mineru_endpoint": "http://own:8000"}, got)

	assert.Nil(t, ResolveParserEngineOverrides(context.Background(), &Tenant{}),
		"a workspace with no config and no platform layer contributes nothing")
}

// A workspace that never configured anything inherits the platform layer
// wholesale — this is the case that makes centralised mode usable at all.
func TestResolveParserEngineOverrides_PlatformOnly(t *testing.T) {
	withPlatformDefault(t, &ParserEngineConfig{
		MinerUEndpoint:        "http://platform:8000",
		MinerUTianshuEndpoint: "http://tianshu:8000",
	})

	got := ResolveParserEngineOverrides(context.Background(), &Tenant{})
	assert.Equal(t, "http://platform:8000", got["mineru_endpoint"])
	assert.Equal(t, "http://tianshu:8000", got["mineru_tianshu_endpoint"])
}

// The merge is field by field, not block by block. A workspace that overrode
// only its MinerU endpoint must keep inheriting every other engine's platform
// settings; replacing the whole block would silently disable Tianshu for them.
func TestResolveParserEngineOverrides_WorkspaceWinsPerField(t *testing.T) {
	withPlatformDefault(t, &ParserEngineConfig{
		MinerUEndpoint:        "http://platform:8000",
		MinerUTianshuEndpoint: "http://tianshu:8000",
		PaddleOCRVLEndpoint:   "http://paddle:8080",
	})
	tenant := &Tenant{ParserEngineConfig: &ParserEngineConfig{
		MinerUEndpoint: "http://own:9000",
	}}

	got := ResolveParserEngineOverrides(context.Background(), tenant)
	assert.Equal(t, "http://own:9000", got["mineru_endpoint"], "workspace wins on the field it set")
	assert.Equal(t, "http://tianshu:8000", got["mineru_tianshu_endpoint"], "and inherits the rest")
	assert.Equal(t, "http://paddle:8080", got["paddleocr_vl_endpoint"])
}

// The pointer-typed booleans exist so "unset" and "false" stay distinguishable.
// A workspace that deliberately turns formula parsing off must win over a
// platform default that turns it on — an ordinary zero-value merge would drop
// the false and silently re-enable it.
func TestResolveParserEngineOverrides_WorkspaceFalseBeatsPlatformTrue(t *testing.T) {
	withPlatformDefault(t, &ParserEngineConfig{MinerUEnableFormula: boolPtr(true)})
	tenant := &Tenant{ParserEngineConfig: &ParserEngineConfig{MinerUEnableFormula: boolPtr(false)}}

	got := ResolveParserEngineOverrides(context.Background(), tenant)
	assert.Equal(t, "false", got["mineru_enable_formula"])
}

func TestResolveParserEngineOverrides_UnsetWorkspaceBoolInheritsPlatform(t *testing.T) {
	withPlatformDefault(t, &ParserEngineConfig{MinerUEnableFormula: boolPtr(true)})
	tenant := &Tenant{ParserEngineConfig: &ParserEngineConfig{MinerUEndpoint: "http://own:9000"}}

	got := ResolveParserEngineOverrides(context.Background(), tenant)
	assert.Equal(t, "true", got["mineru_enable_formula"],
		"a field the workspace never mentioned must fall through to the platform")
}

// Merging must not mutate either source map; both come from ToOverridesMap,
// but a future caching change there would turn an in-place merge into
// cross-request contamination.
func TestResolveParserEngineOverrides_DoesNotMutateSources(t *testing.T) {
	platform := &ParserEngineConfig{MinerUEndpoint: "http://platform:8000"}
	withPlatformDefault(t, platform)
	tenant := &Tenant{ParserEngineConfig: &ParserEngineConfig{PaddleOCRVLEndpoint: "http://paddle:8080"}}

	merged := ResolveParserEngineOverrides(context.Background(), tenant)
	require.Len(t, merged, 2)
	assert.Equal(t, map[string]string{"mineru_endpoint": "http://platform:8000"},
		platform.ToOverridesMap())
	assert.Equal(t, map[string]string{"paddleocr_vl_endpoint": "http://paddle:8080"},
		tenant.ParserEngineConfig.ToOverridesMap())
}

func TestResolveChatParserEngineFor_WorkspaceThenPlatform(t *testing.T) {
	withPlatformDefault(t, &ParserEngineConfig{
		ChatParserEngineRules: []ParserEngineRule{
			{FileTypes: []string{"pdf"}, Engine: "platform-engine"},
			{FileTypes: []string{"docx"}, Engine: "platform-docx"},
		},
	})
	tenant := &Tenant{ParserEngineConfig: &ParserEngineConfig{
		ChatParserEngineRules: []ParserEngineRule{
			{FileTypes: []string{"pdf"}, Engine: "workspace-engine"},
		},
	}}

	ctx := context.Background()
	assert.Equal(t, "workspace-engine", ResolveChatParserEngineFor(ctx, tenant, "pdf"),
		"workspace rules take precedence")
	assert.Equal(t, "platform-docx", ResolveChatParserEngineFor(ctx, tenant, "docx"),
		"extensions the workspace has no rule for fall through to the platform")
	assert.Empty(t, ResolveChatParserEngineFor(ctx, tenant, "xlsx"),
		"no rule anywhere means let DocReader decide")
}

func TestResolveChatParserEngineFor_NilTenant(t *testing.T) {
	withPlatformDefault(t, &ParserEngineConfig{
		ChatParserEngineRules: []ParserEngineRule{{FileTypes: []string{"pdf"}, Engine: "platform-engine"}},
	})
	assert.Equal(t, "platform-engine", ResolveChatParserEngineFor(context.Background(), nil, "pdf"))
}

// A registered resolver that returns nil is the normal state before any system
// administrator saves a platform default, so it must behave exactly like no
// resolver at all.
func TestPlatformParserEngineDefault_NilResult(t *testing.T) {
	withPlatformDefault(t, nil)
	assert.Nil(t, PlatformParserEngineDefault(context.Background()))

	tenant := &Tenant{ParserEngineConfig: &ParserEngineConfig{MinerUEndpoint: "http://own:9000"}}
	assert.Equal(t, map[string]string{"mineru_endpoint": "http://own:9000"},
		ResolveParserEngineOverrides(context.Background(), tenant))
}
