import assert from "node:assert/strict";
import { nextTick } from "vue";
import { test } from "vitest";

import { PAGE_WIDTHS, usePageWidth, widthFrom } from "./usePageWidth";

test("anything that is not a width reads as the standard width", () => {
  assert.equal(widthFrom(null), "standard");
  assert.equal(widthFrom(undefined), "standard");
  assert.equal(widthFrom(""), "standard");
  assert.equal(widthFrom("enormous"), "standard");
  for (const w of PAGE_WIDTHS) assert.equal(widthFrom(w), w);
});

test("each width maps to a class Tailwind can find in the source", () => {
  const { width, widthClass } = usePageWidth();
  const seen = new Set<string>();
  for (const w of PAGE_WIDTHS) {
    width.value = w;
    assert.match(widthClass.value, /^max-w-/);
    seen.add(widthClass.value);
  }
  assert.equal(seen.size, PAGE_WIDTHS.length, "three widths, three different classes");
  width.value = "standard";
});

// The choice is the reader's and should still be there tomorrow.
test("the choice is remembered in this browser", async () => {
  const { width } = usePageWidth();
  width.value = "wide";
  await nextTick();
  assert.equal(localStorage.getItem("yuheng.docs.pageWidth"), "wide");
  width.value = "standard";
  await nextTick();
  assert.equal(localStorage.getItem("yuheng.docs.pageWidth"), "standard");
});

test("two callers share one value", () => {
  const a = usePageWidth();
  const b = usePageWidth();
  a.width.value = "full";
  assert.equal(b.width.value, "full");
  assert.equal(b.widthClass.value, a.widthClass.value);
  a.width.value = "standard";
});
