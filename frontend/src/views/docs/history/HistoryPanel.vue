<template>
  <Drawer :open="visible" swipe-direction="right" @update:open="(v: boolean) => !v && emit('close')">
    <DrawerContent class="max-w-none rounded-none border-0 sm:max-w-none" :style="{ width: 'min(1080px, 94vw)' }">
      <DrawerHeader class="relative flex-row items-center justify-between border-b border-[var(--td-component-stroke)]">
        <DrawerTitle>{{ t("docs.history.title") }}</DrawerTitle>
        <!-- The old t-drawer carried a close button in its header. -->
        <DrawerClose
          class="text-muted-foreground hover:bg-accent hover:text-foreground inline-flex size-7 cursor-pointer items-center justify-center rounded-md"
          :aria-label="t('common.close')"
        >
          <XIcon class="size-4" />
        </DrawerClose>
      </DrawerHeader>
      <div class="grid min-h-0 flex-1 grid-cols-[290px_minmax(0,1fr)] gap-4 p-4 max-[760px]:grid-cols-[minmax(0,1fr)]">
        <!-- The versions, newest first. -->
        <aside class="overflow-y-auto border-r border-[var(--td-component-stroke)] pr-2" @scroll="onScroll">
          <p v-if="loading && !items.length" class="text-placeholder my-3 text-[13px]">{{ t("common.loading") }}</p>
          <p v-else-if="!items.length" class="text-placeholder my-3 text-[13px]">{{ t("docs.history.empty") }}</p>

          <button
            v-for="(item, index) in items"
            :key="item.id"
            type="button"
            data-slot="history-item"
            class="relative flex w-full cursor-pointer flex-col gap-0.5 rounded-md border-0 px-2.5 py-2 text-left"
            :class="[
              item.id === selectedId ? 'bg-[var(--td-brand-color-light)]' : 'hover:bg-accent',
              item.id === compareId ? 'ring-primary ring-1 ring-inset' : '',
            ]"
            @click="select(item, index)"
          >
            <span class="text-foreground text-[13px] tabular-nums">{{ formatWhen(item.created_at) }}</span>
            <span class="text-muted-foreground text-xs">{{ who(item) }}</span>
            <span class="text-placeholder flex gap-2 text-[11px]">
              <span
                :class="
                  item.reason === 'restore' || item.reason === 'import' || item.reason === 'publish'
                    ? 'text-primary'
                    : ''
                "
              >
                {{ t(`docs.history.reason.${item.reason}`) }}
              </span>
              <span>{{ t("docs.history.words", { count: item.word_count }) }}</span>
            </span>
            <span
              v-if="index > 0"
              class="text-placeholder hover:text-primary absolute top-2 right-2 leading-none"
              role="button"
              :aria-label="t('docs.history.compareWith')"
              @click.stop="setCompare(item)"
            >
              <ArrowLeftRightIcon class="size-3.5" />
            </span>
          </button>

          <p v-if="loadingMore" class="text-placeholder my-3 text-[13px]">{{ t("common.loading") }}</p>
        </aside>

        <!-- What the selected version says, or how it differs. -->
        <section class="flex min-h-0 min-w-0 flex-col">
          <header
            v-if="selected"
            class="flex items-start justify-between gap-3 border-b border-[var(--td-component-stroke)] pb-2.5"
          >
            <div>
              <h3 class="m-0 text-[15px]">{{ selected.title || t("docs.tree.untitled") }}</h3>
              <p class="text-placeholder m-0 mt-0.5 text-xs">
                {{ comparing ? t("docs.history.comparing", { a: compareLabel, b: selectedLabel }) : selectedLabel }}
              </p>
            </div>
            <div class="flex flex-none items-center gap-2">
              <!-- A two-way switch, as the old filled radio-button group was. -->
              <div class="bg-muted inline-flex rounded-md p-0.5" role="radiogroup">
                <button
                  v-for="option in MODES"
                  :key="option"
                  type="button"
                  data-slot="history-mode"
                  role="radio"
                  :aria-checked="mode === option"
                  class="h-6 cursor-pointer rounded-[5px] px-2.5 text-xs transition-colors"
                  :class="
                    mode === option
                      ? 'bg-card text-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground'
                  "
                  @click="mode = option"
                >
                  {{ option === "preview" ? t("docs.history.preview") : t("docs.history.changes") }}
                </button>
              </div>
              <Button v-if="canEdit" size="sm" :disabled="restoring" @click="confirmRestore">
                <Loader2Icon v-if="restoring" class="animate-spin" />
                {{ t("docs.history.restore") }}
              </Button>
            </div>
          </header>

          <div v-if="!selected" class="text-placeholder m-3 text-[13px]">{{ t("docs.history.pick") }}</div>

          <!-- Preview: the document as it was, read-only. -->
          <div v-else-if="mode === 'preview'" class="min-h-0 flex-1 overflow-y-auto pt-3">
            <p v-if="detailLoading" class="text-placeholder m-3 text-[13px]">{{ t("common.loading") }}</p>
            <!-- eslint-disable-next-line vue/no-v-html -->
            <div v-else class="text-sm leading-[1.7]" v-html="previewHTML" />
          </div>

          <!-- Changes: the two comparisons the server computed. -->
          <div v-else class="min-h-0 flex-1 overflow-y-auto pt-3">
            <p v-if="diffLoading" class="text-placeholder m-3 text-[13px]">{{ t("common.loading") }}</p>
            <template v-else-if="diff">
              <p class="text-placeholder m-0 mb-2.5 flex gap-3 text-xs tabular-nums">
                <span class="text-success">+{{ diff.line_summary.added }}</span>
                <span class="text-destructive">−{{ diff.line_summary.removed }}</span>
                <span v-if="blocks.length">{{ t("docs.history.blocksChanged", { count: blocks.length }) }}</span>
              </p>

              <p v-if="!rows.length" class="text-placeholder m-3 text-[13px]">{{ t("docs.history.identical") }}</p>
              <ol
                v-else
                class="m-0 list-none overflow-x-auto rounded-md border border-[var(--td-component-stroke)] p-0 font-[family-name:var(--td-font-family-medium,ui-monospace,SFMono-Regular,Menlo,monospace)] text-[12.5px] leading-[1.6]"
              >
                <li
                  v-for="(row, index) in rows"
                  :key="index"
                  class="flex gap-2 px-2 [word-break:break-word] whitespace-pre-wrap"
                  :class="
                    row.type === 'gap'
                      ? 'bg-accent text-placeholder justify-center text-[11px]'
                      : row.line.kind === 'added'
                        ? 'bg-[var(--td-success-color-1)]'
                        : row.line.kind === 'removed'
                          ? 'bg-[var(--td-error-color-1)]'
                          : ''
                  "
                >
                  <template v-if="row.type === 'gap'">
                    <span>{{ t("docs.history.skipped", { count: row.skipped }) }}</span>
                  </template>
                  <template v-else>
                    <span class="text-placeholder w-[34px] flex-none text-right tabular-nums select-none">
                      {{ row.line.old_line || "" }}
                    </span>
                    <span class="text-placeholder w-[34px] flex-none text-right tabular-nums select-none">
                      {{ row.line.new_line || "" }}
                    </span>
                    <span class="min-w-0 flex-1">{{ row.line.text }}</span>
                  </template>
                </li>
              </ol>

              <template v-if="blocks.length">
                <h4 class="my-[18px_8px] text-[13px]">{{ t("docs.history.structure") }}</h4>
                <ul class="m-0 list-none p-0 text-[13px]">
                  <li
                    v-for="block in blocks"
                    :key="block.block_id"
                    class="flex gap-2 border-b border-[var(--td-component-stroke)] py-1 last:border-b-0"
                  >
                    <span class="text-placeholder min-w-[56px] flex-none text-[11px]">
                      {{ t(`docs.history.block.${block.kind}`) }}
                    </span>
                    <span class="text-muted-foreground min-w-0 flex-1 truncate">
                      {{ block.text || block.type }}
                    </span>
                  </li>
                </ul>
              </template>
            </template>
          </div>
        </section>
      </div>
    </DrawerContent>
  </Drawer>

  <Dialog :open="restoreConfirm !== null" @update:open="(v: boolean) => !v && (restoreConfirm = null)">
    <DialogContent class="sm:max-w-[440px]">
      <DialogHeader>
        <DialogTitle>{{ t("docs.history.restore") }}</DialogTitle>
      </DialogHeader>
      <DialogDescription>{{ restoreConfirm }}</DialogDescription>
      <DialogFooter>
        <Button variant="outline" @click="restoreConfirm = null">{{ t("common.cancel") }}</Button>
        <Button :disabled="restoring" @click="onConfirmRestore">
          <Loader2Icon v-if="restoring" class="animate-spin" />
          {{ t("docs.history.restore") }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { generateHTML } from "@tiptap/core";
import { MessagePlugin } from "tdesign-vue-next";
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

import { ArrowLeftRightIcon, Loader2Icon, XIcon } from "@lucide/vue";

import {
  getRevision,
  getRevisionDiff,
  listRevisions,
  restoreRevision,
  type DiffView,
  type RevisionView,
} from "@/api/docs";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Drawer, DrawerClose, DrawerContent, DrawerHeader, DrawerTitle } from "@/components/ui/drawer";

