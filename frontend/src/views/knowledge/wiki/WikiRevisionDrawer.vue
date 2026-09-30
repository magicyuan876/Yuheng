<template>
  <SettingDrawer
    v-model:visible="drawerVisible"
    :title="drawerTitle"
    :icon="HistoryIcon"
    width="760px"
    :min-width="560"
    :max-width="1280"
    storage-key="setting-drawer:width:wiki-revision-history"
    hide-footer
  >
    <!--
      The layout runs edge to edge: the negative margins cancel the drawer
      body's padding, which the old drawer's body override set to zero.
    -->
    <div class="-mx-[18px] -my-4 flex h-full min-h-0 flex-1 items-stretch">
      <!-- Version list -->
      <aside
        class="bg-card box-border flex w-[220px] shrink-0 flex-col overflow-hidden border-r border-[var(--td-component-stroke)] py-3"
      >
        <div class="min-h-0 shrink-0 overflow-y-auto py-0 pr-2 pl-3.5">
          <div
            v-if="currentPage"
            class="cursor-pointer rounded-md py-1.5 pr-2.5 pl-3.5 transition-colors"
            :class="selectedVersion === currentPage.version ? 'bg-accent' : 'hover:bg-accent'"
            @click="selectCurrent"
          >
            <div class="flex min-w-0 items-center gap-2">
              <span
                class="[font-family:var(--td-font-family-mono,monospace)] text-sm leading-5 transition-colors"
                :class="selectedVersion === currentPage.version ? 'text-primary' : 'text-foreground'"
                >v{{ currentPage.version }}</span
              >
              <span
                class="text-xs leading-4 transition-colors"
                :class="selectedVersion === currentPage.version ? 'text-primary' : 'text-placeholder'"
                >{{ t("knowledgeEditor.wikiBrowser.revisionCurrent") }}</span
              >
            </div>
            <div class="text-placeholder mt-0.5 flex min-w-0 items-center justify-between gap-2 text-[11px] leading-4">
              <span>{{ sourceLabel(currentPage.last_edit_source) }}</span>
              <span class="shrink-0 whitespace-nowrap [font-variant-numeric:tabular-nums]">{{
                formatShortTime(currentPage.updated_at)
              }}</span>
            </div>
          </div>

          <div
            v-for="rev in revisions"
            :key="rev.id"
            class="cursor-pointer rounded-md py-1.5 pr-2.5 pl-3.5 transition-colors"
            :class="selectedVersion === rev.version ? 'bg-accent' : 'hover:bg-accent'"
            @click="selectRevision(rev)"
          >
            <div class="flex min-w-0 items-center gap-2">
              <span
                class="[font-family:var(--td-font-family-mono,monospace)] text-sm leading-5 transition-colors"
                :class="selectedVersion === rev.version ? 'text-primary' : 'text-foreground'"
                >v{{ rev.version }}</span
              >
            </div>
            <div class="text-placeholder mt-0.5 flex min-w-0 items-center justify-between gap-2 text-[11px] leading-4">
              <span>{{ sourceLabel(rev.edit_source) }}</span>
              <span class="shrink-0 whitespace-nowrap [font-variant-numeric:tabular-nums]">{{
                formatShortTime(rev.edited_at)
              }}</span>
            </div>
          </div>

          <div v-if="revisions.length < total" class="py-2 pb-1">
            <Button variant="outline" class="w-full font-normal" size="xs" :disabled="loadingList" @click="loadMore">
              <Loader2Icon v-if="loadingList" class="animate-spin" />
              {{ t("knowledgeEditor.wikiBrowser.loadMoreShort") }}
            </Button>
          </div>
        </div>
        <div
          v-if="!loadingList && revisions.length === 0"
          class="text-placeholder flex flex-1 items-center justify-center px-3.5 py-4 text-center text-xs leading-normal"
        >
          {{ t("knowledgeEditor.wikiBrowser.revisionEmpty") }}
        </div>
      </aside>

      <!-- Detail pane -->
      <div class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden px-[18px] pt-3.5 pb-[18px]">
        <template v-if="selectedVersion !== null && canShowDiff">
          <div
            class="mb-3.5 flex items-start justify-between gap-4 border-b border-[var(--td-component-stroke)] pb-3.5"
          >
            <div class="min-w-0 flex-1">
              <div
                class="text-foreground [font-family:var(--td-font-family-mono,monospace)] text-[15px] leading-[1.4] font-semibold"
              >
                {{ versionRangeLabel }}
              </div>
              <div v-if="contextHint" class="text-placeholder mt-1 text-xs leading-normal">{{ contextHint }}</div>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <div
                v-if="viewModeOptions.length > 1"
                class="bg-muted inline-flex items-center gap-0.5 rounded-lg p-0.5"
                role="tablist"
                :aria-label="t('knowledgeEditor.wikiBrowser.revisionViewModeLabel')"
              >
                <button
                  v-for="option in viewModeOptions"
                  :key="option.value"
                  type="button"
                  data-slot="view-mode-tab"
                  class="rounded-md px-2.5 py-[5px] text-xs leading-[1.4] whitespace-nowrap transition-colors"
                  :class="
                    viewMode === option.value
                      ? 'bg-card text-primary font-medium'
                      : 'text-muted-foreground hover:text-foreground bg-transparent'
                  "
                  role="tab"
                  :aria-selected="viewMode === option.value"
                  @click="viewMode = option.value"
                >
                  {{ option.label }}
                </button>
              </div>
              <template v-if="canEdit && selectedRevision">
                <Button
                  variant="ghost"
                  size="xs"
                  class="text-warning hover:text-warning font-normal hover:bg-[var(--td-warning-color-1)] dark:hover:bg-[var(--td-warning-color-1)]"
                  :disabled="reverting"
                  @click="revertConfirmOpen = true"
                >
                  <Loader2Icon v-if="reverting" class="size-3.5 animate-spin" />
                  <Undo2Icon v-else class="size-3.5" />
                  {{ t("knowledgeEditor.wikiBrowser.revertBtn") }}
                </Button>
                <Dialog v-model:open="revertConfirmOpen">
                  <DialogContent class="sm:max-w-[420px]">
                    <DialogHeader>
                      <DialogTitle>{{
                        t("knowledgeEditor.wikiBrowser.revertConfirm", { ver: selectedRevision.version })
                      }}</DialogTitle>
                    </DialogHeader>
                    <DialogFooter>
                      <Button variant="outline" @click="revertConfirmOpen = false">{{ t("common.cancel") }}</Button>
                      <Button
                        class="bg-warning/15 text-warning hover:bg-warning/25"
                        :disabled="reverting"
                        @click="doRevert"
                      >
                        <Loader2Icon v-if="reverting" class="animate-spin" />
                        {{ t("knowledgeEditor.wikiBrowser.revertBtn") }}
                      </Button>
                    </DialogFooter>
                  </DialogContent>
                </Dialog>
              </template>
            </div>
          </div>

          <div
            v-if="loadingDetail || diffLoading"
            class="text-placeholder flex flex-1 items-center justify-center gap-2 text-[13px]"
          >
            <Loader2Icon class="text-primary size-4 animate-spin" />
            <span>{{ t("knowledgeEditor.wikiBrowser.loading") }}</span>
          </div>

          <div v-else-if="viewMode !== 'raw'" class="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto">
            <div v-if="diffSections.length === 0" class="text-placeholder py-6 text-center text-[13px]">
              {{ t("knowledgeEditor.wikiBrowser.revisionDiffEmpty") }}
            </div>
            <template v-for="section in diffSections" :key="section.field">
              <div class="flex flex-col gap-1.5">
                <div class="text-placeholder text-xs font-medium">{{ revisionDiffFieldLabel(section.field) }}</div>
                <pre
                  class="bg-muted m-0 overflow-auto rounded-lg px-3 py-2.5 [font-family:var(--td-font-family-mono,monospace)] text-xs leading-[1.7] break-words whitespace-pre-wrap"
                ><span v-for="(line, idx) in section.lines"
                  :key="`${section.field}-${idx}`"
                  class="block"
                  :class="[
                    line.type === 'add'
                      ? 'bg-[rgba(7,192,95,0.08)] text-foreground'
                      : line.type === 'del'
                        ? 'bg-[rgba(213,73,65,0.06)] text-muted-foreground'
                        : '',
                  ]"
                  >{{ diffPrefix(line.type) }}{{ line.text }}
