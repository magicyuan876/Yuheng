<template>
  <div class="docs-comment">
    <header class="docs-comment-head">
      <span class="docs-comment-author">{{ authorName }}</span>
      <time class="docs-comment-when" :datetime="comment.created_at">{{ when }}</time>
      <span v-if="comment.edited_at" class="docs-comment-edited">{{ t('docs.comments.edited') }}</span>

      <span class="docs-comment-spacer" />
      <template v-if="!editing">
        <button
          v-if="comment.can_edit"
          type="button"
          class="docs-comment-action"
          @click.stop="startEditing"
        >
          {{ t('common.edit') }}
        </button>
        <button
          v-if="comment.can_delete"
          type="button"
          class="docs-comment-action"
          @click.stop="confirmDelete"
        >
          {{ t('common.delete') }}
        </button>
      </template>
    </header>

    <CommentComposer
      v-if="editing"
      :initial="comment.body"
      :busy="busy"
      :submit-label="t('common.save')"
      @submit="commitEdit"
      @cancel="editing = false"
    />
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div v-else class="docs-comment-body" v-html="html" />
  </div>
</template>

<script setup lang="ts">
import { generateHTML } from '@tiptap/core'
import { DialogPlugin } from 'tdesign-vue-next'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { CommentView } from '@/api/docs'

import { officialExtensions } from '../editor/extensions'

import CommentComposer from './CommentComposer.vue'

const props = defineProps<{ comment: CommentView; busy?: boolean }>()
const emit = defineEmits<{ edit: [body: unknown]; delete: [] }>()
const { t, locale } = useI18n()

const editing = ref(false)

const authorName = computed(() =>
  props.comment.creator?.username || props.comment.creator?.email || t('docs.links.someone'))

const when = computed(() => {
  const at = new Date(props.comment.created_at)
  if (Number.isNaN(at.getTime())) return props.comment.created_at
  return at.toLocaleString(locale.value, { dateStyle: 'short', timeStyle: 'short' })
})

/**
 * The comment, rendered read-only.
 *
 * Markup rather than an editor instance, as everywhere else a stored document
 * is shown: a thread can hold dozens of these and none of them is editable
 * until somebody says so. Nothing in the markup can execute — the schema has
 * no script, iframe or style node type, and the server narrows a comment
 * further still.
 */
const html = computed(() => {
  const body = props.comment.body
  if (!body) return ''
  try {
    return generateHTML(body as Record<string, unknown>, officialExtensions() as never)
  } catch {
    return ''
  }
})

function startEditing() {
  editing.value = true
}

function commitEdit(body: unknown) {
  editing.value = false
  emit('edit', body)
}

function confirmDelete() {
  const dialog = DialogPlugin.confirm({
    header: t('docs.comments.deleteTitle'),
    body: props.comment.parent_id
      ? t('docs.comments.deleteConfirm')
      : t('docs.comments.deleteThreadConfirm'),
    confirmBtn: { content: t('common.delete'), theme: 'danger' },
    onConfirm: () => {
      dialog.hide()
      emit('delete')
    },
  })
}
</script>

<style scoped lang="less">
.docs-comment {
  padding: 4px 0;
}

.docs-comment-head {
  display: flex;
  align-items: baseline;
  gap: 6px;
  font-size: 12px;
}

.docs-comment-author {
  font-weight: 500;
  color: var(--td-text-color-primary);
}

.docs-comment-when,
.docs-comment-edited {
  color: var(--td-text-color-placeholder);
  font-size: 11px;
}

.docs-comment-spacer {
  flex: 1;
}

.docs-comment-action {
  border: none;
  background: transparent;
  padding: 0;
  color: var(--td-text-color-placeholder);
  font-size: 11px;
  cursor: pointer;

  &:hover {
    color: var(--td-brand-color);
  }
}

.docs-comment-body {
  margin-top: 2px;
  font-size: 13px;
  line-height: 1.6;
  word-break: break-word;

  :deep(p) {
    margin: 0 0 4px;
  }

  :deep(p:last-child) {
    margin-bottom: 0;
  }

  :deep(pre) {
    padding: 6px 8px;
    border-radius: 4px;
    background: var(--td-bg-color-container-hover);
    overflow-x: auto;
    font-size: 12px;
  }
}
</style>
