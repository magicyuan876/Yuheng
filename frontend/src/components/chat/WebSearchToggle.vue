<template>
  <Tooltip v-if="available">
    <TooltipTrigger as-child>
      <button
        type="button"
        data-slot="web-search-toggle"
        :aria-pressed="enabled"
        :aria-label="t('input.webSearch')"
        :class="[
          'relative flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-[6px] transition-[background,color] duration-[120ms] select-none',
          enabled
            ? 'bg-[rgba(16,185,129,0.1)] text-[#07c05f]'
            : 'text-muted-foreground hover:text-foreground hover:bg-[var(--td-bg-color-secondarycontainer-hover,#f0f0f0)]',
        ]"
        @click.stop="toggle"
      >
        <GlobeIcon class="size-[18px]" :stroke-width="1.8" />
      </button>
    </TooltipTrigger>
    <!-- The input bar's hints are the light variant: a card-coloured bubble with a hairline border. -->
    <TooltipContent
      side="top"
      class="border-border bg-popover text-popover-foreground [&>span>svg]:bg-popover [&>span>svg]:fill-popover border-[0.5px] border-solid shadow-[var(--td-shadow-2)]"
    >
      <span>{{ enabled ? t("input.webSearchOn") : t("input.webSearchOff") }}</span>
    </TooltipContent>
  </Tooltip>
</template>

<script setup lang="ts">
import { onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { GlobeIcon } from "@lucide/vue";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useWebSearchToggle } from "@/composables/useWebSearchToggle";

/**
 * The chat input's web-search switch. It renders nothing until web search is
 * available in this workspace, so there is never a switch that does nothing.
 */
const { t } = useI18n();
const { available, enabled, ensureLoaded } = useWebSearchToggle();

function toggle() {
  enabled.value = !enabled.value;
}

onMounted(() => {
  void ensureLoaded();
});
</script>
