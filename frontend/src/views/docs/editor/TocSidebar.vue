<template>
  <!-- Sticky beside the document, and scrolling on its own once the outline
       is longer than the screen. -->
  <nav
    v-if="entries.length"
    class="border-border sticky top-0 max-h-[calc(100vh-120px)] w-[200px] flex-none self-start overflow-y-auto border-l py-1 pl-4"
    :aria-label="t('docs.pages.toc')"
  >
    <div class="text-placeholder mb-2 text-xs font-semibold tracking-[0.04em] uppercase">
      {{ t("docs.pages.toc") }}
    </div>
    <ul class="m-0 list-none p-0">
      <li
        v-for="entry in entries"
        :key="entry.pos"
        class="my-0.5"
        :style="{ paddingLeft: (entry.level - 1) * 12 + 'px' }"
      >
        <button
          type="button"
          data-slot="toc-link"
          class="text-muted-foreground hover:bg-accent hover:text-foreground block w-full cursor-pointer truncate rounded-[4px] px-1.5 py-[3px] text-left text-[12.5px] leading-[18px]"
          @click="emit('select', entry.pos)"
        >
          {{ entry.text || t("docs.tree.untitled") }}
        </button>
      </li>
    </ul>
  </nav>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";

import type { TocEntry } from "./toc";

defineProps<{ entries: TocEntry[] }>();
const emit = defineEmits<{ select: [pos: number] }>();

const { t } = useI18n();
</script>
