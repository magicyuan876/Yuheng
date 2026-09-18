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
        <button
          v-if="editorEditable"
          type="button"
          class="doc-editor-attach"
          @click="pickFiles"
        >
          <t-icon name="attach" size="14px" />
          <span>{{ t('docs.attachments.attach') }}</span>
        </button>
        <span v-if="saveLabel" class="doc-editor-save">{{ saveLabel }}</span>
        <span>{{ t('docs.pages.wordCount', { count: wordCount }) }}</span>
      </div>
    </div>

    <div v-if="!collab.ready.value" class="doc-editor-loading">
      <t-skeleton animation="gradient" :row-col="[{ width: '90%' }, { width: '75%' }, { width: '85%' }]" />
    </div>
    <EditorContent v-else-if="editor" :editor="editor" class="doc-editor-content" />

    <ul v-if="uploads.tasks.value.length" class="doc-editor-uploads">
      <li v-for="task in uploads.tasks.value" :key="task.key">
        <t-icon name="upload" size="13px" />
        <span class="doc-editor-upload-name">{{ task.name }}</span>
        <t-progress
          theme="line"
          :percentage="Math.max(task.progress, 1)"
          :label="false"
          class="doc-editor-upload-bar"
        />
      </li>
    </ul>

    <SelectionToolbar
      :visible="toolbarVisible"
      :placement="toolbarPlace"
      :editor="editor ?? null"
      :revision="editorRevision"
      @dismiss="toolbarVisible = false"
    />

    <SuggestionMenu
      :open="suggestions.open.value"
      :loading="suggestions.loading.value"
      :items="suggestions.items.value"
      :selected="suggestions.selected.value"
      :kind="suggestions.kind.value"
      :position="suggestions.position.value"
      @choose="suggestions.choose"
      @hover="suggestions.hover"
    />

    <input
      ref="filePicker"
      type="file"
      multiple
      class="doc-editor-file-input"
      @change="onFilesPicked"
    />
  </div>
</template>

<script setup lang="ts">
import { Collaboration } from '@tiptap/extension-collaboration'
import { CollaborationCaret } from '@tiptap/extension-collaboration-caret'
import { EditorContent, useEditor, VueNodeViewRenderer } from '@tiptap/vue-3'
// KaTeX draws with its own stylesheet; without it a formula renders as a
// column of unpositioned glyphs.
import 'katex/dist/katex.min.css'
import { MessagePlugin } from 'tdesign-vue-next'
import { computed, onBeforeUnmount, onMounted, provide, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { resolvePageTitles } from '@/api/docs'

import AttachmentNodeView from './AttachmentNodeView.vue'
import CalloutNodeView from './CalloutNodeView.vue'
import ColumnNodeView from './ColumnNodeView.vue'
import ColumnsNodeView from './ColumnsNodeView.vue'
import { officialExtensions } from './extensions'
import ImageNodeView from './ImageNodeView.vue'
import AudioNodeView from './AudioNodeView.vue'
import DiagramNodeView from './DiagramNodeView.vue'
import EmbedNodeView from './EmbedNodeView.vue'
import {
  DOCS_DIAGRAMS, DOCS_DIRECTORY, DOCS_EMBEDS, DOCS_TITLE_CACHE, type DirectoryPerson,
} from './linkContext'
import PdfNodeView from './PdfNodeView.vue'
import { useDocMedia } from './useDocMedia'
import VideoNodeView from './VideoNodeView.vue'
import MathNodeView from './MathNodeView.vue'
import MentionNodeView from './MentionNodeView.vue'
import MermaidNodeView from './MermaidNodeView.vue'
import PageLinkNodeView from './PageLinkNodeView.vue'
import StatusNodeView from './StatusNodeView.vue'
import SelectionToolbar from './SelectionToolbar.vue'
import SuggestionMenu from './SuggestionMenu.vue'
import { TitleCache } from './titleCache'
import TocNodeView from './TocNodeView.vue'
import { useDocSuggestions } from './useDocSuggestions'
import { awarenessUser, type UserLike } from './session'
import { useDocUploads } from './useDocUploads'
import { IdleScheduler } from './idleWork'
import { DragHandle } from './dragHandle'
import { shouldShow, toolbarPlacement } from './toolbar'
import { pasteEditorProps } from './useDocPaste'
import { extractHeadings } from './toc'
import { countDocument } from './wordCount'
import { useDocCollab } from './useDocCollab'

const props = defineProps<{
  pageId: string
  /** The page's space, which is what an upload is stored against. */
  spaceId: string
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
  'lease-held': 'user-circle',
  superseded: 'error-circle',
  unavailable: 'info-circle',
  none: 'info-circle',
}[banner.value.kind]))

