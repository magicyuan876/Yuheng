<template>
  <div class="access-panel">
    <t-loading :loading="loading" size="small">
      <!-- What is true right now, before any control to change it. Somebody
           opens this panel to find out why, not only to act. -->
      <div class="state-row">
        <t-icon :name="view?.restricted ? 'lock-on' : 'usergroup'" size="18px" />
        <div class="state-text">
          <div class="state-title">
            {{ view?.restricted ? t("docs.access.restrictedTitle") : t("docs.access.inheritedTitle") }}
          </div>
          <p class="state-hint">
            {{
              view?.restricted
                ? t("docs.access.restrictedHint")
                : t("docs.access.inheritedHint", { role: roleName(view?.space_default) })
            }}
          </p>
        </div>
        <t-switch v-if="view?.can_manage" :value="view.restricted" :loading="switching" @change="onToggleRestricted" />
      </div>

      <!-- A page narrowed from above is not broken, and saying nothing about
           it makes the panel look like it is. -->
      <t-alert v-if="view?.inherited_from.length" theme="info" class="above-alert">
        <template #message>
          <span v-if="nearestAncestor?.visible">
            {{ t("docs.access.narrowedByVisible", { title: nearestAncestor.title || t("docs.tree.untitled") }) }}
          </span>
          <span v-else>{{ t("docs.access.narrowedByHidden") }}</span>
        </template>
      </t-alert>

      <template v-if="view?.restricted">
        <div class="list-head">
          <span>{{ t("docs.access.whoHasAccess") }}</span>
          <span class="list-count">{{ view.grants.length }}</span>
        </div>

        <ul class="grant-list">
          <li v-for="g in view.grants" :key="`${g.principal_type}:${g.principal_id}`" class="grant-row">
            <t-icon :name="g.principal_type === 'group' ? 'usergroup' : 'user'" size="16px" class="grant-icon" />
            <div class="grant-who">
              <span class="grant-name">{{ g.name }}</span>
              <span v-if="g.principal_type === 'group' && g.group_member_count !== undefined" class="grant-sub">
                {{ t("docs.access.groupMembers", { n: g.group_member_count }) }}
              </span>
              <span v-else-if="g.email" class="grant-sub">{{ g.email }}</span>
            </div>

            <!-- A grant is a ceiling, so when it exceeds the space role the
                 honest thing is to show what it actually does. -->
            <t-tooltip v-if="!g.in_space" :content="t('docs.access.notInSpaceHint')">
              <t-tag size="small" theme="warning" variant="light">{{ t("docs.access.notInSpace") }}</t-tag>
            </t-tooltip>
            <t-tooltip
              v-else-if="g.effective !== g.role"
              :content="t('docs.access.cappedHint', { granted: roleName(g.role), effective: roleName(g.effective) })"
            >
              <t-tag size="small" theme="warning" variant="light">{{ roleName(g.effective) }}</t-tag>
            </t-tooltip>

            <t-select
              v-if="view.can_manage"
              :value="g.role"
              :options="roleOptions"
              size="small"
              class="grant-role"
              :disabled="busy === principalKey(g)"
              @change="(r: string) => changeRole(g, r)"
            />
            <span v-else class="grant-role-static">{{ roleName(g.role) }}</span>

            <t-button
              v-if="view.can_manage"
              variant="text"
              size="small"
              shape="square"
              theme="danger"
              :loading="busy === principalKey(g)"
              :aria-label="t('docs.access.remove', { name: g.name })"
              @click="remove(g)"
            >
              <template #icon><t-icon name="close" /></template>
            </t-button>
          </li>
          <li v-if="!view.grants.length" class="grant-empty">{{ t("docs.access.nobodyYet") }}</li>
        </ul>

        <div v-if="view.can_manage" class="add-row">
          <t-select v-model="addType" :options="typeOptions" size="small" class="add-type" />
          <t-select
            v-if="addType === 'user'"
            v-model="addUser"
            filterable
            size="small"
            class="add-who"
            :options="memberOptions"
            :loading="memberLoading"
            :placeholder="t('docs.access.pickPerson')"
            @search="memberSearch"
            @focus="() => memberSearch('')"
          />
          <t-select
            v-else
            v-model="addGroup"
            filterable
            size="small"
            class="add-who"
            :options="groupOptions"
            :placeholder="t('docs.access.pickGroup')"
          />
          <t-select v-model="addRole" :options="roleOptions" size="small" class="add-role" />
          <t-button size="small" theme="primary" :loading="adding" :disabled="!canAdd" @click="add">
            {{ t("docs.access.grant") }}
          </t-button>
        </div>
      </template>
    </t-loading>

    <t-dialog
      v-model:visible="confirmInherit"
      :header="t('docs.access.restoreHeader')"
      theme="warning"
      width="440px"
      :confirm-btn="{ content: t('docs.access.restoreConfirm'), theme: 'warning' }"
      :cancel-btn="t('common.cancel')"
      @confirm="restoreInheritance"
    >
      <p>{{ t("docs.access.restoreBody", { n: view?.grants.length ?? 0 }) }}</p>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";

import {
  addPageGrant,
  getPageAccess,
  listGroups,
  removePageGrant,
  setPageRestricted,
  type GrantView,
  type PageAccessView,
  type SpaceRole,
} from "@/api/docs";

import { GRANTABLE_SPACE_ROLES } from "../docsAccess";
import { useMemberSearch } from "../useMemberSearch";

