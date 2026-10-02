<template>
  <div class="w-full">
    <div class="mb-4">
      <div class="mb-1.5 inline-flex max-w-full items-center gap-1.5">
        <h3 class="text-foreground m-0 [font-family:var(--app-font-family)] text-xl font-semibold">
          {{ t("knowledgeEditor.activity.title") }}
        </h3>
        <!-- The chat view's suggested-questions refresh button, restated in utilities. -->
        <button
          type="button"
          data-slot="icon-button"
          class="text-placeholder enabled:hover:text-primary enabled:hover:bg-muted enabled:active:bg-muted inline-flex size-5 shrink-0 items-center justify-center rounded-md transition-[color,background-color] duration-200 ease-[cubic-bezier(0.16,1,0.3,1)] disabled:opacity-70"
          :disabled="loading"
          :title="t('knowledgeEditor.activity.refresh')"
          :aria-label="t('knowledgeEditor.activity.refresh')"
          @click="reload"
        >
          <Loader2Icon v-if="loading" class="size-3 animate-[spin_0.8s_linear_infinite]" />
          <RefreshCwIcon v-else class="size-3" />
        </button>
      </div>
      <p class="text-placeholder m-0 [font-family:var(--app-font-family)] text-sm leading-[22px]">
        {{ t("knowledgeEditor.activity.description") }}
      </p>
      <p
        v-if="hasActiveFilters"
        class="text-muted-foreground mx-0 mt-1.5 mb-0 flex flex-wrap items-center gap-2 text-[13px] leading-5"
      >
        <span>{{ filterSummaryText }}</span>
        <button
          type="button"
          data-slot="text-button"
          class="text-primary cursor-pointer text-[13px] leading-5 hover:underline"
          @click="clearFilters"
        >
          {{ t("knowledgeEditor.activity.clearFilters") }}
        </button>
      </p>
    </div>

    <div class="min-h-[280px]">
      <div v-if="error" class="flex min-h-[240px] flex-col items-center justify-center">
        <Alert variant="destructive" class="max-w-xl">
          <CircleAlertIcon />
          <AlertTitle>{{ error }}</AlertTitle>
          <AlertAction>
            <Button size="sm" variant="outline" @click="reload">{{ t("knowledgeEditor.activity.retry") }}</Button>
          </AlertAction>
        </Alert>
      </div>

      <!-- Rendered alongside the error, as before: the alert sits above the table rather than replacing it. -->
      <div ref="scrollRoot" class="flex min-h-0 [scrollbar-width:thin] flex-col overflow-x-hidden overflow-y-visible">
        <div class="border-border bg-card overflow-x-auto rounded-[10px] border">
          <Table>
            <TableHeader>
              <TableRow
                class="bg-muted hover:bg-muted sticky top-0 z-[2] shadow-[inset_0_-1px_0_var(--td-component-stroke)]"
              >
                <TableHead class="w-[116px] px-4 py-3.5 text-[13px] font-semibold">{{
                  t("knowledgeEditor.activity.columns.time")
                }}</TableHead>
                <TableHead class="w-[132px] px-4 py-3.5 text-[13px] font-semibold">
                  <div class="inline-flex max-w-full items-center gap-1">
                    <span>{{ t("knowledgeEditor.activity.columns.action") }}</span>
                    <Popover v-model:open="actionFilterOpen">
                      <PopoverTrigger as-child>
                        <button
                          type="button"
                          data-slot="icon-button"
                          class="inline-flex size-[22px] cursor-pointer items-center justify-center rounded transition-colors duration-150"
                          :class="
                            action
                              ? 'bg-primary/10 text-primary'
                              : 'text-placeholder hover:bg-accent hover:text-muted-foreground bg-transparent'
                          "
                          :aria-label="t('knowledgeEditor.activity.columns.action')"
                        >
                          <ListFilterIcon class="size-3.5" />
                        </button>
                      </PopoverTrigger>
                      <PopoverContent align="start" class="w-auto p-1.5">
                        <div class="flex max-h-[min(360px,60vh)] max-w-[280px] min-w-[160px] flex-col overflow-hidden">
                          <div class="flex min-h-0 flex-1 [scrollbar-width:thin] flex-col gap-px overflow-y-auto">
                            <button
                              v-for="item in actionFilterList"
                              :key="item.value || '__all__'"
                              type="button"
                              data-slot="menu-option"
                              class="flex cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 text-left text-[13px] leading-[1.4] transition-colors duration-150"
                              :class="
                                (action ?? '') === item.value
                                  ? 'bg-primary/10 text-primary font-medium'
                                  : 'text-foreground hover:bg-muted bg-transparent'
                              "
                              @click="selectActionFilter(item.value)"
                            >
                              <span class="min-w-0 flex-1 truncate">{{ item.label }}</span>
                              <CheckIcon v-if="(action ?? '') === item.value" class="text-primary size-3.5 shrink-0" />
                            </button>
                          </div>
                        </div>
                      </PopoverContent>
                    </Popover>
                  </div>
                </TableHead>
                <TableHead class="min-w-[180px] px-4 py-3.5 text-[13px] font-semibold">{{
                  t("knowledgeEditor.activity.columns.target")
                }}</TableHead>
                <TableHead class="w-[120px] px-4 py-3.5 text-[13px] font-semibold">{{
                  t("knowledgeEditor.activity.columns.actor")
                }}</TableHead>
                <TableHead class="w-[108px] px-4 py-3.5 text-center text-[13px] font-semibold">
                  <div class="inline-flex w-full items-center justify-center gap-1">
                    <span>{{ t("knowledgeEditor.activity.columns.outcome") }}</span>
                    <Popover v-model:open="outcomeFilterOpen">
                      <PopoverTrigger as-child>
                        <button
                          type="button"
                          data-slot="icon-button"
                          class="inline-flex size-[22px] cursor-pointer items-center justify-center rounded transition-colors duration-150"
                          :class="
                            outcome
                              ? 'bg-primary/10 text-primary'
                              : 'text-placeholder hover:bg-accent hover:text-muted-foreground bg-transparent'
                          "
                          :aria-label="t('knowledgeEditor.activity.columns.outcome')"
                        >
                          <ListFilterIcon class="size-3.5" />
                        </button>
                      </PopoverTrigger>
                      <PopoverContent align="end" class="w-auto p-1.5">
                        <div class="flex max-h-[min(360px,60vh)] max-w-[280px] min-w-[160px] flex-col overflow-hidden">
                          <div class="flex min-h-0 flex-1 [scrollbar-width:thin] flex-col gap-px overflow-y-auto">
                            <button
                              v-for="item in outcomeFilterList"
                              :key="item.value || '__all__'"
                              type="button"
                              data-slot="menu-option"
                              class="flex cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 text-left text-[13px] leading-[1.4] transition-colors duration-150"
                              :class="
                                (outcome ?? '') === item.value
                                  ? 'bg-primary/10 text-primary font-medium'
                                  : 'text-foreground hover:bg-muted bg-transparent'
                              "
                              @click="selectOutcomeFilter(item.value)"
                            >
                              <span class="min-w-0 flex-1 truncate">{{ item.label }}</span>
                              <CheckIcon v-if="(outcome ?? '') === item.value" class="text-primary size-3.5 shrink-0" />
                            </button>
                          </div>
                        </div>
                      </PopoverContent>
                    </Popover>
                  </div>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow
                v-for="entry in entries"
                :key="entry.id"
                class="hover:bg-accent cursor-pointer"
                @click="openDetail(entry)"
              >
                <TableCell class="px-4 py-3.5 align-middle">
                  <div class="flex flex-col gap-0.5 leading-[1.3]">
                    <span class="text-muted-foreground text-xs">{{ formatDatePart(entry.created_at) }}</span>
                    <span class="text-foreground text-[13px] font-medium [font-variant-numeric:tabular-nums]">{{
                      formatTimePart(entry.created_at)
                    }}</span>
                  </div>
                </TableCell>
                <TableCell class="px-4 py-3.5 align-middle">
                  <Badge :class="actionBadgeClass(entry.action)">
                    {{ actionLabel(entry.action) }}
                  </Badge>
                </TableCell>
                <TableCell class="px-4 py-3.5 align-middle whitespace-normal">
                  <div class="flex min-w-0 flex-col gap-1 py-0.5 leading-[1.35]">
                    <span v-if="targetSubject(entry)" class="text-foreground text-[13px] break-words">{{
                      targetSubject(entry)
                    }}</span>
                    <span
                      v-if="targetDiff(entry)"
                      class="text-muted-foreground [font-family:var(--td-font-family-mono,monospace)] text-xs leading-[1.4] break-all"
                      >{{ targetDiff(entry) }}</span
                    >
                    <span v-else-if="!targetSubject(entry)" class="text-placeholder">—</span>
                  </div>
                </TableCell>
                <TableCell class="px-4 py-3.5 align-middle">
                  <div class="min-w-0">
                    <span class="text-foreground truncate text-[13px] font-medium">{{ actorLabel(entry) }}</span>
                  </div>
                </TableCell>
                <TableCell class="px-4 py-3.5 text-center align-middle">
                  <Badge :class="outcomeBadgeClass(entry.outcome)">
                    {{ outcomeLabel(entry.outcome) }}
                  </Badge>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>

          <div v-if="!entries.length && !loading" class="flex flex-col items-center gap-2 py-6">
            <Empty>
              <EmptyDescription>{{ emptyDescription }}</EmptyDescription>
            </Empty>
          </div>
          <div
            v-else-if="!entries.length && loading"
            class="text-muted-foreground flex justify-center gap-2 py-10 text-sm"
          >
            <Loader2Icon class="size-5 animate-spin" />
          </div>
        </div>

        <div ref="loadSentinel" class="pointer-events-none h-px w-full" aria-hidden="true" />

        <div
          v-if="loading && entries.length > 0"
          class="text-muted-foreground flex items-center justify-center gap-2.5 p-3 text-xs"
        >
          <Loader2Icon class="size-4 animate-spin" />
          <span>{{ t("knowledgeEditor.activity.loadingMore") }}</span>
        </div>
        <p v-else-if="!hasMore && entries.length > 0" class="text-placeholder m-0 px-0 pt-2 pb-3.5 text-center text-xs">
          {{ t("knowledgeEditor.activity.end") }}
        </p>
      </div>
    </div>

    <!-- SettingDrawer sits on the z-[2500] layer, above the z-[1000] editor modal. -->
    <SettingDrawer
      v-model:visible="detailVisible"
      class="kb-activity-detail-drawer"
      :title="detailTitle"
      :description="detailDescription"
      :icon="ClipboardPasteIcon"
      width="640px"
      :min-width="480"
      :max-width="960"
      storage-key="setting-drawer:width:kb-activity-detail"
      hide-footer
    >
      <template v-if="selectedEntry">
        <section class="border-border flex flex-col gap-2.5 border-b py-4 first:pt-0 last:border-b-0">
          <h4
            class="text-foreground before:bg-primary m-0 mb-1 flex items-center gap-2 text-[13px] font-semibold select-none before:h-3.5 before:w-[3px] before:shrink-0 before:rounded-sm before:content-['']"
          >
            {{ t("knowledgeEditor.activity.drawer.sectionSummary") }}
          </h4>
          <dl class="m-0 flex flex-col gap-2.5">
            <div
              v-for="field in summaryFields(selectedEntry)"
              :key="field.key"
              class="m-0 grid grid-cols-[88px_minmax(0,1fr)] items-baseline gap-3"
            >
              <dt class="text-placeholder m-0 text-xs leading-[1.45] whitespace-nowrap">{{ field.label }}</dt>
              <dd class="text-foreground m-0 text-[13px] leading-[1.55] break-all" :title="field.value">
                {{ field.value }}
              </dd>
            </div>
          </dl>
        </section>

        <section
          v-if="identifierFields(selectedEntry).length > 0"
          class="border-border flex flex-col gap-2.5 border-b py-4 first:pt-0 last:border-b-0"
        >
          <h4
            class="text-foreground before:bg-primary m-0 mb-1 flex items-center gap-2 text-[13px] font-semibold select-none before:h-3.5 before:w-[3px] before:shrink-0 before:rounded-sm before:content-['']"
          >
            {{ t("knowledgeEditor.activity.drawer.sectionIdentifiers") }}
          </h4>
          <dl class="m-0 flex flex-col gap-2.5">
            <div
              v-for="field in identifierFields(selectedEntry)"
              :key="field.key"
              class="m-0 grid grid-cols-[88px_minmax(0,1fr)] items-baseline gap-3"
            >
              <dt class="text-placeholder m-0 text-xs leading-[1.45] whitespace-nowrap">{{ field.label }}</dt>
              <dd
                class="mono text-foreground m-0 [font-family:var(--td-font-family-mono,ui-monospace,SFMono-Regular,Menlo,Consolas,monospace)] text-[13px] leading-[1.55] break-all"
                :title="field.value"
              >
                {{ field.value }}
              </dd>
            </div>
          </dl>
        </section>

        <section
          v-if="taskFields(selectedEntry).length > 0"
          class="border-border flex flex-col gap-2.5 border-b py-4 first:pt-0 last:border-b-0"
        >
          <h4
            class="text-foreground before:bg-primary m-0 mb-1 flex items-center gap-2 text-[13px] font-semibold select-none before:h-3.5 before:w-[3px] before:shrink-0 before:rounded-sm before:content-['']"
          >
            {{ t("knowledgeEditor.activity.drawer.sectionTask") }}
          </h4>
          <dl class="m-0 flex flex-col gap-2.5">
            <div
              v-for="field in taskFields(selectedEntry)"
              :key="field.key"
              class="m-0 grid grid-cols-[88px_minmax(0,1fr)] items-baseline gap-3"
            >
              <dt class="text-placeholder m-0 text-xs leading-[1.45] whitespace-nowrap">{{ field.label }}</dt>
              <dd
                class="text-foreground m-0 text-[13px] leading-[1.55] break-all"
                :class="
                  field.key.endsWith('_id')
                    ? 'mono [font-family:var(--td-font-family-mono,ui-monospace,SFMono-Regular,Menlo,Consolas,monospace)]'
                    : ''
                "
                :title="field.value"
              >
                {{ field.value }}
              </dd>
            </div>
          </dl>
        </section>

        <section class="border-border flex flex-col gap-2.5 border-b py-4 first:pt-0 last:border-b-0">
          <h4
            class="text-foreground before:bg-primary m-0 mb-1 flex items-center gap-2 text-[13px] font-semibold select-none before:h-3.5 before:w-[3px] before:shrink-0 before:rounded-sm before:content-['']"
          >
            {{ t("knowledgeEditor.activity.expanded.details") }}
          </h4>
          <pre
            class="border-border bg-card text-foreground m-0 max-h-[min(420px,50vh)] overflow-auto rounded-lg border px-3.5 py-3 [font-family:var(--td-font-family-mono,ui-monospace,SFMono-Regular,Menlo,Consolas,monospace)] text-xs leading-[1.55] break-all whitespace-pre-wrap"
            >{{ detailsJSON(selectedEntry) }}</pre>
        </section>
      </template>
    </SettingDrawer>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import {
  CheckIcon,
  CircleAlertIcon,
  ClipboardPasteIcon,
  ListFilterIcon,
  Loader2Icon,
  RefreshCwIcon,
} from "@lucide/vue";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import { Alert, AlertAction, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Empty, EmptyDescription } from "@/components/ui/empty";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { AUDIT_ACTION_I18N_ROOTS } from "@/i18n/auditActionRegistry";
import { auditActionLabel } from "@/i18n/auditActionLabel";
import { listKnowledgeBaseActivity, type KnowledgeBaseActivity } from "@/api/knowledge-base";
import type { AuditOutcome } from "@/api/tenant/audit-log";
import { useAuthStore } from "@/stores/auth";

