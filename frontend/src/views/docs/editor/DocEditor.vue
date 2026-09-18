<template>
  <div class="doc-editor" :class="{ 'doc-editor--readonly': !editorEditable }">
    <div v-if="banner.kind !== 'none'" class="doc-editor-banner" :class="`doc-editor-banner--${banner.kind}`">
      <t-icon :name="bannerIcon" size="14px" />
      <span>{{ bannerText }}</span>
    </div>

    <div class="doc-editor-toolbar-row">
      <div class="doc-editor-online">
        <template v-if="onlineUsers.length">
          <t-tooltip v-for="u in onlineUsers.slice(0, 6)" :key="u.id" :content="u.name">
            <span class="online-avatar" :style="{ background: u.color }">
              <img v-if="u.avatar" :src="u.avatar" :alt="u.name" />
              <span v-else>{{ (u.name || '?').slice(0, 1).toUpperCase() }}</span>
            </span>
          </t-tooltip>
          <span v-if="onlineUsers.length > 6" class="online-more">+{{ onlineUsers.length - 6 }}</span>
        </template>
      </div>
      <div class="doc-editor-meta">
        <span>{{ t('docs.pages.wordCount', { count: wordCount }) }}</span>
      </div>
    </div>

    <div v-if="!collab.ready.value && props.collabUrl" class="doc-editor-loading">
      <t-skeleton animation="gradient" :row-col="[{ width: '90%' }, { width: '75%' }, { width: '85%' }]" />
    </div>
    <EditorContent v-else-if="editor" :editor="editor" class="doc-editor-content" />
  </div>
</template>

<script setup lang="ts">
import { Collaboration } from '@tiptap/extension-collaboration'
import { CollaborationCaret } from '@tiptap/extension-collaboration-caret'
import { EditorContent, useEditor } from '@tiptap/vue-3'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { awarenessUser, type UserLike } from './session'
import { officialExtensions } from './extensions'
import { extractHeadings } from './toc'
import { countDocument } from './wordCount'
import { useDocCollab } from './useDocCollab'

const props = defineProps<{
  pageId: string
  tenantId: string | number
  /** The page's own ACL answer (REST `can_edit`); the composable further
   * narrows this against the live collaboration connection. */
  canEdit: boolean
  collabUrl: string
  currentUser: UserLike | null
  getToken: () => string | null
}>()

const emit = defineEmits<{
  /** Fires once per heading-set change, letting the parent drive a TOC sidebar. */
  headings: [entries: { id: string; level: number; text: string; pos: number }[]]
}>()

const { t } = useI18n()

const pageIdRef = computed(() => props.pageId)
const tenantIdRef = computed(() => props.tenantId)
const canEditRef = computed(() => props.canEdit)
const collabUrlRef = computed(() => props.collabUrl)
const currentUserRef = computed(() => props.currentUser)

const collab = useDocCollab({
  pageId: pageIdRef,
  tenantId: tenantIdRef,
  canEditPage: canEditRef,
  collabUrl: collabUrlRef,
  currentUser: currentUserRef,
  getToken: props.getToken,
})

const editorEditable = collab.editable
const banner = collab.banner
const onlineUsers = collab.onlineUsers

const bannerIcon = computed(() => ({
  connecting: 'refresh',
  offline: 'error-circle',
  'read-only': 'lock-on',
  'permission-narrowed': 'lock-on',
  unavailable: 'info-circle',
  none: 'info-circle',
}[banner.value.kind]))

const bannerText = computed(() => {
  switch (banner.value.kind) {
    case 'connecting': return t('docs.pages.editorConnecting')
    case 'offline': return t('docs.pages.editorOffline')
    case 'read-only': return t('docs.pages.editorReadOnly')
    case 'permission-narrowed': return t('docs.pages.editorPermissionNarrowed')
    case 'unavailable': return t('docs.pages.editorUnavailable')
    default: return ''
  }
})

const wordCount = ref(0)

const editor = useEditor({
  editable: editorEditable.value,
  extensions: officialExtensions([
    Collaboration.configure({ document: collab.ydoc.value }),
    ...(collab.provider.value ? [CollaborationCaret.configure({
      provider: collab.provider.value,
      user: props.currentUser ? awarenessUser(props.currentUser) : { name: '', color: '#999999' },
    })] : []),
  ]),
  onUpdate: ({ editor: ed }) => {
    wordCount.value = countDocument(ed.state.doc).words
    emit('headings', extractHeadings(ed.state.doc))
  },
  onCreate: ({ editor: ed }) => {
    wordCount.value = countDocument(ed.state.doc).words
    emit('headings', extractHeadings(ed.state.doc))
  },
})

