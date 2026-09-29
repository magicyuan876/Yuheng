<template>
  <Dialog v-model:open="dialogVisible">
    <DialogContent class="gap-5 sm:max-w-[520px]">
      <DialogHeader>
        <DialogTitle>{{ $t("organization.share.title") }}</DialogTitle>
      </DialogHeader>

      <!-- Share form -->
      <div v-if="!showShareList" class="py-2">
        <form novalidate @submit.prevent="handleShare">
          <!-- The rows keep TDesign's default form layout: a 100px label column,
               right-aligned, beside the control. -->
          <div class="mb-6 flex items-start gap-3">
            <Label class="w-[100px] shrink-0 justify-end gap-0.5 pt-2 text-right">
              <span class="text-destructive">*</span>{{ $t("organization.share.selectOrg") }}
            </Label>
            <div class="min-w-0 flex-1">
              <Select
                :model-value="shareForm.organization_id || undefined"
                @update:model-value="(v) => onSelectOrganization(String(v ?? ''))"
              >
                <SelectTrigger class="w-full" :aria-invalid="!!orgError || undefined">
                  <SelectValue :placeholder="$t('organization.share.selectOrgPlaceholder')" />
                </SelectTrigger>
                <SelectContent position="popper" class="max-h-[320px] p-1">
                  <div v-if="loadingOrgs" class="flex items-center justify-center py-3">
                    <Loader2Icon class="text-muted-foreground size-4 animate-spin" />
                  </div>
                  <template v-else>
                    <SelectItem
                      v-for="org in availableOrganizations"
                      :key="org.id"
                      :value="org.id"
                      :text-value="org.name"
                      class="my-px h-auto px-3 py-1.5"
                    >
                      <div class="flex w-full min-w-[260px] items-center gap-2.5">
                        <div class="flex shrink-0 items-center justify-center">
                          <SpaceAvatar :name="org.name" :avatar="org.avatar" size="small" />
                        </div>
                        <div class="min-w-0 flex-1">
                          <div class="mb-0.5 flex items-center gap-1.5">
                            <span class="text-foreground truncate text-[13px] font-medium">{{ org.name }}</span>
                            <span
                              v-if="org.is_owner"
                              class="bg-primary/10 text-primary inline-flex h-5 items-center rounded-[3px] px-1.5 text-xs"
                            >
                              {{ $t("organization.owner") }}
                            </span>
                            <span
                              v-else-if="org.my_role"
                              class="inline-flex h-5 items-center rounded-[3px] px-1.5 text-xs"
                              :class="{
                                'bg-warning/10 text-warning': org.my_role === 'admin',
                                'bg-muted text-foreground': org.my_role !== 'admin',
                              }"
                            >
                              {{ $t(`organization.role.${org.my_role}`) }}
                            </span>
                          </div>
                          <div class="text-placeholder flex items-center gap-1.5 text-xs">
                            <span class="bg-muted inline-flex items-center gap-[3px] rounded-sm px-1">
                              <UserIcon class="text-muted-foreground size-3 shrink-0" />
                              {{ org.member_count ?? 0 }}
                            </span>
                            <span class="bg-muted inline-flex items-center gap-[3px] rounded-sm px-1">
                              <img
                                src="@/assets/img/zhishiku.svg"
                                class="size-3 shrink-0 opacity-75"
                                alt=""
                                aria-hidden="true"
                              />
                              {{ org.share_count ?? 0 }}
                            </span>
                          </div>
                        </div>
                      </div>
                    </SelectItem>
                  </template>
                </SelectContent>
              </Select>
              <p v-if="orgError" class="text-destructive m-0 mt-1 text-xs">{{ orgError }}</p>
            </div>
          </div>
          <div class="mb-6 flex items-center gap-3">
            <Label class="w-[100px] shrink-0 justify-end text-right">{{ $t("organization.share.permission") }}</Label>
            <!-- A segmented control, standing in for TDesign's button-style radio group. -->
            <div role="radiogroup" class="border-border inline-flex overflow-hidden rounded-md border">
              <button
                v-for="option in permissionOptions"
                :key="option.value"
                type="button"
                role="radio"
                data-slot="segmented-item"
                :aria-checked="shareForm.permission === option.value"
                class="h-8 px-4 text-sm transition-colors not-first:border-l"
                :class="{
                  'bg-primary/10 text-primary': shareForm.permission === option.value,
                  'text-foreground hover:bg-accent': shareForm.permission !== option.value,
                }"
                @click="shareForm.permission = option.value"
              >
                {{ $t(option.label) }}
              </button>
            </div>
          </div>
          <div
            class="bg-accent text-muted-foreground mt-2 flex items-start gap-2 rounded-md p-3 text-[13px] leading-normal"
          >
            <InfoIcon class="mt-0.5 size-3.5 shrink-0" />
            <span>{{ $t("organization.share.permissionTip") }}</span>
          </div>
        </form>
        <div class="border-border mt-6 flex items-center gap-3 border-t pt-4">
          <Button v-if="shares.length > 0" variant="outline" @click="showShareList = true">
            {{ $t("organization.share.sharedTo") }} ({{ shares.length }})
          </Button>
          <div class="flex-1"></div>
          <Button variant="outline" @click="handleClose">{{ $t("common.cancel") }}</Button>
          <Button :disabled="submitting" @click="handleShare">
            <Loader2Icon v-if="submitting" class="animate-spin" />
            {{ $t("common.confirm") }}
          </Button>
        </div>
      </div>

      <!-- Share list -->
      <div v-else>
        <div class="mb-4">
          <Button variant="ghost" @click="showShareList = false">
            <ChevronLeftIcon />
            {{ $t("common.back") }}
          </Button>
        </div>
        <div v-if="loadingShares" class="text-muted-foreground flex justify-center p-8">
          <Loader2Icon class="text-primary size-5 animate-spin" />
        </div>
        <div v-else-if="shares.length === 0" class="text-muted-foreground flex justify-center p-8">
          {{ $t("organization.share.noShares") }}
        </div>
        <div v-else class="flex max-h-[280px] flex-col gap-2.5 overflow-y-auto">
          <div
            v-for="share in shares"
            :key="share.id"
            class="bg-accent border-border flex items-center justify-between rounded-lg border px-4 py-3.5 transition-colors duration-200 hover:bg-[var(--td-bg-color-container-active)]"
          >
            <div class="flex items-center gap-2.5">
              <SpaceAvatar
                :name="share.organization_name || ''"
                :avatar="orgStore.organizations.find((o) => o.id === share.organization_id)?.avatar"
                size="small"
              />
              <span class="font-medium">{{ share.organization_name }}</span>
              <span
                class="inline-flex h-5 items-center rounded-[3px] px-1.5 text-xs"
                :class="{
                  'bg-warning text-primary-foreground': share.permission === 'editor',
                  'bg-muted text-foreground': share.permission !== 'editor',
                }"
              >
                {{
                  share.permission === "editor"
                    ? $t("organization.share.permissionEditable")
                    : $t("organization.share.permissionReadonly")
                }}
              </span>
            </div>
            <div class="flex items-center gap-1.5">
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    :aria-label="$t('organization.settings.editTitle')"
                    @click="handleGoToOrgSettings(share.organization_id)"
                  >
                    <SettingsIcon />
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="top">{{ $t("organization.settings.editTitle") }}</TooltipContent>
              </Tooltip>
              <Button
                variant="ghost"
                size="icon-sm"
                class="text-destructive hover:text-destructive hover:bg-destructive/10"
                @click="handleUnshare(share)"
              >
                <XIcon />
              </Button>
            </div>
          </div>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { ChevronLeftIcon, InfoIcon, Loader2Icon, SettingsIcon, UserIcon, XIcon } from "@lucide/vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { useOrganizationStore } from "@/stores/organization";
