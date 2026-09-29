<template>
  <div
    ref="viewport"
    class="relative min-h-0 flex-1 overflow-x-hidden overflow-y-auto outline-none select-none focus-visible:shadow-[inset_0_0_0_1px_var(--td-brand-color-focus)]"
    role="tree"
    tabindex="0"
    :aria-activedescendant="activeId || undefined"
    @scroll.passive="onScroll"
    @keydown="onKeydown"
    @dragover.prevent
    @drop.prevent="onDropOutside"
  >
    <div v-if="rows.length === 0 && !loading" class="text-muted-foreground px-4 py-8 text-center">
      <div class="text-[13px] font-medium">{{ t("docs.tree.empty") }}</div>
      <div v-if="canEdit" class="text-placeholder mt-1 text-xs">{{ t("docs.tree.emptyHint") }}</div>
    </div>
    <div v-else class="relative w-full" :style="{ height: rows.length * ROW_HEIGHT + 'px' }">
      <div
        v-for="item in visible"
        :key="item.row.node.id"
        class="group absolute top-0 right-0 left-0 box-border flex h-8 cursor-pointer items-center gap-1 rounded-md pr-1.5"
        :class="[
          item.row.node.id === activeId
            ? 'text-primary bg-[var(--td-brand-color-light)]'
            : 'text-foreground hover:bg-accent',
          item.row.node.id === dragId ? 'opacity-40' : '',
          indicator?.id === item.row.node.id && indicator.position === 'inside'
            ? 'shadow-[inset_0_0_0_2px_var(--td-brand-color)]'
            : '',
          indicator?.id === item.row.node.id && indicator.position === 'before'
            ? 'before:absolute before:top-[-1px] before:right-2 before:left-2 before:h-0.5 before:rounded-full before:bg-[var(--td-brand-color)]'
            : '',
          indicator?.id === item.row.node.id && indicator.position === 'after'
            ? 'after:absolute after:right-2 after:bottom-[-1px] after:left-2 after:h-0.5 after:rounded-full after:bg-[var(--td-brand-color)]'
            : '',
        ]"
        role="treeitem"
        :id="'tree-row-' + item.row.node.id"
        :style="{ transform: `translateY(${item.index * ROW_HEIGHT}px)`, paddingLeft: 8 + item.row.depth * 16 + 'px' }"
        :aria-level="item.row.depth + 1"
        :aria-expanded="item.row.node.has_children ? item.row.expanded : undefined"
        :aria-selected="item.row.node.id === activeId"
        :draggable="canEdit && item.row.node.can_edit ? 'true' : 'false'"
        @click="emit('select', item.row.node)"
        @contextmenu.prevent="openMenu($event, item.row.node)"
        @dragstart="onDragStart($event, item.row.node)"
        @dragend="clearDrag"
        @dragover.prevent.stop="onDragOver($event, item.row.node)"
        @dragleave="onDragLeave(item.row.node)"
        @drop.prevent.stop="onDrop(item.row.node)"
      >
        <button
          type="button"
          class="text-muted-foreground hover:bg-secondary flex h-5 w-5 flex-none cursor-pointer items-center justify-center rounded border-0 bg-transparent p-0"
          :class="!item.row.node.has_children ? 'invisible' : ''"
          :aria-label="item.row.expanded ? t('docs.tree.collapse') : t('docs.tree.expand')"
          tabindex="-1"
          @click.stop="emit('toggle', item.row.node)"
        >
          <ChevronRightIcon
            class="size-3.5 transition-transform duration-150"
            :class="item.row.expanded ? 'rotate-90' : ''"
          />
        </button>
        <span class="text-muted-foreground inline-flex w-5 flex-none items-center justify-center" aria-hidden="true">
          <span v-if="item.row.node.icon" class="text-[15px] leading-none">{{ item.row.node.icon }}</span>
          <FileIcon v-else class="size-[15px]" />
        </span>
        <span
          class="min-w-0 flex-1 overflow-hidden text-[13px] leading-5 text-ellipsis whitespace-nowrap"
          :class="!item.row.node.title ? 'text-placeholder' : ''"
        >
          {{ item.row.node.title || t("docs.tree.untitled") }}
        </span>
        <LockIcon
          v-if="item.row.node.restricted"
          class="text-placeholder size-3 flex-none"
          :aria-label="t('docs.pages.restricted')"
        />
        <span
          class="inline-flex flex-none gap-0.5 opacity-0 transition-opacity duration-100 group-hover:opacity-100"
          @click.stop
        >
          <button
            v-if="canEdit && item.row.node.can_edit"
            type="button"
            class="text-muted-foreground hover:bg-secondary hover:text-foreground inline-flex h-[22px] w-[22px] cursor-pointer items-center justify-center rounded border-0 bg-transparent p-0"
            :aria-label="t('docs.tree.newSubpage')"
            tabindex="-1"
            @click="emit('action', 'new-child', item.row.node)"
          >
            <PlusIcon class="size-3.5" />
          </button>
          <button
            type="button"
            class="text-muted-foreground hover:bg-secondary hover:text-foreground inline-flex h-[22px] w-[22px] cursor-pointer items-center justify-center rounded border-0 bg-transparent p-0"
            :aria-label="t('docs.tree.moreActions')"
            tabindex="-1"
            @click="openMenu($event, item.row.node)"
          >
            <MoreHorizontalIcon class="size-3.5" />
          </button>
        </span>
      </div>
    </div>

    <Teleport to="body">
      <div
        v-if="menu"
        class="bg-popover fixed z-3000 min-w-[200px] rounded-[8px] border border-[var(--td-component-stroke)] p-1 shadow-[var(--td-shadow-2)]"
        :style="{ left: menu.x + 'px', top: menu.y + 'px' }"
        role="menu"
        @click.stop
        @contextmenu.prevent
      >
        <button
          v-for="entry in menuEntries"
          :key="entry.action"
          type="button"
          class="enabled:hover:bg-accent flex w-full cursor-pointer items-center gap-2 rounded-md border-0 bg-transparent px-2.5 py-[7px] text-left text-[13px] disabled:cursor-not-allowed"
          :class="
            entry.disabled
              ? 'text-[var(--td-text-color-disabled)]'
              : entry.danger
                ? 'text-destructive'
                : 'text-foreground'
          "
          role="menuitem"
          :disabled="entry.disabled"
          @click="runMenu(entry.action)"
        >
          <component :is="entry.icon" class="size-3.5" />
          <span>{{ entry.label }}</span>
        </button>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch, type Component } from "vue";
