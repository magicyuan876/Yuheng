<template>
  <!--
    The menu is positioned by the parent (Input-field passes a fixed top/left
    through `style`); data-slot opts the subtree into the base resets so the
    bare <button> rows render without browser chrome.
  -->
  <div
    v-if="visible"
    data-slot="mention-menu"
    class="mention-menu border-border bg-card fixed z-[10000] flex max-h-[388px] w-[220px] flex-col overflow-hidden rounded-xl border border-solid shadow-[0_10px_30px_rgba(0,0,0,0.1),0_2px_8px_rgba(0,0,0,0.04)]"
    :style="style"
    ref="menuRef"
    @click.stop
  >
    <div class="min-h-0 flex-auto overflow-y-auto py-1" ref="listRef" @scroll="onScroll">
      <template v-if="!currentGroupType && !isFlatMode">
        <button
          v-for="(group, index) in groupRows"
          :key="group.type"
          type="button"
          :class="[rowButtonClass, 'text-foreground', { 'bg-secondary': index === groupActiveIndex }]"
          @click.stop="enterGroup(group.type)"
          @mouseenter="groupActiveIndex = index"
        >
          <span class="text-muted-foreground inline-flex size-[18px] shrink-0 items-center justify-center">
            <component :is="groupIcons[group.icon]" class="size-4" />
          </span>
          <span class="min-w-0 flex-1 truncate text-left">{{ group.label }}</span>
          <span
            class="bg-secondary text-placeholder min-w-[18px] shrink-0 rounded-full px-1.5 text-center text-xs leading-[18px] tabular-nums"
            >{{ formatGroupCount(group) }}</span
          >
          <ChevronRightIcon class="text-placeholder size-4 shrink-0" />
        </button>
        <div v-if="groupRows.length === 0 && !loading" :class="emptyClass">
          {{ emptyHint || $t("common.noResult") }}
        </div>
      </template>

      <template v-else>
        <!-- The back row is a full-width strip with a rule under it, not a rounded pill. -->
        <button
          v-if="!isFlatMode"
          type="button"
          :class="[
            rowButtonClass,
            'border-border text-muted-foreground mb-1 min-h-[30px] rounded-none border-b border-solid',
          ]"
          @click.stop="leaveGroup"
        >
          <ChevronLeftIcon class="size-4 shrink-0" />
          <span class="min-w-0 flex-1 truncate text-left">{{ currentGroup?.label }}</span>
        </button>

        <div v-if="isFlatMode && groupTabs.length > 1 && kbItems.length > 0" :class="groupHeaderClass">
          {{ $t("common.knowledgeBase") }}
        </div>
        <!-- Knowledge Bases Group -->
        <div
          v-if="(isFlatMode || currentGroupType === 'kb') && kbItems.length > 0"
          :class="groupClass"
          data-group-type="kb"
        >
          <!--
            Each row carries a hover card with the item's details. It opens on
            hover after a short delay, never while the list is scrolling, and
            its content stays hoverable so the links inside can be clicked.
          -->
          <Tooltip
            v-for="(item, index) in kbItems"
            :key="item.id"
            :delay-duration="320"
            :disabled="isScrolling"
            @update:open="(v: boolean) => v && fetchKbDetail(item)"
          >
            <TooltipTrigger as-child>
              <div
                :class="[itemClass, { 'bg-secondary': index === activeIndex }]"
                @click="$emit('select', item)"
                @mouseenter="$emit('update:activeIndex', index)"
              >
                <div class="relative size-[18px] shrink-0">
                  <div
                    :class="[
                      iconClass,
                      index === activeIndex
                        ? item.kbType === 'faq'
                          ? 'text-[var(--yuheng-faq-color,#0052d9)]'
                          : 'text-primary'
                        : 'text-muted-foreground',
                    ]"
                  >
                    <MessageCircleQuestionMarkIcon v-if="item.kbType === 'faq'" class="size-4" />
                    <FolderIcon v-else class="size-4" />
                  </div>
                </div>
                <div class="flex min-w-0 flex-1 items-center gap-1">
                  <span class="truncate font-normal">{{ item.name }}</span>
                  <span class="text-placeholder ml-auto shrink-0 text-xs tabular-nums">{{ item.count || 0 }}</span>
                </div>
              </div>
            </TooltipTrigger>
            <TooltipContent side="right" align="start" :class="detailPopupClass">
              <div :class="detailContentClass">
                <template v-if="detailCache[item.id]?.loading">
                  <div class="py-2"><Loader2Icon class="text-primary size-4 animate-spin" /></div>
                </template>
                <template v-else-if="detailCache[item.id]?.error">
                  <div class="text-destructive py-2 text-xs">{{ detailCache[item.id].error }}</div>
                </template>
                <template v-else-if="detailCache[item.id]?.data">
                  <div :class="detailHeaderClass">
                    <span :class="detailNameClass">{{ detailCache[item.id].data.name }}</span>
                    <span
                      class="shrink-0 rounded-md border border-solid px-1.5 py-px text-xs leading-[18px]"
                      :class="
                        detailCache[item.id].data.type === 'faq'
                          ? 'border-[rgba(0,82,217,0.16)] bg-[rgba(0,82,217,0.08)] text-[var(--yuheng-faq-color,#0052d9)]'
                          : 'border-border bg-secondary text-primary'
                      "
                    >
                      {{
                        detailCache[item.id].data.type === "faq"
                          ? $t("knowledgeEditor.basic.typeFAQ")
                          : $t("knowledgeEditor.basic.typeDocument")
                      }}
                    </span>
                  </div>
                  <p v-if="detailCache[item.id].data.description" :class="detailDescClass">
                    {{ detailCache[item.id].data.description }}
                  </p>
                  <div :class="detailMetaClass">
                    <span v-if="detailCache[item.id].data.type === 'faq'">
                      {{
                        $t("mentionDetail.faqCount", {
                          count: detailCache[item.id].data.chunk_count ?? detailCache[item.id].data.count ?? 0,
                        })
                      }}
                    </span>
                    <span v-else>
                      {{
                        $t("mentionDetail.kbCount", {
                          count: detailCache[item.id].data.knowledge_count ?? detailCache[item.id].data.count ?? 0,
                        })
                      }}
                    </span>
                  </div>
                </template>
              </div>
            </TooltipContent>
          </Tooltip>
        </div>

        <template v-for="group in activeExtraGroups" :key="group.type">
          <div v-if="isFlatMode && groupTabs.length > 1" :class="groupHeaderClass">
            {{ group.label }}
          </div>
          <div :class="groupClass" :data-group-type="group.type">
            <Tooltip
              v-for="(item, index) in group.items"
              :key="`${item.type}:${item.id}`"
              :delay-duration="320"
              :disabled="isScrolling"
            >
              <TooltipTrigger as-child>
                <div
                  :class="[itemClass, { 'bg-secondary': group.offset + index === activeIndex }]"
                  @click="$emit('select', item)"
                  @mouseenter="$emit('update:activeIndex', group.offset + index)"
                >
                  <div class="relative size-[18px] shrink-0">
                    <div
                      :class="[iconClass, group.offset + index === activeIndex ? 'text-primary' : 'text-foreground']"
                    >
                      <component :is="groupIcons[group.icon]" class="size-4" />
                    </div>
                  </div>
                  <div class="flex min-w-0 flex-1 items-center gap-1">
                    <span class="truncate font-normal">{{ item.name }}</span>
                  </div>
                </div>
              </TooltipTrigger>
              <TooltipContent side="right" align="start" :class="detailPopupClass">
                <div :class="detailContentClass">
                  <div :class="detailHeaderClass">
                    <span :class="detailNameClass">{{ item.name }}</span>
                  </div>
                  <p v-if="item.description" :class="detailDescClass">{{ item.description }}</p>
                  <div :class="detailMetaClass">
                    <span v-if="item.kbName" :class="detailLineClass">
                      <FolderIcon class="text-primary mr-0.5 size-3.5 shrink-0" />
                      <span :class="detailLabelClass">{{ $t("mentionDetail.belongsToKb") }}</span>
                      <span :class="detailLinkClass" @click.stop="handleKbClick(item.kbId)">
                        {{ item.kbName }}
                      </span>
                    </span>
                  </div>
                </div>
              </TooltipContent>
            </Tooltip>
          </div>
        </template>

        <div v-if="isFlatMode && groupTabs.length > 1 && fileItems.length > 0" :class="groupHeaderClass">
          {{ $t("common.file") }}
        </div>
        <!-- Files Group -->
        <div
          v-if="(isFlatMode || currentGroupType === 'file') && fileItems.length > 0"
          :class="groupClass"
          data-group-type="file"
        >
          <Tooltip
            v-for="(item, index) in fileItems"
            :key="item.id"
            :delay-duration="320"
            :disabled="isScrolling"
            @update:open="(v: boolean) => v && fetchFileDetail(item)"
          >
            <TooltipTrigger as-child>
              <div
                :class="[itemClass, { 'bg-secondary': fileGroupOffset + index === activeIndex }]"
                @click="$emit('select', item)"
                @mouseenter="$emit('update:activeIndex', fileGroupOffset + index)"
              >
                <div class="relative size-[18px] shrink-0">
                  <div
                    :class="[
                      iconClass,
                      fileGroupOffset + index === activeIndex ? 'text-primary' : 'text-muted-foreground',
                    ]"
                  >
                    <FileIcon class="size-4" />
                  </div>
                </div>
                <span class="min-w-0 flex-1 truncate font-normal">{{ item.name }}</span>
              </div>
            </TooltipTrigger>
            <TooltipContent side="right" align="start" :class="detailPopupClass">
              <div :class="detailContentClass">
                <template v-if="detailCache[item.id]?.loading">
                  <div class="py-2"><Loader2Icon class="text-primary size-4 animate-spin" /></div>
                </template>
                <template v-else-if="detailCache[item.id]?.error">
                  <div class="text-destructive py-2 text-xs">{{ detailCache[item.id].error }}</div>
                </template>
                <template v-else-if="detailCache[item.id]?.data">
                  <div :class="detailHeaderClass">
                    <span :class="detailNameClass">{{
                      detailCache[item.id].data.title || detailCache[item.id].data.file_name || item.name
                    }}</span>
                  </div>
                  <p v-if="detailCache[item.id].data.description" :class="detailDescClass">
                    {{ detailCache[item.id].data.description }}
                  </p>
                  <div :class="detailMetaClass">
                    <span v-if="detailCache[item.id].data.knowledge_base_name || item.kbName" :class="detailLineClass">
                      <FolderIcon class="text-primary mr-0.5 size-3.5 shrink-0" />
                      <span :class="detailLabelClass">{{ $t("mentionDetail.belongsToKb") }}</span>
                      <span
                        :class="detailLinkClass"
                        @click.stop="handleKbClick(detailCache[item.id].data.knowledge_base_id || (item as any).kbId)"
                      >
                        {{ detailCache[item.id].data.knowledge_base_name || item.kbName }}
                      </span>
                    </span>
                  </div>
                </template>
              </div>
            </TooltipContent>
          </Tooltip>
          <!-- Loading indicator -->
          <div v-if="loading" class="flex justify-center px-3 py-2">
            <Loader2Icon class="text-primary size-4 animate-spin" />
          </div>
        </div>

        <div v-if="items.length === 0 && !loading" :class="emptyClass">
          {{ emptyHint || $t("common.noResult") }}
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, watch, ref, nextTick, onBeforeUnmount } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { getKnowledgeBaseById } from "@/api/knowledge-base";
import { getKnowledgeDetails } from "@/api/knowledge-base";
import type { MentionItem, MentionItemType } from "@/types/mention";
import type { Component } from "vue";
import {
  ChevronLeftIcon,
  ChevronRightIcon,
  FileIcon,
  FolderIcon,
  Loader2Icon,
  MessageCircleQuestionMarkIcon,
  TagIcon,
} from "@lucide/vue";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

