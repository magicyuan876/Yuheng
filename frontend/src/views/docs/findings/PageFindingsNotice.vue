<template>
  <!-- A quiet line under the page header, not an alert: overlapping another
       page is worth knowing, rarely urgent, and the reader may well have
       written both on purpose. It can be put away for the rest of the
       session, per page. -->
  <div
    v-if="visible"
    class="border-warning/30 bg-warning/5 text-foreground flex flex-wrap items-center gap-x-2 gap-y-1 rounded-md border px-3 py-1.5 text-[13px]"
    role="status"
    data-testid="page-findings-notice"
  >
    <component
      :is="divergentCount ? GitCompareIcon : pairCount ? CopyIcon : CalendarClockIcon"
      class="text-warning size-3.5 shrink-0"
      aria-hidden="true"
    />
    <span data-testid="page-findings-count">{{ headline }}</span>
    <span v-if="otherCount > 0" class="text-muted-foreground" data-testid="page-findings-other">
      {{ t("docs.findings.more", { count: otherCount }) }}
    </span>

    <Popover v-model:open="detailsOpen">
      <PopoverTrigger as-child>
        <Button variant="link" size="xs" class="h-auto px-1" data-testid="page-findings-details">
          {{ t("docs.findings.details") }}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" class="max-h-[70vh] w-[min(640px,90vw)] overflow-y-auto">
        <div class="text-foreground text-sm font-semibold">{{ t("docs.findings.title") }}</div>
        <ul class="m-0 flex list-none flex-col gap-3 p-0">
          <li v-for="item in items" :key="item.id" class="flex flex-col gap-1.5" data-testid="page-findings-item">
            <!-- A finding about this page and another. -->
            <template v-if="item.related_page">
              <div class="flex flex-wrap items-center gap-2">
                <RouterLink
                  :to="linkTo(item.related_page)"
                  class="text-primary min-w-0 truncate font-medium hover:underline"
                  @click="detailsOpen = false"
                >
                  {{ item.related_page.title || t("docs.tree.untitled") }}
                </RouterLink>
                <Badge variant="secondary">{{ findingTypeLabel(item.type, t) }}</Badge>
                <span class="text-muted-foreground text-xs">
                  {{ t("knowledgeHealth.similarity", { value: formatPercent(item.score) }) }} ·
                  {{ t("knowledgeHealth.overlap", { value: formatPercent(item.overlap_ratio) }) }}
                </span>
                <span v-if="item.assignee" class="text-muted-foreground text-xs" data-testid="page-findings-assignee">
                  {{ t("docs.findings.assignee", { name: item.assignee.username || item.assignee.id }) }}
                </span>
              </div>
              <p v-if="findingTypeHint(item.type, t)" class="text-muted-foreground m-0 text-xs">
                {{ findingTypeHint(item.type, t) }}
              </p>
              <!-- The writer of the newer version is the one who knows it
                   replaces the other: one click, confirmed by a second. The
                   other page stays readable and says what replaced it. -->
              <div v-if="canEdit" class="flex flex-wrap items-center gap-2">
                <Button
                  v-if="confirmingId !== item.id"
                  variant="outline"
                  size="xs"
                  :disabled="busyId !== null"
                  data-testid="page-findings-supersede"
                  @click="confirmingId = item.id"
                >
                  <ReplaceIcon />
                  {{ t("docs.findings.supersede") }}
                </Button>
                <template v-else>
                  <span class="text-muted-foreground text-xs">
                    {{
                      t("docs.findings.supersedeConfirm", { title: item.related_page.title || t("docs.tree.untitled") })
                    }}
                  </span>
                  <Button
                    size="xs"
                    :disabled="busyId !== null"
                    data-testid="page-findings-supersede-ok"
                    @click="supersede(item)"
                  >
                    {{ t("docs.findings.supersedeOk") }}
                  </Button>
                  <Button variant="ghost" size="xs" @click="confirmingId = null">{{ t("common.cancel") }}</Button>
                </template>
              </div>
              <FindingEvidenceList
                v-if="item.evidence.length"
                :evidence="item.evidence"
                :subject-title="pageTitle || t('docs.findings.thisPage')"
                :related-title="item.related_page.title || t('docs.tree.untitled')"
              />
            </template>
            <!-- A finding about this page alone: a review that is due. -->
            <template v-else>
              <div class="flex flex-wrap items-center gap-2">
                <Badge variant="secondary">{{ findingTypeLabel(item.type, t) }}</Badge>
                <span v-if="item.assignee" class="text-muted-foreground text-xs" data-testid="page-findings-assignee">
                  {{ t("docs.findings.assignee", { name: item.assignee.username || item.assignee.id }) }}
                </span>
              </div>
              <p v-if="findingTypeHint(item.type, t)" class="text-muted-foreground m-0 text-xs">
                {{ findingTypeHint(item.type, t) }}
              </p>
              <p v-if="item.type === 'disputed'" class="text-muted-foreground m-0 text-xs">
                {{ t("knowledgeHealth.disputeCount", { count: disputeCount(item) }) }}
              </p>
              <div v-if="canEdit && CONFIRMABLE_FINDING_TYPES.has(item.type)">
                <Button
                  variant="outline"
                  size="xs"
                  :disabled="busyId !== null"
                  data-testid="page-findings-confirm"
                  @click="confirmReviewed(item)"
                >
                  <BadgeCheckIcon />
                  {{ t("knowledgeHealth.confirm") }}
                </Button>
              </div>
            </template>
          </li>
        </ul>
        <p v-if="otherCount > 0" class="text-placeholder m-0 text-xs">
          {{ t("docs.findings.more", { count: otherCount }) }}
        </p>
      </PopoverContent>
    </Popover>

    <Button
      variant="ghost"
      size="icon-xs"
      class="text-muted-foreground ml-auto"
      :aria-label="t('docs.findings.dismiss')"
      :title="t('docs.findings.dismiss')"
      data-testid="page-findings-dismiss"
      @click="dismiss"
    >
      <XIcon />
    </Button>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { RouterLink } from "vue-router";
