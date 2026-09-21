import assert from "node:assert/strict";
import { nextTick } from "vue";
import { test } from "vitest";

import { collapsedFrom, isToggleShortcut, useDocsSidebar } from "./useDocsSidebar";

test('only a stored "1" means collapsed', () => {
  assert.equal(collapsedFrom("1"), true);
  for (const raw of ["0", "", "true", "yes", null, undefined]) assert.equal(collapsedFrom(raw), false, String(raw));
});

test("the tree shows by default, and a toggle is remembered in this browser", async () => {
  const { collapsed, toggle } = useDocsSidebar();
  assert.equal(collapsed.value, false);
  toggle();
  await nextTick();
  assert.equal(collapsed.value, true);
  assert.equal(localStorage.getItem("yuheng.docs.sidebarCollapsed"), "1");
  toggle();
  await nextTick();
  assert.equal(localStorage.getItem("yuheng.docs.sidebarCollapsed"), "0");
});

test("the button and the tree share one value", () => {
  const a = useDocsSidebar();
  const b = useDocsSidebar();
  a.toggle();
  assert.equal(b.collapsed.value, a.collapsed.value);
  a.toggle();
});

// Ctrl+\ and ⌘+\, and nothing that merely shares the key.
test("the shortcut is the backslash with Ctrl or ⌘ and nothing else held", () => {
  const key = (over: Partial<KeyboardEvent>) => ({
    key: "\\",
    code: "Backslash",
    metaKey: false,
    ctrlKey: false,
    altKey: false,
    shiftKey: false,
    ...over,
  });
  assert.equal(isToggleShortcut(key({ ctrlKey: true })), true);
  assert.equal(isToggleShortcut(key({ metaKey: true })), true);
  assert.equal(isToggleShortcut(key({})), false, "a bare backslash is typing");
  assert.equal(isToggleShortcut(key({ ctrlKey: true, shiftKey: true })), false);
  assert.equal(isToggleShortcut(key({ ctrlKey: true, altKey: true })), false);
  assert.equal(isToggleShortcut(key({ ctrlKey: true, key: "/", code: "Slash" })), false);
});