// The group definitions name their icon as a string (the old TDesign icon
// name); this maps each one onto its lucide component.
const groupIcons: Record<string, Component> = {
  folder: FolderIcon,
  tag: TagIcon,
  file: FileIcon,
};

/*
 * Class lists shared by the repeated rows, kept here once instead of copied
 * into every v-for in the template.
 *
 * A group entry and the back row are full-width buttons inset by 6px, the
 * same shape as an item row.
 */
const rowButtonClass =
  "mx-1.5 my-px box-border flex min-h-8 w-[calc(100%-12px)] cursor-pointer items-center gap-2 rounded-md border-0 bg-transparent px-2 font-(family-name:--app-font-family) text-sm leading-5 font-normal transition-[background] duration-150 ease-in-out hover:bg-secondary";
// .mention-item stays as an unstyled hook: scrollToItem() finds the rows by it.
const itemClass =
  "mention-item mx-1.5 my-px box-border flex min-h-8 cursor-pointer items-center gap-2 rounded-md px-2 py-1 font-(family-name:--app-font-family) text-sm text-foreground transition-[background] duration-150 ease-in-out hover:bg-secondary";
const iconClass = "flex size-[18px] shrink-0 items-center justify-center bg-transparent";
// Groups after the first are divided by a hairline.
const groupClass = "pt-0.5 pb-[5px] not-last:border-b not-last:border-solid not-last:border-border";
const groupHeaderClass = "px-3.5 pt-[7px] pb-[5px] text-xs leading-[18px] font-semibold text-placeholder";
const emptyClass = "px-4 py-7 text-center text-sm text-placeholder";

