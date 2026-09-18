import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  attachmentSrc,
  attachmentSrcSet,
  formatBytes,
  isImageFile,
  nodeForAttachment,
  uploadableFiles,
  UploadQueue,
  type UploadedAttachment,
} from './attachments'

function attachment(over: Partial<UploadedAttachment> = {}): UploadedAttachment {
  return {
    id: 'a1', file_name: 'photo.png', mime: 'image/png', size_bytes: 1234,
    kind: 'image', url: '/api/v1/docs/attachments/a1', ...over,
  }
}

/** Stands in for a browser File without needing one. */
function file(name: string, size = 10, type = ''): File {
  return { name, size, type } as unknown as File
}

test('an attachment address is the permission-checked endpoint, not a storage URL', () => {
  assert.equal(attachmentSrc('a1'), '/api/v1/docs/attachments/a1')
  assert.equal(attachmentSrc('a1', 320), '/api/v1/docs/attachments/a1?w=320')
  assert.equal(attachmentSrc('a b/c'), '/api/v1/docs/attachments/a%20b%2Fc', 'ids are escaped')
})

test('a srcset is offered only for the widths the server says it can render', () => {
  assert.equal(
    attachmentSrcSet('a1', [320, 800]),
    '/api/v1/docs/attachments/a1?w=320 320w, /api/v1/docs/attachments/a1?w=800 800w',
  )
  assert.equal(attachmentSrcSet('a1', []), '')
  assert.equal(attachmentSrcSet('a1', undefined), '')
})

test('sizes read the way a person would write them', () => {
  assert.equal(formatBytes(0), '0 B')
  assert.equal(formatBytes(999), '999 B')
  assert.equal(formatBytes(1024), '1 KB')
  assert.equal(formatBytes(1536), '1.5 KB')
  assert.equal(formatBytes(20 * 1024), '20 KB')
  assert.equal(formatBytes(5 * 1024 * 1024), '5 MB')
  assert.equal(formatBytes(1024 ** 4), '1 TB')
  assert.equal(formatBytes(-1), '')
  assert.equal(formatBytes(Number.NaN), '')
})

test('an image is recognised by its type, and by its name when there is no type', () => {
  assert.equal(isImageFile(file('a.png', 1, 'image/png')), true)
  assert.equal(isImageFile(file('a.PNG', 1, '')), true, 'some sources send no type at all')
  assert.equal(isImageFile(file('drawing.svg', 1, '')), true)
  assert.equal(isImageFile(file('report.pdf', 1, 'application/pdf')), false)
  assert.equal(isImageFile(file('notes', 1, '')), false)
})

test('empty entries from a paste are not uploaded', () => {
  // A directory drop and a text paste both surface as zero-length entries.
  const kept = uploadableFiles([file('real.png', 10), file('folder', 0), file('also.png', 4)])
  assert.deepEqual(kept.map((f) => f.name), ['real.png', 'also.png'])
})

test('an uploaded image becomes an image node carrying its own dimensions', () => {
  const node = nodeForAttachment(attachment({ width: 800, height: 600 }))
  assert.equal(node.type, 'image')
  assert.equal(node.attrs.attachmentId, 'a1')
  assert.equal(node.attrs.src, null, 'the address is derived from the id, never stored')
  assert.equal(node.attrs.width, 800)
  assert.equal(node.attrs.height, 600)
  assert.equal(node.attrs.align, 'center')
  assert.equal(node.attrs.alt, 'photo.png')
})

test('a video, an audio file and a PDF each get their own node', () => {
  // The kind the server derived from the bytes decides, not the file's name.
  const video = nodeForAttachment(attachment({ kind: 'video', mime: 'video/mp4', file_name: 'clip.txt' }))
  assert.equal(video.type, 'video')
  assert.equal(video.attrs.attachmentId, 'a1')

  const audio = nodeForAttachment(attachment({ kind: 'audio', mime: 'audio/mpeg' }))
  assert.equal(audio.type, 'audio')

  const pdf = nodeForAttachment(attachment({
    kind: 'file', mime: 'application/pdf', file_name: 'report.pdf',
  }))
  assert.equal(pdf.type, 'pdfEmbed')
  assert.equal(pdf.attrs.name, 'report.pdf')
})

test('anything a browser cannot usefully show becomes a file card', () => {
  for (const [kind, mime] of [['file', 'application/zip'], ['diagram', 'application/xml']] as const) {
    const node = nodeForAttachment(attachment({ kind, file_name: 'thing.bin', mime }))
    assert.equal(node.type, 'attachment', kind)
    assert.equal(node.attrs.attachmentId, 'a1')
    assert.equal(node.attrs.name, 'thing.bin')
    assert.equal(node.attrs.mime, mime)
    assert.equal(node.attrs.size, 1234)
  }
})

test('an image with unknown dimensions still produces a valid node', () => {
  const node = nodeForAttachment(attachment({ width: undefined, height: undefined }))
  assert.equal(node.attrs.width, null)
  assert.equal(node.attrs.height, null)
})

test('the upload queue reports each file until it lands or fails', () => {
  const seen: number[] = []
  const queue = new UploadQueue((tasks) => seen.push(tasks.length))

  const one = queue.start(file('a.png', 100, 'image/png'))
  const two = queue.start(file('b.zip', 200, 'application/zip'))
  assert.equal(one.isImage, true)
  assert.equal(two.isImage, false)
  assert.notEqual(one.key, two.key, 'two uploads of the same name must be distinguishable')
  assert.equal(queue.list().length, 2)
  assert.equal(queue.busy, true)

  queue.progress(one.key, 40)
  assert.equal(queue.get(one.key)?.progress, 40)
  queue.progress(one.key, 500)
  assert.equal(queue.get(one.key)?.progress, 100, 'progress is clamped')

  queue.finish(one.key)
  assert.equal(queue.list().length, 1)

  queue.fail(two.key, 'over quota')
  assert.equal(queue.get(two.key)?.error, 'over quota')
  assert.equal(queue.busy, false, 'a failed upload is not still in flight')

  queue.finish(two.key)
  assert.equal(queue.list().length, 0)
  assert.ok(seen.length >= 5, 'every change notifies the view')
})

test('the upload queue ignores updates for a task that already finished', () => {
  const queue = new UploadQueue()
  const task = queue.start(file('a.png', 10, 'image/png'))
  queue.finish(task.key)

  queue.progress(task.key, 50)
  queue.fail(task.key, 'too late')
  assert.equal(queue.get(task.key), undefined)
  assert.equal(queue.list().length, 0)
})
