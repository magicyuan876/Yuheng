<template>
  <section v-if="entries.length || loading" class="mt-7 border-t border-[var(--td-component-stroke)] pt-4">
    <h3 class="text-placeholder m-0 mb-2 text-xs font-semibold tracking-[0.04em] uppercase">
      {{ t("docs.links.backlinks") }}
    </h3>
    <p v-if="loading && !entries.length" class="text-placeholder m-0 text-[13px]">{{ t("docs.links.searching") }}</p>
    <ul v-else class="m-0 list-none p-0">
      <li v-for="entry in entries" :key="entry.page_id" class="my-[3px]">
        <RouterLink
          :to="linkTo(entry)"
          class="text-primary inline-flex items-center gap-1.5 text-[13.5px] no-underline hover:underline"
        >
          <span v-if="entry.icon" class="text-[13px]">{{ entry.icon }}</span>
          <FileIcon v-else class="size-3.5" />
          <span>{{ entry.title || t("docs.tree.untitled") }}</span>
        </RouterLink>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";

import { FileIcon } from "@lucide/vue";

import type { PageRef } from "@/api/docs";

import { pageSlug } from "../tree/pageTree";

defineProps<{ entries: PageRef[]; loading: boolean }>();
const { t } = useI18n();

function linkTo(entry: PageRef): string {
  if (!entry.space_id || !entry.short_id) return "#";
  return `/docs/spaces/${entry.space_id}/${pageSlug(entry.title, entry.short_id)}`;
}
</script>
