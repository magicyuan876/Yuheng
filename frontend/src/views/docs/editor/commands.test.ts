import assert from 'node:assert/strict'
import { test } from 'node:test'

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
  assert.ok(!none.includes('excalidraw'))
  assert.ok(none.includes('table'), 'the rest of the menu is unaffected')
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
