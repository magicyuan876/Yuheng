<template>
  <div class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto pb-8" data-testid="knowledge-health">
    <!-- Summary: what is open, by type, and when the knowledge base was last
         checked. The re-check is an administrator's action, because it queues
         work for every document; everybody else does not see the button. -->
    <section class="border-border bg-card flex flex-wrap items-center gap-x-6 gap-y-3 rounded-lg border px-4 py-3">
      <div class="flex min-w-0 flex-col gap-1">
        <div class="text-foreground flex items-center gap-2 text-sm font-semibold">
          <HeartPulseIcon class="text-primary size-4" />
          {{ t("knowledgeHealth.title") }}
        </div>
        <p class="text-placeholder m-0 text-xs">{{ t("knowledgeHealth.subtitle") }}</p>
      </div>
      <template v-if="summary">
        <div class="flex flex-wrap items-center gap-1.5" data-testid="health-summary">
          <Badge :variant="summary.open_total > 0 ? 'default' : 'secondary'">
            {{ t("knowledgeHealth.openCount", { count: summary.open_total }) }}
          </Badge>
          <Badge v-for="entry in openTypeEntries" :key="entry.type" variant="outline">
            {{ findingTypeLabel(entry.type, t) }} · {{ entry.count }}
          </Badge>
        </div>
        <span class="text-placeholder text-xs">
          {{
            summary.last_scan_at
              ? t("knowledgeHealth.lastScan", { time: formatDate(summary.last_scan_at) })
              : t("knowledgeHealth.neverScanned")
          }}
        </span>
      </template>
      <div class="ml-auto flex items-center gap-2">
        <span v-if="queuedCount !== null" class="text-muted-foreground text-xs" data-testid="health-queued">
          {{ t("knowledgeHealth.queued", { count: queuedCount }) }}
        </span>
        <Button
          v-if="canRescan"
          variant="outline"
          size="sm"
          :disabled="scanning || !summary || !summary.enabled || !summary.supported"
          :title="t('knowledgeHealth.recheckHint')"
          data-testid="health-recheck"
          @click="rescan"
        >
          <Loader2Icon v-if="scanning" class="animate-spin" />
          <RefreshCwIcon v-else />
          {{ t("knowledgeHealth.recheck") }}
        </Button>
      </div>
    </section>

    <!-- The wiki's own lint report stays where it is built — the Wiki tab's
         issue drawer — and this view only points at it, so there is one list
         of wiki issues with one set of actions, not two that drift apart. -->
    <section
      v-if="isWiki"
      class="border-border bg-card flex flex-wrap items-center gap-3 rounded-lg border px-4 py-3"
      data-testid="health-wiki"
    >
      <BookOpenIcon class="text-muted-foreground size-4" />
      <div class="flex min-w-0 flex-col">
        <span class="text-foreground text-sm font-medium">{{ t("knowledgeHealth.wiki.title") }}</span>
        <span class="text-placeholder text-xs">
          {{
            wikiPendingIssues > 0
              ? t("knowledgeHealth.wiki.pending", { count: wikiPendingIssues })
              : t("knowledgeHealth.wiki.none")
          }}
        </span>
      </div>
      <Button variant="ghost" size="sm" class="ml-auto" @click="emit('open-wiki-issues')">
        {{ t("knowledgeHealth.wiki.open") }}
        <ArrowRightIcon />
      </Button>
    </section>

    <div v-if="summaryLoading && !summary" class="flex flex-col gap-2">
      <Skeleton class="h-20 w-full" />
      <Skeleton class="h-20 w-full" />
    </div>

    <Empty v-else-if="summaryError" class="border-border border" data-testid="health-error">
      <EmptyHeader>
        <EmptyMedia variant="icon"><CircleAlertIcon /></EmptyMedia>
        <EmptyTitle>{{ t("knowledgeHealth.loadFailed") }}</EmptyTitle>
      </EmptyHeader>
      <EmptyContent>
        <Button variant="outline" size="sm" @click="reloadAll">{{ t("knowledgeHealth.retry") }}</Button>
      </EmptyContent>
    </Empty>

    <!-- The engine cannot answer similarity lookups at all: said first,
         because turning the checks on would not help. -->
    <Empty v-else-if="summary && !summary.supported" class="border-border border" data-testid="health-unsupported">
      <EmptyHeader>
        <EmptyMedia variant="icon"><DatabaseIcon /></EmptyMedia>
        <EmptyTitle>{{ t("knowledgeHealth.unsupported.title") }}</EmptyTitle>
        <EmptyDescription>{{ t("knowledgeHealth.unsupported.description") }}</EmptyDescription>
      </EmptyHeader>
    </Empty>

    <Empty v-else-if="summary && !summary.enabled" class="border-border border" data-testid="health-disabled">
      <EmptyHeader>
        <EmptyMedia variant="icon"><PowerOffIcon /></EmptyMedia>
        <EmptyTitle>{{ t("knowledgeHealth.disabled.title") }}</EmptyTitle>
        <EmptyDescription>{{ t("knowledgeHealth.disabled.description") }}</EmptyDescription>
      </EmptyHeader>
    </Empty>

    <template v-else-if="summary">
      <div class="flex flex-wrap items-center gap-1" role="group" :aria-label="t('knowledgeHealth.statusFilter')">
        <Button
          v-for="option in STATUS_OPTIONS"
          :key="option"
          :variant="statusFilter === option ? 'secondary' : 'ghost'"
          size="sm"
          :aria-pressed="statusFilter === option"
          :data-testid="`health-filter-${option}`"
          @click="setFilter(option)"
        >
          {{ t(STATUS_LABEL_KEYS[option]) }}
        </Button>
        <Separator orientation="vertical" class="mx-1 h-4" />
        <Button
          :variant="mineOnly ? 'secondary' : 'ghost'"
          size="sm"
          :aria-pressed="mineOnly"
          data-testid="health-filter-mine"
          @click="toggleMine"
        >
          <UserRoundIcon />
          {{ t("knowledgeHealth.mineOnly") }}
        </Button>
        <span class="text-placeholder ml-auto text-xs">{{ t("knowledgeHealth.total", { count: total }) }}</span>
      </div>

      <div v-if="listLoading && !items.length" class="flex flex-col gap-2">
        <Skeleton class="h-20 w-full" />
        <Skeleton class="h-20 w-full" />
      </div>

      <Empty v-else-if="!items.length" class="border-border border" data-testid="health-empty">
        <EmptyHeader>
          <EmptyMedia variant="icon"><CircleCheckIcon /></EmptyMedia>
          <EmptyTitle>
            {{ statusFilter === "open" ? t("knowledgeHealth.empty.title") : t("knowledgeHealth.emptyFiltered.title") }}
          </EmptyTitle>
          <EmptyDescription>
            {{
              statusFilter === "open"
                ? t("knowledgeHealth.empty.description")
                : t("knowledgeHealth.emptyFiltered.description")
            }}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>

      <ul v-else class="m-0 flex list-none flex-col gap-2 p-0" :class="listLoading ? 'opacity-60' : ''">
        <FindingItem
          v-for="finding in items"
          :key="finding.id"
          :finding="finding"
          :busy="busyIds.includes(finding.id)"
          :can-edit="canEdit"
          @open-knowledge="(id) => emit('open-knowledge', id)"
          @dismiss="(f, reason) => changeStatus(f, 'dismissed', reason)"
          @reopen="(f) => changeStatus(f, 'open')"
          @assign="assign"
          @supersede="askSupersede"
          @confirm="confirmDocument"
        />
      </ul>

      <div v-if="pageCount > 1" class="flex items-center justify-end gap-2 text-xs" data-testid="health-pager">
        <Button
          variant="ghost"
          size="icon-sm"
          :disabled="page <= 1 || listLoading"
          :aria-label="t('knowledgeHealth.previous')"
          @click="goToPage(page - 1)"
        >
          <ChevronLeftIcon />
        </Button>
        <span class="text-muted-foreground">{{ t("knowledgeHealth.page", { page, pages: pageCount }) }}</span>
        <Button
          variant="ghost"
          size="icon-sm"
          :disabled="page >= pageCount || listLoading"
          :aria-label="t('knowledgeHealth.next')"
          @click="goToPage(page + 1)"
        >
          <ChevronRightIcon />
        </Button>
      </div>
    </template>

    <!-- Superseding takes a document out of the knowledge base, deleting an
         upload outright, so it is confirmed with both titles in view. -->
    <Dialog :open="!!pendingSupersede" @update:open="(open: boolean) => !open && (pendingSupersede = null)">
      <DialogContent class="sm:max-w-[480px]" data-testid="supersede-confirm">
        <DialogHeader>
          <DialogTitle>{{ t("knowledgeHealth.supersedeConfirm.title") }}</DialogTitle>
          <DialogDescription v-if="pendingSupersede">
            {{
              t("knowledgeHealth.supersedeConfirm.body", {
                keep: pendingSupersede.keepTitle,
                retire: pendingSupersede.retireTitle,
              })
            }}
          </DialogDescription>
        </DialogHeader>
        <p class="text-muted-foreground m-0 text-xs">{{ t("knowledgeHealth.supersedeConfirm.how") }}</p>
        <DialogFooter>
          <DialogClose as-child>
            <Button variant="outline" :disabled="superseding">{{ t("common.cancel") }}</Button>
          </DialogClose>
          <Button :disabled="superseding" data-testid="supersede-confirm-ok" @click="confirmSupersede">
            <Loader2Icon v-if="superseding" class="animate-spin" />
            {{ t("knowledgeHealth.supersedeConfirm.ok") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import {
  ArrowRightIcon,
  BookOpenIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  CircleAlertIcon,
  CircleCheckIcon,
  DatabaseIcon,
  HeartPulseIcon,
  Loader2Icon,
  PowerOffIcon,
  RefreshCwIcon,
  UserRoundIcon,
} from "@lucide/vue";

import {
  assignFinding,
  getFindingsSummary,
  listFindings,
  scanFindings,
  supersedeFinding,
  updateFindingStatus,
  type Finding,
  type FindingDismissReason,
  type FindingsSummary,
  type FindingStatusFilter,
} from "@/api/findings";
import { findingTypeLabel } from "@/components/findings/findingDisplay";
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
import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";

import { confirmKnowledgeReviewed } from "@/api/stewardship";
import { useAuthStore } from "@/stores/auth";

import FindingItem from "./FindingItem.vue";

const props = withDefaults(
  defineProps<{
    kbId: string;
    /** Knowledge-base administrators may queue a full re-check. */
    canRescan?: boolean;
    /** Editors of the knowledge base may dismiss, reopen and assign. */
    canEdit?: boolean;
    isWiki?: boolean;
    /** Open wiki lint issues, as the knowledge base screen already polls them. */
    wikiPendingIssues?: number;
  }>(),
  { canRescan: false, canEdit: false, isWiki: false, wikiPendingIssues: 0 },
);

const emit = defineEmits<{
  "open-knowledge": [knowledgeId: string];
  "open-wiki-issues": [];
  /** The open count changed, so a badge elsewhere can follow without a refetch. */
  "summary-change": [summary: FindingsSummary];
}>();

const { t } = useI18n();
const authStore = useAuthStore();

const PAGE_SIZE = 20;
const STATUS_OPTIONS: FindingStatusFilter[] = ["open", "dismissed", "resolved", "all"];
const STATUS_LABEL_KEYS: Record<FindingStatusFilter, string> = {
  open: "knowledgeHealth.status.open",
  dismissed: "knowledgeHealth.status.dismissed",
  resolved: "knowledgeHealth.status.resolved",
  all: "knowledgeHealth.status.all",
};

const summary = ref<FindingsSummary | null>(null);
const summaryLoading = ref(false);
const summaryError = ref(false);

const statusFilter = ref<FindingStatusFilter>("open");
/** Only the findings routed to the caller. */
const mineOnly = ref(false);
const page = ref(1);
const items = ref<Finding[]>([]);
const total = ref(0);
const listLoading = ref(false);
/** Findings whose status change has not come back yet; their buttons wait. */
const busyIds = ref<string[]>([]);

const scanning = ref(false);
const queuedCount = ref<number | null>(null);

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)));
const openTypeEntries = computed(() =>
  Object.entries(summary.value?.open_by_type ?? {})
    .filter(([, count]) => count > 0)
    .map(([type, count]) => ({ type, count })),
);

