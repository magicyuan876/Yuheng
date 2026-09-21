// The link that carries a block reference between two pages.
//
// Referencing a block is two steps in every tool that has the feature, and
// for a good reason: the block being quoted and the place it will appear are
// usually on different pages, and often in different tabs. So one side copies
// a link and the other pastes it.
//
// The link is a real address on this site rather than a private scheme, so it
// survives being sent to somebody in a chat window and still opens the page.
// It carries the page id rather than the short id, because the paste has to
// resolve it without a round trip — the short id would mean asking the server
// what page that was before the paste could produce anything.
//
// Both directions live here, with no DOM and no clipboard, so the format is
// pinned by tests rather than by two functions that have to be read together.

/** The path a block-reference link uses. */
export const BLOCK_REF_PATH = "/docs/block";

/** One block's address. */
export interface BlockRefLink {
  pageId: string;
  blockId: string;
}

/**
 * Builds the link that is copied to the clipboard.
 *
 * `origin` is passed in rather than read from `location`, so this is testable
 * and so a link copied in one deployment is not silently pinned to another.
 */
export function formatBlockRefLink(ref: BlockRefLink, origin = ""): string {
  if (!ref.pageId || !ref.blockId) return "";
  const path = `${BLOCK_REF_PATH}/${encodeURIComponent(ref.pageId)}/${encodeURIComponent(ref.blockId)}`;
  return origin ? `${origin.replace(/\/+$/, "")}${path}` : path;
}

/**
 * Reads a block-reference link back, or null for anything that is not one.
 *
 * Deliberately strict. Pasting is how text arrives from everywhere, and a
 * loose match here would turn somebody's ordinary link into a quotation of a
 * block they never chose: the path must be exactly this one, both ids must be
 * present, and nothing may follow them.
 */
export function parseBlockRefLink(text: string): BlockRefLink | null {
  const trimmed = text.trim();
  if (trimmed === "" || /\s/.test(trimmed)) return null;

  let path = trimmed;
  if (/^https?:\/\//i.test(trimmed)) {
    try {
      path = new URL(trimmed).pathname;
    } catch {
      return null;
    }
  } else if (!trimmed.startsWith("/")) {
    return null;
  }

  const parts = path.split("/").filter((p) => p !== "");
  // ['docs', 'block', pageId, blockId]
  if (parts.length !== 4 || parts[0] !== "docs" || parts[1] !== "block") return null;

  const pageId = safeDecode(parts[2]!);
  const blockId = safeDecode(parts[3]!);
  if (!pageId || !blockId) return null;
  return { pageId, blockId };
}

function safeDecode(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    // A stray percent sign is not an address; treating it as one would mean
    // building a reference out of something nobody meant as a link.
    return "";
  }
}
