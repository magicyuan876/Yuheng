import assert from 'node:assert/strict'
import { test } from 'node:test'

import { ReactIsland, type IslandRoot } from './reactIsland'

/** Records what React would have been asked to do. */
function fakeReact() {
  const roots: { container: Element; rendered: unknown[]; unmounted: number }[] = []
  const createRoot = (container: Element): IslandRoot => {
    const record = { container, rendered: [] as unknown[], unmounted: 0 }
    roots.push(record)
    return {
      render: (element: unknown) => record.rendered.push(element),
      unmount: () => {
        record.unmounted++
      },
    }
  }
  return { createRoot, roots }
}

/** Stands in for a DOM element without needing a DOM. */
function element(name: string): Element {
  return { name } as unknown as Element
}

test('a root is created once and reused for further renders', () => {
  const react = fakeReact()
  const island = new ReactIsland(react.createRoot)
  const container = element('a')

  island.render(container, 'first')
  island.render(container, 'second')

  assert.equal(react.roots.length, 1, 'rendering again must not create a second root')
  assert.deepEqual(react.roots[0]!.rendered, ['first', 'second'])
  assert.equal(island.mounted, true)
})

// This is the acceptance criterion: a root that is never unmounted keeps its
// whole tree alive for as long as the page is open.
test('unmounting tears the subtree down exactly once', () => {
  const react = fakeReact()
  const island = new ReactIsland(react.createRoot)
  island.render(element('a'), 'x')

  island.unmount()
  assert.equal(react.roots[0]!.unmounted, 1)
  assert.equal(island.mounted, false)

  // A component's teardown may run twice, or re-entrantly from React's own.
  island.unmount()
  island.unmount()
  assert.equal(react.roots[0]!.unmounted, 1, 'the same root must not be unmounted again')
})

test('unmounting before anything was mounted is harmless', () => {
  const react = fakeReact()
  const island = new ReactIsland(react.createRoot)
  island.unmount()
  assert.equal(react.roots.length, 0)
  assert.equal(island.mounted, false)
})

test('moving to a different container unmounts the old one first', () => {
  const react = fakeReact()
  const island = new ReactIsland(react.createRoot)

  island.render(element('a'), 'x')
  island.render(element('b'), 'y')

  assert.equal(react.roots.length, 2)
  assert.equal(react.roots[0]!.unmounted, 1, 'the abandoned container must not keep its tree')
  assert.equal(react.roots[1]!.unmounted, 0)
  assert.deepEqual(react.roots[1]!.rendered, ['y'])
})

test('a root can be mounted again after being unmounted', () => {
  const react = fakeReact()
  const island = new ReactIsland(react.createRoot)
  const container = element('a')

  island.render(container, 'x')
  island.unmount()
  island.render(container, 'y')

  assert.equal(react.roots.length, 2, 'a fresh root, not the one that was torn down')
  assert.deepEqual(react.roots[1]!.rendered, ['y'])
  assert.equal(island.mounted, true)
})

// The editor is imported dynamically, so its module can resolve after the
// component that asked for it has already gone away.
test('a render after destroy does nothing at all', () => {
  const react = fakeReact()
  const island = new ReactIsland(react.createRoot)
  island.render(element('a'), 'x')

  island.destroy()
  assert.equal(react.roots[0]!.unmounted, 1)
  assert.equal(island.isDestroyed, true)

  island.render(element('a'), 'late')
  assert.equal(react.roots.length, 1, 'a late import must not mount into a dead component')
  assert.equal(island.mounted, false)
})

test('destroying twice is harmless', () => {
  const react = fakeReact()
  const island = new ReactIsland(react.createRoot)
  island.render(element('a'), 'x')
  island.destroy()
  island.destroy()
  assert.equal(react.roots[0]!.unmounted, 1)
})

// React refuses to unmount during its own render. The island has already
// forgotten the root by then, so the tree is collectable and there is nothing
// to report.
test('a root that refuses to unmount is still let go of', () => {
  let attempts = 0
  const island = new ReactIsland(() => ({
    render: () => {},
    unmount: () => {
      attempts++
      throw new Error('cannot unmount while rendering')
    },
  }))

  island.render(element('a'), 'x')
  assert.doesNotThrow(() => island.unmount())
  assert.equal(attempts, 1)
  assert.equal(island.mounted, false, 'the reference is dropped whether or not React co-operated')
})
