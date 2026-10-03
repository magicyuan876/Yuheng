<template>
  <!--
    The members of one workspace, as the system administrator sees them:
    the same roster an Owner manages under Workspace → Members, reached
    through /system/admin so the administrator need not be a member. Add
    by email, change a role in place, remove after an inline confirm.
  -->
  <Drawer :open="open" swipe-direction="right" @update:open="(v: boolean) => emit('update:open', v)">
    <!-- z-[2500] lifts the panel over the settings modal (z 1100) that hosts
         this page; the width replaces the ui drawer's default 3/4. -->
    <DrawerContent
      class="z-[2500] data-[swipe-direction=right]:w-[760px] data-[swipe-direction=right]:max-w-[100vw] data-[swipe-direction=right]:rounded-l-none data-[swipe-direction=right]:sm:max-w-[100vw]"
    >
      <header class="border-border flex items-center gap-2.5 border-b px-[18px] py-3.5">
        <div class="bg-primary/10 text-primary flex size-8 shrink-0 items-center justify-center rounded-[9px]">
          <UsersIcon class="size-4" />
        </div>
        <DrawerTitle class="text-foreground m-0 min-w-0 flex-1 truncate text-[15px] leading-[1.4] font-semibold">
          {{ t("system.usersWorkspaces.members.title", { name: workspace?.name ?? "" }) }}
        </DrawerTitle>
        <DrawerClose as-child>
          <Button variant="ghost" size="icon-sm" :aria-label="t('common.close')">
            <XIcon />
          </Button>
        </DrawerClose>
      </header>

      <div class="box-border flex min-h-0 w-full flex-auto flex-col gap-3.5 overflow-y-auto px-[18px] py-4">
        <p class="text-muted-foreground m-0 text-[13px] leading-[1.55]">
          {{ t("system.usersWorkspaces.members.description") }}
        </p>

        <!-- Add an existing account. -->
        <form
          class="bg-secondary flex flex-wrap items-end gap-2.5 rounded-lg px-4 py-3"
          novalidate
          @submit.prevent="addMember"
        >
          <div class="flex min-w-[220px] flex-1 flex-col gap-1">
            <Label for="admin-member-email" class="text-[12px]">{{
              t("system.usersWorkspaces.members.emailLabel")
            }}</Label>
            <Input
              id="admin-member-email"
              v-model="addForm.email"
              type="email"
              autocomplete="off"
              class="h-8 text-[13px] md:text-[13px]"
              :placeholder="t('system.usersWorkspaces.members.emailPlaceholder')"
              :aria-invalid="!!addError || undefined"
              :disabled="adding"
              @input="addError = ''"
            />
          </div>
          <div class="flex w-[150px] flex-col gap-1">
            <Label for="admin-member-role" class="text-[12px]">{{
              t("system.usersWorkspaces.members.roleLabel")
            }}</Label>
            <Select :model-value="addForm.role" @update:model-value="(v) => (addForm.role = v as TenantRole)">
              <SelectTrigger id="admin-member-role" size="sm" class="w-full text-[13px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent position="popper" class="z-[6200]">
                <SelectItem v-for="role in ROLES" :key="role" :value="role">{{ formatRole(role) }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <Button type="submit" size="sm" class="h-8" :disabled="adding" data-slot="admin-member-add">
            <Loader2Icon v-if="adding" class="animate-spin" />
            <UserPlusIcon v-else />
            {{ t("system.usersWorkspaces.members.addSubmit") }}
          </Button>
          <p v-if="addError" class="text-destructive m-0 w-full text-xs">{{ addError }}</p>
        </form>

        <div v-if="loading && members.length === 0" class="text-placeholder flex items-center gap-2 py-8 text-[13px]">
          <Loader2Icon class="text-primary size-4 animate-spin" />
          <span>{{ t("system.usersWorkspaces.members.loading") }}</span>
        </div>
        <div v-else-if="error" class="text-destructive flex items-center gap-2 py-8 text-[13px]">
          <CircleAlertIcon class="size-4 shrink-0" />
          <span>{{ error }}</span>
          <Button variant="ghost" size="sm" @click="load">{{ t("common.retry") }}</Button>
        </div>
        <div v-else-if="members.length === 0" class="py-8">
          <Empty class="p-0">
            <EmptyDescription>{{ t("system.usersWorkspaces.members.empty") }}</EmptyDescription>
          </Empty>
        </div>
        <div v-else class="border-border bg-card flex flex-col overflow-hidden rounded-[10px] border">
          <Table>
            <TableHeader>
              <TableRow class="bg-secondary hover:bg-secondary">
                <TableHead :class="headClass">{{ t("system.usersWorkspaces.members.columns.member") }}</TableHead>
                <TableHead :class="[headClass, 'w-[170px]']">{{
                  t("system.usersWorkspaces.members.columns.role")
                }}</TableHead>
                <TableHead :class="[headClass, 'w-[130px]']">{{
                  t("system.usersWorkspaces.members.columns.joined")
                }}</TableHead>
                <TableHead :class="[headClass, 'w-[72px] text-right']">{{
                  t("system.usersWorkspaces.members.columns.actions")
                }}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="row in members" :key="row.user_id" data-slot="admin-member-row" class="hover:bg-accent">
                <TableCell :class="cellClass">
                  <div class="flex min-w-0 flex-col gap-0.5 py-0.5">
                    <span class="text-foreground truncate text-[14px] font-medium">{{
                      row.username || row.email
                    }}</span>
                    <span v-if="row.username" class="text-muted-foreground truncate text-[12px] leading-[1.35]">{{
                      row.email
                    }}</span>
                  </div>
                </TableCell>
                <TableCell :class="cellClass">
                  <Select
                    :model-value="row.role"
                    :disabled="busyUserId === row.user_id"
                    @update:model-value="(v) => changeRole(row, v as TenantRole)"
                  >
                    <SelectTrigger size="sm" class="w-[150px] text-[13px]" :aria-label="formatRole(row.role)">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent position="popper" class="z-[6200]">
                      <SelectItem v-for="role in ROLES" :key="role" :value="role">{{ formatRole(role) }}</SelectItem>
                    </SelectContent>
                  </Select>
                </TableCell>
                <TableCell :class="[cellClass, 'text-muted-foreground text-[13px]']">{{
                  formatDate(row.joined_at)
                }}</TableCell>
                <TableCell :class="[cellClass, 'text-right']">
                  <Popover>
                    <PopoverTrigger as-child>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        class="text-destructive hover:bg-destructive/10 hover:text-destructive"
                        :aria-label="t('system.usersWorkspaces.members.remove')"
                        :disabled="busyUserId === row.user_id"
                      >
                        <Loader2Icon v-if="busyUserId === row.user_id" class="animate-spin" />
                        <UserMinusIcon v-else />
                      </Button>
                    </PopoverTrigger>
                    <PopoverContent side="left" class="z-[3050] w-[min(320px,calc(100vw-24px))] gap-3 p-3">
                      <div class="flex items-start gap-2">
                        <CircleAlertIcon class="text-warning mt-0.5 size-4 shrink-0" />
                        <p class="text-foreground m-0 text-[14px] leading-[1.5]">
                          {{ t("system.usersWorkspaces.members.removeConfirm", { email: row.email }) }}
                        </p>
                      </div>
                      <div class="flex justify-end gap-2">
                        <PopoverClose as-child>
                          <Button variant="outline" size="sm">{{ t("common.cancel") }}</Button>
                        </PopoverClose>
                        <PopoverClose as-child>
                          <Button
                            size="sm"
                            class="bg-destructive text-primary-foreground hover:bg-destructive/90"
                            @click="removeMember(row)"
                          >
                            {{ t("system.usersWorkspaces.members.remove") }}
                          </Button>
                        </PopoverClose>
                      </div>
                    </PopoverContent>
                  </Popover>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
          <div
            v-if="total > pageSize"
            class="border-border flex shrink-0 flex-wrap items-center justify-end gap-x-3 gap-y-2 border-t px-3.5 py-2.5"
          >
            <TenantMembersPager
              v-model:page="page"
              v-model:page-size="pageSize"
              :total="total"
              :page-size-options="PAGE_SIZE_OPTIONS"
              @change="load"
            />
          </div>
        </div>
      </div>
    </DrawerContent>
  </Drawer>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import { PopoverClose } from "reka-ui";
import { CircleAlertIcon, Loader2Icon, UserMinusIcon, UserPlusIcon, UsersIcon, XIcon } from "@lucide/vue";
import {
  addWorkspaceMember,
  listWorkspaceMembers,
  removeWorkspaceMember,
  updateWorkspaceMemberRole,
} from "@/api/system";
import type { TenantInfo } from "@/api/tenant";
import type { TenantMember, TenantRole } from "@/api/tenant/members";
import { useRoleLabel } from "@/composables/useRoleLabel";
import TenantMembersPager from "@/views/settings/TenantMembersPager.vue";
import { Button } from "@/components/ui/button";
import { Drawer, DrawerClose, DrawerContent, DrawerTitle } from "@/components/ui/drawer";
import { Empty, EmptyDescription } from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

const props = defineProps<{
  open: boolean;
  workspace: TenantInfo | null;
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  // Fired after any membership change so the page can refresh its member
  // counts.
  (e: "changed"): void;
}>();

const { t, locale } = useI18n();
const { formatRole } = useRoleLabel();

const ROLES: TenantRole[] = ["owner", "admin", "contributor", "viewer"];
const PAGE_SIZE_OPTIONS = [20, 50, 100];
const headClass = "px-4 py-3 text-[13px] font-semibold text-muted-foreground";
const cellClass = "px-4 py-2.5 text-[14px] text-foreground";

const members = ref<TenantMember[]>([]);
const total = ref(0);
const page = ref(1);
const pageSize = ref(20);
const loading = ref(false);
const error = ref("");
// The member whose role change or removal is in flight; its controls are
// disabled so a second click cannot race the first.
const busyUserId = ref<string | null>(null);

const adding = ref(false);
const addError = ref("");
// Viewer is the safest default for an account the administrator places
// somewhere; a wider role is a deliberate choice.
const addForm = reactive<{ email: string; role: TenantRole }>({ email: "", role: "viewer" });

watch(
  () => [props.open, props.workspace?.id] as const,
  ([open]) => {
    if (!open || !props.workspace) return;
    page.value = 1;
    members.value = [];
    addForm.email = "";
    addForm.role = "viewer";
    addError.value = "";
    void load();
  },
  { immediate: true },
);

function formatDate(iso: string): string {
  try {
    return new Intl.DateTimeFormat(locale.value || "zh-CN", { dateStyle: "medium" }).format(new Date(iso));
  } catch {
    return iso;
  }
}

async function load() {
  if (!props.workspace) return;
  loading.value = true;
  error.value = "";
  try {
    const resp = await listWorkspaceMembers(props.workspace.id, { page: page.value, page_size: pageSize.value });
    if (!resp.success || !resp.data) {
      error.value = resp.message || t("system.usersWorkspaces.members.loadFailed");
      return;
    }
    members.value = resp.data.members ?? [];
    total.value = resp.data.total ?? members.value.length;
  } catch (err: any) {
    error.value = err?.message || t("system.usersWorkspaces.members.loadFailed");
  } finally {
    loading.value = false;
  }
}

async function addMember() {
  if (!props.workspace || adding.value) return;
  const email = addForm.email.trim();
  if (!email) {
    addError.value = t("system.usersWorkspaces.members.emailRequired");
    return;
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    addError.value = t("system.usersWorkspaces.members.emailInvalid");
    return;
  }
  adding.value = true;
  try {
    await addWorkspaceMember(props.workspace.id, { email, role: addForm.role });
    MessagePlugin.success(t("system.usersWorkspaces.members.addSuccess"));
    addForm.email = "";
    emit("changed");
    await load();
  } catch (err: any) {
    // 404 is the deliberate "no such account" signal: the administrator
    // creates the account first, which can place it here at the same time.
    addError.value =
      err?.status === 404 || err?.error?.code === 1003
        ? t("system.usersWorkspaces.members.notRegistered")
        : err?.message || t("system.usersWorkspaces.members.addFailed");
  } finally {
    adding.value = false;
  }
}

async function changeRole(row: TenantMember, role: TenantRole) {
  if (!props.workspace || role === row.role || busyUserId.value) return;
  busyUserId.value = row.user_id;
  try {
    await updateWorkspaceMemberRole(props.workspace.id, row.user_id, role);
    row.role = role;
    MessagePlugin.success(t("system.usersWorkspaces.members.roleUpdated"));
    emit("changed");
  } catch (err: any) {
    // The last-owner rule comes back as a 409 with the reason; show it.
    MessagePlugin.error(err?.message || t("system.usersWorkspaces.members.roleUpdateFailed"));
    await load();
  } finally {
    busyUserId.value = null;
  }
}

async function removeMember(row: TenantMember) {
  if (!props.workspace || busyUserId.value) return;
  busyUserId.value = row.user_id;
  try {
    await removeWorkspaceMember(props.workspace.id, row.user_id);
    MessagePlugin.success(t("system.usersWorkspaces.members.removeSuccess"));
    emit("changed");
    // Removing the last row of a page sends the reader to the previous one.
    if (members.value.length === 1 && page.value > 1) page.value -= 1;
    await load();
  } catch (err: any) {
    MessagePlugin.error(err?.message || t("system.usersWorkspaces.members.removeFailed"));
  } finally {
    busyUserId.value = null;
  }
}
</script>
