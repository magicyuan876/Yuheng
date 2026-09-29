<template>
  <Popover v-model:open="visible">
    <PopoverTrigger as-child>
      <slot />
    </PopoverTrigger>
    <!-- Mirrors KBSwitcherDropdown so both scope switchers stay visually identical. -->
    <PopoverContent
      class="block max-h-[min(60vh,420px)] w-auto max-w-[300px] min-w-[200px] overflow-hidden p-1.5"
      align="start"
    >
      <div class="flex max-h-[calc(min(60vh,420px)-12px)] flex-col gap-px overflow-y-auto">
        <button
          v-for="item in sortedList"
          :key="item.value"
          type="button"
          class="flex cursor-pointer items-center gap-2 rounded-md border-0 px-2.5 py-1.5 text-left text-[13px] leading-[1.4] transition-[background,color] duration-150"
          :class="
            item.value === current
              ? 'text-primary bg-[var(--td-brand-color-light)] font-medium'
              : 'text-foreground hover:bg-secondary'
          "
          @click="handleSelect(item.value)"
        >
          <img
            v-if="item.logo"
            :src="item.logo"
            :alt="item.label"
            class="h-4 w-4 flex-none rounded-[3px] object-contain"
          />
          <UserRoundIcon
            v-else
            class="size-4 flex-none"
            :class="item.value === current ? 'text-primary' : 'text-placeholder'"
          />
          <span class="min-w-0 flex-1 truncate" :title="item.label">{{ item.label }}</span>
          <CheckIcon v-if="item.value === current" class="text-primary size-3.5 flex-none" />
        </button>
      </div>
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";

import { CheckIcon, UserRoundIcon } from "@lucide/vue";

import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

// A dumb scope switcher mirroring KBSwitcherDropdown's visual grammar. Logos are
// pre-resolved by the caller; web (no logo) falls back to a generic icon.
interface SourceItem {
  value: string;
  label: string;
  logo?: string;
}

const props = defineProps<{
  sources: SourceItem[];
  current: string;
}>();

const emit = defineEmits<{
  (e: "select", value: string): void;
}>();

// The popover only closes on outside click; selecting an item must close it
// explicitly, otherwise the panel lingers and overlays the list below
// (switching source reloads in place, so nothing else dismisses it).
const visible = ref(false);

// Pin the current source to the top so users always see "where they are" without
// scrolling — same as KBSwitcherDropdown. The rest keeps the caller's order
// (web first, then configured platforms).
const sortedList = computed<SourceItem[]>(() => {
  const all = props.sources || [];
  const current = all.find((s) => s.value === props.current);
  if (!current) return all;
  return [current, ...all.filter((s) => s.value !== props.current)];
});

const handleSelect = (value: string): void => {
  visible.value = false;
  if (value === props.current) return;
  emit("select", value);
};
</script>
