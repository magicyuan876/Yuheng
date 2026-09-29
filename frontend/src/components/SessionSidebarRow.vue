<template>
  <div
    :class="[
      'submenu_item relative',
      !batchMode && activePath === item.path ? 'submenu_item_active' : '',
      batchMode && selectedIds.includes(item.id) ? 'submenu_item_selected' : '',
      batchMode ? 'submenu_item_batch' : '',
    ]"
    @mouseenter="emit('hover-in')"
    @mouseleave="emit('hover-out')"
    @click="batchMode ? emit('toggle-select') : emit('navigate')"
  >
    <Checkbox
      v-if="batchMode"
      class="batch-checkbox"
      :model-value="selectedIds.includes(item.id)"
      @click.stop
      @update:model-value="emit('toggle-select')"
    />
    <form v-if="titleEditing" class="min-w-0 flex-1" @submit.prevent="submitTitleEdit" @click.stop>
      <input
        ref="titleInputRef"
        v-model="titleDraft"
        data-slot="session-title-input"
        class="border-primary bg-card text-foreground h-[26px] w-full rounded-[5px] border px-2 text-sm leading-6 shadow-[0_0_0_2px_var(--td-brand-color-light)] outline-none"
        :maxlength="SESSION_TITLE_MAX_LENGTH"
        @keydown.esc.prevent="cancelTitleEdit"
        @blur="submitTitleEdit"
      />
    </form>
    <span v-else class="submenu_title" :class="batchMode ? 'submenu_title--batch' : ''" :title="item.title">
      <PinIcon v-if="item.is_pinned" class="submenu_pin_icon size-3" />
      <span class="submenu_title-text">{{ item.title }}</span>
    </span>
    <div v-if="!batchMode" class="relative flex-none" @click.stop>
      <Popover v-model:open="menuOpen" @update:open="onMenuOpenChange">
        <PopoverTrigger as-child>
          <!-- menu-more-wrap and menu-more are hook classes: menu.vue hides the button until its row
               is hovered or active, and colours the glyph, through :deep(). -->
          <button
            type="button"
            data-slot="session-row-more"
            class="menu-more-wrap hover:bg-accent inline-flex h-6 w-6 cursor-pointer items-center justify-center rounded-[5px] border-0 p-0 text-inherit transition-[background-color,color,opacity] duration-150"
            aria-haspopup="menu"
            :aria-expanded="menuOpen"
            @click.stop
          >
            <MoreHorizontalIcon class="menu-more size-3.5" />
          </button>
        </PopoverTrigger>
        <!-- The old popup's look: a hairline border instead of the ring, a softer shadow, and a
             darker translucent surface in dark mode. z-3000 keeps it above the sidebar's own
             stacking contexts, as the old overlay did. -->
        <PopoverContent
          class="z-3000 rounded-lg border-[0.5px] border-[var(--td-component-stroke)] bg-[var(--td-bg-color-container)] shadow-[0_0_0_0.5px_rgba(0,0,0,0.03),0_2px_6px_rgba(0,0,0,0.08)] ring-0 dark:border-white/8 dark:bg-[rgba(36,36,36,0.92)] dark:shadow-[0_0_0_0.5px_rgba(255,255,255,0.05),0_2px_6px_rgba(0,0,0,0.2)]"
          :class="menuMode === 'menu' ? 'w-max min-w-[160px] p-1' : 'w-[260px] min-w-[260px] p-3'"
          align="end"
          :side-offset="2"
        >
          <div @click.stop>
            <div v-if="menuMode === 'menu'" class="flex min-w-[152px] flex-col gap-px">
              <template v-for="(option, index) in menuOptions" :key="option.value">
                <div
                  v-if="shouldShowDividerBefore(option.value, index)"
                  class="mx-1.5 my-0.5 h-px bg-[var(--td-component-stroke)]"
                />
                <button
                  type="button"
                  class="flex min-h-8 w-full cursor-pointer items-center gap-2 rounded-[5px] border-0 px-3 text-left text-sm leading-5 whitespace-nowrap"
                  :class="
                    option.theme === 'error'
                      ? 'text-[var(--td-error-color-6)] hover:bg-[var(--td-error-color-1)] [&_svg]:text-[var(--td-error-color-6)]'
                      : 'text-foreground hover:bg-accent [&_svg]:text-muted-foreground'
                  "
                  @click="handleMenuClick(option)"
                >
                  <component
                    :is="option.prefixIcon"
                    v-if="option.prefixIcon"
                    class="inline-flex flex-none [&_svg]:size-4"
                  />
                  <span>{{ option.content }}</span>
                </button>
              </template>
            </div>

            <div v-else class="flex w-[236px] flex-col gap-2.5">
              <div class="text-foreground m-0 text-sm leading-5 font-semibold">
                {{ menuMode === "clear" ? t("chatHeader.clearConfirmTitle") : t("chatHeader.deleteConfirmTitle") }}
              </div>
              <div class="text-muted-foreground text-sm leading-normal break-words">
                {{ menuMode === "clear" ? t("chatHeader.clearConfirmBody") : t("chatHeader.deleteConfirmBody") }}
              </div>
              <div class="mt-0.5 flex justify-end gap-2">
                <button
                  type="button"
                  class="bg-card text-foreground hover:bg-accent h-[30px] min-w-[60px] cursor-pointer rounded-md border-[0.5px] border-[var(--td-component-stroke)] px-3 text-sm leading-7 transition-colors duration-150"
                  @click="backToMenu"
                >
                  {{ t("common.cancel") }}
                </button>
                <button
                  type="button"
                  class="h-[30px] min-w-[60px] cursor-pointer rounded-md border-[0.5px] border-transparent bg-[var(--td-error-color-6)] px-3 text-sm leading-7 text-white transition-colors duration-150 hover:bg-[var(--td-error-color-5)]"
                  @click="confirmDangerAction"
                >
                  {{ menuMode === "clear" ? t("common.clear") : t("common.delete") }}
                </button>
              </div>
            </div>
          </div>
        </PopoverContent>
      </Popover>
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, ref } from "vue";
import { useI18n } from "vue-i18n";

