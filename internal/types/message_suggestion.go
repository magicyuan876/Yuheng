package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	SuggestionPlacementAfterAnswer = "after_answer"

	SuggestionStatusGenerating = "generating"
	SuggestionStatusReady      = "ready"
	SuggestionStatusSuppressed = "suppressed"
	SuggestionStatusFailed     = "failed"

	SuggestionEventImpression = "impression"
	SuggestionEventClick      = "click"
	SuggestionEventDismiss    = "dismiss"
	SuggestionEventRegenerate = "regenerate"
)

// SuggestionAttribution is carried by the next user message after a click, so
// analytics can distinguish a click from a question that was actually sent.
type SuggestionAttribution struct {
	SuggestionSetID string `json:"suggestion_set_id"`
	QuestionID      string `json:"question_id"`
}

// SuggestionItem is a stable, attributable question rendered to an end user.
type SuggestionItem struct {
	ID               string   `json:"id"`
	Text             string   `json:"text"`
	Category         string   `json:"category,omitempty"`
	Source           string   `json:"source"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids,omitempty"`
}

type SuggestionItems []SuggestionItem

func (s SuggestionItems) Value() (driver.Value, error) {
	if s == nil {
		s = SuggestionItems{}
	}
	return json.Marshal(s)
}

func (s *SuggestionItems) Scan(value interface{}) error {
	if value == nil {
		*s = SuggestionItems{}
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		*s = SuggestionItems{}
		return nil
	}
	return json.Unmarshal(b, s)
}

