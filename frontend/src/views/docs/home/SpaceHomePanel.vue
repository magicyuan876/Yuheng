<template>
  <div class="flex w-full max-w-[720px] flex-col gap-7 text-left">
    <div v-if="labels.length" class="flex flex-wrap items-center gap-2">
      <button
        v-for="l in labels"
        :key="l.id"
        type="button"
        data-slot="label-chip"
        class="text-foreground inline-flex cursor-pointer items-center gap-1.5 rounded-full border px-2.5 py-[3px] text-[13px]"
        :class="selected.has(l.id) ? 'border-primary bg-[var(--td-brand-color-light)]' : 'border-border bg-card'"
        @click="toggleLabel(l.id)"
      >
        <span class="size-2 flex-none rounded-full" :class="DOT_CLASS[l.color] ?? 'bg-placeholder'" />
        <span>{{ l.name }}</span>
        <span class="text-placeholder tabular-nums">{{ l.page_count }}</span>
      </button>
      <Button v-if="selected.size" variant="ghost" size="sm" @click="clearLabels">
        {{ t("docs.home.clearFilter") }}
      </Button>
    </div>

    <!-- Filtering replaces the lists: somebody who has narrowed to a label is
         asking one question, and answering it beside three other lists buries
         the answer. -->
    <section v-if="selected.size">
      <h2 class="text-muted-foreground m-0 mb-2.5 text-[13px] font-semibold tracking-[0.04em] uppercase">
        {{ t("docs.home.filtered", { count: filtered.length }) }}
      </h2>
      <div v-if="filtering" class="flex items-center justify-center gap-2 py-3">
        <Loader2Icon class="size-4 animate-spin" />
      </div>
      <template v-else>
        <ul v-if="filtered.length" class="m-0 flex list-none flex-col gap-0.5 p-0">
          <li v-for="n in filtered" :key="n.id">
            <button
              type="button"
              data-slot="home-row"
              class="hover:bg-accent flex w-full cursor-pointer items-center gap-2.5 rounded-md border-0 px-2.5 py-[7px] text-left text-sm text-inherit"
              @click="emit('open', n)"
            >
              <span class="w-5 flex-none text-center">{{ n.icon || "📄" }}</span>
              <span class="min-w-0 flex-1 truncate">{{ n.title || t("docs.tree.untitled") }}</span>
            </button>
          </li>
        </ul>
        <p v-else class="text-placeholder m-0 text-[13px]">{{ t("docs.home.noneWithLabels") }}</p>
      </template>
    </section>

    <template v-else>
      <section v-if="favourites.length">
        <h2 class="text-muted-foreground m-0 mb-2.5 text-[13px] font-semibold tracking-[0.04em] uppercase">
          {{ t("docs.home.favourites") }}
        </h2>
        <ul class="m-0 flex list-none flex-col gap-0.5 p-0">
          <li v-for="n in favourites" :key="n.id">
            <button
              type="button"
              data-slot="home-row"
              class="hover:bg-accent flex w-full cursor-pointer items-center gap-2.5 rounded-md border-0 px-2.5 py-[7px] text-left text-sm text-inherit"
              @click="emit('open', n)"
            >
              <span class="w-5 flex-none text-center">{{ n.icon || "📄" }}</span>
              <span class="min-w-0 flex-1 truncate">{{ n.title || t("docs.tree.untitled") }}</span>
            </button>
          </li>
        </ul>
      </section>

      <section v-if="visited.length">
        <h2 class="text-muted-foreground m-0 mb-2.5 text-[13px] font-semibold tracking-[0.04em] uppercase">
          {{ t("docs.home.recentlyViewed") }}
        </h2>
        <!-- Said plainly rather than left to be discovered: this list is on
             this device only, because it is never written to the server. -->
        <p class="text-placeholder m-0 -mt-1.5 mb-2.5 text-xs">{{ t("docs.home.recentlyViewedNote") }}</p>
        <ul class="m-0 flex list-none flex-col gap-0.5 p-0">
          <li v-for="v in visited" :key="v.pageId">
            <button
              type="button"
              data-slot="home-row"
              class="hover:bg-accent flex w-full cursor-pointer items-center gap-2.5 rounded-md border-0 px-2.5 py-[7px] text-left text-sm text-inherit"
              @click="emit('openVisit', v)"
            >
              <span class="w-5 flex-none text-center">{{ v.icon || "📄" }}</span>
              <span class="min-w-0 flex-1 truncate">{{ v.title || t("docs.tree.untitled") }}</span>
            </button>
          </li>
        </ul>
      </section>

      <section>
        <h2 class="text-muted-foreground m-0 mb-2.5 text-[13px] font-semibold tracking-[0.04em] uppercase">
          {{ t("docs.home.recentlyEdited") }}
        </h2>
        <div v-if="loading" class="flex items-center justify-center gap-2 py-3">
          <Loader2Icon class="size-4 animate-spin" />
        </div>
        <template v-else>
          <ul v-if="recent.length" class="m-0 flex list-none flex-col gap-0.5 p-0">
            <li v-for="n in recent" :key="n.id">
              <button
                type="button"
                data-slot="home-row"
                class="hover:bg-accent flex w-full cursor-pointer items-center gap-2.5 rounded-md border-0 px-2.5 py-[7px] text-left text-sm text-inherit"
                @click="emit('open', n)"
              >
                <span class="w-5 flex-none text-center">{{ n.icon || "📄" }}</span>
                <span class="min-w-0 flex-1 truncate">{{ n.title || t("docs.tree.untitled") }}</span>
                <span class="text-placeholder flex-none text-xs">{{ when(n.content_updated_at || n.updated_at) }}</span>
              </button>
            </li>
          </ul>
          <p v-else class="text-placeholder m-0 text-[13px]">{{ t("docs.home.nothingYet") }}</p>
        </template>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";

import { Loader2Icon } from "@lucide/vue";

import { getSpaceHome, pagesWithLabels, type LabelView, type TreeNode } from "@/api/docs";
import { Button } from "@/components/ui/button";

import { browserStore, readVisits, type Visit } from "./recentlyViewed";

// A space's landing page: what this person starred, where they were, what the
// space has been working on, and the vocabulary to narrow any of it.
//
// Three of the four lists come from one request — a landing page that paints
// in four stages reads as a page that is broken. The fourth, where they were,
// never leaves the browser; see recentlyViewed.ts.

/** A closed set, themed in one place — see LabelColors in the service. */
const DOT_CLASS: Record<string, string> = {
  gray: "bg-[#8b8f96]",
  red: "bg-[#e34d59]",
  orange: "bg-[#ed7b2f]",
  yellow: "bg-[#ebb105]",
  green: "bg-[#2ba471]",
  teal: "bg-[#0594fa]",
  blue: "bg-[#366ef4]",
  purple: "bg-[#834ec2]",
  pink: "bg-[#ed49b4]",
};

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
