package container

import (
	"context"

	"github.com/magicyuan876/yuheng/internal/application/service/findings"
	"github.com/magicyuan876/yuheng/internal/config"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// newDuplicateDetector provides the core's duplicate detector into the
// findings detector group, with the similarity threshold the deployment
// configures (YUHENG_FINDINGS_DUPLICATE_MIN_SCORE).
func newDuplicateDetector(
	chunks interfaces.ChunkRepository,
	finders findings.FinderResolver,
	cfg *config.Config,
) findings.Detector {
	minScore := config.DefaultFindingsDuplicateMinScore
	if cfg != nil && cfg.Findings != nil {
		minScore = cfg.Findings.DuplicateMinScore
	}
	return findings.NewDuplicateDetector(chunks, finders, minScore)
}

// newReviewDetector provides the periodic-review detector into the detector
// group.
func newReviewDetector(stewards interfaces.KnowledgeStewardshipRepository) findings.Detector {
	return findings.NewReviewDetector(stewards)
}

// newDisputeDetector provides the detector that turns down-voted answers into
// disputes of the documents they cite.
func newDisputeDetector(
	feedback interfaces.MessageFeedbackRepository,
	stewards interfaces.KnowledgeStewardshipRepository,
) findings.Detector {
	return findings.NewDisputeDetector(feedback, stewards)
}

// newReviewSweep builds the sweep that schedules the review checks nobody's
// action would. It runs only when knowledge health does.
func newReviewSweep(
	repo interfaces.KnowledgeFindingRepository,
	trigger interfaces.KnowledgeFindingsTrigger,
	cfg *config.Config,
) *findings.ReviewSweep {
	return findings.NewReviewSweep(repo, trigger, cfg != nil && cfg.Findings.IsEnabled())
}

// startReviewSweep starts the review sweep and stops it at shutdown.
func startReviewSweep(sweep *findings.ReviewSweep, cleaner interfaces.ResourceCleaner) {
	sweep.Start(context.Background())
	cleaner.RegisterWithName("KnowledgeReviewSweep", func() error {
		sweep.Stop()
		return nil
	})
}
