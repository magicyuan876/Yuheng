<template>
  <Popover v-model:open="visible">
    <!-- The trigger sits in the answer toolbar next to botmsg's copy and
         save buttons and wears the same 30px square, muted, thin-stroke
         look the toolbar gave every TDesign button there. -->
    <PopoverTrigger as-child>
      <button
        type="button"
        data-slot="chat-request-info-trigger"
        class="text-muted-foreground hover:bg-accent hover:text-foreground inline-flex size-[30px] shrink-0 items-center justify-center rounded-[8px] transition-[background-color,color] duration-150 ease-in-out active:bg-[var(--td-bg-color-container-active)]"
        :title="$t('chat.requestInfoTitle')"
      >
        <InfoIcon class="size-4 stroke-[1.2]" aria-hidden="true" />
      </button>
    </PopoverTrigger>
    <PopoverContent
      side="top"
      class="text-foreground w-auto gap-0 rounded-[8px] p-0 shadow-[var(--td-shadow-2)] ring-0"
      @click.stop
    >
      <div class="max-w-[360px] min-w-[260px] px-3 py-2.5 text-[12px]">
        <div class="border-border mb-2 flex items-center justify-between gap-2 border-b border-solid pb-1.5">
          <span class="text-[12px] font-semibold">{{ $t("chat.requestInfoTitle") }}</span>
          <Button v-if="rows.length > 0" variant="ghost" size="icon-xs" :title="$t('common.copy')" @click="copyAll">
            <CopyIcon />
          </Button>
        </div>
        <div v-if="rows.length === 0" class="text-placeholder pt-1 pb-2">
          {{ $t("chat.requestInfoEmpty") }}
        </div>
        <div v-else class="flex flex-col gap-0.5">
          <div v-for="row in rows" :key="row.key" class="flex items-start gap-2 py-[3px] leading-normal">
            <span class="text-muted-foreground flex-[0_0_72px]">{{ row.label }}</span>
            <span
              class="text-foreground flex-1 font-[family-name:var(--td-font-family-mono,ui-monospace,SFMono-Regular,Menlo,monospace)] text-[11px] break-all"
              >{{ row.value }}</span
            >
          </div>
        </div>
      </div>
      <PopoverArrow class="fill-popover" :width="12" :height="6" />
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import { CopyIcon, InfoIcon } from "@lucide/vue";
import { PopoverArrow } from "reka-ui";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { buildChatRequestDebugPayload, type ChatRequestDebugInfo } from "@/utils/chatRequestDebug";
import { copyWithToast } from "@/utils/clipboard";

const props = defineProps<{
  session: Record<string, unknown>;
  sessionId?: string;
}>();

const { t } = useI18n();
const visible = ref(false);

const debugInfo = computed((): ChatRequestDebugInfo => {
  const s = props.session;
  const dr = s.debugRequest as ChatRequestDebugInfo | undefined;
  return {
    requestId: (s.request_id as string) || dr?.requestId,
    messageId: (s.id as string) || undefined,
    sessionId: props.sessionId || dr?.sessionId,
    url: dr?.url,
    method: dr?.method,
    body: dr?.body ?? null,
    sentAt: dr?.sentAt,
  };
});

const rows = computed(() => {
  const info = debugInfo.value;
  const list: { key: string; label: string; value: string }[] = [];
  const add = (key: string, labelKey: string, val?: string) => {
    if (!val) return;
    list.push({ key, label: t(labelKey), value: val });
  };
  add("requestId", "chat.requestInfoRequestId", info.requestId);
  add("messageId", "chat.requestInfoMessageId", info.messageId);
  add("sessionId", "chat.requestInfoSessionId", info.sessionId);
  if (info.method && info.url) {
    list.push({ key: "url", label: t("chat.requestInfoUrl"), value: `${info.method} ${info.url}` });
  }
  if (info.sentAt) {
    add("sentAt", "chat.requestInfoSentAt", new Date(info.sentAt).toLocaleString());
  }
  return list;
});

const copyAll = async () => {
  const ok = await copyWithToast(buildChatRequestDebugPayload(debugInfo.value), "common.copied");
  if (ok) visible.value = false;
};
</script>
