import assert from "node:assert/strict";
import { test } from "vitest";

import { EMOJI, matchEmoji, MAX_EMOJI_RESULTS, MIN_EMOJI_QUERY } from "./emoji";

test("every entry is one character, uniquely named, and findable", () => {
  const names = new Set<string>();
  const chars = new Set<string>();
  for (const emoji of EMOJI) {
    assert.ok(emoji.char !== "", emoji.name);
    assert.ok(!names.has(emoji.name), `${emoji.name} appears twice`);
    names.add(emoji.name);
    assert.ok(!chars.has(emoji.char), `${emoji.char} appears twice (${emoji.name})`);
    chars.add(emoji.char);
    assert.ok(emoji.keywords.length > 0, `${emoji.name} would only be findable by its exact name`);
  }
});

test("every entry can be found in Chinese as well as English", () => {
  for (const emoji of EMOJI) {
    const chinese = emoji.keywords.filter((k) => /[一-鿿]/.test(k));
    assert.ok(chinese.length > 0, `${emoji.name} has no Chinese keyword`);
  }
});

// A colon is ordinary punctuation. Opening a menu the moment one is typed
// would put a panel over the text in the middle of an ordinary sentence.
test("a colon on its own opens nothing", () => {
  assert.deepEqual(matchEmoji(""), []);
  assert.deepEqual(matchEmoji("   "), []);
  assert.equal(MIN_EMOJI_QUERY >= 1, true);
});

test("an exact name comes first", () => {
  const found = matchEmoji("check");
  assert.equal(found[0]?.name, "check", found.map((f) => f.name).join(", "));
});

test("a name beats a keyword", () => {
  const names = matchEmoji("bug").map((e) => e.name);
  assert.equal(names[0], "bug", names.join(", "));
});

test("a keyword finds an entry its name would not", () => {
  const found = matchEmoji("done");
  assert.ok(
    found.some((e) => e.name === "check"),
    found.map((f) => f.name).join(", "),
  );
});

test("a Chinese query finds the entry", () => {
  const found = matchEmoji("完成"); // "complete"
  assert.ok(found.length > 0);
  assert.ok(
    found.some((e) => e.char === "✅"),
    found.map((f) => f.char).join(" "),
  );
});

test("matching ignores case", () => {
  assert.deepEqual(
    matchEmoji("ROCKET").map((e) => e.char),
    matchEmoji("rocket").map((e) => e.char),
  );
});

test("a query nothing answers to produces an empty menu", () => {
  assert.deepEqual(matchEmoji("zzzznothing"), []);
});

test("the menu is capped rather than showing the whole list", () => {
  // "e" appears in a great many names and keywords.
  assert.ok(matchEmoji("e").length <= MAX_EMOJI_RESULTS);
  assert.equal(matchEmoji("e", EMOJI, 3).length, 3);
});

test("entries that match equally well keep list order", () => {
  const list = [
    { char: "a", name: "same", keywords: ["x"] },
    { char: "b", name: "same2", keywords: ["x"] },
  ];
  assert.deepEqual(
    matchEmoji("x", list).map((e) => e.char),
    ["a", "b"],
  );
});

test("the set covers the reactions a document actually uses", () => {
  for (const query of ["warning", "idea", "link", "search", "comment", "rocket"]) {
    assert.ok(matchEmoji(query).length > 0, query);
  }
});
