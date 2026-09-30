// sources.go is the source-reference half of the model-context registry:
// request-local cN/dN/bN/wN handles for chunks, documents, knowledge bases
// and web pages, and the expansion of cited handles back into public
// citations. Request lifecycles use Registry so source and resource handles
// cannot be encoded or decoded out of order.
package modelcontext

import (
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/magicyuan876/yuheng/internal/models/chat"
	"github.com/magicyuan876/yuheng/internal/types"
)

type ChunkReference struct {
	ChunkID         string
	KnowledgeID     string
	KnowledgeBaseID string
	DocumentTitle   string
	ChunkIndex      int
	ChunkType       string
}

// webMeta is the per-web-page metadata stored next to the raw URL.
type webMeta struct {
	title string
}

// sourceRegistry is scoped to one assistant response. Handles are never persisted or accepted across requests.
type sourceRegistry struct {
	citationsEnabled bool

	chunks *handleTable[ChunkReference]
	docs   *handleTable[struct{}]
	kbs    *handleTable[struct{}]
	webs   *handleTable[webMeta]
}

func newSourceRegistry(citationsEnabled ...bool) *sourceRegistry {
	enabled := true
	if len(citationsEnabled) > 0 {
		enabled = citationsEnabled[0]
	}
	return &sourceRegistry{
		citationsEnabled: enabled,
		chunks:           newHandleTable[ChunkReference]("c", 0, 1),
		docs:             newHandleTable[struct{}]("d", 0, 1),
		kbs:              newHandleTable[struct{}]("b", 0, 1),
		webs:             newHandleTable[webMeta]("w", 0, 1),
	}
}

func (r *sourceRegistry) Count() int {
	if r == nil {
		return 0
	}
	return r.chunks.size() + r.webs.size()
}

// knownHandle implements the shared guard for handle-shaped registration
// input: a model-emitted handle is echoed back only when it already exists,
// and is never accepted as a new durable identity.
func knownHandle[M any](table *handleTable[M], id string) string {
	handle := strings.ToLower(id)
	if table.has(handle) {
		return handle
	}
	return ""
}

func (r *sourceRegistry) RegisterChunk(ref ChunkReference) string {
	if r == nil {
		return ""
	}
	ref.ChunkID = strings.TrimSpace(ref.ChunkID)
	if ref.ChunkID == "" {
		return ""
	}
	if shortSourceHandleRE.MatchString(ref.ChunkID) {
		return knownHandle(r.chunks, ref.ChunkID)
	}
	return r.chunks.register(ref.ChunkID, ref.ChunkID, ref, mergeChunkReference)
}

func mergeChunkReference(dst *ChunkReference, src ChunkReference) {
	if dst.KnowledgeID == "" {
		dst.KnowledgeID = src.KnowledgeID
	}
	if dst.KnowledgeBaseID == "" {
		dst.KnowledgeBaseID = src.KnowledgeBaseID
	}
	if dst.DocumentTitle == "" {
		dst.DocumentTitle = src.DocumentTitle
	}
	if dst.ChunkIndex == 0 {
		dst.ChunkIndex = src.ChunkIndex
	}
	if dst.ChunkType == "" {
		dst.ChunkType = src.ChunkType
	}
}

func (r *sourceRegistry) RegisterDocument(id string) string {
	id = strings.TrimSpace(id)
	if r == nil || id == "" {
		return ""
	}
	if shortSourceHandleRE.MatchString(id) {
		return knownHandle(r.docs, id)
	}
	return r.docs.register(id, id, struct{}{}, nil)
}

func (r *sourceRegistry) RegisterKnowledgeBase(id string) string {
	id = strings.TrimSpace(id)
	if r == nil || id == "" {
		return ""
	}
	if shortSourceHandleRE.MatchString(id) {
		return knownHandle(r.kbs, id)
	}
	return r.kbs.register(id, id, struct{}{}, nil)
}

