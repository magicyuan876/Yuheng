package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SetDocumentMetadata must merge into existing chunk metadata: overwriting
// the whole JSON document silently destroyed foreign keys such as the video
// timeline's video_segment (breaking timestamp jump on citations) the moment
// question generation ran.
func TestSetDocumentMetadataPreservesForeignKeys(t *testing.T) {
	chunk := &Chunk{Metadata: NewVideoSegmentChunkMetadata(1000, 2000)}

	err := chunk.SetDocumentMetadata(&DocumentChunkMetadata{
		GeneratedQuestions:         []GeneratedQuestion{{ID: "q1", Question: "什么是隧穿？"}},
		GeneratedQuestionsRevision: 3,
	})
	require.NoError(t, err)

	var merged map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(chunk.Metadata, &merged))
	assert.Contains(t, merged, ChunkMetadataVideoSegmentKey)
	assert.Contains(t, merged, "generated_questions")

	var seg VideoSegmentMetadata
	require.NoError(t, json.Unmarshal(merged[ChunkMetadataVideoSegmentKey], &seg))
	assert.Equal(t, int64(1000), seg.StartMs)
	assert.Equal(t, int64(2000), seg.EndMs)

	got, err := chunk.DocumentMetadata()
	require.NoError(t, err)
	require.Len(t, got.GeneratedQuestions, 1)
	assert.Equal(t, 3, got.GeneratedQuestionsRevision)
}

func TestSetDocumentMetadataRewritesOwnedKeys(t *testing.T) {
	chunk := &Chunk{}
	require.NoError(t, chunk.SetDocumentMetadata(&DocumentChunkMetadata{
		GeneratedQuestions: []GeneratedQuestion{{ID: "q1", Question: "old"}},
	}))
	require.NoError(t, chunk.SetDocumentMetadata(&DocumentChunkMetadata{
		GeneratedQuestions: []GeneratedQuestion{{ID: "q2", Question: "new"}},
	}))

	got, err := chunk.DocumentMetadata()
	require.NoError(t, err)
	require.Len(t, got.GeneratedQuestions, 1)
	assert.Equal(t, "q2", got.GeneratedQuestions[0].ID)
}

func TestSetDocumentMetadataNilClearsOnlyOwnedKeys(t *testing.T) {
	chunk := &Chunk{Metadata: NewVideoFrameChunkMetadata(500)}
	require.NoError(t, chunk.SetDocumentMetadata(&DocumentChunkMetadata{
		GeneratedQuestions: []GeneratedQuestion{{ID: "q1", Question: "x"}},
	}))
	require.NoError(t, chunk.SetDocumentMetadata(nil))

	var merged map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(chunk.Metadata, &merged))
	assert.Contains(t, merged, ChunkMetadataVideoFrameKey)
	assert.NotContains(t, merged, "generated_questions")

	// A chunk with no other metadata clears to nil.
	bare := &Chunk{}
	require.NoError(t, bare.SetDocumentMetadata(&DocumentChunkMetadata{
		GeneratedQuestions: []GeneratedQuestion{{ID: "q1", Question: "x"}},
	}))
	require.NoError(t, bare.SetDocumentMetadata(nil))
	assert.Nil(t, bare.Metadata)
}
