<script setup lang="ts">
/*
 * A button-style radio group, standing in for TDesign's `<t-radio-group>` of
 * `<t-radio-button>`s as the app themed them (theme.css restyles every
 * outline radio group): joined 32px segments sharing one border, the
 * checked one filled with the brand colour and lettered white. The shadcn
 * RadioGroup draws round radio dots instead, which would change how the
 * knowledge-base editor reads, so this composes Reka's radio primitives
 * directly — keeping roving focus, arrow-key selection and the radiogroup
 * semantics — and only restyles the items.
 */
import { RadioGroupItem, RadioGroupRoot } from "reka-ui";

export interface SegmentedRadioOption {
  value: string;
  label: string;
}

defineProps<{
  modelValue: string;
  options: SegmentedRadioOption[];
  disabled?: boolean;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
}>();

// Reka emits AcceptableValue; every option value here is a string.
function onUpdate(value: unknown) {
  emit("update:modelValue", String(value ?? ""));
}
</script>

<template>
  <RadioGroupRoot
    data-slot="segmented-radio"
    orientation="horizontal"
    class="inline-flex max-w-full flex-wrap items-center gap-y-1"
    :model-value="modelValue"
    :disabled="disabled"
    @update:model-value="onUpdate"
  >
    <!-- Each segment carries the full border and overlaps its left neighbour by
         a pixel, so the shared edges stay one pixel wide; the checked segment
         (and a hovered one) is lifted above its neighbours so its brand-coloured
         edge shows whole. -->
    <RadioGroupItem
      v-for="option in options"
      :key="option.value"
      :value="option.value"
      data-slot="segmented-radio-item"
      class="bg-card text-foreground relative -ml-px inline-flex h-8 items-center border border-solid border-[var(--td-component-stroke)] px-4 text-sm whitespace-nowrap transition-all duration-200 first:ml-0 first:rounded-l-[var(--td-radius-default)] last:rounded-r-[var(--td-radius-default)] not-data-[disabled]:hover:z-[1] not-data-[disabled]:hover:border-[var(--td-brand-color)] not-data-[disabled]:hover:text-[var(--td-brand-color)] data-[disabled]:cursor-not-allowed data-[disabled]:border-[var(--td-component-border)] data-[disabled]:bg-[var(--td-bg-color-component-disabled)] data-[disabled]:text-[var(--td-text-color-disabled)] data-[disabled]:opacity-60 data-[state=checked]:z-[1] data-[state=checked]:border-[var(--td-brand-color)] data-[state=checked]:bg-[var(--td-brand-color)] data-[state=checked]:text-[var(--td-text-color-anti)] data-[state=checked]:not-data-[disabled]:hover:border-[var(--td-brand-color-active)] data-[state=checked]:not-data-[disabled]:hover:bg-[var(--td-brand-color-active)] data-[state=checked]:not-data-[disabled]:hover:text-[var(--td-text-color-anti)] data-[disabled]:data-[state=checked]:border-[var(--td-brand-color-disabled)] data-[disabled]:data-[state=checked]:bg-[var(--td-brand-color-disabled)] data-[disabled]:data-[state=checked]:text-[var(--td-text-color-anti)]"
    >
      {{ option.label }}
    </RadioGroupItem>
  </RadioGroupRoot>
</template>
