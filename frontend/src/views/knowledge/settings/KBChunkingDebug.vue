<template>
  <div class="shrink-0">
    <!-- Inline text trigger: sits next to the strategy selector so users
         discover the test action exactly when they're thinking about which
         strategy to pick. Kept as a text-style button to match the project's
         secondary-action convention (no heavy outline / filled treatment). -->
    <!-- The old trigger was a text button that only shifted colour on hover; no underline. -->
    <Button
      type="button"
      variant="link"
      class="px-0 font-semibold hover:text-[var(--td-brand-color-hover)] hover:no-underline active:text-[var(--td-brand-color-active)]"
      @click="open = true"
    >
      <CirclePlayIcon />
      {{ $t("knowledgeEditor.chunking.debug.toggle") }}
    </Button>

    <!--
      This drawer opens from inside the knowledge-base editor, a z-[1000]
      overlay; SettingDrawer sits on the old TDesign drawer layer (z-[2500],
      overlay included), so it slides in above the editor.
    -->
    <SettingDrawer
      v-model:visible="open"
      :title="$t('knowledgeEditor.chunking.debug.toggle')"
      width="720px"
      :resizable="false"
      storage-key=""
      hide-footer
    >
      <div class="flex flex-col gap-5">
        <!-- Input section -->
        <section class="flex flex-col gap-2">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="text-foreground text-[13px] font-medium">
              {{ $t("knowledgeEditor.chunking.debug.sampleLabel") }}
            </div>
            <div class="flex flex-wrap items-center gap-1">
              <span class="text-placeholder mr-1 text-xs">{{ $t("knowledgeEditor.chunking.debug.presetLabel") }}</span>
              <Button
                v-for="p in samples"
                :key="p.id"
                type="button"
                variant="ghost"
                size="xs"
                class="text-muted-foreground hover:text-primary"
                @click="loadSample(p.id)"
              >
                {{ $t(`knowledgeEditor.chunking.debug.${p.labelKey}`) }}
              </Button>
            </div>
          </div>
          <Textarea
            v-model="sample"
            :placeholder="$t('knowledgeEditor.chunking.debug.samplePlaceholder')"
            :rows="6"
            :maxlength="MAX_CHARS"
            class="max-h-[256px] min-h-[136px] overflow-y-auto"
          />
          <div class="mt-1 flex justify-end">
            <!-- type="button" prevents any accidental parent-form submit. -->
            <Button
              type="button"
              :disabled="!sample || sample.length === 0 || loading"
              @click.prevent.stop="runPreview"
            >
              <Loader2Icon v-if="loading" class="animate-spin" />
              <CirclePlayIcon v-else />
              {{ $t("knowledgeEditor.chunking.debug.runButton") }}
            </Button>
          </div>
        </section>

        <!-- Loading state — explicit so the user sees something is happening
             even if the result block hasn't appeared yet. -->
        <div
          v-if="loading"
          class="bg-accent text-muted-foreground flex items-center gap-2.5 rounded-md px-3.5 py-3 text-[13px]"
        >
          <Loader2Icon class="size-4 animate-spin" />
          <span>{{ $t("knowledgeEditor.chunking.debug.loading") }}</span>
        </div>

        <!-- Error block: prominent so it can't be missed when an API call fails. -->
        <div
          v-else-if="error"
          class="bg-destructive/10 text-destructive flex items-start gap-2.5 rounded-md px-3.5 py-3 text-[13px]"
        >
          <CircleAlertIcon class="mt-0.5 size-4 shrink-0" />
          <div>
            <strong class="mb-0.5 block">{{ $t("knowledgeEditor.chunking.debug.errorPrefix") }}</strong>
            <span class="text-destructive font-normal break-words">{{ error }}</span>
          </div>
        </div>

        <section v-else-if="result" class="flex flex-col gap-0">
          <!-- Tier summary -->
          <div class="border-border mb-4 flex flex-col gap-2 border-b border-dashed pb-3">
            <div class="text-foreground flex flex-wrap items-center gap-2 text-[13px]">
              <span class="text-muted-foreground min-w-[120px] font-medium"
                >{{ $t("knowledgeEditor.chunking.debug.selectedTier") }}:</span
              >
              <Badge :class="tierBadgeClass(result.selected_tier)">
                {{ tierDisplay(result.selected_tier) }}
              </Badge>
              <span v-if="fallbackWarning" class="text-warning text-xs">
                {{ $t("knowledgeEditor.chunking.debug.fallbackWarning") }}
              </span>
            </div>
            <div
              v-if="(result.rejected || []).length > 0"
              class="text-foreground flex flex-wrap items-center gap-2 text-[13px]"
            >
              <span class="text-muted-foreground min-w-[120px] font-medium"
                >{{ $t("knowledgeEditor.chunking.debug.rejected") }}:</span
              >
              <span class="flex flex-wrap gap-1.5">
                <Badge v-for="r in result.rejected || []" :key="r.tier" variant="secondary">
                  {{ tierDisplay(r.tier) }}: {{ r.reason }}
                </Badge>
              </span>
            </div>
          </div>

          <!-- Profile stats -->
          <div
            class="border-border bg-border mb-4 grid [grid-template-columns:repeat(auto-fit,minmax(110px,1fr))] [gap:1px] overflow-hidden rounded-md border"
          >
            <div class="bg-card px-2 py-3 text-center">
              <div class="text-foreground text-lg leading-[1.2] font-semibold [font-variant-numeric:tabular-nums]">
                {{ result.profile.total_lines }}
              </div>
              <div class="text-muted-foreground mt-1 text-[11px]">
                {{ $t("knowledgeEditor.chunking.debug.profile.lines") }}
              </div>
            </div>
            <div class="bg-card px-2 py-3 text-center">
              <div class="text-foreground text-lg leading-[1.2] font-semibold [font-variant-numeric:tabular-nums]">
                {{ result.profile.total_chars }}
              </div>
              <div class="text-muted-foreground mt-1 text-[11px]">
                {{ $t("knowledgeEditor.chunking.debug.profile.chars") }}
              </div>
            </div>
            <div class="bg-card px-2 py-3 text-center">
              <div class="text-foreground text-lg leading-[1.2] font-semibold [font-variant-numeric:tabular-nums]">
                {{ result.profile.md_heading_total }}
              </div>
              <div class="text-muted-foreground mt-1 text-[11px]">
                {{ $t("knowledgeEditor.chunking.debug.profile.headings") }}
              </div>
            </div>
            <div class="bg-card px-2 py-3 text-center">
              <div class="text-foreground text-lg leading-[1.2] font-semibold [font-variant-numeric:tabular-nums]">
                {{ result.profile.form_feed_count }}
              </div>
              <div class="text-muted-foreground mt-1 text-[11px]">
                {{ $t("knowledgeEditor.chunking.debug.profile.pageBreaks") }}
              </div>
            </div>
            <div class="bg-card px-2 py-3 text-center">
              <div class="text-foreground text-lg leading-[1.2] font-semibold [font-variant-numeric:tabular-nums]">
                {{
                  result.profile.german_chapter_count +
                  result.profile.english_chapter_count +
                  result.profile.chinese_chapter_count
                }}
              </div>
              <div class="text-muted-foreground mt-1 text-[11px]">
                {{ $t("knowledgeEditor.chunking.debug.profile.chapterMarkers") }}
              </div>
            </div>
            <div class="bg-card px-2 py-3 text-center">
              <div class="text-foreground text-lg leading-[1.2] font-semibold [font-variant-numeric:tabular-nums]">
                {{ (result.profile.detected_langs || []).join(", ") || "—" }}
              </div>
              <div class="text-muted-foreground mt-1 text-[11px]">
                {{ $t("knowledgeEditor.chunking.debug.profile.languages") }}
              </div>
            </div>
          </div>

          <!-- Chunk stats line -->
          <div
            class="bg-accent text-muted-foreground mb-3 flex flex-wrap items-baseline gap-2 rounded-md px-3.5 py-2.5 text-[13px] [font-variant-numeric:tabular-nums]"
          >
            <span>
              <strong class="text-foreground mr-1 text-sm font-semibold">{{ result.stats.count }}</strong>
              {{ $t("knowledgeEditor.chunking.debug.stats.chunks") }}
            </span>
            <span class="text-placeholder">·</span>
            <span>Ø {{ result.stats.avg_chars }}</span>
            <span class="text-placeholder">·</span>
            <span>σ {{ result.stats.stddev_chars }}</span>
            <span class="text-placeholder">·</span>
            <span>min {{ result.stats.min_chars }}</span>
            <span class="text-placeholder">·</span>
            <span>max {{ result.stats.max_chars }}</span>
            <span v-if="result.stats.truncated_to" class="text-warning ml-auto text-xs">
              {{ $t("knowledgeEditor.chunking.debug.stats.truncated", { total: result.stats.truncated_to }) }}
            </span>
          </div>

          <!-- Chunks list — no inner scroll; the drawer body handles scrolling
               so expanded cards always show their full content. -->
          <ol class="m-0 flex flex-col gap-2 p-0 [list-style:none]">
            <li
              v-for="c in result.chunks"
              :key="c.seq"
              class="bg-card overflow-hidden rounded-md border transition-[border-color,box-shadow] duration-150"
              :class="
                expandedChunks.has(c.seq)
                  ? 'border-[var(--td-brand-color-light-active)] shadow-[inset_0_0_0_1px_var(--td-brand-color-light)]'
                  : 'border-border'
              "
            >
              <button
                type="button"
                data-slot="chunk-toggle"
                class="bg-accent text-muted-foreground m-0 flex w-full cursor-pointer items-center gap-3 border-none px-3.5 py-2.5 text-left text-xs hover:bg-[var(--td-bg-color-component-hover)] focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-[var(--td-brand-color-focus)]"
                :aria-expanded="expandedChunks.has(c.seq)"
                @click="toggleChunk(c.seq)"
              >
                <span class="text-foreground shrink-0 font-semibold [font-variant-numeric:tabular-nums]"
                  >#{{ c.seq }}</span
                >
                <span class="text-foreground shrink-0 [font-variant-numeric:tabular-nums]">
                  {{ c.size_chars }} {{ $t("knowledgeEditor.chunking.characters") }}
                  <span class="text-muted-foreground font-normal">· ~{{ c.size_tokens_approx }} tok</span>
                </span>
                <span
                  class="text-placeholder shrink-0 [font-family:var(--td-font-family-mono,ui-monospace,'SF_Mono',Menlo,Consolas,'Liberation_Mono',monospace)] [font-variant-numeric:tabular-nums]"
                  >{{ c.start }}–{{ c.end }}</span
                >
                <span
                  v-if="c.context_header"
                  class="bg-primary/10 text-primary max-w-[240px] min-w-0 flex-[0_1_auto] truncate rounded-[10px] px-2 py-0.5 [font-family:var(--td-font-family-mono,ui-monospace,'SF_Mono',Menlo,Consolas,'Liberation_Mono',monospace)] text-[11px]"
                  :title="c.context_header"
                >
                  {{ c.context_header }}
                </span>
                <ChevronDownIcon
                  class="text-muted-foreground ml-auto size-4 shrink-0 transition-transform duration-150"
                  :class="expandedChunks.has(c.seq) ? 'rotate-180' : ''"
                />
              </button>
              <div class="border-border bg-card border-t" :class="expandedChunks.has(c.seq) ? '' : 'relative'">
                <pre
                  class="text-foreground m-0 px-3.5 py-3 [font-family:var(--td-font-family-mono,ui-monospace,'SF_Mono',Menlo,Consolas,'Liberation_Mono',monospace)] text-[12.5px] leading-[1.6] break-words whitespace-pre-wrap"
                  :class="
                    expandedChunks.has(c.seq)
                      ? ''
                      : 'after:to-card line-clamp-3 overflow-hidden after:absolute after:inset-x-0 after:bottom-0 after:h-7 after:bg-gradient-to-b after:from-transparent'
                  "
                  >{{ c.content }}</pre>
              </div>
            </li>
          </ol>
        </section>
      </div>
    </SettingDrawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import { ChevronDownIcon, CircleAlertIcon, CirclePlayIcon, Loader2Icon } from "@lucide/vue";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { previewChunking } from "@/api/chunker";
