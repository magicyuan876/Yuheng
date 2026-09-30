package types

import "time"

// Feedback on answers: whether one helped, and why not.

// Message feedback ratings.
const (
	FeedbackUp   = "up"
	FeedbackDown = "down"
)

// MaxFeedbackCommentRunes bounds the comment on an answer.
const MaxFeedbackCommentRunes = 1000

// FindingTypeDisputed is a document cited by answers people marked as not
// helpful since anybody last vouched for it.
const FindingTypeDisputed = "disputed"

// MessageFeedback is one row of message_feedback: a person's rating of an
// answer.
type MessageFeedback struct {
	ID        string `json:"id"         gorm:"type:varchar(36);primaryKey"`
	TenantID  uint64 `json:"tenant_id"  gorm:"not null"`
	SessionID string `json:"session_id" gorm:"type:varchar(36);not null"`
	MessageID string `json:"message_id" gorm:"type:varchar(36);not null"`
	UserID    string `json:"user_id"    gorm:"type:varchar(36);not null"`
	Rating    string `json:"rating"     gorm:"type:varchar(8);not null"`
	Comment   string `json:"comment"    gorm:"type:text;not null;default:''"`
	// ShareQuestion attaches the question the answer answered to the
	// dispute; without it the question, asked in the person's own
	// conversation, stays there.
	ShareQuestion bool      `json:"share_question" gorm:"not null;default:false"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TableName pins the table name.
func (MessageFeedback) TableName() string { return "message_feedback" }

// SetMessageFeedbackRequest is the body of PUT
// /sessions/{id}/messages/{message_id}/feedback.
type SetMessageFeedbackRequest struct {
	// Rating is "up", "down", or empty to take the feedback back.
	Rating string `json:"rating"`
	// Comment says what was wrong. It is shown in the knowledge health of
	// the documents the answer cites, to whoever can read their knowledge
	// base.
	Comment string `json:"comment"`
	// ShareQuestion attaches the question to what is shown.
	ShareQuestion bool `json:"share_question"`
}

// MessageFeedbackView is a person's feedback on one answer.
type MessageFeedbackView struct {
	MessageID     string    `json:"message_id"`
	Rating        string    `json:"rating"`
	Comment       string    `json:"comment"`
	ShareQuestion bool      `json:"share_question"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// DisputeReport is one down-vote as a dispute finding shows it: what the
// person said, the start of the answer, and the question when they attached
// it. It does not say who: the feedback is about the document, and it is given
// without being answerable to the document's maintainer.
type DisputeReport struct {
	FeedbackID string    `json:"feedback_id" gorm:"column:feedback_id"`
	Question   string    `json:"question"    gorm:"column:question"`
	Answer     string    `json:"answer"      gorm:"column:answer"`
	Comment    string    `json:"comment"     gorm:"column:comment"`
	At         time.Time `json:"at"          gorm:"column:at"`
}
