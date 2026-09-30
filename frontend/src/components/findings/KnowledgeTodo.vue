<template>
  <!-- The caller's knowledge to-do: the knowledge-health findings routed to
       them, across the workspace's knowledge bases. The button shows only
       while there is something to do, like the invitation bell beside it; the
       list is a query, not a notification feed, so a bulk import that finds
       a hundred copies adds to a count rather than to a hundred messages. -->
  <template v-if="openCount > 0">
    <button
      type="button"
      data-slot="knowledge-todo"
      data-testid="knowledge-todo"
      class="bg-card text-muted-foreground hover:bg-secondary hover:text-primary focus-visible:outline-ring relative inline-flex h-8 w-8 cursor-pointer items-center justify-center rounded-[10px] border border-[var(--td-component-stroke)] p-0 shadow-[0_2px_6px_rgba(0,0,0,0.04)] transition-[background-color,color,box-shadow] duration-[180ms] hover:shadow-[0_4px_12px_rgba(0,0,0,0.08)] focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-solid"
      :title="t('knowledgeTodo.tooltip')"
      @click="openDrawer"
    >
      <ListTodoIcon class="size-[18px]" />
      <span
        class="bg-destructive absolute top-1 right-1.5 box-content h-5 min-w-2 translate-x-1/2 -translate-y-1/2 rounded-[10px] px-1.5 text-center text-xs leading-5 text-[var(--td-text-color-anti)]"
        data-testid="knowledge-todo-count"
      >
        {{ openCount > 99 ? "99+" : openCount }}
      </span>
    </button>
  </template>

  <Drawer :open="drawerOpen" swipe-direction="right" @update:open="(v: boolean) => (drawerOpen = v)">
    <!-- z-[2500] lifts the panel over the modals of the platform shell, as
         the other page-level drawers do. -->
    <DrawerContent
      class="z-[2500] data-[swipe-direction=right]:w-[720px] data-[swipe-direction=right]:max-w-[100vw] data-[swipe-direction=right]:rounded-l-none data-[swipe-direction=right]:sm:max-w-[100vw]"
    >
      <header class="border-border flex items-center gap-2.5 border-b px-[18px] py-3.5">
        <div class="bg-primary/10 text-primary flex size-8 shrink-0 items-center justify-center rounded-[9px]">
          <ListTodoIcon class="size-4" />
        </div>
        <div class="min-w-0 flex-1">
          <DrawerTitle class="text-foreground m-0 truncate text-[15px] leading-[1.4] font-semibold">
            {{ t("knowledgeTodo.title") }}
          </DrawerTitle>
          <p class="text-placeholder m-0 text-xs">{{ t("knowledgeTodo.subtitle") }}</p>
        </div>
        <DrawerClose as-child>
          <Button variant="ghost" size="icon-sm" :aria-label="t('common.close')">
            <XIcon />
          </Button>
        </DrawerClose>
      </header>

      <div class="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto px-[18px] py-4" data-testid="knowledge-todo-list">
        <template v-if="loading && !items.length">
          <Skeleton class="h-20 w-full" />
          <Skeleton class="h-20 w-full" />
        </template>
        <Empty v-else-if="!items.length" class="border-border border">
          <EmptyHeader>
            <EmptyMedia variant="icon"><CircleCheckIcon /></EmptyMedia>
            <EmptyTitle>{{ t("knowledgeTodo.empty") }}</EmptyTitle>
          </EmptyHeader>
        </Empty>
        <ul v-else class="m-0 flex list-none flex-col gap-2 p-0">
          <!-- Read here, acted on where the documents are: the knowledge
               base's health tab, with the permissions that apply there. -->
          <FindingItem
            v-for="finding in items"
            :key="finding.id"
            :finding="finding"
            @open-knowledge="() => goTo(finding)"
          >
            <template #actions>
              <Button variant="outline" size="xs" data-testid="knowledge-todo-open" @click="goTo(finding)">
                {{ t("knowledgeTodo.handle") }}
                <ArrowRightIcon />
              </Button>
            </template>
          </FindingItem>
        </ul>
        <Button
          v-if="items.length < total"
          variant="ghost"
          size="sm"
          :disabled="loading"
          data-testid="knowledge-todo-more"
          @click="loadMore"
        >
          {{ t("knowledgeTodo.more", { count: total - items.length }) }}
        </Button>
      </div>
    </DrawerContent>
  </Drawer>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { ArrowRightIcon, CircleCheckIcon, ListTodoIcon, XIcon } from "@lucide/vue";

import { countAssignedFindings, listAssignedFindings, type Finding } from "@/api/findings";
import { Button } from "@/components/ui/button";
import { Drawer, DrawerClose, DrawerContent, DrawerTitle } from "@/components/ui/drawer";
import { Empty, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { useAuthStore } from "@/stores/auth";
import FindingItem from "@/views/knowledge/health/FindingItem.vue";

const { t } = useI18n();
const router = useRouter();
const auth = useAuthStore();

/** The count is checked this often: a to-do list, not a chat. */
const POLL_MS = 2 * 60 * 1000;
const PAGE_SIZE = 20;

const openCount = ref(0);
const drawerOpen = ref(false);
const items = ref<Finding[]>([]);
const total = ref(0);
const page = ref(1);
const loading = ref(false);

let timer: ReturnType<typeof setInterval> | null = null;
let listSeq = 0;

async function refreshCount() {
  try {
    openCount.value = await countAssignedFindings();
  } catch (err) {
    // A corner badge: a failed poll leaves the last count up.
    console.debug("knowledge to-do: count failed", err);
  }
}

async function load(nextPage: number) {
  const seq = ++listSeq;
  loading.value = true;
  try {
    const res = await listAssignedFindings({ status: "open", page: nextPage, page_size: PAGE_SIZE });
    if (seq !== listSeq) return;
    items.value = nextPage === 1 ? res.items : [...items.value, ...res.items];
    total.value = res.total;
    page.value = nextPage;
    openCount.value = res.total;
  } catch (err) {
    if (seq === listSeq) console.debug("knowledge to-do: list failed", err);
  } finally {
    if (seq === listSeq) loading.value = false;
  }
}

function openDrawer() {
  drawerOpen.value = true;
  void load(1);
}

const loadMore = () => void load(page.value + 1);

function goTo(finding: Finding) {
  drawerOpen.value = false;
  void router.push({
    name: "knowledgeBaseDetail",
    params: { kbId: finding.knowledge_base_id },
    query: { tab: "health" },
  });
}

onMounted(() => {
  void refreshCount();
  timer = setInterval(() => void refreshCount(), POLL_MS);
});

// Another workspace, another list.
watch(
  () => auth.effectiveTenantId,
  () => {
    items.value = [];
    void refreshCount();
  },
);

onBeforeUnmount(() => {
  if (timer) clearInterval(timer);
  listSeq++;
});
</script>
