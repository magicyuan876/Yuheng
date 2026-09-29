<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import {
  CircleAlertIcon,
  CirclePauseIcon,
  CirclePlayIcon,
  Loader2Icon,
  MoreHorizontalIcon,
  PenLineIcon,
  PlusIcon,
  RefreshCwIcon,
  ScrollTextIcon,
  Trash2Icon,
} from "@lucide/vue";
import {
  listDataSources,
  deleteDataSource,
  triggerSync,
  pauseDataSource,
  resumeDataSource,
  type DataSource,
} from "@/api/datasource";
import { humanizeCron, relativeTime } from "@/utils/cronHumanize";
import DataSourceEditorDialog from "./DataSourceEditorDialog.vue";
import DataSourceSyncLogs from "./DataSourceSyncLogs.vue";
import DataSourceTypeIcon from "./DataSourceTypeIcon.vue";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Empty, EmptyDescription } from "@/components/ui/empty";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useAuthStore } from "@/stores/auth";

const props = defineProps<{ kbId: string }>();
const emit = defineEmits<{ (e: "count", value: number): void }>();
const { t } = useI18n();
const authStore = useAuthStore();

// 后端 /datasource 的 list/logs 是 Viewer+，但所有写操作（POST/PUT/DELETE
// 以及 sync/pause/resume/validate）都是 Admin+。低权限用户保留只读视图，
// 增删改和触发同步全部隐藏，而不是按下去再撞 403。
const canManageDataSource = computed(() => authStore.hasRole("admin"));

const dataSources = ref<DataSource[]>([]);
const loading = ref(false);
const editorVisible = ref(false);
const editingDs = ref<DataSource | null>(null);
const logsVisible = ref(false);
const logsDsId = ref("");
const logsDsName = ref("");
const deleteTarget = ref<DataSource | null>(null);
const pollTimer = ref<number | null>(null);

function stopPolling() {
  if (pollTimer.value !== null) {
    window.clearTimeout(pollTimer.value);
    pollTimer.value = null;
  }
}

function schedulePolling() {
  stopPolling();
  pollTimer.value = window.setTimeout(() => {
    loadList(true);
  }, 3000);
}

async function loadList(silent = false) {
  if (!silent) loading.value = true;
  try {
    const res = await listDataSources(props.kbId);
    dataSources.value = res?.data || res || [];
    emit("count", dataSources.value.length);

    const hasRunningSync = dataSources.value.some((ds) => ds.latest_sync_log?.status === "running");
    if (hasRunningSync) {
      schedulePolling();
    } else {
      stopPolling();
    }
  } catch (e: any) {
    console.error(e);
  } finally {
    if (!silent) loading.value = false;
  }
}

function openCreate() {
  editingDs.value = null;
  editorVisible.value = true;
}

function openEdit(ds: DataSource) {
  editingDs.value = ds;
  editorVisible.value = true;
}

function openLogs(ds: DataSource) {
  logsDsId.value = ds.id;
  logsDsName.value = ds.name;
  logsVisible.value = true;
}

async function removeDataSource(ds: DataSource) {
  try {
    await deleteDataSource(ds.id);
    MessagePlugin.success(t("datasource.deleteSuccess"));
    await loadList();
  } catch (e: any) {
    MessagePlugin.error(e?.message || e?.error || t("datasource.deleteFailed"));
  }
}

async function handleSync(ds: DataSource) {
  try {
    await triggerSync(ds.id);
    MessagePlugin.success(t("datasource.syncTriggered"));
    await loadList(true);
  } catch (e: any) {
    MessagePlugin.error(e?.message || e?.error || t("datasource.syncFailed"));
  }
}

async function handlePause(ds: DataSource) {
  try {
    await pauseDataSource(ds.id);
    MessagePlugin.success(t("datasource.paused"));
    loadList();
  } catch (e: any) {
    MessagePlugin.error(e?.message || e?.error || t("datasource.pauseFailed"));
  }
}

async function handleResume(ds: DataSource) {
  try {
    await resumeDataSource(ds.id);
    MessagePlugin.success(t("datasource.resumed"));
    loadList();
  } catch (e: any) {
    MessagePlugin.error(e?.message || e?.error || t("datasource.resumeFailed"));
  }
}

function statusLabel(status: string) {
  return t(`datasource.status.${status}`);
}

function syncModeLabel(mode: string) {
  return t(`datasource.syncMode.${mode}`);
}

function connectorLabel(type: string) {
  return t(`datasource.connector.${type}`) || type;
}

function scheduleLabel(cron: string) {
  return humanizeCron(cron, t);
}

function lastSyncTime(ds: DataSource) {
  return relativeTime(ds.last_sync_at, t);
}

function lastSyncFullTime(ds: DataSource) {
  if (!ds.last_sync_at) return "";
  return new Date(ds.last_sync_at).toLocaleString();
}

