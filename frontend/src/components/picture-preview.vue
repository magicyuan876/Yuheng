<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

import { RotateCcwIcon, XIcon, ZoomInIcon, ZoomOutIcon } from "@lucide/vue";

const props = defineProps<{ reviewImg: boolean; reviewUrl: string }>();
const emit = defineEmits(["closePreImg"]);

const { t } = useI18n();

// The old t-image-viewer let the reader zoom the picture; the scale is kept
// here within the same bounds a reader would find usable (a tenth to ten times).
const MIN_SCALE = 0.1;
const MAX_SCALE = 10;
const scale = ref(1);

const close = () => {
  emit("closePreImg");
};

function zoom(factor: number) {
  scale.value = Math.min(MAX_SCALE, Math.max(MIN_SCALE, scale.value * factor));
}

function resetZoom() {
  scale.value = 1;
}

function onWheel(event: WheelEvent) {
  zoom(event.deltaY < 0 ? 1.1 : 1 / 1.1);
}

function onKey(event: KeyboardEvent) {
  if (event.key === "Escape" && props.reviewImg) close();
}

onMounted(() => window.addEventListener("keydown", onKey));
onBeforeUnmount(() => {
  window.removeEventListener("keydown", onKey);
  document.body.style.overflow = "";
});

// The page behind stays still while the viewer is open, and every opening
// starts at the natural size.
watch(
  () => props.reviewImg,
  (open) => {
    document.body.style.overflow = open ? "hidden" : "";
    if (open) resetZoom();
  },
);
</script>

<template>
  <Teleport to="body">
    <!-- The backdrop closes the viewer; the image and the toolbar swallow their clicks, so
         dragging, selecting and zooming never dismiss it. -->
    <div
      v-if="reviewImg"
      data-slot="image-viewer"
      class="fixed inset-0 z-[3000] flex items-center justify-center overflow-hidden bg-black/60"
      @click="close"
      @wheel.prevent="onWheel"
    >
      <button
        type="button"
        class="absolute top-4 right-4 inline-flex size-9 cursor-pointer items-center justify-center rounded-full bg-black/40 text-white transition-colors hover:bg-black/60"
        :aria-label="t('common.close')"
        :title="t('common.close')"
        @click.stop="close"
      >
        <XIcon class="size-5" />
      </button>
      <img
        v-if="reviewUrl"
        :src="reviewUrl"
        class="max-h-[90vh] max-w-[90vw] transition-transform duration-150"
        :style="{ transform: `scale(${scale})` }"
        alt=""
        @click.stop
      />
      <div
        class="absolute bottom-6 left-1/2 flex -translate-x-1/2 items-center gap-1 rounded-full bg-black/50 px-2 py-1 text-white"
        @click.stop
      >
        <button
          type="button"
          class="inline-flex size-8 cursor-pointer items-center justify-center rounded-full hover:bg-white/15"
          :aria-label="t('mermaid.zoomOut')"
          :title="t('mermaid.zoomOut')"
          @click="zoom(1 / 1.25)"
        >
          <ZoomOutIcon class="size-4" />
        </button>
        <span class="min-w-12 text-center text-xs tabular-nums">{{ Math.round(scale * 100) }}%</span>
        <button
          type="button"
          class="inline-flex size-8 cursor-pointer items-center justify-center rounded-full hover:bg-white/15"
          :aria-label="t('mermaid.zoomIn')"
          :title="t('mermaid.zoomIn')"
          @click="zoom(1.25)"
        >
          <ZoomInIcon class="size-4" />
        </button>
        <button
          type="button"
          class="inline-flex size-8 cursor-pointer items-center justify-center rounded-full hover:bg-white/15"
          :aria-label="t('mermaid.reset')"
          :title="t('mermaid.reset')"
          @click="resetZoom"
        >
          <RotateCcwIcon class="size-4" />
        </button>
      </div>
    </div>
  </Teleport>
</template>
