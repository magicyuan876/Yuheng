<template>
  <section class="flex flex-col gap-2">
    <div v-if="loading && !usage" class="flex items-center justify-center gap-2 py-2">
      <Loader2Icon class="size-4 animate-spin" />
    </div>
    <template v-else>
      <div class="flex items-center justify-between gap-3">
        <span class="text-foreground text-sm tabular-nums">
          <strong>{{ formatBytes(usage?.used_bytes ?? 0) }}</strong>
          <template v-if="hasLimit"> / {{ formatBytes(usage!.quota_bytes) }}</template>
          <span v-else class="text-placeholder ml-1.5 text-[13px]">{{ t("docs.storage.unlimited") }}</span>
        </span>
        <Button v-if="usage?.can_manage" variant="ghost" size="sm" @click="openEditor">
          {{ t("docs.storage.setQuota") }}
        </Button>
      </div>

      <!-- The bar only means something when there is a ceiling to measure
           against; without one it would imply a limit that does not exist. -->
      <div v-if="hasLimit" class="h-1.5 overflow-hidden rounded-full bg-[var(--td-bg-color-component)]">
        <div
          class="h-full rounded-full transition-[width] duration-200"
          :class="level === 'full' ? 'bg-destructive' : level === 'warning' ? 'bg-warning' : 'bg-success'"
          :style="{ width: `${(fraction ?? 0) * 100}%` }"
        />
      </div>

      <p class="text-placeholder m-0 min-h-[18px] text-xs">
        <template v-if="level === 'full'">{{ t("docs.storage.full") }}</template>
        <template v-else-if="level === 'warning'">{{ t("docs.storage.nearlyFull") }}</template>
        <template v-else-if="usage?.from_default">{{ t("docs.storage.fromDefault") }}</template>
        <template v-else-if="!hasLimit">{{ t("docs.storage.unlimitedHint") }}</template>
      </p>
    </template>

    <Dialog :open="editing" @update:open="(v: boolean) => (editing = v)">
      <DialogContent class="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>{{ t("docs.storage.setQuota") }}</DialogTitle>
        </DialogHeader>
        <div class="flex flex-col gap-2">
          <Input
            v-model="draft"
            :placeholder="t('docs.storage.quotaPlaceholder')"
            :aria-invalid="draftInvalid || undefined"
          />
          <p class="text-placeholder m-0 text-xs leading-normal">{{ t("docs.storage.quotaHint") }}</p>
          <!-- Said plainly: lowering a quota below what is already stored is
               allowed, and it is not a delete. -->
          <p v-if="wouldBeOver" class="text-warning m-0 text-xs leading-normal">
            {{ t("docs.storage.belowUsage", { used: formatBytes(usage?.used_bytes ?? 0) }) }}
          </p>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="editing = false">{{ t("common.cancel") }}</Button>
          <Button :disabled="saving || (draft.trim() !== '' && parsed === null)" @click="save">
            <Loader2Icon v-if="saving" class="animate-spin" />
            {{ t("common.save") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";

import { Loader2Icon } from "@lucide/vue";

import { getSpaceUsage, setSpaceQuota, type SpaceUsage } from "@/api/docs";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";

import { formatBytes, parseBytes, usageFraction, usageLevel } from "./formatBytes";

// How much of a space's storage is used, and the one control that changes it.

const props = defineProps<{ spaceId: string }>();

const { t } = useI18n();

const loading = ref(false);
const saving = ref(false);
const editing = ref(false);
const draft = ref("");
const usage = shallowRef<SpaceUsage | null>(null);

const hasLimit = computed(() => (usage.value?.quota_bytes ?? 0) > 0);
const fraction = computed(() => (usage.value ? usageFraction(usage.value.used_bytes, usage.value.quota_bytes) : null));
const level = computed(() => (usage.value ? usageLevel(usage.value.used_bytes, usage.value.quota_bytes) : "ok"));

/** null while the field is empty, so an untouched dialog is not an error. */
const parsed = computed(() => (draft.value.trim() ? parseBytes(draft.value) : null));
const draftInvalid = computed(() => draft.value.trim() !== "" && parsed.value === null);
const wouldBeOver = computed(
  () => parsed.value !== null && parsed.value > 0 && parsed.value < (usage.value?.used_bytes ?? 0),
);

async function load() {
  if (!props.spaceId) return;
  loading.value = true;
  const id = props.spaceId;
  try {
    const next = await getSpaceUsage(id);
    if (props.spaceId === id) usage.value = next;
  } catch {
    // A missing usage figure is not worth an error banner on a settings
    // page; the section simply shows nothing.
    if (props.spaceId === id) usage.value = null;
  } finally {
    if (props.spaceId === id) loading.value = false;
  }
}

function openEditor() {
  // Prefilled with the current limit rather than blank, so "change it
  // slightly" does not mean retyping it.
  draft.value = hasLimit.value ? String(usage.value?.quota_bytes ?? 0) : "";
  editing.value = true;
}

async function save() {
  const bytes = draft.value.trim() === "" ? 0 : parsed.value;
  if (bytes === null) return;
  saving.value = true;
  try {
    usage.value = await setSpaceQuota(props.spaceId, bytes);
    editing.value = false;
  } catch (err) {
    const msg = (err as { message?: string } | null)?.message;
    void MessagePlugin.error(msg ? `${t("docs.storage.saveFailed")}: ${msg}` : t("docs.storage.saveFailed"));
  } finally {
    saving.value = false;
  }
}

watch(
  () => props.spaceId,
  () => {
    void load();
  },
  { immediate: true },
);
</script>
