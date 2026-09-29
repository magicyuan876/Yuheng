<template>
  <transition name="follow-up-card">
    <div
      v-if="suggestionSet?.status === 'ready'"
      class="follow-ups bg-muted mt-[-4px] mr-auto mb-7 w-full max-w-[720px] rounded-[12px] border border-solid border-[var(--td-component-stroke)] p-3"
      aria-live="polite"
    >
      <div class="text-muted-foreground mb-2 flex items-center justify-between text-[13px] font-semibold">
        <span class="follow-ups__title inline-flex items-center gap-1.5">
          <LightbulbIcon class="text-placeholder size-3.5 shrink-0" aria-hidden="true" />
          <span>{{ t("chat.followUpQuestions") }}</span>
        </span>
        <div class="flex gap-1">
          <!-- The spinner replaces the refresh glyph while a regeneration is
               in flight; the old list stays visible underneath until the new
               set arrives. -->
          <button
            v-if="allowRegenerate"
            type="button"
            data-slot="follow-ups-action"
            :class="actionClass"
            :disabled="loading"
            @click="emit('regenerate')"
          >
            <Loader2Icon v-if="loading" class="size-[13px] animate-spin" aria-hidden="true" />
            <RefreshCwIcon v-else class="size-[13px]" aria-hidden="true" />
            <span>{{ t("chat.refreshSuggestedQuestions") }}</span>
          </button>
          <button
            type="button"
            data-slot="follow-ups-action"
            :class="actionClass"
            :aria-label="t('common.close')"
            @click="dismiss"
          >
            <XIcon class="size-[13px]" aria-hidden="true" />
          </button>
        </div>
      </div>
      <div class="follow-ups__list flex flex-col gap-1.5">
        <!-- The hover border, background and shadow repeat the starter chips'
             hover in suggested-questions.css, so both kinds of suggestion
             react alike. -->
        <button
          v-for="item in suggestionSet?.questions || []"
          :key="item.id"
          type="button"
          data-slot="follow-ups-item"
          class="follow-ups__item group bg-card text-foreground flex w-full items-center justify-between gap-3 rounded-[8px] border border-solid border-[var(--td-component-stroke)] px-[11px] py-[9px] text-left text-[13px] shadow-[0_1px_2px_rgba(0,0,0,0.04)] transition-[border-color,box-shadow,background] duration-200 ease-in-out hover:border-[color-mix(in_srgb,var(--td-text-color-primary)_10%,var(--td-component-stroke))] hover:bg-[color-mix(in_srgb,var(--td-text-color-primary)_4%,var(--td-bg-color-container))] hover:shadow-[0_2px_6px_rgba(0,0,0,0.05)]"
          @click="emit('select', item)"
        >
          <span>{{ item.text }}</span>
          <ArrowUpRightIcon class="group-hover:text-muted-foreground size-[13px] shrink-0" aria-hidden="true" />
        </button>
      </div>
    </div>
  </transition>
</template>

<script setup lang="ts">
import { watch } from "vue";
import { ArrowUpRightIcon, LightbulbIcon, Loader2Icon, RefreshCwIcon, XIcon } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import type { MessageSuggestionItem, MessageSuggestionSet } from "@/api/message-suggestion";

const props = defineProps<{
  suggestionSet?: MessageSuggestionSet | null;
  loading?: boolean;
  allowRegenerate?: boolean;
}>();
const emit = defineEmits<{
  (event: "select", item: MessageSuggestionItem): void;
  (event: "regenerate"): void;
  (event: "impression", set: MessageSuggestionSet): void;
  (event: "dismiss", set: MessageSuggestionSet): void;
}>();
const { t } = useI18n();
const impressed = new Set<string>();

watch(
  () => props.suggestionSet,
  (set) => {
    if (set?.status === "ready" && set.questions.length > 0 && !impressed.has(set.id)) {
      impressed.add(set.id);
      emit("impression", set);
    }
  },
  { immediate: true },
);

// Shared by the regenerate and close buttons in the header.
const actionClass =
  "inline-flex items-center gap-1 rounded-[6px] px-2 py-1 text-[13px] font-normal text-muted-foreground " +
  "transition-[background-color,color] duration-200 enabled:hover:bg-[var(--td-bg-color-container-hover,rgba(0,0,0,0.06))] " +
  "enabled:hover:text-primary disabled:cursor-not-allowed disabled:opacity-60";

const dismiss = () => {
  if (props.suggestionSet) emit("dismiss", props.suggestionSet);
};
</script>

<style scoped>
/*
 * Stays CSS: these are the <transition name="follow-up-card"> classes Vue
 * adds while the card expands, with a clip-path reveal and per-property
 * delays that utility classes cannot express.
 */
.follow-up-card-enter-active {
  transform-origin: left top;
  transition:
    opacity 0.22s ease 0.04s,
    transform 0.28s cubic-bezier(0.22, 0.61, 0.36, 1) 0.04s,
    clip-path 0.28s cubic-bezier(0.22, 0.61, 0.36, 1) 0.04s;
  will-change: opacity, transform, clip-path;
}
.follow-up-card-enter-from {
  opacity: 0;
  transform: translateY(-7px) scale(0.985);
  clip-path: inset(0 0 55% 0 round 12px);
}
.follow-up-card-enter-to {
  opacity: 1;
  transform: translateY(0) scale(1);
  clip-path: inset(0 0 0 0 round 12px);
}

@media (prefers-reduced-motion: reduce) {
  .follow-up-card-enter-active {
    transition: none;
  }
}
</style>
