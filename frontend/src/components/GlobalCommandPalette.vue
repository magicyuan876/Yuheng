<template>
  <Dialog :open="dialogVisible" @update:open="(v) => (dialogVisible = v)">
    <!--
      The palette sits 10vh from the top rather than centred, so its input
      stays put while the result list grows and shrinks beneath it. The
      dialog's own close button is off: the input row carries one.
    -->
    <DialogContent
      class="cmdk-dialog top-[10vh] block w-[640px] max-w-[calc(100%-2rem)] translate-y-0 gap-0 overflow-hidden p-0 sm:max-w-[640px]"
      :show-close-button="false"
      @open-auto-focus="onDialogOpenAutoFocus"
    >
      <DialogTitle class="sr-only">{{ t("commandPalette.placeholder") }}</DialogTitle>
      <div class="flex max-h-[60vh] min-h-[320px] flex-col" @keydown="onKeyDown">
        <!-- Input row -->
        <div class="border-border flex items-center gap-2 border-b px-3.5 py-3">
          <SearchIcon class="text-placeholder size-4 shrink-0" />
          <span
            v-if="activeKbScope"
            class="bg-secondary text-foreground inline-flex h-[26px] max-w-[220px] shrink-0 items-center gap-1.5 rounded-[4px] pr-0.5 pl-2 text-xs font-medium"
            :title="activeKbScope.name"
          >
            <FolderIcon class="text-muted-foreground size-3 shrink-0" />
            <span class="truncate">{{ activeKbScope.name }}</span>
            <button
              type="button"
              data-slot="scope-chip-remove"
              class="text-placeholder hover:text-foreground inline-flex size-5 items-center justify-center rounded-[3px] leading-none hover:bg-black/5"
              :title="t('commandPalette.scope.remove')"
              :aria-label="t('commandPalette.scope.remove')"
              @click="clearKbScope"
            >
              <XIcon class="size-3" />
            </button>
          </span>
          <input
            ref="inputRef"
            v-model="query"
            type="text"
            data-slot="cmdk-input"
            class="text-foreground placeholder:text-placeholder min-w-0 flex-1 border-none bg-transparent text-[15px] outline-none"
            :placeholder="activeKbScope ? t('commandPalette.scope.placeholder') : t('commandPalette.placeholder')"
            autofocus
            spellcheck="false"
            @keydown="onInputKeyDown"
          />
          <span v-if="loading" class="flex items-center">
            <Loader2Icon class="text-primary size-4 animate-spin" />
          </span>
          <Tooltip>
            <TooltipTrigger as-child>
              <button
                type="button"
                data-slot="icon-button"
                class="text-muted-foreground hover:bg-secondary hover:text-foreground inline-flex size-7 shrink-0 items-center justify-center rounded-md transition-[background] duration-100"
                :class="{ 'bg-secondary text-foreground': drawerVisible }"
                :aria-label="t('commandPalette.retrieval')"
                @click="drawerVisible = true"
              >
                <SettingsIcon class="size-4" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="bottom">{{ t("commandPalette.retrieval") }}</TooltipContent>
          </Tooltip>
          <button
            type="button"
            data-slot="icon-button"
            class="text-muted-foreground hover:bg-secondary hover:text-foreground inline-flex size-7 shrink-0 items-center justify-center rounded-md transition-[background] duration-100"
            :aria-label="t('commandPalette.hotkey.esc')"
            @click="handleClose"
          >
            <XIcon class="size-4" />
          </button>
        </div>

        <!-- Results -->
        <div ref="scrollRef" class="min-h-0 flex-1 overflow-y-auto px-1 py-1.5">
          <!-- Empty idle state: recent + quick actions -->
          <template v-if="!query.trim()">
            <ResultGroup
              v-if="recentQueries.length"
              :label="t('commandPalette.group.recent')"
              :action="t('commandPalette.clearRecent')"
              @action="commandPaletteStore.clearRecent()"
            >
              <ResultItem
                v-for="(q, i) in recentQueries"
                :key="'r-' + q"
                icon-name="history"
                :index="flatIndexFor('recent', i)"
                :selected="selectedIndex === flatIndexFor('recent', i)"
                :shortcut="shortcutFor(flatIndexFor('recent', i))"
                :title="q"
                @primary="query = q"
                @hover="selectItemAt($event)"
              />
            </ResultGroup>

            <ResultGroup :label="t('commandPalette.group.quickActions')">
              <ResultItem
                v-for="(c, i) in allCommands"
                :key="'cmd-' + c.id"
                :index="flatIndexFor('commands', i)"
                :selected="selectedIndex === flatIndexFor('commands', i)"
                :shortcut="shortcutFor(flatIndexFor('commands', i))"
                :icon-name="c.icon"
                :title="c.label"
                @primary="c.run"
                @hover="selectItemAt($event)"
              />
            </ResultGroup>
          </template>

          <!-- Active search results -->
          <template v-else>
            <!-- Chunks (per file) -->
            <ResultGroup
              v-if="isGroupVisible('chunks') && fileGroups.length"
              :label="t('commandPalette.group.chunks')"
              :count="totalChunks"
            >
              <template v-for="(item, i) in flatChunkItems" :key="'c-' + item.file.knowledgeId + '-' + item.chunk.id">
                <ResultItem
                  :index="flatIndexFor('chunks', i)"
                  :selected="selectedIndex === flatIndexFor('chunks', i)"
                  :shortcut="shortcutFor(flatIndexFor('chunks', i))"
                  icon-name="file"
                  :badge="
                    item.chunk.match_type === 'vector'
                      ? t('commandPalette.match.vector')
                      : t('commandPalette.match.keyword')
                  "
                  :badge-variant="item.chunk.match_type === 'vector' ? 'vector' : 'keyword'"
                  :score="item.chunk.score"
                  @primary="openChunk(item)"
                  @hover="selectItemAt($event)"
                >
                  <template #title>
                    <span class="overflow-hidden text-ellipsis">{{ item.file.title }}</span>
                    <span
                      v-if="item.file.kbName"
                      class="text-placeholder bg-secondary rounded-[3px] px-1.5 py-px text-[11px] font-normal"
                      >{{ item.file.kbName }}</span
                    >
                  </template>
                  <template #subtitle>
                    <span v-html="highlight(item.chunk.matched_content || item.chunk.content)" />
                  </template>
                </ResultItem>
              </template>
            </ResultGroup>

            <!-- Messages -->
            <ResultGroup
              v-if="isGroupVisible('messages') && messageGroups.length"
              :label="t('commandPalette.group.messages')"
              :count="totalMessages"
            >
              <template v-for="(item, i) in flatMessageItems" :key="'m-' + item.msg.request_id">
                <ResultItem
                  :index="flatIndexFor('messages', i)"
                  :selected="selectedIndex === flatIndexFor('messages', i)"
                  :shortcut="shortcutFor(flatIndexFor('messages', i))"
                  icon-name="chat"
                  :score="item.msg.score"
                  @primary="openMessage(item)"
                  @hover="selectItemAt($event)"
                >
                  <template #title>
                    <span>{{ item.group.sessionTitle || t("commandPalette.untitledSession") }}</span>
                  </template>
                  <template #subtitle>
                    <span
                      class="bg-secondary text-muted-foreground mr-1.5 inline-block rounded-[3px] px-[5px] text-[10px] font-semibold"
                      >{{ item.msg.query_content ? "Q" : "A" }}</span
                    >
                    <span v-html="highlight(item.msg.query_content || item.msg.answer_content)" />
                  </template>
                </ResultItem>
              </template>
            </ResultGroup>

            <!-- KB name matches -->
            <ResultGroup v-if="isGroupVisible('kbs') && kbMatches.length" :label="t('commandPalette.group.kbs')">
              <ResultItem
                v-for="(kb, i) in kbMatches"
                :key="'k-' + kb.id"
                :index="flatIndexFor('kbs', i)"
                :selected="selectedIndex === flatIndexFor('kbs', i)"
                :shortcut="shortcutFor(flatIndexFor('kbs', i))"
                icon-name="folder"
                :title="kb.name"
                @primary="openKb(kb.id)"
                @hover="selectItemAt($event)"
              />
            </ResultGroup>

            <!-- Session (chat) title matches -->
            <ResultGroup
              v-if="isGroupVisible('sessions') && sessionMatches.length"
              :label="t('commandPalette.group.sessionsByTitle')"
            >
              <ResultItem
                v-for="(s, i) in sessionMatches"
                :key="'s-' + s.id"
                :index="flatIndexFor('sessions', i)"
                :selected="selectedIndex === flatIndexFor('sessions', i)"
                :shortcut="shortcutFor(flatIndexFor('sessions', i))"
                icon-name="chat"
                :title="s.title"
                @primary="openSession(s.id)"
                @hover="selectItemAt($event)"
              />
            </ResultGroup>

            <!-- Commands matching the query -->
            <ResultGroup
              v-if="isGroupVisible('commands') && filteredCommands.length"
              :label="t('commandPalette.group.commands')"
            >
              <ResultItem
                v-for="(c, i) in filteredCommands"
                :key="'fcmd-' + c.id"
                :index="flatIndexFor('commands', i)"
                :selected="selectedIndex === flatIndexFor('commands', i)"
                :shortcut="shortcutFor(flatIndexFor('commands', i))"
                :icon-name="c.icon"
                :title="c.label"
                @primary="c.run"
                @hover="selectItemAt($event)"
              />
            </ResultGroup>

            <!-- No results -->
            <div
              v-if="!loading && !hasAnyResults && hasSearched"
              class="text-placeholder flex flex-col items-center gap-3 px-5 pt-10 pb-5 text-[13px]"
            >
              <p class="m-0">{{ t("commandPalette.empty.noResults") }}</p>
              <div class="flex gap-2">
                <Button variant="outline" size="sm" class="border-primary text-primary" @click="askAi">
                  <MessageSquareIcon class="size-3.5" />
                  {{ t("commandPalette.empty.askAi") }}
                </Button>
                <Button variant="outline" size="sm" @click="drawerVisible = true">
                  <SettingsIcon class="size-3.5" />
                  {{ t("commandPalette.empty.adjustRetrieval") }}
                </Button>
              </div>
            </div>
          </template>
        </div>

        <!-- Hotkey footer -->
        <div class="border-border text-placeholder flex flex-wrap gap-4 border-t px-3.5 py-2 text-[11px]">
          <span :class="hotkeyClass"
            ><kbd :class="kbdClass">↑</kbd><kbd :class="kbdClass">↓</kbd> {{ t("commandPalette.hotkey.select") }}</span
          >
          <span :class="hotkeyClass"><kbd :class="kbdClass">↵</kbd> {{ t("commandPalette.hotkey.enter") }}</span>
          <span :class="hotkeyClass"
            ><kbd :class="kbdClass">⌘</kbd><kbd :class="kbdClass">1</kbd>-<kbd :class="kbdClass">9</kbd>
            {{ t("commandPalette.hotkey.cmdNumber") }}</span
          >
          <span :class="hotkeyClass"
            ><kbd :class="kbdClass">⌘</kbd><kbd :class="kbdClass">↵</kbd>
            {{ t("commandPalette.hotkey.cmdEnter") }}</span
          >
          <span :class="hotkeyClass"><kbd :class="kbdClass">Esc</kbd> {{ t("commandPalette.hotkey.esc") }}</span>
        </div>
      </div>
    </DialogContent>
  </Dialog>

  <!--
    Retrieval settings drawer, layered on top of the palette. It is a sibling
    of the dialog rather than a child: both portal to <body>, and the one
    opened later stacks above. Reka's layer stack keeps the dialog from
    treating clicks inside the drawer as outside clicks that would close it.
  -->
  <SettingDrawer
    :visible="drawerVisible"
    :title="t('retrievalSettings.title')"
    width="420px"
    :resizable="false"
    storage-key="cmdk-retrieval-drawer:width"
    hide-footer
    @update:visible="(v) => (drawerVisible = v)"
  >
    <RetrievalSettings />
  </SettingDrawer>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import { storeToRefs } from "pinia";