</span></pre>
              </div>
            </template>
          </div>

          <div v-else-if="selectedRevision" class="min-h-0 flex-1 overflow-auto">
            <pre
              class="bg-muted m-0 rounded-lg px-3.5 py-3 [font-family:var(--td-font-family-mono,monospace)] text-[13px] leading-[1.7] break-words whitespace-pre-wrap"
              >{{ rawRevisionText }}</pre>
          </div>
        </template>

        <div
          v-else
          class="text-placeholder flex flex-1 items-center justify-center p-6 text-center text-[13px] leading-normal"
        >
          {{ t("knowledgeEditor.wikiBrowser.revisionSelectHint") }}
        </div>
      </div>
    </div>
  </SettingDrawer>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import { HistoryIcon, Loader2Icon, Undo2Icon } from "@lucide/vue";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { listWikiRevisions, getWikiRevision, revertWikiPage, type WikiPage, type WikiPageRevision } from "@/api/wiki";
import { diffWikiRevision, type WikiRevisionDiffField, type WikiRevisionSnapshot } from "@/utils/wikiRevisionDiff";

type ViewMode = "incremental" | "cumulative" | "raw";

interface DiffPair {
  fromVersion: number;
  toVersion: number;
  from: WikiRevisionSnapshot;
  to: WikiRevisionSnapshot;
}

