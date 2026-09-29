<template>
  <Teleport to="body" :disabled="!useOverlay">
    <Transition name="references-panel" @after-enter="handlePanelAfterEnter">
      <aside
        v-if="visible"
        class="chat-references-panel bg-popover fixed top-0 right-0 bottom-0 z-[1201] flex w-[min(420px,100vw)] flex-col border-l border-[var(--td-component-stroke)]"
        :class="useOverlay ? 'shadow-[-12px_0_32px_rgba(0,0,0,0.12)]' : 'shadow-[-8px_0_24px_rgba(0,0,0,0.06)]'"
        role="complementary"
        :aria-label="panelTitle"
      >
        <header
          class="flex items-center justify-between gap-3 border-b border-[var(--td-component-stroke)] px-4 pt-4 pb-3"
        >
          <div class="flex min-w-0 items-center gap-2.5">
            <h3 class="text-muted-foreground m-0 text-sm leading-[1.4] font-medium">
              {{ panelTitle }}<span v-if="totalCount" class="text-placeholder font-medium"> · {{ totalCount }}</span>
            </h3>
          </div>
          <button
            type="button"
            data-slot="references-close"
            class="bg-secondary text-muted-foreground hover:text-foreground flex h-9 w-9 shrink-0 cursor-pointer items-center justify-center rounded-[10px] border-0 transition-[background,color] duration-150 hover:bg-[color-mix(in_srgb,var(--td-text-color-primary)_8%,var(--td-bg-color-secondarycontainer))]"
            :aria-label="t('common.close')"
            @click="close"
          >
            <XIcon class="size-5" />
          </button>
        </header>

        <div ref="listElement" class="min-h-0 flex-1 overflow-y-auto px-3 pt-1 pb-6">
          <div v-if="sections.length === 0" class="text-placeholder px-2 py-6 text-center text-[13px]">
            {{ t("chat.referencesDrawerEmpty") }}
          </div>

          <section v-for="section in sections" :key="section.id" class="mt-4 flex flex-col gap-1.5 first:mt-0">
            <h4
              v-if="sections.length > 1"
              class="text-placeholder m-0 mb-2 px-1 text-xs font-semibold tracking-[0.04em] uppercase"
            >
              {{ sectionTitle(section.id) }}
            </h4>

            <article
              v-for="item in section.items"
              :key="item.key"
              :ref="(el) => setItemRef(item.key, el as HTMLElement | null)"
              class="group rounded-xl transition-[background-color] duration-150"
              :class="[item.key === activeHighlightKey ? 'bg-secondary' : 'hover:bg-foreground/[0.04]']"
            >
              <component
                :is="item.kind === 'web' ? 'a' : 'div'"
                class="block px-3 py-2.5 text-inherit no-underline"
                :class="{ 'cursor-pointer': item.kind === 'document' && hasMoreContent(item) }"
                :href="item.kind === 'web' ? item.url : undefined"
                :target="item.kind === 'web' ? '_blank' : undefined"
                :rel="item.kind === 'web' ? 'noopener noreferrer' : undefined"
                :role="item.kind === 'document' && hasMoreContent(item) ? 'button' : undefined"
                :tabindex="item.kind === 'document' && hasMoreContent(item) ? 0 : undefined"
                @mousedown="trackContentPointerDown"
                @click="
                  item.kind === 'document' && hasMoreContent(item) ? toggleDocumentSnippet(item, $event) : undefined
                "
                @keydown.enter="
                  item.kind === 'document' && hasMoreContent(item) ? toggleDocumentSnippet(item) : undefined
                "
                @keydown.space.prevent="
                  item.kind === 'document' && hasMoreContent(item) ? toggleDocumentSnippet(item) : undefined
                "
              >
                <template v-if="item.kind === 'document'">
                  <div class="flex min-w-0 items-start gap-2.5">
                    <FileIcon class="text-foreground mt-[3px] h-4 w-[18px] shrink-0" />
                    <div class="min-w-0 flex-1">
                      <div class="flex min-w-0 items-start gap-2">
                        <h5
                          class="text-foreground m-0 [display:-webkit-box] min-w-0 flex-1 overflow-hidden text-[15px] leading-[1.4] font-semibold break-words [-webkit-box-orient:vertical] [-webkit-line-clamp:2]"
                        >
                          {{ item.title }}
                        </h5>
                        <a
                          v-if="item.knowledgeBaseId && !embeddedMode"
                          class="text-placeholder hover:text-foreground mt-[3px] shrink-0 leading-none transition-[opacity,color] duration-150"
                          :class="item.key === activeHighlightKey ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'"
                          :href="getDocumentHref(item)"
                          target="_blank"
                          rel="noopener noreferrer"
                          :aria-label="t('chat.navigateToDocument')"
                          @click.stop
                        >
                          <ExternalLinkIcon class="size-3.5" />
                        </a>
                      </div>
                      <div v-if="item.videoTimestampsMs?.length" class="mt-1 flex flex-wrap gap-1.5">
                        <a
                          v-for="ts in item.videoTimestampsMs"
                          :key="ts"
                          class="text-primary inline-flex items-center gap-[3px] rounded-[10px] bg-[var(--td-brand-color-light)] px-2 py-px text-xs no-underline hover:bg-[var(--td-brand-color-focus)]"
                          :href="embeddedMode ? undefined : getDocumentHref(item, ts)"
                          :target="embeddedMode ? undefined : '_blank'"
                          rel="noopener noreferrer"
                          :aria-label="t('chat.jumpToVideoTime')"
                          @click.stop
                        >
                          <CirclePlayIcon class="size-3" />
                          {{ formatVideoTimestamp(ts) }}
                        </a>
                      </div>
                      <p
                        v-if="item.snippet && !expandedKeys.has(item.key)"
                        class="text-muted-foreground mt-1 mb-0 [display:-webkit-box] overflow-hidden text-[13px] leading-normal [-webkit-box-orient:vertical] [-webkit-line-clamp:2]"
                      >
                        {{ formatReferenceSnippet(item.snippet) }}
                      </p>
                      <div
                        v-if="expandedKeys.has(item.key)"
                        class="text-muted-foreground mt-1 mb-0 max-h-[360px] overflow-y-auto text-[13px] leading-[1.55] break-words whitespace-pre-wrap"
                      >
                        {{ formatReferenceSnippet(item.content) }}
                      </div>
                    </div>
                  </div>
                </template>
                <template v-else>
                  <div v-if="item.kind === 'web' && item.domain" class="mb-1.5 flex min-w-0 items-center gap-2">
                    <img
                      v-if="item.faviconUrl"
                      class="h-4 w-4 shrink-0 rounded-full object-cover"
                      :src="item.faviconUrl"
                      alt=""
                      loading="lazy"
                      @error="onFaviconError"
                    />
                    <span class="text-placeholder truncate text-[13px] leading-[1.35]">{{ item.domain }}</span>
                  </div>
                  <div v-else-if="item.kind === 'tool' && item.domain" class="mb-1.5 flex min-w-0 items-center gap-2">
                    <WrenchIcon class="text-placeholder size-3.5 shrink-0" />
                    <span class="text-placeholder truncate text-[13px] leading-[1.35]">{{ item.domain }}</span>
                  </div>

                  <h5
                    v-if="shouldShowItemTitle(item)"
                    class="text-foreground m-0 [display:-webkit-box] overflow-hidden text-[15px] leading-[1.4] font-semibold break-words [-webkit-box-orient:vertical] [-webkit-line-clamp:2]"
                  >
                    {{ item.title }}
                  </h5>

                  <p
                    v-if="item.kind !== 'tool' && item.snippet && !expandedKeys.has(item.key)"
                    class="text-muted-foreground mt-1 mb-0 [display:-webkit-box] overflow-hidden text-[13px] leading-normal [-webkit-box-orient:vertical] [-webkit-line-clamp:2]"
                  >
                    {{ formatReferenceSnippet(item.snippet) }}
                  </p>
                  <div
                    v-if="item.kind === 'tool' && item.content"
                    class="text-muted-foreground mt-1 mb-0 text-[13px] leading-[1.55] break-words whitespace-pre-wrap"
                  >
                    {{ formatReferenceSnippet(item.content) }}
                  </div>
                </template>
              </component>
            </article>
          </section>
        </div>
      </aside>
    </Transition>
  </Teleport>

  <Transition name="references-backdrop">
    <div v-if="visible && useOverlay" class="fixed inset-0 z-[1200] bg-black/28" @click="close" />
  </Transition>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

