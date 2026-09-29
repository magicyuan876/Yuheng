<template>
  <!-- The chat-citation-float class is a hook, not styling:
       useChatCitationPopover finds the open card with it to keep it alive
       while hovered and to ignore clicks inside it. -->
  <Teleport to="body">
    <div
      v-if="float.visible"
      class="chat-citation-float bg-card text-foreground absolute z-[10000] max-w-[320px] rounded-[8px] px-3 py-2.5 text-xs leading-normal shadow-[0_6px_18px_rgba(0,0,0,0.18)]"
      :style="{ top: `${float.top}px`, left: `${float.left}px` }"
      @mouseenter="onEnter?.()"
      @mouseleave="onLeave?.()"
    >
      <template v-if="float.type === 'web'">
        <div class="text-primary mb-1 font-semibold">{{ float.title || float.url }}</div>
        <a
          v-if="float.url"
          class="text-primary break-all"
          :href="float.url"
          target="_blank"
          rel="noopener noreferrer"
          >{{ float.url }}</a
        >
      </template>
      <template v-else>
        <div class="text-primary mb-1 font-semibold">{{ float.title }}</div>
        <div v-if="float.loading" class="text-muted-foreground">{{ loadingText }}</div>
        <div v-else-if="float.error" class="text-destructive">{{ float.error }}</div>
        <div v-else class="max-h-[200px] overflow-y-auto whitespace-pre-wrap">{{ float.content }}</div>
      </template>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";
import type { CitationFloatState } from "@/composables/useChatCitationPopover";

defineProps<{
  float: CitationFloatState;
  onEnter?: () => void;
  onLeave?: () => void;
}>();

const { t } = useI18n();
const loadingText = t("common.loading");
</script>
