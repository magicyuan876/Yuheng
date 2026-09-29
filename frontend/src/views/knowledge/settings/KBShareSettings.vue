<template>
  <!-- share-to-space-panel stays as an unstyled root hook class. -->
  <div class="share-to-space-panel flex flex-col gap-4">
    <div>
      <div class="flex min-w-0 items-center">
        <div class="inline-flex min-w-0 items-center gap-1">
          <h2 class="text-foreground m-0 text-xl leading-[1.35] font-semibold">{{ $t("organization.share.title") }}</h2>
          <!--
            The old t-popup opened on hover. Reka's Popover only opens on click,
            so the trigger and the content drive `hintOpen` from pointer events,
            with a short grace period to let the pointer travel between them.
          -->
          <Popover :open="hintOpen" @update:open="(v: boolean) => (hintOpen = v)">
            <PopoverTrigger as-child>
              <button
                type="button"
                data-slot="icon-button"
                class="text-muted-foreground hover:bg-muted hover:text-primary focus-visible:outline-ring inline-flex size-[22px] shrink-0 items-center justify-center rounded-md leading-none transition-colors duration-200 focus-visible:outline-2 focus-visible:outline-offset-1"
                :aria-label="$t('knowledgeEditor.share.hintTitle')"
                :title="$t('knowledgeEditor.share.hintTitle')"
                @pointerenter="openHint"
                @pointerleave="closeHintSoon"
              >
                <InfoIcon class="size-4" />
              </button>
            </PopoverTrigger>
            <PopoverContent
              align="start"
              class="border-border max-h-[min(280px,65vh)] w-[min(400px,calc(100vw-24px))] max-w-[min(400px,calc(100vw-24px))] overflow-hidden rounded-xl border-[0.5px] p-0 shadow-[0_0_0_0.5px_rgba(0,0,0,0.03),0_2px_4px_rgba(0,0,0,0.04),0_8px_24px_rgba(0,0,0,0.1)] ring-0 backdrop-blur-[20px] backdrop-saturate-[180%]"
              @pointerenter="openHint"
              @pointerleave="closeHintSoon"
            >
              <div class="px-4 py-3.5">
                <p class="text-foreground m-0 mb-1.5 text-sm leading-[1.35] font-semibold">
                  {{ $t("knowledgeEditor.share.hintTitle") }}
                </p>
                <p class="text-muted-foreground m-0 mb-2 text-[13px] leading-[1.55]">
                  {{ $t("knowledgeEditor.share.tip1") }}
                </p>
                <p class="text-muted-foreground m-0 text-[13px] leading-[1.55]">
                  {{ $t("knowledgeEditor.share.tip2") }}
                </p>
              </div>
            </PopoverContent>
          </Popover>
        </div>
      </div>
      <p class="text-muted-foreground mt-2 mb-0 text-sm leading-normal">
        {{ $t("knowledgeEditor.share.description") }}
      </p>
    </div>

    <div class="flex flex-col gap-2.5">
      <div class="flex flex-wrap items-center justify-between gap-3 px-0.5">
        <div class="inline-flex min-w-0 items-center gap-2">
          <span class="text-foreground text-sm font-semibold">{{ $t("organization.share.sharedTo") }}</span>
          <span
            class="bg-muted text-foreground inline-flex h-5 min-w-[22px] items-center justify-center rounded-[10px] px-[7px] text-xs leading-none font-semibold"
            >{{ filteredShares.length }}</span
          >
        </div>
        <div class="inline-flex min-w-0 flex-[0_1_auto] items-center gap-2">
          <div class="relative w-56 min-w-0 flex-[0_0_14rem]">
            <SearchIcon
              class="text-placeholder pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2"
            />
            <Input
              v-model="searchQuery"
              :placeholder="$t('organization.share.searchPlaceholder')"
              class="h-6 px-7 text-xs md:text-xs"
            />
            <!-- t-input's `clearable`. -->
            <button
              v-if="searchQuery"
              type="button"
              data-slot="icon-button"
              class="text-placeholder hover:text-muted-foreground absolute top-1/2 right-1.5 inline-flex -translate-y-1/2 items-center"
              :aria-label="$t('common.clear')"
              @click="searchQuery = ''"
            >
              <CircleXIcon class="size-3.5" />
            </button>
          </div>
          <Popover v-if="canShare" v-model:open="addPopupVisible">
            <PopoverTrigger as-child>
              <!-- t-button theme="primary" variant="outline" shape="square" size="small": 24px, brand outline. -->
              <Button
                variant="outline"
                size="icon-xs"
                class="border-primary text-primary hover:bg-primary/10 hover:text-primary dark:border-primary shrink-0"
                :title="$t('knowledgeEditor.share.addShare')"
                :aria-label="$t('knowledgeEditor.share.addShare')"
              >
                <PlusIcon />
              </Button>
            </PopoverTrigger>
            <PopoverContent
              align="end"
              class="border-border w-[min(360px,calc(100vw-32px))] max-w-full gap-0 rounded-[10px] border p-4 shadow-[var(--td-shadow-2),0_8px_24px_rgba(15,23,42,0.08)] ring-0"
            >
              <div @click.stop>
                <div class="text-foreground m-0 mb-2.5 text-[15px] leading-[1.35] font-semibold">
                  {{ $t("organization.share.addShareDialogTitle") }}
                </div>
                <div class="flex flex-col gap-3.5">
                  <div class="flex w-full min-w-0 flex-col gap-2">
                    <label class="text-foreground m-0 block text-sm leading-[1.4] font-medium">{{
                      $t("organization.share.selectOrg")
                    }}</label>
                    <ShareToSpaceOrgSelect
                      v-model="selectedOrgId"
                      :organizations="availableOrganizations"
                      :loading="loadingOrgs"
                    />
                  </div>
                  <div class="flex w-full min-w-0 flex-col gap-2">
                    <label class="text-foreground m-0 block text-sm leading-[1.4] font-medium">{{
                      $t("organization.share.permission")
                    }}</label>
                    <Select v-model="selectedPermission">
                      <SelectTrigger class="w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="viewer">{{ $t("organization.share.permissionReadonly") }}</SelectItem>
                        <SelectItem value="editor">{{ $t("organization.share.permissionEditable") }}</SelectItem>
                      </SelectContent>
                    </Select>
                    <p class="text-placeholder m-0 text-xs leading-[1.45]">
                      {{ $t("organization.share.permissionTip") }}
                    </p>
                  </div>
                </div>
                <div class="border-border mt-4 flex justify-end gap-2 border-t pt-3">
                  <Button variant="outline" :disabled="submitting" @click="addPopupVisible = false">
                    {{ $t("common.cancel") }}
                  </Button>
                  <Button :disabled="!selectedOrgId || submitting" @click="handleShare">
                    <Loader2Icon v-if="submitting" class="animate-spin" />
                    {{ $t("knowledgeEditor.share.addShare") }}
                  </Button>
                </div>
              </div>
            </PopoverContent>
          </Popover>
        </div>
      </div>

      <div
        v-if="loadingShares && shares.length === 0"
        class="text-muted-foreground flex items-center gap-2 pt-5 pb-2 text-[13px]"
      >
        <Loader2Icon class="size-4 animate-spin" />
        <span>{{ $t("organization.share.loading") }}</span>
      </div>
      <div v-else-if="filteredShares.length === 0" class="pt-2 pb-4">
        <Empty>
          <EmptyDescription>{{
            searchQuery.trim()
              ? $t("organization.share.emptySearch", { q: searchQuery })
              : $t("organization.share.noShares")
          }}</EmptyDescription>
        </Empty>
      </div>
      <div v-else class="border-border bg-card relative overflow-x-auto rounded-[10px] border">
        <Table>
          <TableHeader>
            <TableRow class="bg-muted hover:bg-muted">
              <TableHead class="min-w-[180px] py-3 text-[13px] font-semibold">
                {{ $t("organization.share.columns.space") }}
              </TableHead>
              <TableHead class="w-[132px] py-3 text-[13px] font-semibold">
                {{ $t("organization.share.columns.permission") }}
              </TableHead>
              <TableHead class="w-[154px] py-3 text-[13px] font-semibold">
                {{ $t("organization.share.columns.sharedAt") }}
              </TableHead>
              <TableHead v-if="canShare" class="w-[72px] py-3 text-left text-[13px] font-semibold">
                {{ $t("organization.share.columns.operations") }}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <!-- t-table `stripe` and `hover`. -->
            <TableRow v-for="row in filteredShares" :key="row.id" class="even:bg-muted/50 hover:bg-accent">
              <TableCell class="max-w-0 min-w-[180px] py-3 align-middle">
                <div class="flex min-w-0 flex-col gap-0.5 py-0.5">
                  <span class="text-foreground inline-flex min-w-0 items-center gap-2 text-sm font-medium">
                    <SpaceAvatar
                      :name="row.organization_name || ''"
                      :avatar="getOrgForShare(row.organization_id)?.avatar"
                      size="small"
                    />
                    <span class="truncate">{{ row.organization_name }}</span>
                  </span>
                  <span v-if="row.shared_by_username" class="text-muted-foreground truncate text-xs">
                    {{ $t("organization.share.sharedFrom") }} {{ row.shared_by_username }}
                  </span>
                </div>
              </TableCell>
              <TableCell class="py-3 align-middle">
                <div class="flex min-w-0 items-center">
                  <Select
                    v-if="canShare"
                    :model-value="row.permission"
                    @update:model-value="(val) => handleUpdatePermission(row, String(val))"
                  >
                    <SelectTrigger size="sm" class="w-full max-w-[120px]">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="viewer">{{ $t("organization.share.permissionReadonly") }}</SelectItem>
                      <SelectItem value="editor">{{ $t("organization.share.permissionEditable") }}</SelectItem>
                    </SelectContent>
                  </Select>
                  <Badge
                    v-else
                    :class="
                      row.permission === 'editor' || row.permission === 'admin'
                        ? 'bg-warning/10 text-warning'
                        : 'bg-muted text-muted-foreground'
                    "
                  >
                    {{ permissionLabel(row.permission) }}
                  </Badge>
                </div>
              </TableCell>
              <TableCell class="py-3 align-middle">
                {{ formatShareDate(row.created_at) }}
              </TableCell>
              <TableCell v-if="canShare" class="py-3 text-left align-middle">
                <div class="inline-flex items-center gap-0.5">
                  <!-- t-popconfirm placement="left": a small popover with the question and two buttons. -->
                  <Popover
                    :open="unshareTarget?.id === row.id"
                    @update:open="(v: boolean) => (unshareTarget = v ? row : null)"
                  >
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <PopoverTrigger as-child>
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            class="text-destructive hover:text-destructive hover:bg-destructive/10"
                            :aria-label="$t('organization.share.unshareAction')"
                            @click.stop
                          >
                            <Trash2Icon />
                          </Button>
                        </PopoverTrigger>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("organization.share.unshareAction") }}</TooltipContent>
                    </Tooltip>
                    <PopoverContent side="left" class="w-auto max-w-[320px] gap-3 p-3">
                      <p class="text-foreground m-0 text-sm">
                        {{ $t("knowledgeEditor.share.unshareConfirm", { name: row.organization_name }) }}
                      </p>
                      <div class="flex justify-end gap-2">
                        <Button variant="outline" size="sm" @click="unshareTarget = null">
                          {{ $t("common.cancel") }}
                        </Button>
                        <Button
                          size="sm"
                          class="bg-destructive hover:bg-destructive/90 text-white"
                          @click="
                            unshareTarget = null;
                            handleUnshare(row);
                          "
                        >
                          {{ $t("common.confirm") }}
                        </Button>
                      </div>
                    </PopoverContent>
                  </Popover>
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
        <!-- t-table `loading`: a spinner over the rows while they are re-fetched. -->
        <div v-if="loadingShares" class="bg-card/60 absolute inset-0 flex items-center justify-center">
          <Loader2Icon class="text-primary size-5 animate-spin" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import { CircleXIcon, InfoIcon, Loader2Icon, PlusIcon, SearchIcon, Trash2Icon } from "@lucide/vue";
