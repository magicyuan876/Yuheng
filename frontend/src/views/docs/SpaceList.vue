<template>
  <div class="docs-space-list">
    <div class="header" style="--wails-draggable: drag">
      <div class="header-title">
        <div class="title-row">
          <h2>{{ t("docs.title") }}</h2>
          <t-tooltip v-if="canCreate" :content="t('docs.spaces.create')" placement="bottom">
            <t-button
              variant="text"
              theme="default"
              size="small"
              class="header-action-btn"
              style="--wails-draggable: no-drag"
              @click="openCreate"
            >
              <template #icon><t-icon name="add" size="16px" /></template>
            </t-button>
          </t-tooltip>
        </div>
        <p class="header-subtitle">{{ t("docs.subtitle") }}</p>
      </div>
    </div>

    <div class="list-main">
      <!-- Skeleton while the first load is in flight -->
      <div v-if="loading && spaces.length === 0" class="card-grid">
        <div v-for="n in 4" :key="'skel-' + n" class="space-card space-card--skeleton">
          <t-skeleton
            animation="gradient"
            :row-col="[
              [
                { width: '36px', height: '36px', type: 'circle' },
                { width: '50%', height: '20px' },
              ],
            ]"
          />
          <t-skeleton
            animation="gradient"
            :row-col="[
              { width: '100%', height: '14px' },
              { width: '70%', height: '14px' },
            ]"
          />
        </div>
      </div>

      <!-- Empty state -->
      <div v-else-if="!loading && spaces.length === 0" class="empty-state">
        <img src="@/assets/img/docs.svg" class="empty-icon" alt="" aria-hidden="true" />
        <div class="empty-title">{{ t("docs.spaces.empty") }}</div>
        <div class="empty-hint">{{ canCreate ? t("docs.spaces.emptyHint") : t("docs.spaces.emptyHintReadOnly") }}</div>
        <t-button v-if="canCreate" theme="primary" @click="openCreate">{{ t("docs.spaces.create") }}</t-button>
      </div>

      <!-- Cards -->
      <div v-else class="card-grid">
        <div
          v-for="space in spaces"
          :key="space.id"
          class="space-card"
          role="link"
          tabindex="0"
          @click="openSpace(space)"
          @keydown.enter.prevent="openSpace(space)"
        >
          <div class="card-header">
            <SpaceAvatar :name="space.name" :avatar="space.icon || ''" size="small" />
            <div class="card-title-block">
              <span class="card-title" :title="space.name">{{ space.name }}</span>
              <span class="card-slug">/{{ space.slug }}</span>
            </div>
            <t-tag v-if="canManageSpace(space.role)" size="small" variant="light" theme="primary" class="role-tag">
              {{ t("docs.spaces.role." + space.role) }}
            </t-tag>
            <t-tag v-else size="small" variant="light" theme="default" class="role-tag">
              {{ t("docs.spaces.role." + space.role) }}
            </t-tag>
          </div>
          <div class="card-description">{{ space.description || t("docs.spaces.noDescription") }}</div>
          <div class="card-bottom">
            <span class="badge" :class="'badge--' + space.visibility">
              <t-icon :name="space.visibility === 'private' ? 'lock-on' : 'usergroup'" size="12px" />
              {{ t("docs.spaces.visibility." + space.visibility) }}
            </span>
            <span class="badge">
              <t-icon name="user" size="12px" />
              {{ t("docs.spaces.memberCount", { count: space.member_count }) }}
            </span>
            <span class="badge">
              <t-icon name="file" size="12px" />
              {{ t("docs.spaces.pageCount", { count: space.page_count }) }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <t-dialog
      v-model:visible="createVisible"
      :header="t('docs.spaces.createTitle')"
      width="540px"
      :confirm-btn="{ content: t('common.create'), loading: creating }"
      :cancel-btn="t('common.cancel')"
      destroy-on-close
      @confirm="submitCreate"
    >
      <SpaceForm v-model="form" mode="create" :problems="problems" />
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { MessagePlugin } from "tdesign-vue-next";
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

import { createSpace, listSpaces, type DocsSpace } from "@/api/docs";
import SpaceAvatar from "@/components/SpaceAvatar.vue";
import { useAuthStore } from "@/stores/auth";

import {
  canManageSpace,
  normaliseSpaceForm,
  validateSpaceForm,
  type SpaceFormModel,
  type SpaceFormProblem,
} from "./docsAccess";
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

const openCreate = () => {
  form.value = emptyForm();
  problems.value = [];
  createVisible.value = true;
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
    });
    MessagePlugin.success(t("docs.spaces.createSuccess"));
    createVisible.value = false;
    spaces.value = [...spaces.value, created].sort((a, b) => a.name.localeCompare(b.name));
    router.push({ name: "docsSpace", params: { slug: created.slug } });
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.spaces.createFailed")));
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

<style scoped lang="less">
.docs-space-list {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  padding: 20px 28px 0 28px;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  flex-shrink: 0;

  .header-title {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .title-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  h2 {
    margin: 0;
    color: var(--td-text-color-primary);
    font-family: var(--app-font-family);
    font-size: 20px;
    font-weight: 600;
    line-height: 28px;
  }
}

.header-subtitle {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-family: var(--app-font-family);
  font-size: 14px;
  line-height: 20px;
}

.header-action-btn {
  padding: 0 !important;
  min-width: 28px !important;
  width: 28px !important;
  height: 28px !important;
}

.list-main {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-bottom: 24px;
}

.card-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
}

.space-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 132px;
  padding: 14px 16px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
  cursor: pointer;
  transition:
    border-color 0.2s ease,
    box-shadow 0.2s ease;

  &:hover,
  &:focus-visible {
    border-color: var(--td-brand-color);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.06);
    outline: none;
  }

  &--skeleton {
    cursor: default;
  }
}

.card-header {
  display: flex;
  align-items: center;
  gap: 10px;
}

.card-title-block {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.card-title {
  color: var(--td-text-color-primary);
  font-size: 15px;
  font-weight: 600;
  line-height: 22px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-slug {
  color: var(--td-text-color-placeholder);
  font-size: 12px;
  line-height: 16px;
  font-family: var(--td-font-family-mono, ui-monospace, monospace);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.role-tag {
  flex-shrink: 0;
}

.card-description {
  flex: 1;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  line-height: 20px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-bottom {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 22px;
  padding: 0 7px;
  border-radius: 5px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
  background: var(--td-bg-color-secondarycontainer);

  &--open {
    color: var(--td-brand-color);
    background: var(--td-brand-color-light);
  }
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 72px 16px;
  text-align: center;
}

.empty-icon {
  width: 48px;
  height: 48px;
  opacity: 0.6;
}

.empty-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--td-text-color-primary);
}

.empty-hint {
  max-width: 42ch;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  margin-bottom: 8px;
}
</style>
