<template>
  <div class="w-full">
    <div class="mb-8">
      <h2 class="text-foreground mt-0 mb-2 text-xl font-semibold">{{ $t("userProfile.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-normal">{{ $t("userProfile.description") }}</p>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="text-muted-foreground flex items-center justify-center gap-3 py-10 text-sm">
      <Loader2Icon class="animate-spin" />
      <span>{{ $t("tenant.loadingInfo") }}</span>
    </div>

    <!-- Error -->
    <div v-else-if="error" class="py-5">
      <!-- TDesign's error alert sat on the pale error tint with no border and
           dark text; only its icon was red. -->
      <Alert variant="destructive" class="text-foreground border-transparent bg-[var(--td-error-color-1)]">
        <CircleAlertIcon class="text-destructive!" />
        <AlertTitle>{{ error }}</AlertTitle>
        <AlertAction>
          <Button variant="outline" size="sm" @click="loadInfo">{{ $t("tenant.retry") }}</Button>
        </AlertAction>
      </Alert>
    </div>

    <!-- Content -->
    <div v-else class="flex flex-col">
      <!-- 用户 ID -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ $t("tenant.api.userIdLabel") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ $t("tenant.api.userIdDescription") }}</p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <span class="text-foreground text-right text-sm break-words">{{ userInfo?.id || "-" }}</span>
        </div>
      </div>

      <!-- 用户名 -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ $t("tenant.api.usernameLabel") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ $t("tenant.api.usernameDescription") }}
          </p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <span class="text-foreground text-right text-sm break-words">{{ userInfo?.username || "-" }}</span>
        </div>
      </div>

      <!-- 邮箱 -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ $t("tenant.api.emailLabel") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ $t("tenant.api.emailDescription") }}</p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <span class="text-foreground text-right text-sm break-words">{{ userInfo?.email || "-" }}</span>
        </div>
      </div>

      <!-- 注册时间 -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            $t("tenant.api.createdAtLabel")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ $t("tenant.api.createdAtDescription") }}
          </p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <span class="text-foreground text-right text-sm break-words">{{ formatDate(userInfo?.created_at) }}</span>
        </div>
      </div>

      <!-- 修改密码：与其它 setting-row 同款只读行 + 编辑入口，表单进原地 popup -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">
            {{ $t("userProfile.changePassword.label") }}
          </label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{
              oidcOnlyLogin
                ? $t("userProfile.changePassword.oidcOnlyDescription")
                : $t("userProfile.changePassword.description")
            }}
          </p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end gap-2">
          <template v-if="oidcOnlyLogin">
            <span class="text-placeholder text-right text-sm break-words">—</span>
          </template>
          <template v-else>
            <span class="text-muted-foreground text-right text-sm tracking-[0.12em] break-words" aria-hidden="true">
              ••••••••
            </span>
            <Popover v-model:open="passwordPopupVisible">
              <PopoverTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  class="shrink-0"
                  :title="$t('userProfile.changePassword.label')"
                  :aria-label="$t('userProfile.changePassword.label')"
                >
                  <PencilIcon />
                </Button>
              </PopoverTrigger>
              <!-- The popover layer (z 5500) already clears the settings overlay, which
                   the old popup needed a 3050 z-index for. The frosted surface, hairline
                   border and layered shadow (lighter in dark mode) are the old
                   overlay's, carried over from its global stylesheet. -->
              <PopoverContent
                align="end"
                class="border-border w-[min(392px,calc(100vw-24px))] min-w-[300px] rounded-xl border-[0.5px] px-4 py-3.5 shadow-[0_0_0_0.5px_rgba(0,0,0,0.03),0_2px_4px_rgba(0,0,0,0.04),0_8px_24px_rgba(0,0,0,0.1)] ring-0 backdrop-blur-[20px] backdrop-saturate-[1.8] dark:border-white/8 dark:bg-[rgba(36,36,36,0.92)] dark:shadow-[0_0_0_0.5px_rgba(255,255,255,0.05),0_2px_4px_rgba(0,0,0,0.12),0_8px_32px_rgba(0,0,0,0.28)]"
              >
                <div @click.stop>
                  <div class="text-foreground mb-2 text-[15px] leading-snug font-semibold">
                    {{ $t("userProfile.changePassword.label") }}
                  </div>
                  <p class="text-muted-foreground m-0 mb-3 text-[13px] leading-normal">
                    {{ $t("userProfile.changePassword.description") }}
                  </p>
                  <form class="flex flex-col" @submit.prevent>
                    <div class="mb-3.5 flex flex-col gap-1.5">
                      <Label for="user-profile-old-password" class="text-sm font-medium">
                        {{ $t("userProfile.changePassword.currentLabel") }}
                      </Label>
                      <Input
                        id="user-profile-old-password"
                        v-model="passwordForm.oldPassword"
                        type="password"
                        autocomplete="current-password"
                        :disabled="passwordSubmitting"
                        :placeholder="$t('userProfile.changePassword.currentPlaceholder')"
                      />
                      <p v-if="passwordErrors.oldPassword" class="text-destructive m-0 text-xs">
                        {{ passwordErrors.oldPassword }}
                      </p>
                    </div>
                    <div class="mb-3.5 flex flex-col gap-1.5">
                      <Label for="user-profile-new-password" class="text-sm font-medium">
                        {{ $t("userProfile.changePassword.newLabel") }}
                      </Label>
                      <Input
                        id="user-profile-new-password"
                        v-model="passwordForm.newPassword"
                        type="password"
                        autocomplete="new-password"
                        :disabled="passwordSubmitting"
                        :placeholder="$t('userProfile.changePassword.newPlaceholder')"
                      />
                      <p v-if="passwordErrors.newPassword" class="text-destructive m-0 text-xs">
                        {{ passwordErrors.newPassword }}
                      </p>
                    </div>
                    <div class="mb-1 flex flex-col gap-1.5">
                      <Label for="user-profile-confirm-password" class="text-sm font-medium">
                        {{ $t("userProfile.changePassword.confirmLabel") }}
                      </Label>
                      <Input
                        id="user-profile-confirm-password"
                        v-model="passwordForm.confirmPassword"
                        type="password"
                        autocomplete="new-password"
                        :disabled="passwordSubmitting"
                        :placeholder="$t('userProfile.changePassword.confirmPlaceholder')"
                        @keydown.enter="submitPasswordChange"
                      />
                      <p v-if="passwordErrors.confirmPassword" class="text-destructive m-0 text-xs">
                        {{ passwordErrors.confirmPassword }}
                      </p>
                    </div>
                  </form>
                  <div class="mt-4 flex justify-end gap-2">
                    <Button variant="outline" :disabled="passwordSubmitting" @click="closePasswordPopup">
                      {{ $t("common.cancel") }}
                    </Button>
                    <Button :disabled="passwordSubmitting" @click="submitPasswordChange">
                      <Loader2Icon v-if="passwordSubmitting" class="animate-spin" />
                      {{ $t("userProfile.changePassword.submit") }}
                    </Button>
                  </div>
                </div>
              </PopoverContent>
            </Popover>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from "vue";