import { useOrganizationStore } from "@/stores/organization";
import { listKBShares } from "@/api/organization";
import type { KnowledgeBaseShare } from "@/api/organization";
import SpaceAvatar from "@/components/SpaceAvatar.vue";
import ShareToSpaceOrgSelect from "@/components/ShareToSpaceOrgSelect.vue";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Empty, EmptyDescription } from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

const { t } = useI18n();
const orgStore = useOrganizationStore();

function getOrgForShare(organizationId: string) {
  return orgStore.organizations.find((o) => o.id === organizationId);
}

interface Props {
  kbId: string;
  canShare?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  canShare: false,
});

const loadingOrgs = ref(false);
const loadingShares = ref(false);
const submitting = ref(false);
const addPopupVisible = ref(false);
const searchQuery = ref("");
const selectedOrgId = ref("");
const selectedPermission = ref<"viewer" | "editor">("viewer");
// The row whose unshare confirmation is open (t-popconfirm kept that state itself).
const unshareTarget = ref<(KnowledgeBaseShare & { organization_name?: string }) | null>(null);

// Hover-open state of the share hint (see the template). The timer lets the
// pointer cross the gap between the trigger and the content without closing it.
const hintOpen = ref(false);
let hintCloseTimer: ReturnType<typeof setTimeout> | undefined;
function openHint() {
  clearTimeout(hintCloseTimer);
  hintOpen.value = true;
}
function closeHintSoon() {
  clearTimeout(hintCloseTimer);
  hintCloseTimer = setTimeout(() => (hintOpen.value = false), 150);
}
onBeforeUnmount(() => clearTimeout(hintCloseTimer));
const shares = ref<(KnowledgeBaseShare & { organization_name?: string })[]>([]);

