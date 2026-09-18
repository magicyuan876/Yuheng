<template>
  <div class="docs-space-home">
    <aside class="space-sidebar">
      <div class="sidebar-top">
        <t-button variant="text" size="small" class="back-btn" @click="router.push({ name: 'docsSpaceList' })">
          <template #icon><t-icon name="chevron-left" /></template>
          {{ t('docs.spaces.backToList') }}
        </t-button>
        <div v-if="space" class="space-heading">
          <SpaceAvatar :name="space.name" :avatar="space.icon || ''" size="small" />
          <span class="space-name" :title="space.name">{{ space.name }}</span>
          <span class="space-tools">
            <t-tooltip v-if="canEdit" :content="t('docs.tree.newPage')">
              <t-button variant="text" size="small" shape="square" :aria-label="t('docs.tree.newPage')"
                :loading="creating === ROOT_KEY" @click="createUnder(null)">
                <template #icon><t-icon name="add" /></template>
              </t-button>
            </t-tooltip>
            <t-tooltip :content="t('docs.trash.title')">
              <t-button variant="text" size="small" shape="square" :aria-label="t('docs.trash.title')"
                @click="trashVisible = true">
                <template #icon><t-icon name="delete" /></template>
              </t-button>
            </t-tooltip>
            <t-tooltip :content="t('docs.tree.settings')">
              <t-button variant="text" size="small" shape="square" :aria-label="t('docs.tree.settings')"
                @click="router.push({ name: 'docsSpaceSettings', params: { slug } })">
                <template #icon><t-icon name="setting" /></template>
              </t-button>
            </t-tooltip>
          </span>
        </div>
        <t-skeleton v-else animation="gradient" :row-col="[{ width: '70%' }]" />
      </div>

      <PageTree :model="model" :version="version" :active-id="activePageId" :can-edit="canEdit" :loading="treeLoading"
        @select="onSelect" @toggle="onToggle" @move="onMove" @action="onAction" />
    </aside>

    <main class="space-main">
      <PageView v-if="space && shortId" :space="space" :short-id="shortId" :last-event="lastEvent"
        @loaded="onPageLoaded" @renamed="onRenamed" @restored="onRestored" @create-child="createUnder" />
      <div v-else-if="space" class="space-welcome">
        <SpaceAvatar :name="space.name" :avatar="space.icon || ''" size="large" />
        <h1>{{ t('docs.pages.welcome', { name: space.name }) }}</h1>
        <p class="welcome-description">{{ space.description || t('docs.pages.welcomeHint') }}</p>
        <div v-if="rootNodes.length" class="root-pages">
          <div class="root-pages-title">{{ t('docs.pages.rootPages') }}</div>
          <button v-for="n in rootNodes" :key="n.id" type="button" class="root-page" @click="openPage(n)">
            <span class="root-page-icon">{{ n.icon || '📄' }}</span>
            <span class="root-page-title">{{ n.title || t('docs.tree.untitled') }}</span>
          </button>
        </div>
        <t-button v-if="canEdit" theme="primary" :loading="creating === ROOT_KEY" @click="createUnder(null)">
          <template #icon><t-icon name="add" /></template>
          {{ t('docs.tree.newPage') }}
        </t-button>
      </div>
      <div v-else-if="spaceMissing" class="space-missing">{{ t('docs.spaces.loadFailed') }}</div>
    </main>

    <!-- Rename -->
    <t-dialog v-model:visible="renameVisible" :header="t('docs.tree.rename')" width="460px" destroy-on-close
      :confirm-btn="{ content: t('common.save'), loading: renaming }" :cancel-btn="t('common.cancel')"
      @confirm="submitRename">
      <t-input v-model="renameTitle" :maxlength="500" :placeholder="t('docs.tree.untitled')" autofocus
        @enter="submitRename" />
    </t-dialog>

    <!-- Move to another space -->
    <t-dialog v-model:visible="moveVisible" :header="t('docs.tree.moveDialogTitle')" width="480px" destroy-on-close
      :confirm-btn="{ content: t('common.confirm'), loading: moving, disabled: !moveTargetId }"
      :cancel-btn="t('common.cancel')" @confirm="submitMoveToSpace">
      <p class="dialog-hint">{{ t('docs.tree.moveDialogHint') }}</p>
      <t-select v-model="moveTargetId" :options="moveTargets" :loading="spacesLoading"
        :placeholder="t('docs.tree.moveDialogPick')" :empty="t('docs.tree.moveDialogNoSpaces')" />
    </t-dialog>

    <!-- Delete -->
    <t-dialog v-model:visible="deleteVisible" :header="t('docs.tree.delete')" theme="danger" width="460px"
      :confirm-btn="{ content: t('common.delete'), theme: 'danger', loading: deleting }"
      :cancel-btn="t('common.cancel')" @confirm="submitDelete">
      <p>{{ t('docs.tree.deleteConfirm', { title: pendingNode?.title || t('docs.tree.untitled') }) }}</p>
    </t-dialog>

    <t-drawer v-model:visible="trashVisible" :header="t('docs.trash.title')" size="440px" :footer="false"
      destroy-on-close>
      <TrashPanel v-if="space" :space="space" @restored="onRestored" />
    </t-drawer>
  </div>