import { officialExtensions } from "../editor/extensions";

import { foldDiff, interestingBlocks, type HunkRow } from "./hunks";

const props = defineProps<{
  visible: boolean;
  pageId: string;
  canEdit: boolean;
}>();

const emit = defineEmits<{ close: []; restored: [] }>();
const { t, locale } = useI18n();

const items = ref<RevisionView[]>([]);
const cursor = ref("");
const loading = ref(false);
const loadingMore = ref(false);
const exhausted = ref(false);

const selectedId = ref("");
const compareId = ref("");
const MODES = ["preview", "diff"] as const;
const mode = ref<"preview" | "diff">("preview");
const restoring = ref(false);

const detailLoading = ref(false);
const previewHTML = ref("");
const diff = ref<DiffView | null>(null);
const diffLoading = ref(false);

/** Body text of the restore confirmation; null hides the dialog. */
const restoreConfirm = ref<string | null>(null);

const selected = computed(() => items.value.find((item) => item.id === selectedId.value));
const comparing = computed(() => compareId.value !== "");
const rows = computed<HunkRow[]>(() => (diff.value ? foldDiff(diff.value.lines) : []));
const blocks = computed(() => (diff.value ? interestingBlocks(diff.value.blocks) : []));

const selectedLabel = computed(() => (selected.value ? formatWhen(selected.value.created_at) : ""));
const compareLabel = computed(() => {
  const other = items.value.find((item) => item.id === compareId.value);
  return other ? formatWhen(other.created_at) : "";
});

