<template>
  <NodeViewWrapper
    class="docs-diagram"
    :class="[`docs-diagram--${align}`, { 'docs-diagram--selected': selected }]"
  >
    <!-- The reading state, and the only state an export or a share page ever
         reaches: the rendered preview, with no editor loaded at all. -->
    <figure class="docs-diagram-frame" :style="frameStyle">
      <img v-if="previewSrc" :src="previewSrc" :alt="t('docs.media.diagram')" loading="lazy" />
      <p v-else class="docs-diagram-empty">{{ t('docs.media.diagramNoPreview') }}</p>

      <div v-if="editor.isEditable" class="docs-diagram-tools">
        <t-tooltip v-if="canEdit" :content="t('docs.media.diagramEdit')">
          <button type="button" @click="openEditor">
            <t-icon name="edit" size="14px" />
          </button>
        </t-tooltip>
        <t-tooltip :content="t('docs.attachments.remove')">
          <button type="button" @click="deleteNode">
            <t-icon name="delete" size="14px" />
          </button>
        </t-tooltip>
      </div>
    </figure>

    <p v-if="editor.isEditable && !canEdit" class="docs-diagram-note">
      {{ kind === 'drawio' ? t('docs.media.drawioUnavailable') : t('docs.media.excalidrawUnavailable') }}
    </p>

    <DrawioDialog
      v-if="kind === 'drawio' && editing"
      :source="source"
      @save="onDrawioSaved"
      @close="editing = false"
    />
    <ExcalidrawDialog
      v-if="kind === 'excalidraw' && editing"
      :source="source"
      @save="onExcalidrawSaved"
      @close="editing = false"
    />
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from '@tiptap/vue-3'
import { MessagePlugin } from 'tdesign-vue-next'
import { computed, inject, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { useAttachmentUrl } from './useAttachmentUrl'
import DrawioDialog from './DrawioDialog.vue'
import { EMPTY_DRAWIO_XML } from './drawio'
import { emptyScene } from './excalidraw'
import ExcalidrawDialog from './ExcalidrawDialog.vue'
import { DOCS_DIAGRAMS, type DiagramHost } from './linkContext'

const props = defineProps<NodeViewProps>()
const { t } = useI18n()

const host = inject<DiagramHost | null>(DOCS_DIAGRAMS, null)

const kind = computed(() => props.node.type.name as 'drawio' | 'excalidraw')
const align = computed(() => String(props.node.attrs.align ?? 'center'))

/**
 * The reader only ever needs the preview. That is what makes a saved diagram
 * show in an export, a share page and a read-only view without any editor
 * being loaded — the source attachment is fetched only when somebody actually
 * opens the editor.
 */
const previewId = computed(() => (props.node.attrs.previewAttachmentId as string | null) ?? null)
const previewSrc = useAttachmentUrl(previewId)

// draw.io needs a self-hosted editor to be configured; Excalidraw ships with
// the application and only needs its module, which is loaded on demand.
const canEdit = computed(() =>
  kind.value === 'excalidraw' || (kind.value === 'drawio' && !!host?.drawioURL.value))

const frameStyle = computed(() => {
  const width = Number(props.node.attrs.width ?? 0)
  return width > 0 ? { width: `${width}px` } : {}
})

const editing = ref(false)
const source = ref('')

async function openEditor() {
  if (!canEdit.value || !host) return
  const id = props.node.attrs.attachmentId as string | null
  if (id) {
    try {
      source.value = await host.load(id)
    } catch {
      // Opening an editor on the wrong drawing would be worse than refusing:
      // saving would then overwrite the real one with a blank canvas.
      void MessagePlugin.error(t('docs.media.diagramLoadFailed'))
      return
    }
  } else {
    source.value = kind.value === 'drawio' ? EMPTY_DRAWIO_XML : JSON.stringify(emptyScene())
  }
  editing.value = true
}

function onDrawioSaved(payload: { xml: string; svg: string }) {
  void store(payload.xml, payload.svg)
}

function onExcalidrawSaved(payload: { scene: string; svg: string }) {
  void store(payload.scene, payload.svg)
}

async function store(source: string, svg: string) {
  editing.value = false
  if (!host) return
  try {
    const stored = await host.save(kind.value, source, svg, 'diagram')
    props.updateAttributes({
      attachmentId: stored.attachmentId,
      previewAttachmentId: stored.previewAttachmentId,
    })
  } catch (err) {
    void MessagePlugin.error((err as { message?: string })?.message || t('docs.media.diagramSaveFailed'))
  }
}
</script>

<style scoped lang="less">
.docs-diagram {
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

.docs-diagram-frame {
  position: relative;
  margin: 0;
  max-width: 100%;
  line-height: 0;

  img {
    display: block;
    max-width: 100%;
    height: auto;
    border-radius: 6px;
    background: var(--td-bg-color-container);
  }
}

.docs-diagram--selected .docs-diagram-frame img {
  outline: 2px solid var(--td-brand-color);
  outline-offset: 2px;
}

.docs-diagram-empty {
  margin: 0;
  padding: 24px 32px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--td-text-color-placeholder);
  border: 1px dashed var(--td-component-stroke);
  border-radius: 8px;
}

.docs-diagram-tools {
  position: absolute;
  top: 6px;
  right: 6px;
  display: none;
  gap: 2px;
  padding: 2px;
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.55);

  button {
    border: none;
    background: transparent;
    color: #fff;
    border-radius: 4px;
    padding: 3px 4px;
    line-height: 0;
    cursor: pointer;

    &:hover {
      background: rgba(255, 255, 255, 0.24);
    }
  }
}

.docs-diagram-frame:hover .docs-diagram-tools {
  display: flex;
}

.docs-diagram-note {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--td-text-color-placeholder);
}
</style>
