import assert from 'node:assert/strict'
import { test } from 'vitest'

import {
  CALLOUT_INPUT, calloutKindFromMatch, COLUMNS_INPUT, columnCountFromMatch,
  MATH_BLOCK_INPUT, MATH_INLINE_INPUT, MERMAID_INPUT, PAGE_BREAK_INPUT,
} from './inputRules'

/** What Tiptap hands a rule: the text of the block up to the cursor. */
const fires = (pattern: RegExp, text: string) => pattern.test(text)

test('a callout is written the way Markdown dialects write one', () => {
  for (const text of [':::' + ' ', ':::info ', ':::warning ', ':::danger ', ':::tip ', ':::success ']) {
    assert.ok(fires(CALLOUT_INPUT, text), JSON.stringify(text))
  }
})

test('a callout does not fire on prose that happens to contain colons', () => {
  for (const text of [
    'see:::this ',       // not at the start of the block
    ':::unknown ',       // not a kind this editor has
    ':::info',           // no trailing space: still being typed
    '::: extra words ',  // the marker has already been left behind
    ':: ',
    '::::',
  ]) {
    assert.ok(!fires(CALLOUT_INPUT, text), JSON.stringify(text))
  }
})

test('a callout with no kind named gets the default, and note means info', () => {
  assert.equal(calloutKindFromMatch(CALLOUT_INPUT.exec(':::' + ' ')!), 'info')
  assert.equal(calloutKindFromMatch(CALLOUT_INPUT.exec(':::note ')!), 'info')
  assert.equal(calloutKindFromMatch(CALLOUT_INPUT.exec(':::warning ')!), 'warning')
})

test('columns can be written with or without a count', () => {
  assert.ok(fires(COLUMNS_INPUT, ':::columns '))
  assert.ok(fires(COLUMNS_INPUT, ':::columns3 '))
  assert.ok(!fires(COLUMNS_INPUT, ':::columns9 '), 'nine columns is not a layout the schema has')
})

test('a column count is clamped to what the schema will accept', () => {
  assert.equal(columnCountFromMatch(COLUMNS_INPUT.exec(':::columns ')!), 2)
  assert.equal(columnCountFromMatch(COLUMNS_INPUT.exec(':::columns5 ')!), 5)
  assert.equal(columnCountFromMatch(COLUMNS_INPUT.exec(':::columns2 ')!), 2)
})

test('a display formula fires on the space after the delimiters', () => {
  assert.ok(fires(MATH_BLOCK_INPUT, '$$ '))
  assert.ok(!fires(MATH_BLOCK_INPUT, '$$'), 'still being typed')
  assert.ok(!fires(MATH_BLOCK_INPUT, 'cost: $$ '), 'not at the start of the block')
})

// The false positives that matter. Note what is deliberately absent: a line
// ending "$6$" does become a formula, and should — that is exactly how a
// formula is written, and no pattern can tell it apart from a second price
// without reading the sentence.
test('an inline formula is not found where nobody wrote one', () => {
  for (const text of [
    'a $ and another $',   // nothing between the delimiters but words and spaces
    '$ x $',               // padded, so the delimiters are prose
    'word$x$',             // no boundary before the opening delimiter
    'it cost $5 and $6',   // unclosed: still being typed
    '$$',
  ]) {
    assert.ok(!MATH_INLINE_INPUT.test(text), JSON.stringify(text))
  }
})

test('an inline formula is found where somebody meant one', () => {
  for (const [text, body] of [
    ['$x$', 'x'],
    ['the value $x^2$', 'x^2'],
    ['(see $a+b$', 'a+b'],
    ['$E = mc^2$', 'E = mc^2'],
  ] as const) {
    const match = MATH_INLINE_INPUT.exec(text)
    assert.ok(match, JSON.stringify(text))
    assert.equal(match[1], body, JSON.stringify(text))
  }
})

test('a diagram is written as a fenced block naming mermaid', () => {
  assert.ok(fires(MERMAID_INPUT, '```mermaid '))
  assert.ok(!fires(MERMAID_INPUT, '```js '), 'an ordinary fence is the code block extension’s')
  assert.ok(!fires(MERMAID_INPUT, '```mermaid'))
})

test('a page break has its own marker, because --- is already a rule', () => {
  assert.ok(fires(PAGE_BREAK_INPUT, '+++ '))
  assert.ok(!fires(PAGE_BREAK_INPUT, '--- '))
  assert.ok(!fires(PAGE_BREAK_INPUT, '++ '))
})

// Every pattern is used in a fresh `exec` on each keystroke; a global flag
// would make the next call start from the last match and silently skip.
test('no pattern is global or sticky', () => {
  for (const [name, pattern] of Object.entries({
    CALLOUT_INPUT, COLUMNS_INPUT, MATH_BLOCK_INPUT, MATH_INLINE_INPUT,
    MERMAID_INPUT, PAGE_BREAK_INPUT,
  })) {
    assert.equal(pattern.global, false, name)
    assert.equal(pattern.sticky, false, name)
  }
})
