<template>
  <section class="storage-usage">
    <t-loading :loading="loading" size="small">
      <div class="usage-head">
        <span class="usage-figures">
          <strong>{{ formatBytes(usage?.used_bytes ?? 0) }}</strong>
          <template v-if="hasLimit"> / {{ formatBytes(usage!.quota_bytes) }}</template>
          <span v-else class="usage-unlimited">{{ t('docs.storage.unlimited') }}</span>
        </span>
        <t-button v-if="usage?.can_manage" variant="text" size="small" @click="openEditor">
          {{ t('docs.storage.setQuota') }}
        </t-button>
      </div>

      <!-- The bar only means something when there is a ceiling to measure
           against; without one it would imply a limit that does not exist. -->
      <div v-if="hasLimit" class="usage-track" :class="`level-${level}`">
        <div class="usage-fill" :style="{ width: `${(fraction ?? 0) * 100}%` }" />
      </div>

      <p class="usage-note">
        <template v-if="level === 'full'">{{ t('docs.storage.full') }}</template>
        <template v-else-if="level === 'warning'">{{ t('docs.storage.nearlyFull') }}</template>
        <template v-else-if="usage?.from_default">{{ t('docs.storage.fromDefault') }}</template>
        <template v-else-if="!hasLimit">{{ t('docs.storage.unlimitedHint') }}</template>
      </p>
    </t-loading>

    <t-dialog v-model:visible="editing" :header="t('docs.storage.setQuota')" width="440px"
      destroy-on-close :confirm-btn="{ content: t('common.save'), loading: saving }"
      :cancel-btn="t('common.cancel')" @confirm="save">
      <div class="quota-form">
        <t-input v-model="draft" :placeholder="t('docs.storage.quotaPlaceholder')"
          :status="draftInvalid ? 'error' : undefined" />
        <p class="quota-hint">{{ t('docs.storage.quotaHint') }}</p>
        <!-- Said plainly: lowering a quota below what is already stored is
             allowed, and it is not a delete. -->
        <p v-if="wouldBeOver" class="quota-warning">
          {{ t('docs.storage.belowUsage', { used: formatBytes(usage?.used_bytes ?? 0) }) }}
        </p>
      </div>
    </t-dialog>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'

import { getSpaceUsage, setSpaceQuota, type SpaceUsage } from '@/api/docs'

import { formatBytes, parseBytes, usageFraction, usageLevel } from './formatBytes'

// How much of a space's storage is used, and the one control that changes it.

const props = defineProps<{ spaceId: string }>()

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const editing = ref(false)
const draft = ref('')
const usage = shallowRef<SpaceUsage | null>(null)

const hasLimit = computed(() => (usage.value?.quota_bytes ?? 0) > 0)
const fraction = computed(() =>
  usage.value ? usageFraction(usage.value.used_bytes, usage.value.quota_bytes) : null)
const level = computed(() =>
  usage.value ? usageLevel(usage.value.used_bytes, usage.value.quota_bytes) : 'ok')

/** null while the field is empty, so an untouched dialog is not an error. */
const parsed = computed(() => (draft.value.trim() ? parseBytes(draft.value) : null))
const draftInvalid = computed(() => draft.value.trim() !== '' && parsed.value === null)
const wouldBeOver = computed(() =>
  parsed.value !== null && parsed.value > 0 && parsed.value < (usage.value?.used_bytes ?? 0))

async function load() {
  if (!props.spaceId) return
  loading.value = true
  const id = props.spaceId
  try {
    const next = await getSpaceUsage(id)
    if (props.spaceId === id) usage.value = next
  } catch {
    // A missing usage figure is not worth an error banner on a settings
    // page; the section simply shows nothing.
    if (props.spaceId === id) usage.value = null
  } finally {
    if (props.spaceId === id) loading.value = false
  }
}

function openEditor() {
  // Prefilled with the current limit rather than blank, so "change it
  // slightly" does not mean retyping it.
  draft.value = hasLimit.value ? String(usage.value?.quota_bytes ?? 0) : ''
  editing.value = true
}

async function save() {
  const bytes = draft.value.trim() === '' ? 0 : parsed.value
  if (bytes === null) return
  saving.value = true
  try {
    usage.value = await setSpaceQuota(props.spaceId, bytes)
    editing.value = false
  } catch (err) {
    const msg = (err as { message?: string } | null)?.message
    void MessagePlugin.error(msg ? `${t('docs.storage.saveFailed')}: ${msg}` : t('docs.storage.saveFailed'))
  } finally {
    saving.value = false
  }
}

watch(() => props.spaceId, () => {
  void load()
}, { immediate: true })
</script>

<style scoped>
.storage-usage {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.usage-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.usage-figures {
  font-size: 14px;
  font-variant-numeric: tabular-nums;
}

.usage-unlimited {
  margin-left: 6px;
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}

.usage-track {
  height: 6px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--td-bg-color-component);
}

.usage-fill {
  height: 100%;
  border-radius: 999px;
  background: var(--td-success-color);
  transition: width 0.2s ease;
}

.level-warning .usage-fill {
  background: var(--td-warning-color);
}

.level-full .usage-fill {
  background: var(--td-error-color);
}

.usage-note {
  margin: 0;
  min-height: 18px;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}

.quota-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.quota-hint {
  margin: 0;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
  line-height: 1.5;
}

.quota-warning {
  margin: 0;
  color: var(--td-warning-color);
  font-size: 12px;
  line-height: 1.5;
}
</style>
