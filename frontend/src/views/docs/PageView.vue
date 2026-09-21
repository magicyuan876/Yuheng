<template>
  <div class="min-h-0 flex-1 overflow-y-auto px-12 pt-4 pb-16">
    <!-- Breadcrumbs -->
    <nav class="mb-5 flex flex-wrap items-center gap-0.5 text-[13px] text-muted-foreground" aria-label="breadcrumb">
      <Button variant="ghost" size="xs" class="max-w-[24ch] truncate font-normal text-[13px]" @click="goSpace">
        {{ space.name }}
      </Button>
      <template v-for="a in ancestors" :key="a.id">
        <ChevronRightIcon class="size-3 shrink-0 text-placeholder" />
        <Button variant="ghost" size="xs" class="max-w-[24ch] truncate font-normal text-[13px]" @click="goPage(a)">
          {{ a.title || t("docs.tree.untitled") }}
        </Button>
      </template>
      <template v-if="page">
        <ChevronRightIcon class="size-3 shrink-0 text-placeholder" />
        <span class="max-w-[24ch] truncate px-1.5 text-foreground">{{ page.title || t("docs.tree.untitled") }}</span>
      </template>
    </nav>

    <div v-if="loading" class="mx-auto my-6 max-w-[820px] space-y-3">
      <Skeleton class="h-8 w-1/2" />
      <Skeleton class="h-4 w-full" />
      <Skeleton class="h-4 w-[90%]" />
    </div>

    <!-- In the trash -->
    <div v-else-if="gone" class="mx-auto my-20 max-w-[520px] text-center text-muted-foreground">
      <Trash2Icon class="mx-auto size-8" />
      <h2 class="mt-3 mb-2 text-lg text-foreground">{{ t("docs.pages.gone") }}</h2>
      <p class="mb-3">{{ t("docs.pages.goneHint", { time: formatDate(gone.deleted_at) }) }}</p>
      <p v-if="!gone.restorable" class="text-[13px] text-placeholder">{{ t("docs.pages.restoreNotAllowed") }}</p>
      <Button v-else :disabled="restoring" @click="restore">
        <Loader2Icon v-if="restoring" class="animate-spin" />
        {{ t("docs.pages.restore") }}
      </Button>
    </div>

    <div v-else-if="notFound" class="mx-auto my-20 max-w-[520px] text-center text-muted-foreground">
      <CircleAlertIcon class="mx-auto size-8" />
      <h2 class="mt-3 mb-2 text-lg text-foreground">{{ t("docs.pages.notFound") }}</h2>
    </div>

    <article v-else-if="page" class="mx-auto max-w-[820px]">
      <header>
        <!-- Icon, title and status on one line, the way a document names
             itself everywhere else. Top-aligned, and the icon sized to the
             title's first line (32px × 1.25 = 40px), so a long title wraps
             beneath it rather than dragging it down the middle. -->
        <div class="flex items-start gap-2.5">
          <Popover v-if="page.can_edit" v-model:open="iconOpen">
            <PopoverTrigger as-child>
              <Button
                variant="ghost"
                size="icon"
                class="size-10 rounded-lg text-[30px] leading-none text-muted-foreground"
                :aria-label="t('docs.pages.iconPlaceholder')"
              >
                <span v-if="page.icon">{{ page.icon }}</span>
                <FileIcon v-else class="size-7" />
              </Button>
            </PopoverTrigger>
            <PopoverContent align="start" class="flex w-[260px] flex-col gap-2 p-2">
              <Input
                :model-value="iconDraft"
                maxlength="8"
                :placeholder="t('docs.pages.iconPlaceholder')"
                @update:model-value="(v) => (iconDraft = String(v))"
                @keydown.enter="saveIcon(iconDraft)"
              />
              <div class="grid grid-cols-6 gap-1">
                <Button
                  v-for="e in QUICK_ICONS"
                  :key="e"
                  variant="ghost"
                  size="icon-sm"
                  class="text-xl leading-none"
                  @click="saveIcon(e)"
                >
                  {{ e }}
                </Button>
              </div>
              <Button v-if="page.icon" variant="destructive" size="xs" class="self-start" @click="saveIcon('')">
                {{ t("docs.pages.removeIcon") }}
              </Button>
            </PopoverContent>
          </Popover>
          <span
            v-else
            class="flex size-10 shrink-0 items-center justify-center text-[30px] leading-none text-muted-foreground"
          >
            <span v-if="page.icon">{{ page.icon }}</span>
            <FileIcon v-else class="size-7" />
          </span>

          <!-- data-slot opts the bare textarea into the same element reset the
               new components get, so no browser chrome has to be undone by hand. -->
          <textarea
            v-if="page.can_edit"
            ref="titleInput"
            v-model="titleDraft"
            data-slot="page-title"
            class="min-w-0 flex-1 resize-none overflow-hidden text-[32px] leading-[1.25] font-bold text-foreground outline-none [font-family:var(--app-font-family)] placeholder:text-placeholder"
            rows="1"
            :placeholder="t('docs.pages.titlePlaceholder')"
            maxlength="500"
            @input="autosize"
            @keydown.enter.prevent="commitTitle"
            @blur="commitTitle"
          />
          <h1
            v-else
            class="m-0 min-w-0 flex-1 text-[32px] leading-[1.25] font-bold [font-family:var(--app-font-family)]"
            :class="page.title ? 'text-foreground' : 'text-placeholder'"
          >
            {{ page.title || t("docs.pages.titlePlaceholder") }}
          </h1>

          <!-- Centred on the title's first line, like the icon on the other side. -->
          <div v-if="!page.can_edit || page.is_locked || page.restricted" class="mt-[9px] flex shrink-0 gap-1.5">
            <Badge v-if="!page.can_edit" variant="secondary">{{ t("docs.pages.readOnly") }}</Badge>
            <Badge v-if="page.is_locked" variant="outline" class="border-warning/40 text-warning">
              <LockIcon />
              {{ t("docs.pages.locked") }}
            </Badge>
            <Badge v-if="page.restricted" variant="outline" class="border-primary/40 text-primary">
              {{ t("docs.pages.restricted") }}
            </Badge>
          </div>
        </div>

        <!-- The page's actions share a row with the timestamp, so each has to
             be quiet on its own: text-coloured, no border, a background only
             under the pointer. A toggled state (watching, starred, muted,
             locked) keeps the brand colour, so it reads as a state rather
             than as one more button. -->
        <div class="mt-1.5 flex flex-wrap items-center gap-0.5 text-xs text-placeholder">
          <!-- The live word count lives in the editor's own toolbar row;
               showing the persisted one here too would just disagree with
               it while someone is typing. -->
          <span class="mr-2 whitespace-nowrap">{{
            t("docs.pages.lastEdited", { time: formatDate(page.content_updated_at || page.updated_at) })
          }}</span>
          <Button variant="ghost" size="xs" class="page-action" @click="historyOpen = true">
            <HistoryIcon />
            {{ t("docs.history.title") }}
          </Button>
          <Button variant="ghost" size="xs" class="page-action" :aria-pressed="watchState.watched" @click="toggleWatch">
            <BookmarkIcon v-if="watchState.watched" />
            <BookmarkPlusIcon v-else />
            {{ watchState.watched ? t("docs.watch.watching") : t("docs.watch.watch") }}
          </Button>
          <Button
            v-if="watchState.watched"
            variant="ghost"
            size="xs"
            class="page-action"
            :aria-pressed="watchState.muted"
            @click="toggleMute"
          >
            <BellOffIcon v-if="watchState.muted" />
            <BellIcon v-else />
            {{ watchState.muted ? t("docs.watch.muted") : t("docs.watch.mute") }}
          </Button>
          <Button variant="ghost" size="xs" class="page-action" :aria-pressed="favourite" @click="toggleFavourite">
            <StarIcon :class="favourite ? 'fill-current text-warning' : ''" />
            {{ favourite ? t("docs.home.starred") : t("docs.home.star") }}
          </Button>
          <Button variant="ghost" size="xs" class="page-action" @click="accessOpen = true">
            <LockIcon v-if="page.restricted" />
            <UsersIcon v-else />
            {{ page.restricted ? t("docs.access.restricted") : t("docs.access.who") }}
          </Button>
          <Button variant="ghost" size="xs" class="page-action" @click="shareOpen = true">
            <Share2Icon />
            {{ t("docs.share.title") }}
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" size="xs" class="page-action" :disabled="exporting">
                <DownloadIcon />
                {{ t("docs.exportDoc.title") }}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              <DropdownMenuItem @select="runExport('markdown')">{{ t("docs.exportDoc.markdown") }}</DropdownMenuItem>
              <DropdownMenuItem @select="runExport('html')">{{ t("docs.exportDoc.html") }}</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <Button v-if="page.can_edit" variant="ghost" size="xs" class="page-action" @click="openSaveTemplate">
            <LayoutTemplateIcon />
            {{ t("docs.templates.saveAs") }}
          </Button>
          <Button
            v-if="page.role === 'admin'"
            variant="ghost"
            size="xs"
            class="page-action"
            :aria-pressed="page.is_locked"
            @click="toggleLock"
          >
            <LockIcon v-if="page.is_locked" />
            <LockOpenIcon v-else />
            {{ page.is_locked ? t("docs.lock.locked") : t("docs.lock.lock") }}
          </Button>
          <Button v-if="page.can_edit" variant="ghost" size="xs" class="page-action" @click="toggleDraft">
            <PencilIcon v-if="page.status === 'draft'" />
            <CircleCheckIcon v-else />
            {{ page.status === "draft" ? t("docs.lock.draft") : t("docs.lock.published") }}
          </Button>
          <NotificationCentre :revision="notificationRevision" />
        </div>
        <PageLabels
          v-if="page.can_edit || labels.length"
          class="mt-2.5"
          :page-id="page.id"
          :space-id="page.space_id"
          :can-edit="page.can_edit"
          :labels="labels"
          @change="onLabelsChanged"
        />
      </header>

      <div class="mt-4 flex items-start gap-6">
        <DocEditor
          v-if="editingModeKnown"
          ref="docEditor"
          :key="editorKey"
          class="page-body min-w-0 flex-1 text-[15px] leading-[1.75] text-foreground"
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
        :placeholder="t('docs.comments.placeholder')"
        @submit="(body) => docEditor?.submitComment(body)"
        @cancel="() => docEditor?.cancelComment()"
      />

      <BacklinksPanel :entries="backlinks" :loading="backlinksLoading" />

      <section v-if="children.length" class="mt-10 border-t border-border pt-4">
        <h3 class="mb-2 text-[13px] font-semibold tracking-[0.04em] text-muted-foreground uppercase">
          {{ t("docs.pages.subpages") }}
        </h3>
        <ul class="mb-2 list-none">
          <li v-for="c in children" :key="c.id">
            <Button variant="ghost" size="sm" class="h-auto px-2 py-1.5 text-sm font-normal" @click="goPage(c)">
              <span class="w-5 text-center">{{ c.icon || "📄" }}</span>
              {{ c.title || t("docs.tree.untitled") }}
            </Button>
          </li>
        </ul>
        <Button
          v-if="page.can_edit"
          variant="ghost"
          size="sm"
          class="text-muted-foreground"
          @click="emit('createChild', page.id)"
        >
          <PlusIcon />
          {{ t("docs.tree.newSubpage") }}
        </Button>
      </section>
      <Button
        v-else-if="page.can_edit"
        variant="ghost"
        size="sm"
        class="mt-8 text-muted-foreground"
        @click="emit('createChild', page.id)"
      >
        <PlusIcon />
        {{ t("docs.tree.newSubpage") }}
      </Button>
    </article>

    <HistoryPanel
      v-if="page"
      :visible="historyOpen"
      :page-id="page.id"
      :can-edit="page.can_edit"
      @close="historyOpen = false"
      @restored="onRestored"
    />

    <Dialog v-model:open="templateOpen">
      <DialogContent class="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>{{ t("docs.templates.saveAs") }}</DialogTitle>
          <!-- Said before it happens, not discovered afterwards. -->
          <DialogDescription>{{ t("docs.templates.stripNote") }}</DialogDescription>
        </DialogHeader>
        <div class="flex flex-col gap-2.5">
          <Input
            :model-value="templateName"
            maxlength="120"
            :placeholder="t('docs.templates.namePlaceholder')"
            @update:model-value="(v) => (templateName = String(v))"
          />
          <Input
            :model-value="templateCategory"
            maxlength="64"
            :placeholder="t('docs.templates.categoryPlaceholder')"
            @update:model-value="(v) => (templateCategory = String(v))"
          />
          <Textarea
            :model-value="templateDescription"
            maxlength="500"
            rows="3"
            :placeholder="t('docs.templates.descriptionPlaceholder')"
            @update:model-value="(v) => (templateDescription = String(v))"
          />
        </div>
        <DialogFooter>
          <DialogClose as-child>
            <Button variant="outline">{{ t("common.cancel") }}</Button>
          </DialogClose>
          <Button :disabled="savingTemplate || !templateName.trim()" @click="saveAsTemplate">
            <Loader2Icon v-if="savingTemplate" class="animate-spin" />
            {{ t("common.save") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- The panels below carry their own explanatory text, so the dialog has
         no description of its own; saying so keeps the a11y check quiet. -->
    <Dialog v-model:open="shareOpen">
      <DialogContent class="sm:max-w-[560px]" :aria-describedby="undefined">
        <DialogHeader>
          <DialogTitle>{{ t("docs.share.title") }}</DialogTitle>
        </DialogHeader>
        <SharePanel v-if="page" :page-id="page.id" :can-manage="page.can_edit" :restricted="page.restricted" />
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="accessOpen">
      <DialogContent class="sm:max-w-[560px]" :aria-describedby="undefined">
        <DialogHeader>
          <DialogTitle>{{ t("docs.access.title") }}</DialogTitle>
        </DialogHeader>
        <PageAccessPanel v-if="page" :page-id="page.id" @changed="onAccessChanged" />
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { MessagePlugin } from "tdesign-vue-next";
import { computed, nextTick, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

import {
  createTemplate,
  exportPage,
  setPageLocked,
  setPageStatus,
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
  type ExportFormat,
  type PageAccessView,
  type PageRef,
  type PageView as PageViewDto,
  type TreeNode,
  type WatchView,
} from "@/api/docs";

import { useAuthStore } from "@/stores/auth";
import { useDeploymentCapabilitiesStore } from "@/stores/deploymentCapabilities";

import CommentComposer from "./comments/CommentComposer.vue";
import PageAccessPanel from "./access/PageAccessPanel.vue";
import SharePanel from "./share/SharePanel.vue";
import PageLabels from "./labels/PageLabels.vue";
import { browserStore, recordVisit } from "./home/recentlyViewed";
import NotificationCentre from "./notifications/NotificationCentre.vue";
import CommentsPanel from "./comments/CommentsPanel.vue";
import BacklinksPanel from "./editor/BacklinksPanel.vue";
import HistoryPanel from "./history/HistoryPanel.vue";
import DocEditor from "./editor/DocEditor.vue";
import type { TocEntry } from "./editor/toc";
import TocSidebar from "./editor/TocSidebar.vue";
import { pageSlug } from "./tree/pageTree";
import type { DocsEvent } from "./useDocsEvents";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import {
  BellIcon,
  BellOffIcon,
  BookmarkIcon,
  BookmarkPlusIcon,
  ChevronRightIcon,
  CircleAlertIcon,
  CircleCheckIcon,
  DownloadIcon,
  FileIcon,
  HistoryIcon,
  LayoutTemplateIcon,
  Loader2Icon,
  LockIcon,
  LockOpenIcon,
  PencilIcon,
  PlusIcon,
  Share2Icon,
  StarIcon,
  Trash2Icon,
  UsersIcon,
} from "@lucide/vue";

const QUICK_ICONS = ["📄", "📘", "📗", "📙", "📝", "📌", "🚀", "💡", "🔧", "📊", "🗂️", "✅"];

const props = defineProps<{
  space: DocsSpace;
  shortId: string;
  lastEvent: DocsEvent | null;
}>();

const emit = defineEmits<{
  loaded: [page: PageViewDto];
  renamed: [id: string, title: string];
  restored: [page: PageViewDto];
  createChild: [parentId: string];
}>();

const { t } = useI18n();
const router = useRouter();

const page = ref<PageViewDto | null>(null);
const ancestors = ref<DocsPage[]>([]);
const children = ref<TreeNode[]>([]);
const loading = ref(false);
const gone = ref<GonePage | null>(null);
const notFound = ref(false);
const titleDraft = ref("");
const titleInput = ref<HTMLTextAreaElement | null>(null);
const iconOpen = ref(false);
const iconDraft = ref("");

// ---- collaborative editor wiring ------------------------------------------
const authStore = useAuthStore();
const capabilities = useDeploymentCapabilitiesStore();
const docEditor = ref<InstanceType<typeof DocEditor> | null>(null);
const historyOpen = ref(false);
const accessOpen = ref(false);
const shareOpen = ref(false);
const templateOpen = ref(false);
const savingTemplate = ref(false);
const templateName = ref("");
const templateCategory = ref("");
const templateDescription = ref("");

// Exporting this page. The file is built by the request and handed to the
// browser's save dialog, so there is nothing to poll and nothing to clean up.
const exporting = ref(false);
async function runExport(format: ExportFormat) {
  if (exporting.value || !page.value) return;
  exporting.value = true;
  try {
    await exportPage(page.value.id, format);
  } catch (err) {
    void MessagePlugin.error(errorText(err, t("docs.exportDoc.failed")));
  } finally {
    exporting.value = false;
  }
}

function openSaveTemplate() {
  templateName.value = page.value?.title ?? "";
  templateCategory.value = "";
  templateDescription.value = "";
  templateOpen.value = true;
}

async function toggleLock() {
  const current = page.value;
  if (!current) return;
  try {
    const next = await setPageLocked(current.id, !current.is_locked);
    page.value = { ...current, ...next };
  } catch (err) {
    void MessagePlugin.error(errorText(err, t("docs.lock.changeFailed")));
  }
}

async function toggleDraft() {
  const current = page.value;
  if (!current) return;
  try {
    const next = await setPageStatus(current.id, current.status === "draft" ? "published" : "draft");
    page.value = { ...current, ...next };
  } catch (err) {
    void MessagePlugin.error(errorText(err, t("docs.lock.changeFailed")));
  }
}

async function saveAsTemplate() {
  const current = page.value;
  if (!current || !templateName.value.trim()) return;
  savingTemplate.value = true;
  try {
    await createTemplate({
      space_id: current.space_id,
      name: templateName.value,
      category: templateCategory.value,
      description: templateDescription.value,
      from_page_id: current.id,
    });
    templateOpen.value = false;
    void MessagePlugin.success(t("docs.templates.saved"));
  } catch (err) {
    void MessagePlugin.error(errorText(err, t("docs.templates.saveFailed")));
  } finally {
    savingTemplate.value = false;
  }
}

/** The badge in the header follows the panel without a refetch. */
function onAccessChanged(next: PageAccessView) {
  if (page.value) page.value = { ...page.value, restricted: next.restricted || next.inherited_from.length > 0 };
}

/**
 * Whether this reader follows the page, and whether they have silenced it.
 *
 * Watching is a reader's right rather than an editor's: you can follow a page
 * you are not allowed to change, which is what makes review work.
 */
const watchState = ref<WatchView>({ page_id: "", muted: false, watched: false });
/** Bumped when the event stream says a notification arrived, so the bell's
 * count is current without polling. */
const notificationRevision = ref(0);

/** This page's labels, and whether this person starred it. Both arrive with
 * the page itself; neither costs a second request. */
const labels = ref<LabelView[]>([]);
const favourite = ref(false);

/** Where this person has been, kept on this device only. */
const visitStore = browserStore();

function onLabelsChanged(next: LabelView[]) {
  labels.value = next;
}

async function toggleFavourite() {
  const current = page.value;
  if (!current) return;
  const next = !favourite.value;
  favourite.value = next;
  try {
    await setFavourite(current.id, next);
  } catch (err) {
    favourite.value = !next;
    void MessagePlugin.error((err as { message?: string })?.message ?? "");
  }
}

async function loadWatchState(pageId: string) {
  try {
    watchState.value = await getWatchState(pageId);
  } catch {
    // Not knowing whether somebody follows a page is not worth an error over
    // the page itself; the button simply shows the unwatched state.
    watchState.value = { page_id: pageId, muted: false, watched: false };
  }
}

async function toggleWatch() {
  const current = page.value;
  if (!current) return;
  try {
    watchState.value = await setWatch(current.id, !watchState.value.watched);
  } catch (err) {
    void MessagePlugin.error((err as { message?: string })?.message ?? "");
  }
}

async function toggleMute() {
  const current = page.value;
  if (!current) return;
  try {
    watchState.value = await setMuted(current.id, !watchState.value.muted);
  } catch (err) {
    void MessagePlugin.error((err as { message?: string })?.message ?? "");
  }
}

/** The editor owns the comment machinery; the panel is drawn here. */
const comments = computed(() => docEditor.value?.comments ?? null);

/**
 * Selecting a thread highlights it and brings its passage into view, which is
 * what somebody clicking a comment in the list is asking for.
 */
function onSelectComment(commentId: string) {
  const handle = comments.value;
  if (!handle) return;
  handle.select(commentId);

  const placement = handle.placementByID.value.get(commentId);
  const editor = docEditor.value?.editor;
  if (!placement || placement.from === undefined || !editor) return;
  try {
    const coords = editor.view.coordsAtPos(placement.from);
    window.scrollTo({ top: window.scrollY + coords.top - 160, behavior: "smooth" });
  } catch {
    // The position can be stale for a frame after a large remote change;
    // highlighting without scrolling is still useful.
  }
}

function onShowResolved(value: boolean) {
  const handle = comments.value;
  if (!handle) return;
  handle.showResolved.value = value;
  void handle.load();
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
const restoreTick = ref(0);

/**
 * A restore leaves the page's own row — its word count, its "last edited"
 * line — saying what it did before, so that is refreshed here.
 */
async function onRestored() {
  if (!collabUrl.value) restoreTick.value++;
  const sid = props.shortId;
  try {
    const fresh = await getPageByShortId(sid);
    // Refreshed in place rather than through load(): clearing the page would
    // unmount the editor, which is exactly what the collaborative case is
    // trying to avoid.
    if (props.shortId === sid && page.value?.id === fresh.id) {
      page.value = { ...page.value, ...fresh };
    }
  } catch {
    // The page is still on screen and correct; a stale word count in the
    // header is not worth an error message about it.
  }
}
const headings = ref<TocEntry[]>([]);

const tenantId = computed(() => authStore.effectiveTenantId ?? "");
const currentUser = computed(() => authStore.user);
/** The browser-facing collaboration WebSocket address, reported by
 * GET /system/capabilities; empty in a deployment without the service. */
const collabUrl = computed(() => capabilities.docsCollabUrl);
const getToken = () => localStorage.getItem("yuheng_token");

onMounted(() => {
  // Cheap when another view already loaded it: the store caches the answer.
  void capabilities.ensureLoaded();
});

/** The editor is not mounted until the deployment capabilities have been
 * read, because an empty collaboration address means two different things
 * before and after: "not loaded yet" and "this deployment edits pages
 * exclusively". Mounting early would open the wrong transport and, in the
 * exclusive case, take a lease on a page the user is only looking at. */
const editingModeKnown = computed(() => capabilities.loaded);

/** Remounts the editor when the page changes, and also if the collaboration
 * address itself changes, since the editor binds to one Y.Doc and one
 * transport for its lifetime. */
const editorKey = computed(() => `${page.value?.id ?? ""}|${collabUrl.value}|${restoreTick.value}`);

const onHeadings = (entries: TocEntry[]) => {
  headings.value = entries;
};

// ---- backlinks --------------------------------------------------------------
const backlinks = ref<PageRef[]>([]);
const backlinksLoading = ref(false);

async function loadBacklinks(id: string) {
  backlinksLoading.value = true;
  try {
    const rows = await listBacklinks(id);
    if (page.value?.id === id) backlinks.value = rows;
  } catch {
    if (page.value?.id === id) backlinks.value = [];
  } finally {
    backlinksLoading.value = false;
  }
}

/** Puts the caret at the heading and lets the editor scroll it into view. */
const scrollToHeading = (pos: number) => {
  const editor = docEditor.value?.editor;
  if (!editor) return;
  editor
    .chain()
    .setTextSelection(pos + 1)
    .scrollIntoView()
    .run();
};

const formatDate = (iso: string | null | undefined) => {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
};

const errorText = (err: unknown, fallback: string) => {
  const msg = (err as { message?: string } | null)?.message;
  return msg ? `${fallback}: ${msg}` : fallback;
};

async function load() {
  const sid = props.shortId;
  loading.value = true;
  gone.value = null;
  notFound.value = false;
  page.value = null;
  headings.value = [];
  backlinks.value = [];
  children.value = [];
  ancestors.value = [];
  try {
    const p = await getPageByShortId(sid);
    if (props.shortId !== sid) return;
    page.value = p;
    titleDraft.value = p.title;
    labels.value = p.labels ?? [];
    favourite.value = p.favourite ?? false;
    recordVisit(visitStore, {
      pageId: p.id,
      shortId: p.short_id,
      spaceSlug: props.space.slug,
      title: p.title,
      icon: p.icon || undefined,
      at: Date.now(),
    });
    emit("loaded", p);
    await Promise.all([loadAncestors(p.id), loadChildren(p), loadBacklinks(p.id), loadWatchState(p.id)]);
    await nextTick();
    autosize();
  } catch (err: unknown) {
    if (props.shortId !== sid) return;
    const g = gonePageFrom(err);
    if (g) {
      gone.value = g;
    } else if (requestStatus(err) === 404) {
      notFound.value = true;
    } else {
      MessagePlugin.error(errorText(err, t("docs.pages.loadFailed")));
      notFound.value = true;
    }
  } finally {
    if (props.shortId === sid) loading.value = false;
  }
}

async function loadAncestors(id: string) {
  try {
    ancestors.value = await getPageAncestors(id);
  } catch {
    ancestors.value = [];
  }
}

async function loadChildren(p: PageViewDto) {
  if (!p.has_children) {
    children.value = [];
    return;
  }
  try {
    const res = await getPageChildren(p.id, { limit: 200 });
    if (page.value?.id === p.id) children.value = res.items;
  } catch {
    children.value = [];
  }
}

// ---- title & icon -------------------------------------------------------------------
const autosize = () => {
  const el = titleInput.value;
  if (!el) return;
  el.style.height = "auto";
  el.style.height = `${el.scrollHeight}px`;
};

let savingTitle = false;
async function commitTitle() {
  const p = page.value;
  if (!p || savingTitle) return;
  const next = titleDraft.value.replace(/[\r\n]+/g, " ").trim();
  if (next === p.title) {
    titleDraft.value = p.title;
    return;
  }
  savingTitle = true;
  try {
    const updated = await updatePage(p.id, { title: next });
    page.value = { ...p, title: updated.title, updated_at: updated.updated_at };
    titleDraft.value = updated.title;
    emit("renamed", p.id, updated.title);
    router.replace({
      name: "docsSpace",
      params: { slug: props.space.slug, pageSlug: pageSlug(updated.title, updated.short_id) },
    });
  } catch (err: unknown) {
    titleDraft.value = p.title;
    MessagePlugin.error(errorText(err, t("docs.pages.renameFailed")));
  } finally {
    savingTitle = false;
  }
}

async function saveIcon(value: string) {
  const p = page.value;
  if (!p) return;
  try {
    const updated = await updatePage(p.id, { icon: value.trim() });
    page.value = { ...p, icon: updated.icon ?? null };
    iconOpen.value = false;
    iconDraft.value = "";
    emit("renamed", p.id, updated.title);
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.pages.renameFailed")));
  }
}

async function restore() {
  if (!gone.value) return;
  restoring.value = true;
  try {
    const restored = await restorePage(gone.value.page_id);
    MessagePlugin.success(t("docs.pages.restoreSuccess"));
    emit("restored", restored);
    await load();
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.pages.restoreFailed")));
  } finally {
    restoring.value = false;
  }
}
const restoring = ref(false);

// ---- navigation -----------------------------------------------------------------------
const goSpace = () => router.push({ name: "docsSpace", params: { slug: props.space.slug } });
const goPage = (p: Pick<DocsPage, "title" | "short_id">) =>
  router.push({
    name: "docsSpace",
    params: { slug: props.space.slug, pageSlug: pageSlug(p.title, p.short_id) },
  });

// ---- live updates ----------------------------------------------------------------------
watch(
  () => props.lastEvent,
  (ev) => {
    const p = page.value;
    if (!ev || !p) return;
    const payload = ev.payload ?? {};

    // Renaming any page changes what every link to it should read as, wherever
    // that link is. The title is cached in the editor rather than stored in the
    // document, so forgetting the entry is the whole of the update.
    if (ev.type === "docs.page.meta_updated" && ev.page_id && ev.page_id !== p.id) {
      docEditor.value?.forgetTitle(ev.page_id);
    }
    // Another page's body changed, and this one may quote a block of it. What
    // is shown here is cached, not stored, so forgetting the entries is the
    // whole of the update — the same shape as the title cache above.
    if (
      ev.page_id &&
      ev.page_id !== p.id &&
      (ev.type === "docs.page.content_updated" ||
        ev.type === "docs.page.content_replaced" ||
        ev.type === "docs.page.deleted" ||
        ev.type === "docs.page.purged")
    ) {
      docEditor.value?.forgetBlockRefs(ev.page_id);
    }
    // Another page's body may have gained or lost a link to this one; the rows
    // are rebuilt whenever a page is saved.
    if (
      ev.page_id !== p.id &&
      (ev.type === "docs.page.content_updated" ||
        ev.type === "docs.page.content_replaced" ||
        ev.type === "docs.page.deleted" ||
        ev.type === "docs.page.purged")
    ) {
      void loadBacklinks(p.id);
    }

    // Somebody else commented, replied, resolved or deleted on this page.
    if (ev.type === "docs.comment.changed" && ev.page_id === p.id) {
      void comments.value?.load();
    }

    // A notification was written for somebody; the bell re-reads its own count
    // rather than trusting the event, since the event does not say whose.
    if (ev.type === "docs.notification.created") {
      notificationRevision.value++;
    }

    if (ev.page_id === p.id) {
      switch (ev.type) {
        case "docs.page.meta_updated":
          if ("title" in payload && !savingTitle) {
            page.value = { ...p, title: String(payload.title ?? "") };
            titleDraft.value = String(payload.title ?? "");
          }
          if ("icon" in payload) page.value = { ...page.value!, icon: (payload.icon as string | null) ?? null };
          break;
        case "docs.page.content_updated":
        case "docs.page.content_replaced":
          // A body that just changed may have gained or lost a link to this
          // page, and the backlink rows are rebuilt on save.
          void loadBacklinks(p.id);
          // Nothing to refetch: the body is the Yjs document the editor is
          // already connected to, and the collaboration service pushes both
          // a peer's edits and a server-side replace straight into it.
          break;
        case "docs.page.deleted":
        case "docs.page.moved":
        case "docs.page.purged":
          void load();
          break;
        default:
          break;
      }
      return;
    }
    // A child was added, removed or renamed: refresh the subpage list.
    const parentId = payload.parent_id as string | null | undefined;
    if (parentId === p.id || children.value.some((c) => c.id === ev.page_id)) {
      void loadChildren({ ...p, has_children: true });
    }
  },
);

watch(() => props.shortId, load, { immediate: true });
</script>

<style scoped>
/*
 * Everything on this screen is styled with utility classes except this: the
 * document body is rendered by the editor, not by this component, so its
 * headings, lists, code and tables cannot carry classes of ours. They are
 * styled as descendants of the editor's root. The colours are TDesign's
 * tokens, the same ones the utilities resolve to.
 */
.page-body :deep(h1),
.page-body :deep(h2),
.page-body :deep(h3) {
  margin: 1.4em 0 0.5em;
  font-weight: 600;
  line-height: 1.3;
}

.page-body :deep(p) {
  margin: 0.5em 0;
}

.page-body :deep(pre) {
  padding: 12px 14px;
  border-radius: 8px;
  background: var(--td-bg-color-secondarycontainer);
  overflow-x: auto;
  font-size: 13px;
}

.page-body :deep(code) {
  font-family: var(--td-font-family-mono, ui-monospace, monospace);
}

.page-body :deep(blockquote) {
  margin: 0.8em 0;
  padding: 4px 14px;
  border-left: 3px solid var(--td-brand-color);
  color: var(--td-text-color-secondary);
}

.page-body :deep(table) {
  border-collapse: collapse;
  width: 100%;
  margin: 1em 0;
}

.page-body :deep(th),
.page-body :deep(td) {
  border: 1px solid var(--td-component-stroke);
  padding: 6px 10px;
  text-align: left;
}

.page-body :deep(img) {
  max-width: 100%;
  border-radius: 6px;
}

.page-body :deep(a) {
  color: var(--td-brand-color);
}

.page-body :deep(ul),
.page-body :deep(ol) {
  padding-left: 1.6em;
}

/*
 * The action row. `page-action` is the one class the template shares across
 * nine buttons, and it exists so the toggled state can be expressed once:
 * a button with aria-pressed="true" is a state, and keeps the brand colour
 * where the others only colour up under the pointer.
 */
.page-action {
  font-weight: 400;
  color: var(--td-text-color-secondary);
}

.page-action:hover:not(:disabled) {
  color: var(--td-text-color-primary);
}

.page-action[aria-pressed="true"] {
  color: var(--td-brand-color);
}
</style>
