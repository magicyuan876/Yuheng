<template>
  <nav v-if="entries.length" class="toc-sidebar" :aria-label="t('docs.pages.toc')">
    <div class="toc-title">{{ t('docs.pages.toc') }}</div>
    <ul class="toc-list">
      <li
        v-for="entry in entries"
        :key="entry.pos"
        class="toc-item"
        :style="{ paddingLeft: (entry.level - 1) * 12 + 'px' }"
      >
        <button type="button" class="toc-link" @click="emit('select', entry.pos)">
          {{ entry.text || t('docs.tree.untitled') }}
        </button>
      </li>
    </ul>
  </nav>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import type { TocEntry } from './toc'

defineProps<{ entries: TocEntry[] }>()
const emit = defineEmits<{ select: [pos: number] }>()

const { t } = useI18n()
</script>

<style scoped lang="less">
.toc-sidebar {
  width: 200px;
  flex: none;
  padding: 4px 0 4px 16px;
  border-left: 1px solid var(--td-component-stroke);
  position: sticky;
  top: 0;
  align-self: flex-start;
  max-height: calc(100vh - 120px);
  overflow-y: auto;
}

.toc-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--td-text-color-placeholder);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-bottom: 8px;
}

.toc-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.toc-item {
  margin: 2px 0;
}

.toc-link {
  display: block;
  width: 100%;
  border: none;
  background: transparent;
  padding: 3px 6px;
  border-radius: 4px;
  text-align: left;
  font-size: 12.5px;
  line-height: 18px;
  color: var(--td-text-color-secondary);
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;

  &:hover {
    background: var(--td-bg-color-container-hover);
    color: var(--td-text-color-primary);
  }
}
</style>