// MessageSuggestionSet is the durable generation/cache record for one
// assistant message and one effective suggestion configuration.
type MessageSuggestionSet struct {
	ID                 string          `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID           uint64          `json:"tenant_id" gorm:"not null;index"`
	SessionID          string          `json:"session_id" gorm:"type:varchar(36);not null;index"`
	AssistantMessageID string          `json:"assistant_message_id" gorm:"type:varchar(36);not null;index"`
	Placement          string          `json:"placement" gorm:"type:varchar(32);not null"`
	ConfigHash         string          `json:"config_hash" gorm:"type:varchar(64);not null"`
	Locale             string          `json:"locale" gorm:"type:varchar(16);not null;default:''"`
	Status             string          `json:"status" gorm:"type:varchar(16);not null;index"`
	AllowRegenerate    bool            `json:"allow_regenerate" gorm:"not null;default:false"`
	SuppressionReason  string          `json:"suppression_reason,omitempty" gorm:"type:varchar(64);not null;default:''"`
	Questions          SuggestionItems `json:"questions" gorm:"type:jsonb;not null"`
	ModelID            string          `json:"model_id,omitempty" gorm:"type:varchar(64);not null;default:''"`
	PromptTokens       int             `json:"prompt_tokens,omitempty" gorm:"not null;default:0"`
	CompletionTokens   int             `json:"completion_tokens,omitempty" gorm:"not null;default:0"`
	LatencyMs          int64           `json:"latency_ms,omitempty" gorm:"not null;default:0"`
	ErrorCode          string          `json:"error_code,omitempty" gorm:"type:varchar(64);not null;default:''"`
	LeaseUntil         *time.Time      `json:"-"`
	GeneratedAt        *time.Time      `json:"generated_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

func (MessageSuggestionSet) TableName() string { return "message_suggestion_sets" }

func (s *MessageSuggestionSet) BeforeCreate(_ *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	if s.Questions == nil {
		s.Questions = SuggestionItems{}
	}
	return nil
}

// MessageSuggestionEvent stores product analytics separately from the
// security audit log. It references question IDs rather than copying text.
type MessageSuggestionEvent struct {
	ID              uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	TenantID        uint64    `json:"tenant_id" gorm:"not null;index"`
	SessionID       string    `json:"session_id" gorm:"type:varchar(36);not null;index"`
	SuggestionSetID string    `json:"suggestion_set_id" gorm:"type:varchar(36);not null;index"`
	QuestionID      string    `json:"question_id,omitempty" gorm:"type:varchar(64);not null;default:''"`
	EventType       string    `json:"event_type" gorm:"type:varchar(32);not null;index"`
	ActorID         string    `json:"-" gorm:"type:varchar(512);not null;default:''"`
	CreatedAt       time.Time `json:"created_at" gorm:"index"`
}

func (MessageSuggestionEvent) TableName() string { return "message_suggestion_events" }

const (
	SuggestionModeCurated   = "curated"
	SuggestionModeKnowledge = "knowledge"
	SuggestionModeGenerated = "generated"
	SuggestionModeHybrid    = "hybrid"

	SuggestionCategoryClarify = "clarify"
	SuggestionCategoryDeepen  = "deepen"
	SuggestionCategoryAction  = "action"
)

// QuestionSuggestionConfig is the configuration for question suggestions.
// Channel settings may suppress rendering, but never override this
// content/generation policy.
type QuestionSuggestionConfig struct {
	Starters  StarterSuggestionConfig  `yaml:"starters" json:"starters"`
	FollowUps FollowUpSuggestionConfig `yaml:"follow_ups" json:"follow_ups"`
}

// StarterSuggestionConfig controls prompts shown before the first user turn.
type StarterSuggestionConfig struct {
	Enabled bool     `yaml:"enabled" json:"enabled"`
	Mode    string   `yaml:"mode" json:"mode"`
	Items   []string `yaml:"items" json:"items"`
	Count   int      `yaml:"count" json:"count"`
}

// FollowUpSuggestionConfig controls contextual questions generated after a
// completed assistant answer.
type FollowUpSuggestionConfig struct {
	Enabled                        bool     `yaml:"enabled" json:"enabled"`
	Mode                           string   `yaml:"mode" json:"mode"`
	Count                          int      `yaml:"count" json:"count"`
	ModelID                        string   `yaml:"model_id,omitempty" json:"model_id,omitempty"`
	AdditionalInstruction          string   `yaml:"additional_instruction,omitempty" json:"additional_instruction,omitempty"`
	Categories                     []string `yaml:"categories,omitempty" json:"categories,omitempty"`
	MaxContextTurns                int      `yaml:"max_context_turns" json:"max_context_turns"`
	SuppressOnFallback             bool     `yaml:"suppress_on_fallback" json:"suppress_on_fallback"`
	SuppressWhenAnswerAsksQuestion bool     `yaml:"suppress_when_answer_asks_question" json:"suppress_when_answer_asks_question"`
	KnowledgeFallback              bool     `yaml:"knowledge_fallback" json:"knowledge_fallback"`
	AllowRegenerate                bool     `yaml:"allow_regenerate" json:"allow_regenerate"`
}

// EnsureDefaults normalizes suggestion configuration without overriding
// explicit enable/disable choices.
func (c *QuestionSuggestionConfig) EnsureDefaults() {
	if c.Starters.Mode == "" {
		c.Starters.Mode = SuggestionModeHybrid
	}
	if c.Starters.Count <= 0 {
		c.Starters.Count = 6
	}
	if c.Starters.Items == nil {
		c.Starters.Items = []string{}
	}
	if c.FollowUps.Mode == "" {
		c.FollowUps.Mode = SuggestionModeHybrid
	}
	if c.FollowUps.Count <= 0 {
		c.FollowUps.Count = 3
	}
	if c.FollowUps.MaxContextTurns <= 0 {
		c.FollowUps.MaxContextTurns = 2
	}
	if len(c.FollowUps.Categories) == 0 {
		c.FollowUps.Categories = []string{
			SuggestionCategoryClarify,
			SuggestionCategoryDeepen,
			SuggestionCategoryAction,
		}
	}
}

// Validate rejects invalid suggestion settings before they are persisted or
// used to incur a model call.
func (c *QuestionSuggestionConfig) Validate() error {
	if c == nil {
		return nil
	}
	if c.Starters.Count < 1 || c.Starters.Count > 8 {
		return fmt.Errorf("starter suggestion count must be between 1 and 8")
	}
	if c.FollowUps.Count < 1 || c.FollowUps.Count > 5 {
		return fmt.Errorf("follow-up suggestion count must be between 1 and 5")
	}
	if c.FollowUps.MaxContextTurns < 1 || c.FollowUps.MaxContextTurns > 5 {
		return fmt.Errorf("follow-up max_context_turns must be between 1 and 5")
	}
	if !oneOf(c.Starters.Mode, SuggestionModeCurated, SuggestionModeKnowledge, SuggestionModeHybrid) {
		return fmt.Errorf("invalid starter suggestion mode %q", c.Starters.Mode)
	}
	if !oneOf(c.FollowUps.Mode, SuggestionModeGenerated, SuggestionModeKnowledge, SuggestionModeHybrid) {
		return fmt.Errorf("invalid follow-up suggestion mode %q", c.FollowUps.Mode)
	}
	for i, item := range c.Starters.Items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			return fmt.Errorf("starter suggestion %d cannot be empty", i+1)
		}
		if len([]rune(trimmed)) > 200 {
			return fmt.Errorf("starter suggestion %d exceeds 200 characters", i+1)
		}
	}
	if len([]rune(strings.TrimSpace(c.FollowUps.AdditionalInstruction))) > 2000 {
		return fmt.Errorf("follow-up additional_instruction exceeds 2000 characters")
	}
	for _, category := range c.FollowUps.Categories {
		if !oneOf(category, SuggestionCategoryClarify, SuggestionCategoryDeepen, SuggestionCategoryAction) {
			return fmt.Errorf("invalid follow-up suggestion category %q", category)
		}
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

// SuggestedQuestion is a recommended follow-up question with its provenance.
type SuggestedQuestion struct {
	// 问题文本
	Question string `json:"question"`
	// 来源类型: "faq", "document", "wiki"
	Source string `json:"source"`
	// 来源知识库ID（仅 faq/document/wiki 来源时有值）
	KnowledgeBaseID string `json:"knowledge_base_id,omitempty"`
}
