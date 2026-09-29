<template>
  <span
    class="inline-flex shrink-0 items-center justify-center"
    :class="[sizeClass, tintClass, { 'sandbox-badge--mono': logo?.mode === 'mono' }]"
    :style="badgeStyle"
    :aria-hidden="true"
  >
    <component :is="iconComponent" v-if="!logo" :class="iconSizeClass" />
  </span>
</template>

<script setup lang="ts">
import { computed, type Component } from "vue";

import { CloudIcon, ServerIcon, CircleMinusIcon } from "@lucide/vue";

import { providerLogo } from "@/views/settings/providerLogos";

// The sandbox list and the config drawer must show the same mark for a backend,
// so both render this badge instead of each mapping types to icons themselves.
// Vendors we ship a logo for (Docker) win over the generic lucide glyphs.
const props = withDefaults(
  defineProps<{
    type: string;
    size?: "sm" | "md";
  }>(),
  { size: "md" },
);

const logo = computed(() => providerLogo("sandbox", props.type));

const iconComponent = computed<Component>(() => {
  if (props.type === "cube" || props.type === "local") return ServerIcon;
  if (props.type === "disabled") return CircleMinusIcon;
  return CloudIcon;
});

// The size keeps a named class as well: the mono-logo mask below is sized by it.
const sizeClass = computed(() =>
  props.size === "sm" ? "sandbox-badge--sm h-[26px] w-[26px] rounded-[7px]" : "sandbox-badge--md h-9 w-9 rounded-[9px]",
);

const iconSizeClass = computed(() => (props.size === "sm" ? "size-[14px]" : "size-[17px]"));

const tintClass = computed(() => {
  switch (props.type) {
    case "e2b":
      return "bg-[rgba(98,53,187,0.1)] text-[#6235bb]";
    case "docker":
      return "bg-[rgba(29,99,237,0.1)] text-[#1d63ed]";
    case "local":
      return "bg-[rgba(17,128,83,0.1)] text-[#118053]";
    default:
      return "bg-[rgba(0,82,217,0.1)] text-[#0052d9]";
  }
});

const badgeStyle = computed((): Record<string, string> =>
  logo.value?.mode === "mono" ? { "--logo-url": `url("${logo.value.url}")` } : {},
);
</script>

<style scoped>
/* Vendor logos are colourless masks tinted with currentColor; mask sizing has
   no Tailwind utility, so the pseudo-element stays in CSS. */
.sandbox-badge--mono::before {
  content: "";
  background-color: currentColor;
  -webkit-mask-image: var(--logo-url);
  -webkit-mask-position: center;
  -webkit-mask-repeat: no-repeat;
  -webkit-mask-size: contain;
  mask-image: var(--logo-url);
  mask-position: center;
  mask-repeat: no-repeat;
  mask-size: contain;
}

.sandbox-badge--md.sandbox-badge--mono::before {
  width: 22px;
  height: 22px;
}

.sandbox-badge--sm.sandbox-badge--mono::before {
  width: 16px;
  height: 16px;
}
</style>