</template>

<script setup lang="ts">
import { MessagePlugin } from 'tdesign-vue-next'
import { computed, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import {
  createPage,
  deletePage,
  duplicatePage,
  getPageAncestors,
  getSpaceBySlug,
  listSpaces,
  loadAllChildren,
  movePage,
  requestStatus,
  updatePage,
  type DocsSpace,
  type PageView as PageViewDto,
} from '@/api/docs'
import SpaceAvatar from '@/components/SpaceAvatar.vue'

import { canEditSpaceContent, roleAtLeast } from './docsAccess'
import PageView from './PageView.vue'
import PageTree, { type TreeAction } from './tree/PageTree.vue'
import { PageTreeModel, pageSlug, shortIdFromSlug, type MoveTarget, type TreeNodeData } from './tree/pageTree'
import TrashPanel from './TrashPanel.vue'
import { useDocsEvents, type DocsEvent } from './useDocsEvents'

const ROOT_KEY = '__root__'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const slug = computed(() => String(route.params.slug ?? ''))
const shortId = computed(() => {
  const s = route.params.pageSlug
  return typeof s === 'string' && s ? shortIdFromSlug(s) ?? undefined : undefined
})

const space = shallowRef<DocsSpace | null>(null)
const spaceMissing = ref(false)
const spaceId = computed(() => space.value?.id)
const canEdit = computed(() => canEditSpaceContent(space.value?.role))

// ---- tree state ---------------------------------------------------------------------
const model = new PageTreeModel()
const version = ref(0)
const bump = () => {
  version.value += 1
}
const treeLoading = ref(false)
const activePageId = ref<string | undefined>()
const rootNodes = computed(() => {
  void version.value
  return model.childIds(null).map((id) => model.get(id)!).filter(Boolean)
})

const errorText = (err: unknown, fallback: string) => {
  const msg = (err as { message?: string } | null)?.message
  return msg ? `${fallback}: ${msg}` : fallback
}

async function loadChildren(parentId: string | null) {
  if (!space.value) return
  const sid = space.value.id
  const list = await loadAllChildren(sid, parentId)
  if (space.value?.id !== sid) return
  model.setChildren(parentId, list)
  bump()
}

async function loadSpace() {
  const s = slug.value
  space.value = null
  spaceMissing.value = false
  activePageId.value = undefined
  model.reset()
  bump()
  if (!s) return
  try {
    const sp = await getSpaceBySlug(s)
    if (slug.value !== s) return
    space.value = sp
    treeLoading.value = true
    try {
      await loadChildren(null)
    } finally {
      treeLoading.value = false
    }
    if (shortId.value) await revealActive()
  } catch (err: unknown) {
    spaceMissing.value = true
    if (requestStatus(err) !== 404) MessagePlugin.error(errorText(err, t('docs.spaces.loadFailed')))
  }
}

/** Expands and loads the ancestor chain of the page named in the URL. */
async function revealActive() {
  if (!space.value || !activePageId.value) return
  try {
    const ancestors = await getPageAncestors(activePageId.value)
    for (const a of ancestors) {
      if (!model.isLoaded(a.id)) await loadChildren(a.id)
      model.expand(a.id)
    }
    bump()
  } catch {
    // The page view reports its own errors; the tree just stays as it is.
  }
}

const onSelect = (node: TreeNodeData) => {
  openPage(node)
}

const openPage = (node: Pick<TreeNodeData, 'title' | 'short_id'>) => {
  router.push({ name: 'docsSpace', params: { slug: slug.value, pageSlug: pageSlug(node.title, node.short_id) } })
}

const onToggle = async (node: TreeNodeData) => {
  const open = model.toggle(node.id)
  bump()
  if (open && (!model.isLoaded(node.id) || model.isStale(node.id))) {
    try {
      await loadChildren(node.id)
    } catch (err: unknown) {
      MessagePlugin.error(errorText(err, t('docs.tree.loadFailed')))
    }
  }
}

const onMove = async (dragId: string, target: MoveTarget) => {
  const node = model.get(dragId)
  if (!node) return
  try {
    const result = await movePage(dragId, {
      parent_id: target.parentId,
      ...(target.afterId === undefined ? {} : { after_id: target.afterId }),
    })
    if (result.rebalanced || (target.parentId && !model.isLoaded(target.parentId))) {
      await loadChildren(target.parentId)
    } else {
      model.move(dragId, target.parentId, result.page.position)
    }
    if (target.parentId) model.expand(target.parentId)
    bump()
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.tree.moveFailed')))
  }
}

// ---- actions -------------------------------------------------------------------------------
const creating = ref<string | null>(null)
const pendingNode = ref<TreeNodeData | null>(null)

async function createUnder(parentId: string | null) {
  if (!space.value) return
  creating.value = parentId ?? ROOT_KEY
  try {
    const page = await createPage({ space_id: space.value.id, parent_id: parentId, title: '' })
    if (parentId) {
      if (!model.isLoaded(parentId)) await loadChildren(parentId)
      model.expand(parentId)
    }
    model.insert(toNode(page))
    bump()
    openPage(page)
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.tree.createFailed')))
  } finally {
    creating.value = null
  }
}

function toNode(p: PageViewDto): TreeNodeData {
  return {
    id: p.id, short_id: p.short_id, space_id: p.space_id, parent_id: p.parent_id, position: p.position,
    title: p.title, icon: p.icon ?? null, has_children: p.has_children, can_edit: p.can_edit,
    restricted: p.restricted, updated_at: p.updated_at,
  }
}

const onAction = (action: TreeAction, node: TreeNodeData) => {
  pendingNode.value = node
  switch (action) {
    case 'new-child':
      void createUnder(node.id)
      break
    case 'rename':
      renameTitle.value = node.title
      renameVisible.value = true
      break
    case 'duplicate':
      void duplicate(node)
      break
    case 'copy-link':
      void copyLink(node)
      break
    case 'move-space':
      moveTargetId.value = ''
      moveVisible.value = true
      void loadMoveTargets()
      break
    case 'delete':
      deleteVisible.value = true
      break
    default:
      break
  }
}

// rename
const renameVisible = ref(false)
const renameTitle = ref('')
const renaming = ref(false)
async function submitRename() {
  const node = pendingNode.value
  if (!node) return
  renaming.value = true
  try {
    const page = await updatePage(node.id, { title: renameTitle.value.trim() })
    model.update(node.id, { title: page.title })
    bump()
    renameVisible.value = false
    if (activePageId.value === node.id) {
      router.replace({ name: 'docsSpace', params: { slug: slug.value, pageSlug: pageSlug(page.title, page.short_id) } })
    }
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.pages.renameFailed')))
  } finally {
    renaming.value = false
  }
}

async function duplicate(node: TreeNodeData) {
  try {
    const result = await duplicatePage(node.id, { title: t('docs.tree.copySuffix', { title: node.title || t('docs.tree.untitled') }) })
    model.insert(toNode(result.page))
    bump()
    MessagePlugin.success(t('docs.tree.duplicateSuccess'))
    openPage(result.page)
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.tree.duplicateFailed')))
  }
}

async function copyLink(node: TreeNodeData) {
  const href = router.resolve({
    name: 'docsSpace', params: { slug: slug.value, pageSlug: pageSlug(node.title, node.short_id) },
  }).href
  try {
    await navigator.clipboard.writeText(`${window.location.origin}${href}`)
    MessagePlugin.success(t('docs.tree.linkCopied'))
  } catch {
    MessagePlugin.warning(`${window.location.origin}${href}`)
  }
}

// move to space
const moveVisible = ref(false)
const moveTargetId = ref('')
const moving = ref(false)
const spacesLoading = ref(false)
const otherSpaces = ref<DocsSpace[]>([])
const moveTargets = computed(() => otherSpaces.value
  .filter((s) => s.id !== space.value?.id && roleAtLeast(s.role, 'writer'))
  .map((s) => ({ label: s.name, value: s.id })))

async function loadMoveTargets() {
  spacesLoading.value = true
  try {
    otherSpaces.value = await listSpaces()
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.spaces.loadFailed')))
  } finally {
    spacesLoading.value = false
  }
}

async function submitMoveToSpace() {
  const node = pendingNode.value
  const target = otherSpaces.value.find((s) => s.id === moveTargetId.value)
  if (!node || !target) return
  moving.value = true
  try {
    const result = await movePage(node.id, { space_id: target.id, parent_id: null })
    const wasActive = !!activePageId.value && model.isDescendant(node.id, activePageId.value)
    model.remove(node.id)
    if (result.orphaned?.length) await loadChildren(null)
    bump()
    moveVisible.value = false
    MessagePlugin.success(t('docs.tree.moved', { name: target.name }))
    if (result.orphaned?.length) MessagePlugin.info(t('docs.tree.orphanedNotice', { count: result.orphaned.length }))
    if (wasActive) router.push({ name: 'docsSpace', params: { slug: slug.value } })
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.tree.moveFailed')))
  } finally {
    moving.value = false
  }
}

// delete
const deleteVisible = ref(false)
const deleting = ref(false)
async function submitDelete() {
  const node = pendingNode.value
  if (!node) return
  deleting.value = true
  try {
    await deletePage(node.id)
    const wasActive = !!activePageId.value && model.isDescendant(node.id, activePageId.value)
    model.remove(node.id)
    bump()
    deleteVisible.value = false
    MessagePlugin.success(t('docs.tree.deleteSuccess'))
    if (wasActive) router.push({ name: 'docsSpace', params: { slug: slug.value } })
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.tree.deleteFailed')))
  } finally {
    deleting.value = false
  }
}

// ---- page view callbacks -----------------------------------------------------------------
const onPageLoaded = async (page: PageViewDto) => {
  activePageId.value = page.id
  if (!model.has(page.id)) {
    await revealActive()
  } else {
    model.reveal(page.id)
    bump()
  }
}

const onRenamed = (id: string, title: string) => {
  model.update(id, { title })
  bump()
}

const onRestored = async (page: PageViewDto) => {
  if (page.parent_id && !model.isLoaded(page.parent_id)) {
    await loadChildren(page.parent_id)
  } else {
    model.insert(toNode(page))
  }
  if (page.parent_id) model.expand(page.parent_id)
  bump()
  trashVisible.value = false
  openPage(page)
}

const trashVisible = ref(false)

// ---- live updates --------------------------------------------------------------------------
const lastEvent = shallowRef<DocsEvent | null>(null)
useDocsEvents(spaceId, (ev) => {
  lastEvent.value = ev
  if (!space.value) return
  const changed = model.applyEvent(ev, space.value.id, canEdit.value)
  for (const parent of model.takeStale()) {
    if (parent === null || model.isExpanded(parent)) void loadChildren(parent).catch(() => undefined)
  }
  if (changed) bump()
})

watch(slug, loadSpace, { immediate: true })
watch(shortId, (id) => {
  if (!id) activePageId.value = undefined
})
onBeforeUnmount(() => {
  lastEvent.value = null
})
</script>

<style scoped lang="less">
.docs-space-home {
  flex: 1;
  display: flex;
  min-width: 0;
  min-height: 0;
}

.space-sidebar {
  flex: none;
  width: 280px;
  display: flex;
  flex-direction: column;
  min-height: 0;
  border-right: 1px solid var(--td-component-stroke);
  background: var(--td-bg-color-secondarycontainer, var(--td-bg-color-container));
  padding: 12px 8px 8px 8px;
}

.sidebar-top {
  flex: none;
  padding: 0 4px 8px 4px;
}

.back-btn {
  margin-left: -6px;
  margin-bottom: 6px;
}

.space-heading {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.space-name {
  flex: 1;
  min-width: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.space-tools {
  flex: none;
  display: inline-flex;
  gap: 2px;
}

.space-main {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.space-welcome {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 40px 24px;
  text-align: center;
  overflow-y: auto;

  h1 {
    margin: 8px 0 0;
    font-size: 22px;
    font-weight: 600;
    color: var(--td-text-color-primary);
  }
}

.welcome-description {
  max-width: 56ch;
  margin: 0 0 8px;
  color: var(--td-text-color-secondary);
  font-size: 14px;
  line-height: 22px;
}

.root-pages {
  width: min(520px, 100%);
  text-align: left;
  margin-bottom: 8px;
}

.root-pages-title {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-bottom: 6px;
}

.root-page {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 8px 10px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
  cursor: pointer;
  text-align: left;
  margin-bottom: 6px;

  &:hover {
    border-color: var(--td-brand-color);
  }
}

.root-page-icon {
  flex: none;
  width: 20px;
  text-align: center;
}

.root-page-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
}

.space-missing {
  padding: 48px;
  text-align: center;
  color: var(--td-text-color-secondary);
}

.dialog-hint {
  margin: 0 0 12px;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  line-height: 20px;
}
</style>
