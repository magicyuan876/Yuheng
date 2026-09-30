<template>
  <!-- Was the answer helpful? Shown only for an answer that cites documents
       of the knowledge base, because only then does "not helpful" go
       anywhere: to the knowledge health of the documents cited, for their
       owners to look at. The popover says what is shown and to whom. -->
  <button
    type="button"
    data-slot="answer-toolbar-button"
    data-testid="answer-feedback-up"
    :class="[BUTTON, rating === 'up' ? 'text-primary' : 'text-muted-foreground']"
    :title="t('chat.feedback.helpful')"
    :aria-label="t('chat.feedback.helpful')"
    :aria-pressed="rating === 'up'"
    :disabled="busy"
    @click.stop="rate('up')"
  >
    <ThumbsUpIcon class="size-4 stroke-[1.2]" :class="rating === 'up' ? 'fill-current' : ''" aria-hidden="true" />
  </button>
  <Popover v-model:open="open">
    <!-- Anchored rather than triggered: pressing "not helpful" either opens
         the form or, pressed already, takes the feedback back, and the
         component decides which. -->
    <PopoverAnchor as-child>
      <button
        type="button"
        data-slot="answer-toolbar-button"
        data-testid="answer-feedback-down"
        :class="[BUTTON, rating === 'down' ? 'text-warning' : 'text-muted-foreground']"
        :title="t('chat.feedback.notHelpful')"
        :aria-label="t('chat.feedback.notHelpful')"
        :aria-pressed="rating === 'down'"
        :disabled="busy"
        @click.stop="onDown"
      >
        <ThumbsDownIcon
          class="size-4 stroke-[1.2]"
          :class="rating === 'down' ? 'fill-current' : ''"
          aria-hidden="true"
        />
      </button>
    </PopoverAnchor>
    <PopoverContent align="start" class="w-80 p-3" data-testid="answer-feedback-form">
      <div class="text-foreground text-sm font-medium">{{ t("chat.feedback.whatWasWrong") }}</div>
      <Textarea
        v-model="comment"
        class="mt-2 min-h-20 text-sm"
        :maxlength="MAX_COMMENT"
        :placeholder="t('chat.feedback.commentPlaceholder')"
        data-testid="answer-feedback-comment"
      />
      <label class="text-muted-foreground mt-2 flex items-center gap-2 text-xs">
        <Checkbox v-model="shareQuestion" data-testid="answer-feedback-share" />
        {{ t("chat.feedback.shareQuestion") }}
      </label>
      <p class="text-placeholder m-0 mt-2 text-xs">{{ t("chat.feedback.whoSees") }}</p>
      <div class="mt-3 flex justify-end gap-2">
        <Button variant="ghost" size="sm" @click="open = false">{{ t("common.cancel") }}</Button>
        <Button size="sm" :disabled="busy" data-testid="answer-feedback-send" @click="sendDown">
          {{ t("chat.feedback.send") }}
        </Button>
      </div>
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import { ThumbsDownIcon, ThumbsUpIcon } from "@lucide/vue";

import type { AnswerRating } from "@/api/feedback";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Popover, PopoverAnchor, PopoverContent } from "@/components/ui/popover";
import { Textarea } from "@/components/ui/textarea";
import { useAnswerFeedbackStore } from "@/stores/answerFeedback";

const props = defineProps<{ sessionId: string; messageId: string }>();

const { t } = useI18n();
const store = useAnswerFeedbackStore();

// The look of the toolbar's other buttons.
const BUTTON =
  "hover:bg-accent hover:text-foreground inline-flex size-[30px] shrink-0 items-center justify-center rounded-[8px] transition-[background-color,color] duration-150 ease-in-out active:bg-[var(--td-bg-color-container-active)] disabled:opacity-50";
const MAX_COMMENT = 1000;

const open = ref(false);
const busy = ref(false);
const comment = ref("");
const shareQuestion = ref(false);

const saved = computed(() => store.get(props.messageId));
const rating = computed<AnswerRating | "">(() => saved.value?.rating ?? "");

onMounted(() => void store.ensure(props.sessionId));
watch(
  () => props.sessionId,
  (id) => void store.ensure(id),
);

async function save(next: AnswerRating | "", body: { comment?: string; share_question?: boolean } = {}) {
  if (busy.value) return;
  busy.value = true;
  try {
    await store.set(props.sessionId, props.messageId, { rating: next, ...body });
    if (next === "down") void MessagePlugin.success(t("chat.feedback.sent"));
  } catch (err) {
    const msg = err instanceof Error ? err.message : "";
    void MessagePlugin.error(msg ? `${t("chat.feedback.failed")}: ${msg}` : t("chat.feedback.failed"));
  } finally {
    busy.value = false;
  }
}

/** Pressing the pressed button takes the feedback back. */
const rate = (next: AnswerRating) => save(rating.value === next ? "" : next);

/** "Not helpful" asks why before it is sent; pressed already, it is taken
 * back instead. */
function onDown() {
  if (rating.value === "down") {
    open.value = false;
    void save("");
    return;
  }
  comment.value = "";
  shareQuestion.value = false;
  open.value = true;
}

async function sendDown() {
  await save("down", { comment: comment.value.trim(), share_question: shareQuestion.value });
  open.value = false;
}
</script>