function formatWhen(iso: string): string {
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) return iso;
  return at.toLocaleString(locale.value, { dateStyle: "medium", timeStyle: "short" });
}

/** Who a version is by; the ids are a fallback for a person since removed. */
function who(item: RevisionView): string {
  const names = (item.editors ?? [])
    .map((editor) => editor.username || editor.email)
    .filter((name): name is string => !!name);
  if (names.length === 0) return t("docs.links.someone");
  if (names.length <= 2) return names.join("、");
  return t("docs.history.andOthers", { name: names[0], count: names.length - 1 });
}

async function load(more = false) {
  if (!props.pageId) return;
  if (more && (exhausted.value || loadingMore.value)) return;
  const target = more ? loadingMore : loading;
  target.value = true;
  try {
    const page = await listRevisions(props.pageId, more ? { cursor: cursor.value } : {});
    items.value = more ? [...items.value, ...page.items] : page.items;
    cursor.value = page.next_cursor ?? "";
    exhausted.value = !page.next_cursor;
    if (!more && items.value.length > 0) select(items.value[0]!, 0);
  } catch (err) {
    void MessagePlugin.error((err as { message?: string })?.message || t("docs.history.loadFailed"));
  } finally {
    target.value = false;
  }
}

/** Pages in more history as the list is scrolled towards its end. */
function onScroll(event: Event) {
  const el = event.target as HTMLElement;
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 80) void load(true);
}

