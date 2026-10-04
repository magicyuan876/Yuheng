<template>
  <div class="flex min-h-0 min-w-0 flex-1 flex-col p-[20px_28px_0]">
    <div class="mb-4 flex flex-none items-center justify-between">
      <div class="flex flex-col gap-1">
        <div class="flex items-center gap-2">
          <h2 class="text-foreground m-0 font-[family-name:var(--app-font-family)] text-[20px] leading-7 font-semibold">
            {{ t("docs.title") }}
          </h2>
          <Tooltip v-if="canCreate">
            <TooltipTrigger as-child>
              <Button
                variant="ghost"
                size="icon-sm"
                class="text-muted-foreground"
                :aria-label="t('docs.spaces.create')"
                @click="openCreate"
              >
                <PlusIcon />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{{ t("docs.spaces.create") }}</TooltipContent>
          </Tooltip>
        </div>
        <p class="text-muted-foreground m-0 font-[family-name:var(--app-font-family)] text-sm leading-5">
          {{ t("docs.subtitle") }}
        </p>
      </div>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto pb-6">
      <!-- Skeleton while the first load is in flight -->
      <div v-if="loading && spaces.length === 0" class="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-3">
        <div
          v-for="n in 4"
          :key="'skel-' + n"
          class="bg-card flex min-h-[132px] cursor-default flex-col gap-2.5 rounded-[8px] border border-[var(--td-component-stroke)] p-[14px_16px] shadow-[0_1px_3px_rgba(0,0,0,0.04)]"
        >
          <div class="flex items-center gap-2.5">
            <Skeleton class="size-9 rounded-full" />
            <Skeleton class="h-5 w-1/2" />
          </div>
          <Skeleton class="h-3.5 w-full" />
          <Skeleton class="h-3.5 w-[70%]" />
        </div>
      </div>

      <!-- Empty state -->
      <div
        v-else-if="!loading && spaces.length === 0"
        class="flex flex-col items-center gap-2 p-[72px_16px] text-center"
      >
        <img src="@/assets/img/docs.svg" class="h-12 w-12 opacity-60" alt="" aria-hidden="true" />
        <div class="text-foreground text-base font-semibold">{{ t("docs.spaces.empty") }}</div>
        <div class="text-muted-foreground mb-2 max-w-[42ch] text-[13px]">
          {{ canCreate ? t("docs.spaces.emptyHint") : t("docs.spaces.emptyHintReadOnly") }}
        </div>
        <Button v-if="canCreate" @click="openCreate">{{ t("docs.spaces.create") }}</Button>
      </div>

      <!-- Cards -->
      <div v-else class="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-3">
        <div
          v-for="space in spaces"
          :key="space.id"
          class="bg-card flex min-h-[132px] cursor-pointer flex-col gap-2.5 rounded-[8px] border border-[var(--td-component-stroke)] p-[14px_16px] shadow-[0_1px_3px_rgba(0,0,0,0.04)] transition-[border-color,box-shadow] duration-200 hover:border-[var(--td-brand-color)] hover:shadow-[0_4px_12px_rgba(0,0,0,0.06)] focus-visible:border-[var(--td-brand-color)] focus-visible:shadow-[0_4px_12px_rgba(0,0,0,0.06)] focus-visible:outline-none"
          role="link"
          tabindex="0"
          @click="openSpace(space)"
          @keydown.enter.prevent="openSpace(space)"
        >
          <div class="flex items-center gap-2.5">
            <SpaceAvatar :name="space.name" :avatar="space.icon || ''" size="small" />
            <div class="flex min-w-0 flex-1 flex-col">
              <span class="text-foreground truncate text-[15px] leading-[22px] font-semibold" :title="space.name">
                {{ space.name }}
              </span>
              <span
                class="text-placeholder truncate font-[family-name:var(--td-font-family-mono,ui-monospace,monospace)] text-xs leading-4"
                >/{{ space.slug }}</span
              >
            </div>
            <!-- Light, small t-tags: square-ish corners, regular weight. -->
            <Badge
              v-if="canManageSpace(space.role)"
              class="text-primary flex-none rounded-[3px] bg-[var(--td-brand-color-light)] font-normal"
            >
              {{ t("docs.spaces.role." + space.role) }}
            </Badge>
            <Badge v-else variant="secondary" class="flex-none rounded-[3px] font-normal">
              {{ t("docs.spaces.role." + space.role) }}
            </Badge>
          </div>
          <div
            class="text-muted-foreground [display:-webkit-box] flex-1 overflow-hidden text-[13px] leading-5 [-webkit-box-orient:vertical] [-webkit-line-clamp:2]"
          >
            {{ space.description || t("docs.spaces.noDescription") }}
          </div>
          <div class="flex flex-wrap gap-1.5">
            <span
              class="inline-flex h-[22px] items-center gap-1 rounded-[5px] px-[7px] text-xs"
              :class="
                space.visibility === 'open'
                  ? 'text-primary bg-[var(--td-brand-color-light)]'
                  : 'bg-secondary text-muted-foreground'
              "
            >
              <LockIcon v-if="space.visibility === 'private'" class="size-3" />
              <UsersRoundIcon v-else class="size-3" />
              {{ t("docs.spaces.visibility." + space.visibility) }}
            </span>
            <span
              class="bg-secondary text-muted-foreground inline-flex h-[22px] items-center gap-1 rounded-[5px] px-[7px] text-xs"
            >
              <UserRoundIcon class="size-3" />
              {{ t("docs.spaces.memberCount", { count: space.member_count }) }}
            </span>
            <span
              class="bg-secondary text-muted-foreground inline-flex h-[22px] items-center gap-1 rounded-[5px] px-[7px] text-xs"
            >
              <FileIcon class="size-3" />
              {{ t("docs.spaces.pageCount", { count: space.page_count }) }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <Dialog :open="createVisible" @update:open="(v: boolean) => (createVisible = v)">
      <DialogContent class="sm:max-w-[540px]">
        <DialogHeader>
          <DialogTitle>{{ t("docs.spaces.createTitle") }}</DialogTitle>
        </DialogHeader>
        <SpaceForm v-model="form" mode="create" :problems="problems" />
        <KnowledgeBaseSyncField v-model="kbChoice" :options="syncOptions" :disabled="creating" />
        <DialogFooter>
          <Button variant="outline" @click="createVisible = false">{{ t("common.cancel") }}</Button>
          <Button :disabled="creating || !choiceComplete(kbChoice)" @click="submitCreate">
            <Loader2Icon v-if="creating" class="animate-spin" />
            {{ t("common.create") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { FileIcon, Loader2Icon, LockIcon, PlusIcon, UserRoundIcon, UsersRoundIcon } from "@lucide/vue";
import { MessagePlugin } from "tdesign-vue-next";
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

import { createSpace, listSpaces, type DocsSpace, type KnowledgeBaseChoice } from "@/api/docs";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import SpaceAvatar from "@/components/SpaceAvatar.vue";
import { useAuthStore } from "@/stores/auth";

import {
  canManageSpace,
  normaliseSpaceForm,
  validateSpaceForm,
  type SpaceFormModel,
  type SpaceFormProblem,
} from "./docsAccess";
import KnowledgeBaseSyncField from "./KnowledgeBaseSyncField.vue";
import {
  choiceComplete,
  choiceRequest,
  isEmbeddingModelRequired,
  loadSyncOptions,
  type SyncOptions,
} from "./knowledgeBaseSync";
import SpaceForm from "./SpaceForm.vue";

const { t } = useI18n();
const router = useRouter();
const authStore = useAuthStore();

const spaces = ref<DocsSpace[]>([]);
const loading = ref(false);
const canCreate = computed(() => authStore.hasRole("contributor"));

const load = async () => {
  loading.value = true;
  try {
    spaces.value = await listSpaces();
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.spaces.loadFailed")));
  } finally {
    loading.value = false;
  }
};

const openSpace = (space: DocsSpace) => {
  router.push({ name: "docsSpace", params: { slug: space.slug } });
};

// ---- create ------------------------------------------------------------------
const emptyForm = (): SpaceFormModel => ({
  name: "",
  slug: "",
  description: "",
  visibility: "private",
  default_role: "none",
});
const createVisible = ref(false);
const creating = ref(false);
const form = ref<SpaceFormModel>(emptyForm());
const problems = ref<SpaceFormProblem[]>([]);

// What the new space syncs into. A new knowledge base by default: writing
// here rather than in a wiki next door is worth it because what is written
// can then be asked about, and a team that first has to go and make a
// knowledge base mostly does not. Without an embedding model none can be
// made, and the field falls back to not syncing and says why.
const kbChoice = ref<KnowledgeBaseChoice>({ mode: "create" });
const syncOptions = ref<SyncOptions | null>(null);

const loadKnowledgeBaseOptions = async () => {
  syncOptions.value = null;
  const options = await loadSyncOptions({ userId: authStore.currentUserId, isAdmin: authStore.hasRole("admin") });
  syncOptions.value = options;
  if (kbChoice.value.mode === "create" && !options.canCreate) kbChoice.value = { mode: "none" };
};

const openCreate = () => {
  form.value = emptyForm();
  problems.value = [];
  kbChoice.value = { mode: "create" };
  createVisible.value = true;
  void loadKnowledgeBaseOptions();
};

const submitCreate = async () => {
  const model = normaliseSpaceForm({ ...form.value, name: form.value.name.trim(), slug: form.value.slug.trim() });
  problems.value = validateSpaceForm(model);
  if (problems.value.length) return;
  creating.value = true;
  try {
    const created = await createSpace({
      name: model.name,
      slug: model.slug || undefined,
      description: model.description.trim(),
      visibility: model.visibility,
      default_role: model.default_role,
      knowledge_base: choiceRequest(kbChoice.value),
    });
    MessagePlugin.success(t("docs.spaces.createSuccess"));
    createVisible.value = false;
    spaces.value = [...spaces.value, created].sort((a, b) => a.name.localeCompare(b.name));
    router.push({ name: "docsSpace", params: { slug: created.slug } });
  } catch (err: unknown) {
    if (isEmbeddingModelRequired(err)) {
      // The model was removed since the form loaded: show the field as it
      // now is rather than a bare error.
      MessagePlugin.error(t("docs.spaces.kbSync.noEmbedding"));
      void loadKnowledgeBaseOptions();
    } else {
      MessagePlugin.error(errorText(err, t("docs.spaces.createFailed")));
    }
  } finally {
    creating.value = false;
  }
};

function errorText(err: unknown, fallback: string): string {
  const msg = (err as { message?: string } | null)?.message;
  return msg ? `${fallback}: ${msg}` : fallback;
}

onMounted(load);
</script>
