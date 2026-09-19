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

    <FindReplacePanel
      :open="findOpen"
      :editor="editor ?? null"
      :editable="editorEditable"
      :revision="editorRevision"
      @close="findOpen = false"
    />

    <SelectionToolbar
      :visible="toolbarVisible"
      :placement="toolbarPlace"
      :editor="editor ?? null"
      :revision="editorRevision"
      :can-comment="canComment"
      @dismiss="toolbarVisible = false"
      @comment="startComment"
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
import { Extension } from '@tiptap/core'
import { Collaboration } from '@tiptap/extension-collaboration'
import { CollaborationCaret } from '@tiptap/extension-collaboration-caret'
import { EditorContent, useEditor, VueNodeViewRenderer } from '@tiptap/vue-3'
// KaTeX draws with its own stylesheet; without it a formula renders as a
// column of unpositioned glyphs.
import 'katex/dist/katex.min.css'
import { MessagePlugin } from 'tdesign-vue-next'
import { computed, onBeforeUnmount, onMounted, provide, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { resolveBlockRefs, resolvePageTitles } from '@/api/docs'

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
  DOCS_BLOCK_REFS, DOCS_DIAGRAMS, DOCS_DIRECTORY, DOCS_EMBEDS, DOCS_TITLE_CACHE, type DirectoryPerson,
} from './linkContext'
import PdfNodeView from './PdfNodeView.vue'
import { useDocMedia } from './useDocMedia'
import VideoNodeView from './VideoNodeView.vue'
import MathNodeView from './MathNodeView.vue'
import MentionNodeView from './MentionNodeView.vue'
import MermaidNodeView from './MermaidNodeView.vue'
import PageLinkNodeView from './PageLinkNodeView.vue'
import StatusNodeView from './StatusNodeView.vue'
import FindReplacePanel from './FindReplacePanel.vue'
import SelectionToolbar from './SelectionToolbar.vue'
import SuggestionMenu from './SuggestionMenu.vue'
import TransclusionNodeView from './TransclusionNodeView.vue'
import { BlockRefCache } from './blockRefCache'
import { TitleCache } from './titleCache'
import TocNodeView from './TocNodeView.vue'
import { useDocSuggestions } from './useDocSuggestions'
import { awarenessUser, type UserLike } from './session'
import { useDocUploads } from './useDocUploads'
import { IdleScheduler } from './idleWork'
import { blockAt } from './blockMove'
import { formatBlockRefLink } from './blockRefLink'
import { DragHandle } from './dragHandle'
import { findPlugin } from './find'
import { commentDecorationPlugin } from '../comments/decorations'
import { useComments } from '../comments/useComments'
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
  /** False on a surface where commenting makes no sense (a preview, an
   * anonymous share page). Defaults to true: anybody who can see the page
   * can remark on it. */
  canComment?: boolean
  collabUrl: string
  currentUser: UserLike | null
  getToken: () => string | null
}>()

