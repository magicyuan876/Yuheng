<template>
  <t-drawer
    :visible="visible"
    :header="t('docs.history.title')"
    size="min(1080px, 94vw)"
    :footer="false"
    destroy-on-close
    @close="emit('close')"
  >
    <div class="docs-history">
      <!-- The versions, newest first. -->
      <aside class="docs-history-list" @scroll="onScroll">
        <p v-if="loading && !items.length" class="docs-history-note">{{ t('common.loading') }}</p>
        <p v-else-if="!items.length" class="docs-history-note">{{ t('docs.history.empty') }}</p>

        <button
          v-for="(item, index) in items"
          :key="item.id"
          type="button"
          class="docs-history-item"
          :class="{ 'is-active': item.id === selectedId, 'is-compared': item.id === compareId }"
          @click="select(item, index)"
        >
          <span class="docs-history-when">{{ formatWhen(item.created_at) }}</span>
          <span class="docs-history-who">{{ who(item) }}</span>
          <span class="docs-history-meta">
            <span class="docs-history-reason" :data-reason="item.reason">
              {{ t(`docs.history.reason.${item.reason}`) }}
            </span>
            <span>{{ t('docs.history.words', { count: item.word_count }) }}</span>
          </span>
          <span
            v-if="index > 0"
            class="docs-history-compare"
            role="button"
            :aria-label="t('docs.history.compareWith')"
            @click.stop="setCompare(item)"
          >
            <t-icon name="swap" size="14px" />
          </span>
        </button>

        <p v-if="loadingMore" class="docs-history-note">{{ t('common.loading') }}</p>
      </aside>

      <!-- What the selected version says, or how it differs. -->
      <section class="docs-history-detail">
        <header v-if="selected" class="docs-history-detail-head">
          <div>
            <h3>{{ selected.title || t('docs.tree.untitled') }}</h3>
            <p class="docs-history-subtitle">
              {{ comparing ? t('docs.history.comparing', { a: compareLabel, b: selectedLabel }) : selectedLabel }}
            </p>
          </div>
          <div class="docs-history-actions">
            <t-radio-group v-model="mode" variant="default-filled" size="small">
              <t-radio-button value="preview">{{ t('docs.history.preview') }}</t-radio-button>
              <t-radio-button value="diff">{{ t('docs.history.changes') }}</t-radio-button>
            </t-radio-group>
            <t-button
              v-if="canEdit"
              size="small"
              theme="primary"
              :loading="restoring"
              @click="confirmRestore"
            >
              {{ t('docs.history.restore') }}
            </t-button>
          </div>
        </header>

        <div v-if="!selected" class="docs-history-note">{{ t('docs.history.pick') }}</div>

        <!-- Preview: the document as it was, read-only. -->
        <div v-else-if="mode === 'preview'" class="docs-history-body">
          <p v-if="detailLoading" class="docs-history-note">{{ t('common.loading') }}</p>
          <!-- eslint-disable-next-line vue/no-v-html -->
          <div v-else class="docs-history-content" v-html="previewHTML" />
        </div>

        <!-- Changes: the two comparisons the server computed. -->
        <div v-else class="docs-history-body">
          <p v-if="diffLoading" class="docs-history-note">{{ t('common.loading') }}</p>
          <template v-else-if="diff">
            <p class="docs-history-summary">
              <span class="is-added">+{{ diff.line_summary.added }}</span>
              <span class="is-removed">−{{ diff.line_summary.removed }}</span>
              <span v-if="blocks.length">{{ t('docs.history.blocksChanged', { count: blocks.length }) }}</span>
            </p>

            <p v-if="!rows.length" class="docs-history-note">{{ t('docs.history.identical') }}</p>
            <ol v-else class="docs-history-diff">
              <li v-for="(row, index) in rows" :key="index" :class="rowClass(row)">
                <template v-if="row.type === 'gap'">
                  <span class="docs-history-gap">{{ t('docs.history.skipped', { count: row.skipped }) }}</span>
                </template>
                <template v-else>
                  <span class="docs-history-lineno">{{ row.line.old_line || '' }}</span>
                  <span class="docs-history-lineno">{{ row.line.new_line || '' }}</span>
                  <span class="docs-history-text">{{ row.line.text }}</span>
                </template>
              </li>
            </ol>

            <template v-if="blocks.length">
              <h4 class="docs-history-subhead">{{ t('docs.history.structure') }}</h4>
              <ul class="docs-history-blocks">
                <li v-for="block in blocks" :key="block.block_id" :data-kind="block.kind">
                  <span class="docs-history-blockkind">{{ t(`docs.history.block.${block.kind}`) }}</span>
                  <span class="docs-history-blocktext">{{ block.text || block.type }}</span>
                </li>
              </ul>
            </template>
          </template>
        </div>
      </section>
    </div>
  </t-drawer>