import { MessagePlugin } from "tdesign-vue-next";
import { BadgeCheckIcon, CalendarClockIcon, CopyIcon, GitCompareIcon, ReplaceIcon, XIcon } from "@lucide/vue";

import { confirmPageReviewed } from "@/api/docs";
import { listPageFindings, supersedeFromPage, type PageFinding, type RelatedDocsPage } from "@/api/findings";
import FindingEvidenceList from "@/components/findings/FindingEvidenceList.vue";
import {
  CONFIRMABLE_FINDING_TYPES,
  findingTypeHint,
  findingTypeLabel,
  formatPercent,
} from "@/components/findings/findingDisplay";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

import { IdleScheduler } from "../editor/idleWork";
import { pageSlug } from "../tree/pageTree";

const props = defineProps<{
  pageId: string;
  pageTitle?: string;
  /** The space's knowledge base; without one there is nothing to compare against. */
  knowledgeBaseId?: string | null;
  /** A page kept out of the knowledge base is never checked. */
  excluded?: boolean;
  /** The reader may edit the page, and so say it supersedes another. */
  canEdit?: boolean;
}>();

const { t } = useI18n();

const items = ref<PageFinding[]>([]);
const otherCount = ref(0);
const dismissed = ref(false);
const detailsOpen = ref(false);
/** The finding whose "supersede" awaits its confirming second click. */
const confirmingId = ref<string | null>(null);
/** The finding an action is on its way to the server for. */
const busyId = ref<string | null>(null);

/** The other page leaves the knowledge base and every finding naming it
 * goes with it, so the list is fetched again rather than patched. */
async function supersede(item: PageFinding) {
  if (busyId.value || !item.related_page) return;
  const title = item.related_page.title || t("docs.tree.untitled");
  busyId.value = item.id;
  try {
    await supersedeFromPage(props.pageId, item.id);
    void MessagePlugin.success(t("docs.findings.superseded", { title }));
    confirmingId.value = null;
    await load(props.pageId);
  } catch (err) {
    const msg = err instanceof Error ? err.message : "";
    void MessagePlugin.error(
      msg ? `${t("docs.findings.supersedeFailed")}: ${msg}` : t("docs.findings.supersedeFailed"),
    );
  } finally {
    busyId.value = null;
  }
}