import { useI18n } from "vue-i18n";

import {
  ArrowLeftRightIcon,
  ChevronRightIcon,
  CopyIcon,
  FileIcon,
  LinkIcon,
  LockIcon,
  MoreHorizontalIcon,
  PencilIcon,
  PlusIcon,
  Trash2Icon,
} from "@lucide/vue";

import {
  dropPositionFor,
  type DropPosition,
  type MoveTarget,
  type PageTreeModel,
  type TreeNodeData,
  type TreeRow,
} from "./pageTree";

export type TreeAction = "new-child" | "rename" | "duplicate" | "copy-link" | "move-space" | "delete";

const props = defineProps<{
  model: PageTreeModel;
  /** Bumped by the owner after every model mutation. */
  version: number;
  activeId?: string;
  /** Whether the caller may write in this space at all. */
  canEdit: boolean;
  loading?: boolean;
}>();

const emit = defineEmits<{
  select: [node: TreeNodeData];
  toggle: [node: TreeNodeData];
  move: [dragId: string, target: MoveTarget];
  action: [action: TreeAction, node: TreeNodeData];
}>();

const { t } = useI18n();
const ROW_HEIGHT = 32;
const BUFFER = 8;

const viewport = ref<HTMLElement | null>(null);
const scrollTop = ref(0);
const viewportHeight = ref(600);

const rows = computed<TreeRow[]>(() => {
  void props.version;
  return props.model.rows();
});

const visible = computed(() => {
  const start = Math.max(0, Math.floor(scrollTop.value / ROW_HEIGHT) - BUFFER);
  const end = Math.min(rows.value.length, Math.ceil((scrollTop.value + viewportHeight.value) / ROW_HEIGHT) + BUFFER);
  const out: Array<{ row: TreeRow; index: number }> = [];
  for (let i = start; i < end; i++) out.push({ row: rows.value[i], index: i });
  return out;
});

const onScroll = () => {
  scrollTop.value = viewport.value?.scrollTop ?? 0;
};

let resizeObserver: ResizeObserver | null = null;
onMounted(() => {
  if (viewport.value) {
    viewportHeight.value = viewport.value.clientHeight || 600;
    if (typeof ResizeObserver !== "undefined") {
      resizeObserver = new ResizeObserver(() => {
        viewportHeight.value = viewport.value?.clientHeight || 600;
      });
      resizeObserver.observe(viewport.value);
    }
  }
  document.addEventListener("click", closeMenu);
  document.addEventListener("keydown", onDocumentKey);
});
onBeforeUnmount(() => {
  resizeObserver?.disconnect();
  document.removeEventListener("click", closeMenu);
  document.removeEventListener("keydown", onDocumentKey);
});

/** Scroll the active row into view when it changes (e.g. after navigation). */
watch(
  () => [props.activeId, rows.value.length] as const,
  () => {
    if (!props.activeId || !viewport.value) return;
    const index = rows.value.findIndex((r) => r.node.id === props.activeId);
    if (index < 0) return;
    const top = index * ROW_HEIGHT;
    const el = viewport.value;
    if (top < el.scrollTop || top + ROW_HEIGHT > el.scrollTop + el.clientHeight) {
      el.scrollTop = Math.max(0, top - el.clientHeight / 2);
    }
  },
);