import { MoreHorizontalIcon, PinIcon } from "@lucide/vue";

import { Checkbox } from "@/components/ui/checkbox";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

import { normalizeSessionTitleDraft, SESSION_TITLE_MAX_LENGTH } from "./sessionTitleEdit";

interface SessionMenuOption {
  content: string;
  value: string;
  theme?: "default" | "success" | "warning" | "error" | "primary";
  prefixIcon?: any;
}

type MenuMode = "menu" | "clear" | "delete";

const props = defineProps<{
  item: { id: string; path: string; title: string; is_pinned?: boolean };
  batchMode: boolean;
  activePath: string;
  selectedIds: string[];
  menuOptions: SessionMenuOption[];
  /** 渠道文件夹下的会话（样式与聊天区会话共用文案列对齐） */
  nested?: boolean;
}>();

const emit = defineEmits<{
  (e: "navigate"): void;
  (e: "toggle-select"): void;
  (e: "menu-click", data: { value: string }): void;
  (e: "rename-submit", data: { title: string }): void;
  (e: "hover-in"): void;
  (e: "hover-out"): void;
}>();

const { t } = useI18n();

const menuOpen = ref(false);
const menuMode = ref<MenuMode>("menu");
const titleEditing = ref(false);
const titleDraft = ref("");
const titleInputRef = ref<HTMLInputElement | null>(null);

const onMenuOpenChange = (visible: boolean): void => {
  if (!visible) menuMode.value = "menu";
};

const backToMenu = (): void => {
  menuMode.value = "menu";
};

const shouldShowDividerBefore = (value: string, index: number): boolean => {
  if (index === 0) return false;
  return value === "clearMessages" || value === "delete";
};

const startTitleEdit = (): void => {
  menuOpen.value = false;
  menuMode.value = "menu";
  titleDraft.value = props.item.title || "";
  titleEditing.value = true;
  nextTick(() => {
    titleInputRef.value?.focus();
    titleInputRef.value?.select();
  });
};

const cancelTitleEdit = (): void => {
  titleEditing.value = false;
  titleDraft.value = "";
};

const submitTitleEdit = (): void => {
  // Enter 会先触发 form submit，随后 input blur 再进一次；必须同步退出编辑态防重入。
  if (!titleEditing.value) return;
  const nextTitle = normalizeSessionTitleDraft(titleDraft.value);
  const currentTitle = normalizeSessionTitleDraft(props.item.title || "");
  titleEditing.value = false;
  titleDraft.value = "";
  if (!nextTitle || nextTitle === currentTitle) return;
  emit("rename-submit", { title: nextTitle });
};

const handleMenuClick = (option: SessionMenuOption): void => {
  if (option.value === "rename") {
    startTitleEdit();
    return;
  }
  if (option.value === "clearMessages") {
    menuMode.value = "clear";
    return;
  }
  if (option.value === "delete") {
    menuMode.value = "delete";
    return;
  }
  menuOpen.value = false;
  menuMode.value = "menu";
  emit("menu-click", { value: option.value });
};

const confirmDangerAction = (): void => {
  const value = menuMode.value === "clear" ? "clearMessages" : "delete";
  menuOpen.value = false;
  menuMode.value = "menu";
  emit("menu-click", { value });
};
</script>
