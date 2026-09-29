<template>
  <Popover>
    <PopoverTrigger as-child>
      <slot />
    </PopoverTrigger>
    <!-- The max-height mirrors the info popover's cap so both header surfaces stay inside the
         viewport on shorter laptops. -->
    <PopoverContent
      class="block max-h-[min(60vh,420px)] w-auto max-w-[320px] min-w-[220px] overflow-hidden p-1.5"
      align="start"
    >
      <div class="flex max-h-[calc(min(60vh,420px)-12px)] flex-col gap-px overflow-y-auto">
        <button
          v-for="item in sortedList"
          :key="item.id"
          type="button"
          class="flex cursor-pointer items-center gap-2 rounded-md border-0 px-2.5 py-1.5 text-left text-[13px] leading-[1.4] transition-[background,color] duration-150"
          :class="
            item.id === currentKbId
              ? 'text-primary bg-[var(--td-brand-color-light)] font-medium'
              : 'text-foreground hover:bg-secondary'
          "
          @click="handleSelect(item.id)"
        >
          <component
            :is="iconFor(item.type)"
            class="size-4 flex-none"
            :class="item.id === currentKbId ? 'text-primary' : 'text-placeholder'"
          />
          <span class="min-w-0 flex-1 truncate" :title="item.name">{{ item.name }}</span>
          <CheckIcon v-if="item.id === currentKbId" class="text-primary size-3.5 flex-none" />
        </button>
        <div v-if="!sortedList.length" class="text-placeholder p-4 text-center text-xs">
          {{ t("common.noData") }}
        </div>
      </div>
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { computed, type Component } from "vue";
import { useI18n } from "vue-i18n";

import { CheckIcon, FolderIcon, MessageCircleQuestionMarkIcon } from "@lucide/vue";

import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

interface KBEntry {
  id: string;
  name: string;
  type?: string;
}

const props = defineProps<{
  kbList: KBEntry[];
  currentKbId: string;
}>();

const emit = defineEmits<{
  (e: "select", kbId: string): void;
}>();

const { t } = useI18n();

// Sort the list with the current KB pinned to the top so users always
// see "where they are" without scrolling. The rest preserves the
// caller's order (typically "mine first, then shared"); we don't
// re-sort alphabetically because that loses the recency / share-source
// signal embedded in the input order.
const sortedList = computed<KBEntry[]>(() => {
  const all = props.kbList || [];
  const current = all.find((kb) => kb.id === props.currentKbId);
  if (!current) return all;
  return [current, ...all.filter((kb) => kb.id !== props.currentKbId)];
});

const iconFor = (type?: string): Component => {
  if (type === "faq") return MessageCircleQuestionMarkIcon;
  return FolderIcon;
};

const handleSelect = (id: string): void => {
  if (id === props.currentKbId) return;
  emit("select", id);
};
</script>
