<template>
  <div class="flex flex-col gap-4">
    <div>
      <div class="flex items-center justify-between gap-3">
        <h2 class="text-foreground m-0 text-lg leading-[26px] font-semibold">{{ t("docs.groups.title") }}</h2>
        <Button v-if="canManage" size="sm" @click="openCreate">
          <PlusIcon />
          {{ t("docs.groups.create") }}
        </Button>
      </div>
      <p class="text-muted-foreground mt-1.5 mb-0 text-[13px] leading-5">{{ t("docs.groups.subtitle") }}</p>
    </div>

    <Table>
      <TableHeader>
        <TableRow>
          <TableHead :style="{ minWidth: '180px' }">{{ columns[0].title }}</TableHead>
          <TableHead>{{ columns[1].title }}</TableHead>
          <TableHead :style="{ width: '120px' }">{{ columns[2].title }}</TableHead>
          <TableHead :style="{ width: '132px' }">{{ columns[3].title }}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <template v-if="loading">
          <TableRow v-for="i in 3" :key="i">
            <TableCell v-for="c in columns" :key="c.colKey"><Skeleton class="h-4 w-full" /></TableCell>
          </TableRow>
        </template>
        <template v-else>
          <TableRow v-for="row in groups" :key="row.id">
            <TableCell>
              <div class="text-foreground inline-flex items-center gap-2">
                <UsersIcon class="text-primary size-4" />
                <span>{{ row.name }}</span>
                <Badge v-if="row.is_default" variant="outline">{{ t("docs.groups.defaultBadge") }}</Badge>
              </div>
            </TableCell>
            <TableCell>
              <span class="text-muted-foreground">
                {{ row.is_default ? t("docs.groups.defaultHint") : row.description || "—" }}
              </span>
            </TableCell>
            <TableCell>
              <button
                type="button"
                data-slot="link-button"
                class="text-primary cursor-pointer text-sm hover:underline"
                @click="openMembers(row)"
              >
                {{ t("docs.groups.memberCount", { count: row.member_count }) }}
              </button>
            </TableCell>
            <TableCell>
              <div class="flex gap-0.5">
                <Tooltip>
                  <TooltipTrigger as-child>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      :aria-label="t('docs.groups.manageMembers')"
                      @click="openMembers(row)"
                    >
                      <UsersRoundIcon />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{{ t("docs.groups.manageMembers") }}</TooltipContent>
                </Tooltip>
                <Tooltip v-if="canManage">
                  <TooltipTrigger as-child>
                    <Button variant="ghost" size="icon-sm" :aria-label="t('common.edit')" @click="openEdit(row)">
                      <PencilIcon />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{{ t("common.edit") }}</TooltipContent>
                </Tooltip>
                <Popover
                  v-if="canManage && !row.is_default"
                  :open="deleteConfirmId === row.id"
                  @update:open="(v: boolean) => (deleteConfirmId = v ? row.id : null)"
                >
                  <PopoverTrigger as-child>
                    <Button variant="ghost" size="icon-sm" class="text-destructive">
                      <Trash2Icon />
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent align="end" class="w-64">
                    <p class="text-foreground m-0 mb-3 text-sm">
                      {{ t("docs.groups.deleteConfirm", { name: row.name }) }}
                    </p>
                    <div class="flex justify-end gap-2">
                      <Button variant="outline" size="sm" @click="deleteConfirmId = null">
                        {{ t("common.cancel") }}
                      </Button>
                      <Button
                        variant="destructive"
                        size="sm"
                        @click="
                          deleteConfirmId = null;
                          remove(row);
                        "
                      >
                        {{ t("common.confirm") }}
                      </Button>
                    </div>
                  </PopoverContent>
                </Popover>
              </div>
            </TableCell>
          </TableRow>
          <TableRow v-if="!groups.length">
            <TableCell :colspan="columns.length">
              <Empty>
                <EmptyDescription>{{ t("docs.groups.empty") }}</EmptyDescription>
              </Empty>
            </TableCell>
          </TableRow>
        </template>
      </TableBody>
    </Table>

    <!-- Create / edit dialog -->
    <Dialog v-model:open="editVisible">
      <DialogContent class="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle>{{ editing ? t("docs.groups.editTitle") : t("docs.groups.createTitle") }}</DialogTitle>
        </DialogHeader>
        <form class="flex flex-col gap-3" @submit.prevent>
          <div class="flex flex-col gap-1.5">
            <Label for="group-name-input" class="text-sm font-medium">{{ t("docs.groups.name") }}</Label>
            <Input
              id="group-name-input"
              v-model="form.name"
              :maxlength="100"
              :disabled="editing?.is_default"
              :placeholder="t('docs.groups.namePlaceholder')"
            />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="group-description-input" class="text-sm font-medium">{{ t("docs.groups.description") }}</Label>
            <Textarea
              id="group-description-input"
              v-model="form.description"
              :maxlength="4000"
              rows="2"
              class="max-h-[116px]"
              :placeholder="t('docs.groups.descriptionPlaceholder')"
            />
          </div>
        </form>
        <DialogFooter>
          <DialogClose as-child>
            <Button variant="outline">{{ t("common.cancel") }}</Button>
          </DialogClose>
          <Button :disabled="saving || !form.name.trim()" @click="submitEdit">
            <Loader2Icon v-if="saving" class="animate-spin" />
            {{ t("common.save") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Members dialog -->
    <Dialog v-model:open="membersVisible">
      <DialogContent class="sm:max-w-[640px]">
        <DialogHeader>
          <DialogTitle>{{ membersTitle }}</DialogTitle>
        </DialogHeader>
        <div class="flex flex-col gap-3">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <!-- The Input has no prefix slot, so the search icon sits over it and
                 the clear button replaces TDesign's `clearable`. The list reloads
                 on every keystroke, as the old input's change event did. -->
            <div class="relative w-[220px]">
              <SearchIcon
                class="text-placeholder pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
              />
              <Input
                :model-value="memberQuery"
                :placeholder="t('docs.groups.searchPlaceholder')"
                class="pr-8 pl-8"
                @update:model-value="onMemberQueryInput"
                @keydown.enter="() => reloadMembers(1)"
              />
              <button
                v-if="memberQuery"
                type="button"
                data-slot="input-clear"
                class="text-placeholder hover:text-foreground absolute top-1/2 right-2 flex -translate-y-1/2 items-center"
                :aria-label="t('common.clear')"
                @click="onMemberQueryInput('')"
              >
                <XIcon class="size-3.5" />
              </button>
            </div>
            <div
              v-if="canManage && activeGroup && !activeGroup.is_default"
              class="flex min-w-[260px] flex-1 items-center justify-end gap-2"
            >
              <!-- Remote-search multi picker stands in for the old filterable
                   t-select: a popover with a search box and checkbox rows. -->
              <Popover>
                <PopoverTrigger as-child>
                  <Button variant="outline" size="sm" class="max-w-[320px] flex-1 justify-between font-normal">
                    <!-- Like the old multi-select, the trigger lists the picked
                         people, collapsing everything past the second into "+N". -->
                    <span v-if="pendingUserIds.length" class="text-foreground truncate">{{ pendingSummary }}</span>
                    <span v-else class="text-placeholder truncate">{{ t("docs.groups.addMembersPlaceholder") }}</span>
                    <ChevronDownIcon />
                  </Button>
                </PopoverTrigger>
                <PopoverContent align="end" class="w-[280px] p-2">
                  <div class="relative mb-2">
                    <SearchIcon
                      class="text-placeholder pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
                    />
                    <Input
                      :model-value="memberAddQuery"
                      :placeholder="t('docs.groups.searchPlaceholder')"
                      class="pl-8"
                      @update:model-value="onMemberAddSearch"
                    />
                  </div>
                  <div class="flex flex-col gap-1">
                    <div
                      v-if="memberSearch.loading.value"
                      class="text-muted-foreground flex items-center gap-2 px-1 py-2 text-xs"
                    >
                      <Loader2Icon class="animate-spin" />
                    </div>
                    <label
                      v-for="opt in addableOptions"
                      :key="opt.value"
                      class="hover:bg-accent flex cursor-pointer items-center gap-2 rounded px-1 py-1.5 text-sm has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-50"
                    >
                      <Checkbox
                        :model-value="pendingUserIds.includes(opt.value)"
                        :disabled="opt.disabled"
                        @update:model-value="
                          (v: boolean | 'indeterminate') => togglePendingUser(opt.value, opt.label, v === true)
                        "
                      />
                      <span class="truncate">{{ opt.label }}</span>
                    </label>
                    <p
                      v-if="!memberSearch.loading.value && !addableOptions.length"
                      class="text-placeholder m-0 px-1 py-2 text-xs"
                    >
                      {{ t("docs.groups.noMembers") }}
                    </p>
                  </div>
                </PopoverContent>
              </Popover>
              <Button size="sm" :disabled="!pendingUserIds.length || addingMembers" @click="addMembers">
                <Loader2Icon v-if="addingMembers" class="animate-spin" />
                {{ t("docs.groups.addMembers") }}
              </Button>
            </div>
          </div>
          <p v-if="activeGroup?.is_default" class="text-muted-foreground mt-1.5 mb-0 text-[13px] leading-5">
            {{ t("docs.groups.defaultHint") }}
          </p>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{{ memberColumns[0].title }}</TableHead>
                <TableHead v-if="memberColumns.length > 1" :style="{ width: '72px' }">
                  {{ memberColumns[1].title }}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <template v-if="membersLoading">
                <TableRow v-for="i in 3" :key="i">
                  <TableCell v-for="c in memberColumns" :key="c.colKey"><Skeleton class="h-4 w-full" /></TableCell>
                </TableRow>
              </template>
              <template v-else>
                <TableRow v-for="row in memberPage.members" :key="row.user_id">
                  <TableCell>
                    <div class="flex min-w-0 items-center gap-2.5">
                      <div
                        class="bg-secondary text-muted-foreground flex size-7 shrink-0 items-center justify-center overflow-hidden rounded-full"
                      >
                        <img v-if="row.avatar" :src="row.avatar" alt="" class="size-full object-cover" />
                        <UserRoundIcon v-else class="size-3.5" />
                      </div>
                      <div class="flex min-w-0 flex-col">
                        <span class="text-foreground text-sm leading-5">{{ row.username || row.user_id }}</span>
                        <span class="text-placeholder text-xs leading-4">{{ row.email }}</span>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell v-if="memberColumns.length > 1">
                    <Popover
                      :open="removeConfirmId === row.user_id"
                      @update:open="(v: boolean) => (removeConfirmId = v ? row.user_id : null)"
                    >
                      <PopoverTrigger as-child>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          class="text-destructive"
                          :disabled="removingUserId === row.user_id"
                        >
                          <Loader2Icon v-if="removingUserId === row.user_id" class="animate-spin" />
                          <Trash2Icon v-else />
                        </Button>
                      </PopoverTrigger>
                      <PopoverContent align="end" class="w-64">
                        <p class="text-foreground m-0 mb-3 text-sm">{{ t("docs.groups.removeMemberConfirm") }}</p>
                        <div class="flex justify-end gap-2">
                          <Button variant="outline" size="sm" @click="removeConfirmId = null">
                            {{ t("common.cancel") }}
                          </Button>
                          <Button
                            variant="destructive"
                            size="sm"
                            @click="
                              removeConfirmId = null;
                              removeMember(row);
                            "
                          >
                            {{ t("common.confirm") }}
                          </Button>
                        </div>
                      </PopoverContent>
                    </Popover>
                  </TableCell>
                </TableRow>
                <TableRow v-if="!memberPage.members.length">
                  <TableCell :colspan="memberColumns.length">
                    <Empty>
                      <EmptyDescription>{{ t("docs.groups.noMembers") }}</EmptyDescription>
                    </Empty>
                  </TableCell>
                </TableRow>
              </template>
            </TableBody>
          </Table>
          <div
            v-if="memberPage.total > memberPage.page_size"
            class="text-muted-foreground flex items-center gap-2 self-end text-xs"
          >
            <span>{{ memberPage.page }} / {{ totalMemberPages }}</span>
            <Button
              variant="outline"
              size="icon-xs"
              :disabled="memberPage.page <= 1"
              @click="reloadMembers(memberPage.page - 1)"
            >
              <ChevronLeftIcon />
            </Button>
            <Button
              variant="outline"
              size="icon-xs"
              :disabled="memberPage.page >= totalMemberPages"
              @click="reloadMembers(memberPage.page + 1)"
            >
              <ChevronRightIcon />
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { MessagePlugin } from "tdesign-vue-next";
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

import {
  addGroupMembers,
  createGroup,
  deleteGroup,
  listGroupMembers,
  listGroups,
  removeGroupMember,
  updateGroup,
  type GroupMemberPage,
  type GroupMemberRow,
  type TenantGroup,
} from "@/api/docs";
import { useAuthStore } from "@/stores/auth";
import { useMemberSearch } from "@/views/docs/useMemberSearch";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogClose, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Empty, EmptyDescription } from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  ChevronDownIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  Loader2Icon,
  PencilIcon,
  PlusIcon,
  SearchIcon,
  Trash2Icon,
  UserRoundIcon,
  UsersIcon,
  UsersRoundIcon,
  XIcon,
} from "@lucide/vue";

