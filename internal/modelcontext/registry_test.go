package modelcontext

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/models/chat"
	"github.com/magicyuan876/yuheng/internal/types"
)

func TestRegistryProtocolOwnsResourceHandleRules(t *testing.T) {
	prompt := NewRegistry(true).ProtocolPrompt()
	require.Contains(t, prompt, "Source handling protocol")
	require.Contains(t, prompt, "Resource handle protocol")
	require.Contains(t, prompt, "res://NNNN")
}

func TestRegistryCompactsKnownIDsInRetrievalErrors(t *testing.T) {
	registry := NewRegistry(true)
	registry.RegisterDocument("doc-real")
	registry.RegisterKnowledgeBase("kb-real")

	got := registry.ModelToolResult(&types.ToolResult{
		Success: false,
		Error:   "document doc-real belongs to knowledge base kb-real",
	})
	require.Equal(t, "Error: document d1 belongs to knowledge base b1", got)
}

func TestModelToolResultProtectsSummarySlugBeforeSourceCompaction(t *testing.T) {
	const knowledgeID = "07a20bb1-a662-47cf-9929-06fb5d5b5b5e"
	const kbID = "250368ff-f5a2-4e9e-868a-07bc9b857c44"
	registry := NewRegistry(true)
	registry.RegisterDocument(knowledgeID)
	registry.RegisterKnowledgeBase(kbID)

	got := registry.ModelToolResult(&types.ToolResult{
		Success: true, Output: "<knowledge_base_id>" + kbID + "</knowledge_base_id>\n" +
			"<link>[[summary/" + knowledgeID + "|Summary]]</link>\n" +
			"<knowledge_id>" + knowledgeID + "</knowledge_id>",
	})
	require.Contains(t, got, "[[res://0001|Summary]]")
	require.Contains(t, got, "<knowledge_base_id>b1</knowledge_base_id>")
	require.Contains(t, got, "<knowledge_id>d1</knowledge_id>")
	require.NotContains(t, got, "summary/d1")
}

func TestRegistryStreamDecoderRestoresSplitResourceAndCitationHandles(t *testing.T) {
	registry := NewRegistry(true)
	registry.EncodeMessages([]chat.Message{{Role: "user", Content: "resource://AbCdEfGhIjKlMnOpQrStUv"}})
	registry.RegisterChunk(ChunkReference{
		ChunkID:         "chunk-real",
		KnowledgeID:     "doc-real",
		KnowledgeBaseID: "kb-real",
		DocumentTitle:   "Doc",
	})

	decoder := registry.StreamDecoder()
	got := decoder.Feed("image res://0") +
		decoder.Feed("001 claim <ref id=\"c") +
		decoder.Feed("1\"/>") + decoder.Flush()
	require.Contains(t, got, "resource://AbCdEfGhIjKlMnOpQrStUv")
	require.Contains(t, got, `chunk_id="chunk-real"`)
}

func TestRegistryDropsUnknownResourceHandlesFromCompleteAndStreamOutput(t *testing.T) {
	registry := NewRegistry(true)
	require.Equal(t, "broken ", registry.DecodeOutputText("broken res://9999"))

	decoder := registry.StreamDecoder()
	got := decoder.Feed("broken res:/") + decoder.Feed("/99") + decoder.Feed("99 end") + decoder.Flush()
	require.Equal(t, "broken  end", got)
}

func TestHandleTableRoundTripAndIsolation(t *testing.T) {
	chunks := NewHandleTable("c", 3, 0)
	require.Equal(t, "c000", chunks.Register("chunk-a"))
	require.Equal(t, "c000", chunks.Register("chunk-a"))
	require.Equal(t, "c001", chunks.Register("chunk-b"))
	require.Equal(t, "chunk-b", mustResolve(t, chunks, "c001"))

	slugs := NewHandleTable("ref-", 0, 1)
	require.Equal(t, "ref-1", slugs.Register("summary/uuid"))
	require.Equal(t, "summary/uuid", mustResolve(t, slugs, "ref-1"))
	require.Equal(t, "", mustNotResolve(t, slugs, "c000"))
}

func mustResolve(t *testing.T, table *HandleTable, handle string) string {
	t.Helper()
	value, ok := table.Resolve(handle)
	require.True(t, ok)
	return value
}

func mustNotResolve(t *testing.T, table *HandleTable, handle string) string {
	t.Helper()
	value, ok := table.Resolve(handle)
	require.False(t, ok)
	return value
}