/*
 * The detail card beside a hovered row. TooltipContent is a dark bubble by
 * default; this turns it into a light card, and hides its arrow (the old
 * popup had none). The arrow is the only svg that is a direct grandchild of
 * the content root, so `[&>span>svg]` cannot reach the icons inside the card.
 */
const detailPopupClass =
  "z-[10001] block w-auto max-w-[280px] min-w-[220px] rounded-lg border border-solid border-border bg-card px-3.5 py-[13px] text-foreground shadow-[0_10px_28px_rgba(0,0,0,0.1),0_2px_8px_rgba(0,0,0,0.04)] [&>span>svg]:hidden";
const detailContentClass = "text-xs leading-normal text-foreground";
const detailHeaderClass = "mb-2 flex flex-wrap items-center gap-2";
const detailNameClass = "text-sm leading-5 font-semibold break-words";
const detailDescClass = "m-0 mb-2 line-clamp-4 text-xs leading-normal break-words text-muted-foreground";
const detailMetaClass = "flex flex-col items-start gap-[5px] text-xs text-placeholder";
const detailLineClass = "inline-flex w-full items-center gap-1 leading-normal";
const detailLabelClass = "inline-flex shrink-0 items-center leading-normal text-placeholder";
// The knowledge-base name is a link: underlined, brand-coloured on hover.
const detailLinkClass =
  "inline-flex max-w-[160px] cursor-pointer items-center truncate leading-normal underline decoration-placeholder transition-[color,text-decoration-color] duration-200 hover:text-primary hover:decoration-primary";

