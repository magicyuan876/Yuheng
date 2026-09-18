// Turning a notification row into something a person can read.
//
// The server stores what happened — a kind, an actor, a page, a payload — and
// deliberately not a sentence, because the sentence depends on the reader's
// language and the row does not. This is where the two meet.
//
// It is a pure function over the row and a translate function, so the shapes
// a notification can arrive in are exercised directly. That matters more here
// than it looks: a notification whose payload is missing a field it usually
// has must still render as a readable line rather than "undefined did
// something to undefined", and that case arrives from real data — a page
// renamed since, an actor removed from the workspace, a row written by an
// older build.

/** One notification, as the server returns it. */
export interface NotificationLike {
  id: string
  kind: string
  page_id?: string
  comment_id?: string
  actor?: { username?: string; email?: string }
  payload?: Record<string, unknown>
  read_at?: string
  created_at: string
}

/** What the list needs to draw one row. */
export interface Described {
  /** The sentence: who did what. */
  title: string
  /** A line of the comment, when there is one. */
  excerpt: string
  /** A tdesign icon name. */
  icon: string
  /** Where clicking it goes, or empty when the row is not a link. */
  pageId: string
  unread: boolean
}

/** One day's worth of notifications. Generic so grouping does not narrow the
 * rows to the shape this module happens to need. */
export interface DayGroup<T extends NotificationLike = NotificationLike> {
  key: string
  label: 'today' | 'yesterday' | 'earlier'
  date: string
  items: T[]
}

/** The icon for each kind; an unknown kind gets a neutral one. */
const ICONS: Record<string, string> = {
  comment: 'chat-bubble',
  mention: 'user-arrow-right',
  page_updated: 'edit',
  access_granted: 'usergroup',
}

/**
 * Describes one notification.
 *
 * `t` takes a key and values, like vue-i18n's; passing it in rather than
 * importing keeps this testable and keeps the key names in one place.
 */
export function describe(
  row: NotificationLike,
  t: (key: string, values?: Record<string, unknown>) => string,
): Described {
  const actor = actorName(row, t)
  const title = String(row.payload?.title ?? '').trim() || t('docs.tree.untitled')

  const key = ICONS[row.kind] ? `docs.notifications.kind.${row.kind}` : 'docs.notifications.kind.other'
  return {
    title: t(key, { actor, title }),
    excerpt: excerptOf(row),
    icon: ICONS[row.kind] ?? 'notification',
    pageId: row.page_id ?? '',
    unread: !row.read_at,
  }
}

/**
 * Who did it.
 *
 * An actor removed from the workspace, or a row old enough to predate the
 * field, still has to read as a sentence — so there is a word for "somebody"
 * rather than a blank where a name goes.
 */
function actorName(
  row: NotificationLike,
  t: (key: string, values?: Record<string, unknown>) => string,
): string {
  return row.actor?.username || row.actor?.email || t('docs.links.someone')
}

/** How much of a comment a row shows. */
export const MAX_EXCERPT = 120

function excerptOf(row: NotificationLike): string {
  const raw = row.payload?.excerpt
  if (typeof raw !== 'string') return ''
  const collapsed = raw.replace(/\s+/g, ' ').trim()
  if (collapsed.length <= MAX_EXCERPT) return collapsed
  return `${collapsed.slice(0, MAX_EXCERPT)}…`
}

/**
 * Groups notifications by day, so a list reads as "today, yesterday, then
 * the rest" rather than as an undifferentiated column of timestamps.
 *
 * Returns the days in the order they arrived, which is newest first, and the
 * rows within each day likewise.
 */
export function groupByDay<T extends NotificationLike>(
  rows: readonly T[],
  now = new Date(),
): DayGroup<T>[] {
  const out: DayGroup<T>[] = []
  const byKey = new Map<string, number>()

  for (const row of rows) {
    const at = new Date(row.created_at)
    const key = Number.isNaN(at.getTime()) ? 'unknown' : dayKey(at)
    let index = byKey.get(key)
    if (index === undefined) {
      index = out.length
      byKey.set(key, index)
      out.push({ key, label: labelFor(key, now), date: key, items: [] })
    }
    out[index]!.items.push(row)
  }
  return out
}

function dayKey(at: Date): string {
  const month = `${at.getMonth() + 1}`.padStart(2, '0')
  const day = `${at.getDate()}`.padStart(2, '0')
  return `${at.getFullYear()}-${month}-${day}`
}

function labelFor(key: string, now: Date): 'today' | 'yesterday' | 'earlier' {
  if (key === dayKey(now)) return 'today'
  const yesterday = new Date(now)
  yesterday.setDate(yesterday.getDate() - 1)
  if (key === dayKey(yesterday)) return 'yesterday'
  return 'earlier'
}
