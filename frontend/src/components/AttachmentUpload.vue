<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import { getMaxFileSizeMB } from "@/utils";
import { getParserEngines } from "@/api/system";
import {
  deleteTemporaryAttachment,
  getTemporaryAttachment,
  uploadTemporaryAttachment,
  type TemporaryAttachmentStatus,
} from "@/api/chat/temporary-attachments";

const { t } = useI18n();

export interface AttachmentFile {
  file: File;
  id: string;
  name: string;
  size: number;
  type: string;
  preview?: string;
  documentId?: string;
  status: TemporaryAttachmentStatus | "local" | "uploading";
  progress?: number;
  error?: string;
}

const props = defineProps<{
  maxFiles?: number;
  maxSize?: number; // in MB
  disabled?: boolean;
  sessionId?: string;
}>();

const emit = defineEmits<{
  (e: "update:files", files: AttachmentFile[]): void;
  (e: "remove", id: string): void;
}>();

const attachments = ref<AttachmentFile[]>([]);
const fileInputRef = ref<HTMLInputElement>();
const pollTimers = new Map<string, ReturnType<typeof setTimeout>>();
let disposed = false;

// Supported file types (matching backend)
const supportedTypes = ref([
  // Documents
  ".pdf",
  ".doc",
  ".docx",
  ".xls",
  ".xlsx",
  ".ppt",
  ".pptx",
  ".epub",
  ".mhtml",
  // Text
  ".txt",
  ".md",
  ".csv",
  ".json",
  ".xml",
  ".html",
  ".markdown",
  ".yaml",
  ".yml",
  ".log",
  // Images are parsed as documents here; the dedicated image button remains
  // available for direct multimodal chat.
  ".jpg",
  ".jpeg",
  ".png",
  ".gif",
  ".bmp",
  ".tiff",
  ".webp",
  // Audio
  ".mp3",
  ".wav",
  ".m4a",
  ".flac",
  ".ogg",
  ".aac",
]);

onMounted(async () => {
  try {
    const response = await getParserEngines();
    const discovered = (response.data || [])
      .filter((engine) => engine.Available !== false)
      .flatMap((engine) => engine.FileTypes || [])
      .filter((type) => type && type.toLowerCase() !== "url")
      .map((type) => `.${type.replace(/^\./, "").toLowerCase()}`);
    supportedTypes.value = [...new Set([...supportedTypes.value, ...discovered])];
  } catch {
    // The static baseline remains available when engine discovery is offline.
  }
});

const maxFiles = computed(() => props.maxFiles || 5);
const maxSizeMB = computed(() => props.maxSize || getMaxFileSizeMB());
// Named for its unit: the `maxSize` prop is in megabytes, and a computed of
// the same name shadowed it in the template.
const maxSizeBytes = computed(() => maxSizeMB.value * 1024 * 1024);

const triggerFileSelect = () => {
  if (props.disabled) return;
  fileInputRef.value?.click();
};

const handleFileSelect = async (event: Event) => {
  const input = event.target as HTMLInputElement;
  if (!input.files) return;

  await addFiles(Array.from(input.files));
  input.value = ""; // Reset input
};

const addFiles = async (files: File[]) => {
  if (props.disabled) return;

  for (const file of files) {
    // Check max files limit
    if (attachments.value.length >= maxFiles.value) {
      MessagePlugin.warning(t("chat.attachmentTooMany", { max: maxFiles.value }));
      break;
    }

    // Check file size
    if (file.size > maxSizeBytes.value) {
      MessagePlugin.warning(t("chat.attachmentTooLarge", { name: file.name, max: maxSizeMB.value }));
      continue;
    }

    // Check file type
    const ext = "." + file.name.split(".").pop()?.toLowerCase();
    if (!supportedTypes.value.includes(ext)) {
      MessagePlugin.warning(t("chat.attachmentTypeNotSupported", { name: file.name }));
      continue;
    }

    const attachment: AttachmentFile = {
      file,
      id: `${Date.now()}-${Math.random()}`,
      name: file.name,
      size: file.size,
      type: file.type || ext,
      status: props.sessionId ? "uploading" : "local",
      progress: 0,
    };

    attachments.value.push(attachment);
    // Vue wraps objects inserted into a ref-backed array with a reactive proxy.
    // Keep using that proxy in async upload/poll callbacks; mutating the raw
    // object above does not trigger the attachment status UI to re-render.
    const reactiveAttachment = attachments.value[attachments.value.length - 1];
    emit("update:files", [...attachments.value]);
    if (props.sessionId) {
      void uploadAttachment(reactiveAttachment);
    }
  }
};

const emitFiles = () => emit("update:files", [...attachments.value]);

const uploadAttachment = async (attachment: AttachmentFile) => {
  if (!props.sessionId) return;
  try {
    const response = await uploadTemporaryAttachment(props.sessionId, attachment.file, "auto", (progress) => {
      attachment.progress = progress;
      emitFiles();
    });
    attachment.documentId = response.data.id;
    if (disposed || !attachments.value.some((item) => item.id === attachment.id)) {
      await deleteTemporaryAttachment(props.sessionId, response.data.id).catch(() => undefined);
      return;
    }
    attachment.status = response.data.status;
    attachment.progress = 100;
    emitFiles();
    if (attachment.status !== "ready" && attachment.status !== "failed") {
      scheduleStatusPoll(attachment);
    }
  } catch (error: any) {
    attachment.status = "failed";
    attachment.error = error?.message || t("chat.attachmentUploadFailed");
    emitFiles();
  }
};

