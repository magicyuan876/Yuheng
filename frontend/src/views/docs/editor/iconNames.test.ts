// Every icon name the editor's menus ask for has to exist in the icon set.
//
// This test exists because a batch of them did not. `<t-icon>` renders
// nothing at all for a name it does not know — no warning, no fallback, no
// error — so half the buttons on the selection toolbar were simply blank
// squares, and nothing in a type check or any other test said so. The names
// are strings in a catalogue; this is the only place that can hold them to
// the set they are strings *of*.
import assert from 'node:assert/strict'
import { existsSync } from 'node:fs'
import { createRequire } from 'node:module'
import { dirname, join } from 'node:path'
import { test } from 'vitest'

import { blockMenuItems } from './blockMenu'
import { blockCommands } from './commands'
import { TABLE_ACTIONS } from './tableActions'
import { TOOLBAR_ITEMS } from './toolbar'

/** Where the installed icon set keeps one module per icon. */
const iconDir = join(
  dirname(createRequire(import.meta.url).resolve('tdesign-icons-vue-next/package.json')),
  'esm', 'components',
)

function assertIcons(what: string, names: Iterable<string>) {
  const missing = [...new Set(names)].filter((name) => !existsSync(join(iconDir, `${name}.js`)))
  assert.deepEqual(missing, [], `${what} names icons the set does not have: ${missing.join(', ')}`)
}

test('the icon set is where this test expects it', () => {
  assert.ok(existsSync(join(iconDir, 'link.js')), iconDir)
})

test('every selection toolbar icon exists', () => {
  assertIcons('the selection toolbar', TOOLBAR_ITEMS.map((i) => i.icon))
})

test('every slash menu icon exists', () => {
  // Built with everything switched on, so no entry is left out of the check.
  const commands = blockCommands({
    embeds: true, drawings: true, drawio: true, copyBlockRef: () => {},
  })
  assertIcons('the slash menu', commands.map((c) => c.icon))
})

test('every block menu icon exists', () => {
  assertIcons('the block menu', blockMenuItems({ copyBlockRef: () => {} }).map((i) => i.icon))
})

test('every table toolbar icon exists', () => {
  assertIcons('the table toolbar', TABLE_ACTIONS.map((a) => a.icon))
})

test('a heading entry carries a badge, because the set has no H1 glyph', () => {
  // Without one, heading 1 and heading 2 would draw the same icon and read as
  // the same button; the badge is what tells them apart.
  for (const id of ['heading1', 'heading2']) {
    assert.ok(TOOLBAR_ITEMS.find((i) => i.id === id)?.badge, `${id} has no badge`)
  }
  const commands = blockCommands()
  for (const id of ['heading1', 'heading2', 'heading3']) {
    assert.ok(commands.find((c) => c.id === id)?.badge, `${id} has no badge`)
  }
})
