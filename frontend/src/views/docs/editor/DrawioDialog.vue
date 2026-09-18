<template>
  <Teleport to="body">
    <div class="docs-drawio" role="dialog" :aria-label="t('docs.media.diagramEdit')">
      <div class="docs-drawio-bar">
        <span>{{ t('docs.media.diagramEdit') }}</span>
        <button type="button" @click="emit('close')">
          <t-icon name="close" size="16px" />
        </button>
      </div>
      <iframe
        ref="frame"
        class="docs-drawio-frame"
        :src="frameSrc"
        :title="t('docs.media.diagramEdit')"
        referrerpolicy="no-referrer"
      />
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  drawioFrameURL, DrawioSession, isFromEditor, originOf, parseMessage,
} from './drawio'
import { DOCS_DIAGRAMS, type DiagramHost } from './linkContext'

const props = defineProps<{ source: string }>()
const emit = defineEmits<{ save: [payload: { xml: string; svg: string }]; close: [] }>()
const { t } = useI18n()

const host = inject<DiagramHost | null>(DOCS_DIAGRAMS, null)
const frame = ref<HTMLIFrameElement | null>(null)
const session = new DrawioSession(props.source)

const base = computed(() => host?.drawioURL.value ?? '')
const frameSrc = computed(() => (base.value ? drawioFrameURL(base.value, isDark()) : 'about:blank'))
const expectedOrigin = computed(() => originOf(base.value))

function isDark(): boolean {
  return document.documentElement.getAttribute('theme-mode') === 'dark'
    || document.documentElement.classList.contains('dark')
}

/**
 * Handles one message from the editor.
 *
 * The first thing it does is decide whether the message is ours at all. A
 * window with an iframe in it receives messages from anything that can reach
 * it, so the origin and the source frame are both checked before a single byte
 * is read; everything else is ignored in silence, because an unrelated library
 * posting to the window is ordinary rather than an attack.
 */
function onMessage(event: MessageEvent) {
  if (!isFromEditor(event, expectedOrigin.value, frame.value)) return
  const message = parseMessage(event.data)
  if (!message) return

  const action = session.receive(message)
  switch (action.kind) {
    case 'load':
    case 'export': {
      const request = DrawioSession.request(action)
      if (request) frame.value?.contentWindow?.postMessage(request, expectedOrigin.value)
      break
    }
    case 'save':
      emit('save', { xml: action.xml, svg: action.svg })
      break
    case 'close':
      emit('close')
      break
    default:
      break
  }
}

function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') emit('close')
}

onMounted(() => {
  window.addEventListener('message', onMessage)
  window.addEventListener('keydown', onKey)
})

// Both listeners are removed with the dialog. The iframe goes with the
// element, so nothing of the editor outlives this component.
onBeforeUnmount(() => {
  window.removeEventListener('message', onMessage)
  window.removeEventListener('keydown', onKey)
})
</script>

<style scoped lang="less">
.docs-drawio {
  position: fixed;
  inset: 0;
  z-index: 3000;
  display: flex;
  flex-direction: column;
  background: var(--td-bg-color-page);
}

.docs-drawio-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 14px;
  border-bottom: 1px solid var(--td-component-stroke);
  font-size: 14px;
  color: var(--td-text-color-primary);

  button {
    border: none;
    background: transparent;
    color: var(--td-text-color-secondary);
    cursor: pointer;
    border-radius: 4px;
    padding: 4px;
    line-height: 0;

    &:hover {
      background: var(--td-bg-color-container-hover);
    }
  }
}

.docs-drawio-frame {
  flex: 1;
  width: 100%;
  border: none;
}
</style>
