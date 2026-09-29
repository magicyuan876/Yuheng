<template>
  <div class="max-w-[280px] min-w-[208px]">
    <div
      v-if="showBack"
      class="text-muted-foreground hover:text-primary mb-0.5 flex cursor-pointer items-center gap-1.5 border-b border-[var(--td-component-stroke)] px-2.5 py-1.5 text-[13px]"
      @click.stop="emit('back')"
    >
      <ChevronLeftIcon class="size-4" />
      <span>{{ t("knowledgeBase.moveToFolder.action") }}</span>
    </div>

    <div
      ref="listRef"
      class="max-h-[260px] [scrollbar-width:thin] overflow-y-auto [&::-webkit-scrollbar]:w-1 [&::-webkit-scrollbar-thumb]:rounded-sm [&::-webkit-scrollbar-thumb]:bg-[var(--td-scrollbar-color)]"
    >
      <!--
        Rows indent 12px per depth after a 10px base. The "new folder" button
        stays invisible (and unclickable) until its row is hovered; the current
        folder is greyed and does not highlight, but still offers the button.
      -->
      <template v-for="row in renderRows" :key="row.key">
        <div
          v-if="row.kind === 'folder'"
          :data-folder-path="row.path || undefined"
          :data-current="effectiveCurrentPath === row.path || undefined"
          class="group box-border flex h-[30px] items-center gap-1.5 rounded-md pr-2 text-[13px] transition-colors"
          :class="
            effectiveCurrentPath === row.path
              ? 'text-placeholder cursor-default'
              : 'text-foreground hover:bg-accent cursor-pointer'
          "
          :style="{ paddingLeft: `${row.depth * 12 + 10}px` }"
          :title="row.path || undefined"
          @click.stop="choose(row.path)"
        >
          <FolderOpenIcon v-if="row.isRoot" class="text-placeholder size-[15px] shrink-0" />
          <FolderIcon v-else class="text-placeholder size-[15px] shrink-0" />
          <span class="min-w-0 flex-1 truncate">{{ row.label }}</span>
          <button
            type="button"
            data-slot="folder-add"
            class="text-placeholder hover:text-primary pointer-events-none -mr-0.5 inline-flex size-[22px] shrink-0 items-center justify-center rounded opacity-0 transition-all group-hover:pointer-events-auto group-hover:opacity-100 hover:bg-[var(--td-bg-color-component)]"
            :title="
              row.isRoot
                ? t('knowledgeBase.moveToFolder.newFolderAddRoot')
                : t('knowledgeBase.moveToFolder.newFolderAddUnder', { folder: row.label })
            "
            :aria-label="
              row.isRoot
                ? t('knowledgeBase.moveToFolder.newFolderAddRoot')
                : t('knowledgeBase.moveToFolder.newFolderAddUnder', { folder: row.label })
            "
            @click.stop="startCreatingUnder(row.path)"
          >
            <FolderPlusIcon class="size-4" />
          </button>
          <CheckIcon v-if="effectiveCurrentPath === row.path" class="text-placeholder size-3.5 shrink-0" />
        </div>

        <div
          v-else
          data-folder-create
          class="box-border flex h-[30px] cursor-default items-center gap-1.5 rounded-md pr-2 text-[13px]"
          :style="{ paddingLeft: `${row.depth * 12 + 10}px` }"
          @click.stop
        >
          <FolderIcon class="text-placeholder size-[15px] shrink-0" />
          <input
            ref="newFolderInputRef"
            v-model.trim="newFolderName"
            data-slot="folder-name"
            class="border-primary bg-card text-foreground h-6 min-w-0 flex-1 rounded border px-1.5 [font-family:var(--app-font-family)] text-[13px] outline-none"
            :placeholder="t('knowledgeBase.moveToFolder.newFolderPlaceholder')"
            @keydown.enter.stop="commitNewFolder"
            @keydown.esc.stop="cancelCreating"
          />
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import { CheckIcon, ChevronLeftIcon, FolderIcon, FolderOpenIcon, FolderPlusIcon } from "@lucide/vue";
import { folderOptionFromPath, joinFolderPath, normalizeFolderPath, sortFolderOptions } from "../folderTree";

export type FolderOption = { path: string; name: string; depth: number };

type FolderRow = {
  kind: "folder";
  key: string;
  path: string;
  label: string;
  depth: number;
  isRoot?: boolean;
};

