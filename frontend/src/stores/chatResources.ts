import { defineStore } from "pinia";
import { ref, computed } from "vue";
import { listKnowledgeBases, getKnowledgeBaseById } from "@/api/knowledge-base";
import { listModels, type ModelConfig } from "@/api/model";
import { listWebSearchProviders, type WebSearchProviderEntity } from "@/api/web-search-provider";
import { useOrganizationStore } from "@/stores/organization";

/** 空间级资源缓存 TTL */
const CACHE_TTL_MS = 60_000;

type ResourceKey = "knowledgeBases" | "models" | "webSearchProviders";

export type ListCreatorFilter = "all" | "mine" | "others";

function isKbModelReady(kb: any): boolean {
  if (!kb.summary_model_id || kb.summary_model_id === "") return false;
  const strategy = kb.indexing_strategy;
  const needsEmbedding = !strategy || strategy.vector_enabled || strategy.keyword_enabled;
  if (needsEmbedding && (!kb.embedding_model_id || kb.embedding_model_id === "")) return false;
  return true;
}

export const useChatResourcesStore = defineStore("chatResources", () => {
  const rawKnowledgeBases = ref<any[]>([]);
  const allModels = ref<ModelConfig[]>([]);
  const webSearchProviders = ref<WebSearchProviderEntity[]>([]);

  const loadedAt = ref<Partial<Record<ResourceKey, number>>>({});
  const inflight = new Map<ResourceKey, Promise<void>>();
  // creator==='all' 的列表请求单独去重：首屏 platform 预取与对话页 onMounted
  // 可能并发触发，缓存尚未写入时不去重会重复打 listKnowledgeBases。
  let kbAllInflight: Promise<any[]> | null = null;
  // 代际计数：force 与非 force 并发时句柄会被后来者覆盖，旧请求结束时凭此判断
  // 自己是否仍是最新的那次，避免误清正在飞行的句柄。
  let kbAllGen = 0;

  const kbDetailCache = new Map<string, { at: number; data: any }>();
  const kbDetailInflight = new Map<string, Promise<any | null>>();

  const validKnowledgeBases = computed(() => rawKnowledgeBases.value.filter(isKbModelReady));
  const chatModels = computed(() => allModels.value.filter((m) => m.type === "KnowledgeQA"));
  // Mirrors the backend's resolveWebSearchProviderID: a chat turn searches the
  // web with the workspace's default provider, falling back to a platform-shared
  // default. The list endpoint returns exactly those two sets, so "some
  // provider is the default" is the same test.
  const hasDefaultWebSearchProvider = computed(() => webSearchProviders.value.some((p) => p.is_default));

  function isFresh(key: ResourceKey): boolean {
    const at = loadedAt.value[key];
    return !!at && Date.now() - at < CACHE_TTL_MS;
  }

  async function runOnce(key: ResourceKey, force: boolean, loader: () => Promise<void>): Promise<void> {
    if (!force && isFresh(key)) return;
    const existing = inflight.get(key);
    if (existing) return existing;
    const p = loader().finally(() => {
      inflight.delete(key);
    });
    inflight.set(key, p);
    return p;
  }

  /**
   * 知识库列表（支持 creator 筛选）。creator=all 时写入缓存供对话页复用。
   */
  async function fetchKnowledgeBasesForList(params?: { creator?: ListCreatorFilter }, force = false): Promise<any[]> {
    const creator = params?.creator ?? "all";
    // 带 creator 过滤的列表是列表页专用、不进缓存，直接透传请求。
    if (creator !== "all") {
      const res: any = await listKnowledgeBases({ creator });
      return res?.data && Array.isArray(res.data) ? res.data : [];
    }

    if (!force && isFresh("knowledgeBases")) {
      return rawKnowledgeBases.value;
    }
    if (!force && kbAllInflight) return kbAllInflight;

    const gen = ++kbAllGen;
    kbAllInflight = (async () => {
      try {
        const res: any = await listKnowledgeBases();
        const data = res?.data && Array.isArray(res.data) ? res.data : [];
        rawKnowledgeBases.value = data;
        loadedAt.value.knowledgeBases = Date.now();
        const orgStore = useOrganizationStore();
        await orgStore.fetchSharedKnowledgeBases({ force });
        return data;
      } finally {
        if (kbAllGen === gen) kbAllInflight = null;
      }
    })();
    return kbAllInflight;
  }

  async function ensureKnowledgeBases(force = false): Promise<void> {
    await fetchKnowledgeBasesForList({ creator: "all" }, force);
  }

  async function ensureModels(force = false): Promise<void> {
    return runOnce("models", force, async () => {
      const models = await listModels();
      allModels.value = Array.isArray(models) ? models : [];
      loadedAt.value.models = Date.now();
    });
  }

  async function ensureWebSearchProviders(force = false): Promise<void> {
    return runOnce("webSearchProviders", force, async () => {
      try {
        const res = (await listWebSearchProviders()) as unknown as { data?: WebSearchProviderEntity[] };
        webSearchProviders.value = Array.isArray(res?.data) ? res.data : [];
      } catch {
        // No list, no switch: failing closed hides web search rather than
        // offering a toggle the backend would silently ignore.
        webSearchProviders.value = [];
      }
      loadedAt.value.webSearchProviders = Date.now();
    });
  }

  /** 并行预取对话输入栏及列表页常用的空间级资源 */
  async function prefetchChatInput(force = false): Promise<void> {
    const orgStore = useOrganizationStore();
    await Promise.all([ensureKnowledgeBases(force), ensureModels(force), orgStore.fetchOrganizations({ force })]);
  }

  /** 单个知识库详情（侧栏 + 详情页共用，去重并发请求） */
  async function fetchKnowledgeBaseById(kbId: string, force = false): Promise<any | null> {
    if (!kbId) return null;
    const cached = kbDetailCache.get(kbId);
    if (!force && cached && Date.now() - cached.at < CACHE_TTL_MS) {
      return cached.data;
    }
    const existing = kbDetailInflight.get(kbId);
    if (existing) return existing;

    const p = (async () => {
      try {
        const res: any = await getKnowledgeBaseById(kbId);
        const data = res?.data ?? null;
        if (data) {
          kbDetailCache.set(kbId, { at: Date.now(), data });
        }
        return data;
      } catch {
        return null;
      } finally {
        kbDetailInflight.delete(kbId);
      }
    })();
    kbDetailInflight.set(kbId, p);
    return p;
  }

  function invalidateKnowledgeBaseDetail(kbId?: string) {
    if (kbId) {
      kbDetailCache.delete(kbId);
      kbDetailInflight.delete(kbId);
    } else {
      kbDetailCache.clear();
      kbDetailInflight.clear();
    }
  }

  function invalidate(...keys: ResourceKey[]) {
    if (keys.length === 0) {
      loadedAt.value = {};
      rawKnowledgeBases.value = [];
      allModels.value = [];
      webSearchProviders.value = [];
      kbAllInflight = null;
      invalidateKnowledgeBaseDetail();
      // 同时丢弃所有 inflight 句柄，否则失效后仍在飞行的请求会把旧数据写回缓存。
      inflight.clear();
      return;
    }
    keys.forEach((k) => {
      delete loadedAt.value[k];
      inflight.delete(k);
    });
    if (keys.includes("knowledgeBases")) {
      kbAllInflight = null;
      invalidateKnowledgeBaseDetail();
    }
  }

  return {
    rawKnowledgeBases,
    validKnowledgeBases,
    allModels,
    chatModels,
    webSearchProviders,
    hasDefaultWebSearchProvider,
    isFresh,
    fetchKnowledgeBasesForList,
    ensureKnowledgeBases,
    ensureModels,
    ensureWebSearchProviders,
    prefetchChatInput,
    fetchKnowledgeBaseById,
    invalidateKnowledgeBaseDetail,
    invalidate,
  };
});
