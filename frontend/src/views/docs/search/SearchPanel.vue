<template>
  <div class="flex min-h-[200px] flex-col gap-3">
    <div class="relative">
      <SearchIcon class="text-placeholder pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
      <!-- h-10 and 16px text: the old field was TDesign's large size. -->
      <Input
        v-model="query"
        class="h-10 pr-9 pl-8 text-base md:text-base"
        :placeholder="t('docs.search.placeholder')"
        autofocus
        @input="onType"
      />
      <!-- The old field was clearable. -->
      <button
        v-if="query"
        type="button"
        data-slot="search-clear"
        class="text-placeholder hover:text-muted-foreground absolute top-1/2 right-2.5 inline-flex size-5 -translate-y-1/2 cursor-pointer items-center justify-center rounded-full"
        :aria-label="t('common.clear')"
        @click="clearQuery"
      >
        <CircleXIcon class="size-4" />
      </button>
    </div>

    <div v-if="spaceId" class="flex items-center gap-3">
      <div class="flex items-center gap-2">
        <Checkbox id="search-this-space" v-model="thisSpaceOnly" />
        <Label for="search-this-space" class="font-normal">{{ t("docs.search.thisSpaceOnly") }}</Label>
      </div>
    </div>

    <div class="relative">
      <div v-if="loading" class="flex items-center justify-center gap-2 py-4">
        <Loader2Icon class="size-4 animate-spin" />
      </div>
      <template v-else>
        <ul v-if="results.length" class="m-0 flex max-h-[52vh] list-none flex-col gap-0.5 overflow-y-auto p-0">
          <li v-for="hit in results" :key="`${hit.kind}:${hit.page_id}:${hit.comment_id ?? ''}`">
            <button
              type="button"
              data-slot="search-hit"
              class="hover:bg-accent flex w-full cursor-pointer flex-col gap-[3px] rounded-[8px] border-0 p-[9px_10px] text-left text-inherit"
              @click="open(hit)"
            >
              <span class="flex min-w-0 items-center gap-1.5">
                <component :is="iconFor(hit.kind)" class="text-placeholder size-3.5 flex-none" />
                <span class="truncate text-sm font-medium">{{ hit.title || t("docs.tree.untitled") }}</span>
                <Badge v-if="hit.kind !== 'page'" variant="secondary">{{ t(`docs.search.kind.${hit.kind}`) }}</Badge>
              </span>
              <!-- Rendered as text nodes, never as markup: an excerpt is page
                   content, which is exactly the untrusted text this product
                   exists to store. -->
              <span
                class="text-muted-foreground [display:-webkit-box] overflow-hidden text-xs leading-[1.6] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]"
              >
                <span v-for="(segment, i) in segmentsOf(hit)" :key="i" :class="{ mark: segment.match }">{{
                  segment.text
                }}</span>
              </span>
            </button>
          </li>
        </ul>

        <p v-else-if="searched && !loading" class="text-placeholder m-0 px-0.5 py-3 text-[13px]">
          {{ t("docs.search.nothing", { query: lastQuery }) }}
        </p>
        <p v-else-if="!searched" class="text-placeholder m-0 px-0.5 py-3 text-[13px]">
          {{ t("docs.search.hint") }}
        </p>

        <p v-if="truncated" class="text-placeholder m-0 px-0.5 py-3 text-[13px]">{{ t("docs.search.more") }}</p>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch, type Component } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

import { CircleXIcon, FileIcon, LinkIcon, Loader2Icon, MessageSquareIcon, SearchIcon } from "@lucide/vue";

import { searchDocs, type SearchHit } from "@/api/docs";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

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

function clearQuery() {
  query.value = "";
  onType();
}

function iconFor(kind: SearchHit["kind"]): Component {
  if (kind === "comment") return MessageSquareIcon;
  if (kind === "transclusion") return LinkIcon;
  return FileIcon;
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
/* Highlight for matched terms in the excerpt; kept as the one visual detail
   that has no Tailwind utility in the token bridge. */
.mark {
  border-radius: 2px;
  background: var(--td-warning-color-light);
  color: var(--td-text-color-primary);
  font-weight: 600;
}
</style>
