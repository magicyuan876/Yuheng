import assert from 'node:assert/strict'
import { test } from 'vitest'

import enUS from '../../../i18n/locales/en-US'
import koKR from '../../../i18n/locales/ko-KR'
import ruRU from '../../../i18n/locales/ru-RU'
import zhCN from '../../../i18n/locales/zh-CN'
import { blockCommands, matchCommands, type BlockCommand, type CommandTarget } from './commands'

/** Labels stand in for translations; the last segment of the key will do. */
const label = (c: BlockCommand) => c.labelKey.split('.').pop()!

const catalogue = blockCommands()

test('every entry is distinct and describes itself completely', () => {
  const ids = new Set<string>()
  for (const command of catalogue) {
    assert.ok(!ids.has(command.id), `${command.id} appears twice`)
    ids.add(command.id)
    assert.ok(command.labelKey.startsWith('docs.commands.'), command.id)
    assert.ok(command.icon !== '', command.id)
    assert.ok(command.keywords.length > 0, `${command.id} would only be findable by its label`)
  }
})

test('an entry can be found by a word in either language', () => {
  for (const command of catalogue) {
    const chinese = command.keywords.filter((k) => /[一-鿿]/.test(k))
    assert.ok(chinese.length > 0, `${command.id} has no Chinese keyword`)
  }
})

test('turning a feature off takes it out of the menu', () => {
  const none = blockCommands({ embeds: false, drawings: false }).map((c) => c.id)
  assert.ok(!none.includes('embed'))
  assert.ok(!none.includes('whiteboard'))
  assert.ok(none.includes('table'), 'the rest of the menu is unaffected')
})

// draw.io is a separate service a deployment may not run, so its entry stays
// out of the menu until there is one. Inserting it without one would leave a
// diagram nobody can open.
test('the diagram editor is offered only where one is configured', () => {
  assert.ok(!blockCommands().some((c) => c.id === 'diagram'), 'not offered by default')
  assert.ok(!blockCommands({ drawings: true }).some((c) => c.id === 'diagram'),
    'and not merely because the whiteboard is on')
  assert.ok(blockCommands({ drawio: true }).some((c) => c.id === 'diagram'))
})

// The two are different tools for different jobs, not two skins on one, so
// both stay reachable where both are available — and each has to insert its
// own kind of drawing.
// Two entries reading the same word is the bug this menu already had once:
// a Mermaid block and a draw.io diagram both came out as "Diagram", and in
// Chinese both as "流程图". Distinct ids and distinct keys do not catch it —
// only the translated words do, which is why this reads the locales.
test('no two entries in the menu read the same in any language', () => {
  const locales: Array<[string, unknown]> = [
    ['en-US', enUS], ['zh-CN', zhCN], ['ko-KR', koKR], ['ru-RU', ruRU],
  ]
  // Everything switched on, so no entry escapes the check.
  const all = blockCommands({ embeds: true, drawings: true, drawio: true, copyBlockRef: () => {} })

  for (const [name, bundle] of locales) {
    const seen = new Map<string, string>()
    for (const command of all) {
      const text = command.labelKey.split('.').reduce<unknown>(
        (node, key) => (node as Record<string, unknown> | undefined)?.[key], bundle)
      assert.equal(typeof text, 'string', `${name} has no ${command.labelKey}`)

      const clash = seen.get(text as string)
      assert.equal(clash, undefined,
        `${name}: "${text as string}" labels both ${clash} and ${command.id}`)
      seen.set(text as string, command.id)
    }
  }
})

test('the whiteboard and the diagram editor each insert their own node', () => {
  const inserted = (id: string) => {
    const calls: string[] = []
    const chain: Record<string, unknown> = new Proxy({}, {
      get: (_t, name: string) => () => {
        calls.push(name)
        return chain
      },
    })
    blockCommands({ drawings: true, drawio: true }).find((c) => c.id === id)!
      .run({ chain: () => chain as never }, { from: 0, to: 3 })
    return calls
  }

  assert.ok(inserted('whiteboard').includes('insertExcalidraw'))
  assert.ok(!inserted('whiteboard').includes('insertDrawio'))
  assert.ok(inserted('diagram').includes('insertDrawio'))
  assert.ok(!inserted('diagram').includes('insertExcalidraw'))
})

// Offered only when the caller supplied the action, because a page with no id
// yet has no block anybody could point at.
test('the block-reference entry appears only when there is something to copy', () => {
  assert.ok(!blockCommands().some((c) => c.id === 'copyBlockRef'))

  let copied = 0
  const withCopy = blockCommands({ copyBlockRef: () => { copied++ } })
  const entry = withCopy.find((c) => c.id === 'copyBlockRef')
  assert.ok(entry, 'offered once there is a page to point at')

  const { calls, editor } = recordingEditor()
  entry.run(editor, { from: 2, to: 6 })

  assert.equal(copied, 1)
  assert.equal(calls[0]?.name, 'focus')
  assert.equal(calls[1]?.name, 'deleteRange', 'the typed query goes before the link is copied')
  assert.deepEqual(calls[1]?.args, [{ from: 2, to: 6 }])
  assert.equal(calls.at(-1)?.name, 'run')
})

