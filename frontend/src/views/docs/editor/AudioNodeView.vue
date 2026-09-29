<template>
  <NodeViewWrapper class="my-2.5 flex items-center gap-2">
    <audio
      v-if="src"
      class="max-w-[420px] flex-1"
      :class="selected ? 'outline-primary rounded-[20px] outline-2 outline-offset-2' : ''"
      :src="src"
      controls
      preload="metadata"
    />
    <p v-else class="text-placeholder m-0 text-[13px]">{{ t("docs.media.missing") }}</p>
    <button
      v-if="editor.isEditable"
      type="button"
      data-slot="audio-remove"
      class="text-placeholder hover:text-destructive hover:bg-accent cursor-pointer rounded border-0 p-1 leading-0"
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

const attachmentId = computed(() => (props.node.attrs.attachmentId as string | null) ?? null);
const src = useAttachmentUrl(attachmentId);
</script>
