<template>
  <!--
    Create a local account as a system administrator. Two steps: the form,
    then — only when the server minted the password — a one-time view of
    that password, since it can never be fetched again. The dialog cannot
    be dismissed while the request is in flight.
  -->
  <Dialog :open="open" @update:open="onOpenChange">
    <DialogContent
      class="sm:max-w-[480px]"
      @interact-outside="(e: Event) => submitting && e.preventDefault()"
      @escape-key-down="(e: KeyboardEvent) => submitting && e.preventDefault()"
    >
      <template v-if="generated">
        <DialogHeader>
          <DialogTitle class="inline-flex items-center gap-2">
            <KeyRoundIcon class="text-primary size-5 shrink-0" aria-hidden="true" />
            <span>{{ t("system.usersWorkspaces.users.generatedTitle") }}</span>
          </DialogTitle>
          <DialogDescription class="text-muted-foreground m-0 text-[13px] leading-[1.55]">
            {{ t("system.usersWorkspaces.users.generatedHint") }}
          </DialogDescription>
        </DialogHeader>
        <div class="flex flex-col gap-3">
          <div class="text-muted-foreground text-[13px]">
            {{ generated.user.email }}
            <template v-if="generated.membership"> · {{ placedInto(generated.membership) }} </template>
          </div>
          <Textarea
            data-slot="generated-password"
            :model-value="generated.password"
            readonly
            rows="2"
            class="font-[family-name:var(--td-font-family-mono,ui-monospace,SFMono-Regular,Menlo,Consolas,monospace)] text-[13px] break-all"
          />
        </div>
        <DialogFooter>
          <Button variant="outline" @click="onOpenChange(false)">{{ t("common.close") }}</Button>
          <Button @click="copyGenerated">
            <CopyIcon />
            {{ t("system.usersWorkspaces.users.copyPassword") }}
          </Button>
        </DialogFooter>
      </template>

      <template v-else>
        <DialogHeader>
          <DialogTitle class="inline-flex items-center gap-2">
            <UserPlusIcon class="text-primary size-5 shrink-0" aria-hidden="true" />
            <span>{{ t("system.usersWorkspaces.users.dialogTitle") }}</span>
          </DialogTitle>
          <DialogDescription class="text-muted-foreground m-0 text-[13px] leading-[1.55]">
            {{ t("system.usersWorkspaces.users.description") }}
          </DialogDescription>
        </DialogHeader>

        <form class="flex flex-col gap-4" novalidate @submit.prevent="submit">
          <div class="flex flex-col gap-1.5">
            <Label for="create-user-username">{{ t("system.usersWorkspaces.users.usernameLabel") }}</Label>
            <Input
              id="create-user-username"
              v-model="form.username"
              :placeholder="t('system.usersWorkspaces.users.usernamePlaceholder')"
              :maxlength="50"
              :aria-invalid="!!errors.username || undefined"
              autofocus
              @input="errors.username = ''"
              @blur="validateUsername"
            />
            <p v-if="errors.username" class="text-destructive m-0 text-xs">{{ errors.username }}</p>
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="create-user-email">{{ t("system.usersWorkspaces.users.emailLabel") }}</Label>
            <Input
              id="create-user-email"
              v-model="form.email"
              type="email"
              autocomplete="off"
              :placeholder="t('system.usersWorkspaces.users.emailPlaceholder')"
              :aria-invalid="!!errors.email || undefined"
              @input="errors.email = ''"
              @blur="validateEmail"
            />
            <p v-if="errors.email" class="text-destructive m-0 text-xs">{{ errors.email }}</p>
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="create-user-password">{{ t("system.usersWorkspaces.users.passwordLabel") }}</Label>
            <Input
              id="create-user-password"
              v-model="form.password"
              type="password"
              autocomplete="new-password"
              :placeholder="t('system.usersWorkspaces.users.passwordPlaceholder')"
              :aria-invalid="!!errors.password || undefined"
              @input="errors.password = ''"
              @blur="validatePassword"
            />
            <p v-if="errors.password" class="text-destructive m-0 text-xs">{{ errors.password }}</p>
          </div>
          <!-- Placing the account into a workspace is optional; the role
               field appears once a workspace is chosen. -->
          <div class="grid grid-cols-2 gap-3 max-[480px]:grid-cols-1">
            <div class="flex flex-col gap-1.5">
              <Label for="create-user-workspace">{{ t("system.usersWorkspaces.users.workspaceLabel") }}</Label>
              <Select :model-value="form.tenantId" @update:model-value="(v) => (form.tenantId = String(v ?? NONE))">
                <SelectTrigger id="create-user-workspace" class="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent position="popper" class="z-[6200]">
                  <SelectItem :value="NONE">{{ t("system.usersWorkspaces.users.workspaceNone") }}</SelectItem>
                  <SelectItem v-for="ws in workspaces" :key="ws.id" :value="String(ws.id)">{{ ws.name }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div v-if="form.tenantId !== NONE" class="flex flex-col gap-1.5">
              <Label for="create-user-role">{{ t("system.usersWorkspaces.users.roleLabel") }}</Label>
              <Select :model-value="form.role" @update:model-value="(v) => (form.role = v as TenantRole)">
                <SelectTrigger id="create-user-role" class="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent position="popper" class="z-[6200]">
                  <SelectItem v-for="role in ROLES" :key="role" :value="role">{{ formatRole(role) }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <!-- A hidden submit button lets Enter submit the form. -->
          <button type="submit" class="hidden" tabindex="-1" aria-hidden="true" />
        </form>

        <DialogFooter>
          <Button variant="outline" :disabled="submitting" @click="onOpenChange(false)">{{
            t("common.cancel")
          }}</Button>
          <Button :disabled="submitting" @click="submit">
            <Loader2Icon v-if="submitting" class="animate-spin" />
            {{ t("system.usersWorkspaces.users.submit") }}
          </Button>
        </DialogFooter>
      </template>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import { CopyIcon, KeyRoundIcon, Loader2Icon, UserPlusIcon } from "@lucide/vue";
import { createSystemUser, type CreateSystemUserRequest, type CreateSystemUserResponse } from "@/api/system";
import type { TenantInfo } from "@/api/tenant";
import type { TenantMember, TenantRole } from "@/api/tenant/members";
import { copyWithToast } from "@/utils/clipboard";
import { useRoleLabel } from "@/composables/useRoleLabel";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";

const props = defineProps<{
  open: boolean;
  /** Workspaces the account may be placed into; the catalog the page already holds. */
  workspaces: TenantInfo[];
}>();

const emit = defineEmits<{
  (e: "update:open", value: boolean): void;
  (e: "created", response: CreateSystemUserResponse): void;
}>();

const { t } = useI18n();
const { formatRole } = useRoleLabel();

// The Select needs a string value for "no workspace"; ids are rendered as
// strings for the same reason.
const NONE = "none";
const ROLES: TenantRole[] = ["viewer", "contributor", "admin", "owner"];

const submitting = ref(false);
const form = reactive({ username: "", email: "", password: "", tenantId: NONE, role: "viewer" as TenantRole });
const errors = reactive({ username: "", email: "", password: "" });
// Set after a create whose password the server generated; swaps the
// dialog to the one-time password view.
const generated = ref<{ password: string; user: CreateSystemUserResponse["user"]; membership?: TenantMember } | null>(
  null,
);

watch(
  () => props.open,
  (open) => {
    if (!open) return;
    form.username = "";
    form.email = "";
    form.password = "";
    form.tenantId = NONE;
    form.role = "viewer";
    errors.username = "";
    errors.email = "";
    errors.password = "";
    generated.value = null;
  },
);

function validateUsername(): boolean {
  const n = form.username.trim().length;
  if (n === 0) errors.username = t("system.usersWorkspaces.users.usernameRequired");
  else if (n < 2 || n > 50) errors.username = t("system.usersWorkspaces.users.usernameLength");
  else errors.username = "";
  return !errors.username;
}

function validateEmail(): boolean {
  const email = form.email.trim();
  if (!email) errors.email = t("system.usersWorkspaces.users.emailRequired");
  else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) errors.email = t("system.usersWorkspaces.users.emailInvalid");
  else errors.email = "";
  return !errors.email;
}

// An empty password means "generate one"; anything typed must satisfy the
// same policy the server enforces, so the user hears about it before the
// round-trip.
function validatePassword(): boolean {
  const pwd = form.password;
  const ok = pwd === "" || (pwd.length >= 8 && pwd.length <= 32 && /[a-zA-Z]/.test(pwd) && /\d/.test(pwd));
  errors.password = ok ? "" : t("system.usersWorkspaces.users.passwordPolicy");
  return ok;
}

function placedInto(membership: TenantMember): string {
  const ws = props.workspaces.find((w) => String(w.id) === form.tenantId);
  return t("system.usersWorkspaces.users.placedInto", {
    name: ws?.name ?? `#${form.tenantId}`,
    role: formatRole(membership.role),
  });
}

function onOpenChange(open: boolean) {
  if (!open && submitting.value) return;
  emit("update:open", open);
}

async function copyGenerated() {
  if (!generated.value) return;
  const ok = await copyWithToast(generated.value.password, "system.usersWorkspaces.users.copied");
  if (ok) emit("update:open", false);
}

async function submit() {
  if (submitting.value) return;
  const results = [validateUsername(), validateEmail(), validatePassword()];
  if (!results.every(Boolean)) return;

  const payload: CreateSystemUserRequest = { username: form.username.trim(), email: form.email.trim() };
  if (form.password !== "") payload.password = form.password;
  if (form.tenantId !== NONE) {
    payload.tenant_id = Number(form.tenantId);
    payload.role = form.role;
  }

  submitting.value = true;
  try {
    const response = await createSystemUser(payload);
    emit("created", response);
    if (response.generated_password) {
      generated.value = {
        password: response.generated_password,
        user: response.user,
        membership: response.membership,
      };
      return;
    }
    MessagePlugin.success(
      response.membership
        ? `${t("system.usersWorkspaces.users.success")} · ${placedInto(response.membership)}`
        : t("system.usersWorkspaces.users.success"),
    );
    emit("update:open", false);
  } catch (err: any) {
    MessagePlugin.error(err?.message || t("system.usersWorkspaces.users.failed"));
  } finally {
    submitting.value = false;
  }
}
</script>
