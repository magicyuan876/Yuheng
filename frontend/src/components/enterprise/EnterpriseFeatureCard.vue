<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { RouterLink } from "vue-router";
import { ExternalLinkIcon } from "@lucide/vue";
import { Button } from "@/components/ui/button";
import EnterpriseBadge from "./EnterpriseBadge.vue";
import type { EnterpriseFeature } from "@/config/enterpriseFeatures";
import type { ExtensionState } from "@/config/deploymentCapabilities";

// Purely presentational: the caller resolves the state, so the same card serves
// the settings page and the gate, and can be tested by props alone.
const props = defineProps<{
  feature: EnterpriseFeature;
  state: ExtensionState;
  /** Reason code the backend gave for a locked feature, e.g. "license_required". */
  reason?: string;
  /** Whether the viewer administers the tenant. Only admins are sent to the license page. */
  isAdmin?: boolean;
  /** Route of the license page; undefined while the frontend has none. */
  licenseRoute?: string;
  /** The configured info URL; empty means "show no link". */
  infoUrl?: string;
}>();

const { t, te } = useI18n();

// The backend's reason codes are snake_case; the i18n keys are camelCase.
const reasonText = computed(() => {
  const camel = (props.reason ?? "").replace(/_([a-z])/g, (_, c: string) => c.toUpperCase());
  const key = `enterprise.reason.${camel}`;
  return camel && te(key) ? t(key) : t("enterprise.reason.generic");
});

const stageLabel = computed(() => {
  if (props.state === "enabled") return t("enterprise.stage.enabled");
  return t(`enterprise.stage.${props.feature.stage}`);
});

// Exactly one call to action at most. A planned feature is never sold, so it
// gets at most a neutral "learn more" link, whatever the state.
type Cta = "learn-edition" | "learn-more" | "locked" | "none";
const cta = computed<Cta>(() => {
  if (props.state === "enabled") return "none";
  if (props.feature.stage === "planned") return props.infoUrl ? "learn-more" : "none";
  if (props.state === "locked") return "locked";
  return props.infoUrl ? "learn-edition" : "none";
});

const titleId = computed(() => `enterprise-feature-${props.feature.key.replace(/[^a-z0-9]+/gi, "-")}`);
</script>

<template>
  <article
    :aria-labelledby="titleId"
    :data-feature="feature.key"
    :data-state="state"
    :data-stage="feature.stage"
    class="border-border bg-card flex flex-col gap-3 rounded-xl border p-4 max-[480px]:p-3.5"
  >
    <div class="flex items-start gap-3">
      <span class="bg-secondary text-muted-foreground flex size-9 shrink-0 items-center justify-center rounded-lg">
        <component :is="feature.icon" class="size-5" aria-hidden="true" />
      </span>
      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
          <h3 :id="titleId" class="text-foreground m-0 text-sm font-semibold">{{ $t(feature.titleKey) }}</h3>
          <EnterpriseBadge />
          <span class="text-muted-foreground text-xs" data-testid="stage">{{ stageLabel }}</span>
        </div>
        <p class="text-muted-foreground m-0 mt-1 text-[13px] leading-normal">{{ $t(feature.descriptionKey) }}</p>
      </div>
    </div>

    <a
      v-if="cta === 'learn-edition' || cta === 'learn-more'"
      :href="infoUrl"
      target="_blank"
      rel="noopener noreferrer"
      :data-cta="cta"
      class="text-primary focus-visible:ring-ring/50 inline-flex items-center gap-1 self-start rounded-sm text-[13px] hover:underline focus-visible:ring-3 focus-visible:outline-none"
    >
      {{ cta === "learn-edition" ? $t("enterprise.learnEdition") : $t("enterprise.learnMore") }}
      <ExternalLinkIcon class="size-3.5" aria-hidden="true" />
    </a>

    <div v-else-if="cta === 'locked'" class="flex flex-col items-start gap-2" data-cta="locked">
      <p class="text-foreground m-0 text-[13px] leading-normal" data-testid="reason">{{ reasonText }}</p>
      <template v-if="isAdmin">
        <Button v-if="licenseRoute" as-child size="sm" variant="outline">
          <RouterLink :to="licenseRoute" data-testid="license-link">{{ $t("enterprise.openLicense") }}</RouterLink>
        </Button>
      </template>
      <p v-else class="text-muted-foreground m-0 text-[13px]" data-testid="ask-admin">
        {{ $t("enterprise.askAdmin") }}
      </p>
    </div>
  </article>
</template>
