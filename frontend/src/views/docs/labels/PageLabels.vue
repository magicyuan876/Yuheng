<template>
  <div class="page-labels">
    <span v-for="l in model" :key="l.id" class="label-chip" :class="{ removable: canEdit }">
      <span class="label-dot" :class="`dot-${l.color}`" />
      <span class="label-name">{{ l.name }}</span>
      <button
        v-if="canEdit"
        type="button"
        class="label-remove"
        :aria-label="t('docs.labels.remove', { name: l.name })"
        @click="remove(l.id)"
      >
        <t-icon name="close" size="12px" />
      </button>
    </span>

    <t-popup v-if="canEdit" trigger="click" placement="bottom-left" :visible="open" @visible-change="onOpenChange">
      <button type="button" class="label-add">
        <t-icon name="add" size="12px" />
        <span>{{ model.length ? t("docs.labels.add") : t("docs.labels.addFirst") }}</span>
      </button>
      <template #content>
        <div class="label-picker">
          <t-input
            v-model="query"
            :maxlength="MAX_NAME"
            :placeholder="t('docs.labels.search')"
            autofocus
            @enter="createFromQuery"
          />
          <div class="picker-list">
            <button
              v-for="l in matches"
              :key="l.id"
              type="button"
              class="picker-row"
              :class="{ on: chosen.has(l.id) }"
              @click="toggle(l)"
            >
              <span class="label-dot" :class="`dot-${l.color}`" />
              <span class="picker-name">{{ l.name }}</span>
              <t-icon v-if="chosen.has(l.id)" name="check" size="14px" />
            </button>
            <p v-if="!matches.length && !canCreate" class="picker-empty">{{ t("docs.labels.none") }}</p>
          </div>
          <!-- Making one is the same gesture as picking one: a writer files
               their own work without asking an admin for the vocabulary. -->
          <button v-if="canCreate" type="button" class="picker-create" :disabled="creating" @click="createFromQuery">
            <t-icon name="add" size="14px" />
            <span>{{ t("docs.labels.create", { name: trimmedQuery }) }}</span>
          </button>
        </div>
      </template>
    </t-popup>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";

import { createLabel, listLabels, setPageLabels, type LabelView } from "@/api/docs";

// The labels on one page: chips beside the title, and a picker that doubles
// as the place new labels are made.
//
// The server is told the whole set rather than one addition at a time, which
// is what "these are its labels now" means and what makes two people editing
// the same page converge on a set rather than on a race.

const MAX_NAME = 32;
const MAX_PER_PAGE = 20;

const props = defineProps<{ pageId: string; spaceId: string; canEdit: boolean; labels: LabelView[] }>();
const emit = defineEmits<{ change: [LabelView[]] }>();

const { t } = useI18n();

const model = ref<LabelView[]>([...props.labels]);
watch(
  () => props.labels,
  (next) => {
    model.value = [...next];
  },
);

const open = ref(false);
const query = ref("");
const creating = ref(false);
const available = shallowRef<LabelView[]>([]);

const chosen = computed(() => new Set(model.value.map((l) => l.id)));
const trimmedQuery = computed(() => query.value.trim().replace(/\s+/g, " "));

const matches = computed(() => {
  const q = trimmedQuery.value.toLowerCase();
  if (!q) return available.value;
  return available.value.filter((l) => l.name.toLowerCase().includes(q));
});

// Only when the typed name is not already a label: offering to create a
// duplicate of the row directly above is an error waiting to be clicked.
const canCreate = computed(() => {
  const q = trimmedQuery.value;
  return q.length > 0 && q.length <= MAX_NAME && !available.value.some((l) => l.name.toLowerCase() === q.toLowerCase());
});

async function onOpenChange(visible: boolean) {
  open.value = visible;
  if (!visible) {
    query.value = "";
    return;
  }
  try {
    available.value = await listLabels(props.spaceId);
  } catch {
    available.value = [];
  }
}

async function commit(next: LabelView[]) {
  const before = model.value;
  model.value = next;
  try {
    const saved = await setPageLabels(
      props.pageId,
      next.map((l) => l.id),
    );
    model.value = saved;
    emit("change", saved);
  } catch (err: unknown) {
    // Put the chips back: leaving the optimistic set on screen would tell
    // somebody their page is filed when it is not.
    model.value = before;
    const msg = (err as { message?: string } | null)?.message;
    MessagePlugin.error(msg ? `${t("docs.labels.saveFailed")}: ${msg}` : t("docs.labels.saveFailed"));
  }
}

function toggle(label: LabelView) {
  if (chosen.value.has(label.id)) {
    void commit(model.value.filter((l) => l.id !== label.id));
    return;
  }
  if (model.value.length >= MAX_PER_PAGE) {
    MessagePlugin.warning(t("docs.labels.tooMany", { n: MAX_PER_PAGE }));
    return;
  }
  void commit([...model.value, label]);
}

function remove(id: string) {
  void commit(model.value.filter((l) => l.id !== id));
}

async function createFromQuery() {
  if (!canCreate.value || creating.value) return;
  creating.value = true;
  try {
    const made = await createLabel(props.spaceId, { name: trimmedQuery.value });
    available.value = [...available.value, made];
    query.value = "";
    toggle(made);
  } catch (err: unknown) {
    const msg = (err as { message?: string } | null)?.message;
    MessagePlugin.error(msg ? `${t("docs.labels.createFailed")}: ${msg}` : t("docs.labels.createFailed"));
  } finally {
    creating.value = false;
  }
}
</script>

<style scoped>
.page-labels {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}

.label-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 2px 8px;
  border: 1px solid var(--td-component-border);
  border-radius: 999px;
  color: var(--td-text-color-primary);
  font-size: 12px;
  line-height: 18px;
}

.label-dot {
  flex: none;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--label-hue, var(--td-text-color-placeholder));
}

/* The closed set from LabelColors, themed in one place. */
.dot-gray {
  --label-hue: #8b8f96;
}
.dot-red {
  --label-hue: #e34d59;
}
.dot-orange {
  --label-hue: #ed7b2f;
}
.dot-yellow {
  --label-hue: #ebb105;
}
.dot-green {
  --label-hue: #2ba471;
}
.dot-teal {
  --label-hue: #0594fa;
}
.dot-blue {
  --label-hue: #366ef4;
}
.dot-purple {
  --label-hue: #834ec2;
}
.dot-pink {
  --label-hue: #ed49b4;
}

.label-remove {
  display: inline-flex;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--td-text-color-placeholder);
  cursor: pointer;
}

.label-remove:hover {
  color: var(--td-error-color);
}

.label-add {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border: 1px dashed var(--td-component-border);
  border-radius: 999px;
  background: transparent;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
  line-height: 18px;
  cursor: pointer;
}

.label-add:hover {
  color: var(--td-brand-color);
  border-color: var(--td-brand-color);
}

.label-picker {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 240px;
}

.picker-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-height: 220px;
  overflow-y: auto;
}

.picker-row,
.picker-create {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 5px 8px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: inherit;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.picker-row:hover,
.picker-create:hover {
  background: var(--td-bg-color-container-hover);
}

.picker-name {
  flex: 1;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.picker-create {
  border-top: 1px solid var(--td-component-stroke);
  border-radius: 0;
  color: var(--td-brand-color);
}

.picker-empty {
  margin: 0;
  padding: 6px 8px;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}
</style>
