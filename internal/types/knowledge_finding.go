package types

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// Knowledge health: problems found in a knowledge base by comparing its
// documents with each other, recorded as findings.
//
// A finding is produced by a detector (see the findings service package) and
// identified by a fingerprint, so that running the same detector again over
// unchanged content updates the same row instead of adding another. The core
// ships one detector (near-duplicate documents); the finding type, detector
// name and details are open strings so that detectors added later — by the
// core or by an extension — need no schema change.

// Finding types the core produces. Other detectors may define their own.
const (
	// FindingTypeDuplicate is two documents of one knowledge base sharing
	// passages word for word: one is, in part or whole, a copy.
	FindingTypeDuplicate = "duplicate"
	// FindingTypeDivergent is two documents with passages that are nearly
	// the same and not quite: an edited copy, one of which is probably out
	// of date, or two accounts of one thing that disagree.
	FindingTypeDivergent = "divergent"
)

// Finding severities, in increasing order of urgency.
const (
	FindingSeverityInfo    = "info"
	FindingSeverityWarning = "warning"
	FindingSeverityError   = "error"
)

// Finding statuses.
//
// open is a problem nobody has acted on. dismissed is one somebody looked at
// and decided to leave alone; it stays dismissed while the evidence is the
// same, and reopens when the evidence changes. resolved is one the detector
// no longer reports, because the content changed.
const (
	FindingStatusOpen      = "open"
	FindingStatusDismissed = "dismissed"
	FindingStatusResolved  = "resolved"
)

// FindingResolvedBySystem is the resolved_by of a finding closed because the
// detector stopped reporting it, as opposed to a person's user ID.
const FindingResolvedBySystem = "system"

// Finding resolutions: why a finding was closed. A dismissal names one of the
// two reasons a person can have to leave a problem as it is; a finding the
// detectors stopped reporting is cleared.
const (
	// FindingResolutionDistinctScope: the documents look alike but apply
	// to different things — two offices, two products, two years.
	FindingResolutionDistinctScope = "distinct_scope"
	// FindingResolutionIntentional: the overlap is wanted, e.g. a summary
	// page that quotes its sources.
	FindingResolutionIntentional = "intentional"
	// FindingResolutionCleared: the content changed and the problem is
	// gone.
	FindingResolutionCleared = "cleared"
)

// ValidDismissReason reports whether r is a reason a person may give for
// dismissing a finding.
func ValidDismissReason(r string) bool {
	return r == FindingResolutionDistinctScope || r == FindingResolutionIntentional
}

// ValidFindingSeverity reports whether s is one of the defined severities.
func ValidFindingSeverity(s string) bool {
	switch s {
	case FindingSeverityInfo, FindingSeverityWarning, FindingSeverityError:
		return true
	}
	return false
}

