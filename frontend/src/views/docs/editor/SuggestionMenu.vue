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
    <template v-for="(item, index) in items" :key="item.key">
      <p
        v-if="item.group && item.group !== items[index - 1]?.group"
        class="docs-suggest-group"
        aria-hidden="true"
      >
        {{ item.group }}
      </p>
      <button
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
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

/** One row of the menu, already shaped by whoever fetched it. */
export interface SuggestionItem {
  key: string
  title: string
  hint?: string
  /**
   * A section heading shown above this row when it differs from the row
   * before it. Only the command menu groups its rows; already translated,
   * like everything else the row carries.
   */
  group?: string
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
  kind: 'page' | 'mention' | 'command' | 'emoji'
  position: { left: number; top: number }
}>()

const emit = defineEmits<{ choose: [index: number]; hover: [index: number] }>()
const { t } = useI18n()

const menu = ref<HTMLElement | null>(null)

/** What to draw for an entry that brought no icon of its own. */
const fallbackIcon = computed(() => (props.kind === 'mention' ? 'user' : 'file'))

/**
 * The list outgrows the popup and only the first rows are visible, so an
 * arrow-key move has to bring the active row to the eye: nearest keeps a
 * move by one from scrolling the whole list when the row is already there.
 */
watch(() => props.selected, async () => {
  await nextTick()
  menu.value?.querySelector('.is-active')?.scrollIntoView({ block: 'nearest' })
})
</script>

<style scoped lang="less">
.docs-suggest {
  position: fixed;
  z-index: 1200;
  min-width: 240px;
  max-width: 360px;
  max-height: 360px;
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

.docs-suggest-group {
  margin: 0;
  padding: 6px 8px 2px;
  font-size: 11px;
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
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  text-align: center;
  // A badge such as "H1" stands in for an icon the set does not have, so it
  // is drawn at the weight an icon reads at rather than as body text.
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
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
