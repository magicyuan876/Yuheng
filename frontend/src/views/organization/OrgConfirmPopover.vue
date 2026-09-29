<template>
  <!--
    A confirm-in-place popover with a tooltip on its trigger, standing in for
    TDesign's t-popconfirm wrapped around a t-tooltip. The organization
    settings screen uses it for every destructive row action (remove a
    member, reject a request, withdraw a share).

    The tooltip wraps the popover trigger rather than the other way round:
    both triggers are `as-child`, and a tooltip nested inside a popover
    trigger would receive the trigger's attributes on a renderless root and
    drop them. The floating parts are raised above the settings overlay
    (z-index 2000), which the default z-50 would sit beneath.
  -->
  <Popover v-model:open="open">
    <Tooltip>
      <TooltipTrigger as-child>
        <PopoverTrigger as-child>
          <slot />
        </PopoverTrigger>
      </TooltipTrigger>
      <TooltipContent side="top" class="z-[3050]">{{ tooltip }}</TooltipContent>
    </Tooltip>
    <PopoverContent :side="side" class="z-[3050] w-auto max-w-[320px] gap-3 p-4">
      <div class="flex items-start gap-2">
        <CircleAlertIcon class="text-warning mt-0.5 size-4 shrink-0" />
        <p class="text-foreground m-0 text-sm leading-normal">{{ content }}</p>
      </div>
      <div class="flex justify-end gap-2">
        <Button variant="outline" size="sm" @click="open = false">{{ cancelText }}</Button>
        <Button variant="destructive" size="sm" @click="onConfirm">{{ confirmText }}</Button>
      </div>
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { CircleAlertIcon } from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

withDefaults(
  defineProps<{
    // The question asked in the popover.
    content: string;
    // The tooltip shown on the trigger while the popover is closed.
    tooltip: string;
    confirmText: string;
    cancelText: string;
    side?: "top" | "right" | "bottom" | "left";
  }>(),
  { side: "left" },
);

const emit = defineEmits<{
  (e: "confirm"): void;
}>();

const open = ref(false);

// The popover closes before the parent runs the action, as t-popconfirm did,
// so a slow request does not leave the question on screen.
function onConfirm() {
  open.value = false;
  emit("confirm");
}
</script>