function syncResultPills(ds: DataSource) {
  const log = ds.latest_sync_log;
  if (!log) return [];
  const pills: { text: string; cls: string }[] = [];
  if (log.items_created > 0) pills.push({ text: `+${log.items_created}`, cls: "created" });
  if (log.items_updated > 0) pills.push({ text: `~${log.items_updated}`, cls: "updated" });
  if (log.items_deleted > 0) pills.push({ text: `-${log.items_deleted}`, cls: "deleted" });
  if (log.items_failed > 0)
    pills.push({ text: `${log.items_failed} ${t("datasource.logMetric.failed")}`, cls: "failed" });
  if (log.items_skipped > 0)
    pills.push({ text: `${log.items_skipped} ${t("datasource.logMetric.skipped")}`, cls: "skipped" });
  return pills;
}

function lastSyncStatusLabel(ds: DataSource) {
  const log = ds.latest_sync_log;
  if (!log) return "--";
  return t(`datasource.logStatus.${log.status}`);
}

function isSyncRunning(ds: DataSource) {
  return ds.latest_sync_log?.status === "running";
}

function statusColorClass(status: string): string {
  if (status === "active") return "text-success";
  if (status === "paused") return "text-warning";
  if (status === "error") return "text-destructive";
  return "";
}

function syncResultColorClass(status: string): string {
  if (status === "success") return "text-success";
  if (status === "failed") return "text-destructive";
  if (status === "running") return "text-primary";
  if (status === "partial") return "text-warning";
  return "text-muted-foreground";
}

function onEditorSaved() {
  editorVisible.value = false;
  loadList();
}

onMounted(loadList);
onBeforeUnmount(stopPolling);
</script>

