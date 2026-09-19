<template>
  <div class="docs-page-view">
    <!-- Breadcrumbs -->
    <nav class="page-crumbs" aria-label="breadcrumb">
      <button type="button" class="crumb" @click="goSpace">{{ space.name }}</button>
      <template v-for="a in ancestors" :key="a.id">
        <t-icon name="chevron-right" size="12px" class="crumb-sep" />
        <button type="button" class="crumb" @click="goPage(a)">{{ a.title || t('docs.tree.untitled') }}</button>
      </template>
      <template v-if="page">
        <t-icon name="chevron-right" size="12px" class="crumb-sep" />
        <span class="crumb crumb--current">{{ page.title || t('docs.tree.untitled') }}</span>
      </template>
    </nav>

    <div v-if="loading" class="page-loading">
      <t-skeleton animation="gradient" :row-col="[{ width: '50%', height: '32px' }, { width: '100%' }, { width: '90%' }]" />
    </div>

    <!-- In the trash -->
    <div v-else-if="gone" class="page-gone">
      <t-icon name="delete" size="32px" />
      <h2>{{ t('docs.pages.gone') }}</h2>
      <p>{{ t('docs.pages.goneHint', { time: formatDate(gone.deleted_at) }) }}</p>
      <p v-if="!gone.restorable" class="gone-hint">{{ t('docs.pages.restoreNotAllowed') }}</p>
      <t-button v-else theme="primary" :loading="restoring" @click="restore">{{ t('docs.pages.restore') }}</t-button>
    </div>

    <div v-else-if="notFound" class="page-gone">
      <t-icon name="error-circle" size="32px" />
      <h2>{{ t('docs.pages.notFound') }}</h2>
    </div>

    <article v-else-if="page" class="page-article">
      <header class="page-header">
        <div class="page-icon-row">
          <t-popup v-if="page.can_edit" trigger="click" placement="bottom-left" :visible="iconOpen"
            @visible-change="(v: boolean) => (iconOpen = v)">
            <button type="button" class="page-icon" :class="{ 'page-icon--empty': !page.icon }"
              :aria-label="t('docs.pages.iconPlaceholder')">
              <span v-if="page.icon">{{ page.icon }}</span>
              <t-icon v-else name="file" size="28px" />
            </button>
            <template #content>
              <div class="icon-picker">
                <t-input v-model="iconDraft" :maxlength="8" :placeholder="t('docs.pages.iconPlaceholder')"
                  @enter="saveIcon(iconDraft)" />
                <div class="icon-picker-quick">
                  <button v-for="e in QUICK_ICONS" :key="e" type="button" class="icon-quick" @click="saveIcon(e)">
                    {{ e }}
                  </button>
                </div>
                <t-button v-if="page.icon" variant="text" size="small" theme="danger" @click="saveIcon('')">
                  {{ t('docs.pages.removeIcon') }}
                </t-button>
              </div>
            </template>
          </t-popup>
          <span v-else class="page-icon page-icon--static">
            <span v-if="page.icon">{{ page.icon }}</span>
            <t-icon v-else name="file" size="28px" />
          </span>
          <div class="page-badges">
            <t-tag v-if="!page.can_edit" size="small" variant="light">{{ t('docs.pages.readOnly') }}</t-tag>
            <t-tag v-if="page.is_locked" size="small" variant="light" theme="warning">
              <template #icon><t-icon name="lock-on" /></template>
              {{ t('docs.pages.locked') }}
            </t-tag>
            <t-tag v-if="page.restricted" size="small" variant="light" theme="primary">
              {{ t('docs.pages.restricted') }}
            </t-tag>
          </div>
        </div>
        <textarea v-if="page.can_edit" ref="titleInput" v-model="titleDraft" class="page-title page-title--input" rows="1"
          :placeholder="t('docs.pages.titlePlaceholder')" maxlength="500" @input="autosize" @keydown.enter.prevent="commitTitle"
          @blur="commitTitle" />
        <h1 v-else class="page-title" :class="{ 'page-title--untitled': !page.title }">
          {{ page.title || t('docs.pages.titlePlaceholder') }}
        </h1>
        <div class="page-meta">
          <!-- The live word count lives in the editor's own toolbar row;
               showing the persisted one here too would just disagree with
               it while someone is typing. -->
          <span>{{ t('docs.pages.lastEdited', { time: formatDate(page.content_updated_at || page.updated_at) }) }}</span>
          <button type="button" class="page-history-link" @click="historyOpen = true">
            <t-icon name="history" size="14px" />
            <span>{{ t('docs.history.title') }}</span>
          </button>
          <button
            type="button"
            class="page-history-link"
            :aria-pressed="watchState.watched"
            @click="toggleWatch"
          >
            <t-icon :name="watchState.watched ? 'bookmark' : 'bookmark-add'" size="14px" />
            <span>{{ watchState.watched ? t('docs.watch.watching') : t('docs.watch.watch') }}</span>
          </button>
          <button
            v-if="watchState.watched"
            type="button"
            class="page-history-link"
            :aria-pressed="watchState.muted"
            @click="toggleMute"
          >
            <t-icon :name="watchState.muted ? 'notification-off' : 'notification'" size="14px" />
            <span>{{ watchState.muted ? t('docs.watch.muted') : t('docs.watch.mute') }}</span>
          </button>
          <button
            type="button"
            class="page-history-link"
            :aria-pressed="favourite"
            @click="toggleFavourite"
          >
            <t-icon :name="favourite ? 'star-filled' : 'star'" size="14px"
              :class="{ 'star-on': favourite }" />
            <span>{{ favourite ? t('docs.home.starred') : t('docs.home.star') }}</span>
          </button>
          <button
            type="button"
            class="page-history-link"
            @click="accessOpen = true"
          >
            <t-icon :name="page.restricted ? 'lock-on' : 'usergroup'" size="14px" />
            <span>{{ page.restricted ? t('docs.access.restricted') : t('docs.access.who') }}</span>
          </button>
          <NotificationCentre :revision="notificationRevision" />
        </div>
        <PageLabels v-if="page.can_edit || labels.length" class="page-label-row" :page-id="page.id"
          :space-id="page.space_id" :can-edit="page.can_edit" :labels="labels" @change="onLabelsChanged" />
      </header>

      <div class="page-body-row">
        <DocEditor
          v-if="editingModeKnown"
          ref="docEditor"
          :key="editorKey"
          class="page-body"
          :page-id="page.id"
          :space-id="page.space_id"
          :tenant-id="tenantId"
          :can-edit="page.can_edit"
          :collab-url="collabUrl"
          :current-user="currentUser"
          :get-token="getToken"
          @headings="onHeadings"
        />
        <TocSidebar :entries="headings" @select="scrollToHeading" />
      </div>

      <CommentsPanel
        v-if="comments"
        class="page-comments"
        :threads="comments.threads.value"
        :grouped="comments.grouped.value"
        :open="comments.open.value"
        :total="comments.total.value"
        :loading="comments.loading.value"
        :active-id="comments.activeID.value"
        :show-resolved="comments.showResolved.value"
        @select="onSelectComment"
        @reply="(parentId, body) => comments?.add({ body, parent_id: parentId })"
        @edit="(id, body) => comments?.edit(id, body)"
        @resolve="(id, resolved) => comments?.setResolved(id, resolved)"
        @delete="(id) => comments?.remove(id)"
        @update:show-resolved="onShowResolved"
      />

      <!-- The box for a comment being started on the selection. It lives with
           the thread list rather than beside the text: a remark is written
           where the others are, and floating it over the passage would cover
           the very words it is about. -->
      <CommentComposer
        v-if="docEditor?.drafting"
        class="page-comment-draft"
        :placeholder="t('docs.comments.placeholder')"
        @submit="(body) => docEditor?.submitComment(body)"
        @cancel="() => docEditor?.cancelComment()"
      />

      <BacklinksPanel :entries="backlinks" :loading="backlinksLoading" />

      <section v-if="children.length" class="page-children">
        <h3>{{ t('docs.pages.subpages') }}</h3>
        <ul>
          <li v-for="c in children" :key="c.id">
            <button type="button" class="child-link" @click="goPage(c)">
              <span class="child-icon">{{ c.icon || '📄' }}</span>
              <span>{{ c.title || t('docs.tree.untitled') }}</span>
            </button>
          </li>
        </ul>
        <t-button v-if="page.can_edit" variant="text" size="small" @click="emit('createChild', page.id)">
          <template #icon><t-icon name="add" /></template>
          {{ t('docs.tree.newSubpage') }}
        </t-button>
      </section>
      <t-button v-else-if="page.can_edit" variant="text" size="small" class="add-child" @click="emit('createChild', page.id)">
        <template #icon><t-icon name="add" /></template>
        {{ t('docs.tree.newSubpage') }}
      </t-button>
    </article>

    <HistoryPanel
      v-if="page"
      :visible="historyOpen"
      :page-id="page.id"
      :can-edit="page.can_edit"
      @close="historyOpen = false"
      @restored="onRestored"
    />

    <t-dialog v-model:visible="accessOpen" :header="t('docs.access.title')" width="560px" destroy-on-close
      :footer="false">
      <PageAccessPanel v-if="page" :page-id="page.id" @changed="onAccessChanged" />
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { MessagePlugin } from 'tdesign-vue-next'
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import {
  getPageAncestors,
  getPageByShortId,
  getPageChildren,
  gonePageFrom,
  getWatchState,
  listBacklinks,
  requestStatus,
  restorePage,
  setFavourite,
  setMuted,
  setWatch,
  updatePage,
  type DocsPage,
  type DocsSpace,
  type GonePage,
  type LabelView,
  type PageAccessView,
  type PageRef,
  type PageView as PageViewDto,
  type TreeNode,
  type WatchView,
} from '@/api/docs'