const availableOrganizations = computed(() => {
  const sharedOrgIds = new Set(shares.value.map((s) => s.organization_id));
  return orgStore.organizations.filter(
    (org) =>
      !sharedOrgIds.has(org.id) && (org.is_owner === true || org.my_role === "admin" || org.my_role === "editor"),
  );
});

const filteredShares = computed(() => {
  const query = searchQuery.value.trim().toLowerCase();
  if (!query) return shares.value;
  return shares.value.filter((share) => {
    const haystack = [share.organization_name, share.shared_by_username, permissionLabel(share.permission)]
      .filter(Boolean)
      .join(" ")
      .toLowerCase();
    return haystack.includes(query);
  });
});

function permissionLabel(permission: string) {
  if (permission === "editor" || permission === "admin") {
    return t("organization.share.permissionEditable");
  }
  return t("organization.share.permissionReadonly");
}

function formatShareDate(dateStr?: string) {
  if (!dateStr) return "—";
  const date = new Date(dateStr);
  if (Number.isNaN(date.getTime())) return dateStr;
  return date.toLocaleDateString(undefined, { year: "numeric", month: "2-digit", day: "2-digit" });
}

async function loadOrganizations() {
  loadingOrgs.value = true;
  try {
    await orgStore.fetchOrganizations();
  } finally {
    loadingOrgs.value = false;
  }
}