type CreateRow = {
  kind: "create";
  key: string;
  parentPath: string;
  depth: number;
};

type RenderRow = FolderRow | CreateRow;

const props = withDefaults(
  defineProps<{
    options: FolderOption[];
    currentPath?: string;
    showBack?: boolean;
    allowReselect?: boolean;
  }>(),
  {
    currentPath: "",
    showBack: false,
    allowReselect: false,
  },
);

const emit = defineEmits<{
  back: [];
  confirm: [folderPath: string];
  create: [folderPath: string];
}>();

const { t } = useI18n();

const creatingUnder = ref<string | null>(null);
const newFolderName = ref("");
// The input sits inside a v-for, where Vue collects a template ref into an
// array; there is only ever one create row, so the first element is the one.
const newFolderInputRef = ref<HTMLInputElement | HTMLInputElement[] | null>(null);
const listRef = ref<HTMLElement | null>(null);
const localCreatedPaths = ref<string[]>([]);
const selectedPath = ref<string | null>(null);

const effectiveCurrentPath = computed(() =>
  selectedPath.value !== null ? selectedPath.value : (props.currentPath ?? ""),
);

const displayOptions = computed(() => {
  const byPath = new Map<string, FolderOption>();
  props.options.forEach((option) => byPath.set(option.path, option));
  localCreatedPaths.value.forEach((path) => {
    if (!byPath.has(path)) byPath.set(path, folderOptionFromPath(path));
  });
  return sortFolderOptions([...byPath.values()]);
});

const renderRows = computed<RenderRow[]>(() => {
  const rows: RenderRow[] = [
    {
      kind: "folder",
      key: "root",
      path: "",
      label: t("knowledgeBase.folderTree.rootRow"),
      depth: 0,
      isRoot: true,
    },
  ];

  if (creatingUnder.value === "") {
    rows.push({
      kind: "create",
      key: "create-root",
      parentPath: "",
      depth: childCreateDepth(""),
    });
  }

  displayOptions.value.forEach((option) => {
    rows.push({
      kind: "folder",
      key: option.path,
      path: option.path,
      label: option.name,
      depth: option.depth,
    });
    if (creatingUnder.value === option.path) {
      rows.push({
        kind: "create",
        key: `create-${option.path}`,
        parentPath: option.path,
        depth: childCreateDepth(option.path),
      });
    }
  });

  return rows;
});

watch(creatingUnder, async (value) => {
  if (value === null) return;
  await nextTick();
  const input = Array.isArray(newFolderInputRef.value) ? newFolderInputRef.value[0] : newFolderInputRef.value;
  input?.focus();
});

watch(
  () => props.options,
  (options) => {
    const existing = new Set(options.map((option) => option.path));
    localCreatedPaths.value = localCreatedPaths.value.filter((path) => !existing.has(path));
  },
  { deep: true },
);

watch(
  () => props.currentPath,
  () => {
    selectedPath.value = null;
  },
);

function childCreateDepth(parentPath: string): number {
  return parentPath.split("/").filter(Boolean).length;
}

const choose = (path: string) => {
  if (path === effectiveCurrentPath.value && !props.allowReselect) return;
  selectedPath.value = path;
  emit("confirm", path);
};

const startCreatingUnder = (parentPath: string) => {
  if (creatingUnder.value === parentPath) {
    cancelCreating();
    return;
  }
  creatingUnder.value = parentPath;
  newFolderName.value = "";
};

const cancelCreating = () => {
  creatingUnder.value = null;
  newFolderName.value = "";
};

const scrollToFolder = async (path: string) => {
  await nextTick();
  const row = listRef.value?.querySelector(`[data-folder-path="${CSS.escape(path)}"]`);
  row?.scrollIntoView({ block: "nearest" });
};

const commitNewFolder = async () => {
  if (creatingUnder.value === null) return;
  const name = normalizeFolderPath(newFolderName.value);
  if (!name) return;
  const path = joinFolderPath(creatingUnder.value, name);
  if (displayOptions.value.some((option) => option.path === path)) {
    MessagePlugin.warning(t("knowledgeBase.moveToFolder.duplicate"));
    return;
  }

  if (!localCreatedPaths.value.includes(path)) {
    localCreatedPaths.value = [...localCreatedPaths.value, path];
  }
  creatingUnder.value = null;
  newFolderName.value = "";
  emit("create", path);
  await scrollToFolder(path);
};
</script>