import { useAuthStore } from '@/stores/auth'
import { useDeploymentCapabilitiesStore } from '@/stores/deploymentCapabilities'

import CommentComposer from './comments/CommentComposer.vue'
import PageAccessPanel from './access/PageAccessPanel.vue'
import PageLabels from './labels/PageLabels.vue'
import { browserStore, recordVisit } from './home/recentlyViewed'
import NotificationCentre from './notifications/NotificationCentre.vue'
import CommentsPanel from './comments/CommentsPanel.vue'
import BacklinksPanel from './editor/BacklinksPanel.vue'
import HistoryPanel from './history/HistoryPanel.vue'
import DocEditor from './editor/DocEditor.vue'
import type { TocEntry } from './editor/toc'
import TocSidebar from './editor/TocSidebar.vue'
import { pageSlug } from './tree/pageTree'
import type { DocsEvent } from './useDocsEvents'

const QUICK_ICONS = ['📄', '📘', '📗', '📙', '📝', '📌', '🚀', '💡', '🔧', '📊', '🗂️', '✅']

const props = defineProps<{
  space: DocsSpace
  shortId: string
  lastEvent: DocsEvent | null
}>()

const emit = defineEmits<{
  loaded: [page: PageViewDto]
  renamed: [id: string, title: string]
  restored: [page: PageViewDto]
  createChild: [parentId: string]
}>()

