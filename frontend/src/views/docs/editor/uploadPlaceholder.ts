// A placeholder shown where a file is being uploaded.
//
// It is a ProseMirror decoration, not a node. That matters: the document is
// validated against packages/docs-schema on every save and shared with other
// clients as a Yjs document, so a temporary "uploading…" node would either be
// rejected by the server or replicated to everyone else as a phantom block.
// A decoration lives only in this client's view, and ProseMirror maps its
// position through every edit made while the upload is in flight — so typing
// above the insertion point moves the placeholder with the text, and the
// finished image lands exactly where the file was dropped.
import { Plugin, PluginKey } from '@tiptap/pm/state'
import type { EditorState, Transaction } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'

/** Identifies the placeholder plugin's state. */
export const uploadPlaceholderKey = new PluginKey<DecorationSet>('yuhengUploadPlaceholder')

/** What a transaction asks the plugin to do. */
interface PlaceholderAction {
  add?: { key: string; pos: number }
  remove?: string
}

/** Marks a transaction as adding a placeholder at pos. */
export function addPlaceholder(tr: Transaction, key: string, pos: number): Transaction {
  return tr.setMeta(uploadPlaceholderKey, { add: { key, pos } } satisfies PlaceholderAction)
}

/** Marks a transaction as removing a placeholder. */
export function removePlaceholder(tr: Transaction, key: string): Transaction {
  return tr.setMeta(uploadPlaceholderKey, { remove: key } satisfies PlaceholderAction)
}

/**
 * Where a placeholder currently sits, or null when it is gone — because the
 * upload was cancelled, or because the text around it was deleted while the
 * file was still travelling.
 */
export function placeholderPos(state: EditorState, key: string): number | null {
  const set = uploadPlaceholderKey.getState(state)
  if (!set) return null
  const found = set.find(undefined, undefined, (spec) => spec.uploadKey === key)
  return found.length > 0 ? found[0]!.from : null
}

/** Every placeholder currently in the document, in document order. */
export function placeholderKeys(state: EditorState): string[] {
  const set = uploadPlaceholderKey.getState(state)
  if (!set) return []
  return set.find().map((d) => (d.spec as { uploadKey: string }).uploadKey)
}

/**
 * The plugin. `render` builds the DOM for one placeholder; it is injected so
 * this module needs no DOM of its own and can be tested as pure state.
 */
export function uploadPlaceholderPlugin(render: (key: string) => HTMLElement): Plugin {
  return new Plugin<DecorationSet>({
    key: uploadPlaceholderKey,
    state: {
      init: () => DecorationSet.empty,
      apply(tr, set) {
        // Mapping first: an edit elsewhere in the document must move the
        // placeholder rather than strand it.
        let next = set.map(tr.mapping, tr.doc)
        const action = tr.getMeta(uploadPlaceholderKey) as PlaceholderAction | undefined
        if (action?.add) {
          const widget = Decoration.widget(
            action.add.pos,
            () => render(action.add!.key),
            { uploadKey: action.add.key, side: 1 },
          )
          next = next.add(tr.doc, [widget])
        }
        if (action?.remove) {
          const doomed = next.find(undefined, undefined, (spec) => spec.uploadKey === action.remove)
          if (doomed.length > 0) next = next.remove(doomed)
        }
        return next
      },
    },
    props: {
      decorations(state) {
        return uploadPlaceholderKey.getState(state)
      },
    },
  })
}
