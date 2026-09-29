<template>
  <NodeViewWrapper
    class="my-3 overflow-hidden rounded-[8px] border"
    :class="selected ? 'border-primary' : 'border-[var(--td-component-stroke)]'"
  >
    <div class="bg-secondary flex items-center gap-2 px-2.5 py-1.5 text-[13px]">
      <FileTextIcon class="size-4" />
      <span class="min-w-0 flex-1 truncate">{{ name }}</span>
      <a v-if="src" class="text-primary text-xs no-underline" :href="src" target="_blank" rel="noopener">
        {{ t("docs.media.openInTab") }}
      </a>
      <button
        v-if="editor.isEditable"
        type="button"
        data-slot="pdf-remove"
        class="text-placeholder hover:text-destructive cursor-pointer rounded border-0 p-[3px] leading-0"
        :aria-label="t('docs.attachments.remove')"
        @click="deleteNode"
      >
        <Trash2Icon class="size-3.5" />
      </button>
    </div>

    <!-- The browser's own viewer. The response is served with a sandboxing
         policy and an opaque origin, so the document cannot reach anything of
         the site's even if the file turns out not to be a PDF. -->
    <iframe
      v-if="src"
      class="block w-full border-0"
      :src="src"
      :style="frameStyle"
      :title="name"
      loading="lazy"
      referrerpolicy="no-referrer"
    />
    <p v-else class="text-placeholder m-0 p-4 text-[13px]">{{ t("docs.media.missing") }}</p>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed } from "vue";
import { useI18n } from "vue-i18n";

import { FileTextIcon, Trash2Icon } from "@lucide/vue";

import { useAttachmentUrl } from "./useAttachmentUrl";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();

const name = computed(() => String(props.node.attrs.name || t("docs.attachments.unnamed")));
const attachmentId = computed(() => (props.node.attrs.attachmentId as string | null) ?? null);
const src = useAttachmentUrl(attachmentId);
const frameStyle = computed(() => {
  const height = Number(props.node.attrs.height ?? 0);
  return { height: `${height > 0 ? height : 520}px` };
});
</script>
