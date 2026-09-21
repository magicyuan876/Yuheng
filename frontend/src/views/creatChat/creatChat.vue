<template>
  <div class="dialogue-wrap">
    <div class="dialogue-answers">
      <div class="dialogue-title">
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
<style lang="less" scoped>
.dialogue-wrap {
  flex: 1;
  display: flex;
  justify-content: center;
  align-items: center;
  // position: relative;
}

.dialogue-answers {
  display: flex;
  flex-flow: column;
  align-items: center;
  width: 100%;
  max-width: 960px;
  gap: 24px;

  :deep(.answers-input) {
    position: static;
    transform: translateX(0);
  }
}

.dialogue-title {
  display: flex;
  color: var(--td-text-color-primary);
  font-family: var(--app-font-family);
  font-size: 28px;
  font-weight: 600;
  align-items: center;
  margin-bottom: 0;

  .icon {
    display: flex;
    width: 32px;
    height: 32px;
    justify-content: center;
    align-items: center;
    border-radius: 6px;
    background: var(--td-bg-color-container);
    box-shadow: var(--td-shadow-1);
    margin-right: 12px;

    .logo_img {
      height: 24px;
      width: 24px;
    }
  }
}

@media (max-width: 1250px) and (min-width: 1045px) {
  .answers-input {
    transform: translateX(-329px);
  }

  :deep(.t-textarea__inner) {
    width: 654px !important;
  }
}

@media (max-width: 1045px) {
  .answers-input {
    transform: translateX(-250px);
  }

  :deep(.t-textarea__inner) {
    width: 500px !important;
  }
}

@media (max-width: 750px) {
  .answers-input {
    transform: translateX(-250px);
  }

  :deep(.t-textarea__inner) {
    width: 340px !important;
  }
}

@media (max-width: 600px) {
  .answers-input {
    transform: translateX(-250px);
  }

  :deep(.t-textarea__inner) {
    width: 300px !important;
  }
}
</style>
<style lang="less">
.del-menu-popup {
  z-index: 99 !important;

  .t-popup__content {
    width: 100px;
    height: 40px;
    line-height: 30px;
    padding-left: 14px;
    cursor: pointer;
    margin-top: 4px !important;
  }
}
</style>
