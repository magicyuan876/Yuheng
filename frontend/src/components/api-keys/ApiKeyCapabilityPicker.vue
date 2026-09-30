<template>
  <div class="flex flex-col gap-3">
    <div
      v-for="group in groups"
      :key="group.key"
      class="border-border flex flex-col gap-2 border-t pt-2.5 first:border-t-0 first:pt-0"
    >
      <div class="text-foreground flex min-h-6 items-center justify-between gap-3 text-[13px] font-semibold">
        <span>{{ t(capabilityGroupLabelKey(group.key)) }}</span>
        <Button variant="ghost" size="sm" :disabled="disabled" @click="toggleGroup(group.capabilities)">
          {{ groupSelected(group.capabilities) ? t("apiKeys.clearGroup") : t("apiKeys.selectGroup") }}
        </Button>
      </div>
      <div class="flex flex-col gap-2.5">
        <div v-for="capability in group.capabilities" :key="capability" class="flex flex-col">
          <div class="flex items-center gap-2">
            <Checkbox
              :id="`${idPrefix}-${capability}`"
              :model-value="selected.has(capability)"
              :disabled="disabled"
              @update:model-value="(checked) => setCapability(capability, checked === true)"
            />
            <Label :for="`${idPrefix}-${capability}`" class="cursor-pointer text-[13px] font-normal">
              {{ t(capabilityLabelKey(capability)) }}
            </Label>
          </div>
          <p class="text-placeholder m-0 mt-0.5 ml-6 text-xs leading-[18px]">{{ t(capabilityHintKey(capability)) }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import type { TenantAPIKeyCapability } from "@/api/tenant";
import {
  capabilityGroupLabelKey,
  capabilityHintKey,
  capabilityLabelKey,
  type ApiKeyCapabilityGroup,
} from "@/config/apiKeyCapabilities";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";

/**
 * Grouped checkbox list for choosing API-key capabilities.
 *
 * The model is the full capability list of the key. Only the capabilities of
 * the groups shown can be toggled; anything else in the model (say a docs_*
 * grant on a deployment that has since switched the docs module off) is left
 * exactly as it was, so editing a key never drops a grant the form did not
 * show.
 */
const props = withDefaults(
  defineProps<{
    groups: readonly ApiKeyCapabilityGroup[];
    /** Makes the checkbox ids unique when two pickers share a page. */
    idPrefix?: string;
    disabled?: boolean;
  }>(),
  { idPrefix: "api-key-capability", disabled: false },
);

const model = defineModel<TenantAPIKeyCapability[]>({ required: true });

const { t } = useI18n();

const selected = computed(() => new Set(model.value));

function groupSelected(capabilities: TenantAPIKeyCapability[]): boolean {
  return capabilities.every((capability) => selected.value.has(capability));
}

function setCapability(capability: TenantAPIKeyCapability, checked: boolean) {
  update(checked ? [capability] : [], checked ? [] : [capability]);
}

function toggleGroup(capabilities: TenantAPIKeyCapability[]) {
  if (groupSelected(capabilities)) update([], capabilities);
  else update(capabilities, []);
}

// Keeps the model in picker order (groups, then capabilities within a group)
// with anything the picker does not show appended untouched, so the request
// body does not depend on the order the boxes were clicked in.
function update(add: TenantAPIKeyCapability[], remove: TenantAPIKeyCapability[]) {
  const next = new Set(selected.value);
  add.forEach((capability) => next.add(capability));
  remove.forEach((capability) => next.delete(capability));
  const shown = props.groups.flatMap((group) => group.capabilities);
  const shownSet = new Set(shown);
  model.value = [
    ...shown.filter((capability) => next.has(capability)),
    ...[...next].filter((capability) => !shownSet.has(capability)),
  ];
}
</script>
