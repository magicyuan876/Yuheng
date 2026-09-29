<script setup lang="ts">
import { computed } from "vue";
import EnterpriseFeatureCard from "./EnterpriseFeatureCard.vue";
import { useEnterpriseFeatureContext } from "./useEnterpriseFeature";
import { ENTERPRISE_LICENSE_ROUTE, findEnterpriseFeature } from "@/config/enterpriseFeatures";
import { getEnterpriseInfoUrl, isEnterprisePromotionHidden } from "@/config/enterprisePromotion";

// Wraps the real UI of an Enterprise feature. Enabled: the slot. Otherwise the
// teaser card, unless the operator switched promotion off, in which case
// nothing at all. The enabled path never consults the promotion switch.
const props = defineProps<{ feature: string }>();

const { stateOf, reasonOf, isAdmin } = useEnterpriseFeatureContext();

const state = computed(() => stateOf(props.feature));
const entry = computed(() => findEnterpriseFeature(props.feature));
const showTeaser = computed(() => state.value !== "enabled" && !!entry.value && !isEnterprisePromotionHidden());
</script>

<template>
  <slot v-if="state === 'enabled'" />
  <EnterpriseFeatureCard
    v-else-if="showTeaser && entry"
    :feature="entry"
    :state="state"
    :reason="reasonOf(feature)"
    :is-admin="isAdmin"
    :license-route="ENTERPRISE_LICENSE_ROUTE"
    :info-url="getEnterpriseInfoUrl()"
  />
</template>
