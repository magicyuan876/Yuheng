// Client-side model of one space's page tree.
//
// The server exposes the tree one parent at a time (lazy, cursor-paged), and
// pushes change hints over SSE. This model keeps what has been loaded, keeps
// every sibling list sorted by the server's fractional position key, and
// flattens the expanded part into rows for a virtual list. It is plain
// TypeScript (no Vue) so it can be unit-tested; the component bumps a
// version counter after each mutation to re-render.

export interface TreeNodeData {
  id: string
  short_id: string
  space_id: string
  parent_id: string | null
  position: string
  title: string
  icon?: string | null
  has_children: boolean
  can_edit: boolean
  restricted?: boolean
  updated_at?: string
}

export interface TreeRow {
  node: TreeNodeData
  depth: number
  expanded: boolean
}

export type DropPosition = 'before' | 'after' | 'inside'

/** What the API needs for a move; `afterId` undefined means "append". */
export interface MoveTarget {
  parentId: string | null
  afterId: string | null | undefined
}

/** Server-sent tree events this model understands (see internal/docs/events). */
export interface DocsTreeEvent {
  type: string
  space_id?: string
  page_id?: string
  payload?: Record<string, unknown>
}

const ROOT = ''

function key(parentId: string | null | undefined): string {
  return parentId ?? ROOT
}

function byPosition(a: TreeNodeData, b: TreeNodeData): number {
  if (a.position === b.position) return a.id < b.id ? -1 : a.id > b.id ? 1 : 0
  return a.position < b.position ? -1 : 1
}

export class PageTreeModel {
  private nodes = new Map<string, TreeNodeData>()
  /** parent key -> ordered child ids (only for loaded parents) */
  private children = new Map<string, string[]>()
  private loaded = new Set<string>()
  private expandedIds = new Set<string>()
  /** Parents whose children must be re-fetched (a rebalance or a move into an unloaded parent). */
  private stale = new Set<string>()

  /** Forget everything (switching spaces). */
  reset(): void {
    this.nodes.clear()
    this.children.clear()
    this.loaded.clear()
    this.expandedIds.clear()
    this.stale.clear()
  }

  get(id: string): TreeNodeData | undefined {
    return this.nodes.get(id)
  }

  has(id: string): boolean {
    return this.nodes.has(id)
  }

  size(): number {
    return this.nodes.size
  }

  isLoaded(parentId: string | null): boolean {
    return this.loaded.has(key(parentId))
  }

  isExpanded(id: string): boolean {
    return this.expandedIds.has(id)
  }

  isStale(parentId: string | null): boolean {
    return this.stale.has(key(parentId))
  }

  clearStale(parentId: string | null): void {
    this.stale.delete(key(parentId))
  }

  /** Returns and clears every parent that must be re-fetched (null = root). */
  takeStale(): Array<string | null> {
    const out = [...this.stale].map((k) => (k === ROOT ? null : k))
    this.stale.clear()
    return out
  }

  childIds(parentId: string | null): string[] {
    return this.children.get(key(parentId)) ?? []
  }

  /** Replace the children of a parent with a freshly loaded, complete list. */
  setChildren(parentId: string | null, list: TreeNodeData[]): void {
    const k = key(parentId)
    const previous = this.children.get(k) ?? []
    const next = new Set(list.map((n) => n.id))
    for (const id of previous) {
      if (!next.has(id)) this.dropSubtree(id)
    }
    const sorted = [...list].sort(byPosition)
    for (const n of sorted) this.nodes.set(n.id, { ...n, parent_id: parentId })
    this.children.set(k, sorted.map((n) => n.id))
    this.loaded.add(k)
    this.stale.delete(k)
    const parent = parentId ? this.nodes.get(parentId) : undefined
    if (parent) parent.has_children = sorted.length > 0
  }

  /** Append one cursor page of children (the parent stays "not loaded" until complete). */
  appendChildren(parentId: string | null, list: TreeNodeData[], complete: boolean): void {
    const k = key(parentId)
    const ids = this.children.get(k) ?? []
    for (const n of list) {
      if (!this.nodes.has(n.id)) ids.push(n.id)
      this.nodes.set(n.id, { ...n, parent_id: parentId })
    }
    const sorted = ids.map((id) => this.nodes.get(id)!).sort(byPosition)
    this.children.set(k, sorted.map((n) => n.id))
    if (complete) {
      this.loaded.add(k)
      this.stale.delete(k)
    }
  }

  /** Insert a node the server announced; ignored when its parent is not loaded. */
  insert(node: TreeNodeData): void {
    const parent = node.parent_id ? this.nodes.get(node.parent_id) : undefined
    if (node.parent_id && parent) parent.has_children = true
    const k = key(node.parent_id)
    if (!this.loaded.has(k)) return
    this.nodes.set(node.id, { ...node })
    const ids = this.children.get(k) ?? []
    if (!ids.includes(node.id)) ids.push(node.id)
    this.children.set(k, ids.map((id) => this.nodes.get(id)!).sort(byPosition).map((n) => n.id))
  }