interface DetailField {
  key: string;
  label: string;
  value: string;
}

const props = defineProps<{
  kbId: string;
  active?: boolean;
}>();

const { t, te, tm, locale } = useI18n();
const authStore = useAuthStore();

const entries = ref<KnowledgeBaseActivity[]>([]);
const cursor = ref(0);
const hasMore = ref(true);
const loading = ref(false);
const error = ref("");
const outcome = ref<AuditOutcome | undefined>();
const action = ref<string | undefined>();
const loadedOnce = ref(false);
const pageSize = 30;
const scrollRoot = ref<HTMLElement | null>(null);
const loadSentinel = ref<HTMLElement | null>(null);
const detailVisible = ref(false);
const selectedEntry = ref<KnowledgeBaseActivity | null>(null);
const actionFilterOpen = ref(false);
const outcomeFilterOpen = ref(false);
let scrollObserver: IntersectionObserver | null = null;

const outcomeOptions = computed(() =>
  (["accepted", "success", "failed", "partial", "canceled", "denied"] as AuditOutcome[]).map((value) => ({
    value,
    label: outcomeLabel(value),
  })),
);

// Column-header filter lists. Both host a leading "全部" entry (value '')
// so picking it clears the server-side filter for that dimension.
const outcomeFilterList = computed(() => [
  { label: t("knowledgeEditor.activity.allOutcomes"), value: "" },
  ...outcomeOptions.value.map((item) => ({ label: item.label, value: item.value })),
]);

