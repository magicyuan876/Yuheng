import assert from 'node:assert/strict'
import { test } from 'node:test'

import { dropPositionFor, PageTreeModel, pageSlug, shortIdFromSlug, type TreeNodeData } from './pageTree'

function node(id: string, parent: string | null, position: string, extra: Partial<TreeNodeData> = {}): TreeNodeData {
  return {
    id, short_id: id.padEnd(10, '0').slice(0, 10), space_id: 's1', parent_id: parent, position,
    title: id.toUpperCase(), has_children: false, can_edit: true, ...extra,
  }
}

function titles(model: PageTreeModel): string[] {
  return model.rows().map((r) => `${'  '.repeat(r.depth)}${r.node.title}`)
}

test('rows follow position order and only expanded, loaded parents unfold', () => {
  const m = new PageTreeModel()
  m.setChildren(null, [node('b', null, 'a1'), node('a', null, 'a0', { has_children: true }), node('c', null, 'a2')])
  assert.deepEqual(titles(m), ['A', 'B', 'C'])

  m.expand('a')
  assert.deepEqual(titles(m), ['A', 'B', 'C'], 'children not loaded yet: nothing to show')
  m.setChildren('a', [node('a2', 'a', 'a1'), node('a1', 'a', 'a0')])
  assert.deepEqual(titles(m), ['A', '  A1', '  A2', 'B', 'C'])
  m.collapse('a')
  assert.deepEqual(titles(m), ['A', 'B', 'C'])
})

test('cursor pages accumulate and mark the parent loaded only when complete', () => {
  const m = new PageTreeModel()
  m.appendChildren(null, [node('a', null, 'a0'), node('b', null, 'a1')], false)
  assert.equal(m.isLoaded(null), false)
  m.appendChildren(null, [node('c', null, 'a2')], true)
  assert.equal(m.isLoaded(null), true)
  assert.deepEqual(m.childIds(null), ['a', 'b', 'c'])
})

test('insert, update, move and remove keep sibling order and has_children flags', () => {
  const m = new PageTreeModel()
  m.setChildren(null, [node('a', null, 'a0'), node('b', null, 'a1')])
  m.insert(node('mid', null, 'a0V'))
  assert.deepEqual(m.childIds(null), ['a', 'mid', 'b'])

  m.update('mid', { position: 'a2', title: 'Last' })
  assert.deepEqual(m.childIds(null), ['a', 'b', 'mid'])
  assert.equal(m.get('mid')?.title, 'Last')

  // Moving under a loaded parent lands in order; under an unloaded one it leaves the view.
  m.setChildren('a', [])
  m.move('mid', 'a', 'a0')
  assert.deepEqual(m.childIds('a'), ['mid'])
  assert.equal(m.get('a')?.has_children, true)
  m.move('mid', 'b', 'a0')
  assert.equal(m.has('mid'), false, 'b is not loaded, so the node is dropped from the view')
  assert.equal(m.get('b')?.has_children, true)
  assert.equal(m.get('a')?.has_children, false)

  // A node cannot be moved into its own subtree.
  m.setChildren('b', [node('mid', 'b', 'a0')])
  m.setChildren('mid', [node('deep', 'mid', 'a0')])
  m.move('b', 'deep', 'a0')
  assert.equal(m.get('b')?.parent_id, null)

  m.remove('b')
  assert.equal(m.has('deep'), false, 'subtree goes with the node')
  assert.deepEqual(m.childIds(null), ['a'])
})

test('reveal expands ancestors and dropTarget maps gestures to after_id', () => {
  const m = new PageTreeModel()
  m.setChildren(null, [node('a', null, 'a0', { has_children: true }), node('b', null, 'a1'), node('c', null, 'a2')])
  m.setChildren('a', [node('a1', 'a', 'a0', { has_children: true })])
  m.setChildren('a1', [node('a11', 'a1', 'a0')])
  m.reveal('a11')
  assert.equal(m.isExpanded('a'), true)
  assert.equal(m.isExpanded('a1'), true)
  assert.deepEqual(m.ancestorIds('a11'), ['a', 'a1'])

  assert.deepEqual(m.dropTarget('c', 'a', 'before'), { parentId: null, afterId: null })
  assert.deepEqual(m.dropTarget('c', 'a', 'after'), { parentId: null, afterId: 'a' })
  assert.deepEqual(m.dropTarget('c', 'a', 'inside'), { parentId: 'a', afterId: undefined })
  assert.deepEqual(m.dropTarget('a', 'c', 'before'), { parentId: null, afterId: 'b' })
  assert.deepEqual(m.dropTarget('b', 'c', 'before'), { parentId: null, afterId: 'a' },
    'the dragged node itself is skipped as previous sibling')
  assert.equal(m.dropTarget('a', 'a11', 'inside'), null, 'cannot drop into own subtree')
  assert.equal(m.dropTarget('a', 'a', 'after'), null)
})