// KnowledgeFinding is one row of knowledge_findings.
type KnowledgeFinding struct {
	ID              string `json:"id"                gorm:"type:varchar(36);primaryKey"`
	TenantID        uint64 `json:"tenant_id"         gorm:"not null"`
	KnowledgeBaseID string `json:"knowledge_base_id" gorm:"type:varchar(36);not null"`
	Type            string `json:"type"              gorm:"type:varchar(32);not null"`
	Detector        string `json:"detector"          gorm:"type:varchar(64);not null"`
	Severity        string `json:"severity"          gorm:"type:varchar(16);not null"`
	Status          string `json:"status"            gorm:"type:varchar(16);not null;default:'open'"`
	// Fingerprint identifies the problem independently of when and from which
	// side it was detected; it is unique per tenant.
	Fingerprint        string         `json:"fingerprint"          gorm:"type:varchar(128);not null"`
	SubjectKnowledgeID string         `json:"subject_knowledge_id" gorm:"type:varchar(36);not null"`
	RelatedKnowledgeID *string        `json:"related_knowledge_id" gorm:"type:varchar(36)"`
	Score              *float64       `json:"score"                gorm:"type:real"`
	Details            FindingDetails `json:"details"              gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	ResolvedAt         *time.Time     `json:"resolved_at"`
	ResolvedBy         *string        `json:"resolved_by"          gorm:"type:varchar(64)"`
	// Resolution says why a closed finding was closed (FindingResolution*).
	Resolution *string `json:"resolution" gorm:"type:varchar(32)"`
	// AssigneeID is the person the finding is taken to. The runner routes
	// it on every check unless AssignedBy is set: somebody assigned it by
	// hand, and a check must not undo that.
	AssigneeID *string `json:"assignee_id" gorm:"type:varchar(36)"`
	AssignedBy *string `json:"assigned_by" gorm:"type:varchar(36)"`
}

// TableName pins the table name.
func (KnowledgeFinding) TableName() string { return "knowledge_findings" }

// FindingEvidence is one pair of passages supporting a finding: a chunk of
// the subject document, the chunk of the related document it resembles, and
// how similar the two are.
type FindingEvidence struct {
	SubjectChunkID string  `json:"subject_chunk_id"`
	SubjectExcerpt string  `json:"subject_excerpt"`
	RelatedChunkID string  `json:"related_chunk_id"`
	RelatedExcerpt string  `json:"related_excerpt"`
	Score          float64 `json:"score"`
	// Differs is true when the two passages differ inside the text they
	// share; the excerpts are then centred on the first difference.
	Differs bool `json:"differs,omitempty"`
}

// Flipped returns the pair as seen from the related document.
func (e FindingEvidence) Flipped() FindingEvidence {
	return FindingEvidence{
		SubjectChunkID: e.RelatedChunkID, SubjectExcerpt: e.RelatedExcerpt,
		RelatedChunkID: e.SubjectChunkID, RelatedExcerpt: e.SubjectExcerpt,
		Score: e.Score, Differs: e.Differs,
	}
}

// FindingDetails is the evidence column. Every detector fills Evidence and
// EvidenceHash; OverlapRatio is meaningful for comparisons of two documents;
// Extra carries whatever else a detector wants to show, under keys of its own.
type FindingDetails struct {
	Evidence []FindingEvidence `json:"evidence,omitempty"`
	// OverlapRatio is the share of a document's passages that have a match in
	// the other one, taken from the document for which it is larger.
	OverlapRatio float64 `json:"overlap_ratio,omitempty"`
	// EvidenceHash summarises what the finding is based on. A dismissed
	// finding reopens when a later run arrives at a different hash: the
	// person dismissed that evidence, not whatever the documents become.
	EvidenceHash string         `json:"evidence_hash,omitempty"`
	Extra        map[string]any `json:"extra,omitempty"`
}

// Scan implements sql.Scanner.
func (d *FindingDetails) Scan(value any) error {
	var raw []byte
	switch v := value.(type) {
	case nil:
		*d = FindingDetails{}
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return errors.New("finding details: unsupported column type")
	}
	if len(raw) == 0 {
		*d = FindingDetails{}
		return nil
	}
	return json.Unmarshal(raw, d)
}

// Value implements driver.Valuer.
func (d FindingDetails) Value() (driver.Value, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// FindingRun is the outcome of running the detectors over one changed
// knowledge entry, as the repository records it.
type FindingRun struct {
	TenantID        uint64
	KnowledgeBaseID string
	KnowledgeID     string
	// Detectors are the detectors that completed. Only their findings are
	// candidates for being resolved: a detector that failed or could not run
	// says nothing about whether its earlier findings still hold.
	Detectors []string
	// Findings are what those detectors report now. ID, Status, timestamps
	// and the resolution columns are assigned by the repository.
	Findings []*KnowledgeFinding
}

// KnowledgeRef names a knowledge entry in a finding.
type KnowledgeRef struct {
	KnowledgeID string `json:"knowledge_id"`
	Title       string `json:"title"`
}

// KnowledgeFindingView is a finding as the API returns it.
type KnowledgeFindingView struct {
	ID              string            `json:"id"`
	KnowledgeBaseID string            `json:"knowledge_base_id"`
	Type            string            `json:"type"`
	Detector        string            `json:"detector"`
	Severity        string            `json:"severity"`
	Status          string            `json:"status"`
	Score           float64           `json:"score"`
	OverlapRatio    float64           `json:"overlap_ratio"`
	Subject         KnowledgeRef      `json:"subject"`
	Related         *KnowledgeRef     `json:"related"`
	Evidence        []FindingEvidence `json:"evidence"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	ResolvedAt      *time.Time        `json:"resolved_at"`
	ResolvedBy      *string           `json:"resolved_by"`
	Resolution      *string           `json:"resolution"`
	// Assignee is who the finding is taken to; nil when nobody could be
	// found. AssignedManually says a person chose them.
	Assignee         *PersonRef `json:"assignee"`
	AssignedManually bool       `json:"assigned_manually"`
	// KnowledgeBaseName is filled in listings that span knowledge bases.
	KnowledgeBaseName string `json:"knowledge_base_name,omitempty"`
}

