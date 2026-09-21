import assert from 'node:assert/strict'
import { test } from 'vitest'

import { getSchema } from '@tiptap/core'
import { Node as PMNode } from '@tiptap/pm/model'

import { officialExtensions } from './extensions'
import { extractHeadings } from './toc'

const schema = getSchema(officialExtensions())

function doc(content: unknown[]): PMNode {
  return PMNode.fromJSON(schema, { type: 'doc', content })
}

test('headings are collected in document order with their level and text', () => {
  const d = doc([
    { type: 'heading', attrs: { level: 1, id: 'h1' }, content: [{ type: 'text', text: 'Intro' }] },
    { type: 'paragraph', content: [{ type: 'text', text: 'body text' }] },
    { type: 'heading', attrs: { level: 2, id: 'h2' }, content: [{ type: 'text', text: 'Details' }] },
  ])
  const toc = extractHeadings(d)
  assert.deepEqual(toc.map((e) => [e.level, e.text, e.id]), [
    [1, 'Intro', 'h1'],
    [2, 'Details', 'h2'],
  ])
})

test('a heading nested inside a blockquote or list is still found', () => {
  const d = doc([
    {
      type: 'blockquote',
      content: [{ type: 'heading', attrs: { level: 3, id: 'nested' }, content: [{ type: 'text', text: 'Nested' }] }],
    },
  ])
  assert.deepEqual(extractHeadings(d).map((e) => e.text), ['Nested'])
})

test('an empty document has no headings', () => {
  const d = doc([{ type: 'paragraph' }])
  assert.deepEqual(extractHeadings(d), [])
})

test('a heading with marks in its text reports the plain text content', () => {
  const d = doc([{
    type: 'heading',
    attrs: { level: 1, id: 'h1' },
    content: [
      { type: 'text', text: 'Bold ' },
      { type: 'text', text: 'word', marks: [{ type: 'bold' }] },
    ],
  }])
  assert.deepEqual(extractHeadings(d).map((e) => e.text), ['Bold word'])
})

test('a heading missing an id (not yet assigned by BlockId) reports an empty one, not a crash', () => {
  const d = doc([{ type: 'heading', attrs: { level: 1 }, content: [{ type: 'text', text: 'No id yet' }] }])
  assert.deepEqual(extractHeadings(d), [{ id: '', level: 1, text: 'No id yet', pos: 0 }])
})
