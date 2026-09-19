package search

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests use Chinese on purpose. The tsvector column this package
// deliberately does not use tokenises `simple`, which turns a Chinese
// sentence into one token and makes search silently return nothing. A test
// suite written only in English would pass against that broken
// implementation, so the cases that matter most are written in the language
// most of this product's documents are.

func TestAQueryIsCleaned(t *testing.T) {
	q := Parse("  quota  ")
	assert.Equal(t, "quota", q.Text)
	assert.False(t, q.Empty())
}

func TestNothingToSearchForIsAnEmptyQuery(t *testing.T) {
	for _, raw := range []string{"", "   ", "\t\n"} {
		assert.True(t, Parse(raw).Empty(), "input %q", raw)
	}
}

// One character is a real search in Chinese, where a character is a word.
func TestASingleChineseCharacterIsAValidQuery(t *testing.T) {
	q := Parse("配")
	require.False(t, q.Empty())
	assert.Equal(t, "配", q.Text)
}

func TestAnOverlongQueryIsTrimmedRatherThanRefused(t *testing.T) {
	q := Parse(strings.Repeat("配额", 200))
	require.False(t, q.Empty())
	assert.LessOrEqual(t, len([]rune(q.Text)), MaxQueryRunes)
}

// A Chinese phrase has no spaces and must stay one term: splitting it per
// character would make every document match every query.
func TestAChinesePhraseIsOneTerm(t *testing.T) {
	q := Parse("空间配额")
	require.Len(t, q.Terms, 1)
	assert.Equal(t, "空间配额", q.Terms[0])
}

func TestSpacesSeparateTerms(t *testing.T) {
	q := Parse("quota  limit")
	assert.Equal(t, []string{"quota", "limit"}, q.Terms)
}

// SQLite has no LIKE escape character unless one is declared, so a query
// containing % must not become a wildcard.
func TestLikeWildcardsInAQueryAreEscaped(t *testing.T) {
	assert.Equal(t, `100\%`, EscapeLike("100%"))
	assert.Equal(t, `a\_b`, EscapeLike("a_b"))
	assert.Equal(t, `c:\\path`, EscapeLike(`c:\path`))
	assert.Equal(t, "配额", EscapeLike("配额"), "ordinary text is untouched")
}

func TestTheParsedPatternIsEscaped(t *testing.T) {
	q := Parse("50% off")
	assert.Contains(t, q.Pattern, `\%`)
}

// ---- scoring --------------------------------------------------------------------

func TestAChineseTermIsFoundInAChineseSentence(t *testing.T) {
	q := Parse("配额")
	// No spaces anywhere: this is the case a tsvector index would miss.
	score := Score(q, KindPage, "存储说明", "每个空间的配额由工作区管理员设置。")
	assert.Greater(t, score, 0.0, "a substring match must find it")
}

func TestSomethingAbsentScoresZero(t *testing.T) {
	q := Parse("配额")
	assert.Equal(t, 0.0, Score(q, KindPage, "存储说明", "这里讲的是别的事情。"))
}

// Somebody searching "quota" wants the page called Quota before the twelve
// pages that mention one.
func TestATitleHitOutweighsABodyHit(t *testing.T) {
	q := Parse("配额")
	title := Score(q, KindPage, "配额", "无关正文")
	body := Score(q, KindPage, "无关标题", "这里提到了配额")
	assert.Greater(t, title, body)
}

func TestAPageOutweighsACommentAndACommentATransclusion(t *testing.T) {
	q := Parse("配额")
	page := Score(q, KindPage, "无关", "配额")
	comment := Score(q, KindComment, "无关", "配额")
	transclusion := Score(q, KindTransclusion, "无关", "配额")
	assert.Greater(t, page, comment)
	assert.Greater(t, comment, transclusion)
}

func TestMatchingEveryTermBeatsMatchingOne(t *testing.T) {
	q := Parse("quota limit")
	both := Score(q, KindPage, "notes", "the quota limit applies")
	one := Score(q, KindPage, "notes", "the quota applies")
	assert.Greater(t, both, one)
}