function select(item: RevisionView, index: number) {
  selectedId.value = item.id;
  // Selecting a different version abandons a comparison that was about the
  // old one; keeping it would label the panel with a pair nobody chose.
  if (compareId.value === item.id) compareId.value = "";
  void refresh();
  void index;
}

/** Marks a version as the other side of a comparison. */
function setCompare(item: RevisionView) {
  compareId.value = compareId.value === item.id ? "" : item.id;
  mode.value = "diff";
  void refresh();
}

async function refresh() {
  if (!selected.value) return;
  if (mode.value === "preview") return loadPreview();
  return loadDiff();
}

async function loadPreview() {
  const id = selectedId.value;
  if (!id) return;
  detailLoading.value = true;
  try {
    const detail = await getRevision(props.pageId, id);
    // A later click wins: the answer to an abandoned request must not
    // overwrite the one somebody is waiting for.
    if (selectedId.value !== id) return;
    previewHTML.value = renderDocument(detail.content);
  } catch {
    if (selectedId.value === id) previewHTML.value = "";
  } finally {
    detailLoading.value = false;
  }
}

async function loadDiff() {
  const id = selectedId.value;
  if (!id) return;
  diffLoading.value = true;
  try {
    // The older side is always the one on the left of the comparison: a
    // version is compared *forwards*, to what came after it.
    const answer = compareId.value
      ? await getRevisionDiff(props.pageId, olderOf(id, compareId.value), newerOf(id, compareId.value))
      : await getRevisionDiff(props.pageId, id);
    if (selectedId.value !== id) return;
    diff.value = answer;
  } catch (err) {
    if (selectedId.value === id) {
      diff.value = null;
      void MessagePlugin.error((err as { message?: string })?.message || t("docs.history.loadFailed"));
    }
  } finally {
    diffLoading.value = false;
  }
}

/** Version numbers order the pair; ids alone say nothing about which is older. */
function olderOf(a: string, b: string): string {
  return versionOf(a) <= versionOf(b) ? a : b;
}

function newerOf(a: string, b: string): string {
  return versionOf(a) > versionOf(b) ? a : b;
}

function versionOf(id: string): number {
  return items.value.find((item) => item.id === id)?.version ?? 0;
}

/**
 * Renders a stored document read-only.
 *
 * Markup rather than an editor instance: nothing here is editable, and one
 * editor per preview would be a great deal of machinery to show text. Nothing
 * in the markup can execute — the schema has no script, iframe or style node
 * type — which is the same guarantee the paste path and block references rest
 * on.
 */
function renderDocument(content: unknown): string {
  if (!content) return "";
  try {
    return generateHTML(content as Record<string, unknown>, officialExtensions() as never);
  } catch {
    // A version written by an older build, using a node this one does not
    // have. Showing nothing is better than breaking the panel around it.
    return "";
  }
}

function confirmRestore() {
  if (!selected.value) return;
  const when = formatWhen(selected.value.created_at);
  restoreConfirm.value = t("docs.history.restoreConfirm", { when });
}

// As the old confirm did: the dialog goes away first, and the restore's own
// toast reports how it went.
function onConfirmRestore() {
  restoreConfirm.value = null;
  void doRestore();
}

async function doRestore() {
  const id = selectedId.value;
  if (!id || restoring.value) return;
  restoring.value = true;
  try {
    await restoreRevision(props.pageId, id);
    void MessagePlugin.success(t("docs.history.restored"));
    emit("restored");
    // The restore itself is now the newest version, and the state before it
    // is one entry further down; both belong in the list.
    await load();
  } catch (err) {
    void MessagePlugin.error((err as { message?: string })?.message || t("docs.history.restoreFailed"));
  } finally {
    restoring.value = false;
  }
}

watch(
  () => props.visible,
  (open) => {
    if (!open) return;
    items.value = [];
    cursor.value = "";
    exhausted.value = false;
    selectedId.value = "";
    compareId.value = "";
    mode.value = "preview";
    diff.value = null;
    previewHTML.value = "";
    void load();
  },
);

watch(mode, () => void refresh());
</script>
