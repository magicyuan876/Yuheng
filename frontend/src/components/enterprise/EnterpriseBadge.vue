<script setup lang="ts">
import { Badge } from "@/components/ui/badge";
</script>

<template>
  <!-- A quiet outline tag, not a coloured call for attention: it only labels what is Enterprise. -->
  <Badge variant="outline" class="text-muted-foreground">{{ $t("enterprise.badge") }}</Badge>
</template>
