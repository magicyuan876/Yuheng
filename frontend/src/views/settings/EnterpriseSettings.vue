<script setup lang="ts">
import { onMounted } from "vue";
import { ExternalLinkIcon } from "@lucide/vue";
import EnterpriseFeatureCard from "@/components/enterprise/EnterpriseFeatureCard.vue";
import { useEnterpriseFeatureContext } from "@/components/enterprise/useEnterpriseFeature";
import { useDeploymentCapabilitiesStore } from "@/stores/deploymentCapabilities";
import { ENTERPRISE_FEATURES, ENTERPRISE_LICENSE_ROUTE } from "@/config/enterpriseFeatures";
import { getEnterpriseInfoUrl } from "@/config/enterprisePromotion";

// The one place where Enterprise features are described. It lives in Settings
// so that core workflows (chat, knowledge bases, docs) never carry teasers.
const { stateOf, reasonOf, isAdmin } = useEnterpriseFeatureContext();
const infoUrl = getEnterpriseInfoUrl();

// The states come from the capabilities probe; ensureLoaded is idempotent, so
// opening this page directly (deep link) still gets real states.
onMounted(() => {
  void useDeploymentCapabilitiesStore().ensureLoaded();
});
</script>

<template>
  <div>
    <h2 class="text-foreground m-0 mb-3 text-lg font-semibold">{{ $t("enterprise.title") }}</h2>
    <p class="text-muted-foreground m-0 mb-2 text-sm leading-relaxed">{{ $t("enterprise.intro") }}</p>
    <p class="text-muted-foreground m-0 mb-3 text-sm leading-relaxed">{{ $t("enterprise.introPlanned") }}</p>
    <a
      v-if="infoUrl"
      :href="infoUrl"
      target="_blank"
      rel="noopener noreferrer"
      data-testid="edition-link"
      class="text-primary focus-visible:ring-ring/50 mb-5 inline-flex items-center gap-1 rounded-sm text-sm hover:underline focus-visible:ring-3 focus-visible:outline-none"
    >
      {{ $t("enterprise.learnEdition") }}
      <ExternalLinkIcon class="size-3.5" aria-hidden="true" />
    </a>

    <ul class="m-0 mt-2 flex list-none flex-col gap-3 p-0" :aria-label="$t('enterprise.listLabel')">
      <li v-for="feature in ENTERPRISE_FEATURES" :key="feature.key">
        <EnterpriseFeatureCard
          :feature="feature"
          :state="stateOf(feature.key)"
          :reason="reasonOf(feature.key)"
          :is-admin="isAdmin"
          :license-route="ENTERPRISE_LICENSE_ROUTE"
          :info-url="infoUrl"
        />
      </li>
    </ul>
  </div>
</template>