import { shareKnowledgeBase, listKBShares, removeShare } from "@/api/organization";
import type { KnowledgeBaseShare } from "@/api/organization";
import SpaceAvatar from "@/components/SpaceAvatar.vue";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

const { t } = useI18n();
const router = useRouter();
const orgStore = useOrganizationStore();

interface Props {
  visible: boolean;
  knowledgeBaseId: string;
  knowledgeBaseName?: string;
}

const props = defineProps<Props>();
const emit = defineEmits<{
  (e: "update:visible", value: boolean): void;
  (e: "shared"): void;
}>();

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit("update:visible", val),
});

const loadingOrgs = ref(false);
const loadingShares = ref(false);
const submitting = ref(false);
const showShareList = ref(false);
const shares = ref<(KnowledgeBaseShare & { organization_name?: string })[]>([]);

const shareForm = ref({
  organization_id: "",
  permission: "viewer" as "admin" | "editor" | "viewer",
});

// The error under the organization select. It replaces the TDesign form
// rule (required, with the placeholder text as its message), which was
// checked on submit and cleared once a value was chosen.
const orgError = ref("");

function onSelectOrganization(id: string) {
  shareForm.value.organization_id = id;
  if (id) orgError.value = "";
}

// The two permission choices of the segmented control, in display order.
const permissionOptions = [
  { value: "viewer", label: "organization.share.permissionReadonly" },
  { value: "editor", label: "organization.share.permissionEditable" },
] as const;

