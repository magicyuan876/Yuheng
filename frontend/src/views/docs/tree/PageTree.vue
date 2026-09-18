<template>
  <div ref="viewport" class="page-tree" role="tree" tabindex="0" :aria-activedescendant="activeId || undefined"
    @scroll.passive="onScroll" @keydown="onKeydown" @dragover.prevent @drop.prevent="onDropOutside">
    <div v-if="rows.length === 0 && !loading" class="tree-empty">
      <div class="tree-empty-title">{{ t('docs.tree.empty') }}</div>
      <div v-if="canEdit" class="tree-empty-hint">{{ t('docs.tree.emptyHint') }}</div>
    </div>
    <div v-else class="tree-spacer" :style="{ height: rows.length * ROW_HEIGHT + 'px' }">
      <div v-for="item in visible" :key="item.row.node.id" class="tree-row" role="treeitem"
        :id="'tree-row-' + item.row.node.id"
        :class="{
          'tree-row--active': item.row.node.id === activeId,
          'tree-row--dragging': item.row.node.id === dragId,
          'tree-row--drop-inside': indicator?.id === item.row.node.id && indicator.position === 'inside',
          'tree-row--drop-before': indicator?.id === item.row.node.id && indicator.position === 'before',
          'tree-row--drop-after': indicator?.id === item.row.node.id && indicator.position === 'after',
        }"
        :style="{ transform: `translateY(${item.index * ROW_HEIGHT}px)`, paddingLeft: 8 + item.row.depth * 16 + 'px' }"
        :aria-level="item.row.depth + 1" :aria-expanded="item.row.node.has_children ? item.row.expanded : undefined"
        :aria-selected="item.row.node.id === activeId"
        :draggable="canEdit && item.row.node.can_edit ? 'true' : 'false'"
        @click="emit('select', item.row.node)"
        @contextmenu.prevent="openMenu($event, item.row.node)"
        @dragstart="onDragStart($event, item.row.node)" @dragend="clearDrag"
        @dragover.prevent.stop="onDragOver($event, item.row.node)" @dragleave="onDragLeave(item.row.node)"
        @drop.prevent.stop="onDrop(item.row.node)">
        <button type="button" class="tree-toggle" :class="{ 'tree-toggle--hidden': !item.row.node.has_children }"
          :aria-label="item.row.expanded ? t('docs.tree.collapse') : t('docs.tree.expand')" tabindex="-1"
          @click.stop="emit('toggle', item.row.node)">
          <t-icon name="chevron-right" size="14px" :class="{ 'tree-toggle-icon--open': item.row.expanded }" />
        </button>
        <span class="tree-icon" aria-hidden="true">
          <span v-if="item.row.node.icon" class="tree-emoji">{{ item.row.node.icon }}</span>
          <t-icon v-else name="file" size="15px" />
        </span>
        <span class="tree-title" :class="{ 'tree-title--untitled': !item.row.node.title }">
          {{ item.row.node.title || t('docs.tree.untitled') }}
        </span>
        <t-icon v-if="item.row.node.restricted" name="lock-on" size="12px" class="tree-flag"
          :aria-label="t('docs.pages.restricted')" />
        <span class="tree-actions" @click.stop>
          <button v-if="canEdit && item.row.node.can_edit" type="button" class="tree-action" tabindex="-1"
            :aria-label="t('docs.tree.newSubpage')" @click="emit('action', 'new-child', item.row.node)">
            <t-icon name="add" size="14px" />
          </button>
          <button type="button" class="tree-action" tabindex="-1" :aria-label="t('docs.tree.moreActions')"
            @click="openMenu($event, item.row.node)">
            <t-icon name="ellipsis" size="14px" />
          </button>
        </span>
      </div>
    </div>

    <Teleport to="body">
      <div v-if="menu" class="page-tree-menu" :style="{ left: menu.x + 'px', top: menu.y + 'px' }" role="menu"
        @click.stop @contextmenu.prevent>
        <button v-for="entry in menuEntries" :key="entry.action" type="button" class="page-tree-menu-item"
          :class="{ 'page-tree-menu-item--danger': entry.danger }" role="menuitem" :disabled="entry.disabled"
          @click="runMenu(entry.action)">
          <t-icon :name="entry.icon" size="14px" />
          <span>{{ entry.label }}</span>
        </button>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { dropPositionFor, type DropPosition, type MoveTarget, type PageTreeModel, type TreeNodeData, type TreeRow }
  from './pageTree'

export type TreeAction = 'new-child' | 'rename' | 'duplicate' | 'copy-link' | 'move-space' | 'delete'

const props = defineProps<{
  model: PageTreeModel
  /** Bumped by the owner after every model mutation. */
  version: number
  activeId?: string
  /** Whether the caller may write in this space at all. */
  canEdit: boolean
  loading?: boolean
}>()

const emit = defineEmits<{
  select: [node: TreeNodeData]
  toggle: [node: TreeNodeData]
  move: [dragId: string, target: MoveTarget]
  action: [action: TreeAction, node: TreeNodeData]
}>()

