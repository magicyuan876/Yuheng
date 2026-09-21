import { test } from 'vitest'
import assert from 'node:assert/strict'

import { highlight, splitTerms, type Segment } from './highlight.ts'

const marked = (segments: Segment[]) => segments.filter((s) => s.match).map((s) => s.text)
const joined = (segments: Segment[]) => segments.map((s) => s.text).join('')

test('text with no query is one unmarked run', () => {
  assert.deepEqual(highlight('配额说明', ''), [{ text: '配额说明', match: false }])
})

test('the matched run is marked', () => {
  const out = highlight('每个空间的配额由管理员设置', '配额')
  assert.deepEqual(marked(out), ['配额'])
})

// Nothing may be lost or duplicated: the segments are the text.
test('segments reassemble into the original text', () => {
  for (const [text, query] of [
    ['每个空间的配额由管理员设置', '配额'],
    ['quota and limit and quota', 'quota limit'],
    ['nothing matches here', '配额'],
    ['', '配额'],
  ] as const) {
    assert.equal(joined(highlight(text, query)), text, `${text} / ${query}`)
  }
})

test('every occurrence is marked, not just the first', () => {
  const out = highlight('配额说明与配额上限', '配额')
  assert.deepEqual(marked(out), ['配额', '配额'])
})

test('several terms are all marked', () => {
  const out = highlight('the quota and the limit', 'quota limit')
  assert.deepEqual(marked(out), ['quota', 'limit'])
})

test('matching folds case for latin text', () => {
  const out = highlight('The QUOTA applies', 'quota')
  assert.deepEqual(marked(out), ['QUOTA'])
})

// A segment cannot be inside another one, so overlaps merge.
test('overlapping terms merge into one run', () => {
  const out = highlight('abcdef', 'abc bcd')
  assert.deepEqual(marked(out), ['abcd'])
  assert.equal(joined(out), 'abcdef')
})

test('a term that is not there marks nothing', () => {
  const out = highlight('配额说明', '完全不相干')
  assert.deepEqual(marked(out), [])
  assert.equal(joined(out), '配额说明')
})

test('empty text stays empty', () => {
  assert.deepEqual(highlight('', '配额'), [])
})

// A Chinese query has no spaces and is one term; splitting it per character
// would mark every character and highlight nothing useful.
test('a chinese query is one term', () => {
  assert.deepEqual(splitTerms('空间配额'), ['空间配额'])
})

test('spaces separate terms and blanks are dropped', () => {
  assert.deepEqual(splitTerms('  quota   limit  '), ['quota', 'limit'])
  assert.deepEqual(splitTerms('   '), [])
})
