<template>
  <div
    v-if="open"
    ref="menu"
    class="docs-suggest"
    :style="{ left: `${position.left}px`, top: `${position.top}px` }"
    role="listbox"
  >
    <p v-if="loading" class="docs-suggest-note">{{ t('docs.links.searching') }}</p>
    <p v-else-if="!items.length" class="docs-suggest-note">
      {{ kind === 'command' ? t('docs.commands.noMatches') : t('docs.links.noMatches') }}
    </p>
    <button
      v-for="(item, index) in items"
      :key="item.key"
      type="button"
      role="option"
      class="docs-suggest-item"
      :class="{ 'is-active': index === selected }"
      :aria-selected="index === selected"
      @mousedown.prevent="emit('choose', index)"
      @mouseenter="emit('hover', index)"
    >
      <span class="docs-suggest-icon">
        <template v-if="item.icon">{{ item.icon }}</template>
        <t-icon v-else :name="item.iconName ?? fallbackIcon" size="14px" />
      </span>
      <span class="docs-suggest-text">
        <span class="docs-suggest-title">{{ item.title }}</span>
        <span v-if="item.hint" class="docs-suggest-hint">{{ item.hint }}</span>
      </span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

/** One row of the menu, already shaped by whoever fetched it. */
export interface SuggestionItem {
  key: string
  title: string
  hint?: string
  /** An emoji chosen for the page itself, shown in preference to an icon. */
  icon?: string
  /** A tdesign icon name, used when the entry has no emoji of its own. */
  iconName?: string
}

const props = defineProps<{
  open: boolean
  loading: boolean
  items: SuggestionItem[]
  selected: number
  kind: 'page' | 'mention' | 'command'
  position: { left: number; top: number }
}>()

const emit = defineEmits<{ choose: [index: number]; hover: [index: number] }>()
const { t } = useI18n()

/** What to draw for an entry that brought no icon of its own. */
const fallbackIcon = computed(() => (props.kind === 'mention' ? 'user' : 'file'))
</script>

<style scoped lang="less">
.docs-suggest {
  position: fixed;
  z-index: 1200;
  min-width: 240px;
  max-width: 360px;
  max-height: 280px;
  overflow-y: auto;
  padding: 4px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container);
  box-shadow: var(--td-shadow-2);
}

.docs-suggest-note {
  margin: 0;
  padding: 8px 10px;
  font-size: 13px;
  color: var(--td-text-color-placeholder);
}

.docs-suggest-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  border: none;
  background: transparent;
  border-radius: 6px;
  padding: 6px 8px;
  cursor: pointer;
  text-align: left;

  &.is-active {
    background: var(--td-bg-color-container-hover);
  }
}

.docs-suggest-icon {
  flex: none;
  width: 16px;
  text-align: center;
  color: var(--td-text-color-placeholder);
}

.docs-suggest-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.docs-suggest-title {
  font-size: 13.5px;
  color: var(--td-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.docs-suggest-hint {
  font-size: 12px;
  color: var(--td-text-color-placeholder);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
