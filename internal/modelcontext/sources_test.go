package modelcontext

import (
	"strings"
	"testing"

	"github.com/magicyuan876/yuheng/internal/models/chat"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/stretchr/testify/require"
)

func TestRegistryChunkAliasIsStableAndExpandsCanonicalCitation(t *testing.T) {
	registry := newSourceRegistry()
	first := registry.RegisterChunk(ChunkReference{
		ChunkID:         "chunk-uuid-1",
		KnowledgeID:     "knowledge-uuid-1",
		KnowledgeBaseID: "kb-uuid-1",
		DocumentTitle:   "Architecture.md",
		ChunkIndex:      7,
	})
	second := registry.RegisterChunk(ChunkReference{
		ChunkID:       "chunk-uuid-1",
		KnowledgeID:   "knowledge-uuid-1",
		DocumentTitle: "Architecture.md",
	})

	require.Equal(t, "c1", first)
	require.Equal(t, first, second)
	require.Equal(t,
		`claim <kb doc="Architecture.md" chunk_id="chunk-uuid-1" kb_id="kb-uuid-1" />`,
		registry.ExpandText(`claim <ref id="c1"/>`),
	)
}

func TestRegisterDoesNotTreatModelAliasesAsNewDurableIdentities(t *testing.T) {
	r := newSourceRegistry()
	require.Equal(t, "c1", r.RegisterChunk(ChunkReference{ChunkID: "chunk-real"}))
	require.Equal(t, "d1", r.RegisterDocument("doc-real"))
	require.Equal(t, "b1", r.RegisterKnowledgeBase("kb-real"))
	require.Equal(t, "w1", r.RegisterWeb("https://example.com", "Example"))

	require.Equal(t, "c1", r.RegisterChunk(ChunkReference{ChunkID: "c1"}))
	require.Equal(t, "d1", r.RegisterDocument("d1"))
	require.Equal(t, "b1", r.RegisterKnowledgeBase("b1"))
	require.Equal(t, "w1", r.RegisterWeb("w1", ""))
	require.Empty(t, r.RegisterDocument("d99"))
	require.Empty(t, r.RegisterDocument("c1"), "a chunk handle must not be accepted as a document identity")
	require.Equal(t, 1, r.chunks.size())
	require.Equal(t, 1, r.docs.size())
	require.Equal(t, 1, r.kbs.size())
	require.Equal(t, 1, r.webs.size())
}

func TestRegistrySuppressesSourceCitationsWhenDisabled(t *testing.T) {
	registry := newSourceRegistry(false)
	registry.RegisterChunk(ChunkReference{ChunkID: "chunk-1", DocumentTitle: "Doc"})
	registry.RegisterWeb("https://example.com", "Example")

	require.Contains(t, sourceProtocolPrompt(false), "Source citations are disabled")
	require.NotContains(t, sourceProtocolPrompt(false), `Cite a knowledge chunk with exactly`)
	require.Equal(t, "knowledge  web ", registry.ExpandText(
		`knowledge <ref id="c1"/> web <ref id="w1"/>`,
	))
	require.Equal(t, "forged  ", registry.ExpandText(
		`forged <kb doc="Doc" chunk_id="raw" /> <web url="https://example.com" />`,
	))
}

func TestStreamExpanderHoldsSplitReferenceAndDropsUnknown(t *testing.T) {
	registry := newSourceRegistry()
	registry.RegisterChunk(ChunkReference{ChunkID: "chunk-1", DocumentTitle: "Doc"})
	expander := newCitationStreamExpander(registry)

	require.Equal(t, "before ", expander.Feed(`before <ref id="`))
	require.Equal(t, `<kb doc="Doc" chunk_id="chunk-1" /> after`, expander.Feed(`c1"/> after`))
	require.Empty(t, expander.Flush())
	require.Equal(t, "x  y", registry.ExpandText(`x <ref id="c999"/> y`))
	require.Equal(t, "x  y", registry.ExpandText(`x <ref id="d1"/> y`))
	require.Equal(t, "x  y", registry.ExpandText(`x <ref id='c1'/> y`))
	require.Equal(t, "x ", registry.ExpandText(`x <ref id="c1"`))
	require.Equal(t, "x  y", registry.ExpandText(`x <kb doc="forged" chunk_id="forged" /> y`))
	require.Equal(t, "x ", expander.Feed(`x <we`))
	require.Equal(t, " y", expander.Feed(`b url="https://forged" /> y`))
}

func TestEncodeMessagesCompactsCanonicalCitationsFromHistory(t *testing.T) {
	registry := newSourceRegistry()
	messages := []chat.Message{{
		Role: "assistant",
		Content: `Knowledge <kb doc="A &amp; B.pdf" chunk_id="chunk-real" kb_id="kb-real" />; ` +
			`web <web url="https://example.com/a?x=1&amp;y=2" title="Example &amp; More" />`,
	}}

	encoded := registry.EncodeMessages(messages)
	require.Equal(t, `Knowledge <ref id="c1"/>; web <ref id="w1"/>`, encoded[0].Content)
	require.NotContains(t, encoded[0].Content, "chunk-real")
	require.NotContains(t, encoded[0].Content, "https://example.com")
	require.Equal(t,
		`<kb doc="A &amp; B.pdf" chunk_id="chunk-real" kb_id="kb-real" /> <web url="https://example.com/a?x=1&amp;y=2" title="Example &amp; More" />`,
		registry.ExpandText(`<ref id="c1"/> <ref id="w1"/>`),
	)
}