test('an empty query offers the whole menu in its own order', () => {
  const all = matchCommands(catalogue, '', label)
  assert.deepEqual(all.map((c) => c.id), catalogue.map((c) => c.id))
  assert.notEqual(all, catalogue, 'and does not hand out the catalogue itself to be mutated')
})

test('whitespace alone is still an empty query', () => {
  assert.equal(matchCommands(catalogue, '   ', label).length, catalogue.length)
})

// The point of the ranking: the entry whose name begins with what was typed
// is nearly always the one that was meant. Asserted against a catalogue made
// here, so the four ranks are visible in one place and adding a real entry
// cannot quietly change what this proves.
test('the four ways of matching are ranked best first', () => {
  const entry = (id: string, keywords: string[]): BlockCommand => ({
    ...catalogue[0]!, id, labelKey: `docs.commands.${id}`, keywords,
  })
  const synthetic = [
    entry('keywordPart', ['xxaxx']),
    entry('labelPart', ['nothing']),
    entry('keywordPrefix', ['axx']),
    entry('labelPrefix', ['nothing']),
  ]
  const labels: Record<string, string> = {
    keywordPart: 'zzz', labelPart: 'zzazz', keywordPrefix: 'zzz', labelPrefix: 'azz',
  }
  const ids = matchCommands(synthetic, 'a', (c) => labels[c.id]!).map((c) => c.id)
  assert.deepEqual(ids, ['labelPrefix', 'keywordPrefix', 'labelPart', 'keywordPart'])
})

test('a keyword beats a coincidental match inside another name', () => {
  const ids = matchCommands(catalogue, 'list', label).map((c) => c.id)
  // bulletList and orderedList both name "list" as a keyword; taskList only
  // happens to contain it.
  assert.ok(ids.indexOf('bulletList') < ids.indexOf('taskList'), ids.join(', '))
  assert.ok(ids.indexOf('orderedList') < ids.indexOf('taskList'), ids.join(', '))
})

test('a keyword finds an entry its label would not', () => {
  const ids = matchCommands(catalogue, 'h1', label).map((c) => c.id)
  assert.equal(ids[0], 'heading1', ids.join(', '))
})

test('matching ignores case', () => {
  assert.deepEqual(
    matchCommands(catalogue, 'TABLE', label).map((c) => c.id),
    matchCommands(catalogue, 'table', label).map((c) => c.id),
  )
})

test('a Chinese query finds the entry while the interface is in English', () => {
  const ids = matchCommands(catalogue, '表格', label).map((c) => c.id)
  assert.deepEqual(ids, ['table'])
})

test('a query nothing answers to produces an empty menu, not the whole one', () => {
  assert.deepEqual(matchCommands(catalogue, 'zzzznothing', label), [])
})

test('entries that match equally well keep catalogue order', () => {
  const twins: BlockCommand[] = [
    { ...catalogue[0]!, id: 'a', keywords: ['same'] },
    { ...catalogue[0]!, id: 'b', keywords: ['same'] },
  ]
  assert.deepEqual(matchCommands(twins, 'same', () => 'x').map((c) => c.id), ['a', 'b'])
})

/** Records what a command did to a chain instead of doing it. */
function recordingEditor() {
  const calls: { name: string; args: unknown[] }[] = []
  const chain: Record<string, (...args: unknown[]) => unknown> = new Proxy({}, {
    get: (_t, name: string) => (...args: unknown[]) => {
      calls.push({ name, args })
      return chain
    },
  }) as never
  return { calls, editor: { chain: () => chain } as unknown as CommandTarget }
}

// Every entry must remove what was typed. An image inserted after a stray
// "/im" is the bug this exists to prevent, and it is easy to reintroduce by
// adding one entry that forgets.
test('every entry deletes the typed query before it inserts anything', () => {
  for (const command of catalogue) {
    const { calls, editor } = recordingEditor()
    command.run(editor, { from: 4, to: 7 })

    assert.equal(calls[0]?.name, 'focus', command.id)
    assert.equal(calls[1]?.name, 'deleteRange', command.id)
    assert.deepEqual(calls[1]?.args, [{ from: 4, to: 7 }], command.id)
    assert.ok(calls.length >= 3, `${command.id} does nothing after deleting the query`)
    assert.equal(calls.at(-1)?.name, 'run', `${command.id} never runs its chain`)
  }
})

test('each entry calls the editor command its name implies', () => {
  const expected: Record<string, string> = {
    paragraph: 'setParagraph',
    heading1: 'toggleHeading',
    bulletList: 'toggleBulletList',
    codeBlock: 'toggleCodeBlock',
    table: 'insertTable',
    callout: 'setCallout',
    columns: 'insertColumns',
    divider: 'setHorizontalRule',
    toc: 'insertTableOfContents',
    mermaid: 'insertMermaid',
  }
  for (const [id, name] of Object.entries(expected)) {
    const command = catalogue.find((c) => c.id === id)
    assert.ok(command, id)
    const { calls, editor } = recordingEditor()
    command.run(editor, { from: 0, to: 1 })
    assert.equal(calls[2]?.name, name, id)
  }
})