  update(id: string, patch: Partial<TreeNodeData>): void {
    const node = this.nodes.get(id)
    if (!node) return
    Object.assign(node, patch)
    if (patch.position !== undefined) this.resort(node.parent_id)
  }

  /** Remove a node with its whole subtree. */
  remove(id: string): void {
    const node = this.nodes.get(id)
    if (!node) return
    const k = key(node.parent_id)
    const ids = this.children.get(k)
    if (ids) this.children.set(k, ids.filter((x) => x !== id))
    this.dropSubtree(id)
    const parent = node.parent_id ? this.nodes.get(node.parent_id) : undefined
    if (parent && this.loaded.has(key(parent.id)) && this.childIds(parent.id).length === 0) {
      parent.has_children = false
    }
  }

  /**
   * Move a node. When the destination parent is not loaded the node leaves
   * the view (it reappears when that parent is expanded) and the parent is
   * marked as having children.
   */
  move(id: string, parentId: string | null, position: string): void {
    const node = this.nodes.get(id)
    if (!node) return
    if (parentId && this.isDescendant(id, parentId)) return
    const from = key(node.parent_id)
    const fromIds = this.children.get(from)
    if (fromIds) this.children.set(from, fromIds.filter((x) => x !== id))
    const oldParent = node.parent_id ? this.nodes.get(node.parent_id) : undefined
    if (oldParent && this.loaded.has(key(oldParent.id)) && this.childIds(oldParent.id).length === 0) {
      oldParent.has_children = false
    }
    node.parent_id = parentId
    node.position = position
    const newParent = parentId ? this.nodes.get(parentId) : undefined
    if (newParent) newParent.has_children = true
    const to = key(parentId)
    if (!this.loaded.has(to)) {
      this.dropSubtree(id)
      return
    }
    const ids = this.children.get(to) ?? []
    ids.push(id)
    this.children.set(to, ids.map((x) => this.nodes.get(x)!).sort(byPosition).map((n) => n.id))
  }

  expand(id: string): void {
    this.expandedIds.add(id)
  }

  collapse(id: string): void {
    this.expandedIds.delete(id)
  }

  toggle(id: string): boolean {
    if (this.expandedIds.has(id)) {
      this.expandedIds.delete(id)
      return false
    }
    this.expandedIds.add(id)
    return true
  }

  /** Expand every ancestor of a page so it is visible. */
  reveal(id: string): void {
    for (const a of this.ancestorIds(id)) this.expandedIds.add(a)
  }

  /** Ancestor ids root-first (only those present in the model). */
  ancestorIds(id: string): string[] {
    const out: string[] = []
    let cur = this.nodes.get(id)
    const guard = new Set<string>()
    while (cur && cur.parent_id && !guard.has(cur.parent_id)) {
      guard.add(cur.parent_id)
      out.unshift(cur.parent_id)
      cur = this.nodes.get(cur.parent_id)
    }
    return out
  }

  isDescendant(ancestorId: string, id: string): boolean {
    if (ancestorId === id) return true
    return this.ancestorIds(id).includes(ancestorId)
  }

  /** The expanded part of the tree, flattened for a virtual list. */
  rows(): TreeRow[] {
    const out: TreeRow[] = []
    const walk = (parentId: string | null, depth: number) => {
      for (const id of this.childIds(parentId)) {
        const node = this.nodes.get(id)
        if (!node) continue
        const expanded = this.expandedIds.has(id)
        out.push({ node, depth, expanded })
        if (expanded && node.has_children) walk(id, depth + 1)
      }
    }
    walk(null, 0)
    return out
  }

  /**
   * Translate a drop gesture into the API's target, or null when the drop is
   * not allowed (onto itself or into its own subtree).
   */
  dropTarget(dragId: string, targetId: string, position: DropPosition): MoveTarget | null {
    const target = this.nodes.get(targetId)
    if (!target || dragId === targetId || this.isDescendant(dragId, targetId)) return null
    if (position === 'inside') return { parentId: targetId, afterId: undefined }
    const siblings = this.childIds(target.parent_id)
    const index = siblings.indexOf(targetId)
    if (position === 'after') {
      return { parentId: target.parent_id, afterId: targetId }
    }
    // before: after the previous sibling that is not the dragged node itself
    let prev: string | null = null
    for (let i = index - 1; i >= 0; i--) {
      if (siblings[i] !== dragId) {
        prev = siblings[i]
        break
      }
    }
    return { parentId: target.parent_id, afterId: prev }
  }

