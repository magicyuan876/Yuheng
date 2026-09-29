<template>
  <Dialog :open="visible" @update:open="(v: boolean) => !v && handleClose()">
    <!--
      The close button is the dialog's own; the arbitrary variants move it to
      where the TDesign dialog had it (16px in, 28px square, 4px corners) so the
      header's right padding still clears it.
    -->
    <DialogContent
      class="[&>[data-slot=dialog-close]]:text-muted-foreground max-w-[min(420px,calc(100%-24px))] gap-0 overflow-hidden rounded p-0 sm:max-w-[min(420px,calc(100%-24px))] [&>[data-slot=dialog-close]]:top-4 [&>[data-slot=dialog-close]]:right-4 [&>[data-slot=dialog-close]]:rounded"
      @pointer-down-outside.prevent
    >
      <DialogHeader class="px-5 pt-5">
        <div class="flex min-w-0 flex-col gap-1 pr-7">
          <div class="flex min-w-0 items-center gap-2">
            <TagsIcon class="text-muted-foreground size-4 shrink-0" aria-hidden="true" />
            <DialogTitle class="text-[15px] leading-[22px] font-semibold tracking-[0.2px]">{{
              $t("knowledgeBase.batchTagDialogHeading")
            }}</DialogTitle>
          </div>
          <p class="text-placeholder m-0 truncate text-xs leading-[18px]">
            {{ $t("knowledgeBase.batchTagSubtitle", { count }) }}
          </p>
        </div>
      </DialogHeader>

      <div class="flex flex-col px-5 pt-4">
        <section
          class="flex flex-col gap-2.5 border-b border-[var(--td-component-stroke)] py-3 pb-4 first:pt-0 last:border-b-0 last:pb-0"
        >
          <div class="flex items-center justify-between gap-2">
            <h4
              class="text-foreground before:bg-primary m-0 flex flex-1 items-center gap-2 text-[13px] font-semibold select-none before:h-3.5 before:w-[3px] before:shrink-0 before:rounded-sm before:content-['']"
            >
              {{ $t("knowledgeBase.batchTagSelectedSection") }}
            </h4>
            <Button
              v-if="selectedSet.size > 0"
              variant="link"
              class="text-placeholder h-auto shrink-0 p-0 text-xs font-normal hover:no-underline"
              @click="clearAll"
            >
              {{ $t("knowledgeBase.tagClearAction") }}
            </Button>
          </div>
          <div
            v-if="selectedTagsList.length > 0"
            class="flex max-h-[min(120px,24vh)] [scrollbar-width:thin] flex-wrap gap-1.5 overflow-y-auto"
          >
            <button
              v-for="tag in selectedTagsList"
              :key="tag.id"
              type="button"
              data-slot="tag-chip"
              class="bg-muted text-foreground inline-block h-[22px] max-w-full cursor-pointer truncate rounded border border-transparent px-2 text-center [font-family:var(--app-font-family)] text-[11px] leading-5 font-medium antialiased transition-colors outline-none hover:bg-[color-mix(in_srgb,var(--td-bg-color-secondarycontainer)_70%,var(--td-bg-color-container))] focus-visible:shadow-[0_0_0_2px_color-mix(in_srgb,var(--td-component-stroke)_60%,transparent)]"
              :title="tag.name"
              @click="toggleTag(tag.id)"
            >
              {{ tag.name }}
            </button>
          </div>
          <p v-else class="text-placeholder m-0 min-h-[22px] text-xs leading-[22px]">
            {{ $t("knowledgeBase.batchTagNoSelected") }}
          </p>
        </section>

        <section
          class="flex flex-col gap-2.5 border-b border-[var(--td-component-stroke)] py-3 pb-4 first:pt-0 last:border-b-0 last:pb-0"
        >
          <div class="flex items-center justify-between gap-2">
            <h4
              class="text-foreground before:bg-primary m-0 flex flex-1 items-center gap-2 text-[13px] font-semibold select-none before:h-3.5 before:w-[3px] before:shrink-0 before:rounded-sm before:content-['']"
            >
              {{ $t("knowledgeBase.batchTagAvailableSection") }}
            </h4>
            <Button
              v-if="canManage"
              variant="link"
              class="text-placeholder hover:text-primary focus-visible:text-primary h-auto shrink-0 p-0 text-xs font-normal hover:no-underline"
              @click="handleOpenManage"
            >
              {{ $t("knowledgeBase.tagManageLink") }}
            </Button>
          </div>
          <div class="relative">
            <SearchIcon
              class="text-placeholder pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2"
              aria-hidden="true"
            />
            <!-- A 24px filled field that turns into a bordered one on hover or focus, as the TDesign small input did. -->
            <Input
              v-model="searchQuery"
              :placeholder="$t('knowledgeBase.tagEditSearch')"
              class="bg-muted dark:bg-muted hover:bg-card focus-visible:bg-card hover:border-border focus-visible:border-border placeholder:text-placeholder h-6 rounded border-transparent pr-7 pl-7 text-xs focus-visible:ring-0 md:text-xs"
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
          <div
            v-if="availableTagsList.length > 0"
            class="flex max-h-[min(120px,24vh)] [scrollbar-width:thin] flex-wrap gap-1.5 overflow-y-auto"
          >
            <button
              v-for="tag in availableTagsList"
              :key="tag.id"
              type="button"
              data-slot="tag-chip"
              class="text-muted-foreground hover:bg-muted hover:text-foreground inline-block h-[22px] max-w-full cursor-pointer truncate rounded border border-[var(--td-component-stroke)] bg-transparent px-2 text-center [font-family:var(--app-font-family)] text-[11px] leading-5 antialiased transition-colors outline-none focus-visible:shadow-[0_0_0_2px_color-mix(in_srgb,var(--td-component-stroke)_60%,transparent)]"
              :title="tag.knowledge_count !== undefined ? `${tag.name} (${tag.knowledge_count})` : tag.name"
              @click="toggleTag(tag.id)"
            >
              {{ tag.name }}
            </button>
          </div>
          <div
            v-else
            class="text-placeholder flex min-h-[22px] items-center justify-between gap-2 text-xs leading-[22px]"
          >
            <span>{{ searchQuery.trim() ? $t("knowledgeBase.tagEmptyResult") : $t("knowledgeBase.noTags") }}</span>
            <Button
              v-if="searchQuery.trim()"
              variant="ghost"
              size="xs"
              class="font-normal"
              :disabled="creatingTag"
              @click="handleCreateTag"
            >
              <Loader2Icon v-if="creatingTag" class="animate-spin" />
              {{ $t("knowledgeBase.tagCreateAction") }} "{{ searchQuery.trim() }}"
            </Button>
          </div>
          <div>
            <Input
              v-model="newTagName"
              :placeholder="$t('knowledgeBase.tagNewPlaceholder')"
              :maxlength="40"
              :disabled="creatingTag"
              class="placeholder:text-placeholder hover:bg-muted focus-visible:bg-muted hover:border-border focus-visible:border-border h-6 rounded border-dashed border-[var(--td-component-stroke)] bg-transparent text-xs focus-visible:ring-0 md:text-xs dark:bg-transparent"
              @keydown.enter="handleAddNewTag"
            />
          </div>
        </section>
      </div>

      <div
        class="mx-5 mt-3.5 mb-5 flex items-center justify-between gap-3 border-t border-[var(--td-component-stroke)] pt-3.5"
      >
        <span class="text-placeholder text-xs whitespace-nowrap">
          {{ $t("knowledgeBase.tagSelectedCount", { count: selectedSet.size }) }}
        </span>
        <div class="flex shrink-0 items-center gap-2">
          <Button variant="outline" size="xs" :disabled="confirmLoading" @click="handleClose">
            {{ $t("common.cancel") }}
          </Button>
          <Button size="xs" :disabled="confirmLoading" @click="handleConfirm">
            <Loader2Icon v-if="confirmLoading" class="animate-spin" />
            {{ $t("common.confirm") }}
          </Button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import { CircleXIcon, Loader2Icon, SearchIcon, TagsIcon } from "@lucide/vue";
