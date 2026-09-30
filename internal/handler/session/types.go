package session

import (
	"github.com/magicyuan876/yuheng/internal/types"
)

// CreateSessionRequest represents a request to create a new session
// Sessions are now knowledge-base-independent and serve as conversation containers.
// Per-turn configuration (knowledge bases, files, models) arrives with each chat request instead.
type CreateSessionRequest struct {
	// Title for the session (optional)
	Title string `json:"title"`
	// Description for the session (optional)
	Description string `json:"description"`
}

// GenerateTitleRequest defines the request structure for generating a session title
type GenerateTitleRequest struct {
	Messages []types.Message `json:"messages" binding:"required"` // Messages to use as context for title generation
}

// MentionedItemRequest represents a mentioned item in the request
type MentionedItemRequest struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`    // "kb", "file", or "tag"
	KBType string `json:"kb_type"` // "document" or "faq" (only for kb type)
	KBID   string `json:"kb_id"`   // Parent knowledge base for file/tag mentions
	KBName string `json:"kb_name"` // Display name for parent KB
}

// ImageAttachment represents an image in a chat request.
// Frontend sends base64 data in the Data field; the backend saves, runs VLM analysis,
// and populates URL/Caption before proceeding with the chat pipeline.
type ImageAttachment struct {
	Data    string `json:"data,omitempty"`    // base64 data URI from frontend (data:image/png;base64,...)
	URL     string `json:"url,omitempty"`     // serving URL after saving to storage
	Caption string `json:"caption,omitempty"` // VLM analysis result (context-aware, single call)
}

// CreateKnowledgeQARequest defines the request structure for knowledge QA
type CreateKnowledgeQARequest struct {
	Query                 string                       `json:"query"              binding:"required"` // Query text for knowledge base search
	KnowledgeBaseIDs      []string                     `json:"knowledge_base_ids"`                    // Selected knowledge base ID for this request
	KnowledgeIds          []string                     `json:"knowledge_ids"`                         // Selected knowledge ID for this request
	WebSearchEnabled      bool                         `json:"web_search_enabled"`                    // Whether web search is enabled for this request
	SummaryModelID        string                       `json:"summary_model_id"`                      // Optional summary model ID for this request (overrides session default)
	TagIDs                []string                     `json:"tag_ids"`                               // @mentioned tag IDs (display/debug; scoped via MentionedItems)
	MentionedItems        []MentionedItemRequest       `json:"mentioned_items"`                       // @mentioned knowledge bases and files
	DisableTitle          bool                         `json:"disable_title"`                         // Whether to disable auto title generation
	Images                []ImageAttachment            `json:"images"`                                // Attached images for multimodal chat
	AttachmentUploads     []AttachmentUpload           `json:"attachment_uploads,omitempty"`          // Attached files (documents, audio, etc.)
	AttachmentIDs         []string                     `json:"attachment_ids,omitempty"`              // Pre-uploaded session-scoped document IDs
	Channel               string                       `json:"channel"`                               // Source channel: "web", "api", etc.
	SuggestionAttribution *types.SuggestionAttribution `json:"suggestion_attribution,omitempty"`
}

// AttachmentUpload represents a file attachment upload from the client
type AttachmentUpload struct {
	Data     string `json:"data"`      // Base64-encoded file content
	FileName string `json:"file_name"` // Original filename
	FileSize int64  `json:"file_size"` // File size in bytes
}

// SearchKnowledgeRequest defines the request structure for searching knowledge without LLM summarization
type SearchKnowledgeRequest struct {
	Query            string                 `json:"query"              binding:"required"` // Query text to search for
	KnowledgeBaseIDs []string               `json:"knowledge_base_ids"`                    // IDs of knowledge bases to search (multi-KB support)
	KnowledgeIDs     []string               `json:"knowledge_ids"`                         // IDs of specific knowledge (files) to search
	TagIDs           []string               `json:"tag_ids"`                               // Tag IDs for filtering within a single KB
	MentionedItems   []MentionedItemRequest `json:"mentioned_items"`                       // Optional scoped tag mentions
}

// StopSessionRequest represents the stop session request
type StopSessionRequest struct {
	MessageID string `json:"message_id" binding:"required"`
}
