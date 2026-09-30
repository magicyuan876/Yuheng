<template>
  <div class="bg-card flex h-full min-h-0 w-full min-w-[600px] items-stretch" ref="dropzone">
    <Menu></Menu>
    <div v-if="isRouterAlive" class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <RouterView />
    </div>
    <div class="fixed inset-0 z-[999] flex items-center justify-center bg-white/80" v-show="ismask">
      <UploadMask></UploadMask>
    </div>
    <!-- 全局设置模态框，供所有 platform 子路由使用 -->
    <Settings />
    <!-- 全局命令面板 (⌘K)，随 platform 路由存活 -->
    <GlobalCommandPalette />
    <!-- 全局右上角：知识待办与"待处理邀请"铃铛。固定定位，z-index 低于抽屉，
         业务页面右侧抽屉弹出时会自然覆盖；各自仅在有内容时渲染。 -->
    <GlobalCornerActions />
    <!-- 带遮罩层的新手引导：首次进入自动开启，可从用户菜单顶部昵称旁帮助按钮重新打开 -->
    <NewUserGuide />
  </div>
</template>
<script setup lang="ts">
import Menu from "@/components/menu.vue";
import { ref, onMounted, onUnmounted, nextTick, provide } from "vue";
import { useRoute } from "vue-router";
import UploadMask from "@/components/upload-mask.vue";
import Settings from "@/views/settings/Settings.vue";
import GlobalCommandPalette from "@/components/GlobalCommandPalette.vue";
import GlobalCornerActions from "@/components/GlobalCornerActions.vue";
import NewUserGuide from "@/components/NewUserGuide.vue";
import { useChatResourcesStore } from "@/stores/chatResources";
import { getKnowledgeBaseById } from "@/api/knowledge-base/index";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import { collectDroppedFiles } from "./collectDroppedFiles";

const route = useRoute();
const ismask = ref(false);
const { t } = useI18n();

const isRouterAlive = ref(true);
const reloadApp = () => {
  isRouterAlive.value = false;
  nextTick(() => {
    isRouterAlive.value = true;
  });
};
provide("app:reload", reloadApp);

// 用于跟踪拖拽进入/离开的计数器，解决子元素触发 dragleave 的问题
let dragCounter = 0;

// 获取当前知识库ID
const getCurrentKbId = (): string | null => {
  return ((route.params as any)?.kbId as string) || null;
};

const CHAT_DROP_ROUTE_NAMES = new Set(["chat", "globalCreatChat", "kbCreatChat"]);

const isChatDropRoute = () => {
  return CHAT_DROP_ROUTE_NAMES.has(String(route.name || ""));
};

// 检查知识库初始化状态
const checkKnowledgeBaseInitialization = async (): Promise<boolean> => {
  const currentKbId = getCurrentKbId();

  if (!currentKbId) {
    MessagePlugin.error(t("knowledgeBase.missingId"));
    return false;
  }

  try {
    const kbResponse = await getKnowledgeBaseById(currentKbId);
    const kb = kbResponse.data;

    if (!kb.summary_model_id) {
      MessagePlugin.warning(t("knowledgeBase.notInitialized"));
      return false;
    }
    const strategy = kb.indexing_strategy;
    const needsEmbedding = !strategy || strategy.vector_enabled || strategy.keyword_enabled;
    if (needsEmbedding && !kb.embedding_model_id) {
      MessagePlugin.warning(t("knowledgeBase.notInitialized"));
      return false;
    }
    return true;
  } catch (error) {
    MessagePlugin.error(t("knowledgeBase.getInfoFailed"));
    return false;
  }
};

// isFileDrag distinguishes an OS file drag (the only thing the global upload
// drop zone cares about) from an in-app element drag such as the wiki
// folder/page drag-and-drop. Element drags carry only "text/*" types, never
// "Files", so we bail out and let the originating component handle the drop.
const isFileDrag = (event: DragEvent): boolean => {
  const types = event.dataTransfer?.types;
  if (!types) return false;
  return Array.from(types).includes("Files");
};

// 全局拖拽事件处理
const handleGlobalDragEnter = (event: DragEvent) => {
  if (!isFileDrag(event)) return;
  event.preventDefault();
  dragCounter++;
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = "all";
  }
  ismask.value = true;
};

const handleGlobalDragOver = (event: DragEvent) => {
  if (!isFileDrag(event)) return;
  event.preventDefault();
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = "copy";
  }
};

const handleGlobalDragLeave = (event: DragEvent) => {
  if (!isFileDrag(event)) return;
  event.preventDefault();
  dragCounter--;
  if (dragCounter === 0) {
    ismask.value = false;
  }
};

const handleGlobalDrop = async (event: DragEvent) => {
  if (!isFileDrag(event)) return;
  event.preventDefault();
  dragCounter = 0;
  ismask.value = false;

  const droppedFiles = await collectDroppedFiles(event);
  if (droppedFiles.length === 0) {
    MessagePlugin.warning(t("knowledgeBase.dragFileNotText"));
    return;
  }

  if (isChatDropRoute()) {
    event.stopPropagation();
    window.dispatchEvent(
      new CustomEvent("yuheng:chat-file-drop", {
        detail: { files: droppedFiles },
      }),
    );
    return;
  }

  const isInitialized = await checkKnowledgeBaseInitialization();
  if (!isInitialized) {
    return;
  }

  window.dispatchEvent(
    new CustomEvent("yuheng:knowledge-file-drop", {
      detail: { kbId: getCurrentKbId(), files: droppedFiles },
    }),
  );
};

// 组件挂载时添加全局事件监听器
onMounted(() => {
  document.addEventListener("dragenter", handleGlobalDragEnter, true);
  document.addEventListener("dragover", handleGlobalDragOver, true);
  document.addEventListener("dragleave", handleGlobalDragLeave, true);
  document.addEventListener("drop", handleGlobalDrop, true);
  // 后台预取对话输入栏资源，进入 creatChat / chat 时复用缓存
  void useChatResourcesStore().prefetchChatInput();
});

// 组件卸载时移除全局事件监听器
onUnmounted(() => {
  document.removeEventListener("dragenter", handleGlobalDragEnter, true);
  document.removeEventListener("dragover", handleGlobalDragOver, true);
  document.removeEventListener("dragleave", handleGlobalDragLeave, true);
  document.removeEventListener("drop", handleGlobalDrop, true);
  dragCounter = 0;
});
</script>
<style>
/*
 * No utility exists for the non-standard user-drag property, and the rule has
 * to reach every <img> rendered in the routed outlet, not just images this
 * template owns. (-khtml/-moz/-o prefixed copies in the old block were no-ops
 * in every supported browser; -webkit- is the only one that matters.)
 */
img {
  -webkit-user-drag: none;
  user-drag: none;
}
</style>
