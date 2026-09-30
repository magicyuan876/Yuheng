<template>
  <!-- One row per pair of overlapping passages: this document's words on the
       left, the other's on the right, so a reader can judge the overlap
       without opening either. The columns stack on a narrow screen. -->
  <ol class="m-0 flex list-none flex-col gap-2 p-0" data-testid="finding-evidence">
    <li
      v-for="(pair, index) in evidence"
      :key="`${pair.subject_chunk_id}-${pair.related_chunk_id}-${index}`"
      class="border-border rounded-md border"
    >
      <div class="grid grid-cols-1 md:grid-cols-2">
        <blockquote
          class="text-foreground border-border m-0 border-b px-3 py-2 text-[13px] leading-relaxed break-words whitespace-pre-wrap md:border-r md:border-b-0"
        >
          <div class="text-placeholder mb-1 truncate text-xs">{{ subjectTitle }}</div>
          {{ pair.subject_excerpt }}
        </blockquote>
        <blockquote class="text-foreground m-0 px-3 py-2 text-[13px] leading-relaxed break-words whitespace-pre-wrap">
          <div class="text-placeholder mb-1 truncate text-xs">{{ relatedTitle }}</div>
          {{ pair.related_excerpt }}
        </blockquote>
      </div>
      <div class="text-placeholder border-border border-t px-3 py-1 text-xs">
        {{ t("knowledgeHealth.evidenceScore", { value: formatPercent(pair.score) }) }}
      </div>
    </li>
  </ol>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";

import type { FindingEvidence } from "@/api/findings";

import { formatPercent } from "./findingDisplay";

defineProps<{
  evidence: FindingEvidence[];
  subjectTitle: string;
  relatedTitle: string;
}>();

const { t } = useI18n();
</script>
