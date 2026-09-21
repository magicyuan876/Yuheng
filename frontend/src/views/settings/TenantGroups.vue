<template>
  <div class="tenant-groups">
    <div class="section-header">
      <div class="section-header-row">
        <h2>{{ t("docs.groups.title") }}</h2>
        <t-button v-if="canManage" size="small" theme="primary" @click="openCreate">
          <template #icon><t-icon name="add" /></template>
          {{ t("docs.groups.create") }}
        </t-button>
      </div>
      <p class="section-description">{{ t("docs.groups.subtitle") }}</p>
    </div>

    <t-table
      :data="groups"
      :columns="columns"
      row-key="id"
      :loading="loading"
      size="small"
      hover
      :empty="t('docs.groups.empty')"
    >
      <template #name="{ row }">
        <div class="group-name">
          <t-icon name="usergroup" size="16px" class="group-icon" />
          <span>{{ row.name }}</span>
          <t-tag v-if="row.is_default" size="small" variant="outline">{{ t("docs.groups.defaultBadge") }}</t-tag>
        </div>
      </template>
      <template #description="{ row }">
        <span class="group-desc">{{ row.is_default ? t("docs.groups.defaultHint") : row.description || "—" }}</span>
      </template>
      <template #members="{ row }">
        <t-link theme="primary" hover="color" @click="openMembers(row)">
          {{ t("docs.groups.memberCount", { count: row.member_count }) }}
        </t-link>
      </template>
      <template #actions="{ row }">
        <div class="row-actions">
          <t-tooltip :content="t('docs.groups.manageMembers')">
            <t-button variant="text" size="small" @click="openMembers(row)">
              <template #icon><t-icon name="user-list" /></template>
            </t-button>
          </t-tooltip>
          <t-tooltip v-if="canManage" :content="t('common.edit')">
            <t-button variant="text" size="small" @click="openEdit(row)">
              <template #icon><t-icon name="edit" /></template>
            </t-button>
          </t-tooltip>
          <t-popconfirm
            v-if="canManage && !row.is_default"
            theme="danger"
            :content="t('docs.groups.deleteConfirm', { name: row.name })"
            @confirm="remove(row)"
          >
            <t-button variant="text" size="small" theme="danger">
              <template #icon><t-icon name="delete" /></template>
            </t-button>
          </t-popconfirm>
        </div>
      </template>
    </t-table>

    <!-- Create / edit dialog -->
    <t-dialog
      v-model:visible="editVisible"
      :header="editing ? t('docs.groups.editTitle') : t('docs.groups.createTitle')"
      width="480px"
      destroy-on-close
      :cancel-btn="t('common.cancel')"
      :confirm-btn="{ content: t('common.save'), loading: saving, disabled: !form.name.trim() }"
      @confirm="submitEdit"
    >
      <t-form label-align="top" @submit.prevent>
        <t-form-item :label="t('docs.groups.name')">
          <t-input
            v-model="form.name"
            :maxlength="100"
            :disabled="editing?.is_default"
            :placeholder="t('docs.groups.namePlaceholder')"
          />
        </t-form-item>
        <t-form-item :label="t('docs.groups.description')">
          <t-textarea
            v-model="form.description"
            :maxlength="4000"
            :autosize="{ minRows: 2, maxRows: 5 }"
            :placeholder="t('docs.groups.descriptionPlaceholder')"
          />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- Members dialog -->
    <t-dialog v-model:visible="membersVisible" :header="membersTitle" width="640px" :footer="false" destroy-on-close>
      <div class="members-dialog">
        <div class="members-toolbar">
          <t-input
            v-model="memberQuery"
            clearable
            :placeholder="t('docs.groups.searchPlaceholder')"
            class="member-search"
            @change="() => reloadMembers(1)"
            @clear="() => reloadMembers(1)"
            @enter="() => reloadMembers(1)"
          >
            <template #prefix-icon><t-icon name="search" /></template>
          </t-input>
          <div v-if="canManage && activeGroup && !activeGroup.is_default" class="member-add">
            <t-select
              v-model="pendingUserIds"
              multiple
              filterable
              :filter="() => true"
              :loading="memberSearch.loading.value"
              :options="addableOptions"
              :placeholder="t('docs.groups.addMembersPlaceholder')"
              :min-collapsed-num="2"
              class="member-add-select"
              @search="memberSearch.search"
            />
            <t-button
              theme="primary"
              size="small"
              :disabled="!pendingUserIds.length"
              :loading="addingMembers"
              @click="addMembers"
            >
              {{ t("docs.groups.addMembers") }}
            </t-button>
          </div>
        </div>
        <p v-if="activeGroup?.is_default" class="section-description">{{ t("docs.groups.defaultHint") }}</p>
        <t-table
          :data="memberPage.members"
          :columns="memberColumns"
          row-key="user_id"
          size="small"
          :loading="membersLoading"
          :empty="t('docs.groups.noMembers')"
          hover
        >
          <template #user="{ row }">
            <div class="member-cell">
              <div class="member-avatar">
                <img v-if="row.avatar" :src="row.avatar" alt="" />
                <t-icon v-else name="user" size="14px" />
              </div>
              <div class="member-text">
                <span class="member-name">{{ row.username || row.user_id }}</span>
                <span class="member-sub">{{ row.email }}</span>
              </div>
            </div>
          </template>
          <template #actions="{ row }">
            <t-popconfirm theme="danger" :content="t('docs.groups.removeMemberConfirm')" @confirm="removeMember(row)">
              <t-button variant="text" size="small" theme="danger" :loading="removingUserId === row.user_id">
                <template #icon><t-icon name="delete" /></template>
              </t-button>
            </t-popconfirm>
          </template>
        </t-table>
        <t-pagination
          v-if="memberPage.total > memberPage.page_size"
          :total="memberPage.total"
          :current="memberPage.page"
          :page-size="memberPage.page_size"
          size="small"
          :show-page-size="false"
          class="members-pagination"
          @current-change="(p: number) => reloadMembers(p)"
        />
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { MessagePlugin } from "tdesign-vue-next";
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

