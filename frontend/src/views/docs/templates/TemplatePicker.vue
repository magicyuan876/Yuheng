<template>
  <div class="flex max-h-[60vh] flex-col gap-0.5 overflow-y-auto">
    <div v-if="loading && !templates.length" class="flex items-center justify-center gap-2 py-4">
      <Loader2Icon class="size-4 animate-spin" />
    </div>
    <template v-else>
      <!-- Starting blank is the common case and the one people reach for
           when a picker gets in the way, so it is the first choice rather
           than a way out of the dialog. -->
      <button
        type="button"
        data-slot="template-row"
        class="mb-1 flex w-full cursor-pointer items-start gap-2.5 rounded-[8px] border px-3 py-2.5 text-left text-inherit"
        :class="rowClass(chosen === '')"
        @click="choose('')"
      >
        <span class="flex-none text-[20px] leading-[1.3]">📄</span>
        <span class="flex min-w-0 flex-col gap-0.5">
          <span class="text-sm font-medium">{{ t("docs.templates.blank") }}</span>
          <span class="text-placeholder overflow-hidden text-xs text-ellipsis">{{
            t("docs.templates.blankHint")
          }}</span>
        </span>
      </button>

      <template v-for="group in grouped" :key="group.name">
        <h3 class="text-muted-foreground mx-0 mt-3.5 mb-1 text-xs font-semibold tracking-[0.04em] uppercase">
          {{ group.name || t("docs.templates.uncategorised") }}
        </h3>
        <button
          v-for="tpl in group.items"
          :key="tpl.id"
          type="button"
          data-slot="template-row"
          class="flex w-full cursor-pointer items-start gap-2.5 rounded-[8px] border px-3 py-2.5 text-left text-inherit"
          :class="rowClass(chosen === tpl.id)"
          @click="choose(tpl.id)"
        >
          <span class="flex-none text-[20px] leading-[1.3]">{{ tpl.icon || "🧩" }}</span>
          <span class="flex min-w-0 flex-col gap-0.5">
            <span class="flex items-center gap-1.5 text-sm font-medium">
              {{ tpl.name }}
              <Badge v-if="tpl.shared" variant="secondary">{{ t("docs.templates.shared") }}</Badge>
            </span>
            <span v-if="tpl.description" class="text-placeholder overflow-hidden text-xs text-ellipsis">
              {{ tpl.description }}
            </span>
          </span>
        </button>
      </template>

      <p v-if="!templates.length && !loading" class="text-placeholder m-0 mt-3 text-[13px]">
        {{ t("docs.templates.none") }}
      </p>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from "vue";
import { useI18n } from "vue-i18n";

import { Loader2Icon } from "@lucide/vue";

import { listTemplates, type TemplateView } from "@/api/docs";
import { Badge } from "@/components/ui/badge";

// Choosing what a new page starts from.
//
// Grouped by the category their authors gave them, because a flat list of
// thirty templates is a list nobody reads. Within the dialog the choice is
// only recorded; the page is created by whoever opened it, so this component
// never writes anything.

const props = defineProps<{ spaceId: string; modelValue: string }>();
const emit = defineEmits<{ "update:modelValue": [string] }>();

const { t } = useI18n();

const loading = ref(false);
const templates = shallowRef<TemplateView[]>([]);
const chosen = computed(() => props.modelValue);

interface Group {
  name: string;
  items: TemplateView[];
}

/** Grouped in the order the server returned, which already puts the shared
 * ones first within each category. */
const grouped = computed<Group[]>(() => {
  const groups: Group[] = [];
  for (const tpl of templates.value) {
    const name = tpl.category ?? "";
    const last = groups[groups.length - 1];
    if (last && last.name === name) {
      last.items.push(tpl);
      continue;
    }
    groups.push({ name, items: [tpl] });
  }
  return groups;
});

// The chosen row keeps its tint under the pointer, as the old `.on` rule
// (declared after `:hover`) did; only the others take the hover background.
function rowClass(on: boolean): string {
  return on ? "border-primary bg-[var(--td-brand-color-light)]" : "hover:bg-accent border-transparent";
}

function choose(id: string) {
  emit("update:modelValue", id);
}

async function load() {
  if (!props.spaceId) return;
  loading.value = true;
  const id = props.spaceId;
  try {
    const rows = await listTemplates(id);
    if (props.spaceId === id) templates.value = rows;
  } catch {
    // A template library that cannot be reached should not stop somebody
    // creating a page; they get the blank choice, which is what most of them
    // wanted anyway.
    if (props.spaceId === id) templates.value = [];
  } finally {
    if (props.spaceId === id) loading.value = false;
  }
}

watch(
  () => props.spaceId,
  () => {
    void load();
  },
  { immediate: true },
);
</script>
