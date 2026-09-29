<template>
  <div ref="containerRef" class="flex w-full flex-col items-end gap-1.5">
    <!-- 显示@的知识库和文件 -->
    <div v-if="mentioned_items && mentioned_items.length > 0" class="chat-mentioned-items justify-end">
      <span v-for="item in mentioned_items" :key="item.id" class="chat-mentioned-tag" :class="[mentionTagClass(item)]">
        <span class="tag_icon">
          <FolderIcon v-if="item.type === 'kb' && item.kb_type !== 'faq'" class="h-3.5 w-3.5" />
          <MessageCircleQuestionIcon v-else-if="item.type === 'kb'" class="h-3.5 w-3.5" />
          <TagIcon v-else-if="item.type === 'tag'" class="h-3.5 w-3.5" />
          <WrenchIcon v-else-if="item.type === 'mcp'" class="h-3.5 w-3.5" />
          <BookmarkIcon v-else-if="item.type === 'skill'" class="h-3.5 w-3.5" />
          <FileIcon v-else class="h-3.5 w-3.5" />
        </span>
        <span class="tag_name">{{ item.name }}</span>
      </span>
    </div>
    <!-- 显示上传的图片 -->
    <div v-if="hasImages" class="flex max-w-full flex-wrap justify-end gap-1.5">
      <img
        v-for="(img, idx) in props.images"
        :key="idx"
        :src="img.url"
        class="h-[120px] w-[120px] cursor-pointer rounded-md border border-[var(--td-border-level-2-color,#e7e7e7)] object-cover transition-opacity duration-200 hover:opacity-85"
        @click="previewImage($event)"
      />
    </div>
    <!-- 显示上传的附件 -->
    <div v-if="hasAttachments" class="flex max-w-full flex-wrap justify-end gap-2">
      <div
        v-for="(att, idx) in props.attachments"
        :key="idx"
        class="bg-card flex max-w-[260px] min-w-[160px] items-center gap-2.5 rounded-[8px] border border-[var(--td-border-level-1-color,#e7e7e7)] px-3 py-2"
        :class="
          canPreviewAttachment(att)
            ? 'cursor-pointer transition-[border-color,box-shadow] duration-200 hover:border-[var(--td-brand-color-2,rgba(0,82,217,0.25))] hover:shadow-[0_2px_8px_rgba(0,0,0,0.06)]'
            : 'cursor-default'
        "
        @click="openAttachmentPreview(att)"
      >
        <div class="flex shrink-0 items-center justify-center">
          <svg viewBox="0 0 40 48" fill="none" xmlns="http://www.w3.org/2000/svg" width="36" height="44">
            <rect width="40" height="48" rx="4" fill="#4A90D9" />
            <path d="M8 6h16l8 8v28a2 2 0 01-2 2H8a2 2 0 01-2-2V8a2 2 0 012-2z" fill="#5BA3E8" />
            <path d="M24 6l8 8h-6a2 2 0 01-2-2V6z" fill="#3A7BC8" />
            <rect x="10" y="20" width="20" height="2" rx="1" fill="white" fill-opacity="0.9" />
            <rect x="10" y="26" width="20" height="2" rx="1" fill="white" fill-opacity="0.9" />
            <rect x="10" y="32" width="14" height="2" rx="1" fill="white" fill-opacity="0.9" />
          </svg>
        </div>
        <div class="flex min-w-0 flex-1 flex-col gap-0.5">
          <div class="text-foreground truncate text-[13px] font-medium">{{ att.file_name }}</div>
          <div class="text-muted-foreground text-[11px] whitespace-nowrap">
            {{ getFileExt(att.file_name)
            }}<span v-if="att.file_size">&nbsp;·&nbsp;{{ formatFileSize(att.file_size) }}</span>
          </div>
        </div>
      </div>
    </div>
    <div
      class="bg-secondary text-foreground ml-auto box-border flex w-max flex-[1_0_0] flex-col items-start justify-center gap-1 rounded-[8px] px-3 py-2 text-left text-base leading-[1.6] [overflow-wrap:anywhere] [word-break:break-word] whitespace-pre-wrap"
      :class="embeddedMode ? 'max-w-full' : 'max-w-[min(76%,820px)]'"
    >
      {{ content }}
    </div>
    <picturePreview :reviewImg="reviewImg" :reviewUrl="reviewUrl" @closePreImg="closePreImg" />
  </div>
</template>
<script setup>
import { computed, ref, watch, onMounted, nextTick } from "vue";
import { BookmarkIcon, FileIcon, FolderIcon, MessageCircleQuestionIcon, TagIcon, WrenchIcon } from "@lucide/vue";
import { hydrateProtectedFileImages } from "@/utils/security";
import picturePreview from "@/components/picture-preview.vue";
import { useChatAttachmentPreviewDrawer } from "@/composables/useChatAttachmentPreviewDrawer";
import { isPreviewableAttachment, resolveAttachmentFileType } from "@/utils/attachmentPreview";
import "@/components/css/chat-resource-chips.css";

const mentionTagClass = (item) => {
  if (item.type === "kb") return item.kb_type === "faq" ? "faq-tag" : "kb-tag";
  return `${item.type || "file"}-tag`;
};

const props = defineProps({
  content: {
    type: String,
    required: false,
  },
  mentioned_items: {
    type: Array,
    required: false,
    default: () => [],
  },
  images: {
    type: Array,
    required: false,
    default: () => [],
  },
  attachments: {
    type: Array,
    required: false,
    default: () => [],
  },
  channel: {
    type: String,
    required: false,
    default: "",
  },
  embeddedMode: {
    type: Boolean,
    default: false,
  },
  sessionId: {
    type: String,
    default: "",
  },
});

const attachmentPreviewDrawer = useChatAttachmentPreviewDrawer();

const containerRef = ref(null);
const hasImages = computed(() => props.images && props.images.length > 0);
const hasAttachments = computed(() => props.attachments && props.attachments.length > 0);

const getFileExt = (fileName) => {
  return (fileName || "").split(".").pop()?.toUpperCase() || "FILE";
};

const formatFileSize = (bytes) => {
  if (!bytes) return "";
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
  return (bytes / (1024 * 1024)).toFixed(1) + " MB";
};

const canPreviewAttachment = (attachment) => {
  return Boolean(props.sessionId) && isPreviewableAttachment(attachment);
};

const openAttachmentPreview = (attachment) => {
  if (!canPreviewAttachment(attachment) || !attachmentPreviewDrawer) return;
  attachmentPreviewDrawer.open({
    sessionId: props.sessionId,
    attachmentId: attachment.id,
    fileName: attachment.file_name,
    fileType: resolveAttachmentFileType(attachment.file_name, attachment.file_type),
  });
};

const hydrateImages = async () => {
  await nextTick();
  await hydrateProtectedFileImages(containerRef.value);
};

watch(() => props.images, hydrateImages);
onMounted(hydrateImages);

const reviewImg = ref(false);
const reviewUrl = ref("");

const previewImage = (event) => {
  const src = event.target?.src;
  if (src) {
    reviewUrl.value = src;
    reviewImg.value = true;
  }
};

const closePreImg = () => {
  reviewImg.value = false;
  reviewUrl.value = "";
};
</script>
