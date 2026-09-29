<template>
  <Popover v-model:open="open" @update:open="onOpenChange">
    <PopoverTrigger as-child>
      <button
        type="button"
        data-slot="docs-bell"
        class="text-muted-foreground hover:bg-accent hover:text-foreground relative inline-flex h-8 w-8 cursor-pointer items-center justify-center rounded-md border-0"
        :aria-label="t('docs.notifications.title')"
      >
        <BellIcon class="size-[18px]" />
        <span
          v-if="unread > 0"
          class="bg-destructive absolute top-0.5 right-0 min-w-[15px] rounded-full px-[3px] text-[10px] leading-[15px] text-white tabular-nums"
        >
          {{ unread > 99 ? "99+" : unread }}
        </span>
      </button>
    </PopoverTrigger>
    <PopoverContent class="w-[min(380px,90vw)] p-0" align="end">
      <section class="flex max-h-[60vh] flex-col text-[13px]" role="dialog" :aria-label="t('docs.notifications.title')">
        <header class="flex items-center gap-2.5 border-b border-[var(--td-component-stroke)] px-3 py-2.5">
          <h3 class="m-0 flex-1 text-sm">{{ t("docs.notifications.title") }}</h3>
          <div class="flex items-center gap-1">
            <Checkbox id="notify-unread-only" v-model="unreadOnly" @update:model-value="reload" />
            <Label for="notify-unread-only" class="text-muted-foreground cursor-pointer text-xs font-normal">{{
              t("docs.notifications.unreadOnly")
            }}</Label>
          </div>
          <button
            type="button"
            class="text-primary cursor-pointer border-0 p-0 text-xs disabled:cursor-default disabled:text-[var(--td-text-color-disabled)]"
            :disabled="unread === 0"
            @click="markAllRead"
          >
            {{ t("docs.notifications.markAllRead") }}
          </button>
        </header>

        <div ref="scroller" class="min-h-0 flex-1 overflow-y-auto pt-1 pb-2" @scroll="onScroll">
          <p v-if="loading && !items.length" class="text-placeholder m-3">{{ t("common.loading") }}</p>
          <p v-else-if="!items.length" class="text-placeholder m-3">{{ t("docs.notifications.empty") }}</p>

          <template v-for="group in groups" :key="group.key">
            <h4 class="text-placeholder m-[8px_12px_4px] text-[11px] font-medium">
              {{ t(`docs.notifications.day.${group.label}`) }}
            </h4>
            <button
              v-for="row in group.items"
              :key="row.id"
              type="button"
              class="flex w-full cursor-pointer gap-2 border-0 px-3 py-2 text-left"
              :class="
                described(row).unread
                  ? 'bg-[var(--td-brand-color-light)] hover:bg-[var(--td-brand-color-light-hover)]'
                  : 'hover:bg-accent'
              "
              @click="openRow(row)"
            >
              <component :is="iconOf(described(row).icon)" class="text-muted-foreground mt-0.5 size-4 flex-none" />
              <span class="flex min-w-0 flex-col gap-0.5">
                <span class="text-foreground leading-[1.4]">{{ described(row).title }}</span>
                <span
                  v-if="described(row).excerpt"
                  class="text-muted-foreground [display:-webkit-box] overflow-hidden text-xs [-webkit-box-orient:vertical] [-webkit-line-clamp:2]"
                >
                  {{ described(row).excerpt }}
                </span>
                <time class="text-placeholder text-[11px] tabular-nums" :datetime="row.created_at">{{
                  when(row)
                }}</time>
              </span>
            </button>
          </template>

          <p v-if="loadingMore" class="text-placeholder m-3">{{ t("common.loading") }}</p>
        </div>
      </section>
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { MessagePlugin } from "tdesign-vue-next";
import { computed, onBeforeUnmount, onMounted, ref, watch, type Component } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

import { BellIcon, MessageSquareIcon, PencilIcon, UserRoundPlusIcon, UsersRoundIcon } from "@lucide/vue";

import { getPageByShortId, listNotifications, markNotificationsRead, type NotificationView } from "@/api/docs";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

import { pageSlug } from "../tree/pageTree";

import { describe, groupByDay, type NotificationLike } from "./describe";

