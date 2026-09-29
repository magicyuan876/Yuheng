<template>
  <aside class="flex min-w-0 flex-col gap-2 text-[13px]" :aria-label="t('docs.comments.title')">
    <header class="flex items-baseline justify-between gap-2">
      <h3 class="m-0 text-[14px]">
        {{ t("docs.comments.title") }}
        <span v-if="total" class="text-placeholder ml-1 font-normal tabular-nums">{{ open }}/{{ total }}</span>
      </h3>
      <label class="text-muted-foreground inline-flex cursor-pointer items-center gap-1 text-xs">
        <!-- The checkbox reports "indeterminate" as well as true and false; only
             a plain tick means the resolved threads are shown. -->
        <Checkbox
          class="size-3.5"
          :model-value="showResolved"
          @update:model-value="(v) => emit('update:showResolved', v === true)"
        />
        {{ t("docs.comments.showResolved") }}
      </label>
    </header>

    <p v-if="loading && !threads.length" class="text-placeholder my-2">{{ t("common.loading") }}</p>
    <p v-else-if="!threads.length" class="text-placeholder my-2">{{ t("docs.comments.empty") }}</p>

    <!-- Comments that lost their place are grouped rather than scattered, so
         it is obvious they are a category and not one-off oddities. -->
    <template v-for="group in groups" :key="group.key">
      <h4 v-if="group.items.length && group.key !== 'inline'" class="text-placeholder mt-3 mb-1 text-xs font-medium">
        {{ t(`docs.comments.group.${group.key}`) }}
      </h4>

      <article
        v-for="thread in group.items"
        :key="thread.id"
        class="bg-card cursor-pointer rounded-[8px] border px-2.5 py-2"
        :class="[
          // The active thread keeps its brand outline under the pointer too,
          // so the hover tint applies only to the others.
          thread.id === activeId
            ? 'border-primary shadow-[0_0_0_1px_var(--td-brand-color)]'
            : 'border-border hover:border-[var(--td-brand-color-light-active)]',
          { 'opacity-[0.68]': !!thread.resolved_at, 'border-dashed': group.key === 'orphaned' },
        ]"
        @click="emit('select', thread.id)"
      >
        <!-- The passage the comment was about, kept so an orphaned thread
             still says what it was answering. -->
        <p
          v-if="thread.quoted_text"
          class="text-muted-foreground mt-0 mb-1.5 line-clamp-3 border-l-2 border-[var(--td-warning-color-3)] pl-2 text-xs"
        >
          {{ thread.quoted_text }}
        </p>

        <CommentItem
          :comment="thread"
          :busy="busyId === thread.id"
          @edit="(body) => emit('edit', thread.id, body)"
          @delete="emit('delete', thread.id)"
        />

        <CommentItem
          v-for="reply in thread.replies ?? []"
          :key="reply.id"
          class="border-border ml-3 border-l pl-2"
          :comment="reply"
          :busy="busyId === reply.id"
          @edit="(body) => emit('edit', reply.id, body)"
          @delete="emit('delete', reply.id)"
        />

        <footer class="mt-1.5 flex flex-wrap items-center gap-2" @click.stop>
          <CommentComposer
            v-if="replyingTo === thread.id"
            :placeholder="t('docs.comments.replyPlaceholder')"
            :busy="busyId === thread.id"
            @submit="(body) => submitReply(thread.id, body)"
            @cancel="replyingTo = ''"
          />
          <template v-else>
            <button
              type="button"
              data-slot="comment-action"
              class="text-primary text-xs hover:underline"
              @click="replyingTo = thread.id"
            >
              {{ t("docs.comments.reply") }}
            </button>
            <button
              v-if="thread.can_resolve"
              type="button"
              data-slot="comment-action"
              class="text-primary text-xs hover:underline"
              @click="emit('resolve', thread.id, !thread.resolved_at)"
            >
              {{ thread.resolved_at ? t("docs.comments.reopen") : t("docs.comments.resolve") }}
            </button>
            <span v-if="thread.resolved_at" class="text-placeholder text-[11px]">
              {{ t("docs.comments.resolvedBy", { name: displayName(thread.resolved_user) }) }}
            </span>
          </template>
        </footer>
      </article>
    </template>
  </aside>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

import type { CommentView } from "@/api/docs";
import { Checkbox } from "@/components/ui/checkbox";

import CommentComposer from "./CommentComposer.vue";
import CommentItem from "./CommentItem.vue";

const props = defineProps<{
  threads: CommentView[];
  grouped: { inline: CommentView[]; page: CommentView[]; orphaned: CommentView[] };
  open: number;
  total: number;
  loading: boolean;
  activeId: string;
  showResolved: boolean;
  /** The comment a request is in flight for, so its buttons can wait. */
  busyId?: string;
}>();

const emit = defineEmits<{
  select: [commentId: string];
  reply: [parentId: string, body: unknown];
  edit: [commentId: string, body: unknown];
  resolve: [commentId: string, resolved: boolean];
  delete: [commentId: string];
  "update:showResolved": [value: boolean];
}>();

const { t } = useI18n();
const replyingTo = ref("");

/**
 * The order the sidebar reads in: comments on passages first, top to bottom
 * as those passages appear, then comments about the page, then the ones whose
 * text is gone.
 */
const groups = computed(() => [
  { key: "inline" as const, items: props.grouped.inline },
  { key: "page" as const, items: props.grouped.page },
  { key: "orphaned" as const, items: props.grouped.orphaned },
]);

function displayName(user?: { username?: string; email?: string }): string {
  return user?.username || user?.email || t("docs.links.someone");
}

function submitReply(threadId: string, body: unknown) {
  emit("reply", threadId, body);
  replyingTo.value = "";
}

// A thread that went away takes the reply box with it.
watch(
  () => props.threads,
  (threads) => {
    if (replyingTo.value && !threads.some((thread) => thread.id === replyingTo.value)) {
      replyingTo.value = "";
    }
  },
);
</script>
