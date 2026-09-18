import assert from 'node:assert/strict'
import { test } from 'node:test'

import { BLOCK_REF_PATH, formatBlockRefLink, parseBlockRefLink } from './blockRefLink'

test('a copied link carries both halves of the address', () => {
  const link = formatBlockRefLink({ pageId: 'page-1', blockId: 'blockaa' }, 'https://docs.test')
  assert.equal(link, `https://docs.test${BLOCK_REF_PATH}/page-1/blockaa`)
})

test('a link with no origin is a path, which still pastes', () => {
  assert.equal(formatBlockRefLink({ pageId: 'p', blockId: 'b' }), `${BLOCK_REF_PATH}/p/b`)
})

test('a trailing slash on the origin does not double up', () => {
  assert.equal(
    formatBlockRefLink({ pageId: 'p', blockId: 'b' }, 'https://docs.test/'),
    `https://docs.test${BLOCK_REF_PATH}/p/b`,
  )
})

test('half an address is not a link', () => {
  assert.equal(formatBlockRefLink({ pageId: '', blockId: 'b' }), '')
  assert.equal(formatBlockRefLink({ pageId: 'p', blockId: '' }), '')
})

test('what is copied is what is read back', () => {
  const ref = { pageId: 'page-1', blockId: 'blockaa' }
  assert.deepEqual(parseBlockRefLink(formatBlockRefLink(ref, 'https://docs.test')), ref)
  assert.deepEqual(parseBlockRefLink(formatBlockRefLink(ref)), ref)
})

test('a link is read back whatever host it was copied from', () => {
  for (const origin of ['https://docs.test', 'http://localhost:5173', 'https://a.b.c.example']) {
    assert.deepEqual(
      parseBlockRefLink(`${origin}${BLOCK_REF_PATH}/p1/blockaa`),
      { pageId: 'p1', blockId: 'blockaa' },
      origin,
    )
  }
})

test('surrounding whitespace from a clipboard is ignored', () => {
  assert.deepEqual(
    parseBlockRefLink(`  ${BLOCK_REF_PATH}/p1/blockaa\n`),
    { pageId: 'p1', blockId: 'blockaa' },
  )
})

test('ids that needed escaping survive the round trip', () => {
  const ref = { pageId: 'page/one', blockId: 'block aa' }
  assert.deepEqual(parseBlockRefLink(formatBlockRefLink(ref, 'https://docs.test')), ref)
})

// Pasting is how text arrives from everywhere. A loose match here would turn
// somebody's ordinary link into a quotation of a block they never chose.
test('anything that is not exactly this kind of link is refused', () => {
  for (const text of [
    '',
    '   ',
    'just some words',
    'https://docs.test/docs/p/page-1',           // an ordinary page link
    'https://docs.test/docs/block/p1',           // only half an address
    'https://docs.test/docs/block/p1/b/extra',   // something follows it
    'https://docs.test/other/block/p1/b',        // a different path
    '/docs/block/p1/b and then some words',      // not a link on its own
    'docs/block/p1/b',                           // neither absolute nor a URL
    'ftp://docs.test/docs/block/p1/b',           // not a scheme a browser follows here
    'https://docs.test/docs/block//b',           // an empty page id
    'https://docs.test/docs/block/p1/',          // an empty block id
  ]) {
    assert.equal(parseBlockRefLink(text), null, JSON.stringify(text))
  }
})

test('a malformed escape is not treated as an address', () => {
  assert.equal(parseBlockRefLink(`${BLOCK_REF_PATH}/%E0%A4%A/blockaa`), null)
})

test('a link that is not a URL at all does not throw', () => {
  assert.doesNotThrow(() => parseBlockRefLink('https://'))
  assert.equal(parseBlockRefLink('https://'), null)
})