const { t } = useI18n()
const router = useRouter()

const page = ref<PageViewDto | null>(null)
const ancestors = ref<DocsPage[]>([])
const children = ref<TreeNode[]>([])
const loading = ref(false)
const gone = ref<GonePage | null>(null)
const notFound = ref(false)
const titleDraft = ref('')
const titleInput = ref<HTMLTextAreaElement | null>(null)
const iconOpen = ref(false)
const iconDraft = ref('')

// ---- collaborative editor wiring ------------------------------------------
const authStore = useAuthStore()
const capabilities = useDeploymentCapabilitiesStore()
const docEditor = ref<InstanceType<typeof DocEditor> | null>(null)
const historyOpen = ref(false)
const accessOpen = ref(false)

/** The badge in the header follows the panel without a refetch. */
function onAccessChanged(next: PageAccessView) {
  if (page.value) page.value = { ...page.value, restricted: next.restricted || next.inherited_from.length > 0 }
}

/**
 * Whether this reader follows the page, and whether they have silenced it.
 *
 * Watching is a reader's right rather than an editor's: you can follow a page
 * you are not allowed to change, which is what makes review work.
 */
const watchState = ref<WatchView>({ page_id: '', muted: false, watched: false })
/** Bumped when the event stream says a notification arrived, so the bell's
 * count is current without polling. */
const notificationRevision = ref(0)

/** This page's labels, and whether this person starred it. Both arrive with
 * the page itself; neither costs a second request. */
const labels = ref<LabelView[]>([])
const favourite = ref(false)

