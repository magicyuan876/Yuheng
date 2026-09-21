<template>
  <div class="docs-home">
    <div v-if="labels.length" class="home-labels">
      <button
        v-for="l in labels"
        :key="l.id"
        type="button"
        class="label-chip"
        :class="{ on: selected.has(l.id) }"
        @click="toggleLabel(l.id)"
      >
        <span class="label-dot" :class="`dot-${l.color}`" />
        <span class="label-name">{{ l.name }}</span>
        <span class="label-count">{{ l.page_count }}</span>
      </button>
      <t-button v-if="selected.size" variant="text" size="small" @click="clearLabels">
        {{ t("docs.home.clearFilter") }}
      </t-button>
    </div>

    <!-- Filtering replaces the lists: somebody who has narrowed to a label is
         asking one question, and answering it beside three other lists buries
         the answer. -->
    <section v-if="selected.size" class="home-section">
      <h2>{{ t("docs.home.filtered", { count: filtered.length }) }}</h2>
      <t-loading :loading="filtering" size="small">
        <ul v-if="filtered.length" class="home-list">
          <li v-for="n in filtered" :key="n.id">
            <button type="button" class="home-row" @click="emit('open', n)">
              <span class="row-icon">{{ n.icon || "📄" }}</span>
              <span class="row-title">{{ n.title || t("docs.tree.untitled") }}</span>
            </button>
          </li>
        </ul>
        <p v-else-if="!filtering" class="home-empty">{{ t("docs.home.noneWithLabels") }}</p>
      </t-loading>
    </section>

    <template v-else>
      <section v-if="favourites.length" class="home-section">
        <h2>{{ t("docs.home.favourites") }}</h2>
        <ul class="home-list">
          <li v-for="n in favourites" :key="n.id">
            <button type="button" class="home-row" @click="emit('open', n)">
              <span class="row-icon">{{ n.icon || "📄" }}</span>
              <span class="row-title">{{ n.title || t("docs.tree.untitled") }}</span>
            </button>
          </li>
        </ul>
      </section>

      <section v-if="visited.length" class="home-section">
        <h2>{{ t("docs.home.recentlyViewed") }}</h2>
        <!-- Said plainly rather than left to be discovered: this list is on
             this device only, because it is never written to the server. -->
        <p class="home-note">{{ t("docs.home.recentlyViewedNote") }}</p>
        <ul class="home-list">
          <li v-for="v in visited" :key="v.pageId">
            <button type="button" class="home-row" @click="emit('openVisit', v)">
              <span class="row-icon">{{ v.icon || "📄" }}</span>
              <span class="row-title">{{ v.title || t("docs.tree.untitled") }}</span>
            </button>
          </li>
        </ul>
      </section>

      <section class="home-section">
        <h2>{{ t("docs.home.recentlyEdited") }}</h2>
        <t-loading :loading="loading" size="small">
          <ul v-if="recent.length" class="home-list">
            <li v-for="n in recent" :key="n.id">
              <button type="button" class="home-row" @click="emit('open', n)">
                <span class="row-icon">{{ n.icon || "📄" }}</span>
                <span class="row-title">{{ n.title || t("docs.tree.untitled") }}</span>
                <span class="row-when">{{ when(n.content_updated_at || n.updated_at) }}</span>
              </button>
            </li>
          </ul>
          <p v-else-if="!loading" class="home-empty">{{ t("docs.home.nothingYet") }}</p>
        </t-loading>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";

import { getSpaceHome, pagesWithLabels, type LabelView, type TreeNode } from "@/api/docs";

import { browserStore, readVisits, type Visit } from "./recentlyViewed";

// A space's landing page: what this person starred, where they were, what the
// space has been working on, and the vocabulary to narrow any of it.
//
// Three of the four lists come from one request — a landing page that paints
// in four stages reads as a page that is broken. The fourth, where they were,
// never leaves the browser; see recentlyViewed.ts.

