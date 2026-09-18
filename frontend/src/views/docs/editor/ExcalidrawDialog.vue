<template>
  <Teleport to="body">
    <div class="docs-excalidraw" role="dialog" :aria-label="t('docs.media.diagramEdit')">
      <div class="docs-excalidraw-bar">
        <span>{{ t('docs.media.diagramEdit') }}</span>
        <span class="docs-excalidraw-spacer" />
        <t-button size="small" theme="primary" :loading="saving" @click="save">
          {{ t('common.save') }}
        </t-button>
        <button
          type="button"
          class="docs-excalidraw-close"
          :aria-label="t('docs.media.closeEditor')"
          @click="emit('close')"
        >
          <t-icon name="close" size="16px" />
        </button>
      </div>

      <div class="docs-excalidraw-body">
        <div ref="mount" class="docs-excalidraw-mount" />
        <p v-if="status === 'loading'" class="docs-excalidraw-note">{{ t('docs.media.diagramLoading') }}</p>
        <p v-else-if="status === 'failed'" class="docs-excalidraw-note docs-excalidraw-note--error">
          {{ t('docs.media.diagramEditorFailed') }}
        </p>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { MessagePlugin } from 'tdesign-vue-next'
import { onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  loadExcalidraw, parseScene, renderSceneToSVG, serialiseScene,
  type ExcalidrawModule, type ExcalidrawScene,
} from './excalidraw'
import { ReactIsland } from './reactIsland'

const props = defineProps<{ source: string }>()
const emit = defineEmits<{ save: [payload: { scene: string; svg: string }]; close: [] }>()
const { t } = useI18n()

const mount = ref<HTMLElement | null>(null)
const status = ref<'loading' | 'ready' | 'failed'>('loading')
const saving = ref(false)

/**
 * The React subtree, and the editor's own handle onto its canvas.
 *
 * Both are shallow refs holding foreign objects: making Excalidraw's element
 * tree reactive would have Vue walk a very large structure on every change for
 * no benefit, and would fight React for ownership of it.
 */
const island = shallowRef<ReactIsland | null>(null)
const module = shallowRef<ExcalidrawModule | null>(null)
const api = shallowRef<{
  getSceneElements: () => readonly unknown[]
  getAppState: () => Record<string, unknown>
  getFiles: () => Record<string, unknown>
} | null>(null)

const initial: ExcalidrawScene = parseScene(props.source)

onMounted(async () => {
  let loaded: ExcalidrawModule
  let react: { createElement: (t: unknown, p: unknown) => unknown }
  let client: { createRoot: (el: Element) => { render: (e: unknown) => void; unmount: () => void } }
  try {
    // Imported here and nowhere else: this is the largest module in the
    // application and most readers of most pages never open a drawing.
    ;[loaded, react, client] = await Promise.all([
      loadExcalidraw(),
      import('react') as unknown as Promise<{ createElement: (t: unknown, p: unknown) => unknown }>,
      import('react-dom/client') as unknown as Promise<{
        createRoot: (el: Element) => { render: (e: unknown) => void; unmount: () => void }
      }>,
    ])
  } catch {
    status.value = 'failed'
    return
  }

  // The component may already be gone: the import takes as long as it takes,
  // and nothing stops somebody closing the dialogue meanwhile.
  if (!mount.value) {
    status.value = 'failed'
    return
  }

  module.value = loaded
  const created = new ReactIsland((container) => client.createRoot(container))
  island.value = created
  created.render(mount.value, react.createElement(loaded.Excalidraw, {
    initialData: { elements: initial.elements, appState: initial.appState, files: initial.files },
    excalidrawAPI: (instance: typeof api.value) => {
      api.value = instance
    },
    langCode: navigator.language?.startsWith('zh') ? 'zh-CN' : 'en',
    theme: isDark() ? 'dark' : 'light',
  }))
  status.value = 'ready'
})

/**
 * Tears the React subtree down with the dialogue.
 *
 * Without this the whole element tree, its listeners and everything they
 * closed over would stay alive for as long as the page is open, once per
 * drawing anybody opened. ReactIsland.destroy also makes a late import
 * harmless: if the module resolves after this ran, nothing is mounted.
 */
onBeforeUnmount(() => {
  island.value?.destroy()
  island.value = null
  api.value = null
  module.value = null
})

function isDark(): boolean {
  return document.documentElement.getAttribute('theme-mode') === 'dark'
    || document.documentElement.classList.contains('dark')
}

async function save() {
  const editor = api.value
  const loaded = module.value
  if (!editor || !loaded || saving.value) return
  saving.value = true
  try {
    const elements = editor.getSceneElements()
    const appState = editor.getAppState()
    const files = editor.getFiles()
    const scene = serialiseScene(elements, appState, files)
    // The rendering is produced by the editor itself, so it looks exactly like
    // what was being drawn, and it is the only thing a reader ever loads.
    const svg = await renderSceneToSVG(loaded, {
      ...parseScene(scene), elements: [...elements], files,
    })
    emit('save', { scene, svg })
  } catch (err) {
    void MessagePlugin.error((err as { message?: string })?.message || t('docs.media.diagramSaveFailed'))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped lang="less">
.docs-excalidraw {
  position: fixed;
  inset: 0;
  z-index: 3000;
  display: flex;
  flex-direction: column;
  background: var(--td-bg-color-page);
}

.docs-excalidraw-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 14px;
  border-bottom: 1px solid var(--td-component-stroke);
  font-size: 14px;
  color: var(--td-text-color-primary);
}

.docs-excalidraw-spacer {
  flex: 1;
}

.docs-excalidraw-close {
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

.docs-excalidraw-body {
  position: relative;
  flex: 1;
  min-height: 0;
}

.docs-excalidraw-mount {
  width: 100%;
  height: 100%;
}

.docs-excalidraw-note {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0;
  font-size: 14px;
  color: var(--td-text-color-placeholder);
  pointer-events: none;

  &--error {
    color: var(--td-error-color);
  }
}
</style>