const { t } = useI18n();
const authStore = useAuthStore();
const canManage = computed(() => authStore.hasRole("admin"));

const errorText = (err: unknown, fallback: string): string => {
  const msg = (err as { message?: string } | null)?.message;
  return msg ? `${fallback}: ${msg}` : fallback;
};

// ---- list ---------------------------------------------------------------------------
const groups = ref<TenantGroup[]>([]);
const loading = ref(false);

const load = async () => {
  loading.value = true;
  try {
    groups.value = await listGroups();
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.groups.loadFailed")));
  } finally {
    loading.value = false;
  }
};

const columns = computed(() => [
  { colKey: "name", title: t("docs.groups.columns.name"), minWidth: 180, ellipsis: true },
  { colKey: "description", title: t("docs.groups.columns.description"), ellipsis: true },
  { colKey: "members", title: t("docs.groups.columns.members"), width: 120 },
  { colKey: "actions", title: t("docs.groups.columns.actions"), width: 132 },
]);

// ---- create / edit ------------------------------------------------------------------
const editVisible = ref(false);
const editing = ref<TenantGroup | null>(null);
const saving = ref(false);
const form = ref({ name: "", description: "" });

const openCreate = () => {
  editing.value = null;
  form.value = { name: "", description: "" };
  editVisible.value = true;
};

