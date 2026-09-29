<script setup lang="ts"></script>
<template>
  <div class="flex flex-col items-center justify-center">
    <!--
      The old stylesheet sized a `.upload-img` class that this image never
      carried, so the illustration has always rendered at its intrinsic size;
      it is left unsized to keep the overlay looking the way it does.
    -->
    <img src="@/assets/img/upload-mask.svg" alt="" />
    <span
      class="text-primary mt-3 mb-4 inline-block font-[family-name:var(--app-font-family)] text-2xl leading-[26px] font-semibold"
    >
      {{ $t("file.upload") }}
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
