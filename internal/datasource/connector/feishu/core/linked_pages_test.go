package core

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/types"
)

func linkedTextBlock(text, rawURL string) DocxBlock {
	return DocxBlock{
		BlockType: BlockTypeText,
		Text: &BlockText{Elements: []TextElement{{
			TextRun: &TextRun{Content: text, Style: &TextRunStyle{Link: &TextRunLink{URL: rawURL}}},
		}}},
	}
}

func TestLinkRunURL(t *testing.T) {
	// Feishu returns link URLs percent-encoded.
	r := &TextRun{Content: "文章", Style: &TextRunStyle{Link: &TextRunLink{
		URL: "https%3A%2F%2Fexample.com%2Fa%3Fb%3D1",
	}}}
	assert.Equal(t, "https://example.com/a?b=1", linkRunURL(r))

	// Already-plain URLs pass through; '+' must not become a space.
	r.Style.Link.URL = "https://example.com/a+b"
	assert.Equal(t, "https://example.com/a+b", linkRunURL(r))

	// Undecodable input falls back to the raw string.
	r.Style.Link.URL = "https://example.com/100%zz"
	assert.Equal(t, "https://example.com/100%zz", linkRunURL(r))

	assert.Empty(t, linkRunURL(nil))
	assert.Empty(t, linkRunURL(&TextRun{Content: "无链接"}))
	assert.Empty(t, linkRunURL(&TextRun{Content: "x", Style: &TextRunStyle{}}))
}

func TestCollectDocLinks(t *testing.T) {
	blocks := []DocxBlock{
		linkedTextBlock("文章一", "https://example.com/1"),
		// Duplicate URL keeps first title only.
		linkedTextBlock("重复", "https://example.com/1"),
		// Bare-URL run: title falls back to the URL.
		linkedTextBlock("  ", "https://example.com/2"),
		// Heading links are collected too.
		{BlockType: BlockTypeHeading1 + 1, Heading2: &BlockText{Elements: []TextElement{{
			TextRun: &TextRun{Content: "标题链", Style: &TextRunStyle{Link: &TextRunLink{URL: "https://example.com/3"}}},
		}}}},
		// Plain runs and non-text blocks contribute nothing.
		{BlockType: BlockTypeText, Text: &BlockText{Elements: []TextElement{{TextRun: &TextRun{Content: "纯文本"}}}}},
		{BlockType: BlockTypeImage},
	}
	links := collectDocLinks(blocks)
	require.Len(t, links, 3)
	assert.Equal(t, pendingLink{URL: "https://example.com/1", Title: "文章一"}, links[0])
	assert.Equal(t, pendingLink{URL: "https://example.com/2", Title: "https://example.com/2"}, links[1])
	assert.Equal(t, pendingLink{URL: "https://example.com/3", Title: "标题链"}, links[2])
}

func TestIsCrawlableLinkURL(t *testing.T) {
	tests := []struct {
		url     string
		docHost string
		want    bool
	}{
		{"https://news.qq.com/rain/a/1", "", true},
		{"http://example.com", "", true},
		{"mailto:a@b.com", "", false},
		{"javascript:alert(1)", "", false},
		{"ftp://example.com/f", "", false},
		{"", "", false},
		{"not a url://", "", false},
		{"/relative/path", "", false},
		// Feishu/Lark product hosts, including tenant subdomains.
		{"https://feishu.cn/docs/x", "", false},
		{"https://xxx.feishu.cn/wiki/x", "", false},
		{"https://example.larkenterprise.com/wiki/x", "", false},
		{"https://open.larksuite.com/document", "", false},
		{"https://sample.larkoffice.com/docx/x", "", false},
		// Suffix matching must not over-match lookalike domains.
		{"https://notfeishu.cn.example.com/a", "", true},
		// The document's own (possibly custom) web host is excluded.
		{"https://kb.mycorp.com/wiki/other", "kb.mycorp.com", false},
		{"https://kb.mycorp.com/wiki/other", "", true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, IsCrawlableLinkURL(tt.url, tt.docHost), "url=%q docHost=%q", tt.url, tt.docHost)
	}
}