const props = defineProps<{
  /** Bumped by the page's event stream when something new arrives, so the
   * badge is current without polling. */
  revision?: number;
}>();

const { t, locale } = useI18n();
const router = useRouter();

const open = ref(false);
const items = ref<NotificationView[]>([]);
const unread = ref(0);
const loading = ref(false);
const loadingMore = ref(false);
const unreadOnly = ref(false);
const cursor = ref("");
const exhausted = ref(false);
const scroller = ref<HTMLElement | null>(null);

const groups = computed(() => groupByDay(items.value));

/** Memoised per render pass; describe is cheap but called several times per row. */
function described(row: NotificationView) {
  return describe(row as NotificationLike, t as (key: string, values?: Record<string, unknown>) => string);
}

/** The notification kinds speak in tdesign names; this is where they meet lucide. */
const KIND_ICONS: Record<string, Component> = {
  "chat-bubble": MessageSquareIcon,
  "user-arrow-right": UserRoundPlusIcon,
  edit: PencilIcon,
  usergroup: UsersRoundIcon,
  notification: BellIcon,
};

function iconOf(name: string): Component {
  return KIND_ICONS[name] ?? BellIcon;
}

function when(row: NotificationView): string {
  const at = new Date(row.created_at);
  if (Number.isNaN(at.getTime())) return "";
  return at.toLocaleTimeString(locale.value, { hour: "2-digit", minute: "2-digit" });
}

async function load(more = false) {
  if (more && (exhausted.value || loadingMore.value)) return;
  const target = more ? loadingMore : loading;
  target.value = true;
  try {
    const page = await listNotifications({
      unread: unreadOnly.value,
      cursor: more ? cursor.value : undefined,
    });
    items.value = more ? [...items.value, ...page.items] : page.items;
    unread.value = page.unread;
    cursor.value = page.next_cursor ?? "";
    exhausted.value = !page.next_cursor;
  } catch {
    // A bell that cannot load is not worth an error dialogue over the page
    // somebody is reading; the count simply stays as it was.
  } finally {
    target.value = false;
  }
}

function reload() {
  cursor.value = "";
  exhausted.value = false;
  void load();
}

function onOpenChange(visible: boolean) {
  open.value = visible;
  if (visible) reload();
}

function onScroll(event: Event) {
  const el = event.target as HTMLElement;
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 60) void load(true);
}

/**
 * Opening a notification marks it read and goes to the page it is about.
 *
 * Marked read optimistically: the row is already on screen being clicked, and
 * a badge that lags behind the thing somebody just opened is worse than one
 * that is briefly wrong in the other direction.
 */
async function openRow(row: NotificationView) {
  if (!row.read_at) {
    row.read_at = new Date().toISOString();
    unread.value = Math.max(0, unread.value - 1);
    void markNotificationsRead([row.id]).catch(() => {});
  }
  open.value = false;

  // The address comes from the payload, written when the notification was.
  // The page itself is then fetched, which is what answers "still there, and
  // still yours" — and gives the current title, since a page renamed since
  // should be opened under the name it has now.
  const shortId = String(row.payload?.short_id ?? "");
  const slug = String(row.payload?.space_slug ?? "");
  if (!shortId || !slug) return;
  try {
    const page = await getPageByShortId(shortId);
    await router.push({
      name: "docsSpace",
      params: { slug, pageSlug: pageSlug(page.title, page.short_id) },
    });
  } catch {
    // The notification stays: it is still a true record of what happened,
    // even though the page is no longer reachable.
    void MessagePlugin.info(t("docs.notifications.pageGone"));
  }
}

async function markAllRead() {
  try {
    await markNotificationsRead([]);
    for (const row of items.value) row.read_at = row.read_at ?? new Date().toISOString();
    unread.value = 0;
    if (unreadOnly.value) reload();
  } catch (err) {
    void MessagePlugin.error((err as { message?: string })?.message ?? "");
  }
}

// The count is loaded once on mount so the badge is right before anybody
// opens the panel, and again whenever the event stream says something
// arrived.
onMounted(() => void load());
watch(
  () => props.revision,
  () => {
    if (open.value) reload();
    else void load();
  },
);

onBeforeUnmount(() => {
  open.value = false;
});
</script>