import { createKnowledgeBaseTag } from "@/api/knowledge-base";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

interface Tag {
  id: string;
  name: string;
  color?: string;
  knowledge_count?: number;
}

const props = defineProps<{
  visible: boolean;
  count: number;
  kbId: string;
  tagList: Tag[];
  preSelectedTagIds?: string[];
  canManage?: boolean;
  confirmLoading?: boolean;
}>();

const emit = defineEmits<{
  (e: "update:visible", value: boolean): void;
  (e: "confirm", tagIds: string[]): void;
  (e: "tag-created"): void;
  (e: "open-manage"): void;
}>();

const { t } = useI18n();

const searchQuery = ref("");
const selectedSet = ref<Set<string>>(new Set());
const creatingTag = ref(false);
const newTagName = ref("");

watch(
  () => props.visible,
  (val) => {
    if (val) {
      selectedSet.value = new Set(props.preSelectedTagIds ?? []);
      searchQuery.value = "";
      newTagName.value = "";
    }
  },
);

const tagMap = computed(() => new Map(props.tagList.map((tag) => [tag.id, tag])));

const selectedTagsList = computed(() => {
  return Array.from(selectedSet.value)
    .map((id) => tagMap.value.get(id))
    .filter((tag): tag is Tag => Boolean(tag));
});

