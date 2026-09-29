<template>
  <!-- On a wide screen with the references panel open the header docks: it
       stops floating over the messages and becomes a full-width bar above
       them. Below 960px it floats in both cases. -->
  <header
    class="chat-header pointer-events-auto absolute top-[10px] left-3 z-[6] box-border inline-flex min-w-0 items-center gap-0.5 rounded-[8px] bg-[color-mix(in_srgb,var(--td-bg-color-container)_88%,transparent)] backdrop-blur-[8px]"
    :class="{
      'is-editing max-w-[min(360px,calc(100%-24px))] p-0.5': titleEditing,
      'max-w-[min(280px,calc(100%-24px))] py-0.5 pr-0.5 pl-2': !titleEditing,
      'is-docked min-[960px]:border-border min-[960px]:bg-card min-[960px]:relative min-[960px]:top-auto min-[960px]:left-auto min-[960px]:z-[5] min-[960px]:m-0 min-[960px]:w-full min-[960px]:max-w-none min-[960px]:shrink-0 min-[960px]:self-stretch min-[960px]:rounded-none min-[960px]:border-b min-[960px]:border-solid min-[960px]:backdrop-blur-none min-[960px]:transition-[border-color] min-[960px]:duration-300 min-[960px]:ease-[cubic-bezier(0.22,0.61,0.36,1)]':
        hasReferencesPanel,
      'min-[960px]:px-3 min-[960px]:py-2': hasReferencesPanel && titleEditing,
      'min-[960px]:px-3 min-[960px]:py-2.5': hasReferencesPanel && !titleEditing,
    }"
  >
    <form v-if="titleEditing" class="w-60 max-w-full min-w-0 flex-auto" @submit.prevent="submitTitleEdit" @click.stop>
      <input
        ref="titleInputRef"
        v-model="titleDraft"
        data-slot="chat-header-title-input"
        class="border-primary bg-card text-foreground box-border h-7 w-full rounded-[5px] border border-solid px-2 text-[14px] leading-[26px] shadow-[0_0_0_2px_var(--td-brand-color-light)] outline-none disabled:opacity-70"
        :maxlength="SESSION_TITLE_MAX_LENGTH"
        :disabled="busyAction === 'rename'"
        :placeholder="t('chatHeader.renamePlaceholder')"
        @keydown.esc.prevent="cancelTitleEdit"
        @blur="submitTitleEdit"
      />
    </form>
    <h1
      v-else
      class="text-muted-foreground m-0 inline-flex min-w-0 cursor-default items-center gap-1 p-0 text-[14px] leading-5 font-medium"
      :title="displayTitle"
      @dblclick="startTitleEdit"
    >
      <PinIcon v-if="session?.is_pinned" class="text-placeholder size-3 flex-none" aria-hidden="true" />
      <span class="min-w-0 truncate">{{ displayTitle }}</span>
    </h1>
    <Popover v-if="!titleEditing" :open="menuVisible" @update:open="onMenuOpenChange">
      <PopoverTrigger as-child>
        <button
          type="button"
          data-slot="chat-header-menu-trigger"
          class="text-placeholder enabled:hover:bg-accent enabled:hover:text-foreground inline-flex size-6 flex-none items-center justify-center rounded-[5px] p-0 transition-[background-color,color] duration-150 ease-in-out enabled:active:bg-[var(--td-bg-color-container-active)] disabled:cursor-not-allowed disabled:opacity-45"
          :class="{ 'cursor-wait!': Boolean(busyAction) }"
          :disabled="!session || Boolean(busyAction)"
          :aria-label="t('chatHeader.moreActions')"
          @click.stop
        >
          <Loader2Icon v-if="busyAction" class="size-3.5 animate-spin" aria-hidden="true" />
          <EllipsisIcon v-else class="size-4" aria-hidden="true" />
        </button>
      </PopoverTrigger>
      <!-- The same card holds the action list and, in place of it, the
           clear/delete confirmation, so the card widens and pads out when it
           switches to the confirmation. -->
      <PopoverContent
        align="start"
        :side-offset="6"
        class="bg-card z-[99] gap-0 overflow-hidden rounded-[8px] border-[0.5px] border-solid border-[var(--td-component-stroke)] shadow-[0_0_0_0.5px_rgba(0,0,0,0.03),0_2px_6px_rgba(0,0,0,0.08)] ring-0 dark:border-[rgba(255,255,255,0.08)] dark:bg-[rgba(36,36,36,0.92)] dark:shadow-[0_0_0_0.5px_rgba(255,255,255,0.05),0_2px_6px_rgba(0,0,0,0.2)]"
        :class="menuMode === 'menu' ? 'w-max min-w-[168px] p-1' : 'w-[260px] min-w-[260px] p-3'"
        @click.stop
      >
        <div v-if="menuMode === 'menu'" class="flex min-w-[160px] flex-col gap-px">
          <button
            type="button"
            data-slot="chat-header-menu-item"
            :class="menuItemClass"
            @click="onMenuAction(session?.is_pinned ? 'unpin' : 'pin')"
          >
            <PinIcon
              class="text-muted-foreground size-4 flex-none"
              :class="{ 'fill-current': session?.is_pinned }"
              aria-hidden="true"
            />
            <span>{{ session?.is_pinned ? t("menu.unpin") : t("menu.pin") }}</span>
          </button>
          <button
            type="button"
            data-slot="chat-header-menu-item"
            :class="menuItemClass"
            @click="onMenuAction('rename')"
          >
            <SquarePenIcon class="text-muted-foreground size-4 flex-none" aria-hidden="true" />
            <span>{{ t("menu.renameSession") }}</span>
          </button>
          <div class="mx-1.5 my-0.5 h-px bg-[var(--td-component-stroke)]" />
          <button
            type="button"
            data-slot="chat-header-menu-item"
            :class="menuItemClass"
            @click="onMenuAction('copyId')"
          >
            <CopyIcon class="text-muted-foreground size-4 flex-none" aria-hidden="true" />
            <span>{{ t("chatHeader.copySessionId") }}</span>
          </button>
          <button
            type="button"
            data-slot="chat-header-menu-item"
            :class="menuItemClass"
            @click="onMenuAction('copyLink')"
          >
            <LinkIcon class="text-muted-foreground size-4 flex-none" aria-hidden="true" />
            <span>{{ t("chatHeader.copyLink") }}</span>
          </button>
          <button
            type="button"
            data-slot="chat-header-menu-item"
            :class="menuItemClass"
            @click="onMenuAction('copyMarkdown')"
          >
            <FilesIcon class="text-muted-foreground size-4 flex-none" aria-hidden="true" />
            <span>{{ t("chatHeader.copyMarkdown") }}</span>
          </button>
          <button
            type="button"
            data-slot="chat-header-menu-item"
            :class="menuItemClass"
            @click="onMenuAction('openNewWindow')"
          >
            <AppWindowIcon class="text-muted-foreground size-4 flex-none" aria-hidden="true" />
            <span>{{ t("chatHeader.openNewWindow") }}</span>
          </button>
          <div class="mx-1.5 my-0.5 h-px bg-[var(--td-component-stroke)]" />
          <button
            type="button"
            data-slot="chat-header-menu-item"
            :class="menuItemClass"
            @click="enterConfirmMode('clear')"
          >
            <EraserIcon class="text-muted-foreground size-4 flex-none" aria-hidden="true" />
            <span>{{ t("menu.clearMessages") }}</span>
          </button>
          <button
            type="button"
            data-slot="chat-header-menu-item"
            :class="cn(menuItemClass, 'text-destructive hover:bg-[var(--td-error-color-1)]')"
            @click="enterConfirmMode('delete')"
          >
            <Trash2Icon class="text-destructive size-4 flex-none" aria-hidden="true" />
            <span>{{ t("chatHeader.deleteSession") }}</span>
          </button>
        </div>

        <div v-else class="flex w-[236px] flex-col gap-2.5">
          <div class="text-foreground m-0 text-[14px] leading-5 font-semibold">
            {{ menuMode === "clear" ? t("chatHeader.clearConfirmTitle") : t("chatHeader.deleteConfirmTitle") }}
          </div>
          <div class="text-muted-foreground text-[14px] leading-normal break-words">
            {{ menuMode === "clear" ? t("chatHeader.clearConfirmBody") : t("chatHeader.deleteConfirmBody") }}
          </div>
          <div class="mt-0.5 flex justify-end gap-2">
            <button
              type="button"
              data-slot="chat-header-confirm-button"
              :class="[
                confirmButtonClass,
                'bg-card text-foreground enabled:hover:bg-accent border-[var(--td-component-stroke)]',
              ]"
              :disabled="Boolean(busyAction)"
              @click="backToMenu"
            >
              {{ t("common.cancel") }}
            </button>
            <button
              type="button"
              data-slot="chat-header-confirm-button"
              :class="[
                confirmButtonClass,
                'border-transparent bg-[var(--td-error-color-6)] text-white enabled:hover:bg-[var(--td-error-color-5)]',
              ]"
              :disabled="Boolean(busyAction)"
              @click="menuMode === 'clear' ? submitClearMessages() : submitDeleteSession()"
            >
              {{ menuMode === "clear" ? t("common.clear") : t("common.delete") }}
            </button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  </header>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import {
  AppWindowIcon,
  CopyIcon,
  EllipsisIcon,
  EraserIcon,
  FilesIcon,
  LinkIcon,
  Loader2Icon,
  PinIcon,
  SquarePenIcon,
  Trash2Icon,
} from "@lucide/vue";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import { copyToClipboard } from "@/utils/clipboard";
import { getMessageList } from "@/api/chat";
import { clearSession, removeSession, renameSession, setSessionPinned } from "./sessionMutations";
import { normalizeSessionTitleDraft, SESSION_TITLE_MAX_LENGTH } from "./sessionTitleEdit";
import { buildSessionMarkdown, collectAllSessionMessages } from "@/utils/sessionMarkdown";

