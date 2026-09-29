<template>
  <NodeViewWrapper
    class="my-3 flex flex-col"
    :class="ALIGN_CLASS[align] ?? ALIGN_CLASS.center"
    :data-drag-handle="editor.isEditable ? '' : undefined"
  >
    <figure class="group relative m-0 max-w-full leading-none" :style="frameStyle">
      <img
        v-if="src"
        class="block h-auto max-w-full rounded-md"
        :class="selected ? 'outline-primary outline-2 outline-offset-2' : ''"
        :src="src"
        :alt="node.attrs.alt || ''"
        :title="node.attrs.title || undefined"
        loading="lazy"
        @load="onLoad"
      />

      <div
        v-if="editor.isEditable"
        class="absolute top-1.5 right-1.5 hidden gap-0.5 rounded-md bg-black/55 p-0.5 group-hover:flex"
      >
        <Tooltip v-for="option in alignments" :key="option.value">
          <TooltipTrigger as-child>
            <button
              type="button"
              data-slot="image-tool"
              class="cursor-pointer rounded border-0 px-1 py-[3px] leading-0 text-white hover:bg-white/24"
              :class="align === option.value ? 'bg-white/24' : ''"
              @click="setAlign(option.value)"
            >
              <component :is="option.icon" class="size-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent>{{ t(option.label) }}</TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger as-child>
            <button
              type="button"
              data-slot="image-tool"
              class="cursor-pointer rounded border-0 px-1 py-[3px] leading-0 text-white hover:bg-white/24"
              @click="editAlt"
            >
              <PencilIcon class="size-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent>{{ t("docs.attachments.imageAlt") }}</TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger as-child>
            <button
              type="button"
              data-slot="image-tool"
              class="cursor-pointer rounded border-0 px-1 py-[3px] leading-0 text-white hover:bg-white/24"
              @click="deleteNode"
            >
              <Trash2Icon class="size-3.5" />
            </button>
          </TooltipTrigger>
          <TooltipContent>{{ t("docs.attachments.remove") }}</TooltipContent>
        </Tooltip>
      </div>

      <!-- Dragging the right edge sets an explicit width; the schema bounds
           it to 16-8192, and the server rejects anything outside that. -->
      <span
        v-if="editor.isEditable"
        class="bg-primary absolute top-1/2 -right-1 h-9 w-2 -translate-y-1/2 cursor-ew-resize rounded opacity-0 group-hover:opacity-85"
        role="separator"
        aria-orientation="vertical"
        @pointerdown="startResize"
      />
    </figure>
    <figcaption v-if="node.attrs.alt" class="text-placeholder mt-1.5 text-[12.5px]">{{ node.attrs.alt }}</figcaption>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { NodeViewWrapper, type NodeViewProps } from "@tiptap/vue-3";
import { computed, ref, type Component } from "vue";
import { useI18n } from "vue-i18n";

import {
  AlignCenterVerticalIcon,
  AlignEndVerticalIcon,
  AlignStartVerticalIcon,
  PencilIcon,
  Trash2Icon,
} from "@lucide/vue";

import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

import { useAttachmentUrl } from "./useAttachmentUrl";

const props = defineProps<NodeViewProps>();
const { t } = useI18n();

const MIN_WIDTH = 16;
const MAX_WIDTH = 8192;

const ALIGN_CLASS: Record<string, string> = {
  left: "items-start",
  center: "items-center",
  right: "items-end",
};

const alignments: Array<{ value: "left" | "center" | "right"; icon: Component; label: string }> = [
  { value: "left", icon: AlignStartVerticalIcon, label: "docs.attachments.alignLeft" },
  { value: "center", icon: AlignCenterVerticalIcon, label: "docs.attachments.alignCenter" },
  { value: "right", icon: AlignEndVerticalIcon, label: "docs.attachments.alignRight" },
];

const align = computed(() => String(props.node.attrs.align ?? "center"));

/** Our own files are addressed by id and fetched with the token as a blob
 * URL, because a bare <img> cannot send Authorization; an external image
 * keeps its own URL. */
const attachmentId = computed(() => (props.node.attrs.attachmentId as string | null) ?? null);
const blobUrl = useAttachmentUrl(attachmentId);
const src = computed(() => (attachmentId.value ? blobUrl.value : String(props.node.attrs.src ?? "")));

const frameStyle = computed(() => {
  const width = Number(props.node.attrs.width ?? 0);
  return width > 0 ? { width: `${width}px` } : {};
});

function setAlign(value: "left" | "center" | "right") {
  props.updateAttributes({ align: value });
}

function editAlt() {
  const next = window.prompt(t("docs.attachments.imageAltPrompt"), String(props.node.attrs.alt ?? ""));
  if (next === null) return;
  props.updateAttributes({ alt: next.trim() || null });
}

/** Records the natural size the first time the browser knows it, so the
 * srcset can stop offering renderings larger than the original. */
function onLoad(event: Event) {
  if (props.node.attrs.width) return;
  const img = event.target as HTMLImageElement;
  if (img.naturalWidth > 0) {
    props.updateAttributes({ width: img.naturalWidth, height: img.naturalHeight });
  }
}

const resizing = ref(false);

function startResize(event: PointerEvent) {
  if (resizing.value) return;
  const frame = (event.currentTarget as HTMLElement).parentElement;
  if (!frame) return;
  resizing.value = true;
  const startX = event.clientX;
  const startWidth = frame.getBoundingClientRect().width;
  const ratio = Number(props.node.attrs.height ?? 0) / Number(props.node.attrs.width ?? 1);

  const move = (e: PointerEvent) => {
    const width = Math.round(Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, startWidth + (e.clientX - startX))));
    frame.style.width = `${width}px`;
  };
  const up = (e: PointerEvent) => {
    window.removeEventListener("pointermove", move);
    window.removeEventListener("pointerup", up);
    resizing.value = false;
    const width = Math.round(Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, startWidth + (e.clientX - startX))));
    const height = ratio > 0 ? Math.round(width * ratio) : null;
    props.updateAttributes({ width, height });
  };
  window.addEventListener("pointermove", move);
  window.addEventListener("pointerup", up);
  event.preventDefault();
}
</script>
