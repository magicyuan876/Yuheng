import assert from "node:assert/strict";
import { test } from "vitest";

import { looksLikeMarkdown, markdownToHTML } from "./markdownPaste";

test("structured Markdown is recognised", () => {
  for (const text of [
    "# Heading",
    "## A heading with words",
    "- one\n- two",
    "* one\n* two",
    "1. first\n2. second",
    "> quoted",
    "```js\nconst x = 1\n```",
    "| a | b |\n| --- | --- |\n| 1 | 2 |",
    "---",
    "- [ ] a task",
    "- [x] a finished task",
    "Some text\n\n## Then a heading",
  ]) {
    assert.ok(looksLikeMarkdown(text), JSON.stringify(text));
  }
});

// The direction that matters more: silently restructuring something somebody
// pasted as text is worse than leaving a stray asterisk in place.
test("prose that merely contains punctuation is left as text", () => {
  for (const text of [
    "the range is 3 - 5",
    "a sentence about C# and nothing else",
    "one line of ordinary prose",
    "two lines\nof ordinary prose",
    "an em dash — like this one",
    "a*b*c is arithmetic, not emphasis",
    "",
    "   ",
    "see #1234 for details",
    'i said "hello" > and they said "hi"',
  ]) {
    assert.ok(!looksLikeMarkdown(text), JSON.stringify(text));
  }
});

test("a single line with a link is worth converting", () => {
  assert.ok(looksLikeMarkdown("see [the docs](https://example.test)"));
  assert.ok(looksLikeMarkdown("![a picture](https://example.test/a.png)"));
  assert.ok(looksLikeMarkdown("run `npm test` first"));
  assert.ok(looksLikeMarkdown("this is **important**"));
});

test("a heading becomes a heading", () => {
  const html = markdownToHTML("# Title");
  assert.match(html, /<h1[^>]*>Title<\/h1>/);
});

test("a list becomes a list", () => {
  const html = markdownToHTML("- one\n- two");
  assert.match(html, /<ul>/);
  assert.equal((html.match(/<li>/g) ?? []).length, 2);
});

test("a table becomes a table", () => {
  const html = markdownToHTML("| a | b |\n| --- | --- |\n| 1 | 2 |");
  assert.match(html, /<table>/);
  assert.match(html, /<th>/);
});

test("a fenced block becomes a code block", () => {
  const html = markdownToHTML("```js\nconst x = 1\n```");
  assert.match(html, /<pre>/);
  assert.match(html, /<code/);
});

// Nothing is sanitised here: it goes through the same DOMPurify config and
// the same schema as any other pasted HTML. What matters is that the
// conversion does not swallow the paste.
test("markup a renderer passes through is left for the sanitiser, not dropped", () => {
  const html = markdownToHTML("before\n\n<script>alert(1)</script>\n\nafter");
  assert.match(html, /before/);
  assert.match(html, /after/);
});

test("text that cannot be converted still arrives as text", () => {
  const html = markdownToHTML("plain words");
  assert.match(html, /plain words/);
});