import { useCommandPaletteStore } from "@/stores/commandPalette";
import { useCmdkSearch, type CmdkFileGroup, type CmdkChunk, type CmdkMsgGroup } from "./GlobalCommandPalette/useSearch";
import { highlightText } from "./GlobalCommandPalette/useHighlight";
import { useStartChat } from "./GlobalCommandPalette/useStartChat";
import { buildCommands, filterCommands } from "./GlobalCommandPalette/commands";
import ResultGroup from "./GlobalCommandPalette/ResultGroup.vue";
import ResultItem from "./GlobalCommandPalette/ResultItem.vue";
import RetrievalSettings from "@/views/settings/RetrievalSettings.vue";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { FolderIcon, Loader2Icon, MessageSquareIcon, SearchIcon, SettingsIcon, XIcon } from "@lucide/vue";
import type { MessageSearchGroupItem } from "@/api/chat-history";

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const commandPaletteStore = useCommandPaletteStore();
const { open, initialQuery, recentQueries } = storeToRefs(commandPaletteStore);
const { startChat } = useStartChat();

// Per-group display caps — keep the palette compact.
const CHUNK_LIMIT = 5;
const MSG_LIMIT = 4;

// Active KB scope: when the palette opens on a KB detail page, we default to
// searching within that KB. The user can remove the chip to expand scope.
const activeKbScope = ref<{ id: string; name: string } | null>(null);
const scopeDismissed = ref(false); // user clicked ✕, remember for this session

