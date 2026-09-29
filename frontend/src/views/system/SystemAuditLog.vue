<template>
  <div class="flex min-h-0 w-full flex-col">
    <header class="mb-5 flex items-start justify-between gap-4">
      <div>
        <h2 class="text-foreground m-0 mb-2 text-xl font-semibold">
          {{ t("system.globalSettings.audit.tabLabel") }}
        </h2>
        <p class="text-muted-foreground m-0 text-sm leading-[1.5]">
          {{ t("system.globalSettings.audit.description") }}
        </p>
      </div>
      <button
        type="button"
        data-slot="icon-button"
        class="text-placeholder hover:enabled:bg-secondary hover:enabled:text-primary grid size-5 shrink-0 cursor-pointer place-items-center rounded-md transition-colors duration-200 ease-[cubic-bezier(0.16,1,0.3,1)] disabled:cursor-default disabled:opacity-70"
        :disabled="auditLoading"
        :title="t('system.globalSettings.audit.refresh')"
        :aria-label="t('system.globalSettings.audit.refresh')"
        @click="reloadAuditLog"
      >
        <Loader2Icon v-if="auditLoading" class="size-3 animate-spin" />
        <RefreshCwIcon v-else class="size-3" />
      </button>
    </header>

    <div class="flex min-h-0 flex-1 flex-col">
      <div v-if="auditError" class="flex min-h-0 flex-1 flex-col justify-start">
        <Alert class="border-transparent bg-[var(--td-error-color-light)]">
          <CircleAlertIcon class="text-destructive" />
          <AlertDescription class="text-foreground">{{ auditError }}</AlertDescription>
          <AlertAction>
            <Button size="sm" variant="outline" @click="reloadAuditLog">
              {{ t("system.globalSettings.audit.retry") }}
            </Button>
          </AlertAction>
        </Alert>
      </div>

      <div
        v-else-if="!auditLoading && auditEntries.length === 0"
        class="flex min-h-[280px] flex-1 flex-col items-center justify-center"
      >
        <Empty>
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <InboxIcon />
            </EmptyMedia>
          </EmptyHeader>
          <EmptyDescription>{{ t("system.globalSettings.audit.empty") }}</EmptyDescription>
        </Empty>
      </div>

      <div
        v-else
        ref="auditScrollRoot"
        class="max-h-[calc(100vh-260px)] min-h-0 flex-1 overflow-x-hidden overflow-y-auto"
      >
        <div class="border-border bg-card overflow-x-auto rounded-[10px] border">
          <Table class="table-fixed">
            <TableHeader>
              <TableRow class="hover:bg-transparent">
                <TableHead
                  class="bg-secondary text-placeholder sticky top-0 z-[2] h-auto w-[120px] px-4 py-3.5 text-[13px] font-semibold whitespace-normal shadow-[inset_0_-1px_0_var(--td-component-stroke)]"
                >
                  {{ t("system.globalSettings.audit.columns.time") }}
                </TableHead>
                <TableHead
                  class="bg-secondary text-placeholder sticky top-0 z-[2] h-auto w-[180px] px-4 py-3.5 text-[13px] font-semibold whitespace-normal shadow-[inset_0_-1px_0_var(--td-component-stroke)]"
                >
                  {{ t("system.globalSettings.audit.columns.actor") }}
                </TableHead>
                <TableHead
                  class="bg-secondary text-placeholder sticky top-0 z-[2] h-auto w-[150px] px-4 py-3.5 text-[13px] font-semibold whitespace-normal shadow-[inset_0_-1px_0_var(--td-component-stroke)]"
                >
                  {{ t("system.globalSettings.audit.columns.action") }}
                </TableHead>
                <TableHead
                  class="bg-secondary text-placeholder sticky top-0 z-[2] h-auto min-w-[240px] px-4 py-3.5 text-[13px] font-semibold whitespace-normal shadow-[inset_0_-1px_0_var(--td-component-stroke)]"
                >
                  {{ t("system.globalSettings.audit.columns.target") }}
                </TableHead>
                <TableHead
                  class="bg-secondary text-placeholder sticky top-0 z-[2] h-auto w-[80px] px-4 py-3.5 text-center text-[13px] font-semibold whitespace-normal shadow-[inset_0_-1px_0_var(--td-component-stroke)]"
                >
                  {{ t("system.globalSettings.audit.columns.outcome") }}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow
                v-for="row in auditEntries"
                :key="row.id"
                class="hover:bg-accent cursor-pointer"
                @click="openAuditDetail({ row })"
              >
                <TableCell class="px-4 py-3.5 align-middle whitespace-normal">
                  <div class="flex flex-col gap-0.5 leading-[1.3]">
                    <span class="text-muted-foreground text-xs">{{ formatAuditDatePart(row.created_at) }}</span>
                    <span class="text-foreground text-[13px] font-medium tabular-nums">
                      {{ formatAuditTimePart(row.created_at) }}
                    </span>
                  </div>
                </TableCell>
                <TableCell class="px-4 py-3.5 align-middle whitespace-normal">
                  <div class="flex min-w-0 flex-col gap-0.5 leading-[1.3]">
                    <span
                      class="text-foreground overflow-hidden text-[13px] font-medium text-ellipsis whitespace-nowrap"
                    >
                      {{
                        row.actor_user_id
                          ? auditActorLabel(row.actor_user_id)
                          : t("system.globalSettings.audit.systemActor")
                      }}
                    </span>
                    <span v-if="row.actor_role" class="text-muted-foreground text-xs">
                      {{ auditActorRoleLabel(row.actor_role) }}
                    </span>
                  </div>
                </TableCell>
                <TableCell class="px-4 py-3.5 align-middle whitespace-normal">
                  <Badge class="rounded-sm" :class="auditActionBadgeClass(row.action)">
                    {{ formatAuditAction(row.action) }}
                  </Badge>
                </TableCell>
                <TableCell class="px-4 py-3.5 align-middle whitespace-normal">
                  <div class="flex min-w-0 flex-col gap-1 py-0.5 leading-[1.35]">
                    <span
                      v-if="auditTargetKey(row)"
                      class="text-foreground font-mono text-[13px] font-medium break-all"
                    >
                      {{ auditTargetKey(row) }}
                    </span>
                    <span
                      v-if="auditTargetDiff(row)"
                      class="text-muted-foreground font-mono text-xs leading-[1.4] break-all"
                    >
                      {{ auditTargetDiff(row) }}
                    </span>
                    <span v-else-if="!auditTargetKey(row)" class="text-placeholder">—</span>
                  </div>
                </TableCell>
                <TableCell class="px-4 py-3.5 text-center align-middle whitespace-normal">
                  <Badge class="rounded-sm" :class="auditOutcomeBadgeClass(row.outcome)">
                    {{ t("system.globalSettings.audit.outcome." + row.outcome) }}
                  </Badge>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>

        <div ref="auditLoadSentinelEl" class="pointer-events-none h-px w-full" aria-hidden="true" />

        <div
          v-if="auditLoading && auditEntries.length > 0"
          class="text-muted-foreground flex items-center justify-center gap-2.5 p-3 text-xs"
        >
          <Loader2Icon class="size-3.5 animate-spin" />
          <span>{{ t("system.globalSettings.audit.loading") }}</span>
        </div>

        <p
          v-if="!auditHasMore && auditEntries.length > 0 && !auditLoading"
          class="m-0 pt-2 pb-3.5 text-center text-xs text-[var(--td-text-color-disabled)]"
        >
          {{ t("system.globalSettings.audit.end") }}
        </p>
      </div>
    </div>

    <SettingDrawer
      v-model:visible="auditDetailVisible"
      :title="auditDetailTitle"
      :description="auditDetailDescription"
      :icon="ClipboardPasteIcon"
      width="640px"
      :min-width="480"
      :max-width="960"
      storage-key="setting-drawer:width:system-audit-detail"
      hide-footer
    >
      <template v-if="selectedAuditEntry">
        <section class="border-border flex flex-col gap-3.5 border-b pt-3 pb-4 first:pt-0 last:border-b-0 last:pb-0">
          <h4
            class="text-foreground before:bg-primary m-0 mb-1 flex items-center gap-2 text-[13px] font-semibold select-none before:h-3.5 before:w-[3px] before:rounded-[2px] before:content-['']"
          >
            {{ t("system.globalSettings.audit.drawer.sectionSummary") }}
          </h4>
          <dl class="m-0 flex flex-col gap-2.5">
            <div
              v-for="field in auditSummaryFields(selectedAuditEntry)"
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
          v-if="auditIdentifierFields(selectedAuditEntry).length > 0"
          class="border-border flex flex-col gap-3.5 border-b pt-3 pb-4 first:pt-0 last:border-b-0 last:pb-0"
        >
          <h4
            class="text-foreground before:bg-primary m-0 mb-1 flex items-center gap-2 text-[13px] font-semibold select-none before:h-3.5 before:w-[3px] before:rounded-[2px] before:content-['']"
          >
            {{ t("system.globalSettings.audit.drawer.sectionIdentifiers") }}
          </h4>
          <dl class="m-0 flex flex-col gap-2.5">
            <div
              v-for="field in auditIdentifierFields(selectedAuditEntry)"
              :key="field.key"
              class="m-0 grid grid-cols-[88px_minmax(0,1fr)] items-baseline gap-3"
            >
              <dt class="text-placeholder m-0 text-xs leading-[1.45] whitespace-nowrap">{{ field.label }}</dt>
              <dd class="text-foreground m-0 font-mono text-[13px] leading-[1.55] break-all" :title="field.value">
                {{ field.value }}
              </dd>
            </div>
          </dl>
        </section>

        <section
          v-if="auditRequestFields(selectedAuditEntry).length > 0"
          class="border-border flex flex-col gap-3.5 border-b pt-3 pb-4 first:pt-0 last:border-b-0 last:pb-0"
        >
          <h4
            class="text-foreground before:bg-primary m-0 mb-1 flex items-center gap-2 text-[13px] font-semibold select-none before:h-3.5 before:w-[3px] before:rounded-[2px] before:content-['']"
          >
            {{ t("system.globalSettings.audit.drawer.sectionRequest") }}
          </h4>
          <dl class="m-0 flex flex-col gap-2.5">
            <div
              v-for="field in auditRequestFields(selectedAuditEntry)"
              :key="field.key"
              class="m-0 grid grid-cols-[88px_minmax(0,1fr)] items-baseline gap-3"
            >
              <dt class="text-placeholder m-0 text-xs leading-[1.45] whitespace-nowrap">{{ field.label }}</dt>
              <dd class="text-foreground m-0 font-mono text-[13px] leading-[1.55] break-all" :title="field.value">
                {{ field.value }}
              </dd>
            </div>
          </dl>
        </section>

        <section class="border-border flex flex-col gap-3.5 border-b pt-3 pb-4 first:pt-0 last:border-b-0 last:pb-0">
          <h4
            class="text-foreground before:bg-primary m-0 mb-1 flex items-center gap-2 text-[13px] font-semibold select-none before:h-3.5 before:w-[3px] before:rounded-[2px] before:content-['']"
          >
            {{ t("system.globalSettings.audit.expanded.details") }}
          </h4>
          <pre
            class="border-border bg-card text-foreground m-0 max-h-[min(420px,50vh)] overflow-auto rounded-[8px] border px-3.5 py-3 font-mono text-xs leading-[1.55] break-all whitespace-pre-wrap"
            >{{ auditDetailsJSON(selectedAuditEntry) }}</pre>
        </section>
      </template>
    </SettingDrawer>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { listSystemAuditLog, type AuditAction, type AuditLog, type AuditOutcome } from "@/api/system";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import { AUDIT_ACTION_I18N_ROOTS } from "@/i18n/auditActionRegistry";
