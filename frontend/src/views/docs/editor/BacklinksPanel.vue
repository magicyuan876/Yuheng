<template>
  <section v-if="entries.length || loading" class="docs-backlinks">
    <h3 class="docs-backlinks-title">{{ t("docs.links.backlinks") }}</h3>
    <p v-if="loading && !entries.length" class="docs-backlinks-note">{{ t("docs.links.searching") }}</p>
    <ul v-else class="docs-backlinks-list">
      <li v-for="entry in entries" :key="entry.page_id">
        <RouterLink :to="linkTo(entry)">
          <span v-if="entry.icon" class="docs-backlinks-icon">{{ entry.icon }}</span>
          <t-icon v-else name="file" size="14px" />
          <span>{{ entry.title || t("docs.tree.untitled") }}</span>
        </RouterLink>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";

import type { PageRef } from "@/api/docs";

import { pageSlug } from "../tree/pageTree";

defineProps<{ entries: PageRef[]; loading: boolean }>();
const { t } = useI18n();

function linkTo(entry: PageRef): string {
  if (!entry.space_id || !entry.short_id) return "#";
  return `/docs/spaces/${entry.space_id}/${pageSlug(entry.title, entry.short_id)}`;
}
</script>

<style scoped lang="less">
.docs-backlinks {
  margin-top: 28px;
  padding-top: 16px;
  border-top: 1px solid var(--td-component-stroke);
}

.docs-backlinks-title {
  margin: 0 0 8px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--td-text-color-placeholder);
}

.docs-backlinks-note {
  margin: 0;
  font-size: 13px;
  color: var(--td-text-color-placeholder);
}

.docs-backlinks-list {
  list-style: none;
  margin: 0;
  padding: 0;

  li {
    margin: 3px 0;
  }

  a {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 13.5px;
    color: var(--td-brand-color);
    text-decoration: none;

    &:hover {
      text-decoration: underline;
    }
  }
}

.docs-backlinks-icon {
  font-size: 13px;
}
</style>