interface ChatHeaderSession {
  id: string;
  title?: string;
  description?: string;
  tenant_id?: number | string;
  is_pinned?: boolean;
}

type MenuMode = "menu" | "clear" | "delete";

const props = defineProps<{
  session: ChatHeaderSession | null;
  hasReferencesPanel?: boolean;
}>();

const { t } = useI18n();
const busyAction = ref("");
const menuVisible = ref(false);
const menuMode = ref<MenuMode>("menu");
const titleEditing = ref(false);
const titleDraft = ref("");
const titleInputRef = ref<HTMLInputElement | null>(null);

const displayTitle = computed(() => props.session?.title?.trim() || t("menu.newSession"));
// Shared by every row of the action list, and by both confirmation buttons.
const menuItemClass =
  "box-border flex min-h-8 w-full items-center gap-2 rounded-[5px] px-3 text-left text-[14px] leading-5 " +
  "whitespace-nowrap text-foreground hover:bg-accent";
const confirmButtonClass =
  "h-[30px] min-w-[60px] rounded-[6px] border-[0.5px] border-solid px-3 text-[14px] leading-[28px] " +
  "transition-[background-color,color,border-color] duration-150 ease-in-out disabled:cursor-not-allowed disabled:opacity-55";

// The menu cannot open while there is no session or an action is running —
// the trigger is disabled then too, this guards the keyboard path. Closing
// it, however it closes, returns the card to the action list.
function onMenuOpenChange(open: boolean): void {
  if (open && (!props.session || busyAction.value)) return;
  menuVisible.value = open;
  if (!open) menuMode.value = "menu";
}

