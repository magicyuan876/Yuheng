package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/types"
)

// fakeFindings answers for one knowledge entry.
type fakeFindings struct {
	knowledgeID string
	rows        []*types.KnowledgeFindingView
	asked       []uint64
}

func (f *fakeFindings) OpenForKnowledge(_ context.Context, tenantID uint64, knowledgeID string,
) ([]*types.KnowledgeFindingView, error) {
	f.asked = append(f.asked, tenantID)
	if knowledgeID != f.knowledgeID {
		return nil, nil
	}
	return f.rows, nil
}

func duplicateWith(id, relatedKnowledge string) *types.KnowledgeFindingView {
	return &types.KnowledgeFindingView{
		ID: id, Type: types.FindingTypeDuplicate, Severity: types.FindingSeverityWarning,
		Score: 0.98, OverlapRatio: 0.7, Status: types.FindingStatusOpen,
		Subject: types.KnowledgeRef{KnowledgeID: "k-main"},
		Related: &types.KnowledgeRef{KnowledgeID: relatedKnowledge, Title: "secret title " + relatedKnowledge},
		Evidence: []types.FindingEvidence{{
			SubjectChunkID: "c1", SubjectExcerpt: "ours",
			RelatedChunkID: "c2", RelatedExcerpt: "theirs " + relatedKnowledge,
			Score: 0.98,
		}},
	}
}

// A reader of a page sees a finding only when they may read the page on the
// other side; every other finding — a restricted page, a page of a space they
// are not in, a document that is not a page at all — is only counted, and
// nothing of it (title, excerpt, ID) reaches them.
func TestPageFindingsShowOnlyWhatTheReaderMayRead(t *testing.T) {
	fake := &fakeFindings{knowledgeID: "k-main"}
	p := newPageEnvWith(t, func(d *Deps) { d.Findings = fake })

	main := p.create(t, p.alice, nil, "Main")
	open := p.create(t, p.alice, nil, "Open copy")
	restricted := p.create(t, p.alice, nil, "Restricted copy")
	elsewhereSpace, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Private"})
	require.NoError(t, err)
	elsewhere, err := p.svc.Pages.Create(ctx(), p.alice, CreatePageInput{
		SpaceID: elsewhereSpace.ID, Title: "Elsewhere copy",
	})
	require.NoError(t, err)
	for id, knowledge := range map[string]string{
		main.ID: "k-main", open.ID: "k-open", restricted.ID: "k-restricted",
		elsewhere.ID: "k-elsewhere",
	} {
		require.NoError(t, p.repos.Pages.SetKnowledgeID(ctx(), 1, id, strp(knowledge)))
	}
	p.cut(t, p.alice, restricted.ID)
	fake.rows = []*types.KnowledgeFindingView{
		duplicateWith("f-open", "k-open"),
		duplicateWith("f-restricted", "k-restricted"),
		duplicateWith("f-elsewhere", "k-elsewhere"),
		duplicateWith("f-upload", "k-uploaded-file"),
	}

	carol, err := p.svc.Pages.Findings(ctx(), p.carol, p.decision(t, p.carol, main.ID))
	require.NoError(t, err)
	require.Len(t, carol.Items, 1)
	item := carol.Items[0]
	assert.Equal(t, "f-open", item.ID)
	require.NotNil(t, item.RelatedPage)
	assert.Equal(t, open.ID, item.RelatedPage.ID)
	assert.Equal(t, "Open copy", item.RelatedPage.Title, "the page's own title, not the knowledge entry's")
	assert.Equal(t, p.space.Slug, item.RelatedPage.SpaceSlug)
	assert.NotEmpty(t, item.RelatedPage.ShortID)
	assert.Equal(t, 3, carol.OtherCount)
	for _, it := range carol.Items {
		for _, e := range it.Evidence {
			assert.NotContains(t, e.RelatedExcerpt, "k-restricted")
			assert.NotContains(t, e.RelatedExcerpt, "k-elsewhere")
			assert.NotContains(t, e.RelatedExcerpt, "k-uploaded-file")
		}
	}

	// Alice reads all three pages; the uploaded file is still only counted.
	alice, err := p.svc.Pages.Findings(ctx(), p.alice, p.decision(t, p.alice, main.ID))
	require.NoError(t, err)
	ids := make([]string, 0, len(alice.Items))
	for _, it := range alice.Items {
		ids = append(ids, it.ID)
	}
	assert.ElementsMatch(t, []string{"f-open", "f-restricted", "f-elsewhere"}, ids)
	assert.Equal(t, 1, alice.OtherCount)
	for _, a := range fake.asked {
		assert.Equal(t, uint64(1), a, "findings are read in the page's tenant")
	}

	// A page in the trash is no longer a readable other side.
	_, err = p.svc.Pages.Delete(ctx(), p.alice, p.decision(t, p.alice, open.ID))
	require.NoError(t, err)
	carol, err = p.svc.Pages.Findings(ctx(), p.carol, p.decision(t, p.carol, main.ID))
	require.NoError(t, err)
	assert.Empty(t, carol.Items)
	assert.Equal(t, 4, carol.OtherCount)
}

func TestPageFindingsAreEmptyWithoutAMirror(t *testing.T) {
	fake := &fakeFindings{knowledgeID: "k-main", rows: []*types.KnowledgeFindingView{duplicateWith("f", "k-x")}}
	p := newPageEnvWith(t, func(d *Deps) { d.Findings = fake })
	page := p.create(t, p.alice, nil, "Not indexed")

	view, err := p.svc.Pages.Findings(ctx(), p.alice, p.decision(t, p.alice, page.ID))
	require.NoError(t, err)
	assert.NotNil(t, view.Items, "an empty list, not null")
	assert.Empty(t, view.Items)
	assert.Zero(t, view.OtherCount)
	assert.Empty(t, fake.asked, "a page with no knowledge entry asks nothing")

	// Without the dependency at all, every page has none.
	bare := newPageEnv(t)
	other := bare.create(t, bare.alice, nil, "Page")
	require.NoError(t, bare.repos.Pages.SetKnowledgeID(ctx(), 1, other.ID, strp("k-main")))
	view, err = bare.svc.Pages.Findings(ctx(), bare.alice, bare.decision(t, bare.alice, other.ID))
	require.NoError(t, err)
	assert.Empty(t, view.Items)
}