async function loadShares() {
  if (!props.kbId) return;
  loadingShares.value = true;
  try {
    const result = await listKBShares(props.kbId);
    if (result.success && result.data) {
      const sharesData = (result.data as { shares?: KnowledgeBaseShare[] }).shares || result.data;
      const sharesList = Array.isArray(sharesData) ? sharesData : [];
      shares.value = sharesList.map((share: KnowledgeBaseShare) => ({
        ...share,
        organization_name:
          share.organization_name ||
          orgStore.organizations.find((o) => o.id === share.organization_id)?.name ||
          share.organization_id,
      }));
    }
  } catch (e) {
    console.error("Failed to load shares:", e);
  } finally {
    loadingShares.value = false;
  }
}

async function handleShare() {
  if (!selectedOrgId.value) return;

  submitting.value = true;
  try {
    const result = await orgStore.shareKnowledgeBase(props.kbId, {
      organization_id: selectedOrgId.value,
      permission: selectedPermission.value,
    });
    if (result.success) {
      MessagePlugin.success(t("organization.share.shareSuccess"));
      selectedOrgId.value = "";
      selectedPermission.value = "viewer";
      addPopupVisible.value = false;
      await loadShares();
    } else {
      MessagePlugin.error(result.message || t("organization.share.shareFailed"));
    }
  } catch (e: unknown) {
    const message = e instanceof Error ? e.message : t("organization.share.shareFailed");
    MessagePlugin.error(message);
  } finally {
    submitting.value = false;
  }
}

async function handleUpdatePermission(share: KnowledgeBaseShare, newPermission: string) {
  if (share.permission === newPermission) return;

  try {
    const result = await orgStore.changeKnowledgeBaseSharePermission(props.kbId, share.id, {
      permission: newPermission as "viewer" | "editor",
    });
    if (result.success) {
      MessagePlugin.success(t("organization.roleUpdated"));
      await loadShares();
    } else {
      MessagePlugin.error(result.message || t("organization.roleUpdateFailed"));
    }
  } catch (e: unknown) {
    const message = e instanceof Error ? e.message : t("organization.roleUpdateFailed");
    MessagePlugin.error(message);
  }
}

async function handleUnshare(share: KnowledgeBaseShare) {
  try {
    const result = await orgStore.unshareKnowledgeBase(props.kbId, share.id, share.organization_id);
    if (result.success) {
      MessagePlugin.success(t("organization.share.unshareSuccess"));
      await loadShares();
    } else {
      MessagePlugin.error(result.message || t("organization.share.unshareFailed"));
    }
  } catch (e: unknown) {
    const message = e instanceof Error ? e.message : t("organization.share.unshareFailed");
    MessagePlugin.error(message);
  }
}

watch(
  () => props.kbId,
  async (newKbId) => {
    if (newKbId) {
      await Promise.all([loadOrganizations(), loadShares()]);
    }
  },
  { immediate: true },
);
</script>
