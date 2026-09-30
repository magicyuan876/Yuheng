// Package findings is knowledge health: it runs detectors over a knowledge
// base when one of its documents changes and records what they find.
//
// The pieces:
//
//   - A Detector looks at one changed document in its knowledge base and
//     reports Candidates: problems that involve it. The core ships one, the
//     duplicate detector (duplicate.go), which compares stored vectors and
//     calls no model.
//   - The Runner runs every detector for a change and reconciles the results
//     with what is stored: new problems are recorded, problems a detector no
//     longer reports are resolved, and a dismissal survives until the evidence
//     changes.
//   - The Trigger schedules a check after a change, debounced, as the asynq
//     task TypeKnowledgeFindings; TaskHandler runs it.
//
// # Adding a detector
//
// A detector is provided into the dig value group DetectorGroup, the way the
// core provides its own:
//
//	c.Provide(newMyDetector, dig.Group(findings.DetectorGroup))
//
// An extension does this from its hook (see the extension package); nothing in
// the core has to change, and the runner picks the detector up on the next
// start. A detector owns its Name, its finding types and the keys it puts
// under FindingDetails.Extra; the runner only resolves findings of detectors
// that ran, so a detector that fails or does not apply leaves its earlier
// findings alone.
package findings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/magicyuan876/yuheng/internal/types"
)

// DetectorGroup is the dig value group detectors are provided into. The
// runner's parameter struct names the same group in its struct tag, which
// must be a literal; a test pins the two together.
const DetectorGroup = "finding_detectors"

// ErrUnsupported is returned, possibly wrapped, by a detector that cannot run
// for the scope it was given: the knowledge base is of a kind it does not
// check, or its retrieval engine lacks what it needs. The runner treats it as
// "did not run" rather than as a failure, so it is not retried and the
// detector's earlier findings are not resolved.
var ErrUnsupported = errors.New("detector does not apply here")

// Scope is what a detector is asked about: one knowledge entry that changed,
// in its knowledge base. Both are loaded by the runner's caller and must not
// be modified.
type Scope struct {
	TenantID      uint64
	KnowledgeBase *types.KnowledgeBase
	Knowledge     *types.Knowledge
}

// KnowledgeID is the changed entry's ID.
func (s Scope) KnowledgeID() string {
	if s.Knowledge == nil {
		return ""
	}
	return s.Knowledge.ID
}

// KnowledgeBaseID is the knowledge base's ID.
func (s Scope) KnowledgeBaseID() string {
	if s.KnowledgeBase == nil {
		return ""
	}
	return s.KnowledgeBase.ID
}

// Candidate is one problem a detector reports.
type Candidate struct {
	// Type names the kind of problem, e.g. types.FindingTypeDuplicate.
	Type string
	// Severity is one of the types.FindingSeverity* values.
	Severity string
	// SubjectKnowledgeID is the entry the finding is mainly about.
	SubjectKnowledgeID string
	// RelatedKnowledgeID is the other entry of a finding about two; empty for
	// a finding about one.
	RelatedKnowledgeID string
	// Score is the strength of the evidence, on a scale the detector defines
	// (the duplicate detector reports cosine similarity).
	Score float64
	// Fingerprint identifies the problem so a re-run updates the same row.
	// Empty means PairFingerprint(Type, knowledge base, subject, related),
	// which is right for any problem defined by the two entries alone.
	Fingerprint string
	// Details is the evidence shown to people. EvidenceHash should be set:
	// it decides whether a dismissed finding reopens.
	Details types.FindingDetails
	// Assign says who the finding is taken to. The zero value, the
	// subject's responsible person, suits a finding about one document.
	Assign AssignRule
}

// AssignRule says who a finding is taken to, in terms of the stewardship of
// its documents (types.KnowledgeSteward). The runner applies it on every
// check, so a hand-over or a new edit re-routes the open findings; a finding a
// person assigned by hand is left alone.
type AssignRule int

const (
	// AssignSubjectOwner takes the finding to whoever answers for the
	// subject: its owner, else its last reviewer, else the creator of its
	// knowledge base.
	AssignSubjectOwner AssignRule = iota
	// AssignLatestHand takes a finding about two documents to the person
	// who last worked on the one worked on most recently. A copy is made
	// by whoever wrote the newer text, and is cheapest to fix while they
	// still have it in mind.
	AssignLatestHand
	// AssignStalestOwner takes a finding about two documents to whoever
	// answers for the one nobody has vouched for the longest. When two
	// accounts disagree, the older one is the likelier to be out of date;
	// the other side sees the finding on its own document all the same.
	AssignStalestOwner
)

// Detector finds problems involving one changed knowledge entry.
type Detector interface {
	// Name identifies the detector in stored findings. It must be unique
	// and stable across releases: changing it orphans the findings stored
	// under the old name.
	Name() string
	// Detect reports every problem the detector sees that involves the
	// scope's entry. Returning ErrUnsupported means it does not apply; any
	// other error fails the check, which is retried.
	Detect(ctx context.Context, scope Scope) ([]Candidate, error)
}

// SupportChecker is optionally implemented by a detector that can tell,
// without running, whether it applies to a knowledge base. The health summary
// reports a base as supported when any detector says so.
type SupportChecker interface {
	Supports(ctx context.Context, kb *types.KnowledgeBase) (bool, error)
}

// PairFingerprint identifies a problem between two entries of a knowledge
// base independently of which of them is named first: the check triggered by
// either document arrives at the same fingerprint. related may be empty for a
// problem about one entry.
func PairFingerprint(findingType, kbID, a, b string) string {
	if b != "" && b < a {
		a, b = b, a
	}
	sum := sha256.Sum256([]byte(kbID + "|" + a + "|" + b))
	return findingType + ":" + hex.EncodeToString(sum[:])
}
