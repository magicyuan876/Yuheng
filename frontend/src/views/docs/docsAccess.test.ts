import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  canManageSpace,
  isValidSlug,
  normaliseSpaceForm,
  roleAtLeast,
  sortSpaceMembers,
  suggestSlug,
  validateSpaceForm,
  wouldLeaveNoAdmin,
} from './docsAccess'

test('suggestSlug mirrors the server derivation', () => {
  assert.equal(suggestSlug('Product  Specs!'), 'product-specs')
  assert.equal(suggestSlug('  Engineering '), 'engineering')
  assert.equal(suggestSlug('研发知识库'), '', 'no ASCII content yields nothing (server generates)')
  assert.equal(suggestSlug('A'), '', 'shorter than two characters is rejected')
  assert.equal(suggestSlug('x'.repeat(80)).length, 64)
})

test('isValidSlug enforces the URL rules', () => {
  assert.equal(isValidSlug('eng'), true)
  assert.equal(isValidSlug('eng-2'), true)
  assert.equal(isValidSlug('Eng'), false)
  assert.equal(isValidSlug('-eng'), false)
  assert.equal(isValidSlug('a'), false)
  assert.equal(isValidSlug('has space'), false)
})

test('normaliseSpaceForm couples visibility and default role', () => {
  assert.equal(normaliseSpaceForm({ visibility: 'private', default_role: 'writer' }).default_role, 'none')
  assert.equal(normaliseSpaceForm({ visibility: 'open', default_role: 'none' }).default_role, 'reader')
  assert.equal(normaliseSpaceForm({ visibility: 'open', default_role: 'admin' }).default_role, 'reader')
  assert.equal(normaliseSpaceForm({ visibility: 'open', default_role: 'writer' }).default_role, 'writer')
})

test('validateSpaceForm reports i18n keys per field', () => {
  const base = { name: 'Team', slug: '', description: '', visibility: 'private' as const, default_role: 'none' as const }
  assert.deepEqual(validateSpaceForm(base), [])
  assert.deepEqual(validateSpaceForm({ ...base, name: '  ' }), [{ field: 'name', key: 'nameRequired' }])
  assert.deepEqual(validateSpaceForm({ ...base, slug: 'Bad Slug' }), [{ field: 'slug', key: 'slugInvalid' }])
  assert.deepEqual(validateSpaceForm(base, { requireSlug: true }), [{ field: 'slug', key: 'slugRequired' }])
  assert.deepEqual(validateSpaceForm({ ...base, name: 'x'.repeat(101) }), [{ field: 'name', key: 'nameTooLong' }])
})

test('role helpers follow the level ladder', () => {
  assert.equal(roleAtLeast('writer', 'reader'), true)
  assert.equal(roleAtLeast('reader', 'writer'), false)
  assert.equal(roleAtLeast(undefined, 'reader'), false)
  assert.equal(canManageSpace('admin'), true)
  assert.equal(canManageSpace('writer'), false)
})

test('sortSpaceMembers orders admins first, groups before users, then by name', () => {
  const sorted = sortSpaceMembers([
    { role: 'reader', principal_type: 'user', name: 'bob' },
    { role: 'writer', principal_type: 'group', name: 'Backend' },
    { role: 'admin', principal_type: 'user', name: 'alice' },
    { role: 'reader', principal_type: 'group', name: 'everyone' },
  ])
  assert.deepEqual(sorted.map((m) => m.name), ['alice', 'Backend', 'everyone', 'bob'])
})

test('wouldLeaveNoAdmin mirrors the last-administrator rule', () => {
  const members = [
    { role: 'admin' as const, principal_type: 'user' as const, principal_id: 'alice' },
    { role: 'writer' as const, principal_type: 'user' as const, principal_id: 'bob' },
  ]
  assert.equal(wouldLeaveNoAdmin(members, { principal_type: 'user', principal_id: 'alice', role: 'reader' }), true)
  assert.equal(wouldLeaveNoAdmin(members, { principal_type: 'user', principal_id: 'alice', role: null }), true)
  assert.equal(wouldLeaveNoAdmin(members, { principal_type: 'user', principal_id: 'bob', role: null }), false)
  assert.equal(wouldLeaveNoAdmin(members, { principal_type: 'user', principal_id: 'bob', role: 'admin' }), false)
})