// Only show organizations where user can share (editor or admin); exclude viewer-only orgs and already shared
const availableOrganizations = computed(() => {
  const sharedOrgIds = new Set(shares.value.map((s) => s.organization_id));
  return orgStore.organizations.filter(
    (org) =>
      !sharedOrgIds.has(org.id) && (org.is_owner === true || org.my_role === "admin" || org.my_role === "editor"),
  );
});

watch(
  () => props.visible,
  async (newVal) => {
    if (newVal) {
      showShareList.value = false;
      shareForm.value = { organization_id: "", permission: "viewer" };
      orgError.value = "";
      await Promise.all([loadOrganizations(), loadShares()]);
    }
  },
);

async function loadOrganizations() {
  loadingOrgs.value = true;
  try {
    await orgStore.fetchOrganizations();
  } finally {
    loadingOrgs.value = false;
  }
}

async function loadShares() {
  if (!props.knowledgeBaseId) return;
  loadingShares.value = true;
  try {
    const result = await listKBShares(props.knowledgeBaseId);
    if (result.success && result.data) {
      // Enrich shares with organization names
      shares.value = result.data.shares.map((share: KnowledgeBaseShare) => ({
        ...share,
        organization_name:
          orgStore.organizations.find((o) => o.id === share.organization_id)?.name || share.organization_id,
      }));
    }
  } catch (e) {
    console.error("Failed to load shares:", e);
  } finally {
    loadingShares.value = false;
  }
}

async function handleShare() {
  if (!shareForm.value.organization_id) {
    orgError.value = t("organization.share.selectOrgPlaceholder");
    return;
  }

  submitting.value = true;
  try {
    const result = await shareKnowledgeBase(props.knowledgeBaseId, {
      organization_id: shareForm.value.organization_id,
      permission: shareForm.value.permission,
    });
    if (result.success) {
      MessagePlugin.success(t("organization.share.shareSuccess"));
      await loadShares();
      shareForm.value = { organization_id: "", permission: "viewer" };
      emit("shared");
    } else {
      MessagePlugin.error(result.message || t("organization.share.shareFailed"));
    }
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("organization.share.shareFailed"));
  } finally {
    submitting.value = false;
  }
}

async function handleUnshare(share: KnowledgeBaseShare) {
  try {
    const result = await removeShare(props.knowledgeBaseId, share.id);
    if (result.success) {
      MessagePlugin.success(t("organization.share.unshareSuccess"));
      await loadShares();
      emit("shared");
    } else {
      MessagePlugin.error(result.message || t("organization.share.unshareFailed"));
    }
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("organization.share.unshareFailed"));
  }
}

function handleClose() {
  emit("update:visible", false);
}

// Navigate to organization settings
function handleGoToOrgSettings(orgId: string) {
  router.push({
    path: "/platform/organizations",
    query: { orgId },
  });
  // 关闭当前弹窗
  emit("update:visible", false);
}
</script>
