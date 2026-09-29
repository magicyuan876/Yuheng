<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="visible"
        class="fixed inset-0 z-[1000] flex items-center justify-center bg-black/50 backdrop-blur-sm"
        @click.self="handleClose"
      >
        <div
          class="modal-panel bg-card relative flex h-[80vh] w-[90vw] flex-col overflow-hidden rounded-xl shadow-[0_8px_32px_rgba(0,0,0,0.12)]"
          :class="mode === 'join' ? 'max-h-[580px] max-w-[700px]' : 'max-h-[650px] max-w-[900px]'"
        >
          <!-- 关闭按钮 -->
          <button
            data-slot="modal-close"
            type="button"
            class="bg-secondary text-muted-foreground hover:text-foreground absolute top-5 right-5 z-10 flex size-8 cursor-pointer items-center justify-center rounded-md transition-all duration-200"
            @click="handleClose"
            :aria-label="$t('common.close')"
          >
            <XIcon class="size-5" />
          </button>

          <div class="flex h-full overflow-hidden">
            <!-- 左侧导航 -->
            <div class="bg-secondary border-border flex w-[200px] shrink-0 flex-col border-r">
              <div class="border-border border-b px-5 py-6">
                <h2 class="text-foreground m-0 font-[family-name:var(--app-font-family)] text-lg font-semibold">
                  {{ modalTitle }}
                </h2>
              </div>
              <div class="flex-1 overflow-y-auto px-2 py-3">
                <div
                  v-for="(item, index) in navItems"
                  :key="index"
                  class="mb-1 flex cursor-pointer items-center rounded-md px-3 py-2.5 text-sm transition-all duration-200"
                  :class="
                    currentSection === item.key
                      ? 'bg-secondary text-primary font-medium'
                      : 'text-muted-foreground hover:bg-accent hover:text-foreground'
                  "
                  @click="currentSection = item.key"
                >
                  <component :is="item.icon" class="mr-2 size-[18px] shrink-0" />
                  <span class="flex-1">{{ item.label }}</span>
                </div>
              </div>
            </div>

            <!-- 右侧内容区域 -->
            <div class="flex min-w-0 flex-1 flex-col overflow-hidden">
              <div class="flex-1 overflow-y-auto px-8 py-6">
                <!-- 创建组织 - 基本信息 -->
                <div v-if="mode === 'create'" v-show="currentSection === 'basic'" class="mb-8">
                  <div class="mb-6">
                    <h3
                      class="text-foreground m-0 mb-2 font-[family-name:var(--app-font-family)] text-base font-semibold"
                    >
                      {{ $t("organization.editor.basicTitle") }}
                    </h3>
                    <p class="text-placeholder m-0 font-[family-name:var(--app-font-family)] text-sm leading-[22px]">
                      {{ $t("organization.editor.basicDesc") }}
                    </p>
                  </div>
                  <div>
                    <div class="mb-6 last:mb-0">
                      <label
                        class="text-foreground after:text-destructive mb-2 block font-[family-name:var(--app-font-family)] text-sm font-medium after:ml-1 after:content-['*']"
                      >
                        {{ $t("organization.name") }}
                      </label>
                      <div class="flex items-center gap-3">
                        <SpaceAvatar :name="createForm.name || '?'" size="medium" />
                        <Input
                          v-model="createForm.name"
                          :placeholder="$t('organization.namePlaceholder')"
                          :maxlength="100"
                          class="min-w-0 flex-1"
                        />
                      </div>
                      <p class="text-placeholder mt-2 text-xs leading-[18px]">
                        {{ $t("organization.editor.nameTip") }}
                      </p>
                    </div>
                    <div class="mb-6 last:mb-0">
                      <label
                        class="text-foreground mb-2 block font-[family-name:var(--app-font-family)] text-sm font-medium"
                      >
                        {{ $t("organization.description") }}
                      </label>
                      <Textarea
                        v-model="createForm.description"
                        :placeholder="$t('organization.descriptionPlaceholder')"
                        :maxlength="500"
                        rows="3"
                        class="max-h-[138px] min-h-[78px]"
                      />
                      <p class="text-placeholder mt-2 text-xs leading-[18px]">
                        {{ $t("organization.editor.descriptionTip") }}
                      </p>
                    </div>
                  </div>
                </div>

                <!-- 创建组织 - 权限说明 -->
                <div v-if="mode === 'create'" v-show="currentSection === 'permissions'" class="mb-8">
                  <div class="mb-6">
                    <h3
                      class="text-foreground m-0 mb-2 font-[family-name:var(--app-font-family)] text-base font-semibold"
                    >
                      {{ $t("organization.editor.permissionsTitle") }}
                    </h3>
                    <p class="text-placeholder m-0 font-[family-name:var(--app-font-family)] text-sm leading-[22px]">
                      {{ $t("organization.editor.permissionsDesc") }}
                    </p>
                  </div>
                  <div>
                    <div class="flex flex-col gap-4">
                      <div class="bg-secondary border-border rounded-lg border p-4">
                        <div class="mb-3 flex items-center gap-3">
                          <div
                            class="text-primary-foreground flex size-10 items-center justify-center rounded-lg bg-[linear-gradient(135deg,var(--td-brand-color),var(--td-brand-color-active))]"
                          >
                            <ShieldCheckIcon class="size-[1em]" />
                          </div>
                          <div class="flex items-center gap-2">
                            <span class="text-foreground text-[15px] font-semibold">{{
                              $t("organization.role.admin")
                            }}</span>
                            <Badge>{{ $t("organization.editor.fullAccess") }}</Badge>
                          </div>
                        </div>
                        <ul class="m-0 list-none p-0">
                          <li class="text-muted-foreground flex items-center gap-2 py-1.5 text-[13px]">
                            <CheckIcon class="text-primary size-3.5" />{{ $t("organization.editor.adminPerm1") }}
                          </li>
                          <li class="text-muted-foreground flex items-center gap-2 py-1.5 text-[13px]">
                            <CheckIcon class="text-primary size-3.5" />{{ $t("organization.editor.adminPerm2") }}
                          </li>
                          <li class="text-muted-foreground flex items-center gap-2 py-1.5 text-[13px]">
                            <CheckIcon class="text-primary size-3.5" />{{ $t("organization.editor.adminPerm3") }}
                          </li>
                          <li class="text-muted-foreground flex items-center gap-2 py-1.5 text-[13px]">
                            <CheckIcon class="text-primary size-3.5" />{{ $t("organization.editor.adminPerm4") }}
                          </li>
                        </ul>
                      </div>
                      <div class="bg-secondary border-border rounded-lg border p-4">
                        <div class="mb-3 flex items-center gap-3">
                          <div
                            class="text-primary-foreground flex size-10 items-center justify-center rounded-lg bg-[linear-gradient(135deg,var(--td-warning-color),var(--td-warning-color-active))]"
                          >
                            <PencilIcon class="size-[1em]" />
                          </div>
                          <div class="flex items-center gap-2">
                            <span class="text-foreground text-[15px] font-semibold">{{
                              $t("organization.role.editor")
                            }}</span>
                            <Badge class="bg-warning text-primary-foreground">
                              {{ $t("organization.editor.editAccess") }}
                            </Badge>
                          </div>
                        </div>
                        <ul class="m-0 list-none p-0">
                          <li class="text-muted-foreground flex items-center gap-2 py-1.5 text-[13px]">
                            <CheckIcon class="text-primary size-3.5" />{{ $t("organization.editor.editorPerm1") }}
                          </li>
                          <li class="text-muted-foreground flex items-center gap-2 py-1.5 text-[13px]">
                            <CheckIcon class="text-primary size-3.5" />{{ $t("organization.editor.editorPerm2") }}
                          </li>
                          <li class="text-muted-foreground flex items-center gap-2 py-1.5 text-[13px]">
                            <XIcon class="text-destructive size-3.5" />{{ $t("organization.editor.shareKBPerm") }}
                          </li>
                          <li class="text-muted-foreground flex items-center gap-2 py-1.5 text-[13px]">
                            <XIcon class="text-destructive size-3.5" />{{ $t("organization.editor.editorPerm3") }}
                          </li>
                        </ul>
                      </div>
                      <div class="bg-secondary border-border rounded-lg border p-4">
                        <div class="mb-3 flex items-center gap-3">
                          <div
                            class="text-primary-foreground flex size-10 items-center justify-center rounded-lg bg-[var(--td-bg-color-component-disabled)]"
                          >
                            <EyeIcon class="size-[1em]" />
                          </div>
                          <div class="flex items-center gap-2">
                            <span class="text-foreground text-[15px] font-semibold">{{
                              $t("organization.role.viewer")
                            }}</span>
                            <Badge variant="secondary">{{ $t("organization.editor.viewAccess") }}</Badge>
                          </div>
                        </div>
                        <ul class="m-0 list-none p-0">
                          <li class="text-muted-foreground flex items-center gap-2 py-1.5 text-[13px]">
                            <CheckIcon class="text-primary size-3.5" />{{ $t("organization.editor.viewerPerm1") }}
                          </li>
                          <li class="text-muted-foreground flex items-center gap-2 py-1.5 text-[13px]">
                            <XIcon class="text-destructive size-3.5" />{{ $t("organization.editor.shareKBPerm") }}
                          </li>
                          <li class="text-muted-foreground flex items-center gap-2 py-1.5 text-[13px]">
                            <XIcon class="text-destructive size-3.5" />{{ $t("organization.editor.viewerPerm2") }}
                          </li>
                          <li class="text-muted-foreground flex items-center gap-2 py-1.5 text-[13px]">
                            <XIcon class="text-destructive size-3.5" />{{ $t("organization.editor.viewerPerm3") }}
                          </li>
                        </ul>
                      </div>
                    </div>
                    <div
                      class="text-primary mt-5 flex items-start gap-2 rounded-lg bg-[var(--td-brand-color-light)] px-4 py-3 text-[13px] leading-5"
                    >
                      <InfoIcon class="mt-0.5 size-[1em] shrink-0" />
                      <span>{{ $t("organization.editor.ownerNote") }}</span>
                    </div>
                  </div>
                </div>

                <!-- 加入组织 -->
                <div v-if="mode === 'join'" v-show="currentSection === 'join'" class="mb-8">
                  <div class="mb-6">
                    <h3
                      class="text-foreground m-0 mb-2 font-[family-name:var(--app-font-family)] text-base font-semibold"
                    >
                      {{ $t("organization.editor.joinTitle") }}
                    </h3>
                    <p class="text-placeholder m-0 font-[family-name:var(--app-font-family)] text-sm leading-[22px]">
                      {{ $t("organization.editor.joinDesc") }}
                    </p>
                  </div>
                  <div>
                    <div class="px-0 pt-6 pb-8 text-center">
                      <div
                        class="text-primary mx-auto mb-4 flex size-20 items-center justify-center rounded-full bg-[linear-gradient(135deg,var(--td-brand-color-light),#07c05f0d)]"
                      >
                        <UserPlusIcon class="size-12" />
                      </div>
                      <p class="text-placeholder m-0 text-sm">{{ $t("organization.editor.joinIllustration") }}</p>
                    </div>
                    <div class="mb-6 last:mb-0">
                      <label
                        class="text-foreground after:text-destructive mb-2 block font-[family-name:var(--app-font-family)] text-sm font-medium after:ml-1 after:content-['*']"
                      >
                        {{ $t("organization.inviteCode") }}
                      </label>
                      <Input
                        v-model="joinForm.invite_code"
                        :placeholder="$t('organization.inviteCodePlaceholder')"
                        :maxlength="32"
                        class="text-center text-base tracking-[1px] md:text-base"
                      />
                      <p class="text-placeholder mt-2 text-xs leading-[18px]">
                        {{ $t("organization.editor.inviteCodeTip") }}
                      </p>
                    </div>
                    <div class="bg-secondary mt-8 rounded-lg p-5">
                      <div class="text-foreground mb-4 text-sm font-medium">
                        {{ $t("organization.editor.howToGetCode") }}
                      </div>
                      <div class="flex flex-col gap-3">
                        <div class="flex items-center gap-3">
                          <span
                            class="bg-primary text-primary-foreground flex size-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold"
                            >1</span
                          >
                          <span class="text-muted-foreground text-[13px]">{{ $t("organization.editor.step1") }}</span>
                        </div>
                        <div class="flex items-center gap-3">
                          <span
                            class="bg-primary text-primary-foreground flex size-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold"
                            >2</span
                          >
                          <span class="text-muted-foreground text-[13px]">{{ $t("organization.editor.step2") }}</span>
                        </div>
                        <div class="flex items-center gap-3">
                          <span
                            class="bg-primary text-primary-foreground flex size-6 shrink-0 items-center justify-center rounded-full text-xs font-semibold"
                            >3</span
                          >
                          <span class="text-muted-foreground text-[13px]">{{ $t("organization.editor.step3") }}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- 底部按钮 -->
              <div class="border-border flex shrink-0 justify-end gap-3 border-t px-8 py-4">
                <Button variant="outline" @click="handleClose">
                  {{ $t("common.cancel") }}
                </Button>
                <Button :disabled="submitting" @click="handleSubmit">
                  <Loader2Icon v-if="submitting" class="animate-spin" />
                  {{ mode === "create" ? $t("common.create") : $t("organization.join.preview") }}
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>

    <!-- 加入确认弹窗 -->
    <!-- The confirmation opens on top of the editor modal above, which sits at
         z-index 1000; the dialog's own z-50 would put it underneath, so the
         content is lifted over it. (Its overlay stays at z-50 and so dims only
         the page, not the editor; ui/dialog offers no way to raise it.) -->
    <Dialog v-model:open="showJoinConfirm">
      <DialogContent
        class="z-[1001] sm:max-w-[480px]"
        @keydown.enter="!previewInfo?.is_already_member && !joining && confirmJoin()"
      >
        <DialogHeader>
          <DialogTitle>{{ $t("organization.join.confirmTitle") }}</DialogTitle>
        </DialogHeader>
        <div v-if="previewInfo" class="py-2">
          <div class="bg-secondary border-border rounded-lg border p-4">
            <div class="mb-4 flex gap-3">
              <div
                class="text-primary-foreground flex size-12 shrink-0 items-center justify-center rounded-lg bg-[linear-gradient(135deg,var(--td-brand-color),var(--td-brand-color-active))]"
              >
                <UsersIcon class="size-6" />
              </div>
              <div class="min-w-0 flex-1">
                <h4 class="text-foreground m-0 mb-1 text-base font-semibold">{{ previewInfo.name }}</h4>
                <p class="text-placeholder m-0 line-clamp-2 overflow-hidden text-[13px] leading-5 text-ellipsis">
                  {{ previewInfo.description || $t("organization.noDescription") }}
                </p>
              </div>
            </div>
            <div class="border-border flex gap-6 border-t pt-3">
              <div class="text-muted-foreground flex items-center gap-1.5 text-[13px]">
                <UserIcon class="text-placeholder size-4" />
                <span>{{ $t("organization.join.memberCount", { count: previewInfo.member_count }) }}</span>
              </div>
              <div class="text-muted-foreground flex items-center gap-1.5 text-[13px]">
                <FolderIcon class="text-placeholder size-4" />
                <span>{{ $t("organization.join.shareCount", { count: previewInfo.share_count }) }}</span>
              </div>
            </div>
          </div>
          <div
            v-if="previewInfo.is_already_member"
            class="text-primary mt-4 flex items-center gap-2 rounded-lg bg-[var(--td-brand-color-light)] px-4 py-3 text-sm"
          >
            <CircleCheckIcon class="size-[18px]" />
            <span>{{ $t("organization.join.alreadyMember") }}</span>
          </div>
        </div>
        <DialogFooter>
          <template v-if="previewInfo?.is_already_member">
            <DialogClose as-child>
              <Button>{{ $t("common.close") }}</Button>
            </DialogClose>
          </template>
          <template v-else>
            <DialogClose as-child>
              <Button variant="outline">{{ $t("common.cancel") }}</Button>
            </DialogClose>
            <Button :disabled="joining" @click="confirmJoin">
              <Loader2Icon v-if="joining" class="animate-spin" />
              {{ $t("organization.join.confirm") }}
            </Button>
          </template>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useOrganizationStore } from "@/stores/organization";