/** Where this person has been, kept on this device only. */
const visitStore = browserStore()

function onLabelsChanged(next: LabelView[]) {
  labels.value = next
}

async function toggleFavourite() {
  const current = page.value
  if (!current) return
  const next = !favourite.value
  favourite.value = next
  try {
    await setFavourite(current.id, next)
  } catch (err) {
    favourite.value = !next
    void MessagePlugin.error((err as { message?: string })?.message ?? '')
  }
}

async function loadWatchState(pageId: string) {
  try {
    watchState.value = await getWatchState(pageId)
  } catch {
    // Not knowing whether somebody follows a page is not worth an error over
    // the page itself; the button simply shows the unwatched state.
    watchState.value = { page_id: pageId, muted: false, watched: false }
  }
}

async function toggleWatch() {
  const current = page.value
  if (!current) return
  try {
    watchState.value = await setWatch(current.id, !watchState.value.watched)
  } catch (err) {
    void MessagePlugin.error((err as { message?: string })?.message ?? '')
  }
}

async function toggleMute() {
  const current = page.value
  if (!current) return
  try {
    watchState.value = await setMuted(current.id, !watchState.value.muted)
  } catch (err) {
    void MessagePlugin.error((err as { message?: string })?.message ?? '')
  }
}

/** The editor owns the comment machinery; the panel is drawn here. */
const comments = computed(() => docEditor.value?.comments ?? null)

/**
 * Selecting a thread highlights it and brings its passage into view, which is
 * what somebody clicking a comment in the list is asking for.
 */
function onSelectComment(commentId: string) {
  const handle = comments.value
  if (!handle) return
  handle.select(commentId)

  const placement = handle.placementByID.value.get(commentId)
  const editor = docEditor.value?.editor
  if (!placement || placement.from === undefined || !editor) return
  try {
    const coords = editor.view.coordsAtPos(placement.from)
    window.scrollTo({ top: window.scrollY + coords.top - 160, behavior: 'smooth' })
  } catch {
    // The position can be stale for a frame after a large remote change;
    // highlighting without scrolling is still useful.
  }
}

function onShowResolved(value: boolean) {
  const handle = comments.value
  if (!handle) return
  handle.showResolved.value = value
  void handle.load()
}

/**
 * Counted up when a restore needs the editor rebuilt; part of its key.
 *
 * Only in exclusive-edit mode. With a collaboration service the restored body
 * arrives through the same transport an ordinary edit does, and rebuilding
 * would throw away the cursor for no reason. Without one, the write went
 * straight to the stored body and the open editor knows nothing about it, so
 * it has to be built again from what is now on the server.
 */
const restoreTick = ref(0)

/**
 * A restore leaves the page's own row — its word count, its "last edited"
 * line — saying what it did before, so that is refreshed here.
 */
async function onRestored() {
  if (!collabUrl.value) restoreTick.value++
  const sid = props.shortId
  try {
    const fresh = await getPageByShortId(sid)
    // Refreshed in place rather than through load(): clearing the page would
    // unmount the editor, which is exactly what the collaborative case is
    // trying to avoid.
    if (props.shortId === sid && page.value?.id === fresh.id) {
      page.value = { ...page.value, ...fresh }
    }
  } catch {
    // The page is still on screen and correct; a stale word count in the
    // header is not worth an error message about it.
  }
}
const headings = ref<TocEntry[]>([])

const tenantId = computed(() => authStore.effectiveTenantId ?? '')
const currentUser = computed(() => authStore.user)
/** The browser-facing collaboration WebSocket address, reported by
 * GET /system/capabilities; empty in a deployment without the service. */
const collabUrl = computed(() => capabilities.docsCollabUrl)
const getToken = () => localStorage.getItem('yuheng_token')

onMounted(() => {
  // Cheap when another view already loaded it: the store caches the answer.
  void capabilities.ensureLoaded()
})

/** The editor is not mounted until the deployment capabilities have been
 * read, because an empty collaboration address means two different things
 * before and after: "not loaded yet" and "this deployment edits pages
 * exclusively". Mounting early would open the wrong transport and, in the
 * exclusive case, take a lease on a page the user is only looking at. */
const editingModeKnown = computed(() => capabilities.loaded)

