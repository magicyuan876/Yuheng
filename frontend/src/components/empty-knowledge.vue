<script setup lang="ts"></script>
<template>
  <div class="flex flex-1 flex-col items-center justify-center">
    <img class="size-[162px]" src="@/assets/img/upload.svg" alt="" />
    <span
      class="text-placeholder mt-3 mb-4 font-[family-name:var(--app-font-family)] text-base leading-[26px] font-semibold"
    >
      {{ $t("knowledgeBase.emptyKnowledgeDragDrop") }}
    </span>
    <span
      class="w-[217px] text-center font-[family-name:var(--app-font-family)] text-xs font-normal text-[var(--td-text-color-disabled)]"
    >
      {{ $t("knowledgeBase.pdfDocFormat") }}
    </span>
    <span
      class="w-[217px] text-center font-[family-name:var(--app-font-family)] text-xs font-normal text-[var(--td-text-color-disabled)]"
    >
      {{ $t("knowledgeBase.textMarkdownFormat") }}
    </span>
  </div>
</template>
