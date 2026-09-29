<template>
  <div class="flex flex-col gap-3">
    <p class="text-muted-foreground m-0 text-[13px] leading-5">{{ t("docs.trash.subtitle") }}</p>
    <div class="flex items-center justify-between">
      <Button variant="ghost" size="sm" :disabled="loading" @click="load">
        <Loader2Icon v-if="loading" class="animate-spin" />
        <RefreshCwIcon v-else />
        {{ t("common.refresh") }}
      </Button>
      <Button
        v-if="isAdmin && entries.length"
        variant="outline"
        size="sm"
        class="border-destructive text-destructive hover:bg-destructive/10 hover:text-destructive"
        :disabled="emptying"
        @click="confirmState = { kind: 'empty' }"
      >
        <Loader2Icon v-if="emptying" class="animate-spin" />
        {{ t("docs.trash.emptyAll") }}
      </Button>
    </div>

    <div v-if="loading && entries.length === 0" class="space-y-2.5 px-2 py-8">
      <Skeleton class="h-4 w-full" />
      <Skeleton class="h-4 w-4/5" />
      <Skeleton class="h-4 w-[90%]" />
    </div>
    <div v-else-if="entries.length === 0" class="text-placeholder px-2 py-8 text-center">
      {{ t("docs.trash.empty") }}
    </div>
    <ul v-else class="m-0 flex list-none flex-col gap-1.5 p-0">
      <li
        v-for="e in entries"
        :key="e.id"
        class="bg-card flex items-center gap-2.5 rounded-[8px] border border-[var(--td-component-stroke)] px-2.5 py-2"
      >
        <span class="w-[22px] flex-none text-center">{{ e.icon || "📄" }}</span>
        <div class="min-w-0 flex-1">
          <div class="text-foreground truncate text-sm">{{ e.title || t("docs.tree.untitled") }}</div>
          <div class="text-placeholder text-xs">
            {{
              t("docs.trash.deletedBy", {
                name: e.deleted_by_user?.username || e.deleted_by || "—",
                time: formatDate(e.deleted_at),
              })
            }}
          </div>
        </div>
        <span class="inline-flex flex-none gap-1">
          <Button v-if="e.can_restore" size="sm" variant="outline" :disabled="busy === e.id" @click="restore(e)">
            <Loader2Icon v-if="busy === e.id" class="animate-spin" />
            {{ t("docs.trash.restore") }}
          </Button>
          <Button
            v-if="isAdmin"
            size="sm"
            variant="ghost"
            class="text-destructive hover:text-destructive"
            :disabled="busy === e.id"
            @click="confirmState = { kind: 'purge', entry: e }"
          >
            <Loader2Icon v-if="busy === e.id" class="animate-spin" />
            {{ t("docs.trash.purge") }}
          </Button>
        </span>
      </li>
    </ul>

    <Dialog :open="confirmState !== null" @update:open="(v: boolean) => !v && (confirmState = null)">
      <DialogContent class="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>
            {{
              confirmState?.kind === "empty"
                ? t("docs.trash.emptyConfirm")
                : t("docs.trash.purgeConfirm", { title: confirmState?.entry.title || t("docs.tree.untitled") })
            }}
          </DialogTitle>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="confirmState = null">{{ t("common.cancel") }}</Button>
          <Button variant="destructive" :disabled="busy !== null" @click="confirmAction">
            <Loader2Icon v-if="busy !== null" class="animate-spin" />
            {{ t("common.confirm") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { Loader2Icon, RefreshCwIcon } from "@lucide/vue";
import { MessagePlugin } from "tdesign-vue-next";
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

import {
  emptyTrash,
  listTrash,
  purgeTrashPage,
  restorePage,
  type DocsSpace,
  type PageView,
  type TrashEntry,
} from "@/api/docs";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";

import { canManageSpace } from "./docsAccess";

const props = defineProps<{ space: DocsSpace }>();
const emit = defineEmits<{ restored: [page: PageView] }>();
const { t } = useI18n();

const entries = ref<TrashEntry[]>([]);
const loading = ref(false);
const emptying = ref(false);
const busy = ref<string | null>(null);
const isAdmin = computed(() => canManageSpace(props.space.role));

type ConfirmState = { kind: "empty" } | { kind: "purge"; entry: TrashEntry } | null;
const confirmState = ref<ConfirmState>(null);

const formatDate = (iso: string | null | undefined) => {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
};

const errorText = (err: unknown, fallback: string) => {
  const msg = (err as { message?: string } | null)?.message;
  return msg ? `${fallback}: ${msg}` : fallback;
};

async function load() {
  loading.value = true;
  try {
    entries.value = await listTrash(props.space.id);
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.trash.loadFailed")));
  } finally {
    loading.value = false;
  }
}

async function restore(e: TrashEntry) {
  busy.value = e.id;
  try {
    const page = await restorePage(e.id);
    entries.value = entries.value.filter((x) => x.id !== e.id);
    MessagePlugin.success(t("docs.trash.restoreSuccess"));
    emit("restored", page);
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.trash.restoreFailed")));
  } finally {
    busy.value = null;
  }
}

async function purge(e: TrashEntry) {
  busy.value = e.id;
  try {
    await purgeTrashPage(props.space.id, e.id);
    entries.value = entries.value.filter((x) => x.id !== e.id);
    MessagePlugin.success(t("docs.trash.purgeSuccess"));
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.trash.purgeFailed")));
  } finally {
    busy.value = null;
  }
}

async function emptyAll() {
  emptying.value = true;
  try {
    await emptyTrash(props.space.id);
    entries.value = [];
    MessagePlugin.success(t("docs.trash.emptySuccess"));
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.trash.emptyFailed")));
  } finally {
    emptying.value = false;
  }
}

async function confirmAction() {
  const state = confirmState.value;
  confirmState.value = null;
  if (state?.kind === "purge") await purge(state.entry);
  else if (state?.kind === "empty") await emptyAll();
}

onMounted(load);
</script>