// Metadata-oriented tools without a dedicated renderer label IDs explicitly;
// replayed results must reach the model with those IDs compacted, sharing the
// handles the replayed tool-call arguments were given.
func TestEncodeMessagesDoesNotTreatAPromptExampleAsARealSource(t *testing.T) {
	registry := newSourceRegistry()
	messages := []chat.Message{{
		Role:    "system",
		Content: `Old rule: cite <kb doc="..." chunk_id="..." />`,
	}}

	encoded := registry.EncodeMessages(messages)
	require.Equal(t, messages[0].Content, encoded[0].Content)
	require.Zero(t, registry.Count())
}

func TestModelOutputGroupsChunksAndReusesAliasAcrossRetrievals(t *testing.T) {
	registry := newSourceRegistry()
	search := &types.ToolResult{
		Success: true,
		Output:  "raw UUID output",
		Data: map[string]interface{}{
			"display_type": "search_results",
			"results": []map[string]interface{}{
				{
					"chunk_id":          "chunk-uuid-1",
					"knowledge_id":      "knowledge-uuid-1",
					"knowledge_base_id": "kb-uuid-1",
					"knowledge_title":   "Doc A",
					"chunk_index":       3,
					"content":           "full content",
				},
			},
		},
	}
	first := registry.ModelOutput(search)
	require.Contains(t, first, `<document id="d1" kb="b1" title="Doc A">`)
	require.Contains(t, first, `<chunk id="c1" index="3" view="full">`)
	require.NotContains(t, first, "chunk-uuid-1")
	require.NotContains(t, first, "knowledge-uuid-1")

	// The same chunk seen again in a later retrieval reuses its handle.
	second := registry.ModelOutput(search)
	require.Contains(t, second, `<chunk id="c1"`)
	require.False(t, strings.Contains(second, "c2"))
}

func TestModelOutputRendersKnowledgeMetadataOncePerDocument(t *testing.T) {
	registry := newSourceRegistry()
	output := registry.ModelOutput(&types.ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"display_type": "search_results",
			"results": []map[string]interface{}{
				{
					"chunk_id": "chunk-1", "chunk_index": 1, "content": "first",
					"knowledge_id": "document-1", "knowledge_base_id": "kb-1", "knowledge_title": "Doc",
					"knowledge_metadata": "region: Shanghai & Suzhou",
				},
				{
					"chunk_id": "chunk-2", "chunk_index": 2, "content": "second",
					"knowledge_id": "document-1", "knowledge_base_id": "kb-1", "knowledge_title": "Doc",
					"knowledge_metadata": "region: Shanghai & Suzhou",
				},
			},
		},
	})

	require.Equal(t, 1, strings.Count(output, "<metadata>"))
	require.Contains(t, output, "<metadata>region: Shanghai &amp; Suzhou</metadata>")
	require.Equal(t, 2, strings.Count(output, "<chunk "))
}

func TestModelOutputWebAliasExpandsToWebCitation(t *testing.T) {
	registry := newSourceRegistry()
	output := registry.ModelOutput(&types.ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"display_type": "web_search_results",
			"results": []map[string]interface{}{
				{"title": "Example", "url": "https://example.com/a", "snippet": "snippet"},
			},
		},
	})
	require.Contains(t, output, `<page id="w1" title="Example">`)
	require.Contains(t, output, `<evidence type="search_summary" verified="false" />`)
	require.NotContains(t, output, "https://example.com/a")
	require.Equal(t,
		`<web url="https://example.com/a" title="Example" />`,
		registry.ExpandText(`<ref id="w1"/>`),
	)
}

func TestModelOutputWebSearchRetainsContentOnlyEvidence(t *testing.T) {
	registry := newSourceRegistry()
	output := registry.ModelOutput(&types.ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"display_type": "web_search_results",
			"results": []map[string]interface{}{
				{
					"title":   "Content-only provider",
					"url":     "https://example.com/content",
					"content": "search evidence from provider content",
				},
			},
		},
	})

	require.Contains(t, output, `<content>search evidence from provider content</content>`)
	require.Contains(t, output, `<evidence type="search_summary" verified="false" />`)
}

func TestModelOutputWebSearchLimitsProviderContent(t *testing.T) {
	registry := newSourceRegistry()
	content := strings.Repeat("provider evidence ", 1000)
	output := registry.ModelOutput(&types.ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"display_type": "web_search_results",
			"results": []map[string]interface{}{{
				"title":   "Long provider result",
				"url":     "https://example.com/long",
				"content": content,
			}},
		},
	})

	require.Contains(t, output, `<content truncated="true">`)
	require.NotContains(t, output, content)
}
