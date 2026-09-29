<template>
  <NodeViewWrapper class="my-3 flex flex-col" :class="ALIGN_CLASS[align] ?? ALIGN_CLASS.center">
    <!-- The reading state, and the only state an export or a share page ever
         reaches: the rendered preview, with no editor loaded at all. -->
    <figure class="group relative m-0 max-w-full leading-none" :style="frameStyle">
      <img
        v-if="previewSrc"
        class="bg-card block h-auto max-w-full rounded-md"
        :class="selected ? 'outline-primary outline-2 outline-offset-2' : ''"
        :src="previewSrc"
        :alt="t('docs.media.diagram')"
        loading="lazy"
      />
      <p
        v-else
        class="text-placeholder m-0 rounded-[8px] border border-dashed border-[var(--td-component-stroke)] p-[24px_32px] text-[13px] leading-normal"
      >
        {{ t("docs.media.diagramNoPreview") }}
      </p>

      <div
        v-if="editor.isEditable"
        class="absolute top-1.5 right-1.5 hidden gap-0.5 rounded-md bg-black/55 p-0.5 group-hover:flex"
      >
        <Tooltip v-if="canEdit">
          <TooltipTrigger as-child>
            <button
              type="button"
              data-slot="diagram-tool"
              class="cursor-pointer rounded border-0 px-1 py-[3px] leading-0 text-white hover:bg-white/24"
              @click="openEditor"
            >
              <PencilIcon class="size-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent>{{ t("docs.media.diagramEdit") }}</TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger as-child>
            <button
              type="button"
              data-slot="diagram-tool"
              class="cursor-pointer rounded border-0 px-1 py-[3px] leading-0 text-white hover:bg-white/24"
              @click="deleteNode"
            >
              <Trash2Icon class="size-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent>{{ t("docs.attachments.remove") }}</TooltipContent>
        </Tooltip>
      </div>
    </figure>

    <p v-if="editor.isEditable && !canEdit" class="text-placeholder mt-1 mb-0 text-xs">
      {{ kind === "drawio" ? t("docs.media.drawioUnavailable") : t("docs.media.excalidrawUnavailable") }}
    </p>

    <DrawioDialog v-if="kind === 'drawio' && editing" :source="source" @save="onDrawioSaved" @close="editing = false" />
    <ExcalidrawDialog
      v-if="kind === 'excalidraw' && editing"
      :source="source"
      @save="onExcalidrawSaved"
      @close="editing = false"
    />
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { MessagePlugin } from "tdesign-vue-next";
import { computed, inject, ref } from "vue";
import { useI18n } from "vue-i18n";

import { PencilIcon, Trash2Icon } from "@lucide/vue";

import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

import { useAttachmentUrl } from "./useAttachmentUrl";
import DrawioDialog from "./DrawioDialog.vue";
import { EMPTY_DRAWIO_XML } from "./drawio";
import { emptyScene } from "./excalidraw";
import ExcalidrawDialog from "./ExcalidrawDialog.vue";
import { DOCS_DIAGRAMS, type DiagramHost } from "./linkContext";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();

const host = inject<DiagramHost | null>(DOCS_DIAGRAMS, null);

const ALIGN_CLASS: Record<string, string> = {
  left: "items-start",
  center: "items-center",
  right: "items-end",
};

const kind = computed(() => props.node.type.name as "drawio" | "excalidraw");
const align = computed(() => String(props.node.attrs.align ?? "center"));

/**
 * The reader only ever needs the preview. That is what makes a saved diagram
 * show in an export, a share page and a read-only view without any editor
 * being loaded — the source attachment is fetched only when somebody actually
 * opens the editor.
 */
const previewId = computed(() => (props.node.attrs.previewAttachmentId as string | null) ?? null);
const previewSrc = useAttachmentUrl(previewId);

// draw.io needs a self-hosted editor to be configured; Excalidraw ships with
// the application and only needs its module, which is loaded on demand.
const canEdit = computed(() => kind.value === "excalidraw" || (kind.value === "drawio" && !!host?.drawioURL.value));

const frameStyle = computed(() => {
  const width = Number(props.node.attrs.width ?? 0);
  return width > 0 ? { width: `${width}px` } : {};
});

const editing = ref(false);
const source = ref("");

async function openEditor() {
  if (!canEdit.value || !host) return;
  const id = props.node.attrs.attachmentId as string | null;
  if (id) {
    try {
      source.value = await host.load(id);
    } catch {
      // Opening an editor on the wrong drawing would be worse than refusing:
      // saving would then overwrite the real one with a blank canvas.
      void MessagePlugin.error(t("docs.media.diagramLoadFailed"));
      return;
    }
  } else {
    source.value = kind.value === "drawio" ? EMPTY_DRAWIO_XML : JSON.stringify(emptyScene());
  }
  editing.value = true;
}

function onDrawioSaved(payload: { xml: string; svg: string }) {
  void store(payload.xml, payload.svg);
}

function onExcalidrawSaved(payload: { scene: string; svg: string }) {
  void store(payload.scene, payload.svg);
}

async function store(source: string, svg: string) {
  editing.value = false;
  if (!host) return;
  try {
    const stored = await host.save(kind.value, source, svg, "diagram");
    props.updateAttributes({
      attachmentId: stored.attachmentId,
      previewAttachmentId: stored.previewAttachmentId,
    });
  } catch (err) {
    void MessagePlugin.error((err as { message?: string })?.message || t("docs.media.diagramSaveFailed"));
  }
}
</script>