  /** Apply a server event; returns true when the tree changed. */
  applyEvent(ev: DocsTreeEvent, spaceId: string, canEdit: boolean): boolean {
    const p = (ev.payload ?? {}) as Record<string, unknown>
    switch (ev.type) {
      case 'docs.page.created':
      case 'docs.page.restored': {
        if (ev.space_id !== spaceId || !ev.page_id || this.nodes.has(ev.page_id)) return false
        const parentId = (p.parent_id as string | null | undefined) ?? null
        this.insert({
          id: ev.page_id,
          short_id: String(p.short_id ?? ''),
          space_id: spaceId,
          parent_id: parentId,
          position: String(p.position ?? ''),
          title: String(p.title ?? ''),
          icon: (p.icon as string | null | undefined) ?? null,
          has_children: ev.type === 'docs.page.restored' ? Number(p.count ?? 1) > 1 : false,
          can_edit: canEdit,
        })
        return true
      }
      case 'docs.page.meta_updated': {
        if (!ev.page_id || !this.nodes.has(ev.page_id)) return false
        const patch: Partial<TreeNodeData> = {}
        if ('title' in p) patch.title = String(p.title ?? '')
        if ('icon' in p) patch.icon = (p.icon as string | null) ?? null
        this.update(ev.page_id, patch)
        return true
      }
      case 'docs.page.moved': {
        if (!ev.page_id) return false
        if (p.left === true || (p.cross_space === true && ev.space_id !== spaceId)) {
          // The page left this space; orphans stay behind and need a reload.
          const had = this.nodes.has(ev.page_id)
          this.remove(ev.page_id)
          const orphaned = Array.isArray(p.orphaned) ? (p.orphaned as string[]) : []
          if (orphaned.length) this.stale.add(ROOT)
          return had || orphaned.length > 0
        }
        if (ev.space_id !== spaceId) return false
        const parentId = (p.parent_id as string | null | undefined) ?? null
        if (!this.nodes.has(ev.page_id)) {
          // Arrived from another space: reload the destination parent.
          this.stale.add(key(parentId))
          const parent = parentId ? this.nodes.get(parentId) : undefined
          if (parent) parent.has_children = true
          return true
        }
        this.move(ev.page_id, parentId, String(p.position ?? this.nodes.get(ev.page_id)?.position ?? ''))
        return true
      }
      case 'docs.page.deleted': {
        if (!ev.page_id || !this.nodes.has(ev.page_id)) return false
        this.remove(ev.page_id)
        return true
      }
      case 'docs.page.purged': {
        const ids = Array.isArray(p.ids) ? (p.ids as string[]) : ev.page_id ? [ev.page_id] : []
        let changed = false
        for (const id of ids) {
          if (this.nodes.has(id)) {
            this.remove(id)
            changed = true
          }
        }
        return changed
      }
      case 'docs.page.tree_rebalanced': {
        if (ev.space_id !== spaceId) return false
        this.stale.add(key((p.parent_id as string | null | undefined) ?? null))
        return true
      }
      default:
        return false
    }
  }

  private resort(parentId: string | null): void {
    const k = key(parentId)
    const ids = this.children.get(k)
    if (!ids) return
    this.children.set(k, ids.map((id) => this.nodes.get(id)!).sort(byPosition).map((n) => n.id))
  }

  private dropSubtree(id: string): void {
    const stack = [id]
    while (stack.length) {
      const cur = stack.pop()!
      for (const c of this.children.get(cur) ?? []) stack.push(c)
      this.children.delete(cur)
      this.loaded.delete(cur)
      this.expandedIds.delete(cur)
      this.stale.delete(cur)
      this.nodes.delete(cur)
    }
  }
}

// ---- URL helpers -----------------------------------------------------------------

export const SHORT_ID_LENGTH = 10
const SHORT_ID_RE = /^[0-9a-z]{10}$/

/** `/docs/spaces/{space}/{title-slug}-{short_id}`: the title part is cosmetic. */
export function pageSlug(title: string, shortId: string): string {
  const slug = title
    .toLowerCase()
    .normalize('NFKD')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 60)
    .replace(/-+$/g, '')
  return slug ? `${slug}-${shortId}` : shortId
}

/** The short id is always the last ten characters. */
export function shortIdFromSlug(slug: string): string | null {
  const candidate = slug.slice(-SHORT_ID_LENGTH)
  if (!SHORT_ID_RE.test(candidate)) return null
  if (slug.length > SHORT_ID_LENGTH && slug[slug.length - SHORT_ID_LENGTH - 1] !== '-') return null
  return candidate
}

/** Where in a row a pointer at `offsetY` (of `height`) lands. */
export function dropPositionFor(offsetY: number, height: number, canNest: boolean): DropPosition {
  const ratio = height > 0 ? offsetY / height : 0.5
  if (!canNest) return ratio < 0.5 ? 'before' : 'after'
  if (ratio < 0.25) return 'before'
  if (ratio > 0.75) return 'after'
  return 'inside'
}
