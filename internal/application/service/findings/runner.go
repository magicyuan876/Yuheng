package findings

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"go.uber.org/dig"

	"github.com/magicyuan876/yuheng/internal/extension"
	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// RunnerParams collects the detectors provided into DetectorGroup.
type RunnerParams struct {
	dig.In

	// The tag must spell DetectorGroup; TestRunnerParamsNameTheDetectorGroup
	// fails if the two drift apart.
	Detectors []Detector `group:"finding_detectors"`
	Repo      interfaces.KnowledgeFindingRepository
}

// Runner runs the detectors over a changed knowledge entry and records the
// outcome.
type Runner struct {
	detectors []Detector
	repo      interfaces.KnowledgeFindingRepository
}

// NewRunnerFromContainer builds the runner from the container's detectors. It
// refuses to be built before the extension hooks have run, for the reason
// NewEngineCatalog does: a runner built earlier would silently lack the
// detectors an extension provides.
func NewRunnerFromContainer(p RunnerParams) (*Runner, error) {
	if err := extension.RequireApplied("finding detectors"); err != nil {
		return nil, err
	}
	return NewRunner(p.Repo, p.Detectors...)
}

// NewRunner builds a runner over the given detectors. Names must be present
// and unique: findings are stored and resolved by detector name, so two
// detectors sharing one would resolve each other's findings.
func NewRunner(repo interfaces.KnowledgeFindingRepository, detectors ...Detector) (*Runner, error) {
	seen := make(map[string]bool, len(detectors))
	kept := make([]Detector, 0, len(detectors))
	for _, d := range detectors {
		if d == nil {
			continue
		}
		name := d.Name()
		if name == "" {
			return nil, errors.New("findings: a detector has no name")
		}
		if len(name) > 64 {
			return nil, fmt.Errorf("findings: detector name %q is longer than 64 characters", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("findings: two detectors are named %q", name)
		}
		seen[name] = true
		kept = append(kept, d)
	}
	// dig does not promise an order within a value group; running in name
	// order makes logs and tests reproducible.
	sort.Slice(kept, func(i, j int) bool { return kept[i].Name() < kept[j].Name() })
	return &Runner{detectors: kept, repo: repo}, nil
}

// Detectors lists the detector names, in the order they run.
func (r *Runner) Detectors() []string {
	out := make([]string, len(r.detectors))
	for i, d := range r.detectors {
		out[i] = d.Name()
	}
	return out
}

// Run runs every detector for the scope and reconciles what they report.
//
// A detector that does not apply is skipped. One that fails is reported in
// the returned error, and its earlier findings are left untouched; the other
// detectors' results are still recorded, and the error makes the task retry,
// which runs everything again — harmless, since recording is idempotent.
func (r *Runner) Run(ctx context.Context, scope Scope) error {
	if scope.KnowledgeBase == nil || scope.Knowledge == nil {
		return errors.New("findings: a run needs a knowledge base and a knowledge entry")
	}
	run := types.FindingRun{
		TenantID: scope.TenantID, KnowledgeBaseID: scope.KnowledgeBaseID(), KnowledgeID: scope.KnowledgeID(),
	}
	byFingerprint := map[string]*types.KnowledgeFinding{}
	var failures []error
	for _, d := range r.detectors {
		candidates, err := d.Detect(ctx, scope)
		if errors.Is(err, ErrUnsupported) {
			logger.Debugf(ctx, "[Findings] detector %s does not apply to knowledge %s: %v",
				d.Name(), run.KnowledgeID, err)
			continue
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("detector %s: %w", d.Name(), err))
			continue
		}
		run.Detectors = append(run.Detectors, d.Name())
		for _, c := range candidates {
			f, err := r.toFinding(run.KnowledgeBaseID, d.Name(), c)
			if err != nil {
				failures = append(failures, fmt.Errorf("detector %s: %w", d.Name(), err))
				continue
			}
			// One problem reported twice keeps its strongest evidence.
			if prev, ok := byFingerprint[f.Fingerprint]; ok && *prev.Score >= *f.Score {
				continue
			}
			byFingerprint[f.Fingerprint] = f
		}
	}
	if len(run.Detectors) > 0 {
		fingerprints := make([]string, 0, len(byFingerprint))
		for fp := range byFingerprint {
			fingerprints = append(fingerprints, fp)
		}
		sort.Strings(fingerprints)
		for _, fp := range fingerprints {
			run.Findings = append(run.Findings, byFingerprint[fp])
		}
		if err := r.repo.Reconcile(ctx, run); err != nil {
			return fmt.Errorf("recording the findings of knowledge %s: %w", run.KnowledgeID, err)
		}
		logger.Infof(ctx, "[Findings] checked knowledge %s with %v: %d finding(s)",
			run.KnowledgeID, run.Detectors, len(run.Findings))
	}
	return errors.Join(failures...)
}

// toFinding validates a candidate and turns it into a row.
func (r *Runner) toFinding(kbID, detector string, c Candidate) (*types.KnowledgeFinding, error) {
	if c.Type == "" || len(c.Type) > 32 {
		return nil, fmt.Errorf("finding type %q must have 1-32 characters", c.Type)
	}
	if !types.ValidFindingSeverity(c.Severity) {
		return nil, fmt.Errorf("finding severity %q is not one of info, warning, error", c.Severity)
	}
	if c.SubjectKnowledgeID == "" || c.SubjectKnowledgeID == c.RelatedKnowledgeID {
		return nil, errors.New("a finding needs a subject distinct from its related entry")
	}
	fingerprint := c.Fingerprint
	if fingerprint == "" {
		fingerprint = PairFingerprint(c.Type, kbID, c.SubjectKnowledgeID, c.RelatedKnowledgeID)
	}
	if len(fingerprint) > 128 {
		return nil, fmt.Errorf("fingerprint of a %s finding is longer than 128 characters", c.Type)
	}
	score := c.Score
	f := &types.KnowledgeFinding{
		KnowledgeBaseID: kbID, Type: c.Type, Detector: detector, Severity: c.Severity,
		Fingerprint: fingerprint, SubjectKnowledgeID: c.SubjectKnowledgeID, Score: &score, Details: c.Details,
	}
	if c.RelatedKnowledgeID != "" {
		related := c.RelatedKnowledgeID
		f.RelatedKnowledgeID = &related
	}
	return f, nil
}

// Supports reports whether any detector applies to the knowledge base. A
// detector that cannot say (it does not implement SupportChecker) is assumed
// to apply.
func (r *Runner) Supports(ctx context.Context, kb *types.KnowledgeBase) bool {
	for _, d := range r.detectors {
		checker, ok := d.(SupportChecker)
		if !ok {
			return true
		}
		supported, err := checker.Supports(ctx, kb)
		if err != nil {
			logger.Warnf(ctx, "[Findings] detector %s could not tell whether it applies to %s: %v",
				d.Name(), kb.ID, err)
			continue
		}
		if supported {
			return true
		}
	}
	return false
}
