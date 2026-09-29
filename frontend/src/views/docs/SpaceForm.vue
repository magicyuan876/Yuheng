<template>
  <!-- Label on top, 18px between fields: the old top-aligned t-form's rhythm. A field with a
       problem marks its control aria-invalid, which gives it the error border. -->
  <form class="flex flex-col gap-[18px]" @submit.prevent>
    <div class="flex flex-col gap-1.5">
      <Label for="space-form-name">{{ t("docs.spaces.form.name") }}</Label>
      <Input
        id="space-form-name"
        v-model="model.name"
        :maxlength="100"
        :placeholder="t('docs.spaces.form.namePlaceholder')"
        :aria-invalid="invalid('name')"
        @input="onNameInput"
      />
      <p v-if="tipOf('name')" class="text-destructive m-0 text-xs">{{ tipOf("name") }}</p>
    </div>

    <div class="flex flex-col gap-1.5">
      <Label for="space-form-slug">{{ t("docs.spaces.form.slug") }}</Label>
      <Input
        id="space-form-slug"
        v-model="model.slug"
        :maxlength="64"
        :placeholder="t('docs.spaces.form.slugPlaceholder')"
        :aria-invalid="invalid('slug')"
        @input="slugTouched = true"
      />
      <p class="m-0 text-xs" :class="tipOf('slug') ? 'text-destructive' : 'text-muted-foreground'">
        {{ tipOf("slug") || t("docs.spaces.form.slugHint") }}
      </p>
    </div>

    <div class="flex flex-col gap-1.5">
      <Label for="space-form-description">{{ t("docs.spaces.form.description") }}</Label>
      <Textarea
        id="space-form-description"
        v-model="model.description"
        class="[field-sizing:content] max-h-[130px] min-h-[52px]"
        :maxlength="4000"
        :placeholder="t('docs.spaces.form.descriptionPlaceholder')"
        :aria-invalid="invalid('description')"
      />
      <p v-if="tipOf('description')" class="text-destructive m-0 text-xs">{{ tipOf("description") }}</p>
    </div>

    <div class="flex flex-col gap-1.5">
      <Label>{{ t("docs.spaces.form.visibility") }}</Label>
      <div class="flex flex-col gap-1.5">
        <!-- A segmented control, standing in for the filled button-style t-radio-group. -->
        <div
          role="radiogroup"
          :aria-label="t('docs.spaces.form.visibility')"
          class="bg-secondary inline-flex w-fit gap-0.5 rounded-md p-0.5"
        >
          <button
            v-for="v in SPACE_VISIBILITIES"
            :key="v"
            type="button"
            role="radio"
            data-slot="segmented-item"
            :aria-checked="model.visibility === v"
            class="h-7 rounded-[5px] px-3 text-sm transition-colors"
            :class="
              model.visibility === v
                ? 'bg-card text-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground'
            "
            @click="selectVisibility(v)"
          >
            {{ t("docs.spaces.visibility." + v) }}
          </button>
        </div>
        <span class="text-muted-foreground text-xs leading-[18px]">
          {{ t("docs.spaces.visibilityHint." + model.visibility) }}
        </span>
      </div>
    </div>

    <div v-if="model.visibility === 'open'" class="flex flex-col gap-1.5">
      <Label>{{ t("docs.spaces.form.defaultRole") }}</Label>
      <Select v-model="model.default_role">
        <SelectTrigger class="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="o in defaultRoleOptions" :key="o.value" :value="o.value">{{ o.label }}</SelectItem>
        </SelectContent>
      </Select>
      <p class="text-muted-foreground m-0 text-xs">{{ t("docs.spaces.form.defaultRoleHint") }}</p>
    </div>
  </form>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";

import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";

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

// The old radio group fired its change only when the value really changed.
const selectVisibility = (v: SpaceFormModel["visibility"]) => {
  if (model.value.visibility === v) return;
  model.value.visibility = v;
  onVisibilityChange();
};

const defaultRoleOptions = computed(() =>
  OPEN_SPACE_DEFAULT_ROLES.map((r) => ({
    label: t("docs.spaces.role." + r),
    value: r,
  })),
);

const problemFor = (field: keyof SpaceFormModel) => props.problems.find((p) => p.field === field);
// true, or undefined so the attribute is left off altogether.
const invalid = (field: keyof SpaceFormModel) => (problemFor(field) ? true : undefined);
const tipOf = (field: keyof SpaceFormModel) => {
  const p = problemFor(field);
  return p ? t("docs.spaces.form." + p.key) : "";
};
</script>
