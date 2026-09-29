<template>
  <NodeViewWrapper
    class="bg-card my-2.5 flex max-w-[420px] items-center gap-2 rounded-[8px] border px-2.5 py-2"
    :class="selected ? 'border-primary' : 'border-[var(--td-component-stroke)]'"
    :data-drag-handle="editor.isEditable ? '' : undefined"
  >
    <a
      class="flex min-w-0 flex-1 items-center gap-2.5 text-inherit no-underline"
      :href="href"
      :download="name"
      target="_blank"
      rel="noopener"
    >
      <component :is="icon" class="text-primary size-5 flex-none" />
      <span class="flex min-w-0 flex-col">
        <span class="text-foreground truncate text-[13.5px]">{{ name }}</span>
        <span class="text-placeholder text-xs tabular-nums">{{ meta }}</span>
      </span>
    </a>
    <button
      v-if="editor.isEditable"
      type="button"
      data-slot="attachment-remove"
      class="text-placeholder hover:text-destructive hover:bg-accent flex-none cursor-pointer rounded border-0 p-1 leading-0"
      :aria-label="t('docs.attachments.remove')"
      @click="deleteNode"
    >
      <Trash2Icon class="size-3.5" />
    </button>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed, type Component } from "vue";
import { useI18n } from "vue-i18n";

import {
  CirclePlayIcon,
  FileArchiveIcon,
  FileIcon,
  FileSpreadsheetIcon,
  FileTextIcon,
  Trash2Icon,
  Volume2Icon,
} from "@lucide/vue";

import { attachmentSrc, formatBytes } from "./attachments";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();

const name = computed(() => String(props.node.attrs.name || t("docs.attachments.unnamed")));

const href = computed(() => {
  const id = props.node.attrs.attachmentId as string | null;
  return id ? attachmentSrc(id) : "#";
});

const meta = computed(() => {
  const size = Number(props.node.attrs.size ?? 0);
  const parts = [formatBytes(size), shortType.value].filter(Boolean);
  return parts.join(" · ");
});

/** The tail of the media type, which reads better than the whole of it. */
const shortType = computed(() => {
  const mime = String(props.node.attrs.mime ?? "");
  if (!mime) return "";
  const sub = mime.split("/")[1] ?? mime;
  return sub.split(/[.+]/).pop()?.toUpperCase() ?? "";
});

const icon = computed<Component>(() => {
  const mime = String(props.node.attrs.mime ?? "");
  if (mime.startsWith("video/")) return CirclePlayIcon;
  if (mime.startsWith("audio/")) return Volume2Icon;
  if (mime === "application/pdf") return FileTextIcon;
  if (mime.includes("spreadsheet") || mime.includes("excel") || mime === "text/csv") return FileSpreadsheetIcon;
  if (mime.includes("word")) return FileTextIcon;
  if (mime.includes("zip") || mime.includes("compressed")) return FileArchiveIcon;
  return FileIcon;
});
</script>