const { t } = useI18n()
const ROW_HEIGHT = 32
const BUFFER = 8

const viewport = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const viewportHeight = ref(600)

const rows = computed<TreeRow[]>(() => {
  void props.version
  return props.model.rows()
})

const visible = computed(() => {
  const start = Math.max(0, Math.floor(scrollTop.value / ROW_HEIGHT) - BUFFER)
  const end = Math.min(rows.value.length, Math.ceil((scrollTop.value + viewportHeight.value) / ROW_HEIGHT) + BUFFER)
  const out: Array<{ row: TreeRow; index: number }> = []
  for (let i = start; i < end; i++) out.push({ row: rows.value[i], index: i })
  return out
})

const onScroll = () => {
  scrollTop.value = viewport.value?.scrollTop ?? 0
}

let resizeObserver: ResizeObserver | null = null
onMounted(() => {
  if (viewport.value) {
    viewportHeight.value = viewport.value.clientHeight || 600
    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => {
        viewportHeight.value = viewport.value?.clientHeight || 600
      })
      resizeObserver.observe(viewport.value)
    }
  }
  document.addEventListener('click', closeMenu)
  document.addEventListener('keydown', onDocumentKey)
})
onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  document.removeEventListener('click', closeMenu)
  document.removeEventListener('keydown', onDocumentKey)
})

/** Scroll the active row into view when it changes (e.g. after navigation). */
watch(() => [props.activeId, rows.value.length] as const, () => {
  if (!props.activeId || !viewport.value) return
  const index = rows.value.findIndex((r) => r.node.id === props.activeId)
  if (index < 0) return
  const top = index * ROW_HEIGHT
  const el = viewport.value
  if (top < el.scrollTop || top + ROW_HEIGHT > el.scrollTop + el.clientHeight) {
    el.scrollTop = Math.max(0, top - el.clientHeight / 2)
  }
})

// ---- drag and drop -------------------------------------------------------------
const dragId = ref<string | null>(null)
const indicator = ref<{ id: string; position: DropPosition } | null>(null)

const onDragStart = (e: DragEvent, node: TreeNodeData) => {
  if (!props.canEdit || !node.can_edit) {
    e.preventDefault()
    return
  }
  dragId.value = node.id
  e.dataTransfer?.setData('text/plain', node.id)
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}

const onDragOver = (e: DragEvent, node: TreeNodeData) => {
  if (!dragId.value) return
  const el = e.currentTarget as HTMLElement
  const offsetY = e.clientY - el.getBoundingClientRect().top
  const position = dropPositionFor(offsetY, ROW_HEIGHT, node.can_edit)
  const allowed = props.model.dropTarget(dragId.value, node.id, position) !== null
  if (e.dataTransfer) e.dataTransfer.dropEffect = allowed ? 'move' : 'none'
  indicator.value = allowed ? { id: node.id, position } : null
}

const onDragLeave = (node: TreeNodeData) => {
  if (indicator.value?.id === node.id) indicator.value = null
}

const onDrop = (node: TreeNodeData) => {
  const id = dragId.value
  const ind = indicator.value
  clearDrag()
  if (!id || !ind || ind.id !== node.id) return
  const target = props.model.dropTarget(id, node.id, ind.position)
  if (target) emit('move', id, target)
}

/** Dropping on the empty area below the rows appends to the root. */
const onDropOutside = () => {
  const id = dragId.value
  clearDrag()
  if (!id) return
  const node = props.model.get(id)
  if (node && node.parent_id !== null) emit('move', id, { parentId: null, afterId: undefined })
}

const clearDrag = () => {
  dragId.value = null
  indicator.value = null
}

// ---- context menu -----------------------------------------------------------------
const menu = ref<{ x: number; y: number; node: TreeNodeData } | null>(null)

const openMenu = (e: MouseEvent, node: TreeNodeData) => {
  const x = Math.min(e.clientX, window.innerWidth - 220)
  const y = Math.min(e.clientY, window.innerHeight - 260)
  menu.value = { x, y, node }
}

const closeMenu = () => {
  menu.value = null
}

const onDocumentKey = (e: KeyboardEvent) => {
  if (e.key === 'Escape') closeMenu()
}

const menuEntries = computed(() => {
  const node = menu.value?.node
  const editable = !!node && props.canEdit && node.can_edit
  return [
    { action: 'new-child' as TreeAction, icon: 'add', label: t('docs.tree.newSubpage'), disabled: !editable },
    { action: 'rename' as TreeAction, icon: 'edit-1', label: t('docs.tree.rename'), disabled: !editable },
    { action: 'duplicate' as TreeAction, icon: 'file-copy', label: t('docs.tree.duplicate'), disabled: !props.canEdit },
    { action: 'copy-link' as TreeAction, icon: 'link', label: t('docs.tree.copyLink'), disabled: false },
    { action: 'move-space' as TreeAction, icon: 'swap', label: t('docs.tree.moveToSpace'), disabled: !editable },
    { action: 'delete' as TreeAction, icon: 'delete', label: t('docs.tree.delete'), disabled: !editable, danger: true },
  ]
})

