<template>
  <SettingDrawer
    v-model:visible="drawerVisible"
    :title="$t('knowledgeBase.tagManageTitle')"
    :description="$t('knowledgeBase.tagManageDescription')"
    :icon="TagsIcon"
    width="480px"
    :min-width="420"
    :max-width="640"
    :resizable="true"
    storage-key="setting-drawer:width:kb-tag-manage"
    :hide-footer="true"
  >
    <section
      class="flex flex-col gap-3.5 border-b border-[var(--td-component-stroke)] pt-3 pb-4 first:pt-0 last:border-b-0 last:pb-0"
    >
      <h4
        class="text-foreground before:bg-primary m-0 mb-1 flex items-center gap-2 text-[13px] font-semibold select-none before:h-3.5 before:w-[3px] before:shrink-0 before:rounded-sm before:content-['']"
      >
        {{ $t("knowledgeBase.tagManageListSection") }}
      </h4>

      <div class="flex items-center gap-1.5">
        <div class="relative min-w-0 flex-1">
          <SearchIcon
            class="text-placeholder pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2"
            aria-hidden="true"
          />
          <!-- A 24px filled field that turns into a bordered one on hover or focus, as the TDesign small input did. -->
          <Input
            v-model.trim="searchQuery"
            :placeholder="$t('knowledgeBase.tagSearchPlaceholder')"
            class="bg-muted dark:bg-muted hover:bg-card focus-visible:bg-card hover:border-border focus-visible:border-border placeholder:text-placeholder h-6 rounded-md border-transparent pr-7 pl-7 text-[13px] focus-visible:ring-0 md:text-[13px]"
          />
          <button
            v-if="searchQuery"
            type="button"
            data-slot="input-clear"
            class="text-placeholder hover:text-muted-foreground absolute top-1/2 right-1.5 -translate-y-1/2"
            :aria-label="$t('common.clear')"
            @click="searchQuery = ''"
          >
            <CircleXIcon class="size-3.5" aria-hidden="true" />
          </button>
        </div>
        <Tooltip>
          <TooltipTrigger as-child>
            <Button
              variant="ghost"
              size="icon"
              class="text-muted-foreground hover:text-foreground hover:bg-muted rounded-md disabled:opacity-45"
              :disabled="creatingTag"
              :aria-label="$t('knowledgeBase.tagCreateAction')"
              @click="startCreateTag"
            >
              <PlusIcon class="size-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="top">{{ $t("knowledgeBase.tagCreateAction") }}</TooltipContent>
        </Tooltip>
      </div>

      <!-- The list stays in place under the spinner, as it did under t-loading. -->
      <div class="relative min-h-[80px]" :aria-busy="loading && !tags.length">
        <div v-if="!loading && !tags.length && !creatingTag" class="py-6">
          <Empty>
            <EmptyDescription>{{ $t("knowledgeBase.tagEmptyResult") }}</EmptyDescription>
          </Empty>
        </div>

        <ul v-else class="m-0 grid list-none grid-cols-2 gap-1.5 p-0">
          <template v-if="loading && !tags.length">
            <li v-for="n in 4" :key="'tag-skel-' + n">
              <Skeleton class="h-11 w-full" />
            </li>
          </template>

          <template v-else>
            <li
              v-if="creatingTag"
              class="bg-muted relative box-border flex min-h-11 items-center justify-between gap-1 rounded-md border border-[var(--td-component-border)] py-[5px] pr-1.5 pl-2"
              @click.stop
            >
              <div class="flex min-w-0 flex-1 items-center gap-1.5">
                <span
                  class="bg-muted text-placeholder inline-flex size-6 shrink-0 items-center justify-center rounded-md"
                  aria-hidden="true"
                >
                  <TagsIcon class="size-[15px]" />
                </span>
                <Input
                  ref="newTagInputRef"
                  v-model="newTagName"
                  :maxlength="40"
                  class="h-6 min-w-0 flex-1 rounded-none border-transparent bg-transparent px-0 text-[13px] font-medium shadow-none focus-visible:border-transparent focus-visible:ring-0 md:text-[13px] dark:bg-transparent"
                  :placeholder="$t('knowledgeBase.tagNamePlaceholder')"
                  @keydown.enter="submitCreateTag"
                  @keydown="(e: KeyboardEvent) => onEditKeydown(e, cancelCreateTag)"
                />
              </div>
              <div class="flex shrink-0 items-center">
                <Button
                  variant="ghost"
                  size="icon-xs"
                  class="text-muted-foreground hover:text-foreground focus-visible:text-foreground hover:bg-card dark:hover:bg-card"
                  :disabled="creatingTagLoading"
                  :title="$t('common.create')"
                  :aria-label="$t('common.create')"
                  @click.stop="submitCreateTag"
                >
                  <Loader2Icon v-if="creatingTagLoading" class="size-3.5 animate-spin" />
                  <CheckIcon v-else class="size-3.5" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-xs"
                  class="text-muted-foreground"
                  :title="$t('common.cancel')"
                  :aria-label="$t('common.cancel')"
                  @click.stop="cancelCreateTag"
                >
                  <XIcon class="size-3.5" />
                </Button>
              </div>
            </li>

            <!--
              A tile's actions appear on hover or keyboard focus, and always
              on touch screens, which have no hover to reveal them.
            -->
            <li
              v-for="tag in tags"
              :key="tag.id"
              class="group relative box-border flex min-h-11 items-center justify-between gap-1 rounded-md border py-[5px] pr-1.5 pl-2 transition-colors"
              :class="
                editingTagId === tag.id
                  ? 'bg-muted border-[var(--td-component-border)]'
                  : 'bg-card border-[var(--td-component-stroke)] hover:border-[var(--td-component-border)] hover:bg-[color-mix(in_srgb,var(--td-bg-color-secondarycontainer)_40%,var(--td-bg-color-container))]'
              "
              @click.stop
            >
              <template v-if="editingTagId === tag.id">
                <div class="flex min-w-0 flex-1 items-center gap-1.5">
                  <span
                    class="bg-muted text-placeholder inline-flex size-6 shrink-0 items-center justify-center rounded-md"
                    aria-hidden="true"
                  >
                    <TagsIcon class="size-[15px]" />
                  </span>
                  <Input
                    :ref="(el) => setEditingTagInputRef(el as ComponentPublicInstance | null, tag.id)"
                    v-model="editingTagName"
                    :maxlength="40"
                    class="h-6 min-w-0 flex-1 rounded-none border-transparent bg-transparent px-0 text-[13px] font-medium shadow-none focus-visible:border-transparent focus-visible:ring-0 md:text-[13px] dark:bg-transparent"
                    :placeholder="$t('knowledgeBase.tagNamePlaceholder')"
                    @keydown.enter="submitEditTag"
                    @keydown="(e: KeyboardEvent) => onEditKeydown(e, cancelEditTag)"
                  />
                </div>
                <div class="flex shrink-0 items-center">
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    class="text-muted-foreground hover:text-foreground focus-visible:text-foreground hover:bg-card dark:hover:bg-card"
                    :disabled="editingTagSubmitting"
                    :title="$t('common.save')"
                    :aria-label="$t('common.save')"
                    @click.stop="submitEditTag"
                  >
                    <Loader2Icon v-if="editingTagSubmitting" class="size-3.5 animate-spin" />
                    <CheckIcon v-else class="size-3.5" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    class="text-muted-foreground"
                    :title="$t('common.cancel')"
                    :aria-label="$t('common.cancel')"
                    @click.stop="cancelEditTag"
                  >
                    <XIcon class="size-3.5" />
                  </Button>
                </div>
              </template>
              <template v-else>
                <div class="flex min-w-0 flex-1 items-center gap-1.5">
                  <span
                    class="bg-muted text-placeholder inline-flex size-6 shrink-0 items-center justify-center rounded-md"
                    aria-hidden="true"
                  >
                    <TagsIcon class="size-[15px]" />
                  </span>
                  <span class="flex min-w-0 flex-1 flex-col gap-px">
                    <span class="text-foreground truncate text-[13px] leading-[1.3] font-medium" :title="tag.name">{{
                      tag.name
                    }}</span>
                    <span class="text-placeholder truncate text-[11px] leading-[1.3]">
                      {{
                        isFaq
                          ? $t("knowledgeBase.tagManageFaqCount", { count: tag.chunk_count || 0 })
                          : $t("knowledgeBase.tagManageDocCount", { count: tag.knowledge_count || 0 })
                      }}
                    </span>
                  </span>
                </div>
                <div
                  class="flex shrink-0 items-center opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100 [@media(hover:none)]:opacity-100"
                  @click.stop
                >
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    class="text-muted-foreground hover:text-foreground"
                    :title="$t('knowledgeBase.tagEditAction')"
                    :aria-label="$t('knowledgeBase.tagEditAction')"
                    @click="startEditTag(tag)"
                  >
                    <PenLineIcon class="size-3.5" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    class="text-destructive hover:text-destructive hover:bg-[var(--td-error-color-1)] dark:hover:bg-[var(--td-error-color-1)]"
                    :title="$t('knowledgeBase.tagDeleteAction')"
                    :aria-label="$t('knowledgeBase.tagDeleteAction')"
                    @click.stop="deleteTarget = tag"
                  >
                    <Trash2Icon class="size-3.5" />
                  </Button>
                </div>
              </template>
            </li>
          </template>
        </ul>

        <div v-if="loading && !tags.length" class="absolute inset-0 flex items-center justify-center">
          <Loader2Icon class="text-primary size-5 animate-spin" />
        </div>

        <div v-if="hasMore && tags.length" class="flex justify-center pt-2">
          <Button
            variant="ghost"
            size="xs"
            class="text-placeholder font-normal"
            :disabled="loadingMore"
            @click="loadTags(false)"
          >
            <Loader2Icon v-if="loadingMore" class="animate-spin" />
            {{ $t("tenant.loadMore") }}
          </Button>
        </div>
      </div>
    </section>

    <Dialog :open="deleteTarget !== null" @update:open="(v: boolean) => !v && (deleteTarget = null)">
      <DialogContent class="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>{{ deleteTarget ? getDeleteConfirmContent(deleteTarget) : "" }}</DialogTitle>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="deleteTarget = null">{{ $t("common.cancel") }}</Button>
          <Button
            variant="destructive"
            @click="
              if (deleteTarget) deleteTag(deleteTarget);
              deleteTarget = null;
            "
          >
            {{ $t("common.delete") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </SettingDrawer>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, computed, type ComponentPublicInstance } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import {
  CheckIcon,
  CircleXIcon,
  Loader2Icon,
  PenLineIcon,
  PlusIcon,
  SearchIcon,
  TagsIcon,
  Trash2Icon,
  XIcon,
} from "@lucide/vue";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Empty, EmptyDescription } from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  listKnowledgeTags,
  createKnowledgeBaseTag,
  updateKnowledgeBaseTag,
  deleteKnowledgeBaseTag,
} from "@/api/knowledge-base/index";