import type { PreviewChunkingResponse, StrategyTier } from "@/types/chunker";
import { CHUNKING_SAMPLES, DEFAULT_SAMPLE_ID } from "./chunkingSamples";

interface Props {
  config: {
    chunkSize: number;
    chunkOverlap: number;
    separators: string[];
    enableParentChild: boolean;
    parentChunkSize: number;
    childChunkSize: number;
    strategy?: string;
    tokenLimit?: number;
    languages?: string[];
  };
}

const props = defineProps<Props>();
const { t } = useI18n();

// Mirrors handler.previewMaxChars on the backend. Keep in sync.
const MAX_CHARS = 64 * 1024;

const open = ref(false);
const sample = ref("");
const loading = ref(false);
const error = ref("");
const result = ref<PreviewChunkingResponse | null>(null);
const expandedChunks = ref(new Set<number>());

const samples = CHUNKING_SAMPLES;

// Auto-load the default preset the first time the user opens the drawer with
// an empty textarea. We don't overwrite their input on subsequent opens, and
// we don't pre-load on component mount (zero cost when the drawer is unused).
watch(open, (isOpen) => {
  if (isOpen && sample.value.trim() === "") {
    loadSample(DEFAULT_SAMPLE_ID);
  }
});

const loadSample = (id: string) => {
  const preset = samples.find((s) => s.id === id);
  if (!preset) return;
  sample.value = preset.text;
  // Clear any previous run so the user isn't looking at stale results
  // attributed to the old text.
  result.value = null;
  error.value = "";
  expandedChunks.value = new Set();
};