const listReady = computed(() => !!summary.value && summary.value.enabled && summary.value.supported);

// Every load remembers which request it is, so an answer that arrives after
// the knowledge base, filter or page has moved on is dropped rather than
// painted over the current one.
let listSeq = 0;
let summarySeq = 0;

async function loadSummary() {
  const kbId = props.kbId;
  const seq = ++summarySeq;
  summaryLoading.value = true;
  summaryError.value = false;
  try {
    const next = await getFindingsSummary(kbId);
    if (seq !== summarySeq) return;
    summary.value = next;
    emit("summary-change", next);
  } catch (err) {
    if (seq !== summarySeq) return;
    console.debug("knowledge health: summary failed", err);
    summaryError.value = true;
  } finally {
    if (seq === summarySeq) summaryLoading.value = false;
  }
}

async function loadList() {
  if (!listReady.value) return;
  const kbId = props.kbId;
  const seq = ++listSeq;
  listLoading.value = true;
  try {
    const res = await listFindings(kbId, {
      status: statusFilter.value,
      mine: mineOnly.value,
      page: page.value,
      page_size: PAGE_SIZE,
    });
    if (seq !== listSeq) return;
    items.value = res.items;
    total.value = res.total;
    // A page emptied by dismissals elsewhere steps back to the last one there is.
    if (!res.items.length && res.total > 0 && page.value > 1) {
      page.value = Math.max(1, Math.ceil(res.total / PAGE_SIZE));
      void loadList();
    }
  } catch (err) {
    if (seq !== listSeq) return;
    MessagePlugin.error(errorText(err, t("knowledgeHealth.loadFailed")));
  } finally {
    if (seq === listSeq) listLoading.value = false;
  }
}