const actionFilterList = computed(() => {
  const bag = tm("knowledgeEditor.activity.actions") as unknown;
  const list =
    bag !== null && typeof bag === "object"
      ? Object.keys(bag as Record<string, string>).map((key) => ({
          label: (bag as Record<string, string>)[key],
          value: key,
        }))
      : [];
  return [{ label: t("knowledgeEditor.activity.allActions"), value: "" }, ...list];
});

const hasActiveFilters = computed(() => Boolean(action.value || outcome.value));

const filterSummaryText = computed(() => {
  const parts: string[] = [];
  if (action.value) {
    parts.push(`${t("knowledgeEditor.activity.columns.action")}：${actionLabel(action.value)}`);
  }
  if (outcome.value) {
    parts.push(`${t("knowledgeEditor.activity.columns.outcome")}：${outcomeLabel(outcome.value)}`);
  }
  return parts.join("；");
});

function selectActionFilter(value: string) {
  actionFilterOpen.value = false;
  const next = value || undefined;
  if (action.value === next) return;
  action.value = next;
  if (props.active) reload();
}

function selectOutcomeFilter(value: string) {
  outcomeFilterOpen.value = false;
  const next = (value || undefined) as AuditOutcome | undefined;
  if (outcome.value === next) return;
  outcome.value = next;
  if (props.active) reload();
}

