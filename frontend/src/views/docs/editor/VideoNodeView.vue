<template>
  <NodeViewWrapper class="group relative my-3 flex" :class="[ALIGN_CLASS[align] ?? ALIGN_CLASS.center]">
    <video
      v-if="src"
      class="max-w-full rounded-[8px] bg-black"
      :class="selected ? 'outline-primary outline-2 outline-offset-2' : ''"
      :src="src"
      :style="frameStyle"
      controls
      preload="metadata"
      playsinline
    />
    <p
      v-else
      class="text-placeholder m-0 rounded-[8px] border border-dashed border-[var(--td-component-stroke)] p-3 text-[13px]"
    >
      {{ t("docs.media.missing") }}
    </p>
    <button
      v-if="editor.isEditable"
      type="button"
      data-slot="video-remove"
      class="absolute top-1.5 right-1.5 cursor-pointer rounded border-0 bg-black/55 px-1 py-[3px] leading-0 text-white opacity-0 group-hover:opacity-100"
      :aria-label="t('docs.attachments.remove')"
      @click="deleteNode"
    >
      <Trash2Icon class="size-3.5" />
    </button>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed } from "vue";
import { useI18n } from "vue-i18n";

import { Trash2Icon } from "@lucide/vue";

import { useAttachmentUrl } from "./useAttachmentUrl";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();

const ALIGN_CLASS: Record<string, string> = {
  left: "justify-start",
  center: "justify-center",
  right: "justify-end",
};

const align = computed(() => String(props.node.attrs.align ?? "center"));
const attachmentId = computed(() => (props.node.attrs.attachmentId as string | null) ?? null);
const src = useAttachmentUrl(attachmentId);
const frameStyle = computed(() => {
  const width = Number(props.node.attrs.width ?? 0);
  return width > 0 ? { width: `${width}px` } : {};
});
</script>