// KnowledgeFindingRow is a finding with the titles of the entries it names,
// which the list query joins in.
type KnowledgeFindingRow struct {
	KnowledgeFinding
	SubjectTitle      string
	RelatedTitle      string
	KnowledgeBaseName string
}

// View renders the row for the API.
func (r *KnowledgeFindingRow) View() *KnowledgeFindingView {
	v := &KnowledgeFindingView{
		ID: r.ID, KnowledgeBaseID: r.KnowledgeBaseID, Type: r.Type, Detector: r.Detector,
		Severity: r.Severity, Status: r.Status, OverlapRatio: r.Details.OverlapRatio,
		Subject:   KnowledgeRef{KnowledgeID: r.SubjectKnowledgeID, Title: r.SubjectTitle},
		Evidence:  append([]FindingEvidence{}, r.Details.Evidence...),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ResolvedAt: r.ResolvedAt, ResolvedBy: r.ResolvedBy,
		Resolution: r.Resolution, AssignedManually: r.AssignedBy != nil && *r.AssignedBy != "",
		KnowledgeBaseName: r.KnowledgeBaseName,
	}
	if r.AssigneeID != nil && *r.AssigneeID != "" {
		// Named by the service, which knows the users; the ID alone until
		// then.
		v.Assignee = &PersonRef{ID: *r.AssigneeID, Active: true}
	}
	if r.Score != nil {
		v.Score = *r.Score
	}
	if r.RelatedKnowledgeID != nil {
		v.Related = &KnowledgeRef{KnowledgeID: *r.RelatedKnowledgeID, Title: r.RelatedTitle}
	}
	return v
}

// KnowledgeFindingFilter narrows a finding list. Status "all" (or empty after
// defaulting) lists every status.
type KnowledgeFindingFilter struct {
	Status      string
	Type        string
	KnowledgeID string
	// AssigneeID lists only the findings assigned to that person. The
	// service sets it from Mine; the API does not take other people's.
	AssigneeID string
	// Mine lists only the findings assigned to the caller.
	Mine     bool
	Page     int
	PageSize int
}

// KnowledgeFindingPage is one page of a finding list.
type KnowledgeFindingPage struct {
	Items    []*KnowledgeFindingView `json:"items"`
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

// KnowledgeFindingSummary is the health overview of one knowledge base.
type KnowledgeFindingSummary struct {
	OpenTotal  int64            `json:"open_total"`
	OpenByType map[string]int64 `json:"open_by_type"`
	// LastScanAt is when a knowledge entry of the base was last checked; nil
	// when none ever was.
	LastScanAt *time.Time `json:"last_scan_at"`
	// Enabled is the deployment switch (YUHENG_FINDINGS_ENABLED).
	Enabled bool `json:"enabled"`
	// Supported says whether this knowledge base can be checked at all: its
	// retrieval engine must be able to compare stored vectors, and FAQ bases
	// are not checked (they deduplicate their entries on import).
	Supported bool `json:"supported"`
}

// KnowledgeFindingScanResult answers a request to check a whole base again.
type KnowledgeFindingScanResult struct {
	Queued int `json:"queued"`
}

// ChunkSimilarity is one pair of stored chunk vectors found to be close: a
// chunk of the knowledge entry asked about and a chunk of another entry of the
// same knowledge base.
type ChunkSimilarity struct {
	SubjectChunkID     string  `json:"subject_chunk_id"     gorm:"column:subject_chunk_id"`
	RelatedChunkID     string  `json:"related_chunk_id"     gorm:"column:related_chunk_id"`
	RelatedKnowledgeID string  `json:"related_knowledge_id" gorm:"column:related_knowledge_id"`
	Score              float64 `json:"score"                gorm:"column:score"`
}

// KnowledgeFindingsPayload is the payload of TypeKnowledgeFindings.
//
// It deliberately carries nothing but the scope — no tracing context, no
// timestamp, no language — because the task is deduplicated by asynq's
// Unique option, which compares payloads byte for byte: two changes to one
// entry within the debounce window must produce the same payload to collapse
// into one run.
type KnowledgeFindingsPayload struct {
	TenantID        uint64 `json:"tenant_id"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
	KnowledgeID     string `json:"knowledge_id"`
}
