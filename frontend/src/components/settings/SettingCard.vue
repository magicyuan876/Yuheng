<template>
  <div
    class="flex min-w-0 flex-col gap-2 rounded-[8px] border border-[var(--td-component-stroke)] p-4 transition-[border-color,box-shadow,background-color] duration-200"
    :class="
      disabled
        ? 'bg-secondary hover:border-[var(--td-brand-color-light)] hover:shadow-none'
        : 'bg-card hover:border-primary hover:shadow-[0_2px_8px_rgba(0,0,0,0.05)]'
    "
  >
    <div class="flex min-w-0 items-center justify-between gap-2">
      <h3
        class="m-0 min-w-0 flex-1 truncate text-[15px] leading-[1.4] font-semibold"
        :class="disabled ? 'text-muted-foreground' : 'text-foreground'"
        :title="title"
      >
        {{ title }}
      </h3>
      <div class="flex shrink-0 items-center gap-1">
        <slot name="controls" />
        <DropdownMenu v-if="actions && actions.length > 0">
          <DropdownMenuTrigger as-child>
            <Button variant="ghost" size="icon-xs" class="text-placeholder hover:text-foreground hover:bg-secondary">
              <EllipsisVerticalIcon class="size-3.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem
              v-for="action in actions"
              :key="action.value"
              :class="action.theme === 'error' ? 'text-destructive focus:text-destructive' : ''"
              @select="emit('action', action.value)"
            >
              {{ action.content }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
    <div v-if="$slots.tags" class="flex min-h-5 flex-wrap items-center gap-1.5">
      <slot name="tags" />
    </div>
    <p
      v-if="description"
      class="text-muted-foreground m-0 [display:-webkit-box] overflow-hidden text-[13px] leading-normal break-all [-webkit-box-orient:vertical] [-webkit-line-clamp:2]"
    >
      {{ description }}
    </p>
    <div v-if="$slots.meta" class="text-placeholder flex flex-wrap items-center gap-3 text-xs">
      <slot name="meta" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { EllipsisVerticalIcon } from "@lucide/vue";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

interface DropdownOption {
  content: string;
  value: string;
  theme?: "default" | "success" | "warning" | "error" | "primary";
}

interface Props {
  title: string;
  description?: string;
  disabled?: boolean;
  actions?: DropdownOption[];
}

withDefaults(defineProps<Props>(), {
  description: "",
  disabled: false,
  actions: () => [],
});

const emit = defineEmits<{
  (e: "action", value: string): void;
}>();
</script>
