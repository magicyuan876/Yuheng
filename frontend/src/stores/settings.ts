import { defineStore } from "pinia";
import { nextTick } from "vue";
import { SETTINGS_STORAGE_KEY, loadSettings } from "@/stores/settingsStorage";

// 定义设置接口
interface Settings {
  selectedKnowledgeBases: string[]; // 当前选中的知识库ID列表
  selectedFiles: string[]; // 当前选中的文件ID列表
  selectedFileKbMap: Record<string, string>; // 文件ID -> 知识库ID，用于刷新后带 kb_id 拉取共享知识库文件
  selectedTags: Array<{ id: string; name: string; kbId: string; kbName?: string }>;
  ollamaConfig: OllamaConfig; // Ollama配置
  conversationModels: ConversationModels;
  /**
   * The user's web-search switch in the chat input. It is a preference, sent
   * as web_search_enabled only while web search is actually available (see
   * useWebSearchToggle), so a workspace that loses its provider does not
   * leave the flag stuck on.
   */
  webSearchEnabled: boolean;
}

interface ConversationModels {
  selectedChatModelId: string; // 用户当前选择的对话模型ID
}

// Ollama 配置接口
interface OllamaConfig {
  baseUrl: string; // Ollama 服务地址
}

// 默认设置
const defaultSettings: Settings = {
  selectedKnowledgeBases: [], // 默认为空数组
  selectedFiles: [], // 默认为空数组
  selectedFileKbMap: {}, // 文件ID -> 知识库ID
  selectedTags: [],
  ollamaConfig: {
    baseUrl: "http://localhost:11434",
  },
  conversationModels: {
    selectedChatModelId: "", // 用户当前选择的对话模型ID
  },
  webSearchEnabled: false,
};

