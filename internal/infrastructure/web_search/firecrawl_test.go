package web_search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/magicyuan876/yuheng/internal/types"
)

func TestFirecrawlProviderSearchMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/search" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer fc-test" {
			t.Fatalf("Authorization = %q", got)
		}
		var request firecrawlSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Query != "hello" || request.Limit != 2 {
			t.Fatalf("request body = %+v", request)
		}
		if len(request.Sources) != 1 || request.Sources[0].Type != "web" {
			t.Fatalf("request sources = %+v", request.Sources)
		}
		if request.ScrapeOptions == nil || len(request.ScrapeOptions.Formats) != 1 ||
			request.ScrapeOptions.Formats[0] != "markdown" || !request.ScrapeOptions.OnlyMainContent {
			t.Fatalf("request scrapeOptions = %+v", request.ScrapeOptions)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(firecrawlSearchResponse{
			Success: true,
			Data: firecrawlSearchData{Web: []firecrawlWebResult{
				{
					Title:       "One",
					URL:         "https://example.com/1",
					Description: "first result",
					Markdown:    "# body",
					Date:        "2026-05-01T00:00:00Z",
				},
				{Title: "Two", URL: "https://example.com/2", Description: "second result"},
				{Title: "Three", URL: "https://example.com/3"},
			}},
		})
	}))
	defer srv.Close()

	p := &FirecrawlProvider{client: srv.Client(), baseURL: srv.URL + "/v2/search", apiKey: "fc-test", scrapeContent: true}
	results, err := p.Search(context.Background(), "hello", 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Snippet != "first result" ||
		results[0].Content != "# body" || results[0].Source != "firecrawl" {
		t.Fatalf("unexpected results: %+v", results)
	}
	if results[0].PublishedAt == nil || !results[0].PublishedAt.Equal(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected date: %v", results[0].PublishedAt)
	}
	if results[1].PublishedAt != nil || results[1].Content != "" {
		t.Fatalf("unexpected second result: %+v", results[1])
	}
}

func TestFirecrawlProviderOmitsScrapeOptionsByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request firecrawlSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.ScrapeOptions != nil {
			t.Fatalf("scrapeOptions should be omitted by default: %+v", request.ScrapeOptions)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(firecrawlSearchResponse{
			Success: true,
			Data: firecrawlSearchData{Web: []firecrawlWebResult{
				{Title: "Firecrawl", URL: "https://firecrawl.dev/"},
			}},
		})
	}))
	defer srv.Close()

	p := &FirecrawlProvider{client: srv.Client(), baseURL: srv.URL, apiKey: "fc-test"}
	results, err := p.Search(context.Background(), "test", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
}

func TestFirecrawlProviderValidationAndStatus(t *testing.T) {
	if _, err := NewFirecrawlProvider(types.WebSearchProviderParameters{}); err == nil {
		t.Fatal("expected missing API key error")
	}
	if _, err := NewFirecrawlProvider(types.WebSearchProviderParameters{
		APIKey:      "fc-test",
		ExtraConfig: map[string]string{"scrape_content": "sometimes"},
	}); err == nil {
		t.Fatal("expected invalid scrape_content error")
	}
	if _, err := NewFirecrawlProvider(types.WebSearchProviderParameters{
		APIKey:      "fc-test",
		ExtraConfig: map[string]string{"scrape_content": "true"},
	}); err != nil {
		t.Fatalf("valid parameters rejected: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer srv.Close()
	p := &FirecrawlProvider{client: srv.Client(), baseURL: srv.URL, apiKey: "key"}
	if _, err := p.Search(context.Background(), "q", 1, false); err == nil {
		t.Fatal("expected status error")
	}
	if _, err := p.Search(context.Background(), " ", 1, false); err == nil {
		t.Fatal("expected empty query error")
	}
}

func TestFirecrawlProviderSuccessFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"error":"invalid token"}`))
	}))
	defer srv.Close()
	p := &FirecrawlProvider{client: srv.Client(), baseURL: srv.URL, apiKey: "key"}
	if _, err := p.Search(context.Background(), "q", 1, false); err == nil {
		t.Fatal("expected API error for success=false")
	}
}