const openEdit = (g: TenantGroup) => {
  editing.value = g;
  form.value = { name: g.name, description: g.description };
  editVisible.value = true;
};

const submitEdit = async () => {
  saving.value = true;
  try {
    if (editing.value) {
      const body: { name?: string; description?: string } = { description: form.value.description.trim() };
      if (!editing.value.is_default) body.name = form.value.name.trim();
      const updated = await updateGroup(editing.value.id, body);
      groups.value = groups.value.map((g) => (g.id === updated.id ? updated : g));
      MessagePlugin.success(t("docs.groups.updateSuccess"));
    } else {
      const created = await createGroup({ name: form.value.name.trim(), description: form.value.description.trim() });
      groups.value = [...groups.value, created];
      MessagePlugin.success(t("docs.groups.createSuccess"));
    }
    editVisible.value = false;
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, editing.value ? t("docs.groups.updateFailed") : t("docs.groups.createFailed")));
  } finally {
    saving.value = false;
  }
};

const remove = async (g: TenantGroup) => {
  try {
    await deleteGroup(g.id);
    groups.value = groups.value.filter((x) => x.id !== g.id);
    MessagePlugin.success(t("docs.groups.deleteSuccess"));
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.groups.deleteFailed")));
  }
};

// ---- members -------------------------------------------------------------------------
const membersVisible = ref(false);
const activeGroup = ref<TenantGroup | null>(null);
const memberQuery = ref("");
const membersLoading = ref(false);
const memberPage = ref<GroupMemberPage>({ members: [], total: 0, page: 1, page_size: 20 });
const removingUserId = ref("");
// Which row's delete confirmation popover is open (null = none).
const deleteConfirmId = ref<string | null>(null);
const removeConfirmId = ref<string | null>(null);
const pendingUserIds = ref<string[]>([]);
const addingMembers = ref(false);
const memberAddQuery = ref("");
const memberSearch = useMemberSearch();

