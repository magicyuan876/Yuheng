package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/acl"
)

// write replaces a page's body with a document containing one paragraph.
func (p *pageEnv) write(t *testing.T, who *acl.Identity, pageID, text string) {
	t.Helper()
	doc := `{"type":"doc","content":[{"type":"paragraph","content":[` +
		`{"type":"text","text":` + quote(text) + `}]}]}`
	_, err := p.svc.Pages.ReplaceContent(ctx(), who, p.decision(t, who, pageID),
		ReplaceInput{Content: json.RawMessage(doc), Reason: "rest"})
	require.NoError(t, err)
}

func quote(s string) string {
	out, _ := json.Marshal(s)
	return string(out)
}

func (p *pageEnv) find(t *testing.T, who *acl.Identity, query string) *SearchResults {
	t.Helper()
	res, err := p.svc.Pages.Search(ctx(), who, query, "", 0)
	require.NoError(t, err)
	return res
}

func titlesOf(res *SearchResults) []string {
	out := make([]string, 0, len(res.Hits))
	for _, hit := range res.Hits {
		out = append(out, hit.Title)
	}
	return out
}

// Chinese throughout, because that is the language a tsvector-based
// implementation would silently fail on.
func TestAPageIsFoundByItsBody(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "存储说明")
	p.write(t, p.alice, page.Page.ID, "每个空间的配额由工作区管理员设置。")

	res := p.find(t, p.alice, "配额")
	require.Len(t, res.Hits, 1)
	assert.Equal(t, "存储说明", res.Hits[0].Title)
	assert.Equal(t, "page", res.Hits[0].Kind)
	assert.Contains(t, res.Hits[0].Excerpt, "配额")
}

func TestAPageIsFoundByItsTitle(t *testing.T) {
	p := newPageEnv(t)
	p.create(t, p.alice, nil, "配额说明")

	res := p.find(t, p.alice, "配额")
	require.Len(t, res.Hits, 1)
	assert.Equal(t, "配额说明", res.Hits[0].Title)
}

func TestAnEmptyQueryFindsNothing(t *testing.T) {
	p := newPageEnv(t)
	p.create(t, p.alice, nil, "配额说明")

	for _, q := range []string{"", "   "} {
		res := p.find(t, p.alice, q)
		assert.Empty(t, res.Hits, "query %q", q)
	}
}

func TestSomethingAbsentFindsNothing(t *testing.T) {
	p := newPageEnv(t)
	p.create(t, p.alice, nil, "存储说明")
	assert.Empty(t, p.find(t, p.alice, "完全不相干的词").Hits)
}

// A title hit outranks a body hit, because somebody searching "配额" wants
// the page called that first.
func TestTitleHitsRankAboveBodyHits(t *testing.T) {
	p := newPageEnv(t)
	body := p.create(t, p.alice, nil, "无关标题")
	p.write(t, p.alice, body.Page.ID, "这里顺便提到了配额。")
	p.create(t, p.alice, nil, "配额")

	res := p.find(t, p.alice, "配额")
	require.Len(t, res.Hits, 2)
	assert.Equal(t, "配额", res.Hits[0].Title)
}

// The rule the whole module turns on: a title is information.
func TestARestrictedPageIsNotInSomebodyElsesResults(t *testing.T) {
	p := newPageEnv(t)
	secret := p.create(t, p.alice, nil, "Q3 裁员配额")
	p.write(t, p.alice, secret.Page.ID, "机密的配额数字。")
	p.cut(t, p.alice, secret.Page.ID)

	assert.NotEmpty(t, p.find(t, p.alice, "配额").Hits, "alice can still find it")

	res := p.find(t, p.carol, "配额")
	for _, hit := range res.Hits {
		assert.NotEqual(t, secret.Page.ID, hit.PageID)
		assert.NotContains(t, hit.Title, "裁员")
	}
}

func TestATrashedPageIsNotFound(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "配额说明")
	_, err := p.svc.Pages.Delete(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID))
	require.NoError(t, err)

	assert.Empty(t, p.find(t, p.alice, "配额").Hits)
}

// Another space's pages are not somebody's to search.
func TestAPageInAnInvisibleSpaceIsNotFound(t *testing.T) {
	p := newPageEnv(t)
	other, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Elsewhere"})
	require.NoError(t, err)
	_, err = p.svc.Pages.Create(ctx(), p.alice, CreatePageInput{
		SpaceID: other.Space.ID, Title: "别处的配额",
	})
	require.NoError(t, err)

	// viewer is a tenant member with no membership in either space.
	assert.Empty(t, p.find(t, p.viewer, "配额").Hits)
}

