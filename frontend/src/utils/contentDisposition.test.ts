import assert from 'node:assert/strict'
import test from 'node:test'

import { fileNameFromDisposition } from './contentDisposition'

test('a header with no name at all falls back', () => {
  assert.equal(fileNameFromDisposition(undefined, 'export.zip'), 'export.zip')
  assert.equal(fileNameFromDisposition('attachment', 'export.zip'), 'export.zip')
})

test('the plain parameter is read', () => {
  assert.equal(
    fileNameFromDisposition('attachment; filename="Onboarding.md"', 'x'),
    'Onboarding.md',
  )
  assert.equal(fileNameFromDisposition('attachment; filename=Onboarding.md', 'x'), 'Onboarding.md')
})

// The point of the whole function: a Chinese title survives the round trip.
test('the encoded parameter wins over the ASCII fallback', () => {
  const header =
    "attachment; filename=\"________.md\"; filename*=UTF-8''%E5%AD%98%E5%82%A8%E9%85%8D%E9%A2%9D%E8%AF%B4%E6%98%8E.md"
  assert.equal(fileNameFromDisposition(header, 'x'), '存储配额说明.md')
})

test('a broken encoding does not lose the download', () => {
  const header = "attachment; filename=\"Notes.md\"; filename*=UTF-8''%E4%B8"
  assert.equal(fileNameFromDisposition(header, 'x'), 'Notes.md')
})

test('a spaced-out header is still parsed', () => {
  assert.equal(fileNameFromDisposition('attachment; filename = "A B.md"', 'x'), 'A B.md')
})
