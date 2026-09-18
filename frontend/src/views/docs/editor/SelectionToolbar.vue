<template>
  <div
    v-if="visible"
    ref="bar"
    class="docs-toolbar"
    :class="{ 'docs-toolbar--below': placement.below }"
    role="toolbar"
    :aria-label="t('docs.toolbar.label')"
    :style="{ left: `${placement.left}px`, top: `${placement.top}px`, width: `${TOOLBAR_WIDTH}px` }"
    @keydown="onKeyDown"
  >
    <template v-for="(group, gi) in groups" :key="gi">
      <span v-if="gi > 0" class="docs-toolbar-divider" aria-hidden="true" />
      <button
        v-for="item in group"
        :key="item.id"
        :ref="(el) => registerButton(item.id, el)"
        type="button"
        class="docs-toolbar-button"
        :class="{ 'is-active': isActive(item) }"
        :title="tooltip(item)"
        :aria-label="t(item.labelKey)"
        :aria-pressed="item.activeName ? isActive(item) : undefined"
        :tabindex="item.id === focusedId ? 0 : -1"
        @mousedown.prevent
        @click="run(item)"
        @focus="focusedId = item.id"
      >
        <t-icon :name="item.icon" size="16px" />
      </button>
    </template>

    <form v-if="linkOpen" class="docs-toolbar-link" @submit.prevent="commitLink">
      <input
        ref="linkInput"
        v-model="linkDraft"
        type="url"
        class="docs-toolbar-link-input"
        :placeholder="t('docs.toolbar.linkPlaceholder')"
        :aria-label="t('docs.toolbar.link')"
        :aria-invalid="linkDraft !== '' && !linkValid"
        @keydown.esc.prevent.stop="closeLink"
      />
      <button type="submit" class="docs-toolbar-link-apply" :disabled="!linkValid">
        {{ t('common.confirm') }}
      </button>
      <button
        v-if="hasLink"
        type="button"
        class="docs-toolbar-link-apply"
        @click="removeLink"
      >
        {{ t('docs.toolbar.linkRemove') }}
      </button>
    </form>
  </div>
</template>

<script setup lang="ts">
import type { Editor } from '@tiptap/core'
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { isSafeLinkHref } from './paste'
import {
  moveFocus, TOOLBAR_ITEMS, TOOLBAR_WIDTH, toolbarGroups, type ToolbarItem,
} from './toolbar'

const props = defineProps<{
  visible: boolean
  placement: { left: number; top: number; below: boolean }
  editor: Editor | null
  /** Bumped whenever the document or selection changed, so the pressed states
   * are recomputed; the editor itself is not reactive. */
  revision: number
}>()

const emit = defineEmits<{ dismiss: [] }>()
const { t } = useI18n()

const bar = ref<HTMLElement | null>(null)
const buttons = new Map<string, HTMLElement>()
const groups = toolbarGroups()

/**
 * The bar is one tab stop: exactly one button is reachable by Tab, and the
 * arrow keys move between them. Which one that is has to survive the bar being
 * hidden and shown again, so it is held here rather than read from the DOM.
 */
const focusedId = ref<string>(TOOLBAR_ITEMS[0]!.id)

const flat = computed(() => groups.flat())

/**
 * The link row.
 *
 * It lives in the bar rather than in a dialogue for one reason: a dialogue
 * takes focus away from the document, and the selection the link is going on
 * is the thing that would be lost. The address is checked with the same
 * function that checks a pasted one, so a link cannot be typed in that could
 * not be pasted in.
 */
const linkOpen = ref(false)
const linkDraft = ref('')
const linkInput = ref<HTMLInputElement | null>(null)
const linkValid = computed(() => isSafeLinkHref(linkDraft.value.trim()))
const hasLink = computed(() => {
  void props.revision
  return props.editor?.isActive('link') ?? false
})

function openLink() {
  void props.revision
  linkDraft.value = String(props.editor?.getAttributes('link')?.href ?? '')
  linkOpen.value = true
  void nextTick(() => linkInput.value?.focus())
}

function closeLink() {
  linkOpen.value = false
  linkDraft.value = ''
  props.editor?.commands.focus()
}

function commitLink() {
  const href = linkDraft.value.trim()
  if (!isSafeLinkHref(href) || !props.editor) return
  props.editor.chain().focus().extendMarkRange('link').setLink({ href }).run()
  closeLink()
}

function removeLink() {
  props.editor?.chain().focus().extendMarkRange('link').unsetLink().run()
  closeLink()
}

function registerButton(id: string, el: unknown) {
  if (el && el instanceof HTMLElement) buttons.set(id, el)
  else buttons.delete(id)
}