import { useRouter } from "vue-router";
import { MessagePlugin } from "tdesign-vue-next";
import { getCurrentUser, changePassword, logout as logoutApi, type UserInfo } from "@/api/auth";
import { useAuthStore } from "@/stores/auth";
import { useI18n } from "vue-i18n";

import { Alert, AlertAction, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { CircleAlertIcon, Loader2Icon, PencilIcon } from "@lucide/vue";

const { t, locale } = useI18n();
const router = useRouter();
const authStore = useAuthStore();

const userInfo = ref<UserInfo | null>(null);
const loading = ref(true);
const error = ref("");

const passwordPopupVisible = ref(false);
const passwordSubmitting = ref(false);
const passwordForm = reactive({
  oldPassword: "",
  newPassword: "",
  confirmPassword: "",
});

// The old t-form reported rule failures per field; the same messages are
// kept, now held in a plain reactive map cleared on open and on submit.
const passwordErrors = reactive({
  oldPassword: "",
  newPassword: "",
  confirmPassword: "",
});

const oidcOnlyLogin = computed(() => userInfo.value?.preferences?.oidc_only_login === true);

watch(passwordPopupVisible, (open) => {
  if (open) {
    resetPasswordForm();
  }
});

const clearPasswordErrors = () => {
  passwordErrors.oldPassword = "";
  passwordErrors.newPassword = "";
  passwordErrors.confirmPassword = "";
};

// Same rules the t-form enforced, in the same order; the first failure per
// field wins, matching TDesign's error display.
const validatePasswordForm = (): boolean => {
  clearPasswordErrors();
  let ok = true;

  if (!passwordForm.oldPassword) {
    passwordErrors.oldPassword = t("userProfile.changePassword.currentRequired");
    ok = false;
  }

  if (!passwordForm.newPassword) {
    passwordErrors.newPassword = t("auth.passwordRequired");
    ok = false;
  } else if (passwordForm.newPassword.length < 8) {
    passwordErrors.newPassword = t("auth.passwordMinLength");
    ok = false;
  } else if (passwordForm.newPassword.length > 32) {
    passwordErrors.newPassword = t("auth.passwordMaxLength");
    ok = false;
  } else if (!/[a-zA-Z]/.test(passwordForm.newPassword)) {
    passwordErrors.newPassword = t("auth.passwordMustContainLetter");
    ok = false;
  } else if (!/\d/.test(passwordForm.newPassword)) {
    passwordErrors.newPassword = t("auth.passwordMustContainNumber");
    ok = false;
  } else if (passwordForm.newPassword === passwordForm.oldPassword) {
    passwordErrors.newPassword = t("userProfile.changePassword.sameAsCurrent");
    ok = false;
  }

  if (!passwordForm.confirmPassword) {
    passwordErrors.confirmPassword = t("auth.confirmPasswordRequired");
    ok = false;
  } else if (passwordForm.confirmPassword !== passwordForm.newPassword) {
    passwordErrors.confirmPassword = t("auth.passwordMismatch");
    ok = false;
  }

  return ok;
};

const loadInfo = async () => {
  try {
    loading.value = true;
    error.value = "";
    const resp = await getCurrentUser();
    if ((resp as any).success && resp.data) {
      userInfo.value = resp.data.user;
    } else {
      error.value = resp.message || t("tenant.messages.fetchFailed");
    }
  } catch (err: any) {
    error.value = err?.message || t("tenant.messages.networkError");
  } finally {
    loading.value = false;
  }
};

const formatDate = (dateStr: string | undefined) => {
  if (!dateStr) return t("tenant.unknown");
  try {
    const d = new Date(dateStr);
    const fmt = new Intl.DateTimeFormat(locale.value || "zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
    return fmt.format(d);
  } catch {
    return t("tenant.formatError");
  }
};

const resetPasswordForm = () => {
  passwordForm.oldPassword = "";
  passwordForm.newPassword = "";
  passwordForm.confirmPassword = "";
  clearPasswordErrors();
};

const closePasswordPopup = () => {
  if (passwordSubmitting.value) return;
  passwordPopupVisible.value = false;
  resetPasswordForm();
};

const submitPasswordChange = async () => {
  if (passwordSubmitting.value) return;
  if (!validatePasswordForm()) return;

  passwordSubmitting.value = true;
  try {
    const resp = await changePassword({
      old_password: passwordForm.oldPassword,
      new_password: passwordForm.newPassword,
    });
    if (!resp.success) {
      MessagePlugin.error(resp.message || t("userProfile.changePassword.failed"));
      return;
    }

    passwordPopupVisible.value = false;
    MessagePlugin.success(t("userProfile.changePassword.success"));
    resetPasswordForm();

    // Backend revokes all sessions on success; mirror that locally and
    // force a fresh login with the new credential.
    try {
      await logoutApi();
    } catch {
      /* ignore — local cleanup still proceeds */
    }
    authStore.logout();
    router.push("/login");
  } catch (err: any) {
    MessagePlugin.error(err?.message || t("userProfile.changePassword.failed"));
  } finally {
    passwordSubmitting.value = false;
  }
};

onMounted(loadInfo);
</script>
