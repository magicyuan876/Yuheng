<template>
  <div class="docs-space-page">
    <div class="page-header">
      <t-button variant="text" size="small" class="back-btn" @click="router.push({ name: 'docsSpaceList' })">
        <template #icon><t-icon name="chevron-left" /></template>
        {{ t('docs.spaces.backToList') }}
      </t-button>
      <div v-if="space" class="space-heading">
        <SpaceAvatar :name="space.name" :avatar="space.icon || ''" size="large" />
        <div class="space-heading-text">
          <h2>{{ space.name }}</h2>
          <div class="space-meta">
            <span class="mono">/{{ space.slug }}</span>
            <t-tag size="small" variant="light">{{ t('docs.spaces.visibility.' + space.visibility) }}</t-tag>
            <t-tag size="small" variant="light" theme="primary">
              {{ t('docs.spaces.overview.yourRole') }}: {{ t('docs.spaces.role.' + space.role) }}
            </t-tag>
          </div>
        </div>
      </div>
    </div>

    <div v-if="loading" class="loading-block">
      <t-skeleton animation="gradient" :row-col="[{ width: '40%' }, { width: '100%' }, { width: '80%' }]" />
    </div>
    <div v-else-if="!space" class="missing-block">{{ t('docs.spaces.loadFailed') }}</div>

    <t-tabs v-else v-model="tab" class="space-tabs">
      <!-- Overview -->
      <t-tab-panel value="overview" :label="t('docs.spaces.tabs.overview')">
        <dl class="overview-grid">
          <dt>{{ t('docs.spaces.form.description') }}</dt>
          <dd>{{ space.description || t('docs.spaces.noDescription') }}</dd>
          <dt>{{ t('docs.spaces.overview.slug') }}</dt>
          <dd class="mono">{{ space.slug }}</dd>
          <dt>{{ t('docs.spaces.overview.visibility') }}</dt>
          <dd>
            {{ t('docs.spaces.visibility.' + space.visibility) }}
            <span v-if="space.visibility === 'open'" class="dd-hint">
              {{ t('docs.spaces.overview.defaultRole') }}: {{ t('docs.spaces.role.' + space.default_role) }}
            </span>
          </dd>
          <dt>{{ t('docs.spaces.overview.knowledgeBase') }}</dt>
          <dd>
            <span v-if="space.knowledge_base_id" class="mono">{{ space.knowledge_base_id }}</span>
            <span v-else>{{ t('docs.spaces.overview.knowledgeBaseNone') }}</span>
            <span class="dd-hint">{{ t('docs.spaces.overview.knowledgeBaseHint') }}</span>
          </dd>
          <dt>{{ t('docs.spaces.overview.created') }}</dt>
          <dd>{{ formatDate(space.created_at) }}</dd>
          <dt>{{ t('docs.spaces.overview.updated') }}</dt>
          <dd>{{ formatDate(space.updated_at) }}</dd>
        </dl>
      </t-tab-panel>

      <!-- Members -->
      <t-tab-panel value="members" :label="t('docs.spaces.tabs.members')">
        <div class="members-toolbar">
          <p class="section-hint">{{ canManage ? t('docs.members.hint') : t('docs.members.readOnlyHint') }}</p>
          <t-button v-if="canManage" size="small" theme="primary" @click="openAddMember">
            <template #icon><t-icon name="user-add" /></template>
            {{ t('docs.members.add') }}
          </t-button>
        </div>
        <t-table :data="members" :columns="memberColumns" row-key="key" :loading="membersLoading" size="small"
          :empty="t('docs.members.empty')" hover>
          <template #member="{ row }">
            <div class="member-cell">
              <div class="member-avatar" :class="{ 'member-avatar--group': row.principal_type === 'group' }">
                <img v-if="row.principal_type === 'user' && row.avatar" :src="row.avatar" alt="" />
                <t-icon v-else :name="row.principal_type === 'group' ? 'usergroup' : 'user'" size="14px" />
              </div>
              <div class="member-text">
                <span class="member-name">
                  {{ row.name }}
                  <t-tag v-if="row.is_default_group" size="small" variant="outline" class="default-tag">
                    {{ t('docs.members.defaultGroup') }}
                  </t-tag>
                </span>
                <span v-if="row.principal_type === 'user'" class="member-sub">{{ row.email }}</span>
                <span v-else class="member-sub">
                  {{ t('docs.members.groupMembers', { count: row.group_member_count ?? 0 }) }}
                </span>
              </div>
            </div>
          </template>
          <template #type="{ row }">
            <t-tag size="small" variant="light">{{ t('docs.members.' + row.principal_type) }}</t-tag>
          </template>
          <template #role="{ row }">
            <t-select v-if="canManage" size="small" :value="row.role" :options="roleOptions" class="role-select"
              :disabled="savingKey === row.key || isLastAdmin(row)" @change="(v: unknown) => changeRole(row, v)" />
            <span v-else>{{ t('docs.spaces.role.' + row.role) }}</span>
          </template>
          <template #added="{ row }">{{ formatDate(row.created_at) }}</template>
          <template #actions="{ row }">
            <t-tooltip v-if="isLastAdmin(row)" :content="t('docs.members.lastAdmin')">
              <t-button variant="text" size="small" theme="danger" disabled>
                <template #icon><t-icon name="delete" /></template>
              </t-button>
            </t-tooltip>
            <t-popconfirm v-else :content="t('docs.members.removeConfirm', { name: row.name })" theme="danger"
              @confirm="removeMember(row)">
              <t-button variant="text" size="small" theme="danger" :loading="savingKey === row.key">
                <template #icon><t-icon name="delete" /></template>
              </t-button>
            </t-popconfirm>
          </template>
        </t-table>
      </t-tab-panel>

      <!-- Settings (space admins) -->
      <t-tab-panel v-if="canManage" value="settings" :label="t('docs.spaces.tabs.settings')">
        <div class="settings-panel">
          <SpaceForm v-model="form" mode="edit" :problems="problems" />
          <div class="settings-actions">
            <t-button theme="primary" :loading="saving" :disabled="!dirty" @click="saveSettings">
              {{ t('common.save') }}
            </t-button>
            <t-button variant="outline" :disabled="!dirty || saving" @click="resetForm">{{ t('common.cancel') }}</t-button>
          </div>

          <div class="danger-zone">
            <div class="danger-title">{{ t('docs.spaces.dangerZone') }}</div>
            <p class="danger-hint">{{ t('docs.spaces.dangerZoneHint') }}</p>
            <t-button theme="danger" variant="outline" @click="deleteVisible = true">
              {{ t('docs.spaces.deleteSpace') }}
            </t-button>
          </div>
        </div>
      </t-tab-panel>
    </t-tabs>

    <!-- Add member dialog -->
    <t-dialog v-model:visible="addVisible" :header="t('docs.members.addTitle')" width="520px" destroy-on-close
      :confirm-btn="{ content: t('common.add'), loading: adding, disabled: !addForm.principal_id }"
      :cancel-btn="t('common.cancel')" @confirm="submitAddMember">
      <t-form label-align="top" @submit.prevent>
        <t-form-item :label="t('docs.members.principalType')">
          <t-radio-group v-model="addForm.principal_type" variant="default-filled" @change="addForm.principal_id = ''">
            <t-radio-button value="user">{{ t('docs.members.user') }}</t-radio-button>
            <t-radio-button value="group">{{ t('docs.members.group') }}</t-radio-button>
          </t-radio-group>
        </t-form-item>
        <t-form-item v-if="addForm.principal_type === 'user'" :label="t('docs.members.pickUser')">
          <t-select v-model="addForm.principal_id" filterable :loading="memberSearch.loading.value"
            :options="memberSearch.options.value" :filter="() => true"
            :placeholder="t('docs.members.pickUserPlaceholder')" @search="memberSearch.search" />
        </t-form-item>
        <t-form-item v-else :label="t('docs.members.pickGroup')">
          <t-select v-model="addForm.principal_id" filterable :loading="groupsLoading" :options="groupOptions"
            :placeholder="t('docs.members.pickGroupPlaceholder')" />
        </t-form-item>
        <t-form-item :label="t('docs.members.role')" :tips="t('docs.members.roleHint')">
          <t-select v-model="addForm.role" :options="roleOptions" />
        </t-form-item>
      </t-form>
    </t-dialog>

    <!-- Delete dialog -->
    <t-dialog v-model:visible="deleteVisible" :header="t('docs.spaces.deleteConfirmTitle')" theme="danger"
      :confirm-btn="{ content: t('common.delete'), theme: 'danger', loading: deleting }" :cancel-btn="t('common.cancel')"
      @confirm="confirmDelete">
      <p>{{ t('docs.spaces.deleteConfirm', { name: space?.name ?? '' }) }}</p>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { MessagePlugin } from 'tdesign-vue-next'
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import {
  deleteSpace,
  getSpaceBySlug,
  listGroups,
  listSpaceMembers,
  removeSpaceMember,
  setSpaceMembers,
  updateSpace,
  type DocsSpace,
  type PrincipalType,
  type SpaceMember,
  type SpaceRole,
  type TenantGroup,
} from '@/api/docs'
import SpaceAvatar from '@/components/SpaceAvatar.vue'