import { auditActionLabel } from "@/i18n/auditActionLabel";
import { useAuthStore } from "@/stores/auth";

import { Alert, AlertAction, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia } from "@/components/ui/empty";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { CircleAlertIcon, ClipboardPasteIcon, InboxIcon, Loader2Icon, RefreshCwIcon } from "@lucide/vue";

interface AuditDetailField {
  key: string;
  label: string;
  value: string;
}

const authStore = useAuthStore();
const { t, tm, te, locale } = useI18n();

const auditEntries = ref<AuditLog[]>([]);
const auditLoading = ref(false);
const auditError = ref("");
const auditCursor = ref<number>(0);
const auditHasMore = ref(true);
const AUDIT_PAGE_SIZE = 50;

const auditScrollRoot = ref<HTMLElement | null>(null);
const auditLoadSentinelEl = ref<HTMLElement | null>(null);
let auditScrollObserver: IntersectionObserver | null = null;

const auditDetailVisible = ref(false);
const selectedAuditEntry = ref<AuditLog | null>(null);

/**
 * Tinted-badge colours matching the old t-tag themes. The action tag was
 * "light-outline" (tint plus a border in the same hue); the outcome tag was
 * plain "light", so only the action map carries a border colour.
 */
function auditActionBadgeClass(action: AuditAction): string {
  switch (auditActionTheme(action)) {
    case "success":
      return "border-success/40 bg-success/10 text-success";
    case "warning":
      return "border-warning/40 bg-warning/10 text-warning";
    case "danger":
      return "border-destructive/40 bg-destructive/10 text-destructive";
    case "primary":
      return "border-primary/40 bg-primary/10 text-primary";
    default:
      return "border-border bg-muted text-muted-foreground";
  }
}

