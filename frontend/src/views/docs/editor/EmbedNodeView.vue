<template>
  <NodeViewWrapper class="my-3 flex flex-col" :class="ALIGN_CLASS[align] ?? ALIGN_CLASS.center">
    <div
      class="bg-secondary relative w-full max-w-full overflow-hidden rounded-[8px]"
      :class="selected ? 'outline-primary outline-2 outline-offset-2' : ''"
      :style="frameStyle"
    >
      <!-- The address is the one the server derived from its allow-list, never
           the one stored on the node. Sandboxed, with no referrer and no
           access to this document. -->
      <iframe
        v-if="embedUrl"
        class="block h-full w-full border-0"
        :src="embedUrl"
        :title="provider"
        sandbox="allow-scripts allow-same-origin allow-popups allow-presentation allow-forms"
        referrerpolicy="no-referrer"
        loading="lazy"
        allowfullscreen
      />
      <p v-else-if="checking" class="text-placeholder m-0 flex h-full items-center justify-center gap-1.5 text-[13px]">
        {{ t("docs.media.embedChecking") }}
      </p>
      <p v-else class="text-destructive m-0 flex h-full items-center justify-center gap-1.5 text-[13px]">
        <CircleXIcon class="size-3.5" />
        <span>{{ t("docs.media.embedRefused") }}</span>
        <a class="text-primary" :href="url" target="_blank" rel="noopener noreferrer nofollow">
          {{ t("docs.media.openInTab") }}
        </a>
      </p>
    </div>

    <div v-if="editor.isEditable" class="mt-1 flex items-center gap-2 text-xs">
      <a
        class="text-placeholder hover:text-primary no-underline"
        :href="url"
        target="_blank"
        rel="noopener noreferrer nofollow"
      >
        {{ host }}
      </a>
      <button
        type="button"
        data-slot="embed-remove"
        class="text-placeholder hover:text-destructive cursor-pointer rounded border-0 p-0.5 leading-0"
        :aria-label="t('docs.attachments.remove')"
        @click="deleteNode"
      >
        <Trash2Icon class="size-3.5" />
      </button>
    </div>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed, inject } from "vue";
import { useI18n } from "vue-i18n";

import { CircleXIcon, Trash2Icon } from "@lucide/vue";

import { DOCS_EMBEDS, type EmbedResolverHandle } from "./linkContext";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();

const resolver = inject<EmbedResolverHandle | null>(DOCS_EMBEDS, null);

const ALIGN_CLASS: Record<string, string> = {
  left: "items-start",
  center: "items-center",
  right: "items-end",
};

const provider = computed(() => String(props.node.attrs.provider ?? ""));
const url = computed(() => String(props.node.attrs.url ?? ""));
const align = computed(() => String(props.node.attrs.align ?? "center"));

/**
 * What to frame, decided by the server rather than by the document.
 *
 * The node stores the address its author pasted; the frame address is derived
 * from the deployment's allow-list every time the page is opened. A deployment
 * that narrows its list therefore stops framing an existing embed at once,
 * rather than at whatever point somebody next saves the page.
 */
const answer = computed(() => {
  void resolver?.revision.value;
  return resolver?.get(provider.value, url.value);
});

const checking = computed(() => answer.value === undefined);
const embedUrl = computed(() => answer.value?.embedUrl ?? "");

const host = computed(() => {
  try {
    return new URL(url.value).hostname;
  } catch {
    return url.value;
  }
});

const frameStyle = computed(() => {
  const width = Number(props.node.attrs.width ?? 0);
  const height = Number(props.node.attrs.height ?? 0);
  return {
    ...(width > 0 ? { width: `${width}px` } : {}),
    height: `${height > 0 ? height : 420}px`,
  };
});
</script>
