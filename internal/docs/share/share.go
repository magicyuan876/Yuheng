// Package share decides whether an anonymous visitor may see a shared page.
//
// Everything here is a pure function over a link's stored fields and the
// current time, so the rules can be read and tested without a database, an
// HTTP request or a clock. The service layer does the I/O; this package
// decides.
//
// The one rule worth stating before any code: a share link is a capability.
// Anybody holding the URL is the audience, so the key has to be unguessable
// and every other check — expiry, revocation, password — has to be applied on
// every request rather than once at the start of a session.
package share

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"strings"
	"time"
	"unicode"
)

// KeyBytes is how much entropy a link key carries.
//
// 16 bytes is 128 bits. The key is the whole secret — there is no account,
// no rate limit that can meaningfully apply to a URL somebody pastes into a
// chat — so it is sized to be beyond guessing rather than to be short.
const KeyBytes = 16

// KeyLength is the length of an encoded key, which is fixed because the
// alphabet below encodes a whole number of bits per character.
const KeyLength = 26

// keyAlphabet is Crockford base32 without the letters that are misread aloud
// or by eye (I, L, O, U). A share key gets read over the phone and typed by
// hand more often than one would like.
const keyAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// ErrBadKey is returned for a key that could not have been issued.
var ErrBadKey = errors.New("share: malformed key")

// NewKey generates an unguessable link key.
func NewKey() (string, error) {
	raw := make([]byte, KeyBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return encode(raw), nil
}

// encode renders bytes in the key alphabet, five bits at a time.
func encode(raw []byte) string {
	out := make([]byte, 0, KeyLength)
	var acc, bits uint32
	for _, b := range raw {
		acc = acc<<8 | uint32(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			out = append(out, keyAlphabet[(acc>>bits)&31])
		}
	}
	if bits > 0 {
		out = append(out, keyAlphabet[(acc<<(5-bits))&31])
	}
	return string(out)
}

// NormaliseKey accepts a key as somebody might have retyped it — lower case,
// with the hyphens a UI may have added for readability — and returns the
// canonical form, or ErrBadKey.
//
// Being generous here is safe: the key is looked up as an exact match after
// this, so a normalisation that lets through something unissued simply fails
// to find a row.
func NormaliseKey(raw string) (string, error) {
	var b strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		if r == '-' || r == ' ' {
			continue
		}
		up := unicode.ToUpper(r)
		if !strings.ContainsRune(keyAlphabet, up) {
			return "", ErrBadKey
		}
		b.WriteRune(up)
	}
	if b.Len() != KeyLength {
		return "", ErrBadKey
	}
	return b.String(), nil
}

// State is what a visitor gets when they follow a link.
type State string

const (
	// StateOK means show the page.
	StateOK State = "ok"
	// StatePassword means the link exists but needs its password first. It is
	// deliberately distinguishable from "gone": hiding the difference would
	// make a correct password indistinguishable from a dead link.
	StatePassword State = "password"
	// StateExpired means the link had an end date and it has passed.
	StateExpired State = "expired"
	// StateRevoked means somebody turned the link off.
	StateRevoked State = "revoked"
	// StateGone means the page behind the link is no longer shareable — in
	// the trash, purged, or restricted since the link was made.
	StateGone State = "gone"
)

// Link is the part of a stored share this package reasons about.
type Link struct {
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
	HasPassword bool
	// PageLive is false once the page is in the trash or purged.
	PageLive bool
	// PageRestricted is true when the page (or an ancestor) cuts permission
	// inheritance. See the note on Evaluate.
	PageRestricted bool
}

// Evaluate decides what to do with a visit.
//
// unlocked says the visitor has already proved the password in this session.
// The order matters and is chosen so that a visitor learns as little as
// possible about a link they cannot use: revoked and expired links look the
// same whether or not they had a password.
func Evaluate(l Link, now time.Time, unlocked bool) State {
	switch {
	case l.RevokedAt != nil:
		return StateRevoked
	case l.ExpiresAt != nil && !now.Before(*l.ExpiresAt):
		return StateExpired
	case !l.PageLive:
		return StateGone
	// A page restricted after the link was made stops being shared, without
	// anybody having to remember to revoke the link. Restricting a page is a
	// statement about who may see it, and a public URL contradicts it; the
	// rule is "restricted is never shared", applied at read time so it cannot
	// be got wrong by a code path that forgets to clean up.
	case l.PageRestricted:
		return StateGone
	case l.HasPassword && !unlocked:
		return StatePassword
	}
	return StateOK
}

// Visible reports whether a state should render the page.
func (s State) Visible() bool { return s == StateOK }

// MaxPasswordAttempts bounds guesses against one link within a window.
//
// A share password is usually a word somebody said out loud in a meeting, so
// it is not strong; the bound is what stands between that and a script.
const MaxPasswordAttempts = 10

// PasswordWindow is the period MaxPasswordAttempts applies over.
const PasswordWindow = 15 * time.Minute

// SamePassword compares in constant time.
//
// The password is stored hashed, so this compares hashes rather than secrets,
// but a timing-independent comparison costs nothing and removes the question.
func SamePassword(a, b []byte) bool { return subtle.ConstantTimeCompare(a, b) == 1 }

// MinPasswordRunes is the shortest password worth calling one.
const MinPasswordRunes = 4

// MaxPasswordRunes bounds what will be hashed. bcrypt truncates past 72
// bytes, so a longer value would silently mean something else.
const MaxPasswordRunes = 64

// CleanPassword trims a proposed password and reports whether it can be used.
// An empty result means "no password on this link", which is a valid choice.
func CleanPassword(raw string) (string, error) {
	pw := strings.TrimSpace(raw)
	if pw == "" {
		return "", nil
	}
	n := len([]rune(pw))
	if n < MinPasswordRunes {
		return "", errors.New("a share password needs at least 4 characters")
	}
	if n > MaxPasswordRunes {
		return "", errors.New("a share password may not exceed 64 characters")
	}
	return pw, nil
}

// MaxLifetime bounds how far ahead a link may be set to expire.
//
// Not a security boundary — somebody can always make a new link — but a link
// that outlives the reason it was made is the common way shared content
// leaks, and an unbounded date field invites exactly that.
const MaxLifetime = 365 * 24 * time.Hour

// CleanExpiry validates a proposed expiry. A nil result means the link does
// not expire.
func CleanExpiry(at *time.Time, now time.Time) (*time.Time, error) {
	if at == nil {
		return nil, nil
	}
	if !at.After(now) {
		return nil, errors.New("a share link cannot expire in the past")
	}
	if at.After(now.Add(MaxLifetime)) {
		return nil, errors.New("a share link may not last more than a year")
	}
	utc := at.UTC()
	return &utc, nil
}

// ViewMilestones are the view counts a link's owner is told about.
//
// Growth, not every visit: a link that reached ten people is worth knowing
// about, and the eleventh visit is not. This is §8.3's fifth notification
// row, which is a threshold rather than an event for that reason.
var ViewMilestones = []int64{10, 50, 100, 500, 1000, 5000, 10000}

// Milestone reports the milestone a view count has just reached, or zero.
//
// It takes the count AFTER the visit and reports only an exact hit, so each
// milestone fires once no matter how many visits arrive.
func Milestone(views int64) int64 {
	for _, m := range ViewMilestones {
		if views == m {
			return m
		}
	}
	return 0
}