const emit = defineEmits<{
  /** Fires once per heading-set change, letting the parent drive a TOC sidebar. */
  headings: [entries: { id: string; level: number; text: string; pos: number }[]]
  /** Fires when a comment has been started on the selection, so the page can
   * open its comment panel and put the cursor in the box. */
  commentDraft: []
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

// ---- block references ------------------------------------------------------
// One lookup shared by every reference in the document. Resolved under the
// reader's own permissions, on the server, so a reference to a page they may
// not open shows nothing rather than its contents.
const blockRefRevision = ref(0)
const blockRefs = new BlockRefCache({
  resolve: (list) => resolveBlockRefs(
    list.map((r) => ({ source_page_id: r.sourcePageId, source_block_id: r.sourceBlockId })),
  ).then((rows) => rows.map((r) => ({
    sourcePageId: r.source_page_id,
    sourceBlockId: r.source_block_id,
    state: r.state,
    content: r.content,
    title: r.title,
    icon: r.icon,
    sourceShortId: r.source_short_id,
  }))),
  onChange: () => {
    blockRefRevision.value++
  },
})
provide(DOCS_BLOCK_REFS, { get: (ref) => blockRefs.get(ref), revision: blockRefRevision })

/**
 * Forgets what is cached about one source page.
 *
 * Exposed so the page view can call it when a page-content event names a page
 * this document quotes: the correction happens over there, and this is what
 * makes it show up here without a reload.
 */
function forgetBlockRefs(sourcePageId: string) {
  blockRefs.invalidatePage(sourcePageId)
}

/**
 * Copies a link to the block the cursor is in.
 *
 * The other half of referencing a block: this puts the address on the
 * clipboard, and pasting it on another page turns it into the quotation. Two
 * steps rather than a picker, because the block being quoted and the place it
 * will appear are usually on different pages and often in different tabs.
 *
 * `blockAt` is the drag handle's own idea of "which block is this", so the
 * link names the block a person would say they were standing in — the list
 * item rather than the paragraph inside it.
 */
function copyCurrentBlockRef() {
  const ed = editor.value
  if (!ed || ed.isDestroyed) return
  const block = blockAt(ed.state, ed.state.selection.from)
  const blockId = block ? String(block.node.attrs.id ?? '') : ''
  if (!blockId) {
    // Every block gets an id as it is typed, so this means an empty document
    // or a block type that carries none.
    void MessagePlugin.warning(t('docs.transclusion.noBlock'))
    return
  }
  const link = formatBlockRefLink({ pageId: props.pageId, blockId }, window.location.origin)
  void navigator.clipboard?.writeText(link).then(
    () => void MessagePlugin.success(t('docs.transclusion.copied')),
    // A clipboard a browser refuses is not an error worth a dialogue; showing
    // the link lets somebody copy it by hand.
    () => void MessagePlugin.info(link),
  )
}

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
    // Offered only on a page that has an id to point at.
    copyBlockRef: props.pageId ? copyCurrentBlockRef : undefined,
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
/**
 * Find and replace.
 *
 * The panel is mounted with the editor rather than opened on demand so the
 * highlight plugin is always present; `findOpen` only decides whether it is
 * drawn and whether it is searching.
 */
/**
 * Comments.
 *
 * Assembled here rather than in the page view because this is where both
 * things it needs live: the editor, and the Y.Doc whose relative positions
 * the anchors are written against. The panel is drawn by the page, through
 * the handle exposed below.
 */
const comments = useComments({
  pageId: pageIdRefForUploads,
  ydoc: computed(() => collab.ydoc.value ?? null),
  onError: (message) => void MessagePlugin.error(message || t('docs.comments.loadFailed')),
})

const commentExtension = Extension.create({
  name: 'yuhengComments',
  addProseMirrorPlugins: () => [commentDecorationPlugin((id) => comments.select(id))],
})

/**
 * Starts a comment on the current selection.
 *
 * Called from the selection toolbar. The anchor and the quotation are both
 * taken now, while the selection is still there: the anchor is what makes the
 * comment survive other people editing around it, and the quotation is what
 * places it if that anchor ever stops resolving.
 */
const drafting = ref<{ anchor: unknown; quoted: string } | null>(null)

function startComment() {
  const ed = editor.value
  if (!ed || ed.isDestroyed) return
  const { from, to } = ed.state.selection
  if (to <= from) return

  drafting.value = {
    anchor: comments.anchorFor(from, to),
    quoted: ed.state.doc.textBetween(from, to, ' '),
  }
  toolbarVisible.value = false
  emit('commentDraft')
}

/** Discards a draft comment, when the composer is cancelled. */
function cancelComment() {
  drafting.value = null
}

/** Stores the draft comment. */
async function submitComment(body: unknown) {
  const draft = drafting.value
  if (!draft) return
  drafting.value = null
  await comments.add({
    body,
    anchor: draft.anchor ?? undefined,
    quoted_text: draft.quoted,
  })
}

const findOpen = ref(false)

const findExtension = Extension.create({
  name: 'yuhengFind',
  addProseMirrorPlugins: () => [findPlugin()],
  addKeyboardShortcuts: () => ({
    'Mod-f': () => {
      findOpen.value = true
      return true
    },
  }),
})

/**
 * Whether this caller may leave a comment.
 *
 * Deliberately not tied to edit rights: a reader may comment, which is what
 * makes review possible without handing out write access. The server decides
 * for real; this only decides whether to offer the button.
 */
const canComment = computed(() => props.canComment !== false)

const toolbarVisible = ref(false)
const toolbarPlace = ref({ left: 0, top: 0, below: false })
const editorRevision = ref(0)

function refreshToolbar() {
  const ed = editor.value
  if (!ed || ed.isDestroyed) return
  editorRevision.value++
  if (!shouldShow(ed.state, { editable: editorEditable.value, canComment: canComment.value })) {
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
    DragHandle.configure({
      offset: 28,
      label: t('docs.toolbar.moveBlock'),
      addLabel: t('docs.toolbar.addBlock'),
      menuLabel: t('docs.toolbar.blockMenu'),
      translate: (key) => t(key),
      copyBlockRef: props.pageId ? copyCurrentBlockRef : undefined,
    }),
    findExtension,
    commentExtension,
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
    transclusion: VueNodeViewRenderer(TransclusionNodeView),
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
    // The document moved under the comment highlights; they are recomputed at
    // the next idle moment, coalesced like the word count.
    comments.touch()
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
    comments.bind(ed as never)
    void comments.load()
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
  blockRefs.dispose()
  comments.dispose()
  media.dispose()
  editor.value?.destroy()
})

defineExpose({
  editor, collab, comments, drafting,
  forgetTitle, forgetBlockRefs, startComment, cancelComment, submitComment,
})
</script>

<style scoped lang="less">
// Created by the drag-handle plugin rather than by this template, so it needs
// :deep to be reached from a scoped block.
// Comment highlights, drawn as decorations by the comment plugin. Never a
// mark: see the note at the top of views/docs/comments/decorations.ts.
:deep(.docs-comment-mark) {
  background: var(--td-warning-color-1);
  border-bottom: 2px solid var(--td-warning-color-5);
  cursor: pointer;

  &.is-active {
    background: var(--td-warning-color-3);
  }

  // Placed by its quotation rather than by its stored position: a good guess,
  // and the reader should be able to tell it is one.
  &.is-approximate {
    border-bottom-style: dashed;
  }
}

// Drawn by the find plugin as a decoration, so it is likewise out of scope.
:deep(.docs-find-match) {
  background: var(--td-warning-color-2);
  border-radius: 2px;

  &.is-current {
    background: var(--td-warning-color-4);
  }
}

// The strip holding the "+" and the drag handle beside the hovered block.
// One container so the pair centres on the block as a unit and the hover
// target between the two buttons has no gap to fall through.
:deep(.docs-drag-tools) {
  position: absolute;
  visibility: hidden;
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 20px;
  // Centred on the block: `place` puts the strip's top at the block's
  // middle, this lifts it back up by half of itself.
  transform: translateY(-50%);
  user-select: none;
}

:deep(.docs-drag-handle) {
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

// The "+" above the drag handle: a new empty block after the hovered one.
:deep(.docs-drag-plus) {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 22px;
  padding: 0;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--td-text-color-placeholder);
  font-size: 15px;
  line-height: 1;
  cursor: pointer;
  user-select: none;

  &:hover {
    background: var(--td-bg-color-container-hover);
    color: var(--td-text-color-secondary);
  }
}

// The block menu is appended to the document body by the drag-handle
// plugin, so its styles live in the unscoped block below; scoped ones
// cannot reach it.

.doc-editor {
  position: relative;
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

<!-- The block menu is appended to the document body by the drag-handle
plugin, so its styles live in an unscoped block; the scoped ones above
cannot reach it. Same self-drawn look as the slash menu. -->
<style lang="less">
.docs-block-menu {
  position: fixed;
  z-index: 1300;
  min-width: 200px;
  max-width: 280px;
  max-height: 320px;
  overflow-y: auto;
  padding: 4px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container);
  box-shadow: var(--td-shadow-2);
  outline: none;

  &:focus-visible {
    outline: 2px solid var(--td-brand-color);
    outline-offset: -2px;
  }
}

.docs-block-menu-section {
  margin: 0;
  padding: 6px 8px 2px;
  font-size: 11px;
  color: var(--td-text-color-placeholder);
}

.docs-block-menu-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  border: none;
  background: transparent;
  border-radius: 6px;
  padding: 6px 8px;
  cursor: pointer;
  text-align: left;
  font-size: 13.5px;
  color: var(--td-text-color-primary);

  &.is-active {
    background: var(--td-bg-color-container-hover);
  }
}
</style>
