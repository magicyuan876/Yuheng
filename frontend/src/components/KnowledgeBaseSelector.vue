<template>
  <!--
    A full-screen transparent overlay catches the outside click; the panel
    itself is fixed-positioned by JS (dropdownStyle), so only its inner
    styling lives here. data-slot opts the whole subtree into the base
    resets (border-box, bare controls without browser chrome).
  -->
  <div v-if="visible" data-slot="kb-selector" class="fixed inset-0 z-[9999] touch-none bg-transparent" @click="close">
    <div
      class="animate-in border-border bg-card fade-in-0 zoom-in-98 fixed! z-[10000] m-0 flex origin-top-left flex-col overflow-hidden rounded-[10px] border-[0.5px] border-solid shadow-[var(--td-shadow-2)] duration-150 ease-out"
      @click.stop
      @wheel.stop
      :style="dropdownStyle"
    >
      <!-- 搜索 -->
      <div class="border-border border-b-[0.5px] border-solid px-2.5 py-2">
        <input
          ref="searchInput"
          v-model="searchQuery"
          type="text"
          data-slot="kb-search-input"
          :placeholder="$t('knowledgeBase.searchPlaceholder')"
          class="border-border bg-secondary focus:border-success focus:bg-card w-full rounded-[6px] border-[0.5px] border-solid px-2.5 py-1.5 text-xs transition-[border] duration-[120ms] outline-none"
          @keydown.down.prevent="moveSelection(1)"
          @keydown.up.prevent="moveSelection(-1)"
          @keydown.enter.prevent="toggleSelection"
          @keydown.esc="close"
        />
      </div>

      <!-- 列表 -->
      <div
        class="max-h-[260px] min-h-0 flex-1 overflow-y-auto overscroll-contain px-2 py-1.5 [-webkit-overflow-scrolling:touch]"
        ref="kbList"
        @wheel.stop
      >
        <!--
          .kb-item stays as an unstyled hook: moveSelection() finds the rows
          by it to scroll the highlighted one into view. A selected row keeps
          its tint on hover, as before, so hover only applies to the others.
        -->
        <div
          v-for="(kb, index) in filteredKnowledgeBases"
          :key="kb.id"
          class="kb-item mb-1 flex cursor-pointer items-center rounded-[6px] px-2 py-1.5 transition-[background] duration-[120ms] last:mb-0"
          :class="{
            'bg-[var(--td-brand-color-light)]': isSelected(kb.id),
            'bg-secondary': !isSelected(kb.id) && highlightedIndex === index,
            'hover:bg-secondary': !isSelected(kb.id) && highlightedIndex !== index,
          }"
          @click="toggleKb(kb.id)"
          @mouseenter="highlightedIndex = index"
        >
          <div class="flex w-full items-center gap-2">
            <div
              class="flex size-4 shrink-0 items-center justify-center rounded-[3px] border-[1.5px] border-solid"
              :class="isSelected(kb.id) ? 'border-success bg-success' : 'border-border'"
            >
              <svg v-if="isSelected(kb.id)" class="size-2.5" width="12" height="12" viewBox="0 0 12 12" fill="none">
                <path
                  d="M10 3L4.5 8.5L2 6"
                  stroke="#fff"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
            </div>
            <div
              class="flex size-4 shrink-0 items-center justify-center"
              :class="kb.type === 'faq' ? 'text-primary' : 'text-[var(--td-brand-color-active)]'"
            >
              <svg v-if="kb.type === 'faq'" width="14" height="14" viewBox="0 0 24 24" fill="none">
                <path
                  d="M12 22C17.5228 22 22 17.5228 22 12C22 6.47715 17.5228 2 12 2C6.47715 2 2 6.47715 2 12C2 17.5228 6.47715 22 12 22Z"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
                <path
                  d="M9 9C9 7.89543 9.89543 7 11 7H13C14.1046 7 15 7.89543 15 9C15 10.1046 14.1046 11 13 11H12V14"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
                <circle cx="12" cy="17" r="1" fill="currentColor" />
              </svg>
              <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none">
                <path
                  d="M22 19C22 19.5304 21.7893 20.0391 21.4142 20.4142C21.0391 20.7893 20.5304 21 20 21H4C3.46957 21 2.96086 20.7893 2.58579 20.4142C2.21071 20.0391 2 19.5304 2 19V5C2 4.46957 2.21071 3.96086 2.58579 3.58579C2.96086 3.21071 3.46957 3 4 3H9L11 6H20C20.5304 6 21.0391 6.21071 21.4142 6.58579C21.7893 6.96086 22 7.46957 22 8V19Z"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
            </div>
            <div class="flex min-w-0 flex-row items-center gap-1">
              <span class="text-foreground truncate text-xs leading-[1.4]">{{ kb.name }}</span>
              <span class="text-placeholder shrink-0 text-[11px]"
                >({{ kb.type === "faq" ? kb.chunk_count || 0 : kb.knowledge_count || 0 }})</span
              >
            </div>
          </div>
        </div>

        <div v-if="filteredKnowledgeBases.length === 0" class="text-placeholder px-2 py-5 text-center text-xs">
          {{ searchQuery ? $t("knowledgeBase.noMatch") : $t("knowledgeBase.noKnowledge") }}
        </div>
      </div>

      <!-- 底部操作 -->
      <div class="border-border bg-secondary flex gap-2 border-t border-solid px-2.5 py-2">
        <button type="button" :class="actionButtonClass" @click="selectAll">{{ $t("common.selectAll") }}</button>
        <button type="button" :class="actionButtonClass" @click="clearAll">{{ $t("common.clear") }}</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from "vue";
