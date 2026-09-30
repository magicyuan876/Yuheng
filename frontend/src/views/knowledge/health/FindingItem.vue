<template>
  <li
    class="border-border bg-card rounded-lg border px-4 py-3"
    data-testid="finding-item"
    :data-finding-id="finding.id"
    :data-finding-type="finding.type"
  >
    <div class="flex flex-wrap items-center gap-2">
      <Badge variant="secondary">{{ findingTypeLabel(finding.type, t) }}</Badge>
      <Badge variant="outline" :class="severityBadgeClass(finding.severity)">
        {{ severityLabel(finding.severity, t) }}
      </Badge>
      <Badge v-if="finding.status !== 'open'" variant="outline" data-testid="finding-status">{{ statusLabel }}</Badge>
      <span v-if="finding.knowledge_base_name" class="text-muted-foreground truncate text-xs">
        {{ finding.knowledge_base_name }}
      </span>
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
    <p v-if="typeHint" class="text-muted-foreground m-0 mt-1 text-xs" data-testid="finding-hint">{{ typeHint }}</p>

    <div class="text-muted-foreground mt-1.5 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
      <span data-testid="finding-score">{{
        t("knowledgeHealth.similarity", { value: formatPercent(finding.score) })
      }}</span>
      <span :title="t('knowledgeHealth.overlapHint')">
        {{ t("knowledgeHealth.overlap", { value: formatPercent(finding.overlap_ratio) }) }}
      </span>
      <!-- Who deals with it. Routed by the documents' stewardship unless a
           person chose; an editor of the knowledge base can choose again. -->
      <span class="inline-flex items-center gap-1" data-testid="finding-assignee">
        <UserRoundIcon class="size-3.5" aria-hidden="true" />
        <template v-if="finding.assignee">
          {{ finding.assignee.username || finding.assignee.id }}
          <span v-if="!finding.assignee.active" class="text-warning">{{ t("knowledgeHealth.assigneeInactive") }}</span>
          <span v-if="finding.assigned_manually" class="text-placeholder">{{
            t("knowledgeHealth.assignedByHand")
          }}</span>
        </template>
        <span v-else class="text-placeholder">{{ t("knowledgeHealth.unassigned") }}</span>
        <MemberPicker
          v-if="canEdit && finding.status === 'open'"
          :current-id="finding.assignee?.id"
          allow-none
          :none-label="t('knowledgeHealth.assignAutomatic')"
          @select="(id) => emit('assign', finding, id)"
        >
          <template #trigger>
            <Button variant="ghost" size="xs" class="h-5 px-1" :disabled="busy" data-testid="finding-assign">
              {{ t("knowledgeHealth.assign") }}
            </Button>
          </template>
        </MemberPicker>
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
        <!-- A host's own actions, e.g. "go and deal with it" in the to-do
             list, where the finding is read rather than acted on. -->
        <slot name="actions" />
        <!-- Dismissing says why: the two reasons a person has to leave a
             finding as it is. Resolved findings are the detector's to close:
             the content changed and the overlap is gone. -->
        <DropdownMenu v-if="canEdit && finding.status === 'open'">
          <DropdownMenuTrigger as-child>
            <Button variant="outline" size="xs" :disabled="busy" data-testid="finding-dismiss">
              <EyeOffIcon />
              {{ t("knowledgeHealth.dismiss") }}
              <ChevronDownIcon />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem
              v-for="reason in DISMISS_REASONS"
              :key="reason"
              :data-testid="`finding-dismiss-${reason}`"
              @select="emit('dismiss', finding, reason)"
            >
              <div class="flex flex-col">
                <span>{{ t(`knowledgeHealth.resolution.${reason}`) }}</span>
                <span class="text-muted-foreground text-xs">{{ t(`knowledgeHealth.resolutionHint.${reason}`) }}</span>
              </div>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <Button
          v-else-if="canEdit && finding.status === 'dismissed'"
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
import { ArrowLeftRightIcon, ChevronDownIcon, EyeOffIcon, RotateCcwIcon, UserRoundIcon } from "@lucide/vue";

import type { Finding, FindingDismissReason } from "@/api/findings";
import FindingEvidenceList from "@/components/findings/FindingEvidenceList.vue";
import MemberPicker from "@/components/findings/MemberPicker.vue";
import {
  findingTypeHint,
  findingTypeLabel,
  formatPercent,
  resolutionLabel,
  severityBadgeClass,
  severityLabel,
} from "@/components/findings/findingDisplay";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

const props = defineProps<{
  finding: Finding;
  /** A change to this finding is on its way to the server. */
  busy?: boolean;
  /** The caller may edit the knowledge base: dismiss, reopen, assign. */
  canEdit?: boolean;
}>();

const emit = defineEmits<{
  "open-knowledge": [knowledgeId: string];
  dismiss: [finding: Finding, reason: FindingDismissReason];
  reopen: [finding: Finding];
  /** An empty ID hands the finding back to the automatic routing. */
  assign: [finding: Finding, assigneeId: string];
}>();

const DISMISS_REASONS: FindingDismissReason[] = ["distinct_scope", "intentional"];

const { t } = useI18n();
const expanded = ref(false);

const typeHint = computed(() => findingTypeHint(props.finding.type, t));

const statusLabel = computed(() => {
  switch (props.finding.status) {
    case "dismissed": {
      const why = resolutionLabel(props.finding.resolution, t);
      return why ? `${t("knowledgeHealth.status.dismissed")} · ${why}` : t("knowledgeHealth.status.dismissed");
    }
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
