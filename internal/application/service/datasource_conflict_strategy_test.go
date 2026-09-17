package service

import (
	"context"
	"errors"
	"testing"

	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
)

// conflictFakeRepo reports one pre-existing knowledge row for every
// external_id lookup, so ingestItem always takes the "already exists" branch
// where the conflict strategy applies.
type conflictFakeRepo struct {
	interfaces.KnowledgeRepository
	existing     *types.Knowledge
	prefixCalls  []string
	prefixReturn []*types.Knowledge
}

func (r *conflictFakeRepo) FindByDataSourceExternalID(
	_ context.Context, _ uint64, _, _, _ string,
) (*types.Knowledge, error) {
	return r.existing, nil
}

func (r *conflictFakeRepo) HardDeleteKnowledge(context.Context, uint64, string) error { return nil }

func (r *conflictFakeRepo) FindByMetadataKeyPrefix(
	_ context.Context, _ uint64, _, key, prefix string,
) ([]*types.Knowledge, error) {
	r.prefixCalls = append(r.prefixCalls, key+"|"+prefix)
	return r.prefixReturn, nil
}

func conflictTestItem() *types.FetchedItem {
	return &types.FetchedItem{
		ExternalID:      "nt-parent",
		Title:           "Parent Doc",
		Content:         []byte("# hello\n"),
		ContentType:     "text/markdown",
		FileName:        "parent.md",
		ReplacesSubtree: true,
	}
}

// Only the exact value "skip" may change behaviour. Every other value — empty
// (rows predating the column, in-memory structs), an unrecognised string, and
// the explicit "overwrite" — must keep the historical delete-then-recreate
// path, so upgrading cannot silently change any existing deployment.
func TestEffectiveConflictStrategyDefaultsToOverwrite(t *testing.T) {
	cases := map[string]string{
		"":          types.ConflictStrategyOverwrite,
		"overwrite": types.ConflictStrategyOverwrite,
		"OVERWRITE": types.ConflictStrategyOverwrite,
		"Skip":      types.ConflictStrategyOverwrite, // case-sensitive on purpose
		"garbage":   types.ConflictStrategyOverwrite,
		"skip":      types.ConflictStrategySkip,
	}
	for value, want := range cases {
		ds := &types.DataSource{ConflictStrategy: value}
		if got := ds.EffectiveConflictStrategy(); got != want {
			t.Errorf("ConflictStrategy=%q → %q, want %q", value, got, want)
		}
	}
	var nilDS *types.DataSource
	if got := nilDS.EffectiveConflictStrategy(); got != types.ConflictStrategyOverwrite {
		t.Errorf("nil data source → %q, want overwrite", got)
	}
}

// skip: the existing item must be left completely alone — not deleted, not
// re-created, and its subtree not swept — and reported as a skip, not a failure.
func TestIngestItem_SkipKeepsExistingUntouched(t *testing.T) {
	repo := &conflictFakeRepo{
		existing: &types.Knowledge{ID: "existing-knowledge"},
		prefixReturn: []*types.Knowledge{
			childWithExternalID("child-1", "nt-parent#file#1", "ds-1"),
		},
	}
	ks := &sweepFakeKS{repo: repo}
	s := &DataSourceService{knowledgeService: ks}
	ds := &types.DataSource{
		ID: "ds-1", Type: "feishu", TenantID: 7, KnowledgeBaseID: "kb-1",
		ConflictStrategy: types.ConflictStrategySkip,
	}

	isUpdate, err := s.ingestItem(context.Background(), ds, conflictTestItem(), nil)
	if !errors.Is(err, errConflictSkip) {
		t.Fatalf("err = %v, want errConflictSkip", err)
	}
	if isUpdate {
		t.Error("isUpdate = true, want false: nothing was replaced")
	}
	if len(ks.events) != 0 {
		t.Errorf("events = %+v, want none: no delete and no create", ks.events)
	}
	if len(repo.prefixCalls) != 0 {
		t.Errorf("subtree swept (%+v); skip must not delete children either", repo.prefixCalls)
	}
}