async function reloadAll() {
  await loadSummary();
  await loadList();
}

watch(
  () => props.kbId,
  (kbId) => {
    summary.value = null;
    items.value = [];
    total.value = 0;
    page.value = 1;
    statusFilter.value = "open";
    mineOnly.value = false;
    queuedCount.value = null;
    if (kbId) void reloadAll();
  },
  { immediate: true },
);

function setFilter(next: FindingStatusFilter) {
  if (next === statusFilter.value) return;
  statusFilter.value = next;
  page.value = 1;
  items.value = [];
  void loadList();
}

function toggleMine() {
  mineOnly.value = !mineOnly.value;
  page.value = 1;
  items.value = [];
  void loadList();
}

function goToPage(next: number) {
  page.value = Math.min(pageCount.value, Math.max(1, next));
  void loadList();
}

/** Moves one finding by `delta` in the summary's open counts. */
function adjustOpenCount(type: string, delta: number) {
  const s = summary.value;
  if (!s) return;
  const byType = { ...s.open_by_type, [type]: Math.max(0, (s.open_by_type[type] ?? 0) + delta) };
  summary.value = { ...s, open_total: Math.max(0, s.open_total + delta), open_by_type: byType };
}

/**
 * Dismissing and reopening are shown at once and undone if the server says
 * no. Under a status filter the finding leaves the list, since it no longer
 * matches; under "all" it stays and only its status changes. The rollback
 * puts back only this finding, at the place it had, so another change made
 * meanwhile is not thrown away with it.
 */
