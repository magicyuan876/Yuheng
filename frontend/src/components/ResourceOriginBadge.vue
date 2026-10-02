<template>
  <span
    class="inline-flex max-w-[140px] items-center gap-[3px] rounded-[8px] px-1.5 py-px text-[11px] leading-[1.4] font-medium"
    :class="variantClass"
    :title="tooltipText"
  >
    <component :is="iconComponent" class="size-3 shrink-0" />
    <span class="truncate">{{ displayText }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed, type Component } from "vue";
import { useI18n } from "vue-i18n";

import { UserRoundIcon, UsersRoundIcon } from "@lucide/vue";

import { useAuthStore } from "@/stores/auth";

/**
 * ResourceOriginBadge – a unified, compact label that explains *who* a KB
 * comes from. Replaces the ad-hoc "我的" pills scattered across
 * KnowledgeBaseList. The variants below cover the origin shapes the list
 * views actually surface; future origins (e.g. "system" / "imported") should
 * add a new variant rather than re-using one of these.
 *
 * Variants:
 *  - mine        : created by the current user
 *  - tenant      : created by another member of the workspace — label shows
 *                  the workspace name; use when context doesn't say
 *  - creator     : same data shape as `tenant`, but the surrounding section
 *                  header already names the workspace ("本空间 · 其他成员"),
 *                  so the badge only carries the creator name instead of
 *                  repeating the workspace on every card. Falls back to the
 *                  i18n label when the creator name is unknown.
 *
 * Pass `creatorName` to surface "by 张三" in the tooltip for the `tenant`
 * variant, or to drive the visible label of the `creator` variant; omit it
 * for the `mine` variant where the subject is implicit.
 */
const props = withDefaults(
  defineProps<{
    variant: "mine" | "tenant" | "creator";
    /** Optional creator display name, surfaces in tooltip for `tenant` variant. */
    creatorName?: string;
  }>(),
  { creatorName: "" },
);

const { t } = useI18n();
const authStore = useAuthStore();

const ICONS: Record<typeof props.variant | "default", Component> = {
  mine: UserRoundIcon,
  tenant: UsersRoundIcon,
  creator: UserRoundIcon,
  default: UsersRoundIcon,
};

const iconComponent = computed(() => ICONS[props.variant] ?? ICONS.default);

/** Tint per variant; the closed set the old less block carried. */
const VARIANT_CLASS: Record<typeof props.variant, string> = {
  mine: "bg-[var(--td-success-color-light)] text-primary",
  tenant: "bg-secondary text-muted-foreground",
  creator: "bg-secondary text-muted-foreground",
};

const variantClass = computed(() => VARIANT_CLASS[props.variant]);

const displayText = computed(() => {
  switch (props.variant) {
    case "mine":
      return t("resourceOrigin.mine");
    case "tenant":
      // Prefer the tenant name when known so the badge says where the
      // resource lives, not a vague "tenant" label. Falls back to i18n.
      return authStore.currentTenantName || t("resourceOrigin.tenant");
    case "creator":
      // Section header already provides the「本空间」context, so we just
      // show who created it. Fall back to a generic label when the user
      // can't be resolved (creator_name 缺失，例如已删除账号 / 老数据)。
      return props.creatorName || t("resourceOrigin.tenant");
    default:
      return "";
  }
});

const tooltipText = computed(() => {
  switch (props.variant) {
    case "mine":
      return t("resourceOrigin.mineTooltip");
    case "tenant":
      if (props.creatorName) {
        return t("resourceOrigin.tenantTooltipWithCreator", { creator: props.creatorName });
      }
      return t("resourceOrigin.tenantTooltip");
    case "creator":
      // 卡片标签只露名字；tooltip 把完整含义补回来。
      if (props.creatorName) {
        return t("resourceOrigin.tenantTooltipWithCreator", { creator: props.creatorName });
      }
      return t("resourceOrigin.tenantTooltip");
    default:
      return "";
  }
});
</script>