</template>

<script setup lang="ts">
import { generateHTML } from '@tiptap/core'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  getRevision, getRevisionDiff, listRevisions, restoreRevision,
  type DiffView, type RevisionView,
} from '@/api/docs'

import { officialExtensions } from '../editor/extensions'

import { foldDiff, interestingBlocks, type HunkRow } from './hunks'

const props = defineProps<{
  visible: boolean
  pageId: string
  canEdit: boolean
}>()

const emit = defineEmits<{ close: []; restored: [] }>()
const { t, locale } = useI18n()

const items = ref<RevisionView[]>([])
const cursor = ref('')
const loading = ref(false)
const loadingMore = ref(false)
const exhausted = ref(false)

const selectedId = ref('')
const compareId = ref('')
const mode = ref<'preview' | 'diff'>('preview')
const restoring = ref(false)

const detailLoading = ref(false)
const previewHTML = ref('')
const diff = ref<DiffView | null>(null)
const diffLoading = ref(false)

const selected = computed(() => items.value.find((item) => item.id === selectedId.value))
const comparing = computed(() => compareId.value !== '')
const rows = computed<HunkRow[]>(() => (diff.value ? foldDiff(diff.value.lines) : []))
const blocks = computed(() => (diff.value ? interestingBlocks(diff.value.blocks) : []))

const selectedLabel = computed(() => (selected.value ? formatWhen(selected.value.created_at) : ''))
const compareLabel = computed(() => {
  const other = items.value.find((item) => item.id === compareId.value)
  return other ? formatWhen(other.created_at) : ''
})

function formatWhen(iso: string): string {
  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) return iso
  return at.toLocaleString(locale.value, { dateStyle: 'medium', timeStyle: 'short' })
}

/** Who a version is by; the ids are a fallback for a person since removed. */
function who(item: RevisionView): string {
  const names = (item.editors ?? [])
    .map((editor) => editor.username || editor.email)
    .filter((name): name is string => !!name)
  if (names.length === 0) return t('docs.links.someone')
  if (names.length <= 2) return names.join('、')
  return t('docs.history.andOthers', { name: names[0], count: names.length - 1 })
}

function rowClass(row: HunkRow): string {
  if (row.type === 'gap') return 'is-gap'
  return `is-${row.line.kind}`
}

async function load(more = false) {
  if (!props.pageId) return
  if (more && (exhausted.value || loadingMore.value)) return
  const target = more ? loadingMore : loading
  target.value = true
  try {
    const page = await listRevisions(props.pageId, more ? { cursor: cursor.value } : {})
    items.value = more ? [...items.value, ...page.items] : page.items
    cursor.value = page.next_cursor ?? ''
    exhausted.value = !page.next_cursor
    if (!more && items.value.length > 0) select(items.value[0]!, 0)
  } catch (err) {
    void MessagePlugin.error((err as { message?: string })?.message || t('docs.history.loadFailed'))
  } finally {
    target.value = false
  }
}

/** Pages in more history as the list is scrolled towards its end. */
function onScroll(event: Event) {
  const el = event.target as HTMLElement
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 80) void load(true)
}

