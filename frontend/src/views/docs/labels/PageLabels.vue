<template>
  <div class="flex flex-wrap items-center gap-1.5">
    <span
      v-for="l in model"
      :key="l.id"
      class="border-border text-foreground inline-flex items-center gap-[5px] rounded-full border px-2 py-0.5 text-xs leading-[18px]"
    >
      <span class="size-2 flex-none rounded-full" :class="DOT_CLASS[l.color] ?? 'bg-placeholder'" />
      <span>{{ l.name }}</span>
      <button
        v-if="canEdit"
        type="button"
        data-slot="label-remove"
        class="text-placeholder hover:text-destructive inline-flex cursor-pointer border-0 p-0"
        :aria-label="t('docs.labels.remove', { name: l.name })"
        @click="remove(l.id)"
      >
        <XIcon class="size-3" />
      </button>
    </span>

    <Popover v-if="canEdit" :open="open" @update:open="onOpenChange">
      <PopoverTrigger as-child>
        <button
          type="button"
          data-slot="label-add"
          class="border-border text-placeholder hover:border-primary hover:text-primary inline-flex cursor-pointer items-center gap-1 rounded-full border border-dashed px-2 py-0.5 text-xs leading-[18px]"
        >
          <PlusIcon class="size-3" />
          <span>{{ model.length ? t("docs.labels.add") : t("docs.labels.addFirst") }}</span>
        </button>
      </PopoverTrigger>
      <PopoverContent class="w-[240px] p-2" align="start">
        <div class="flex flex-col gap-2">
          <Input
            v-model="query"
            :maxlength="MAX_NAME"
            :placeholder="t('docs.labels.search')"
            autofocus
            @keydown.enter="(e: KeyboardEvent) => !e.isComposing && createFromQuery()"
          />
          <div class="flex max-h-[220px] flex-col gap-0.5 overflow-y-auto">
            <button
              v-for="l in matches"
              :key="l.id"
              type="button"
              class="hover:bg-accent flex w-full cursor-pointer items-center gap-2 rounded border-0 px-2 py-[5px] text-left text-[13px] text-inherit"
              @click="toggle(l)"
            >
              <span class="size-2 flex-none rounded-full" :class="DOT_CLASS[l.color] ?? 'bg-placeholder'" />
              <span class="min-w-0 flex-1 truncate">{{ l.name }}</span>
              <CheckIcon v-if="chosen.has(l.id)" class="size-3.5" />
            </button>
            <p v-if="!matches.length && !canCreate" class="text-placeholder m-0 px-2 py-1.5 text-xs">
              {{ t("docs.labels.none") }}
            </p>
          </div>
          <!-- Making one is the same gesture as picking one: a writer files
               their own work without asking an admin for the vocabulary. -->
          <button
            v-if="canCreate"
            type="button"
            class="text-primary hover:bg-accent flex w-full cursor-pointer items-center gap-2 rounded-none border-0 border-t border-[var(--td-component-stroke)] px-2 py-[5px] text-left text-[13px]"
            :disabled="creating"
            @click="createFromQuery"
          >
            <PlusIcon class="size-3.5" />
            <span class="truncate">{{ t("docs.labels.create", { name: trimmedQuery }) }}</span>
          </button>
        </div>
      </PopoverContent>
    </Popover>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";

import { CheckIcon, PlusIcon, XIcon } from "@lucide/vue";

import { createLabel, listLabels, setPageLabels, type LabelView } from "@/api/docs";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

// The labels on one page: chips beside the title, and a picker that doubles
// as the place new labels are made.
//
// The server is told the whole set rather than one addition at a time, which
// is what "these are its labels now" means and what makes two people editing
// the same page converge on a set rather than on a race.

const MAX_NAME = 32;
const MAX_PER_PAGE = 20;

/** The closed set from LabelColors, kept in one place. */
const DOT_CLASS: Record<string, string> = {
  gray: "bg-[#8b8f96]",
  red: "bg-[#e34d59]",
  orange: "bg-[#ed7b2f]",
  yellow: "bg-[#ebb105]",
  green: "bg-[#2ba471]",
  teal: "bg-[#0594fa]",
  blue: "bg-[#366ef4]",
  purple: "bg-[#834ec2]",
  pink: "bg-[#ed49b4]",
};

const props = defineProps<{ pageId: string; spaceId: string; canEdit: boolean; labels: LabelView[] }>();
const emit = defineEmits<{ change: [LabelView[]] }>();

const { t } = useI18n();

const model = ref<LabelView[]>([...props.labels]);
watch(
  () => props.labels,
  (next) => {
    model.value = [...next];
  },
);

const open = ref(false);
const query = ref("");
const creating = ref(false);
const available = shallowRef<LabelView[]>([]);

const chosen = computed(() => new Set(model.value.map((l) => l.id)));
const trimmedQuery = computed(() => query.value.trim().replace(/\s+/g, " "));

const matches = computed(() => {
  const q = trimmedQuery.value.toLowerCase();
  if (!q) return available.value;
  return available.value.filter((l) => l.name.toLowerCase().includes(q));
});

// Only when the typed name is not already a label: offering to create a
// duplicate of the row directly above is an error waiting to be clicked.
const canCreate = computed(() => {
  const q = trimmedQuery.value;
  return q.length > 0 && q.length <= MAX_NAME && !available.value.some((l) => l.name.toLowerCase() === q.toLowerCase());
});

async function onOpenChange(visible: boolean) {
  open.value = visible;
  if (!visible) {
    query.value = "";
    return;
  }
  try {
    available.value = await listLabels(props.spaceId);
  } catch {
    available.value = [];
  }
}

async function commit(next: LabelView[]) {
  const before = model.value;
  model.value = next;
  try {
    const saved = await setPageLabels(
      props.pageId,
      next.map((l) => l.id),
    );
    model.value = saved;
    emit("change", saved);
  } catch (err: unknown) {
    // Put the chips back: leaving the optimistic set on screen would tell
    // somebody their page is filed when it is not.
    model.value = before;
    const msg = (err as { message?: string } | null)?.message;
    MessagePlugin.error(msg ? `${t("docs.labels.saveFailed")}: ${msg}` : t("docs.labels.saveFailed"));
  }
}

function toggle(label: LabelView) {
  if (chosen.value.has(label.id)) {
    void commit(model.value.filter((l) => l.id !== label.id));
    return;
  }
  if (model.value.length >= MAX_PER_PAGE) {
    MessagePlugin.warning(t("docs.labels.tooMany", { n: MAX_PER_PAGE }));
    return;
  }
  void commit([...model.value, label]);
}

function remove(id: string) {
  void commit(model.value.filter((l) => l.id !== id));
}

async function createFromQuery() {
  if (!canCreate.value || creating.value) return;
  creating.value = true;
  try {
    const made = await createLabel(props.spaceId, { name: trimmedQuery.value });
    available.value = [...available.value, made];
    query.value = "";
    toggle(made);
  } catch (err: unknown) {
    const msg = (err as { message?: string } | null)?.message;
    MessagePlugin.error(msg ? `${t("docs.labels.createFailed")}: ${msg}` : t("docs.labels.createFailed"));
  } finally {
    creating.value = false;
  }
}
</script>
