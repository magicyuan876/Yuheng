package session

import (
	"fmt"
	"strings"

	"github.com/magicyuan876/yuheng/internal/types"
)

// persistStripFields lists bulky Data keys to drop before SSE replay / DB storage.
var persistStripFields = map[string][]string{
	"knowledge_chunks_list": {"chunks"},
	"grep_results":          {"chunk_results"},
}

// ShouldOmitRawToolOutput reports whether the raw XML/text Output should be
// excluded from SSE replay and persisted agent_steps. The full Output remains
// available in-memory for the current turn.
func ShouldOmitRawToolOutput(_ string, data map[string]interface{}) bool {
	if data == nil {
		return false
	}
	displayType, ok := data["display_type"].(string)
	return ok && displayType != ""
}

func sanitizeToolData(data map[string]interface{}, extraOmit []string) map[string]interface{} {
	if data == nil {
		return nil
	}
	out := make(map[string]interface{}, len(data))
	for k, v := range data {
		out[k] = v
	}
	displayType := toolDisplayStringField(data, "display_type")
	for _, key := range persistStripFields[displayType] {
		delete(out, key)
	}
	for _, key := range extraOmit {
		delete(out, key)
	}
	return out
}

// SanitizeToolResultForClient builds stream / persistence metadata for the UI.
func SanitizeToolResultForClient(toolName string, result *types.ToolResult) map[string]interface{} {
	meta := map[string]interface{}{}
	if result == nil {
		return meta
	}
	if result.Data != nil {
		for k, v := range sanitizeToolData(result.Data, nil) {
			meta[k] = v
		}
	}
	if !ShouldOmitRawToolOutput("", result.Data) && result.Output != "" {
		meta["output"] = result.Output
	}
	return meta
}

// StreamContentForToolResult is the short SSE Content field for tool results.
func StreamContentForToolResult(toolName string, success bool, errMsg string, data map[string]interface{}) string {
	if !success {
		return errMsg
	}
	if ShouldOmitRawToolOutput(toolName, data) {
		return compactToolSummary(success, errMsg, data)
	}
	return ""
}

func compactToolSummary(success bool, errMsg string, data map[string]interface{}) string {
	if !success {
		if errMsg != "" {
			return "Error: " + errMsg
		}
		return "Error: tool call failed"
	}
	switch toolDisplayStringField(data, "display_type") {
	case "knowledge_chunks_list":
		title := toolDisplayStringField(data, "knowledge_title")
		if title == "" {
			title = toolDisplayStringField(data, "knowledge_id")
		}
		fetched := toolDisplayIntField(data, "fetched_chunks")
		total := toolDisplayIntField(data, "total_chunks")
		if q := toolDisplayStringField(data, "faq_question"); q != "" {
			return fmt.Sprintf("Loaded FAQ entry: %s (content omitted from history)", q)
		}
		if title != "" && total > 0 {
			return fmt.Sprintf("Listed %d/%d chunks from %s (content omitted from history)", fetched, total, title)
		}
		if title != "" {
			return fmt.Sprintf("Listed chunks from %s (content omitted from history)", title)
		}
	case "grep_results":
		chunks := toolDisplayIntField(data, "total_matches")
		docs := toolDisplayIntField(data, "document_count")
		if docs == 0 {
			docs = toolDisplayIntField(data, "result_count")
		}
		if chunks > 0 {
			return fmt.Sprintf("Keyword search found %d matching chunks across %d document(s) (details omitted from history)", chunks, docs)
		}
	case "search_results":
		count := toolDisplayIntField(data, "result_count")
		if count == 0 {
			count = toolDisplayIntField(data, "count")
		}
		if count > 0 {
			return fmt.Sprintf("Semantic search returned %d result(s) (details omitted from history)", count)
		}
	case "attachment_parsing":
		parsed := toolDisplayIntField(data, "parsed_count")
		skipped := toolDisplayIntField(data, "skipped_count")
		if skipped > 0 {
			return fmt.Sprintf("Parsed %d attachment(s), %d skipped (still processing)", parsed, skipped)
		}
		if parsed > 0 {
			return fmt.Sprintf("Parsed %d attachment(s)", parsed)
		}
	}
	if displayType := toolDisplayStringField(data, "display_type"); displayType != "" {
		return fmt.Sprintf("Tool completed (%s; payload omitted from history)", displayType)
	}
	return "Tool completed (payload omitted from history)"
}

func toolDisplayStringField(data map[string]interface{}, key string) string {
	if data == nil {
		return ""
	}
	v, ok := data[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func toolDisplayIntField(data map[string]interface{}, key string) int {
	if data == nil {
		return 0
	}
	v, ok := data[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	default:
		return 0
	}
}