const {
  query,
  loading,
  hasSearched,
  fileGroups,
  messageGroups,
  kbMatches,
  knowledgeBases,
  sessionMatches,
  totalChunks,
  totalMessages,
  clearResults,
} = useCmdkSearch({
  lockedKbIds: () => (activeKbScope.value ? [activeKbScope.value.id] : []),
});

const drawerVisible = ref(false);
const inputRef = ref<HTMLInputElement | null>(null);
const scrollRef = ref<HTMLElement | null>(null);

// Selection is identity-based (by item key) rather than positional, so
// results arriving asynchronously (e.g. slow KB search) can't silently
// reassign the user's highlight to a different row. `selectedIndex` is
// derived: it's the current position of `selectedKey` in flatItems, or 0 if
// the selected item is gone.
const selectedKey = ref<string | null>(null);

// Proxy the store's `open` to the dialog's open state.
const dialogVisible = computed<boolean>({
  get: () => open.value,
  set: (v) => {
    if (!v) handleClose();
  },
});

// Flatten visible items into one indexable list for keyboard nav.
interface FlatChunkItem {
  kind: "chunk";
  file: CmdkFileGroup;
  chunk: CmdkChunk;
}
interface FlatMsgItem {
  kind: "msg";
  group: CmdkMsgGroup;
  msg: MessageSearchGroupItem;
}

