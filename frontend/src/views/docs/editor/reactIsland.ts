// Mounting a React component inside this Vue application.
//
// Excalidraw is a React component and the only one in the product, so rather
// than adopting React everywhere it is mounted as an island: a Vue component
// owns a plain element, this hands that element to React for the lifetime of
// the component, and takes it back when the component goes away.
//
// The part worth being careful about is the taking back. A React root that is
// never unmounted keeps its whole tree, its listeners and everything the tree
// closed over alive for as long as the page is open, and a document with a
// drawing on every second page would leak one per visit. So the lifecycle is
// written here, separately from the component and from React itself, with the
// root factory injected — which is what lets a test prove that mounting twice
// unmounts once, that unmounting twice is harmless, and that nothing is held
// after teardown.

/** The part of React's root API this island uses. */
export interface IslandRoot {
  render: (element: unknown) => void
  unmount: () => void
}

/** Creates a root on an element; React's createRoot has this shape. */
export type RootFactory = (container: Element) => IslandRoot

/**
 * One React subtree living inside a Vue component.
 *
 * Every method is safe to call in any order and any number of times, because
 * the calling component's lifecycle is not something this can control: Vue may
 * unmount it mid-render, and a page change may replace its element while an
 * async import is still in flight.
 */
export class ReactIsland {
  private root: IslandRoot | null = null
  private container: Element | null = null
  private destroyed = false

  constructor(private readonly createRoot: RootFactory) {}

  /** True while a subtree is mounted. */
  get mounted(): boolean {
    return this.root !== null
  }

  /**
   * Renders an element into the container, creating the root the first time.
   *
   * Re-rendering into the same container reuses the root, which is what React
   * expects; a different container is a different mount, so the old one is
   * unmounted first rather than left behind.
   */
  render(container: Element, element: unknown): void {
    if (this.destroyed) return
    if (this.root && this.container !== container) {
      this.unmount()
    }
    if (!this.root) {
      this.root = this.createRoot(container)
      this.container = container
    }
    this.root.render(element)
  }

  /** Tears the subtree down. Safe to call when nothing is mounted. */
  unmount(): void {
    const root = this.root
    // Cleared before unmounting, so a re-entrant call from a component's own
    // teardown cannot unmount the same root twice.
    this.root = null
    this.container = null
    if (!root) return
    try {
      root.unmount()
    } catch {
      // React throws when asked to unmount during its own render. The root is
      // already forgotten here, so the tree is collectable either way and
      // there is nothing useful to report.
    }
  }

  /**
   * Tears down permanently. A render after this does nothing, which is what
   * makes an async import that resolves after the component was destroyed
   * harmless rather than a leak.
   */
  destroy(): void {
    this.unmount()
    this.destroyed = true
  }

  /** True once destroy has been called. */
  get isDestroyed(): boolean {
    return this.destroyed
  }
}
