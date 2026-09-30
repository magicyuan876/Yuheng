package modelcontext

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/magicyuan876/yuheng/internal/types"
)

// modelWebSearchEvidenceMaxRunes caps each snippet or content excerpt of a web
// search hit: search evidence is unverified, so a page must not crowd out the
// knowledge-base context.
const modelWebSearchEvidenceMaxRunes = 500

// ModelOutput returns a compact, source-centric representation for the LLM of
// the retrieval results the chat pipeline renders: knowledge-base hits
// ("search_results") and web search hits ("web_search_results"). The canonical
// ToolResult.Output remains untouched for UI, logs and storage.
func (r *sourceRegistry) ModelOutput(result *types.ToolResult) string {
	if result == nil {
		return ""
	}
	if !result.Success {
		if result.Error != "" {
			return "Error: " + r.CompactKnownText(result.Error)
		}
		return "Error: retrieval failed"
	}
	switch stringValue(result.Data, "display_type") {
	case "search_results":
		return r.modelKnowledgeOutput(mapsValue(result.Data["results"]), result.Output)
	case "web_search_results":
		return r.modelWebSearchOutput(mapsValue(result.Data["results"]), result.Output)
	default:
		return r.CompactKnownText(result.Output)
	}
}

type modelChunk struct {
	handle     string
	docHandle  string
	kbHandle   string
	title      string
	metadata   string
	chunkType  string
	index      int
	view       string
	match      string
	content    string
	question   string
	answers    []string
	images     []map[string]interface{}
	docRealID  string
	kbRealID   string
	chunkReal  string
	inputOrder int
}

func (r *sourceRegistry) modelKnowledgeOutput(rows []map[string]interface{}, fallback string) string {
	chunks := make([]modelChunk, 0, len(rows))
	for idx, row := range rows {
		chunkID := firstNonEmpty(stringValue(row, "chunk_id"), stringValue(row, "faq_id"), stringValue(row, "id"))
		knowledgeID := stringValue(row, "knowledge_id")
		kbID := firstNonEmpty(stringValue(row, "knowledge_base_id"), stringValue(row, "knowledge_base"))
		title := firstNonEmpty(stringValue(row, "knowledge_title"), stringValue(row, "title"))
		if chunkID == "" {
			continue
		}
		chunkType := stringValue(row, "chunk_type")
		if stringValue(row, "faq_id") != "" && chunkType == "" {
			chunkType = "faq"
		}
		chunkIndex := intValue(row, "chunk_index")
		if chunkIndex == 0 {
			chunkIndex = intValue(row, "index")
		}
		chunkHandle := r.RegisterChunk(ChunkReference{
			ChunkID:         chunkID,
			KnowledgeID:     knowledgeID,
			KnowledgeBaseID: kbID,
			DocumentTitle:   title,
			ChunkIndex:      chunkIndex,
			ChunkType:       chunkType,
		})
		chunks = append(chunks, modelChunk{
			handle:     chunkHandle,
			docHandle:  r.RegisterDocument(knowledgeID),
			kbHandle:   r.RegisterKnowledgeBase(kbID),
			title:      title,
			metadata:   stringValue(row, "knowledge_metadata"),
			chunkType:  chunkType,
			index:      chunkIndex,
			view:       viewForRow(row),
			match:      firstNonEmpty(stringValue(row, "match_snippet"), stringValue(row, "matched_content")),
			content:    stringValue(row, "content"),
			question:   firstNonEmpty(stringValue(row, "faq_question"), stringValue(row, "faq_standard_question")),
			answers:    stringSliceValue(row["faq_answers"]),
			images:     mapsValue(row["images"]),
			docRealID:  knowledgeID,
			kbRealID:   kbID,
			chunkReal:  chunkID,
			inputOrder: idx,
		})
	}
	if len(chunks) == 0 {
		return r.CompactKnownText(fallback)
	}
	return renderKnowledgeChunks(chunks)
}

// viewForRow tells the model whether it sees a chunk's full text or only the
// matched excerpt.
func viewForRow(row map[string]interface{}) string {
	if stringValue(row, "content") != "" {
		return "full"
	}
	return "match"
}

