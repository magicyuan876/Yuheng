package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/magicyuan876/yuheng/internal/docs/repository"
)

// Slug rules: lowercase ASCII letters, digits and hyphens, starting with a
// letter or digit, 2..64 characters. Slugs appear in URLs and are unique per
// tenant among live spaces.
const (
	MinSlugLen = 2
	MaxSlugLen = 64
)

var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)

// ValidSlug reports whether s satisfies the slug rules.
func ValidSlug(s string) bool { return slugRE.MatchString(s) }

// Slugify derives a slug candidate from a name: ASCII letters and digits are
// kept (lowercased), runs of anything else become one hyphen. Names with no
// ASCII content (for example Chinese titles) yield "", and the caller falls
// back to a generated slug.
func Slugify(name string) string {
	var b strings.Builder
	lastHyphen := true
	for _, r := range strings.ToLower(name) {
		switch {
		case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > MaxSlugLen {
		s = strings.Trim(s[:MaxSlugLen], "-")
	}
	if len(s) < MinSlugLen {
		return ""
	}
	return s
}

// randomSuffix returns n lowercase base-32 characters.
func randomSuffix(n int) string {
	const alphabet = "abcdefghijkmnpqrstuvwxyz23456789"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// Fall back to a time-free deterministic pattern; uniqueness is still
		// enforced by the database.
		for i := range buf {
			buf[i] = alphabet[i%len(alphabet)]
		}
		return string(buf)
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf)
}

// uniqueSlug returns base if free, else base-2, base-3, ... and finally a
// random suffix. exclude names a space whose own slug does not count as
// taken (updates).
func (b *base) uniqueSlug(ctx context.Context, tenantID uint64, base string, exclude string) (string, error) {
	if base == "" {
		base = "space-" + randomSuffix(6)
	}
	candidates := []string{base}
	for i := 2; i <= 9; i++ {
		suffix := fmt.Sprintf("-%d", i)
		stem := base
		if len(stem)+len(suffix) > MaxSlugLen {
			stem = strings.TrimRight(stem[:MaxSlugLen-len(suffix)], "-")
		}
		candidates = append(candidates, stem+suffix)
	}
	for _, c := range candidates {
		taken, err := b.slugTaken(ctx, tenantID, c, exclude)
		if err != nil {
			return "", err
		}
		if !taken {
			return c, nil
		}
	}
	stem := base
	if len(stem)+7 > MaxSlugLen {
		stem = strings.TrimRight(stem[:MaxSlugLen-7], "-")
	}
	return stem + "-" + randomSuffix(6), nil
}

func (b *base) slugTaken(ctx context.Context, tenantID uint64, slug, exclude string) (bool, error) {
	existing, err := b.d.Repos.Spaces.GetBySlug(ctx, tenantID, slug)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return existing.ID != exclude, nil
}
