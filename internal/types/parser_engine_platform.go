package types

import (
	"context"
	"sync/atomic"
)

// Platform-level parser engine configuration.
//
// Parser engine settings resolve in three layers, lowest precedence first:
//
//	ENV (deployment)  <  platform default (system admin)  <  workspace override
//
// The ENV layer is applied inside DocReader — each engine reads its own
// variables — so an override we do not emit leaves the deployment value in
// force. This file collapses the two DB layers into the override map DocReader
// receives.
//
// Why a registered hook rather than constructor injection: the platform layer
// is read from system_settings, which lives in the service package, while the
// merge belongs next to ParserEngineConfig here in types — and types cannot
// import interfaces without a cycle. The same shape is already used for the
// runtime-tunable upload limits (utils.RegisterMaxFileSizeResolvers, wired in
// cmd/server/bootstrap.go), and it means the six existing call sites become
// layer-aware without threading a new dependency through four constructors.
//
// Unregistered (tests, lite wiring gaps) resolves to "no platform layer",
// which is exactly the pre-existing two-layer behaviour.
var platformParserEngineDefault atomic.Pointer[func(context.Context) *ParserEngineConfig]

// RegisterPlatformParserEngineDefault installs the platform-layer resolver.
// Called once at startup. Passing nil clears it (used by tests).
func RegisterPlatformParserEngineDefault(fn func(context.Context) *ParserEngineConfig) {
	if fn == nil {
		platformParserEngineDefault.Store(nil)
		return
	}
	platformParserEngineDefault.Store(&fn)
}

// PlatformParserEngineDefault returns the platform-level configuration, or nil
// when none is configured or no resolver has been registered.
func PlatformParserEngineDefault(ctx context.Context) *ParserEngineConfig {
	fn := platformParserEngineDefault.Load()
	if fn == nil {
		return nil
	}
	return (*fn)(ctx)
}

// ResolveParserEngineOverrides merges the platform default with the
// workspace's own override and returns the map handed to DocReader.
//
// Merging happens on the override MAP, not on the struct: ToOverridesMap omits
// unset fields by construction, so "platform map, workspace map laid over it"
// expresses the intended precedence exactly. A struct-level merge would mean
// hand-writing ~40 field comparisons and re-deriving, for each one, whether a
// zero value means "unset" or "deliberately false" — the pointer-typed bools
// in ParserEngineConfig exist precisely because that distinction matters.
//
// Returns nil when neither layer contributes anything, matching what these
// call sites produced before the platform layer existed.
func ResolveParserEngineOverrides(ctx context.Context, tenant *Tenant) map[string]string {
	var own map[string]string
	if tenant != nil {
		own = tenant.ParserEngineConfig.ToOverridesMap()
	}
	platform := PlatformParserEngineDefault(ctx).ToOverridesMap()

	if len(platform) == 0 {
		return own
	}
	if len(own) == 0 {
		return platform
	}

	merged := make(map[string]string, len(platform)+len(own))
	for k, v := range platform {
		merged[k] = v
	}
	// Field by field, so a workspace that overrode only its MinerU endpoint
	// still inherits the platform's Tianshu and PaddleOCR settings instead of
	// losing every engine it did not mention.
	for k, v := range own {
		merged[k] = v
	}
	return merged
}

// ResolveParserEngineOverridesFromContext is ResolveParserEngineOverrides for
// call sites that carry the tenant on the context rather than in hand.
func ResolveParserEngineOverridesFromContext(ctx context.Context) map[string]string {
	tenant, _ := ctx.Value(TenantInfoContextKey).(*Tenant)
	return ResolveParserEngineOverrides(ctx, tenant)
}

// ResolveChatParserEngineFor picks the parser engine for a session-scoped chat
// attachment of the given extension: workspace rules first, platform rules as
// the fallback. Empty means "let DocReader decide".
func ResolveChatParserEngineFor(ctx context.Context, tenant *Tenant, ext string) string {
	if tenant != nil && tenant.ParserEngineConfig != nil {
		if engine := tenant.ParserEngineConfig.ResolveChatParserEngine(ext); engine != "" {
			return engine
		}
	}
	return PlatformParserEngineDefault(ctx).ResolveChatParserEngine(ext)
}
