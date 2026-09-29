<template>
  <div class="flex min-h-20 flex-col gap-3.5">
    <div v-if="loading && !links.length" class="flex items-center justify-center gap-2 py-6">
      <Loader2Icon class="size-4 animate-spin" />
    </div>
    <template v-else>
      <Alert v-if="!sharingAvailable" class="bg-primary/5 border-primary/30 m-0">
        <InfoIcon />
        <AlertDescription>{{ t("docs.share.disabled") }}</AlertDescription>
      </Alert>

      <template v-else>
        <ul v-if="links.length" class="m-0 flex list-none flex-col gap-2.5 p-0">
          <li v-for="link in links" :key="link.id" class="flex items-start gap-2">
            <div class="min-w-0 flex-1">
              <button
                type="button"
                data-slot="share-link-url"
                class="text-primary flex max-w-full cursor-pointer items-center gap-1.5 border-0 p-0 text-[13px]"
                :title="urlOf(link)"
                @click="copy(link)"
              >
                <LinkIcon class="size-3.5 flex-none" />
                <span class="truncate">{{ urlOf(link) }}</span>
              </button>
              <div class="text-placeholder mt-1 flex flex-wrap items-center gap-2.5 text-xs">
                <!-- A link that resolves to nothing is the one fact the owner
                     most needs; it leads. -->
                <Badge v-if="!link.live" class="bg-warning/10 text-warning">{{ t("docs.share.notLive") }}</Badge>
                <span v-if="link.has_password" class="inline-flex items-center gap-[3px]">
                  <LockIcon class="size-3" /> {{ t("docs.share.hasPassword") }}
                </span>
                <span v-if="link.include_children" class="inline-flex items-center gap-[3px]">
                  <FolderTreeIcon class="size-3" /> {{ t("docs.share.withChildren") }}
                </span>
                <span v-if="link.expires_at" class="inline-flex items-center gap-[3px]">
                  <ClockIcon class="size-3" /> {{ t("docs.share.expires", { date: dateOf(link.expires_at) }) }}
                </span>
                <span v-if="link.allow_search_index" class="inline-flex items-center gap-[3px]">
                  <SearchIcon class="size-3" /> {{ t("docs.share.indexed") }}
                </span>
                <span>{{ t("docs.share.views", { n: link.view_count }) }}</span>
              </div>
            </div>
            <Button
              v-if="canManage"
              variant="ghost"
              size="sm"
              class="text-destructive hover:text-destructive flex-none"
              :disabled="busy === link.id"
              @click="confirmRevoke(link)"
            >
              <Loader2Icon v-if="busy === link.id" class="animate-spin" />
              {{ t("docs.share.revoke") }}
            </Button>
          </li>
        </ul>
        <p v-else class="text-placeholder m-0 text-[13px]">{{ t("docs.share.none") }}</p>

        <!-- Restricted pages cannot be published at all, and saying why is
             more useful than a disabled button with no explanation. -->
        <Alert v-if="restricted" class="bg-warning/5 border-warning/40 m-0">
          <TriangleAlertIcon />
          <AlertDescription>{{ t("docs.share.restrictedPage") }}</AlertDescription>
        </Alert>

        <form
          v-else-if="canManage"
          class="flex flex-col gap-2 border-t border-[var(--td-component-stroke)] pt-3"
          @submit.prevent="create"
        >
          <div class="flex flex-wrap gap-4">
            <div class="flex items-center gap-2">
              <Checkbox id="share-option-children" v-model="includeChildren" />
              <Label for="share-option-children" class="font-normal">{{ t("docs.share.optionChildren") }}</Label>
            </div>
            <div class="flex items-center gap-2">
              <Checkbox id="share-option-index" v-model="allowIndex" />
              <Label for="share-option-index" class="font-normal">{{ t("docs.share.optionIndex") }}</Label>
            </div>
          </div>
          <div class="flex flex-wrap gap-2">
            <Input
              v-model="password"
              type="password"
              class="h-7 min-w-[140px] flex-1 text-xs md:text-xs"
              :placeholder="t('docs.share.optionPassword')"
              :maxlength="64"
            />
            <Input
              v-model="expiresAt"
              type="date"
              class="h-7 w-[160px] text-xs md:text-xs"
              :min="today"
              :placeholder="t('docs.share.optionExpiry')"
            />
            <Button size="sm" type="submit" :disabled="creating">
              <Loader2Icon v-if="creating" class="animate-spin" />
              {{ t("docs.share.create") }}
            </Button>
          </div>
        </form>
      </template>
    </template>

    <Dialog :open="revokeVisible" @update:open="(v: boolean) => (revokeVisible = v)">
      <DialogContent class="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>{{ t("docs.share.revokeHeader") }}</DialogTitle>
        </DialogHeader>
        <p class="text-foreground text-sm">{{ t("docs.share.revokeBody") }}</p>
        <DialogFooter>
          <Button variant="outline" @click="revokeVisible = false">{{ t("common.cancel") }}</Button>
          <Button variant="destructive" @click="revoke">
            <Loader2Icon v-if="busy" class="animate-spin" />
            {{ t("docs.share.revoke") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";

import {
  ClockIcon,
  FolderTreeIcon,
  InfoIcon,
  LinkIcon,
  Loader2Icon,
  LockIcon,
  SearchIcon,
  TriangleAlertIcon,
} from "@lucide/vue";

import { createShare, listShares, revokeShare, type ShareView } from "@/api/docs";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useDeploymentCapabilitiesStore } from "@/stores/deploymentCapabilities";

// Publishing a page to the internet, and seeing what is already published.
//
// The list is shown to every reader of the page, not only to the people who
// may change it: "this is on the internet" is exactly the kind of fact whose
// audience should be as wide as possible, because the person most likely to
// notice a mistake is not always the person who made it.

const props = defineProps<{ pageId: string; canManage: boolean; restricted: boolean }>();

const { t, locale } = useI18n();
const capabilities = useDeploymentCapabilitiesStore();

/** Whether this deployment allows anything to be published at all. Hiding
 * the control beats offering one whose only outcome is a refusal. */
const sharingAvailable = computed(() => capabilities.isSupported("docs.public_sharing"));

const loading = ref(false);
const creating = ref(false);
const busy = ref("");
const links = shallowRef<ShareView[]>([]);

const includeChildren = ref(false);
const allowIndex = ref(false);
const password = ref("");
const expiresAt = ref("");

const revokeVisible = ref(false);
const pending = ref<ShareView | null>(null);

const canManage = computed(() => props.canManage);

/** Earliest day the picker offers: today, in the reader's own calendar (the
 * old picker disabled only days before today). */
const today = (() => {
  const now = new Date();
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
})();

/** The address a visitor opens. Built here rather than returned by the
 * server, which does not know how this deployment is addressed from outside. */
function urlOf(link: ShareView): string {
  return `${window.location.origin}/d/${link.key}`;
}

function dateOf(iso?: string): string {
  if (!iso) return "";
  const at = new Date(iso);
  return Number.isNaN(at.getTime()) ? "" : at.toLocaleDateString(locale.value);
}

function fail(err: unknown, fallback: string) {
  const msg = (err as { message?: string } | null)?.message;
  void MessagePlugin.error(msg ? `${fallback}: ${msg}` : fallback);
}

async function load() {
  if (!props.pageId) return;
  loading.value = true;
  const id = props.pageId;
  try {
    const rows = await listShares(id);
    if (props.pageId === id) links.value = rows;
  } catch (err) {
    fail(err, t("docs.share.loadFailed"));
  } finally {
    if (props.pageId === id) loading.value = false;
  }
}

async function create() {
  creating.value = true;
  try {
    const made = await createShare(props.pageId, {
      include_children: includeChildren.value,
      allow_search_index: allowIndex.value,
      password: password.value || undefined,
      expires_at: expiresAt.value ? new Date(expiresAt.value).toISOString() : undefined,
    });
    links.value = [made, ...links.value];
    password.value = "";
    expiresAt.value = "";
    await copy(made);
  } catch (err) {
    fail(err, t("docs.share.createFailed"));
  } finally {
    creating.value = false;
  }
}

function confirmRevoke(link: ShareView) {
  pending.value = link;
  revokeVisible.value = true;
}

async function revoke() {
  const link = pending.value;
  revokeVisible.value = false;
  if (!link) return;
  busy.value = link.id;
  try {
    await revokeShare(props.pageId, link.id);
    links.value = links.value.filter((l) => l.id !== link.id);
  } catch (err) {
    fail(err, t("docs.share.revokeFailed"));
  } finally {
    busy.value = "";
  }
}

async function copy(link: ShareView) {
  try {
    await navigator.clipboard.writeText(urlOf(link));
    void MessagePlugin.success(t("docs.share.copied"));
  } catch {
    // Clipboard access is refused in plenty of ordinary situations (an
    // insecure origin, a permissions policy). The URL is on screen and
    // selectable, so this is a convenience that failed, not the feature.
    void MessagePlugin.info(t("docs.share.copyManually"));
  }
}

watch(
  () => props.pageId,
  () => {
    void load();
  },
  { immediate: true },
);
</script>
