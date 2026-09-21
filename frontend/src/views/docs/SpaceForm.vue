<template>
  <t-form label-align="top" :data="model" class="docs-space-form" @submit.prevent>
    <t-form-item :label="t('docs.spaces.form.name')" name="name" :status="statusOf('name')" :tips="tipOf('name')">
      <t-input
        v-model="model.name"
        :maxlength="100"
        :placeholder="t('docs.spaces.form.namePlaceholder')"
        @input="onNameInput"
      />
    </t-form-item>
    <t-form-item
      :label="t('docs.spaces.form.slug')"
      name="slug"
      :status="statusOf('slug')"
      :tips="tipOf('slug') || t('docs.spaces.form.slugHint')"
    >
      <t-input
        v-model="model.slug"
        :maxlength="64"
        :placeholder="t('docs.spaces.form.slugPlaceholder')"
        @input="slugTouched = true"
      />
    </t-form-item>
    <t-form-item
      :label="t('docs.spaces.form.description')"
      name="description"
      :status="statusOf('description')"
      :tips="tipOf('description')"
    >
      <t-textarea
        v-model="model.description"
        :autosize="{ minRows: 2, maxRows: 5 }"
        :maxlength="4000"
        :placeholder="t('docs.spaces.form.descriptionPlaceholder')"
      />
    </t-form-item>
    <t-form-item :label="t('docs.spaces.form.visibility')" name="visibility">
      <div class="visibility-field">
        <t-radio-group v-model="model.visibility" variant="default-filled" @change="onVisibilityChange">
          <t-radio-button v-for="v in SPACE_VISIBILITIES" :key="v" :value="v">
            {{ t("docs.spaces.visibility." + v) }}
          </t-radio-button>
        </t-radio-group>
        <span class="field-hint">{{ t("docs.spaces.visibilityHint." + model.visibility) }}</span>
      </div>
    </t-form-item>
    <t-form-item
      v-if="model.visibility === 'open'"
      :label="t('docs.spaces.form.defaultRole')"
      name="default_role"
      :tips="t('docs.spaces.form.defaultRoleHint')"
    >
      <t-select v-model="model.default_role" :options="defaultRoleOptions" />
    </t-form-item>
  </t-form>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";

import {
  normaliseSpaceForm,
  OPEN_SPACE_DEFAULT_ROLES,
  SPACE_VISIBILITIES,
  suggestSlug,
  type SpaceFormModel,
  type SpaceFormProblem,
} from "./docsAccess";

const props = withDefaults(
  defineProps<{
    mode?: "create" | "edit";
    problems?: SpaceFormProblem[];
  }>(),
  { mode: "create", problems: () => [] },
);

const model = defineModel<SpaceFormModel>({ required: true });
const { t } = useI18n();

// In create mode the slug follows the name until the user edits it.
const slugTouched = ref(props.mode === "edit");

const onNameInput = () => {
  if (slugTouched.value) return;
  model.value.slug = suggestSlug(model.value.name);
};

const onVisibilityChange = () => {
  Object.assign(model.value, normaliseSpaceForm(model.value));
};

const defaultRoleOptions = computed(() =>
  OPEN_SPACE_DEFAULT_ROLES.map((r) => ({
    label: t("docs.spaces.role." + r),
    value: r,
  })),
);

const problemFor = (field: keyof SpaceFormModel) => props.problems.find((p) => p.field === field);
const statusOf = (field: keyof SpaceFormModel) => (problemFor(field) ? "error" : undefined);
const tipOf = (field: keyof SpaceFormModel) => {
  const p = problemFor(field);
  return p ? t("docs.spaces.form." + p.key) : "";
};
</script>

<style scoped lang="less">
.docs-space-form {
  :deep(.t-form__item) {
    margin-bottom: 18px;
  }
}

.visibility-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field-hint {
  font-size: 12px;
  line-height: 18px;
  color: var(--td-text-color-secondary);
}
</style>
