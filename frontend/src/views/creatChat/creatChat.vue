<template>
  <div class="flex flex-1 items-center justify-center">
    <div class="dialogue-answers flex w-full max-w-[960px] flex-col items-center gap-6">
      <div
        class="text-foreground mb-0 flex items-center [font-family:var(--app-font-family)] text-[28px] font-semibold"
      >
        <span>{{ $t("createChat.title") }}</span>
      </div>
      <InputField ref="inputFieldRef" @send-msg="sendMsg"></InputField>
    </div>
  </div>

  <ContextualGuide tour="chat" :when="showChatContextualGuide" />

  <!-- 知识库编辑器（创建/编辑统一组件） -->
  <KnowledgeBaseEditorModal
    :visible="uiStore.showKBEditorModal"
    :mode="uiStore.kbEditorMode"
    :kb-id="uiStore.currentKBId || undefined"
    :initial-type="uiStore.kbEditorType"
    @update:visible="(val) => (val ? null : uiStore.closeKBEditor())"
    @success="handleKBEditorSuccess"
  />
</template>
<script setup lang="ts">
import { ref, computed } from "vue";
import ContextualGuide from "@/components/ContextualGuide.vue";
import InputField from "@/components/Input-field.vue";
import { createSessions } from "@/api/chat/index";
import { useMenuStore } from "@/stores/menu";
import { useUIStore } from "@/stores/ui";
import { useRoute, useRouter } from "vue-router";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import KnowledgeBaseEditorModal from "@/views/knowledge/KnowledgeBaseEditorModal.vue";
import { useKnowledgeBaseCreationNavigation } from "@/hooks/useKnowledgeBaseCreationNavigation";

const router = useRouter();
const route = useRoute();
const usemenuStore = useMenuStore();
const uiStore = useUIStore();
const { t } = useI18n();
const { navigateToKnowledgeBaseList } = useKnowledgeBaseCreationNavigation();

const showChatContextualGuide = computed(() => {
  return route.name === "globalCreatChat" || route.name === "kbCreatChat";
});

const inputFieldRef = ref();

const sendMsg = (
  value: string,
  modelId: string,
  mentionedItems: any[],
  imageFiles: any[] = [],
  attachmentFiles: any[] = [],
) => {
  createNewSession(value, modelId, mentionedItems, imageFiles, attachmentFiles);
};

async function createNewSession(
  value: string,
  modelId: string,
  mentionedItems: any[] = [],
  imageFiles: any[] = [],
  attachmentFiles: any[] = [],
) {
  // 会话只存基础信息（空间、标题），检索范围在每次请求时由 @提及/选择决定。
  const sessionData: any = {};

  try {
    const res = await createSessions(sessionData);
    if (res.data && res.data.id) {
      await navigateToSession(res.data.id, value, modelId, mentionedItems, imageFiles, attachmentFiles);
    } else {
      console.error("[createChat] Failed to create session");
      MessagePlugin.error(t("createChat.messages.createFailed"));
    }
  } catch (error) {
    console.error("[createChat] Create session error:", error);
    MessagePlugin.error(t("createChat.messages.createError"));
  }
}

const navigateToSession = async (
  sessionId: string,
  value: string,
  modelId: string,
  mentionedItems: any[],
  imageFiles: any[] = [],
  attachmentFiles: any[] = [],
) => {
  const now = new Date().toISOString();
  const obj = {
    title: t("createChat.newSessionTitle"),
    path: `chat/${sessionId}`,
    id: sessionId,
    isMore: false,
    isNoTitle: true,
    created_at: now,
    updated_at: now,
  };
  usemenuStore.updataMenuChildren(obj);
  usemenuStore.changeIsFirstSession(true);
  usemenuStore.changeFirstQuery(value, mentionedItems, modelId, imageFiles, attachmentFiles);
  router.push(`/platform/chat/${sessionId}`);
};

const handleKBEditorSuccess = (kbId: string) => {
  navigateToKnowledgeBaseList(kbId);
};
</script>
<style scoped>
/*
 * Stays CSS: these rules reach inside InputField (its root .answers-input,
 * which also carries this component's scope id, and its textarea), so they
 * cannot be utility classes on this template; and the textarea widths are
 * breakpoint-specific values, not tokens. The textarea is matched as an
 * element rather than by the TDesign class it currently wears, so the rules
 * keep working when InputField moves to the new stack.
 *
 * The Less version also set per-breakpoint translateX() offsets on
 * .answers-input, but a more specific rule (the static reset below, nested
 * under the page wrapper) always beat them, so they never applied
 * and are not carried over.
 */
/* Nested under the wrapper so it outranks InputField's own absolute
 * positioning whatever order the two stylesheets load in. */
.dialogue-answers :deep(.answers-input) {
  position: static;
  transform: translateX(0);
}

@media (max-width: 1250px) and (min-width: 1045px) {
  :deep(.answers-input textarea) {
    width: 654px !important;
  }
}

@media (max-width: 1045px) {
  :deep(.answers-input textarea) {
    width: 500px !important;
  }
}

@media (max-width: 750px) {
  :deep(.answers-input textarea) {
    width: 340px !important;
  }
}

@media (max-width: 600px) {
  :deep(.answers-input textarea) {
    width: 300px !important;
  }
}
</style>
