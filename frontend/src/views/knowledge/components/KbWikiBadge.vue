<template>
  <Tooltip>
    <TooltipTrigger as-child>
      <span
        class="text-muted-foreground inline-flex shrink-0 items-center justify-center"
        role="img"
        :aria-label="t('knowledgeList.features.wiki')"
      >
        <BookOpenTextIcon class="block size-[15px]" :stroke-width="1.75" aria-hidden="true" />
      </span>
    </TooltipTrigger>
    <TooltipContent side="top">{{ t("knowledgeList.features.wiki") }}</TooltipContent>
  </Tooltip>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";
import { BookOpenTextIcon } from "@lucide/vue";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

const { t } = useI18n();
</script>