// Who can see this page, and why.
//
// The panel exists to explain as much as to change: a restricted page that
// cannot say who narrowed it, or a grant that silently does nothing because
// the person is not in the space, turns a permission question into a support
// ticket. So every row carries what it actually does, not only what it says.

const props = defineProps<{ pageId: string }>();
const emit = defineEmits<{ changed: [PageAccessView] }>();

const { t } = useI18n();

const view = shallowRef<PageAccessView | null>(null);
const loading = ref(false);
const switching = ref(false);
const adding = ref(false);
const busy = ref("");
const confirmInherit = ref(false);

const addType = ref<"user" | "group">("user");
const addUser = ref("");
const addGroup = ref("");
const addRole = ref<SpaceRole>("reader");
const groupOptions = shallowRef<{ label: string; value: string }[]>([]);

const { options: memberOptions, loading: memberLoading, search: memberSearch } = useMemberSearch();

// The same vocabulary the space member list uses; two sets of words for one
// set of roles would be a translation bug waiting to happen.
const roleOptions = computed(() => GRANTABLE_SPACE_ROLES.map((r) => ({ label: t(`docs.spaces.role.${r}`), value: r })));

const typeOptions = computed(() => [
  { label: t("docs.access.person"), value: "user" },
  { label: t("docs.access.group"), value: "group" },
]);

/** Nearest restricted ancestor: the list is ordered root-first, so the one
 * that actually governs this page is the last. */
const nearestAncestor = computed(() => {
  const rows = view.value?.inherited_from ?? [];
  return rows.length ? rows[rows.length - 1] : null;
});

const canAdd = computed(() => (addType.value === "user" ? !!addUser.value : !!addGroup.value));

const principalKey = (g: GrantView) => `${g.principal_type}:${g.principal_id}`;

function roleName(role?: SpaceRole): string {
  return t(`docs.spaces.role.${role || "none"}`);
}

function apply(next: PageAccessView) {
  view.value = next;
  emit("changed", next);
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
    const next = await getPageAccess(id);
    if (props.pageId === id) view.value = next;
  } catch (err) {
    fail(err, t("docs.access.loadFailed"));
  } finally {
    if (props.pageId === id) loading.value = false;
  }
}

async function loadGroups() {
  try {
    const rows = await listGroups();
    groupOptions.value = rows.map((g) => ({ label: g.name, value: g.id }));
  } catch {
    groupOptions.value = [];
  }
}

// Restricting is one click; going back is not. Restoring inheritance drops
// every grant, and somebody who has spent ten minutes building a list should
// be told that before it goes.
function onToggleRestricted(next: boolean) {
  if (!next) {
    confirmInherit.value = true;
    return;
  }
  void restrict(true);
}

async function restrict(next: boolean) {
  switching.value = true;
  try {
    apply(await setPageRestricted(props.pageId, next));
  } catch (err) {
    fail(err, t("docs.access.changeFailed"));
  } finally {
    switching.value = false;
  }
}

async function restoreInheritance() {
  confirmInherit.value = false;
  await restrict(false);
}

async function add() {
  if (!canAdd.value) return;
  adding.value = true;
  try {
    apply(
      await addPageGrant(props.pageId, {
        principal_type: addType.value,
        principal_id: addType.value === "user" ? addUser.value : addGroup.value,
        role: addRole.value,
      }),
    );
    addUser.value = "";
    addGroup.value = "";
  } catch (err) {
    fail(err, t("docs.access.grantFailed"));
  } finally {
    adding.value = false;
  }
}

async function changeRole(g: GrantView, role: string) {
  if (role === g.role) return;
  busy.value = principalKey(g);
  try {
    apply(
      await addPageGrant(props.pageId, {
        principal_type: g.principal_type,
        principal_id: g.principal_id,
        role: role as SpaceRole,
      }),
    );
  } catch (err) {
    fail(err, t("docs.access.grantFailed"));
  } finally {
    busy.value = "";
  }
}

async function remove(g: GrantView) {
  busy.value = principalKey(g);
  try {
    apply(await removePageGrant(props.pageId, g.principal_type, g.principal_id));
  } catch (err) {
    fail(err, t("docs.access.changeFailed"));
  } finally {
    busy.value = "";
  }
}

watch(
  () => props.pageId,
  () => {
    void load();
    void loadGroups();
  },
  { immediate: true },
);
</script>

<style scoped>
.access-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 120px;
}

.state-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.state-text {
  flex: 1;
  min-width: 0;
}

.state-title {
  font-size: 14px;
  font-weight: 600;
}

.state-hint {
  margin: 2px 0 0;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}

.above-alert {
  margin: 0;
}

.list-head {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--td-text-color-secondary);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.03em;
  text-transform: uppercase;
}

.list-count {
  color: var(--td-text-color-placeholder);
  font-variant-numeric: tabular-nums;
}

.grant-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.grant-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
}

.grant-icon {
  flex: none;
  color: var(--td-text-color-placeholder);
}

.grant-who {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.grant-name {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  font-size: 14px;
}

.grant-sub {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}

.grant-role {
  flex: none;
  width: 104px;
}

.grant-role-static {
  flex: none;
  color: var(--td-text-color-secondary);
  font-size: 13px;
}

.grant-empty {
  padding: 8px 0;
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}

.add-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding-top: 4px;
  border-top: 1px solid var(--td-component-stroke);
}

.add-type {
  width: 96px;
}

.add-who {
  flex: 1;
  min-width: 160px;
}

.add-role {
  width: 104px;
}
</style>
