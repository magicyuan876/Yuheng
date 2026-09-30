<template>
  <div class="w-full">
    <div class="mb-8">
      <h2 class="text-foreground mt-0 mb-2 text-xl font-semibold">{{ $t("system.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-normal">{{ $t("system.sectionDescription") }}</p>
    </div>

    <!-- Loading state -->
    <div v-if="loading" class="text-muted-foreground flex items-center justify-center gap-3 py-10 text-sm">
      <Loader2Icon class="animate-spin" />
      <span>{{ $t("system.loadingInfo") }}</span>
    </div>

    <!-- Error state -->
    <div v-else-if="error" class="py-5">
      <!-- TDesign's error alert sat on the pale error tint with no border and
           dark text; only its icon was red. -->
      <Alert variant="destructive" class="text-foreground border-transparent bg-[var(--td-error-color-1)]">
        <CircleAlertIcon class="text-destructive!" />
        <AlertTitle>{{ error }}</AlertTitle>
        <AlertAction>
          <Button variant="outline" size="sm" @click="loadInfo">{{ $t("system.retry") }}</Button>
        </AlertAction>
      </Alert>
    </div>

    <!-- Content -->
    <div v-else class="flex flex-col">
      <!-- System version -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ $t("system.versionLabel") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ $t("system.versionDescription") }}</p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <span class="text-foreground text-right text-sm break-words">
            {{ systemInfo?.version || $t("system.unknown") }}
            <span v-if="systemInfo?.commit_id" class="text-placeholder ml-1.5 text-xs">
              ({{ systemInfo.commit_id }})
            </span>
          </span>
        </div>
      </div>

      <!-- Frontend version -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">
            {{ $t("system.frontendVersionLabel") }}
          </label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ $t("system.frontendVersionDescription") }}
          </p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <span class="text-foreground text-right text-sm break-words">
            {{ frontendVersion }}
            <Badge
              v-if="
                systemInfo?.version &&
                systemInfo.version !== 'unknown' &&
                frontendVersion !== 'unknown' &&
                systemInfo.version !== frontendVersion
              "
              variant="secondary"
              class="bg-warning/10 text-warning ml-2"
              >{{ $t("system.versionMismatch") }}</Badge
            >
            <span v-if="frontendCommit && frontendCommit !== 'unknown'" class="text-placeholder ml-1.5 text-xs">
              ({{ frontendCommit }})
            </span>
          </span>
        </div>
      </div>

      <!-- Build time -->
      <div
        v-if="systemInfo?.build_time"
        class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b"
      >
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ $t("system.buildTimeLabel") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ $t("system.buildTimeDescription") }}</p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <span class="text-foreground text-right text-sm break-words">{{ systemInfo.build_time }}</span>
        </div>
      </div>

      <!-- Go version -->
      <div
        v-if="systemInfo?.go_version"
        class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b"
      >
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ $t("system.goVersionLabel") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ $t("system.goVersionDescription") }}</p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <span class="text-foreground text-right text-sm break-words">{{ systemInfo.go_version }}</span>
        </div>
      </div>

      <!-- Service started at -->
      <div
        v-if="systemInfo?.started_at"
        class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b"
      >
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ $t("system.startedAtLabel") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ $t("system.startedAtDescription") }}</p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <span class="text-foreground text-right text-sm break-words">{{
            formatStartedAt(systemInfo.started_at)
          }}</span>
        </div>
      </div>

      <!-- Service uptime -->
      <div
        v-if="displayUptimeSeconds != null"
        class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b"
      >
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ $t("system.uptimeLabel") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ $t("system.uptimeDescription") }}</p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <span class="text-foreground text-right text-sm break-words">{{ formatUptime(displayUptimeSeconds) }}</span>
        </div>
      </div>

      <!-- DB Version -->
      <div
        v-if="systemInfo?.db_version || systemInfo?.db_migration_error"
        class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b"
      >
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ $t("system.dbVersionLabel") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ $t("system.dbVersionDescription") }}</p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <span class="text-foreground text-right text-sm break-words">
            {{ systemInfo?.db_version || $t("system.unknown") }}
            <Badge v-if="systemInfo?.db_migration_error" variant="destructive" class="ml-2">
              {{ $t("system.dbMigrationFailedTag") }}
            </Badge>
          </span>
        </div>
      </div>

      <!-- DB migration error: full-width banner under the row -->
      <div v-if="systemInfo?.db_migration_error" class="border-border block py-0 pb-5 [&:not(:last-child)]:border-b">
        <Alert variant="destructive" class="text-foreground w-full border-transparent bg-[var(--td-error-color-1)]">
          <CircleAlertIcon class="text-destructive!" />
          <AlertTitle>{{ $t("system.dbMigrationFailedTitle") }}</AlertTitle>
          <!-- A plain body rather than AlertDescription: that part underlines links
               and spaces paragraphs 16px apart, and the old alert did neither. -->
          <div class="col-start-2 text-sm">
            <p class="text-foreground m-0 mb-2 text-[13px] leading-normal">
              {{ $t("system.dbMigrationFailedDesc") }}
            </p>
            <pre
              class="bg-accent text-muted-foreground m-0 mb-3 max-h-[200px] overflow-auto rounded px-3 py-2 text-xs leading-normal break-words whitespace-pre-wrap"
              >{{ systemInfo.db_migration_error }}</pre>
            <div class="text-muted-foreground flex items-center gap-2 text-[13px]">
              <a
                v-if="troubleshootingDocsURL"
                class="text-primary hover:underline"
                :href="troubleshootingDocsURL"
                target="_blank"
                rel="noopener noreferrer"
                >{{ $t("system.dbMigrationViewDocs") }}</a
              >
              <span v-if="troubleshootingDocsURL && reportIssueURL" class="text-placeholder">·</span>
              <a
                v-if="reportIssueURL"
                class="text-primary hover:underline"
                :href="reportIssueURL"
                target="_blank"
                rel="noopener noreferrer"
                >{{ $t("system.dbMigrationReportIssue") }}</a
              >
            </div>
          </div>
        </Alert>
      </div>

      <!-- Keyword Index Engine -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">
            {{ $t("system.keywordIndexEngineLabel") }}
          </label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ $t("system.keywordIndexEngineDescription") }}
          </p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <span class="text-foreground text-right text-sm break-words">
            {{ systemInfo?.keyword_index_engine || $t("system.unknown") }}
          </span>
        </div>
      </div>

      <!-- Vector Store Engine -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">
            {{ $t("system.vectorStoreEngineLabel") }}
          </label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ $t("system.vectorStoreEngineDescription") }}
          </p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <span class="text-foreground text-right text-sm break-words">
            {{ systemInfo?.vector_store_engine || $t("system.unknown") }}
          </span>
        </div>
      </div>

      <!-- Graph Database Engine -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">
            {{ $t("system.graphDatabaseEngineLabel") }}
          </label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ $t("system.graphDatabaseEngineDescription") }}
          </p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <span class="text-foreground text-right text-sm break-words">
            {{ systemInfo?.graph_database_engine || $t("system.unknown") }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ISSUE_TRACKER_URL, docsUrl } from "@/config/externalLinks";