const flatChunkItems = computed<FlatChunkItem[]>(() => {
  const out: FlatChunkItem[] = [];
  for (const f of fileGroups.value) {
    for (const c of f.chunks) {
      out.push({ kind: "chunk", file: f, chunk: c });
      if (out.length >= CHUNK_LIMIT) return out;
    }
  }
  return out;
});

const flatMessageItems = computed<FlatMsgItem[]>(() => {
  const out: FlatMsgItem[] = [];
  for (const g of messageGroups.value) {
    for (const m of g.items) {
      out.push({ kind: "msg", group: g, msg: m });
      if (out.length >= MSG_LIMIT) return out;
    }
  }
  return out;
});

// ─── Commands ───
// All palette commands live in a shared module so quick-action (empty state)
// and the "commands" tab can use the same data.
const allCommands = computed(() => {
  return buildCommands({
    router,
    t,
    close: () => commandPaletteStore.closePalette(),
  });
});

const filteredCommands = computed(() => filterCommands(allCommands.value, query.value));

// Stable index layout — keep group order consistent so flatIndexFor is cheap.
// Groups always render in the same order; individual groups hide themselves
// when their backing list is empty, so users with nothing to show don't see
// empty headers.
const groupOrder = computed<readonly string[]>(() => {
  if (!query.value.trim()) {
    // Empty state: recents + full command list.
    return ["recent", "commands"] as const;
  }
  if (activeKbScope.value) {
    // Scoped to one KB: only chunks make sense (messages disabled in useSearch).
    return ["chunks"] as const;
  }
  return ["chunks", "messages", "kbs", "sessions", "commands"] as const;
});