function enterConfirmMode(mode: "clear" | "delete"): void {
  menuMode.value = mode;
}

function backToMenu(): void {
  if (busyAction.value) return;
  menuMode.value = "menu";
}

function onMenuAction(value: string): void {
  if (value === "rename") {
    menuVisible.value = false;
    startTitleEdit();
    return;
  }
  menuVisible.value = false;
  handleMenuClick({ value });
}

async function copyText(text: string): Promise<void> {
  const ok = await copyToClipboard(text);
  if (!ok) throw new Error("clipboard unavailable");
}

function currentSessionLink(): string {
  const url = new URL(window.location.href);
  url.search = "";
  url.hash = "";
  return url.toString();
}

function startTitleEdit(): void {
  if (!props.session || busyAction.value) return;
  menuVisible.value = false;
  titleDraft.value = props.session.title || "";
  titleEditing.value = true;
  nextTick(() => {
    titleInputRef.value?.focus();
    titleInputRef.value?.select();
  });
}

function cancelTitleEdit(): void {
  titleEditing.value = false;
  titleDraft.value = "";
}

async function submitTitleEdit(): Promise<void> {
  // Enter 会先触发 form submit，随后 input blur 再进一次；必须同步退出编辑态防重入。
  if (!titleEditing.value || busyAction.value) return;
  const session = props.session;
  if (!session) {
    cancelTitleEdit();
    return;
  }

  const title = normalizeSessionTitleDraft(titleDraft.value);
  const currentTitle = normalizeSessionTitleDraft(session.title || "");
  titleEditing.value = false;
  titleDraft.value = "";
  if (!title || title === currentTitle) return;

  busyAction.value = "rename";
  try {
    await renameSession(session.id, title, session.description || "");
    MessagePlugin.success(t("menu.renameSessionSuccess"));
  } catch {
    MessagePlugin.error(t("menu.renameSessionFailed"));
  } finally {
    busyAction.value = "";
  }
}