const availableTagsList = computed(() => {
  const query = searchQuery.value.trim().toLowerCase();
  return props.tagList.filter((tag) => {
    if (selectedSet.value.has(tag.id)) return false;
    if (query && !(tag.name || "").toLowerCase().includes(query)) return false;
    return true;
  });
});

function toggleTag(tagId: string) {
  const next = new Set(selectedSet.value);
  if (next.has(tagId)) {
    next.delete(tagId);
  } else {
    next.add(tagId);
  }
  selectedSet.value = next;
}

function clearAll() {
  selectedSet.value = new Set();
}

async function handleCreateTag() {
  const name = searchQuery.value.trim();
  if (!name) return;
  creatingTag.value = true;
  try {
    const res: any = await createKnowledgeBaseTag(props.kbId, { name });
    const newTag = res?.data || res;
    const next = new Set(selectedSet.value);
    next.add(newTag.id);
    selectedSet.value = next;
    searchQuery.value = "";
    emit("tag-created");
    MessagePlugin.success(t("knowledgeBase.tagCreateSuccess"));
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.operationFailed"));
  } finally {
    creatingTag.value = false;
  }
}

async function handleAddNewTag() {
  const name = newTagName.value.trim();
  if (!name) return;
  const exists = props.tagList.find((t) => t.name === name);
  if (exists) {
    const next = new Set(selectedSet.value);
    next.add(exists.id);
    selectedSet.value = next;
    newTagName.value = "";
    return;
  }
  creatingTag.value = true;
  try {
    const res: any = await createKnowledgeBaseTag(props.kbId, { name });
    const newTag = res?.data || res;
    const next = new Set(selectedSet.value);
    next.add(newTag.id);
    selectedSet.value = next;
    newTagName.value = "";
    emit("tag-created");
    MessagePlugin.success(t("knowledgeBase.tagCreateSuccess"));
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.operationFailed"));
  } finally {
    creatingTag.value = false;
  }
}

function handleConfirm() {
  if (props.confirmLoading) return;
  emit("confirm", Array.from(selectedSet.value));
}

function handleClose() {
  emit("update:visible", false);
}

function handleOpenManage() {
  emit("update:visible", false);
  emit("open-manage");
}
</script>
