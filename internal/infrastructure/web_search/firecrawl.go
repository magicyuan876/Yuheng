package web_search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

const (
	// defaultFirecrawlSearchURL is the hardcoded official Firecrawl v2 search API URL.
	// Not configurable by tenants — prevents SSRF.
	defaultFirecrawlSearchURL = "https://api.firecrawl.dev/v2/search"
	defaultFirecrawlTimeout   = 30 * time.Second
	defaultFirecrawlResults   = 10
	maxFirecrawlResults       = 100
	maxFirecrawlResponseBytes = 8 << 20
	maxFirecrawlContentRunes  = 12000
	firecrawlScrapeContentKey = "scrape_content"
	firecrawlMaxQueryLength   = 500
)

// FirecrawlProvider implements web search using the official Firecrawl Search API (v2).
type FirecrawlProvider struct {
	client        *http.Client
	baseURL       string
	apiKey        string
	scrapeContent bool
}

// NewFirecrawlProvider creates a Firecrawl provider from tenant-specific parameters.
func NewFirecrawlProvider(params types.WebSearchProviderParameters) (interfaces.WebSearchProvider, error) {
	if err := ValidateFirecrawlParameters(params); err != nil {
		return nil, err
	}
	client, err := NewSearchHTTPClient(defaultFirecrawlTimeout, params.ProxyURL)
	if err != nil {
		return nil, err
	}
	return &FirecrawlProvider{
		client:        client,
		baseURL:       defaultFirecrawlSearchURL,
		apiKey:        strings.TrimSpace(params.APIKey),
		scrapeContent: parseExaBool(params.ExtraConfig, firecrawlScrapeContentKey),
	}, nil
}

// ValidateFirecrawlParameters validates the tenant-supplied Firecrawl configuration.
func ValidateFirecrawlParameters(params types.WebSearchProviderParameters) error {
	if strings.TrimSpace(params.APIKey) == "" {
		return fmt.Errorf("API key is required for Firecrawl provider")
	}
	if raw := strings.TrimSpace(params.ExtraConfig[firecrawlScrapeContentKey]); raw != "" {
		if _, err := strconv.ParseBool(raw); err != nil {
			return fmt.Errorf("invalid Firecrawl %s value: %s", firecrawlScrapeContentKey, raw)
		}
	}
	return nil
}

// Name returns the provider type identifier.
func (p *FirecrawlProvider) Name() string { return "firecrawl" }

// Search performs a web search through the official Firecrawl Search API.
func (p *FirecrawlProvider) Search(
	ctx context.Context,
	query string,
	maxResults int,
	includeDate bool,
) ([]*types.WebSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is empty")
	}
	if len(query) > firecrawlMaxQueryLength {
		query = query[:firecrawlMaxQueryLength]
	}
	if maxResults <= 0 {
		maxResults = defaultFirecrawlResults
	}
	if maxResults > maxFirecrawlResults {
		maxResults = maxFirecrawlResults
	}

	reqBody := firecrawlSearchRequest{
		Query:   query,
		Limit:   maxResults,
		Sources: []firecrawlSource{{Type: "web"}},
	}
	if p.scrapeContent {
		reqBody.ScrapeOptions = &firecrawlScrapeOptions{
			Formats:         []string{"markdown"},
			OnlyMainContent: true,
		}
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Firecrawl request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create Firecrawl request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	logger.Infof(ctx, "[WebSearch][Firecrawl] query=%q maxResults=%d scrapeContent=%t", query, maxResults, p.scrapeContent)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Firecrawl request: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxFirecrawlResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read Firecrawl response: %w", err)
	}
	if len(respBody) > maxFirecrawlResponseBytes {
		return nil, fmt.Errorf("Firecrawl response exceeds %d bytes", maxFirecrawlResponseBytes)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		logger.Warnf(ctx, "[WebSearch][Firecrawl] API returned status %d: %s", resp.StatusCode, string(respBody))
		return nil, firecrawlHTTPError(resp.StatusCode, respBody)
	}

	var response firecrawlSearchResponse
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Firecrawl response: %w", err)
	}
	if !response.Success {
		detail := strings.TrimSpace(response.Error)
		if detail == "" {
			detail = "unknown error"
		}
		return nil, fmt.Errorf("Firecrawl API error: %s", detail)
	}

	results := make([]*types.WebSearchResult, 0, len(response.Data.Web))
	for _, item := range response.Data.Web {
		if strings.TrimSpace(item.Title) == "" && strings.TrimSpace(item.URL) == "" {
			continue
		}
		result := &types.WebSearchResult{
			Title:   item.Title,
			URL:     item.URL,
			Snippet: strings.TrimSpace(item.Description),
			Content: truncateExaText(strings.TrimSpace(item.Markdown), maxFirecrawlContentRunes),
			Source:  "firecrawl",
		}
		if includeDate {
			if publishedAt, ok := parseFirecrawlDate(item.Date); ok {
				result.PublishedAt = &publishedAt
			}
		}
		results = append(results, result)
		if len(results) >= maxResults {
			break
		}
	}
	logger.Infof(ctx, "[WebSearch][Firecrawl] returned %d results", len(results))
	return results, nil
}

func firecrawlHTTPError(statusCode int, body []byte) error {
	var apiError struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &apiError) == nil {
		detail := strings.TrimSpace(apiError.Error)
		if detail == "" {
			detail = strings.TrimSpace(apiError.Message)
		}
		if detail != "" {
			return fmt.Errorf("Firecrawl API returned status %d: %s", statusCode, detail)
		}
	}
	detail := strings.TrimSpace(string(body))
	if len(detail) > 4096 {
		detail = detail[:4096]
	}
	if detail == "" {
		return fmt.Errorf("Firecrawl API returned status %d", statusCode)
	}
	return fmt.Errorf("Firecrawl API returned status %d: %s", statusCode, detail)
}

func parseFirecrawlDate(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

type firecrawlSearchRequest struct {
	Query         string                  `json:"query"`
	Limit         int                     `json:"limit"`
	Sources       []firecrawlSource       `json:"sources"`
	ScrapeOptions *firecrawlScrapeOptions `json:"scrapeOptions,omitempty"`
}

type firecrawlSource struct {
	Type string `json:"type"`
}

type firecrawlScrapeOptions struct {
	Formats         []string `json:"formats"`
	OnlyMainContent bool     `json:"onlyMainContent"`
}

type firecrawlSearchResponse struct {
	Success bool                `json:"success"`
	Error   string              `json:"error,omitempty"`
	Data    firecrawlSearchData `json:"data"`
}

type firecrawlSearchData struct {
	Web []firecrawlWebResult `json:"web"`
}

type firecrawlWebResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Markdown    string `json:"markdown,omitempty"`
	Date        string `json:"date,omitempty"`
}
