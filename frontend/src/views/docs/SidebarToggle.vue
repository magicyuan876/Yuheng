<script setup lang="ts">
// Folds the space's page tree away and brings it back.
//
// It lives in the main column rather than in the tree, because when the tree
// is folded there is nothing of it left to click. Mounted by the space
// layout, so it is there for every kind of main content — a page, the space
// home, search — and it owns the keyboard shortcut for the same reason.
import { computed, onBeforeUnmount, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { PanelLeftCloseIcon, PanelLeftOpenIcon } from "@lucide/vue";

import { Button } from "@/components/ui/button";
import { isToggleShortcut, useDocsSidebar } from "./useDocsSidebar";

const { t } = useI18n();
const { collapsed, toggle } = useDocsSidebar();

const label = computed(() => (collapsed.value ? t("docs.tree.expandSidebar") : t("docs.tree.collapseSidebar")));
const shortcut = /Mac|iPhone|iPad/.test(navigator.userAgent) ? "⌘ \\" : "Ctrl+\\";

function onKeydown(e: KeyboardEvent) {
  if (!isToggleShortcut(e)) return;
  e.preventDefault();
  toggle();
}

onMounted(() => window.addEventListener("keydown", onKeydown));
onBeforeUnmount(() => window.removeEventListener("keydown", onKeydown));
</script>

<template>
  <Button
    variant="ghost"
    size="icon-sm"
    class="absolute top-3 left-3 z-10 text-muted-foreground hover:text-foreground"
    :aria-label="label"
    :aria-expanded="!collapsed"
    :title="`${label} (${shortcut})`"
    @click="toggle"
  >
    <PanelLeftOpenIcon v-if="collapsed" />
    <PanelLeftCloseIcon v-else />
  </Button>
</template>
