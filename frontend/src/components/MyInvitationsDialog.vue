<template>
  <!-- "My invitations" inbox as a dialog. The previous version of
       this was a full /platform/invitations route, but the inbox is
       intrinsically transient — open it, act, dismiss — and a modal
       keeps the user in whatever context the bell icon was clicked
       from rather than yanking them to a separate page. The trigger
       lives in UserMenu's avatar-row bell. -->
  <Dialog v-model:open="visibleModel">
    <DialogContent class="sm:max-w-[540px]">
      <DialogHeader>
        <DialogTitle>{{ $t("tenantInvitation.myInbox.title") }}</DialogTitle>
        <DialogDescription class="m-0 text-[13px] leading-[1.55]">
          {{ $t("tenantInvitation.myInbox.description") }}
        </DialogDescription>
      </DialogHeader>

      <div v-if="loading" class="flex items-center gap-2 py-2">
        <Loader2Icon class="text-primary size-4 animate-spin" />
        <span>{{ $t("tenantMember.loading") }}</span>
      </div>

      <div v-else-if="error" class="py-2">
        <Alert variant="destructive" class="flex items-center gap-2">
          <CircleXIcon class="size-4 shrink-0" />
          <AlertDescription class="flex-1">{{ error }}</AlertDescription>
          <Button size="sm" @click="reload">{{ $t("tenantMember.retry") }}</Button>
        </Alert>
      </div>

      <Empty v-else-if="invitations.length === 0" class="px-0 pt-4 pb-2">
        <EmptyMedia>
          <InboxIcon class="text-placeholder size-12" :stroke-width="1.25" />
        </EmptyMedia>
        <EmptyDescription>{{ $t("tenantInvitation.myInbox.empty") }}</EmptyDescription>
      </Empty>

      <!-- The list caps its height so a user with many invitations scrolls
           within the dialog rather than the dialog growing past the viewport. -->
      <ul v-else class="m-0 flex max-h-[60vh] list-none flex-col gap-2.5 overflow-y-auto p-0">
        <li
          v-for="row in invitations"
          :key="row.id"
          class="bg-card border-border flex items-stretch gap-3 rounded-lg border px-3.5 py-3"
        >
          <div class="flex min-w-0 flex-auto flex-col gap-1.5">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-foreground text-sm font-semibold">
                {{ row.tenant_name || $t("tenantInvitation.myInbox.tenantLabel") + " #" + row.tenant_id }}
              </span>
              <span
                class="inline-flex h-[22px] items-center rounded-[3px] px-2 text-xs leading-none"
                :class="roleTagClass(row.role)"
              >
                {{ $t("tenantMember.role." + row.role) }}
              </span>
            </div>
            <div class="text-muted-foreground flex flex-col gap-[3px] text-xs">
              <span class="inline-flex items-center gap-1">
                <UserIcon class="text-placeholder size-3.5 shrink-0" />
                <span>{{ $t("tenantInvitation.myInbox.from") }}：</span>
                <span class="text-foreground">{{ inviterDisplay(row) }}</span>
              </span>
              <span class="inline-flex items-center gap-1">
                <ClockIcon class="text-placeholder size-3.5 shrink-0" />
                <span class="text-foreground">
                  {{ $t("tenantInvitation.myInbox.expiresIn", { date: formatDate(row.expires_at) }) }}
                </span>
              </span>
              <span v-if="row.message" class="inline-flex items-start gap-1">
                <MessageSquareIcon class="text-placeholder mt-0.5 size-3.5 shrink-0" />
                <span>{{ $t("tenantInvitation.myInbox.messageLabel") }}：</span>
                <span class="text-foreground break-words">{{ row.message }}</span>
              </span>
            </div>
          </div>
          <div class="flex shrink-0 flex-col items-stretch justify-center gap-1.5">
            <Button size="sm" :disabled="acting === row.id" @click="onAccept(row)">
              <Loader2Icon v-if="acting === row.id" class="animate-spin" />
              {{ $t("tenantInvitation.myInbox.acceptButton") }}
            </Button>
            <Button variant="outline" size="sm" :disabled="acting === row.id" @click="onDecline(row)">
              <Loader2Icon v-if="acting === row.id" class="animate-spin" />
              {{ $t("tenantInvitation.myInbox.declineButton") }}
            </Button>
          </div>
        </li>
      </ul>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { ClockIcon, CircleXIcon, InboxIcon, Loader2Icon, MessageSquareIcon, UserIcon } from "@lucide/vue";
