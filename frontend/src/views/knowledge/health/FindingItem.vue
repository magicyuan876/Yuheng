<template>
  <li
    class="border-border bg-card rounded-lg border px-4 py-3"
    data-testid="finding-item"
    :data-finding-id="finding.id"
  >
    <div class="flex flex-wrap items-center gap-2">
      <Badge variant="secondary">{{ findingTypeLabel(finding.type, t) }}</Badge>
      <Badge variant="outline" :class="severityBadgeClass(finding.severity)">
        {{ severityLabel(finding.severity, t) }}
      </Badge>
      <Badge v-if="finding.status !== 'open'" variant="outline">{{ statusLabel }}</Badge>
      <span class="text-placeholder ml-auto text-xs">{{ formatDate(finding.updated_at || finding.created_at) }}</span>
    </div>

    <!-- The two documents, each opening the same detail drawer the document
         list opens. A finding about one document alone has no right side. -->
    <div class="mt-2 flex flex-wrap items-center gap-1.5 text-sm">
      <button
        type="button"
        data-slot="finding-doc-link"
        data-testid="finding-subject"
        class="text-primary max-w-full cursor-pointer truncate text-left font-medium hover:underline"
        @click="emit('open-knowledge', finding.subject.knowledge_id)"
      >
        {{ finding.subject.title || finding.subject.knowledge_id }}
      </button>
      <template v-if="finding.related">
        <ArrowLeftRightIcon class="text-placeholder size-3.5 shrink-0" aria-hidden="true" />
        <button
          type="button"
          data-slot="finding-doc-link"
          data-testid="finding-related"
          class="text-primary max-w-full cursor-pointer truncate text-left font-medium hover:underline"
          @click="emit('open-knowledge', finding.related.knowledge_id)"
        >
          {{ finding.related.title || finding.related.knowledge_id }}
        </button>
      </template>
    </div>

    <div class="text-muted-foreground mt-1.5 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
      <span data-testid="finding-score">{{
        t("knowledgeHealth.similarity", { value: formatPercent(finding.score) })
      }}</span>
      <span :title="t('knowledgeHealth.overlapHint')">
        {{ t("knowledgeHealth.overlap", { value: formatPercent(finding.overlap_ratio) }) }}
      </span>
      <Button
        v-if="finding.evidence.length"
        variant="ghost"
        size="xs"
        class="text-muted-foreground -ml-2"
        :aria-expanded="expanded"
        data-testid="finding-evidence-toggle"
        @click="expanded = !expanded"
      >
        <ChevronDownIcon class="transition-transform" :class="expanded ? 'rotate-180' : ''" />
        {{
          expanded
            ? t("knowledgeHealth.hideEvidence")
            : t("knowledgeHealth.showEvidence", { count: finding.evidence.length })
        }}
      </Button>
      <div class="ml-auto flex gap-1">
        <!-- Resolved findings are the detector's to close: the content changed
             and the overlap is gone, so there is nothing to dismiss or reopen. -->
        <Button
          v-if="finding.status === 'open'"
          variant="outline"
          size="xs"
          :disabled="busy"
          data-testid="finding-dismiss"
          @click="emit('dismiss', finding)"
        >
          <EyeOffIcon />
          {{ t("knowledgeHealth.dismiss") }}
        </Button>
        <Button
          v-else-if="finding.status === 'dismissed'"
          variant="outline"
          size="xs"
          :disabled="busy"
          data-testid="finding-reopen"
          @click="emit('reopen', finding)"
        >
          <RotateCcwIcon />
          {{ t("knowledgeHealth.reopen") }}
        </Button>
      </div>
    </div>

    <FindingEvidenceList
      v-if="expanded"
      class="mt-3"
      :evidence="finding.evidence"
      :subject-title="finding.subject.title"
      :related-title="finding.related?.title ?? ''"
    />
  </li>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import { ArrowLeftRightIcon, ChevronDownIcon, EyeOffIcon, RotateCcwIcon } from "@lucide/vue";

import type { Finding } from "@/api/findings";
import FindingEvidenceList from "@/components/findings/FindingEvidenceList.vue";
import {
  findingTypeLabel,
  formatPercent,
  severityBadgeClass,
  severityLabel,
} from "@/components/findings/findingDisplay";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

const props = defineProps<{
  finding: Finding;
  /** A status change for this finding is on its way to the server. */
  busy?: boolean;
}>();

const emit = defineEmits<{
  "open-knowledge": [knowledgeId: string];
  dismiss: [finding: Finding];
  reopen: [finding: Finding];
}>();

const { t } = useI18n();
const expanded = ref(false);

const statusLabel = computed(() => {
  switch (props.finding.status) {
    case "dismissed":
      return t("knowledgeHealth.status.dismissed");
    case "resolved":
      return t("knowledgeHealth.status.resolved");
    default:
      return t("knowledgeHealth.status.open");
  }
});

const formatDate = (iso: string | null | undefined) => {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
};
</script>
