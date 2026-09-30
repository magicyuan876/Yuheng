// Package modelcontext owns every request-local handle exposed to a language
// model. Application code should use Registry instead of coordinating source
// references and durable-resource codecs independently.
//
// Its boundaries are deliberate:
//   - UUIDs, wiki slugs, URLs, and resource:// handles are durable identities.
//   - cN/dN/bN/wN/res://NNNN/ref-N values are temporary model handles.
//   - temporary handles are never persisted or accepted outside their registry.
//   - every model response is decoded before storage or UI consume it.
package modelcontext

import (
	"github.com/magicyuan876/yuheng/internal/models/chat"
	"github.com/magicyuan876/yuheng/internal/types"
)

const resourceHandleProtocolPrompt = `

## Resource handle protocol (system-owned)
Some durable resources and high-entropy Wiki slugs are represented by request-local res://NNNN handles.
- Copy only handles that appeared in supplied context, preserving them exactly in links and images.
- Never invent, edit, or expand any handle. The system restores it after generation.`

// Registry is the single request-scoped boundary between durable application
// identities and temporary model handles.
//
// Durable resource references are encoded before source identifiers. This
// ordering is intentionally private: summary/<knowledge-id> wiki slugs must be
// protected as one resource-like handle before the embedded document ID can be
// compacted to dN. Callers therefore cannot accidentally reverse the codecs.
type Registry struct {
	sources   *sourceRegistry
	resources *resourceRegistry
}

// NewRegistry creates a registry for one model request.
func NewRegistry(citationsEnabled bool) *Registry {
	return &Registry{
		sources:   newSourceRegistry(citationsEnabled),
		resources: newResourceRegistry(),
	}
}

// ProtocolPrompt returns the system-owned model handle and citation protocol.
func (r *Registry) ProtocolPrompt() string {
	if r == nil || r.sources == nil {
		return ""
	}
	return r.sources.ProtocolPrompt() + resourceHandleProtocolPrompt
}

// EncodeMessages returns a model-facing copy with every temporary handle
// encoded in the only safe order.
func (r *Registry) EncodeMessages(messages []chat.Message) []chat.Message {
	if r == nil {
		return messages
	}
	return r.sources.EncodeMessages(r.resources.EncodeMessages(messages))
}

// DecodeResponse restores resources and expands citations in a
// non-streaming response.
func (r *Registry) DecodeResponse(response *types.ChatResponse) {
	if r == nil || response == nil {
		return
	}
	response.Content = r.DecodeOutputText(response.Content)
	response.ReasoningContent = r.DecodeOutputText(response.ReasoningContent)
}

// StreamDecoder creates one ordered decoder for a response text channel.
func (r *Registry) StreamDecoder() *StreamDecoder {
	if r == nil {
		return &StreamDecoder{}
	}
	return &StreamDecoder{
		resources: newResourceStreamDecoder(r.resources),
		sources:   newCitationStreamExpander(r.sources),
		orphans:   newOrphanResourceStreamFilter(),
	}
}

// OrphanResourceHandles reports model-generated resource handles with no
// backing durable reference.
func (r *Registry) OrphanResourceHandles(decoded string) []string {
	if r == nil {
		return nil
	}
	return r.resources.OrphanHandles(decoded)
}

func (r *Registry) RegisterChunk(ref ChunkReference) string {
	if r == nil || r.sources == nil {
		return ""
	}
	return r.sources.RegisterChunk(ref)
}

func (r *Registry) RegisterDocument(id string) string {
	if r == nil || r.sources == nil {
		return ""
	}
	return r.sources.RegisterDocument(id)
}

func (r *Registry) RegisterKnowledgeBase(id string) string {
	if r == nil || r.sources == nil {
		return ""
	}
	return r.sources.RegisterKnowledgeBase(id)
}

func (r *Registry) RegisterWeb(rawURL, title string) string {
	if r == nil || r.sources == nil {
		return ""
	}
	return r.sources.RegisterWeb(rawURL, title)
}

func (r *Registry) RegisterSearchResults(results []*types.SearchResult) {
	if r == nil || r.sources == nil {
		return
	}
	r.sources.RegisterSearchResults(results)
}

func (r *Registry) ChunkHandle(id string) string {
	if r == nil || r.sources == nil {
		return ""
	}
	return r.sources.ChunkHandle(id)
}

// CompactKnownText replaces only previously registered durable source IDs.
func (r *Registry) CompactKnownText(text string) string {
	if r == nil || r.sources == nil {
		return text
	}
	text = r.resources.EncodeText(text)
	return r.sources.CompactKnownText(text)
}

// ModelToolResult renders retrieved context, carried in the ToolResult shape
// the retrieval renderers understand, using registered model handles. The
// durable-resource codec runs before the source codec so UUID-bearing summary
// slugs are protected before any embedded document ID can be compacted, and
// once more afterwards for references rendered from structured result data.
func (r *Registry) ModelToolResult(result *types.ToolResult) string {
	if result == nil {
		return ""
	}
	if r == nil || r.sources == nil {
		if result.Success {
			return result.Output
		}
		return result.Error
	}
	copyResult := *result
	copyResult.Output = r.resources.EncodeText(result.Output)
	copyResult.Error = r.resources.EncodeText(result.Error)
	modelOutput := r.sources.CompactKnownText(r.sources.ModelOutput(&copyResult))
	return r.resources.EncodeText(modelOutput)
}

// DecodeOutputText applies the public citation policy to complete text. It is
// primarily used by non-streaming cleanup/fallback paths.
func (r *Registry) DecodeOutputText(text string) string {
	if r == nil {
		return text
	}
	text = r.resources.DecodeText(text)
	text = r.resources.StripOrphanHandles(text)
	return r.sources.ExpandText(text)
}
