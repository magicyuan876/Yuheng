<template>
  <!--
    The system-administrator roster as a tags input: one tag per email.
    Typing an email and pressing Enter promotes, the × on a tag revokes,
    each after an inline confirm. NOT a system_setting row — it is backed
    by the user table through the promote / revoke APIs.

    Self-edit safety: the current user is excluded from the visible tags.
    They cannot revoke themselves anyway (the backend refuses), and showing
    a tag that cannot be removed is worse than not showing it.
  -->
  <div class="flex items-center justify-end gap-2 max-[860px]:w-full max-[860px]:justify-start">
    <Popover :open="popconfirm.visible" @update:open="popconfirm.onVisibleChange">
      <PopoverAnchor as-child>
        <div class="min-w-0 flex-1">
          <TagsFieldInput
            v-model="adminEmails"
            class="w-[320px] max-[860px]:w-full"
            :placeholder="t('system.usersWorkspaces.admins.placeholder')"
            :aria-label="t('system.usersWorkspaces.admins.label')"
            :disabled="busy"
            @change="onAdminsChange"
          />
        </div>
      </PopoverAnchor>
      <PopoverContent side="left" class="w-72">
        <p class="mb-3 text-[13px] leading-[1.5]">{{ popconfirm.content }}</p>
        <div class="flex justify-end gap-2">
          <Button size="sm" variant="outline" @click="popconfirm.finish(false)">
            {{ t("system.globalSettings.confirm.cancelBtn") }}
          </Button>
          <Button size="sm" :class="confirmBtnClass(popconfirm.confirmBtn.theme)" @click="popconfirm.finish(true)">
            {{ popconfirm.confirmBtn.content }}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
    <div
      v-if="busy"
      class="text-muted-foreground inline-flex min-w-[52px] shrink-0 items-center gap-[5px] text-xs"
      role="status"
    >
      <Loader2Icon class="size-3.5 animate-spin" />
      <span>{{ t("system.globalSettings.saving") }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import { Loader2Icon } from "@lucide/vue";
import { listSystemAdmins, promoteUserToSystemAdmin, revokeSystemAdmin } from "@/api/system";
import { useAuthStore } from "@/stores/auth";
import TagsFieldInput from "@/components/settings/TagsFieldInput.vue";
import { Button } from "@/components/ui/button";
import { Popover, PopoverAnchor, PopoverContent } from "@/components/ui/popover";
import { confirmBtnClass, createInlinePopconfirm } from "./inlineConfirm";

const { t, te } = useI18n();
const authStore = useAuthStore();
const currentUserId = computed(() => authStore.currentUserId);

// Two parallel structures:
//   - adminEmails: the v-model bound to the tags input (excludes the
//     current user; that's the visible source of truth).
//   - adminEmailToId: email → user UUID, populated from the list
//     endpoint. Needed because revoke takes a UUID, not an email.
// Both reset on every reload to avoid stale entries persisting after a
// peer administrator makes a change. busy disables the input and shows
// the spinner only while promote / revoke calls are in flight — not while
// the inline confirm is waiting for a click.
const adminEmails = ref<string[]>([]);
const adminEmailToId = ref<Record<string, string>>({});
const busy = ref(false);
const popconfirm = createInlinePopconfirm();

function text(path: string, params?: Record<string, string>): string {
  if (!te(path)) return path;
  const msg = params ? t(path, params) : t(path);
  return typeof msg === "string" ? msg : path;
}

// loadAdmins refreshes the tags list + the email→id lookup table.
async function loadAdmins() {
  try {
    const resp = await listSystemAdmins({ limit: 200 });
    const map: Record<string, string> = {};
    const emails: string[] = [];
    for (const u of resp.admins ?? []) {
      // Empty emails would collapse to a single tag "" that can't be
      // round-tripped to a user_id; skip them.
      if (!u.email) continue;
      map[u.email] = u.id;
      if (u.id !== currentUserId.value) emails.push(u.email);
    }
    adminEmailToId.value = map;
    adminEmails.value = emails;
  } catch (err: any) {
    MessagePlugin.error(err?.message || t("system.usersWorkspaces.admins.loadFailed"));
  }
}

function confirmAdminChange(action: "promote" | "revoke", email: string): Promise<boolean> {
  const base = `system.usersWorkspaces.admins.confirm.${action}`;
  return popconfirm.ask({
    content: text(`${base}.body`, { email }),
    theme: action === "revoke" ? "danger" : "warning",
    confirmBtn: {
      content: text(`${base}.confirmBtn`),
      theme: action === "revoke" ? "danger" : "primary",
    },
  });
}

// onAdminsChange diffs the new tag list against the canonical state and
// dispatches one promote / revoke per delta. Failures roll the whole tag
// list back to the server-side truth — simpler than undoing individual
// ops, and rare enough that a full reload surprises nobody.
async function onAdminsChange(next: string[]) {
  if (busy.value) return;

  const authoritative = new Set<string>();
  for (const email of Object.keys(adminEmailToId.value)) {
    if (adminEmailToId.value[email] !== currentUserId.value) authoritative.add(email);
  }
  const nextSet = new Set(next.map((e) => e.trim()).filter(Boolean));

  const added: string[] = [];
  for (const email of nextSet) if (!authoritative.has(email)) added.push(email);
  const removed: string[] = [];
  for (const email of authoritative) if (!nextSet.has(email)) removed.push(email);
  if (added.length === 0 && removed.length === 0) return;

  // Confirm before any privilege change; a declined confirm snaps the
  // input back to the roster as it is.
  for (const email of added) {
    if (!(await confirmAdminChange("promote", email))) {
      await loadAdmins();
      return;
    }
  }
  for (const email of removed) {
    if (!adminEmailToId.value[email]) continue;
    if (!(await confirmAdminChange("revoke", email))) {
      await loadAdmins();
      return;
    }
  }

  busy.value = true;
  let applied = 0;
  try {
    for (const email of added) {
      await promoteUserToSystemAdmin({ email });
      applied++;
    }
    for (const email of removed) {
      const userId = adminEmailToId.value[email];
      if (!userId) continue;
      await revokeSystemAdmin(userId);
      applied++;
    }
    await loadAdmins();
    if (applied > 0) MessagePlugin.success(t("system.usersWorkspaces.admins.saveSuccess"));
  } catch (err: any) {
    MessagePlugin.error(err?.message || t("system.usersWorkspaces.admins.saveFailed"));
    await loadAdmins();
  } finally {
    busy.value = false;
  }
}

onMounted(loadAdmins);
</script>