func TestSearchCanBeNarrowedToOneSpace(t *testing.T) {
	p := newPageEnv(t)
	p.create(t, p.alice, nil, "这里的配额")
	other, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Elsewhere"})
	require.NoError(t, err)
	_, err = p.svc.Pages.Create(ctx(), p.alice, CreatePageInput{
		SpaceID: other.Space.ID, Title: "别处的配额",
	})
	require.NoError(t, err)

	all, err := p.svc.Pages.Search(ctx(), p.alice, "配额", "", 0)
	require.NoError(t, err)
	assert.Len(t, all.Hits, 2)

	here, err := p.svc.Pages.Search(ctx(), p.alice, "配额", p.space.ID, 0)
	require.NoError(t, err)
	require.Len(t, here.Hits, 1)
	assert.Equal(t, "这里的配额", here.Hits[0].Title)
}

// Naming a space you cannot see answers nothing rather than erroring, which
// would confirm it exists.
func TestNarrowingToAnInvisibleSpaceFindsNothing(t *testing.T) {
	p := newPageEnv(t)
	other, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Elsewhere"})
	require.NoError(t, err)

	res, err := p.svc.Pages.Search(ctx(), p.viewer, "配额", other.Space.ID, 0)
	require.NoError(t, err)
	assert.Empty(t, res.Hits)
}

// ---- comments, which the user asked for explicitly -----------------------------

func TestACommentIsFound(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "设计讨论")
	_, err := p.svc.Pages.CreateComment(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		CreateCommentInput{Body: say("我们决定放弃这个配额方案。")})
	require.NoError(t, err)

	res := p.find(t, p.alice, "配额方案")
	require.Len(t, res.Hits, 1)
	assert.Equal(t, "comment", res.Hits[0].Kind)
	assert.Equal(t, page.Page.ID, res.Hits[0].PageID)
	assert.NotEmpty(t, res.Hits[0].CommentID, "so a client can scroll to it")
	assert.Contains(t, res.Hits[0].Excerpt, "配额方案")
}

// A comment is readable exactly when its page is.
func TestACommentOnARestrictedPageIsNotFoundByOthers(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "机密讨论")
	_, err := p.svc.Pages.CreateComment(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		CreateCommentInput{Body: say("内部配额数字")})
	require.NoError(t, err)
	p.cut(t, p.alice, page.Page.ID)

	assert.Empty(t, p.find(t, p.carol, "内部配额").Hits)
	assert.NotEmpty(t, p.find(t, p.alice, "内部配额").Hits)
}

// An edited comment must not be findable by words it no longer contains.
func TestAnEditedCommentIsSearchedByItsNewText(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "讨论")
	d := p.decision(t, p.alice, page.Page.ID)
	made, err := p.svc.Pages.CreateComment(ctx(), p.alice, d, CreateCommentInput{
		Body: say("原来的说法")})
	require.NoError(t, err)

	_, err = p.svc.Pages.UpdateComment(ctx(), p.alice, d, made.ID,
		say("改过的说法"))
	require.NoError(t, err)

	assert.Empty(t, p.find(t, p.alice, "原来的说法").Hits, "the old words are gone")
	assert.NotEmpty(t, p.find(t, p.alice, "改过的说法").Hits)
}

func TestADeletedCommentIsNotFound(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "讨论")
	d := p.decision(t, p.alice, page.Page.ID)
	made, err := p.svc.Pages.CreateComment(ctx(), p.alice, d, CreateCommentInput{
		Body: say("要删掉的配额")})
	require.NoError(t, err)
	require.NoError(t, p.svc.Pages.DeleteComment(ctx(), p.alice, d, made.ID))

	assert.Empty(t, p.find(t, p.alice, "要删掉的配额").Hits)
}

// Matching the JSON rather than the text would hit structure keywords.
func TestSearchDoesNotMatchCommentStructure(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "讨论")
	_, err := p.svc.Pages.CreateComment(ctx(), p.alice, p.decision(t, p.alice, page.Page.ID),
		CreateCommentInput{Body: say("普通的一句话")})
	require.NoError(t, err)

	assert.Empty(t, p.find(t, p.alice, "paragraph").Hits)
}

// ---- text shown by reference: the debt T2.5 recorded ---------------------------

// setUpReference makes page A show a block of page B and returns both.
func (p *pageEnv) setUpReference(t *testing.T) (referring, source *PageView) {
	t.Helper()
	source = p.create(t, p.alice, nil, "源页面")
	referring = p.create(t, p.alice, nil, "引用页面")

	// The source owns the block; saving it stores the snapshot.
	sourceDoc := `{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"block-quota-1"},` +
		`"content":[{"type":"text","text":"这一段写的是配额上限。"}]}]}`
	referringDoc := `{"type":"doc","content":[` +
		`{"type":"transclusion","attrs":{"sourcePageId":"` + source.Page.ID +
		`","sourceBlockId":"block-quota-1"}}]}`

	// The referring page first, so the reference is registered before the
	// source's save looks for who wants its blocks.
	_, err := p.svc.Pages.ReplaceContent(ctx(), p.alice,
		p.decision(t, p.alice, referring.Page.ID),
		ReplaceInput{Content: json.RawMessage(referringDoc), Reason: "rest"})
	require.NoError(t, err)

	_, err = p.svc.Pages.ReplaceContent(ctx(), p.alice,
		p.decision(t, p.alice, source.Page.ID),
		ReplaceInput{Content: json.RawMessage(sourceDoc), Reason: "rest"})
	require.NoError(t, err)
	return referring, source
}