const bannerText = computed(() => {
  switch (banner.value.kind) {
    case 'connecting': return t('docs.pages.editorConnecting')
    case 'offline': return t('docs.pages.editorOffline')
    case 'read-only': return t('docs.pages.editorReadOnly')
    case 'permission-narrowed': return t('docs.pages.editorPermissionNarrowed')
    case 'lease-held': return t('docs.pages.editorLeaseHeld', { name: banner.value.holder })
    case 'superseded': return t('docs.pages.editorSuperseded')
    case 'unavailable': return t('docs.pages.editorUnavailable')
    default: return ''
  }
})

const spaceIdRef = computed(() => props.spaceId)
const pageIdRefForUploads = computed(() => props.pageId)

const uploads = useDocUploads({
  spaceId: spaceIdRef,
  pageId: pageIdRefForUploads,
  canEdit: editorEditable,
  onError: (message) => void MessagePlugin.error(message),
  placeholderLabel: (task) => t('docs.attachments.uploading', { name: task.name }),
})

const filePicker = ref<HTMLInputElement | null>(null)

function pickFiles() {
  filePicker.value?.click()
}

function onFilesPicked(event: Event) {
  const input = event.target as HTMLInputElement
  if (editor.value && input.files) uploads.insert(editor.value, [...input.files])
  // Clearing lets the same file be chosen twice in a row.
  input.value = ''
}

// ---- page links and mentions ----------------------------------------------
// One title lookup shared by every link in the document, and one directory
// shared by every mention. Both are provided rather than passed: a node view
// is created by ProseMirror and cannot be handed props.
const titleRevision = ref(0)
const titles = new TitleCache({
  resolve: (ids) => resolvePageTitles(ids).then((rows) => rows.map((r) => ({
    pageId: r.page_id, title: r.title, icon: r.icon,
    shortId: r.short_id, spaceId: r.space_id, resolved: r.resolved,
  }))),
  onChange: () => {
    titleRevision.value++
  },
})
provide(DOCS_TITLE_CACHE, { get: (id: string) => titles.get(id), revision: titleRevision })

const directoryRevision = ref(0)
const directory = new Map<string, DirectoryPerson>()
provide(DOCS_DIRECTORY, {
  get: (id: string) => {
    void directoryRevision.value
    return directory.get(id)
  },
  revision: directoryRevision,
})

// ---- media, embeds and diagrams --------------------------------------------
// The embed allow-list lives on the server, so an embed node cannot work out
// its own frame address; the resolver asks once per address and every node
// showing it shares the answer.
const media = useDocMedia({ spaceId: spaceIdRef, pageId: pageIdRefForUploads })
provide(DOCS_EMBEDS, media.embeds)
provide(DOCS_DIAGRAMS, media.diagrams)
onMounted(() => {
  void media.load()
})

const suggestions = useDocSuggestions({
  pageId: pageIdRefForUploads,
  spaceId: spaceIdRef,
  rememberPerson: (person) => {
    directory.set(person.user_id, {
      userId: person.user_id, username: person.username, email: person.email, avatar: person.avatar,
    })
    directoryRevision.value++
  },
  translate: (key) => t(key),
  allow: () => ({
    embeds: media.policy.value.providers.length > 0,
    drawings: true,
  }),
})

