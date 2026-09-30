<template>
  <!--
    The trigger nests Tooltip → PopoverTrigger → button, each as-child, so the
    one <button> carries both the hover hint and the click-to-open popover.
    TDesign's placement="bottom-right" is side="bottom" + align="end".
  -->
  <Popover v-if="kbInfo">
    <Tooltip>
      <TooltipTrigger as-child>
        <PopoverTrigger as-child>
          <!--
            The info button is intentionally lighter than the sibling settings
            button: text-only by default, with a soft circular hover state. Two
            identical filled circles next to each other read as a stamp, so the
            info trigger sits closer to a typical "auxiliary glyph next to a
            title" and yields visual hierarchy to the settings gear (the
            primary action). The warning variant adds a red dot in the corner
            (the ::after pseudo-element) when the bound vector store is down.
          -->
          <button
            type="button"
            data-slot="kb-info-button"
            class="hover:not-disabled:bg-secondary hover:not-disabled:text-foreground relative inline-flex size-[26px] cursor-pointer items-center justify-center rounded-full border-none bg-transparent p-0 transition-[background,color] duration-200 ease-in-out disabled:cursor-not-allowed disabled:opacity-40"
            :class="isVectorStoreDown ? warningButtonClass : 'text-placeholder'"
          >
            <InfoIcon class="size-4" />
          </button>
        </PopoverTrigger>
      </TooltipTrigger>
      <TooltipContent side="top">{{ t("knowledgeBase.infoCard.tooltip") }}</TooltipContent>
    </Tooltip>
    <PopoverContent side="bottom" align="end" class="w-auto gap-0 p-0">
      <!--
        Capped to ~70% of the viewport so the popup never overflows the screen
        on shorter laptops; the body scrolls and the header stays put.
      -->
      <div
        class="text-foreground flex max-h-[min(70vh,560px)] max-w-[360px] min-w-[280px] flex-col overflow-hidden px-4 py-3 text-xs"
      >
        <div class="border-border mb-2 flex-none border-b border-solid pb-2 text-[13px] font-semibold">
          {{ t("knowledgeBase.infoCard.title") }}
        </div>
        <!--
          Scrolling body so the popover never grows past max-height. Negative
          side margins line the scrollbar up with the card edge while keeping
          the row labels aligned with the header.
        -->
        <div class="-mx-4 flex flex-auto flex-col overflow-y-auto px-4">
          <section :class="sectionClass">
            <h4 :class="sectionTitleClass">
              {{ t("knowledgeBase.infoCard.basic") }}
            </h4>
            <div :class="rowClass">
              <span :class="labelClass">{{ t("knowledgeBase.infoCard.type") }}</span>
              <span :class="valueClass">
                {{
                  kbInfo.type === "faq" ? t("knowledgeEditor.basic.typeFAQ") : t("knowledgeEditor.basic.typeDocument")
                }}
              </span>
            </div>
            <div v-if="kbInfo.description" :class="rowClass">
              <span :class="labelClass">{{ t("knowledgeBase.description") }}</span>
              <span class="text-muted-foreground block flex-1 break-words whitespace-pre-wrap">{{
                kbInfo.description
              }}</span>
            </div>
            <div v-if="kbInfo.created_at" :class="rowClass">
              <span :class="labelClass">{{ t("knowledgeBase.infoCard.createdAt") }}</span>
              <span :class="valueClass">{{ formatStringDate(new Date(kbInfo.created_at)) }}</span>
            </div>
            <div v-if="hasDistinctUpdate" :class="rowClass">
              <span :class="labelClass">{{ t("knowledgeBase.accessInfo.lastUpdated") }}</span>
              <span :class="valueClass">{{ lastUpdatedLabel }}</span>
            </div>
            <div v-if="supportedFileTypesSorted.length" :class="rowClass">
              <span :class="labelClass">{{ t("knowledgeBase.infoCard.supportedFileTypes") }}</span>
              <span :class="valueClass">
                <span
                  v-for="ft in supportedFileTypesSorted"
                  :key="ft"
                  class="text-muted-foreground inline-flex items-center rounded-[3px] bg-[var(--td-bg-color-component,#f5f7fa)] px-1.5 py-px font-mono text-[11px] leading-[1.4]"
                  >.{{ ft }}</span
                >
              </span>
            </div>
          </section>
          <section :class="sectionClass">
            <h4 :class="sectionTitleClass">
              {{ t("knowledgeBase.infoCard.access") }}
            </h4>
            <div :class="rowClass">
              <span :class="labelClass">{{ t("knowledgeBase.accessInfo.myRole") }}</span>
              <span :class="valueClass">
                <span :class="[tagClass, roleTagClass[roleTagTheme]]">
                  {{ accessRoleLabel }}
                </span>
                <span class="text-placeholder text-[11px]">{{ accessPermissionSummary }}</span>
              </span>
            </div>
            <div v-if="currentSharedKb" :class="rowClass">
              <span :class="labelClass">{{ t("knowledgeBase.accessInfo.fromOrg") }}</span>
              <span :class="valueClass">
                「{{ currentSharedKb.org_name }}」 · {{ t("knowledgeBase.accessInfo.sharedAt") }}
                {{ formatStringDate(new Date(currentSharedKb.shared_at)) }}
              </span>
            </div>
            <div v-else-if="effectiveKBPermission" :class="rowClass">
              <span :class="labelClass">{{ t("knowledgeBase.infoCard.source") }}</span>
              <span :class="valueClass">{{ t("knowledgeList.detail.sourceTypeKbShare") }}</span>
            </div>
            <div v-if="(kbInfo.share_count ?? 0) > 0" :class="rowClass">
              <span :class="labelClass">{{ t("knowledgeBase.infoCard.sharedTo") }}</span>
              <span :class="valueClass">
                {{ t("knowledgeList.sharedToOrgs", { count: kbInfo.share_count }) }}
              </span>
            </div>
          </section>
          <section v-if="capabilities.length" :class="sectionClass">
            <h4 :class="sectionTitleClass">
              {{ t("knowledgeBase.infoCard.capabilities") }}
            </h4>
            <div :class="rowClass">
              <span :class="labelClass">{{ t("knowledgeBase.infoCard.enabled") }}</span>
              <span :class="valueClass">
                <span v-for="cap in capabilities" :key="cap.key" :class="[tagClass, capabilityTagClass[cap.theme]]">
                  {{ cap.label }}
                </span>
              </span>
            </div>
          </section>
          <section v-if="chunkingRows.length" :class="sectionClass">
            <h4 :class="sectionTitleClass">
              {{ t("knowledgeBase.infoCard.chunking") }}
            </h4>
            <div v-for="row in chunkingRows" :key="row.key" :class="rowClass">
              <span :class="labelClass">{{ row.label }}</span>
              <span :class="valueClass">{{ row.value }}</span>
            </div>
          </section>
          <section v-if="statRows.length" :class="sectionClass">
            <h4 :class="sectionTitleClass">
              {{ t("knowledgeBase.infoCard.stats") }}
            </h4>
            <div v-for="stat in statRows" :key="stat.key" :class="rowClass">
              <span :class="labelClass">{{ stat.label }}</span>
              <span :class="cn(valueClass, 'font-medium tabular-nums')">{{ stat.value }}</span>
            </div>
          </section>
          <section v-if="kbInfo.vector_store_source || kbInfo.storage_provider_config?.provider" :class="sectionClass">
            <h4 :class="sectionTitleClass">
              {{ t("knowledgeBase.infoCard.binding") }}
            </h4>
            <div v-if="kbInfo.vector_store_source" :class="rowClass">
              <span :class="labelClass">{{ t("knowledgeBase.infoCard.vectorStore") }}</span>
              <span :class="valueClass">
                <VectorStoreBadge
                  :source="kbInfo.vector_store_source"
                  :name="kbInfo.vector_store_name"
                  :engine-type="kbInfo.vector_store_engine_type"
                  :status="kbInfo.vector_store_status"
                />
              </span>
            </div>
            <div v-if="kbInfo.storage_provider_config?.provider" :class="rowClass">
              <span :class="labelClass">{{ t("knowledgeBase.infoCard.fileStorage") }}</span>
              <span :class="cn(valueClass, 'text-muted-foreground font-mono text-[11px]')">
                {{ kbInfo.storage_provider_config.provider }}
              </span>
            </div>
          </section>
        </div>
      </div>
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { chunkOverlapOrDefault } from "@/config/chunking";
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { InfoIcon } from "@lucide/vue";
import VectorStoreBadge from "@/components/VectorStoreBadge.vue";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { useOrganizationStore } from "@/stores/organization";
import { useAuthStore } from "@/stores/auth";
import { formatStringDate } from "@/utils";

