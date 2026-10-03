<template>
  <!--
    Reset another user's password. A 64px header and a footer, both ruled
    off, 24px side padding, 12px radius, and a tighter layout under 480px.
    The dialog cannot be dismissed by the overlay or the close button while
    the request is in flight.
  -->
  <Dialog :open="open" @update:open="onOpenChange">
    <DialogContent
      :show-close-button="false"
      class="border-border gap-0 overflow-hidden rounded-xl border p-0 shadow-[0_12px_32px_rgba(15,23,42,0.12),0_2px_8px_rgba(15,23,42,0.08)] ring-0 max-[480px]:max-w-[calc(100vw-24px)] sm:max-w-[440px]"
      @interact-outside="(e: Event) => submitting && e.preventDefault()"
    >
      <DialogHeader
        class="border-border min-h-16 flex-row items-center justify-between gap-3 border-b px-6 max-[480px]:min-h-14 max-[480px]:px-5"
      >
        <DialogTitle class="text-lg leading-[26px] font-semibold max-[480px]:text-[17px]">
          {{ t("system.usersWorkspaces.passwordReset.dialogTitle") }}
        </DialogTitle>
        <DialogClose v-if="!submitting" as-child>
          <Button variant="ghost" size="icon-sm" class="rounded-md" :aria-label="t('common.close')">
            <XIcon />
          </Button>
        </DialogClose>
      </DialogHeader>
      <div class="px-6 pt-5 pb-1 max-[480px]:px-5 max-[480px]:pt-4">
        <Alert class="mb-5 rounded-lg border-0 bg-[var(--td-warning-color-focus)] px-3.5 py-3">
          <CircleAlertIcon class="text-warning" />
          <AlertTitle class="text-foreground text-[13px] leading-5 font-normal">
            {{ t("system.usersWorkspaces.passwordReset.warning") }}
          </AlertTitle>
        </Alert>
        <div class="mb-4 grid">
          <Label class="min-h-7 text-sm leading-[22px]" for="password-reset-email">
            {{ t("system.usersWorkspaces.passwordReset.emailLabel") }}
          </Label>
          <div class="relative">
            <Input
              id="password-reset-email"
              v-model="form.email"
              type="text"
              autocomplete="off"
              class="rounded-md pr-8"
              :aria-invalid="errors.email ? true : undefined"
              :disabled="submitting"
              :placeholder="t('system.usersWorkspaces.passwordReset.emailPlaceholder')"
              @input="errors.email = ''"
              @blur="validateEmail"
            />
            <button
              v-if="form.email && !submitting"
              type="button"
              data-slot="input-clear"
              class="text-muted-foreground hover:text-foreground absolute top-1/2 right-2.5 -translate-y-1/2 cursor-pointer"
              :aria-label="t('common.clear')"
              @mousedown.prevent
              @click="form.email = ''"
            >
              <XIcon class="size-3.5" />
            </button>
          </div>
          <p v-if="errors.email" class="text-destructive m-0 mt-1 text-xs">{{ errors.email }}</p>
        </div>
        <div class="mb-4 grid">
          <Label class="min-h-7 text-sm leading-[22px]" for="password-reset-new">
            {{ t("system.usersWorkspaces.passwordReset.newPasswordLabel") }}
          </Label>
          <div class="relative">
            <LockIcon class="text-placeholder absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
            <Input
              id="password-reset-new"
              v-model="form.newPassword"
              type="password"
              autocomplete="new-password"
              :aria-invalid="errors.newPassword ? true : undefined"
              :disabled="submitting"
              :placeholder="t('system.usersWorkspaces.passwordReset.newPasswordPlaceholder')"
              class="rounded-md pl-8"
              @input="errors.newPassword = ''"
              @blur="validateNewPassword"
            />
          </div>
          <p v-if="errors.newPassword" class="text-destructive m-0 mt-1 text-xs">{{ errors.newPassword }}</p>
        </div>
        <div class="mb-4 grid">
          <Label class="min-h-7 text-sm leading-[22px]" for="password-reset-confirm">
            {{ t("system.usersWorkspaces.passwordReset.confirmPasswordLabel") }}
          </Label>
          <div class="relative">
            <LockIcon class="text-placeholder absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
            <Input
              id="password-reset-confirm"
              v-model="form.confirmPassword"
              type="password"
              autocomplete="new-password"
              :aria-invalid="errors.confirmPassword ? true : undefined"
              :disabled="submitting"
              :placeholder="t('system.usersWorkspaces.passwordReset.confirmPasswordPlaceholder')"
              class="rounded-md pl-8"
              @keydown.enter="submit"
              @input="errors.confirmPassword = ''"
              @blur="validateConfirmPassword"
            />
          </div>
          <p v-if="errors.confirmPassword" class="text-destructive m-0 mt-1 text-xs">{{ errors.confirmPassword }}</p>
        </div>
      </div>
      <DialogFooter
        class="border-border m-0 rounded-none bg-transparent px-6 pt-4 pb-5 max-[480px]:px-5 max-[480px]:pt-3.5 max-[480px]:pb-[18px]"
      >
        <Button variant="outline" class="min-w-[88px] rounded-md" :disabled="submitting" @click="onOpenChange(false)">
          {{ t("system.globalSettings.confirm.cancelBtn") }}
        </Button>
        <Button
          class="bg-destructive text-primary-foreground hover:bg-destructive/80 min-w-[88px] rounded-md"
          :disabled="submitting"
          @click="submit"
        >
          <Loader2Icon v-if="submitting" class="animate-spin" />
          {{ t("system.usersWorkspaces.passwordReset.confirmBtn") }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import { CircleAlertIcon, Loader2Icon, LockIcon, XIcon } from "@lucide/vue";
import { resetUserPassword } from "@/api/system";
import { Alert, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ (e: "update:open", value: boolean): void }>();

const { t } = useI18n();

const submitting = ref(false);
const form = reactive({ email: "", newPassword: "", confirmPassword: "" });
const errors = reactive({ email: "", newPassword: "", confirmPassword: "" });

function resetForm() {
  form.email = "";
  form.newPassword = "";
  form.confirmPassword = "";
  errors.email = "";
  errors.newPassword = "";
  errors.confirmPassword = "";
}

// The form starts clean every time the dialog opens.
watch(
  () => props.open,
  (open) => {
    if (open) resetForm();
  },
);

// One validator per field. Each field's @blur runs its own validator and
// submit runs all three; the first failing rule's message wins.
function validateEmail(): boolean {
  const email = form.email.trim();
  if (!email) {
    errors.email = t("system.usersWorkspaces.passwordReset.validation.emailRequired");
  } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    errors.email = t("system.usersWorkspaces.passwordReset.validation.emailInvalid");
  } else {
    errors.email = "";
  }
  return !errors.email;
}