const scheduleStatusPoll = (attachment: AttachmentFile) => {
  clearPoll(attachment.id);
  pollTimers.set(
    attachment.id,
    setTimeout(() => void pollStatus(attachment), 800),
  );
};

const pollStatus = async (attachment: AttachmentFile) => {
  if (!props.sessionId || !attachment.documentId || !attachments.value.some((item) => item.id === attachment.id))
    return;
  try {
    const response = await getTemporaryAttachment(props.sessionId, attachment.documentId);
    attachment.status = response.data.status;
    attachment.error = response.data.error_message;
    emitFiles();
    if (attachment.status !== "ready" && attachment.status !== "failed") scheduleStatusPoll(attachment);
  } catch (error: any) {
    attachment.status = "failed";
    attachment.error = error?.message || t("chat.attachmentParseFailed");
    emitFiles();
  }
};

const clearPoll = (id: string) => {
  const timer = pollTimers.get(id);
  if (timer) clearTimeout(timer);
  pollTimers.delete(id);
};

const removeAttachment = (id: string) => {
  const index = attachments.value.findIndex((a) => a.id === id);
  if (index !== -1) {
    const attachment = attachments.value[index];
    clearPoll(id);
    attachments.value.splice(index, 1);
    emitFiles();
    emit("remove", id);
    if (props.sessionId && attachment.documentId) {
      void deleteTemporaryAttachment(props.sessionId, attachment.documentId).catch(() => undefined);
    }
  }
};

const formatFileSize = (bytes: number): string => {
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
  return (bytes / (1024 * 1024)).toFixed(1) + " MB";
};

const getFileExt = (fileName: string): string => {
  return fileName.split(".").pop()?.toUpperCase() || "FILE";
};

const statusLabel = (attachment: AttachmentFile): string => {
  if (attachment.status === "uploading") return t("chat.attachmentUploading", { progress: attachment.progress || 0 });
  if (attachment.status === "uploaded" || attachment.status === "processing") return t("chat.attachmentParsing");
  if (attachment.status === "ready") return t("chat.attachmentReady");
  if (attachment.status === "failed") return attachment.error || t("chat.attachmentParseFailed");
  return "";
};

onUnmounted(() => {
  disposed = true;
  pollTimers.forEach((timer) => clearTimeout(timer));
  pollTimers.clear();
});

defineExpose({
  attachments,
  triggerFileSelect,
  addFiles,
  clear: () => {
    pollTimers.forEach((timer) => clearTimeout(timer));
    pollTimers.clear();
    attachments.value = [];
    emit("update:files", []);
  },
});
</script>

<template>
  <div class="attachment-upload w-full">
    <!-- Hidden file input -->
    <input
      ref="fileInputRef"
      type="file"
      :accept="supportedTypes.join(',')"
      multiple
      class="hidden"
      @change="handleFileSelect"
    />

    <!-- Attachment list -->
    <div v-if="attachments.length > 0" class="flex flex-wrap gap-2 px-3 pt-2 pb-1">
      <div
        v-for="attachment in attachments"
        :key="attachment.id"
        class="border-border bg-card relative flex max-w-[240px] min-w-[140px] cursor-default flex-row items-center gap-2.5 rounded-lg border border-solid py-2 pr-8 pl-2.5"
      >
        <div class="flex shrink-0 items-center justify-center">
          <svg viewBox="0 0 40 48" fill="none" xmlns="http://www.w3.org/2000/svg" width="32" height="38">
            <rect width="40" height="48" rx="4" fill="#4A90D9" />
            <path d="M8 6h16l8 8v28a2 2 0 01-2 2H8a2 2 0 01-2-2V8a2 2 0 012-2z" fill="#5BA3E8" />
            <path d="M24 6l8 8h-6a2 2 0 01-2-2V6z" fill="#3A7BC8" />
            <rect x="10" y="20" width="20" height="2" rx="1" fill="white" fill-opacity="0.9" />
            <rect x="10" y="26" width="20" height="2" rx="1" fill="white" fill-opacity="0.9" />
            <rect x="10" y="32" width="14" height="2" rx="1" fill="white" fill-opacity="0.9" />
          </svg>
        </div>
        <div class="flex min-w-0 flex-1 flex-col gap-0.5">
          <div class="text-foreground truncate text-[13px] font-medium">{{ attachment.name }}</div>
          <div class="text-muted-foreground text-[11px] whitespace-nowrap">
            {{ getFileExt(attachment.name) }}&nbsp;·&nbsp;{{ formatFileSize(attachment.size) }}
          </div>
          <div
            v-if="attachment.status !== 'local'"
            class="max-w-[170px] truncate text-[11px]"
            :class="{
              'text-success': attachment.status === 'ready',
              'text-destructive': attachment.status === 'failed',
              'text-muted-foreground': attachment.status !== 'ready' && attachment.status !== 'failed',
            }"
          >
            <span
              v-if="
                attachment.status === 'uploading' ||
                attachment.status === 'uploaded' ||
                attachment.status === 'processing'
              "
              class="mr-[3px] inline-block size-[9px] animate-spin rounded-full border border-solid border-current border-r-transparent [animation-duration:0.8s]"
            />
            {{ statusLabel(attachment) }}
          </div>
        </div>
        <span
          class="absolute top-1 right-1.5 flex size-[18px] cursor-pointer items-center justify-center rounded-full bg-black/[0.18] text-[13px] leading-none text-white hover:bg-black/40"
          @click="removeAttachment(attachment.id)"
          :aria-label="$t('common.remove')"
          >×</span
        >
      </div>
    </div>

    <!-- Upload button (shown in control bar) -->
    <slot name="trigger" :trigger="triggerFileSelect" :count="attachments.length" />
  </div>
</template>