const props = defineProps<{
  visible: boolean;
  kbId: string;
  slug: string;
  currentPage: WikiPage | null;
  canEdit?: boolean;
}>();

const emit = defineEmits<{
  (e: "update:visible", visible: boolean): void;
  (e: "reverted", page: WikiPage): void;
}>();

const { t } = useI18n();

const drawerVisible = computed({
  get: () => props.visible,
  set: (val) => emit("update:visible", val),
});

const drawerTitle = computed(() =>
  t("knowledgeEditor.wikiBrowser.historyTitle", { title: props.currentPage?.title || props.slug }),
);

const PAGE_SIZE = 50;

const revisions = ref<WikiPageRevision[]>([]);
const total = ref(0);
const loadingList = ref(false);

const selectedVersion = ref<number | null>(null);
const selectedRevision = ref<WikiPageRevision | null>(null);
const detailContent = ref("");
const loadingDetail = ref(false);
const viewMode = ref<ViewMode>("incremental");
const reverting = ref(false);
const revertConfirmOpen = ref(false);
const diffLoading = ref(false);
const diffPair = ref<DiffPair | null>(null);

const snapshotCache = new Map<number, WikiRevisionSnapshot>();

const currentVersion = computed(() => props.currentPage?.version ?? null);

const isCurrentSelected = computed(
  () => currentVersion.value !== null && selectedVersion.value === currentVersion.value,
);

const canShowDiff = computed(() => {
  if (selectedVersion.value === null || !props.currentPage) return false;
  if (viewMode.value === "raw") return true;
  if (viewMode.value === "cumulative") {
    return !isCurrentSelected.value && selectedVersion.value! < currentVersion.value!;
  }
  // incremental: v1 diffs from empty; later versions diff from the previous one
  return selectedVersion.value! >= 1;
});

const viewModeOptions = computed(() => {
  if (isCurrentSelected.value) return [];
  const ver = selectedVersion.value!;
  const cur = currentVersion.value!;
  const options: Array<{ value: ViewMode; label: string }> = [];
  options.push({ value: "incremental", label: t("knowledgeEditor.wikiBrowser.revisionDiffIncremental") });
  if (ver < cur) {
    options.push({ value: "cumulative", label: t("knowledgeEditor.wikiBrowser.revisionDiffCumulative") });
  }
  options.push({ value: "raw", label: t("knowledgeEditor.wikiBrowser.revisionRaw") });
  return options;
});

const versionRangeLabel = computed(() => {
  if (viewMode.value === "raw") {
    return selectedRevision.value ? `v${selectedRevision.value.version}` : "";
  }
  if (!diffPair.value) return "";
  if (diffPair.value.fromVersion < 1) {
    return t("knowledgeEditor.wikiBrowser.revisionInitialRange", { ver: diffPair.value.toVersion });
  }
  return `v${diffPair.value.fromVersion} → v${diffPair.value.toVersion}`;
});

