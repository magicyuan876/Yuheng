// Package fracindex implements fractional indexing: order keys that sort
// bytewise and always leave room for a new key between any two existing ones,
// so reordering a sibling rewrites exactly one row.
//
// A key is an integer part followed by an optional fraction, both written in
// base 62 (0-9 < A-Z < a-z, which is also the byte order). The first byte of
// the integer part encodes its length so that keys of different magnitudes
// still compare correctly as plain strings: 'a' introduces a two-byte
// positive integer ("a0" is zero), 'b' a three-byte one, ..., 'z' a
// 27-byte one; 'Z' introduces a two-byte negative integer ("Zz" is minus one)
// down to 'A'. Appending to a list therefore yields short keys ("a0", "a1",
// ...) while inserting between neighbours grows the fraction by one digit.
//
// Fractions never end in '0' so that the ordering has no ties ("a1" and "a10"
// would otherwise denote the same position). The database column that stores
// keys must compare bytewise (Postgres COLLATE "C").
//
// The scheme itself is the widely documented one used by collaborative
// editors; this file is an independent implementation with a jittered
// variant (random low-order digits) so that two clients inserting at the same
// gap at the same time almost never produce the same key.
package fracindex

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
)

const digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

const base = len(digits)

// First is the key produced for the first element of an empty list.
const First = "a0"

// MaxLen is the longest key this package accepts or produces. It matches
// the column width in the docs schema (VARCHAR(64)).
const MaxLen = 64

// JitterDigits is how many random digits Jittered appends.
const JitterDigits = 3

// ErrInvalid reports a malformed key.
var ErrInvalid = errors.New("fracindex: invalid key")

// ErrOrder reports bounds passed in the wrong order.
var ErrOrder = errors.New("fracindex: lower bound must sort before upper bound")

// ErrRange reports that no key exists beyond the given bound.
var ErrRange = errors.New("fracindex: key range exhausted")

// smallestInteger is the one integer value the scheme cannot go below; it is
// reserved so that Between("", x) always has an answer for any valid x.
var smallestInteger = "A" + strings.Repeat("0", 26)

func digitIndex(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 36
	}
	return -1
}

// intLen returns the total length of an integer part introduced by head.
func intLen(head byte) (int, error) {
	switch {
	case head >= 'a' && head <= 'z':
		return int(head-'a') + 2, nil
	case head >= 'A' && head <= 'Z':
		return int('Z'-head) + 2, nil
	}
	return 0, fmt.Errorf("%w: bad integer head %q", ErrInvalid, head)
}

// split separates a key into its integer part and fraction.
func split(key string) (integer, fraction string, err error) {
	if key == "" {
		return "", "", fmt.Errorf("%w: empty", ErrInvalid)
	}
	n, err := intLen(key[0])
	if err != nil {
		return "", "", err
	}
	if len(key) < n {
		return "", "", fmt.Errorf("%w: integer part truncated in %q", ErrInvalid, key)
	}
	return key[:n], key[n:], nil
}

// Validate reports whether key is a well-formed order key.
func Validate(key string) error {
	if len(key) > MaxLen {
		return fmt.Errorf("%w: longer than %d bytes", ErrInvalid, MaxLen)
	}
	integer, fraction, err := split(key)
	if err != nil {
		return err
	}
	for i := 0; i < len(key); i++ {
		if i > 0 && digitIndex(key[i]) < 0 {
			return fmt.Errorf("%w: byte %q is not a base-62 digit", ErrInvalid, key[i])
		}
	}
	if fraction != "" && fraction[len(fraction)-1] == '0' {
		return fmt.Errorf("%w: fraction must not end in 0", ErrInvalid)
	}
	if integer == smallestInteger {
		return fmt.Errorf("%w: smallest integer is reserved", ErrInvalid)
	}
	return nil
}

// increment returns the next integer, or ok=false at the top of the range.
func increment(integer string) (string, bool) {
	head := integer[0]
	body := []byte(integer[1:])
	for i := len(body) - 1; i >= 0; i-- {
		d := digitIndex(body[i])
		if d+1 < base {
			body[i] = digits[d+1]
			return string(head) + string(body), true
		}
		body[i] = '0'
	}
	switch head {
	case 'Z':
		return First, true
	case 'z':
		return "", false
	}
	next := head + 1
	n, _ := intLen(next)
	return string(next) + strings.Repeat("0", n-1), true
}

// decrement returns the previous integer, or ok=false at the bottom.
func decrement(integer string) (string, bool) {
	head := integer[0]
	body := []byte(integer[1:])
	for i := len(body) - 1; i >= 0; i-- {
		d := digitIndex(body[i])
		if d > 0 {
			body[i] = digits[d-1]
			return string(head) + string(body), true
		}
		body[i] = digits[base-1]
	}
	switch head {
	case 'a':
		return "Zz", true
	case 'A':
		return "", false
	}
	prev := head - 1
	n, _ := intLen(prev)
	return string(prev) + strings.Repeat(string(digits[base-1]), n-1), true
}

