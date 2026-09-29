<script setup lang="ts">
/**
 * The settings screens' text field: the shadcn Input plus the two TDesign
 * `t-input` features those screens relied on and the Input does not have —
 * a leading icon (`prefix-icon`) and a clear button (`clearable`).
 *
 * Everything not declared here (placeholder, type, maxlength, autocomplete,
 * listeners such as keydown) is forwarded to the <input> itself, so the field
 * behaves like a plain Input. `class` styles the wrapper (width, flex);
 * `input-class` reaches the <input> (font size, height).
 */
import type { Component, HTMLAttributes } from "vue";
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { XIcon } from "@lucide/vue";

import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

defineOptions({ inheritAttrs: false });

const props = withDefaults(
  defineProps<{
    modelValue?: string | null;
    clearable?: boolean;
    disabled?: boolean;
    prefixIcon?: Component;
    class?: HTMLAttributes["class"];
    inputClass?: HTMLAttributes["class"];
  }>(),
  {
    modelValue: "",
    clearable: false,
    disabled: false,
    prefixIcon: undefined,
    class: undefined,
    inputClass: undefined,
  },
);

const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
  (e: "clear"): void;
}>();

const { t } = useI18n();

// Like TDesign, the clear button exists only for a non-empty, editable field.
const showClear = computed(() => props.clearable && !props.disabled && !!props.modelValue);

const onInput = (value: string | number) => emit("update:modelValue", String(value));

const clear = () => {
  emit("update:modelValue", "");
  emit("clear");
};
</script>

<template>
  <div :class="cn('group/settings-input relative w-full', props.class)">
    <component
      :is="prefixIcon"
      v-if="prefixIcon"
      class="text-placeholder pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
    />
    <Input
      v-bind="$attrs"
      :model-value="modelValue ?? ''"
      :disabled="disabled"
      :class="cn(prefixIcon && 'pl-8', clearable && 'pr-8', inputClass)"
      @update:model-value="onInput"
    />
    <!-- TDesign revealed its clear icon on hover; keyboard focus reveals it too. -->
    <button
      v-if="showClear"
      type="button"
      data-slot="input-clear"
      class="text-placeholder hover:text-foreground absolute top-1/2 right-2 flex -translate-y-1/2 items-center opacity-0 transition-opacity group-focus-within/settings-input:opacity-100 group-hover/settings-input:opacity-100 focus-visible:opacity-100"
      :aria-label="t('common.clear')"
      @click="clear"
    >
      <XIcon class="size-3.5" />
    </button>
  </div>
</template>
