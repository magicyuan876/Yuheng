<template>
  <!-- The knowledge base a space syncs into, by name and linked to it. An id the list does
       not know is shown as such: the knowledge base was deleted, or the reader may not see it. -->
  <span v-if="!id" class="text-sm">{{ t("docs.spaces.kbSync.notSynced") }}</span>
  <Skeleton v-else-if="loading" class="h-4 w-40" />
  <RouterLink
    v-else-if="knowledgeBase"
    :to="{ name: 'knowledgeBaseDetail', params: { kbId: knowledgeBase.id } }"
    class="text-primary inline-flex w-fit items-center gap-1 text-sm hover:underline"
    data-testid="kb-bound-link"
  >
    <LibraryIcon class="size-3.5" />
    {{ knowledgeBase.name || knowledgeBase.id }}
  </RouterLink>
  <span v-else class="flex flex-col gap-0.5 text-sm" data-testid="kb-bound-missing">
    <span class="font-[family-name:var(--td-font-family-mono,ui-monospace,monospace)]">{{ id }}</span>
    <span class="text-warning text-xs">{{ t("docs.spaces.kbSync.missing") }}</span>
  </span>
</template>

<script setup lang="ts">
import { LibraryIcon } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { RouterLink } from "vue-router";

import { Skeleton } from "@/components/ui/skeleton";

import type { KnowledgeBaseSummary } from "./knowledgeBaseSync";

defineProps<{
  id?: string | null;
  knowledgeBase: KnowledgeBaseSummary | null;
  loading: boolean;
}>();

const { t } = useI18n();
</script>