type TagRow = {
  id: string;
  seq_id: number;
  name: string;
  knowledge_count?: number;
  chunk_count?: number;
};

// The Input component exposes nothing of its own; its root element is the
// <input>, which is what gets focused and selected.
type TagInputInstance = ComponentPublicInstance;

const focusAndSelect = (instance: TagInputInstance | null | undefined) => {
  const el = instance?.$el;
  if (el instanceof HTMLInputElement) {
    el.focus();
    el.select();
  }
};

const TAG_PAGE_SIZE = 50;

const props = defineProps<{
  visible: boolean;
  kbId: string;
  isFaq?: boolean;
}>();

const emit = defineEmits<{
  "update:visible": [boolean];
  changed: [payload?: { deletedTagId?: string }];
}>();

const { t } = useI18n();

const drawerVisible = computed({
  get: () => props.visible,
  set: (value: boolean) => emit("update:visible", value),
});

const tags = ref<TagRow[]>([]);
const loading = ref(false);
const loadingMore = ref(false);
const page = ref(1);
const hasMore = ref(false);
const total = ref(0);
const searchQuery = ref("");
let searchDebounce: ReturnType<typeof setTimeout> | null = null;

const creatingTag = ref(false);
const creatingTagLoading = ref(false);
const newTagName = ref("");
const newTagInputRef = ref<TagInputInstance | null>(null);

