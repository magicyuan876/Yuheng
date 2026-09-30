<template>
  <!-- session-source-filter, --inline and --emphasized are hook classes: menu.vue reaches them with
       :deep() to fade the inline trigger in on hover and keep it visible for a non-default source. -->
  <div
    class="session-source-filter"
    :class="[
      inline ? 'session-source-filter--inline max-w-full min-w-0 p-0' : 'pt-0.5 pb-1.5',
      { 'session-source-filter--emphasized': emphasized },
    ]"
  >
    <button
      ref="triggerRef"
      type="button"
      data-slot="source-filter-trigger"
      class="flex cursor-pointer items-center border-0 text-left font-[family-name:var(--app-font-family)] transition-[background,color] duration-150"
      :class="
        inline
          ? 'hover:text-placeholder aria-expanded:text-placeholder w-auto max-w-full justify-end gap-0.5 rounded-none p-0 text-[var(--td-text-color-disabled)]'
          : 'text-muted-foreground hover:bg-accent hover:text-foreground aria-expanded:bg-accent aria-expanded:text-foreground min-h-7 w-full justify-between gap-2 rounded-md py-1 pr-2.5 pl-3.5'
      "
      :aria-expanded="open"
      aria-haspopup="listbox"
      @click.stop="toggleOpen"
    >
      <span class="inline-flex min-w-0 items-center" :class="inline ? 'flex-[0_1_auto] gap-1' : 'flex-auto gap-[5px]'">
        <img
          v-if="currentOption?.logo"
          :src="currentOption.logo"
          :alt="currentOption.label"
          class="flex-none object-contain"
          :class="inline ? 'h-3 w-3 opacity-70' : 'h-3.5 w-3.5 opacity-[0.82]'"
        />
        <component
          :is="iconFor(currentOption)"
          v-else
          class="flex-none"
          :class="inline ? 'size-3 text-[var(--td-text-color-disabled)]' : 'text-placeholder size-3.5'"
        />
        <span
          class="truncate tracking-[0.01em]"
          :class="inline ? 'text-[11px] leading-4 font-semibold' : 'text-xs leading-[18px] font-medium'"
          :title="currentOption?.label"
        >
          {{ currentOption?.label }}
        </span>
      </span>
      <ChevronDownIcon
        class="flex-none transition-[transform,color] duration-[180ms]"
        :class="[
          open ? 'rotate-180' : '',
          inline
            ? 'size-2.5 text-[var(--td-text-color-disabled)] opacity-85'
            : open
              ? 'text-muted-foreground size-3'
              : 'text-placeholder size-3',
        ]"
      />
    </button>
    <Teleport to="body">
      <div
        v-if="open"
        data-slot="source-filter-panel"
        class="fixed z-3000 w-max max-w-[min(200px,calc(100vw-16px))] min-w-[108px] rounded-[7px] border border-[var(--td-component-stroke)] bg-[var(--td-bg-color-sidebar,var(--td-bg-color-container))] p-[3px] shadow-[0_2px_10px_rgba(0,0,0,0.05),0_0_1px_rgba(0,0,0,0.04)]"
        :style="panelStyle"
        role="listbox"
        @click.stop
      >
        <button
          v-for="item in sources"
          :key="item.value"
          type="button"
          class="text-foreground flex min-h-7 w-full cursor-pointer items-center justify-between gap-1.5 rounded-[5px] border-0 px-1.5 py-1 text-left font-[family-name:var(--app-font-family)] whitespace-nowrap transition-[background,color] duration-150"
          :class="item.value === current ? 'bg-secondary' : 'hover:bg-accent'"
          role="option"
          :aria-selected="item.value === current"
          @click="handleSelect(item.value)"
        >
          <span class="inline-flex min-w-0 flex-auto items-center gap-[5px]">
            <img
              v-if="item.logo"
              :src="item.logo"
              :alt="item.label"
              class="h-3.5 w-3.5 flex-none object-contain"
              :class="item.value === current ? 'opacity-[0.92]' : 'opacity-[0.82]'"
            />
            <component
              :is="iconFor(item)"
              v-else
              class="size-3.5 flex-none"
              :class="item.value === current ? 'text-muted-foreground' : 'text-placeholder'"
            />
            <span class="truncate text-xs leading-4 font-medium tracking-[0.01em]" :title="item.label">{{
              item.label
            }}</span>
          </span>
          <CheckIcon
            class="text-placeholder ml-0.5 size-[13px] flex-[0_0_13px]"
            :class="item.value === current ? 'visible' : 'invisible'"
          />
        </button>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, type Component } from "vue";

import { CheckIcon, ChevronDownIcon, LinkIcon, MessageSquareIcon, ServerIcon } from "@lucide/vue";

import { DEFAULT_SESSION_BUCKET_KEY } from "./sessionSidebarSourceFilter";

interface SourceItem {
  value: string;
  label: string;
  logo?: string;
}

const props = defineProps<{
  sources: SourceItem[];
  current: string;
  /** 列表顶部的轻量文字触发器（无图标，右对齐） */
  inline?: boolean;
  /** 非默认来源时始终显示（便于切回网页对话） */
  emphasized?: boolean;
}>();

const emit = defineEmits<{
  (e: "select", value: string): void;
}>();

const PANEL_GAP = 4;
const VIEWPORT_MARGIN = 8;

const open = ref(false);
const triggerRef = ref<HTMLButtonElement | null>(null);
const panelStyle = ref<Record<string, string>>({});

const currentOption = computed(() => props.sources.find((item) => item.value === props.current) ?? props.sources[0]);

const iconFor = (item: SourceItem | undefined): Component => {
  if (!item) return MessageSquareIcon;
  if (item.value === DEFAULT_SESSION_BUCKET_KEY) return MessageSquareIcon;
  if (item.value === "api") return ServerIcon;
  return LinkIcon;
};

const updatePanelPosition = (): void => {
  const trigger = triggerRef.value;
  if (!trigger) return;
  const rect = trigger.getBoundingClientRect();
  if (props.inline) {
    panelStyle.value = {
      top: `${rect.bottom + PANEL_GAP}px`,
      right: `${Math.max(VIEWPORT_MARGIN, window.innerWidth - rect.right)}px`,
      left: "auto",
    };
    return;
  }
  const panelWidth = Math.min(Math.max(rect.width, 108), window.innerWidth - VIEWPORT_MARGIN * 2);
  const left = Math.max(VIEWPORT_MARGIN, Math.min(rect.left, window.innerWidth - panelWidth - VIEWPORT_MARGIN));
  panelStyle.value = {
    top: `${rect.bottom + PANEL_GAP}px`,
    left: `${left}px`,
    right: "auto",
    minWidth: `${panelWidth}px`,
  };
};

const removeListeners = (): void => {
  document.removeEventListener("click", close);
  window.removeEventListener("resize", close);
  window.removeEventListener("scroll", close, true);
};

const close = (): void => {
  open.value = false;
  removeListeners();
};

const toggleOpen = (): void => {
  if (open.value) {
    close();
    return;
  }
  updatePanelPosition();
  open.value = true;
  nextTick(() => {
    document.addEventListener("click", close);
    window.addEventListener("resize", close);
    window.addEventListener("scroll", close, true);
  });
};

const handleSelect = (value: string): void => {
  close();
  if (value === props.current) return;
  emit("select", value);
};

onBeforeUnmount(() => {
  removeListeners();
});
</script>