const runMenu = (action: TreeAction) => {
  const node = menu.value?.node
  closeMenu()
  if (node) emit('action', action, node)
}

// ---- keyboard ------------------------------------------------------------------------
const onKeydown = (e: KeyboardEvent) => {
  if (!rows.value.length) return
  const index = rows.value.findIndex((r) => r.node.id === props.activeId)
  const current = index >= 0 ? rows.value[index] : null
  switch (e.key) {
    case 'ArrowDown': {
      e.preventDefault()
      const next = rows.value[Math.min(rows.value.length - 1, index + 1)]
      if (next) emit('select', next.node)
      break
    }
    case 'ArrowUp': {
      e.preventDefault()
      const prev = rows.value[Math.max(0, index - 1)]
      if (prev) emit('select', prev.node)
      break
    }
    case 'ArrowRight':
      if (current?.node.has_children && !current.expanded) {
        e.preventDefault()
        emit('toggle', current.node)
      }
      break
    case 'ArrowLeft':
      if (current?.expanded) {
        e.preventDefault()
        emit('toggle', current.node)
      } else if (current?.node.parent_id) {
        e.preventDefault()
        const parent = props.model.get(current.node.parent_id)
        if (parent) emit('select', parent)
      }
      break
    default:
      break
  }
}
</script>

<style scoped lang="less">
.page-tree {
  position: relative;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  outline: none;
  user-select: none;

  &:focus-visible {
    box-shadow: inset 0 0 0 1px var(--td-brand-color-focus);
  }
}

.tree-spacer {
  position: relative;
  width: 100%;
}

.tree-row {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 32px;
  display: flex;
  align-items: center;
  gap: 4px;
  padding-right: 6px;
  border-radius: 6px;
  color: var(--td-text-color-primary);
  cursor: pointer;
  box-sizing: border-box;

  &:hover {
    background: var(--td-bg-color-container-hover);

    .tree-actions {
      opacity: 1;
    }
  }

  &--active {
    background: var(--td-brand-color-light);
    color: var(--td-brand-color);
  }

  &--dragging {
    opacity: 0.4;
  }

  &--drop-inside {
    box-shadow: inset 0 0 0 2px var(--td-brand-color);
  }

  &--drop-before::before,
  &--drop-after::after {
    content: '';
    position: absolute;
    left: 8px;
    right: 8px;
    height: 2px;
    background: var(--td-brand-color);
    border-radius: 1px;
  }

  &--drop-before::before {
    top: -1px;
  }

  &--drop-after::after {
    bottom: -1px;
  }
}

.tree-toggle {
  flex: none;
  width: 20px;
  height: 20px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--td-text-color-secondary);
  cursor: pointer;
  padding: 0;

  &:hover {
    background: var(--td-bg-color-secondarycontainer);
  }

  &--hidden {
    visibility: hidden;
  }
}

.tree-toggle-icon--open {
  transform: rotate(90deg);
}

.tree-toggle :deep(.t-icon) {
  transition: transform 0.15s ease;
}

.tree-icon {
  flex: none;
  width: 20px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--td-text-color-secondary);
}

.tree-emoji {
  font-size: 15px;
  line-height: 1;
}

.tree-title {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  line-height: 20px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;

  &--untitled {
    color: var(--td-text-color-placeholder);
  }
}

.tree-flag {
  flex: none;
  color: var(--td-text-color-placeholder);
}

.tree-actions {
  flex: none;
  display: inline-flex;
  gap: 2px;
  opacity: 0;
  transition: opacity 0.12s ease;
}

.tree-action {
  width: 22px;
  height: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--td-text-color-secondary);
  cursor: pointer;
  padding: 0;

  &:hover {
    background: var(--td-bg-color-secondarycontainer);
    color: var(--td-text-color-primary);
  }
}

.tree-empty {
  padding: 32px 16px;
  text-align: center;
  color: var(--td-text-color-secondary);
}

.tree-empty-title {
  font-size: 13px;
  font-weight: 500;
}

.tree-empty-hint {
  margin-top: 4px;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}
</style>

<style lang="less">
/* Teleported to body: not scoped on purpose. */
.page-tree-menu {
  position: fixed;
  z-index: 3000;
  min-width: 200px;
  padding: 4px;
  border-radius: 8px;
  border: 1px solid var(--td-component-stroke);
  background: var(--td-bg-color-container);
  box-shadow: var(--td-shadow-2);
}

.page-tree-menu-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 7px 10px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--td-text-color-primary);
  font-size: 13px;
  text-align: left;
  cursor: pointer;

  &:hover:not(:disabled) {
    background: var(--td-bg-color-container-hover);
  }

  &:disabled {
    color: var(--td-text-color-disabled);
    cursor: not-allowed;
  }

  &--danger:not(:disabled) {
    color: var(--td-error-color);
  }
}
</style>
