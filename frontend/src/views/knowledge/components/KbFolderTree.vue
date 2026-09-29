<template>
  <aside
    class="box-border flex min-h-0 shrink-0 flex-col border-r border-[var(--td-component-stroke)]"
    :class="collapsed ? 'mr-2 w-auto pr-2' : 'mr-3 w-[268px] pr-3'"
  >
    <div class="flex h-8 shrink-0 items-center justify-between gap-1.5">
      <template v-if="!collapsed">
        <span class="text-foreground truncate text-[13px] font-semibold">{{
          t("knowledgeBase.folderTree.title")
        }}</span>
        <Tooltip>
          <TooltipTrigger as-child>
            <button
              type="button"
              data-slot="icon-button"
              class="text-muted-foreground hover:text-primary hover:bg-accent inline-flex size-6 items-center justify-center rounded-md transition-colors"
              :aria-label="t('knowledgeBase.folderTree.collapse')"
              @click="emit('update:collapsed', true)"
            >
              <ChevronsLeftIcon class="size-[15px]" />
            </button>
          </TooltipTrigger>
          <TooltipContent side="top">{{ t("knowledgeBase.folderTree.collapse") }}</TooltipContent>
        </Tooltip>
      </template>
      <Tooltip v-else>
        <TooltipTrigger as-child>
          <button
            type="button"
            data-slot="icon-button"
            class="text-muted-foreground hover:text-primary hover:bg-accent inline-flex size-6 items-center justify-center rounded-md transition-colors"
            :aria-label="t('knowledgeBase.folderTree.expand')"
            @click="emit('update:collapsed', false)"
          >
            <ChevronsRightIcon class="size-[15px]" />
          </button>
        </TooltipTrigger>
        <TooltipContent side="right">{{ t("knowledgeBase.folderTree.expand") }}</TooltipContent>
      </Tooltip>
    </div>

    <div
      v-if="!collapsed"
      class="min-h-0 flex-1 [scrollbar-width:thin] overflow-x-hidden overflow-y-auto pt-1 pb-3 [&::-webkit-scrollbar]:w-1 [&::-webkit-scrollbar-thumb]:rounded-sm [&::-webkit-scrollbar-thumb]:bg-[var(--td-scrollbar-color)]"
    >
      <template v-if="loading && !tree">
        <div v-for="n in 5" :key="'folder-skel-' + n" class="px-2 py-[7px]">
          <Skeleton class="h-4 w-full" />
        </div>
      </template>
      <template v-else>
        <!--
          Each depth indents the row by 10px. The count gives way to the "more"
          button while an editable row is hovered or its menu is open; `group`
          carries the hover to those children.
        -->
        <div
          v-for="row in rows"
          :key="row.path || '__root__'"
          class="group text-foreground hover:bg-accent box-border flex h-[30px] w-full cursor-pointer items-center gap-1 rounded-md pr-2 text-left [font-family:var(--app-font-family)] text-[13px] transition-colors select-none focus-visible:shadow-[0_0_0_2px_color-mix(in_srgb,var(--td-brand-color)_30%,transparent)] focus-visible:outline-none"
          :class="{ 'bg-accent': selectedPath === row.path }"
          :style="{ paddingLeft: `${row.depth * 10}px` }"
          :title="row.kind === 'root' ? t('knowledgeBase.folderTree.rootRowTip') : row.path"
          role="button"
          tabindex="0"
          @click="emit('select', row.path)"
          @keydown.enter="emit('select', row.path)"
        >
          <span
            v-if="row.hasChildren"
            class="text-placeholder hover:text-foreground hover:bg-muted inline-flex size-4 shrink-0 cursor-pointer items-center justify-center rounded"
            role="button"
            :aria-label="
              t(
                isExpanded(row.path)
                  ? 'knowledgeBase.folderTree.collapseFolder'
                  : 'knowledgeBase.folderTree.expandFolder',
              )
            "
            @click.stop="toggle(row.path)"
          >
            <ChevronDownIcon v-if="isExpanded(row.path)" class="size-3.5" />
            <ChevronRightIcon v-else class="size-3.5" />
          </span>
          <span v-else class="inline-flex size-4 shrink-0" aria-hidden="true" />

          <component
            :is="row.kind === 'root' || (row.hasChildren && isExpanded(row.path)) ? FolderOpenIcon : FolderIcon"
            class="size-[15px] shrink-0"
            :class="selectedPath === row.path ? 'text-primary' : 'text-placeholder'"
          />

          <input
            v-if="isRenaming(row)"
            ref="renameInputRef"
            v-model="renameValue"
            data-slot="folder-rename"
            class="border-primary bg-card text-foreground h-[22px] min-w-0 flex-1 rounded border px-1.5 [font-family:var(--app-font-family)] text-[13px] outline-none"
            :placeholder="t('knowledgeBase.folderTree.renamePlaceholder')"
            @click.stop
            @keydown.enter="commitRename(row)"
            @keydown.esc="cancelRename"
            @blur="commitRename(row)"
          />
          <template v-else>
            <span
              class="min-w-0 flex-1 truncate"
              :class="{ 'text-primary': selectedPath === row.path, 'font-medium': row.kind === 'root' }"
            >
              {{ row.kind === "root" ? t("knowledgeBase.folderTree.rootRow") : row.name }}
            </span>
            <span class="ml-0.5 flex h-5 w-[22px] shrink-0 items-center justify-center">
              <span
                class="text-placeholder text-[11px] leading-none tabular-nums"
                :class="{
                  'group-hover:hidden': canEdit && row.kind === 'folder',
                  hidden: menuOpenPath === row.path,
                }"
                >{{ row.totalCount }}</span
              >
              <Popover
                v-if="canEdit && row.kind === 'folder'"
                :open="menuOpenPath === row.path"
                @update:open="(visible: boolean) => onFolderMenuVisible(row.path, visible)"
              >
                <PopoverTrigger as-child>
                  <button
                    type="button"
                    data-slot="folder-more"
                    class="hover:text-primary hover:bg-muted size-5 items-center justify-center rounded transition-colors"
                    :class="
                      menuOpenPath === row.path
                        ? 'text-primary inline-flex'
                        : 'text-placeholder hidden group-hover:inline-flex'
                    "
                    :aria-label="t('docs.tree.moreActions')"
                    @click.stop
                  >
                    <EllipsisIcon class="size-3.5" />
                  </button>
                </PopoverTrigger>
                <PopoverContent align="end" class="w-auto min-w-[148px] gap-px rounded-[10px] p-1">
                  <div class="flex min-w-[140px] flex-col gap-px" @click.stop>
                    <div
                      data-menu-item="rename"
                      class="group/item text-foreground hover:bg-accent flex cursor-pointer items-center gap-2.5 rounded-md px-3 py-2 text-sm leading-5 transition-all active:scale-[0.98] active:bg-[var(--td-bg-color-container-active)]"
                      @click="onFolderMenuRename(row)"
                    >
                      <PenLineIcon class="text-muted-foreground group-hover/item:text-foreground size-4 shrink-0" />
                      <span>{{ t("knowledgeBase.folderTree.rename") }}</span>
                    </div>
                  </div>
                </PopoverContent>
              </Popover>
            </span>
          </template>
        </div>
      </template>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import {
  ChevronDownIcon,
  ChevronRightIcon,
  ChevronsLeftIcon,
  ChevronsRightIcon,
  EllipsisIcon,
  FolderIcon,
  FolderOpenIcon,
  PenLineIcon,
} from "@lucide/vue";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { KnowledgeFolderTree } from "@/api/knowledge-base/index";
import { buildFolderRows, folderAncestorPaths, joinFolderPath, ROOT_FOLDER_PATH, type FolderRow } from "../folderTree";

