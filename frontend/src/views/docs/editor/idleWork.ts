// Deferring the work that looks at the whole document.
//
// Counting words and collecting headings both walk every node. Doing that on
// every keystroke is fine on a short page and is exactly what makes a long one
// feel heavy: the walk itself is cheap, but running it sixty times a second on
// fifty thousand words is not, and it happens on the same thread that has to
// paint the character that was just typed.
//
// So it is deferred. Typing schedules the work and carries on; the walk runs
// once the browser is idle, or at the latest after a bounded wait, so a person
// who keeps typing still sees the count catch up rather than freeze.
//
// The scheduler is written here, with its timers injected, so the coalescing
// and the teardown are exercised by tests rather than by trying a long
// document by hand.

/** The timing primitives, injected so a test can drive them. */
export interface IdleClock {
  /** Runs fn when the browser is next idle. */
  requestIdle: (fn: () => void) => number;
  cancelIdle: (handle: number) => void;
  /** The upper bound on how long fn may be put off. */
  setTimeout: (fn: () => void, ms: number) => number;
  clearTimeout: (handle: number) => void;
}

/** The default clock, using requestIdleCallback where the browser has it. */
export function browserClock(): IdleClock {
  const ric = (globalThis as { requestIdleCallback?: (fn: () => void) => number }).requestIdleCallback;
  const cic = (globalThis as { cancelIdleCallback?: (h: number) => void }).cancelIdleCallback;
  return {
    // Safari has no requestIdleCallback; a short timeout is the same promise
    // with worse timing, which is the right fallback for work that is only
    // ever cosmetic.
    requestIdle: ric ? (fn) => ric(fn) : (fn) => setTimeout(fn, 1) as unknown as number,
    cancelIdle: cic ? (h) => cic(h) : (h) => clearTimeout(h as unknown as ReturnType<typeof setTimeout>),
    setTimeout: (fn, ms) => setTimeout(fn, ms) as unknown as number,
    clearTimeout: (h) => clearTimeout(h as unknown as ReturnType<typeof setTimeout>),
  };
}

/** How long the work may be put off while somebody keeps typing. */
export const DEFAULT_MAX_DELAY_MS = 400;

/**
 * Runs one piece of work at most once per idle moment.
 *
 * Calling `schedule` any number of times before it runs produces exactly one
 * run, with the state as it is when that run happens — which is what makes
 * this correct for a derived value: the answer is about the document now, not
 * about the document at any of the moments the work was requested.
 */
export class IdleScheduler {
  private idleHandle: number | null = null;
  private deadlineHandle: number | null = null;
  private cancelled = false;

  constructor(
    private readonly work: () => void,
    private readonly clock: IdleClock = browserClock(),
    private readonly maxDelayMs: number = DEFAULT_MAX_DELAY_MS,
  ) {}

  /** True while a run is pending. */
  get pending(): boolean {
    return this.idleHandle !== null || this.deadlineHandle !== null;
  }

  /** Asks for the work to run soon. Cheap to call on every keystroke. */
  schedule(): void {
    if (this.cancelled || this.pending) return;
    this.idleHandle = this.clock.requestIdle(() => {
      this.idleHandle = null;
      this.run();
    });
    // The idle callback may never come on a page that is never idle, so there
    // is always a deadline behind it.
    this.deadlineHandle = this.clock.setTimeout(() => {
      this.deadlineHandle = null;
      this.run();
    }, this.maxDelayMs);
  }

  /** Runs the work now, cancelling anything pending. */
  flush(): void {
    if (this.cancelled) return;
    this.clear();
    this.work();
  }

  /** Stops permanently. A schedule after this does nothing. */
  cancel(): void {
    this.clear();
    this.cancelled = true;
  }

  private run(): void {
    if (this.cancelled) return;
    this.clear();
    this.work();
  }

  private clear(): void {
    if (this.idleHandle !== null) {
      this.clock.cancelIdle(this.idleHandle);
      this.idleHandle = null;
    }
    if (this.deadlineHandle !== null) {
      this.clock.clearTimeout(this.deadlineHandle);
      this.deadlineHandle = null;
    }
  }
}