// A page that transcludes a block reads, to its reader, as though that text
// is part of it, so searching for the text should find that page.
func TestReferencedTextFindsTheReferringPage(t *testing.T) {
	p := newPageEnv(t)
	referring, source := p.setUpReference(t)

	res := p.find(t, p.alice, "配额上限")
	var found *SearchHit
	for _, hit := range res.Hits {
		if hit.Kind == "transclusion" {
			found = hit
		}
	}
	require.NotNil(t, found, "the referring page is a result")
	assert.Equal(t, referring.Page.ID, found.PageID)
	assert.Equal(t, source.Page.ID, found.SourcePageID)
}

// The check T2.5 deferred to here: the SOURCE page's permissions decide
// whether this text may be shown, or search becomes a way to read a
// restricted page through a page that quotes it.
func TestReferencedTextIsHiddenWhenTheSourceIsRestricted(t *testing.T) {
	p := newPageEnv(t)
	referring, source := p.setUpReference(t)
	p.cut(t, p.alice, source.Page.ID)

	res := p.find(t, p.carol, "配额上限")
	for _, hit := range res.Hits {
		assert.NotEqual(t, referring.Page.ID, hit.PageID,
			"carol may read the referring page but not the text it quotes")
	}
	// And alice, who may read the source, still finds it.
	assert.NotEmpty(t, p.find(t, p.alice, "配额上限").Hits)
}

// ---- shaping the result list ----------------------------------------------------

// A page whose title and body both match is one result, not two.
func TestAPageMatchingTwiceIsOneResult(t *testing.T) {
	p := newPageEnv(t)
	page := p.create(t, p.alice, nil, "配额说明")
	p.write(t, p.alice, page.Page.ID, "这里又一次提到配额。")

	res := p.find(t, p.alice, "配额")
	assert.Len(t, res.Hits, 1)
}

func TestTheResultListIsBoundedAndSaysSo(t *testing.T) {
	p := newPageEnv(t)
	for i := 0; i < 5; i++ {
		p.create(t, p.alice, nil, "配额说明")
	}

	res, err := p.svc.Pages.Search(ctx(), p.alice, "配额", "", 2)
	require.NoError(t, err)
	assert.Len(t, res.Hits, 2)
	assert.True(t, res.Truncated)
}

// A query containing % must not become a wildcard that matches everything.
func TestAWildcardInAQueryIsLiteral(t *testing.T) {
	p := newPageEnv(t)
	p.create(t, p.alice, nil, "普通页面")
	p.create(t, p.alice, nil, "完成度 100% 的页面")

	res := p.find(t, p.alice, "100%")
	require.Len(t, res.Hits, 1)
	assert.Contains(t, res.Hits[0].Title, "100%")

	// A bare "%" is a search for a per-cent sign, not for everything: it
	// finds the one page that contains one.
	bare := p.find(t, p.alice, "%")
	require.Len(t, bare.Hits, 1)
	assert.Contains(t, bare.Hits[0].Title, "100%")
}

func TestTheQueryIsEchoedBack(t *testing.T) {
	p := newPageEnv(t)
	res := p.find(t, p.alice, "  配额  ")
	assert.Equal(t, "配额", res.Query)
}

func TestTitlesAreWhatTheyLookLike(t *testing.T) {
	p := newPageEnv(t)
	p.create(t, p.alice, nil, "配额说明")
	assert.Equal(t, []string{"配额说明"}, titlesOf(p.find(t, p.alice, "配额")))
}

// A result from another space must carry that space's slug, or the client
// builds a URL into the wrong space.
func TestAHitCarriesItsOwnSpacesSlug(t *testing.T) {
	p := newPageEnv(t)
	other, err := p.svc.Spaces.Create(ctx(), p.alice, CreateSpaceInput{Name: "Elsewhere"})
	require.NoError(t, err)
	_, err = p.svc.Pages.Create(ctx(), p.alice, CreatePageInput{
		SpaceID: other.Space.ID, Title: "别处的配额",
	})
	require.NoError(t, err)
	p.create(t, p.alice, nil, "这里的配额")

	res := p.find(t, p.alice, "配额")
	require.Len(t, res.Hits, 2)
	for _, hit := range res.Hits {
		assert.NotEmpty(t, hit.SpaceSlug, "every hit knows where it lives")
		if hit.Title == "别处的配额" {
			assert.Equal(t, other.Space.Slug, hit.SpaceSlug)
		} else {
			assert.Equal(t, p.space.Slug, hit.SpaceSlug)
		}
	}
}