const contextHint = computed(() => {
  if (viewMode.value === "raw" && selectedRevision.value) {
    return [sourceLabel(selectedRevision.value.edit_source), formatShortTime(selectedRevision.value.edited_at)]
      .filter(Boolean)
      .join(" · ");
  }
  if (viewMode.value === "incremental") {
    if ((isCurrentSelected.value && (currentVersion.value ?? 0) <= 1) || selectedVersion.value === 1) {
      return t("knowledgeEditor.wikiBrowser.revisionInitialCreationHint");
    }
    return isCurrentSelected.value
      ? t("knowledgeEditor.wikiBrowser.revisionLatestChangeHint")
      : t("knowledgeEditor.wikiBrowser.revisionIncrementalHint", { ver: selectedVersion.value ?? 0 });
  }
  if (viewMode.value === "cumulative") {
    return t("knowledgeEditor.wikiBrowser.revisionCumulativeHint");
  }
  return "";
});

const rawRevisionText = computed(() => {
  if (!selectedRevision.value) return detailContent.value;
  const parts: string[] = [];
  if (selectedRevision.value.title) parts.push(selectedRevision.value.title);
  if (selectedRevision.value.summary) {
    if (parts.length) parts.push("");
    parts.push(selectedRevision.value.summary);
  }
  if (detailContent.value) {
    if (parts.length) parts.push("");
    parts.push(detailContent.value);
  }
  return parts.join("\n");
});

const diffSections = computed(() => {
  if (!diffPair.value || viewMode.value === "raw") return [];
  return diffWikiRevision(diffPair.value.from, diffPair.value.to);
});

watch(
  () => [props.visible, props.slug] as const,
  ([visible]) => {
    if (visible && props.slug) {
      resetAndLoad();
    }
  },
);

watch(
  () => viewModeOptions.value,
  (options) => {
    if (options.length === 0) return;
    if (!options.some((option) => option.value === viewMode.value)) {
      viewMode.value = options[0].value;
    }
  },
);

watch(
  () =>
    [
      selectedVersion.value,
      viewMode.value,
      props.currentPage?.version,
      props.currentPage?.content,
      props.currentPage?.title,
      props.currentPage?.summary,
    ] as const,
  () => {
    if (props.visible && viewMode.value !== "raw") {
      void loadDiffPair();
    }
  },
);

function snapshotFromPage(page: WikiPage): WikiRevisionSnapshot {
  return {
    title: page.title || "",
    summary: page.summary || "",
    content: page.content || "",
  };
}

function snapshotFromRevisionData(data: WikiPageRevision, content: string): WikiRevisionSnapshot {
  return {
    title: data.title || "",
    summary: data.summary || "",
    content,
  };
}

async function loadVersionSnapshot(version: number): Promise<WikiRevisionSnapshot> {
  if (!props.currentPage) {
    return { title: "", summary: "", content: "" };
  }
  if (version === props.currentPage.version) {
    return snapshotFromPage(props.currentPage);
  }
  const cached = snapshotCache.get(version);
  if (cached) return cached;
  const res = await getWikiRevision(props.kbId, props.slug, version);
  const snap = snapshotFromRevisionData(res, res.content || "");
  snapshotCache.set(version, snap);
  return snap;
}

let diffRequestSeq = 0;

async function loadDiffPair() {
  const seq = ++diffRequestSeq;
  if (!props.currentPage || selectedVersion.value === null || !canShowDiff.value) {
    diffPair.value = null;
    diffLoading.value = false;
    return;
  }

  const currentVer = props.currentPage.version;
  let fromVer = 0;
  let toVer = 0;

  if (viewMode.value === "incremental") {
    toVer = isCurrentSelected.value ? currentVer : selectedVersion.value!;
    fromVer = toVer - 1;
  } else {
    fromVer = selectedVersion.value!;
    toVer = currentVer;
  }

  if (fromVer < 0 || toVer < 1 || fromVer >= toVer) {
    diffPair.value = null;
    diffLoading.value = false;
    return;
  }

  diffLoading.value = true;
  try {
    const from = fromVer < 1 ? { title: "", summary: "", content: "" } : await loadVersionSnapshot(fromVer);
    if (seq !== diffRequestSeq) return;
    const to = await loadVersionSnapshot(toVer);
    if (seq !== diffRequestSeq) return;
    diffPair.value = { fromVersion: fromVer, toVersion: toVer, from, to };
  } catch (e: any) {
    if (seq !== diffRequestSeq) return;
    diffPair.value = null;
    MessagePlugin.error(e?.message || t("knowledgeEditor.wikiBrowser.revisionLoadFailed"));
  } finally {
    if (seq === diffRequestSeq) diffLoading.value = false;
  }
}

