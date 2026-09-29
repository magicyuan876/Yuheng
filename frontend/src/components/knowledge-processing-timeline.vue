<script setup lang="ts">
import { ref, reactive, onMounted, onBeforeUnmount, watch, computed, nextTick } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import { copyWithToast } from "@/utils/clipboard";
import {
  getKnowledgeSpans,
  reparseKnowledge,
  cancelKnowledgeParse,
  getKnowledgeDetails,
} from "@/api/knowledge-base/index";
import {
  groupPostprocessGraphSpans,
  knowledgeSpansPayloadHasTrace,
  summarizePostprocessTasks,
  type KnowledgeTraceNode,
} from "@/utils/knowledgeTrace";
import { resolveTimelineHeaderStatus } from "@/utils/knowledgeProcessingStatus";
import {
  ChevronDownIcon,
  ChevronRightIcon,
  CircleAlertIcon,
  CircleXIcon,
  CopyIcon,
  InfoIcon,
  Loader2Icon,
  RefreshCwIcon,
  XIcon,
} from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import type { KnowledgeProcessOverrides } from "@/types/knowledgeProcess";

type SpanNode = KnowledgeTraceNode;

interface LastError {
  name: string;
  error_code: string;
  error_message: string;
  finished_at?: string;
}

interface SpansResponse {
  knowledge_id: string;
  attempt: number;
  latest_attempt: number;
  current_attempt?: number;
  parse_status: string;
  current_stage?: string;
  trace: SpanNode;
  last_error?: LastError | null;
}

// IMPORTANT: Vue 3 coerces missing Boolean props to `false`, NOT
// `undefined` — so an optional boolean parent omits is indistinguishable
// from one explicitly set to false. That bit us: every parent that
// didn't pass auto-poll silently got polling disabled. Use withDefaults
// to make the intent explicit: polling is ON unless a parent opts out.
const props = withDefaults(
  defineProps<{
    knowledgeId: string;
    parseStatus?: string;
    autoPoll?: boolean;
    compact?: boolean;
    // gracePoll selects the polling rule. See shouldPollNow() for
    // the full semantics:
    //   true  — follow parse_status + any running subspan + a
    //           QUIESCE_GRACE_MS grace window after the tree quiesces.
    //           This is what the user-visible drawer mount wants:
    //           late-arriving subspans (wiki has a 30s debounce, etc.)
    //           surface without forcing a manual refresh.
    //   false — strict mode. Poll ONLY while parse_status itself is
    //           non-terminal; ignore post-pipeline async subspans.
    //           This is what background mounts (the hidden badge
    //           driver in doc-content.vue) want, so they stop firing
    //           /spans the instant parsing finishes — async wiki/
    //           summary work doesn't keep a headless poller alive
    //           after the user closed the trace drawer.
    gracePoll?: boolean;
    /** Document title shown as the drawer primary heading. */
    docTitle?: string;
    /** Show a close control (secondary trace drawer). */
    showClose?: boolean;
  }>(),
  {
    autoPoll: true,
    compact: false,
    gracePoll: true,
    docTitle: "",
    showClose: false,
  },
);

const emit = defineEmits<{
  (e: "update:hasSpans", has: boolean): void;
  (
    e: "update:summary",
    summary: { totalMs: number; status: string; stageIndex: number; stageTotal: number; stageLabel: string },
  ): void;
  (e: "close"): void;
}>();

const { t } = useI18n();

const STAGES = ["docreader", "chunking", "embedding", "multimodal", "postprocess"] as const;
const POLL_INTERVAL_MS = 2000;

const data = ref<SpansResponse | null>(null);
const processOverrides = ref<KnowledgeProcessOverrides | null>(null);
const currentKnowledgeFileType = ref("");
const loading = ref(false);
const refreshing = ref(false);
const selectedAttempt = ref<number | undefined>(undefined);
const expandedRows = ref<Set<string>>(new Set(["__root__"]));
const selectedSpanId = ref<string | null>(null);
const expandedJsonKeys = ref<Set<string>>(new Set());
const nowTick = ref(Date.now());
const detailTab = ref<"overview" | "input" | "output" | "metadata" | "raw">("overview");
const lastFetchedAt = ref<number>(0);
const scrollRef = ref<HTMLElement | null>(null);
// Stages the user has explicitly toggled (in either direction). Once
// a stage is in this set, the auto-expand-when-children-appear rule
// stops applying to it — the user's last action wins forever.
//
// Why this beats the old "firstLoadDone once" approach:
//   - Old logic only auto-expanded on the very first fetch. If a stage
//     had no subspans yet at that moment, it'd stay collapsed even
//     after children arrived later, forcing a manual click.
//   - New logic re-evaluates on every fetch: stages with children get
//     expanded unless the user has spoken. Discoverability + respect
//     for manual collapse, both.
// Tracks every row the user has manually expanded or collapsed (by
// row.key). Auto-expand checks this set so deeper subspans that the
// user explicitly hid stay hidden across polls, instead of being
// reopened by the next fetch's re-evaluation pass.
const userToggledRows = ref<Set<string>>(new Set());
// Tracks consecutive fetch failures so the "更新于" caption can surface
// staleness. When the parse_status is mid-flight but every fetch is
// hitting an error, the loop keeps going silently — without this
// indicator the user sees a spinning auto-refresh icon while the
// caption ages without explanation.
const failedAttempts = ref<number>(0);
const lastFetchOk = ref<boolean>(true);

const attemptStatuses = reactive<Map<number, string>>(new Map());

// ===== Polling, take N+1: brutally simple =====
//
// Prior incarnations tried "smart" polling (self-rescheduling chains,
// watchers, watchdogs, fetchInFlight gates routing through tick
// callbacks). They all had edge cases where the loop silently died
// while the LIVE badge kept showing.
//
// New design: ONE setInterval that fires every tick for the entire
// lifetime of the component. The tick callback checks state and
// either calls fetchSpans or no-ops. No re-arming, no clearing, no
// watchers, no chains. The interval cannot get "stranded" because
// nothing ever stops it until unmount.
//
// A 2-second no-op (3 ref reads) is free. The simplicity is the win.
let pollTimer: ReturnType<typeof setInterval> | null = null;
let nowTimer: ReturnType<typeof setInterval> | null = null;
let unmounted = false;
// Drops overlapping fetches. Does NOT gate the polling decision —
// the tick callback owns scheduling. If fetchInFlight is stuck (e.g.
// network black hole), the worst case is one stuck request blocking
// retries until axios timeout (30s); the loop itself stays alive.
let fetchInFlight = false;

const stages = computed<SpanNode[]>(() => {
  const trace = data.value?.trace;
  const children = trace?.children || [];
  const byName = new Map<string, SpanNode>();
  for (const c of children) {
    if (c && c.kind === "stage" && c.name) {
      byName.set(c.name, c.name === "postprocess" ? groupPostprocessGraphSpans(c) : c);
    }
  }
  return STAGES.map((n) => byName.get(n) || ({ name: n, kind: "stage", status: "pending" } as SpanNode));
});

const currentStageLabel = computed(() => {
  const running = stages.value.find((s) => s.status === "running");
  const failed = stages.value.find((s) => s.status === "failed");
  const target =
    failed || running || stages.value.find((s) => s.status === "pending") || stages.value[stages.value.length - 1];
  return target ? t(`knowledgeStages.stage.${target.name}`) : "";
});

const currentStageIndex = computed(() => {
  const idx = stages.value.findIndex((s) => s.status === "running" || s.status === "failed");
  if (idx >= 0) return idx + 1;
  const traversed = stages.value.filter((s) => s.status === "done" || s.status === "skipped").length;
  return Math.min(traversed + 1, stages.value.length);
});

