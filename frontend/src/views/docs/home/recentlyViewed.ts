// What this person looked at lately.
//
// Kept in the browser rather than on the server, deliberately. Recording it
// server-side would mean a write on every page view — the most frequent thing
// that happens in a documentation tool — for something only ever shown back
// to the person who did it. See the note in internal/docs/service/home.go.
//
// The consequence is honest and worth stating in the interface: the list does
// not follow somebody between devices. It is a convenience, not a record.
//
// Storage is injected so the list logic is tested without a browser, and
// because localStorage is not always there to be had: a private window, a
// browser with site data blocked, or a preview frame all throw on access
// rather than returning nothing. Every read and write goes through a guard
// that treats that as an empty list, which is the only behaviour that leaves
// the page working.

/** One page somebody opened. */
export interface Visit {
  pageId: string;
  shortId: string;
  spaceSlug: string;
  title: string;
  icon?: string;
  /** Milliseconds since the epoch. */
  at: number;
}

/** How many visits are kept. Past this, the oldest fall off the end. */
export const MAX_VISITS = 12;

/** Where the list lives. Versioned, so a shape change discards rather than
 * misreads what an older build wrote. */
export const STORAGE_KEY = "yuheng.docs.recentlyViewed.v1";

/** The bit of Storage this needs, so a test can supply a map. */
export interface VisitStore {
  getItem: (key: string) => string | null;
  setItem: (key: string, value: string) => void;
}

/** The browser's store, or null where it cannot be had. */
export function browserStore(): VisitStore | null {
  try {
    const store = globalThis.localStorage;
    // Touched rather than merely checked: a browser with site data blocked
    // has the object and throws on use.
    store.getItem(STORAGE_KEY);
    return store;
  } catch {
    return null;
  }
}

/** Reads the list, newest first. */
export function readVisits(store: VisitStore | null): Visit[] {
  if (!store) return [];
  let raw: string | null = null;
  try {
    raw = store.getItem(STORAGE_KEY);
  } catch {
    return [];
  }
  if (!raw) return [];

  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(isVisit).slice(0, MAX_VISITS);
  } catch {
    // Written by an older build, or corrupted. An unreadable list is an
    // empty one; the next visit starts it again.
    return [];
  }
}

/**
 * Records a visit and returns the new list.
 *
 * Revisiting a page moves it to the front rather than adding a second entry —
 * the list answers "where was I", and the same page appearing three times
 * answers it worse.
 */
export function recordVisit(store: VisitStore | null, visit: Visit): Visit[] {
  if (!visit.pageId) return readVisits(store);

  const existing = readVisits(store).filter((row) => row.pageId !== visit.pageId);
  writeVisits(store, [visit, ...existing].slice(0, MAX_VISITS));
  // Read back rather than return what was computed: where the write did not
  // land — no store, or one that refuses — the caller should be told the
  // truth about what was kept.
  return readVisits(store);
}

/** Removes a page from the list, for one that has been deleted. */
export function forgetVisit(store: VisitStore | null, pageId: string): Visit[] {
  writeVisits(
    store,
    readVisits(store).filter((row) => row.pageId !== pageId),
  );
  return readVisits(store);
}

/** Empties the list. */
export function clearVisits(store: VisitStore | null): void {
  writeVisits(store, []);
}

function writeVisits(store: VisitStore | null, visits: Visit[]): void {
  if (!store) return;
  try {
    store.setItem(STORAGE_KEY, JSON.stringify(visits));
  } catch {
    // Out of quota, or a store that refuses writes. Losing the convenience
    // is the right outcome; failing the navigation that triggered it is not.
  }
}

/** Whether a parsed row is a visit this build can use. */
function isVisit(row: unknown): row is Visit {
  if (!row || typeof row !== "object") return false;
  const visit = row as Partial<Visit>;
  return (
    typeof visit.pageId === "string" &&
    visit.pageId !== "" &&
    typeof visit.shortId === "string" &&
    typeof visit.spaceSlug === "string" &&
    typeof visit.at === "number"
  );
}