export const useSettingsStore = defineStore("settings", {
  state: () => ({
    // 从本地存储加载设置，如果没有则使用默认设置
    settings: loadSettings(defaultSettings),
    // 进入会话时拍下"全局默认"的快照；离开会话时还原。非持久化字段：
    // 刷新页面相当于重新走"进入会话"流程，自然会重新拍快照。
    _defaultsSnapshot: null as Settings | null,
    /** 正在从 session.last_request_state 恢复输入栏，避免 watch 覆盖 KB 选择 */
    _isApplyingSessionState: false,
  }),

  getters: {
    conversationModels: (state) => state.settings.conversationModels,

    isWebSearchEnabled: (state) => state.settings.webSearchEnabled,
  },

  actions: {
    // Every mutation that should survive a reload writes the whole settings
    // object back; session-state restores deliberately do not (see below).
    persist() {
      localStorage.setItem(SETTINGS_STORAGE_KEY, JSON.stringify(this.settings));
    },

    updateConversationModels(models: Partial<ConversationModels>) {
      this.settings.conversationModels = { ...this.settings.conversationModels, ...models };
      this.persist();
    },

    setWebSearchEnabled(enabled: boolean) {
      this.settings.webSearchEnabled = enabled;
      this.persist();
    },

    // 更新 Ollama 配置
    updateOllamaConfig(config: Partial<OllamaConfig>) {
      this.settings.ollamaConfig = { ...this.settings.ollamaConfig, ...config };
      this.persist();
    },

    // 选择知识库（替换整个列表）
    selectKnowledgeBases(kbIds: string[]) {
      this.settings.selectedKnowledgeBases = kbIds;
      this.persist();
    },

    // 添加单个知识库
    addKnowledgeBase(kbId: string) {
      if (!this.settings.selectedKnowledgeBases.includes(kbId)) {
        this.settings.selectedKnowledgeBases.push(kbId);
        this.persist();
      }
    },

    // 移除单个知识库
    removeKnowledgeBase(kbId: string) {
      this.settings.selectedKnowledgeBases = this.settings.selectedKnowledgeBases.filter((id: string) => id !== kbId);
      this.persist();
    },

    // 清空知识库选择
    clearKnowledgeBases() {
      this.settings.selectedKnowledgeBases = [];
      this.persist();
    },

    // File selection actions
    addFile(fileId: string) {
      if (!this.settings.selectedFiles.includes(fileId)) {
        this.settings.selectedFiles.push(fileId);
        this.persist();
      }
    },

    removeFile(fileId: string) {
      this.settings.selectedFiles = this.settings.selectedFiles.filter((id: string) => id !== fileId);
      delete this.settings.selectedFileKbMap[fileId];
      this.persist();
    },

    addTag(tag: { id: string; name: string; kbId: string; kbName?: string }) {
      if (!this.settings.selectedTags.some((t) => t.id === tag.id && t.kbId === tag.kbId)) {
        this.settings.selectedTags.push(tag);
        this.persist();
      }
    },

    removeTag(tagId: string, kbId?: string) {
      this.settings.selectedTags = this.settings.selectedTags.filter(
        (t) => !(t.id === tagId && (!kbId || t.kbId === kbId)),
      );
      this.persist();
    },

    setFileKbMap(updates: Record<string, string>) {
      Object.assign(this.settings.selectedFileKbMap, updates);
      this.persist();
    },

    // —— 会话级输入态恢复 —— //
    //
    // 输入栏的模型 / KB / @mention 等选择由本 store 持有，跨会话共享。
    // 但用户的诉求是：点开旧会话时，能看到当时发起请求的那一套状态。
    // 实现策略：进入会话时把"当前的全局默认"暂存到一个非持久化的 `_defaultsSnapshot`
    // 字段里，然后用 session.last_request_state 覆盖 store；离开会话时从快照还原。
    // 快照不写 localStorage，因为它只在「正处于某个旧会话」这段路由期内有意义；
    // 刷新页面相当于"重新进入会话" → 重新拍快照 + 覆盖，不会丢失用户的全局默认。

    // 拍下当前 settings 作为"离开会话后要还原回去的默认"。
    // 已存在快照时不覆盖，避免会话间切换（B→B'）把已恢复的 store 错当成默认。
    snapshotAsDefaultsIfNeeded() {
      if (this._defaultsSnapshot) return;
      this._defaultsSnapshot = JSON.parse(JSON.stringify(this.settings));
    },

    // 还原默认（如果有快照），用于离开会话或跨会话切换时。
    restoreDefaultsIfSnapshotted() {
      if (!this._defaultsSnapshot) return;
      this.settings = this._defaultsSnapshot;
      this._defaultsSnapshot = null;
      // 不写 localStorage：默认值在快照之前已经写过 localStorage，这里恢复
      // 的就是 localStorage 里既有的值，再写一次只会增加无意义的 IO。
    },

    // 根据 session.last_request_state 覆盖输入栏相关字段。
    // 只触碰本次记录的字段，**不**清空 store 中其它无关字段（如模型列表）。
    // 任何字段缺失则保留 store 现值，做"尽力恢复"。
    applyLastRequestState(state: SessionLastRequestStatePayload | null | undefined) {
      if (!state) return;
      this._isApplyingSessionState = true;
      try {
        if (state.model_id !== undefined) {
          this.settings.conversationModels = {
            ...this.settings.conversationModels,
            selectedChatModelId: state.model_id || "",
          };
        }
        if (Array.isArray(state.knowledge_base_ids)) {
          this.settings.selectedKnowledgeBases = [...state.knowledge_base_ids];
        }
        if (Array.isArray(state.knowledge_ids)) {
          this.settings.selectedFiles = [...state.knowledge_ids];
          // selectedFileKbMap 此时无法重建（state 里没存 KB 归属），交给前端按
          // 需要 lazy 拉取。保留 store 现值，避免误删用户刚加进来的文件映射。
        }
        if (Array.isArray(state.mentioned_items)) {
          const fromMentions = state.mentioned_items
            .filter((item) => item.type === "tag" && item.id && item.kb_id)
            .map((item) => ({ id: item.id, name: item.name || item.id, kbId: item.kb_id!, kbName: item.kb_name }));
          const covered = new Set(fromMentions.map((t) => t.id));
          const orphanTagIds = (state.tag_ids || []).filter((id) => id && !covered.has(id));
          if (
            orphanTagIds.length > 0 &&
            Array.isArray(state.knowledge_base_ids) &&
            state.knowledge_base_ids.length === 1
          ) {
            const kbId = state.knowledge_base_ids[0];
            orphanTagIds.forEach((id) => {
              fromMentions.push({ id, name: id, kbId, kbName: undefined });
            });
          }
          this.settings.selectedTags = fromMentions;
        } else if (Array.isArray(state.tag_ids)) {
          this.settings.selectedTags = this.settings.selectedTags.filter((tag) => state.tag_ids?.includes(tag.id));
        }
        if (typeof state.web_search_enabled === "boolean") {
          this.settings.webSearchEnabled = state.web_search_enabled;
        }
      } finally {
        // 复位必须延后到下一次 flush 之后：监听 store 字段的 watcher 默认
        // flush:'pre'，是异步执行的；若在此处同步复位，watcher 真正运行时标志早已
        // 为 false，守卫形同虚设、恢复出来的 KB 仍会被覆盖。放到 nextTick
        // 可保证本次状态变更触发的 watcher 在标志仍为 true 时执行。
        nextTick(() => {
          this._isApplyingSessionState = false;
        });
      }
      // 注意：故意不写 localStorage —— 旧会话的状态不应污染"用户默认"。
      // 离开会话时 restoreDefaultsIfSnapshotted 会把 localStorage 里那份完整
      // 的默认值再次同步回 this.settings。
    },
  },
});

// 后端 sessions.last_request_state JSON 形状（与 SessionLastRequestState 对齐）。
// 字段全部可选——历史会话或新建会话首发前的请求没有这条记录。
export interface SessionLastRequestStatePayload {
  model_id?: string;
  knowledge_base_ids?: string[];
  knowledge_ids?: string[];
  tag_ids?: string[];
  mentioned_items?: Array<{
    id: string;
    name?: string;
    type: string;
    kb_id?: string;
    kb_name?: string;
  }>;
  web_search_enabled?: boolean;
}
