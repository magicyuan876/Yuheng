import assert from 'node:assert/strict'
import { test } from 'vitest'

import {
  SETTINGS_SECTION_CAPABILITY,
  isDeploymentCapabilitySupported,
  type DeploymentCapabilityMap,
} from './deploymentCapabilities'

test('capability filtering is fail-open unless backend explicitly disables a feature', () => {
  assert.equal(isDeploymentCapabilitySupported({}, 'organizations'), true)

  const capabilities: DeploymentCapabilityMap = {
    organizations: { supported: false, reason: 'route_not_registered' },
    'settings.storage': { supported: true },
  }
  assert.equal(isDeploymentCapabilitySupported(capabilities, 'organizations'), false)
  assert.equal(isDeploymentCapabilitySupported(capabilities, 'settings.storage'), true)
})

test('only route-backed settings sections require deployment capabilities', () => {
  assert.equal(SETTINGS_SECTION_CAPABILITY.websearch, 'settings.websearch')
  assert.equal(SETTINGS_SECTION_CAPABILITY.storage, 'settings.storage')
  assert.equal(SETTINGS_SECTION_CAPABILITY.parser, undefined)
  assert.equal(SETTINGS_SECTION_CAPABILITY['runtime-queues'], undefined)
})
