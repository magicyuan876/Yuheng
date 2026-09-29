<script setup lang="ts">
/**
 * The clear button of TDesign's `clearable` t-select, for the shadcn Select,
 * which has none. A cleared value matters on the settings screens: an empty
 * string means "leave it to the server default", and without this button a
 * picked option could never be taken back.
 *
 * Place it next to the Select inside a `relative group/select-clear` wrapper
 * (see the parser and web-search drawers). It sits over the trigger, left of
 * the chevron, and — like TDesign's — shows only while the pointer or the
 * focus is on the field. It lives outside the trigger, so clicking it clears
 * without opening the list.
 */
import { useI18n } from "vue-i18n";
import { XIcon } from "@lucide/vue";

defineProps<{ visible: boolean }>();
const emit = defineEmits<{ (e: "clear"): void }>();

const { t } = useI18n();
</script>

<template>
  <button
    v-if="visible"
    type="button"
    data-slot="select-clear"
    class="text-placeholder hover:text-foreground absolute top-1/2 right-8 flex -translate-y-1/2 items-center opacity-0 transition-opacity group-focus-within/select-clear:opacity-100 group-hover/select-clear:opacity-100 focus-visible:opacity-100"
    :aria-label="t('common.clear')"
    @click="emit('clear')"
  >
    <XIcon class="size-3.5" />
  </button>
</template>