const props = defineProps<{ spaceId: string; spaceSlug: string }>();
const emit = defineEmits<{ open: [TreeNode]; openVisit: [Visit] }>();

const { t, locale } = useI18n();

const loading = ref(false);
const recent = shallowRef<TreeNode[]>([]);
const favourites = shallowRef<TreeNode[]>([]);
const labels = shallowRef<LabelView[]>([]);

const selected = ref(new Set<string>());
const filtering = ref(false);
const filtered = shallowRef<TreeNode[]>([]);

const store = browserStore();
const visits = shallowRef<Visit[]>([]);
// Only this space's: a landing page listing pages of somewhere else is noise.
const visited = computed(() => visits.value.filter((v) => v.spaceSlug === props.spaceSlug).slice(0, 6));

async function load() {
  if (!props.spaceId) return;
  loading.value = true;
  const id = props.spaceId;
  try {
    const home = await getSpaceHome(id);
    if (props.spaceId !== id) return;
    recent.value = home.recently_edited;
    favourites.value = home.favourites;
    labels.value = home.labels;
  } catch (err: unknown) {
    const msg = (err as { message?: string } | null)?.message;
    MessagePlugin.error(msg ? `${t("docs.home.loadFailed")}: ${msg}` : t("docs.home.loadFailed"));
  } finally {
    if (props.spaceId === id) loading.value = false;
  }
}

async function refilter() {
  const ids = [...selected.value];
  if (!ids.length) {
    filtered.value = [];
    return;
  }
  filtering.value = true;
  try {
    filtered.value = await pagesWithLabels(props.spaceId, ids);
  } catch {
    filtered.value = [];
  } finally {
    filtering.value = false;
  }
}

function toggleLabel(id: string) {
  const next = new Set(selected.value);
  if (!next.delete(id)) next.add(id);
  selected.value = next;
}

function clearLabels() {
  selected.value = new Set();
}

/** A date somebody can read at a glance rather than a timestamp. */
function when(iso?: string): string {
  if (!iso) return "";
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) return "";
  const minutes = Math.round((Date.now() - at.getTime()) / 60000);
  if (minutes < 1) return t("docs.home.justNow");
  if (minutes < 60) return t("docs.home.minutesAgo", { n: minutes });
  if (minutes < 60 * 24) return t("docs.home.hoursAgo", { n: Math.round(minutes / 60) });
  if (minutes < 60 * 24 * 7) return t("docs.home.daysAgo", { n: Math.round(minutes / (60 * 24)) });
  return at.toLocaleDateString(locale.value);
}

watch(
  () => props.spaceId,
  () => {
    clearLabels();
    visits.value = readVisits(store);
    void load();
  },
  { immediate: true },
);

watch(selected, () => {
  void refilter();
});

defineExpose({ reload: load });
</script>

<style scoped>
.docs-home {
  display: flex;
  flex-direction: column;
  gap: 28px;
  width: 100%;
  max-width: 720px;
  text-align: left;
}

.home-labels {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.label-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
  border: 1px solid var(--td-component-border);
  border-radius: 999px;
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
  font-size: 13px;
  cursor: pointer;
}

.label-chip.on {
  border-color: var(--td-brand-color);
  background: var(--td-brand-color-light);
}

.label-dot {
  flex: none;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--label-hue, var(--td-text-color-placeholder));
}

/* A closed set, themed in one place — see LabelColors in the service. */
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

.label-count {
  color: var(--td-text-color-placeholder);
  font-variant-numeric: tabular-nums;
}

.home-section h2 {
  margin: 0 0 10px;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.home-note {
  margin: -6px 0 10px;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}

.home-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.home-row {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 7px 10px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: inherit;
  font-size: 14px;
  text-align: left;
  cursor: pointer;
}

.home-row:hover {
  background: var(--td-bg-color-container-hover);
}

.row-title {
  flex: 1;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.row-when {
  flex: none;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}

.home-empty {
  margin: 0;
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}
</style>
