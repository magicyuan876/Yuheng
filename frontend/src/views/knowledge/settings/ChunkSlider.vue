<template>
  <!--
    The slider the chunking form used to get from t-slider: a track with a
    value readout beside it (above it when embedded) and, in the full layout,
    the tick labels ("marks") under the track. shadcn's Slider has no marks,
    so they are laid out here against the same min/max scale.
  -->
  <div class="flex w-full" :class="embedded ? 'flex-col items-stretch gap-2' : 'items-center justify-end gap-4'">
    <div class="relative" :class="[embedded ? 'w-full' : 'w-[200px] shrink-0', marks ? 'pb-6' : '']">
      <Slider
        :model-value="[modelValue]"
        :min="min"
        :max="max"
        :step="step"
        class="py-2"
        @update:model-value="(v) => v && v[0] !== undefined && emit('update:modelValue', v[0])"
      />
      <template v-if="marks">
        <span
          v-for="mark in marks"
          :key="mark"
          class="text-muted-foreground absolute top-6 -translate-x-1/2 text-xs whitespace-nowrap"
          :style="{ left: `${((mark - min) / (max - min)) * 100}%` }"
          >{{ mark }}</span
        >
      </template>
    </div>
    <span
      class="text-foreground text-sm font-medium"
      :class="embedded ? 'order-first min-w-0 text-left' : 'min-w-[80px] text-right'"
      >{{ modelValue }} {{ unit }}</span
    >
  </div>
</template>

<script setup lang="ts">
import { Slider } from "@/components/ui/slider";

defineProps<{
  modelValue: number;
  min: number;
  max: number;
  step: number;
  /** Tick values shown under the track; omitted in the compact layout. */
  marks?: number[];
  /** Compact, stacked layout of the upload dialog. */
  embedded?: boolean;
  /** Suffix of the value readout ("characters"). */
  unit: string;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: number];
}>();
</script>
