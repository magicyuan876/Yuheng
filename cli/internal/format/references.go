package format

import sdk "github.com/magicyuan876/yuheng/client"

// ReferenceIndex is the bounded citation pointer exposed by projected JSON,
// text, and MCP output. ChunkID is the chunk the model cited; ParentChunkID is
// retained when callers prefer to fetch the larger self-contained passage.
// Full chunk text and retrieval metadata remain available only in raw NDJSON.
type ReferenceIndex struct {
	KBID          string `json:"kb_id,omitempty"`
	ChunkID       string `json:"chunk_id"`
	ParentChunkID string `json:"parent_chunk_id,omitempty"`
}

// IndexReferences projects full SDK search results into stable lookup keys.
// It never mutates the SDK events, which keeps the raw NDJSON path lossless.
// KBID is left empty for a reference that belongs to no knowledge base (a web
// search hit, for instance) rather than attributed to the KB the chat targeted.
func IndexReferences(refs []*sdk.SearchResult) []ReferenceIndex {
	indexes := make([]ReferenceIndex, 0, len(refs))
	for _, r := range refs {
		if r == nil || r.ID == "" {
			continue
		}
		indexes = append(indexes, ReferenceIndex{
			KBID:          r.KnowledgeBaseID,
			ChunkID:       r.ID,
			ParentChunkID: r.ParentChunkID,
		})
	}
	return indexes
}
