<script setup lang="ts">
import { ref, watch, computed } from "vue";
import { useI18n } from "vue-i18n";
import { ListTreeIcon, Loader2Icon, RefreshCwIcon } from "@lucide/vue";
import { Drawer, DrawerContent, DrawerTitle } from "@/components/ui/drawer";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Button } from "@/components/ui/button";
import { getSyncLogs, type SyncLog, type SyncItemError } from "@/api/datasource";

const props = defineProps<{
  dataSourceId: string;
  dataSourceName?: string;
}>();
const visible = defineModel<boolean>("visible", { default: false });
const { t } = useI18n();

const logs = ref<SyncLog[]>([]);
const loading = ref(false);
const loadingMore = ref(false);
const hasMore = ref(false);
const expandedId = ref("");
const pageSize = 50;

async function fetchLogs(reset = true) {
  if (!props.dataSourceId) return;

  if (reset) {
    loading.value = true;
  } else {
    loadingMore.value = true;
  }

  try {
    const offset = reset ? 0 : logs.value.length;
    const res = await getSyncLogs(props.dataSourceId, pageSize, offset);
    const items = res?.data || res || [];
    logs.value = reset ? items : [...logs.value, ...items];
    hasMore.value = items.length === pageSize;
  } catch {
    /* ignore */
  }

  if (reset) {
    loading.value = false;
  } else {
    loadingMore.value = false;
  }
}

watch(visible, (v) => {
  if (!v) return;
  expandedId.value = "";
  fetchLogs(true);
});

function toggleExpand(id: string) {
  expandedId.value = expandedId.value === id ? "" : id;
}

function loadMore() {
  if (loading.value || loadingMore.value || !hasMore.value) return;
  fetchLogs(false);
}

// --- Stats ---
const stats = computed(() => {
  const total = logs.value.length;
  const success = logs.value.filter((l) => l.status === "success").length;
  const failed = logs.value.filter((l) => l.status === "failed").length;
  const totalItems = logs.value.reduce((acc, l) => acc + (l.items_created || 0) + (l.items_updated || 0), 0);
  return { total, success, failed, totalItems };
});

// --- Helpers ---

function statusColorClass(status: string): string {
  switch (status) {
    case "success":
      return "text-success bg-success";
    case "running":
      return "text-primary bg-primary";
    case "failed":
      return "text-destructive bg-destructive";
    case "partial":
      return "text-warning bg-warning";
    default:
      return "text-placeholder bg-placeholder";
  }
}