const props = withDefaults(
  defineProps<{
    tree: KnowledgeFolderTree | null;
    /** Selected folder path; the empty string is the knowledge base top level. */
    selectedPath: string;
    loading?: boolean;
    collapsed?: boolean;
    canEdit?: boolean;
  }>(),
  {
    loading: false,
    collapsed: false,
    canEdit: false,
  },
);

const emit = defineEmits<{
  select: [path: string];
  "update:collapsed": [collapsed: boolean];
  rename: [payload: { from: string; to: string }];
}>();

const { t } = useI18n();

// The root starts expanded so the uploaded structure is visible without a click.
const expanded = ref(new Set<string>([ROOT_FOLDER_PATH]));
// null, not '', because '' is the root's own path: a falsy sentinel would put
// the root row into rename mode permanently.
const renamingPath = ref<string | null>(null);
const menuOpenPath = ref<string | null>(null);
const renameValue = ref("");
const renameInputRef = ref<HTMLInputElement | HTMLInputElement[] | null>(null);

const rows = computed(() => buildFolderRows(props.tree, expanded.value));

const isExpanded = (path: string) => expanded.value.has(path);

// The root has no name of its own to edit, and excluding it here means no
// sentinel value can ever put it into rename mode.
const isRenaming = (row: FolderRow) => row.kind === "folder" && renamingPath.value === row.path;

const toggle = (path: string) => {
  const next = new Set(expanded.value);
  if (next.has(path)) next.delete(path);
  else next.add(path);
  expanded.value = next;
};

const startRename = async (row: FolderRow) => {
  renamingPath.value = row.path;
  renameValue.value = row.name;
  await nextTick();
  const input = Array.isArray(renameInputRef.value) ? renameInputRef.value[0] : renameInputRef.value;
  input?.focus();
  input?.select();
};

const onFolderMenuVisible = (path: string, visible: boolean) => {
  menuOpenPath.value = visible ? path : null;
};

const onFolderMenuRename = async (row: FolderRow) => {
  menuOpenPath.value = null;
  await startRename(row);
};

const cancelRename = () => {
  renamingPath.value = null;
  renameValue.value = "";
};

const commitRename = (row: FolderRow) => {
  if (!isRenaming(row)) return;
  const name = renameValue.value.trim();
  cancelRename();
  // Only the last segment is edited here; the folder keeps its place in the tree.
  if (!name || name === row.name) return;
  const parent = row.path.slice(0, Math.max(0, row.path.length - row.name.length - 1));
  emit("rename", { from: row.path, to: joinFolderPath(parent, name) });
};

// Keep the selected folder reachable: expand the root and every folder above
// the active path, both on first load and when the selection changes from
// elsewhere (e.g. opening a folder from the document list).
watch(
  () => [props.selectedPath, props.tree] as const,
  () => {
    const next = new Set(expanded.value);
    folderAncestorPaths(props.selectedPath).forEach((path) => next.add(path));
    expanded.value = next;
  },
  { immediate: true },
);

// First load also opens the top-level folders, so a two-level upload is visible
// in full without any expanding.
watch(
  () => props.tree,
  (tree) => {
    if (!tree?.folders?.length || expanded.value.size > 1) return;
    const next = new Set(expanded.value);
    tree.folders.forEach((folder) => next.add(folder.path));
    expanded.value = next;
  },
  { immediate: true },
);
</script>