import { useSettingsStore } from "@/stores/settings";
import { listKnowledgeBases } from "@/api/knowledge-base";
import { useI18n } from "vue-i18n";
import { getRootZoom, rectToCssPx, cssViewportSize } from "@/utils/zoom";

interface KnowledgeBase {
  id: string;
  name: string;
  type?: "document" | "faq";
  knowledge_count?: number;
  chunk_count?: number;
  embedding_model_id?: string;
  summary_model_id?: string;
}

const { t } = useI18n();

// "Select all" / "Clear" in the footer: equal-width outlined buttons that
// pick up the success colour on hover.
const actionButtonClass =
  "flex-1 cursor-pointer rounded-[6px] border border-solid border-border bg-card px-2.5 py-1.5 text-xs text-muted-foreground transition-all duration-[120ms] hover:border-success hover:bg-[var(--td-brand-color-light)] hover:text-success";

const props = defineProps<{
  visible: boolean;
  anchorEl?: any | null; // 支持 DOM 节点、ref、组件实例
  dropdownWidth?: number;
  offsetY?: number;
}>();

const emit = defineEmits(["close", "update:visible"]);

const settingsStore = useSettingsStore();

// 本地状态
const searchQuery = ref("");
const highlightedIndex = ref(0);
const knowledgeBases = ref<KnowledgeBase[]>([]);
const searchInput = ref<HTMLInputElement | null>(null);
const kbList = ref<HTMLElement | null>(null);
const dropdownStyle = ref<Record<string, string>>({});

// props 默认
const dropdownWidth = props.dropdownWidth ?? 300;
const offsetY = props.offsetY ?? 8;

// 过滤：只显示已初始化（有 embedding & summary）的
const filteredKnowledgeBases = computed(() => {
  const valid = knowledgeBases.value.filter((k) => k.embedding_model_id && k.summary_model_id);
  if (!searchQuery.value) return valid;
  const q = searchQuery.value.toLowerCase();
  return valid.filter((k) => k.name.toLowerCase().includes(q));
});

const selectedKbIds = computed(() => settingsStore.settings.selectedKnowledgeBases || []);

