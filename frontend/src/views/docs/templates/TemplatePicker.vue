<template>
  <div class="template-picker">
    <t-loading :loading="loading" size="small">
      <!-- Starting blank is the common case and the one people reach for
           when a picker gets in the way, so it is the first choice rather
           than a way out of the dialog. -->
      <button type="button" class="tpl-row tpl-blank" :class="{ on: chosen === '' }"
        @click="choose('')">
        <span class="tpl-icon">📄</span>
        <span class="tpl-main">
          <span class="tpl-name">{{ t('docs.templates.blank') }}</span>
          <span class="tpl-desc">{{ t('docs.templates.blankHint') }}</span>
        </span>
      </button>

      <template v-for="group in grouped" :key="group.name">
        <h3 class="tpl-group">{{ group.name || t('docs.templates.uncategorised') }}</h3>
        <button v-for="tpl in group.items" :key="tpl.id" type="button" class="tpl-row"
          :class="{ on: chosen === tpl.id }" @click="choose(tpl.id)">
          <span class="tpl-icon">{{ tpl.icon || '🧩' }}</span>
          <span class="tpl-main">
            <span class="tpl-name">
              {{ tpl.name }}
              <t-tag v-if="tpl.shared" size="small" variant="light">
                {{ t('docs.templates.shared') }}
              </t-tag>
            </span>
            <span v-if="tpl.description" class="tpl-desc">{{ tpl.description }}</span>
          </span>
        </button>
      </template>

      <p v-if="!templates.length && !loading" class="tpl-empty">{{ t('docs.templates.none') }}</p>
    </t-loading>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { listTemplates, type TemplateView } from '@/api/docs'

// Choosing what a new page starts from.
//
// Grouped by the category their authors gave them, because a flat list of
// thirty templates is a list nobody reads. Within the dialog the choice is
// only recorded; the page is created by whoever opened it, so this component
// never writes anything.

const props = defineProps<{ spaceId: string; modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [string] }>()

const { t } = useI18n()

const loading = ref(false)
const templates = shallowRef<TemplateView[]>([])
const chosen = computed(() => props.modelValue)

interface Group {
  name: string
  items: TemplateView[]
}

/** Grouped in the order the server returned, which already puts the shared
 * ones first within each category. */
const grouped = computed<Group[]>(() => {
  const groups: Group[] = []
  for (const tpl of templates.value) {
    const name = tpl.category ?? ''
    const last = groups[groups.length - 1]
    if (last && last.name === name) {
      last.items.push(tpl)
      continue
    }
    groups.push({ name, items: [tpl] })
  }
  return groups
})

function choose(id: string) {
  emit('update:modelValue', id)
}

async function load() {
  if (!props.spaceId) return
  loading.value = true
  const id = props.spaceId
  try {
    const rows = await listTemplates(id)
    if (props.spaceId === id) templates.value = rows
  } catch {
    // A template library that cannot be reached should not stop somebody
    // creating a page; they get the blank choice, which is what most of them
    // wanted anyway.
    if (props.spaceId === id) templates.value = []
  } finally {
    if (props.spaceId === id) loading.value = false
  }
}

watch(() => props.spaceId, () => {
  void load()
}, { immediate: true })
</script>

<style scoped>
.template-picker {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-height: 60vh;
  overflow-y: auto;
}

.tpl-group {
  margin: 14px 0 4px;
  color: var(--td-text-color-secondary);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.tpl-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  width: 100%;
  padding: 10px 12px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.tpl-row:hover {
  background: var(--td-bg-color-container-hover);
}

.tpl-row.on {
  border-color: var(--td-brand-color);
  background: var(--td-brand-color-light);
}

.tpl-blank {
  margin-bottom: 4px;
}

.tpl-icon {
  flex: none;
  font-size: 20px;
  line-height: 1.3;
}

.tpl-main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.tpl-name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 500;
}

.tpl-desc {
  overflow: hidden;
  color: var(--td-text-color-placeholder);
  font-size: 12px;
  text-overflow: ellipsis;
}

.tpl-empty {
  margin: 12px 0 0;
  color: var(--td-text-color-placeholder);
  font-size: 13px;
}
</style>
