package types

import (
	"bufio"
	"embed"
	"strings"
	"sync"
	"unicode/utf8"
)

// The OpenCC "t2s" configuration is two dictionaries applied with maximum
// matching: TSPhrases (word-level, e.g. 一目瞭然→一目了然, 上鍊→上链) and
// TSCharacters (character-level fallback). Both files are copied unmodified
// from the OpenCC project and are Apache-2.0 licensed; see
// internal/types/opencc/README.md. Embedding the tables and doing the longest
// match on a plain map costs about forty kilobytes and a handful of lines, and
// it keeps the GPL-licensed double-array trie that opencc's Go port pulled in
// out of every server binary.
//
//go:embed opencc/TSPhrases.txt opencc/TSCharacters.txt
var openccT2SData embed.FS

type t2sTable struct {
	dict   map[string]string
	maxLen int // longest key, in runes
}

var (
	t2sOnce  sync.Once
	t2sDict  *t2sTable
	t2sFiles = []string{"opencc/TSPhrases.txt", "opencc/TSCharacters.txt"}
)

// loadT2S parses the dictionary files. Each line is "<traditional>\t<simplified>
// [<alternative> ...]"; OpenCC's t2s takes the first candidate, so do we.
// Phrases load first so that a character entry never shadows a phrase.
func loadT2S() *t2sTable {
	t := &t2sTable{dict: make(map[string]string, 4600)}
	for _, name := range t2sFiles {
		f, err := openccT2SData.Open(name)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimRight(sc.Text(), "\r")
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, rest, ok := strings.Cut(line, "\t")
			if !ok || key == "" {
				continue
			}
			value := rest
			if sp := strings.IndexByte(rest, ' '); sp >= 0 {
				value = rest[:sp]
			}
			if value == "" {
				continue
			}
			if _, exists := t.dict[key]; exists {
				continue
			}
			t.dict[key] = value
			if n := utf8.RuneCountInString(key); n > t.maxLen {
				t.maxLen = n
			}
		}
		_ = f.Close()
	}
	return t
}

// toSimplified converts Traditional Chinese to Simplified Chinese using
// OpenCC's t2s dictionaries with maximum matching. Text that is already
// simplified, ASCII, or otherwise absent from the tables passes through.
func toSimplified(s string) string {
	if s == "" {
		return s
	}
	t2sOnce.Do(func() { t2sDict = loadT2S() })
	t := t2sDict
	if t == nil || len(t.dict) == 0 {
		return s
	}

	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(runes); {
		// ASCII never appears in the tables; skip the map probes for it.
		if runes[i] < utf8.RuneSelf {
			b.WriteRune(runes[i])
			i++
			continue
		}
		matched := false
		limit := t.maxLen
		if rem := len(runes) - i; rem < limit {
			limit = rem
		}
		for l := limit; l >= 1; l-- {
			if v, ok := t.dict[string(runes[i:i+l])]; ok {
				b.WriteString(v)
				i += l
				matched = true
				break
			}
		}
		if !matched {
			b.WriteRune(runes[i])
			i++
		}
	}
	return b.String()
}