type DetailState = { loading: boolean; error?: string; data?: any };

const props = defineProps<{
  visible: boolean;
  style: any;
  items: MentionItem[];
  activeIndex: number;
  hasMore?: boolean;
  loading?: boolean;
  // 空态下替换默认 "无结果" 文案，用于给上游（如"被智能体工具兼容性过滤掉了"）透传具体原因
  emptyHint?: string;
  // 输入 @ 后的筛选关键词；非空时平铺展示匹配项，不再要求进入二级目录
  query?: string;
  // 分组入口展示用的总数（如文件搜索的 total），避免仅用首屏已加载条数
  groupCounts?: Partial<Record<MentionItemType, number>>;
}>();

const emit = defineEmits(["select", "update:activeIndex", "loadMore"]);

const router = useRouter();
const { t } = useI18n();
const menuRef = ref<HTMLElement | null>(null);
const listRef = ref<HTMLElement | null>(null);
const detailCache = ref<Record<string, DetailState>>({});
const isScrolling = ref(false);
const currentGroupType = ref<MentionItemType | null>(null);
const groupActiveIndex = ref(0);
let scrollTimer: ReturnType<typeof setTimeout> | null = null;

onBeforeUnmount(() => {
  if (scrollTimer) clearTimeout(scrollTimer);
});