/** Forgets a cached title so a rename shows up in every link to that page
 * without a reload. The page view calls it on the rename event. */
function forgetTitle(pageId?: string) {
  titles.invalidate(pageId)
}

const wordCount = ref(0)

/** Exclusive-edit mode saves on a timer rather than keystroke by keystroke,
 * so the editor says where a change has got to. A collaborative session
 * needs no such reassurance: every keystroke is already on the wire. */
const saveLabel = computed(() => {
  if (!collab.exclusive.value || !editorEditable.value) return ''
  switch (collab.saveState.value) {
    case 'saving': return t('docs.pages.editorSaving')
    case 'saved': return t('docs.pages.editorSaved')
    case 'failed': return t('docs.pages.editorSaveFailed')
    default: return ''
  }
})

/**
 * Counting the words and collecting the headings both walk every node, and
 * neither answer is one anybody is waiting on keystroke by keystroke. Running
 * them on every update is what makes a very long page feel heavy, so they are
 * scheduled for the next idle moment instead and coalesced into one walk.
 */
const derive = new IdleScheduler(() => {
  const ed = editor.value
  if (!ed || ed.isDestroyed) return
  wordCount.value = countDocument(ed.state.doc).words
  emit('headings', extractHeadings(ed.state.doc))
})

/**
 * The floating toolbar's state.
 *
 * `editorRevision` exists because the editor is not reactive: a Vue component
 * cannot watch `editor.isActive('bold')`, so the bar is told when to look
 * again. Everything else about the bar — whether it applies at all, and where
 * it goes — is decided in toolbar.ts.
 */
const toolbarVisible = ref(false)
const toolbarPlace = ref({ left: 0, top: 0, below: false })
const editorRevision = ref(0)

function refreshToolbar() {
  const ed = editor.value
  if (!ed || ed.isDestroyed) return
  editorRevision.value++
  if (!shouldShow(ed.state, editorEditable.value)) {
    toolbarVisible.value = false
    return
  }
  try {
    const { from, to } = ed.state.selection
    const start = ed.view.coordsAtPos(from)
    // Biased to the left of `to`, so the box ends where the selection does
    // rather than at the start of the next line.
    const end = ed.view.coordsAtPos(to, -1)
    toolbarPlace.value = toolbarPlacement({
      left: Math.min(start.left, end.left),
      right: Math.max(start.right, end.right),
      top: Math.min(start.top, end.top),
      bottom: Math.max(start.bottom, end.bottom),
    }, { width: window.innerWidth, height: window.innerHeight })
    toolbarVisible.value = true
  } catch {
    // Coordinates can be stale for a frame after a large remote change; no
    // bar is better than one in the wrong place.
    toolbarVisible.value = false
  }
}