import { useI18n } from "vue-i18n";
import type { OrganizationPreview } from "@/api/organization";
import SpaceAvatar from "@/components/SpaceAvatar.vue";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  CheckIcon,
  CircleCheckIcon,
  EyeIcon,
  FolderIcon,
  InfoIcon,
  Loader2Icon,
  PencilIcon,
  ShieldCheckIcon,
  UserIcon,
  UserPlusIcon,
  UsersIcon,
  XIcon,
  type LucideIcon,
} from "@lucide/vue";

const { t } = useI18n();
const orgStore = useOrganizationStore();

// Props
const props = defineProps<{
  visible: boolean;
  mode: "create" | "join";
}>();

// Emits
const emit = defineEmits<{
  (e: "update:visible", value: boolean): void;
  (e: "success"): void;
}>();

const currentSection = ref<string>("basic");
const submitting = ref(false);
const showJoinConfirm = ref(false);
const previewInfo = ref<OrganizationPreview | null>(null);
const joining = ref(false);

const createForm = ref({
  name: "",
  description: "",
});

const joinForm = ref({
  invite_code: "",
});

// 计算属性
const modalTitle = computed(() => {
  return props.mode === "create" ? t("organization.createOrg") : t("organization.joinOrg");
});

const navItems = computed<{ key: string; icon: LucideIcon; label: string }[]>(() => {
  if (props.mode === "create") {
    return [
      { key: "basic", icon: InfoIcon, label: t("organization.editor.navBasic") },
      { key: "permissions", icon: ShieldCheckIcon, label: t("organization.editor.navPermissions") },
    ];
  } else {
    return [{ key: "join", icon: UserPlusIcon, label: t("organization.editor.navJoin") }];
  }
});

