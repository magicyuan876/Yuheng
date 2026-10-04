<template>
  <div class="flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto p-[16px_28px_24px]">
    <div class="mb-3 flex flex-col gap-3">
      <Button
        variant="ghost"
        size="sm"
        class="text-muted-foreground hover:text-foreground -ml-2.5 self-start"
        @click="router.push({ name: 'docsSpace', params: { slug: route.params.slug as string } })"
      >
        <ChevronLeftIcon />
        {{ t("docs.spaces.backToSpace") }}
      </Button>
      <div v-if="space" class="flex items-center gap-3.5">
        <SpaceAvatar :name="space.name" :avatar="space.icon || ''" size="large" />
        <div class="flex min-w-0 flex-col gap-1">
          <h2 class="text-foreground m-0 text-[22px] leading-[30px] font-semibold">{{ space.name }}</h2>
          <div class="text-muted-foreground flex flex-wrap items-center gap-2 text-[13px]">
            <span class="font-[family-name:var(--td-font-family-mono,ui-monospace,monospace)]">/{{ space.slug }}</span>
            <Badge variant="secondary">{{ t("docs.spaces.visibility." + space.visibility) }}</Badge>
            <Badge class="bg-primary/10 text-primary">
              {{ t("docs.spaces.overview.yourRole") }}: {{ t("docs.spaces.role." + space.role) }}
            </Badge>
          </div>
        </div>
      </div>
    </div>

    <div v-if="loading" class="space-y-2.5 py-6">
      <Skeleton class="h-4 w-2/5" />
      <Skeleton class="h-4 w-full" />
      <Skeleton class="h-4 w-4/5" />
    </div>
    <div v-else-if="!space" class="text-muted-foreground py-6">{{ t("docs.spaces.loadFailed") }}</div>

    <Tabs v-else v-model="tab" class="w-full">
      <TabsList>
        <TabsTrigger value="overview">{{ t("docs.spaces.tabs.overview") }}</TabsTrigger>
        <TabsTrigger value="members">{{ t("docs.spaces.tabs.members") }}</TabsTrigger>
        <TabsTrigger v-if="canManage" value="settings">{{ t("docs.spaces.tabs.settings") }}</TabsTrigger>
      </TabsList>

      <!-- Overview -->
      <TabsContent value="overview" class="pt-4">
        <dl class="m-0 grid max-w-[760px] grid-cols-[max-content_minmax(0,1fr)] gap-x-6 gap-y-3">
          <dt class="text-muted-foreground text-[13px] leading-[22px]">{{ t("docs.spaces.form.description") }}</dt>
          <dd class="text-foreground m-0 flex flex-col gap-0.5 text-sm leading-[22px]">
            {{ space.description || t("docs.spaces.noDescription") }}
          </dd>
          <dt class="text-muted-foreground text-[13px] leading-[22px]">{{ t("docs.spaces.overview.slug") }}</dt>
          <dd
            class="text-foreground m-0 flex flex-col gap-0.5 font-[family-name:var(--td-font-family-mono,ui-monospace,monospace)] text-sm leading-[22px]"
          >
            {{ space.slug }}
          </dd>
          <dt class="text-muted-foreground text-[13px] leading-[22px]">{{ t("docs.spaces.overview.visibility") }}</dt>
          <dd class="text-foreground m-0 flex flex-col gap-0.5 text-sm leading-[22px]">
            {{ t("docs.spaces.visibility." + space.visibility) }}
            <span v-if="space.visibility === 'open'" class="text-placeholder text-xs">
              {{ t("docs.spaces.overview.defaultRole") }}: {{ t("docs.spaces.role." + space.default_role) }}
            </span>
          </dd>
          <dt class="text-muted-foreground text-[13px] leading-[22px]">
            {{ t("docs.spaces.overview.knowledgeBase") }}
          </dt>
          <dd class="text-foreground m-0 flex flex-col gap-0.5 text-sm leading-[22px]" data-testid="kb-bound">
            <BoundKnowledgeBase :id="space.knowledge_base_id" :knowledge-base="boundKB" :loading="!syncOptions" />
            <span class="text-placeholder text-xs">{{ t("docs.spaces.overview.knowledgeBaseHint") }}</span>
          </dd>
          <dt class="text-muted-foreground text-[13px] leading-[22px]">{{ t("docs.spaces.overview.created") }}</dt>
          <dd class="text-foreground m-0 flex flex-col gap-0.5 text-sm leading-[22px]">
            {{ formatDate(space.created_at) }}
          </dd>
          <dt class="text-muted-foreground text-[13px] leading-[22px]">{{ t("docs.spaces.overview.updated") }}</dt>
          <dd class="text-foreground m-0 flex flex-col gap-0.5 text-sm leading-[22px]">
            {{ formatDate(space.updated_at) }}
          </dd>
          <dt class="text-muted-foreground text-[13px] leading-[22px]">{{ t("docs.storage.title") }}</dt>
          <dd class="text-foreground m-0 flex flex-col gap-0.5 text-sm leading-[22px]">
            <StorageUsage :space-id="space.id" />
          </dd>
          <dt class="text-muted-foreground text-[13px] leading-[22px]">{{ t("docs.exportDoc.title") }}</dt>
          <dd class="text-foreground m-0 flex flex-col gap-0.5 text-sm leading-[22px]">
            <div class="flex items-center gap-2">
              <Select v-model="exportFormat" :disabled="exporting">
                <SelectTrigger class="w-[180px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="markdown">{{ t("docs.exportDoc.markdown") }}</SelectItem>
                  <SelectItem value="html">{{ t("docs.exportDoc.html") }}</SelectItem>
                </SelectContent>
              </Select>
              <Button size="sm" :disabled="exporting" @click="runSpaceExport">
                <Loader2Icon v-if="exporting" class="animate-spin" />
                {{ t("docs.exportDoc.exportSpace") }}
              </Button>
            </div>
            <span v-if="exporting" class="text-placeholder text-xs">{{ t("docs.exportDoc.preparing") }}</span>
          </dd>
          <template v-if="canWrite">
            <dt class="text-muted-foreground text-[13px] leading-[22px]">{{ t("docs.importDoc.title") }}</dt>
            <dd class="text-foreground m-0 flex flex-col gap-0.5 text-sm leading-[22px]">
              <div class="flex items-center gap-2">
                <input
                  ref="importInput"
                  type="file"
                  accept=".md,.markdown,.zip"
                  class="hidden"
                  :disabled="importing"
                  @change="onImportPicked"
                />
                <Button size="sm" :disabled="importing" @click="pickImport">
                  <Loader2Icon v-if="importing" class="animate-spin" />
                  {{ t("docs.importDoc.pick") }}
                </Button>
              </div>
              <span class="text-placeholder text-xs">
                {{ importing ? t("docs.importDoc.running") : t("docs.importDoc.hint") }}
              </span>
              <ul
                v-if="importSkipped.length"
                class="text-muted-foreground mt-2 mb-0 max-h-40 list-disc overflow-y-auto pl-[18px] text-xs"
              >
                <li class="-ml-[18px] list-none font-semibold">{{ t("docs.importDoc.skippedTitle") }}</li>
                <li v-for="(line, i) in importSkipped" :key="i">{{ line }}</li>
              </ul>
            </dd>
          </template>
        </dl>
      </TabsContent>

      <!-- Members -->
      <TabsContent value="members" class="pt-4">
        <div class="mb-3 flex items-center justify-between gap-3">
          <p class="text-muted-foreground m-0 text-[13px]">
            {{ canManage ? t("docs.members.hint") : t("docs.members.readOnlyHint") }}
          </p>
          <Button v-if="canManage" size="sm" @click="openAddMember">
            <UserRoundPlusIcon />
            {{ t("docs.members.add") }}
          </Button>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead v-for="c in memberColumns" :key="c.colKey" :style="{ width: c.width, minWidth: c.minWidth }">
                {{ c.title }}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <template v-if="membersLoading">
              <TableRow v-for="i in 3" :key="i">
                <TableCell v-for="c in memberColumns" :key="c.colKey"><Skeleton class="h-4 w-full" /></TableCell>
              </TableRow>
            </template>
            <TableRow v-else-if="!members.length">
              <TableCell :colspan="memberColumns.length">
                <div class="text-muted-foreground py-6 text-center text-[13px]">{{ t("docs.members.empty") }}</div>
              </TableCell>
            </TableRow>
            <TableRow v-else v-for="row in members" :key="row.key">
              <TableCell>
                <div class="flex min-w-0 items-center gap-2.5">
                  <div
                    class="flex size-7 shrink-0 items-center justify-center overflow-hidden"
                    :class="
                      row.principal_type === 'group'
                        ? 'text-primary rounded-[8px] bg-[var(--td-brand-color-light)]'
                        : 'bg-secondary text-muted-foreground rounded-full'
                    "
                  >
                    <img
                      v-if="row.principal_type === 'user' && row.avatar"
                      :src="row.avatar"
                      alt=""
                      class="size-full object-cover"
                    />
                    <UsersRoundIcon v-else-if="row.principal_type === 'group'" class="size-3.5" />
                    <UserRoundIcon v-else class="size-3.5" />
                  </div>
                  <div class="flex min-w-0 flex-col">
                    <span class="text-foreground flex items-center gap-1.5 text-sm leading-5">
                      {{ row.name }}
                      <Badge v-if="row.is_default_group" variant="outline">{{ t("docs.members.defaultGroup") }}</Badge>
                    </span>
                    <span v-if="row.principal_type === 'user'" class="text-placeholder truncate text-xs leading-4">
                      {{ row.email }}
                    </span>
                    <span v-else class="text-placeholder truncate text-xs leading-4">
                      {{ t("docs.members.groupMembers", { count: row.group_member_count ?? 0 }) }}
                    </span>
                  </div>
                </div>
              </TableCell>
              <TableCell>
                <Badge variant="secondary">{{ t("docs.members." + row.principal_type) }}</Badge>
              </TableCell>
              <TableCell>
                <Select
                  v-if="canManage"
                  :model-value="row.role"
                  :disabled="savingKey === row.key || isLastAdmin(row)"
                  @update:model-value="(v) => changeRole(row, v)"
                >
                  <SelectTrigger size="sm" class="w-[128px]">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="o in roleOptions" :key="o.value" :value="o.value">{{ o.label }}</SelectItem>
                  </SelectContent>
                </Select>
                <span v-else>{{ t("docs.spaces.role." + row.role) }}</span>
              </TableCell>
              <TableCell>{{ formatDate(row.created_at) }}</TableCell>
              <TableCell v-if="canManage">
                <!-- A disabled button swallows no pointer events, so the tooltip hangs on a
                     wrapper that does. -->
                <Tooltip v-if="isLastAdmin(row)">
                  <TooltipTrigger as-child>
                    <span class="inline-flex" tabindex="0">
                      <Button variant="ghost" size="icon-sm" class="text-destructive" disabled>
                        <Trash2Icon />
                      </Button>
                    </span>
                  </TooltipTrigger>
                  <TooltipContent>{{ t("docs.members.lastAdmin") }}</TooltipContent>
                </Tooltip>
                <Button
                  v-else
                  variant="ghost"
                  size="icon-sm"
                  class="text-destructive hover:text-destructive"
                  :disabled="savingKey === row.key"
                  @click="removeTarget = row"
                >
                  <Loader2Icon v-if="savingKey === row.key" class="animate-spin" />
                  <Trash2Icon v-else />
                </Button>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </TabsContent>

      <!-- Settings (space admins) -->
      <TabsContent v-if="canManage" value="settings" class="pt-4">
        <div class="flex max-w-[640px] flex-col gap-4">
          <SpaceForm v-model="form" mode="edit" :problems="problems" />
          <div class="flex gap-2">
            <Button :disabled="!dirty || saving" @click="saveSettings">
              <Loader2Icon v-if="saving" class="animate-spin" />
              {{ t("common.save") }}
            </Button>
            <Button variant="outline" :disabled="!dirty || saving" @click="resetForm">
              {{ t("common.cancel") }}
            </Button>
          </div>

          <section class="border-border mt-6 flex flex-col gap-3 border-t pt-5" data-testid="kb-sync-settings">
            <div class="flex flex-col gap-1">
              <span class="text-muted-foreground text-[13px]">{{ t("docs.spaces.kbSync.current") }}</span>
              <BoundKnowledgeBase :id="space.knowledge_base_id" :knowledge-base="boundKB" :loading="!syncOptions" />
            </div>
            <KnowledgeBaseSyncField
              v-model="kbChoice"
              :options="syncOptions"
              :current="boundKB"
              :disabled="bindingKB"
            />
            <p v-if="kbEffect" class="text-muted-foreground m-0 text-xs" data-testid="kb-sync-effect">{{ kbEffect }}</p>
            <div class="flex gap-2">
              <Button
                data-testid="kb-sync-save"
                :disabled="!kbDirty || !choiceComplete(kbChoice) || bindingKB"
                @click="saveKnowledgeBase"
              >
                <Loader2Icon v-if="bindingKB" class="animate-spin" />
                {{ t("common.save") }}
              </Button>
              <Button variant="outline" :disabled="!kbDirty || bindingKB" @click="resetKnowledgeBase">
                {{ t("common.cancel") }}
              </Button>
            </div>
          </section>

          <div class="mt-6 rounded-[8px] border border-[var(--td-error-color-3)] bg-[var(--td-error-color-1)] p-4">
            <div class="text-destructive mb-1 font-semibold">{{ t("docs.spaces.dangerZone") }}</div>
            <p class="text-muted-foreground m-0 mb-3 text-[13px]">{{ t("docs.spaces.dangerZoneHint") }}</p>
            <Button
              variant="outline"
              class="text-destructive hover:text-destructive hover:bg-destructive/10 border-[var(--td-error-color)] bg-transparent dark:border-[var(--td-error-color)]"
              @click="deleteVisible = true"
            >
              {{ t("docs.spaces.deleteSpace") }}
            </Button>
          </div>
        </div>
      </TabsContent>
    </Tabs>

    <!-- Add member dialog -->
    <Dialog :open="addVisible" @update:open="(v: boolean) => (addVisible = v)">
      <DialogContent class="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>{{ t("docs.members.addTitle") }}</DialogTitle>
        </DialogHeader>
        <form class="flex flex-col gap-4" @submit.prevent>
          <div class="flex flex-col gap-1.5">
            <Label>{{ t("docs.members.principalType") }}</Label>
            <RadioGroup
              v-model="addForm.principal_type"
              class="flex gap-4"
              @update:model-value="addForm.principal_id = ''"
            >
              <div class="flex items-center gap-2">
                <RadioGroupItem id="principal-type-user" value="user" />
                <Label for="principal-type-user" class="font-normal">{{ t("docs.members.user") }}</Label>
              </div>
              <div class="flex items-center gap-2">
                <RadioGroupItem id="principal-type-group" value="group" />
                <Label for="principal-type-group" class="font-normal">{{ t("docs.members.group") }}</Label>
              </div>
            </RadioGroup>
          </div>

          <div v-if="addForm.principal_type === 'user'" class="flex flex-col gap-1.5">
            <Label>{{ t("docs.members.pickUser") }}</Label>
            <Popover v-model:open="userPickOpen">
              <PopoverTrigger as-child>
                <Button type="button" variant="outline" class="w-full justify-between font-normal">
                  <span class="truncate">{{ pickedUserLabel || t("docs.members.pickUserPlaceholder") }}</span>
                  <ChevronsUpDownIcon class="text-placeholder size-3.5" />
                </Button>
              </PopoverTrigger>
              <PopoverContent class="w-(--reka-popover-trigger-width) gap-0 p-0" align="start">
                <div class="border-border border-b p-2">
                  <Input
                    v-model="userQuery"
                    class="h-8"
                    :placeholder="t('docs.members.pickUserPlaceholder')"
                    @input="onUserQuery"
                  />
                </div>
                <div class="max-h-56 overflow-y-auto p-1">
                  <div
                    v-if="memberSearch.loading.value"
                    class="text-muted-foreground flex items-center justify-center gap-2 py-4 text-[13px]"
                  >
                    <Loader2Icon class="size-3.5 animate-spin" />
                  </div>
                  <template v-else>
                    <button
                      v-for="o in memberSearch.options.value"
                      :key="o.value"
                      type="button"
                      class="hover:bg-accent focus:bg-accent flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none"
                      @click="selectUserOption(o)"
                    >
                      <CheckIcon
                        class="size-3.5"
                        :class="o.value === addForm.principal_id ? 'opacity-100' : 'opacity-0'"
                      />
                      <span class="truncate">{{ o.label }}</span>
                    </button>
                    <div
                      v-if="!memberSearch.options.value.length"
                      class="text-placeholder py-4 text-center text-[13px]"
                    >
                      {{ t("common.noData") }}
                    </div>
                  </template>
                </div>
              </PopoverContent>
            </Popover>
          </div>

          <div v-else class="flex flex-col gap-1.5">
            <Label>{{ t("docs.members.pickGroup") }}</Label>
            <Select v-model="addForm.principal_id">
              <SelectTrigger>
                <SelectValue :placeholder="t('docs.members.pickGroupPlaceholder')" />
              </SelectTrigger>
              <SelectContent>
                <div
                  v-if="groupsLoading"
                  class="text-muted-foreground flex items-center justify-center gap-2 py-3 text-[13px]"
                >
                  <Loader2Icon class="size-3.5 animate-spin" />
                </div>
                <template v-else>
                  <SelectItem v-for="g in groupOptions" :key="g.value" :value="g.value" :disabled="g.disabled">
                    {{ g.label }}
                  </SelectItem>
                </template>
              </SelectContent>
            </Select>
          </div>

          <div class="flex flex-col gap-1.5">
            <Label>{{ t("docs.members.role") }}</Label>
            <Select v-model="addForm.role">
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="o in roleOptions" :key="o.value" :value="o.value">{{ o.label }}</SelectItem>
              </SelectContent>
            </Select>
            <p class="text-muted-foreground m-0 text-xs">{{ t("docs.members.roleHint") }}</p>
          </div>
        </form>
        <DialogFooter>
          <Button variant="outline" @click="addVisible = false">{{ t("common.cancel") }}</Button>
          <Button :disabled="!addForm.principal_id || adding" @click="submitAddMember">
            <Loader2Icon v-if="adding" class="animate-spin" />
            {{ t("common.add") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Remove member confirm -->
    <Dialog :open="removeTarget !== null" @update:open="(v: boolean) => !v && (removeTarget = null)">
      <DialogContent class="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>{{ t("docs.members.removeConfirm", { name: removeTarget?.name ?? "" }) }}</DialogTitle>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="removeTarget = null">{{ t("common.cancel") }}</Button>
          <Button variant="destructive" :disabled="removing" @click="confirmRemove">
            <Loader2Icon v-if="removing" class="animate-spin" />
            {{ t("common.delete") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete space dialog -->
    <Dialog :open="deleteVisible" @update:open="(v: boolean) => (deleteVisible = v)">
      <DialogContent class="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>{{ t("docs.spaces.deleteConfirmTitle") }}</DialogTitle>
        </DialogHeader>
        <p class="text-foreground text-sm">{{ t("docs.spaces.deleteConfirm", { name: space?.name ?? "" }) }}</p>
        <DialogFooter>
          <Button variant="outline" @click="deleteVisible = false">{{ t("common.cancel") }}</Button>
          <Button variant="destructive" :disabled="deleting" @click="confirmDelete">
            <Loader2Icon v-if="deleting" class="animate-spin" />
            {{ t("common.delete") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import {
  ChevronsUpDownIcon,
  CheckIcon,
  ChevronLeftIcon,
  Loader2Icon,
  Trash2Icon,
  UserRoundIcon,
  UserRoundPlusIcon,
  UsersRoundIcon,
} from "@lucide/vue";
import { MessagePlugin } from "tdesign-vue-next";
import { computed, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

import {
  bindSpaceKnowledgeBase,
  deleteSpace,
  getSpaceBySlug,
  listSpaceMembers,
  removeSpaceMember,
  setSpaceMembers,
  updateSpace,
  type DocsSpace,
  type PrincipalType,
  type SpaceMember,
  type SpaceRole,
  downloadExport,
  getImportJob,
  getExportJob,
  startImport,
  startSpaceExport,
  type ExportFormat,
  type ExportJobView,
  type ImportJobView,
  type KnowledgeBaseChoice,
} from "@/api/docs";
import { listGroups, type TenantGroup } from "@/api/tenant/groups";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import SpaceAvatar from "@/components/SpaceAvatar.vue";
import { useAuthStore } from "@/stores/auth";

import StorageUsage from "./quota/StorageUsage.vue";

import {
  canManageSpace,
  GRANTABLE_SPACE_ROLES,
  normaliseSpaceForm,
  sortSpaceMembers,
  validateSpaceForm,
  wouldLeaveNoAdmin,
  type SpaceFormModel,
  type SpaceFormProblem,
} from "./docsAccess";
import BoundKnowledgeBase from "./BoundKnowledgeBase.vue";
import KnowledgeBaseSyncField from "./KnowledgeBaseSyncField.vue";
import {
  choiceComplete,
  choiceRequest,
  currentChoice,
  isEmbeddingModelRequired,
  loadSyncOptions,
  sameChoice,
  type KnowledgeBaseSummary,
  type SyncOptions,
} from "./knowledgeBaseSync";
import SpaceForm from "./SpaceForm.vue";
import { useMemberSearch, type MemberOption } from "./useMemberSearch";

type MemberRow = SpaceMember & { key: string };

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

const space = ref<DocsSpace | null>(null);
const loading = ref(true);
const tab = ref<"overview" | "members" | "settings">("overview");
const canManage = computed(() => canManageSpace(space.value?.role));
// Importing needs write access, which is a lower bar than managing the space.
const canWrite = computed(() => space.value?.role === "writer" || canManage.value);

const slugParam = computed(() => String(route.params.slug ?? ""));

const errorText = (err: unknown, fallback: string): string => {
  const msg = (err as { message?: string } | null)?.message;
  return msg ? `${fallback}: ${msg}` : fallback;
};

const formatDate = (iso: string) => {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
};

const load = async () => {
  loading.value = true;
  try {
    space.value = await getSpaceBySlug(slugParam.value);
    resetForm();
    resetKnowledgeBase();
    void loadKnowledgeBaseOptions();
    await loadMembers();
  } catch (err: unknown) {
    space.value = null;
    MessagePlugin.error(errorText(err, t("docs.spaces.loadFailed")));
  } finally {
    loading.value = false;
  }
};

// ---- members ------------------------------------------------------------------
const members = ref<MemberRow[]>([]);
const membersLoading = ref(false);
const savingKey = ref("");

const rowKey = (m: SpaceMember) => `${m.principal_type}:${m.principal_id}`;
const setMembers = (rows: SpaceMember[]) => {
  members.value = sortSpaceMembers(rows).map((m) => ({ ...m, key: rowKey(m) }));
};

const loadMembers = async () => {
  if (!space.value) return;
  membersLoading.value = true;
  try {
    setMembers(await listSpaceMembers(space.value.id));
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.members.loadFailed")));
  } finally {
    membersLoading.value = false;
  }
};

const roleOptions = computed(() => GRANTABLE_SPACE_ROLES.map((r) => ({ label: t("docs.spaces.role." + r), value: r })));

interface MemberColumn {
  colKey: string;
  title: string;
  width?: string;
  minWidth?: string;
}

const memberColumns = computed<MemberColumn[]>(() => {
  const cols: MemberColumn[] = [
    { colKey: "member", title: t("docs.members.columns.member"), minWidth: "220px" },
    { colKey: "type", title: t("docs.members.columns.type"), width: "96px" },
    { colKey: "role", title: t("docs.members.columns.role"), width: "150px" },
    { colKey: "added", title: t("docs.members.columns.added"), width: "180px" },
  ];
  if (canManage.value) cols.push({ colKey: "actions", title: t("docs.members.columns.actions"), width: "72px" });
  return cols;
});

const isLastAdmin = (row: MemberRow) =>
  row.role === "admin" && wouldLeaveNoAdmin(members.value, { ...row, role: null });

const changeRole = async (row: MemberRow, value: unknown) => {
  if (!space.value) return;
  const role = value as SpaceRole;
  if (role === row.role) return;
  if (wouldLeaveNoAdmin(members.value, { ...row, role })) {
    MessagePlugin.warning(t("docs.members.lastAdmin"));
    return;
  }
  savingKey.value = row.key;
  try {
    setMembers(
      await setSpaceMembers(space.value.id, [
        { principal_type: row.principal_type, principal_id: row.principal_id, role },
      ]),
    );
    MessagePlugin.success(t("docs.members.saveSuccess"));
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.members.saveFailed")));
  } finally {
    savingKey.value = "";
  }
};

const removeTarget = ref<MemberRow | null>(null);
const removing = ref(false);

const confirmRemove = async () => {
  const row = removeTarget.value;
  if (!space.value || !row) return;
  savingKey.value = row.key;
  removing.value = true;
  try {
    await removeSpaceMember(space.value.id, row.principal_type, row.principal_id);
    members.value = members.value.filter((m) => m.key !== row.key);
    MessagePlugin.success(t("docs.members.removeSuccess"));
    removeTarget.value = null;
    // Removing yourself may have cost you admin rights; reload the space.
    await load();
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.members.removeFailed")));
  } finally {
    savingKey.value = "";
    removing.value = false;
  }
};

// ---- add member ----------------------------------------------------------------
const addVisible = ref(false);
const adding = ref(false);
const addForm = ref<{ principal_type: PrincipalType; principal_id: string; role: SpaceRole }>({
  principal_type: "user",
  principal_id: "",
  role: "reader",
});
const memberSearch = useMemberSearch();
const groups = ref<TenantGroup[]>([]);
const groupsLoading = ref(false);
const groupOptions = computed(() => {
  const present = new Set(members.value.filter((m) => m.principal_type === "group").map((m) => m.principal_id));
  return groups.value.map((g) => ({
    label: g.is_default ? `${g.name} (${t("docs.members.defaultGroup")})` : g.name,
    value: g.id,
    disabled: present.has(g.id),
  }));
});

// Remote-search user picker (the t-select `filterable` pattern): popover with a
// debounced search input feeding the shared member-search composable.
const userPickOpen = ref(false);
const userQuery = ref("");

const pickedUserLabel = computed(
  () => memberSearch.options.value.find((o) => o.value === addForm.value.principal_id)?.label ?? "",
);

const onUserQuery = () => memberSearch.search(userQuery.value);

const selectUserOption = (o: MemberOption) => {
  addForm.value.principal_id = o.value;
  userPickOpen.value = false;
};

const openAddMember = async () => {
  addForm.value = { principal_type: "user", principal_id: "", role: "reader" };
  userQuery.value = "";
  addVisible.value = true;
  void memberSearch.load("");
  groupsLoading.value = true;
  try {
    groups.value = await listGroups();
  } catch {
    groups.value = [];
  } finally {
    groupsLoading.value = false;
  }
};

const submitAddMember = async () => {
  if (!space.value || !addForm.value.principal_id) return;
  adding.value = true;
  try {
    setMembers(await setSpaceMembers(space.value.id, [{ ...addForm.value }]));
    MessagePlugin.success(t("docs.members.saveSuccess"));
    addVisible.value = false;
    space.value.member_count = members.value.length;
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.members.saveFailed")));
  } finally {
    adding.value = false;
  }
};

// ---- settings -------------------------------------------------------------------
const form = ref<SpaceFormModel>({ name: "", slug: "", description: "", visibility: "private", default_role: "none" });
const problems = ref<SpaceFormProblem[]>([]);
const saving = ref(false);

const resetForm = () => {
  if (!space.value) return;
  form.value = {
    name: space.value.name,
    slug: space.value.slug,
    description: space.value.description,
    visibility: space.value.visibility,
    default_role: space.value.default_role,
  };
  problems.value = [];
};

const dirty = computed(() => {
  const s = space.value;
  if (!s) return false;
  const f = form.value;
  return (
    f.name.trim() !== s.name ||
    f.slug.trim() !== s.slug ||
    f.description.trim() !== s.description ||
    f.visibility !== s.visibility ||
    (f.visibility === "open" && f.default_role !== s.default_role)
  );
});

const saveSettings = async () => {
  if (!space.value) return;
  const model = normaliseSpaceForm({ ...form.value, name: form.value.name.trim(), slug: form.value.slug.trim() });
  problems.value = validateSpaceForm(model, { requireSlug: true });
  if (problems.value.length) return;
  saving.value = true;
  try {
    const slugChanged = model.slug !== space.value.slug;
    space.value = await updateSpace(space.value.id, {
      name: model.name,
      slug: model.slug,
      description: model.description.trim(),
      visibility: model.visibility,
      default_role: model.default_role,
    });
    resetForm();
    MessagePlugin.success(t("docs.spaces.updateSuccess"));
    if (slugChanged) {
      router.replace({ name: "docsSpaceSettings", params: { slug: space.value.slug } });
    }
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.spaces.updateFailed")));
  } finally {
    saving.value = false;
  }
};

// ---- knowledge base ---------------------------------------------------------------
// The knowledge base the space syncs into, named rather than shown as an id,
// and changeable by the space's administrators. A change queues every page at
// once on the server: mirrors leave the old knowledge base, and on a rebinding
// are written anew into the new one, which means embedding them again. The
// note under the field says which of these is about to happen before Save.
const syncOptions = ref<SyncOptions | null>(null);
const kbChoice = ref<KnowledgeBaseChoice>({ mode: "none" });
const bindingKB = ref(false);

const loadKnowledgeBaseOptions = async () => {
  syncOptions.value = null;
  syncOptions.value = await loadSyncOptions({
    userId: authStore.currentUserId,
    isAdmin: authStore.hasRole("admin"),
  });
};

const boundKB = computed<KnowledgeBaseSummary | null>(() => {
  const id = space.value?.knowledge_base_id;
  if (!id) return null;
  return syncOptions.value?.all.find((kb) => kb.id === id) ?? null;
});

const resetKnowledgeBase = () => {
  kbChoice.value = currentChoice(space.value?.knowledge_base_id);
};

const kbDirty = computed(() => !sameChoice(kbChoice.value, currentChoice(space.value?.knowledge_base_id)));

const kbEffect = computed(() => {
  if (!kbDirty.value) return "";
  const from = space.value?.knowledge_base_id;
  if (!from) return kbChoice.value.mode === "none" ? "" : t("docs.spaces.kbSync.bindEffect");
  const name = boundKB.value?.name || from;
  return kbChoice.value.mode === "none"
    ? t("docs.spaces.kbSync.unbindEffect", { name })
    : t("docs.spaces.kbSync.rebindEffect", { name });
});

const saveKnowledgeBase = async () => {
  if (!space.value) return;
  bindingKB.value = true;
  try {
    space.value = await bindSpaceKnowledgeBase(space.value.id, { knowledge_base: choiceRequest(kbChoice.value) });
    MessagePlugin.success(t("docs.spaces.kbSync.saveSuccess"));
    resetKnowledgeBase();
    // A knowledge base made by this change is not in the list yet.
    void loadKnowledgeBaseOptions();
  } catch (err: unknown) {
    if (isEmbeddingModelRequired(err)) {
      MessagePlugin.error(t("docs.spaces.kbSync.noEmbedding"));
      void loadKnowledgeBaseOptions();
    } else {
      MessagePlugin.error(errorText(err, t("docs.spaces.kbSync.saveFailed")));
    }
  } finally {
    bindingKB.value = false;
  }
};

// ---- delete ---------------------------------------------------------------------
const deleteVisible = ref(false);
const deleting = ref(false);
const confirmDelete = async () => {
  if (!space.value) return;
  deleting.value = true;
  try {
    await deleteSpace(space.value.id);
    MessagePlugin.success(t("docs.spaces.deleteSuccess"));
    deleteVisible.value = false;
    router.push({ name: "docsSpaceList" });
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.spaces.deleteFailed")));
  } finally {
    deleting.value = false;
  }
};

watch(slugParam, (next, prev) => {
  if (next && next !== prev) void load();
});
onMounted(load);
// Exporting the whole space.
//
// Asynchronous, because a space is a thousand pages and a zip: the request
// starts a job and this polls it until the archive is ready, then saves it.
// The archive holds only the pages this person can read, which is why the
// result message reports what was left out rather than pretending it is
// complete.
const exportFormat = ref<ExportFormat>("markdown");
const exporting = ref(false);

// Polling stops after this long. A job that has not finished by then has not
// failed — the operator can come back to it — but this page should not keep
// asking forever.
const EXPORT_POLL_MS = 1500;
const EXPORT_DEADLINE_MS = 10 * 60 * 1000;

async function runSpaceExport() {
  if (exporting.value || !space.value) return;
  exporting.value = true;
  try {
    const job = await startSpaceExport(space.value.id, exportFormat.value);
    const done = await waitForExport(job.id);
    if (!done) return;
    if (done.status === "failed") {
      void MessagePlugin.error(done.error || t("docs.exportDoc.failed"));
      return;
    }
    await downloadExport(done.id, done.file_name || "export.zip");
    if (done.skipped > 0) {
      void MessagePlugin.warning(t("docs.exportDoc.partial", { count: done.exported, skipped: done.skipped }));
    } else {
      void MessagePlugin.success(t("docs.exportDoc.done", { count: done.exported }));
    }
  } catch (err) {
    void MessagePlugin.error(errorText(err, t("docs.exportDoc.failed")));
  } finally {
    exporting.value = false;
  }
}

/** Polls one job to completion, or gives up at the deadline. */
async function waitForExport(jobId: string): Promise<ExportJobView | null> {
  const deadline = Date.now() + EXPORT_DEADLINE_MS;
  for (;;) {
    const job = await getExportJob(jobId);
    if (job.status !== "pending" && job.status !== "running") return job;
    if (Date.now() > deadline) {
      void MessagePlugin.warning(t("docs.exportDoc.preparing"));
      return null;
    }
    await new Promise((resolve) => setTimeout(resolve, EXPORT_POLL_MS));
  }
}

// Importing a bundle into this space.
//
// Asynchronous like the export: the upload starts a job and this polls it.
// The skipped list is kept on screen after the job ends rather than shown as
// a toast that disappears -- it names the files that did not make it, and
// somebody needs to be able to read it and go and look at them.
const importInput = ref<HTMLInputElement | null>(null);
const importing = ref(false);
const importSkipped = ref<string[]>([]);

function pickImport() {
  importInput.value?.click();
}

async function onImportPicked(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  // The input is cleared either way, so picking the same file twice in a row
  // still fires a change event.
  input.value = "";
  if (!file || !space.value || importing.value) return;

  importing.value = true;
  importSkipped.value = [];
  try {
    const job = await startImport(space.value.id, file);
    const done = await waitForImport(job.id);
    if (!done) return;
    importSkipped.value = done.skipped ?? [];
    if (done.status === "failed") {
      void MessagePlugin.error(done.error || t("docs.importDoc.failed"));
      return;
    }
    if (done.skipped?.length) {
      void MessagePlugin.warning(t("docs.importDoc.partial", { count: done.created, skipped: done.skipped.length }));
    } else {
      void MessagePlugin.success(t("docs.importDoc.done", { count: done.created }));
    }
    if (done.attachments > 0) {
      void MessagePlugin.info(t("docs.importDoc.attachments", { count: done.attachments }));
    }
  } catch (err) {
    void MessagePlugin.error(errorText(err, t("docs.importDoc.failed")));
  } finally {
    importing.value = false;
  }
}

/** Polls one import to completion, or gives up at the deadline. */
async function waitForImport(jobId: string): Promise<ImportJobView | null> {
  const deadline = Date.now() + EXPORT_DEADLINE_MS;
  for (;;) {
    const job = await getImportJob(jobId);
    if (job.done) return job;
    if (Date.now() > deadline) {
      void MessagePlugin.warning(t("docs.importDoc.running"));
      return null;
    }
    await new Promise((resolve) => setTimeout(resolve, EXPORT_POLL_MS));
  }
}
</script>
