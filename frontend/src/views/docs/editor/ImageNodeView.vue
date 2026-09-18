<template>
  <NodeViewWrapper
    class="docs-image"
    :class="[`docs-image--${align}`, { 'docs-image--selected': selected }]"
    :data-drag-handle="editor.isEditable ? '' : undefined"
  >
    <figure class="docs-image-frame" :style="frameStyle">
      <img
        :src="src"
        :srcset="srcset || undefined"
        :sizes="srcset ? '(max-width: 720px) 100vw, 720px' : undefined"
        :alt="node.attrs.alt || ''"
        :title="node.attrs.title || undefined"
        loading="lazy"
        @load="onLoad"
      />

      <div v-if="editor.isEditable" class="docs-image-tools">
        <t-tooltip v-for="option in alignments" :key="option.value" :content="t(option.label)">
          <button
            type="button"
            class="docs-image-tool"
            :class="{ 'is-active': align === option.value }"
            @click="setAlign(option.value)"
          >
            <t-icon :name="option.icon" size="14px" />
          </button>
        </t-tooltip>
        <t-tooltip :content="t('docs.attachments.imageAlt')">
          <button type="button" class="docs-image-tool" @click="editAlt">
            <t-icon name="edit" size="14px" />
          </button>
        </t-tooltip>
        <t-tooltip :content="t('docs.attachments.remove')">
          <button type="button" class="docs-image-tool" @click="deleteNode">
            <t-icon name="delete" size="14px" />
          </button>
        </t-tooltip>
      </div>

      <!-- Dragging the right edge sets an explicit width; the schema bounds
           it to 16-8192, and the server rejects anything outside that. -->
      <span
        v-if="editor.isEditable"
        class="docs-image-resize"
        role="separator"
        aria-orientation="vertical"
        @pointerdown="startResize"
      />
    </figure>
    <figcaption v-if="node.attrs.alt" class="docs-image-caption">{{ node.attrs.alt }}</figcaption>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from '@tiptap/vue-3'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { attachmentSrc, attachmentSrcSet } from './attachments'

const props = defineProps<NodeViewProps>()
const { t } = useI18n()

const MIN_WIDTH = 16
const MAX_WIDTH = 8192

// TDesign names these after the axis of the bar in the glyph rather than the
// direction of the alignment, so the horizontal-alignment icons are the
// "vertical-align" ones. Verified against tdesign-icons-vue-next's exports.
const alignments = [
  { value: 'left', icon: 'format-vertical-align-left', label: 'docs.attachments.alignLeft' },
  { value: 'center', icon: 'format-vertical-align-center', label: 'docs.attachments.alignCenter' },
  { value: 'right', icon: 'format-vertical-align-right', label: 'docs.attachments.alignRight' },
] as const

const align = computed(() => String(props.node.attrs.align ?? 'center'))

/** Our own files are addressed by id; an external image keeps its own URL. */
const src = computed(() => {
  const id = props.node.attrs.attachmentId as string | null
  return id ? attachmentSrc(id) : String(props.node.attrs.src ?? '')
})

const srcset = computed(() => {
  const id = props.node.attrs.attachmentId as string | null
  return id ? attachmentSrcSet(id, variants.value) : ''
})

/** Widths smaller than the stored one, which is what the server will render. */
const variants = computed(() => {
  const width = Number(props.node.attrs.width ?? 0)
  const offered = [320, 800, 1600]
  return width > 0 ? offered.filter((w) => w < width) : offered
})

const frameStyle = computed(() => {
  const width = Number(props.node.attrs.width ?? 0)
  return width > 0 ? { width: `${width}px` } : {}
})

function setAlign(value: 'left' | 'center' | 'right') {
  props.updateAttributes({ align: value })
}

function editAlt() {
  const next = window.prompt(t('docs.attachments.imageAltPrompt'), String(props.node.attrs.alt ?? ''))
  if (next === null) return
  props.updateAttributes({ alt: next.trim() || null })
}

/** Records the natural size the first time the browser knows it, so the
 * srcset can stop offering renderings larger than the original. */
function onLoad(event: Event) {
  if (props.node.attrs.width) return
  const img = event.target as HTMLImageElement
  if (img.naturalWidth > 0) {
    props.updateAttributes({ width: img.naturalWidth, height: img.naturalHeight })
  }
}

const resizing = ref(false)

function startResize(event: PointerEvent) {
  if (resizing.value) return
  const frame = (event.currentTarget as HTMLElement).parentElement
  if (!frame) return
  resizing.value = true
  const startX = event.clientX
  const startWidth = frame.getBoundingClientRect().width
  const ratio = Number(props.node.attrs.height ?? 0) / Number(props.node.attrs.width ?? 1)

  const move = (e: PointerEvent) => {
    const width = Math.round(Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, startWidth + (e.clientX - startX))))
    frame.style.width = `${width}px`
  }
  const up = (e: PointerEvent) => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
    resizing.value = false
    const width = Math.round(Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, startWidth + (e.clientX - startX))))
    const height = ratio > 0 ? Math.round(width * ratio) : null
    props.updateAttributes({ width, height })
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
  event.preventDefault()
}
</script>

<style scoped lang="less">
.docs-image {
  display: flex;
  flex-direction: column;
  margin: 12px 0;

  &--left {
    align-items: flex-start;
  }

  &--center {
    align-items: center;
  }

  &--right {
    align-items: flex-end;
  }
}

.docs-image-frame {
  position: relative;
  margin: 0;
  max-width: 100%;
  line-height: 0;

  img {
    display: block;
    max-width: 100%;
    height: auto;
    border-radius: 6px;
  }
}

.docs-image--selected .docs-image-frame img {
  outline: 2px solid var(--td-brand-color);
  outline-offset: 2px;
}

.docs-image-tools {
  position: absolute;
  top: 6px;
  right: 6px;
  display: none;
  gap: 2px;
  padding: 2px;
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.55);
}

.docs-image-frame:hover .docs-image-tools {
  display: flex;
}

.docs-image-tool {
  border: none;
  background: transparent;
  color: #fff;
  border-radius: 4px;
  padding: 3px 4px;
  cursor: pointer;
  line-height: 0;

  &:hover,
  &.is-active {
    background: rgba(255, 255, 255, 0.24);
  }
}

.docs-image-resize {
  position: absolute;
  top: 50%;
  right: -4px;
  width: 8px;
  height: 36px;
  transform: translateY(-50%);
  border-radius: 4px;
  background: var(--td-brand-color);
  opacity: 0;
  cursor: ew-resize;
}

.docs-image-frame:hover .docs-image-resize {
  opacity: 0.85;
}

.docs-image-caption {
  margin-top: 6px;
  font-size: 12.5px;
  color: var(--td-text-color-placeholder);
}
</style>
