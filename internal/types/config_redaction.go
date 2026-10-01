package types

import "strings"

// WebSearchConfigForResponse returns a copy safe for HTTP responses.
// When maskSecrets is true, a configured proxy_url is replaced with
// RedactedSecretPlaceholder.
func WebSearchConfigForResponse(cfg *WebSearchConfig, maskSecrets bool) *WebSearchConfig {
	if cfg == nil {
		return nil
	}
	out := *EffectiveWebSearchConfig(cfg)
	if !maskSecrets {
		return &out
	}
	if strings.TrimSpace(out.ProxyURL) != "" {
		out.ProxyURL = RedactedSecretPlaceholder
	}
	return &out
}

// ParserEngineConfigForResponse returns a copy with secret fields redacted
// when maskSecrets is true.
func ParserEngineConfigForResponse(cfg *ParserEngineConfig, maskSecrets bool) *ParserEngineConfig {
	if cfg == nil {
		return nil
	}
	out := *cfg
	if !maskSecrets {
		return &out
	}
	if out.MinerUAPIKey != "" {
		out.MinerUAPIKey = RedactedSecretPlaceholder
	}
	if out.PaddleOCRVLCloudToken != "" {
		out.PaddleOCRVLCloudToken = RedactedSecretPlaceholder
	}
	if out.MinerUTianshuAPIKey != "" {
		out.MinerUTianshuAPIKey = RedactedSecretPlaceholder
	}
	return &out
}

// CredentialsConfigForResponse returns a copy with app_secret redacted when
// maskSecrets is true.
func CredentialsConfigForResponse(cfg *CredentialsConfig, maskSecrets bool) *CredentialsConfig {
	if cfg == nil {
		return nil
	}
	out := *cfg
	if !maskSecrets {
		return &out
	}
	return &out
}

// MergeWebSearchConfigForUpdate applies preserve semantics to secret fields on
// tenant KV PUT.
func MergeWebSearchConfigForUpdate(incoming, existing *WebSearchConfig) *WebSearchConfig {
	out := *EffectiveWebSearchConfig(incoming)
	var prev WebSearchConfig
	if existing != nil {
		prev = *EffectiveWebSearchConfig(existing)
	}
	out.ProxyURL = PreserveIfRedacted(out.ProxyURL, prev.ProxyURL)
	return &out
}

// MergeParserEngineConfigForUpdate applies preserve semantics to secret fields
// on tenant KV PUT.
func MergeParserEngineConfigForUpdate(incoming, existing *ParserEngineConfig) *ParserEngineConfig {
	if incoming == nil {
		return nil
	}
	out := *incoming
	var prev ParserEngineConfig
	if existing != nil {
		prev = *existing
	}
	out.MinerUAPIKey = PreserveIfRedacted(out.MinerUAPIKey, prev.MinerUAPIKey)
	out.PaddleOCRVLCloudToken = PreserveIfRedacted(out.PaddleOCRVLCloudToken, prev.PaddleOCRVLCloudToken)
	out.MinerUTianshuAPIKey = PreserveIfRedacted(out.MinerUTianshuAPIKey, prev.MinerUTianshuAPIKey)
	// Preserve the stored chat attachment parser rules when the settings UI
	// omits this field on engine updates, so saving the engine form cannot
	// silently wipe them.
	if incoming.ChatParserEngineRules == nil && existing != nil {
		out.ChatParserEngineRules = existing.ChatParserEngineRules
	}
	return &out
}
