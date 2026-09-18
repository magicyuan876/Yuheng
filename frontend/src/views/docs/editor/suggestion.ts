// The trigger machinery behind the `[[` and `@` menus.
//
// Written here rather than taken from a package because the decision it makes
// is small, particular, and worth being able to test: given the text before
// the cursor, is a menu open, and what has been typed into it? Everything
// stateful about the menu — the list, the network, the keyboard — lives above
// this, and the plugin below only reports where the trigger is.
//
// It is also the machinery T2.4's slash menu will want, which is the other
// reason it is a general trigger rather than two special cases.
import { Plugin, PluginKey } from '@tiptap/pm/state'
import type { EditorState } from '@tiptap/pm/state'

/** One thing that opens a menu. */
export interface Trigger {
  /** A name the view switches on: 'page', 'mention', 'command'. */
  name: string
  /** The characters that open it, e.g. '[[' or '@'. */
  chars: string
  /** True when the trigger only counts at the start of a line or after a
   * space, which is what stops an email address opening the mention menu. */
  requireBoundary?: boolean
  /** How many characters may be typed into the query before it gives up.
   * A menu nobody is choosing from should close rather than keep matching. */
  maxQuery?: number
}

/** An open menu. */
export interface ActiveTrigger {
  name: string
  /** The document position the trigger characters start at. */
  from: number
  /** The position just after the cursor's query text. */
  to: number
  /** What has been typed since the trigger, without the trigger itself. */
  query: string
}

const DEFAULT_MAX_QUERY = 60

/**
 * Finds the trigger the cursor is currently inside, if any.
 *
 * Only ever looks at the text of the single block the cursor is in, so a
 * trigger cannot reach across a paragraph, and the answer costs nothing to
 * recompute on every keystroke.
 */
export function findTrigger(state: EditorState, triggers: readonly Trigger[]): ActiveTrigger | null {
  const { selection } = state
  if (!selection.empty) return null
  const $from = selection.$from
  // Text from the start of the parent block up to the cursor.
  const before = $from.parent.textBetween(0, $from.parentOffset, undefined, '￼')
  const blockStart = $from.start()

  let best: ActiveTrigger | null = null
  for (const trigger of triggers) {
    const at = before.lastIndexOf(trigger.chars)
    if (at < 0) continue
    const query = before.slice(at + trigger.chars.length)
    if (query.length > (trigger.maxQuery ?? DEFAULT_MAX_QUERY)) continue
    // A newline or the object replacement character means the trigger and the
    // cursor are not in the same run of text any more.
    if (/[\n￼]/.test(query)) continue
    if (trigger.requireBoundary && at > 0 && !/\s/.test(before[at - 1]!)) continue
    const from = blockStart + at
    // The nearest trigger wins: typing "@" inside a `[[` query means the
    // mention menu, which is what the last one opened is.
    if (!best || from > best.from) {
      best = { name: trigger.name, from, to: blockStart + before.length, query }
    }
  }
  return best
}

/** The plugin's key, so a view can read the current trigger. */
export const suggestionKey = new PluginKey<ActiveTrigger | null>('yuhengSuggestion')

/** Reads the open trigger out of an editor state. */
export function activeTrigger(state: EditorState): ActiveTrigger | null {
  return suggestionKey.getState(state) ?? null
}

/**
 * Tracks the open trigger and tells the view when it changes.
 *
 * `onChange` is called with null when the menu should close, which happens for
 * every reason at once: moving the cursor away, selecting a range, deleting
 * the trigger, or typing past the query limit. Having one answer rather than a
 * list of closing conditions is what keeps the menu from getting stuck open.
 */
export function suggestionPlugin(
  triggers: readonly Trigger[],
  onChange: (active: ActiveTrigger | null, state: EditorState) => void,
): Plugin {
  return new Plugin<ActiveTrigger | null>({
    key: suggestionKey,
    state: {
      init: (_config, state) => findTrigger(state, triggers),
      apply: (tr, previous, _old, next) => {
        const current = findTrigger(next, triggers)
        if (!sameTrigger(previous, current)) onChange(current, next)
        // A transaction that changed nothing relevant keeps the same object,
        // so a view watching it does not re-render.
        return sameTrigger(previous, current) ? previous : current
      },
    },
  })
}

/** Two triggers are the same menu in the same state. */
export function sameTrigger(a: ActiveTrigger | null, b: ActiveTrigger | null): boolean {
  if (a === null || b === null) return a === b
  return a.name === b.name && a.from === b.from && a.to === b.to && a.query === b.query
}

/** Moves a selected index around a list, wrapping at both ends. */
export function moveSelection(index: number, delta: number, length: number): number {
  if (length <= 0) return 0
  return (((index + delta) % length) + length) % length
}
