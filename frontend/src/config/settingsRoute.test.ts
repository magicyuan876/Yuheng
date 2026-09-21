import assert from 'node:assert/strict'
import { test } from 'vitest'

import {
  buildSettingsRouteQuery,
  normalizeSettingsSection,
  settingsQueryUnchanged,
} from './settingsRoute'

test('every settings nav item writes only section', () => {
  assert.deepEqual(
    buildSettingsRouteQuery('models', {
      section: 'system-global',
      tab: 'im',
      agentId: 'agt_1',
    }),
    { section: 'models' },
  )
  assert.deepEqual(
    buildSettingsRouteQuery('general', { section: 'system-global' }),
    { section: 'general' },
  )
  assert.deepEqual(
    buildSettingsRouteQuery('runtime-queues', { section: 'system-global' }),
    { section: 'runtime-queues' },
  )
})

test('removed integration sections in legacy URLs normalize back to settings home', () => {
  assert.equal(normalizeSettingsSection('api'), 'general')
  assert.equal(normalizeSettingsSection('claw'), 'general')
  assert.equal(normalizeSettingsSection('integrations', 'embed'), 'general')
  assert.equal(normalizeSettingsSection('integrations'), 'general')
  assert.equal(normalizeSettingsSection('integration-chrome'), 'general')
  assert.equal(normalizeSettingsSection('system-global'), 'system-global')
  assert.equal(normalizeSettingsSection('models'), 'models')
})

test('canonical settings query skips a redundant replace', () => {
  assert.equal(
    settingsQueryUnchanged(
      { section: 'models' },
      { section: 'models' },
    ),
    true,
  )
  assert.equal(
    settingsQueryUnchanged(
      { section: 'integrations', tab: 'claw' },
      { section: 'general' },
    ),
    false,
  )
})