import { CirclePlayIcon, ExternalLinkIcon, FileIcon, WrenchIcon, XIcon } from "@lucide/vue";

import { useChatReferencesDrawer } from "@/composables/useChatReferencesDrawer";
import {
  buildReferenceSections,
  formatReferenceSnippet,
  formatVideoTimestamp,
  resolveReferenceHighlightKey,
  type ReferenceListItem,
} from "@/utils/referenceSources";

const props = defineProps<{
  embeddedMode?: boolean;
  overlayBreakpoint?: number;
}>();

const { t } = useI18n();
const router = useRouter();
const drawer = useChatReferencesDrawer();

const listElement = ref<HTMLElement | null>(null);
const itemElements = new Map<string, HTMLElement>();
const expandedKeys = reactive(new Set<string>());
const pointerDownSelectionText = ref("");
const panelEntered = ref(false);

const visible = computed(() => drawer?.visible.value ?? false);
const references = computed(() => drawer?.references.value ?? []);
const highlight = computed(() => drawer?.highlight.value ?? null);

const useOverlay = computed(() => {
  if (props.embeddedMode) return true;
  if (typeof window === "undefined") return false;
  return window.innerWidth < (props.overlayBreakpoint ?? 960);
});

const sections = computed(() => buildReferenceSections(references.value));
const totalCount = computed(() => sections.value.reduce((sum, section) => sum + section.items.length, 0));