/*
 * The card's repeated building blocks. Each appears on every row or section,
 * so the class lists live here once rather than being copied into every
 * element of the template.
 *
 * A section is separated from the next by a hairline; the first loses its
 * top padding and the last its rule and bottom padding, as before.
 */
const sectionClass =
  "flex flex-col gap-2.5 border-b border-solid border-border pt-3 pb-4 first:pt-0 last:border-b-0 last:pb-0";
// The section heading carries a short brand-coloured bar in front of it.
const sectionTitleClass =
  "m-0 mb-1 flex items-center gap-2 text-[13px] font-semibold text-foreground select-none before:h-[14px] before:w-[3px] before:shrink-0 before:rounded-[2px] before:bg-primary before:content-['']";
const rowClass = "flex items-start gap-2 p-0 leading-[1.6]";
const labelClass = "flex-[0_0_80px] text-muted-foreground";
const valueClass = "inline-flex flex-1 flex-wrap items-center gap-1.5 break-words text-foreground";
// TDesign's small tag: 20px tall, 4px side padding, 12px text, 3px radius.
const tagClass = "inline-flex h-5 items-center rounded-[3px] px-1 text-xs leading-5 whitespace-nowrap";
// The red status dot in the trigger's corner, ringed in the card colour so it
// reads as a badge on top of the glyph.
const warningButtonClass =
  "text-destructive after:absolute after:top-[2px] after:right-[2px] after:size-[7px] after:rounded-full after:border-[1.5px] after:border-solid after:border-card after:bg-destructive after:content-['']";