function formatDuration(ms?: number): string {
  if (ms === undefined || ms === null || isNaN(ms) || ms < 0) return "—";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(2)}s`;
  const mins = Math.floor(ms / 60000);
  const rem = ((ms % 60000) / 1000).toFixed(1);
  return `${mins}m${rem}s`;
}

function formatRelativeTime(ts: number): string {
  if (!ts) return "—";
  const sec = Math.max(0, Math.floor((nowTick.value - ts) / 1000));
  if (sec < 1) return t("knowledgeStages.justNow");
  if (sec < 60) return t("knowledgeStages.secondsAgo", { n: sec });
  const min = Math.floor(sec / 60);
  return t("knowledgeStages.minutesAgo", { n: min });
}

// A skipped stage did not execute. Its tiny bookkeeping interval is not
// processing time, so do not present it as (for example) multimodal time.
function formatSpanDuration(node: SpanNode): string {
  if (node.status === "skipped" || node.status === "pending") return "—";
  return formatDuration(node.duration_ms);
}

function isPolling(status?: string): boolean {
  // finalizing is the post-process fan-out window — subspans
  // (summary / question / graph.chunk[*]) are still actively producing
  // events, so we must keep polling and drawing LIVE.
  return status === "pending" || status === "processing" || status === "finalizing";
}

// Hard-terminal statuses override traceActive: even if child spans were
// left in 'running' state (e.g. a cancel raced ahead of a worker's
// FailSpan), the parse pipeline is definitively done for this attempt
// and we should stop polling instead of refreshing forever.
function isHardTerminal(status?: string): boolean {
  return status === "cancelled" || status === "failed" || status === "completed";
}

// True if any node in the trace is still running/pending. Crucial for
// postprocess — its subspans (summary/question/graph.chunk[*]) run
// asynchronously AFTER the main pipeline closes, so parse_status
// flips to 'completed' while there's plenty of work still happening.
// Without checking the tree we'd stop polling at that moment and
// strand the user staring at half-rendered postprocess until they
// manually refresh.
function spanTreeActive(node?: SpanNode): boolean {
  if (!node) return false;
  const s = node.status;
  if (s === "running" || s === "pending") return true;
  const kids = node.children || [];
  for (let i = 0; i < kids.length; i++) {
    if (spanTreeActive(kids[i])) return true;
  }
  return false;
}

const traceActive = computed<boolean>(() => spanTreeActive(data.value?.trace));

// Drives BOTH the LIVE badge AND polling. Single source of truth:
//   1. If we have data: still live while parse_status is polling
//      OR the span tree has any running descendant.
//   2. If we don't have data yet (initial paint): trust the parent's
//      parseStatus hint so the UI shows LIVE immediately.
const isLive = computed<boolean>(() => {
  if (data.value) {
    // Hard-terminal parse_status wins over a stale traceActive: cancel
    // and irrecoverable failure can leave child spans stranded as
    // 'running' (worker process died, cancel raced FailSpan, etc.),
    // and we must NOT keep polling forever on those.
    if (isHardTerminal(data.value.parse_status)) return false;
    return isPolling(data.value.parse_status) || traceActive.value;
  }
  if (isHardTerminal(props.parseStatus)) return false;
  return isPolling(props.parseStatus);
});

// Walk every node in the tree and return the freshest updated_at /
// finished_at timestamp we can see. Used by the quiescent grace
// window below to decide "did this trace finish recently, or is it
// just an old completed one we shouldn't waste polls on?"
function spanTreeLastActivity(node?: SpanNode): number {
  if (!node) return 0;
  let max = 0;
  const stamps: (string | null | undefined)[] = [
    (node as any).updated_at,
    node.finished_at,
    node.started_at,
    (node as any).created_at,
  ];
  for (const s of stamps) {
    const t = parseTime(s || undefined);
    if (t !== null && t > max) max = t;
  }
  const kids = node.children || [];
  for (let i = 0; i < kids.length; i++) {
    const t = spanTreeLastActivity(kids[i]);
    if (t > max) max = t;
  }
  return max;
}

// Async post-pipeline tasks (summary, question, graph.chunk[*],
// wiki) open their postprocess.* subspans AFTER the parse pipeline
// has finalised — wiki in particular fires 30s after enqueue thanks
// to wikiIngestDelay. At that exact moment parse_status is already
// 'completed' AND every existing span is 'done', so isLive flips to
// false and polling stops. The user is then stranded watching a
// stale tree until they hit refresh.
//
// QUIESCE_GRACE_MS keeps the poll loop alive for a bounded window
// after the trace appears quiescent so late subspans surface
// automatically. Sized generously past wikiIngestDelay (30s) plus a
// typical ingest run, but short enough that opening an
// already-completed knowledge from days ago doesn't waste many
// polls before the loop falls silent.
const QUIESCE_GRACE_MS = 2 * 60 * 1000;

const lastTraceActivityAt = computed<number>(() => spanTreeLastActivity(data.value?.trace));

const isWithinQuiesceGrace = computed<boolean>(() => {
  if (isLive.value) return false;
  const last = lastTraceActivityAt.value;
  if (!last) return false;
  return nowTick.value - last < QUIESCE_GRACE_MS;
});

// shouldPollNow encodes the per-tick "do I fetch?" decision. Split
// from isLive (which also drives the LIVE badge — its semantics are
// "trace is actually running") so the polling rules can diverge from
// the badge rules without entangling them.
//
// Two modes, selected by the gracePoll prop:
//
//   gracePoll = true  (default — for the visible drawer mount):
//     Follow the trace through everything the user might want to see
//     live: the main parse pipeline (parse_status), any running
//     subspan (traceActive), AND the QUIESCE_GRACE_MS window after
//     the tree quiesces (so wiki ingest's 30s-debounced subspan
//     surfaces on its own without a manual refresh).
//
//   gracePoll = false (for background mounts e.g. doc-content's
//     hidden badge driver):
//     Strict mode. Poll ONLY while the parse pipeline itself is
//     non-terminal. Post-pipeline async work (wiki / summary /
//     question / graph subspans) is intentionally ignored —
//     otherwise the hidden mount would keep firing /spans every
//     2s until those finish, even though the user has closed the
//     trace drawer and is only looking at the header badge (whose
//     job is "is parsing done yet?", not "is async post-work done
//     yet?"). The next user-initiated drawer-open will get a fresh
//     fetch via onMounted, picking up any post-pipeline progress.
function shouldPollNow(): boolean {
  if (!data.value) return isPolling(props.parseStatus);
  if (props.gracePoll) {
    return isLive.value || isWithinQuiesceGrace.value;
  }
  return isPolling(data.value.parse_status);
}

async function fetchSpans(opts: { manual?: boolean } = {}) {
  if (!props.knowledgeId) return;
  if (fetchInFlight) return;
  fetchInFlight = true;
  if (opts.manual) refreshing.value = true;
  if (!data.value) loading.value = true;
  let attemptOk = false;
  try {
    const res: any = await getKnowledgeSpans(props.knowledgeId, selectedAttempt.value);
    if (res?.success && res.data) {
      data.value = res.data as SpansResponse;
      attemptOk = true;
      if (selectedAttempt.value === undefined) {
        selectedAttempt.value = data.value.attempt;
      }
      // Auto-expand rule, applied on EVERY fetch and walked over
      // EVERY level of the tree (stage + subspan + sub-subspan, …):
      //   - Row has children + user hasn't toggled it → expand
      //   - Row has no children → leave whatever state it was in
      //   - User has toggled the row (either direction) → don't touch
      // Walking the full depth (not just stages) is what makes deep
      // subspans like postprocess.wiki — whose own children like
      // postprocess.wiki.extract / .summary / .classify / .page[*]
      // appear later in the run — surface automatically. Without this
      // the user would click the chevron on postprocess.wiki and see
      // nothing change, because the row would default to collapsed
      // and the auto-expand pass would skip it.
      const expanded = new Set(expandedRows.value);
      expanded.add("__root__");
      const autoExpand = (n: SpanNode) => {
        const key = n.span_id || `stage:${n.name}`;
        const kids = n.children || [];
        if (kids.length > 0 && !userToggledRows.value.has(key)) {
          expanded.add(key);
        }
        for (const c of kids) autoExpand(c);
      };
      for (const stage of data.value.trace?.children || []) autoExpand(stage);
      expandedRows.value = expanded;
      const latestAttempt = data.value.latest_attempt || data.value.attempt || 0;
      const tabStatus =
        resolveTimelineHeaderStatus({
          parseStatus: data.value.parse_status,
          traceStatus: data.value.trace?.status,
          isLatestAttempt: data.value.attempt === latestAttempt,
        }) || "running";
      attemptStatuses.set(data.value.attempt, tabStatus);
      ensureAttemptStatuses();
      emit("update:hasSpans", knowledgeSpansPayloadHasTrace(data.value));
    } else {
      emit("update:hasSpans", false);
    }
  } catch (e) {
    // Surface the error in the console — silent failures here is
    // exactly what hid the polling-stalled bug from us before.
    console.warn("[KnowledgeTimeline] fetchSpans failed", e);
    emit("update:hasSpans", false);
  } finally {
    // Track every attempt, not just successful ones — otherwise a
    // failing endpoint would leave "更新于 X 秒前" frozen forever while
    // the spinner spins. Pair with failedAttempts to render a "fetch
    // failed" hint when consecutive errors pile up.
    lastFetchedAt.value = Date.now();
    lastFetchOk.value = attemptOk;
    if (attemptOk) {
      failedAttempts.value = 0;
    } else {
      failedAttempts.value += 1;
    }
    loading.value = false;
    refreshing.value = false;
    fetchInFlight = false;
  }
}

function ensureAttemptStatuses() {
  const latest = data.value?.latest_attempt || 0;
  if (latest <= 1) return;
  for (let n = 1; n <= latest; n++) {
    if (attemptStatuses.has(n)) continue;
    getKnowledgeSpans(props.knowledgeId, n)
      .then((res: any) => {
        if (res?.success && res.data?.trace) {
          attemptStatuses.set(
            n,
            resolveTimelineHeaderStatus({
              parseStatus: res.data.parse_status,
              traceStatus: res.data.trace?.status,
              isLatestAttempt: n === latest,
            }) || "running",
          );
        }
      })
      .catch(() => {});
  }
}

function localizedErrorTitle(code?: string): string {
  if (!code) return "";
  const key = `knowledgeStages.errorCode.${code}`;
  const localized = t(key);
  return localized === key ? code : localized;
}

function localizedErrorSuggestion(code?: string): string {
  if (!code) return "";
  const key = `knowledgeStages.errorCode.${code}_SUGGESTION`;
  const localized = t(key);
  if (localized !== key) return localized;
  const fallback = t("knowledgeStages.errorCode.UNKNOWN_SUGGESTION");
  return fallback === "knowledgeStages.errorCode.UNKNOWN_SUGGESTION" ? "" : fallback;
}

async function copyValue(value: any) {
  const text = typeof value === "string" ? value : JSON.stringify(value, null, 2);
  await copyWithToast(text, "knowledgeStages.copied", "knowledgeStages.copyDetails");
}

async function copySpan(node: SpanNode) {
  await copyValue(node);
}

async function onRetry() {
  if (!props.knowledgeId) return;
  try {
    await reparseKnowledge(props.knowledgeId);
    selectedAttempt.value = undefined;
    attemptStatuses.clear();
    selectedSpanId.value = null;
    await fetchSpans();
  } catch {
    // ignore
  }
}

async function onManualRefresh() {
  if (refreshing.value || loading.value) return;
  await fetchSpans({ manual: true });
}

const cancelling = ref(false);
// The stop-parse confirmation is a controlled popover so that both of its
// buttons can close it; the confirm button also starts the cancel.
const cancelConfirmOpen = ref(false);

function confirmCancelParse() {
  cancelConfirmOpen.value = false;
  void onCancelParseConfirm();
}

// The processing-config card opened on hover (it was a hover-triggered
// TDesign popup). Reka's Popover only opens on click, so hover is driven
// by hand: entering the trigger or the card opens it, and leaving either
// closes it after a short grace period, long enough for the pointer to
// travel from the button into the card without the card vanishing.
const processConfigOpen = ref(false);
let processConfigCloseTimer: ReturnType<typeof setTimeout> | null = null;

function openProcessConfig() {
  if (processConfigCloseTimer) {
    clearTimeout(processConfigCloseTimer);
    processConfigCloseTimer = null;
  }
  processConfigOpen.value = true;
}

function scheduleCloseProcessConfig() {
  if (processConfigCloseTimer) clearTimeout(processConfigCloseTimer);
  processConfigCloseTimer = setTimeout(() => {
    processConfigOpen.value = false;
    processConfigCloseTimer = null;
  }, 150);
}

// Mirrors the backend CancelKnowledgeParse gate (pending / processing /
// finalizing). Uses the freshest status we have: live span data first,
// the parent's hint before the first fetch lands.
const canCancelParse = computed<boolean>(() => {
  const status = data.value?.parse_status ?? props.parseStatus;
  return isPolling(status);
});

async function onCancelParseConfirm() {
  if (cancelling.value) return;
  const id = props.knowledgeId;
  if (!id) return;
  cancelling.value = true;
  try {
    await cancelKnowledgeParse(id);
    MessagePlugin.success(t("knowledgeBase.cancelParseSubmitted"));
    await fetchSpans({ manual: true });
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("knowledgeBase.cancelParseFailed"));
  } finally {
    cancelling.value = false;
  }
}

function onAttemptChange(n: number) {
  if (Number.isNaN(n)) return;
  selectedAttempt.value = n;
  selectedSpanId.value = null;
  // New attempt: forget per-row user choices so the auto-expand
  // rule re-evaluates cleanly against the new attempt's tree.
  userToggledRows.value = new Set();
  expandedRows.value = new Set(["__root__"]);
  fetchSpans();
}

watch(
  () => props.knowledgeId,
  () => {
    selectedAttempt.value = undefined;
    data.value = null;
    processOverrides.value = null;
    currentKnowledgeFileType.value = "";
    expandedRows.value = new Set(["__root__"]);
    selectedSpanId.value = null;
    attemptStatuses.clear();
    userToggledRows.value = new Set();
    fetchSpans();
    fetchProcessOverrides();
  },
);

function onKeydown(ev: KeyboardEvent) {
  if (ev.key === "Escape" && selectedSpanId.value) {
    selectedSpanId.value = null;
  }
}

async function fetchProcessOverrides() {
  if (props.compact || !props.knowledgeId) return;
  try {
    const res: any = await getKnowledgeDetails(props.knowledgeId);
    if (res?.success && res.data) {
      processOverrides.value = res.data.metadata?.process_overrides ?? null;
      currentKnowledgeFileType.value = normalizeFileType(
        res.data.file_type || getFileTypeFromName(res.data.file_name || res.data.title || ""),
      );
    }
  } catch {
    processOverrides.value = null;
    currentKnowledgeFileType.value = "";
  }
}

onMounted(() => {
  fetchSpans();
  fetchProcessOverrides();
  // One permanent interval for the entire component lifetime. The
  // tick decides whether to actually fetch — no clearing, no
  // re-arming, no watchers wired into it. If this interval ever
  // stops firing, the entire JS event loop is wedged (in which case
  // nothing else would work either).
  if (props.autoPoll) {
    pollTimer = setInterval(() => {
      if (unmounted) return;
      if (fetchInFlight) return;
      if (!shouldPollNow()) return;
      fetchSpans();
    }, POLL_INTERVAL_MS);
  }
  nowTimer = setInterval(() => {
    nowTick.value = Date.now();
  }, 1000);
  window.addEventListener("keydown", onKeydown);
});

onBeforeUnmount(() => {
  unmounted = true;
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
  if (nowTimer) {
    clearInterval(nowTimer);
    nowTimer = null;
  }
  window.removeEventListener("keydown", onKeydown);
  if (processConfigCloseTimer) {
    clearTimeout(processConfigCloseTimer);
    processConfigCloseTimer = null;
  }
});

// ---------- Waterfall helpers ----------

function rowKey(node: SpanNode, fallback: string): string {
  return node.span_id || fallback;
}

function parseTime(s?: string | null): number | null {
  if (!s) return null;
  const t = Date.parse(s);
  return Number.isNaN(t) ? null : t;
}

function nodeStart(node: SpanNode): number | null {
  return parseTime(node.started_at || undefined);
}

function nodeEnd(node: SpanNode): number | null {
  const e = parseTime(node.finished_at || undefined);
  if (e !== null) return e;
  const s = nodeStart(node);
  if (s !== null && typeof node.duration_ms === "number" && node.duration_ms > 0) {
    return s + node.duration_ms;
  }
  return null;
}

function collectStarts(node: SpanNode | undefined, out: number[]) {
  if (!node) return;
  const s = nodeStart(node);
  if (s !== null) out.push(s);
  for (const c of node.children || []) collectStarts(c, out);
}

function collectEnds(node: SpanNode | undefined, out: number[]) {
  if (!node) return;
  const e = nodeEnd(node);
  if (e !== null) out.push(e);
  for (const c of node.children || []) collectEnds(c, out);
}

const traceRoot = computed<SpanNode | null>(() => {
  const trace = data.value?.trace;
  if (!trace) return null;
  const synthChildren: SpanNode[] = stages.value.map((stage) => stage);
  return {
    ...trace,
    name: trace.name || "knowledge_processing",
    kind: trace.kind || "root",
    children: synthChildren,
  };
});

const t0 = computed<number | null>(() => {
  const root = traceRoot.value;
  if (!root) return null;
  const direct = nodeStart(root);
  if (direct !== null) return direct;
  const all: number[] = [];
  collectStarts(root, all);
  if (all.length === 0) return null;
  return Math.min(...all);
});

const tEnd = computed<number | null>(() => {
  const root = traceRoot.value;
  if (!root) return null;
  const direct = parseTime(root.finished_at || undefined);
  const all: number[] = [];
  collectEnds(root, all);
  let candidate: number | null = direct;
  if (all.length > 0) {
    const max = Math.max(...all);
    candidate = candidate === null ? max : Math.max(candidate, max);
  }
  // Extend the right edge to "now" whenever the trace is still
  // actively producing spans — this covers both parse_status mid-flight
  // AND the postprocess-async case where the top-level status closes
  // but subspans keep ticking. Both conditions are captured by isLive.
  if (isLive.value) {
    const now = nowTick.value;
    candidate = candidate === null ? now : Math.max(candidate, now);
  }
  return candidate;
});

const totalMs = computed<number>(() => {
  if (t0.value === null || tEnd.value === null) return 0;
  // The trace's own duration_ms only covers the parsing pipeline up to
  // FinalizeAttempt. Async post-processing subspans (summary / question /
  // graph) keep producing rows AFTER the root closes — so the time axis
  // must scale to the latest descendant end, otherwise their bars get
  // clipped past the right edge. Take the max of (root duration, observed
  // span tail) regardless of polling state.
  const observed = Math.max(0, tEnd.value - t0.value);
  const traceDur = data.value?.trace?.duration_ms;
  if (typeof traceDur === "number" && traceDur > 0) {
    return Math.max(traceDur, observed);
  }
  return observed;
});

const showRuler = computed(() => totalMs.value >= 50);

const rulerTicks = computed(() => {
  if (!showRuler.value) return [] as { left: string; label: string }[];
  const total = totalMs.value;
  const fmt = (ms: number) => formatDuration(ms);
  return [
    { left: "0%", label: fmt(0) },
    { left: "25%", label: fmt(total * 0.25) },
    { left: "50%", label: fmt(total * 0.5) },
    { left: "75%", label: fmt(total * 0.75) },
    { left: "100%", label: fmt(total) },
  ];
});

// "Now" position on the waterfall scale, used to draw the live cursor
// while polling so the user can see time advancing even when the running
// stage's bar grows slowly toward the right edge.
const nowMarkerPct = computed<number | null>(() => {
  if (!isLive.value || !t0.value || !totalMs.value) return null;
  const pct = ((nowTick.value - t0.value) / totalMs.value) * 100;
  return Math.max(0, Math.min(100, pct));
});

interface FlatRow {
  key: string;
  depth: number;
  node: SpanNode;
  hasChildren: boolean;
  isRoot: boolean;
  isStage: boolean;
  parentKey?: string;
}

const flatRows = computed<FlatRow[]>(() => {
  const root = traceRoot.value;
  if (!root) return [];
  const rows: FlatRow[] = [];

  const rootKey = rowKey(root, "__root__");
  rows.push({
    key: rootKey,
    depth: 0,
    node: root,
    hasChildren: (root.children || []).length > 0,
    isRoot: true,
    isStage: false,
  });

  for (const stage of root.children || []) {
    const stageKey = rowKey(stage, `stage:${stage.name}`);
    const stageChildren = stage.children || [];
    rows.push({
      key: stageKey,
      depth: 1,
      node: stage,
      hasChildren: stageChildren.length > 0,
      isRoot: false,
      isStage: true,
      parentKey: rootKey,
    });
    if (!expandedRows.value.has(stageKey)) continue;

    const walk = (n: SpanNode, depth: number, idxPath: string, parentKey: string) => {
      const key = rowKey(n, `${idxPath}:${n.name}`);
      const kids = n.children || [];
      rows.push({
        key,
        depth,
        node: n,
        hasChildren: kids.length > 0,
        isRoot: false,
        isStage: false,
        parentKey,
      });
      // Honour the expand/collapse state for subspans too — without
      // this gate, clicking the chevron on a subspan (e.g.
      // postprocess.wiki) only rotated the icon but the children
      // were always rendered, so the toggle looked broken.
      if (!expandedRows.value.has(key)) return;
      kids.forEach((c, i) => walk(c, depth + 1, `${idxPath}/${i}`, key));
    };
    stageChildren.forEach((c, i) => walk(c, 2, `${stageKey}/${i}`, stageKey));
  }

  return rows;
});

const selectedRow = computed<FlatRow | null>(() => {
  const id = selectedSpanId.value;
  if (!id) return null;
  return flatRows.value.find((r) => r.key === id) || null;
});

const detailOpen = computed(() => selectedSpanId.value !== null && selectedRow.value !== null);

function barStyle(node: SpanNode): Record<string, string> {
  const total = totalMs.value;
  if (!total || t0.value === null) return { display: "none" };
  const start = nodeStart(node);
  if (start === null) return { display: "none" };
  // For a span with no recorded finished_at, use "now" as the end
  // whenever it's plausibly still running — either the trace overall
  // is live, or this individual span's status says it's in flight.
  // Without the second clause, postprocess subspans that survived past
  // parse_status='completed' would collapse to zero width.
  const liveBar = isLive.value || node.status === "running" || node.status === "pending";
  const end = nodeEnd(node) ?? (liveBar ? nowTick.value : start);
  const leftPct = ((start - t0.value) / total) * 100;
  const widthPct = Math.max(0.4, ((end - start) / total) * 100);
  return {
    left: `${Math.max(0, Math.min(100, leftPct))}%`,
    width: `${Math.min(100 - Math.max(0, leftPct), widthPct)}%`,
  };
}

// Wrapping outline bar — when a span's children extend past the parent's
// own finished_at (typical for postprocess: stage closes in ~9ms but its
// async summary/question subspans run for tens of seconds), we render a
// faint outline from the parent's start to the latest descendant end.
// This makes "this stage's downstream work took N seconds total" visible
// at a glance without conflating it with the stage's self-duration.
function descendantMaxEnd(node: SpanNode): number | null {
  const ends: number[] = [];
  for (const c of node.children || []) {
    collectEnds(c, ends);
  }
  if (ends.length === 0) return null;
  return Math.max(...ends);
}

function wrapStyle(node: SpanNode): Record<string, string> | null {
  const total = totalMs.value;
  if (!total || t0.value === null) return null;
  const start = nodeStart(node);
  if (start === null) return null;
  const selfEnd = nodeEnd(node) ?? start;
  const childEnd = descendantMaxEnd(node);
  if (childEnd === null) return null;
  // Only render the wrapping bar when descendants extend at least 50ms
  // past the parent — otherwise the outline is indistinguishable from
  // the solid self-bar and only adds visual noise.
  if (childEnd - selfEnd < 50) return null;
  const leftPct = ((start - t0.value) / total) * 100;
  const widthPct = Math.max(0.4, ((childEnd - start) / total) * 100);
  return {
    left: `${Math.max(0, Math.min(100, leftPct))}%`,
    width: `${Math.min(100 - Math.max(0, leftPct), widthPct)}%`,
  };
}

function wrapDurationMs(node: SpanNode): number {
  const start = nodeStart(node);
  const childEnd = descendantMaxEnd(node);
  if (start === null || childEnd === null) return 0;
  return Math.max(0, childEnd - start);
}

function barOffsetPct(node: SpanNode): number | null {
  const total = totalMs.value;
  if (!total || t0.value === null) return null;
  const start = nodeStart(node);
  if (start === null) return null;
  return Math.max(0, Math.min(100, ((start - t0.value) / total) * 100));
}

function barOffsetMs(node: SpanNode): number {
  const start = nodeStart(node);
  if (start === null || t0.value === null) return 0;
  return Math.max(0, start - t0.value);
}

function liveElapsedMs(node: SpanNode): number {
  const s = nodeStart(node);
  if (s === null) return 0;
  return Math.max(0, nowTick.value - s);
}

function isPlaceholder(node: SpanNode): boolean {
  return !node.span_id && !node.started_at;
}

function isRowExpanded(key: string): boolean {
  return expandedRows.value.has(key);
}

function treeToggleAriaLabel(row: FlatRow): string {
  if (!row.hasChildren || row.isRoot) return "";
  return isRowExpanded(row.key) ? t("knowledgeStages.collapseBranch") : t("knowledgeStages.expandBranch");
}

function scrollRowIntoView(key: string) {
  nextTick(() => {
    const root = scrollRef.value;
    if (!root) return;
    const safeKey = typeof CSS !== "undefined" && CSS.escape ? CSS.escape(key) : key.replace(/"/g, '\\"');
    const el = root.querySelector(`.kp-row[data-span-key="${safeKey}"]`);
    el?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  });
}

function toggleTree(row: FlatRow, ev?: MouseEvent) {
  if (ev) ev.stopPropagation();
  if (!row.hasChildren) return;
  const wasExpanded = expandedRows.value.has(row.key);
  const next = new Set(expandedRows.value);
  if (next.has(row.key)) next.delete(row.key);
  else next.add(row.key);
  expandedRows.value = next;
  // Record the manual toggle at every depth so the next poll's
  // auto-expand pass leaves this row alone. Without this, a user
  // who collapsed a noisy subspan (e.g. postprocess.wiki with a
  // dozen page[*] children) would see it pop back open on the
  // next 2s tick.
  const touched = new Set(userToggledRows.value);
  touched.add(row.key);
  userToggledRows.value = touched;
  // Expanding via chevron should also surface the detail panel for
  // this stage/span — otherwise the name column only toggles the tree.
  if (!wasExpanded && next.has(row.key)) {
    selectRow(row);
  }
}

function selectRow(row: FlatRow) {
  if (selectedSpanId.value === row.key) {
    return;
  }
  selectedSpanId.value = row.key;
  detailTab.value = "overview";
  scrollRowIntoView(row.key);
}

function closeDetail() {
  selectedSpanId.value = null;
}

function isObjectWithKeys(v: any): boolean {
  return v && typeof v === "object" && !Array.isArray(v) && Object.keys(v).length > 0;
}

function hasContent(v: any): boolean {
  if (v === null || v === undefined || v === "") return false;
  if (Array.isArray(v)) return v.length > 0;
  if (typeof v === "object") return Object.keys(v).length > 0;
  return true;
}

function prettyJSON(v: any): string {
  try {
    return JSON.stringify(v, null, 2);
  } catch {
    return String(v);
  }
}

function localizedStatus(status: string): string {
  const key = `knowledgeStages.status.${status}`;
  const localized = t(key);
  return localized === key ? status : localized;
}

function rowLabel(row: FlatRow): string {
  if (row.isRoot) return t("knowledgeStages.root");
  if (row.isStage) return t(`knowledgeStages.stage.${row.node.name}`);
  if (row.node.name === "postprocess.graph") return t("knowledgeStages.processConfig.graph");
  const graphChunk = /^postprocess\.graph\.chunk\[(\d+)\]$/.exec(row.node.name);
  if (graphChunk) return `${t("knowledgeStages.processConfig.graph")} #${Number(graphChunk[1]) + 1}`;
  return row.node.name;
}

