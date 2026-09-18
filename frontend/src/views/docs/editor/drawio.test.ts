import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  decodeExport,
  drawioFrameURL,
  DrawioSession,
  EMPTY_DRAWIO_XML,
  isFromEditor,
  originOf,
  parseMessage,
} from './drawio'

/** A data URI carrying the given SVG, the way draw.io answers an export. */
function svgDataURI(svg: string): string {
  return 'data:image/svg+xml;base64,' + Buffer.from(svg, 'utf8').toString('base64')
}

test('the frame address puts the editor into the mode this integration drives', () => {
  const url = new URL(drawioFrameURL('https://draw.corp.example.test/', false))
  assert.equal(url.origin, 'https://draw.corp.example.test')
  assert.equal(url.searchParams.get('embed'), '1', 'without this it sends no messages at all')
  assert.equal(url.searchParams.get('proto'), 'json', 'and without this they are not parseable')
  assert.equal(url.searchParams.get('ui'), 'min')

  assert.equal(new URL(drawioFrameURL('https://draw.corp.example.test/', true)).searchParams.get('ui'), 'dark')
})

test('a query already on the configured address survives', () => {
  const url = new URL(drawioFrameURL('https://draw.corp.example.test/?lang=zh', false))
  assert.equal(url.searchParams.get('lang'), 'zh')
  assert.equal(url.searchParams.get('embed'), '1')
})

// This is the whole trust boundary: a window with an iframe in it receives
// messages from anything that can reach it.
test('only messages from the frame we opened are ours', () => {
  const frameWindow = {}
  const frame = { contentWindow: frameWindow }
  const origin = 'https://draw.corp.example.test'

  assert.equal(isFromEditor({ origin, source: frameWindow }, origin, frame), true)

  // Another origin, even saying the right thing.
  assert.equal(isFromEditor({ origin: 'https://evil.test', source: frameWindow }, origin, frame), false)
  // The right origin from a different frame on it.
  assert.equal(isFromEditor({ origin, source: {} }, origin, frame), false)
  // No frame open at all.
  assert.equal(isFromEditor({ origin, source: frameWindow }, origin, null), false)
  assert.equal(isFromEditor({ origin, source: frameWindow }, origin, { contentWindow: undefined }), false)
  // No expected origin means nothing is trusted, which is what an
  // unconfigured editor should amount to.
  assert.equal(isFromEditor({ origin, source: frameWindow }, '', frame), false)
})

test('an origin is derived from the configured address, and a bad one trusts nothing', () => {
  assert.equal(originOf('https://draw.corp.example.test/x?y=1'), 'https://draw.corp.example.test')
  assert.equal(originOf('not a url'), '')
  assert.equal(originOf(''), '')
})

test('anything unparseable is simply not for us', () => {
  assert.equal(parseMessage(''), null)
  assert.equal(parseMessage('not json'), null)
  assert.equal(parseMessage('"a string"'), null)
  assert.equal(parseMessage('null'), null)
  assert.equal(parseMessage(42), null)
  assert.deepEqual(parseMessage('{"event":"init"}'), { event: 'init' })
})

test('the editor is handed the drawing once it announces itself', () => {
  const session = new DrawioSession('<mxfile>original</mxfile>')
  assert.equal(session.initialised, false)

  const action = session.receive({ event: 'init' })
  assert.deepEqual(action, { kind: 'load', xml: '<mxfile>original</mxfile>' })
  assert.equal(session.initialised, true)

  const sent = DrawioSession.request(action)
  assert.ok(sent)
  const payload = JSON.parse(sent)
  assert.equal(payload.action, 'load')
  assert.equal(payload.xml, '<mxfile>original</mxfile>')
  assert.equal(payload.autosave, 0, 'saving is explicit, so the editor must not do it on its own')
})

test('a save asks for a rendering, and both are stored together', () => {
  const session = new DrawioSession(EMPTY_DRAWIO_XML)
  session.receive({ event: 'init' })

  const asked = session.receive({ event: 'save', xml: '<mxfile>edited</mxfile>' })
  assert.deepEqual(asked, { kind: 'export' })
  const request = JSON.parse(DrawioSession.request(asked)!)
  assert.equal(request.action, 'export')
  // The embedded form carries the source inside the rendering, which is what
  // lets a diagram be reopened from its preview if the source is ever lost.
  assert.equal(request.format, 'xmlsvg')

  const stored = session.receive({ event: 'export', data: svgDataURI('<svg>drawing</svg>') })
  assert.deepEqual(stored, { kind: 'save', xml: '<mxfile>edited</mxfile>', svg: '<svg>drawing</svg>' })
})

// A diagram with a source but no rendering would show as broken to every
// reader, which is worse than one that was never saved.
test('a save without a rendering stores nothing', () => {
  const session = new DrawioSession(EMPTY_DRAWIO_XML)
  session.receive({ event: 'init' })
  session.receive({ event: 'save', xml: '<mxfile>edited</mxfile>' })

  assert.deepEqual(session.receive({ event: 'export', data: '' }), { kind: 'none' })
  assert.deepEqual(session.receive({ event: 'export' }), { kind: 'none' })
})

test('an export that is not an SVG is refused rather than guessed at', () => {
  assert.equal(decodeExport('data:image/png;base64,iVBORw0KGgo='), '')
  assert.equal(decodeExport('data:text/html;base64,PHNjcmlwdD4='), '')
  assert.equal(decodeExport('https://evil.test/x.svg'), '')
  assert.equal(decodeExport('data:image/svg+xml;base64,!!!not base64!!!'), '')
  assert.equal(decodeExport(undefined), '')
  assert.equal(decodeExport(42), '')

  assert.equal(decodeExport(svgDataURI('<svg/>')), '<svg/>')
  // Non-ASCII survives the base64 round trip.
  assert.equal(decodeExport(svgDataURI('<svg><text>图</text></svg>')), '<svg><text>图</text></svg>')
})

test('closing the editor is its own outcome', () => {
  const session = new DrawioSession(EMPTY_DRAWIO_XML)
  session.receive({ event: 'init' })
  assert.deepEqual(session.receive({ event: 'exit' }), { kind: 'close' })
})

test('a message the integration does not use changes nothing', () => {
  const session = new DrawioSession(EMPTY_DRAWIO_XML)
  for (const event of ['configure', 'autosave', 'prompt', undefined, '']) {
    assert.deepEqual(session.receive({ event }), { kind: 'none' })
  }
  assert.equal(DrawioSession.request({ kind: 'none' }), null)
  assert.equal(DrawioSession.request({ kind: 'close' }), null)
})

test('a second save starts from what the first one stored', () => {
  const session = new DrawioSession('<mxfile>first</mxfile>')
  session.receive({ event: 'init' })
  session.receive({ event: 'save', xml: '<mxfile>second</mxfile>' })
  session.receive({ event: 'export', data: svgDataURI('<svg>2</svg>') })

  // Saving again without the editor reporting an XML falls back to what is
  // now current, not to the drawing the session opened with.
  session.receive({ event: 'save' })
  const stored = session.receive({ event: 'export', data: svgDataURI('<svg>3</svg>') })
  assert.deepEqual(stored, { kind: 'save', xml: '<mxfile>second</mxfile>', svg: '<svg>3</svg>' })
})

test('a new diagram starts from a drawing the editor can open', () => {
  assert.match(EMPTY_DRAWIO_XML, /^<mxfile>/)
  assert.match(EMPTY_DRAWIO_XML, /mxGraphModel/)
})
