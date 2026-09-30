<template>
  <!-- Who maintains this document and when somebody last vouched for it. The
       owner is the person knowledge health takes the document's problems to;
       confirming it restarts the review clock and settles the problems that
       only asked for a look. A docs page's owner is changed on the page. -->
  <section v-if="stewardship" class="flex flex-col gap-2.5" data-testid="knowledge-stewardship">
    <div class="flex items-start gap-3 leading-[1.6]">
      <span class="text-muted-foreground flex-[0_0_72px] text-xs">{{ t("stewardship.owner") }}</span>
      <span class="text-foreground flex min-w-0 flex-wrap items-center gap-1.5 text-[13px]">
        <span v-if="stewardship.owner" data-testid="stewardship-owner">
          {{ stewardship.owner.username || stewardship.owner.id }}
        </span>
        <span v-else class="text-placeholder" data-testid="stewardship-owner">{{ t("stewardship.noOwner") }}</span>
        <Badge v-if="stewardship.owner && !stewardship.owner.active" variant="outline" class="text-warning">
          {{ t("stewardship.ownerLeft") }}
        </Badge>
        <MemberPicker
          v-if="canEdit && stewardship.owner_editable"
          :current-id="stewardship.owner?.id"
          allow-none
          :none-label="t('stewardship.noOwner')"
          @select="transfer"
        >
          <template #trigger>
            <Button variant="ghost" size="xs" class="h-5 px-1" :disabled="busy" data-testid="stewardship-transfer">
              {{ t("stewardship.transfer") }}
            </Button>
          </template>
        </MemberPicker>
        <span v-else-if="stewardship.origin === 'docs'" class="text-placeholder text-xs">
          {{ t("stewardship.changeOnPage") }}
        </span>
      </span>
    </div>

    <div class="flex items-start gap-3 leading-[1.6]">
      <span class="text-muted-foreground flex-[0_0_72px] text-xs">{{ t("stewardship.reviewed") }}</span>
      <span class="text-foreground flex min-w-0 flex-wrap items-center gap-1.5 text-[13px]">
        <span v-if="stewardship.reviewed_at" data-testid="stewardship-reviewed">
          {{
            t("stewardship.reviewedBy", {
              time: formatDate(stewardship.reviewed_at),
              name: stewardship.reviewed_by?.username || stewardship.reviewed_by?.id || "",
            })
          }}
        </span>
        <span v-else class="text-placeholder" data-testid="stewardship-reviewed">{{
          t("stewardship.neverReviewed")
        }}</span>
      </span>
    </div>

    <div class="flex items-start gap-3 leading-[1.6]">
      <span class="text-muted-foreground flex-[0_0_72px] text-xs">{{ t("stewardship.review") }}</span>
      <span class="text-foreground flex min-w-0 flex-wrap items-center gap-1.5 text-[13px]">
        <template v-if="stewardship.review_due_at">
          <span data-testid="stewardship-due">{{
            t("stewardship.dueAt", { time: formatDate(stewardship.review_due_at) })
          }}</span>
          <Badge v-if="stewardship.overdue" variant="outline" class="border-warning/40 bg-warning/10 text-warning">
            {{ t("stewardship.overdue") }}
          </Badge>
        </template>
        <span v-else class="text-placeholder" data-testid="stewardship-due">{{ t("stewardship.noReview") }}</span>
        <Button
          v-if="canEdit"
          variant="outline"
          size="xs"
          class="h-6"
          :disabled="busy"
          data-testid="stewardship-confirm"
          @click="confirm"
        >
          <BadgeCheckIcon />
          {{ t("stewardship.confirm") }}
        </Button>
      </span>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import { BadgeCheckIcon } from "@lucide/vue";

import {
  confirmKnowledgeReviewed,
  getStewardship,
  setKnowledgeOwner,
  type KnowledgeStewardship,
} from "@/api/stewardship";
import MemberPicker from "@/components/findings/MemberPicker.vue";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

const props = defineProps<{
  knowledgeId: string;
  /** The caller may edit the knowledge base: transfer, confirm. */
  canEdit?: boolean;
}>();

const { t } = useI18n();

const stewardship = ref<KnowledgeStewardship | null>(null);
const busy = ref(false);
let seq = 0;

async function load(id: string) {
  const mine = ++seq;
  try {
    const next = await getStewardship(id);
    if (mine === seq) stewardship.value = next;
  } catch (err) {
    // A side panel of the drawer: missing it is not worth an error.
    if (mine === seq) console.debug("stewardship unavailable", err);
  }
}

watch(
  () => props.knowledgeId,
  (id) => {
    seq++;
    stewardship.value = null;
    if (id) void load(id);
  },
  { immediate: true },
);

async function run(action: () => Promise<KnowledgeStewardship>, done: string, failed: string) {
  if (busy.value) return;
  busy.value = true;
  const id = props.knowledgeId;
  try {
    const next = await action();
    if (id === props.knowledgeId) stewardship.value = next;
    MessagePlugin.success(done);
  } catch (err) {
    const msg = (err as { message?: string } | null)?.message;
    MessagePlugin.error(msg ? `${failed}: ${msg}` : failed);
  } finally {
    busy.value = false;
  }
}

const transfer = (ownerId: string) =>
  run(
    () => setKnowledgeOwner(props.knowledgeId, ownerId),
    t("stewardship.transferred"),
    t("stewardship.transferFailed"),
  );

const confirm = () =>
  run(() => confirmKnowledgeReviewed(props.knowledgeId), t("stewardship.confirmed"), t("stewardship.confirmFailed"));

const formatDate = (iso: string | null | undefined) => {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleDateString();
};
</script>