function rowKindLabel(row: FlatRow): string {
  if (row.isRoot) return "root";
  if (row.isStage) return "stage";
  return row.node.kind || "span";
}

function jsonExpandKey(section: string, key: string): string {
  return `${selectedSpanId.value || ""}::${section}::${key}`;
}

function toggleJsonKey(section: string, key: string) {
  const k = jsonExpandKey(section, key);
  const next = new Set(expandedJsonKeys.value);
  if (next.has(k)) next.delete(k);
  else next.add(k);
  expandedJsonKeys.value = next;
}

function isJsonExpanded(section: string, key: string): boolean {
  return expandedJsonKeys.value.has(jsonExpandKey(section, key));
}

function formatTime(s?: string | null): string {
  if (!s) return "—";
  const ts = Date.parse(s);
  if (Number.isNaN(ts)) return s;
  const d = new Date(ts);
  const ms = String(d.getMilliseconds()).padStart(3, "0");
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  const ss = String(d.getSeconds()).padStart(2, "0");
  // Date prefix: omit year when same as current year to keep the row
  // compact, but always show month+day so traces from yesterday/last
  // week aren't ambiguous. The full ISO date is preserved in the
  // tooltip via the original string.
  const now = new Date();
  const yyyy = d.getFullYear();
  const mo = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  const datePart = yyyy === now.getFullYear() ? `${mo}-${dd}` : `${yyyy}-${mo}-${dd}`;
  return `${datePart} ${hh}:${mm}:${ss}.${ms}`;
}

function humanizeKey(k: string): string {
  return k
    .replace(/[_-]+/g, " ")
    .replace(/\s+/g, " ")
    .trim()
    .replace(/\b([a-z])/g, (_, c: string) => c.toUpperCase());
}

