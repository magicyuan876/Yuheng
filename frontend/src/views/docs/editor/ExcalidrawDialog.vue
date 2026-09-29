<template>
  <Teleport to="body">
    <div
      class="bg-background fixed inset-0 z-3000 flex flex-col"
      role="dialog"
      :aria-label="t('docs.media.diagramEdit')"
    >
      <div
        class="text-foreground flex items-center gap-2.5 border-b border-[var(--td-component-stroke)] px-3.5 py-2 text-sm"
      >
        <span>{{ t("docs.media.diagramEdit") }}</span>
        <span class="flex-1" />
        <Button size="sm" :disabled="saving" @click="save">
          <Loader2Icon v-if="saving" class="animate-spin" />
          {{ t("common.save") }}
        </Button>
        <button
          type="button"
          data-slot="excalidraw-close"
          class="text-muted-foreground hover:bg-accent inline-flex cursor-pointer rounded border-0 p-1 leading-0"
          :aria-label="t('docs.media.closeEditor')"
          @click="emit('close')"
        >
          <XIcon class="size-4" />
        </button>
      </div>

      <div class="relative min-h-0 flex-1">
        <div ref="mount" class="h-full w-full" />
        <p
          v-if="status === 'loading'"
          class="text-placeholder pointer-events-none absolute inset-0 m-0 flex items-center justify-center text-sm"
        >
          {{ t("docs.media.diagramLoading") }}
        </p>
        <p
          v-else-if="status === 'failed'"
          class="text-destructive pointer-events-none absolute inset-0 m-0 flex items-center justify-center text-sm"
        >
          {{ t("docs.media.diagramEditorFailed") }}
        </p>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { MessagePlugin } from "tdesign-vue-next";
import { onBeforeUnmount, onMounted, ref, shallowRef } from "vue";
import { useI18n } from "vue-i18n";

import { Loader2Icon, XIcon } from "@lucide/vue";

import { Button } from "@/components/ui/button";

import {
  DEFAULT_ITEM_STYLE,
  loadExcalidraw,
  parseScene,
  renderSceneToSVG,
  serialiseScene,
  type ExcalidrawModule,
  type ExcalidrawScene,
} from "./excalidraw";
import { ReactIsland } from "./reactIsland";

const props = defineProps<{ source: string }>();
const emit = defineEmits<{ save: [payload: { scene: string; svg: string }]; close: [] }>();
const { t } = useI18n();

const mount = ref<HTMLElement | null>(null);
const status = ref<"loading" | "ready" | "failed">("loading");
const saving = ref(false);

/**
 * The React subtree, and the editor's own handle onto its canvas.
 *
 * Both are shallow refs holding foreign objects: making Excalidraw's element
 * tree reactive would have Vue walk a very large structure on every change for
 * no benefit, and would fight React for ownership of it.
 */
const island = shallowRef<ReactIsland | null>(null);
const module = shallowRef<ExcalidrawModule | null>(null);
const api = shallowRef<{
  getSceneElements: () => readonly unknown[];
  getAppState: () => Record<string, unknown>;
  getFiles: () => Record<string, unknown>;
} | null>(null);

const initial: ExcalidrawScene = parseScene(props.source);

onMounted(async () => {
  let loaded: ExcalidrawModule;
  let react: { createElement: (t: unknown, p: unknown) => unknown };
  let client: { createRoot: (el: Element) => { render: (e: unknown) => void; unmount: () => void } };
  try {
    // Imported here and nowhere else: this is the largest module in the
    // application and most readers of most pages never open a drawing.
    [loaded, react, client] = await Promise.all([
      loadExcalidraw(),
      import("react") as unknown as Promise<{ createElement: (t: unknown, p: unknown) => unknown }>,
      import("react-dom/client") as unknown as Promise<{
        createRoot: (el: Element) => { render: (e: unknown) => void; unmount: () => void };
      }>,
    ]);
  } catch {
    status.value = "failed";
    return;
  }

  // The component may already be gone: the import takes as long as it takes,
  // and nothing stops somebody closing the dialogue meanwhile.
  if (!mount.value) {
    status.value = "failed";
    return;
  }

  module.value = loaded;
  const created = new ReactIsland((container) => client.createRoot(container));
  island.value = created;
  created.render(
    mount.value,
    react.createElement(loaded.Excalidraw, {
      initialData: {
        elements: initial.elements,
        // The drawing wins where it has an opinion: one that was saved in its
        // own style reopens in that style, and only what it is silent about
        // falls back to the clean defaults. A new drawing is silent about all
        // of it, which is how it starts out looking like a diagram rather than
        // a doodle.
        appState: { ...DEFAULT_ITEM_STYLE, ...initial.appState },
        files: initial.files,
      },
      excalidrawAPI: (instance: typeof api.value) => {
        api.value = instance;
      },
      langCode: navigator.language?.startsWith("zh") ? "zh-CN" : "en",
      theme: isDark() ? "dark" : "light",
    }),
  );
  status.value = "ready";
});

/**
 * Tears the React subtree down with the dialogue.
 *
 * Without this the whole element tree, its listeners and everything they
 * closed over would stay alive for as long as the page is open, once per
 * drawing anybody opened. ReactIsland.destroy also makes a late import
 * harmless: if the module resolves after this ran, nothing is mounted.
 */
onBeforeUnmount(() => {
  island.value?.destroy();
  island.value = null;
  api.value = null;
  module.value = null;
});

function isDark(): boolean {
  return (
    document.documentElement.getAttribute("theme-mode") === "dark" ||
    document.documentElement.classList.contains("dark")
  );
}

async function save() {
  const editor = api.value;
  const loaded = module.value;
  if (!editor || !loaded || saving.value) return;
  saving.value = true;
  try {
    const elements = editor.getSceneElements();
    const appState = editor.getAppState();
    const files = editor.getFiles();
    const scene = serialiseScene(elements, appState, files);
    // The rendering is produced by the editor itself, so it looks exactly like
    // what was being drawn, and it is the only thing a reader ever loads.
    const svg = await renderSceneToSVG(loaded, {
      ...parseScene(scene),
      elements: [...elements],
      files,
    });
    emit("save", { scene, svg });
  } catch (err) {
    void MessagePlugin.error((err as { message?: string })?.message || t("docs.media.diagramSaveFailed"));
  } finally {
    saving.value = false;
  }
}
</script>
