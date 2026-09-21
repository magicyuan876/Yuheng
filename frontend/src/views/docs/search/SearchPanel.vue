<template>
  <div class="search-panel">
    <t-input
      v-model="query"
      :placeholder="t('docs.search.placeholder')"
      clearable
      autofocus
      size="large"
      @input="onType"
    >
      <template #prefix-icon><t-icon name="search" /></template>
    </t-input>

    <div v-if="spaceId" class="scope-row">
      <t-checkbox v-model="thisSpaceOnly">{{ t("docs.search.thisSpaceOnly") }}</t-checkbox>
    </div>

    <t-loading :loading="loading" size="small">
      <ul v-if="results.length" class="hit-list">
        <li v-for="hit in results" :key="`${hit.kind}:${hit.page_id}:${hit.comment_id ?? ''}`">
          <button type="button" class="hit" @click="open(hit)">
            <span class="hit-head">
              <t-icon :name="iconFor(hit.kind)" size="14px" class="hit-icon" />
              <span class="hit-title">{{ hit.title || t("docs.tree.untitled") }}</span>
              <t-tag v-if="hit.kind !== 'page'" size="small" variant="light">
                {{ t(`docs.search.kind.${hit.kind}`) }}
              </t-tag>
            </span>
            <!-- Rendered as text nodes, never as markup: an excerpt is page
                 content, which is exactly the untrusted text this product
                 exists to store. -->
            <span class="hit-excerpt">
              <span v-for="(segment, i) in segmentsOf(hit)" :key="i" :class="{ mark: segment.match }">{{
                segment.text
              }}</span>
            </span>
          </button>
        </li>
      </ul>

      <p v-else-if="searched && !loading" class="search-empty">
        {{ t("docs.search.nothing", { query: lastQuery }) }}
      </p>
      <p v-else-if="!searched" class="search-hint">{{ t("docs.search.hint") }}</p>

      <p v-if="truncated" class="search-more">{{ t("docs.search.more") }}</p>
    </t-loading>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

import { searchDocs, type SearchHit } from "@/api/docs";

import { highlight } from "./highlight";
import { pageSlug } from "../tree/pageTree";

// Searching pages, comments and text shown by reference.
//
// Debounced rather than fired per keystroke: a search that runs on every
// character sends eight requests for a four-character Chinese query and
// renders the answer to the third one last.

const props = defineProps<{ spaceId?: string }>();
const emit = defineEmits<{ close: [] }>();

const { t } = useI18n();
const router = useRouter();

const query = ref("");
const thisSpaceOnly = ref(true);
const loading = ref(false);
const searched = ref(false);
const truncated = ref(false);
const lastQuery = ref("");
const results = shallowRef<SearchHit[]>([]);

const scope = computed(() => (props.spaceId && thisSpaceOnly.value ? props.spaceId : undefined));

let timer: ReturnType<typeof setTimeout> | null = null;
/** Guards against an older response overwriting a newer one. */
let sequence = 0;

function onType() {
  if (timer) clearTimeout(timer);
  const text = query.value.trim();
  if (!text) {
    results.value = [];
    searched.value = false;
    truncated.value = false;
    return;
  }
  timer = setTimeout(() => void run(text), 220);
}

async function run(text: string) {
  const mine = ++sequence;
  loading.value = true;
  try {
    const res = await searchDocs(text, { space: scope.value });
    if (mine !== sequence) return;
    results.value = res.hits;
    truncated.value = res.truncated;
    lastQuery.value = res.query;
    searched.value = true;
  } catch {
    if (mine !== sequence) return;
    results.value = [];
    searched.value = true;
    truncated.value = false;
  } finally {
    if (mine === sequence) loading.value = false;
  }
}

function segmentsOf(hit: SearchHit) {
  return highlight(hit.excerpt, lastQuery.value);
}

function iconFor(kind: SearchHit["kind"]): string {
  if (kind === "comment") return "chat";
  if (kind === "transclusion") return "link";
  return "file";
}

function open(hit: SearchHit) {
  emit("close");
  router.push({
    name: "docsSpace",
    params: { slug: hit.space_slug, pageSlug: pageSlug(hit.title, hit.short_id) },
    ...(hit.comment_id ? { query: { comment: hit.comment_id } } : {}),
  });
}

// Re-run when the scope changes, so ticking the box does not need a keystroke.
watch(thisSpaceOnly, () => {
  const text = query.value.trim();
  if (text) void run(text);
});
</script>

<style scoped>
.search-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 200px;
}

.scope-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.hit-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-height: 52vh;
  margin: 0;
  padding: 0;
  overflow-y: auto;
  list-style: none;
}

.hit {
  display: flex;
  flex-direction: column;
  gap: 3px;
  width: 100%;
  padding: 9px 10px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.hit:hover {
  background: var(--td-bg-color-container-hover);
}

.hit-head {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.hit-icon {
  flex: none;
  color: var(--td-text-color-placeholder);
}

.hit-title {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  font-size: 14px;
  font-weight: 500;
}

.hit-excerpt {
  display: -webkit-box;
  overflow: hidden;
  color: var(--td-text-color-secondary);
  font-size: 12px;
  line-height: 1.6;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.mark {
  border-radius: 2px;
  background: var(--td-warning-color-light);
  color: var(--td-text-color-primary);
  font-weight: 600;
}

.search-empty,
.search-hint,
.search-more {
  margin: 0;
  padding: 12px 2px;
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}
</style>
