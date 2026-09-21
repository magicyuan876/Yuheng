import assert from 'node:assert/strict'
import { test } from 'vitest'

import {
  CELL_COLORS, CELL_COLORS_SOFT, CELL_COLORS_STRONG, TABLE_ACTIONS,
  TABLE_TOOLBAR_OFFSET, TABLE_TOOLBAR_WIDTH,
  canRun, isCellColor, runAction, tableGroups, tableToolbarPlacement,
} from './tableActions'

/** An editor stand-in that records what was chained on it. */
function fakeEditor(opts: { allow?: boolean; calls?: string[] } = {}) {
  const calls = opts.calls ?? []
  const chain: Record<string, unknown> = {}
  const make = (name: string) => (...args: unknown[]) => {
    calls.push(args.length ? `${name}(${JSON.stringify(args[0])})` : name)
    return { run: () => opts.allow ?? true, ...chain }
  }
  for (const name of [
    'focus', 'addRowBefore', 'addRowAfter', 'deleteRow', 'addColumnBefore',
    'addColumnAfter', 'deleteColumn', 'mergeCells', 'splitCell',
    'toggleHeaderRow', 'deleteTable',
  ]) chain[name] = make(name)
  return {
    calls,
    chain: () => chain as never,
    can: () => ({ chain: () => chain as never }),
  }
}

test('every action names an icon and belongs to a group', () => {
  for (const action of TABLE_ACTIONS) {
    assert.ok(action.icon, `${action.id} has no icon`)
    assert.ok(action.labelKey.startsWith('docs.table.'), `${action.id} label`)
    // Every entry does exactly one thing: a command, a tableOps op, or the
    // palette. Two of them, or none, is a catalogue mistake.
    const kinds = [action.command, action.op, action.palette].filter(Boolean).length
    assert.equal(kinds, 1, `${action.id} declares ${kinds} kinds of behaviour`)
  }
})

test('ids are unique, so the bar can key on them', () => {
  const ids = TABLE_ACTIONS.map((a) => a.id)
  assert.equal(new Set(ids).size, ids.length)
})

test('groups come out in order, one array per run of the same group', () => {
  const groups = tableGroups()
  assert.deepEqual(groups.map((g) => g[0]!.group), ['row', 'column', 'cell', 'table'])
  assert.equal(groups.flat().length, TABLE_ACTIONS.length)
})

test('canRun asks the editor, and says no without one', () => {
  const action = TABLE_ACTIONS.find((a) => a.id === 'addRowAfter')!
  assert.equal(canRun(null, action), false)
  assert.equal(canRun(fakeEditor({ allow: true }) as never, action), true)
  assert.equal(canRun(fakeEditor({ allow: false }) as never, action), false)
})

test('canRun says yes for the palette, which runs no command', () => {
  const palette = TABLE_ACTIONS.find((a) => a.palette)!
  assert.equal(canRun(fakeEditor() as never, palette), true)
})

test('canRun survives a command the schema does not have', () => {
  assert.equal(
    canRun(fakeEditor() as never, { id: 'x', labelKey: 'y', icon: 'i', group: 'row', command: 'nope' }),
    false,
  )
})

test('runAction focuses the editor before running the command', () => {
  const editor = fakeEditor()
  const ok = runAction(editor as never, TABLE_ACTIONS.find((a) => a.id === 'deleteRow')!)
  assert.equal(ok, true)
  assert.deepEqual(editor.calls, ['focus', 'deleteRow'])
})

test('runAction does nothing for the palette entry', () => {
  const editor = fakeEditor()
  assert.equal(runAction(editor as never, TABLE_ACTIONS.find((a) => a.palette)!), false)
  assert.deepEqual(editor.calls, [])
})

test('the cell colours are distinct', () => {
  assert.equal(new Set(CELL_COLORS).size, CELL_COLORS.length)
})

test('the two colour bands line up, ten hues each', () => {
  // The grid draws ten to a row, so a band of any other length would leave
  // the pale swatch sitting above a different hue than the one it tints.
  assert.equal(CELL_COLORS_SOFT.length, 10)
  assert.equal(CELL_COLORS_STRONG.length, 10)
  assert.deepEqual(CELL_COLORS, [...CELL_COLORS_SOFT, ...CELL_COLORS_STRONG])
})

test('every offered colour is one the server would accept', () => {
  // The palette is the one place a bad colour could be introduced wholesale;
  // the check is the same regex the schema applies on save.
  for (const color of CELL_COLORS) {
    assert.ok(isCellColor(color), `${color} would be rejected on save`)
  }
})

test('isCellColor accepts the notations the schema allows', () => {
  for (const ok of ['#fff', '#ffffff', '#ffffffff', 'red', 'rebeccapurple',
    'rgb(1, 2, 3)', 'rgba(1,2,3,0.5)', 'hsl(1, 2%, 3%)']) {
    assert.ok(isCellColor(ok), ok)
  }
})

test('isCellColor refuses what the schema would reject', () => {
  // url() is the one that matters: it is how a stylesheet value smuggles a
  // request to somewhere else in, and the server rejects it for that reason.
  for (const bad of ['url(x)', 'javascript:alert(1)', '#12', '', ' #fff',
    'rgb(1,2,3); background: url(x)', '#gggggg']) {
    assert.equal(isCellColor(bad), false, bad)
  }
})

const viewport = { width: 1200, height: 800 }

test('the bar sits above the table, aligned with its left edge', () => {
  const at = tableToolbarPlacement({ left: 300, right: 900, top: 400, bottom: 600 }, viewport)
  assert.equal(at.left, 300)
  assert.equal(at.top, 400 - TABLE_TOOLBAR_OFFSET)
})

test('a table at the top of the window puts the bar just inside it', () => {
  const at = tableToolbarPlacement({ left: 100, right: 400, top: 10, bottom: 300 }, viewport)
  assert.equal(at.top, 18)
})

test('a table near the right edge does not push the bar off screen', () => {
  const at = tableToolbarPlacement({ left: 1150, right: 1190, top: 400, bottom: 500 }, viewport)
  assert.equal(at.left, viewport.width - TABLE_TOOLBAR_WIDTH - 8)
})

test('a viewport narrower than the bar still leaves it at the margin', () => {
  const at = tableToolbarPlacement({ left: 10, right: 200, top: 400, bottom: 500 }, { width: 320, height: 600 })
  assert.equal(at.left, 8)
})