<template>
  <div class="w-full">
    <div class="mb-7">
      <h2 class="text-foreground m-0 mb-2 text-xl font-semibold">{{ t("datasource.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-[1.6]">{{ t("datasource.description") }}</p>
    </div>

    <div class="min-h-[120px]">
      <!-- t-loading hid the list while a non-silent reload ran; polling reloads are silent. -->
      <div v-if="loading" class="flex justify-center py-10">
        <Loader2Icon class="text-primary size-5 animate-spin" />
      </div>

      <div v-else-if="dataSources.length === 0 && !canManageDataSource" class="py-8">
        <Empty>
          <EmptyDescription>{{ t("datasource.empty") }}</EmptyDescription>
        </Empty>
      </div>

      <div v-else class="grid [grid-template-columns:repeat(auto-fill,minmax(320px,1fr))] gap-3">
        <component
          :is="canManageDataSource ? 'button' : 'div'"
          v-for="ds in dataSources"
          :key="ds.id"
          :type="canManageDataSource ? 'button' : undefined"
          data-slot="ds-card"
          class="group ds-surface-card relative flex min-w-0 items-start gap-3 px-4 py-3.5 text-left text-inherit [font:inherit]"
          :class="[
            `ds-card--${ds.type}`,
            canManageDataSource ? 'ds-surface-card--interactive w-full cursor-pointer' : '',
          ]"
          @click="canManageDataSource ? openEdit(ds) : undefined"
        >
          <div
            class="mt-px flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-[9px] text-[15px] font-semibold tracking-[0.02em] text-[#07c05f]"
            :class="
              ['feishu', 'notion', 'yuque', 'ima', 'rss'].includes(ds.type)
                ? 'bg-card shadow-[inset_0_0_0_1px_var(--td-component-stroke)]'
                : 'bg-[rgba(7,192,95,0.12)]'
            "
          >
            <DataSourceTypeIcon :type="ds.type" variant="badge" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex min-w-0 items-center gap-1.5">
              <h3
                class="text-foreground m-0 min-w-0 flex-1 truncate text-sm leading-[1.4] font-semibold"
                :title="ds.name"
              >
                {{ ds.name }}
              </h3>
              <div class="ml-auto flex shrink-0 items-center gap-0.5" @click.stop>
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <!-- Hidden until the card is hovered or holds focus, as before. -->
                    <Button
                      variant="ghost"
                      size="icon-xs"
                      class="text-placeholder hover:bg-muted hover:text-foreground focus-visible:bg-muted focus-visible:text-foreground shrink-0 p-0.5 opacity-0 transition-[opacity,color] duration-150 group-focus-within:opacity-100 group-hover:opacity-100 aria-expanded:opacity-100"
                    >
                      <MoreHorizontalIcon class="size-4" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" class="min-w-[140px]">
                    <DropdownMenuItem v-if="canManageDataSource" @select="openEdit(ds)">
                      <PenLineIcon /> {{ t("datasource.edit") }}
                    </DropdownMenuItem>
                    <DropdownMenuItem v-if="canManageDataSource" :disabled="isSyncRunning(ds)" @select="handleSync(ds)">
                      <RefreshCwIcon :class="isSyncRunning(ds) ? 'animate-spin' : ''" />
                      {{ isSyncRunning(ds) ? t("datasource.logStatus.running") : t("datasource.syncNow") }}
                    </DropdownMenuItem>
                    <DropdownMenuItem @select="openLogs(ds)">
                      <ScrollTextIcon /> {{ t("datasource.logs") }}
                    </DropdownMenuItem>
                    <DropdownMenuItem v-if="canManageDataSource && ds.status === 'active'" @select="handlePause(ds)">
                      <CirclePauseIcon /> {{ t("datasource.pause") }}
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      v-else-if="canManageDataSource && ds.status === 'paused'"
                      @select="handleResume(ds)"
                    >
                      <CirclePlayIcon /> {{ t("datasource.resume") }}
                    </DropdownMenuItem>
                    <template v-if="canManageDataSource">
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        class="text-destructive focus:text-destructive focus:bg-destructive/10"
                        @select="deleteTarget = ds"
                      >
                        <Trash2Icon /> {{ t("datasource.delete") }}
                      </DropdownMenuItem>
                    </template>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            </div>
            <p
              class="text-muted-foreground mt-0.5 mb-0 flex min-w-0 flex-wrap items-center gap-1 text-xs leading-normal"
            >
              {{ connectorLabel(ds.type) }} · {{ syncModeLabel(ds.sync_mode) }}
              <span class="text-[var(--td-text-color-disabled)] select-none">·</span>
              <span class="inline-flex items-center gap-1" :class="statusColorClass(ds.status)">
                <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-current" aria-hidden="true" />
                {{ statusLabel(ds.status) }}
              </span>
            </p>
            <p class="text-placeholder mt-1 mb-0 flex min-w-0 flex-wrap items-center gap-1 text-xs leading-[1.45]">
              {{ scheduleLabel(ds.sync_schedule) }}
              <span class="text-[var(--td-text-color-disabled)] select-none">·</span>
              <Tooltip :disabled="!lastSyncFullTime(ds)">
                <TooltipTrigger as-child>
                  <span>{{ lastSyncTime(ds) || "--" }}</span>
                </TooltipTrigger>
                <TooltipContent>{{ lastSyncFullTime(ds) }}</TooltipContent>
              </Tooltip>
              <template v-if="ds.latest_sync_log">
                <span class="text-[var(--td-text-color-disabled)] select-none">·</span>
                <span class="font-medium" :class="syncResultColorClass(ds.latest_sync_log.status)">
                  {{ lastSyncStatusLabel(ds) }}
                </span>
                <span
                  v-for="pill in syncResultPills(ds)"
                  :key="pill.cls"
                  class="text-[11px] text-[var(--td-text-color-disabled)] tabular-nums"
                  >{{ pill.text }}</span
                >
              </template>
            </p>
            <div
              v-if="ds.error_message"
              class="bg-destructive/10 text-destructive mt-2 flex items-start gap-1.5 rounded-md px-2.5 py-2 text-left text-xs leading-[1.45]"
            >
              <CircleAlertIcon class="size-3.5 shrink-0" />
              <span>{{ ds.error_message }}</span>
            </div>
          </div>
        </component>

        <button
          v-if="canManageDataSource"
          type="button"
          data-slot="ds-add-card"
          class="ds-surface-card text-placeholder hover:text-primary hover:border-primary focus-visible:text-primary focus-visible:border-primary flex h-full min-h-[68px] w-full cursor-pointer flex-col items-center justify-center gap-2 border-dashed bg-transparent hover:bg-[color-mix(in_srgb,var(--td-brand-color)_6%,transparent)] focus-visible:bg-[color-mix(in_srgb,var(--td-brand-color)_6%,transparent)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--td-brand-color)]"
          @click="openCreate"
        >
          <span
            class="text-primary flex h-8 w-8 items-center justify-center rounded-lg bg-[color-mix(in_srgb,var(--td-brand-color)_10%,transparent)]"
            aria-hidden="true"
          >
            <PlusIcon class="size-[18px]" />
          </span>
          <span class="text-[13px] leading-[1.4] font-medium">{{ t("datasource.add") }}</span>
        </button>
      </div>
    </div>

    <DataSourceEditorDialog
      v-model:visible="editorVisible"
      :kb-id="kbId"
      :data-source="editingDs"
      @saved="onEditorSaved"
    />

    <DataSourceSyncLogs v-model:visible="logsVisible" :data-source-id="logsDsId" :data-source-name="logsDsName" />

    <Dialog :open="deleteTarget !== null" @update:open="(v: boolean) => !v && (deleteTarget = null)">
      <DialogContent class="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>{{ t("datasource.deleteConfirm") }}</DialogTitle>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="deleteTarget = null">{{ t("common.cancel") }}</Button>
          <Button
            variant="destructive"
            @click="
              if (deleteTarget) removeDataSource(deleteTarget);
              deleteTarget = null;
            "
          >
            {{ t("datasource.delete") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<style>
@import "./datasource-surface.css";
</style>
