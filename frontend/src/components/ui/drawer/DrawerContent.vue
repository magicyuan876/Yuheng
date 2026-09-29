<script lang="ts" setup>
import type { DrawerContentEmits, DrawerContentProps } from "reka-ui";
import type { HTMLAttributes } from "vue";
import { reactiveOmit } from "@vueuse/core";
import { DrawerContent, DrawerHandle, DrawerPortal, useForwardPropsEmits } from "reka-ui";
import { cn } from "@/lib/utils";
import DrawerOverlay from "./DrawerOverlay.vue";

defineOptions({
  inheritAttrs: false,
});

const props = defineProps<DrawerContentProps & { class?: HTMLAttributes["class"] }>();
const emits = defineEmits<DrawerContentEmits>();

const delegatedProps = reactiveOmit(props, "class");

const forwarded = useForwardPropsEmits(delegatedProps, emits);
</script>

<!--
  Side drawers carry no default max width: shadcn caps them at max-w-sm through
  a data-attribute variant, which outranks a consumer's plain `sm:max-w-none`
  and which tailwind-merge does not see as the same property, so every wide
  drawer in the application was silently held to 384px. Each side drawer here
  sets its own width.
-->
<template>
  <DrawerPortal>
    <DrawerOverlay />
    <DrawerContent
      data-slot="drawer-content"
      v-bind="{ ...$attrs, ...forwarded }"
      :class="
        cn(
          'bg-popover text-popover-foreground group/drawer-content fixed z-[2500] flex h-auto flex-col text-sm data-[swipe-direction=down]:inset-x-0 data-[swipe-direction=down]:bottom-0 data-[swipe-direction=down]:mt-24 data-[swipe-direction=down]:max-h-[80vh] data-[swipe-direction=down]:rounded-t-xl data-[swipe-direction=down]:border-t data-[swipe-direction=left]:inset-y-0 data-[swipe-direction=left]:left-0 data-[swipe-direction=left]:w-3/4 data-[swipe-direction=left]:rounded-r-xl data-[swipe-direction=left]:border-r data-[swipe-direction=right]:inset-y-0 data-[swipe-direction=right]:right-0 data-[swipe-direction=right]:w-3/4 data-[swipe-direction=right]:rounded-l-xl data-[swipe-direction=right]:border-l data-[swipe-direction=up]:inset-x-0 data-[swipe-direction=up]:top-0 data-[swipe-direction=up]:mb-24 data-[swipe-direction=up]:max-h-[80vh] data-[swipe-direction=up]:rounded-b-xl data-[swipe-direction=up]:border-b',
          'transform-[translate3d(var(--drawer-swipe-movement-x,0px),var(--drawer-swipe-movement-y,0px),0)] will-change-transform',
          'transition-transform duration-500 ease-[cubic-bezier(0.32,0.72,0,1)] data-swiping:duration-0 data-swiping:select-none',
          'data-[state=open]:animate-in data-[state=closed]:animate-out',
          'data-[swipe-direction=down]:data-[state=open]:slide-in-from-bottom data-[swipe-direction=down]:data-[state=closed]:slide-out-to-bottom',
          'data-[swipe-direction=up]:data-[state=open]:slide-in-from-top data-[swipe-direction=up]:data-[state=closed]:slide-out-to-top',
          'data-[swipe-direction=left]:data-[state=open]:slide-in-from-left data-[swipe-direction=left]:data-[state=closed]:slide-out-to-left',
          'data-[swipe-direction=right]:data-[state=open]:slide-in-from-right data-[swipe-direction=right]:data-[state=closed]:slide-out-to-right',
          props.class,
        )
      "
    >
      <DrawerHandle
        class="bg-muted mx-auto mt-4 hidden h-1 w-[100px] shrink-0 rounded-full group-data-[swipe-direction=down]/drawer-content:block"
      />
      <slot />
    </DrawerContent>
  </DrawerPortal>
</template>