function select(item: RevisionView, index: number) {
  selectedId.value = item.id
  // Selecting a different version abandons a comparison that was about the
  // old one; keeping it would label the panel with a pair nobody chose.
  if (compareId.value === item.id) compareId.value = ''
  void refresh()
  void index
}

/** Marks a version as the other side of a comparison. */
function setCompare(item: RevisionView) {
  compareId.value = compareId.value === item.id ? '' : item.id
  mode.value = 'diff'
  void refresh()
}

async function refresh() {
  if (!selected.value) return
  if (mode.value === 'preview') return loadPreview()
  return loadDiff()
}

async function loadPreview() {
  const id = selectedId.value
  if (!id) return
  detailLoading.value = true
  try {
    const detail = await getRevision(props.pageId, id)
    // A later click wins: the answer to an abandoned request must not
    // overwrite the one somebody is waiting for.
    if (selectedId.value !== id) return
    previewHTML.value = renderDocument(detail.content)
  } catch {
    if (selectedId.value === id) previewHTML.value = ''
  } finally {
    detailLoading.value = false
  }
}

async function loadDiff() {
  const id = selectedId.value
  if (!id) return
  diffLoading.value = true
  try {
    // The older side is always the one on the left of the comparison: a
    // version is compared *forwards*, to what came after it.
    const answer = compareId.value
      ? await getRevisionDiff(props.pageId, olderOf(id, compareId.value), newerOf(id, compareId.value))
      : await getRevisionDiff(props.pageId, id)
    if (selectedId.value !== id) return
    diff.value = answer
  } catch (err) {
    if (selectedId.value === id) {
      diff.value = null
      void MessagePlugin.error((err as { message?: string })?.message || t('docs.history.loadFailed'))
    }
  } finally {
    diffLoading.value = false
  }
}

/** Version numbers order the pair; ids alone say nothing about which is older. */
function olderOf(a: string, b: string): string {
  return versionOf(a) <= versionOf(b) ? a : b
}

function newerOf(a: string, b: string): string {
  return versionOf(a) > versionOf(b) ? a : b
}

function versionOf(id: string): number {
  return items.value.find((item) => item.id === id)?.version ?? 0
}

/**
 * Renders a stored document read-only.
 *
 * Markup rather than an editor instance: nothing here is editable, and one
 * editor per preview would be a great deal of machinery to show text. Nothing
 * in the markup can execute — the schema has no script, iframe or style node
 * type — which is the same guarantee the paste path and block references rest
 * on.
 */
function renderDocument(content: unknown): string {
  if (!content) return ''
  try {
    return generateHTML(content as Record<string, unknown>, officialExtensions() as never)
  } catch {
    // A version written by an older build, using a node this one does not
    // have. Showing nothing is better than breaking the panel around it.
    return ''
  }
}

function confirmRestore() {
  if (!selected.value) return
  const when = formatWhen(selected.value.created_at)
  const dialog = DialogPlugin.confirm({
    header: t('docs.history.restore'),
    body: t('docs.history.restoreConfirm', { when }),
    confirmBtn: { content: t('docs.history.restore'), theme: 'primary' },
    onConfirm: async () => {
      dialog.hide()
      await doRestore()
    },
  })
}

async function doRestore() {
  const id = selectedId.value
  if (!id || restoring.value) return
  restoring.value = true
  try {
    await restoreRevision(props.pageId, id)
    void MessagePlugin.success(t('docs.history.restored'))
    emit('restored')
    // The restore itself is now the newest version, and the state before it
    // is one entry further down; both belong in the list.
    await load()
  } catch (err) {
    void MessagePlugin.error((err as { message?: string })?.message || t('docs.history.restoreFailed'))
  } finally {
    restoring.value = false
  }
}

watch(() => props.visible, (open) => {
  if (!open) return
  items.value = []
  cursor.value = ''
  exhausted.value = false
  selectedId.value = ''
  compareId.value = ''
  mode.value = 'preview'
  diff.value = null
  previewHTML.value = ''
  void load()
})

