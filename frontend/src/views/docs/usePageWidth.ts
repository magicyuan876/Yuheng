// How wide the document column is, and remembering the choice.
//
// Three settings, after the tools people already know: a reading width that
// keeps lines short, a wide one for tables and diagrams, and the full window.
// The choice is this reader's, kept in this browser. It is not a property of
// the document — that is what Feishu does, and it would need a field on the
// page and a round trip to change; a reader deciding how they want to read
// is the cheaper thing and the more common want. If a per-document setting
// is wanted later, it layers on top of this as a default.
import { computed, ref, watch } from "vue";

export type PageWidth = "standard" | "wide" | "full";

export const PAGE_WIDTHS: readonly PageWidth[] = ["standard", "wide", "full"];

const STORAGE_KEY = "yuheng.docs.pageWidth";
const DEFAULT_WIDTH: PageWidth = "standard";

/**
 * The utility class for each width. Written out in full, not built from the
 * name: Tailwind finds classes by scanning source for their literal text, so
 * `max-w-[${n}px]` assembled at runtime would never be generated.
 */
const WIDTH_CLASS: Record<PageWidth, string> = {
  standard: "max-w-[820px]",
  wide: "max-w-[1120px]",
  full: "max-w-none",
};

/** Turns whatever storage holds into a width, or the default if it is not one. */
export function widthFrom(raw: string | null | undefined): PageWidth {
  return (PAGE_WIDTHS as readonly string[]).includes(raw ?? "") ? (raw as PageWidth) : DEFAULT_WIDTH;
}

function read(): PageWidth {
  try {
    return widthFrom(localStorage.getItem(STORAGE_KEY));
  } catch {
    // A locked-down browser with no storage still gets a page; it just does
    // not remember the choice.
    return DEFAULT_WIDTH;
  }
}

// One value for the whole application, so two views of the same page agree
// and a change in one is seen by the other without either reloading storage.
const width = ref<PageWidth>(read());

watch(width, (next) => {
  try {
    localStorage.setItem(STORAGE_KEY, next);
  } catch {
    // See read(): remembering is best-effort.
  }
});

export function usePageWidth() {
  const widthClass = computed(() => WIDTH_CLASS[width.value]);
  return { width, widthClass, options: PAGE_WIDTHS };
}
