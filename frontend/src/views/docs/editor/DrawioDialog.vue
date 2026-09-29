<template>
  <Teleport to="body">
    <div
      class="bg-background fixed inset-0 z-3000 flex flex-col"
      role="dialog"
      :aria-label="t('docs.media.diagramEdit')"
    >
      <div
        class="text-foreground flex items-center justify-between border-b border-[var(--td-component-stroke)] px-3.5 py-2 text-sm"
      >
        <span>{{ t("docs.media.diagramEdit") }}</span>
        <button
          type="button"
          data-slot="drawio-close"
          class="text-muted-foreground hover:bg-accent inline-flex cursor-pointer rounded border-0 p-1 leading-0"
          :aria-label="t('common.close')"
          @click="emit('close')"
        >
          <XIcon class="size-4" />
        </button>
      </div>
      <iframe
        ref="frame"
        class="w-full flex-1 border-0"
        :src="frameSrc"
        :title="t('docs.media.diagramEdit')"
        referrerpolicy="no-referrer"
      />
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, inject, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

import { XIcon } from "@lucide/vue";

import { drawioFrameURL, DrawioSession, isFromEditor, originOf, parseMessage } from "./drawio";
import { DOCS_DIAGRAMS, type DiagramHost } from "./linkContext";

const props = defineProps<{ source: string }>();
const emit = defineEmits<{ save: [payload: { xml: string; svg: string }]; close: [] }>();
const { t } = useI18n();

const host = inject<DiagramHost | null>(DOCS_DIAGRAMS, null);
const frame = ref<HTMLIFrameElement | null>(null);
const session = new DrawioSession(props.source);

const base = computed(() => host?.drawioURL.value ?? "");
const frameSrc = computed(() => (base.value ? drawioFrameURL(base.value, isDark()) : "about:blank"));
const expectedOrigin = computed(() => originOf(base.value));

function isDark(): boolean {
  return (
    document.documentElement.getAttribute("theme-mode") === "dark" ||
    document.documentElement.classList.contains("dark")
  );
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
  if (!isFromEditor(event, expectedOrigin.value, frame.value)) return;
  const message = parseMessage(event.data);
  if (!message) return;

  const action = session.receive(message);
  switch (action.kind) {
    case "load":
    case "export": {
      const request = DrawioSession.request(action);
      if (request) frame.value?.contentWindow?.postMessage(request, expectedOrigin.value);
      break;
    }
    case "save":
      emit("save", { xml: action.xml, svg: action.svg });
      break;
    case "close":
      emit("close");
      break;
    default:
      break;
  }
}

function onKey(event: KeyboardEvent) {
  if (event.key === "Escape") emit("close");
}

onMounted(() => {
  window.addEventListener("message", onMessage);
  window.addEventListener("keydown", onKey);
});

// Both listeners are removed with the dialog. The iframe goes with the
// element, so nothing of the editor outlives this component.
onBeforeUnmount(() => {
  window.removeEventListener("message", onMessage);
  window.removeEventListener("keydown", onKey);
});
</script>