watch(mode, () => void refresh())
</script>

<style scoped lang="less">
.docs-history {
  display: grid;
  grid-template-columns: 290px minmax(0, 1fr);
  gap: 16px;
  height: 100%;
  min-height: 0;
}

@media (max-width: 760px) {
  .docs-history {
    grid-template-columns: minmax(0, 1fr);
  }
}

.docs-history-list {
  overflow-y: auto;
  border-right: 1px solid var(--td-component-stroke);
  padding-right: 8px;
}

.docs-history-item {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 100%;
  padding: 8px 10px;
  border: none;
  border-radius: 6px;
  background: transparent;
  text-align: left;
  cursor: pointer;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }

  &.is-active {
    background: var(--td-brand-color-light);
  }

  &.is-compared {
    box-shadow: inset 0 0 0 1px var(--td-brand-color);
  }
}

.docs-history-when {
  font-size: 13px;
  color: var(--td-text-color-primary);
  font-variant-numeric: tabular-nums;
}

.docs-history-who {
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.docs-history-meta {
  display: flex;
  gap: 8px;
  font-size: 11px;
  color: var(--td-text-color-placeholder);
}

.docs-history-reason {
  &[data-reason='restore'],
  &[data-reason='import'],
  &[data-reason='publish'] {
    color: var(--td-brand-color);
  }
}

.docs-history-compare {
  position: absolute;
  top: 8px;
  right: 8px;
  color: var(--td-text-color-placeholder);
  line-height: 0;

  &:hover {
    color: var(--td-brand-color);
  }
}

.docs-history-detail {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}

.docs-history-detail-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--td-component-stroke);

  h3 {
    margin: 0;
    font-size: 15px;
  }
}

.docs-history-subtitle {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.docs-history-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: none;
}

.docs-history-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-top: 12px;
}

.docs-history-note {
  margin: 12px 0;
  font-size: 13px;
  color: var(--td-text-color-placeholder);
}

.docs-history-summary {
  display: flex;
  gap: 12px;
  margin: 0 0 10px;
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  color: var(--td-text-color-placeholder);

  .is-added {
    color: var(--td-success-color);
  }

  .is-removed {
    color: var(--td-error-color);
  }
}

.docs-history-diff {
  margin: 0;
  padding: 0;
  list-style: none;
  font-family: var(--td-font-family-medium, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 12.5px;
  line-height: 1.6;
  border: 1px solid var(--td-component-stroke);
  border-radius: 6px;
  overflow-x: auto;

  li {
    display: flex;
    gap: 8px;
    padding: 0 8px;
    white-space: pre-wrap;
    word-break: break-word;

    &.is-added {
      background: var(--td-success-color-1);
    }

    &.is-removed {
      background: var(--td-error-color-1);
    }

    &.is-gap {
      justify-content: center;
      background: var(--td-bg-color-container-hover);
      color: var(--td-text-color-placeholder);
      font-size: 11px;
    }
  }
}

.docs-history-lineno {
  flex: none;
  width: 34px;
  text-align: right;
  color: var(--td-text-color-placeholder);
  user-select: none;
  font-variant-numeric: tabular-nums;
}

.docs-history-text {
  flex: 1;
  min-width: 0;
}

.docs-history-subhead {
  margin: 18px 0 8px;
  font-size: 13px;
}

.docs-history-blocks {
  margin: 0;
  padding: 0;
  list-style: none;
  font-size: 13px;

  li {
    display: flex;
    gap: 8px;
    padding: 4px 0;
    border-bottom: 1px solid var(--td-component-stroke);

    &:last-child {
      border-bottom: 0;
    }
  }
}

.docs-history-blockkind {
  flex: none;
  min-width: 56px;
  font-size: 11px;
  color: var(--td-text-color-placeholder);
}

.docs-history-blocktext {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--td-text-color-secondary);
}

.docs-history-content {
  font-size: 14px;
  line-height: 1.7;
}
</style>