// overwrite (and the empty default) keeps the pre-existing behaviour: delete
// the old row, re-create, then sweep stale children.
func TestIngestItem_OverwriteReplacesExisting(t *testing.T) {
	for _, strategy := range []string{types.ConflictStrategyOverwrite, ""} {
		t.Run("strategy="+strategy, func(t *testing.T) {
			repo := &conflictFakeRepo{existing: &types.Knowledge{ID: "existing-knowledge"}}
			ks := &sweepFakeKS{repo: repo}
			s := &DataSourceService{knowledgeService: ks}
			ds := &types.DataSource{
				ID: "ds-1", Type: "feishu", TenantID: 7, KnowledgeBaseID: "kb-1",
				ConflictStrategy: strategy,
			}

			isUpdate, err := s.ingestItem(context.Background(), ds, conflictTestItem(), nil)
			if err != nil {
				t.Fatalf("ingestItem error: %v", err)
			}
			if !isUpdate {
				t.Error("isUpdate = false, want true: the existing row was replaced")
			}
			want := []string{"delete:existing-knowledge", "create:parent.md"}
			if len(ks.events) != len(want) || ks.events[0] != want[0] || ks.events[1] != want[1] {
				t.Fatalf("events = %+v, want %+v", ks.events, want)
			}
		})
	}
}

// A brand-new item is created normally under skip: the setting governs
// conflicts with existing rows, it does not stop new content from arriving.
func TestIngestItem_SkipStillCreatesNewItems(t *testing.T) {
	repo := &conflictFakeRepo{existing: nil} // nothing exists yet
	ks := &sweepFakeKS{repo: repo}
	s := &DataSourceService{knowledgeService: ks}
	ds := &types.DataSource{
		ID: "ds-1", Type: "feishu", TenantID: 7, KnowledgeBaseID: "kb-1",
		ConflictStrategy: types.ConflictStrategySkip,
	}

	if _, err := s.ingestItem(context.Background(), ds, conflictTestItem(), nil); err != nil {
		t.Fatalf("ingestItem error: %v", err)
	}
	if len(ks.events) != 1 || ks.events[0] != "create:parent.md" {
		t.Fatalf("events = %+v, want [create:parent.md]", ks.events)
	}
}

// A conflict skip is counted as Skipped, never as Failed, and is not a
// retryable failure (it must not withhold the connector's cursor advance).
func TestApplyFetchedItem_ConflictSkipCountsAsSkipped(t *testing.T) {
	repo := &conflictFakeRepo{existing: &types.Knowledge{ID: "existing-knowledge"}}
	ks := &sweepFakeKS{repo: repo}
	s := &DataSourceService{knowledgeService: ks}
	ds := &types.DataSource{
		ID: "ds-1", Type: "feishu", TenantID: 7, KnowledgeBaseID: "kb-1",
		ConflictStrategy: types.ConflictStrategySkip,
	}
	result := &types.SyncResult{}

	retryable := s.applyFetchedItem(context.Background(), ds, conflictTestItem(), nil, result)

	if retryable {
		t.Error("retryable = true, want false: a deliberate skip is not a failure")
	}
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}
	if result.Failed != 0 || result.Created != 0 || result.Updated != 0 {
		t.Errorf("counters = failed:%d created:%d updated:%d, want all 0",
			result.Failed, result.Created, result.Updated)
	}
	if len(result.Errors) != 0 {
		t.Errorf("Errors = %+v, want none: a skip is not an error sample", result.Errors)
	}
}

// Source-side deletions are governed by SyncDeletions alone; the conflict
// strategy must not suppress them (skip is about content conflicts, not about
// keeping items the source removed).
func TestApplyFetchedItem_SkipDoesNotBlockDeletions(t *testing.T) {
	repo := &conflictFakeRepo{existing: &types.Knowledge{ID: "existing-knowledge"}}
	ks := &sweepFakeKS{repo: repo}
	s := &DataSourceService{knowledgeService: ks}
	ds := &types.DataSource{
		ID: "ds-1", Type: "feishu", TenantID: 7, KnowledgeBaseID: "kb-1",
		ConflictStrategy: types.ConflictStrategySkip,
		SyncDeletions:    true,
	}
	result := &types.SyncResult{}

	s.applyFetchedItem(context.Background(), ds,
		&types.FetchedItem{ExternalID: "nt-parent", IsDeleted: true}, nil, result)

	if result.Deleted != 1 {
		t.Fatalf("Deleted = %d, want 1: skip must not block source deletions", result.Deleted)
	}
	if len(ks.deleted) != 1 || ks.deleted[0] != "existing-knowledge" {
		t.Fatalf("deleted = %+v, want [existing-knowledge]", ks.deleted)
	}
}
