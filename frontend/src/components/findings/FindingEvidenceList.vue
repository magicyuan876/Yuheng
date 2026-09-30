<template>
  <!-- One row per pair of overlapping passages: this document's words on the
       left, the other's on the right, so a reader can judge the overlap
       without opening either. The columns stack on a narrow screen. Where the
       two differ, the differences are marked: telling "15 days" from "10 days"
       is the whole point of a divergent finding. -->
  <ol class="m-0 flex list-none flex-col gap-2 p-0" data-testid="finding-evidence">
    <li
      v-for="row in rows"
      :key="row.key"
      class="border-border rounded-md border"
      :class="row.differs ? 'border-warning/40' : ''"
      :data-differs="row.differs ? 'true' : undefined"
    >
      <div class="grid grid-cols-1 md:grid-cols-2">
        <blockquote
          class="text-foreground border-border m-0 border-b px-3 py-2 text-[13px] leading-relaxed break-words whitespace-pre-wrap md:border-r md:border-b-0"
        >
          <div class="text-placeholder mb-1 truncate text-xs">{{ subjectTitle }}</div>
          <template v-for="(segment, i) in row.subject" :key="i">
            <mark v-if="segment.changed" :class="MARK" data-testid="finding-diff-mark">{{ segment.text }}</mark>
            <template v-else>{{ segment.text }}</template>
          </template>
        </blockquote>
        <blockquote class="text-foreground m-0 px-3 py-2 text-[13px] leading-relaxed break-words whitespace-pre-wrap">
          <div class="text-placeholder mb-1 truncate text-xs">{{ relatedTitle }}</div>
          <template v-for="(segment, i) in row.related" :key="i">
            <mark v-if="segment.changed" :class="MARK" data-testid="finding-diff-mark">{{ segment.text }}</mark>
            <template v-else>{{ segment.text }}</template>
          </template>
        </blockquote>
      </div>
      <div class="text-placeholder border-border flex items-center gap-2 border-t px-3 py-1 text-xs">
        <span>{{ t("knowledgeHealth.evidenceScore", { value: formatPercent(row.score) }) }}</span>
        <span v-if="row.differs" class="text-warning">{{ t("knowledgeHealth.evidenceDiffers") }}</span>
      </div>
    </li>
  </ol>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";

import type { FindingEvidence } from "@/api/findings";

import { formatPercent } from "./findingDisplay";
import { diffExcerpts, type DiffSegment } from "./textDiff";

const props = defineProps<{
  evidence: FindingEvidence[];
  subjectTitle: string;
  relatedTitle: string;
}>();

const { t } = useI18n();

const MARK = "bg-warning/25 text-foreground rounded-[2px] px-px";

interface Row {
  key: string;
  score: number;
  differs: boolean;
  subject: DiffSegment[];
  related: DiffSegment[];
}

const rows = computed<Row[]>(() =>
  props.evidence.map((pair, index) => {
    const differs = !!pair.differs;
    const segments = differs
      ? diffExcerpts(pair.subject_excerpt, pair.related_excerpt)
      : {
          subject: [{ text: pair.subject_excerpt, changed: false }],
          related: [{ text: pair.related_excerpt, changed: false }],
        };
    return {
      key: `${pair.subject_chunk_id}-${pair.related_chunk_id}-${index}`,
      score: pair.score,
      differs,
      subject: segments.subject,
      related: segments.related,
    };
  }),
);
</script>