const editingTagId = ref<string | null>(null);
const editingTagName = ref("");
const editingTagSubmitting = ref(false);
const editingTagInputRefs = new Map<string, TagInputInstance | null>();
const deleteTarget = ref<TagRow | null>(null);

const setEditingTagInputRef = (el: TagInputInstance | null, tagId: string) => {
  if (el) {
    editingTagInputRefs.set(tagId, el);
  } else {
    editingTagInputRefs.delete(tagId);
  }
};

const getDeleteConfirmContent = (tag: { name: string }) =>
  t(props.isFaq ? "knowledgeBase.tagDeleteDesc" : "knowledgeBase.tagDeleteDescDoc", { name: tag.name });

const onEditKeydown = (e: KeyboardEvent, cancel: () => void) => {
  if (e.key === "Escape") {
    e.stopPropagation();
    e.preventDefault();
    cancel();
  }
};

const resetLocalState = () => {
  cancelCreateTag();
  cancelEditTag();
  searchQuery.value = "";
};

const loadTags = async (reset = false) => {
  if (!props.kbId) {
    tags.value = [];
    total.value = 0;
    hasMore.value = false;
    page.value = 1;
    return;
  }
  if (reset) {
    page.value = 1;
    tags.value = [];
    total.value = 0;
    hasMore.value = false;
  } else if (loading.value || loadingMore.value) {
    return;
  }

  const currentPage = page.value || 1;
  loading.value = currentPage === 1;
  loadingMore.value = currentPage > 1;

  try {
    const res: any = await listKnowledgeTags(props.kbId, {
      page: currentPage,
      page_size: TAG_PAGE_SIZE,
      keyword: searchQuery.value || undefined,
    });
    const pageData = (res?.data || {}) as { data?: TagRow[]; total?: number };
    const pageTags = (pageData.data || []).map((tag) => ({
      ...tag,
      id: String(tag.id),
    }));

    if (currentPage === 1) {
      tags.value = pageTags;
    } else {
      tags.value = [...tags.value, ...pageTags];
    }

    total.value = pageData.total || tags.value.length;
    hasMore.value = tags.value.length < total.value;
    if (hasMore.value) {
      page.value = currentPage + 1;
    }
  } catch (error) {
    console.error("Failed to load tags", error);
  } finally {
    loading.value = false;
    loadingMore.value = false;
  }
};

