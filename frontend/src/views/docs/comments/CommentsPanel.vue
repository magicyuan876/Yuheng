<template>
  <aside class="docs-comments" :aria-label="t('docs.comments.title')">
    <header class="docs-comments-head">
      <h3>
        {{ t("docs.comments.title") }}
        <span v-if="total" class="docs-comments-count">{{ open }}/{{ total }}</span>
      </h3>
      <label class="docs-comments-toggle">
        <input v-model="resolvedShown" type="checkbox" />
        {{ t("docs.comments.showResolved") }}
      </label>
    </header>

    <p v-if="loading && !threads.length" class="docs-comments-note">{{ t("common.loading") }}</p>
    <p v-else-if="!threads.length" class="docs-comments-note">{{ t("docs.comments.empty") }}</p>

    <!-- Comments that lost their place are grouped rather than scattered, so
         it is obvious they are a category and not one-off oddities. -->
    <template v-for="group in groups" :key="group.key">
      <h4 v-if="group.items.length && group.key !== 'inline'" class="docs-comments-group">
        {{ t(`docs.comments.group.${group.key}`) }}
      </h4>

      <article
        v-for="thread in group.items"
        :key="thread.id"
        class="docs-comments-thread"
        :class="{
          'is-active': thread.id === activeId,
          'is-resolved': !!thread.resolved_at,
          'is-orphaned': group.key === 'orphaned',
        }"
        @click="emit('select', thread.id)"
      >
        <p v-if="thread.quoted_text" class="docs-comments-quote">{{ thread.quoted_text }}</p>

        <CommentItem
          :comment="thread"
          :busy="busyId === thread.id"
          @edit="(body) => emit('edit', thread.id, body)"
          @delete="emit('delete', thread.id)"
        />

        <CommentItem
          v-for="reply in thread.replies ?? []"
          :key="reply.id"
          class="docs-comments-reply"
          :comment="reply"
          :busy="busyId === reply.id"
          @edit="(body) => emit('edit', reply.id, body)"
          @delete="emit('delete', reply.id)"
        />

        <footer class="docs-comments-actions" @click.stop>
          <CommentComposer
            v-if="replyingTo === thread.id"
            :placeholder="t('docs.comments.replyPlaceholder')"
            :busy="busyId === thread.id"
            @submit="(body) => submitReply(thread.id, body)"
            @cancel="replyingTo = ''"
          />
          <template v-else>
            <button type="button" class="docs-comments-action" @click="replyingTo = thread.id">
              {{ t("docs.comments.reply") }}
            </button>
            <button
              v-if="thread.can_resolve"
              type="button"
              class="docs-comments-action"
              @click="emit('resolve', thread.id, !thread.resolved_at)"
            >
              {{ thread.resolved_at ? t("docs.comments.reopen") : t("docs.comments.resolve") }}
            </button>
            <span v-if="thread.resolved_at" class="docs-comments-resolved">
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

const resolvedShown = computed({
  get: () => props.showResolved,
  set: (value: boolean) => emit("update:showResolved", value),
});

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

<style scoped lang="less">
.docs-comments {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  font-size: 13px;
}

.docs-comments-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;

  h3 {
    margin: 0;
    font-size: 14px;
  }
}

.docs-comments-count {
  margin-left: 4px;
  color: var(--td-text-color-placeholder);
  font-weight: 400;
  font-variant-numeric: tabular-nums;
}

.docs-comments-toggle {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
  cursor: pointer;
}

.docs-comments-note {
  margin: 8px 0;
  color: var(--td-text-color-placeholder);
}

.docs-comments-group {
  margin: 12px 0 4px;
  font-size: 12px;
  font-weight: 500;
  color: var(--td-text-color-placeholder);
}

.docs-comments-thread {
  padding: 8px 10px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container);
  cursor: pointer;

  &:hover {
    border-color: var(--td-brand-color-light-active);
  }

  &.is-active {
    border-color: var(--td-brand-color);
    box-shadow: 0 0 0 1px var(--td-brand-color);
  }

  &.is-resolved {
    opacity: 0.68;
  }

  &.is-orphaned {
    border-style: dashed;
  }
}

// The passage the comment was about, kept so an orphaned thread still says
// what it was answering.
.docs-comments-quote {
  margin: 0 0 6px;
  padding-left: 8px;
  border-left: 2px solid var(--td-warning-color-3);
  color: var(--td-text-color-secondary);
  font-size: 12px;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
}

.docs-comments-reply {
  margin-left: 12px;
  padding-left: 8px;
  border-left: 1px solid var(--td-component-stroke);
}

.docs-comments-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
}

.docs-comments-action {
  border: none;
  background: transparent;
  padding: 0;
  color: var(--td-brand-color);
  font-size: 12px;
  cursor: pointer;

  &:hover {
    text-decoration: underline;
  }
}

.docs-comments-resolved {
  font-size: 11px;
  color: var(--td-text-color-placeholder);
}
</style>