func renderKnowledgeChunks(chunks []modelChunk) string {
	type docGroup struct {
		handle   string
		kbHandle string
		title    string
		metadata string
		chunks   []modelChunk
		order    int
	}
	groupsByKey := make(map[string]*docGroup)
	var groups []*docGroup
	for _, chunk := range chunks {
		key := chunk.docHandle
		if key == "" {
			key = "chunk:" + chunk.handle
		}
		group := groupsByKey[key]
		if group == nil {
			group = &docGroup{
				handle: chunk.docHandle, kbHandle: chunk.kbHandle, title: chunk.title,
				metadata: chunk.metadata, order: chunk.inputOrder,
			}
			groupsByKey[key] = group
			groups = append(groups, group)
		} else if group.metadata == "" {
			group.metadata = chunk.metadata
		}
		group.chunks = append(group.chunks, chunk)
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].order < groups[j].order })

	var b strings.Builder
	b.WriteString("<retrieval type=\"knowledge\" mode=\"semantic\">\n")
	for _, group := range groups {
		b.WriteString("  <document")
		if group.handle != "" {
			fmt.Fprintf(&b, " id=\"%s\"", escapeAttr(group.handle))
		}
		if group.kbHandle != "" {
			fmt.Fprintf(&b, " kb=\"%s\"", escapeAttr(group.kbHandle))
		}
		if group.title != "" {
			fmt.Fprintf(&b, " title=\"%s\"", escapeAttr(group.title))
		}
		b.WriteString(">\n")
		if group.metadata != "" {
			fmt.Fprintf(&b, "    <metadata>%s</metadata>\n", escapeText(group.metadata))
		}
		for _, chunk := range group.chunks {
			fmt.Fprintf(&b, "    <chunk id=\"%s\" index=\"%d\" view=\"%s\"", chunk.handle, chunk.index, chunk.view)
			if chunk.chunkType != "" {
				fmt.Fprintf(&b, " type=\"%s\"", escapeAttr(chunk.chunkType))
			}
			b.WriteString(">\n")
			if chunk.question != "" {
				fmt.Fprintf(&b, "      <question>%s</question>\n", escapeText(chunk.question))
			}
			if chunk.match != "" {
				fmt.Fprintf(&b, "      <match>%s</match>\n", escapeText(chunk.match))
			}
			if chunk.content != "" {
				fmt.Fprintf(&b, "      <content>%s</content>\n", escapeText(chunk.content))
			}
			for _, answer := range chunk.answers {
				fmt.Fprintf(&b, "      <answer>%s</answer>\n", escapeText(answer))
			}
			for _, image := range chunk.images {
				imageURL := stringValue(image, "url")
				if imageURL == "" {
					continue
				}
				caption := stringValue(image, "caption")
				fmt.Fprintf(&b, "      ![%s](%s)\n", caption, imageURL)
			}
			b.WriteString("    </chunk>\n")
		}
		b.WriteString("  </document>\n")
	}
	b.WriteString("</retrieval>")
	return b.String()
}

func (r *sourceRegistry) modelWebSearchOutput(rows []map[string]interface{}, fallback string) string {
	if len(rows) == 0 {
		return r.CompactKnownText(fallback)
	}
	var b strings.Builder
	b.WriteString("<retrieval type=\"web\" mode=\"search\">\n")
	count := 0
	for _, row := range rows {
		rawURL := stringValue(row, "url")
		if rawURL == "" {
			continue
		}
		handle := r.RegisterWeb(rawURL, stringValue(row, "title"))
		fmt.Fprintf(&b, "  <page id=\"%s\" title=\"%s\">\n", handle, escapeAttr(stringValue(row, "title")))
		b.WriteString("    <evidence type=\"search_summary\" verified=\"false\" />\n")
		if snippet := stringValue(row, "snippet"); snippet != "" {
			writeLimitedWebEvidence(&b, "match", snippet, modelWebSearchEvidenceMaxRunes, nil)
		}
		if content := stringValue(row, "content"); content != "" && content != stringValue(row, "snippet") {
			writeLimitedWebEvidence(&b, "content", content, modelWebSearchEvidenceMaxRunes, nil)
		}
		if published := stringValue(row, "published_at"); published != "" {
			fmt.Fprintf(&b, "    <published>%s</published>\n", escapeText(published))
		}
		b.WriteString("  </page>\n")
		count++
	}
	b.WriteString("</retrieval>")
	if count == 0 {
		return r.CompactKnownText(fallback)
	}
	return b.String()
}

func writeLimitedWebEvidence(builder *strings.Builder, tag, value string, maxRunes int, remaining *int) {
	if value == "" {
		return
	}
	limit := maxRunes
	if remaining != nil && *remaining < limit {
		limit = *remaining
	}
	limited, truncated := truncateModelEvidence(value, limit)
	if remaining != nil {
		*remaining -= len([]rune(limited))
		if *remaining < 0 {
			*remaining = 0
		}
	}
	fmt.Fprintf(builder, "    <%s", tag)
	if truncated {
		builder.WriteString(" truncated=\"true\"")
	}
	fmt.Fprintf(builder, ">%s</%s>\n", escapeText(limited), tag)
}

func truncateModelEvidence(value string, maxRunes int) (string, bool) {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value, false
	}
	if maxRunes <= 0 {
		return "", true
	}
	return string(runes[:maxRunes]), true
}

func mapsValue(value interface{}) []map[string]interface{} {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(encoded, &rows); err != nil {
		return nil
	}
	return rows
}

func stringValue(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	switch value := values[key].(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	case json.Number:
		return value.String()
	default:
		return ""
	}
}

func intValue(values map[string]interface{}, key string) int {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		result, _ := value.Int64()
		return int(result)
	default:
		return 0
	}
}

func stringSliceValue(value interface{}) []string {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var values []string
	if err := json.Unmarshal(encoded, &values); err != nil {
		return nil
	}
	return values
}

func escapeText(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return replacer.Replace(value)
}
