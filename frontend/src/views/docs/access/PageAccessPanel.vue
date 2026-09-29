<template>
  <div class="relative flex min-h-[120px] flex-col gap-4">
    <!-- The old t-loading: the first load shows only the spinner; a reload keeps the content
         visible under a veil, so the panel does not jump while it refreshes. -->
    <div v-if="loading && !view" class="flex items-center justify-center gap-2 py-8">
      <Loader2Icon class="size-4 animate-spin" />
    </div>
    <div
      v-else-if="loading"
      class="bg-card/60 absolute inset-0 z-10 flex items-center justify-center"
      role="status"
      aria-live="polite"
    >
      <Loader2Icon class="text-primary size-4 animate-spin" />
    </div>
    <template v-if="view || !loading">
      <!-- What is true right now, before any control to change it. Somebody
           opens this panel to find out why, not only to act. -->
      <div class="flex items-start gap-3">
        <LockIcon v-if="view?.restricted" class="text-foreground mt-0.5 size-[18px]" />
        <UsersRoundIcon v-else class="text-foreground mt-0.5 size-[18px]" />
        <div class="min-w-0 flex-1">
          <div class="text-sm font-semibold">
            {{ view?.restricted ? t("docs.access.restrictedTitle") : t("docs.access.inheritedTitle") }}
          </div>
          <p class="text-placeholder m-0 mt-0.5 text-xs">
            {{
              view?.restricted
                ? t("docs.access.restrictedHint")
                : t("docs.access.inheritedHint", { role: roleName(view?.space_default) })
            }}
          </p>
        </div>
        <Switch
          v-if="view?.can_manage"
          :model-value="view.restricted"
          :disabled="switching"
          :aria-label="t('docs.access.restrictedTitle')"
          @update:model-value="onToggleRestricted"
        />
      </div>

      <!-- A page narrowed from above is not broken, and saying nothing about
           it makes the panel look like it is. -->
      <Alert v-if="view?.inherited_from.length" class="bg-primary/5 border-primary/30 m-0">
        <InfoIcon />
        <AlertDescription>
          <span v-if="nearestAncestor?.visible">
            {{ t("docs.access.narrowedByVisible", { title: nearestAncestor.title || t("docs.tree.untitled") }) }}
          </span>
          <span v-else>{{ t("docs.access.narrowedByHidden") }}</span>
        </AlertDescription>
      </Alert>

      <template v-if="view?.restricted">
        <div class="text-muted-foreground flex items-center gap-1.5 text-xs font-semibold tracking-[0.03em] uppercase">
          <span>{{ t("docs.access.whoHasAccess") }}</span>
          <span class="text-placeholder tabular-nums">{{ view.grants.length }}</span>
        </div>

        <ul class="m-0 flex flex-col gap-1 p-0">
          <li
            v-for="g in view.grants"
            :key="`${g.principal_type}:${g.principal_id}`"
            class="flex items-center gap-2 py-1"
          >
            <UsersRoundIcon v-if="g.principal_type === 'group'" class="text-placeholder size-4 flex-none" />
            <UserRoundIcon v-else class="text-placeholder size-4 flex-none" />
            <div class="flex min-w-0 flex-1 flex-col">
              <span class="truncate text-sm">{{ g.name }}</span>
              <span
                v-if="g.principal_type === 'group' && g.group_member_count !== undefined"
                class="text-placeholder truncate text-xs"
              >
                {{ t("docs.access.groupMembers", { n: g.group_member_count }) }}
              </span>
              <span v-else-if="g.email" class="text-placeholder truncate text-xs">{{ g.email }}</span>
            </div>

            <!-- A grant is a ceiling, so when it exceeds the space role the
                 honest thing is to show what it actually does. -->
            <Tooltip v-if="!g.in_space">
              <TooltipTrigger as-child>
                <Badge class="bg-warning/10 text-warning flex-none">{{ t("docs.access.notInSpace") }}</Badge>
              </TooltipTrigger>
              <TooltipContent>{{ t("docs.access.notInSpaceHint") }}</TooltipContent>
            </Tooltip>
            <Tooltip v-else-if="g.effective !== g.role">
              <TooltipTrigger as-child>
                <Badge class="bg-warning/10 text-warning flex-none">{{ roleName(g.effective) }}</Badge>
              </TooltipTrigger>
              <TooltipContent>
                {{ t("docs.access.cappedHint", { granted: roleName(g.role), effective: roleName(g.effective) }) }}
              </TooltipContent>
            </Tooltip>

            <Select
              v-if="view.can_manage"
              :model-value="g.role"
              :disabled="busy === principalKey(g)"
              @update:model-value="(r) => r && changeRole(g, String(r))"
            >
              <SelectTrigger size="sm" class="w-[104px] flex-none">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="o in roleOptions" :key="o.value" :value="o.value">{{ o.label }}</SelectItem>
              </SelectContent>
            </Select>
            <span v-else class="text-muted-foreground flex-none text-[13px]">{{ roleName(g.role) }}</span>

            <Button
              v-if="view.can_manage"
              variant="ghost"
              size="icon-xs"
              class="text-destructive hover:text-destructive flex-none [&_svg]:size-3.5"
              :disabled="busy === principalKey(g)"
              :aria-label="t('docs.access.remove', { name: g.name })"
              @click="remove(g)"
            >
              <Loader2Icon v-if="busy === principalKey(g)" class="animate-spin" />
              <XIcon v-else />
            </Button>
          </li>
          <li v-if="!view.grants.length" class="text-placeholder py-2 text-[13px]">
            {{ t("docs.access.nobodyYet") }}
          </li>
        </ul>

        <div
          v-if="view.can_manage"
          class="flex flex-wrap items-center gap-2 border-t border-[var(--td-component-stroke)] pt-1"
        >
          <Select v-model="addType">
            <SelectTrigger size="sm" class="w-[96px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="o in typeOptions" :key="o.value" :value="o.value">{{ o.label }}</SelectItem>
            </SelectContent>
          </Select>

          <Popover v-if="addType === 'user'" v-model:open="userPickOpen" @update:open="onUserPickOpen">
            <PopoverTrigger as-child>
              <Button variant="outline" size="sm" class="min-w-[160px] flex-1 justify-between px-2 font-normal">
                <span class="truncate">{{ pickedUserLabel || t("docs.access.pickPerson") }}</span>
                <ChevronsUpDownIcon class="text-placeholder size-3.5" />
              </Button>
            </PopoverTrigger>
            <PopoverContent class="w-(--reka-popover-trigger-width) gap-0 p-0" align="start">
              <div class="border-border border-b p-2">
                <Input
                  v-model="userQuery"
                  class="h-8"
                  :placeholder="t('docs.access.pickPerson')"
                  @input="onUserQuery"
                />
              </div>
              <div class="max-h-56 overflow-y-auto p-1">
                <div
                  v-if="memberLoading"
                  class="text-muted-foreground flex items-center justify-center gap-2 py-4 text-[13px]"
                >
                  <Loader2Icon class="size-3.5 animate-spin" />
                </div>
                <template v-else>
                  <button
                    v-for="o in memberOptions"
                    :key="o.value"
                    type="button"
                    class="hover:bg-accent focus:bg-accent flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none"
                    @click="selectUserOption(o)"
                  >
                    <CheckIcon class="size-3.5" :class="o.value === addUser ? 'opacity-100' : 'opacity-0'" />
                    <span class="truncate">{{ o.label }}</span>
                  </button>
                  <div v-if="!memberOptions.length" class="text-placeholder py-4 text-center text-[13px]">
                    {{ t("common.noData") }}
                  </div>
                </template>
              </div>
            </PopoverContent>
          </Popover>
          <Select v-else v-model="addGroup">
            <SelectTrigger size="sm" class="min-w-[160px] flex-1">
              <SelectValue :placeholder="t('docs.access.pickGroup')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="o in groupOptions" :key="o.value" :value="o.value">{{ o.label }}</SelectItem>
            </SelectContent>
          </Select>

          <Select v-model="addRole">
            <SelectTrigger size="sm" class="w-[104px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="o in roleOptions" :key="o.value" :value="o.value">{{ o.label }}</SelectItem>
            </SelectContent>
          </Select>
          <Button size="sm" :disabled="!canAdd || adding" @click="add">
            <Loader2Icon v-if="adding" class="animate-spin" />
            {{ t("docs.access.grant") }}
          </Button>
        </div>
      </template>
    </template>

    <Dialog :open="confirmInherit" @update:open="(v: boolean) => (confirmInherit = v)">
      <DialogContent class="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>{{ t("docs.access.restoreHeader") }}</DialogTitle>
        </DialogHeader>
        <p class="text-foreground text-sm">{{ t("docs.access.restoreBody", { n: view?.grants.length ?? 0 }) }}</p>
        <DialogFooter>
          <Button variant="outline" @click="confirmInherit = false">{{ t("common.cancel") }}</Button>
          <Button class="bg-warning hover:bg-warning/90 text-[var(--td-text-color-anti)]" @click="restoreInheritance">
            {{ t("docs.access.restoreConfirm") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";

import {
  ChevronsUpDownIcon,
  CheckIcon,
  InfoIcon,
  Loader2Icon,
  LockIcon,
  UserRoundIcon,
  UsersRoundIcon,
  XIcon,
} from "@lucide/vue";

import {
  addPageGrant,
  getPageAccess,
  listGroups,
  removePageGrant,
  setPageRestricted,
  type GrantView,
  type PageAccessView,
  type SpaceRole,
} from "@/api/docs";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

import { GRANTABLE_SPACE_ROLES } from "../docsAccess";
import { useMemberSearch, type MemberOption } from "../useMemberSearch";

// Who can see this page, and why.
//
// The panel exists to explain as much as to change: a restricted page that
// cannot say who narrowed it, or a grant that silently does nothing because
// the person is not in the space, turns a permission question into a support
// ticket. So every row carries what it actually does, not only what it says.

const props = defineProps<{ pageId: string }>();
const emit = defineEmits<{ changed: [PageAccessView] }>();

const { t } = useI18n();

const view = shallowRef<PageAccessView | null>(null);
const loading = ref(false);
const switching = ref(false);
const adding = ref(false);
const busy = ref("");
const confirmInherit = ref(false);

const addType = ref<"user" | "group">("user");
const addUser = ref("");
const addGroup = ref("");
const addRole = ref<SpaceRole>("reader");
const groupOptions = shallowRef<{ label: string; value: string }[]>([]);

const { options: memberOptions, loading: memberLoading, search: memberSearch } = useMemberSearch();

// Remote-search user picker (the t-select `filterable` pattern).
const userPickOpen = ref(false);
const userQuery = ref("");

const pickedUserLabel = computed(() => memberOptions.value.find((o) => o.value === addUser.value)?.label ?? "");

const onUserQuery = () => memberSearch(userQuery.value);

// The old filterable select searched with an empty query on focus, so the
// list is populated before the first keystroke.
const onUserPickOpen = (open: boolean) => {
  if (open) memberSearch(userQuery.value);
};

const selectUserOption = (o: MemberOption) => {
  addUser.value = o.value;
  userPickOpen.value = false;
};

// The same vocabulary the space member list uses; two sets of words for one
// set of roles would be a translation bug waiting to happen.
const roleOptions = computed(() => GRANTABLE_SPACE_ROLES.map((r) => ({ label: t(`docs.spaces.role.${r}`), value: r })));

const typeOptions = computed(() => [
  { label: t("docs.access.person"), value: "user" },
  { label: t("docs.access.group"), value: "group" },
]);

/** Nearest restricted ancestor: the list is ordered root-first, so the one
 * that actually governs this page is the last. */
const nearestAncestor = computed(() => {
  const rows = view.value?.inherited_from ?? [];
  return rows.length ? rows[rows.length - 1] : null;
});

const canAdd = computed(() => (addType.value === "user" ? !!addUser.value : !!addGroup.value));

const principalKey = (g: GrantView) => `${g.principal_type}:${g.principal_id}`;

function roleName(role?: SpaceRole): string {
  return t(`docs.spaces.role.${role || "none"}`);
}

function apply(next: PageAccessView) {
  view.value = next;
  emit("changed", next);
}

function fail(err: unknown, fallback: string) {
  const msg = (err as { message?: string } | null)?.message;
  void MessagePlugin.error(msg ? `${fallback}: ${msg}` : fallback);
}

async function load() {
  if (!props.pageId) return;
  loading.value = true;
  const id = props.pageId;
  try {
    const next = await getPageAccess(id);
    if (props.pageId === id) view.value = next;
  } catch (err) {
    fail(err, t("docs.access.loadFailed"));
  } finally {
    if (props.pageId === id) loading.value = false;
  }
}

async function loadGroups() {
  try {
    const rows = await listGroups();
    groupOptions.value = rows.map((g) => ({ label: g.name, value: g.id }));
  } catch {
    groupOptions.value = [];
  }
}

// Restricting is one click; going back is not. Restoring inheritance drops
// every grant, and somebody who has spent ten minutes building a list should
// be told that before it goes.
function onToggleRestricted(next: boolean) {
  if (!next) {
    confirmInherit.value = true;
    return;
  }
  void restrict(true);
}

async function restrict(next: boolean) {
  switching.value = true;
  try {
    apply(await setPageRestricted(props.pageId, next));
  } catch (err) {
    fail(err, t("docs.access.changeFailed"));
  } finally {
    switching.value = false;
  }
}

async function restoreInheritance() {
  confirmInherit.value = false;
  await restrict(false);
}

async function add() {
  if (!canAdd.value) return;
  adding.value = true;
  try {
    apply(
      await addPageGrant(props.pageId, {
        principal_type: addType.value,
        principal_id: addType.value === "user" ? addUser.value : addGroup.value,
        role: addRole.value,
      }),
    );
    addUser.value = "";
    addGroup.value = "";
  } catch (err) {
    fail(err, t("docs.access.grantFailed"));
  } finally {
    adding.value = false;
  }
}

async function changeRole(g: GrantView, role: string) {
  if (role === g.role) return;
  busy.value = principalKey(g);
  try {
    apply(
      await addPageGrant(props.pageId, {
        principal_type: g.principal_type,
        principal_id: g.principal_id,
        role: role as SpaceRole,
      }),
    );
  } catch (err) {
    fail(err, t("docs.access.grantFailed"));
  } finally {
    busy.value = "";
  }
}

async function remove(g: GrantView) {
  busy.value = principalKey(g);
  try {
    apply(await removePageGrant(props.pageId, g.principal_type, g.principal_id));
  } catch (err) {
    fail(err, t("docs.access.changeFailed"));
  } finally {
    busy.value = "";
  }
}

watch(
  () => props.pageId,
  () => {
    void load();
    void loadGroups();
  },
  { immediate: true },
);
</script>
