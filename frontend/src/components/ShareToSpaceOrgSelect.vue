<template>
  <Select :model-value="modelValue" @update:model-value="$emit('update:modelValue', String($event ?? ''))">
    <SelectTrigger class="w-full">
      <!-- The trigger shows the organisation's name only, as the old select's label did; the
           option row's avatar and role belong to the list. -->
      <SelectValue :placeholder="placeholder || $t('organization.share.selectOrgPlaceholder')">
        <template v-if="selectedName">{{ selectedName }}</template>
      </SelectValue>
    </SelectTrigger>
    <SelectContent>
      <div v-if="loading" class="flex items-center justify-center gap-2 py-3">
        <Loader2Icon class="size-3.5 animate-spin" />
      </div>
      <template v-else>
        <SelectItem v-for="org in organizations" :key="org.id" :value="org.id">
          <div class="flex items-center gap-2">
            <SpaceAvatar :name="org.name" :avatar="org.avatar" size="small" />
            <span class="truncate">{{ org.name }}</span>
            <span v-if="roleLabel(org)" class="text-placeholder text-xs">{{ roleLabel(org) }}</span>
          </div>
        </SelectItem>
      </template>
    </SelectContent>
  </Select>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";

import { Loader2Icon } from "@lucide/vue";

import type { Organization } from "@/api/organization";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import SpaceAvatar from "@/components/SpaceAvatar.vue";

const props = defineProps<{
  modelValue: string;
  organizations: Organization[];
  loading?: boolean;
  placeholder?: string;
}>();

defineEmits<{
  "update:modelValue": [value: string];
}>();

const { t } = useI18n();

const selectedName = computed(() => props.organizations.find((org) => org.id === props.modelValue)?.name ?? "");

function roleLabel(org: Organization) {
  if (org.is_owner) return t("organization.owner");
  if (org.my_role) return t(`organization.role.${org.my_role}`);
  return "";
}
</script>