interface KvEntry {
  key: string;
  label: string;
  kind: "scalar" | "bool" | "array" | "object";
  display: string;
  raw: any;
  // True for short payloads — the panel skips the click-to-expand
  // affordance and renders the JSON inline directly so users see the
  // values without an extra click. Long payloads stay folded so the
  // panel doesn't blow up vertically when an output has hundreds of
  // entries.
  defaultExpanded: boolean;
}

function buildKvEntries(obj: any): KvEntry[] {
  if (!isObjectWithKeys(obj)) return [];
  const entries: KvEntry[] = [];
  for (const [key, value] of Object.entries(obj)) {
    entries.push(toKvEntry(key, value));
  }
  return entries;
}

// Threshold for inline auto-expansion. Short payloads — small arrays /
// shallow objects — render inline directly. Anything larger keeps the
// click-to-expand summary so the detail panel doesn't grow without bound.
const KV_INLINE_ARRAY_LIMIT = 8;
const KV_INLINE_OBJECT_KEY_LIMIT = 8;
const KV_INLINE_JSON_BYTES_LIMIT = 600;

function shouldInlineExpand(value: any): boolean {
  if (Array.isArray(value)) {
    if (value.length > KV_INLINE_ARRAY_LIMIT) return false;
    try {
      return JSON.stringify(value).length <= KV_INLINE_JSON_BYTES_LIMIT;
    } catch {
      return false;
    }
  }
  if (value && typeof value === "object") {
    const keys = Object.keys(value);
    if (keys.length > KV_INLINE_OBJECT_KEY_LIMIT) return false;
    try {
      return JSON.stringify(value).length <= KV_INLINE_JSON_BYTES_LIMIT;
    } catch {
      return false;
    }
  }
  return false;
}

function toKvEntry(key: string, value: any): KvEntry {
  const label = humanizeKey(key);
  if (value === null || value === undefined) {
    return { key, label, kind: "scalar", display: "—", raw: value, defaultExpanded: false };
  }
  if (typeof value === "boolean") {
    return { key, label, kind: "bool", display: value ? "true" : "false", raw: value, defaultExpanded: false };
  }
  if (typeof value === "number") {
    return { key, label, kind: "scalar", display: value.toLocaleString(), raw: value, defaultExpanded: false };
  }
  if (typeof value === "string") {
    return { key, label, kind: "scalar", display: value, raw: value, defaultExpanded: false };
  }
  if (Array.isArray(value)) {
    return {
      key,
      label,
      kind: "array",
      display: `Array · ${value.length}`,
      raw: value,
      defaultExpanded: shouldInlineExpand(value),
    };
  }
  if (typeof value === "object") {
    const n = Object.keys(value as object).length;
    return {
      key,
      label,
      kind: "object",
      display: `Object · ${n} keys`,
      raw: value,
      defaultExpanded: shouldInlineExpand(value),
    };
  }
  return { key, label, kind: "scalar", display: String(value), raw: value, defaultExpanded: false };
}

interface AttemptTab {
  n: number;
  status: string;
  active: boolean;
}

const attemptTabs = computed<AttemptTab[]>(() => {
  const latest = data.value?.latest_attempt || 0;
  if (latest <= 1) return [];
  const active = selectedAttempt.value ?? data.value?.attempt ?? latest;
  const out: AttemptTab[] = [];
  for (let n = 1; n <= latest; n++) {
    out.push({
      n,
      status: attemptStatuses.get(n) || "unknown",
      active: n === active,
    });
  }
  return out;
});

function attemptGlyph(status: string): { ch: string; cls: string } {
  switch (status) {
    case "done":
      return { ch: "✓", cls: "text-success" };
    case "failed":
      return { ch: "✗", cls: "text-destructive" };
    case "running":
    case "pending":
    case "processing":
      return { ch: "●", cls: "text-warning animate-[kpLivePulse_1.4s_ease-in-out_infinite]" };
    default:
      return { ch: "–", cls: "text-placeholder" };
  }
}

// True when the panel is showing the most recent attempt (or there's
// only one). Historical attempts must keep their own per-attempt
// trace.status; only the latest attempt's header should defer to the
// knowledge-level parse_status.
const viewingLatestAttempt = computed<boolean>(() => {
  const latest = data.value?.latest_attempt || 0;
  if (latest <= 1) return true;
  const active = selectedAttempt.value ?? data.value?.attempt ?? latest;
  return active === latest;
});

// The authoritative status for the header badge. During the async
// post-pipeline window (summary / question / graph / wiki), the latest
// attempt's ROOT span closes — so trace.status reads 'done' — while
// those subspans can still be running or can later make the knowledge fail.
// The latest knowledge row is therefore authoritative for ALL statuses,
// including terminal ones. Historical attempts keep their own root status.
const headerStatus = computed(() => {
  return resolveTimelineHeaderStatus({
    parseStatus: data.value?.parse_status,
    traceStatus: data.value?.trace?.status,
    isLatestAttempt: viewingLatestAttempt.value,
  });
});

const headerStatusText = computed(() => {
  const s = headerStatus.value;
  return s ? localizedStatus(s) : "";
});

// The header badge's tint, one class string per tone. These were the
// success / danger / warning / default themes of a small light TDesign tag;
// the colours are the same TDesign variables, so the badge still matches
// the rest of the page in both themes.
const headerStatusClass = computed(() => {
  switch (headerStatus.value) {
    case "done":
    case "completed":
      return "bg-(--td-success-color-light) text-success";
    case "failed":
      return "bg-(--td-error-color-light) text-destructive";
    case "running":
    case "processing":
    case "pending":
    case "finalizing":
      return "bg-(--td-warning-color-light) text-warning";
    default:
      return "bg-muted text-foreground";
  }
});

const showLastError = computed(() => Boolean(data.value?.last_error && data.value?.parse_status === "failed"));

const stagesStatDisplay = computed(() => {
  const total = stages.value.length;
  const completedCount = stages.value.filter((s) => s.status === "done" || s.status === "skipped").length;
  const inProgress = stages.value.some(
    (s) => s.status === "running" || s.status === "failed" || s.status === "pending",
  );
  if (inProgress) {
    return {
      label: t("knowledgeStages.head.stagesProgress"),
      value: `${currentStageIndex.value}/${total}`,
    };
  }
  return {
    label: t("knowledgeStages.head.stagesDone"),
    value: `${completedCount}/${total}`,
  };
});

const postprocessTaskStats = computed(() => summarizePostprocessTasks(data.value?.trace));

const headMetaParts = computed(() => {
  if (!data.value) return [];
  const parts: string[] = [t("knowledgeStages.title")];
  if (totalMs.value > 0) {
    parts.push(t("knowledgeStages.total", { d: formatDuration(totalMs.value) }));
  }
  const st = stagesStatDisplay.value;
  parts.push(`${st.label} ${st.value}`);
  const postprocess = postprocessTaskStats.value;
  if (postprocess.total > 0) {
    parts.push(
      t("knowledgeStages.head.postprocessTasks", {
        running: postprocess.running,
        failed: postprocess.failed,
        completed: postprocess.completed,
      }),
    );
  }
  if (data.value.parse_status === "completed" && postprocess.running > 0) {
    parts.push(
      t("knowledgeStages.head.completedWithActiveTrace", {
        n: postprocess.running,
      }),
    );
  }
  if (attemptTabs.value.length === 0 && data.value.current_attempt) {
    parts.push(t("knowledgeStages.attempt", { n: data.value.current_attempt }));
  }
  if (lastFetchedAt.value && isLive.value) {
    let updated = formatRelativeTime(lastFetchedAt.value);
    if (!lastFetchOk.value) {
      updated += ` (${t("knowledgeStages.fetchFailedShort")})`;
    }
    parts.push(`${t("knowledgeStages.head.updated")} ${updated}`);
  }
  return parts;
});

const primaryHeadTitle = computed(() => props.docTitle || t("knowledgeStages.title"));

// Emit summary upstream so the doc-content drawer can show a one-line
// status pill without mounting a second copy of the tree.
watch(
  [
    () => totalMs.value,
    () => headerStatus.value,
    () => currentStageIndex.value,
    () => currentStageLabel.value,
    () => stages.value.length,
  ],
  () => {
    emit("update:summary", {
      totalMs: totalMs.value,
      status: headerStatus.value,
      stageIndex: currentStageIndex.value,
      stageTotal: stages.value.length,
      stageLabel: currentStageLabel.value,
    });
  },
  { immediate: true },
);

function tabHasContent(tab: "input" | "output" | "metadata"): boolean {
  const node = selectedRow.value?.node;
  if (!node) return false;
  return hasContent((node as any)[tab]);
}

const traceMetadata = computed(() => {
  const m = data.value?.trace?.metadata;
  return hasContent(m) ? m : null;
});

watch([selectedSpanId, detailTab], () => {
  if (detailTab.value === "metadata" && !tabHasContent("metadata")) {
    detailTab.value = "overview";
  }
});

// Identity / lineage info for the Overview tab. Surfaces fields that
// were previously buried in the raw payload so the panel doesn't feel
// thin even when the span has no input/output/metadata.
interface IdentityField {
  key: string;
  label: string;
  value: string;
  mono: boolean;
  copyable: boolean;
}

function identityFields(row: FlatRow): IdentityField[] {
  const out: IdentityField[] = [];
  const node = row.node as any;
  out.push({
    key: "name",
    label: t("knowledgeStages.detail.name"),
    value: rowLabel(row),
    mono: false,
    copyable: false,
  });
  out.push({
    key: "kind",
    label: t("knowledgeStages.detail.kind"),
    value: rowKindLabel(row),
    mono: true,
    copyable: false,
  });
  out.push({
    key: "status",
    label: t("knowledgeStages.detail.status"),
    value: localizedStatus(row.node.status),
    mono: false,
    copyable: false,
  });
  if (row.isStage) {
    const idx = stages.value.findIndex((s) => s.name === row.node.name);
    if (idx >= 0) {
      out.push({
        key: "stageIndex",
        label: t("knowledgeStages.detail.stageOrder"),
        value: `${idx + 1} / ${stages.value.length}`,
        mono: true,
        copyable: false,
      });
    }
  }
  if (row.hasChildren) {
    out.push({
      key: "children",
      label: t("knowledgeStages.detail.childCount"),
      value: String((row.node.children || []).length),
      mono: true,
      copyable: false,
    });
  }
  if (node.span_id) out.push({ key: "span_id", label: "span_id", value: node.span_id, mono: true, copyable: true });
  if (node.parent_span_id)
    out.push({
      key: "parent_span_id",
      label: "parent_span_id",
      value: node.parent_span_id,
      mono: true,
      copyable: true,
    });
  if (data.value?.knowledge_id)
    out.push({
      key: "knowledge_id",
      label: "knowledge_id",
      value: data.value.knowledge_id,
      mono: true,
      copyable: true,
    });
  if (data.value?.current_attempt)
    out.push({
      key: "attempt",
      label: t("knowledgeStages.head.attempt"),
      value: `#${data.value.current_attempt}`,
      mono: true,
      copyable: false,
    });
  return out;
}

// Quick stage-by-stage breakdown shown inside root's overview.
interface StageRowSummary {
  name: string;
  label: string;
  status: string;
  duration_ms?: number;
  pct: number;
}

const stageBreakdown = computed<StageRowSummary[]>(() => {
  const total = totalMs.value || 1;
  return stages.value.map((s) => ({
    name: s.name,
    label: t(`knowledgeStages.stage.${s.name}`),
    status: s.status,
    duration_ms: s.duration_ms,
    pct: typeof s.duration_ms === "number" && s.duration_ms > 0 ? Math.min(100, (s.duration_ms / total) * 100) : 0,
  }));
});

function normalizeFileType(value: string): string {
  return String(value || "")
    .trim()
    .replace(/^\./, "")
    .toLowerCase();
}