/** Vouching for the page settles its review at the check that follows; the
 * finding leaves the notice now, the person having done what it asked. */
async function confirmReviewed(item: PageFinding) {
  if (busyId.value) return;
  busyId.value = item.id;
  try {
    await confirmPageReviewed(props.pageId);
    items.value = items.value.filter((i) => i.id !== item.id);
    void MessagePlugin.success(t("knowledgeHealth.confirmedPending"));
  } catch (err) {
    const msg = err instanceof Error ? err.message : "";
    void MessagePlugin.error(
      msg ? `${t("knowledgeHealth.confirmFailed")}: ${msg}` : t("knowledgeHealth.confirmFailed"),
    );
  } finally {
    busyId.value = null;
  }
}

/** Pages that say nearly what this one says, but not quite: worth more
 * attention than a copy, so the notice leads with them. */
const divergentCount = computed(() => items.value.filter((i) => i.type === "divergent").length);
const disputeCount = (item: PageFinding) => {
  const count = item.extra?.count;
  return typeof count === "number" ? count : 0;
};

/** Findings about this page and another; the rest are about this page alone. */
const pairCount = computed(() => items.value.filter((i) => i.related_page).length);

const headline = computed(() => {
  if (divergentCount.value) return t("docs.findings.noticeDivergent", { count: divergentCount.value });
  if (pairCount.value) return t("docs.findings.notice", { count: pairCount.value });
  if (items.value.some((i) => i.type === "disputed")) return t("docs.findings.disputed");
  return t("docs.findings.reviewDue");
});

const eligible = computed(() => !!props.pageId && !!props.knowledgeBaseId && !props.excluded);
const visible = computed(() => eligible.value && !dismissed.value && items.value.length > 0);

// Kept per browser tab: a person who put the notice away does not want it
// back on every visit to the page today, but a new session is a fair time to
// mention it again.
const STORAGE_PREFIX = "yuheng.docs.findingsNotice.dismissed.";

function readDismissed(pageId: string): boolean {
  try {
    return sessionStorage.getItem(STORAGE_PREFIX + pageId) === "1";
  } catch {
    return false;
  }
}

function dismiss() {
  dismissed.value = true;
  detailsOpen.value = false;
  try {
    sessionStorage.setItem(STORAGE_PREFIX + props.pageId, "1");
  } catch {
    // Storage can be unavailable (private mode, quota); hiding it for this
    // view is still what was asked.
  }
}

const linkTo = (p: RelatedDocsPage) => ({
  name: "docsSpace",
  params: { slug: p.space_slug, pageSlug: pageSlug(p.title, p.short_id) },
});

// The request is made once the browser is idle after the page has rendered,
// never on the page's own critical path: the notice is a side note, and the
// editor and its content matter more. A failure is only logged — a missing
// side note is not worth an error over the page.
let seq = 0;
let scheduler: IdleScheduler | null = null;

async function load(pageId: string) {
  const mine = ++seq;
  try {
    const res = await listPageFindings(pageId);
    if (mine !== seq || pageId !== props.pageId || !eligible.value) return;
    items.value = res.items;
    otherCount.value = res.other_count;
  } catch (err) {
    if (mine === seq) console.debug("docs: page findings unavailable", err);
  }
}

watch(
  () => [props.pageId, eligible.value] as const,
  ([pageId, ok]) => {
    scheduler?.cancel();
    scheduler = null;
    seq++;
    items.value = [];
    otherCount.value = 0;
    detailsOpen.value = false;
    if (!ok || !pageId) return;
    dismissed.value = readDismissed(pageId);
    if (dismissed.value) return;
    scheduler = new IdleScheduler(() => void load(pageId));
    scheduler.schedule();
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  scheduler?.cancel();
  seq++;
});
</script>
