<template>
  <div class="docs-trash">
    <p class="trash-hint">{{ t('docs.trash.subtitle') }}</p>
    <div class="trash-toolbar">
      <t-button variant="text" size="small" :loading="loading" @click="load">
        <template #icon><t-icon name="refresh" /></template>
        {{ t('common.refresh') }}
      </t-button>
      <t-popconfirm v-if="isAdmin && entries.length" :content="t('docs.trash.emptyConfirm')" theme="danger"
        @confirm="emptyAll">
        <t-button variant="outline" theme="danger" size="small" :loading="emptying">
          {{ t('docs.trash.emptyAll') }}
        </t-button>
      </t-popconfirm>
    </div>

    <div v-if="loading && entries.length === 0" class="trash-loading">
      <t-skeleton animation="gradient" :row-col="[{ width: '100%' }, { width: '80%' }, { width: '90%' }]" />
    </div>
    <div v-else-if="entries.length === 0" class="trash-empty">{{ t('docs.trash.empty') }}</div>
    <ul v-else class="trash-list">
      <li v-for="e in entries" :key="e.id" class="trash-item">
        <span class="trash-icon">{{ e.icon || '📄' }}</span>
        <div class="trash-text">
          <div class="trash-title">{{ e.title || t('docs.tree.untitled') }}</div>
          <div class="trash-sub">
            {{ t('docs.trash.deletedBy', { name: e.deleted_by_user?.username || e.deleted_by || '—',
              time: formatDate(e.deleted_at) }) }}
          </div>
        </div>
        <span class="trash-actions">
          <t-button v-if="e.can_restore" size="small" variant="outline" :loading="busy === e.id"
            @click="restore(e)">{{ t('docs.trash.restore') }}</t-button>
          <t-popconfirm v-if="isAdmin" :content="t('docs.trash.purgeConfirm', { title: e.title || t('docs.tree.untitled') })"
            theme="danger" @confirm="purge(e)">
            <t-button size="small" variant="text" theme="danger" :loading="busy === e.id">
              {{ t('docs.trash.purge') }}
            </t-button>
          </t-popconfirm>
        </span>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { MessagePlugin } from 'tdesign-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  emptyTrash,
  listTrash,
  purgeTrashPage,
  restorePage,
  type DocsSpace,
  type PageView,
  type TrashEntry,
} from '@/api/docs'

import { canManageSpace } from './docsAccess'

const props = defineProps<{ space: DocsSpace }>()
const emit = defineEmits<{ restored: [page: PageView] }>()
const { t } = useI18n()

const entries = ref<TrashEntry[]>([])
const loading = ref(false)
const emptying = ref(false)
const busy = ref<string | null>(null)
const isAdmin = computed(() => canManageSpace(props.space.role))

const formatDate = (iso: string | null | undefined) => {
  if (!iso) return ''
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString()
}

const errorText = (err: unknown, fallback: string) => {
  const msg = (err as { message?: string } | null)?.message
  return msg ? `${fallback}: ${msg}` : fallback
}

async function load() {
  loading.value = true
  try {
    entries.value = await listTrash(props.space.id)
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.trash.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function restore(e: TrashEntry) {
  busy.value = e.id
  try {
    const page = await restorePage(e.id)
    entries.value = entries.value.filter((x) => x.id !== e.id)
    MessagePlugin.success(t('docs.trash.restoreSuccess'))
    emit('restored', page)
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.trash.restoreFailed')))
  } finally {
    busy.value = null
  }
}

async function purge(e: TrashEntry) {
  busy.value = e.id
  try {
    await purgeTrashPage(props.space.id, e.id)
    entries.value = entries.value.filter((x) => x.id !== e.id)
    MessagePlugin.success(t('docs.trash.purgeSuccess'))
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.trash.purgeFailed')))
  } finally {
    busy.value = null
  }
}

async function emptyAll() {
  emptying.value = true
  try {
    await emptyTrash(props.space.id)
    entries.value = []
    MessagePlugin.success(t('docs.trash.emptySuccess'))
  } catch (err: unknown) {
    MessagePlugin.error(errorText(err, t('docs.trash.emptyFailed')))
  } finally {
    emptying.value = false
  }
}

onMounted(load)
</script>

<style scoped lang="less">
.docs-trash {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.trash-hint {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  line-height: 20px;
}

.trash-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.trash-empty,
.trash-loading {
  padding: 32px 8px;
  text-align: center;
  color: var(--td-text-color-placeholder);
}

.trash-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.trash-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container);
}

.trash-icon {
  flex: none;
  width: 22px;
  text-align: center;
}

.trash-text {
  flex: 1;
  min-width: 0;
}

.trash-title {
  font-size: 14px;
  color: var(--td-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.trash-sub {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}

.trash-actions {
  flex: none;
  display: inline-flex;
  gap: 4px;
}
</style>
