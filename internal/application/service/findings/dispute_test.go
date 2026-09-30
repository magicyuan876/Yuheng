package findings

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/types"
)

type fakeDisputes struct {
	count   int
	reports []types.DisputeReport
	since   time.Time
}

func (f *fakeDisputes) DisputesSince(_ context.Context, _ uint64, _ string, since time.Time, _ int,
) (int, []types.DisputeReport, error) {
	f.since = since
	return f.count, f.reports, nil
}

// Down-votes since the last review make a dispute for the owner, a warning
// from the third; none, or a review since, make nothing — which resolves it.
func TestDisputeDetector(t *testing.T) {
	st := steward(10, 0)
	stewards := fakeStewards{byID: map[string]*types.KnowledgeSteward{"a": st}}
	report := types.DisputeReport{FeedbackID: "fb1", Question: "How many days?", At: time.Now()}

	disputes := &fakeDisputes{count: 1, reports: []types.DisputeReport{report}}
	out, err := NewDisputeDetector(disputes, stewards).Detect(context.Background(), scopeOf("a"))
	require.NoError(t, err)
	require.Len(t, out, 1)
	c := out[0]
	assert.Equal(t, types.FindingTypeDisputed, c.Type)
	assert.Equal(t, types.FindingSeverityInfo, c.Severity)
	assert.Equal(t, AssignSubjectOwner, c.Assign)
	assert.Equal(t, 1, c.Details.Extra["count"])
	assert.True(t, disputes.since.Equal(st.LastVouchedAt()), "counted from the last review")
	first := c.Details.EvidenceHash

	disputes.count = 3
	out, _ = NewDisputeDetector(disputes, stewards).Detect(context.Background(), scopeOf("a"))
	assert.Equal(t, types.FindingSeverityWarning, out[0].Severity)
	assert.NotEqual(t, first, out[0].Details.EvidenceHash, "another down-vote reopens a dismissal")

	disputes.count, disputes.reports = 0, nil
	out, err = NewDisputeDetector(disputes, stewards).Detect(context.Background(), scopeOf("a"))
	require.NoError(t, err)
	assert.Empty(t, out)
}
