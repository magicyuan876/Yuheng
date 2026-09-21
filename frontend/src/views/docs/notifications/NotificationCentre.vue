<template>
  <t-popup v-model:visible="open" trigger="click" placement="bottom-right" :overlay-style="{ padding: 0 }">
    <button type="button" class="docs-bell" :aria-label="t('docs.notifications.title')" @click="onOpen">
      <t-icon name="notification" size="18px" />
      <span v-if="unread > 0" class="docs-bell-badge">{{ unread > 99 ? "99+" : unread }}</span>
    </button>

    <template #content>
      <section class="docs-notify" role="dialog" :aria-label="t('docs.notifications.title')">
        <header class="docs-notify-head">
          <h3>{{ t("docs.notifications.title") }}</h3>
          <label class="docs-notify-filter">
            <input v-model="unreadOnly" type="checkbox" @change="reload" />
            {{ t("docs.notifications.unreadOnly") }}
          </label>
          <button type="button" class="docs-notify-action" :disabled="unread === 0" @click="markAllRead">
            {{ t("docs.notifications.markAllRead") }}
          </button>
        </header>

        <div ref="scroller" class="docs-notify-body" @scroll="onScroll">
          <p v-if="loading && !items.length" class="docs-notify-note">{{ t("common.loading") }}</p>
          <p v-else-if="!items.length" class="docs-notify-note">{{ t("docs.notifications.empty") }}</p>

          <template v-for="group in groups" :key="group.key">
            <h4 class="docs-notify-day">{{ t(`docs.notifications.day.${group.label}`) }}</h4>
            <button
              v-for="row in group.items"
              :key="row.id"
              type="button"
              class="docs-notify-row"
              :class="{ 'is-unread': described(row).unread }"
              @click="openRow(row)"
            >
              <t-icon :name="described(row).icon" size="16px" class="docs-notify-icon" />
              <span class="docs-notify-text">
                <span class="docs-notify-title">{{ described(row).title }}</span>
                <span v-if="described(row).excerpt" class="docs-notify-excerpt">
                  {{ described(row).excerpt }}
                </span>
                <time class="docs-notify-when" :datetime="row.created_at">{{ when(row) }}</time>
              </span>
            </button>
          </template>

          <p v-if="loadingMore" class="docs-notify-note">{{ t("common.loading") }}</p>
        </div>
      </section>
    </template>
  </t-popup>
</template>

<script setup lang="ts">
import { MessagePlugin } from "tdesign-vue-next";
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

import { getPageByShortId, listNotifications, markNotificationsRead, type NotificationView } from "@/api/docs";

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

function onOpen() {
  if (!open.value) reload();
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

<style scoped lang="less">
.docs-bell {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--td-text-color-secondary);
  cursor: pointer;

  &:hover {
    background: var(--td-bg-color-container-hover);
    color: var(--td-text-color-primary);
  }
}

.docs-bell-badge {
  position: absolute;
  top: 2px;
  right: 0;
  min-width: 15px;
  padding: 0 3px;
  border-radius: 999px;
  background: var(--td-error-color);
  color: #fff;
  font-size: 10px;
  line-height: 15px;
  font-variant-numeric: tabular-nums;
}

.docs-notify {
  width: min(380px, 90vw);
  max-height: 60vh;
  display: flex;
  flex-direction: column;
  font-size: 13px;
}

.docs-notify-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--td-component-stroke);

  h3 {
    margin: 0;
    flex: 1;
    font-size: 14px;
  }
}

.docs-notify-filter {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
  cursor: pointer;
}

.docs-notify-action {
  border: none;
  background: transparent;
  padding: 0;
  color: var(--td-brand-color);
  font-size: 12px;
  cursor: pointer;

  &:disabled {
    color: var(--td-text-color-disabled);
    cursor: default;
  }
}

.docs-notify-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 4px 0 8px;
}

.docs-notify-note {
  margin: 12px;
  color: var(--td-text-color-placeholder);
}

.docs-notify-day {
  margin: 8px 12px 4px;
  font-size: 11px;
  font-weight: 500;
  color: var(--td-text-color-placeholder);
}

.docs-notify-row {
  display: flex;
  gap: 8px;
  width: 100%;
  padding: 8px 12px;
  border: none;
  background: transparent;
  text-align: left;
  cursor: pointer;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }

  &.is-unread {
    background: var(--td-brand-color-light);

    &:hover {
      background: var(--td-brand-color-light-hover);
    }
  }
}

.docs-notify-icon {
  flex: none;
  margin-top: 2px;
  color: var(--td-text-color-secondary);
}

.docs-notify-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.docs-notify-title {
  color: var(--td-text-color-primary);
  line-height: 1.4;
}

.docs-notify-excerpt {
  color: var(--td-text-color-secondary);
  font-size: 12px;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.docs-notify-when {
  color: var(--td-text-color-placeholder);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}
</style>