/** Remounts the editor when the page changes, and also if the collaboration
 * address itself changes, since the editor binds to one Y.Doc and one
 * transport for its lifetime. */
const editorKey = computed(() => `${page.value?.id ?? ''}|${collabUrl.value}|${restoreTick.value}`)

const onHeadings = (entries: TocEntry[]) => {
  headings.value = entries
}

// ---- backlinks --------------------------------------------------------------
const backlinks = ref<PageRef[]>([])
const backlinksLoading = ref(false)

async function loadBacklinks(id: string) {
  backlinksLoading.value = true
  try {
    const rows = await listBacklinks(id)
    if (page.value?.id === id) backlinks.value = rows
  } catch {
    if (page.value?.id === id) backlinks.value = []
  } finally {
    backlinksLoading.value = false
  }
}

/** Puts the caret at the heading and lets the editor scroll it into view. */
const scrollToHeading = (pos: number) => {
  const editor = docEditor.value?.editor
  if (!editor) return
  editor.chain().setTextSelection(pos + 1).scrollIntoView().run()
}

const formatDate = (iso: string | null | undefined) => {
  if (!iso) return ''
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString()
}

const errorText = (err: unknown, fallback: string) => {
  const msg = (err as { message?: string } | null)?.message
  return msg ? `${fallback}: ${msg}` : fallback
}

async function load() {
  const sid = props.shortId
  loading.value = true
  gone.value = null
  notFound.value = false
  page.value = null
  headings.value = []
  backlinks.value = []
  children.value = []
  ancestors.value = []
  try {
    const p = await getPageByShortId(sid)
    if (props.shortId !== sid) return
    page.value = p
    titleDraft.value = p.title
    labels.value = p.labels ?? []
    favourite.value = p.favourite ?? false
    recordVisit(visitStore, {
      pageId: p.id, shortId: p.short_id, spaceSlug: props.space.slug,
      title: p.title, icon: p.icon || undefined, at: Date.now(),
    })
    emit('loaded', p)
    await Promise.all([
      loadAncestors(p.id), loadChildren(p), loadBacklinks(p.id), loadWatchState(p.id),
    ])
    await nextTick()
    autosize()
  } catch (err: unknown) {
    if (props.shortId !== sid) return
    const g = gonePageFrom(err)
    if (g) {
      gone.value = g
    } else if (requestStatus(err) === 404) {
      notFound.value = true
    } else {
      MessagePlugin.error(errorText(err, t('docs.pages.loadFailed')))
      notFound.value = true
    }
  } finally {
    if (props.shortId === sid) loading.value = false
  }
}

async function loadAncestors(id: string) {
  try {
    ancestors.value = await getPageAncestors(id)
  } catch {
    ancestors.value = []
  }
}

async function loadChildren(p: PageViewDto) {
  if (!p.has_children) {
    children.value = []
    return
  }
  try {
    const res = await getPageChildren(p.id, { limit: 200 })
    if (page.value?.id === p.id) children.value = res.items
  } catch {
    children.value = []
  }
}