const kbItems = computed(() => props.items.filter((item) => item.type === "kb"));
const fileItems = computed(() => props.items.filter((item) => item.type === "file"));

const mentionGroupDefs = computed<Array<{ type: MentionItemType; label: string; icon: string }>>(() => [
  { type: "kb", label: t("common.knowledgeBase"), icon: "folder" },
  { type: "tag", label: "标签", icon: "tag" },
  { type: "file", label: t("common.file"), icon: "file" },
]);

const mentionGroups = computed(() => {
  let offset = 0;
  return mentionGroupDefs.value.map((def) => {
    const items = props.items.filter((item) => item.type === def.type);
    const loadedCount = items.length;
    const count = props.groupCounts?.[def.type] ?? loadedCount;
    const group = { ...def, items, offset, count, loadedCount };
    offset += items.length;
    return group;
  });
});

const formatGroupCount = (group: { type: MentionItemType; count: number; loadedCount: number }) => {
  if (props.groupCounts?.[group.type] != null) {
    return props.groupCounts[group.type]!;
  }
  if (group.type === "file" && props.hasMore) {
    return `${group.loadedCount}+`;
  }
  return group.count;
};

const groupTabs = computed(() => mentionGroups.value.filter((group) => group.count > 0));
const groupRows = computed(() => groupTabs.value);
const isFlatMode = computed(() => (props.query ?? "").trim().length > 0);
const currentGroup = computed(() => mentionGroups.value.find((group) => group.type === currentGroupType.value));
const extraGroups = computed(() =>
  mentionGroups.value.filter((group) => group.type !== "kb" && group.type !== "file" && group.count > 0),
);
const activeExtraGroups = computed(() => {
  if (isFlatMode.value) return extraGroups.value;
  return extraGroups.value.filter((group) => group.type === currentGroupType.value);
});
const fileGroupOffset = computed(() => mentionGroups.value.find((group) => group.type === "file")?.offset || 0);

const enterGroup = (type: MentionItemType) => {
  const group = mentionGroups.value.find((item) => item.type === type && item.count > 0);
  if (!group || !listRef.value) return;

  currentGroupType.value = type;
  emit("update:activeIndex", group.offset);

  nextTick(() => {
    if (!listRef.value) return;
    listRef.value.scrollTo({
      top: 0,
    });
  });
};

const leaveGroup = () => {
  if (isFlatMode.value) return false;
  if (!currentGroupType.value) return false;
  const rowIndex = groupRows.value.findIndex((group) => group.type === currentGroupType.value);
  groupActiveIndex.value = Math.max(0, rowIndex);
  currentGroupType.value = null;
  nextTick(() => {
    if (listRef.value) listRef.value.scrollTop = 0;
  });
  return true;
};

const updateActiveGroupFromIndex = (index: number) => {
  const group = groupTabs.value.find((item) => index >= item.offset && index < item.offset + item.count);
  if (group) currentGroupType.value = group.type;
};

watch(groupTabs, (groups) => {
  if (groupActiveIndex.value >= groups.length) {
    groupActiveIndex.value = Math.max(0, groups.length - 1);
  }
});

const moveActive = (delta: number) => {
  if (isFlatMode.value) {
    const next = Math.min(props.items.length - 1, Math.max(0, props.activeIndex + delta));
    emit("update:activeIndex", next);
    scrollToItem(next);
    return;
  }

  if (!currentGroupType.value) {
    const maxIndex = Math.max(0, groupRows.value.length - 1);
    groupActiveIndex.value = Math.min(maxIndex, Math.max(0, groupActiveIndex.value + delta));
    return;
  }

  const group = currentGroup.value;
  if (!group) return;
  const currentLocalIndex = props.activeIndex - group.offset;
  const nextLocalIndex = Math.min(group.count - 1, Math.max(0, currentLocalIndex + delta));
  emit("update:activeIndex", group.offset + nextLocalIndex);
  scrollToItem(nextLocalIndex);
};

