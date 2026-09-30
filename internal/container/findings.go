package container

import (
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