const emptyDescription = computed(() =>
  hasActiveFilters.value ? t("knowledgeEditor.activity.emptyFiltered") : t("knowledgeEditor.activity.empty"),
);

function clearFilters() {
  actionFilterOpen.value = false;
  outcomeFilterOpen.value = false;
  action.value = undefined;
  outcome.value = undefined;
  if (props.active) reload();
}

const detailTitle = computed(() => (selectedEntry.value ? actionLabel(selectedEntry.value.action) : ""));

const detailDescription = computed(() => (selectedEntry.value ? formatDateTime(selectedEntry.value.created_at) : ""));

function details(entry: KnowledgeBaseActivity): Record<string, unknown> {
  if (!entry.details) return {};
  if (typeof entry.details === "object") return entry.details;
  try {
    return JSON.parse(entry.details) as Record<string, unknown>;
  } catch {
    return {};
  }
}

function actionLabel(action: string): string {
  return auditActionLabel({ tm }, AUDIT_ACTION_I18N_ROOTS.kbActivity, action);
}

function outcomeLabel(value: AuditOutcome): string {
  const key = `knowledgeEditor.activity.outcomes.${value}`;
  return te(key) ? t(key) : value;
}

function outcomeBadgeClass(value: AuditOutcome): string {
  if (value === "accepted") return "bg-primary/10 text-primary hover:bg-primary/10";
  if (value === "success") return "bg-success/10 text-success hover:bg-success/10";
  if (value === "failed" || value === "denied") return "bg-destructive/10 text-destructive hover:bg-destructive/10";
  if (value === "partial" || value === "canceled") return "bg-warning/10 text-warning hover:bg-warning/10";
  return "bg-muted text-muted-foreground hover:bg-muted";
}