func TestInlineTextRendersMarkdownLinks(t *testing.T) {
	bt := &BlockText{Elements: []TextElement{
		{TextRun: &TextRun{Content: "参见"}},
		{TextRun: &TextRun{Content: "这篇文章", Style: &TextRunStyle{Link: &TextRunLink{URL: "https://example.com/a"}}}},
		// Empty visible text degrades to plain content (no "[]()" noise).
		{TextRun: &TextRun{Content: " ", Style: &TextRunStyle{Link: &TextRunLink{URL: "https://example.com/b"}}}},
	}}
	assert.Equal(t, "参见[这篇文章](https://example.com/a) ", inlineText(bt))
	assert.Empty(t, inlineText(nil))
}

func TestFetchOptionsFromConfigLinkedPages(t *testing.T) {
	// Off by default.
	assert.False(t, FetchOptionsFromConfig(nil).SyncLinkedPages)
	assert.False(t, FetchOptionsFromConfig(&types.DataSourceConfig{}).SyncLinkedPages)

	cfg := &types.DataSourceConfig{Settings: map[string]interface{}{"sync_linked_pages": true}}
	assert.True(t, FetchOptionsFromConfig(cfg).SyncLinkedPages)

	// Garbage values keep the default.
	cfg = &types.DataSourceConfig{Settings: map[string]interface{}{"sync_linked_pages": "yes"}}
	assert.False(t, FetchOptionsFromConfig(cfg).SyncLinkedPages)
}

func TestAppendLinkedPageItems(t *testing.T) {
	in := DocxFetchInput{
		DocToken:   "node-1",
		ObjToken:   "doc-1",
		URL:        "https://example.larkenterprise.com/wiki/node-1",
		ResourceID: "res-1",
		BaseMeta:   map[string]string{"channel": "feishu"},
	}
	blocks := []DocxBlock{
		linkedTextBlock("外部文章", "https://news.qq.com/rain/a/1"),
		// Internal Feishu link: filtered, no item, no keep entry.
		linkedTextBlock("内部文档", "https://example.larkenterprise.com/wiki/other"),
		linkedTextBlock("mailto", "mailto:a@b.com"),
	}

	items, keep := appendLinkedPageItems(context.Background(), in, blocks, nil, nil)
	require.Len(t, items, 1)
	require.Len(t, keep, 1)

	it := items[0]
	assert.Equal(t, "外部文章", it.Title)
	assert.Equal(t, "https://news.qq.com/rain/a/1", it.URL)
	assert.Empty(t, it.Content, "link items are URL-only so ingest routes them to the web crawler")
	assert.Equal(t, keep[0], it.ExternalID)
	assert.Equal(t, types.SubtreeChildID("node-1", "link", linkChildToken("https://news.qq.com/rain/a/1")), it.ExternalID)
	assert.Equal(t, "true", it.Metadata["linked_page"])
	assert.Equal(t, "node-1", it.Metadata["parent_node_token"])
	assert.Equal(t, "https://news.qq.com/rain/a/1", it.Metadata["linked_page_url"])
	assert.Equal(t, "feishu", it.Metadata["channel"])

	// The child ID is stable across syncs (pure function of the URL).
	again, _ := appendLinkedPageItems(context.Background(), in, blocks, nil, nil)
	require.Len(t, again, 1)
	assert.Equal(t, it.ExternalID, again[0].ExternalID)
}

func TestAppendLinkedPageItemsCap(t *testing.T) {
	in := DocxFetchInput{DocToken: "node-1", ObjToken: "doc-1", BaseMeta: map[string]string{}}
	var blocks []DocxBlock
	for i := 0; i < maxLinkedPagesPerDoc+7; i++ {
		blocks = append(blocks, linkedTextBlock("l", fmt.Sprintf("https://example.com/%d", i)))
	}
	items, keep := appendLinkedPageItems(context.Background(), in, blocks, nil, nil)
	assert.Len(t, items, maxLinkedPagesPerDoc)
	assert.Len(t, keep, maxLinkedPagesPerDoc)
	// Deterministic: the first N links in document order survive.
	assert.Equal(t, "https://example.com/0", items[0].URL)
}