function validateNewPassword(): boolean {
  const pwd = form.newPassword;
  if (!pwd) {
    errors.newPassword = t("system.usersWorkspaces.passwordReset.validation.passwordRequired");
  } else if (pwd.length < 8 || pwd.length > 32) {
    errors.newPassword = t("system.usersWorkspaces.passwordReset.validation.passwordLength");
  } else if (!/[a-zA-Z]/.test(pwd)) {
    errors.newPassword = t("system.usersWorkspaces.passwordReset.validation.passwordLetter");
  } else if (!/\d/.test(pwd)) {
    errors.newPassword = t("system.usersWorkspaces.passwordReset.validation.passwordNumber");
  } else {
    errors.newPassword = "";
  }
  return !errors.newPassword;
}

function validateConfirmPassword(): boolean {
  if (!form.confirmPassword) {
    errors.confirmPassword = t("system.usersWorkspaces.passwordReset.validation.confirmRequired");
  } else if (form.confirmPassword !== form.newPassword) {
    errors.confirmPassword = t("system.usersWorkspaces.passwordReset.validation.passwordMismatch");
  } else {
    errors.confirmPassword = "";
  }
  return !errors.confirmPassword;
}

function onOpenChange(open: boolean) {
  if (!open && submitting.value) return;
  emit("update:open", open);
}

async function submit() {
  if (submitting.value) return;
  // Run every validator (no short-circuit) so all failing fields show
  // their message at once.
  const results = [validateEmail(), validateNewPassword(), validateConfirmPassword()];
  if (!results.every(Boolean)) return;

  submitting.value = true;
  try {
    await resetUserPassword({ email: form.email.trim(), new_password: form.newPassword });
    MessagePlugin.success(t("system.usersWorkspaces.passwordReset.success"));
    emit("update:open", false);
  } catch (err: any) {
    MessagePlugin.error(err?.message || t("system.usersWorkspaces.passwordReset.failed"));
  } finally {
    submitting.value = false;
  }
}
</script>
