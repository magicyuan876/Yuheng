<template>
  <NodeViewWrapper as="span" class="inline">
    <a
      class="inline-flex items-baseline gap-[3px] rounded-[3px] border-b px-[3px] no-underline"
      :class="[stateClass, selected ? 'outline-primary outline-2 outline-offset-1 outline-solid' : '']"
      :href="href"
      :title="resolved === false ? t('docs.links.brokenHint') : page?.title"
      @click.prevent="open"
    >
      <span v-if="page?.icon" class="text-xs">{{ page.icon }}</span>
      <UnlinkIcon v-else-if="resolved === false" class="size-[13px]" />
      <FileIcon v-else class="size-[13px]" />
      <span class="whitespace-nowrap">{{ label }}</span>
    </a>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed, inject } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

import { FileIcon, UnlinkIcon } from "@lucide/vue";

import { pageSlug } from "../tree/pageTree";

import { DOCS_TITLE_CACHE, type TitleCacheHandle } from "./linkContext";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();
const router = useRouter();

const cache = inject<TitleCacheHandle | null>(DOCS_TITLE_CACHE, null);
const pageId = computed(() => String(props.node.attrs.pageId ?? ""));

/**
 * The link's text is never stored in the document, only looked up. That is
 * what makes renaming a page update every link to it, and what keeps a link to
 * a page the reader cannot open from showing its title.
 */
const page = computed(() => {
  // Touching the revision makes this recompute when an answer arrives.
  void cache?.revision.value;
  return cache?.get(pageId.value);
});

const resolved = computed(() => (page.value === undefined ? undefined : page.value.resolved));

const label = computed(() => {
  if (page.value === undefined) return t("docs.links.loading");
  if (!page.value.resolved) return t("docs.links.broken");
  return page.value.title || t("docs.tree.untitled");
});

const href = computed(() => {
  const p = page.value;
  if (!p?.resolved || !p.spaceId || !p.shortId) return "#";
  return `/docs/spaces/${p.spaceId}/${pageSlug(p.title, p.shortId)}`;
});

// One class set per state, so no two states fight over the same property: a
// broken link is dashed and inert, a loading one hides its underline, a resolved
// one is the brand-coloured link with the light hover.
const stateClass = computed(() => {
  if (resolved.value === false) {
    return "text-placeholder cursor-default border-dashed border-[var(--td-text-color-placeholder)]";
  }
  if (page.value === undefined)
    return "text-placeholder cursor-pointer border-transparent hover:bg-[var(--td-brand-color-light)]";
  return "text-primary cursor-pointer border-[var(--td-brand-color-4)] hover:bg-[var(--td-brand-color-light)]";
});

function open() {
  const p = page.value;
  if (!p?.resolved || !p.spaceId || !p.shortId) return;
  void router.push(href.value);
}
</script>