const editor = useEditor({
  editable: editorEditable.value,
  editorProps: {
    ...uploads.editorProps,
    // Before the upload handler, so a pasted file is still claimed there, and
    // everything else goes through the sanitiser on its way in.
    ...pasteEditorProps(),
    // The menu takes the arrow keys, Enter, Tab and Escape while it is open
    // and lets every other key through, so the query stays ordinary text in
    // the document until something is chosen.
    handleKeyDown: (_view: unknown, event: KeyboardEvent) => suggestions.handleKey(event),
  },
  extensions: officialExtensions([
    uploads.extension,
    suggestions.extension,
    DragHandle.configure({ offset: 28, label: t('docs.toolbar.moveBlock') }),
    Collaboration.configure({ document: collab.ydoc.value }),
    // Live cursors need a collaboration service to relay awareness; in
    // exclusive-edit mode there is never a second writer to draw.
    ...(collab.provider.value ? [CollaborationCaret.configure({
      provider: collab.provider.value,
      user: props.currentUser ? awarenessUser(props.currentUser) : { name: '', color: '#999999' },
    })] : []),
  ], {
    image: VueNodeViewRenderer(ImageNodeView),
    attachment: VueNodeViewRenderer(AttachmentNodeView),
    callout: VueNodeViewRenderer(CalloutNodeView),
    columns: VueNodeViewRenderer(ColumnsNodeView),
    column: VueNodeViewRenderer(ColumnNodeView),
    status: VueNodeViewRenderer(StatusNodeView),
    toc: VueNodeViewRenderer(TocNodeView),
    mathInline: VueNodeViewRenderer(MathNodeView),
    mathBlock: VueNodeViewRenderer(MathNodeView),
    mermaid: VueNodeViewRenderer(MermaidNodeView),
    pageLink: VueNodeViewRenderer(PageLinkNodeView),
    mention: VueNodeViewRenderer(MentionNodeView),
    video: VueNodeViewRenderer(VideoNodeView),
    audio: VueNodeViewRenderer(AudioNodeView),
    pdfEmbed: VueNodeViewRenderer(PdfNodeView),
    embed: VueNodeViewRenderer(EmbedNodeView),
    drawio: VueNodeViewRenderer(DiagramNodeView),
    excalidraw: VueNodeViewRenderer(DiagramNodeView),
  }),
  onUpdate: () => {
    derive.schedule()
    refreshToolbar()
  },
  onSelectionUpdate: () => {
    refreshToolbar()
  },
  onBlur: () => {
    // Not hidden on blur: focus moves into the bar itself when a button is
    // clicked, and hiding here would take the bar away mid-click. The bar
    // hides when the selection collapses, which is what actually ends it.
  },
  onCreate: ({ editor: ed }) => {
    // The first count is immediate: an empty word count on a page that has
    // just opened reads as a page that failed to load.
    wordCount.value = countDocument(ed.state.doc).words
    emit('headings', extractHeadings(ed.state.doc))
    uploads.bind(ed)
    suggestions.bind(ed)
  },
})

watch(editorEditable, (val) => {
  editor.value?.setEditable(val)
  refreshToolbar()
})

onBeforeUnmount(() => {
  // Before the editor is destroyed: a pending walk would otherwise run
  // against a document nobody is looking at.
  derive.cancel()
  uploads.bind(null)
  suggestions.bind(null)
  titles.dispose()
  media.dispose()
  editor.value?.destroy()
})

defineExpose({ editor, collab, forgetTitle })
</script>

<style scoped lang="less">
// Created by the drag-handle plugin rather than by this template, so it needs
// :deep to be reached from a scoped block.
:deep(.docs-drag-handle) {
  position: absolute;
  visibility: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 24px;
  padding: 0;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--td-text-color-placeholder);
  font-size: 15px;
  line-height: 1;
  cursor: grab;
  user-select: none;

  &:hover {
    background: var(--td-bg-color-container-hover);
    color: var(--td-text-color-secondary);
  }

  &:active {
    cursor: grabbing;
  }
}

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
  &--lease-held,
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
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.doc-editor-save {
  font-variant-numeric: tabular-nums;
}

.doc-editor-attach {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border: none;
  background: transparent;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
  cursor: pointer;
  padding: 2px 4px;
  border-radius: 4px;

  &:hover {
    color: var(--td-text-color-primary);
    background: var(--td-bg-color-container-hover);
  }
}

.doc-editor-file-input {
  display: none;
}

.doc-editor-uploads {
  list-style: none;
  margin: 8px 0 0;
  padding: 0;

  li {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: var(--td-text-color-placeholder);
    padding: 2px 0;
  }
}

.doc-editor-upload-name {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.doc-editor-upload-bar {
  flex: 1;
  max-width: 200px;
}

:deep(.docs-upload-placeholder) {
  display: inline-block;
  padding: 1px 8px;
  border-radius: 4px;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
  background: var(--td-bg-color-secondarycontainer);
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

  :deep(.ProseMirror .page-break) {
    height: 0;
    margin: 20px 0;
    border-top: 2px dashed var(--td-component-stroke);
    position: relative;
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
