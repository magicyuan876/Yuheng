// Resolves page ids to what a link should read as.
//
// A page link stores only an id, so every link on screen needs a title from
// somewhere. Asking per link would mean one request per link on a page that
// might have dozens; this collects the ids asked for within a tick and
// resolves them in one batch, then keeps the answers so scrolling back over a
// link costs nothing.
//
// Nothing here is Vue, and the network call is injected, so the batching and
// the invalidation are exercised directly by tests.

/** What a link needs to draw itself. */
export interface ResolvedPage {
  pageId: string;
  title: string;
  icon?: string;
  shortId?: string;
  spaceId?: string;
  /** False when the page is gone or the reader may not see it. The editor
   * shows one broken state for both; the server does not distinguish them
   * either, so a link cannot be used to probe for a page. */
  resolved: boolean;
}

/** The batch call this cache is built on. */
export type ResolveTitles = (pageIds: string[]) => Promise<ResolvedPage[]>;

export interface TitleCacheOptions {
  resolve: ResolveTitles;
  /** Called whenever an answer arrives, so views can re-read. */
  onChange?: () => void;
  /** How long to gather ids before sending; a tick by default. */
  batchDelayMs?: number;
  /** How many ids one request may carry. */
  batchSize?: number;
}

const DEFAULT_BATCH_DELAY_MS = 16;
const DEFAULT_BATCH_SIZE = 200;

/**
 * A page title lookup that batches, caches and can be invalidated.
 *
 * `get` never waits: it answers with what is known and schedules a lookup for
 * what is not, which is what lets a node view render synchronously and fill in
 * a moment later.
 */
export class TitleCache {
  private readonly known = new Map<string, ResolvedPage>();
  private readonly pending = new Set<string>();
  private timer: ReturnType<typeof setTimeout> | null = null;
  private readonly opts: Required<Omit<TitleCacheOptions, "onChange">> & { onChange: () => void };

  constructor(options: TitleCacheOptions) {
    this.opts = {
      resolve: options.resolve,
      onChange: options.onChange ?? (() => {}),
      batchDelayMs: options.batchDelayMs ?? DEFAULT_BATCH_DELAY_MS,
      batchSize: options.batchSize ?? DEFAULT_BATCH_SIZE,
    };
  }

  /** What is known about a page right now; undefined means "asking". */
  get(pageId: string): ResolvedPage | undefined {
    if (!pageId) return undefined;
    const have = this.known.get(pageId);
    if (have) return have;
    this.request(pageId);
    return undefined;
  }

  /** Queues a page for the next batch. */
  request(pageId: string): void {
    if (!pageId || this.known.has(pageId) || this.pending.has(pageId)) return;
    this.pending.add(pageId);
    if (this.timer === null) {
      this.timer = setTimeout(() => {
        this.timer = null;
        void this.flush();
      }, this.opts.batchDelayMs);
    }
  }

  /** Resolves everything queued, now. */
  async flush(): Promise<void> {
    if (this.timer !== null) {
      clearTimeout(this.timer);
      this.timer = null;
    }
    while (this.pending.size > 0) {
      const batch = [...this.pending].slice(0, this.opts.batchSize);
      for (const id of batch) this.pending.delete(id);
      let answers: ResolvedPage[] = [];
      try {
        answers = await this.opts.resolve(batch);
      } catch {
        // A failed lookup is not an unresolved page: saying "broken link"
        // because the network hiccuped would be a lie. The ids simply go back
        // to being unknown, and the next render asks again.
        continue;
      }
      for (const answer of answers) this.known.set(answer.pageId, answer);
      // An id nobody answered for is unresolved, which is what the server
      // means by leaving it out.
      for (const id of batch) {
        if (!this.known.has(id)) {
          this.known.set(id, { pageId: id, title: "", resolved: false });
        }
      }
      this.opts.onChange();
    }
  }

  /**
   * Forgets one page, or everything.
   *
   * Renaming a page is the reason this exists: the rename happens elsewhere,
   * the links to it are cached here, and dropping the entry is what makes them
   * catch up without a reload.
   */
  invalidate(pageId?: string): void {
    if (pageId) {
      this.known.delete(pageId);
      this.pending.delete(pageId);
    } else {
      this.known.clear();
      this.pending.clear();
    }
    this.opts.onChange();
  }

  /** Stops any scheduled batch; the cache is unusable afterwards. */
  dispose(): void {
    if (this.timer !== null) {
      clearTimeout(this.timer);
      this.timer = null;
    }
    this.pending.clear();
  }

  /** How many answers are held, for tests and diagnostics. */
  get size(): number {
    return this.known.size;
  }
}
