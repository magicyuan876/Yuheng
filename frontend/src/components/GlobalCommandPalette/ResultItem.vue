<template>
  <button
    type="button"
    data-slot="cmdk-item"
    class="text-foreground flex w-full cursor-pointer items-start gap-2.5 rounded-lg border-0 px-3 py-2 text-left transition-[background] duration-100"
    :class="selected ? 'bg-secondary' : ''"
    :data-cmdk-index="index"
    @click="$emit('primary')"
    @mousemove="onHover"
  >
    <div class="text-muted-foreground mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center">
      <slot name="icon">
        <component :is="paletteIcon(iconName, FileIcon)" class="size-3.5" />
      </slot>
    </div>
    <div class="min-w-0 flex-1">
      <div
        class="flex items-center gap-1.5 overflow-hidden text-[13px] leading-5 font-medium text-nowrap text-ellipsis whitespace-nowrap"
      >
        <slot name="title">
          <span v-html="title" />
        </slot>
        <span
          v-if="badge"
          class="shrink-0 rounded-[3px] px-[5px] py-px text-[10px] leading-[1.4] font-medium"
          :class="
            badgeVariant === 'vector'
              ? 'bg-primary/10 text-primary'
              : badgeVariant === 'keyword'
                ? 'bg-warning/10 text-warning'
                : 'bg-secondary text-muted-foreground'
          "
        >
          {{ badge }}
        </span>
        <span v-if="score != null" class="text-placeholder ml-auto shrink-0 text-[11px]"
          >{{ (score * 100).toFixed(0) }}%</span
        >
      </div>
      <div
        v-if="$slots.subtitle || subtitle"
        class="text-muted-foreground mt-0.5 [display:-webkit-box] overflow-hidden text-xs leading-[18px] break-words text-ellipsis [-webkit-box-orient:vertical] [-webkit-line-clamp:2]"
      >
        <slot name="subtitle">
          <span v-html="subtitle" />
        </slot>
      </div>
    </div>
    <div class="flex shrink-0 items-center gap-1">
      <slot name="actions" />
      <span
        v-if="shortcut"
        class="text-placeholder inline-flex items-center gap-0.5 text-[10px] transition-opacity duration-100"
        :class="selected ? 'opacity-100' : 'opacity-55'"
      >
        <kbd
          class="bg-secondary text-muted-foreground inline-block min-w-3.5 rounded-[3px] border border-[var(--td-component-stroke)] px-1 text-center font-[inherit] leading-[14px]"
          >⌘</kbd
        >
        <kbd
          class="bg-secondary text-muted-foreground inline-block min-w-3.5 rounded-[3px] border border-[var(--td-component-stroke)] px-1 text-center font-[inherit] leading-[14px]"
          >{{ shortcut }}</kbd
        >
      </span>
    </div>
  </button>
</template>

<script setup lang="ts">
import { FileIcon } from "@lucide/vue";

import { paletteIcon } from "./paletteIcons";

/**
 * A single result row inside the command palette.
 * Host component owns selection state; this one just mirrors it via the
 * `selected` prop and emits hover/primary events.
 */
defineProps<{
  index: number;
  selected?: boolean;
  iconName?: string;
  title?: string;
  subtitle?: string;
  badge?: string;
  badgeVariant?: "vector" | "keyword" | "default";
  score?: number;
  /** Visible hint for the ⌘N shortcut that triggers this row (N: 1-9). */
  shortcut?: number | string;
}>();

const emit = defineEmits<{
  (e: "primary"): void;
  (e: "hover", index: number): void;
}>();

const onHover = (e: MouseEvent) => {
  const idx = Number((e.currentTarget as HTMLElement).dataset.cmdkIndex);
  if (!Number.isNaN(idx)) emit("hover", idx);
};
</script>

<style scoped>
/* Highlight spans inside title/subtitle HTML, drawn by the host's matcher. */
:deep(.search-highlight) {
  background: rgba(255, 213, 0, 0.35);
  color: inherit;
  padding: 0 1px;
  border-radius: 2px;
}
</style>