func (r *sourceRegistry) RegisterWeb(rawURL, title string) string {
	rawURL = strings.TrimSpace(rawURL)
	if r == nil || rawURL == "" {
		return ""
	}
	if shortSourceHandleRE.MatchString(rawURL) {
		return knownHandle(r.webs, rawURL)
	}
	// Dedup on the canonical (fragment-stripped) URL while decoding back to
	// the raw URL the model was originally shown.
	return r.webs.register(canonicalWebURL(rawURL), rawURL, webMeta{title: title}, func(dst *webMeta, src webMeta) {
		if dst.title == "" && src.title != "" {
			dst.title = src.title
		}
	})
}

func canonicalWebURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimSpace(raw)
	}
	parsed.Fragment = ""
	return parsed.String()
}

func (r *sourceRegistry) RegisterSearchResults(results []*types.SearchResult) {
	for _, result := range results {
		if result == nil {
			continue
		}
		r.RegisterDocument(result.KnowledgeID)
		r.RegisterKnowledgeBase(result.KnowledgeBaseID)
		r.RegisterChunk(ChunkReference{
			ChunkID:         result.ID,
			KnowledgeID:     result.KnowledgeID,
			KnowledgeBaseID: result.KnowledgeBaseID,
			DocumentTitle:   firstNonEmpty(result.KnowledgeTitle, result.KnowledgeFilename),
			ChunkIndex:      result.ChunkIndex,
			ChunkType:       result.ChunkType,
		})
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (r *sourceRegistry) ChunkHandle(id string) string {
	handle, _ := r.chunks.handleForKey(id)
	return handle
}

// shortSourceHandleRE matches a temporary source handle (c1, d2, b3, w4). A
// value of that shape is never registered as a durable identity: it is a
// handle the model echoed back, and treating it as new would make handles drift.
var shortSourceHandleRE = regexp.MustCompile(`(?i)^[cdbw][1-9][0-9]*$`)

// EncodeMessages compacts canonical public citations in replayed assistant
// turns back into this request's private protocol, so durable chunk IDs and
// web URLs from conversation history never become model-visible again.
func (r *sourceRegistry) EncodeMessages(messages []chat.Message) []chat.Message {
	if r == nil || len(messages) == 0 {
		return messages
	}
	out := make([]chat.Message, len(messages))
	copy(out, messages)
	for i := range out {
		if out[i].Role != "assistant" {
			continue
		}
		out[i].Content = r.CompactPublicCitations(out[i].Content)
		out[i].ReasoningContent = r.CompactPublicCitations(out[i].ReasoningContent)
		if len(out[i].MultiContent) > 0 {
			out[i].MultiContent = append([]chat.MessageContentPart(nil), out[i].MultiContent...)
			for j := range out[i].MultiContent {
				if out[i].MultiContent[j].Type == "text" {
					out[i].MultiContent[j].Text = r.CompactPublicCitations(out[i].MultiContent[j].Text)
				}
			}
		}
	}
	return out
}

func (r *sourceRegistry) handleForDurable(real string) string {
	if handle, ok := r.chunks.handleForKey(real); ok {
		return handle
	}
	if handle, ok := r.docs.handleForKey(real); ok {
		return handle
	}
	if handle, ok := r.kbs.handleForKey(real); ok {
		return handle
	}
	if handle, ok := r.webs.handleForKey(canonicalWebURL(real)); ok {
		return handle
	}
	return ""
}

// CompactKnownText is intentionally limited to identifiers already registered
// from structured runtime data. It is used for metadata envelopes, not
// arbitrary retrieved prose.
func (r *sourceRegistry) CompactKnownText(text string) string {
	if r == nil || text == "" {
		return text
	}
	// The snapshot spans all four source tables and is sorted longest-value
	// first GLOBALLY: a web URL may contain a registered document UUID as a
	// substring, so per-table passes could corrupt the longer value.
	pairs := r.chunks.pairs()
	pairs = append(pairs, r.docs.pairs()...)
	pairs = append(pairs, r.kbs.pairs()...)
	pairs = append(pairs, r.webs.pairs()...)
	sort.SliceStable(pairs, func(i, j int) bool { return len(pairs[i].value) > len(pairs[j].value) })
	for _, item := range pairs {
		if item.value != "" {
			text = strings.ReplaceAll(text, item.value, item.handle)
		}
	}
	return text
}
