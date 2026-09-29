<template>
  <div class="w-full">
    <div class="mb-8">
      <h2 class="text-foreground mt-0 mb-2 text-xl font-semibold">{{ $t("ollamaSettings.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-normal">{{ $t("ollamaSettings.description") }}</p>
    </div>

    <div class="flex flex-col">
      <!-- Ollama 服务状态 -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="min-w-0 flex-1 pr-8">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            $t("ollamaSettings.status.label")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-[1.6]">{{ $t("ollamaSettings.status.desc") }}</p>
        </div>
        <div class="flex w-[360px] max-w-[360px] shrink-0 flex-col items-end">
          <div class="flex items-center gap-3">
            <Badge v-if="testing" variant="secondary">
              <Loader2Icon class="animate-spin" />
              {{ $t("ollamaSettings.status.testing") }}
            </Badge>
            <Badge v-else-if="connectionStatus === true" variant="secondary" class="bg-success/10 text-success">
              <CircleCheckIcon />
              {{ $t("ollamaSettings.status.available") }}
            </Badge>
            <Badge v-else-if="connectionStatus === false" variant="destructive">
              <CircleXIcon />
              {{ $t("ollamaSettings.status.unavailable") }}
            </Badge>
            <Badge v-else variant="secondary">
              <CircleHelpIcon />
              {{ $t("ollamaSettings.status.untested") }}
            </Badge>
            <Button variant="ghost" size="sm" :disabled="testing" @click="testConnection">
              <Loader2Icon v-if="testing" class="animate-spin" />
              <RefreshCwIcon v-else />
              {{ $t("ollamaSettings.status.retest") }}
            </Button>
          </div>
        </div>
      </div>

      <!-- Ollama 服务地址 -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="min-w-0 flex-1 pr-8">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            $t("ollamaSettings.address.label")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-[1.6]">{{ $t("ollamaSettings.address.desc") }}</p>
        </div>
        <div class="flex w-[360px] max-w-[360px] shrink-0 flex-col items-end">
          <div class="flex w-full items-center gap-2">
            <Input
              v-model="localBaseUrl"
              :placeholder="$t('ollamaSettings.address.placeholder')"
              disabled
              class="flex-1"
            />
          </div>
          <!-- TDesign's warning alert: pale warning tint, no border, dark text,
               only the icon in the warning colour. -->
          <Alert
            v-if="connectionStatus === false"
            class="mt-2 w-full border-transparent bg-[var(--td-warning-color-1)]"
          >
            <CircleAlertIcon class="text-warning!" />
            <AlertDescription class="text-foreground">{{ $t("ollamaSettings.address.failed") }}</AlertDescription>
          </Alert>
        </div>
      </div>
    </div>

    <!-- 下载新模型 -->
    <div v-if="connectionStatus === true" class="border-border mt-8 mb-8 border-t pt-8 last:mb-0">
      <div class="mb-6 flex items-start justify-between">
        <div class="flex-1">
          <h3 class="text-foreground mt-0 mb-1.5 text-[17px] font-semibold">
            {{ $t("ollamaSettings.download.title") }}
          </h3>
          <p class="text-placeholder m-0 text-[13px] leading-normal">
            {{ $t("ollamaSettings.download.descPrefix") }}
            <a
              href="https://ollama.com/search"
              target="_blank"
              rel="noopener noreferrer"
              class="text-primary inline-flex items-center gap-0.5 hover:underline"
            >
              {{ $t("ollamaSettings.download.browse") }}
              <LinkIcon class="size-3.5" />
            </a>
          </p>
        </div>
      </div>

      <div class="flex flex-col gap-4">
        <div class="flex items-center gap-2">
          <Input v-model="downloadModelName" :placeholder="$t('ollamaSettings.download.placeholder')" class="flex-1" />
          <Button
            variant="secondary"
            size="sm"
            class="h-8 shrink-0"
            :disabled="downloading || !downloadModelName.trim()"
            @click="downloadModel"
          >
            <Loader2Icon v-if="downloading" class="animate-spin" />
            <DownloadIcon v-else />
            {{ $t("ollamaSettings.download.download") }}
          </Button>
        </div>

        <div v-if="downloadProgress > 0" class="bg-secondary border-border rounded-lg border p-4">
          <div class="text-foreground mb-2.5 flex justify-between text-[13px] font-medium">
            <span>{{ $t("ollamaSettings.download.downloading", { name: downloadModelName }) }}</span>
            <span>{{ downloadProgress.toFixed(2) }}%</span>
          </div>
          <!-- A plain filled bar stands in for t-progress; the track and fill
               use the same brand colour pair. -->
          <div class="bg-background h-1.5 overflow-hidden rounded-full">
            <div class="bg-primary h-full rounded-full transition-all" :style="{ width: `${downloadProgress}%` }" />
          </div>
        </div>
      </div>
    </div>

    <!-- 已下载的模型 -->
    <div v-if="connectionStatus === true" class="border-border mt-8 mb-8 border-t pt-8 last:mb-0">
      <div class="mb-6 flex items-start justify-between">
        <div class="flex-1">
          <h3 class="text-foreground mt-0 mb-1.5 text-[17px] font-semibold">
            {{ $t("ollamaSettings.installed.title") }}
          </h3>
          <p class="text-placeholder m-0 text-[13px] leading-normal">{{ $t("ollamaSettings.installed.desc") }}</p>
        </div>
        <Button variant="ghost" size="sm" :disabled="loadingModels" @click="refreshModels">
          <Loader2Icon v-if="loadingModels" class="animate-spin" />
          <RefreshCwIcon v-else />
          {{ $t("common.refresh") }}
        </Button>
      </div>

      <div v-if="loadingModels" class="text-placeholder flex items-center justify-center gap-2 py-12 text-sm">
        <Loader2Icon class="animate-spin" />
        <span>{{ $t("common.loading") }}</span>
      </div>
      <div v-else-if="downloadedModels.length > 0" class="grid grid-cols-2 gap-3 max-md:grid-cols-1">
        <div
          v-for="model in downloadedModels"
          :key="model.name"
          class="border-border bg-secondary hover:border-primary hover:bg-card flex items-center justify-between rounded-md border px-3 py-2.5 transition-all"
        >
          <div class="min-w-0 flex-1">
            <div class="text-foreground mb-1 font-[family-name:var(--app-font-family-mono)] text-sm font-medium">
              {{ model.name }}
            </div>
            <div class="text-muted-foreground flex gap-3 text-xs">
              <span>{{ formatSize(model.size) }}</span>
              <span>{{ formatDate(model.modified_at) }}</span>
            </div>
          </div>
        </div>
      </div>
      <div v-else class="py-12 text-center">
        <p class="text-placeholder m-0 text-sm">{{ $t("ollamaSettings.installed.empty") }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useSettingsStore } from "@/stores/settings";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import {
  checkOllamaStatus,
  listOllamaModels,
  downloadOllamaModel,
  getDownloadProgress,
  type OllamaModelInfo,
} from "@/api/initialization";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  CircleAlertIcon,
  CircleCheckIcon,
  CircleHelpIcon,
  CircleXIcon,
  DownloadIcon,
  LinkIcon,
  Loader2Icon,
  RefreshCwIcon,
} from "@lucide/vue";

const settingsStore = useSettingsStore();
const { t } = useI18n();

const localBaseUrl = ref(settingsStore.settings.ollamaConfig?.baseUrl ?? "");

const testing = ref(false);
const connectionStatus = ref<boolean | null>(null);
const loadingModels = ref(false);
const downloadedModels = ref<OllamaModelInfo[]>([]);
const downloading = ref(false);
const downloadModelName = ref("");
const downloadProgress = ref(0);

// 测试连接
const testConnection = async () => {
  testing.value = true;
  connectionStatus.value = null;

  try {
    // 保存配置
    settingsStore.updateOllamaConfig({ baseUrl: localBaseUrl.value });

    // 调用真实 Ollama API 测试连接
    const result = await checkOllamaStatus();

    // 如果接口返回了 baseUrl 且与当前输入框的值不同，更新为接口返回的值
    if (result.baseUrl && result.baseUrl !== localBaseUrl.value) {
      localBaseUrl.value = result.baseUrl;
      settingsStore.updateOllamaConfig({ baseUrl: result.baseUrl });
    }

    connectionStatus.value = result.available;

    if (connectionStatus.value) {
      MessagePlugin.success(t("ollamaSettings.toasts.connected"));
      refreshModels();
    } else {
      MessagePlugin.error(result.error || t("ollamaSettings.toasts.connectFailed"));
    }
  } catch (error: any) {
    connectionStatus.value = false;
    MessagePlugin.error(error.message || t("ollamaSettings.toasts.connectFailed"));
  } finally {
    testing.value = false;
  }
};

// 刷新模型列表
const refreshModels = async () => {
  loadingModels.value = true;

  try {
    // 调用真实 Ollama API 获取模型列表（现在返回完整的模型信息）
    const models = await listOllamaModels();
    downloadedModels.value = models;
  } catch (error: any) {
    console.error("获取模型列表失败:", error);
    MessagePlugin.error(error.message || t("ollamaSettings.toasts.listFailed"));
  } finally {
    loadingModels.value = false;
  }
};

// 格式化文件大小
const formatSize = (bytes: number): string => {
  if (!bytes || bytes === 0 || isNaN(bytes)) return "0 B";
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + " KB";
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(2) + " MB";
  return (bytes / (1024 * 1024 * 1024)).toFixed(2) + " GB";
};

// 格式化日期
const formatDate = (dateStr: string): string => {
  if (!dateStr) return t("ollama.unknown");

  const date = new Date(dateStr);
  if (isNaN(date.getTime())) return t("ollama.unknown");

  const now = new Date();
  const diff = now.getTime() - date.getTime();
  const days = Math.floor(diff / (1000 * 60 * 60 * 24));

  if (days === 0) return t("ollama.today");
  if (days === 1) return t("ollama.yesterday");
  if (days < 7) return t("ollama.daysAgo", { days });
  return date.toLocaleDateString();
};

// 下载模型
const downloadModel = async () => {
  if (!downloadModelName.value.trim()) return;

  downloading.value = true;
  downloadProgress.value = 0;

  try {
    // 调用真实 Ollama API 下载模型
    const result = await downloadOllamaModel(downloadModelName.value);

    if (result.status === "failed") {
      MessagePlugin.error(t("ollamaSettings.toasts.downloadFailed"));
      downloading.value = false;
      downloadProgress.value = 0;
      return;
    }

    MessagePlugin.success(t("ollamaSettings.toasts.downloadStarted", { name: downloadModelName.value }));

    // 查询下载进度
    const taskId = result.taskId;
    const progressInterval = setInterval(async () => {
      try {
        const task = await getDownloadProgress(taskId);
        downloadProgress.value = task.progress;

        if (task.status === "completed") {
          clearInterval(progressInterval);
          MessagePlugin.success(t("ollamaSettings.toasts.downloadCompleted", { name: downloadModelName.value }));
          downloadModelName.value = "";
          downloadProgress.value = 0;
          downloading.value = false;
          refreshModels();
        } else if (task.status === "failed") {
          clearInterval(progressInterval);
          MessagePlugin.error(task.message || t("ollamaSettings.toasts.downloadFailed"));
          downloading.value = false;
          downloadProgress.value = 0;
        }
      } catch (error) {
        clearInterval(progressInterval);
        MessagePlugin.error(t("ollamaSettings.toasts.progressFailed"));
        downloading.value = false;
        downloadProgress.value = 0;
      }
    }, 1000);
  } catch (error: any) {
    console.error("下载失败:", error);
    MessagePlugin.error(error.message || t("ollamaSettings.toasts.downloadFailed"));
    downloading.value = false;
    downloadProgress.value = 0;
  }
};

// 初始化 Ollama 服务地址
const initOllamaBaseUrl = async () => {
  try {
    const result = await checkOllamaStatus();
    // 如果接口返回了 baseUrl，优先使用接口返回的值
    if (result.baseUrl) {
      localBaseUrl.value = result.baseUrl;
      // 如果 store 中没有保存过，也保存到 store 中
      if (!settingsStore.settings.ollamaConfig?.baseUrl) {
        settingsStore.updateOllamaConfig({ baseUrl: result.baseUrl });
      }
    } else if (!localBaseUrl.value) {
      // 如果接口没返回且 store 中也没有，使用默认值
      localBaseUrl.value = "http://localhost:11434";
    }

    // 直接使用初始化时获取的状态，避免重复调用
    connectionStatus.value = result.available;
    if (result.available) {
      refreshModels();
    }

    return result;
  } catch (error) {
    console.error("初始化 Ollama 地址失败:", error);
    // 如果获取失败，使用默认值或 store 中的值
    if (!localBaseUrl.value) {
      localBaseUrl.value = "http://localhost:11434";
    }
    return null;
  }
};

// 组件挂载时自动检查连接
onMounted(async () => {
  // 初始化服务地址，如果启用则直接使用返回的状态，避免重复调用
  await initOllamaBaseUrl();
});
</script>