const taskDetailKeys = [
  "task_id",
  "trigger",
  "processing_status",
  "source_kb_id",
  "target_kb_id",
  "sync_log_id",
  "mode",
  "attempt",
  "count",
  "total",
  "processed",
  "failed",
  "skipped",
  "failure_stage",
] as const;

function taskFields(entry: KnowledgeBaseActivity): DetailField[] {
  const value = details(entry);
  return taskDetailKeys.flatMap((key) => {
    const raw = value[key];
    if (raw === undefined || raw === null || raw === "") return [];
    const labelKey = `knowledgeEditor.activity.detailFields.${key}`;
    const valueKey = `knowledgeEditor.activity.detailValues.${String(raw)}`;
    return [
      {
        key,
        label: te(labelKey) ? t(labelKey) : key,
        value: te(valueKey) ? t(valueKey) : String(raw),
      },
    ];
  });
}

function actionBadgeClass(action: string): string {
  if (action.includes("failed") || action.includes("denied"))
    return "bg-destructive/10 text-destructive border-destructive/40";
  if (action.includes("deleted") || action.includes("removed") || action.includes("canceled"))
    return "bg-warning/10 text-warning border-warning/40";
  if (action.includes("completed") || action.includes("created") || action.includes("added"))
    return "bg-success/10 text-success border-success/40";
  if (action.includes("started") || action.includes("updated") || action.includes("changed"))
    return "bg-primary/10 text-primary border-primary/40";
  return "bg-muted text-muted-foreground border-border";
}

