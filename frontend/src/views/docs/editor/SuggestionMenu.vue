<template>
  <div
    v-if="open"
    ref="menu"
    class="bg-popover fixed z-1200 max-h-[360px] max-w-[360px] min-w-[240px] overflow-y-auto rounded-[8px] border border-[var(--td-component-stroke)] p-1 shadow-[var(--td-shadow-2)]"
    :style="{ left: `${position.left}px`, top: `${position.top}px` }"
    role="listbox"
  >
    <p v-if="loading" class="text-placeholder m-0 px-2.5 py-2 text-[13px]">{{ t("docs.links.searching") }}</p>
    <p v-else-if="!items.length" class="text-placeholder m-0 px-2.5 py-2 text-[13px]">
      {{ kind === "command" ? t("docs.commands.noMatches") : t("docs.links.noMatches") }}
    </p>
    <template v-for="(item, index) in items" :key="item.key">
      <p
        v-if="item.group && item.group !== items[index - 1]?.group"
        class="text-placeholder m-0 px-2 pt-1.5 pb-0.5 text-[11px]"
        aria-hidden="true"
      >
        {{ item.group }}
      </p>
      <button
        type="button"
        role="option"
        data-slot="suggestion-item"
        class="flex w-full cursor-pointer items-center gap-2 rounded-md border-0 px-2 py-1.5 text-left"
        :class="index === selected ? 'bg-accent' : ''"
        :aria-selected="index === selected"
        @mousedown.prevent="emit('choose', index)"
        @mouseenter="emit('hover', index)"
      >
        <!-- A badge such as "H1" stands in for an icon the set does not have, so it is drawn at
             the weight an icon reads at rather than as body text. -->
        <span
          class="text-placeholder inline-flex w-[18px] flex-none items-center justify-center text-center text-xs leading-none font-semibold"
        >
          <template v-if="item.icon">{{ item.icon }}</template>
          <component :is="editorIcon(item.iconName, fallbackIcon)" v-else class="size-3.5" />
        </span>
        <span class="flex min-w-0 flex-col">
          <span class="text-foreground truncate text-[13.5px]">{{ item.title }}</span>
          <span v-if="item.hint" class="text-placeholder truncate text-xs">{{ item.hint }}</span>
        </span>
      </button>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch, type Component } from "vue";
import { useI18n } from "vue-i18n";

import { FileIcon, UserRoundIcon } from "@lucide/vue";

import { editorIcon } from "./lucideIconMap";

/** One row of the menu, already shaped by whoever fetched it. */
export interface SuggestionItem {
  key: string;
  title: string;
  hint?: string;
  /**
   * A section heading shown above this row when it differs from the row
   * before it. Only the command menu groups its rows; already translated,
   * like everything else the row carries.
   */
  group?: string;
  /** An emoji chosen for the page itself, shown in preference to an icon. */
  icon?: string;
  /** A tdesign icon name, used when the entry has no emoji of its own. */
  iconName?: string;
}

const props = defineProps<{
  open: boolean;
  loading: boolean;
  items: SuggestionItem[];
  selected: number;
  kind: "page" | "mention" | "command" | "emoji";
  position: { left: number; top: number };
}>();

const emit = defineEmits<{ choose: [index: number]; hover: [index: number] }>();
const { t } = useI18n();

const menu = ref<HTMLElement | null>(null);

/** What to draw for an entry that brought no icon of its own. */
const fallbackIcon = computed<Component>(() => (props.kind === "mention" ? UserRoundIcon : FileIcon));

/**
 * The list outgrows the popup and only the first rows are visible, so an
 * arrow-key move has to bring the active row to the eye: nearest keeps a
 * move by one from scrolling the whole list when the row is already there.
 */
watch(
  () => props.selected,
  async () => {
    await nextTick();
    menu.value?.querySelector("[aria-selected='true']")?.scrollIntoView({ block: "nearest" });
  },
);
</script>