// ---- title & icon -------------------------------------------------------------------
const autosize = () => {
  const el = titleInput.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${el.scrollHeight}px`
}

let savingTitle = false
async function commitTitle() {
  const p = page.value
  if (!p || savingTitle) return
  const next = titleDraft.value.replace(/[\r\n]+/g, ' ').trim()
  if (next === p.title) {
    titleDraft.value = p.title
    return
  }
  savingTitle = true
  try {
    const updated = await updatePage(p.id, { title: next })
    page.value = { ...p, title: updated.title, updated_at: updated.updated_at }
    titleDraft.value = updated.title
    emit('renamed', p.id, updated.title)
    router.replace({
      name: 'docsSpace',
      params: { slug: props.space.slug, pageSlug: pageSlug(updated.title, updated.short_id) },
    })
  } catch (err: unknown) {
    titleDraft.value = p.title
    MessagePlugin.error(errorText(err, t('docs.pages.renameFailed')))
  } finally {
    savingTitle = false
  }
}

async function saveIcon(value: string) {
  const p = page.value
  if (!p) return
  try {
    const updated = await updatePage(p.id, { icon: value.trim() })
    page.value = { ...p, icon: updated.icon ?? null }
    iconOpen.value = false
    iconDraft.value = ''
    emit('renamed', p.id, updated.title)
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.pages.renameFailed')))
  }
}

async function restore() {
  if (!gone.value) return
  restoring.value = true
  try {
    const restored = await restorePage(gone.value.page_id)
    MessagePlugin.success(t('docs.pages.restoreSuccess'))
    emit('restored', restored)
    await load()
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.pages.restoreFailed')))
  } finally {
    restoring.value = false
  }
}
const restoring = ref(false)

// ---- navigation -----------------------------------------------------------------------
const goSpace = () => router.push({ name: 'docsSpace', params: { slug: props.space.slug } })
const goPage = (p: Pick<DocsPage, 'title' | 'short_id'>) => router.push({
  name: 'docsSpace', params: { slug: props.space.slug, pageSlug: pageSlug(p.title, p.short_id) },
})

// ---- live updates ----------------------------------------------------------------------
watch(() => props.lastEvent, (ev) => {
  const p = page.value
  if (!ev || !p) return
  const payload = ev.payload ?? {}

  // Renaming any page changes what every link to it should read as, wherever
  // that link is. The title is cached in the editor rather than stored in the
  // document, so forgetting the entry is the whole of the update.
  if (ev.type === 'docs.page.meta_updated' && ev.page_id && ev.page_id !== p.id) {
    docEditor.value?.forgetTitle(ev.page_id)
  }
  // Another page's body changed, and this one may quote a block of it. What
  // is shown here is cached, not stored, so forgetting the entries is the
  // whole of the update — the same shape as the title cache above.
  if (ev.page_id && ev.page_id !== p.id
    && (ev.type === 'docs.page.content_updated' || ev.type === 'docs.page.content_replaced'
      || ev.type === 'docs.page.deleted' || ev.type === 'docs.page.purged')) {
    docEditor.value?.forgetBlockRefs(ev.page_id)
  }
  // Another page's body may have gained or lost a link to this one; the rows
  // are rebuilt whenever a page is saved.
  if (ev.page_id !== p.id
    && (ev.type === 'docs.page.content_updated' || ev.type === 'docs.page.content_replaced'
      || ev.type === 'docs.page.deleted' || ev.type === 'docs.page.purged')) {
    void loadBacklinks(p.id)
  }

  // Somebody else commented, replied, resolved or deleted on this page.
  if (ev.type === 'docs.comment.changed' && ev.page_id === p.id) {
    void comments.value?.load()
  }

  // A notification was written for somebody; the bell re-reads its own count
  // rather than trusting the event, since the event does not say whose.
  if (ev.type === 'docs.notification.created') {
    notificationRevision.value++
  }

  if (ev.page_id === p.id) {
    switch (ev.type) {
      case 'docs.page.meta_updated':
        if ('title' in payload && !savingTitle) {
          page.value = { ...p, title: String(payload.title ?? '') }
          titleDraft.value = String(payload.title ?? '')
        }
        if ('icon' in payload) page.value = { ...page.value!, icon: (payload.icon as string | null) ?? null }
        break
      case 'docs.page.content_updated':
      case 'docs.page.content_replaced':
        // A body that just changed may have gained or lost a link to this
        // page, and the backlink rows are rebuilt on save.
        void loadBacklinks(p.id)
        // Nothing to refetch: the body is the Yjs document the editor is
        // already connected to, and the collaboration service pushes both
        // a peer's edits and a server-side replace straight into it.
        break
      case 'docs.page.deleted':
      case 'docs.page.moved':
      case 'docs.page.purged':
        void load()
        break
      default:
        break
    }
    return
  }
  // A child was added, removed or renamed: refresh the subpage list.
  const parentId = payload.parent_id as string | null | undefined
  if (parentId === p.id || children.value.some((c) => c.id === ev.page_id)) {
    void loadChildren({ ...p, has_children: true })
  }
})

watch(() => props.shortId, load, { immediate: true })
</script>

<style scoped lang="less">
.docs-page-view {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 16px 48px 64px;
}

.page-crumbs {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 2px;
  margin-bottom: 20px;
  font-size: 13px;
  color: var(--td-text-color-secondary);
}

.crumb {
  border: none;
  background: transparent;
  padding: 2px 6px;
  border-radius: 4px;
  color: inherit;
  cursor: pointer;
  font-size: inherit;
  max-width: 24ch;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;

  &:hover {
    background: var(--td-bg-color-container-hover);
    color: var(--td-text-color-primary);
  }

  &--current {
    color: var(--td-text-color-primary);
    cursor: default;

    &:hover {
      background: transparent;
    }
  }
}

.crumb-sep {
  color: var(--td-text-color-placeholder);
}

.page-article {
  max-width: 820px;
  margin: 0 auto;
}

.page-icon-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.page-icon {
  width: 44px;
  height: 44px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 8px;
  background: transparent;
  font-size: 30px;
  line-height: 1;
  color: var(--td-text-color-secondary);
  cursor: pointer;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }

  &--static {
    cursor: default;

    &:hover {
      background: transparent;
    }
  }
}

.page-badges {
  display: inline-flex;
  gap: 6px;
}

.icon-picker {
  width: 260px;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.icon-picker-quick {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 4px;
}

.icon-quick {
  border: none;
  background: transparent;
  border-radius: 6px;
  font-size: 20px;
  padding: 4px 0;
  cursor: pointer;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }
}

.page-title {
  width: 100%;
  margin: 0;
  padding: 0;
  border: none;
  outline: none;
  resize: none;
  background: transparent;
  color: var(--td-text-color-primary);
  font-family: var(--app-font-family);
  font-size: 32px;
  font-weight: 700;
  line-height: 1.25;
  overflow: hidden;

  &--input::placeholder,
  &--untitled {
    color: var(--td-text-color-placeholder);
  }
}

.page-meta {
  margin-top: 8px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.page-label-row {
  margin-top: 10px;
}

.star-on {
  color: var(--td-warning-color);
}

.page-body-row {
  display: flex;
  align-items: flex-start;
  gap: 24px;
  margin-top: 16px;
}

.page-body {
  flex: 1;
  min-width: 0;
  font-size: 15px;
  line-height: 1.75;
  color: var(--td-text-color-primary);

  :deep(h1),
  :deep(h2),
  :deep(h3) {
    margin: 1.4em 0 0.5em;
    font-weight: 600;
    line-height: 1.3;
  }

  :deep(p) {
    margin: 0.5em 0;
  }

  :deep(pre) {
    padding: 12px 14px;
    border-radius: 8px;
    background: var(--td-bg-color-secondarycontainer);
    overflow-x: auto;
    font-size: 13px;
  }

  :deep(code) {
    font-family: var(--td-font-family-mono, ui-monospace, monospace);
  }

  :deep(blockquote) {
    margin: 0.8em 0;
    padding: 4px 14px;
    border-left: 3px solid var(--td-brand-color);
    color: var(--td-text-color-secondary);
  }

  :deep(table) {
    border-collapse: collapse;
    width: 100%;
    margin: 1em 0;
  }

  :deep(th),
  :deep(td) {
    border: 1px solid var(--td-component-stroke);
    padding: 6px 10px;
    text-align: left;
  }

  :deep(img) {
    max-width: 100%;
    border-radius: 6px;
  }

  :deep(a) {
    color: var(--td-brand-color);
  }

  :deep(ul),
  :deep(ol) {
    padding-left: 1.6em;
  }
}

.page-empty {
  margin-top: 24px;
  color: var(--td-text-color-placeholder);
}

.page-children {
  margin-top: 40px;
  padding-top: 16px;
  border-top: 1px solid var(--td-component-stroke);

  h3 {
    margin: 0 0 8px;
    font-size: 13px;
    font-weight: 600;
    color: var(--td-text-color-secondary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  ul {
    list-style: none;
    margin: 0 0 8px;
    padding: 0;
  }
}

.child-link {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--td-text-color-primary);
  font-size: 14px;
  cursor: pointer;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }
}

.child-icon {
  width: 20px;
  text-align: center;
}

.add-child {
  margin-top: 32px;
}

.page-loading {
  max-width: 820px;
  margin: 24px auto;
}

.page-gone {
  max-width: 520px;
  margin: 80px auto;
  text-align: center;
  color: var(--td-text-color-secondary);

  h2 {
    margin: 12px 0 8px;
    color: var(--td-text-color-primary);
    font-size: 18px;
  }

  p {
    margin: 0 0 12px;
  }
}

.gone-hint {
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}
</style>
