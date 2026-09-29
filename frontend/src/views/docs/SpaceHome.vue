<template>
  <div class="flex min-h-0 min-w-0 flex-1">
    <!-- Folded away by SidebarToggle in the main column. Hidden outright rather than narrowed:
         a width transition would wrap the tree's rows on the way down, and there is nothing to
         see in the tree while it is closing. -->
    <aside
      class="w-[280px] flex-none flex-col border-r border-[var(--td-component-stroke)] bg-[var(--td-bg-color-secondarycontainer,var(--td-bg-color-container))] p-[12px_8px_8px]"
      :class="sidebar.collapsed.value ? 'hidden' : 'flex'"
    >
      <div class="flex-none p-[0_4px_8px]">
        <Button variant="ghost" size="sm" class="mb-1.5 -ml-1.5" @click="router.push({ name: 'docsSpaceList' })">
          <ChevronLeftIcon />
          {{ t("docs.spaces.backToList") }}
        </Button>
        <div v-if="space" class="flex min-w-0 items-center gap-2">
          <SpaceAvatar :name="space.name" :avatar="space.icon || ''" size="small" />
          <span class="text-foreground min-w-0 flex-1 truncate text-sm font-semibold" :title="space.name">
            {{ space.name }}
          </span>
          <span class="inline-flex flex-none gap-0.5">
            <Tooltip v-if="canEdit">
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :aria-label="t('docs.tree.newPage')"
                  :disabled="creating === ROOT_KEY"
                  @click="createUnder(null)"
                >
                  <Loader2Icon v-if="creating === ROOT_KEY" class="animate-spin" />
                  <PlusIcon v-else />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ t("docs.tree.newPage") }}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :aria-label="t('docs.search.title')"
                  @click="searchVisible = true"
                >
                  <SearchIcon />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ t("docs.search.title") }}</TooltipContent>
            </Tooltip>
            <Tooltip v-if="canEdit">
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :aria-label="t('docs.templates.newFrom')"
                  @click="openTemplatePicker(null)"
                >
                  <LayoutTemplateIcon />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ t("docs.templates.newFrom") }}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button variant="ghost" size="icon-sm" :aria-label="t('docs.trash.title')" @click="trashVisible = true">
                  <Trash2Icon />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ t("docs.trash.title") }}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :aria-label="t('docs.tree.settings')"
                  @click="router.push({ name: 'docsSpaceSettings', params: { slug } })"
                >
                  <SettingsIcon />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ t("docs.tree.settings") }}</TooltipContent>
            </Tooltip>
          </span>
        </div>
        <Skeleton v-else class="h-4 w-[70%]" />
      </div>

      <PageTree
        :model="model"
        :version="version"
        :active-id="activePageId"
        :can-edit="canEdit"
        :loading="treeLoading"
        @select="onSelect"
        @toggle="onToggle"
        @move="onMove"
        @action="onAction"
      />
    </aside>

    <main class="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <SidebarToggle />
      <PageView
        v-if="space && shortId"
        :space="space"
        :short-id="shortId"
        :last-event="lastEvent"
        @loaded="onPageLoaded"
        @renamed="onRenamed"
        @restored="onRestored"
        @create-child="createUnder"
      />
      <div
        v-else-if="space"
        class="flex flex-1 flex-col items-center justify-center gap-3 overflow-y-auto p-[40px_24px] text-center"
      >
        <SpaceAvatar :name="space.name" :avatar="space.icon || ''" size="large" />
        <h1 class="text-foreground mt-2 mb-0 text-[22px] font-semibold">
          {{ t("docs.pages.welcome", { name: space.name }) }}
        </h1>
        <p class="text-muted-foreground m-0 mb-2 max-w-[56ch] text-sm leading-[22px]">
          {{ space.description || t("docs.pages.welcomeHint") }}
        </p>
        <SpaceHomePanel :space-id="space.id" :space-slug="slug" @open="openPage" @open-visit="openVisit" />
        <div v-if="rootNodes.length" class="mb-2 w-[min(520px,100%)] text-left">
          <div class="text-placeholder mb-1.5 text-xs tracking-[0.04em] uppercase">
            {{ t("docs.pages.rootPages") }}
          </div>
          <button
            v-for="n in rootNodes"
            :key="n.id"
            type="button"
            data-slot="root-page"
            class="bg-card text-foreground hover:border-primary mb-1.5 flex w-full cursor-pointer items-center gap-2 rounded-[8px] border border-[var(--td-component-stroke)] px-2.5 py-2 text-left"
            @click="openPage(n)"
          >
            <span class="w-5 flex-none text-center">{{ n.icon || "📄" }}</span>
            <span class="min-w-0 flex-1 truncate text-sm">{{ n.title || t("docs.tree.untitled") }}</span>
          </button>
        </div>
        <Button v-if="canEdit" :disabled="creating === ROOT_KEY" @click="createUnder(null)">
          <Loader2Icon v-if="creating === ROOT_KEY" class="animate-spin" />
          <PlusIcon v-else />
          {{ t("docs.tree.newPage") }}
        </Button>
      </div>
      <div v-else-if="spaceMissing" class="text-muted-foreground p-12 text-center">
        {{ t("docs.spaces.loadFailed") }}
      </div>
    </main>

    <!-- Search -->
    <Dialog :open="searchVisible" @update:open="(v: boolean) => (searchVisible = v)">
      <DialogContent class="sm:max-w-[640px]">
        <DialogHeader>
          <DialogTitle>{{ t("docs.search.title") }}</DialogTitle>
        </DialogHeader>
        <SearchPanel :space-id="space?.id" @close="searchVisible = false" />
      </DialogContent>
    </Dialog>

    <!-- Start from a template -->
    <Dialog :open="templateVisible" @update:open="(v: boolean) => (templateVisible = v)">
      <DialogContent class="sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle>{{ t("docs.templates.newFrom") }}</DialogTitle>
        </DialogHeader>
        <TemplatePicker v-if="space" v-model="templateChoice" :space-id="space.id" />
        <DialogFooter>
          <Button variant="outline" @click="templateVisible = false">{{ t("common.cancel") }}</Button>
          <!-- No choice is a valid choice: it creates a blank page, as the old dialog did. -->
          <Button @click="createFromTemplate">
            {{ t("docs.templates.createPage") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Rename -->
    <Dialog :open="renameVisible" @update:open="(v: boolean) => (renameVisible = v)">
      <DialogContent class="sm:max-w-[460px]">
        <DialogHeader>
          <DialogTitle>{{ t("docs.tree.rename") }}</DialogTitle>
        </DialogHeader>
        <Input
          v-model="renameTitle"
          :maxlength="500"
          :placeholder="t('docs.tree.untitled')"
          autofocus
          @keydown.enter="submitRename"
        />
        <DialogFooter>
          <Button variant="outline" @click="renameVisible = false">{{ t("common.cancel") }}</Button>
          <Button :disabled="renaming" @click="submitRename">
            <Loader2Icon v-if="renaming" class="animate-spin" />
            {{ t("common.save") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Move to another space -->
    <Dialog :open="moveVisible" @update:open="(v: boolean) => (moveVisible = v)">
      <DialogContent class="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>{{ t("docs.tree.moveDialogTitle") }}</DialogTitle>
        </DialogHeader>
        <p class="text-muted-foreground m-0 mb-3 text-[13px] leading-5">{{ t("docs.tree.moveDialogHint") }}</p>
        <Select v-model="moveTargetId">
          <SelectTrigger>
            <SelectValue :placeholder="t('docs.tree.moveDialogPick')" />
          </SelectTrigger>
          <SelectContent>
            <div
              v-if="spacesLoading"
              class="text-muted-foreground flex items-center justify-center gap-2 py-3 text-[13px]"
            >
              <Loader2Icon class="size-3.5 animate-spin" />
            </div>
            <template v-else>
              <SelectItem v-for="o in moveTargets" :key="o.value" :value="o.value">{{ o.label }}</SelectItem>
              <div v-if="!moveTargets.length" class="text-placeholder px-2 py-3 text-center text-[13px]">
                {{ t("docs.tree.moveDialogNoSpaces") }}
              </div>
            </template>
          </SelectContent>
        </Select>
        <DialogFooter>
          <Button variant="outline" @click="moveVisible = false">{{ t("common.cancel") }}</Button>
          <Button :disabled="!moveTargetId || moving" @click="submitMoveToSpace">
            <Loader2Icon v-if="moving" class="animate-spin" />
            {{ t("common.confirm") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete -->
    <Dialog :open="deleteVisible" @update:open="(v: boolean) => (deleteVisible = v)">
      <DialogContent class="sm:max-w-[460px]">
        <DialogHeader>
          <DialogTitle>{{ t("docs.tree.delete") }}</DialogTitle>
        </DialogHeader>
        <p class="text-foreground text-sm">
          {{ t("docs.tree.deleteConfirm", { title: pendingNode?.title || t("docs.tree.untitled") }) }}
        </p>
        <DialogFooter>
          <Button variant="outline" @click="deleteVisible = false">{{ t("common.cancel") }}</Button>
          <Button variant="destructive" :disabled="deleting" @click="submitDelete">
            <Loader2Icon v-if="deleting" class="animate-spin" />
            {{ t("common.delete") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Drawer :open="trashVisible" swipe-direction="right" @update:open="(v: boolean) => (trashVisible = v)">
      <DrawerContent
        class="max-w-none rounded-none border-0 sm:max-w-none"
        :style="{ width: '440px', maxWidth: '90vw' }"
      >
        <DrawerHeader class="relative flex-row items-center justify-between">
          <DrawerTitle>{{ t("docs.trash.title") }}</DrawerTitle>
          <DrawerClose as-child>
            <Button variant="ghost" size="icon-sm" :aria-label="t('common.close')">
              <XIcon />
            </Button>
          </DrawerClose>
        </DrawerHeader>
        <div class="flex min-h-0 flex-1 flex-col overflow-y-auto px-4 pb-4">
          <TrashPanel v-if="space" :space="space" @restored="onRestored" />
        </div>
      </DrawerContent>
    </Drawer>
  </div>
</template>

<script setup lang="ts">
import {
  ChevronLeftIcon,
  LayoutTemplateIcon,
  Loader2Icon,
  PlusIcon,
  SearchIcon,
  SettingsIcon,
  Trash2Icon,
  XIcon,
} from "@lucide/vue";
import { MessagePlugin } from "tdesign-vue-next";
import { computed, onBeforeUnmount, ref, shallowRef, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

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
} from "@/api/docs";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Drawer, DrawerClose, DrawerContent, DrawerHeader, DrawerTitle } from "@/components/ui/drawer";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import SpaceAvatar from "@/components/SpaceAvatar.vue";

import { canEditSpaceContent, roleAtLeast } from "./docsAccess";
import SpaceHomePanel from "./home/SpaceHomePanel.vue";
import SearchPanel from "./search/SearchPanel.vue";
import TemplatePicker from "./templates/TemplatePicker.vue";
import type { Visit } from "./home/recentlyViewed";
import PageView from "./PageView.vue";
import SidebarToggle from "./SidebarToggle.vue";
import { useDocsSidebar } from "./useDocsSidebar";
import PageTree, { type TreeAction } from "./tree/PageTree.vue";
import { PageTreeModel, pageSlug, shortIdFromSlug, type MoveTarget, type TreeNodeData } from "./tree/pageTree";
import TrashPanel from "./TrashPanel.vue";
import { useDocsEvents, type DocsEvent } from "./useDocsEvents";

const ROOT_KEY = "__root__";

const { t } = useI18n();
const route = useRoute();
const router = useRouter();

const slug = computed(() => String(route.params.slug ?? ""));
const shortId = computed(() => {
  const s = route.params.pageSlug;
  return typeof s === "string" && s ? (shortIdFromSlug(s) ?? undefined) : undefined;
});

const space = shallowRef<DocsSpace | null>(null);
const spaceMissing = ref(false);
const spaceId = computed(() => space.value?.id);
const canEdit = computed(() => canEditSpaceContent(space.value?.role));

// ---- tree state ---------------------------------------------------------------------
const model = new PageTreeModel();
const version = ref(0);
const bump = () => {
  version.value += 1;
};
const treeLoading = ref(false);
const activePageId = ref<string | undefined>();
const rootNodes = computed(() => {
  void version.value;
  return model
    .childIds(null)
    .map((id) => model.get(id)!)
    .filter(Boolean);
});

const errorText = (err: unknown, fallback: string) => {
  const msg = (err as { message?: string } | null)?.message;
  return msg ? `${fallback}: ${msg}` : fallback;
};

async function loadChildren(parentId: string | null) {
  if (!space.value) return;
  const sid = space.value.id;
  const list = await loadAllChildren(sid, parentId);
  if (space.value?.id !== sid) return;
  model.setChildren(parentId, list);
  bump();
}

async function loadSpace() {
  const s = slug.value;
  space.value = null;
  spaceMissing.value = false;
  activePageId.value = undefined;
  model.reset();
  bump();
  if (!s) return;
  try {
    const sp = await getSpaceBySlug(s);
    if (slug.value !== s) return;
    space.value = sp;
    treeLoading.value = true;
    try {
      await loadChildren(null);
    } finally {
      treeLoading.value = false;
    }
    if (shortId.value) await revealActive();
  } catch (err: unknown) {
    spaceMissing.value = true;
    if (requestStatus(err) !== 404) MessagePlugin.error(errorText(err, t("docs.spaces.loadFailed")));
  }
}

/** Expands and loads the ancestor chain of the page named in the URL. */
async function revealActive() {
  if (!space.value || !activePageId.value) return;
  try {
    const ancestors = await getPageAncestors(activePageId.value);
    for (const a of ancestors) {
      if (!model.isLoaded(a.id)) await loadChildren(a.id);
      model.expand(a.id);
    }
    bump();
  } catch {
    // The page view reports its own errors; the tree just stays as it is.
  }
}

const onSelect = (node: TreeNodeData) => {
  openPage(node);
};

const openPage = (node: Pick<TreeNodeData, "title" | "short_id">) => {
  router.push({ name: "docsSpace", params: { slug: slug.value, pageSlug: pageSlug(node.title, node.short_id) } });
};

/** Opening a row of the on-device list: it carries the slug the URL needs,
 * so no lookup is required. */
const openVisit = (v: Visit) => {
  router.push({ name: "docsSpace", params: { slug: slug.value, pageSlug: pageSlug(v.title, v.shortId) } });
};

const onToggle = async (node: TreeNodeData) => {
  const open = model.toggle(node.id);
  bump();
  if (open && (!model.isLoaded(node.id) || model.isStale(node.id))) {
    try {
      await loadChildren(node.id);
    } catch (err: unknown) {
      MessagePlugin.error(errorText(err, t("docs.tree.loadFailed")));
    }
  }
};

const onMove = async (dragId: string, target: MoveTarget) => {
  const node = model.get(dragId);
  if (!node) return;
  try {
    const result = await movePage(dragId, {
      parent_id: target.parentId,
      ...(target.afterId === undefined ? {} : { after_id: target.afterId }),
    });
    if (result.rebalanced || (target.parentId && !model.isLoaded(target.parentId))) {
      await loadChildren(target.parentId);
    } else {
      model.move(dragId, target.parentId, result.page.position);
    }
    if (target.parentId) model.expand(target.parentId);
    bump();
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.tree.moveFailed")));
  }
};

// ---- actions -------------------------------------------------------------------------------
const creating = ref<string | null>(null);

// Starting from a template is a separate, explicit act: putting a chooser in
// front of every new page would tax the common case to serve the rare one.
const searchVisible = ref(false);
const templateVisible = ref(false);
const templateChoice = ref("");
const templateParent = ref<string | null>(null);
const pendingNode = ref<TreeNodeData | null>(null);

/** Opens the template chooser for a new page under parentId. */
function openTemplatePicker(parentId: string | null) {
  templateParent.value = parentId;
  templateChoice.value = "";
  templateVisible.value = true;
}

async function createFromTemplate() {
  templateVisible.value = false;
  await createUnder(templateParent.value, templateChoice.value || undefined);
}

async function createUnder(parentId: string | null, templateId?: string) {
  if (!space.value) return;
  creating.value = parentId ?? ROOT_KEY;
  try {
    const page = await createPage({
      space_id: space.value.id,
      parent_id: parentId,
      title: "",
      ...(templateId ? { template_id: templateId } : {}),
    });
    if (parentId) {
      if (!model.isLoaded(parentId)) await loadChildren(parentId);
      model.expand(parentId);
    }
    model.insert(toNode(page));
    bump();
    openPage(page);
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.tree.createFailed")));
  } finally {
    creating.value = null;
  }
}

function toNode(p: PageViewDto): TreeNodeData {
  return {
    id: p.id,
    short_id: p.short_id,
    space_id: p.space_id,
    parent_id: p.parent_id,
    position: p.position,
    title: p.title,
    icon: p.icon ?? null,
    has_children: p.has_children,
    can_edit: p.can_edit,
    restricted: p.restricted,
    updated_at: p.updated_at,
  };
}

const onAction = (action: TreeAction, node: TreeNodeData) => {
  pendingNode.value = node;
  switch (action) {
    case "new-child":
      void createUnder(node.id);
      break;
    case "rename":
      renameTitle.value = node.title;
      renameVisible.value = true;
      break;
    case "duplicate":
      void duplicate(node);
      break;
    case "copy-link":
      void copyLink(node);
      break;
    case "move-space":
      moveTargetId.value = "";
      moveVisible.value = true;
      void loadMoveTargets();
      break;
    case "delete":
      deleteVisible.value = true;
      break;
    default:
      break;
  }
};

// rename
const renameVisible = ref(false);
const renameTitle = ref("");
const renaming = ref(false);
async function submitRename() {
  const node = pendingNode.value;
  if (!node) return;
  renaming.value = true;
  try {
    const page = await updatePage(node.id, { title: renameTitle.value.trim() });
    model.update(node.id, { title: page.title });
    bump();
    renameVisible.value = false;
    if (activePageId.value === node.id) {
      router.replace({
        name: "docsSpace",
        params: { slug: slug.value, pageSlug: pageSlug(page.title, page.short_id) },
      });
    }
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.pages.renameFailed")));
  } finally {
    renaming.value = false;
  }
}

async function duplicate(node: TreeNodeData) {
  try {
    const result = await duplicatePage(node.id, {
      title: t("docs.tree.copySuffix", { title: node.title || t("docs.tree.untitled") }),
    });
    model.insert(toNode(result.page));
    bump();
    MessagePlugin.success(t("docs.tree.duplicateSuccess"));
    openPage(result.page);
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.tree.duplicateFailed")));
  }
}

async function copyLink(node: TreeNodeData) {
  const href = router.resolve({
    name: "docsSpace",
    params: { slug: slug.value, pageSlug: pageSlug(node.title, node.short_id) },
  }).href;
  try {
    await navigator.clipboard.writeText(`${window.location.origin}${href}`);
    MessagePlugin.success(t("docs.tree.linkCopied"));
  } catch {
    MessagePlugin.warning(`${window.location.origin}${href}`);
  }
}

// move to space
const moveVisible = ref(false);
const moveTargetId = ref("");
const moving = ref(false);
const spacesLoading = ref(false);
const otherSpaces = ref<DocsSpace[]>([]);
const moveTargets = computed(() =>
  otherSpaces.value
    .filter((s) => s.id !== space.value?.id && roleAtLeast(s.role, "writer"))
    .map((s) => ({ label: s.name, value: s.id })),
);

async function loadMoveTargets() {
  spacesLoading.value = true;
  try {
    otherSpaces.value = await listSpaces();
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.spaces.loadFailed")));
  } finally {
    spacesLoading.value = false;
  }
}

async function submitMoveToSpace() {
  const node = pendingNode.value;
  const target = otherSpaces.value.find((s) => s.id === moveTargetId.value);
  if (!node || !target) return;
  moving.value = true;
  try {
    const result = await movePage(node.id, { space_id: target.id, parent_id: null });
    const wasActive = !!activePageId.value && model.isDescendant(node.id, activePageId.value);
    model.remove(node.id);
    if (result.orphaned?.length) await loadChildren(null);
    bump();
    moveVisible.value = false;
    MessagePlugin.success(t("docs.tree.moved", { name: target.name }));
    if (result.orphaned?.length) MessagePlugin.info(t("docs.tree.orphanedNotice", { count: result.orphaned.length }));
    if (wasActive) router.push({ name: "docsSpace", params: { slug: slug.value } });
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.tree.moveFailed")));
  } finally {
    moving.value = false;
  }
}

// delete
const deleteVisible = ref(false);
const deleting = ref(false);
async function submitDelete() {
  const node = pendingNode.value;
  if (!node) return;
  deleting.value = true;
  try {
    await deletePage(node.id);
    const wasActive = !!activePageId.value && model.isDescendant(node.id, activePageId.value);
    model.remove(node.id);
    bump();
    deleteVisible.value = false;
    MessagePlugin.success(t("docs.tree.deleteSuccess"));
    if (wasActive) router.push({ name: "docsSpace", params: { slug: slug.value } });
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.tree.deleteFailed")));
  } finally {
    deleting.value = false;
  }
}

// ---- page view callbacks -----------------------------------------------------------------
const onPageLoaded = async (page: PageViewDto) => {
  activePageId.value = page.id;
  if (!model.has(page.id)) {
    await revealActive();
  } else {
    model.reveal(page.id);
    bump();
  }
};

const onRenamed = (id: string, title: string) => {
  model.update(id, { title });
  bump();
};

const onRestored = async (page: PageViewDto) => {
  if (page.parent_id && !model.isLoaded(page.parent_id)) {
    await loadChildren(page.parent_id);
  } else {
    model.insert(toNode(page));
  }
  if (page.parent_id) model.expand(page.parent_id);
  bump();
  trashVisible.value = false;
  openPage(page);
};

const trashVisible = ref(false);

// ---- live updates --------------------------------------------------------------------------
const lastEvent = shallowRef<DocsEvent | null>(null);
useDocsEvents(spaceId, (ev) => {
  lastEvent.value = ev;
  if (!space.value) return;
  const changed = model.applyEvent(ev, space.value.id, canEdit.value);
  for (const parent of model.takeStale()) {
    if (parent === null || model.isExpanded(parent)) void loadChildren(parent).catch(() => undefined);
  }
  if (changed) bump();
});

// Whether the tree is folded away; the toggle that changes it sits in the
// main column (SidebarToggle) and shares this state.
const sidebar = useDocsSidebar();

watch(slug, loadSpace, { immediate: true });
watch(shortId, (id) => {
  if (!id) activePageId.value = undefined;
});
onBeforeUnmount(() => {
  lastEvent.value = null;
});
</script>
