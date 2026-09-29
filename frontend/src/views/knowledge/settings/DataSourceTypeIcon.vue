<script setup lang="ts">
import { datasourceIconMap } from "./datasourceIcons";

withDefaults(
  defineProps<{
    type: string;
    size?: number;
    /** inline: 类型选择等小尺寸场景；badge: 嵌入 ds-card__badge 等父级徽章容器 */
    variant?: "inline" | "badge";
  }>(),
  {
    size: 20,
    variant: "inline",
  },
);

const iconMap = datasourceIconMap;

function fallbackText(type: string) {
  switch (type) {
    case "feishu":
      return "F";
    case "lark":
      return "L";
    case "notion":
      return "N";
    case "yuque":
      return "Y";
    case "ima":
      return "I";
    default:
      return type.slice(0, 1).toUpperCase() || "?";
  }
}
</script>

<template>
  <span
    class="inline-flex shrink-0 items-center justify-center overflow-hidden"
    :class="variant === 'inline' ? 'rounded-md bg-[var(--td-bg-color-component)]' : 'h-full w-full bg-transparent'"
    :style="variant === 'inline' ? { width: `${size}px`, height: `${size}px` } : undefined"
  >
    <img
      v-if="iconMap[type]"
      :src="iconMap[type]"
      :alt="type"
      class="block object-contain"
      :class="variant === 'badge' ? 'h-6 w-6' : ''"
      :style="variant === 'inline' ? { width: `${size}px`, height: `${size}px` } : undefined"
    />
    <span
      v-else
      class="font-semibold"
      :class="variant === 'badge' ? 'text-[15px] tracking-[0.02em]' : 'text-placeholder text-[11px]'"
      >{{ fallbackText(type) }}</span
    >
  </span>
</template>