import {
  addGroupMembers,
  createGroup,
  deleteGroup,
  listGroupMembers,
  listGroups,
  removeGroupMember,
  updateGroup,
  type GroupMemberPage,
  type GroupMemberRow,
  type TenantGroup,
} from "@/api/docs";
import { useAuthStore } from "@/stores/auth";
import { useMemberSearch } from "@/views/docs/useMemberSearch";

const { t } = useI18n();
const authStore = useAuthStore();
const canManage = computed(() => authStore.hasRole("admin"));

const errorText = (err: unknown, fallback: string): string => {
  const msg = (err as { message?: string } | null)?.message;
  return msg ? `${fallback}: ${msg}` : fallback;
};

// ---- list ---------------------------------------------------------------------------
const groups = ref<TenantGroup[]>([]);
const loading = ref(false);

const load = async () => {
  loading.value = true;
  try {
    groups.value = await listGroups();
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.groups.loadFailed")));
  } finally {
    loading.value = false;
  }
};

const columns = computed(() => [
  { colKey: "name", title: t("docs.groups.columns.name"), minWidth: 180, ellipsis: true },
  { colKey: "description", title: t("docs.groups.columns.description"), ellipsis: true },
  { colKey: "members", title: t("docs.groups.columns.members"), width: 120 },
  { colKey: "actions", title: t("docs.groups.columns.actions"), width: 132 },
]);

// ---- create / edit ------------------------------------------------------------------
const editVisible = ref(false);
const editing = ref<TenantGroup | null>(null);
const saving = ref(false);
const form = ref({ name: "", description: "" });

const openCreate = () => {
  editing.value = null;
  form.value = { name: "", description: "" };
  editVisible.value = true;
};

const openEdit = (g: TenantGroup) => {
  editing.value = g;
  form.value = { name: g.name, description: g.description };
  editVisible.value = true;
};

const submitEdit = async () => {
  saving.value = true;
  try {
    if (editing.value) {
      const body: { name?: string; description?: string } = { description: form.value.description.trim() };
      if (!editing.value.is_default) body.name = form.value.name.trim();
      const updated = await updateGroup(editing.value.id, body);
      groups.value = groups.value.map((g) => (g.id === updated.id ? updated : g));
      MessagePlugin.success(t("docs.groups.updateSuccess"));
    } else {
      const created = await createGroup({ name: form.value.name.trim(), description: form.value.description.trim() });
      groups.value = [...groups.value, created];
      MessagePlugin.success(t("docs.groups.createSuccess"));
    }
    editVisible.value = false;
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, editing.value ? t("docs.groups.updateFailed") : t("docs.groups.createFailed")));
  } finally {
    saving.value = false;
  }
};

