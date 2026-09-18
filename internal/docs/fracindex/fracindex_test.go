package fracindex

import (
	"math/rand"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBetweenKnownValues(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{"", "", "a0"},
		{"a0", "", "a1"},
		{"", "a0", "Zz"},
		{"a0", "a1", "a0V"},
		{"a0", "a0V", "a0G"},
		{"a0V", "a1", "a0l"},
		{"az", "", "b00"},
		{"", "b00", "az"},
		{"Zz", "", "a0"},
		{"", "Zz", "Zy"},
		{"", "Z0", "Yzz"},
		{"a0", "a0G", "a08"},
		{"a05", "a1", "a0Y"},
		{"a0G", "a0H", "a0GV"},
		{"a1", "a1V", "a1G"},
	}
	for _, c := range cases {
		got, err := Between(c.a, c.b)
		require.NoError(t, err, "Between(%q, %q)", c.a, c.b)
		require.Equal(t, c.want, got, "Between(%q, %q)", c.a, c.b)
		require.NoError(t, Validate(got))
		if c.a != "" {
			require.Greater(t, got, c.a)
		}
		if c.b != "" {
			require.Less(t, got, c.b)
		}
	}
}

func TestAppendKeepsKeysShort(t *testing.T) {
	prev := ""
	for i := 0; i < 5000; i++ {
		next, err := Between(prev, "")
		require.NoError(t, err)
		require.Greater(t, next, prev)
		require.LessOrEqual(t, len(next), 4, "append should not grow fractions: %q", next)
		prev = next
	}
	require.Equal(t, "c0Hd", prev, "key 4999: a0..az cover 62, b00..bzz 3844, so 1093 into the c range")
}

func TestPrependKeepsKeysShort(t *testing.T) {
	next := ""
	for i := 0; i < 5000; i++ {
		prev, err := Between("", next)
		require.NoError(t, err)
		if next != "" {
			require.Less(t, prev, next)
		}
		require.LessOrEqual(t, len(prev), 4)
		next = prev
	}
}

func TestRandomInsertionsStayOrderedAndUnique(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	keys := []string{}
	for i := 0; i < 3000; i++ {
		idx := rng.Intn(len(keys) + 1)
		var a, b string
		if idx > 0 {
			a = keys[idx-1]
		}
		if idx < len(keys) {
			b = keys[idx]
		}
		k, err := Between(a, b)
		require.NoError(t, err, "between %q and %q", a, b)
		require.NoError(t, Validate(k))
		keys = append(keys[:idx], append([]string{k}, keys[idx:]...)...)
	}
	require.True(t, sort.StringsAreSorted(keys))
	seen := map[string]bool{}
	for _, k := range keys {
		require.False(t, seen[k], "duplicate key %q", k)
		seen[k] = true
	}
}

func TestRepeatedInsertsAtOneGapGrowLinearly(t *testing.T) {
	// Always inserting right after "a0" is the worst case: one digit per
	// insert. The service rebalances long before MaxLen is reached.
	a, b := "a0", "a1"
	for i := 0; i < 40; i++ {
		k, err := Between(a, b)
		require.NoError(t, err)
		require.Less(t, len(k), MaxLen)
		b = k
	}
}

func TestJitteredStaysInBoundsAndDiverges(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	keys := []string{}
	for i := 0; i < 2000; i++ {
		idx := rng.Intn(len(keys) + 1)
		var a, b string
		if idx > 0 {
			a = keys[idx-1]
		}
		if idx < len(keys) {
			b = keys[idx]
		}
		k, err := Jittered(a, b)
		require.NoError(t, err)
		require.NoError(t, Validate(k))
		if a != "" {
			require.Greater(t, k, a)
		}
		if b != "" {
			require.Less(t, k, b)
		}
		keys = append(keys[:idx], append([]string{k}, keys[idx:]...)...)
	}
	require.True(t, sort.StringsAreSorted(keys))

	// Concurrent appenders at the same gap produce (almost always) distinct
	// keys; with three base-62 digits fifty draws share a key with
	// probability well under one percent, so demand at least forty distinct.
	distinct := map[string]bool{}
	for i := 0; i < 50; i++ {
		k, err := Jittered("a0", "")
		require.NoError(t, err)
		require.Greater(t, k, "a0")
		distinct[k] = true
	}
	require.GreaterOrEqual(t, len(distinct), 40)

	// When the jitter cannot fit under the upper bound the plain key is used.
	k, err := Jittered("a0", "a0V")
	require.NoError(t, err)
	require.Less(t, k, "a0V")
}

func TestNBetween(t *testing.T) {
	keys, err := NBetween("", "", 5)
	require.NoError(t, err)
	require.Equal(t, []string{"a0", "a1", "a2", "a3", "a4"}, keys)

	keys, err = NBetween("a0", "a1", 7)
	require.NoError(t, err)
	require.Len(t, keys, 7)
	require.True(t, sort.StringsAreSorted(keys))
	for _, k := range keys {
		require.Greater(t, k, "a0")
		require.Less(t, k, "a1")
		require.NoError(t, Validate(k))
	}

	keys, err = NBetween("", "a0", 3)
	require.NoError(t, err)
	require.Equal(t, []string{"Zx", "Zy", "Zz"}, keys)

	keys, err = NBetween("", "", 0)
	require.NoError(t, err)
	require.Nil(t, keys)
}

func TestValidateRejectsMalformedKeys(t *testing.T) {
	bad := []string{
		"",
		"0",                           // digit cannot introduce an integer
		"a",                           // truncated integer
		"a10",                         // fraction ends in 0
		"a1-",                         // not base 62
		"A" + strings.Repeat("0", 26), // reserved smallest
		strings.Repeat("a", MaxLen+1), // too long
	}
	for _, k := range bad {
		require.ErrorIs(t, Validate(k), ErrInvalid, "key %q", k)
	}
	good := []string{"a0", "Zz", "a0V", "b00", "zzzzzzzzzzzzzzzzzzzzzzzzzzz", "A" + strings.Repeat("0", 25) + "1"}
	for _, k := range good {
		require.NoError(t, Validate(k), "key %q", k)
	}
}

func TestBetweenErrors(t *testing.T) {
	_, err := Between("a1", "a0")
	require.ErrorIs(t, err, ErrOrder)
	_, err = Between("a0", "a0")
	require.ErrorIs(t, err, ErrOrder)
	_, err = Between("-a", "")
	require.ErrorIs(t, err, ErrInvalid)
	_, err = Between("", "A"+strings.Repeat("0", 25)+"1")
	require.ErrorIs(t, err, ErrRange)

	// The top of the integer range falls back to growing the fraction.
	top := strings.Repeat("z", 27)
	k, err := Between(top, "")
	require.NoError(t, err)
	require.Equal(t, top+"V", k)
}