/** A button reads as pressed when the mark or node it stands for is active. */
function isActive(item: ToolbarItem): boolean {
  // Touched so the computation re-runs when the selection moves.
  void props.revision
  if (!item.activeName || !props.editor) return false
  try {
    return props.editor.isActive(item.activeName, item.activeAttrs)
  } catch {
    // isActive throws for a name the schema does not have, which happens
    // while extensions are still being swapped on a page change.
    return false
  }
}

function tooltip(item: ToolbarItem): string {
  const label = t(item.labelKey)
  if (!item.shortcut) return label
  return `${label} (${item.shortcut.replace('Mod', isApple() ? '⌘' : 'Ctrl')})`
}

function isApple(): boolean {
  return /mac|iphone|ipad/i.test(navigator.platform || navigator.userAgent)
}

function run(item: ToolbarItem) {
  const editor = props.editor
  if (!editor) return
  // The link entry opens a row of its own rather than changing the text.
  if (item.id === 'link') {
    if (linkOpen.value) closeLink()
    else openLink()
    return
  }

  const chain = editor.chain().focus() as unknown as Record<string, () => { run: () => void }>
  const call = ACTIONS[item.id]
  if (!call) return
  call(chain)?.run()
}

/** What each button does, kept as data so `run` stays one path. */
const ACTIONS: Record<string, (c: Record<string, (...a: never[]) => { run: () => void }>) => { run: () => void } | undefined> = {
  bold: (c) => c.toggleBold?.(),
  italic: (c) => c.toggleItalic?.(),
  underline: (c) => c.toggleUnderline?.(),
  strike: (c) => c.toggleStrike?.(),
  code: (c) => c.toggleCode?.(),
  highlight: (c) => c.toggleHighlight?.(),
  heading1: (c) => (c.toggleHeading as (a: unknown) => { run: () => void })?.({ level: 1 }),
  heading2: (c) => (c.toggleHeading as (a: unknown) => { run: () => void })?.({ level: 2 }),
  bulletList: (c) => c.toggleBulletList?.(),
  blockquote: (c) => c.toggleBlockquote?.(),
  clearFormat: (c) => (c.unsetAllMarks?.() as unknown as Record<string, () => { run: () => void }>)
    ?.clearNodes?.(),
}

/**
 * The keys the bar owns while focus is inside it.
 *
 * Home and End are included because they are what a person used to a toolbar
 * reaches for, and cost one line each.
 */
function onKeyDown(event: KeyboardEvent) {
  const items = flat.value
  const at = items.findIndex((i) => i.id === focusedId.value)
  const go = (index: number) => {
    focusedId.value = items[index]!.id
    void nextTick(() => buttons.get(focusedId.value)?.focus())
    event.preventDefault()
  }

  switch (event.key) {
    case 'ArrowRight': return go(moveFocus(at, 1, items.length))
    case 'ArrowLeft': return go(moveFocus(at, -1, items.length))
    case 'Home': return go(0)
    case 'End': return go(items.length - 1)
    case 'Escape':
      // Back to the text, with the selection intact: somebody who opened the
      // bar by accident should not lose their place.
      event.preventDefault()
      emit('dismiss')
      props.editor?.commands.focus()
      return
    default:
  }
}

// A bar that has just appeared starts from its first entry, so the arrow keys
// behave the same way every time rather than resuming wherever they left off.
watch(() => props.visible, (shown) => {
  if (shown) {
    focusedId.value = TOOLBAR_ITEMS[0]!.id
  } else {
    // A link row left open over a selection that no longer exists would apply
    // to whatever is selected next.
    linkOpen.value = false
    linkDraft.value = ''
  }
})
</script>

<style scoped lang="less">
.docs-toolbar {
  position: fixed;
  z-index: 1400;
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 4px 6px;
  box-sizing: border-box;
  border: 1px solid var(--td-component-stroke);
  border-radius: 8px;
  background: var(--td-bg-color-container);
  box-shadow: 0 6px 20px rgb(0 0 0 / 12%);
}

.docs-toolbar-divider {
  width: 1px;
  height: 18px;
  margin: 0 4px;
  background: var(--td-component-stroke);
}

.docs-toolbar-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--td-text-color-primary);
  cursor: pointer;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }

  &:focus-visible {
    outline: 2px solid var(--td-brand-color);
    outline-offset: -2px;
  }

  &.is-active {
    background: var(--td-brand-color-light);
    color: var(--td-brand-color);
  }
}

.docs-toolbar-link {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: 1;
  min-width: 0;
  margin-left: 4px;
}

.docs-toolbar-link-input {
  flex: 1;
  min-width: 0;
  height: 26px;
  padding: 0 8px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 6px;
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
  font-size: 13px;

  &[aria-invalid='true'] {
    border-color: var(--td-error-color);
  }
}

.docs-toolbar-link-apply {
  height: 26px;
  padding: 0 8px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--td-brand-color);
  font-size: 13px;
  cursor: pointer;

  &:disabled {
    color: var(--td-text-color-disabled);
    cursor: default;
  }
}
</style>
