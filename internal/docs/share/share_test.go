package share

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAKeyIsTheRightShapeAndNeverRepeats(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		key, err := NewKey()
		require.NoError(t, err)
		assert.Len(t, key, KeyLength)
		assert.False(t, seen[key], "keys must not repeat")
		seen[key] = true
		for _, r := range key {
			assert.True(t, strings.ContainsRune(keyAlphabet, r), "character %q is outside the alphabet", r)
		}
	}
}

// The key gets read over the phone; the letters that are misread by eye or
// by ear are not in it.
func TestTheAlphabetLeavesOutTheAmbiguousLetters(t *testing.T) {
	for _, r := range "ILOU" {
		assert.False(t, strings.ContainsRune(keyAlphabet, r), "%q should not be in the alphabet", r)
	}
}

func TestAKeyIsAcceptedAsSomebodyMightRetypeIt(t *testing.T) {
	key, err := NewKey()
	require.NoError(t, err)

	for _, variant := range []string{
		key,
		strings.ToLower(key),
		" " + key + " ",
		key[:6] + "-" + key[6:],
	} {
		got, err := NormaliseKey(variant)
		require.NoError(t, err, "variant %q", variant)
		assert.Equal(t, key, got)
	}
}

func TestSomethingThatCouldNotHaveBeenIssuedIsRejected(t *testing.T) {
	key, err := NewKey()
	require.NoError(t, err)

	for _, bad := range []string{
		"", "too-short", key + "X", key[:KeyLength-1],
		strings.Repeat("I", KeyLength), // a letter outside the alphabet
		strings.Repeat("!", KeyLength),
	} {
		_, err := NormaliseKey(bad)
		require.ErrorIs(t, err, ErrBadKey, "input %q", bad)
	}
}

func live() Link { return Link{PageLive: true} }

func TestAPlainLinkIsVisible(t *testing.T) {
	assert.Equal(t, StateOK, Evaluate(live(), time.Now(), false))
}

func TestARevokedLinkIsRevokedWhateverElseIsWrong(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	l := live()
	l.RevokedAt = &past
	l.ExpiresAt = &past
	l.HasPassword = true
	assert.Equal(t, StateRevoked, Evaluate(l, now, false))
}

func TestAnExpiredLinkIsExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Second)
	l := live()
	l.ExpiresAt = &past
	assert.Equal(t, StateExpired, Evaluate(l, now, false))

	// The instant it expires counts as expired, not as the last second of
	// validity.
	l.ExpiresAt = &now
	assert.Equal(t, StateExpired, Evaluate(l, now, false))

	future := now.Add(time.Second)
	l.ExpiresAt = &future
	assert.Equal(t, StateOK, Evaluate(l, now, false))
}

// A visitor should not be able to tell a dead link with a password from a
// dead link without one.
func TestADeadLinkDoesNotAskForItsPassword(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	for _, l := range []Link{
		{PageLive: true, HasPassword: true, RevokedAt: &past},
		{PageLive: true, HasPassword: true, ExpiresAt: &past},
		{PageLive: false, HasPassword: true},
	} {
		assert.NotEqual(t, StatePassword, Evaluate(l, now, false))
	}
}

func TestAPasswordIsAskedForUntilItIsGiven(t *testing.T) {
	now := time.Now()
	l := live()
	l.HasPassword = true
	assert.Equal(t, StatePassword, Evaluate(l, now, false))
	assert.Equal(t, StateOK, Evaluate(l, now, true))
}

func TestATrashedPageIsGone(t *testing.T) {
	l := Link{PageLive: false}
	assert.Equal(t, StateGone, Evaluate(l, time.Now(), true))
}

// Restricting a page is a statement about who may see it, and a public URL
// contradicts it. The link stops working without anybody having to remember
// to revoke it.
func TestRestrictingAPageStopsItsLinksWithoutRevokingThem(t *testing.T) {
	l := live()
	l.PageRestricted = true
	assert.Equal(t, StateGone, Evaluate(l, time.Now(), true))
	assert.Nil(t, l.RevokedAt, "the link was not revoked; it simply stopped resolving")
}

func TestOnlyOKRenders(t *testing.T) {
	assert.True(t, StateOK.Visible())
	for _, s := range []State{StatePassword, StateExpired, StateRevoked, StateGone} {
		assert.False(t, s.Visible(), "%s must not render", s)
	}
}

func TestAPasswordIsBounded(t *testing.T) {
	empty, err := CleanPassword("   ")
	require.NoError(t, err)
	assert.Equal(t, "", empty, "no password is a valid choice")

	got, err := CleanPassword("  hunter2  ")
	require.NoError(t, err)
	assert.Equal(t, "hunter2", got)

	_, err = CleanPassword("abc")
	require.Error(t, err)

	// bcrypt truncates past 72 bytes, so a longer value would silently mean
	// something else.
	_, err = CleanPassword(strings.Repeat("a", MaxPasswordRunes+1))
	require.Error(t, err)
}

func TestHashesAreComparedInConstantTime(t *testing.T) {
	assert.True(t, SamePassword([]byte("abc"), []byte("abc")))
	assert.False(t, SamePassword([]byte("abc"), []byte("abd")))
	assert.False(t, SamePassword([]byte("abc"), []byte("ab")))
}

func TestAnExpiryIsBounded(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	none, err := CleanExpiry(nil, now)
	require.NoError(t, err)
	assert.Nil(t, none, "a link may simply not expire")

	soon := now.Add(time.Hour)
	got, err := CleanExpiry(&soon, now)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.True(t, got.Equal(soon))

	past := now.Add(-time.Second)
	_, err = CleanExpiry(&past, now)
	require.Error(t, err)

	tooFar := now.Add(MaxLifetime + time.Hour)
	_, err = CleanExpiry(&tooFar, now)
	require.Error(t, err)
}

// Growth, not every visit: the tenth reader is news and the eleventh is not.
func TestAMilestoneFiresOnceAtEachThreshold(t *testing.T) {
	assert.EqualValues(t, 10, Milestone(10))
	assert.EqualValues(t, 0, Milestone(11))
	assert.EqualValues(t, 0, Milestone(9))
	assert.EqualValues(t, 50, Milestone(50))
	assert.EqualValues(t, 10000, Milestone(10000))
	assert.EqualValues(t, 0, Milestone(10001), "past the last milestone it goes quiet")
	assert.EqualValues(t, 0, Milestone(0))
	assert.EqualValues(t, 0, Milestone(1))
}

func TestEveryMilestoneIsReachable(t *testing.T) {
	for _, m := range ViewMilestones {
		assert.Equal(t, m, Milestone(m))
	}
}
