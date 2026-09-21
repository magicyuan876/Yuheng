import assert from "node:assert/strict";
import { test } from "vitest";

import { getSchema } from "@tiptap/core";
import { Node as PMNode } from "@tiptap/pm/model";

import { countDocument, countWords } from "./wordCount";
import { officialExtensions } from "./extensions";

const schema = getSchema(officialExtensions());

test("Latin runs count as one word each; CJK characters count individually", () => {
  assert.deepEqual(countWords("hello world"), { words: 2, chars: 10 });
  assert.deepEqual(countWords("你好世界"), { words: 4, chars: 4 });
  assert.deepEqual(countWords("hello 你好"), { words: 3, chars: 7 });
});

test("punctuation and symbols count toward characters only", () => {
  const { words, chars } = countWords("a, b! c?");
  assert.equal(words, 3, "a, b, c are three words");
  assert.equal(chars, 6, "a , b ! c ? -- six non-space characters; the two spaces do not count");
});

test("an empty or whitespace-only string counts nothing", () => {
  assert.deepEqual(countWords(""), { words: 0, chars: 0 });
  assert.deepEqual(countWords("   \n\t "), { words: 0, chars: 0 });
});

test("numbers count as words too", () => {
  assert.deepEqual(countWords("room 404"), { words: 2, chars: 7 });
});

test("countDocument walks every text node in a real ProseMirror document", () => {
  const doc = PMNode.fromJSON(schema, {
    type: "doc",
    content: [
      { type: "heading", attrs: { level: 1 }, content: [{ type: "text", text: "Title" }] },
      { type: "paragraph", content: [{ type: "text", text: "hello world" }] },
    ],
  });
  const { words } = countDocument(doc);
  assert.equal(words, 3, "Title + hello + world");
});