import {
  canManageSpace,
  GRANTABLE_SPACE_ROLES,
  normaliseSpaceForm,
  sortSpaceMembers,
  validateSpaceForm,
  wouldLeaveNoAdmin,
  type SpaceFormModel,
  type SpaceFormProblem,
} from './docsAccess'
import SpaceForm from './SpaceForm.vue'
import { useMemberSearch } from './useMemberSearch'

type MemberRow = SpaceMember & { key: string }

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const space = ref<DocsSpace | null>(null)
const loading = ref(true)
const tab = ref<'overview' | 'members' | 'settings'>('overview')
const canManage = computed(() => canManageSpace(space.value?.role))

const slugParam = computed(() => String(route.params.slug ?? ''))

const errorText = (err: unknown, fallback: string): string => {
  const msg = (err as { message?: string } | null)?.message
  return msg ? `${fallback}: ${msg}` : fallback
}

const formatDate = (iso: string) => {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString()
}

const load = async () => {
  loading.value = true
  try {
    space.value = await getSpaceBySlug(slugParam.value)
    resetForm()
    await loadMembers()
  } catch (err: unknown) {
    space.value = null
    MessagePlugin.error(errorText(err, t('docs.spaces.loadFailed')))
  } finally {
    loading.value = false
  }
}

// ---- members ------------------------------------------------------------------
const members = ref<MemberRow[]>([])
const membersLoading = ref(false)
const savingKey = ref('')