function resetAndLoad() {
  detailRequestSeq++;
  diffRequestSeq++;
  snapshotCache.clear();
  revisions.value = [];
  total.value = 0;
  selectedVersion.value = props.currentPage?.version ?? null;
  selectedRevision.value = null;
  detailContent.value = "";
  loadingDetail.value = false;
  diffPair.value = null;
  diffLoading.value = false;
  viewMode.value = "incremental";
  loadList(0);
  void loadDiffPair();
}

async function loadList(offset: number) {
  loadingList.value = true;
  try {
    const res = await listWikiRevisions(props.kbId, props.slug, { limit: PAGE_SIZE, offset });
    const items: WikiPageRevision[] = res.revisions ?? [];
    if (offset === 0) {
      revisions.value = items;
    } else {
      // Snapshots are created while the user pages through, which shifts the
      // newest-first window. Drop versions we already hold so an overlapping
      // page cannot produce duplicate rows (and duplicate :key values).
      const seen = new Set(revisions.value.map((r) => r.version));
      revisions.value = [...revisions.value, ...items.filter((r) => !seen.has(r.version))];
    }
    total.value = res.total ?? revisions.value.length;
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("knowledgeEditor.wikiBrowser.revisionLoadFailed"));
  } finally {
    loadingList.value = false;
  }
}

function loadMore() {
  if (loadingList.value) return;
  loadList(revisions.value.length);
}

function selectCurrent() {
  detailRequestSeq++;
  selectedVersion.value = props.currentPage?.version ?? null;
  selectedRevision.value = null;
  detailContent.value = "";
  loadingDetail.value = false;
  viewMode.value = "incremental";
}

// Monotonic token guarding the detail fetch: clicking through the list fires
// overlapping requests, and a slow earlier one must not overwrite the body of
// the revision the user is actually looking at.
let detailRequestSeq = 0;

async function selectRevision(rev: WikiPageRevision) {
  const seq = ++detailRequestSeq;
  selectedVersion.value = rev.version;
  selectedRevision.value = rev;
  detailContent.value = "";
  viewMode.value = "incremental";
  loadingDetail.value = true;
  try {
    const res = await getWikiRevision(props.kbId, props.slug, rev.version);
    if (seq !== detailRequestSeq) return;
    selectedRevision.value = { ...rev, ...res };
    detailContent.value = res.content || "";
    snapshotCache.set(rev.version, snapshotFromRevisionData(res, res.content || ""));
  } catch (e: any) {
    if (seq !== detailRequestSeq) return;
    MessagePlugin.error(e?.message || t("knowledgeEditor.wikiBrowser.revisionLoadFailed"));
  } finally {
    if (seq === detailRequestSeq) loadingDetail.value = false;
  }
}

async function doRevert() {
  if (!selectedRevision.value) return;
  reverting.value = true;
  try {
    const res = await revertWikiPage(props.kbId, props.slug, selectedRevision.value.version);
    const updated = res;
    MessagePlugin.success(t("knowledgeEditor.wikiBrowser.revertSuccess", { ver: selectedRevision.value.version }));
    revertConfirmOpen.value = false;
    emit("reverted", updated);
    // Stay open: reload so the just-created snapshot of the pre-revert
    // version shows up and the "current" entry reflects the new version.
    resetAndLoad();
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("knowledgeEditor.wikiBrowser.revertFailed"));
  } finally {
    reverting.value = false;
  }
}

function diffPrefix(type: "same" | "add" | "del"): string {
  return type === "add" ? "+ " : type === "del" ? "- " : "  ";
}

function revisionDiffFieldLabel(field: WikiRevisionDiffField): string {
  switch (field) {
    case "title":
      return t("knowledgeEditor.wikiBrowser.revisionDiffTitle");
    case "summary":
      return t("knowledgeEditor.wikiBrowser.revisionDiffSummary");
    default:
      return t("knowledgeEditor.wikiBrowser.revisionDiffContent");
  }
}

function sourceLabel(source?: string): string {
  switch (source) {
    case "user":
      return t("knowledgeEditor.wikiBrowser.editSourceUser");
    case "revert":
      return t("knowledgeEditor.wikiBrowser.editSourceRevert");
    default:
      return t("knowledgeEditor.wikiBrowser.editSourcePipeline");
  }
}

function formatShortTime(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const now = new Date();
  if (d.toDateString() === now.toDateString()) {
    return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
  }
  return d.toLocaleString(undefined, {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
</script>