async function changeStatus(finding: Finding, next: "dismissed" | "open", reason?: FindingDismissReason) {
  if (busyIds.value.includes(finding.id) || finding.status === next) return;
  const previous = finding.status;
  const index = items.value.findIndex((f) => f.id === finding.id);
  const leavesList = statusFilter.value !== "all" && statusFilter.value !== next;
  const openDelta = next === "open" ? 1 : previous === "open" ? -1 : 0;

  if (leavesList) {
    items.value = items.value.filter((f) => f.id !== finding.id);
    total.value = Math.max(0, total.value - 1);
  } else {
    items.value = items.value.map((f) =>
      f.id === finding.id ? { ...f, status: next, resolution: next === "dismissed" ? reason : null } : f,
    );
  }
  adjustOpenCount(finding.type, openDelta);
  busyIds.value = [...busyIds.value, finding.id];

  try {
    const updated = await updateFindingStatus(props.kbId, finding.id, next, reason);
    if (!leavesList && updated?.id) {
      items.value = items.value.map((f) => (f.id === updated.id ? updated : f));
    }
    if (summary.value) emit("summary-change", summary.value);
    MessagePlugin.success(next === "dismissed" ? t("knowledgeHealth.dismissed") : t("knowledgeHealth.reopened"));
    if (leavesList && !items.value.length && total.value > 0) {
      page.value = Math.min(page.value, Math.max(1, Math.ceil(total.value / PAGE_SIZE)));
      void loadList();
    }
  } catch (err) {
    if (leavesList) {
      if (!items.value.some((f) => f.id === finding.id)) {
        const restored = [...items.value];
        restored.splice(Math.min(Math.max(index, 0), restored.length), 0, { ...finding, status: previous });
        items.value = restored;
        total.value += 1;
      }
    } else {
      items.value = items.value.map((f) => (f.id === finding.id ? { ...f, status: previous } : f));
    }
    adjustOpenCount(finding.type, -openDelta);
    MessagePlugin.error(errorText(err, t("knowledgeHealth.updateFailed")));
  } finally {
    busyIds.value = busyIds.value.filter((id) => id !== finding.id);
  }
}