import { ref, computed, onMounted, onUnmounted } from "vue";
import { getSystemInfo, type SystemInfo } from "@/api/system";
import { useI18n } from "vue-i18n";

import { Alert, AlertAction, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { CircleAlertIcon, Loader2Icon } from "@lucide/vue";

const { t, locale } = useI18n();

// Reactive state
const systemInfo = ref<SystemInfo | null>(null);
const loading = ref(true);
const error = ref("");
const frontendVersion = __FRONTEND_VERSION__;
const frontendCommit = __FRONTEND_COMMIT__;

let uptimeTicker: ReturnType<typeof setInterval> | null = null;
const uptimeTick = ref(0);

const displayUptimeSeconds = computed(() => {
  void uptimeTick.value;
  const info = systemInfo.value;
  if (info?.started_at) {
    const boot = new Date(info.started_at).getTime();
    if (!Number.isNaN(boot)) {
      return Math.floor((Date.now() - boot) / 1000);
    }
  }
  if (info?.uptime_seconds != null) return info.uptime_seconds;
  return null;
});

function formatStartedAt(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(locale.value);
}

function formatUptime(totalSeconds: number): string {
  const sec = Math.max(0, Math.floor(totalSeconds));
  const days = Math.floor(sec / 86400);
  const hours = Math.floor((sec % 86400) / 3600);
  const minutes = Math.floor((sec % 3600) / 60);
  const seconds = sec % 60;
  const parts: string[] = [];
  if (days > 0) parts.push(t("system.uptimeDays", { n: days }));
  if (hours > 0 || days > 0) parts.push(t("system.uptimeHours", { n: hours }));
  if (minutes > 0 || hours > 0 || days > 0) parts.push(t("system.uptimeMinutes", { n: minutes }));
  if (parts.length === 0) return t("system.uptimeSeconds", { n: seconds });
  if (seconds > 0 && days === 0) parts.push(t("system.uptimeSeconds", { n: seconds }));
  return parts.join(" ");
}

const troubleshootingDocsURL = docsUrl("01-getting-started/05-backup-and-upgrade#迁移失败时如何读启动报错");

// Pre-fills a new issue with the current migration error so users don't have to
// paste it manually. Body is intentionally minimal — the bug template will fill
// in the rest. Encode aggressively to survive newlines / quotes.
const reportIssueURL = computed(() => {
  // 未配置内部工单地址时返回空串，模板据此隐藏「反馈问题」入口。
  if (!ISSUE_TRACKER_URL) return "";
  const base = ISSUE_TRACKER_URL;
  const params = new URLSearchParams({
    template: "bug_report.yml",
    title: "[Bug]: Database migration failed at startup",
    labels: "bug",
  });
  const errMsg = systemInfo.value?.db_migration_error;
  if (errMsg) {
    const body = [
      "### Environment",
      `- Platform version: ${systemInfo.value?.version || "unknown"}`,
      `- Commit: ${systemInfo.value?.commit_id || "unknown"}`,
      `- Frontend version: ${frontendVersion} (${frontendCommit})`,
      `- DB version reported: ${systemInfo.value?.db_version || "unknown"}`,
      "",
      "### Migration error",
      "```",
      errMsg,
      "```",
    ].join("\n");
    params.set("body", body);
  }
  return `${base}?${params.toString()}`;
});

// Methods
const loadInfo = async () => {
  try {
    loading.value = true;
    error.value = "";

    const systemResponse = await getSystemInfo();

    if (systemResponse.data) {
      systemInfo.value = systemResponse.data;
    } else {
      error.value = t("system.messages.fetchFailed");
    }
  } catch (err: any) {
    error.value = err?.message || t("system.messages.networkError");
  } finally {
    loading.value = false;
  }
};

// Lifecycle
onMounted(() => {
  loadInfo();
  uptimeTicker = setInterval(() => {
    uptimeTick.value++;
  }, 30_000);
});

onUnmounted(() => {
  if (uptimeTicker) {
    clearInterval(uptimeTicker);
    uptimeTicker = null;
  }
});
</script>