test('server events keep the view in step', () => {
  const m = new PageTreeModel()
  m.setChildren(null, [node('a', null, 'a0'), node('b', null, 'a1')])
  const sid = 's1'

  assert.equal(m.applyEvent({
    type: 'docs.page.created', space_id: sid, page_id: 'n',
    payload: { parent_id: null, position: 'a0V', title: 'New', short_id: 'n000000000' },
  }, sid, true), true)
  assert.deepEqual(m.childIds(null), ['a', 'n', 'b'])
  assert.equal(m.applyEvent({ type: 'docs.page.created', space_id: 'other', page_id: 'x', payload: {} }, sid, true),
    false, 'other spaces are ignored')

  assert.equal(m.applyEvent({ type: 'docs.page.meta_updated', space_id: sid, page_id: 'n', payload: { title: 'Renamed' } },
    sid, true), true)
  assert.equal(m.get('n')?.title, 'Renamed')

  assert.equal(m.applyEvent({
    type: 'docs.page.moved', space_id: sid, page_id: 'n', payload: { parent_id: null, position: 'a2', cross_space: false },
  }, sid, true), true)
  assert.deepEqual(m.childIds(null), ['a', 'b', 'n'])

  // A page that left for another space disappears; orphans mark the root stale.
  assert.equal(m.applyEvent({
    type: 'docs.page.moved', space_id: sid, page_id: 'n',
    payload: { left: true, cross_space: true, orphaned: ['z'] },
  }, sid, true), true)
  assert.equal(m.has('n'), false)
  assert.equal(m.isStale(null), true)
  m.clearStale(null)

  // A page arriving from another space marks its destination stale.
  assert.equal(m.applyEvent({
    type: 'docs.page.moved', space_id: sid, page_id: 'incoming', payload: { parent_id: 'a', cross_space: true },
  }, sid, true), true)
  assert.equal(m.isStale('a'), true)
  assert.equal(m.get('a')?.has_children, true)

  assert.equal(m.applyEvent({ type: 'docs.page.deleted', space_id: sid, page_id: 'b', payload: {} }, sid, true), true)
  assert.deepEqual(m.childIds(null), ['a'])
  assert.equal(m.applyEvent({
    type: 'docs.page.restored', space_id: sid, page_id: 'b',
    payload: { parent_id: null, position: 'a1', title: 'B', short_id: 'b000000000', count: 3 },
  }, sid, true), true)
  assert.equal(m.get('b')?.has_children, true, 'restored with descendants')
  assert.equal(m.applyEvent({ type: 'docs.page.tree_rebalanced', space_id: sid, payload: { parent_id: null } }, sid, true),
    true)
  assert.equal(m.isStale(null), true)
  assert.equal(m.applyEvent({ type: 'docs.page.purged', space_id: sid, payload: { ids: ['a', 'nope'] } }, sid, true), true)
  assert.equal(m.has('a'), false)
})

test('page slugs carry the short id last and parse back', () => {
  assert.equal(pageSlug('Getting Started!', 'abc123def0'), 'getting-started-abc123def0')
  assert.equal(pageSlug('', 'abc123def0'), 'abc123def0')
  assert.equal(pageSlug('研发规范', 'abc123def0'), 'abc123def0', 'non-ASCII titles fall back to the id alone')
  assert.equal(shortIdFromSlug('getting-started-abc123def0'), 'abc123def0')
  assert.equal(shortIdFromSlug('abc123def0'), 'abc123def0')
  assert.equal(shortIdFromSlug('ABC123DEF0'), null)
  assert.equal(shortIdFromSlug('xabc123def0'), null, 'a missing separator is not a slug')
  assert.equal(shortIdFromSlug('short'), null)
})

test('dropPositionFor splits a row into before/inside/after', () => {
  assert.equal(dropPositionFor(2, 32, true), 'before')
  assert.equal(dropPositionFor(16, 32, true), 'inside')
  assert.equal(dropPositionFor(30, 32, true), 'after')
  assert.equal(dropPositionFor(12, 32, false), 'before')
  assert.equal(dropPositionFor(20, 32, false), 'after')
})