const rowKey = (m: SpaceMember) => `${m.principal_type}:${m.principal_id}`
const setMembers = (rows: SpaceMember[]) => {
  members.value = sortSpaceMembers(rows).map((m) => ({ ...m, key: rowKey(m) }))
}

const loadMembers = async () => {
  if (!space.value) return
  membersLoading.value = true
  try {
    setMembers(await listSpaceMembers(space.value.id))
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.members.loadFailed')))
  } finally {
    membersLoading.value = false
  }
}

const roleOptions = computed(() => GRANTABLE_SPACE_ROLES.map((r) => ({ label: t('docs.spaces.role.' + r), value: r })))

const memberColumns = computed(() => {
  const cols = [
    { colKey: 'member', title: t('docs.members.columns.member'), ellipsis: true, minWidth: 220 },
    { colKey: 'type', title: t('docs.members.columns.type'), width: 96 },
    { colKey: 'role', title: t('docs.members.columns.role'), width: 150 },
    { colKey: 'added', title: t('docs.members.columns.added'), width: 180 },
  ]
  if (canManage.value) cols.push({ colKey: 'actions', title: t('docs.members.columns.actions'), width: 72 })
  return cols
})

const isLastAdmin = (row: MemberRow) =>
  row.role === 'admin' && wouldLeaveNoAdmin(members.value, { ...row, role: null })

const changeRole = async (row: MemberRow, value: unknown) => {
  if (!space.value) return
  const role = value as SpaceRole
  if (role === row.role) return
  if (wouldLeaveNoAdmin(members.value, { ...row, role })) {
    MessagePlugin.warning(t('docs.members.lastAdmin'))
    return
  }
  savingKey.value = row.key
  try {
    setMembers(await setSpaceMembers(space.value.id, [
      { principal_type: row.principal_type, principal_id: row.principal_id, role },
    ]))
    MessagePlugin.success(t('docs.members.saveSuccess'))
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.members.saveFailed')))
  } finally {
    savingKey.value = ''
  }
}

const removeMember = async (row: MemberRow) => {
  if (!space.value) return
  savingKey.value = row.key
  try {
    await removeSpaceMember(space.value.id, row.principal_type, row.principal_id)
    members.value = members.value.filter((m) => m.key !== row.key)
    MessagePlugin.success(t('docs.members.removeSuccess'))
    // Removing yourself may have cost you admin rights; reload the space.
    await load()
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.members.removeFailed')))
  } finally {
    savingKey.value = ''
  }
}

// ---- add member ----------------------------------------------------------------
const addVisible = ref(false)
const adding = ref(false)
const addForm = ref<{ principal_type: PrincipalType; principal_id: string; role: SpaceRole }>({
  principal_type: 'user', principal_id: '', role: 'reader',
})
const memberSearch = useMemberSearch()
const groups = ref<TenantGroup[]>([])
const groupsLoading = ref(false)
const groupOptions = computed(() => {
  const present = new Set(members.value.filter((m) => m.principal_type === 'group').map((m) => m.principal_id))
  return groups.value.map((g) => ({
    label: g.is_default ? `${g.name} (${t('docs.members.defaultGroup')})` : g.name,
    value: g.id,
    disabled: present.has(g.id),
  }))
})

const openAddMember = async () => {
  addForm.value = { principal_type: 'user', principal_id: '', role: 'reader' }
  addVisible.value = true
  void memberSearch.load('')
  groupsLoading.value = true
  try {
    groups.value = await listGroups()
  } catch {
    groups.value = []
  } finally {
    groupsLoading.value = false
  }
}

