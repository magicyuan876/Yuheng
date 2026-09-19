package draft

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnInstructionIsTrimmedAndBounded(t *testing.T) {
	got, err := CleanInstruction("  写一份配额说明  ")
	require.NoError(t, err)
	assert.Equal(t, "写一份配额说明", got)

	_, err = CleanInstruction("   ")
	require.Error(t, err)

	_, err = CleanInstruction(strings.Repeat("字", MaxInstructionRunes+1))
	require.Error(t, err)
}

func TestThePromptCarriesTheRequestAndTheSources(t *testing.T) {
	system, user := BuildPrompt(Request{
		Instruction: "总结配额规则",
		PageTitle:   "存储配额",
		Sources: []Source{
			{ID: "k1", Title: "配额文档", Text: "每个空间有独立配额。"},
			{ID: "k2", Title: "计费文档", Text: "超出配额会被拒绝上传。"},
		},
	})
	assert.Contains(t, system, "Markdown")
	assert.Contains(t, user, "总结配额规则")
	assert.Contains(t, user, "存储配额")
	assert.Contains(t, user, "每个空间有独立配额")
	assert.Contains(t, user, "超出配额会被拒绝上传")
	assert.Contains(t, user, "配额文档", "sources are titled so it can attribute them")
}

// Inventing facts is the failure worth preventing; being told there are no
// sources is better than being left to write from memory.
func TestNoSourcesIsSaidOutLoud(t *testing.T) {
	_, user := BuildPrompt(Request{Instruction: "写点什么"})
	assert.Contains(t, user, "No sources")
}

func TestASourceWithNoTitleStillGetsOne(t *testing.T) {
	_, user := BuildPrompt(Request{
		Instruction: "总结",
		Sources:     []Source{{ID: "k1", Text: "内容"}},
	})
	assert.Contains(t, user, "Source 1")
}

// Sending more than the budget risks a truncation the model handles by
// silently dropping the end, which produces a confident answer missing
// whatever was cut.
func TestTheSourceMaterialIsBounded(t *testing.T) {
	huge := strings.Repeat("字", MaxSourceRunes*2)
	_, user := BuildPrompt(Request{
		Instruction: "总结",
		Sources:     []Source{{ID: "k1", Title: "大文档", Text: huge}},
	})
	assert.Less(t, len([]rune(user)), MaxSourceRunes+2000)
	assert.Contains(t, user, "truncated", "and the cut is admitted")
}

func TestTheBudgetIsSharedAcrossSources(t *testing.T) {
	big := strings.Repeat("甲", MaxSourceRunes)
	_, user := BuildPrompt(Request{
		Instruction: "总结",
		Sources: []Source{
			{ID: "k1", Title: "第一份", Text: big},
			{ID: "k2", Title: "第二份", Text: "第二份的内容"},
		},
	})
	assert.Less(t, len([]rune(user)), MaxSourceRunes+2000)
}

// ---- cleaning what came back ----------------------------------------------------

func TestPlainMarkdownIsLeftAlone(t *testing.T) {
	body := "## 配额\n\n每个空间有独立配额。"
	assert.Equal(t, body, CleanOutput(body))
}

// Models wrap their answer in a fence often enough that not handling it
// would put ``` into a third of all drafts.
func TestAWrappingFenceIsStripped(t *testing.T) {
	assert.Equal(t, "## 配额\n\n说明", CleanOutput("```markdown\n## 配额\n\n说明\n```"))
	assert.Equal(t, "## 配额", CleanOutput("```\n## 配额\n```"))
	assert.Equal(t, "## 配额", CleanOutput("  ```md\n## 配额\n```  "))
}

// A document that legitimately opens with a code block must not have it
// eaten.
func TestADocumentStartingWithACodeBlockKeepsIt(t *testing.T) {
	body := "```go\nfmt.Println(\"hi\")\n```\n\n这是说明文字。"
	assert.Equal(t, body, CleanOutput(body), "the fence does not wrap the whole answer")
}

func TestAFenceOpenerThatIsNotALanguageTagIsNotAFence(t *testing.T) {
	body := "``` not a language tag\ncontent\n```"
	assert.Equal(t, body, CleanOutput(body))
}

func TestEmptyOutputIsEmpty(t *testing.T) {
	assert.Equal(t, "", CleanOutput(""))
	assert.Equal(t, "", CleanOutput("   \n  "))
}

// ---- source ids -----------------------------------------------------------------

func TestSourceIdsAreDistinctAndOrdered(t *testing.T) {
	ids := SourceIDs([]Source{
		{ID: "k2"}, {ID: "k1"}, {ID: "k2"}, {ID: ""}, {ID: "k3"},
	})
	assert.Equal(t, []string{"k2", "k1", "k3"}, ids)
}

func TestNoSourcesIsAnEmptyList(t *testing.T) {
	assert.Empty(t, SourceIDs(nil))
}