func TestAnExactPhraseBeatsScatteredTerms(t *testing.T) {
	q := Parse("storage quota")
	exact := Score(q, KindPage, "notes", "the storage quota applies")
	scattered := Score(q, KindPage, "notes", "storage is fine and the quota is separate")
	assert.Greater(t, exact, scattered)
}

func TestMatchingIsCaseInsensitiveForLatin(t *testing.T) {
	q := Parse("Quota")
	assert.Greater(t, Score(q, KindPage, "notes", "the QUOTA applies"), 0.0)
}

func TestAnEmptyQueryScoresNothing(t *testing.T) {
	assert.Equal(t, 0.0, Score(Parse(""), KindPage, "配额", "配额"))
}

// sortable is a stand-in for the service layer's result type.
type sortable struct {
	id, title string
	score     float64
}

func (s sortable) SortKey() (float64, string, string) { return s.score, s.title, s.id }

// A result list that reorders between two identical searches shifts under
// the reader's cursor.
func TestSortingIsStableAndDeterministic(t *testing.T) {
	hits := []sortable{
		{id: "c", title: "Beta", score: 10},
		{id: "a", title: "Alpha", score: 10},
		{id: "b", title: "Gamma", score: 50},
	}
	Sort(hits)
	assert.Equal(t, "Gamma", hits[0].title, "score first")
	assert.Equal(t, "Alpha", hits[1].title, "then title")
	assert.Equal(t, "Beta", hits[2].title)
}

// Two untitled pages with equal scores must not swap places between
// requests, which is what the id tie-break is for.
func TestEqualScoresAndTitlesFallBackToTheId(t *testing.T) {
	hits := []sortable{
		{id: "zebra", title: "", score: 10},
		{id: "apple", title: "", score: 10},
	}
	Sort(hits)
	assert.Equal(t, "apple", hits[0].id)
	assert.Equal(t, "zebra", hits[1].id)
}

// ---- excerpts -------------------------------------------------------------------

func TestAShortBodyIsItsOwnExcerpt(t *testing.T) {
	assert.Equal(t, "短正文", Excerpt(Parse("正文"), "短正文"))
}

// The classic way this is got wrong: a byte window cuts a Chinese character
// in half and renders as a replacement glyph.
func TestAnExcerptNeverSplitsACharacter(t *testing.T) {
	body := strings.Repeat("配额说明", 200)
	out := Excerpt(Parse("配额"), body)
	assert.NotContains(t, out, "\uFFFD", "no replacement characters")
	assert.True(t, len([]rune(out)) <= ExcerptRunes+2, "and it stays within the window")
}

func TestAnExcerptIsCentredOnTheMatch(t *testing.T) {
	body := strings.Repeat("前", 300) + "配额在这里" + strings.Repeat("后", 300)
	out := Excerpt(Parse("配额"), body)
	assert.Contains(t, out, "配额", "the reason it matched is visible")
	assert.True(t, strings.HasPrefix(out, "…"), "and the cut is marked")
	assert.True(t, strings.HasSuffix(out, "…"))
}

func TestAnExcerptOfTextWithNoMatchIsItsBeginning(t *testing.T) {
	body := strings.Repeat("无关内容", 200)
	out := Excerpt(Parse("配额"), body)
	assert.True(t, strings.HasPrefix(out, "无关内容"))
	assert.True(t, strings.HasSuffix(out, "…"))
}

// A document full of newlines should read as a sentence, not a column.
func TestAnExcerptCollapsesWhitespace(t *testing.T) {
	out := Excerpt(Parse("配额"), "标题\n\n\n配额\t\t说明")
	assert.Equal(t, "标题 配额 说明", out)
}

func TestAMatchNearTheEndStillShowsTheMatch(t *testing.T) {
	body := strings.Repeat("前", 400) + "配额"
	out := Excerpt(Parse("配额"), body)
	assert.Contains(t, out, "配额")
	assert.True(t, len([]rune(out)) <= ExcerptRunes+2)
}

func TestAnExcerptOfNothingIsNothing(t *testing.T) {
	assert.Equal(t, "", Excerpt(Parse("配额"), ""))
}
