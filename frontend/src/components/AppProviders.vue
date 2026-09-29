<script setup lang="ts">
/*
 * The context providers the new stack's components need, mounted once at the
 * root. Reka's Tooltip resolves its provider by injection and throws when none
 * is found, so without this every screen had to wrap its own tooltips — and
 * one that forgot failed to render at all. App.vue keeps TDesign's
 * config-provider beside it while MessagePlugin and DialogPlugin remain; this
 * file is where the new stack's half of that root lives.
 */
import { TooltipProvider } from "@/components/ui/tooltip";
</script>

<template>
  <!-- TDesign's tooltips opened after ~300ms; Reka's default is 700ms. -->
  <TooltipProvider :delay-duration="300">
    <slot />
  </TooltipProvider>
</template>