// digitAt reads position i of a fraction, treating the end as an infinite
// run of zeros.
func digitAt(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return '0'
}

// midpoint returns a fraction strictly between a and b, where b == "" means
// "unbounded above" and a == "" means zero. Both must be free of trailing
// zeros and a must sort before b.
func midpoint(a, b string) string {
	if b != "" {
		n := 0
		for n < len(b) && digitAt(a, n) == b[n] {
			n++
		}
		if n > 0 {
			rest := ""
			if n < len(a) {
				rest = a[n:]
			}
			return b[:n] + midpoint(rest, b[n:])
		}
	}
	da := 0
	if a != "" {
		da = digitIndex(a[0])
	}
	db := base
	if b != "" {
		db = digitIndex(b[0])
	}
	if db-da > 1 {
		mid := (da + db) / 2
		if (da+db)%2 == 1 {
			mid++
		}
		return string(digits[mid])
	}
	// Consecutive digits: the answer starts with a's digit and continues
	// between a's remainder and infinity, unless b's own first digit works.
	if len(b) > 1 {
		return b[:1]
	}
	rest := ""
	if len(a) > 1 {
		rest = a[1:]
	}
	return string(digits[da]) + midpoint(rest, "")
}

// Between returns a key that sorts strictly between a and b. An empty a
// means "before everything", an empty b "after everything"; both empty
// yields First. The result is the shortest key the scheme offers, so a list
// built by repeated appends stays compact.
func Between(a, b string) (string, error) {
	if a != "" {
		if err := Validate(a); err != nil {
			return "", err
		}
	}
	if b != "" {
		if err := Validate(b); err != nil {
			return "", err
		}
	}
	if a != "" && b != "" && a >= b {
		return "", fmt.Errorf("%w: %q >= %q", ErrOrder, a, b)
	}
	switch {
	case a == "" && b == "":
		return First, nil
	case a == "":
		intB, fracB, _ := split(b)
		if fracB != "" {
			return intB, nil
		}
		prev, ok := decrement(intB)
		if !ok || prev == smallestInteger {
			return "", ErrRange
		}
		return prev, nil
	case b == "":
		intA, fracA, _ := split(a)
		if next, ok := increment(intA); ok {
			return next, nil
		}
		return intA + midpoint(fracA, ""), nil
	}
	intA, fracA, _ := split(a)
	intB, fracB, _ := split(b)
	if intA == intB {
		return intA + midpoint(fracA, fracB), nil
	}
	if next, ok := increment(intA); ok && next < b {
		return next, nil
	}
	return intA + midpoint(fracA, ""), nil
}

// Jittered is Between with random low-order digits appended, so concurrent
// writers filling the same gap diverge instead of colliding. The result is
// still strictly between a and b; when the jitter would not fit under b the
// unjittered key is returned.
func Jittered(a, b string) (string, error) {
	key, err := Between(a, b)
	if err != nil {
		return "", err
	}
	if len(key)+JitterDigits > MaxLen {
		return key, nil
	}
	var buf [JitterDigits]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return key, nil
	}
	var sb strings.Builder
	sb.WriteString(key)
	for i, r := range buf {
		if i == len(buf)-1 {
			// The last digit is never '0' so the fraction stays canonical.
			sb.WriteByte(digits[1+int(r)%(base-1)])
			continue
		}
		sb.WriteByte(digits[int(r)%base])
	}
	candidate := sb.String()
	if b != "" && candidate >= b {
		return key, nil
	}
	return candidate, nil
}

// NBetween returns n keys that sort strictly between a and b in ascending
// order, spread so that later insertions have room. It is what a rebalance
// uses to renumber a whole sibling list ("", "", n) with the shortest keys.
func NBetween(a, b string, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}
	if n == 1 {
		k, err := Between(a, b)
		if err != nil {
			return nil, err
		}
		return []string{k}, nil
	}
	if b == "" {
		c, err := Between(a, "")
		if err != nil {
			return nil, err
		}
		rest, err := NBetween(c, "", n-1)
		if err != nil {
			return nil, err
		}
		return append([]string{c}, rest...), nil
	}
	if a == "" {
		c, err := Between("", b)
		if err != nil {
			return nil, err
		}
		rest, err := NBetween("", c, n-1)
		if err != nil {
			return nil, err
		}
		return append(rest, c), nil
	}
	mid := n / 2
	c, err := Between(a, b)
	if err != nil {
		return nil, err
	}
	left, err := NBetween(a, c, mid)
	if err != nil {
		return nil, err
	}
	right, err := NBetween(c, b, n-mid-1)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, n)
	out = append(out, left...)
	out = append(out, c)
	return append(out, right...), nil
}
