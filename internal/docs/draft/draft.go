// Package draft turns knowledge-base material into a page body.
//
// Everything here is a pure function over strings: how the model is asked,
// and how its answer is cleaned up before anybody sees it. The service layer
// does the retrieval, the model call and the write.
//
// ---- decision: the model writes Markdown, not the document format
//
// The editor's format is a ProseMirror JSON tree with a strict schema. A
// language model asked to emit it produces something that is nearly valid
// nearly all of the time, and the failures are the expensive kind: a missing
// content array, an attribute of the wrong type, a node that does not exist.
// Markdown is the format these models are best at, it degrades into
// something readable when they get it slightly wrong, and this module
// already owns a converter from it (internal/docs/markdown, T0.4). So the
// prompt asks for Markdown and the converter produces the document.
//
// ---- decision: the draft says where it came from
//
// A page written from retrieved material records the knowledge entries it
// was distilled from, in docs_pages.source_refs. Somebody reading a document
// six months later, wondering whether a claim in it is still true, needs to
// know it was assembled from three particular documents rather than written
// by a colleague who checked. That is not a nicety: a generated page that
// looks hand-written is a page nobody will think to re-verify.
package draft

import (
	"fmt"
	"strings"
)

// MaxInstructionRunes bounds what somebody may ask for.
const MaxInstructionRunes = 2000

// MaxSourceRunes bounds how much material is put in front of the model.
//
// Sized for the context window of a small model, because a deployment may
// have bound a small one: sending more than this risks a truncation the
// model handles by silently dropping the end of the material, which produces
// a confident answer missing whatever was cut.
const MaxSourceRunes = 24000

// MaxSources bounds how many knowledge entries one draft may draw on.
const MaxSources = 20

// Source is one piece of retrieved material.
type Source struct {
	// ID is the knowledge entry, recorded in the page's source_refs.
	ID string
	// Title is shown to the model so it can attribute sections sensibly.
	Title string
	Text  string
}

// Request is a draft to be written.
type Request struct {
	// Instruction is what the person asked for.
	Instruction string
	// PageTitle is the page being written, which anchors the model on the
	// subject even when the instruction is terse.
	PageTitle string
	Sources   []Source
}

// CleanInstruction trims and bounds what was asked.
func CleanInstruction(raw string) (string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", fmt.Errorf("say what the draft should cover")
	}
	if len([]rune(text)) > MaxInstructionRunes {
		return "", fmt.Errorf("an instruction may not exceed %d characters", MaxInstructionRunes)
	}
	return text, nil
}

// systemPrompt is what the model is told it is doing.
//
// Written to constrain rather than to flatter: the failure modes worth
// preventing are inventing facts the sources do not contain, and returning
// prose about the task instead of the document.
const systemPrompt = `You write documentation pages for a team wiki.

Rules:
- Write ONLY the page body in Markdown. No preamble, no explanation of what
  you did, no closing remarks.
- Do not repeat the page title as a heading; the page already has one.
- Use only what the provided sources say. If the sources do not answer part
  of the request, write what they do support and say plainly what is missing
  rather than filling the gap.
- Keep the language of the sources. If they are in Chinese, write in Chinese.
- Prefer headings, short paragraphs and lists over long prose.`

// BuildPrompt assembles the system and user messages.
//
// Returns them separately rather than as one string, because a chat model
// treats a system message differently from a user one, and instructions that
// arrive as user text are ones a later instruction can talk it out of.
func BuildPrompt(req Request) (system, user string) {
	var b strings.Builder

	if title := strings.TrimSpace(req.PageTitle); title != "" {
		fmt.Fprintf(&b, "Page title: %s\n\n", title)
	}
	fmt.Fprintf(&b, "Request: %s\n\n", req.Instruction)

	if len(req.Sources) == 0 {
		b.WriteString("No sources were supplied. Say so instead of writing from memory.\n")
		return systemPrompt, b.String()
	}

	b.WriteString("Sources:\n\n")
	budget := MaxSourceRunes
	for i, source := range req.Sources {
		if budget <= 0 {
			break
		}
		title := strings.TrimSpace(source.Title)
		if title == "" {
			title = fmt.Sprintf("Source %d", i+1)
		}
		text, used := clip(source.Text, budget)
		budget -= used
		fmt.Fprintf(&b, "--- %s ---\n%s\n\n", title, text)
	}
	return systemPrompt, b.String()
}

// clip cuts text to a rune budget, reporting how much it used.
func clip(text string, budget int) (string, int) {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= budget {
		return string(runes), len(runes)
	}
	return string(runes[:budget]) + "\n…(truncated)", budget
}

// CleanOutput turns what the model returned into Markdown fit for the
// converter.
//
// Models wrap their answer in a fenced block often enough that not handling
// it would put ``` into a third of all drafts. The fence is only stripped
// when it wraps the WHOLE answer: a document that legitimately contains a
// code block starts with prose, and cutting its first fence would eat a
// chunk of the page.
func CleanOutput(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}

	if strings.HasPrefix(text, "```") {
		// Find the end of the opening fence line and the closing fence.
		firstNewline := strings.IndexByte(text, '\n')
		if firstNewline > 0 && strings.HasSuffix(text, "```") {
			opener := strings.TrimSpace(text[3:firstNewline])
			// Only a language tag may follow the fence; anything else means
			// this is a code block that happens to start the document.
			if isLanguageTag(opener) {
				inner := text[firstNewline+1 : len(text)-3]
				return strings.TrimSpace(inner)
			}
		}
	}
	return text
}

// isLanguageTag reports whether a fence opener is a bare language name.
func isLanguageTag(s string) bool {
	if s == "" {
		return true
	}
	if len(s) > 20 || strings.ContainsAny(s, " \t") {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && r != '-' && r != '+' {
			return false
		}
	}
	return true
}

// SourceIDs is the distinct ids of the sources used, in order, for the
// page's source_refs.
func SourceIDs(sources []Source) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(sources))
	for _, source := range sources {
		if source.ID == "" || seen[source.ID] {
			continue
		}
		seen[source.ID] = true
		out = append(out, source.ID)
	}
	return out
}