function auditOutcomeBadgeClass(outcome: AuditOutcome): string {
  switch (auditOutcomeTheme(outcome)) {
    case "success":
      return "bg-success/10 text-success";
    case "danger":
      return "bg-destructive/10 text-destructive";
    default:
      return "bg-muted text-muted-foreground";
  }
}

function formatAuditDatePart(s: string | undefined): string {
  if (!s) return "-";
  try {
    return new Intl.DateTimeFormat(locale.value || "zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(new Date(s));
  } catch {
    return s;
  }
}

function formatAuditTimePart(s: string | undefined): string {
  if (!s) return "";
  try {
    return new Intl.DateTimeFormat(locale.value || "zh-CN", {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    }).format(new Date(s));
  } catch {
    return "";
  }
}

function auditActionTheme(action: AuditAction): "success" | "warning" | "danger" | "primary" | "default" {
  switch (action) {
    case "system.admin_promoted":
      return "success";
    case "system.admin_revoked":
    case "system.setting_changed":
    case "system.queue_task_retried":
    case "system.queue_task_run_now":
      return "warning";
    case "system.user_password_reset":
    case "system.queue_task_deleted":
    case "system.queue_task_cancelled":
    case "system.queue_archived_purged":
      return "danger";
    case "rbac.access_denied":
      return "danger";
    default:
      return "default";
  }
}

function auditOutcomeTheme(o: AuditOutcome): "success" | "danger" | "default" {
  if (o === "denied") return "danger";
  if (o === "success") return "success";
  return "default";
}

function formatAuditAction(action: AuditAction): string {
  return auditActionLabel({ tm }, AUDIT_ACTION_I18N_ROOTS.systemGlobal, action);
}

function auditActorLabel(userId: string): string {
  const me = authStore.user;
  if (me && me.id === userId) {
    return me.username?.trim() || me.email?.trim() || userId.slice(0, 8);
  }
  return userId.slice(0, 8);
}

function auditActorRoleLabel(role: string): string {
  const key = `system.globalSettings.audit.actorRole.${role}`;
  if (te(key)) return t(key);
  return role;
}

function auditDetailsObject(row: AuditLog): Record<string, unknown> | null {
  if (row.details && typeof row.details === "object") {
    return row.details as Record<string, unknown>;
  }
  return null;
}

function auditTargetKey(row: AuditLog): string {
  const details = auditDetailsObject(row);
  if (row.action === "system.setting_changed") {
    if (row.target_type === "tenant_storage_quota") {
      return t("system.globalSettings.audit.target.bulkQuota");
    }
    if (details && typeof details.key === "string" && details.key) return details.key;
    return row.target_id || row.target_type || "";
  }
  if (
    row.action === "system.admin_promoted" ||
    row.action === "system.admin_revoked" ||
    row.action === "system.user_password_reset"
  ) {
    if (!details) return row.target_user_id ? row.target_user_id.slice(0, 8) : "";
    const name = typeof details.target_username === "string" ? details.target_username : "";
    const mail = typeof details.target_email === "string" ? details.target_email : "";
    if (name && mail) return `${name} (${mail})`;
    return name || mail || (row.target_user_id ? row.target_user_id.slice(0, 8) : "");
  }
  if (
    row.action === "system.queue_task_retried" ||
    row.action === "system.queue_task_run_now" ||
    row.action === "system.queue_task_cancelled" ||
    row.action === "system.queue_task_deleted"
  ) {
    const queue = details && typeof details.queue === "string" ? details.queue : "";
    const taskID = details && typeof details.task_id === "string" ? details.task_id : row.target_id;
    return queue && taskID ? `${queue}:${taskID}` : taskID || queue;
  }
  if (row.action === "system.queue_archived_purged") {
    const queue = details && typeof details.queue === "string" ? details.queue : "";
    return queue || row.target_id || "";
  }
  if (row.target_user_id) return row.target_user_id.slice(0, 8);
  if (row.target_id) {
    return row.target_type ? `${row.target_type}:${row.target_id}` : row.target_id;
  }
  return "";
}

function auditTargetDiff(row: AuditLog): string {
  const details = auditDetailsObject(row);
  if (!details) return "";
  if (row.action === "system.setting_changed") {
    if (row.target_type === "tenant_storage_quota") {
      const affected = typeof details.affected === "number" ? details.affected : null;
      const gb = typeof details.quota_gb === "number" ? details.quota_gb : null;
      if (affected !== null && gb !== null) {
        return t("system.globalSettings.audit.target.bulkQuotaDiff", {
          count: String(affected),
          gb: String(gb),
        });
      }
      return "";
    }
    return formatSettingDiff(details);
  }
  if (row.action === "system.admin_promoted" && typeof details.idempotent === "boolean") {
    if (details.idempotent === true) {
      return t("system.globalSettings.audit.target.promoteIdempotent");
    }
    return "";
  }
  if (row.action === "system.admin_revoked" && typeof details.changed === "boolean") {
    if (details.changed === false) {
      return t("system.globalSettings.audit.target.revokeNoop");
    }
    return "";
  }
  if (row.action === "rbac.access_denied" && typeof details.required_role === "string") {
    return t("system.globalSettings.audit.target.requiredRole", { role: details.required_role });
  }
  return "";
}

const SETTING_DIFF_MAX_LEN = 80;
function formatSettingDiff(details: Record<string, unknown>): string {
  const fmt = (v: unknown): string => {
    if (v === null || v === undefined) {
      return t("system.globalSettings.audit.target.valueNull");
    }
    if (typeof v === "string") return v;
    if (typeof v === "number" || typeof v === "boolean") return String(v);
    try {
      return JSON.stringify(v);
    } catch {
      return String(v);
    }
  };
  const truncate = (s: string): string =>
    s.length > SETTING_DIFF_MAX_LEN ? s.slice(0, SETTING_DIFF_MAX_LEN - 1) + "…" : s;
  const oldStr = truncate(fmt(details.old_value));
  const newStr = truncate(fmt(details.new_value));
  if (oldStr === newStr) return "";
  return `${oldStr} → ${newStr}`;
}

function formatAuditDateTime(s: string | undefined): string {
  if (!s) return "—";
  const date = formatAuditDatePart(s);
  const time = formatAuditTimePart(s);
  return time ? `${date} ${time}` : date;
}

function auditActorDisplay(row: AuditLog): string {
  if (!row.actor_user_id) {
    return t("system.globalSettings.audit.systemActor");
  }
  const name = auditActorLabel(row.actor_user_id);
  return row.actor_role ? `${name} (${auditActorRoleLabel(row.actor_role)})` : name;
}

function auditSummaryFields(row: AuditLog): AuditDetailField[] {
  const fields: AuditDetailField[] = [
    {
      key: "time",
      label: t("system.globalSettings.audit.columns.time"),
      value: formatAuditDateTime(row.created_at),
    },
    {
      key: "actor",
      label: t("system.globalSettings.audit.columns.actor"),
      value: auditActorDisplay(row),
    },
    {
      key: "action",
      label: t("system.globalSettings.audit.columns.action"),
      value: formatAuditAction(row.action),
    },
    {
      key: "outcome",
      label: t("system.globalSettings.audit.columns.outcome"),
      value: t("system.globalSettings.audit.outcome." + row.outcome),
    },
  ];

  const targetKey = auditTargetKey(row);
  if (targetKey) {
    fields.push({
      key: "target",
      label: t("system.globalSettings.audit.columns.target"),
      value: targetKey,
    });
  }

  const diff = auditTargetDiff(row);
  if (diff) {
    fields.push({
      key: "targetDiff",
      label: t("system.globalSettings.audit.drawer.targetChange"),
      value: diff,
    });
  }

  return fields;
}

function auditIdentifierFields(row: AuditLog): AuditDetailField[] {
  const fields: AuditDetailField[] = [];
  if (row.actor_user_id) {
    fields.push({
      key: "actorId",
      label: t("system.globalSettings.audit.expanded.actorId"),
      value: row.actor_user_id,
    });
  }
  if (row.target_user_id) {
    fields.push({
      key: "targetUserId",
      label: t("system.globalSettings.audit.expanded.targetUserId"),
      value: row.target_user_id,
    });
  }
  if (row.target_type) {
    fields.push({
      key: "targetType",
      label: t("system.globalSettings.audit.expanded.targetType"),
      value: row.target_type,
    });
  }
  if (row.target_id) {
    fields.push({
      key: "targetId",
      label: t("system.globalSettings.audit.expanded.targetId"),
      value: row.target_id,
    });
  }
  return fields;
}

function auditRequestFields(row: AuditLog): AuditDetailField[] {
  const fields: AuditDetailField[] = [];
  if (row.request_method) {
    fields.push({
      key: "method",
      label: t("system.globalSettings.audit.drawer.requestMethod"),
      value: row.request_method,
    });
  }
  if (row.request_path) {
    fields.push({
      key: "path",
      label: t("system.globalSettings.audit.columns.path"),
      value: row.request_path,
    });
  }
  return fields;
}

const auditDetailTitle = computed(() =>
  selectedAuditEntry.value ? formatAuditAction(selectedAuditEntry.value.action) : "",
);

const auditDetailDescription = computed(() =>
  selectedAuditEntry.value ? formatAuditDateTime(selectedAuditEntry.value.created_at) : "",
);

function openAuditDetail(context: { row: AuditLog }) {
  selectedAuditEntry.value = context.row;
  auditDetailVisible.value = true;
}

function auditDetailsJSON(row: AuditLog): string {
  if (row.details === null || row.details === undefined) return "{}";
  if (typeof row.details === "string") return row.details;
  try {
    return JSON.stringify(row.details, null, 2);
  } catch {
    return String(row.details);
  }
}

async function loadAuditLog(reset: boolean) {
  if (auditLoading.value) return;
  if (!reset && !auditHasMore.value) return;

  auditLoading.value = true;
  auditError.value = "";
  try {
    const resp = await listSystemAuditLog({
      after_id: reset ? undefined : auditCursor.value || undefined,
      limit: AUDIT_PAGE_SIZE,
    });
    if (resp.success) {
      const rows = resp.data || [];
      auditEntries.value = reset ? rows : [...auditEntries.value, ...rows];
      auditCursor.value = resp.next_cursor || 0;
      auditHasMore.value = !!resp.next_cursor && rows.length > 0;
    } else {
      auditError.value = resp.message || t("system.globalSettings.audit.errors.generic");
    }
  } catch (err: any) {
    const status = err?.status;
    if (status === 403) {
      auditError.value = t("system.globalSettings.audit.forbidden");
    } else {
      auditError.value = err?.message || t("system.globalSettings.audit.errors.generic");
    }
  } finally {
    auditLoading.value = false;
  }
}

function detachAuditInfiniteScroll() {
  auditScrollObserver?.disconnect();
  auditScrollObserver = null;
}

function attachAuditInfiniteScroll() {
  detachAuditInfiniteScroll();
  const root = auditScrollRoot.value;
  const sentinel = auditLoadSentinelEl.value;
  if (!root || !sentinel) return;

  auditScrollObserver = new IntersectionObserver(
    (entries) => {
      const hitBottom = entries.some((e) => e.isIntersecting);
      if (!hitBottom || !auditHasMore.value || auditLoading.value) return;
      void loadAuditLog(false);
    },
    { root, rootMargin: "100px 0px", threshold: 0 },
  );
  auditScrollObserver.observe(sentinel);
}

function reloadAuditLog() {
  auditCursor.value = 0;
  auditHasMore.value = true;
  void loadAuditLog(true);
}

watch(
  () => [auditEntries.value.length, auditError.value],
  async () => {
    await nextTick();
    if (auditError.value) {
      detachAuditInfiniteScroll();
      return;
    }
    attachAuditInfiniteScroll();
  },
  { flush: "post" },
);

onMounted(async () => {
  await loadAuditLog(true);
  await nextTick();
  attachAuditInfiniteScroll();
});

onUnmounted(() => {
  detachAuditInfiniteScroll();
});
</script>
