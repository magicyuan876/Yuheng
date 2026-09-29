import assert from "node:assert/strict";
import { test } from "vitest";
import { chatRootClasses, chatScrollBoxClasses } from "./chatLayout";

// Replaces a source-text test ("references drawer smoothly shifts the chat
// area while opening") that regex-matched the chat view's Less. The chat view
// binds these functions' results directly, so this checks what the view gets.

const classesOf = (list: string[]) => list.join(" ").split(/\s+/).filter(Boolean);

test("opening the references panel reserves its width on a wide screen only", () => {
  const closed = classesOf(chatRootClasses({ sidebarCollapsed: false, referencesPanelOpen: false }));
  const open = classesOf(chatRootClasses({ sidebarCollapsed: false, referencesPanelOpen: true }));

  assert.ok(open.includes("has-references-panel"));
  assert.ok(open.includes("min-[960px]:pr-[420px]"), "420px of right padding from 960px up");
  assert.ok(!open.some((c) => /^pr-/.test(c)), "no reserved width on a narrow screen");
  assert.ok(!closed.includes("has-references-panel"));
  assert.ok(!closed.some((c) => c.includes("pr-[420px]")));
});

test("the message scroller drops its top gap while the panel docks the header", () => {
  const closed = classesOf(chatScrollBoxClasses({ referencesPanelOpen: false }));
  const open = classesOf(chatScrollBoxClasses({ referencesPanelOpen: true }));
  assert.ok(closed.includes("pt-2") && !closed.includes("min-[960px]:pt-0"));
  assert.ok(open.includes("min-[960px]:pt-0"));
});

test("the chat width follows the sidebar", () => {
  assert.ok(
    classesOf(chatRootClasses({ sidebarCollapsed: true, referencesPanelOpen: false })).includes(
      "max-w-[calc(100vw-60px)]",
    ),
  );
  assert.ok(
    classesOf(chatRootClasses({ sidebarCollapsed: false, referencesPanelOpen: false })).includes(
      "max-w-[calc(100vw-260px)]",
    ),
  );
});

test("keeps the hook classes the embedded wiki fix drawer styles through", () => {
  assert.ok(classesOf(chatRootClasses({ sidebarCollapsed: false, referencesPanelOpen: false })).includes("chat"));
  assert.ok(classesOf(chatScrollBoxClasses({ referencesPanelOpen: false })).includes("chat_scroll_box"));
});
