<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <Button
        variant="ghost"
        size="icon-sm"
        class="text-destructive hover:text-destructive"
        :title="label"
        :aria-label="label"
        @click.stop
      >
        <Trash2Icon />
      </Button>
    </PopoverTrigger>
    <PopoverContent align="end" class="w-64">
      <p class="text-foreground m-0 mb-3 text-[13px] leading-[1.5]">{{ message }}</p>
      <div class="flex justify-end gap-2">
        <Button size="sm" variant="outline" @click="open = false">{{ t("common.cancel") }}</Button>
        <Button size="sm" variant="destructive" data-slot="api-key-revoke-confirm" @click="confirm">
          {{ label }}
        </Button>
      </div>
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { useI18n } from "vue-i18n";
import { Trash2Icon } from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

/** Trash button that asks before revoking a key; emits `confirm` once the user agrees. */
defineProps<{ label: string; message: string }>();
const emit = defineEmits<{ confirm: [] }>();

const { t } = useI18n();
const open = ref(false);

function confirm() {
  open.value = false;
  emit("confirm");
}
</script>
