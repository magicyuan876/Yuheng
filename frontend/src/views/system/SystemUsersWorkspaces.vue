<template>
  <!--
    Users & workspaces — the system administrator's view of who exists and
    where they belong. Gated server-side by RequireSystemAdmin on every
    route it calls; Settings.vue only renders the entry for system
    administrators. Workspace owners manage their own members under
    Workspace → Members; this page is for the workspaces they are not in,
    for creating workspaces (always with an Owner) and for accounts.
  -->
  <div class="w-full">
    <div class="mb-6">
      <h2 class="text-foreground m-0 mb-2 text-xl font-semibold">{{ t("system.usersWorkspaces.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-[1.5]">{{ t("system.usersWorkspaces.description") }}</p>
    </div>

    <!-- Workspaces -->
    <section class="mb-8" :aria-labelledby="'users-workspaces-list'">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div class="inline-flex min-w-0 items-center gap-2">
          <h3 id="users-workspaces-list" class="text-foreground m-0 text-base leading-[1.4] font-bold">
            {{ t("system.usersWorkspaces.workspaces.title") }}
          </h3>
          <span
            class="bg-secondary text-foreground inline-flex h-5 min-w-[22px] items-center justify-center rounded-[10px] px-[7px] text-[12px] leading-none font-semibold"
            >{{ workspaces.length }}</span
          >
        </div>
        <Button size="sm" data-slot="workspace-create" @click="createWorkspaceVisible = true">
          <PlusIcon />
          {{ t("system.usersWorkspaces.workspaces.create") }}
        </Button>
      </div>

      <div
        v-if="workspacesLoading && workspaces.length === 0"
        class="text-placeholder flex items-center justify-center gap-2 py-10 text-[13px]"
      >
        <Loader2Icon class="size-4 animate-spin" />
        <span>{{ t("system.usersWorkspaces.workspaces.loading") }}</span>
      </div>
      <div v-else-if="workspacesError" class="text-destructive flex items-center gap-2 py-10 text-[13px]">
        <CircleAlertIcon class="size-4 shrink-0" />
        <span>{{ workspacesError }}</span>
        <Button variant="ghost" size="sm" @click="loadWorkspaces">{{ t("common.retry") }}</Button>
      </div>
      <div v-else-if="workspaces.length === 0" class="py-10">
        <Empty class="p-0">
          <EmptyDescription>{{ t("system.usersWorkspaces.workspaces.empty") }}</EmptyDescription>
        </Empty>
      </div>
      <div v-else class="border-border bg-card flex flex-col overflow-hidden rounded-[10px] border">
        <Table>
          <TableHeader>
            <TableRow class="bg-secondary hover:bg-secondary">
              <TableHead :class="headClass">{{ t("system.usersWorkspaces.workspaces.columns.name") }}</TableHead>
              <TableHead :class="[headClass, 'w-[110px]']">{{
                t("system.usersWorkspaces.workspaces.columns.members")
              }}</TableHead>
              <TableHead :class="[headClass, 'w-[150px]']">{{
                t("system.usersWorkspaces.workspaces.columns.created")
              }}</TableHead>
              <TableHead :class="[headClass, 'w-[120px] text-right']">{{
                t("system.usersWorkspaces.workspaces.columns.actions")
              }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="ws in workspaces" :key="ws.id" data-slot="workspace-row" class="hover:bg-accent">
              <TableCell :class="cellClass">
                <div class="flex min-w-0 flex-col gap-0.5 py-0.5">
                  <span class="text-foreground truncate text-[14px] font-medium">{{ ws.name }}</span>
                  <span v-if="ws.description" class="text-muted-foreground truncate text-[12px] leading-[1.35]">{{
                    ws.description
                  }}</span>
                </div>
              </TableCell>
              <TableCell :class="cellClass">{{ ws.member_count ?? "–" }}</TableCell>
              <TableCell :class="[cellClass, 'text-muted-foreground text-[13px]']">{{
                formatDate(ws.created_at)
              }}</TableCell>
              <TableCell :class="[cellClass, 'text-right']">
                <Button variant="ghost" size="sm" data-slot="workspace-members" @click="openMembers(ws)">
                  <UsersIcon />
                  {{ t("system.usersWorkspaces.workspaces.manageMembers") }}
                </Button>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </section>

    <!-- Users -->
    <section :aria-labelledby="'users-workspaces-users'">
      <div class="border-border flex items-start justify-between gap-4 border-b pb-3 max-[860px]:flex-col">
        <div>
          <h3 id="users-workspaces-users" class="text-foreground m-0 mb-1 text-base leading-[1.4] font-bold">
            {{ t("system.usersWorkspaces.users.title") }}
          </h3>
          <p class="text-muted-foreground m-0 text-[13px] leading-[1.5]">
            {{ t("system.usersWorkspaces.users.description") }}
          </p>
        </div>
        <Button size="sm" class="shrink-0" data-slot="user-create" @click="createUserVisible = true">
          <UserPlusIcon />
          {{ t("system.usersWorkspaces.users.create") }}
        </Button>
      </div>

      <div class="flex flex-col">
        <div :class="settingRowClass">
          <div :class="settingRowLabelClass">
            <div class="text-foreground mb-1 flex flex-wrap items-center gap-1.5 text-[15px] leading-[1.4] font-medium">
              <span>{{ t("system.usersWorkspaces.admins.label") }}</span>
              <Badge :class="settingTagClass('danger')">{{ t("system.globalSettings.badgeHighRisk") }}</Badge>
            </div>
            <p class="text-muted-foreground m-0 max-w-[480px] text-[13px] leading-[1.5]">
              {{ t("system.usersWorkspaces.admins.description") }}
            </p>
          </div>
          <div :class="settingRowControlClass">
            <SystemAdministratorsField />
          </div>
        </div>

        <div :class="settingRowClass">
          <div :class="settingRowLabelClass">
            <div class="text-foreground mb-1 flex flex-wrap items-center gap-1.5 text-[15px] leading-[1.4] font-medium">
              <span>{{ t("system.usersWorkspaces.passwordReset.label") }}</span>
              <Badge :class="settingTagClass('danger')">{{ t("system.globalSettings.badgeHighRisk") }}</Badge>
            </div>
            <p class="text-muted-foreground m-0 max-w-[480px] text-[13px] leading-[1.5]">
              {{ t("system.usersWorkspaces.passwordReset.description") }}
            </p>
          </div>
          <div :class="settingRowControlClass">
            <Button
              variant="destructive"
              class="hover:border-destructive/40 min-w-28 rounded-md px-3"
              @click="passwordResetVisible = true"
            >
              <LockIcon />
              {{ t("system.usersWorkspaces.passwordReset.action") }}
            </Button>
          </div>
        </div>
      </div>
    </section>

    <CreateTenantDialog v-model:visible="createWorkspaceVisible" @created="onWorkspaceCreated" />
    <CreateSystemUserDialog v-model:open="createUserVisible" :workspaces="workspaces" @created="onUserCreated" />
    <ResetUserPasswordDialog v-model:open="passwordResetVisible" />
    <WorkspaceMembersDrawer v-model:open="membersVisible" :workspace="membersWorkspace" @changed="loadWorkspaces" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { CircleAlertIcon, Loader2Icon, LockIcon, PlusIcon, UserPlusIcon, UsersIcon } from "@lucide/vue";
import { listAllTenants, type TenantInfo } from "@/api/tenant";
import type { CreateSystemUserResponse } from "@/api/system";
import CreateTenantDialog from "@/components/CreateTenantDialog.vue";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Empty, EmptyDescription } from "@/components/ui/empty";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import CreateSystemUserDialog from "./CreateSystemUserDialog.vue";
import ResetUserPasswordDialog from "./ResetUserPasswordDialog.vue";
import SystemAdministratorsField from "./SystemAdministratorsField.vue";
import WorkspaceMembersDrawer from "./WorkspaceMembersDrawer.vue";
import { settingRowClass, settingRowControlClass, settingRowLabelClass, settingTagClass } from "./inlineConfirm";

const { t, locale } = useI18n();

const headClass = "px-4 py-3 text-[13px] font-semibold text-muted-foreground";
const cellClass = "px-4 py-2.5 text-[14px] text-foreground";

const workspaces = ref<TenantInfo[]>([]);
const workspacesLoading = ref(false);
const workspacesError = ref("");

const createWorkspaceVisible = ref(false);
const createUserVisible = ref(false);
const passwordResetVisible = ref(false);
const membersVisible = ref(false);
const membersWorkspace = ref<TenantInfo | null>(null);

function formatDate(iso: string): string {
  try {
    return new Intl.DateTimeFormat(locale.value || "zh-CN", { dateStyle: "medium" }).format(new Date(iso));
  } catch {
    return iso;
  }
}

async function loadWorkspaces() {
  workspacesLoading.value = true;
  workspacesError.value = "";
  try {
    const resp = await listAllTenants();
    if (!resp.success || !resp.data) {
      workspacesError.value = resp.message || t("system.usersWorkspaces.workspaces.loadFailed");
      return;
    }
    workspaces.value = resp.data.items ?? [];
  } finally {
    workspacesLoading.value = false;
  }
}

function openMembers(ws: TenantInfo) {
  membersWorkspace.value = ws;
  membersVisible.value = true;
}

// The dialog already toasted; the list just needs the new row. The
// administrator stays on this page rather than being switched into the
// new workspace — they may well have created it for someone else.
function onWorkspaceCreated() {
  void loadWorkspaces();
}

// A user created straight into a workspace changes that workspace's
// member count.
function onUserCreated(response: CreateSystemUserResponse) {
  if (response.membership) void loadWorkspaces();
}

onMounted(loadWorkspaces);
</script>