// ---- drag and drop -------------------------------------------------------------
const dragId = ref<string | null>(null);
const indicator = ref<{ id: string; position: DropPosition } | null>(null);

const onDragStart = (e: DragEvent, node: TreeNodeData) => {
  if (!props.canEdit || !node.can_edit) {
    e.preventDefault();
    return;
  }
  dragId.value = node.id;
  e.dataTransfer?.setData("text/plain", node.id);
  if (e.dataTransfer) e.dataTransfer.effectAllowed = "move";
};

const onDragOver = (e: DragEvent, node: TreeNodeData) => {
  if (!dragId.value) return;
  const el = e.currentTarget as HTMLElement;
  const offsetY = e.clientY - el.getBoundingClientRect().top;
  const position = dropPositionFor(offsetY, ROW_HEIGHT, node.can_edit);
  const allowed = props.model.dropTarget(dragId.value, node.id, position) !== null;
  if (e.dataTransfer) e.dataTransfer.dropEffect = allowed ? "move" : "none";
  indicator.value = allowed ? { id: node.id, position } : null;
};

const onDragLeave = (node: TreeNodeData) => {
  if (indicator.value?.id === node.id) indicator.value = null;
};

const onDrop = (node: TreeNodeData) => {
  const id = dragId.value;
  const ind = indicator.value;
  clearDrag();
  if (!id || !ind || ind.id !== node.id) return;
  const target = props.model.dropTarget(id, node.id, ind.position);
  if (target) emit("move", id, target);
};

/** Dropping on the empty area below the rows appends to the root. */
const onDropOutside = () => {
  const id = dragId.value;
  clearDrag();
  if (!id) return;
  const node = props.model.get(id);
  if (node && node.parent_id !== null) emit("move", id, { parentId: null, afterId: undefined });
};

const clearDrag = () => {
  dragId.value = null;
  indicator.value = null;
};

// ---- context menu -----------------------------------------------------------------
const menu = ref<{ x: number; y: number; node: TreeNodeData } | null>(null);

const openMenu = (e: MouseEvent, node: TreeNodeData) => {
  const x = Math.min(e.clientX, window.innerWidth - 220);
  const y = Math.min(e.clientY, window.innerHeight - 260);
  menu.value = { x, y, node };
};

const closeMenu = () => {
  menu.value = null;
};

const onDocumentKey = (e: KeyboardEvent) => {
  if (e.key === "Escape") closeMenu();
};

interface MenuEntry {
  action: TreeAction;
  icon: Component;
  label: string;
  disabled: boolean;
  danger?: boolean;
}

const menuEntries = computed<MenuEntry[]>(() => {
  const node = menu.value?.node;
  const editable = !!node && props.canEdit && node.can_edit;
  return [
    { action: "new-child", icon: PlusIcon, label: t("docs.tree.newSubpage"), disabled: !editable },
    { action: "rename", icon: PencilIcon, label: t("docs.tree.rename"), disabled: !editable },
    { action: "duplicate", icon: CopyIcon, label: t("docs.tree.duplicate"), disabled: !props.canEdit },
    { action: "copy-link", icon: LinkIcon, label: t("docs.tree.copyLink"), disabled: false },
    { action: "move-space", icon: ArrowLeftRightIcon, label: t("docs.tree.moveToSpace"), disabled: !editable },
    { action: "delete", icon: Trash2Icon, label: t("docs.tree.delete"), disabled: !editable, danger: true },
  ];
});

const runMenu = (action: TreeAction) => {
  const node = menu.value?.node;
  closeMenu();
  if (node) emit("action", action, node);
};

// ---- keyboard ------------------------------------------------------------------------
const onKeydown = (e: KeyboardEvent) => {
  if (!rows.value.length) return;
  const index = rows.value.findIndex((r) => r.node.id === props.activeId);
  const current = index >= 0 ? rows.value[index] : null;
  switch (e.key) {
    case "ArrowDown": {
      e.preventDefault();
      const next = rows.value[Math.min(rows.value.length - 1, index + 1)];
      if (next) emit("select", next.node);
      break;
    }
    case "ArrowUp": {
      e.preventDefault();
      const prev = rows.value[Math.max(0, index - 1)];
      if (prev) emit("select", prev.node);
      break;
    }
    case "ArrowRight":
      if (current?.node.has_children && !current.expanded) {
        e.preventDefault();
        emit("toggle", current.node);
      }
      break;
    case "ArrowLeft":
      if (current?.expanded) {
        e.preventDefault();
        emit("toggle", current.node);
      } else if (current?.node.parent_id) {
        e.preventDefault();
        const parent = props.model.get(current.node.parent_id);
        if (parent) emit("select", parent);
      }
      break;
    default:
      break;
  }
};
</script>
