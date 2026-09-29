<template>
  <!--
    The wrapper shrinks with its flex row (a long tag truncates instead of
    pushing the card wider), which is why it is inline-block with min-width 0
    rather than a plain inline span.
  -->
  <div
    ref="wrapperRef"
    class="relative inline-block max-w-full min-w-0 flex-[0_1_auto] overflow-visible"
    @mouseenter="handleMouseEnter"
    @mouseleave="handleMouseLeave"
  >
    <slot />
    <Teleport to="body">
      <Transition
        enter-active-class="transition-opacity duration-150 ease-in-out"
        leave-active-class="transition-opacity duration-150 ease-in-out"
        enter-from-class="opacity-0"
        leave-to-class="opacity-0"
      >
        <div
          v-if="showTooltip && content"
          ref="tooltipRef"
          class="border-border bg-card text-foreground pointer-events-none fixed z-[9999] max-w-[320px] min-w-[100px] rounded-md border px-3.5 py-2.5 text-xs leading-[1.6] font-normal break-words shadow-[0_0_8px_0_rgba(0,0,0,0.08)]"
          :class="tooltipClass"
          :style="tooltipStyle"
        >
          <!--
            The arrow is two stacked CSS triangles: the outer one in the
            border colour, the inner one a pixel closer in the surface colour,
            so the tooltip's 1px border appears to run around the point.
          -->
          <span class="absolute size-0 border-[5px] border-transparent" :class="arrowClasses.outer" />
          <span class="absolute size-0 border-[5px] border-transparent" :class="arrowClasses.inner" />
          <div class="text-foreground text-xs leading-[1.6] font-normal break-words whitespace-pre-wrap">
            {{ content }}
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onUnmounted, watch } from "vue";
import { getRootZoom, rectToCssPx, cssViewportSize } from "@/utils/zoom";

const props = defineProps<{
  content: string;
  placement?: "top" | "bottom" | "left" | "right";
  type?: "answer" | "similar" | "negative";
}>();

const showTooltip = ref(false);
const tooltipRef = ref<HTMLElement | null>(null);
const wrapperRef = ref<HTMLElement | null>(null);
const tooltipStyle = ref<{ top: string; left: string }>({ top: "0px", left: "0px" });

const tooltipClass = computed(() => {
  return {
    [`tooltip-${props.type || "answer"}`]: true,
    [`placement-${props.placement || "top"}`]: true,
  };
});

// Per-placement arrow position and colour. Each side points the triangle
// towards the tag, so the coloured border edge differs by placement.
const arrowClasses = computed(() => {
  switch (props.placement || "top") {
    case "bottom":
      return {
        outer: "-top-2.5 left-1/2 -translate-x-1/2 border-b-border",
        inner: "-top-[9px] left-1/2 -translate-x-1/2 border-b-card",
      };
    case "left":
      return {
        outer: "top-1/2 -right-2.5 -translate-y-1/2 border-l-border",
        inner: "top-1/2 -right-[9px] -translate-y-1/2 border-l-card",
      };
    case "right":
      return {
        outer: "top-1/2 -left-2.5 -translate-y-1/2 border-r-border",
        inner: "top-1/2 -left-[9px] -translate-y-1/2 border-r-card",
      };
    default:
      return {
        outer: "-bottom-2.5 left-1/2 -translate-x-1/2 border-t-border",
        inner: "-bottom-[9px] left-1/2 -translate-x-1/2 border-t-card",
      };
  }
});

const updatePosition = async () => {
  if (!wrapperRef.value || !tooltipRef.value) return;

  await nextTick();

  // 再次检查，确保DOM已渲染
  if (!tooltipRef.value) return;

  // The tooltip is `position: fixed` and rendered under the root `zoom`.
  // Normalize the visual-pixel rects so subsequent arithmetic stays in CSS px.
  const zoom = getRootZoom();
  const rect = rectToCssPx(wrapperRef.value.getBoundingClientRect(), zoom);
  const tooltipRect = rectToCssPx(tooltipRef.value.getBoundingClientRect(), zoom);
  const { width: vw, height: vh } = cssViewportSize(zoom);
  const placement = props.placement || "top";

  let top = 0;
  let left = 0;

  switch (placement) {
    case "top":
      top = rect.top - tooltipRect.height - 8;
      left = rect.left + rect.width / 2 - tooltipRect.width / 2;
      break;
    case "bottom":
      top = rect.bottom + 8;
      left = rect.left + rect.width / 2 - tooltipRect.width / 2;
      break;
    case "left":
      top = rect.top + rect.height / 2 - tooltipRect.height / 2;
      left = rect.left - tooltipRect.width - 8;
      break;
    case "right":
      top = rect.top + rect.height / 2 - tooltipRect.height / 2;
      left = rect.right + 8;
      break;
  }

  // 边界检测
  const padding = 8;
  if (left < padding) left = padding;
  if (left + tooltipRect.width > vw - padding) {
    left = vw - tooltipRect.width - padding;
  }
  if (top < padding) {
    // 如果上方空间不足，改为下方显示
    if (placement === "top") {
      top = rect.bottom + 8;
    } else {
      top = padding;
    }
  }
  if (top + tooltipRect.height > vh - padding) {
    top = vh - tooltipRect.height - padding;
  }

  tooltipStyle.value = {
    top: `${top}px`,
    left: `${left}px`,
  };
};

const handleMouseEnter = () => {
  showTooltip.value = true;
  nextTick(() => {
    updatePosition();
  });
};

const handleMouseLeave = () => {
  showTooltip.value = false;
};

onMounted(() => {
  window.addEventListener("scroll", updatePosition, true);
  window.addEventListener("resize", updatePosition);
});

onUnmounted(() => {
  window.removeEventListener("scroll", updatePosition, true);
  window.removeEventListener("resize", updatePosition);
});

watch(showTooltip, (newVal) => {
  if (newVal) {
    nextTick(() => {
      updatePosition();
    });
  }
});
</script>
