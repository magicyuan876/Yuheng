<template>
  <NodeViewWrapper
    as="div"
    class="docs-transclusion"
    :class="{
      'docs-transclusion--selected': selected,
      'docs-transclusion--broken': state === 'missing',
    }"
    :data-state="state"
  >
    <div class="docs-transclusion-body" contenteditable="false">
      <!-- eslint-disable-next-line vue/no-v-html -->
      <div v-if="state === 'ok'" class="docs-transclusion-content" v-html="html" />
      <p v-else-if="state === 'pending'" class="docs-transclusion-note">
        {{ t("docs.transclusion.pending") }}
      </p>
      <p v-else-if="state === 'missing'" class="docs-transclusion-note docs-transclusion-note--broken">
        <t-icon name="link-unlink" size="14px" />
        {{ t("docs.transclusion.missing") }}
      </p>
      <p v-else class="docs-transclusion-note">{{ t("docs.links.loading") }}</p>
    </div>

    <footer class="docs-transclusion-source" contenteditable="false">
      <t-icon name="quote" size="13px" />
      <span>{{ t("docs.transclusion.from") }}</span>
      <a v-if="resolved?.title" class="docs-transclusion-link" :href="href" @click.prevent="openSource">
        <span v-if="resolved.icon" class="docs-transclusion-icon">{{ resolved.icon }}</span>
        {{ resolved.title }}
      </a>
      <span v-else class="docs-transclusion-unknown">{{ t("docs.links.broken") }}</span>
    </footer>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { generateHTML } from "@tiptap/core";
import { NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed, inject } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

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

<style scoped lang="less">
.docs-transclusion {
  position: relative;
  margin: 12px 0;
  padding: 10px 12px 6px;
  border: 1px solid var(--td-component-stroke);
  border-left: 3px solid var(--td-brand-color);
  border-radius: 6px;
  background: var(--td-bg-color-container-select);

  &--selected {
    outline: 2px solid var(--td-brand-color);
    outline-offset: 1px;
  }

  &--broken {
    border-left-color: var(--td-text-color-placeholder);
  }
}

// The quoted block is somebody else's text: it is shown, never typed into.
.docs-transclusion-body {
  cursor: default;
  user-select: text;
}

.docs-transclusion-content {
  // The first and last blocks inside lose their outer margins so the quote
  // does not look padded twice.
  :deep(> *:first-child) {
    margin-top: 0;
  }

  :deep(> *:last-child) {
    margin-bottom: 0;
  }
}

.docs-transclusion-note {
  margin: 0;
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: var(--td-text-color-placeholder);

  &--broken {
    color: var(--td-text-color-secondary);
  }
}

.docs-transclusion-source {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 8px;
  padding-top: 6px;
  border-top: 1px dashed var(--td-component-stroke);
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.docs-transclusion-link {
  color: var(--td-brand-color);
  text-decoration: none;

  &:hover {
    text-decoration: underline;
  }
}

.docs-transclusion-icon {
  margin-right: 2px;
}

.docs-transclusion-unknown {
  color: var(--td-text-color-placeholder);
}
</style>