const fallbackWarning = computed(() => {
  if (!result.value) return false;
  return result.value.selected_tier === "legacy" && (result.value.rejected || []).length > 0;
});

const runPreview = async () => {
  loading.value = true;
  error.value = "";
  result.value = null;
  expandedChunks.value = new Set();
  try {
    // Send all fields explicitly (including empty / 0 / []) so the
    // preview faithfully reflects what would happen on save. Mirrors
    // the buildSubmitData convention in KnowledgeBaseEditorModal.
    const resp = await previewChunking({
      text: sample.value,
      chunking_config: {
        chunk_size: props.config.chunkSize,
        chunk_overlap: props.config.chunkOverlap,
        separators: props.config.separators,
        enable_parent_child: props.config.enableParentChild,
        parent_chunk_size: props.config.parentChunkSize,
        child_chunk_size: props.config.childChunkSize,
        strategy: props.config.strategy ?? "",
        token_limit: props.config.tokenLimit ?? 0,
        languages: props.config.languages ?? [],
      },
    });
    // The axios interceptor in utils/request.ts already unwraps the
    // outer envelope and returns the response body. So resp here is
    // { success: true, data: PreviewChunkingResponse } directly.
    // If the backend ever responds with 200 + { success: false, error },
    // surface that error instead of swallowing it under a generic message.
    if (!resp) {
      throw new Error("empty response");
    }
    if (resp.success !== true) {
      throw new Error((resp as any).error || "preview failed");
    }
    if (!resp.data) {
      throw new Error("response missing data");
    }
    result.value = resp.data;
  } catch (e: any) {
    // Pull a useful message out of the error shapes our request layer
    // produces: rejected interceptor sends { status, message, ... }.
    const msg = e?.message || (typeof e === "string" ? e : "") || "unknown error";
    error.value = msg;
    // Console log so users can debug from DevTools too.
    console.error("[KBChunkingDebug] previewChunking failed:", e);
    // Toast for visibility.
    MessagePlugin.error(t("knowledgeEditor.chunking.debug.errorPrefix") + ": " + msg);
  } finally {
    loading.value = false;
  }
};

const toggleChunk = (seq: number) => {
  const next = new Set(expandedChunks.value);
  if (next.has(seq)) next.delete(seq);
  else next.add(seq);
  expandedChunks.value = next;
};

// `recursive` and `legacy` use the same SplitText path under the hood
// (see internal/infrastructure/chunker/strategy.go); their distinction is
// only a debugging hint about how the tier was reached. Surface them under
// the user-facing legacy label to avoid implying two different splitters.
const normalizeTier = (tier: StrategyTier): StrategyTier => (tier === "recursive" ? "legacy" : tier);

const tierDisplay = (tier: StrategyTier) => {
  return t(`knowledgeEditor.chunking.strategies.${normalizeTier(tier)}.label`);
};

const tierBadgeClass = (tier: StrategyTier): string => {
  switch (normalizeTier(tier)) {
    case "heading":
    case "heuristic":
      return "bg-success/10 text-success border-success/40";
    case "legacy":
    default:
      return "bg-muted text-muted-foreground border-border";
  }
};
</script>
