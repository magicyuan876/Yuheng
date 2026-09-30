package findings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/magicyuan876/yuheng/internal/types"
)

// Disputes: documents cited by answers people marked as not helpful.
//
// A down-vote is somebody saying an answer was wrong, and the documents it
// cites are where it came from. The detector counts the down-votes on answers
// citing a document since anybody last vouched for it, and while there are
// any, a disputed finding takes them — the question, the start of the answer,
// what the person said — to the document's owner. Vouching for the document,
// by confirming it or changing it, starts the count again: the owner has
// looked, and the finding resolves at the check that follows. A new down-vote
// after that is a new dispute.

// DisputeDetectorName is the dispute detector's stored name.
const DisputeDetectorName = "feedback"

const (
	// disputeWarningCount is how many down-votes make a dispute a warning
	// rather than a note: once can be the question, three is the document.
	disputeWarningCount = 3
	// disputeReports is how many reports a finding shows, newest first.
	disputeReports = 3
)

// DisputeLookup counts and lists the down-votes on answers citing an entry.
// interfaces.MessageFeedbackRepository satisfies it.
type DisputeLookup interface {
	DisputesSince(ctx context.Context, tenantID uint64, knowledgeID string, since time.Time,
		limit int) (int, []types.DisputeReport, error)
}

// DisputeDetector reports entries cited by answers people disputed.
type DisputeDetector struct {
	disputes DisputeLookup
	stewards StewardLookup
}

// NewDisputeDetector returns the detector.
func NewDisputeDetector(disputes DisputeLookup, stewards StewardLookup) *DisputeDetector {
	return &DisputeDetector{disputes: disputes, stewards: stewards}
}

// Name implements Detector.
func (d *DisputeDetector) Name() string { return DisputeDetectorName }

// Detect implements Detector.
func (d *DisputeDetector) Detect(ctx context.Context, scope Scope) ([]Candidate, error) {
	id := scope.KnowledgeID()
	stewards, err := d.stewards.Stewards(ctx, scope.TenantID, []string{id})
	if err != nil {
		return nil, err
	}
	st, ok := stewards[id]
	if !ok {
		return nil, nil
	}
	count, reports, err := d.disputes.DisputesSince(ctx, scope.TenantID, id, st.LastVouchedAt(), disputeReports)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}
	severity := types.FindingSeverityInfo
	if count >= disputeWarningCount {
		severity = types.FindingSeverityWarning
	}
	return []Candidate{{
		Type: types.FindingTypeDisputed, Severity: severity, Assign: AssignSubjectOwner,
		SubjectKnowledgeID: id, Score: float64(count),
		Details: types.FindingDetails{
			// A new down-vote changes the hash, so a dismissed dispute
			// reopens when somebody else finds the answer wrong too.
			EvidenceHash: disputeHash(count, reports),
			Extra:        map[string]any{"count": count, "reports": reports},
		},
	}}, nil
}

func disputeHash(count int, reports []types.DisputeReport) string {
	latest := ""
	if len(reports) > 0 {
		latest = reports[0].FeedbackID + "@" + reports[0].At.UTC().Format(time.RFC3339Nano)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%s", count, latest)))
	return hex.EncodeToString(sum[:])
}
