<template>
  <NodeViewWrapper as="span" class="docs-pagelink-wrap">
    <a
      class="docs-pagelink"
      :class="{
        'docs-pagelink--broken': resolved === false,
        'docs-pagelink--loading': page === undefined,
        'docs-pagelink--selected': selected,
      }"
      :href="href"
      :title="resolved === false ? t('docs.links.brokenHint') : page?.title"
      @click.prevent="open"
    >
      <span v-if="page?.icon" class="docs-pagelink-icon">{{ page.icon }}</span>
      <t-icon v-else :name="resolved === false ? 'link-unlink' : 'file'" size="13px" />
      <span class="docs-pagelink-text">{{ label }}</span>
    </a>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from '@tiptap/vue-3'
import { computed, inject } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import { pageSlug } from '../tree/pageTree'

import { DOCS_TITLE_CACHE, type TitleCacheHandle } from './linkContext'

const props = defineProps<NodeViewProps>()
const { t } = useI18n()
const router = useRouter()

const cache = inject<TitleCacheHandle | null>(DOCS_TITLE_CACHE, null)
const pageId = computed(() => String(props.node.attrs.pageId ?? ''))

/**
 * The link's text is never stored in the document, only looked up. That is
 * what makes renaming a page update every link to it, and what keeps a link to
 * a page the reader cannot open from showing its title.
 */
const page = computed(() => {
  // Touching the revision makes this recompute when an answer arrives.
  void cache?.revision.value
  return cache?.get(pageId.value)
})

const resolved = computed(() => (page.value === undefined ? undefined : page.value.resolved))

const label = computed(() => {
  if (page.value === undefined) return t('docs.links.loading')
  if (!page.value.resolved) return t('docs.links.broken')
  return page.value.title || t('docs.tree.untitled')
})

const href = computed(() => {
  const p = page.value
  if (!p?.resolved || !p.spaceId || !p.shortId) return '#'
  return `/docs/spaces/${p.spaceId}/${pageSlug(p.title, p.shortId)}`
})

function open() {
  const p = page.value
  if (!p?.resolved || !p.spaceId || !p.shortId) return
  void router.push(href.value)
}
</script>

<style scoped lang="less">
.docs-pagelink-wrap {
  display: inline;
}

.docs-pagelink {
  display: inline-flex;
  align-items: baseline;
  gap: 3px;
  padding: 0 3px;
  border-radius: 3px;
  color: var(--td-brand-color);
  text-decoration: none;
  border-bottom: 1px solid var(--td-brand-color-4);
  cursor: pointer;

  &:hover {
    background: var(--td-brand-color-light);
  }

  &--broken {
    color: var(--td-text-color-placeholder);
    border-bottom-style: dashed;
    border-bottom-color: var(--td-text-color-placeholder);
    cursor: default;

    &:hover {
      background: transparent;
    }
  }

  &--loading {
    color: var(--td-text-color-placeholder);
    border-bottom-color: transparent;
  }

  &--selected {
    outline: 2px solid var(--td-brand-color);
    outline-offset: 1px;
  }
}

.docs-pagelink-icon {
  font-size: 12px;
}

.docs-pagelink-text {
  white-space: nowrap;
}
</style>