const startCreateTag = () => {
  if (!props.kbId || creatingTag.value) return;
  cancelEditTag();
  creatingTag.value = true;
  nextTick(() => focusAndSelect(newTagInputRef.value));
};

const cancelCreateTag = () => {
  creatingTag.value = false;
  newTagName.value = "";
};

const submitCreateTag = async () => {
  if (!props.kbId) return;
  const name = newTagName.value.trim();
  if (!name) {
    MessagePlugin.warning(t("knowledgeBase.tagNameRequired"));
    return;
  }
  creatingTagLoading.value = true;
  try {
    await createKnowledgeBaseTag(props.kbId, { name });
    MessagePlugin.success(t("knowledgeBase.tagCreateSuccess"));
    cancelCreateTag();
    await loadTags(true);
    emit("changed");
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.operationFailed"));
  } finally {
    creatingTagLoading.value = false;
  }
};

const startEditTag = (tag: TagRow) => {
  cancelCreateTag();
  editingTagId.value = tag.id;
  editingTagName.value = tag.name;
  nextTick(() => focusAndSelect(editingTagInputRefs.get(tag.id)));
};

const cancelEditTag = () => {
  editingTagId.value = null;
  editingTagName.value = "";
};

const submitEditTag = async () => {
  if (!props.kbId || !editingTagId.value) return;
  const name = editingTagName.value.trim();
  if (!name) {
    MessagePlugin.warning(t("knowledgeBase.tagNameRequired"));
    return;
  }
  const current = tags.value.find((tag) => tag.id === editingTagId.value);
  if (current && name === current.name) {
    cancelEditTag();
    return;
  }
  editingTagSubmitting.value = true;
  try {
    await updateKnowledgeBaseTag(props.kbId, editingTagId.value, { name });
    MessagePlugin.success(t("knowledgeBase.tagEditSuccess"));
    cancelEditTag();
    await loadTags(true);
    emit("changed");
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.operationFailed"));
  } finally {
    editingTagSubmitting.value = false;
  }
};

const deleteTag = async (tag: TagRow) => {
  if (!props.kbId) return;
  cancelCreateTag();
  cancelEditTag();
  try {
    await deleteKnowledgeBaseTag(props.kbId, tag.seq_id, { force: true });
    MessagePlugin.success(t("knowledgeBase.tagDeleteSuccess"));
    await loadTags(true);
    emit("changed", { deletedTagId: tag.id });
    void (async () => {
      await new Promise((resolve) => setTimeout(resolve, 800));
      emit("changed", { deletedTagId: tag.id });
    })();
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.operationFailed"));
  }
};

watch(
  () => props.visible,
  (open) => {
    if (open && props.kbId) {
      void loadTags(true);
    } else if (!open) {
      resetLocalState();
    }
  },
);

watch(searchQuery, (newVal, oldVal) => {
  if (newVal === oldVal || !props.visible || !props.kbId) return;
  if (searchDebounce) clearTimeout(searchDebounce);
  searchDebounce = setTimeout(() => {
    void loadTags(true);
  }, 300);
});
</script>
