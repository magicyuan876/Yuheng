<template>
  <span class="inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs leading-[1.4]" :class="toneClass">
    <component :is="iconComponent" class="size-3.5" />
    <span>{{ displayName }}</span>
    <span v-if="engineType && (effectiveSource === 'user' || effectiveSource === 'env')" class="text-[11px] opacity-70">
      ({{ engineType }})
    </span>
    <!-- The old light, small, danger t-tag. -->
    <span
      v-if="isUnavailable"
      class="ml-1 inline-flex h-5 items-center rounded-[3px] bg-[var(--td-error-color-1)] px-1.5 text-xs text-[var(--td-error-color-6)]"
    >
      {{ $t("vectorStoreBadge.unavailable") }}
    </span>
  </span>
</template>

<script setup lang="ts">
import { computed, type Component } from "vue";
import { useI18n } from "vue-i18n";

import { CircleHelpIcon, DatabaseIcon } from "@lucide/vue";

import type { VectorStoreSource, VectorStoreStatus } from "@/api/knowledge-base";

const props = defineProps<{
  source?: VectorStoreSource;
  name?: string;
  engineType?: string;
  status?: VectorStoreStatus;
}>();

const { t } = useI18n();

// A caller without a source (the KB detail has not loaded yet, or the
// response carried no store view) gets the env look rather than a blank
// badge: a KB with no bound store runs on the env stores.
const effectiveSource = computed<VectorStoreSource>(() => props.source || "env");

const isUnavailable = computed(() => props.status === "unavailable" || effectiveSource.value === "unavailable");

const iconComponent = computed<Component>(() => {
  switch (effectiveSource.value) {
    case "env":
    case "user":
      // Both env- and user-bound KBs sit on top of a vector store; the
      // distinction is purely organizational (configured at process
      // start vs. created in the UI), so they share the same icon.
      return DatabaseIcon;
    case "unavailable":
    default:
      return CircleHelpIcon;
  }
});

// The tint per source. An unavailable store wears the error tint whatever its
// source says, because the warning is what the reader needs to see first.
const toneClass = computed(() => {
  if (isUnavailable.value) return "bg-[var(--td-error-color-1,#fde9e6)] text-[var(--td-error-color-7,#b32700)]";
  switch (effectiveSource.value) {
    case "env":
      return "bg-[var(--td-brand-color-1,#ecf2fe)] text-[var(--td-brand-color-7,#0052d9)]";
    case "user":
      return "bg-[var(--td-success-color-1,#e8f8f2)] text-[var(--td-success-color-7,#00754a)]";
    default:
      return "bg-[var(--td-bg-color-component,#f5f7fa)] text-foreground";
  }
});

const displayName = computed(() => {
  if (effectiveSource.value === "env") return t("vectorStoreBadge.systemDefault");
  return props.name || t("vectorStoreBadge.unknownStore");
});
</script>
