// Whether the space's page tree is showing, and remembering the answer.
//
// The tree is useful for finding a page and in the way once it is found: a
// reader who has settled into a long document wants the width back. So it
// folds away, on a button and on the keyboard, and stays folded until asked —
// in this browser, for this reader, like the page width (usePageWidth).
import { ref, watch } from "vue";

const STORAGE_KEY = "yuheng.docs.sidebarCollapsed";

/** Turns whatever storage holds into an answer; anything but "1" is "showing". */
export function collapsedFrom(raw: string | null | undefined): boolean {
  return raw === "1";
}

function read(): boolean {
  try {
    return collapsedFrom(localStorage.getItem(STORAGE_KEY));
  } catch {
    return false;
  }
}

// One value for the whole application: the toggle in the main column and the
// rule that hides the tree read the same ref, so they cannot disagree.
const collapsed = ref(read());

watch(collapsed, (next) => {
  try {
    localStorage.setItem(STORAGE_KEY, next ? "1" : "0");
  } catch {
    // Remembering is best-effort; see usePageWidth.
  }
});

export function useDocsSidebar() {
  return {
    collapsed,
    toggle: () => {
      collapsed.value = !collapsed.value;
    },
  };
}

/**
 * The keyboard shortcut: Ctrl+\ (⌘+\ on a Mac), which is what Notion, Slack
 * and Feishu use for the same thing. Shift and Alt are excluded so a
 * different chord on the same key is left alone.
 */
export function isToggleShortcut(
  e: Pick<KeyboardEvent, "key" | "code" | "metaKey" | "ctrlKey" | "altKey" | "shiftKey">,
): boolean {
  return (e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && (e.key === "\\" || e.code === "Backslash");
}
