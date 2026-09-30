<template>
  <!-- The buttons in the top right corner of every platform page: the
       knowledge to-do and the invitation bell. Each renders only while it has
       something to show, so the corner is usually empty.

       z-100 sits far below the drawers (2501), so a page's side drawer covers
       the corner, and above ordinary page content (0-10), so list cards never
       do. -->
  <div class="fixed top-3 right-4 z-100 flex items-center gap-2" data-testid="global-corner-actions">
    <KnowledgeTodo />
    <GlobalInvitationBell />
  </div>
</template>

<script setup lang="ts">
import GlobalInvitationBell from "@/components/GlobalInvitationBell.vue";
import KnowledgeTodo from "@/components/findings/KnowledgeTodo.vue";
</script>
