package interfaces

import (
	"context"

	"github.com/magicyuan876/yuheng/internal/types"
)

// ParserEngineResolver collapses the layered parser engine configuration into
// the override map DocReader consumes.
//
// Three layers, lowest precedence first:
//
//	ENV (deployment)  <  platform default (system admin)  <  workspace override
//
// The ENV layer is applied inside DocReader itself — each engine reads its own
// variables — so an override this resolver does not emit leaves the deployment
// value in force. That is why "unset" and "set to empty" must stay
// distinguishable, and why merging happens on the override map rather than on
// the struct.
//
// Every call site that previously read tenant.ParserEngineConfig.ToOverridesMap()
// directly must go through here instead, or the platform layer is silently
// skipped on that path.
type ParserEngineResolver interface {
	// PlatformDefault returns the platform-level configuration, or nil when
	// none is set. A malformed stored value is treated as "not set" rather
	// than an error: parsing must not stop because one settings row is bad.
	PlatformDefault(ctx context.Context) *types.ParserEngineConfig
	// ResolveOverrides merges the platform default with the workspace's own
	// override, the workspace winning field by field. Returns nil when neither
	// layer contributes anything.
	ResolveOverrides(ctx context.Context, tenant *types.Tenant) map[string]string
	// ResolveChatParserEngine picks the engine for a chat attachment of the
	// given extension: workspace rules first, then platform rules.
	ResolveChatParserEngine(ctx context.Context, tenant *types.Tenant, ext string) string
	// SetPlatformDefault persists the platform-level configuration. Callers
	// must apply secret-preservation (types.MergeParserEngineConfigForUpdate)
	// before calling, so a masked placeholder never overwrites a stored key.
	SetPlatformDefault(ctx context.Context, cfg *types.ParserEngineConfig) (*types.ParserEngineConfig, error)
}
