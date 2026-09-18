import assert from 'node:assert/strict'
import { test } from 'node:test'

import { loadConfig } from '../src/config.ts'

const MIN = {
  COLLAB_BACKEND_URL: 'http://app:8080',
  COLLAB_SHARED_SECRET: 'a-shared-secret-of-sufficient-length',
}

test('defaults match the documented values', () => {
  const cfg = loadConfig({ ...MIN } as NodeJS.ProcessEnv)
  assert.equal(cfg.port, 1234)
  assert.equal(cfg.storeDebounceMs, 2000)
  assert.equal(cfg.storeMaxWaitMs, 10_000)
  assert.equal(cfg.maxYDocBytes, 20 * 1024 * 1024)
  assert.equal(cfg.recheckIntervalMs, 5 * 60_000)
  assert.equal(cfg.redisUrl, '')
  assert.equal(cfg.logLevel, 'info')
})

test('a trailing slash on the backend URL is dropped so paths do not double up', () => {
  const cfg = loadConfig({ ...MIN, COLLAB_BACKEND_URL: 'http://app:8080/' } as NodeJS.ProcessEnv)
  assert.equal(cfg.backendUrl, 'http://app:8080')
})

test('missing or malformed settings fail loudly', () => {
  assert.throws(() => loadConfig({ COLLAB_SHARED_SECRET: MIN.COLLAB_SHARED_SECRET } as NodeJS.ProcessEnv),
    /COLLAB_BACKEND_URL is required/)
  assert.throws(() => loadConfig({ ...MIN, COLLAB_BACKEND_URL: 'app:8080' } as NodeJS.ProcessEnv),
    /must start with http/)
  assert.throws(() => loadConfig({ ...MIN, COLLAB_SHARED_SECRET: 'short' } as NodeJS.ProcessEnv),
    /at least 16 characters/)
  assert.throws(() => loadConfig({ ...MIN, COLLAB_PORT: 'abc' } as NodeJS.ProcessEnv), /COLLAB_PORT/)
  assert.throws(() => loadConfig({ ...MIN, COLLAB_PORT: '-1' } as NodeJS.ProcessEnv), /COLLAB_PORT/)
  assert.equal(loadConfig({ ...MIN, COLLAB_PORT: '0' } as NodeJS.ProcessEnv).port, 0, 'port 0 means "pick one"')
  assert.throws(() => loadConfig({ ...MIN, COLLAB_LOG_LEVEL: 'loud' } as NodeJS.ProcessEnv), /COLLAB_LOG_LEVEL/)
})

test('the maximum store wait is never below the debounce', () => {
  const cfg = loadConfig({ ...MIN, COLLAB_STORE_DEBOUNCE_MS: '5000', COLLAB_STORE_MAX_WAIT_MS: '1000' } as NodeJS.ProcessEnv)
  assert.equal(cfg.storeDebounceMs, 5000)
  assert.equal(cfg.storeMaxWaitMs, 5000)
})