/**
 * Assigning waits for the server rather than guessing: it decides whether the
 * person may take the finding on, and names them in its answer. Under "only
 * mine" a finding given to somebody else leaves the list.
 */
async function assign(finding: Finding, assigneeId: string) {
  if (busyIds.value.includes(finding.id)) return;
  busyIds.value = [...busyIds.value, finding.id];
  try {
    const updated = await assignFinding(props.kbId, finding.id, assigneeId);
    if (mineOnly.value && updated.assignee?.id !== currentUserId()) {
      items.value = items.value.filter((f) => f.id !== finding.id);
      total.value = Math.max(0, total.value - 1);
    } else {
      items.value = items.value.map((f) => (f.id === updated.id ? updated : f));
    }
    MessagePlugin.success(assigneeId ? t("knowledgeHealth.assigned") : t("knowledgeHealth.assignedAutomatic"));
  } catch (err) {
    MessagePlugin.error(errorText(err, t("knowledgeHealth.assignFailed")));
  } finally {
    busyIds.value = busyIds.value.filter((id) => id !== finding.id);
  }
}

const currentUserId = () => authStore.user?.id ?? "";

/**
 * Confirming vouches for the document; the check it schedules resolves the
 * finding a little later. The finding leaves the open list now, since the
 * person has done what it asked.
 */
async function confirmDocument(finding: Finding) {
  if (busyIds.value.includes(finding.id)) return;
  busyIds.value = [...busyIds.value, finding.id];
  try {
    await confirmKnowledgeReviewed(finding.subject.knowledge_id);
    if (statusFilter.value === "open") {
      items.value = items.value.filter((f) => f.id !== finding.id);
      total.value = Math.max(0, total.value - 1);
      adjustOpenCount(finding.type, -1);
      if (summary.value) emit("summary-change", summary.value);
    }
    MessagePlugin.success(t("knowledgeHealth.confirmedPending"));
  } catch (err) {
    MessagePlugin.error(errorText(err, t("knowledgeHealth.confirmFailed")));
  } finally {
    busyIds.value = busyIds.value.filter((id) => id !== finding.id);
  }
}

interface PendingSupersede {
  finding: Finding;
  keepId: string;
  keepTitle: string;
  retireTitle: string;
}
const pendingSupersede = ref<PendingSupersede | null>(null);
const superseding = ref(false);

function askSupersede(finding: Finding, keepId: string) {
  if (!finding.related) return;
  const keepSubject = keepId === finding.subject.knowledge_id;
  const keep = keepSubject ? finding.subject : finding.related;
  const retire = keepSubject ? finding.related : finding.subject;
  pendingSupersede.value = {
    finding,
    keepId,
    keepTitle: keep.title || keep.knowledge_id,
    retireTitle: retire.title || retire.knowledge_id,
  };
}

/**
 * The document that leaves takes every finding naming it along, this one
 * included, so the list and the summary are fetched again rather than
 * patched: which other findings went with it is the server's to know.
 */
async function confirmSupersede() {
  const pending = pendingSupersede.value;
  if (!pending || superseding.value) return;
  superseding.value = true;
  try {
    const res = await supersedeFinding(props.kbId, pending.finding.id, pending.keepId);
    MessagePlugin.success(
      t(res.how === "excluded" ? "knowledgeHealth.supersededExcluded" : "knowledgeHealth.supersededDeleted", {
        title: pending.retireTitle,
      }),
    );
    pendingSupersede.value = null;
    await reloadAll();
  } catch (err) {
    MessagePlugin.error(errorText(err, t("knowledgeHealth.supersedeFailed")));
  } finally {
    superseding.value = false;
  }
}

async function rescan() {
  if (!props.canRescan || scanning.value) return;
  scanning.value = true;
  try {
    const res = await scanFindings(props.kbId);
    queuedCount.value = res.queued;
    MessagePlugin.success(t("knowledgeHealth.queued", { count: res.queued }));
  } catch (err) {
    MessagePlugin.error(errorText(err, t("knowledgeHealth.recheckFailed")));
  } finally {
    scanning.value = false;
  }
}

const formatDate = (iso: string | null | undefined) => {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
};

const errorText = (err: unknown, fallback: string) => {
  const msg = (err as { message?: string } | null)?.message;
  return msg ? `${fallback}: ${msg}` : fallback;
};
</script>