const remove = async (g: TenantGroup) => {
  try {
    await deleteGroup(g.id);
    groups.value = groups.value.filter((x) => x.id !== g.id);
    MessagePlugin.success(t("docs.groups.deleteSuccess"));
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.groups.deleteFailed")));
  }
};

// ---- members -------------------------------------------------------------------------
const membersVisible = ref(false);
const activeGroup = ref<TenantGroup | null>(null);
const memberQuery = ref("");
const membersLoading = ref(false);
const memberPage = ref<GroupMemberPage>({ members: [], total: 0, page: 1, page_size: 20 });
const removingUserId = ref("");
const pendingUserIds = ref<string[]>([]);
const addingMembers = ref(false);
const memberSearch = useMemberSearch();

const membersTitle = computed(() =>
  activeGroup.value ? `${t("docs.groups.membersTitle")} · ${activeGroup.value.name}` : t("docs.groups.membersTitle"),
);

const memberColumns = computed(() => {
  const cols = [{ colKey: "user", title: t("docs.members.columns.member"), ellipsis: true }];
  if (canManage.value && activeGroup.value && !activeGroup.value.is_default) {
    cols.push({ colKey: "actions", title: t("docs.groups.columns.actions"), ellipsis: false, width: 72 } as never);
  }
  return cols;
});

const addableOptions = computed(() => {
  const present = new Set(memberPage.value.members.map((m) => m.user_id));
  return memberSearch.options.value.map((o) => ({ ...o, disabled: present.has(o.value) }));
});

const openMembers = async (g: TenantGroup) => {
  activeGroup.value = g;
  memberQuery.value = "";
  pendingUserIds.value = [];
  membersVisible.value = true;
  void memberSearch.load("");
  await reloadMembers(1);
};

const reloadMembers = async (page: number) => {
  if (!activeGroup.value) return;
  membersLoading.value = true;
  try {
    memberPage.value = await listGroupMembers(activeGroup.value.id, {
      q: memberQuery.value,
      page,
      page_size: memberPage.value.page_size,
    });
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.groups.membersLoadFailed")));
  } finally {
    membersLoading.value = false;
  }
};

const refreshCount = (updated: TenantGroup) => {
  groups.value = groups.value.map((g) => (g.id === updated.id ? updated : g));
  activeGroup.value = updated;
};

const addMembers = async () => {
  if (!activeGroup.value || !pendingUserIds.value.length) return;
  addingMembers.value = true;
  try {
    refreshCount(await addGroupMembers(activeGroup.value.id, pendingUserIds.value));
    pendingUserIds.value = [];
    MessagePlugin.success(t("docs.groups.addSuccess"));
    await reloadMembers(memberPage.value.page);
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.groups.addFailed")));
  } finally {
    addingMembers.value = false;
  }
};

const removeMember = async (row: GroupMemberRow) => {
  if (!activeGroup.value) return;
  removingUserId.value = row.user_id;
  try {
    await removeGroupMember(activeGroup.value.id, row.user_id);
    MessagePlugin.success(t("docs.groups.removeSuccess"));
    refreshCount({ ...activeGroup.value, member_count: Math.max(0, activeGroup.value.member_count - 1) });
    await reloadMembers(memberPage.value.page);
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t("docs.groups.removeFailed")));
  } finally {
    removingUserId.value = "";
  }
};

onMounted(load);
</script>

<style scoped lang="less">
.tenant-groups {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.section-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;

  h2 {
    margin: 0;
    font-size: 18px;
    line-height: 26px;
    font-weight: 600;
    color: var(--td-text-color-primary);
  }
}

.section-description {
  margin: 6px 0 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  line-height: 20px;
}

.group-name {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--td-text-color-primary);
}

.group-icon {
  color: var(--td-brand-color);
}

.group-desc {
  color: var(--td-text-color-secondary);
}

.row-actions {
  display: flex;
  gap: 2px;
}

.members-dialog {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.members-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.member-search {
  width: 220px;
}

.member-add {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 260px;
  justify-content: flex-end;
}

.member-add-select {
  flex: 1;
  max-width: 320px;
}

.member-cell {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.member-avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-secondary);
  overflow: hidden;
  flex-shrink: 0;

  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
}

.member-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.member-name {
  color: var(--td-text-color-primary);
  font-size: 14px;
  line-height: 20px;
}

.member-sub {
  color: var(--td-text-color-placeholder);
  font-size: 12px;
  line-height: 16px;
}

.members-pagination {
  align-self: flex-end;
}
</style>
