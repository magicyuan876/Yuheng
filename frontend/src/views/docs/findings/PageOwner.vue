<template>
  <!-- Who maintains the page: the person knowledge health takes the page's
       problems to. Its maintainer or an administrator of the page may hand it
       over, to somebody who can edit it — the server checks that and says so. -->
  <Popover v-if="page.steward_id" v-model:open="open">
    <PopoverTrigger as-child>
      <Button
        variant="ghost"
        size="xs"
        class="page-action"
        :disabled="!page.can_change_owner || busy"
        :title="t('docs.owner.hint')"
        data-testid="page-owner"
      >
        <UserRoundCogIcon />
        {{ t("docs.owner.label", { name: stewardName }) }}
      </Button>
    </PopoverTrigger>
    <PopoverContent v-if="page.can_change_owner" align="start" class="w-72 p-2" data-testid="page-owner-picker">
      <div class="text-muted-foreground px-1 pb-2 text-xs">{{ t("docs.owner.pick") }}</div>
      <Input v-model="query" :placeholder="t('knowledgeHealth.memberSearch')" class="h-8 text-sm" />
      <ul class="m-0 mt-2 flex max-h-64 list-none flex-col overflow-y-auto p-0">
        <li v-for="c in candidates" :key="c.user_id">
          <button
            type="button"
            data-slot="page-owner-option"
            data-testid="page-owner-option"
            :data-user-id="c.user_id"
            class="hover:bg-accent flex w-full cursor-pointer items-center gap-2 rounded-sm border-0 bg-transparent px-2 py-1.5 text-left text-sm"
            @click="choose(c.user_id)"
          >
            <span class="truncate">{{ c.username || c.email || c.user_id }}</span>
            <CheckIcon v-if="c.user_id === page.steward_id" class="text-primary ml-auto size-3.5 shrink-0" />
          </button>
        </li>
        <li v-if="!candidates.length" class="text-placeholder px-2 py-1.5 text-xs">
          {{ t("knowledgeHealth.memberEmpty") }}
        </li>
      </ul>
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import { CheckIcon, UserRoundCogIcon } from "@lucide/vue";

import { setPageOwner, suggestMentions, type MentionCandidate, type PageView } from "@/api/docs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

const props = defineProps<{ page: PageView }>();
const emit = defineEmits<{ changed: [page: PageView] }>();

const { t } = useI18n();

const LIMIT = 20;
const SEARCH_DELAY_MS = 250;

const open = ref(false);
const query = ref("");
const candidates = ref<MentionCandidate[]>([]);
const busy = ref(false);

const stewardName = computed(
  () => props.page.steward?.username || props.page.steward?.email || t("docs.owner.unknown"),
);

// The people who can see the page are the ones worth offering; whether the
// one chosen may also edit it is the server's to decide.
let seq = 0;
let timer: ReturnType<typeof setTimeout> | null = null;

async function load(q: string) {
  const mine = ++seq;
  try {
    const list = await suggestMentions(props.page.id, { q, limit: LIMIT });
    if (mine === seq) candidates.value = list;
  } catch (err) {
    if (mine === seq) console.debug("docs: owner candidates unavailable", err);
  }
}

watch(open, (isOpen) => {
  if (isOpen) void load(query.value.trim());
});

watch(query, (q) => {
  if (timer) clearTimeout(timer);
  timer = setTimeout(() => void load(q.trim()), SEARCH_DELAY_MS);
});

onBeforeUnmount(() => {
  if (timer) clearTimeout(timer);
  seq++;
});

async function choose(userId: string) {
  open.value = false;
  if (userId === props.page.steward_id || busy.value) return;
  busy.value = true;
  try {
    const next = await setPageOwner(props.page.id, userId);
    emit("changed", next);
    void MessagePlugin.success(t("docs.owner.changed"));
  } catch (err) {
    const msg = err instanceof Error ? err.message : "";
    void MessagePlugin.error(msg ? `${t("docs.owner.changeFailed")}: ${msg}` : t("docs.owner.changeFailed"));
  } finally {
    busy.value = false;
  }
}
</script>
