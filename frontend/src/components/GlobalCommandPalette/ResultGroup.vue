<template>
  <div
    v-if="$slots.default"
    class="flex flex-col gap-0.5 py-1 [&+&]:mt-1 [&+&]:border-t [&+&]:border-[var(--td-component-stroke)] [&+&]:pt-2"
  >
    <div
      class="text-placeholder flex items-center gap-1.5 px-3 pt-1 pb-0.5 text-[11px] font-medium tracking-[0.3px] uppercase"
    >
      <span>{{ label }}</span>
      <span v-if="count != null" class="font-normal text-[var(--td-text-color-disabled)]">({{ count }})</span>
      <span v-if="action" class="flex-1" />
      <button
        v-if="action"
        type="button"
        data-slot="group-action"
        class="text-primary hover:bg-primary/8 cursor-pointer rounded border-0 bg-transparent px-1.5 py-0.5 text-[11px] tracking-normal normal-case"
        @click="$emit('action')"
      >
        {{ action }}
      </button>
    </div>
    <div class="flex flex-col">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  label: string;
  count?: number;
  action?: string;
}>();

defineEmits<{
  (e: "action"): void;
}>();
</script>
