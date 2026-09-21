import assert from 'node:assert/strict'
import { test } from 'vitest'

import { getSchema } from '@tiptap/core'

import { officialExtensions } from '../editor/extensions'

import {
  findQuotation, isInline, MIN_QUOTE_LENGTH, normalise,
  partitionPlacements, placeComment, placeComments,
  type Placement, type ResolveAnchor,
} from './placement'

const schema = getSchema(officialExtensions())

function docOf(...paragraphs: string[]) {
  return schema.nodeFromJSON({
    type: 'doc',
    content: paragraphs.map((text) => ({
      type: 'paragraph',
      content: text ? [{ type: 'text', text }] : undefined,
    })),
  })
}

/** A resolver that always answers with the same range. */
const resolvesTo = (from: number, to: number): ResolveAnchor => () => ({ from, to })
/** A resolver for a position that no longer exists. */
const resolvesToNothing: ResolveAnchor = () => null

const anchored = { id: 'c1', anchor: { start: {}, end: {} }, quotedText: 'the passage' }

test('a comment with no anchor is about the page', () => {
  const doc = docOf('some text')
  assert.deepEqual(
    placeComment(doc, { id: 'c1' }, resolvesTo(1, 5)),
    { id: 'c1', kind: 'page' },
  )
  assert.deepEqual(
    placeComment(doc, { id: 'c1', anchor: null }, resolvesTo(1, 5)),
    { id: 'c1', kind: 'page' },
  )
})

// Acceptance (T3.2): somebody inserting several paragraphs before the
// commented text leaves the comment on the same words. That is what the
// relative position is for, and this is the case where it works.
test('a resolvable position is used as it is', () => {
  const doc = docOf('first', 'the passage being discussed')
  const placement = placeComment(doc, anchored, resolvesTo(8, 19))

  assert.equal(placement.kind, 'anchored')
  assert.equal(placement.from, 8)
  assert.equal(placement.to, 19)
})

// The position is the only thing that knows the difference between two
// identical sentences, so it is trusted when it works — the quotation is a
// fallback, not a cross-check.
test('the quotation is not consulted while the position resolves', () => {
  const doc = docOf('the passage', 'the passage')
  const placement = placeComment(doc, anchored, resolvesTo(14, 25))

  assert.equal(placement.kind, 'anchored', 'even though the quotation is ambiguous')
  assert.equal(placement.from, 14)
})

test('a position that no longer resolves falls back to the quotation', () => {
  const doc = docOf('before', 'the passage being discussed')
  const placement = placeComment(doc, anchored, resolvesToNothing)

  assert.equal(placement.kind, 'quoted')
  const covered = doc.textBetween(placement.from!, placement.to!)
  assert.equal(covered, 'the passage')
})

// Acceptance (T3.2): once the text is gone, the comment says so rather than
// pointing somewhere else.
test('a comment whose text has been deleted is orphaned', () => {
  const doc = docOf('nothing here resembles it')
  const placement = placeComment(doc, anchored, resolvesToNothing)

  assert.equal(placement.kind, 'orphaned')
  assert.equal(placement.from, undefined)
})

// Pointing at the wrong paragraph is worse than admitting the place was lost:
// the reader would see a remark about text it was never about, with nothing
// to suggest anything went wrong.
test('a quotation that matches twice orphans the comment rather than guessing', () => {
  const doc = docOf('the passage', 'and again the passage')
  assert.equal(placeComment(doc, anchored, resolvesToNothing).kind, 'orphaned')
})

test('a quotation matching twice inside one paragraph is also ambiguous', () => {
  const doc = docOf('the passage and the passage')
  assert.equal(placeComment(doc, anchored, resolvesToNothing).kind, 'orphaned')
})

// A range that collapsed to nothing no longer covers a passage.
test('a position that collapsed to a point is treated as lost', () => {
  const doc = docOf('the passage being discussed')
  const placement = placeComment(doc, anchored, resolvesTo(5, 5))
  assert.equal(placement.kind, 'quoted', 'and the quotation answers instead')
})