watch(editorEditable, (val) => {
  editor.value?.setEditable(val)
})

onBeforeUnmount(() => {
  editor.value?.destroy()
})

defineExpose({ editor, collab })
</script>

<style scoped lang="less">
.doc-editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.doc-editor-banner {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 12.5px;
  background: var(--td-warning-color-light, #fef3e6);
  color: var(--td-warning-color, #e37318);

  &--offline,
  &--connecting {
    background: var(--td-bg-color-secondarycontainer);
    color: var(--td-text-color-secondary);
  }

  &--read-only,
  &--unavailable {
    background: var(--td-bg-color-secondarycontainer);
    color: var(--td-text-color-placeholder);
  }
}

.doc-editor-toolbar-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 22px;
}

.doc-editor-online {
  display: flex;
  align-items: center;
  gap: 4px;
}

.online-avatar {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 11px;
  font-weight: 600;
  overflow: hidden;
  border: 2px solid var(--td-bg-color-container);
  margin-left: -6px;

  &:first-child {
    margin-left: 0;
  }

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
}

.online-more {
  font-size: 11px;
  color: var(--td-text-color-placeholder);
  margin-left: 4px;
}

.doc-editor-meta {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.doc-editor-loading {
  padding: 8px 0;
}

.doc-editor-content {
  :deep(.ProseMirror) {
    outline: none;
    font-size: 15px;
    line-height: 1.75;
    color: var(--td-text-color-primary);
    min-height: 120px;
  }

  :deep(.ProseMirror p) {
    margin: 0.5em 0;
  }

  :deep(.ProseMirror h1),
  :deep(.ProseMirror h2),
  :deep(.ProseMirror h3),
  :deep(.ProseMirror h4),
  :deep(.ProseMirror h5),
  :deep(.ProseMirror h6) {
    margin: 1.4em 0 0.5em;
    font-weight: 600;
    line-height: 1.3;
  }

  :deep(.ProseMirror blockquote) {
    margin: 0.8em 0;
    padding: 4px 14px;
    border-left: 3px solid var(--td-brand-color);
    color: var(--td-text-color-secondary);
  }

  :deep(.ProseMirror pre) {
    padding: 12px 14px;
    border-radius: 8px;
    background: var(--td-bg-color-secondarycontainer);
    overflow-x: auto;
    font-size: 13px;
  }

  :deep(.ProseMirror code) {
    font-family: var(--td-font-family-mono, ui-monospace, monospace);
  }

  :deep(.ProseMirror pre code) {
    background: none;
    padding: 0;
  }

  :deep(.ProseMirror table) {
    border-collapse: collapse;
    width: 100%;
    margin: 1em 0;
  }

  :deep(.ProseMirror th),
  :deep(.ProseMirror td) {
    border: 1px solid var(--td-component-stroke);
    padding: 6px 10px;
    text-align: left;
  }

  :deep(.ProseMirror a) {
    color: var(--td-brand-color);
  }

  :deep(.ProseMirror mark) {
    border-radius: 2px;
    padding: 0 2px;
  }

  :deep(.ProseMirror ul[data-type='taskList']) {
    list-style: none;
    padding-left: 4px;
  }

  :deep(.ProseMirror li[data-type='taskItem']) {
    display: flex;
    align-items: flex-start;
    gap: 6px;

    > label {
      margin-top: 3px;
    }

    > div {
      flex: 1;
    }
  }

  :deep(.ProseMirror details) {
    margin: 0.6em 0;
    border: 1px solid var(--td-component-stroke);
    border-radius: 6px;
    padding: 4px 10px;
  }

  :deep(.ProseMirror ul),
  :deep(.ProseMirror ol) {
    padding-left: 1.6em;
  }

  :deep(.collaboration-carets__caret) {
    position: relative;
    margin-left: -1px;
    margin-right: -1px;
    border-left: 1px solid;
    border-right: 1px solid;
    word-break: normal;
    pointer-events: none;
  }

  :deep(.collaboration-carets__label) {
    position: absolute;
    top: -1.1em;
    left: -1px;
    padding: 1px 5px;
    border-radius: 3px 3px 3px 0;
    font-size: 11px;
    font-weight: 600;
    color: #fff;
    white-space: nowrap;
    user-select: none;
  }
}

.doc-editor--readonly .doc-editor-content :deep(.ProseMirror) {
  cursor: default;
}
</style>
