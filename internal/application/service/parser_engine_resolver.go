package service

import (
	"context"
	"encoding/json"

	apperrors "github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// errNoSystemSettingService is returned when a write is attempted on a
// deployment where the system-setting service was not wired. Reads degrade
// silently (no platform layer); writes must fail loudly, because reporting
// success while persisting nothing would leave the operator believing the
// platform default is in force.
var errNoSystemSettingService = apperrors.NewInternalServerError(
	"system settings are unavailable; cannot save the platform parser configuration")

// parserEngineResolver implements interfaces.ParserEngineResolver.
//
// The merge itself lives in types.ResolveParserEngineOverrides, next to
// ParserEngineConfig; this type owns the two things that need the service
// layer: reading the platform row out of system_settings, and writing it back.
// The read half is also registered as the process-wide hook (see
// RegisterPlatformLayer) so the existing DocReader call sites become
// layer-aware without threading a dependency through four constructors.
type parserEngineResolver struct {
	settings interfaces.SystemSettingService
}

// NewParserEngineResolver builds the resolver. A nil setting service is
// tolerated and degrades to "no platform layer", which is the pre-existing
// two-layer behaviour — a wiring gap must not stop documents from parsing.
func NewParserEngineResolver(settings interfaces.SystemSettingService) interfaces.ParserEngineResolver {
	return &parserEngineResolver{settings: settings}
}

// RegisterPlatformLayer installs this resolver's read path as the process-wide
// platform layer consulted by types.ResolveParserEngineOverrides. Called once
// from the startup bootstrap.
func (r *parserEngineResolver) RegisterPlatformLayer() {
	types.RegisterPlatformParserEngineDefault(r.PlatformDefault)
}

// PlatformDefault returns the platform-level parser engine configuration, or
// nil when none is configured.
//
// Decoding failures are logged and treated as "not configured" rather than
// propagated: a malformed row must not take document parsing down, and the
// workspace/ENV layers still produce a working configuration.
func (r *parserEngineResolver) PlatformDefault(ctx context.Context) *types.ParserEngineConfig {
	if r == nil || r.settings == nil {
		return nil
	}
	row, err := r.settings.Get(ctx, types.SettingKeyParserEngineDefault)
	if err != nil || row == nil || len(row.Value) == 0 {
		return nil
	}
	var cfg types.ParserEngineConfig
	if err := json.Unmarshal(row.Value, &cfg); err != nil {
		logger.Warnf(ctx, "[parser_engine] platform default is not decodable, ignoring: %v", err)
		return nil
	}
	return &cfg
}

// ResolveOverrides merges the platform default with the workspace override.
func (r *parserEngineResolver) ResolveOverrides(
	ctx context.Context, tenant *types.Tenant,
) map[string]string {
	return types.ResolveParserEngineOverrides(ctx, tenant)
}

// ResolveChatParserEngine picks the parser engine for a session-scoped chat
// attachment of the given extension.
func (r *parserEngineResolver) ResolveChatParserEngine(
	ctx context.Context, tenant *types.Tenant, ext string,
) string {
	return types.ResolveChatParserEngineFor(ctx, tenant, ext)
}

// SetPlatformDefault persists the platform-level configuration.
//
// Secret preservation (a masked value must not overwrite the stored one) is
// applied by the caller through types.MergeParserEngineConfigForUpdate before
// reaching here, mirroring how the workspace-level KV endpoint does it.
func (r *parserEngineResolver) SetPlatformDefault(
	ctx context.Context, cfg *types.ParserEngineConfig,
) (*types.ParserEngineConfig, error) {
	if r.settings == nil {
		return nil, errNoSystemSettingService
	}
	if cfg == nil {
		cfg = &types.ParserEngineConfig{}
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	// Round-trip through map[string]any: the registry's "json" encoder accepts
	// an object, and marshalling the struct first guarantees we only ever
	// persist fields ParserEngineConfig actually declares.
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if _, err := r.settings.Update(ctx, types.SettingKeyParserEngineDefault, payload); err != nil {
		return nil, err
	}
	return cfg, nil
}