async function togglePin(pinned: boolean): Promise<void> {
  const session = props.session;
  if (!session || busyAction.value) return;
  busyAction.value = "pin";
  try {
    await setSessionPinned(session.id, pinned);
    MessagePlugin.success(t(pinned ? "chatHeader.pinSuccess" : "chatHeader.unpinSuccess"));
  } catch {
    MessagePlugin.error(t(pinned ? "menu.pinFailed" : "menu.unpinFailed"));
  } finally {
    busyAction.value = "";
  }
}

async function copySessionId(): Promise<void> {
  if (!props.session) return;
  try {
    await copyText(props.session.id);
    MessagePlugin.success(t("chatHeader.sessionIdCopied"));
  } catch {
    MessagePlugin.error(t("chatHeader.copyFailed"));
  }
}

async function copyLink(): Promise<void> {
  try {
    await copyText(currentSessionLink());
    MessagePlugin.success(t("chatHeader.linkCopied"));
  } catch {
    MessagePlugin.error(t("chatHeader.copyFailed"));
  }
}

async function copyMarkdown(): Promise<void> {
  const session = props.session;
  if (!session || busyAction.value) return;
  busyAction.value = "markdown";
  try {
    const messages = await collectAllSessionMessages(async (beforeTime, limit) => {
      const response: any = await getMessageList({
        session_id: session.id,
        created_at: beforeTime,
        limit,
      });
      if (!response?.success || !Array.isArray(response.data)) {
        throw new Error(response?.message || "failed to load session messages");
      }
      return response.data;
    });
    const markdown = buildSessionMarkdown({
      sessionId: session.id,
      title: session.title || t("menu.newSession"),
      messages,
      labels: {
        sessionId: t("chatHeader.markdown.sessionId"),
        exportedAt: t("chatHeader.markdown.exportedAt"),
        user: t("chatHeader.markdown.user"),
        assistant: t("chatHeader.markdown.assistant"),
        attachments: t("chatHeader.markdown.attachments"),
        references: t("chatHeader.markdown.references"),
      },
    });
    await copyText(markdown);
    MessagePlugin.success(t("chatHeader.markdownCopied"));
  } catch {
    MessagePlugin.error(t("chatHeader.markdownCopyFailed"));
  } finally {
    busyAction.value = "";
  }
}

async function submitClearMessages(): Promise<void> {
  const session = props.session;
  if (!session || busyAction.value) return;
  busyAction.value = "clear";
  try {
    await clearSession(session.id);
    menuVisible.value = false;
    menuMode.value = "menu";
    MessagePlugin.success(t("menu.clearMessagesSuccess"));
  } catch {
    MessagePlugin.error(t("menu.clearMessagesFailed"));
  } finally {
    busyAction.value = "";
  }
}

async function submitDeleteSession(): Promise<void> {
  const session = props.session;
  if (!session || busyAction.value) return;
  busyAction.value = "delete";
  try {
    await removeSession(session.id);
    menuVisible.value = false;
    menuMode.value = "menu";
    MessagePlugin.success(t("chatHeader.deleteSuccess"));
  } catch {
    MessagePlugin.error(t("chat.deleteSessionFailed"));
  } finally {
    busyAction.value = "";
  }
}

function handleMenuClick(data: { value: string }): void {
  switch (data.value) {
    case "pin":
      void togglePin(true);
      break;
    case "unpin":
      void togglePin(false);
      break;
    case "copyId":
      void copySessionId();
      break;
    case "copyLink":
      void copyLink();
      break;
    case "copyMarkdown":
      void copyMarkdown();
      break;
    case "openNewWindow":
      window.open(currentSessionLink(), "_blank", "noopener,noreferrer");
      break;
  }
}
</script>