const submitAddMember = async () => {
  if (!space.value || !addForm.value.principal_id) return
  adding.value = true
  try {
    setMembers(await setSpaceMembers(space.value.id, [{ ...addForm.value }]))
    MessagePlugin.success(t('docs.members.saveSuccess'))
    addVisible.value = false
    space.value.member_count = members.value.length
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.members.saveFailed')))
  } finally {
    adding.value = false
  }
}

// ---- settings -------------------------------------------------------------------
const form = ref<SpaceFormModel>({ name: '', slug: '', description: '', visibility: 'private', default_role: 'none' })
const problems = ref<SpaceFormProblem[]>([])
const saving = ref(false)

const resetForm = () => {
  if (!space.value) return
  form.value = {
    name: space.value.name,
    slug: space.value.slug,
    description: space.value.description,
    visibility: space.value.visibility,
    default_role: space.value.default_role,
  }
  problems.value = []
}

const dirty = computed(() => {
  const s = space.value
  if (!s) return false
  const f = form.value
  return f.name.trim() !== s.name || f.slug.trim() !== s.slug || f.description.trim() !== s.description
    || f.visibility !== s.visibility || (f.visibility === 'open' && f.default_role !== s.default_role)
})

const saveSettings = async () => {
  if (!space.value) return
  const model = normaliseSpaceForm({ ...form.value, name: form.value.name.trim(), slug: form.value.slug.trim() })
  problems.value = validateSpaceForm(model, { requireSlug: true })
  if (problems.value.length) return
  saving.value = true
  try {
    const slugChanged = model.slug !== space.value.slug
    space.value = await updateSpace(space.value.id, {
      name: model.name,
      slug: model.slug,
      description: model.description.trim(),
      visibility: model.visibility,
      default_role: model.default_role,
    })
    resetForm()
    MessagePlugin.success(t('docs.spaces.updateSuccess'))
    if (slugChanged) {
      router.replace({ name: 'docsSpace', params: { slug: space.value.slug } })
    }
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.spaces.updateFailed')))
  } finally {
    saving.value = false
  }
}

// ---- delete ---------------------------------------------------------------------
const deleteVisible = ref(false)
const deleting = ref(false)
const confirmDelete = async () => {
  if (!space.value) return
  deleting.value = true
  try {
    await deleteSpace(space.value.id)
    MessagePlugin.success(t('docs.spaces.deleteSuccess'))
    deleteVisible.value = false
    router.push({ name: 'docsSpaceList' })
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.spaces.deleteFailed')))
  } finally {
    deleting.value = false
  }
}

watch(slugParam, (next, prev) => {
  if (next && next !== prev) void load()
})
onMounted(load)
</script>

<style scoped lang="less">
.docs-space-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  padding: 16px 28px 24px 28px;
  overflow-y: auto;
}

.page-header {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 12px;
}

.back-btn {
  align-self: flex-start;
  padding-left: 0 !important;
  color: var(--td-text-color-secondary);
}

.space-heading {
  display: flex;
  align-items: center;
  gap: 14px;

  h2 {
    margin: 0;
    font-size: 22px;
    line-height: 30px;
    font-weight: 600;
    color: var(--td-text-color-primary);
  }
}

.space-heading-text {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.space-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  color: var(--td-text-color-secondary);
  font-size: 13px;
}

.mono {
  font-family: var(--td-font-family-mono, ui-monospace, monospace);
}

.loading-block,
.missing-block {
  padding: 24px 0;
  color: var(--td-text-color-secondary);
}

.space-tabs {
  :deep(.t-tabs__content) {
    padding-top: 16px;
  }
}

.overview-grid {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: 12px 24px;
  max-width: 760px;
  margin: 0;

  dt {
    color: var(--td-text-color-secondary);
    font-size: 13px;
    line-height: 22px;
  }

  dd {
    margin: 0;
    color: var(--td-text-color-primary);
    font-size: 14px;
    line-height: 22px;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
}

.dd-hint {
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}

.members-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.section-hint {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
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

  &--group {
    border-radius: 8px;
    color: var(--td-brand-color);
    background: var(--td-brand-color-light);
  }
}

.member-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.member-name {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--td-text-color-primary);
  font-size: 14px;
  line-height: 20px;
}

.member-sub {
  color: var(--td-text-color-placeholder);
  font-size: 12px;
  line-height: 16px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.role-select {
  width: 128px;
}

.settings-panel {
  max-width: 640px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.settings-actions {
  display: flex;
  gap: 8px;
}

.danger-zone {
  margin-top: 24px;
  padding: 16px;
  border: 1px solid var(--td-error-color-3);
  border-radius: 8px;
  background: var(--td-error-color-1);
}

.danger-title {
  font-weight: 600;
  color: var(--td-error-color);
  margin-bottom: 4px;
}

.danger-hint {
  margin: 0 0 12px;
  color: var(--td-text-color-secondary);
  font-size: 13px;
}
</style>