const membersTitle = computed(() =>
  activeGroup.value ? `${t("docs.groups.membersTitle")} · ${activeGroup.value.name}` : t("docs.groups.membersTitle"),
);

const memberColumns = computed(() => {
  const cols = [{ colKey: "user", title: t("docs.members.columns.member"), ellipsis: true }];
  if (canManage.value && activeGroup.value && !activeGroup.value.is_default) {
    cols.push({ colKey: "actions", title: t("docs.groups.columns.actions"), ellipsis: false, width: 72 } as never);
  }
  return cols;
});

const addableOptions = computed(() => {
  const present = new Set(memberPage.value.members.map((m) => m.user_id));
  return memberSearch.options.value.map((o) => ({ ...o, disabled: present.has(o.value) }));
});

const totalMemberPages = computed(() => Math.max(1, Math.ceil(memberPage.value.total / memberPage.value.page_size)));

// Labels of the picked users, remembered at pick time: the option list is
// replaced by every remote search, and the trigger must still name them.
const pendingLabels = ref<Record<string, string>>({});

const pendingSummary = computed(() => {
  const names = pendingUserIds.value.map((id) => pendingLabels.value[id] ?? id);
  if (names.length <= 2) return names.join(", ");
  return `${names.slice(0, 2).join(", ")} +${names.length - 2}`;
});