const props = defineProps<{
  // The KB detail / list object. Typed as any to avoid coupling to the
  // backend's exhaustive shape; the popover reads optional fields and
  // falls back gracefully when they are missing.
  kbInfo: any;
  // Optional pre-computed list of supported file extensions (e.g.
  // ["pdf", "docx", …]). The popover does not fetch parser engines on
  // its own — call sites that already know which formats are reachable
  // pass them in; FAQ KBs simply omit the prop.
  supportedFileTypes?: string[];
}>();

const { t } = useI18n();
const orgStore = useOrganizationStore();
const authStore = useAuthStore();

// "Owner" here mirrors the per-page guards: the original creator
// (creator_id) — not "in my tenant". creator_id is empty for a
// tenant-owned KB (created through an API key); those fall through to
// the role/share check.
const isOwner = computed<boolean>(() => {
  const kb = props.kbInfo;
  if (!kb) return false;
  const creatorId = kb.creator_id || "";
  const userId = authStore.user?.id || "";
  if (!creatorId) return false;
  return creatorId === userId;
});

const currentSharedKb = computed(() => {
  const id = props.kbInfo?.id;
  if (!id) return null;
  return orgStore.sharedKnowledgeBases?.find?.((s: any) => s.knowledge_base?.id === id) ?? null;
});

const isViaShare = computed<boolean>(() => !!currentSharedKb.value);

const effectiveKBPermission = computed<string>(() => {
  const id = props.kbInfo?.id || "";
  return orgStore.getKBPermission?.(id) || props.kbInfo?.my_permission || "";
});

const accessRoleLabel = computed<string>(() => {
  if (!isViaShare.value && isOwner.value) return t("knowledgeBase.accessInfo.roleOwner");
  const perm = effectiveKBPermission.value;
  if (perm) return t(`organization.role.${perm}`);
  return "--";
});

const accessPermissionSummary = computed<string>(() => {
  if (!isViaShare.value && isOwner.value) return t("knowledgeBase.accessInfo.permissionOwner");
  const perm = effectiveKBPermission.value;
  if (perm === "admin") return t("knowledgeBase.accessInfo.permissionAdmin");
  if (perm === "editor") return t("knowledgeBase.accessInfo.permissionEditor");
  if (perm === "viewer") return t("knowledgeBase.accessInfo.permissionViewer");
  return "--";
});

type RoleTheme = "success" | "primary" | "warning" | "default";
const roleTagTheme = computed<RoleTheme>(() => {
  if (!isViaShare.value && isOwner.value) return "success";
  const perm = effectiveKBPermission.value;
  if (perm === "admin") return "primary";
  if (perm === "editor") return "warning";
  return "default";
});

// The solid (dark) tag variant the role badge used, one class list per theme.
const roleTagClass: Record<RoleTheme, string> = {
  success: "bg-success text-primary-foreground",
  primary: "bg-primary text-primary-foreground",
  warning: "bg-warning text-primary-foreground",
  default: "bg-[var(--td-bg-color-component)] text-foreground",
};

const isVectorStoreDown = computed<boolean>(() => props.kbInfo?.vector_store_status === "unavailable");

// KB.UpdatedAt is auto-bumped by GORM only when the KB row itself is
// touched (rename, config edit, …). For a freshly created KB whose
// only mutations were document uploads, updated_at == created_at and
// surfacing "last updated" alongside "created at" reads as
// duplicated noise. Hide the row in that case and let it reappear
// once the KB metadata is actually edited.
const hasDistinctUpdate = computed<boolean>(() => {
  const created = props.kbInfo?.created_at;
  const updated = props.kbInfo?.updated_at;
  if (!updated) return false;
  if (!created) return true;
  return new Date(updated).getTime() !== new Date(created).getTime();
});

const lastUpdatedLabel = computed<string>(() => {
  const raw = props.kbInfo?.updated_at;
  return raw ? formatStringDate(new Date(raw)) : "";
});

