package comment

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// doc wraps blocks into a document body, the way a client sends one.
func doc(blocks ...string) json.RawMessage {
	return json.RawMessage(`{"type":"doc","content":[` + strings.Join(blocks, ",") + `]}`)
}

func para(text string) string {
	quoted, _ := json.Marshal(text)
	return `{"type":"paragraph","content":[{"type":"text","text":` + string(quoted) + `}]}`
}

func TestAnOrdinaryCommentIsAccepted(t *testing.T) {
	node, text, err := ParseBody(doc(para("looks good to me")))
	require.NoError(t, err)
	require.NotNil(t, node)
	assert.Equal(t, "looks good to me", text)
}

// The text is derived once here because three callers need it and must not
// disagree: notifications, the search index and a reply preview.
func TestThePlainTextComesBackWithTheBody(t *testing.T) {
	_, text, err := ParseBody(doc(para("first line"), para("second line")))
	require.NoError(t, err)
	assert.Contains(t, text, "first line")
	assert.Contains(t, text, "second line")
}

func TestWhatAThreadIsActuallyWrittenWithIsAllowed(t *testing.T) {
	bodies := map[string]json.RawMessage{
		"a list": doc(`{"type":"bulletList","content":[
			{"type":"listItem","content":[` + para("one") + `]},
			{"type":"listItem","content":[` + para("two") + `]}]}`),
		"a quote":   doc(`{"type":"blockquote","content":[` + para("as you said") + `]}`),
		"a snippet": doc(`{"type":"codeBlock","content":[{"type":"text","text":"x := 1"}]}`),
		"a line break": doc(`{"type":"paragraph","content":[
			{"type":"text","text":"a"},{"type":"hardBreak"},{"type":"text","text":"b"}]}`),
		"formatting": doc(`{"type":"paragraph","content":[
			{"type":"text","text":"bold","marks":[{"type":"bold"}]},
			{"type":"text","text":"code","marks":[{"type":"code"}]}]}`),
	}
	for name, body := range bodies {
		_, _, err := ParseBody(body)
		assert.NoError(t, err, name)
	}
}

// A comment is a remark, not a document.
func TestWhatBelongsInAPageIsRefusedInAComment(t *testing.T) {
	bodies := map[string]json.RawMessage{
		"a heading": doc(`{"type":"heading","attrs":{"level":1},"content":[{"type":"text","text":"no"}]}`),
		"an image":  doc(`{"type":"image","attrs":{"src":"https://x.test/a.png"}}`),
		"a diagram": doc(`{"type":"mermaid","attrs":{"source":"graph TD;"}}`),
		"a callout": doc(`{"type":"callout","attrs":{"kind":"info"},"content":[` + para("no") + `]}`),
		"a table": doc(`{"type":"table","content":[{"type":"tableRow","content":[
			{"type":"tableCell","content":[` + para("no") + `]}]}]}`),
	}
	for name, body := range bodies {
		_, _, err := ParseBody(body)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "may not contain", name)
	}
}

// "Invalid comment" tells somebody who pasted a table into the box nothing
// about what to do next.
func TestTheRefusalNamesWhatWasRefused(t *testing.T) {
	_, _, err := ParseBody(doc(`{"type":"image","attrs":{"src":"https://x.test/a.png"}}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image")
}

func TestAnEmptyCommentIsRefused(t *testing.T) {
	for name, body := range map[string]json.RawMessage{
		"nothing at all":     json.RawMessage(``),
		"whitespace":         json.RawMessage(`   `),
		"an empty document":  doc(),
		"an empty paragraph": doc(`{"type":"paragraph"}`),
		"only spaces":        doc(para("   ")),
	} {
		_, _, err := ParseBody(body)
		assert.Error(t, err, name)
	}
}

// A mention on its own is a real comment — it renders as no text but says
// something.
func TestAMentionOnItsOwnIsAComment(t *testing.T) {
	body := doc(`{"type":"paragraph","content":[
		{"type":"mention","attrs":{"userId":"11111111-1111-4111-8111-111111111111"}}]}`)
	_, _, err := ParseBody(body)
	assert.NoError(t, err)
}

func TestACommentIsBounded(t *testing.T) {
	long := doc(para(strings.Repeat("a", MaxBodyRunes+1)))
	_, _, err := ParseBody(long)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "characters")

	blocks := make([]string, 0, MaxBodyNodes+10)
	for i := 0; i < MaxBodyNodes+10; i++ {
		blocks = append(blocks, para(fmt.Sprintf("line %d", i)))
	}
	_, _, err = ParseBody(doc(blocks...))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nodes")
}

// Read from the document rather than the text: a mention is a node carrying a
// user id, not a string that begins with "@".
func TestMentionsAreReadFromTheDocument(t *testing.T) {
	body := doc(`{"type":"paragraph","content":[
		{"type":"text","text":"ask "},
		{"type":"mention","attrs":{"userId":"aaaaaaaa-1111-4111-8111-111111111111"}},
		{"type":"text","text":" and "},
		{"type":"mention","attrs":{"userId":"bbbbbbbb-2222-4222-8222-222222222222"}},
		{"type":"text","text":" — not @carol, who is only mentioned in words"}]}`)

	node, _, err := ParseBody(body)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"aaaaaaaa-1111-4111-8111-111111111111",
		"bbbbbbbb-2222-4222-8222-222222222222",
	}, Mentions(node))
}

func TestTheSamePersonNamedTwiceIsNotifiedOnce(t *testing.T) {
	body := doc(`{"type":"paragraph","content":[
		{"type":"mention","attrs":{"userId":"aaaaaaaa-1111-4111-8111-111111111111"}},
		{"type":"text","text":" and again "},
		{"type":"mention","attrs":{"userId":"aaaaaaaa-1111-4111-8111-111111111111"}}]}`)

	node, _, err := ParseBody(body)
	require.NoError(t, err)
	assert.Equal(t, []string{"aaaaaaaa-1111-4111-8111-111111111111"}, Mentions(node))
}

func TestACommentWithNobodyNamedMentionsNobody(t *testing.T) {
	node, _, err := ParseBody(doc(para("just words")))
	require.NoError(t, err)
	assert.Empty(t, Mentions(node))
	assert.Empty(t, Mentions(nil))
}

func TestAMalformedBodyIsRefused(t *testing.T) {
	for _, raw := range []string{`{`, `[]`, `"text"`, `{"type":"paragraph"}`} {
		_, _, err := ParseBody(json.RawMessage(raw))
		assert.Error(t, err, raw)
	}
}