const groupSizes = computed<Record<string, number>>(() => ({
  recent: recentQueries.value.length,
  commands: query.value.trim() ? filteredCommands.value.length : allCommands.value.length,
  chunks: flatChunkItems.value.length,
  messages: flatMessageItems.value.length,
  kbs: kbMatches.value.length,
  sessions: sessionMatches.value.length,
}));

const flatIndexFor = (group: string, localIndex: number): number => {
  let base = 0;
  for (const g of groupOrder.value) {
    if (g === group) return base + localIndex;
    base += groupSizes.value[g] || 0;
  }
  return base + localIndex;
};

/**
 * Unified flattened list driving keyboard nav, ⌘N shortcuts and the primary
 * action. Each entry carries a stable `key` (so selection survives async
 * result arrivals) plus a `run` callback. Order follows `groupOrder`.
 */
interface FlatItem {
  key: string;
  group: string;
  run: (ev?: { cmd: boolean }) => void;
}

const flatItems = computed<FlatItem[]>(() => {
  const out: FlatItem[] = [];
  for (const g of groupOrder.value) {
    if (g === "recent") {
      recentQueries.value.forEach((q, i) => {
        out.push({
          key: `recent:${i}:${q}`,
          group: g,
          run: () => {
            query.value = q;
          },
        });
      });
    } else if (g === "commands") {
      const list = query.value.trim() ? filteredCommands.value : allCommands.value;
      list.forEach((cmd) => {
        out.push({ key: `cmd:${cmd.id}`, group: g, run: () => cmd.run() });
      });
    } else if (g === "chunks") {
      flatChunkItems.value.forEach((item) => {
        out.push({
          key: `chunk:${item.chunk.id}`,
          group: g,
          run: (ev) => (ev?.cmd ? cmdEnterChunk(item) : openChunk(item)),
        });
      });
    } else if (g === "messages") {
      flatMessageItems.value.forEach((item) => {
        out.push({
          key: `msg:${item.msg.request_id}`,
          group: g,
          run: () => openMessage(item),
        });
      });
    } else if (g === "kbs") {
      kbMatches.value.forEach((kb) => {
        out.push({ key: `kb:${kb.id}`, group: g, run: () => openKb(kb.id) });
      });
    } else if (g === "sessions") {
      sessionMatches.value.forEach((s) => {
        out.push({ key: `session:${s.id}`, group: g, run: () => openSession(s.id) });
      });
    }
  }
  return out;
});

const totalItems = computed(() => flatItems.value.length);

/** Derived: position of the currently-selected item, or 0 if it no longer exists. */
const selectedIndex = computed<number>(() => {
  if (!selectedKey.value) return 0;
  const idx = flatItems.value.findIndex((it) => it.key === selectedKey.value);
  return idx >= 0 ? idx : 0;
});

const hasAnyResults = computed(() => {
  // "Any result" means at least one of the groups visible under the current
  // state has items. The empty-state message should only appear when the user
  // really has nothing to click on.
  for (const g of groupOrder.value) {
    if ((groupSizes.value[g] || 0) > 0) return true;
  }
  return false;
});

/** Whether a named group should be rendered under the current state. */
const isGroupVisible = (name: string): boolean => groupOrder.value.includes(name as never);

// ─── Navigation helpers ───

const primaryActionForSelected = (ev?: { cmd: boolean }) => {
  const item = flatItems.value[selectedIndex.value];
  item?.run(ev);
};

const openChunk = (item: FlatChunkItem) => {
  commandPaletteStore.pushRecent(query.value);
  commandPaletteStore.closePalette();
  if (!item.file.kbId) return;
  const currentKbId = typeof route.params.kbId === "string" ? route.params.kbId : "";
  // If the user is already on this KB page, router.push to the same path+query
  // is a no-op (vue-router dedupes identical navigations), so the document
  // auto-open logic never fires. Dispatch a global event that KnowledgeBase.vue
  // listens for instead; this also avoids reloading the KB list on every click.
  if (currentKbId === item.file.kbId) {
    window.dispatchEvent(
      new CustomEvent("yuheng:open-knowledge", {
        detail: { kbId: item.file.kbId, knowledgeId: item.file.knowledgeId },
      }),
    );
    return;
  }
  router.push({
    path: `/platform/knowledge-bases/${item.file.kbId}`,
    query: { knowledge_id: item.file.knowledgeId },
  });
};

