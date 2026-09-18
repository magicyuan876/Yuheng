import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  getStorageEngineConfig,
  getStorageEngineStatus,
  getPromptTemplates,
  getParserEngines,
  getSystemInfo,
  type PromptTemplatesConfig,
  type StorageEngineStatusItem,
  type ParserEngineInfo,
  type SystemInfo,
} from '@/api/system'
import { getTenantRetrievalConfig } from '@/api/retrieval'

const CACHE_TTL_MS = 60_000

export function pickUsableStorageProvider(
  candidate: string | undefined,
  engines: StorageEngineStatusItem[],
  allowedProviders: string[],
): string {
  const provider = candidate?.trim() || ''
  const isUsable = (name: string) => {
    if (!name) return false
    const status = engines.find((item) => item.name === name)
    if (status) return status.allowed !== false && status.available !== false
    if (engines.length > 0) return false
    if (allowedProviders.length > 0) return allowedProviders.includes(name)
    return false
  }

  if (isUsable(provider)) return provider
  const fallback = engines.find((item) => item.allowed !== false && item.available !== false)?.name
  if (fallback) return fallback
  return allowedProviders[0] || provider || 'local'
}

type EditorResourceKey =
  | 'storageEngine'
  | 'promptTemplates'
  | 'tenantRetrievalConfig'
  | 'parserEngines'
  | 'systemInfo'

export const useEditorResourcesStore = defineStore('editorResources', () => {
  const storageConfig = ref<Awaited<ReturnType<typeof getStorageEngineConfig>>['data'] | null>(null)
  const storageStatus = ref<StorageEngineStatusItem[]>([])
  const storageAllowedProviders = ref<string[]>([])
  const promptTemplates = ref<PromptTemplatesConfig | null>(null)
  const tenantRetrievalConfig = ref<Record<string, unknown> | null>(null)
  const parserEngines = ref<ParserEngineInfo[]>([])
  const systemInfo = ref<SystemInfo | null>(null)

  const loadedAt = ref<Partial<Record<EditorResourceKey, number>>>({})
  const inflight = new Map<EditorResourceKey, Promise<void>>()

  function isFresh(key: EditorResourceKey): boolean {
    const at = loadedAt.value[key]
    return !!at && Date.now() - at < CACHE_TTL_MS
  }

  async function runOnce(key: EditorResourceKey, force: boolean, loader: () => Promise<void>): Promise<void> {
    if (!force && isFresh(key)) return
    const existing = inflight.get(key)
    if (existing) return existing
    const p = loader().finally(() => inflight.delete(key))
    inflight.set(key, p)
    return p
  }

  async function ensureStorageEngine(force = false): Promise<void> {
    return runOnce('storageEngine', force, async () => {
      const [configRes, statusRes] = await Promise.all([
        getStorageEngineConfig(),
        getStorageEngineStatus(),
      ])
      storageConfig.value = configRes?.data ?? null
      storageStatus.value = statusRes?.data?.engines ?? []
      storageAllowedProviders.value = statusRes?.data?.allowed_providers ?? []
      loadedAt.value.storageEngine = Date.now()
    })
  }

  function resolveUsableStorageProvider(candidate?: string): string {
    return pickUsableStorageProvider(
      candidate,
      storageStatus.value || [],
      storageAllowedProviders.value || [],
    )
  }

  async function ensurePromptTemplates(force = false): Promise<void> {
    return runOnce('promptTemplates', force, async () => {
      const tmplRes = await getPromptTemplates()
      promptTemplates.value = tmplRes?.data ?? null
      loadedAt.value.promptTemplates = Date.now()
    })
  }

  async function ensureTenantRetrievalConfig(force = false): Promise<void> {
    return runOnce('tenantRetrievalConfig', force, async () => {
      const retrievalRes: any = await getTenantRetrievalConfig()
      tenantRetrievalConfig.value = retrievalRes?.data ?? null
      loadedAt.value.tenantRetrievalConfig = Date.now()
    })
  }

  async function ensureParserEngines(force = false): Promise<void> {
    return runOnce('parserEngines', force, async () => {
      const resp = await getParserEngines()
      parserEngines.value = resp?.data && Array.isArray(resp.data) ? resp.data : []
      loadedAt.value.parserEngines = Date.now()
    })
  }

  async function ensureSystemInfo(force = false): Promise<void> {
    return runOnce('systemInfo', force, async () => {
      const response = await getSystemInfo()
      systemInfo.value = response?.data ?? null
      loadedAt.value.systemInfo = Date.now()
    })
  }

  function invalidate(...keys: EditorResourceKey[]) {
    if (keys.length === 0) {
      loadedAt.value = {}
      storageConfig.value = null
      storageStatus.value = []
      storageAllowedProviders.value = []
      promptTemplates.value = null
      tenantRetrievalConfig.value = null
      parserEngines.value = []
      systemInfo.value = null
      inflight.clear()
      return
    }
    keys.forEach((k) => {
      delete loadedAt.value[k]
      inflight.delete(k)
    })
  }

  return {
    storageConfig,
    storageStatus,
    storageAllowedProviders,
    promptTemplates,
    tenantRetrievalConfig,
    parserEngines,
    systemInfo,
    ensureStorageEngine,
    resolveUsableStorageProvider,
    ensurePromptTemplates,
    ensureTenantRetrievalConfig,
    ensureParserEngines,
    ensureSystemInfo,
    invalidate,
  }
})
