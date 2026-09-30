import assert from "node:assert/strict";
import { afterEach, test, vi } from "vitest";

import { SETTINGS_STORAGE_KEY, loadSettings } from "./settingsStorage";

function makeDefaults() {
  return { selectedTags: [] as string[], nested: { items: ["a"] }, webSearchEnabled: false };
}

afterEach(() => {
  localStorage.clear();
  vi.restoreAllMocks();
});

test("loadSettings returns a deep copy of the defaults when nothing is stored", () => {
  const defaults = makeDefaults();
  const loaded = loadSettings(defaults);
  loaded.nested.items.push("b");
  loaded.selectedTags.push("tag-1");
  assert.deepEqual(defaults, makeDefaults());
});

test("loadSettings lays stored fields over the defaults", () => {
  localStorage.setItem(SETTINGS_STORAGE_KEY, JSON.stringify({ selectedTags: ["t1"] }));
  const loaded = loadSettings(makeDefaults());
  assert.deepEqual(loaded.selectedTags, ["t1"]);
  // A field the stored copy lacks takes its default.
  assert.equal(loaded.webSearchEnabled, false);
  assert.deepEqual(loaded.nested, { items: ["a"] });
});

for (const [label, raw] of [
  ["invalid JSON", "{broken"],
  ["null", "null"],
  ["an array", "[1]"],
]) {
  test(`loadSettings resets ${label} to the defaults and removes the stored key`, () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    localStorage.setItem(SETTINGS_STORAGE_KEY, raw);
    const loaded = loadSettings(makeDefaults());
    assert.deepEqual(loaded, makeDefaults());
    assert.equal(localStorage.getItem(SETTINGS_STORAGE_KEY), null);
  });
}