import { MessagePlugin } from "tdesign-vue-next";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Empty, EmptyDescription, EmptyMedia } from "@/components/ui/empty";
import { useAuthStore } from "@/stores/auth";
import {
  listMyInvitations,
  acceptInvitation,
  declineInvitation,
  type TenantInvitation,
} from "@/api/tenant/invitations";
import type { TenantRole } from "@/api/tenant/members";

// v-model:visible — the parent (UserMenu) owns the open/close state
// so the bell icon stays the single source of truth. Reload is run
// on every open transition so the list reflects accept/decline
// actions taken in another tab while the dialog was closed.
const props = defineProps<{ visible: boolean }>();
const emit = defineEmits<{ (e: "update:visible", v: boolean): void }>();
const visibleModel = computed({
  get: () => props.visible,
  set: (v) => emit("update:visible", v),
});

const { t, locale } = useI18n();
const authStore = useAuthStore();

const invitations = ref<TenantInvitation[]>([]);
const loading = ref(false);
const error = ref("");
const acting = ref<number | null>(null);

// The role tag keeps the solid fills the old TDesign tag themes had
// (owner primary, admin warning, contributor success, the rest neutral).
function roleTagClass(role: TenantRole): string {
  switch (role) {
    case "owner":
      return "bg-primary text-primary-foreground";
    case "admin":
      return "bg-warning text-primary-foreground";
    case "contributor":
      return "bg-success text-primary-foreground";
    default:
      return "bg-muted text-foreground";
  }
}

function inviterDisplay(row: TenantInvitation): string {
  return row.inviter_name?.trim() || row.inviter_email?.trim() || row.invited_by || "—";
}

function formatDate(s: string): string {
  if (!s) return "-";
  try {
    return new Intl.DateTimeFormat(locale.value || "zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    }).format(new Date(s));
  } catch {
    return s;
  }
}

async function reload() {
  loading.value = true;
  error.value = "";
  try {
    const resp = await listMyInvitations();
    if (resp.success && resp.data) {
      invitations.value = resp.data.invitations;
      authStore.setPendingInvitationCount(invitations.value.filter((i) => i.status === "pending").length);
    } else {
      error.value = resp.message || t("tenantInvitation.errors.generic");
    }
  } catch (err: any) {
    error.value = err?.message || t("tenantInvitation.errors.generic");
  } finally {
    loading.value = false;
  }
}

async function onAccept(row: TenantInvitation) {
  acting.value = row.id;
  try {
    const resp = await acceptInvitation(row.id);
    if (resp.success) {
      invitations.value = invitations.value.filter((x) => x.id !== row.id);
      authStore.setPendingInvitationCount(Math.max(0, authStore.pendingInvitationCount - 1));
      await authStore.refreshFromAuthMe();
      MessagePlugin.success(
        t("tenantInvitation.myInbox.acceptSuccess", {
          tenant: row.tenant_name || `#${row.tenant_id}`,
        }),
      );
    } else {
      MessagePlugin.error(resp.message || t("tenantInvitation.errors.generic"));
    }
  } catch (err: any) {
    const status = err?.status;
    if (status === 404) MessagePlugin.error(t("tenantInvitation.errors.notFound"));
    else if (status === 403) MessagePlugin.error(t("tenantInvitation.errors.forbidden"));
    else if (status === 409) MessagePlugin.error(err?.message || t("tenantInvitation.errors.notPending"));
    else MessagePlugin.error(err?.message || t("tenantInvitation.errors.generic"));
  } finally {
    acting.value = null;
  }
}

async function onDecline(row: TenantInvitation) {
  acting.value = row.id;
  try {
    const resp = await declineInvitation(row.id);
    if (resp.success) {
      invitations.value = invitations.value.filter((x) => x.id !== row.id);
      authStore.setPendingInvitationCount(Math.max(0, authStore.pendingInvitationCount - 1));
      MessagePlugin.success(t("tenantInvitation.myInbox.declineSuccess"));
    } else {
      MessagePlugin.error(resp.message || t("tenantInvitation.errors.generic"));
    }
  } catch (err: any) {
    const status = err?.status;
    if (status === 404) MessagePlugin.error(t("tenantInvitation.errors.notFound"));
    else if (status === 403) MessagePlugin.error(t("tenantInvitation.errors.forbidden"));
    else if (status === 409) MessagePlugin.error(err?.message || t("tenantInvitation.errors.notPending"));
    else MessagePlugin.error(err?.message || t("tenantInvitation.errors.generic"));
  } finally {
    acting.value = null;
  }
}

// Refetch on every open. Costs one round-trip but the user clicked
// here specifically to see the latest state; serving stale data
// would only confuse the empty-state vs "1 invitation" cases.
watch(
  () => props.visible,
  (v) => {
    if (v) reload();
  },
);
</script>
