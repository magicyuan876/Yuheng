<template>
  <NodeViewWrapper class="docs-embed" :class="[`docs-embed--${align}`, { 'docs-embed--selected': selected }]">
    <div class="docs-embed-frame" :style="frameStyle">
      <!-- The address is the one the server derived from its allow-list, never
           the one stored on the node. Sandboxed, with no referrer and no
           access to this document. -->
      <iframe
        v-if="embedUrl"
        :src="embedUrl"
        :title="provider"
        sandbox="allow-scripts allow-same-origin allow-popups allow-presentation allow-forms"
        referrerpolicy="no-referrer"
        loading="lazy"
        allowfullscreen
      />
      <p v-else-if="checking" class="docs-embed-note">{{ t("docs.media.embedChecking") }}</p>
      <p v-else class="docs-embed-note docs-embed-note--refused">
        <t-icon name="error-circle" size="14px" />
        <span>{{ t("docs.media.embedRefused") }}</span>
        <a :href="url" target="_blank" rel="noopener noreferrer nofollow">{{ t("docs.media.openInTab") }}</a>
      </p>
    </div>

    <div v-if="editor.isEditable" class="docs-embed-tools">
      <a class="docs-embed-source" :href="url" target="_blank" rel="noopener noreferrer nofollow">{{ host }}</a>
      <button type="button" :aria-label="t('docs.attachments.remove')" @click="deleteNode">
        <t-icon name="delete" size="14px" />
      </button>
    </div>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed, inject } from "vue";
import { useI18n } from "vue-i18n";

import { DOCS_EMBEDS, type EmbedResolverHandle } from "./linkContext";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();

const resolver = inject<EmbedResolverHandle | null>(DOCS_EMBEDS, null);

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

<style scoped lang="less">
.docs-embed {
  display: flex;
  flex-direction: column;
  margin: 12px 0;

  &--left {
    align-items: flex-start;
  }

  &--center {
    align-items: center;
  }

  &--right {
    align-items: flex-end;
  }

  &--selected .docs-embed-frame {
    outline: 2px solid var(--td-brand-color);
    outline-offset: 2px;
  }
}

.docs-embed-frame {
  position: relative;
  width: 100%;
  max-width: 100%;
  border-radius: 8px;
  overflow: hidden;
  background: var(--td-bg-color-secondarycontainer);

  iframe {
    display: block;
    width: 100%;
    height: 100%;
    border: none;
  }
}

.docs-embed-note {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 100%;
  margin: 0;
  font-size: 13px;
  color: var(--td-text-color-placeholder);

  &--refused {
    color: var(--td-error-color);
  }

  a {
    color: var(--td-brand-color);
  }
}

.docs-embed-tools {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  font-size: 12px;

  button {
    border: none;
    background: transparent;
    color: var(--td-text-color-placeholder);
    border-radius: 4px;
    padding: 2px;
    line-height: 0;
    cursor: pointer;

    &:hover {
      color: var(--td-error-color);
    }
  }
}

.docs-embed-source {
  color: var(--td-text-color-placeholder);
  text-decoration: none;

  &:hover {
    color: var(--td-brand-color);
  }
}
</style>