function getFileTypeFromName(name: string): string {
  const clean = String(name || "").split(/[?#]/)[0];
  const dot = clean.lastIndexOf(".");
  return dot >= 0 ? clean.slice(dot + 1) : "";
}

function formatParserRulesForCurrentFile(rules: Array<{ file_types?: string[]; engine?: string }>): string {
  const currentType = currentKnowledgeFileType.value;
  if (currentType) {
    const matched = rules.find((rule) => (rule.file_types || []).some((ft) => normalizeFileType(ft) === currentType));
    if (matched?.engine) {
      return `${currentType}→${matched.engine}`;
    }
  }
  return rules.map((r) => `${(r.file_types || []).join("/")}→${r.engine}`).join(", ");
}

// Human-readable summary of the per-upload parse overrides stored in
// knowledge.metadata.process_overrides. Empty overrides → KB defaults.
const processConfigLines = computed<string[]>(() => {
  const o = processOverrides.value;
  if (!o) return [t("knowledgeStages.processConfig.kbDefault")];
  const k = (s: string) => `knowledgeStages.processConfig.${s}`;
  const onOff = (v: boolean) => (v ? t(k("on")) : t(k("off")));
  const lines: string[] = [];

  const cc = o.chunking_config;
  if (cc) {
    const parts: string[] = [];
    if (cc.chunk_size != null) parts.push(t(k("chunkSize"), { n: cc.chunk_size }));
    if (cc.enable_parent_child != null) {
      parts.push(cc.enable_parent_child ? t(k("parentChildOn")) : t(k("parentChildOff")));
    }
    if (parts.length) lines.push(`${t(k("chunking"))}: ${parts.join(" · ")}`);
  }

  const rules = o.parser_engine_rules || cc?.parser_engine_rules;
  if (rules?.length) {
    lines.push(`${t(k("parser"))}: ${formatParserRulesForCurrentFile(rules)}`);
  }

  const mm = o.vlm_config?.enabled ?? o.enable_multimodel;
  if (mm != null) lines.push(`${t(k("multimodal"))}: ${onOff(mm)}`);

  if (o.asr_config?.enabled != null) lines.push(`${t(k("asr"))}: ${onOff(o.asr_config.enabled)}`);

  const qg = o.question_generation_config;
  if (qg?.enabled != null) {
    lines.push(`${t(k("question"))}: ${qg.enabled ? t(k("questionOn"), { n: qg.question_count ?? 3 }) : t(k("off"))}`);
  }

  const graph = o.graph_enabled ?? o.extract_config?.enabled;
  if (graph != null) lines.push(`${t(k("graph"))}: ${onOff(graph)}`);

  return lines.length ? lines : [t("knowledgeStages.processConfig.kbDefault")];
});
// ---------- Status styling ----------
//
// The waterfall paints the same handful of span statuses in several
// places (row dots, bars, the outline behind a bar, the detail chip, the
// stage breakdown). Each used to be a `.kp-<part>-<status>` rule; they are
// now one function per part returning the utility classes for a status, so
// the palette for a part reads in one place and Tailwind sees every class
// as a literal string.

// The status dot next to a row name, in the detail header, in the stage
// breakdown and in the compact popover. `placeholder` marks a stage the
// trace has not reached yet: an empty dashed ring, whatever its status.
function dotClass(status: string, placeholder = false): string {
  if (placeholder) return "border border-dashed border-border bg-transparent";
  switch (status) {
    case "done":
    case "completed":
      return "bg-success";
    case "running":
    case "processing":
      return "bg-warning animate-[kpLivePulse_1.4s_ease-in-out_infinite]";
    case "failed":
      return "bg-destructive";
    case "cancelled":
      return "border border-dashed border-placeholder bg-transparent";
    case "skipped":
      return "bg-placeholder opacity-40";
    case "pending":
      return "border border-solid border-border bg-transparent";
    default:
      return "bg-placeholder";
  }
}

// The span's own bar in the waterfall. Status palette — NOT all green. The
// project brand color happens to be green, which made done/running
// visually identical (both solid green). Done stays green (universal
// "success" semantic); running goes amber + striped (CI-style "in
// progress" — recognized everywhere from GitHub Actions to Jenkins).
//
// The running bar is `relative` rather than `absolute`: it carries the
// indeterminate sweep as an ::after that has to be clipped to the bar.
// It is the only in-flow child of its cell, so `left` / `top` place it
// exactly where the absolute bars sit.
function barClass(status: string): string {
  switch (status) {
    case "done":
      return "absolute top-3 h-2 bg-success";
    case "failed":
      return "absolute top-3 h-2 bg-destructive";
    case "cancelled":
      return "absolute top-[13px] h-1.5 border border-dashed border-destructive bg-transparent";
    case "skipped":
      return "absolute top-3 h-2 bg-placeholder opacity-40";
    case "pending":
      return "hidden";
    case "running":
      // Muted amber base. The diagonal stripes do the "in flight"
      // signaling — the tone just supplies a subtle hint. Earlier
      // iteration used full --td-warning-color + halo shadow + hard
      // white stripes; users found that visually screaming.
      return (
        "relative top-3 h-2 overflow-hidden bg-(--td-warning-color-3) " +
        "bg-[linear-gradient(135deg,rgba(255,255,255,0.22)_25%,transparent_25%,transparent_50%,rgba(255,255,255,0.22)_50%,rgba(255,255,255,0.22)_75%,transparent_75%,transparent)] " +
        "bg-size-[14px_14px] animate-[kpStripes_1.6s_linear_infinite] " +
        "after:absolute after:inset-0 after:animate-[kpSweep_1.6s_linear_infinite] " +
        "after:bg-[linear-gradient(90deg,transparent_0%,rgba(255,255,255,0.5)_50%,transparent_100%)]"
      );
    default:
      return "absolute top-3 h-2 bg-placeholder";
  }
}

// The dashed outline drawn behind a bar whose descendants outlive it. The
// border tints are the old fixed rgba values; hover (in the template)
// replaces them with the secondary text colour.
function wrapClass(status: string): string {
  switch (status) {
    case "done":
      return "border-[rgba(7,192,95,0.35)]";
    case "failed":
      return "border-[rgba(229,87,64,0.5)]";
    case "running":
      return "border-[rgba(250,157,59,0.5)]";
    case "cancelled":
      return "border-[rgba(229,87,64,0.3)]";
    default:
      return "border-border";
  }
}

// The fill of a stage's bar in the root overview's breakdown table. Same
// palette as the waterfall bars, sized to the table's 6px track.
function breakdownBarClass(status: string): string {
  switch (status) {
    case "done":
      return "bg-success";
    case "failed":
      return "bg-destructive";
    case "cancelled":
      return "border border-dashed border-destructive bg-transparent";
    case "skipped":
      return "bg-placeholder opacity-40";
    case "pending":
      return "hidden";
    case "running":
      return (
        "bg-(--td-warning-color-3) " +
        "bg-[linear-gradient(135deg,rgba(255,255,255,0.22)_25%,transparent_25%,transparent_50%,rgba(255,255,255,0.22)_50%,rgba(255,255,255,0.22)_75%,transparent_75%,transparent)] " +
        "bg-size-[14px_14px] animate-[kpStripes_1.6s_linear_infinite]"
      );
    default:
      return "bg-placeholder";
  }
}

// The status chip in the detail header — a soft tinted background, the way
// a light TDesign tag looks.
function chipClass(status: string): string {
  switch (status) {
    case "done":
      return "bg-(--td-success-color-light) text-success";
    case "running":
      return "bg-(--td-warning-color-light) text-warning";
    case "failed":
      return "bg-(--td-error-color-light) text-destructive";
    case "cancelled":
    case "pending":
      return "bg-(--td-bg-color-component) text-muted-foreground";
    case "skipped":
      return "bg-(--td-bg-color-component) text-placeholder";
    default:
      return "bg-(--td-bg-color-component) text-foreground";
  }
}

// A waterfall row's background. The active row and hovered rows share the
// secondary container colour; stage rows keep their tint on hover (their
// rule used to outrank the hover rule), span rows are a hair off the card
// colour until hovered.
function rowClass(row: FlatRow): string {
  if (selectedSpanId.value === row.key) return "bg-muted before:bg-primary";
  if (row.isStage) return "bg-[color-mix(in_srgb,var(--td-bg-color-secondarycontainer)_55%,transparent)]";
  if (!row.isRoot) {
    return "bg-[color-mix(in_srgb,var(--td-bg-color-container)_92%,var(--td-bg-color-secondarycontainer))] hover:bg-muted";
  }
  return "hover:bg-muted";
}

// A detail tab's text colour. A tab with nothing to show stays greyed out
// even while it is the active one; hover always brings the text forward.
function tabClass(tab: typeof detailTab.value, empty = false): string {
  const active = detailTab.value === tab;
  const colour = empty ? "text-placeholder" : active ? "text-foreground" : "text-muted-foreground";
  const underline = active
    ? "font-semibold after:absolute after:right-3.5 after:-bottom-px after:left-3.5 after:h-0.5 after:rounded-t-[2px] after:bg-primary"
    : "";
  return `${colour} ${underline}`;
}
</script>

<template>
  <!-- `kp-timeline` stays as a hook: doc-content.vue sizes this root through it. -->
  <div
    class="kp-timeline text-foreground overflow-hidden font-(family-name:--app-font-family) text-[13px]"
    :class="compact ? 'h-auto w-full max-w-[320px]' : 'h-full w-full'"
  >
    <!-- =========================================================
         COMPACT MODE — used by the card hover popover. Untouched.
         ========================================================= -->
    <template v-if="compact">
      <div class="flex items-center gap-1.5">
        <span
          v-for="s in stages"
          :key="s.name"
          class="inline-block size-2 rounded-full"
          :class="dotClass(s.status)"
          :title="t(`knowledgeStages.stage.${s.name}`) + ' · ' + t(`knowledgeStages.status.${s.status}`)"
        />
      </div>
      <div class="text-muted-foreground mt-1 truncate text-xs">
        <template v-if="totalMs > 0">
          {{ t("knowledgeStages.totalDuration", { d: formatDuration(totalMs) }) }}
        </template>
        <template v-else>
          <span>{{ t("knowledgeStages.title") }}：</span>
          <span class="text-primary font-semibold">{{ currentStageIndex }}/{{ stages.length }}</span>
          <span> · {{ currentStageLabel }}</span>
        </template>
      </div>
    </template>

    <!-- =========================================================
         FULL MODE — Langfuse-style waterfall, lives inside the
         secondary drawer. Bottom-docked detail panel.
         ========================================================= -->
    <template v-else>
      <div class="bg-card relative flex size-full min-h-0 min-w-0 flex-col overflow-hidden">
        <!-- ============== HEADER ============== -->
        <div class="border-border bg-card flex-none border-0 border-b border-solid px-5 pt-3.5 pb-2.5">
          <div class="flex min-w-0 items-center gap-2">
            <h2
              class="text-foreground m-0 min-w-0 flex-1 truncate text-[15px] leading-[1.35] font-semibold"
              :title="primaryHeadTitle"
            >
              {{ primaryHeadTitle }}
            </h2>
            <span
              v-if="data && headerStatusText"
              class="inline-flex h-5 shrink-0 items-center rounded-(--td-radius-default) px-1.5 text-xs leading-none whitespace-nowrap"
              :class="headerStatusClass"
            >
              {{ headerStatusText }}
            </span>
            <!-- LIVE badge — sits next to the title while polling, telegraphs the
                 pipeline is actively refreshing. Pulsing dot + uppercase mono label. -->
            <span
              v-if="isLive"
              class="text-warning inline-flex items-center gap-[5px] rounded-md bg-(--td-warning-color-light) px-2 py-0.5 text-[10px] leading-none font-semibold tracking-[0.06em] uppercase"
              :title="t('knowledgeStages.liveTooltip')"
            >
              <span class="bg-warning size-1.5 animate-[kpLivePulse_1.4s_ease-in-out_infinite] rounded-full" />
              <span class="font-(family-name:--app-font-family-mono)">{{ t("knowledgeStages.live") }}</span>
            </span>
            <div class="ml-auto flex shrink-0 items-center gap-1">
              <Popover :open="processConfigOpen" @update:open="(v: boolean) => (processConfigOpen = v)">
                <PopoverTrigger as-child>
                  <button
                    type="button"
                    data-slot="icon-button"
                    class="text-placeholder hover:bg-muted hover:text-foreground inline-flex size-[26px] items-center justify-center rounded-(--td-radius-default) transition-colors duration-150"
                    :title="t('knowledgeStages.processConfig.title')"
                    :aria-label="t('knowledgeStages.processConfig.title')"
                    @mouseenter="openProcessConfig"
                    @mouseleave="scheduleCloseProcessConfig"
                  >
                    <InfoIcon class="size-3.5" />
                  </button>
                </PopoverTrigger>
                <PopoverContent
                  side="bottom"
                  align="end"
                  class="w-auto max-w-[340px] gap-1 px-3 py-2"
                  @open-auto-focus.prevent
                  @mouseenter="openProcessConfig"
                  @mouseleave="scheduleCloseProcessConfig"
                >
                  <div class="text-foreground mb-0.5 text-[13px] font-semibold">
                    {{ t("knowledgeStages.processConfig.title") }}
                  </div>
                  <div
                    v-for="(line, i) in processConfigLines"
                    :key="i"
                    class="text-muted-foreground text-xs leading-[1.6] [word-break:break-word]"
                  >
                    {{ line }}
                  </div>
                </PopoverContent>
              </Popover>
              <button
                type="button"
                data-slot="icon-button"
                class="text-placeholder enabled:hover:bg-muted enabled:hover:text-foreground inline-flex size-[26px] items-center justify-center rounded-(--td-radius-default) transition-colors duration-150 disabled:cursor-not-allowed disabled:opacity-40"
                :disabled="loading || refreshing"
                :title="isLive ? t('knowledgeStages.autoRefreshOn') : t('knowledgeStages.refresh')"
                :aria-label="isLive ? t('knowledgeStages.autoRefreshOn') : t('knowledgeStages.refresh')"
                @click="onManualRefresh"
              >
                <!-- A fast spin while a manual refresh is in flight; a slow amber
                     rotation while auto-polling, which says "refresh is happening
                     on its own" without an extra label or badge. -->
                <RefreshCwIcon
                  class="size-3.5"
                  :class="{
                    'animate-spin [animation-duration:0.9s]': refreshing,
                    'text-warning animate-spin [animation-duration:4s]': isLive && !refreshing,
                  }"
                />
              </button>
              <Popover
                v-if="canCancelParse"
                :open="cancelConfirmOpen"
                @update:open="(v: boolean) => (cancelConfirmOpen = v)"
              >
                <PopoverTrigger as-child>
                  <!-- Stop-parse control — stays a quiet placeholder icon until hover, then
                       reveals its destructive intent with the error tint. Matches the other
                       header icon buttons rather than shouting with a full outline button. -->
                  <button
                    type="button"
                    data-slot="icon-button"
                    class="text-placeholder enabled:hover:text-destructive inline-flex size-[26px] items-center justify-center rounded-(--td-radius-default) transition-colors duration-150 enabled:hover:bg-(--td-error-color-light) disabled:cursor-not-allowed disabled:opacity-40"
                    :disabled="cancelling"
                    :title="t('knowledgeBase.cancelParse')"
                    :aria-label="t('knowledgeBase.cancelParse')"
                    @click.stop
                  >
                    <Loader2Icon v-if="cancelling" class="size-[15px] animate-spin [animation-duration:0.9s]" />
                    <CircleXIcon v-else class="size-[15px]" />
                  </button>
                </PopoverTrigger>
                <PopoverContent side="bottom" class="w-72">
                  <div class="flex gap-2">
                    <CircleAlertIcon class="text-warning mt-0.5 size-4 shrink-0" />
                    <p class="text-foreground m-0 text-sm">
                      {{ t("knowledgeBase.cancelParseConfirmBody", { title: props.docTitle || props.knowledgeId }) }}
                    </p>
                  </div>
                  <div class="flex justify-end gap-2">
                    <Button variant="outline" size="sm" @click="cancelConfirmOpen = false">
                      {{ t("common.cancel") }}
                    </Button>
                    <Button variant="destructive" size="sm" @click="confirmCancelParse">
                      {{ t("knowledgeBase.cancelParse") }}
                    </Button>
                  </div>
                </PopoverContent>
              </Popover>
              <Button
                v-if="data?.parse_status === 'failed'"
                size="sm"
                variant="outline"
                class="border-primary text-primary hover:text-primary dark:border-primary"
                @click="onRetry"
              >
                <RefreshCwIcon class="size-3.5" />
                <span>{{ t("knowledgeStages.retry") }}</span>
              </Button>
              <button
                v-if="showClose"
                type="button"
                data-slot="icon-button"
                class="text-placeholder hover:bg-muted hover:text-foreground inline-flex size-[26px] items-center justify-center rounded-(--td-radius-default) transition-colors duration-150"
                :aria-label="t('knowledgeStages.close')"
                :title="t('knowledgeStages.close')"
                @click="emit('close')"
              >
                <XIcon class="size-4" />
              </button>
            </div>
          </div>

          <p
            v-if="headMetaParts.length > 0"
            class="text-muted-foreground mx-0 mt-2 mb-0 text-xs leading-normal [word-break:break-word]"
          >
            <template v-for="(part, idx) in headMetaParts" :key="idx">
              <span v-if="idx > 0" class="text-placeholder mx-1.5" aria-hidden="true">·</span>
              <span class="inline">{{ part }}</span>
            </template>
          </p>

          <!-- Attempts strip -->
          <div v-if="attemptTabs.length > 0" class="mt-2.5 flex gap-1.5 overflow-x-auto pb-0.5">
            <button
              v-for="tab in attemptTabs"
              :key="tab.n"
              type="button"
              data-slot="attempt"
              class="inline-flex items-center gap-[5px] rounded-(--td-radius-default) border border-solid px-2.5 py-1 text-xs leading-[1.4] whitespace-nowrap transition-colors duration-150"
              :class="
                tab.active
                  ? 'border-primary bg-primary text-primary-foreground'
                  : 'border-border bg-card text-muted-foreground hover:border-placeholder hover:bg-muted hover:text-foreground'
              "
              @click="onAttemptChange(tab.n)"
            >
              <span class="font-(family-name:--app-font-family-mono) text-[11px] font-semibold tracking-normal"
                >#{{ tab.n }}</span
              >
              <span
                class="text-[9px] leading-none"
                :class="[attemptGlyph(tab.status).cls, { 'text-primary-foreground!': tab.active }]"
                >{{ attemptGlyph(tab.status).ch }}</span
              >
            </button>
          </div>

          <!-- Last error block — pinned in the header so long trace trees don't bury it -->
          <div
            v-if="showLastError && data?.last_error"
            class="mt-2.5 flex overflow-hidden rounded-md border border-solid border-(--td-error-color-3) bg-(--td-error-color-light)"
            role="alert"
          >
            <div class="bg-destructive w-[3px] shrink-0" />
            <div class="min-w-0 flex-1 px-3.5 py-2.5">
              <div class="mb-1 flex items-center gap-2">
                <span
                  class="bg-destructive text-primary-foreground inline-flex size-4 shrink-0 items-center justify-center rounded-full text-[11px] font-bold"
                  >!</span
                >
                <span class="text-destructive text-xs font-semibold">{{
                  localizedErrorTitle(data.last_error.error_code)
                }}</span>
                <span
                  v-if="data.last_error.error_code"
                  class="bg-destructive text-primary-foreground ml-auto rounded-sm px-1.5 py-px font-(family-name:--app-font-family-mono) text-[11px] tracking-normal"
                  >{{ data.last_error.error_code }}</span
                >
              </div>
              <div class="text-muted-foreground mb-1 text-xs">
                {{ localizedErrorSuggestion(data.last_error.error_code) }}
              </div>
              <div
                v-if="data.last_error.error_message"
                class="text-placeholder font-(family-name:--app-font-family-mono) text-[11px] tracking-normal [word-break:break-word] whitespace-pre-wrap"
              >
                {{ data.last_error.error_message }}
              </div>
            </div>
          </div>
        </div>

        <!-- ============== BODY (Waterfall) ============== -->
        <div class="bg-card flex min-h-0 flex-auto flex-col overflow-hidden">
          <div
            v-if="loading && !data"
            class="text-placeholder flex flex-auto items-center justify-center gap-2 px-5 py-14 text-[13px]"
          >
            <Loader2Icon class="text-primary size-5 animate-spin" />
          </div>
          <div
            v-else-if="!data && !loading"
            class="text-placeholder flex flex-auto items-center justify-center gap-2 px-5 py-14 text-[13px]"
          >
            <span>{{ t("knowledgeStages.noActivity") }}</span>
          </div>

          <template v-else-if="data">
            <!-- Ruler sits outside the scroll region so the time axis never
                 scrolls away or fights position:sticky inside overflow. -->
            <div
              v-if="showRuler"
              class="border-border bg-card grid h-6 flex-none grid-cols-[minmax(220px,42%)_64px_1fr] items-end border-0 border-b border-dashed px-5 pt-3 pb-1.5 shadow-[0_4px_8px_-6px_rgba(0,0,0,0.12)]"
            >
              <div class="h-full" />
              <div class="h-full" />
              <div class="relative mr-4 h-full">
                <span
                  v-for="(tick, i) in rulerTicks"
                  :key="i"
                  class="text-placeholder absolute bottom-0 flex flex-col text-[10px]"
                  :class="
                    i === 0
                      ? 'items-start'
                      : i === rulerTicks.length - 1
                        ? '-translate-x-full items-end'
                        : '-translate-x-1/2 items-center'
                  "
                  :style="{ left: tick.left }"
                >
                  <span class="bg-border h-[5px] w-px" />
                  <span class="mt-0.5 font-(family-name:--app-font-family-mono) text-[11px] tracking-normal">{{
                    tick.label
                  }}</span>
                </span>
              </div>
            </div>

            <div ref="scrollRef" class="min-h-0 flex-auto overflow-auto pb-4">
              <div class="flex flex-col">
                <!-- `kp-row` stays as a hook: scrollRowIntoView() finds rows by it.
                     The ::before is the brand-coloured edge of the selected row. -->
                <div
                  v-for="row in flatRows"
                  :key="row.key"
                  class="kp-row group/row relative grid h-8 cursor-pointer grid-cols-[minmax(220px,42%)_64px_1fr] items-center px-5 transition-colors duration-150 before:absolute before:top-1 before:bottom-1 before:left-0 before:w-0.5 before:rounded-r-[2px] before:transition-colors before:duration-150"
                  :data-span-key="row.key"
                  :class="[rowClass(row), { 'font-semibold': row.isRoot }]"
                  :title="row.hasChildren && !row.isRoot ? t('knowledgeStages.rowSelectHint') : undefined"
                  @click="selectRow(row)"
                >
                  <div class="min-w-0">
                    <div class="flex min-w-0 items-center gap-[7px]" :style="{ paddingLeft: row.depth * 16 + 'px' }">
                      <button
                        v-if="row.hasChildren && !row.isRoot"
                        type="button"
                        data-slot="tree-toggle"
                        class="text-placeholder group-hover/row:bg-accent group-hover/row:text-foreground hover:bg-accent hover:text-foreground -my-[3px] inline-flex size-[22px] shrink-0 items-center justify-center rounded-(--td-radius-default) transition-colors duration-150"
                        :aria-expanded="isRowExpanded(row.key)"
                        :aria-label="treeToggleAriaLabel(row)"
                        @click="toggleTree(row, $event)"
                      >
                        <ChevronDownIcon v-if="isRowExpanded(row.key)" class="size-3.5" />
                        <ChevronRightIcon v-else class="size-3.5" />
                      </button>
                      <span v-else class="-my-[3px] inline-block size-[22px] shrink-0" />
                      <span
                        class="size-[7px] shrink-0 rounded-full"
                        :class="dotClass(row.node.status, isPlaceholder(row.node))"
                      />
                      <span
                        class="text-foreground truncate"
                        :class="{
                          'text-[13px] font-semibold': row.isRoot,
                          'font-(family-name:--app-font-family-mono) text-[11px]': !row.isRoot && !row.isStage,
                          'text-xs': !row.isRoot && row.isStage,
                        }"
                        >{{ rowLabel(row) }}</span
                      >
                      <span
                        class="text-placeholder ml-auto shrink-0 pl-2 font-(family-name:--app-font-family-mono) text-[10px] tracking-[0.5px] uppercase"
                        >{{ rowKindLabel(row) }}</span
                      >
                    </div>
                  </div>

                  <div
                    class="text-muted-foreground pr-3 text-right font-(family-name:--app-font-family-mono) text-[11px] tracking-normal"
                  >
                    <template v-if="row.node.status === 'running'">
                      <span class="text-warning font-semibold">{{ formatDuration(liveElapsedMs(row.node)) }}</span>
                    </template>
                    <template v-else>
                      {{ formatSpanDuration(row.node) }}
                    </template>
                  </div>

                  <div class="relative mr-4 h-8">
                    <!-- Vertical "now" cursor — animates left during polling so the user can
                         visually confirm time is advancing even when the running bar grows
                         slowly toward the right edge of the trace. -->
                    <span
                      v-if="nowMarkerPct !== null && row.isRoot"
                      class="bg-warning before:bg-warning pointer-events-none absolute top-1 bottom-1 z-[1] w-px opacity-65 transition-[left] duration-1000 ease-linear before:absolute before:-top-0.5 before:-left-[3px] before:size-[7px] before:animate-[kpLivePulse_1.4s_ease-in-out_infinite] before:rounded-full"
                      :style="{ left: nowMarkerPct + '%' }"
                    />
                    <div
                      v-if="isPlaceholder(row.node)"
                      class="border-border absolute top-[13px] right-1 h-1.5 w-3.5 rounded-sm border border-dashed bg-transparent"
                    />
                    <template v-else>
                      <!-- Wrapping outline: descendants extend past this
                         span's own end (e.g. async postprocess subspans
                         under a closed stage). Renders behind the solid
                         self-bar so both are visible. -->
                      <div
                        v-if="wrapStyle(row.node)"
                        class="group/bar hover:border-muted-foreground pointer-events-auto absolute top-[9px] z-[1] h-3.5 min-w-1 rounded-sm border border-dashed bg-transparent transition-[left,width] duration-800 ease-[cubic-bezier(0.2,0.8,0.2,1)]"
                        :class="wrapClass(row.node.status)"
                        :style="wrapStyle(row.node) || {}"
                      >
                        <span
                          class="bg-foreground text-primary-foreground pointer-events-none absolute left-1/2 z-10 flex -translate-x-1/2 items-center gap-1 rounded-(--td-radius-default) px-2 py-1 text-[11px] whitespace-nowrap opacity-0 transition-opacity duration-150 group-hover/bar:opacity-100"
                          :class="row.isRoot ? 'top-[calc(100%+8px)]' : 'bottom-[calc(100%+8px)]'"
                        >
                          <span class="font-medium">{{ rowLabel(row) }}</span>
                          <span class="text-(--td-font-white-3)">·</span>
                          <span class="font-(family-name:--app-font-family-mono) text-[11px] tracking-normal">{{
                            formatDuration(wrapDurationMs(row.node))
                          }}</span>
                          <span class="text-(--td-font-white-3)">·</span>
                          <span>{{ t("knowledgeStages.detail.includingChildren") }}</span>
                        </span>
                      </div>
                      <!-- Smooth left/width changes so polling-driven re-renders feel like
                           the bar is growing, not jumping. The 800ms easing sits under the
                           nowTick 1Hz cadence. -->
                      <div
                        class="group/bar z-[2] min-w-0.5 rounded-sm transition-[left,width,filter] duration-800 ease-[cubic-bezier(0.2,0.8,0.2,1)] group-hover/row:brightness-105"
                        :class="barClass(row.node.status)"
                        :style="barStyle(row.node)"
                      >
                        <span
                          class="bg-foreground text-primary-foreground pointer-events-none absolute left-1/2 z-10 flex -translate-x-1/2 items-center gap-1 rounded-(--td-radius-default) px-2 py-1 text-[11px] whitespace-nowrap opacity-0 transition-opacity duration-150 group-hover/bar:opacity-100"
                          :class="row.isRoot ? 'top-[calc(100%+8px)]' : 'bottom-[calc(100%+8px)]'"
                        >
                          <span class="font-medium">{{ rowLabel(row) }}</span>
                          <span class="text-(--td-font-white-3)">·</span>
                          <span class="font-(family-name:--app-font-family-mono) text-[11px] tracking-normal">{{
                            row.node.status === "running"
                              ? formatDuration(liveElapsedMs(row.node))
                              : formatSpanDuration(row.node)
                          }}</span>
                          <span class="text-(--td-font-white-3)">·</span>
                          <span>{{ localizedStatus(row.node.status) }}</span>
                        </span>
                      </div>
                      <span
                        v-if="barOffsetPct(row.node) !== null && barOffsetMs(row.node) > 0"
                        class="text-placeholder pointer-events-none absolute -bottom-px -translate-x-1/2 font-(family-name:--app-font-family-mono) text-[11px] tracking-normal whitespace-nowrap transition-opacity duration-150 group-hover/row:opacity-100"
                        :class="selectedSpanId === row.key ? 'opacity-100' : 'opacity-0'"
                        :style="{ left: barOffsetPct(row.node) + '%' }"
                      >
                        +{{ formatDuration(barOffsetMs(row.node)) }}
                      </span>
                    </template>
                  </div>
                </div>
              </div>
            </div>
          </template>
        </div>

        <!-- ============== DETAIL PANEL ============== -->
        <div
          class="border-border bg-card flex flex-none flex-col overflow-hidden border-0 border-t border-solid transition-[height] duration-240 ease-[cubic-bezier(0.2,0.8,0.2,1)]"
          :class="detailOpen ? 'h-1/2 min-h-80' : 'h-0'"
        >
          <template v-if="selectedRow">
            <div
              class="border-border flex flex-none items-center justify-between gap-2 border-0 border-b border-solid px-5 pt-3 pb-2.5"
            >
              <div class="flex min-w-0 items-center gap-2">
                <span class="size-2 shrink-0 rounded-full" :class="dotClass(selectedRow.node.status)" />
                <span class="text-foreground min-w-0 truncate text-[13px] font-semibold">{{
                  rowLabel(selectedRow)
                }}</span>
                <span
                  class="text-placeholder shrink-0 font-(family-name:--app-font-family-mono) text-[10px] tracking-[0.5px] uppercase"
                  >{{ rowKindLabel(selectedRow) }}</span
                >
                <span
                  class="inline-flex shrink-0 items-center rounded-(--td-radius-default) px-2 py-px text-[11px] font-medium"
                  :class="chipClass(selectedRow.node.status)"
                >
                  {{ localizedStatus(selectedRow.node.status) }}
                </span>
              </div>
              <div class="flex items-center gap-1">
                <button
                  type="button"
                  data-slot="icon-button"
                  class="text-placeholder hover:bg-muted hover:text-foreground inline-flex size-[26px] items-center justify-center rounded-(--td-radius-default) transition-colors duration-150"
                  :title="t('knowledgeStages.copyDetails')"
                  @click.stop="copySpan(selectedRow.node)"
                >
                  <CopyIcon class="size-[18px]" />
                </button>
                <button
                  type="button"
                  data-slot="icon-button"
                  class="text-placeholder hover:bg-muted hover:text-foreground inline-flex size-[26px] items-center justify-center rounded-(--td-radius-default) transition-colors duration-150"
                  :title="t('knowledgeStages.close')"
                  @click="closeDetail"
                >
                  <XIcon class="size-[18px]" />
                </button>
              </div>
            </div>

            <!-- Tabs -->
            <div class="border-border bg-card flex flex-none border-0 border-b border-solid px-5">
              <button
                type="button"
                data-slot="tab"
                class="hover:text-foreground relative inline-flex items-center gap-1 px-3.5 pt-[9px] pb-2.5 text-[13px] transition-colors duration-150"
                :class="tabClass('overview')"
                @click="detailTab = 'overview'"
              >
                {{ t("knowledgeStages.tab.overview") }}
              </button>
              <button
                type="button"
                data-slot="tab"
                class="hover:text-foreground relative inline-flex items-center gap-1 px-3.5 pt-[9px] pb-2.5 text-[13px] transition-colors duration-150"
                :class="tabClass('input', !tabHasContent('input'))"
                @click="detailTab = 'input'"
              >
                {{ t("knowledgeStages.detail.input") }}
              </button>
              <button
                type="button"
                data-slot="tab"
                class="hover:text-foreground relative inline-flex items-center gap-1 px-3.5 pt-[9px] pb-2.5 text-[13px] transition-colors duration-150"
                :class="tabClass('output', !tabHasContent('output'))"
                @click="detailTab = 'output'"
              >
                {{ t("knowledgeStages.detail.output") }}
              </button>
              <button
                v-if="tabHasContent('metadata')"
                type="button"
                data-slot="tab"
                class="hover:text-foreground relative inline-flex items-center gap-1 px-3.5 pt-[9px] pb-2.5 text-[13px] transition-colors duration-150"
                :class="tabClass('metadata')"
                @click="detailTab = 'metadata'"
              >
                {{ t("knowledgeStages.detail.metadata") }}
              </button>
              <button
                type="button"
                data-slot="tab"
                class="hover:text-foreground relative inline-flex items-center gap-1 px-3.5 pt-[9px] pb-2.5 text-[13px] transition-colors duration-150"
                :class="tabClass('raw')"
                @click="detailTab = 'raw'"
              >
                {{ t("knowledgeStages.tab.raw") }}
              </button>
            </div>

            <div class="flex flex-auto flex-col gap-4 overflow-y-auto px-5 pt-4 pb-[18px]">
              <!-- Overview tab -->
              <template v-if="detailTab === 'overview'">
                <!-- Timing -->
                <div class="flex flex-col gap-2">
                  <div class="text-muted-foreground text-[11px] font-medium tracking-[0.5px] uppercase">
                    {{ t("knowledgeStages.detail.timing") }}
                  </div>
                  <div
                    class="divide-muted border-border bg-card flex flex-col divide-y divide-solid overflow-hidden rounded-md border border-solid"
                  >
                    <div class="bg-card grid min-w-0 grid-cols-[130px_1fr] items-center gap-3 px-3 py-2 text-xs">
                      <span class="text-muted-foreground truncate text-[11px] font-medium">{{
                        t("knowledgeStages.detail.started")
                      }}</span>
                      <span
                        class="text-foreground inline-flex min-w-0 items-center gap-1.5 font-(family-name:--app-font-family-mono) text-[11px] tracking-normal [overflow-wrap:anywhere] [word-break:break-word]"
                        >{{ formatTime(selectedRow.node.started_at) }}</span
                      >
                    </div>
                    <div class="bg-card grid min-w-0 grid-cols-[130px_1fr] items-center gap-3 px-3 py-2 text-xs">
                      <span class="text-muted-foreground truncate text-[11px] font-medium">{{
                        t("knowledgeStages.detail.finished")
                      }}</span>
                      <span
                        class="text-foreground inline-flex min-w-0 items-center gap-1.5 font-(family-name:--app-font-family-mono) text-[11px] tracking-normal [overflow-wrap:anywhere] [word-break:break-word]"
                      >
                        <template v-if="selectedRow.node.status === 'running'">
                          <span class="text-warning font-(family-name:--app-font-family) text-[11px] italic">{{
                            t("knowledgeStages.detail.inProgress")
                          }}</span>
                        </template>
                        <template v-else>
                          {{ formatTime(selectedRow.node.finished_at) }}
                        </template>
                      </span>
                    </div>
                    <div class="bg-card grid min-w-0 grid-cols-[130px_1fr] items-center gap-3 px-3 py-2 text-xs">
                      <span class="text-muted-foreground truncate text-[11px] font-medium">{{
                        t("knowledgeStages.detail.duration")
                      }}</span>
                      <span
                        class="text-foreground inline-flex min-w-0 items-center gap-1.5 font-(family-name:--app-font-family-mono) text-[11px] tracking-normal [overflow-wrap:anywhere] [word-break:break-word]"
                      >
                        <template v-if="selectedRow.node.status === 'running'">
                          {{ formatDuration(liveElapsedMs(selectedRow.node)) }}
                          <span
                            class="text-warning ml-1.5 inline-block rounded-sm bg-(--td-warning-color-light) px-1.5 font-(family-name:--app-font-family) text-[9px] font-semibold tracking-[0.5px] uppercase"
                            >{{ t("knowledgeStages.detail.elapsed") }}</span
                          >
                        </template>
                        <template v-else>
                          {{ formatSpanDuration(selectedRow.node) }}
                        </template>
                      </span>
                    </div>
                    <div
                      v-if="!selectedRow.isRoot && barOffsetMs(selectedRow.node) > 0"
                      class="bg-card grid min-w-0 grid-cols-[130px_1fr] items-center gap-3 px-3 py-2 text-xs"
                    >
                      <span class="text-muted-foreground truncate text-[11px] font-medium">{{
                        t("knowledgeStages.detail.offset")
                      }}</span>
                      <span
                        class="text-foreground inline-flex min-w-0 items-center gap-1.5 font-(family-name:--app-font-family-mono) text-[11px] tracking-normal [overflow-wrap:anywhere] [word-break:break-word]"
                        >+{{ formatDuration(barOffsetMs(selectedRow.node)) }}</span
                      >
                    </div>
                  </div>
                </div>

                <!-- Identity / lineage -->
                <div class="flex flex-col gap-2">
                  <div class="text-muted-foreground text-[11px] font-medium tracking-[0.5px] uppercase">
                    {{ t("knowledgeStages.detail.identity") }}
                  </div>
                  <div
                    class="divide-muted border-border bg-card flex flex-col divide-y divide-solid overflow-hidden rounded-md border border-solid"
                  >
                    <div
                      v-for="entry in identityFields(selectedRow)"
                      :key="entry.key"
                      class="bg-card grid min-w-0 grid-cols-[130px_1fr] items-center gap-3 px-3 py-2 text-xs"
                    >
                      <span class="text-muted-foreground truncate text-[11px] font-medium">{{ entry.label }}</span>
                      <span
                        class="text-foreground inline-flex min-w-0 items-center gap-1.5 [overflow-wrap:anywhere] [word-break:break-word]"
                        :class="{
                          'font-(family-name:--app-font-family-mono) text-[11px] tracking-normal': entry.mono,
                          'overflow-hidden': entry.copyable,
                        }"
                      >
                        <span :class="{ 'min-w-0 flex-1 truncate': entry.copyable }">{{ entry.value }}</span>
                        <button
                          v-if="entry.copyable"
                          type="button"
                          data-slot="icon-button"
                          class="text-placeholder hover:bg-accent hover:text-primary inline-flex size-[22px] shrink-0 items-center justify-center rounded-sm"
                          :title="t('knowledgeStages.copy')"
                          @click.stop="copyValue(entry.value)"
                        >
                          <CopyIcon class="size-3.5" />
                        </button>
                      </span>
                    </div>
                  </div>
                </div>

                <div v-if="traceMetadata" class="flex flex-col gap-2">
                  <div class="text-muted-foreground text-[11px] font-medium tracking-[0.5px] uppercase">
                    {{ t("knowledgeStages.detail.traceMetadata") }}
                  </div>
                  <p class="text-placeholder mx-0 mt-1 mb-2 text-xs leading-normal">
                    {{ t("knowledgeStages.detail.metadataHint") }}
                  </p>
                  <div
                    class="divide-muted border-border bg-card flex flex-col divide-y divide-solid overflow-hidden rounded-md border border-solid"
                  >
                    <div
                      v-for="entry in buildKvEntries(traceMetadata)"
                      :key="entry.key"
                      class="bg-card grid min-w-0 grid-cols-[130px_1fr] items-start gap-3 px-3 py-2 text-xs"
                    >
                      <span
                        class="text-muted-foreground truncate font-(family-name:--app-font-family-mono) text-[11px] font-medium tracking-normal"
                        >{{ entry.key }}</span
                      >
                      <span
                        class="text-foreground inline-flex min-w-0 items-center gap-1.5 text-xs [overflow-wrap:anywhere] [word-break:break-word]"
                        >{{ entry.display }}</span
                      >
                    </div>
                  </div>
                </div>

                <!-- Stage breakdown (root only) -->
                <div v-if="selectedRow.isRoot" class="flex flex-col gap-2">
                  <div class="text-muted-foreground text-[11px] font-medium tracking-[0.5px] uppercase">
                    {{ t("knowledgeStages.detail.stageBreakdown") }}
                  </div>
                  <div class="border-border bg-card flex flex-col gap-1.5 rounded-md border border-solid px-3 py-2.5">
                    <div
                      v-for="s in stageBreakdown"
                      :key="s.name"
                      class="grid grid-cols-[110px_1fr_64px] items-center gap-2.5 text-xs"
                    >
                      <span class="text-foreground inline-flex items-center gap-1.5">
                        <span class="size-[7px] shrink-0 rounded-full" :class="dotClass(s.status)" />
                        {{ s.label }}
                      </span>
                      <div class="bg-muted relative h-1.5 overflow-hidden rounded-sm">
                        <div
                          class="absolute inset-y-0 left-0 rounded-sm transition-[width] duration-800 ease-[cubic-bezier(0.2,0.8,0.2,1)]"
                          :class="breakdownBarClass(s.status)"
                          :style="{ width: s.pct + '%' }"
                        />
                      </div>
                      <span
                        class="text-muted-foreground text-right font-(family-name:--app-font-family-mono) text-[11px] tracking-normal"
                        >{{
                          s.status === "skipped" || s.status === "pending" ? "—" : formatDuration(s.duration_ms)
                        }}</span
                      >
                    </div>
                  </div>
                </div>

                <!-- Error -->
                <div
                  v-if="
                    (selectedRow.node.status === 'failed' || selectedRow.node.status === 'cancelled') &&
                    (selectedRow.node.error_code || selectedRow.node.error_message)
                  "
                  class="flex flex-col gap-2 rounded-md border border-solid border-(--td-error-color-3) bg-(--td-error-color-light) px-3 py-2.5"
                >
                  <div class="flex items-center gap-2">
                    <span
                      class="bg-destructive text-primary-foreground inline-flex size-4 shrink-0 items-center justify-center rounded-full text-[11px] font-bold"
                      >!</span
                    >
                    <span class="text-destructive text-xs font-semibold">{{
                      localizedErrorTitle(selectedRow.node.error_code) || t("knowledgeStages.detail.error")
                    }}</span>
                    <span
                      v-if="selectedRow.node.error_code"
                      class="bg-destructive text-primary-foreground ml-auto rounded-sm px-1.5 py-px font-(family-name:--app-font-family-mono) text-[10px] tracking-normal"
                      >{{ selectedRow.node.error_code }}</span
                    >
                  </div>
                  <pre
                    v-if="selectedRow.node.error_message"
                    class="border-border bg-card text-muted-foreground m-0 max-h-40 overflow-auto rounded-(--td-radius-default) border border-solid px-2.5 py-2 font-(family-name:--app-font-family-mono) text-[11px] tracking-normal [word-break:break-word] whitespace-pre-wrap"
                    >{{ selectedRow.node.error_message }}</pre>
                </div>

                <div
                  v-if="!selectedRow.node.span_id && !selectedRow.node.started_at"
                  class="border-border bg-muted text-muted-foreground rounded-md border-0 border-l-2 border-solid px-3 py-2.5 text-xs"
                >
                  {{ t("knowledgeStages.detail.placeholderHint") }}
                </div>
              </template>

              <!-- Input / Output / Metadata tabs -->
              <template v-else-if="detailTab === 'input' || detailTab === 'output' || detailTab === 'metadata'">
                <div
                  v-if="!tabHasContent(detailTab)"
                  class="text-placeholder flex items-center justify-center py-12 text-[13px]"
                >
                  <span>{{
                    detailTab === "metadata"
                      ? t("knowledgeStages.detail.metadataEmpty")
                      : t("knowledgeStages.detail.empty")
                  }}</span>
                </div>
                <template v-else>
                  <div class="flex flex-col gap-2">
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-muted-foreground text-[11px] font-medium tracking-[0.5px] uppercase">{{
                        t("knowledgeStages.detail." + detailTab)
                      }}</span>
                      <button
                        type="button"
                        data-slot="section-action"
                        class="border-border bg-card text-muted-foreground hover:border-primary hover:bg-primary hover:text-primary-foreground inline-flex items-center gap-1 rounded-(--td-radius-default) border border-solid px-2 py-[3px] text-[11px] transition-colors duration-150"
                        @click="copyValue((selectedRow.node as any)[detailTab])"
                      >
                        <CopyIcon class="size-3.5" />
                        <span>{{ t("knowledgeStages.copy") }}</span>
                      </button>
                    </div>

                    <div
                      v-if="isObjectWithKeys((selectedRow.node as any)[detailTab])"
                      class="divide-muted border-border bg-card flex flex-col divide-y divide-solid overflow-hidden rounded-md border border-solid"
                    >
                      <div
                        v-for="entry in buildKvEntries((selectedRow.node as any)[detailTab])"
                        :key="entry.key"
                        class="bg-card grid min-w-0 grid-cols-[130px_1fr] items-start gap-3 px-3 py-2 text-xs"
                      >
                        <span
                          class="text-muted-foreground truncate font-(family-name:--app-font-family-mono) text-[11px] font-medium tracking-normal"
                          >{{ entry.key }}</span
                        >
                        <div
                          class="text-foreground flex min-w-0 flex-col items-stretch gap-1 [overflow-wrap:anywhere] [word-break:break-word]"
                        >
                          <span
                            v-if="entry.kind === 'bool'"
                            class="font-(family-name:--app-font-family-mono) text-[11px] font-medium tracking-normal"
                            :class="entry.raw ? 'text-success' : 'text-destructive'"
                            >{{ entry.display }}</span
                          >
                          <span v-else-if="entry.kind === 'scalar'" class="text-xs">{{ entry.display }}</span>
                          <!-- Short payloads render inline so the user
                               sees the data without an extra click. The
                               summary chip ("Array · 3") is shown above
                               the JSON for context. -->
                          <div v-else-if="entry.defaultExpanded" class="flex min-w-0 flex-col gap-1">
                            <span
                              class="text-placeholder font-(family-name:--app-font-family-mono) text-[11px] tracking-normal"
                              >{{ entry.display }}</span
                            >
                            <pre
                              class="border-border bg-muted text-foreground m-0 max-h-[360px] overflow-auto rounded-md border border-solid px-3 py-2.5 font-(family-name:--app-font-family-mono) text-[11px] leading-[1.6] tracking-normal [word-break:break-word] whitespace-pre-wrap"
                              >{{ prettyJSON(entry.raw) }}</pre>
                          </div>
                          <div v-else class="flex min-w-0 flex-col gap-1.5">
                            <button
                              type="button"
                              data-slot="json-toggle"
                              class="group/toggle text-muted-foreground flex flex-wrap items-baseline gap-2 text-left text-xs"
                              @click.stop="toggleJsonKey(detailTab, entry.key)"
                            >
                              <span
                                class="text-muted-foreground font-(family-name:--app-font-family-mono) text-[11px] tracking-normal"
                                >{{ entry.display }}</span
                              >
                              <span class="text-primary text-[11px] font-medium group-hover/toggle:underline">{{
                                isJsonExpanded(detailTab, entry.key)
                                  ? t("knowledgeStages.detail.hideJson")
                                  : t("knowledgeStages.detail.showJson")
                              }}</span>
                            </button>
                            <pre
                              v-if="isJsonExpanded(detailTab, entry.key)"
                              class="border-border bg-muted text-foreground m-0 max-h-[360px] overflow-auto rounded-md border border-solid px-3 py-2.5 font-(family-name:--app-font-family-mono) text-[11px] leading-[1.6] tracking-normal [word-break:break-word] whitespace-pre-wrap"
                              >{{ prettyJSON(entry.raw) }}</pre>
                          </div>
                        </div>
                      </div>
                    </div>
                    <pre
                      v-else
                      class="border-border bg-muted text-foreground m-0 max-h-[360px] overflow-auto rounded-md border border-solid px-3 py-2.5 font-(family-name:--app-font-family-mono) text-[11px] leading-[1.6] tracking-normal [word-break:break-word] whitespace-pre-wrap"
                      >{{ prettyJSON((selectedRow.node as any)[detailTab]) }}</pre>
                  </div>
                </template>
              </template>

              <!-- Raw JSON tab -->
              <template v-else-if="detailTab === 'raw'">
                <div class="flex flex-col gap-2">
                  <div class="flex items-center justify-between gap-2">
                    <span class="text-muted-foreground text-[11px] font-medium tracking-[0.5px] uppercase">{{
                      t("knowledgeStages.tab.raw")
                    }}</span>
                    <button
                      type="button"
                      data-slot="section-action"
                      class="border-border bg-card text-muted-foreground hover:border-primary hover:bg-primary hover:text-primary-foreground inline-flex items-center gap-1 rounded-(--td-radius-default) border border-solid px-2 py-[3px] text-[11px] transition-colors duration-150"
                      @click="copyValue(selectedRow.node)"
                    >
                      <CopyIcon class="size-3.5" />
                      <span>{{ t("knowledgeStages.copy") }}</span>
                    </button>
                  </div>
                  <pre
                    class="border-border bg-muted text-foreground m-0 max-h-[480px] overflow-auto rounded-md border border-solid px-3 py-2.5 font-(family-name:--app-font-family-mono) text-[11px] leading-[1.6] tracking-normal [word-break:break-word] whitespace-pre-wrap"
                    >{{ prettyJSON(selectedRow.node) }}</pre>
                </div>
              </template>
            </div>
          </template>
        </div>
      </div>
    </template>
  </div>
</template>

<style>
/*
 * Only the keyframes stay as CSS: utilities reference them by name through
 * `animate-[…]`, and Tailwind has no way to declare a keyframe inline.
 *
 * Deliberately NOT scoped. A scoped block renames every @keyframes it
 * declares (`kpSpin` becomes `kpSpin-<hash>`) and rewrites only the
 * `animation` declarations inside that same block, so a utility class,
 * which lives in the global stylesheet, would name a keyframe that no
 * longer exists. The `kp` prefix keeps the global names from colliding.
 */

/* The pulsing dot of the LIVE badge, the running status dot and the "now" cursor. */
@keyframes kpLivePulse {
  0%,
  100% {
    opacity: 1;
    transform: scale(1);
  }

  50% {
    opacity: 0.45;
    transform: scale(0.8);
  }
}

/* The diagonal stripes crawling along a running bar. */
@keyframes kpStripes {
  to {
    background-position: 14px 0;
  }
}

/* Indeterminate sweep on the running bar — gives obvious motion while
   waiting for the next poll. */
@keyframes kpSweep {
  0% {
    transform: translateX(-100%);
  }

  100% {
    transform: translateX(100%);
  }
}
</style>