// helper: 从 props.anchorEl 获取真实 DOM 元素（支持多种传入形式）
const resolveAnchorEl = () => {
  const a = props.anchorEl;
  if (!a) return null;
  // 如果是 Vue ref：取 .value
  if (typeof a === "object" && "value" in a) {
    return a.value ?? null;
  }
  // 如果是组件实例（可能有 $el）
  if (typeof a === "object" && "$el" in a) {
    return a.$el ?? null;
  }
  // 直接 DOM 节点或 DOMRect
  return a;
};

const isSelected = (id: string) => selectedKbIds.value.includes(id);

const toggleKb = (id: string) => {
  if (isSelected(id)) {
    settingsStore.removeKnowledgeBase(id);
  } else {
    settingsStore.addKnowledgeBase(id);
  }
};

const toggleSelection = () => {
  const kb = filteredKnowledgeBases.value[highlightedIndex.value];
  if (kb) toggleKb(kb.id);
};

const moveSelection = (dir: number) => {
  const max = filteredKnowledgeBases.value.length;
  if (max === 0) return;
  highlightedIndex.value = Math.max(0, Math.min(max - 1, highlightedIndex.value + dir));
  nextTick(() => {
    const items = kbList.value?.querySelectorAll(".kb-item");
    items?.[highlightedIndex.value]?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  });
};

const selectAll = () => settingsStore.selectKnowledgeBases(filteredKnowledgeBases.value.map((k) => k.id));
const clearAll = () => settingsStore.clearKnowledgeBases();

const close = () => {
  emit("update:visible", false);
  emit("close");
};

const loadKnowledgeBases = async () => {
  try {
    const res: any = await listKnowledgeBases();
    if (res?.data && Array.isArray(res.data)) knowledgeBases.value = res.data;
  } catch (e) {
    console.error(t("knowledgeBase.loadingFailed"), e);
  }
};

