<template>
  <!-- Choosing a member of the workspace: who maintains a document, who deals
       with a finding. The server decides whether the person chosen may take
       it on (they must be able to edit the documents), and says why not. -->
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <slot name="trigger" />
    </PopoverTrigger>
    <PopoverContent align="start" class="w-72 p-2" data-testid="member-picker">
      <Input
        v-model="query"
        :placeholder="t('knowledgeHealth.memberSearch')"
        class="h-8 text-sm"
        data-testid="member-picker-search"
      />
      <ul class="m-0 mt-2 flex max-h-64 list-none flex-col overflow-y-auto p-0">
        <li v-if="allowNone">
          <button
            type="button"
            data-slot="member-option"
            data-testid="member-option-none"
            :class="OPTION"
            @click="choose('')"
          >
            <span class="text-muted-foreground">{{ noneLabel || t("knowledgeHealth.memberNone") }}</span>
          </button>
        </li>
        <li v-for="m in members" :key="m.user_id">
          <button
            type="button"
            data-slot="member-option"
            data-testid="member-option"
            :data-user-id="m.user_id"
            :class="OPTION"
            :aria-current="m.user_id === currentId ? 'true' : undefined"
            @click="choose(m.user_id)"
          >
            <span class="truncate">{{ m.username || m.email }}</span>
            <span v-if="m.username && m.email" class="text-placeholder truncate text-xs">{{ m.email }}</span>
            <CheckIcon v-if="m.user_id === currentId" class="text-primary ml-auto size-3.5 shrink-0" />
          </button>
        </li>
        <li v-if="loading" class="text-placeholder px-2 py-1.5 text-xs">{{ t("knowledgeHealth.memberLoading") }}</li>
        <li v-else-if="!members.length" class="text-placeholder px-2 py-1.5 text-xs">
          {{ t("knowledgeHealth.memberEmpty") }}
        </li>
      </ul>
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { CheckIcon } from "@lucide/vue";

import { listMembers, type TenantMember } from "@/api/tenant/members";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { useAuthStore } from "@/stores/auth";

defineProps<{
  /** The person chosen now, ticked in the list. */
  currentId?: string | null;
  /** Offer a "nobody / automatic" choice, which selects the empty ID. */
  allowNone?: boolean;
  noneLabel?: string;
}>();

const emit = defineEmits<{ select: [userId: string] }>();

const { t } = useI18n();
const auth = useAuthStore();

const OPTION =
  "hover:bg-accent flex w-full cursor-pointer items-center gap-2 rounded-sm border-0 bg-transparent px-2 py-1.5 text-left text-sm";
const PAGE_SIZE = 20;
const SEARCH_DELAY_MS = 250;

const open = ref(false);
const query = ref("");
const members = ref<TenantMember[]>([]);
const loading = ref(false);

let seq = 0;
let timer: ReturnType<typeof setTimeout> | null = null;

async function load(q: string) {
  const tenantId = auth.effectiveTenantId;
  if (!tenantId) return;
  const mine = ++seq;
  loading.value = true;
  try {
    const res = await listMembers(Number(tenantId), { q, page_size: PAGE_SIZE });
    if (mine !== seq) return;
    // Only people who can be asked: an invitation not yet accepted, or a
    // suspended membership, cannot take anything on.
    members.value = (res.data?.members ?? []).filter((m) => m.status === "active");
  } catch (err) {
    if (mine === seq) console.debug("member picker: listing members failed", err);
  } finally {
    if (mine === seq) loading.value = false;
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

function choose(userId: string) {
  open.value = false;
  emit("select", userId);
}
</script>