const togglePendingUser = (userId: string, label: string, checked: boolean) => {
  if (checked) {
    pendingLabels.value = { ...pendingLabels.value, [userId]: label };
    if (!pendingUserIds.value.includes(userId)) pendingUserIds.value = [...pendingUserIds.value, userId];
  } else {
    pendingUserIds.value = pendingUserIds.value.filter((id) => id !== userId);
  }
};

const onMemberQueryInput = (value: string | number) => {
  memberQuery.value = String(value);
  void reloadMembers(1);
};

const onMemberAddSearch = (value: string | number) => {
  memberAddQuery.value = String(value);
  memberSearch.search(memberAddQuery.value);
};

const openMembers = async (g: TenantGroup) => {
  activeGroup.value = g;
  memberQuery.value = "";
  memberAddQuery.value = "";
  pendingUserIds.value = [];
  membersVisible.value = true;
  void memberSearch.load("");
  await reloadMembers(1);
};

const reloadMembers = async (page: number) => {
  if (!activeGroup.value) return;
  membersLoading.value = true;
  try {
    memberPage.value = await listGroupMembers(activeGroup.value.id, {
      q: memberQuery.value,
      page,
      page_size: memberPage.value.page_size,
    });
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.groups.membersLoadFailed")));
  } finally {
    membersLoading.value = false;
  }
};

const refreshCount = (updated: TenantGroup) => {
  groups.value = groups.value.map((g) => (g.id === updated.id ? updated : g));
  activeGroup.value = updated;
};

const addMembers = async () => {
  if (!activeGroup.value || !pendingUserIds.value.length) return;
  addingMembers.value = true;
  try {
    refreshCount(await addGroupMembers(activeGroup.value.id, pendingUserIds.value));
    pendingUserIds.value = [];
    MessagePlugin.success(t("docs.groups.addSuccess"));
    await reloadMembers(memberPage.value.page);
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.groups.addFailed")));
  } finally {
    addingMembers.value = false;
  }
};

const removeMember = async (row: GroupMemberRow) => {
  if (!activeGroup.value) return;
  removingUserId.value = row.user_id;
  try {
    await removeGroupMember(activeGroup.value.id, row.user_id);
    MessagePlugin.success(t("docs.groups.removeSuccess"));
    refreshCount({ ...activeGroup.value, member_count: Math.max(0, activeGroup.value.member_count - 1) });
    await reloadMembers(memberPage.value.page);
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.groups.removeFailed")));
  } finally {
    removingUserId.value = "";
  }
};

onMounted(load);
</script>
