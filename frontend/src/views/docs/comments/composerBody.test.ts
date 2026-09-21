import assert from "node:assert/strict";
import { test } from "vitest";

import { bodyFromText, textOf } from "./composerBody";

/** The text of each paragraph, which is what a body is for. */
function paragraphs(body: ReturnType<typeof bodyFromText>): string[] {
  return body.content.map((block) => {
    const node = block as { content?: { type?: string; text?: string }[] };
    return (node.content ?? []).map((c) => (c.type === "hardBreak" ? "\n" : (c.text ?? ""))).join("");
  });
}

test("one sentence is one paragraph", () => {
  const body = bodyFromText("looks good to me");
  assert.equal(body.type, "doc");
  assert.deepEqual(paragraphs(body), ["looks good to me"]);
});

test("a blank line starts a new paragraph", () => {
  assert.deepEqual(paragraphs(bodyFromText("first\n\nsecond")), ["first", "second"]);
  assert.deepEqual(paragraphs(bodyFromText("first\n\n\n\nsecond")), ["first", "second"]);
});

// A single newline is what somebody means by pressing Shift+Enter: still the
// same remark, on a new line.
test("a single newline is a line break inside one paragraph", () => {
  const body = bodyFromText("first line\nsecond line");
  assert.equal(body.content.length, 1);
  assert.deepEqual(paragraphs(body), ["first line\nsecond line"]);
});

test("nothing typed is an empty body rather than an empty paragraph", () => {
  for (const text of ["", "   ", "\n\n", " \n \n "]) {
    assert.deepEqual(bodyFromText(text).content, [], JSON.stringify(text));
  }
});

// The round trip is the point of this module: editing a comment reads the
// body back into the box and writes it out again, and anything lost there is
// lost from somebody's remark without them touching it.
test("what is written out reads back the same", () => {
  for (const text of [
    "one sentence",
    "first\n\nsecond",
    "a line\nand another",
    "three\n\nseparate\n\nparagraphs",
    "a line\nbreak\n\nthen a paragraph",
    "punctuation: it stays! (all of it) — even this",
    "中文也一样",
  ]) {
    assert.equal(textOf(bodyFromText(text)), text, JSON.stringify(text));
  }
});

test("trailing spaces on a line do not survive to change the text", () => {
  assert.equal(textOf(bodyFromText("a line   \nanother")), "a line\nanother");
});

test("reading back something that is not a body gives nothing", () => {
  for (const body of [null, undefined, {}, { content: "not an array" }, 42, "text"]) {
    assert.equal(textOf(body), "");
  }
});

// A comment written elsewhere may hold things the box cannot represent. The
// node is lost on re-save, but the words are not — far smaller a surprise
// than a comment that quietly sheds half its content.
test("a mention comes back as its label rather than disappearing", () => {
  const body = {
    type: "doc",
    content: [
      {
        type: "paragraph",
        content: [
          { type: "text", text: "ask " },
          { type: "mention", attrs: { userId: "u-1", label: "alice" } },
          { type: "text", text: " about it" },
        ],
      },
    ],
  };
  assert.equal(textOf(body), "ask @alice about it");
});

test("a mention with no label contributes nothing rather than a raw id", () => {
  const body = {
    type: "doc",
    content: [
      {
        type: "paragraph",
        content: [
          { type: "text", text: "ask " },
          { type: "mention", attrs: { userId: "11111111-1111-4111-8111-111111111111" } },
        ],
      },
    ],
  };
  assert.equal(textOf(body), "ask", "an id in the middle of a sentence would be worse");
});

test("text inside a list or a quote is not dropped", () => {
  const body = {
    type: "doc",
    content: [
      {
        type: "bulletList",
        content: [
          {
            type: "listItem",
            content: [{ type: "paragraph", content: [{ type: "text", text: "a point" }] }],
          },
        ],
      },
    ],
  };
  assert.equal(textOf(body), "a point");
});

test("an empty paragraph between two others does not become a stray blank", () => {
  const body = {
    type: "doc",
    content: [
      { type: "paragraph", content: [{ type: "text", text: "first" }] },
      { type: "paragraph" },
      { type: "paragraph", content: [{ type: "text", text: "second" }] },
    ],
  };
  assert.equal(
    textOf(body)
      .split("\n\n")
      .filter((p) => p.trim() !== "").length,
    2,
  );
});

test("a body is always a document, whatever went in", () => {
  for (const text of ["", "anything", "a\n\nb"]) {
    const body = bodyFromText(text);
    assert.equal(body.type, "doc");
    assert.ok(Array.isArray(body.content));
  }
});