function targetLabel(value: string): string {
  const bag = tm("knowledgeEditor.activity.targets") as unknown;
  const key = value || "knowledge_base";
  if (bag !== null && typeof bag === "object" && typeof (bag as Record<string, string>)[key] === "string") {
    return (bag as Record<string, string>)[key];
  }
  return value || t("knowledgeEditor.activity.knowledgeBase");
}

function targetSubject(entry: KnowledgeBaseActivity): string {
  const value = details(entry);
  const label = String(value.title || value.name || "").trim();
  const count = Number(value.count ?? 0);
  if (label && count > 1) {
    return t("knowledgeEditor.activity.titleWithCount", { title: label, count });
  }
  if (label) return label;
  // Aggregate / clone events carry no human-readable name — fall back
  // to the localized object-type label so the column always has a meaningful
  // primary subject instead of being blank or showing a raw identifier.
  return targetLabel(entry.target_type);
}

function targetDiff(entry: KnowledgeBaseActivity): string {
  const value = details(entry);
  // Data source: the connector type is more identifying than a row count.
  if (entry.target_type === "data_source" && value.type) {
    return String(value.type);
  }
  if (entry.action.startsWith("faq.import_")) {
    if (entry.action === "faq.import_started" && value.total !== undefined && value.total !== null) {
      return t("knowledgeEditor.activity.countItems", { count: Number(value.total) });
    }
    if (entry.action !== "faq.import_started" && value.total !== undefined && value.total !== null) {
      const success = Number(value.count ?? 0);
      const failed = Number(value.failed ?? 0);
      const skipped = Number(value.skipped ?? 0);
      return t("knowledgeEditor.activity.importSummary", { success, failed, skipped });
    }
  }
  // Aggregate events: show volume only when the primary subject has no title.
  const count = value.count !== undefined && value.count !== null ? Number(value.count) : Number(value.total ?? 0);
  if (count > 0 && !String(value.title || value.name || "").trim()) {
    return t("knowledgeEditor.activity.countItems", { count });
  }
  // Deliberately no raw target_id here — identifiers live in the detail drawer.
  return "";
}

function actorLabel(entry: KnowledgeBaseActivity): string {
  if (!entry.actor_user_id) return t("knowledgeEditor.activity.systemActor");
  const me = authStore.user;
  if (me?.id === entry.actor_user_id) {
    return me.username?.trim() || me.email?.trim() || entry.actor_user_id.slice(0, 8);
  }
  return entry.actor_user_id.slice(0, 8);
}