const supportedFileTypesSorted = computed<string[]>(() => {
  if (!props.supportedFileTypes?.length) return [];
  return [...props.supportedFileTypes].sort();
});

type CapabilityTheme = "primary" | "success" | "warning" | "default";

// The light (tinted) tag variant the capability list used, one class list per
// theme. The theme names are kept so the computed below reads as before.
const capabilityTagClass: Record<CapabilityTheme, string> = {
  primary: "bg-[var(--td-brand-color-light)] text-primary",
  success: "bg-[var(--td-success-color-light)] text-success",
  warning: "bg-[var(--td-warning-color-light)] text-warning",
  default: "bg-[var(--td-bg-color-component)] text-foreground",
};
const capabilities = computed<Array<{ key: string; label: string; theme: CapabilityTheme }>>(() => {
  const kb: any = props.kbInfo;
  if (!kb) return [];
  const items: Array<{ key: string; label: string; theme: CapabilityTheme }> = [];
  if (kb.vlm_config?.enabled) {
    items.push({ key: "vlm", label: "VLM", theme: "primary" });
  }
  if (kb.asr_config?.enabled) {
    items.push({ key: "asr", label: "ASR", theme: "primary" });
  }
  if (kb.extract_config?.enabled) {
    items.push({
      key: "kg",
      label: t("knowledgeList.features.knowledgeGraph"),
      theme: "success",
    });
  }
  if (kb.indexing_strategy?.wiki_enabled) {
    items.push({ key: "wiki", label: "Wiki", theme: "warning" });
  }
  return items;
});

const chunkingStrategyLabel = computed<string>(() => {
  const raw: string = (props.kbInfo?.chunking_config?.strategy || "").toLowerCase();
  const key = raw === "" || raw === "recursive" ? "legacy" : raw;
  const path = `knowledgeEditor.chunking.strategies.${key}.label`;
  const translated = t(path);
  return translated === path ? raw : translated;
});

const chunkingRows = computed<Array<{ key: string; label: string; value: string }>>(() => {
  const kb: any = props.kbInfo;
  if (!kb || kb.type === "faq") return [];
  const cfg = kb.chunking_config;
  if (!cfg) return [];
  const rows: Array<{ key: string; label: string; value: string }> = [];
  if (chunkingStrategyLabel.value) {
    rows.push({
      key: "strategy",
      label: t("knowledgeEditor.chunking.strategyLabel"),
      value: chunkingStrategyLabel.value,
    });
  }
  const chars = t("knowledgeEditor.chunking.characters");
  if (typeof cfg.chunk_size === "number" && cfg.chunk_size > 0) {
    rows.push({
      key: "size",
      label: t("knowledgeEditor.chunking.sizeLabel"),
      value: `${cfg.chunk_size} ${chars}`,
    });
  }
  // Always shown: an overlap that was never chosen is the backend default,
  // not "none", and a deliberate 0 is shown as 0.
  rows.push({
    key: "overlap",
    label: t("knowledgeEditor.chunking.overlapLabel"),
    value: `${chunkOverlapOrDefault(cfg.chunk_overlap)} ${chars}`,
  });
  if (cfg.enable_parent_child) {
    const parent = cfg.parent_chunk_size || 4096;
    const child = cfg.child_chunk_size || 384;
    rows.push({
      key: "parent-child",
      label: t("knowledgeEditor.chunking.parentChildLabel"),
      value: `${t("knowledgeBase.infoCard.parentShort")} ${parent} / ${t("knowledgeBase.infoCard.childShort")} ${child}`,
    });
  }
  if (typeof cfg.token_limit === "number" && cfg.token_limit > 0) {
    rows.push({
      key: "token-limit",
      label: t("knowledgeEditor.chunking.tokenLimitLabel"),
      value: String(cfg.token_limit),
    });
  }
  return rows;
});

const statRows = computed<Array<{ key: string; label: string; value: number | string }>>(() => {
  const kb: any = props.kbInfo;
  if (!kb) return [];
  const items: Array<{ key: string; label: string; value: number | string }> = [];
  // FAQ KBs store every Q/A pair as a chunk, so chunk_count is the
  // user-facing entry total. Document KBs use knowledge_count for the
  // file-level total (chunk_count there counts internal splits and is
  // not meaningful to surface here). Mirrors the same branching used
  // by the list card.
  if (kb.type === "faq") {
    if (typeof kb.chunk_count === "number") {
      items.push({
        key: "faq",
        label: t("knowledgeBase.infoCard.faqCount"),
        value: kb.chunk_count,
      });
    }
  } else if (typeof kb.knowledge_count === "number") {
    items.push({
      key: "knowledge",
      label: t("knowledgeBase.infoCard.documentCount"),
      value: kb.knowledge_count,
    });
  }
  return items;
});
</script>