const cmdEnterChunk = (item: FlatChunkItem) => {
  commandPaletteStore.pushRecent(query.value);
  commandPaletteStore.closePalette();
  const kbIds = item.file.kbId ? [item.file.kbId] : [];
  startChat(query.value, kbIds, [item.file.knowledgeId]);
};

const openMessage = (item: FlatMsgItem) => {
  commandPaletteStore.pushRecent(query.value);
  commandPaletteStore.closePalette();
  if (item.group.sessionId) {
    router.push(`/platform/chat/${item.group.sessionId}`);
  }
};

const openKb = (kbId: string) => {
  commandPaletteStore.pushRecent(query.value);
  commandPaletteStore.closePalette();
  router.push(`/platform/knowledge-bases/${kbId}`);
};

const openSession = (sessionId: string) => {
  if (!sessionId) return;
  commandPaletteStore.pushRecent(query.value);
  commandPaletteStore.closePalette();
  router.push(`/platform/chat/${sessionId}`);
};

const askAi = () => {
  if (!query.value.trim()) return;
  commandPaletteStore.pushRecent(query.value);
  commandPaletteStore.closePalette();
  startChat(query.value);
};

const highlight = (text: string) => highlightText(text, query.value);

// ─── Keyboard navigation ───

const scrollSelectedIntoView = () => {
  nextTick(() => {
    const el = scrollRef.value?.querySelector<HTMLElement>(`[data-cmdk-index="${selectedIndex.value}"]`);
    el?.scrollIntoView({ block: "nearest" });
  });
};

const moveSelection = (delta: number) => {
  const total = totalItems.value;
  if (total === 0) return;
  const next = (selectedIndex.value + delta + total) % total;
  selectedKey.value = flatItems.value[next]?.key || null;
  scrollSelectedIntoView();
};

const selectItemAt = (idx: number) => {
  const item = flatItems.value[idx];
  if (!item) return;
  selectedKey.value = item.key;
};

/**
 * Return the ⌘N shortcut digit for the item at the given flat index, or
 * undefined if it's past the 9th slot. The ResultItem renders a kbd hint
 * iff this is defined, which is the only cue the user has that ⌘1-9 works.
 */
const shortcutFor = (flatIndex: number): number | undefined => {
  if (flatIndex < 0 || flatIndex > 8) return undefined;
  return flatIndex + 1;
};

const onKeyDown = (e: KeyboardEvent) => {
  if (e.key === "ArrowDown") {
    e.preventDefault();
    moveSelection(1);
  } else if (e.key === "ArrowUp") {
    e.preventDefault();
    moveSelection(-1);
  } else if ((e.metaKey || e.ctrlKey) && e.key >= "1" && e.key <= "9") {
    // ⌘1-9 — jump straight to the Nth visible item (shortcut badge on each
    // row reveals the binding). No modifier combos with ⌘Enter: digits take
    // precedence because ⌘+digit can't be confused with ⌘+enter.
    const n = parseInt(e.key, 10);
    const item = flatItems.value[n - 1];
    if (item) {
      e.preventDefault();
      item.run({ cmd: false });
    }
  } else if (e.key === "Enter") {
    e.preventDefault();
    primaryActionForSelected({ cmd: e.metaKey || e.ctrlKey });
  } else if (e.key === "Escape") {
    e.preventDefault();
    handleClose();
  }
};

// Keys handled specifically on the input element. Main purpose: let the user
// escape a KB scope chip with the keyboard. Mirrors how chat apps treat
// pill/tag tokens — Backspace on an empty input removes the preceding chip.
const onInputKeyDown = (e: KeyboardEvent) => {
  if (e.key === "Backspace" && !query.value && activeKbScope.value) {
    e.preventDefault();
    clearKbScope();
  }
};