function formatTime(ts: string | null) {
  if (!ts) return "--";
  const d = new Date(ts);
  if (isNaN(d.getTime())) return "--";
  return d.toLocaleString(undefined, {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

function formatDate(ts: string | null) {
  if (!ts) return "";
  const d = new Date(ts);
  if (isNaN(d.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

function formatHourMin(ts: string | null) {
  if (!ts) return "";
  const d = new Date(ts);
  if (isNaN(d.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function duration(log: SyncLog) {
  if (!log.started_at || !log.finished_at) return "--";
  const ms = new Date(log.finished_at).getTime() - new Date(log.started_at).getTime();
  if (ms < 0) return "--";
  if (ms < 1000) return `<1s`;
  const sec = Math.round(ms / 1000);
  if (sec < 60) return `${sec}s`;
  return `${Math.floor(sec / 60)}m${sec % 60}s`;
}

function hasPills(log: SyncLog) {
  return (
    log.items_created > 0 ||
    log.items_updated > 0 ||
    log.items_deleted > 0 ||
    log.items_skipped > 0 ||
    log.items_failed > 0
  );
}

// Live progress line for a running sync. The backend throttle-writes
// {progress: {action, target}} into result while the run is in flight (the
// final summary overwrites it), so this only ever renders on running logs.
function progressText(log: SyncLog): string {
  if (log.status !== "running") return "";
  const p = (log.result as any)?.progress;
  if (!p || !p.action) return "";
  const key = `datasource.progress.${p.action}`;
  const localised = t(key, { name: p.target || "" });
  return localised === key ? `${p.action} ${p.target || ""}` : localised;
}

// Cap the per-item failure list so a sync that failed thousands of documents
// doesn't render an unbounded wall of text; the remainder is summarised.
const FAILED_ITEMS_CAP = 50;

function failedItems(log: SyncLog): SyncItemError[] {
  return (log.result?.errors || []).slice(0, FAILED_ITEMS_CAP);
}

// Render one failure sample. The backend sends a stable i18n `code` (+ params)
// so the reason is localised to the viewer's language; `message` is the fallback
// for old logs / codes this client doesn't know. The document title is kept
// separate and prefixed as "title — reason".
function formatSyncError(e: SyncItemError): string {
  let reason = "";
  if (e.code) {
    const key = `datasource.syncError.${e.code}`;
    const localised = t(key, (e.params || {}) as Record<string, unknown>);
    reason = localised === key ? e.message || e.code : localised;
  } else {
    reason = e.message || "";
  }
  return e.title ? (reason ? `${e.title} — ${reason}` : e.title) : reason;
}

// Group logs by date
const groupedLogs = computed(() => {
  const groups: { date: string; logs: SyncLog[] }[] = [];
  let currentDate = "";
  for (const log of logs.value) {
    const d = formatDate(log.started_at);
    if (d !== currentDate) {
      currentDate = d;
      groups.push({ date: d, logs: [] });
    }
    groups[groups.length - 1].logs.push(log);
  }
  return groups;
});

const drawerTitle = computed(() =>
  props.dataSourceName ? `${t("datasource.syncHistory")} · ${props.dataSourceName}` : t("datasource.syncHistory"),
);

// Only the very last entry of the whole timeline ends the rail at its dot;
// every other entry, including the last of each date group, keeps the rail
// running on to the next group.
function isLastLog(log: SyncLog): boolean {
  return logs.value[logs.value.length - 1] === log;
}

function onOpenChange(open: boolean) {
  // Blur first, as SettingDrawer does: a focused control inside a drawer
  // that is tearing down can throw from its blur/resize handlers.
  if (!open && document.activeElement instanceof HTMLElement) document.activeElement.blur();
  visible.value = open;
}
</script>

<template>
  <!--
    Built on the Drawer primitives rather than SettingDrawer because
    SettingDrawer's header-extra slot sits below the title, where t-drawer
    put the refresh button beside it.
  -->
  <Drawer :open="visible" swipe-direction="right" @update:open="onOpenChange">
    <!-- The swipe-direction variants carry the primitive's own width, radius and
         border; overriding them under the same variant lets cn() replace them. -->
    <DrawerContent
      class="data-[swipe-direction=right]:w-[480px] data-[swipe-direction=right]:max-w-full data-[swipe-direction=right]:rounded-none data-[swipe-direction=right]:border-l-0 data-[swipe-direction=right]:sm:max-w-full"
    >
      <header
        class="flex w-full items-center justify-between gap-2 border-b border-[var(--td-border-level-1-color)] px-6 py-5"
      >
        <DrawerTitle class="text-foreground min-w-0 truncate text-base font-semibold">{{ drawerTitle }}</DrawerTitle>
        <Tooltip>
          <TooltipTrigger as-child>
            <Button
              variant="ghost"
              size="icon-xs"
              class="shrink-0"
              :disabled="loading"
              :aria-label="t('datasource.refreshLogs')"
              @click="fetchLogs()"
            >
              <RefreshCwIcon class="size-4" :class="loading ? 'animate-spin' : ''" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{{ t("datasource.refreshLogs") }}</TooltipContent>
        </Tooltip>
      </header>

      <div class="min-h-0 flex-1 overflow-y-auto px-6 py-5">
        <div v-if="loading" class="flex justify-center py-15">
          <Loader2Icon class="text-primary size-6 animate-spin" />
        </div>

        <div
          v-else-if="logs.length === 0"
          class="text-placeholder flex flex-col items-center justify-center gap-3 py-20 text-[13px]"
        >
          <ListTreeIcon class="size-10" />
          <p class="m-0">{{ t("datasource.noLogs") }}</p>
        </div>

        <template v-else>
          <!-- Summary -->
          <div class="mb-3 flex gap-2 border-b border-[var(--td-border-level-1-color)] pb-6">
            <div
              class="bg-card flex flex-1 flex-col gap-1 rounded-xl border border-[var(--td-border-level-1-color)] px-2 py-4 text-center shadow-[0_1px_2px_rgba(0,0,0,0.02)]"
            >
              <span class="text-foreground text-xl leading-[1.2] font-bold [font-variant-numeric:tabular-nums]">{{
                stats.total
              }}</span>
              <span class="text-placeholder text-[11px] font-medium tracking-[0.5px] uppercase">{{
                t("datasource.logSummary.total")
              }}</span>
            </div>
            <div
              class="bg-card flex flex-1 flex-col gap-1 rounded-xl border border-[var(--td-border-level-1-color)] px-2 py-4 text-center shadow-[0_1px_2px_rgba(0,0,0,0.02)]"
            >
              <span class="text-success text-xl leading-[1.2] font-bold [font-variant-numeric:tabular-nums]">{{
                stats.success
              }}</span>
              <span class="text-placeholder text-[11px] font-medium tracking-[0.5px] uppercase">{{
                t("datasource.logSummary.success")
              }}</span>
            </div>
            <div
              class="bg-card flex flex-1 flex-col gap-1 rounded-xl border border-[var(--td-border-level-1-color)] px-2 py-4 text-center shadow-[0_1px_2px_rgba(0,0,0,0.02)]"
            >
              <span class="text-destructive text-xl leading-[1.2] font-bold [font-variant-numeric:tabular-nums]">{{
                stats.failed
              }}</span>
              <span class="text-placeholder text-[11px] font-medium tracking-[0.5px] uppercase">{{
                t("datasource.logSummary.failed")
              }}</span>
            </div>
            <div
              class="bg-card flex flex-1 flex-col gap-1 rounded-xl border border-[var(--td-border-level-1-color)] px-2 py-4 text-center shadow-[0_1px_2px_rgba(0,0,0,0.02)]"
            >
              <span class="text-foreground text-xl leading-[1.2] font-bold [font-variant-numeric:tabular-nums]">{{
                stats.totalItems
              }}</span>
              <span class="text-placeholder text-[11px] font-medium tracking-[0.5px] uppercase">{{
                t("datasource.logSummary.items")
              }}</span>
            </div>
          </div>

          <!-- Timeline grouped by date -->
          <div class="flex flex-col">
            <div v-for="group in groupedLogs" :key="group.date" class="mb-2">
              <div
                class="text-placeholder bg-card sticky top-0 z-[1] pt-3 pb-2 pl-6 text-[11px] font-semibold tracking-[0.5px] uppercase"
              >
                {{ group.date }}
              </div>

              <div
                v-for="log in group.logs"
                :key="log.id"
                class="group relative mb-1 flex cursor-pointer"
                @click="toggleExpand(log.id)"
              >
                <!-- Dot -->
                <div
                  class="relative flex w-6 shrink-0 flex-col items-center before:absolute before:top-0 before:left-1/2 before:w-[1.5px] before:-translate-x-1/2 before:bg-[var(--td-border-level-1-color)] before:content-['']"
                  :class="isLastLog(log) ? 'before:bottom-1/2' : 'before:bottom-0'"
                >
                  <span
                    class="relative z-[1] mt-[18px] h-2 w-2 shrink-0 rounded-full shadow-[0_0_0_4px_var(--td-bg-color-container)]"
                    :class="statusColorClass(log.status).split(' ')[1]"
                  />
                </div>

                <!-- Content -->
                <div class="group-hover:bg-muted min-w-0 flex-1 rounded-[10px] px-3.5 py-3 transition-colors">
                  <div class="flex items-center gap-2">
                    <span class="text-[13px] font-medium" :class="statusColorClass(log.status).split(' ')[0]">
                      {{ t(`datasource.logStatus.${log.status}`) }}
                    </span>
                    <span class="text-placeholder text-xs [font-variant-numeric:tabular-nums]">{{
                      formatHourMin(log.started_at)
                    }}</span>
                    <span
                      v-if="log.finished_at"
                      class="text-placeholder ml-auto rounded bg-[var(--td-bg-color-component)] px-1.5 py-0.5 text-[11px] font-medium [font-variant-numeric:tabular-nums]"
                      >{{ duration(log) }}</span
                    >
                  </div>

                  <!-- Live progress for a running sync -->
                  <div v-if="progressText(log)" class="text-muted-foreground mt-1 max-w-full truncate text-xs">
                    {{ progressText(log) }}
                  </div>

                  <!-- Pills -->
                  <div v-if="hasPills(log)" class="mt-2 flex flex-wrap gap-1">
                    <span
                      v-if="log.items_created > 0"
                      class="bg-success/10 text-success rounded px-1.5 py-px text-[11px] leading-[18px] font-medium [font-variant-numeric:tabular-nums]"
                      >+{{ log.items_created }}</span
                    >
                    <span
                      v-if="log.items_updated > 0"
                      class="bg-primary/10 text-primary rounded px-1.5 py-px text-[11px] leading-[18px] font-medium [font-variant-numeric:tabular-nums]"
                      >~{{ log.items_updated }}</span
                    >
                    <span
                      v-if="log.items_deleted > 0"
                      class="bg-warning/10 text-warning rounded px-1.5 py-px text-[11px] leading-[18px] font-medium [font-variant-numeric:tabular-nums]"
                      >-{{ log.items_deleted }}</span
                    >
                    <span
                      v-if="log.items_skipped > 0"
                      class="text-placeholder rounded bg-[var(--td-bg-color-component)] px-1.5 py-px text-[11px] leading-[18px] font-medium [font-variant-numeric:tabular-nums]"
                      >{{ log.items_skipped }} {{ t("datasource.logMetric.skipped") }}</span
                    >
                    <span
                      v-if="log.items_failed > 0"
                      class="bg-destructive/10 text-destructive rounded px-1.5 py-px text-[11px] leading-[18px] font-medium [font-variant-numeric:tabular-nums]"
                      >{{ log.items_failed }} {{ t("datasource.logMetric.failed") }}</span
                    >
                  </div>

                  <!-- Expanded -->
                  <div
                    v-if="expandedId === log.id"
                    class="mt-3 flex flex-col gap-1.5 border-t border-dashed border-[var(--td-border-level-2-color)] pt-3"
                    @click.stop
                  >
                    <div class="text-foreground flex justify-between text-xs leading-5">
                      <span class="text-placeholder">{{ t("datasource.logDetail.startTime") }}</span>
                      <span>{{ formatTime(log.started_at) }}</span>
                    </div>
                    <div class="text-foreground flex justify-between text-xs leading-5">
                      <span class="text-placeholder">{{ t("datasource.logDetail.endTime") }}</span>
                      <span>{{ formatTime(log.finished_at) }}</span>
                    </div>
                    <div v-if="log.items_total > 0" class="text-foreground flex justify-between text-xs leading-5">
                      <span class="text-placeholder">{{ t("datasource.logMetric.total") }}</span>
                      <span>{{ log.items_total }}</span>
                    </div>
                    <!-- Localised failure summary; raw error_message only for a
                     pure infra failure with no per-document detail. -->
                    <div
                      v-if="log.items_failed > 0"
                      class="bg-destructive/10 text-destructive mt-2 rounded-md px-3 py-2 text-xs leading-normal break-words"
                    >
                      {{ t("datasource.logDetail.docsFailedSummary", { n: log.items_failed }) }}
                    </div>
                    <div
                      v-else-if="log.error_message"
                      class="bg-destructive/10 text-destructive mt-2 rounded-md px-3 py-2 text-xs leading-normal break-words"
                    >
                      {{ log.error_message }}
                    </div>

                    <!-- Per-item failures: which documents failed and why.
                     The true count is items_failed (a bounded int); result.errors
                     is only a capped sample the backend retains for display. -->
                    <div v-if="failedItems(log).length" class="mt-2 flex flex-col gap-0.5">
                      <div class="text-destructive mb-0.5 text-[11px] font-semibold">
                        {{ t("datasource.logDetail.failedItems") }} ({{ log.items_failed }})
                      </div>
                      <div
                        v-for="(e, i) in failedItems(log)"
                        :key="i"
                        class="bg-card text-muted-foreground truncate border-l-2 border-l-[var(--td-error-color-3)] px-2 py-0.5 text-[11px] leading-normal"
                        :title="formatSyncError(e)"
                      >
                        {{ formatSyncError(e) }}
                      </div>
                      <div
                        v-if="log.items_failed > failedItems(log).length"
                        class="text-placeholder px-2 py-0.5 text-[11px]"
                      >
                        {{
                          t("datasource.logDetail.failedItemsMore", { n: log.items_failed - failedItems(log).length })
                        }}
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="px-0 pt-4 pb-2 text-center">
              <Button v-if="hasMore" variant="outline" class="w-full" :disabled="loadingMore" @click="loadMore">
                <Loader2Icon v-if="loadingMore" class="animate-spin" />
                {{ t("common.loadMore") }}
              </Button>
              <span v-else class="text-placeholder text-xs">{{ t("common.noMoreData") }}</span>
            </div>
          </div>
        </template>
      </div>
    </DrawerContent>
  </Drawer>
</template>