// 方法
const resetForm = () => {
  createForm.value = { name: "", description: "" };
  joinForm.value = { invite_code: "" };
  currentSection.value = props.mode === "create" ? "basic" : "join";
  showJoinConfirm.value = false;
  previewInfo.value = null;
};

const handleClose = () => {
  emit("update:visible", false);
  setTimeout(resetForm, 300);
};

const handleSubmit = async () => {
  if (props.mode === "create") {
    await handleCreate();
  } else {
    await handleJoin();
  }
};

const handleCreate = async () => {
  if (!createForm.value.name.trim()) {
    MessagePlugin.warning(t("organization.nameRequired"));
    currentSection.value = "basic";
    return;
  }

  submitting.value = true;
  try {
    const result = await orgStore.create(createForm.value.name.trim(), createForm.value.description.trim());
    if (result) {
      MessagePlugin.success(t("organization.createSuccess"));
      emit("success");
      handleClose();
    } else {
      MessagePlugin.error(orgStore.error || t("organization.createFailed"));
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("organization.createFailed"));
  } finally {
    submitting.value = false;
  }
};

const handleJoin = async () => {
  if (!joinForm.value.invite_code.trim()) {
    MessagePlugin.warning(t("organization.inviteCodeRequired"));
    return;
  }

  submitting.value = true;
  try {
    // First preview the organization
    const preview = await orgStore.preview(joinForm.value.invite_code.trim());
    if (preview) {
      previewInfo.value = preview;
      showJoinConfirm.value = true;
    } else {
      MessagePlugin.error(orgStore.error || t("organization.join.invalidCode"));
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("organization.join.invalidCode"));
  } finally {
    submitting.value = false;
  }
};

const confirmJoin = async () => {
  if (!joinForm.value.invite_code.trim()) {
    return;
  }

  joining.value = true;
  try {
    const result = await orgStore.join(joinForm.value.invite_code.trim());
    if (result) {
      MessagePlugin.success(t("organization.joinSuccess"));
      showJoinConfirm.value = false;
      emit("success");
      handleClose();
    } else {
      MessagePlugin.error(orgStore.error || t("organization.joinFailed"));
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("organization.joinFailed"));
  } finally {
    joining.value = false;
  }
};

// 监听
watch(
  () => props.visible,
  (newVal) => {
    if (newVal) {
      resetForm();
    }
  },
);

watch(
  () => props.mode,
  () => {
    currentSection.value = props.mode === "create" ? "basic" : "join";
  },
);
</script>

<style scoped>
/* The modal transition predates the utility stack and cannot be expressed
   with utility classes; it is the one rule kept as CSS. */
.modal-enter-active,
.modal-leave-active {
  transition: all 0.3s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.modal-enter-from .modal-panel,
.modal-leave-to .modal-panel {
  transform: scale(0.95);
}
</style>
