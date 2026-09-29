<template>
  <Popover v-model:open="visible">
    <PopoverTrigger as-child>
      <slot />
    </PopoverTrigger>
    <PopoverContent class="w-auto max-w-[240px] min-w-[180px] overflow-hidden p-0" align="start">
      <div class="flex flex-col overflow-hidden p-0">
        <div class="text-placeholder border-border border-b px-3 pt-2 pb-1.5 text-[11px] font-semibold select-none">
          {{ headerLabel }}
        </div>
        <div class="flex flex-col gap-px p-1.5">
          <button
            v-for="item in modes"
            :key="item.value"
            type="button"
            class="hover:bg-secondary flex w-full cursor-pointer items-center gap-2 rounded-md border-0 px-2.5 py-1.5 text-left text-[13px] leading-[1.4] transition-[background,color] duration-150"
            :class="item.value === current ? 'text-primary font-medium' : 'text-foreground'"
            @click="handleSelect(item.value)"
          >
            <span class="flex-1">{{ item.label }}</span>
            <CheckIcon v-if="item.value === current" class="text-primary size-3.5 flex-none" />
          </button>
        </div>
      </div>
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { ref } from "vue";

import { CheckIcon } from "@lucide/vue";

import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

import type { SessionGroupMode } from "./sessionGrouping";

interface GroupModeItem {
  value: SessionGroupMode;
  label: string;
}

defineProps<{
  modes: GroupModeItem[];
  current: SessionGroupMode;
  headerLabel: string;
}>();

const emit = defineEmits<{
  (e: "select", value: SessionGroupMode): void;
}>();

const visible = ref(false);

const handleSelect = (value: SessionGroupMode): void => {
  visible.value = false;
  emit("select", value);
};
</script>
