package client

import (
	"context"
	"net/http"
)

// SystemInfo represents system version and configuration information
type SystemInfo struct {
	Version             string `json:"version"`
	Edition             string `json:"edition"`
	CommitID            string `json:"commit_id,omitempty"`
	BuildTime           string `json:"build_time,omitempty"`
	GoVersion           string `json:"go_version,omitempty"`
	KeywordIndexEngine  string `json:"keyword_index_engine,omitempty"`
	VectorStoreEngine   string `json:"vector_store_engine,omitempty"`
	GraphDatabaseEngine string `json:"graph_database_engine,omitempty"`
	DBVersion           string `json:"db_version,omitempty"`
	DBMigrationError    string `json:"db_migration_error,omitempty"`
	StartedAt           string `json:"started_at,omitempty"`
	UptimeSeconds       int64  `json:"uptime_seconds,omitempty"`
}

// ParserEngine represents a document parser engine
type ParserEngine struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Available   bool   `json:"available"`
}

// DeploymentCapability describes whether a deployment exposes a feature route.
type DeploymentCapability struct {
	Supported bool   `json:"supported"`
	Reason    string `json:"reason,omitempty"`
}

// DeploymentCapabilitiesData is the payload of GET /system/capabilities.
type DeploymentCapabilitiesData struct {
	Edition      string                          `json:"edition"`
	Capabilities map[string]DeploymentCapability `json:"capabilities"`
}

// GetSystemInfo gets system version and configuration information
func (c *Client) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/system/info", nil, nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Code int         `json:"code"`
		Data *SystemInfo `json:"data"`
	}
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

// GetDeploymentCapabilities returns the deployment feature snapshot for SPA menu gating.
func (c *Client) GetDeploymentCapabilities(ctx context.Context) (*DeploymentCapabilitiesData, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/system/capabilities", nil, nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Code int                         `json:"code"`
		Data *DeploymentCapabilitiesData `json:"data"`
	}
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

// ListParserEngines lists available document parser engines
func (c *Client) ListParserEngines(ctx context.Context) ([]ParserEngine, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/system/parser-engines", nil, nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Code      int            `json:"code"`
		Data      []ParserEngine `json:"data"`
		Connected bool           `json:"connected"`
	}
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

// CheckParserEngines checks parser engine availability with given config overrides
func (c *Client) CheckParserEngines(ctx context.Context, config any) ([]ParserEngine, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/system/parser-engines/check", config, nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		Code int            `json:"code"`
		Data []ParserEngine `json:"data"`
	}
	if err := parseResponse(resp, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

// ReconnectDocReader reconnects the document parser service to a new address
func (c *Client) ReconnectDocReader(ctx context.Context, addr string) error {
	req := map[string]string{"addr": addr}
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/system/docreader/reconnect", req, nil)
	if err != nil {
		return err
	}
	return parseResponse(resp, nil)
}
