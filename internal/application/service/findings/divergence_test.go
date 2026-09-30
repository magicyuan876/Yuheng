package findings

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// apply replays a script and checks it turns a into b, so the tests below
// exercise the diff itself and not only its classification.
func apply(t *testing.T, a, b []rune, ops []diffOp) {
	t.Helper()
	var out []rune
	ai, bi := 0, 0
	for _, op := range ops {
		require.Equal(t, ai, op.aFrom, "ops are contiguous in a")
		require.Equal(t, bi, op.bFrom, "ops are contiguous in b")
		switch op.kind {
		case opEqual:
			require.Equal(t, string(a[op.aFrom:op.aTo]), string(b[op.bFrom:op.bTo]))
			out = append(out, a[op.aFrom:op.aTo]...)
		case opInsert:
			require.Equal(t, op.aFrom, op.aTo)
			out = append(out, b[op.bFrom:op.bTo]...)
		case opDelete:
			require.Equal(t, op.bFrom, op.bTo)
		}
		ai, bi = op.aTo, op.bTo
	}
	require.Equal(t, len(a), ai)
	require.Equal(t, len(b), bi)
	require.Equal(t, string(b), string(out))
}

func TestDiffRunesProducesAValidShortestScript(t *testing.T) {
	cases := [][2]string{
		{"", ""},
		{"abc", ""},
		{"", "abc"},
		{"abc", "abc"},
		{"abcabba", "cbabac"},
		{"员工每年享有15天带薪年假", "员工每年享有10天带薪年假"},
		{"kitten", "sitting"},
	}
	for _, c := range cases {
		a, b := []rune(c[0]), []rune(c[1])
		ops, ok := diffRunes(a, b, 100)
		require.True(t, ok, "%q -> %q", c[0], c[1])
		apply(t, a, b, ops)
	}
	// Myers' classic example needs five edits.
	ops, _ := diffRunes([]rune("abcabba"), []rune("cbabac"), 100)
	edits := 0
	for _, op := range ops {
		if op.kind != opEqual {
			edits += (op.aTo - op.aFrom) + (op.bTo - op.bFrom)
		}
	}
	assert.Equal(t, 5, edits)

	_, ok := diffRunes([]rune(strings.Repeat("a", 50)), []rune(strings.Repeat("b", 50)), 20)
	assert.False(t, ok, "a script longer than the bound is not computed")
}

const leave = "员工每年享有15天带薪年假，需提前两周在OA系统提交申请，经直属主管审批后生效。"

func TestComparePassages(t *testing.T) {
	t.Run("a copy does not differ", func(t *testing.T) {
		assert.False(t, comparePassages(leave, leave).Differs)
	})
	t.Run("nor does layout", func(t *testing.T) {
		assert.False(t, comparePassages("Leave:\n  fifteen days a year.", "Leave: fifteen   days a year.").Differs)
	})
	t.Run("a changed number differs, and says where", func(t *testing.T) {
		changed := strings.Replace(leave, "15", "10", 1)
		c := comparePassages(leave, changed)
		require.True(t, c.Differs)
		assert.Equal(t, "5", string([]rune(leave)[c.SubjectAt]))
		assert.Equal(t, "0", string([]rune(changed)[c.RelatedAt]))
	})
	t.Run("a change at the very end, where both passages end", func(t *testing.T) {
		assert.True(t, comparePassages(leave, strings.Replace(leave, "直属主管", "人力资源部", 1)).Differs)
	})
	t.Run("passages cut at different places do not differ", func(t *testing.T) {
		before := "差旅报销需在出差结束后三十天内提交发票原件。"
		after := "加班需要提前在系统中登记，并由部门负责人确认。"
		assert.False(t, comparePassages(before+leave, leave+after).Differs, "one starts earlier, the other ends later")
		assert.False(t, comparePassages("上一节讲的是考勤制度。"+leave, "另一份手册的开头。"+leave).Differs,
			"different text before the shared part")
	})
	t.Run("but a change inside the shared part does", func(t *testing.T) {
		c := comparePassages("上一节讲的是考勤制度。"+leave, "另一份手册的开头。"+strings.Replace(leave, "两周", "一周", 1))
		assert.True(t, c.Differs)
	})
	t.Run("a reworded passage differs", func(t *testing.T) {
		assert.True(t, comparePassages(leave, "年假共15天且带薪。申请流程：至少提前14天在OA里发起，主管批准即可。").Differs)
	})
}

func TestExcerptAroundKeepsTheDifferenceInView(t *testing.T) {
	long := strings.Repeat("前文。", 100) + "关键数字15天" + strings.Repeat("后文。", 100)
	at := strings.Index(normalizeSpace(long), "15")
	at = utf8.RuneCountInString(normalizeSpace(long)[:at])
	got := excerptAround(long, at)
	assert.Contains(t, got, "关键数字15天")
	assert.True(t, strings.HasPrefix(got, "…"))
	assert.True(t, strings.HasSuffix(got, "…"))
	assert.Equal(t, duplicateExcerptRunes, utf8.RuneCountInString(got))

	assert.Equal(t, "short", excerptAround("short", 3))
	head := excerptAround(long, 0)
	assert.False(t, strings.HasPrefix(head, "…"))
	assert.Equal(t, duplicateExcerptRunes, utf8.RuneCountInString(head))
}
