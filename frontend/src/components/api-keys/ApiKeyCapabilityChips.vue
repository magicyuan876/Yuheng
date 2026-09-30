<template>
  <div class="flex flex-wrap items-center gap-[5px]">
    <span v-for="capability in visible" :key="capability" :class="chipClass">
      {{ t(capabilityLabelKey(capability)) }}
    </span>
    <Popover v-if="hiddenCount > 0">
      <PopoverTrigger as-child>
        <button
          type="button"
          data-slot="capability-more"
          class="border-success/35 text-success inline-flex h-[22px] cursor-pointer items-center rounded-md border border-dashed bg-transparent px-2 text-xs font-medium"
          :aria-label="t('apiKeys.viewAllCapabilities')"
        >
          {{ t("apiKeys.capabilityMore", { count: hiddenCount }) }}
        </button>
      </PopoverTrigger>
      <PopoverContent align="start" class="max-h-[360px] w-[320px] max-w-[min(360px,88vw)] overflow-auto px-3.5 py-3">
        <div
          v-for="group in grouped"
          :key="group.key"
          class="border-border mt-2.5 border-t border-dashed pt-2.5 first:mt-0 first:border-t-0 first:pt-0"
        >
          <div class="text-foreground mb-1.5 text-xs leading-[1.4] font-semibold">
            {{ t(capabilityGroupLabelKey(group.key)) }}
          </div>
          <div class="flex flex-wrap gap-[5px]">
            <span v-for="capability in group.capabilities" :key="capability" :class="chipClass">
              {{ t(capabilityLabelKey(capability)) }}
            </span>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import type { TenantAPIKeyCapability } from "@/api/tenant";
import { capabilityGroupLabelKey, capabilityLabelKey, type ApiKeyCapabilityGroup } from "@/config/apiKeyCapabilities";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

/**
 * A key's granted capabilities as chips: the first few inline, the rest
 * behind a "+N" popover that lists everything by group. Capabilities the
 * groups do not know (a newer backend) are not shown rather than shown raw.
 */
const props = withDefaults(
  defineProps<{
    capabilities: readonly TenantAPIKeyCapability[] | null | undefined;
    groups: readonly ApiKeyCapabilityGroup[];
    maxVisible?: number;
  }>(),
  { maxVisible: 4 },
);

const { t } = useI18n();

const chipClass =
  "bg-success/10 text-success inline-flex h-[22px] items-center rounded-md px-2 text-xs font-medium whitespace-nowrap";

const grouped = computed(() => {
  const granted = new Set(props.capabilities ?? []);
  return props.groups
    .map((group) => ({ key: group.key, capabilities: group.capabilities.filter((c) => granted.has(c)) }))
    .filter((group) => group.capabilities.length > 0);
});

const ordered = computed(() => grouped.value.flatMap((group) => group.capabilities));
const visible = computed(() => ordered.value.slice(0, props.maxVisible));
const hiddenCount = computed(() => Math.max(0, ordered.value.length - props.maxVisible));
</script>