const activeHighlightKey = computed(() => resolveReferenceHighlightKey(references.value, highlight.value));

const panelTitle = computed(() => {
  const webCount = sections.value.find((section) => section.id === "web")?.items.length ?? 0;
  const docCount = sections.value.find((section) => section.id === "documents")?.items.length ?? 0;
  const toolCount = sections.value.find((section) => section.id === "tools")?.items.length ?? 0;
  if (toolCount > 0 && webCount === 0 && docCount === 0) {
    return t("chat.referencesDrawerTitleTools");
  }
  if ([webCount, docCount, toolCount].filter((count) => count > 0).length > 1) {
    return t("chat.referencesDrawerTitleMixed");
  }
  if (webCount > 0) {
    return t("chat.referencesDrawerTitleWeb");
  }
  if (docCount > 0) {
    return t("chat.referencesDrawerTitleDocs");
  }
  return t("chat.referencesDrawerTitle");
});

function sectionTitle(id: "web" | "documents" | "tools") {
  if (id === "web") return t("chat.referencesDrawerWebSection");
  if (id === "tools") return t("chat.referencesDrawerToolsSection");
  return t("chat.referencesDrawerDocsSection");
}

function close() {
  drawer?.close();
}

function setItemRef(key: string, el: HTMLElement | null) {
  if (!el) {
    itemElements.delete(key);
    return;
  }
  itemElements.set(key, el);
}

function onFaviconError(event: Event) {
  const img = event.target as HTMLImageElement | null;
  if (img) img.style.display = "none";
}

function hasMoreContent(item: ReferenceListItem) {
  const content = String(item.content || "").trim();
  const snippet = String(item.snippet || "")
    .replace(/…$/, "")
    .trim();
  if (!content) return false;
  if (!snippet) return true;
  return content.length > snippet.length && !content.startsWith(snippet) ? true : content.length > snippet.length + 8;
}