test('a position outside the document is refused', () => {
  const doc = docOf('short')
  assert.equal(placeComment(doc, anchored, resolvesTo(0, 9999)).kind, 'orphaned')
  assert.equal(placeComment(doc, anchored, resolvesTo(-5, 3)).kind, 'orphaned')
  assert.equal(placeComment(doc, anchored, () => ({ from: NaN, to: 3 })).kind, 'orphaned')
})

// y-prosemirror throws rather than returning null for some shapes of stale
// position; a sidebar that disappears over one old comment is a poor trade.
test('a resolver that throws is a resolver that said no', () => {
  const doc = docOf('the passage being discussed')
  const placement = placeComment(doc, anchored, () => {
    throw new Error('stale')
  })
  assert.equal(placement.kind, 'quoted')
})

// The server collapses whitespace when it stores a quotation, so the fallback
// survives an edit that only re-wrapped a paragraph.
test('the quotation is matched with whitespace collapsed', () => {
  const doc = docOf('the   passage\nbeing discussed')
  const placement = placeComment(
    doc,
    { id: 'c1', anchor: {}, quotedText: 'the passage being' },
    resolvesToNothing,
  )
  assert.equal(placement.kind, 'quoted')
})

test('a quotation too short to identify anything is not searched for', () => {
  const doc = docOf('a a a a a')
  for (const quote of ['', ' ', 'a', 'ab']) {
    assert.equal(
      placeComment(doc, { id: 'c1', anchor: {}, quotedText: quote }, resolvesToNothing).kind,
      'orphaned',
      JSON.stringify(quote),
    )
  }
  assert.ok(MIN_QUOTE_LENGTH >= 4)
})

test('a comment with no quotation at all is orphaned once its position goes', () => {
  const doc = docOf('some text')
  assert.equal(placeComment(doc, { id: 'c1', anchor: {} }, resolvesToNothing).kind, 'orphaned')
})

test('findQuotation reports the range the words occupy', () => {
  const doc = docOf('alpha beta gamma')
  const found = findQuotation(doc, 'beta')
  assert.ok(found)
  assert.equal(doc.textBetween(found.from, found.to), 'beta')
})

test('findQuotation looks inside nested blocks', () => {
  const doc = schema.nodeFromJSON({
    type: 'doc',
    content: [{
      type: 'blockquote',
      content: [{ type: 'paragraph', content: [{ type: 'text', text: 'quoted words here' }] }],
    }],
  })
  const found = findQuotation(doc, 'quoted words')
  assert.ok(found)
  assert.equal(doc.textBetween(found.from, found.to), 'quoted words')
})

test('placing many comments answers for each of them', () => {
  const doc = docOf('the passage being discussed')
  const placements = placeComments(doc, [
    { id: 'page-level' },
    { id: 'lost', anchor: {}, quotedText: 'not in this document' },
    { id: 'by-quote', anchor: {}, quotedText: 'the passage' },
  ], resolvesToNothing)

  assert.deepEqual(placements.map((p) => p.kind), ['page', 'orphaned', 'quoted'])
})

test('normalise matches the server, so both sides find the same text', () => {
  assert.equal(normalise('  one\n\ttwo   three  '), 'one two three')
  assert.equal(normalise(''), '')
})

test('isInline says which placements are in the text', () => {
  const kinds: Placement[] = [
    { id: 'a', kind: 'anchored', from: 1, to: 2 },
    { id: 'b', kind: 'quoted', from: 3, to: 4 },
    { id: 'c', kind: 'page' },
    { id: 'd', kind: 'orphaned' },
  ]
  assert.deepEqual(kinds.map(isInline), [true, true, false, false])
})

// A sidebar shows comments in the order the passages they are about appear,
// and the ones that lost their place together rather than scattered.
test('placements are partitioned and the inline ones ordered by position', () => {
  const { inline, page, orphaned } = partitionPlacements([
    { id: 'second', kind: 'anchored', from: 50, to: 60 },
    { id: 'page', kind: 'page' },
    { id: 'first', kind: 'quoted', from: 10, to: 20 },
    { id: 'lost', kind: 'orphaned' },
  ])

  assert.deepEqual(inline.map((p) => p.id), ['first', 'second'])
  assert.deepEqual(page.map((p) => p.id), ['page'])
  assert.deepEqual(orphaned.map((p) => p.id), ['lost'])
})
