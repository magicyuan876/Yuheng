<template>
  <NodeViewWrapper
    as="div"
    class="relative my-3 rounded-md border border-l-[3px] border-[var(--td-component-stroke)] bg-[var(--td-bg-color-container-select)] px-3 pt-2.5 pb-1.5"
    :class="[
      selected ? 'outline-primary outline-2 outline-offset-1 outline-solid' : '',
      state === 'missing' ? 'border-l-[var(--td-text-color-placeholder)]' : 'border-l-[var(--td-brand-color)]',
    ]"
    :data-state="state"
  >
    <!-- The quoted block is somebody else's text: it is shown, never typed into. -->
    <div class="cursor-default select-text" contenteditable="false">
      <!-- eslint-disable-next-line vue/no-v-html -->
      <div v-if="state === 'ok'" class="transclusion-content" v-html="html" />
      <p v-else-if="state === 'pending'" class="text-placeholder m-0 flex items-center gap-1 text-[13px]">
        {{ t("docs.transclusion.pending") }}
      </p>
      <p v-else-if="state === 'missing'" class="text-muted-foreground m-0 flex items-center gap-1 text-[13px]">
        <UnlinkIcon class="size-3.5" />
        {{ t("docs.transclusion.missing") }}
      </p>
      <p v-else class="text-placeholder m-0 flex items-center gap-1 text-[13px]">{{ t("docs.links.loading") }}</p>
    </div>

    <footer
      class="text-placeholder mt-2 flex items-center gap-1 border-t border-dashed border-[var(--td-component-stroke)] pt-1.5 text-xs"
      contenteditable="false"
    >
      <TextQuoteIcon class="size-[13px]" />
      <span>{{ t("docs.transclusion.from") }}</span>
      <a
        v-if="resolved?.title"
        class="text-primary no-underline hover:underline"
        :href="href"
        @click.prevent="openSource"
      >
        <span v-if="resolved.icon" class="mr-0.5">{{ resolved.icon }}</span>
        {{ resolved.title }}
      </a>
      <span v-else class="text-placeholder">{{ t("docs.links.broken") }}</span>
    </footer>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { generateHTML } from "@tiptap/core";
import { NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed, inject } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

import { TextQuoteIcon, UnlinkIcon } from "@lucide/vue";

import { pageSlug } from "../tree/pageTree";

import { officialExtensions } from "./extensions";
import { DOCS_BLOCK_REFS, type BlockRefHandle } from "./linkContext";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();
const router = useRouter();

const refs = inject<BlockRefHandle | null>(DOCS_BLOCK_REFS, null);

const address = computed(() => ({
  sourcePageId: String(props.node.attrs.sourcePageId ?? ""),
  sourceBlockId: String(props.node.attrs.sourceBlockId ?? ""),
}));

/**
 * What the reference currently shows.
 *
 * The text is never in this document: it is looked up under the *reader's*
 * permissions, which is what keeps a reference to a page they may not open
 * from showing its contents, and what makes a correction on the source page
 * reach this one without an edit here.
 */
const resolved = computed(() => {
  // Touching the revision makes this recompute when an answer arrives.
  void refs?.revision.value;
  const { sourcePageId, sourceBlockId } = address.value;
  if (!sourcePageId || !sourceBlockId) return undefined;
  return refs?.get({ sourcePageId, sourceBlockId });
});

/** Undefined while the answer is still on its way. */
const state = computed(() => resolved.value?.state);

/**
 * The block, rendered read-only.
 *
 * Rendered to markup rather than mounted as an editor: there is one of these
 * per reference and a page may hold dozens, and none of them is editable
 * here. Nothing in the markup can execute — the schema has no script, iframe
 * or style node type, so no such element can exist to be rendered, and every
 * piece of text goes out through ProseMirror's serialiser, which escapes it.
 * That is the same guarantee the paste path rests on.
 */
const html = computed(() => {
  const content = resolved.value?.content;
  if (!content) return "";
  try {
    return generateHTML({ type: "doc", content: [content as Record<string, unknown>] }, officialExtensions() as never);
  } catch {
    // A block using a node this build does not implement yet. Showing the
    // frame with nothing in it is better than breaking the page around it.
    return "";
  }
});

const href = computed(() => {
  const shortId = resolved.value?.sourceShortId;
  return shortId ? `/docs/p/${pageSlug(shortId, resolved.value?.title ?? "")}` : "#";
});

function openSource() {
  const shortId = resolved.value?.sourceShortId;
  if (!shortId) return;
  void router.push(href.value);
}
</script>

<style scoped>
/* The quoted content's first and last blocks lose their outer margins so the
   quote does not look padded twice; rendered HTML, so :deep is required. */
.transclusion-content :deep(> *:first-child) {
  margin-top: 0;
}

.transclusion-content :deep(> *:last-child) {
  margin-bottom: 0;
}
</style>
