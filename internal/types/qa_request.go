package types

// QARequest consolidates all parameters for KnowledgeQA service calls,
// replacing the previous 14-parameter method signatures.
// EventBus is passed separately to avoid circular dependency with the event package.
type QARequest struct {
	Session            *Session           // The conversation session
	Query              string             // User query text
	AssistantMessageID string             // Pre-created assistant message ID
	SummaryModelID     string             // Optional model override; empty = use KB default
	KnowledgeBaseIDs   []string           // Knowledge base IDs to search (from request + @mentions)
	KnowledgeIDs       []string           // Specific knowledge (file) IDs to search
	TagScopes          []TagScope         // Tag-constrained KB scopes from @mentions
	ImageURLs          []string           // Image URLs for multimodal input
	ImageDescription   string             // VLM-generated image description (fallback for non-vision models)
	UserMessageID      string             // Created user message ID
	WebSearchEnabled   bool               // Whether web search is enabled for this request
	QuotedContext      string             // Quoted message content from quote-reply (appended at LLM prompt stage, not used for retrieval)
	Attachments        MessageAttachments // File attachments (processed and ready for prompt injection)
}