// 计算下拉位置：水平居中对齐到按钮中点，处理视口边界
const updateDropdownPosition = () => {
  const anchor = resolveAnchorEl();

  // Cache root zoom for this update. Both fallback and rect-anchored paths
  // need to convert visual measurements to CSS pixels (see utils/zoom.ts).
  const zoom = getRootZoom();
  const { width: vwFallback, height: vhFallback } = cssViewportSize(zoom);

  // fallback 函数
  const applyFallback = () => {
    const topFallback = Math.max(80, vhFallback / 2 - 160);
    dropdownStyle.value = {
      position: "fixed",
      width: `${dropdownWidth}px`,
      left: `${Math.round((vwFallback - dropdownWidth) / 2)}px`,
      top: `${Math.round(topFallback)}px`,
      transform: "none",
      margin: "0",
      padding: "0",
    };
  };

  if (!anchor) {
    applyFallback();
    return;
  }

  // 获取 anchor 的 bounding rect（相对于视口）
  let rawRect: { top: number; left: number; right: number; bottom: number; width: number; height: number } | null =
    null;
  try {
    if (typeof anchor.getBoundingClientRect === "function") {
      const r = anchor.getBoundingClientRect();
      rawRect = { top: r.top, left: r.left, right: r.right, bottom: r.bottom, width: r.width, height: r.height };
    } else if (anchor.width !== undefined && anchor.left !== undefined) {
      // Already a DOMRect-like
      rawRect = anchor as DOMRect;
    }
  } catch (e) {
    console.error("[KnowledgeBaseSelector] Error getting bounding rect:", e);
  }

  if (!rawRect || rawRect.width === 0 || rawRect.height === 0) {
    applyFallback();
    return;
  }

  // Convert to CSS pixels so subsequent comparisons against the dropdown's
  // own width/height stay in one coordinate system.
  const rect = rectToCssPx(rawRect, zoom);
  console.log("[KB Selector] Button rect (css px):", rect);
  const vw = vwFallback;
  const vh = vhFallback;

  // 左对齐到触发元素的左边缘
  // 使用 Math.floor 而不是 Math.round，避免像素对齐问题
  let left = Math.floor(rect.left);

  // 边界处理：不超出视口左右（留 16px margin）
  const minLeft = 16;
  const maxLeft = Math.max(16, vw - dropdownWidth - 16);
  left = Math.max(minLeft, Math.min(maxLeft, left));

  // 垂直定位：紧贴按钮，使用合理的高度避免空白
  const preferredDropdownHeight = 280; // 优选高度（紧凑且够用）
  const minDropdownHeight = 200; // 最小高度
  const topMargin = 20; // 顶部留白
  const spaceBelow = vh - rect.bottom; // 下方剩余空间
  const spaceAbove = rect.top; // 上方剩余空间

  console.log("[KB Selector] Space check:", {
    spaceBelow,
    spaceAbove,
    windowHeight: vh,
  });

  let actualHeight: number;
  let shouldOpenBelow: boolean;

  // 优先考虑下方空间
  if (spaceBelow >= minDropdownHeight + offsetY) {
    // 下方有足够空间，向下弹出
    actualHeight = Math.min(preferredDropdownHeight, spaceBelow - offsetY - 16);
    shouldOpenBelow = true;
    console.log("[KB Selector] Position: below button", { actualHeight });
  } else {
    // 向上弹出，优先使用 preferredHeight，必要时才扩展到 maxHeight
    const availableHeight = spaceAbove - offsetY - topMargin;
    if (availableHeight >= preferredDropdownHeight) {
      // 有足够空间显示优选高度
      actualHeight = preferredDropdownHeight;
    } else {
      // 空间不够，使用可用空间（但不小于最小高度）
      actualHeight = Math.max(minDropdownHeight, availableHeight);
    }
    shouldOpenBelow = false;
    console.log("[KB Selector] Position: above button", { actualHeight });
  }

  // 根据弹出方向使用不同的定位方式
  if (shouldOpenBelow) {
    // 向下弹出：使用 top 定位
    const top = Math.floor(rect.bottom + offsetY);
    console.log("[KB Selector] Opening below, top:", top);
    dropdownStyle.value = {
      position: "fixed",
      width: `${dropdownWidth}px`,
      left: `${left}px`,
      top: `${top}px`,
      maxHeight: `${actualHeight}px`,
      transform: "none",
      margin: "0",
      padding: "0",
    };
  } else {
    // 向上弹出：使用 bottom 定位
    const bottom = vh - rect.top + offsetY;
    console.log("[KB Selector] Opening above, bottom:", bottom);
    dropdownStyle.value = {
      position: "fixed",
      width: `${dropdownWidth}px`,
      left: `${left}px`,
      bottom: `${bottom}px`,
      maxHeight: `${actualHeight}px`,
      transform: "none",
      margin: "0",
      padding: "0",
    };
  }
};

// 事件监听器引用，用于清理
let resizeHandler: (() => void) | null = null;
let scrollHandler: (() => void) | null = null;

// 当 visible 变化时处理
watch(
  () => props.visible,
  async (v) => {
    if (v) {
      await loadKnowledgeBases();
      // 等 DOM 渲染完再计算位置
      await nextTick();
      // 多次更新位置确保准确
      requestAnimationFrame(() => {
        updateDropdownPosition();
        requestAnimationFrame(() => {
          updateDropdownPosition();
          setTimeout(() => {
            updateDropdownPosition();
          }, 50);
        });
      });
      // 确保 focus
      nextTick(() => searchInput.value?.focus());
      // 监听 resize/scroll 做微调（使用 passive 提高性能）
      resizeHandler = () => updateDropdownPosition();
      scrollHandler = () => updateDropdownPosition();
      window.addEventListener("resize", resizeHandler, { passive: true });
      window.addEventListener("scroll", scrollHandler, { passive: true, capture: true });
    } else {
      searchQuery.value = "";
      highlightedIndex.value = 0;
      // 清理事件监听器
      if (resizeHandler) {
        window.removeEventListener("resize", resizeHandler);
        resizeHandler = null;
      }
      if (scrollHandler) {
        window.removeEventListener("scroll", scrollHandler, { capture: true });
        scrollHandler = null;
      }
    }
  },
);
</script>