function getSelectedText() {
  if (typeof window === "undefined") return "";
  return window.getSelection()?.toString().trim() || "";
}

function trackContentPointerDown() {
  pointerDownSelectionText.value = getSelectedText();
}

function shouldIgnoreContentToggle(event?: MouseEvent) {
  if (!event) return false;
  const selectedText = getSelectedText();
  if (selectedText || pointerDownSelectionText.value) {
    pointerDownSelectionText.value = "";
    return true;
  }
  pointerDownSelectionText.value = "";
  return false;
}

function toggleDocumentSnippet(item: ReferenceListItem, event?: MouseEvent) {
  if (shouldIgnoreContentToggle(event)) return;
  if (expandedKeys.has(item.key)) {
    expandedKeys.delete(item.key);
    return;
  }
  expandedKeys.add(item.key);
}

function getDocumentHref(item: ReferenceListItem, videoTimestampMs?: number) {
  if (!item.knowledgeBaseId) return "";
  const query: Record<string, string> = {};
  if (item.knowledgeId) query.knowledge_id = item.knowledgeId;
  if (videoTimestampMs != null) query.t = String(videoTimestampMs);
  return router.resolve({
    path: `/platform/knowledge-bases/${item.knowledgeBaseId}`,
    query,
  }).href;
}

function shouldShowItemTitle(item: ReferenceListItem) {
  if (item.kind !== "web") return true;
  const title = item.title?.trim();
  const domain = item.domain?.trim();
  return Boolean(title && title !== domain);
}

async function scrollToHighlight() {
  if (!panelEntered.value) return;
  const key = activeHighlightKey.value;
  if (!key) return;
  await nextTick();
  const el = itemElements.get(key);
  const container = listElement.value;
  if (!el || !container) return;

  // Keep citation positioning inside the drawer. Native element scrolling may
  // also adjust the outer chat viewport while the fixed panel is still
  // entering, which makes the conversation column visibly jump sideways.
  const itemRect = el.getBoundingClientRect();
  const containerRect = container.getBoundingClientRect();
  let nextTop: number | null = null;
  if (itemRect.top < containerRect.top) {
    nextTop = container.scrollTop + itemRect.top - containerRect.top - 8;
  } else if (itemRect.bottom > containerRect.bottom) {
    nextTop = container.scrollTop + itemRect.bottom - containerRect.bottom + 8;
  }
  if (nextTop !== null) {
    container.scrollTo({ top: Math.max(0, nextTop), behavior: "smooth" });
  }
}

function handlePanelAfterEnter() {
  panelEntered.value = true;
  void scrollToHighlight();
}

watch(activeHighlightKey, () => {
  void scrollToHighlight();
});

// A user may click the same citation again after manually scrolling the drawer
// away from its card. The resolved key does not change in that case, but the
// highlight target object does, so replay the scroll for every activation.
watch(highlight, () => {
  void scrollToHighlight();
});

watch(visible, (open) => {
  if (!open) {
    panelEntered.value = false;
    expandedKeys.clear();
    return;
  }
});
</script>

<style scoped>
/* The open/close transitions; named Transition classes have no utility form. */
.references-panel-enter-active {
  transition:
    transform 0.24s cubic-bezier(0.22, 0.61, 0.36, 1),
    opacity 0.24s cubic-bezier(0.22, 0.61, 0.36, 1);
}

.references-panel-leave-active {
  transition:
    transform 0.3s cubic-bezier(0.22, 0.61, 0.36, 1),
    opacity 0.3s cubic-bezier(0.22, 0.61, 0.36, 1);
}

.references-panel-enter-from,
.references-panel-leave-to {
  transform: translateX(100%);
  opacity: 0.6;
}

.references-backdrop-enter-active {
  transition: opacity 0.24s ease;
}

.references-backdrop-leave-active {
  transition: opacity 0.3s ease;
}

.references-backdrop-enter-from,
.references-backdrop-leave-to {
  opacity: 0;
}
</style>
