<template>
  <Dialog :open="token !== ''" @update:open="onOpenChange">
    <!-- An outside click must not throw the only copy of the secret away. -->
    <DialogContent class="sm:max-w-[480px]" @interact-outside.prevent>
      <DialogHeader>
        <DialogTitle>{{ title }}</DialogTitle>
        <DialogDescription>{{ t("apiKeys.secret.description") }}</DialogDescription>
      </DialogHeader>
      <Textarea
        :model-value="token"
        readonly
        class="font-mono text-xs break-all"
        data-slot="api-key-secret"
        @focus="($event.target as HTMLTextAreaElement).select()"
      />
      <DialogFooter>
        <Button @click="copy">
          <CopyIcon />
          {{ t("apiKeys.secret.copy") }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<script setup lang="ts">
import { useI18n } from "vue-i18n";
import { CopyIcon } from "@lucide/vue";
import { copyWithToast } from "@/utils/clipboard";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";

/**
 * Shows a freshly created API key's secret — the one time the backend ever
 * returns it — with a copy button.
 *
 * The token is the dialog's only state: it is open while the token is set,
 * and closing it (copying, Escape) clears the token, so the secret does not
 * outlive the dialog in the parent's state.
 */
defineProps<{ title: string }>();
const token = defineModel<string>("token", { required: true });

const { t } = useI18n();

function onOpenChange(open: boolean) {
  if (!open) token.value = "";
}

async function copy() {
  if (await copyWithToast(token.value, "apiKeys.secret.copySuccess")) token.value = "";
}
</script>