function formatDatePart(value: string): string {
  if (!value) return "—";
  try {
    return new Intl.DateTimeFormat(locale.value || "zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(new Date(value));
  } catch {
    return value;
  }
}

function formatTimePart(value: string): string {
  if (!value) return "";
  try {
    return new Intl.DateTimeFormat(locale.value || "zh-CN", {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    }).format(new Date(value));
  } catch {
    return "";
  }
}

function formatDateTime(value: string): string {
  const date = formatDatePart(value);
  const time = formatTimePart(value);
  return time ? `${date} ${time}` : date;
}

function summaryFields(entry: KnowledgeBaseActivity): DetailField[] {
  const fields: DetailField[] = [
    {
      key: "time",
      label: t("knowledgeEditor.activity.columns.time"),
      value: formatDateTime(entry.created_at),
    },
    {
      key: "actor",
      label: t("knowledgeEditor.activity.columns.actor"),
      value: actorLabel(entry),
    },
    {
      key: "action",
      label: t("knowledgeEditor.activity.columns.action"),
      value: actionLabel(entry.action),
    },
    {
      key: "outcome",
      label: t("knowledgeEditor.activity.columns.outcome"),
      value: outcomeLabel(entry.outcome),
    },
  ];

  const subject = targetSubject(entry);
  if (subject) {
    fields.push({
      key: "target",
      label: t("knowledgeEditor.activity.columns.target"),
      value: subject,
    });
  }

  const diff = targetDiff(entry);
  if (diff && diff !== subject) {
    fields.push({
      key: "targetDiff",
      label: t("knowledgeEditor.activity.drawer.targetChange"),
      value: diff,
    });
  }

  return fields;
}

function identifierFields(entry: KnowledgeBaseActivity): DetailField[] {
  const fields: DetailField[] = [];
  if (entry.target_type) {
    fields.push({
      key: "targetType",
      label: t("knowledgeEditor.activity.expanded.targetType"),
      value: targetLabel(entry.target_type),
    });
  }
  if (entry.target_id) {
    fields.push({
      key: "targetId",
      label: t("knowledgeEditor.activity.expanded.targetId"),
      value: entry.target_id,
    });
  }
  if (entry.actor_user_id) {
    fields.push({
      key: "actorId",
      label: t("knowledgeEditor.activity.expanded.actorId"),
      value: entry.actor_user_id,
    });
  }
  return fields;
}

function detailsJSON(entry: KnowledgeBaseActivity): string {
  const value = details(entry);
  if (!Object.keys(value).length) return "{}";
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function openDetail(entry: KnowledgeBaseActivity) {
  selectedEntry.value = entry;
  detailVisible.value = true;
}

async function fetchPage(reset = false) {
  if (!props.kbId || loading.value || (!reset && !hasMore.value)) return;
  loading.value = true;
  error.value = "";
  if (reset) {
    entries.value = [];
  }
  try {
    const response = await listKnowledgeBaseActivity(props.kbId, {
      limit: pageSize,
      after_id: reset ? undefined : cursor.value || undefined,
      outcome: outcome.value,
      action: action.value,
    });
    const page = response.data || [];
    entries.value = reset ? page : [...entries.value, ...page];
    cursor.value = response.next_cursor || 0;
    hasMore.value = !!response.next_cursor && page.length > 0;
    loadedOnce.value = true;
  } catch (err: any) {
    error.value = err?.message || t("knowledgeEditor.activity.loadFailed");
  } finally {
    loading.value = false;
  }
}

function reload() {
  cursor.value = 0;
  hasMore.value = true;
  void fetchPage(true);
}

function detachInfiniteScroll() {
  scrollObserver?.disconnect();
  scrollObserver = null;
}

function attachInfiniteScroll() {
  detachInfiniteScroll();
  const root = scrollRoot.value;
  const sentinel = loadSentinel.value;
  if (!root || !sentinel || error.value) return;

  scrollObserver = new IntersectionObserver(
    (hits) => {
      const hitBottom = hits.some((item) => item.isIntersecting);
      if (!hitBottom || !hasMore.value || loading.value) return;
      void fetchPage(false);
    },
    { root, rootMargin: "100px 0px", threshold: 0 },
  );
  scrollObserver.observe(sentinel);
}

watch(
  () => props.active,
  (active) => {
    if (active && !loadedOnce.value) reload();
    if (!active) detachInfiniteScroll();
  },
  { immediate: true },
);

watch(
  () => props.kbId,
  () => {
    loadedOnce.value = false;
    entries.value = [];
    if (props.active) reload();
  },
);

watch(
  [() => props.active, () => entries.value.length, () => error.value, hasMore],
  async ([active]) => {
    if (!active) {
      detachInfiniteScroll();
      return;
    }
    await nextTick();
    attachInfiniteScroll();
  },
  { flush: "post" },
);

onUnmounted(() => detachInfiniteScroll());
</script>