const confirmActive = () => {
  if (isFlatMode.value) {
    const item = props.items[props.activeIndex];
    if (item) emit("select", item);
    return;
  }

  if (!currentGroupType.value) {
    const group = groupRows.value[groupActiveIndex.value];
    if (group) enterGroup(group.type);
    return;
  }

  const group = currentGroup.value;
  if (!group) return;
  const localIndex = props.activeIndex - group.offset;
  const item = group.items[localIndex];
  if (item) emit("select", item);
};

defineExpose({
  moveActive,
  confirmActive,
  leaveGroup,
});

async function fetchKbDetail(item: { id: string }) {
  if (detailCache.value[item.id]?.data || detailCache.value[item.id]?.loading) return;
  detailCache.value = { ...detailCache.value, [item.id]: { loading: true } };
  try {
    const res = await getKnowledgeBaseById(item.id);
    detailCache.value = { ...detailCache.value, [item.id]: { loading: false, data: res.data } };
  } catch (e: any) {
    detailCache.value = { ...detailCache.value, [item.id]: { loading: false, error: e?.message || "Failed to load" } };
  }
}

async function fetchFileDetail(item: { id: string }) {
  if (detailCache.value[item.id]?.data || detailCache.value[item.id]?.loading) return;
  detailCache.value = { ...detailCache.value, [item.id]: { loading: true } };
  try {
    const res = await getKnowledgeDetails(item.id);
    detailCache.value = { ...detailCache.value, [item.id]: { loading: false, data: res.data } };
  } catch (e: any) {
    detailCache.value = { ...detailCache.value, [item.id]: { loading: false, error: e?.message || "Failed to load" } };
  }
}

function handleKbClick(kbId: string | undefined) {
  if (!kbId) return;
  router.push(`/platform/knowledge-bases/${kbId}`);
}

const onScroll = (e: Event) => {
  isScrolling.value = true;
  if (scrollTimer) clearTimeout(scrollTimer);
  scrollTimer = setTimeout(() => {
    isScrolling.value = false;
  }, 150);

  const target = e.target as HTMLElement;
  const { scrollTop, scrollHeight, clientHeight } = target;
  if (
    (currentGroupType.value === "file" || isFlatMode.value) &&
    scrollHeight - scrollTop - clientHeight < 50 &&
    props.hasMore &&
    !props.loading
  ) {
    emit("loadMore");
  }
};

watch(
  () => props.activeIndex,
  (newIndex) => {
    if (isFlatMode.value) {
      scrollToItem(newIndex);
      return;
    }
    if (currentGroupType.value) {
      updateActiveGroupFromIndex(newIndex);
      const group = currentGroup.value;
      if (group) scrollToItem(newIndex - group.offset);
    }
  },
);

watch(isFlatMode, (flat) => {
  if (flat) {
    currentGroupType.value = null;
    nextTick(() => {
      if (listRef.value) listRef.value.scrollTop = 0;
    });
  }
});

watch(
  () => props.visible,
  (newVisible) => {
    if (newVisible) {
      nextTick(() => {
        if (listRef.value) listRef.value.scrollTop = 0;
        currentGroupType.value = null;
        groupActiveIndex.value = 0;
      });
    }
  },
);

const scrollToItem = (index: number) => {
  nextTick(() => {
    if (!listRef.value) return;

    const items = listRef.value.querySelectorAll(".mention-item");
    if (!items || items.length <= index) return;

    const activeItem = items[index] as HTMLElement;
    const menu = listRef.value;

    if (activeItem) {
      const menuRect = menu.getBoundingClientRect();
      const itemRect = activeItem.getBoundingClientRect();

      // 检查是否在上方被遮挡
      if (itemRect.top < menuRect.top) {
        menu.scrollTop -= menuRect.top - itemRect.top;
      }
      // 检查是否在下方被遮挡
      else if (itemRect.bottom > menuRect.bottom) {
        menu.scrollTop += itemRect.bottom - menuRect.bottom;
      }
    }
  });
};
</script>