// The footer's hotkey hints. Five spans and a dozen <kbd>s share these, so
// they live here rather than repeated in the template. The kbd resets its
// browser-default monospace font to match the old look.
const hotkeyClass = "inline-flex items-center gap-1";
const kbdClass =
  "bg-secondary border-border text-muted-foreground inline-block min-w-4 rounded-[3px] border px-[5px] py-px text-center font-[inherit] text-[10px] leading-[14px]";

const handleClose = () => {
  drawerVisible.value = false;
  commandPaletteStore.closePalette();
};

// ─── Global ⌘K shortcut ───

const isEditingElement = (el: EventTarget | null): boolean => {
  if (!el) return false;
  const node = el as HTMLElement;
  const tag = (node.tagName || "").toUpperCase();
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true;
  if (node.isContentEditable) return true;
  return false;
};

const onGlobalKey = (e: KeyboardEvent) => {
  const isCmd = e.metaKey || e.ctrlKey;
  if (isCmd && e.key.toLowerCase() === "k") {
    // Always allow ⌘K to open, even from an input.
    e.preventDefault();
    if (open.value) handleClose();
    else commandPaletteStore.openPalette("");
    return;
  }
  // Plain "/" opens the palette when nothing is focused.
  if (e.key === "/" && !open.value && !isEditingElement(e.target)) {
    e.preventDefault();
    commandPaletteStore.openPalette("");
  }
};

// When the palette opens programmatically (from router redirect or elsewhere),
// sync the query and initialize scope. Focus is moved to the input by
// `onDialogOpenAutoFocus` once the dialog content is mounted — doing it here
// in a plain `nextTick` is too early (the portalled input may not be attached
// to the document yet, so `.focus()` silently no-ops).
watch(open, (val) => {
  if (val) {
    query.value = initialQuery.value || "";
    selectedKey.value = null;
    // Infer KB scope from current route. Reset scopeDismissed on each open so
    // the default behavior is "scope to current KB unless user clicks ✕".
    scopeDismissed.value = false;
    const kbIdFromRoute = typeof route.params.kbId === "string" ? route.params.kbId : "";
    if (kbIdFromRoute) {
      // Best-effort name resolution; falls back to short id.
      const match = knowledgeBases.value.find((k) => k.id === kbIdFromRoute);
      activeKbScope.value = {
        id: kbIdFromRoute,
        name: match?.name || kbIdFromRoute,
      };
    } else {
      activeKbScope.value = null;
    }
  } else {
    clearResults();
    query.value = "";
    drawerVisible.value = false;
    activeKbScope.value = null;
  }
});

// Fired by the dialog's focus scope once its content is mounted — DOM is
// guaranteed attached, focus sticks. We also schedule a couple of retries because
// some browsers defer focus when the dialog is mid-layout; costs nothing.
const focusInputWithRetry = () => {
  const tryFocus = () => {
    const el = inputRef.value;
    if (!el) return false;
    el.focus();
    // Move cursor to end so an initialQuery is editable, not overwritten.
    if (typeof el.setSelectionRange === "function" && el.value) {
      const len = el.value.length;
      try {
        el.setSelectionRange(len, len);
      } catch {
        /* not all input types support this */
      }
    }
    return document.activeElement === el;
  };
  if (tryFocus()) return;
  requestAnimationFrame(() => {
    if (tryFocus()) return;
    setTimeout(tryFocus, 50);
  });
};

// Reka would otherwise focus the first tabbable element, which is the scope
// chip's remove button when a KB scope is active; the input is where typing
// belongs, so take over the initial focus.
const onDialogOpenAutoFocus = (e: Event) => {
  e.preventDefault();
  focusInputWithRetry();
};

// Once KB list loads, backfill the scope name if we only had an id at open time.
watch(knowledgeBases, (list) => {
  if (activeKbScope.value && activeKbScope.value.name === activeKbScope.value.id) {
    const match = list.find((k) => k.id === activeKbScope.value!.id);
    if (match) activeKbScope.value = { id: match.id, name: match.name };
  }
});

const clearKbScope = () => {
  activeKbScope.value = null;
  scopeDismissed.value = true;
  nextTick(() => inputRef.value?.focus());
};

onMounted(() => {
  window.addEventListener("keydown", onGlobalKey);
});

onUnmounted(() => {
  window.removeEventListener("keydown", onGlobalKey);
});
</script>
