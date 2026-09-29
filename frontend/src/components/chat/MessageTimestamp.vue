<template>
  <time
    v-if="label"
    class="text-placeholder block w-full pt-1 pb-2 text-center text-xs leading-5 tabular-nums select-none"
    :datetime="datetime"
    >{{ label }}</time
  >
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { getConversationTimestampModel } from "@/utils/messageTimestamp";

const props = defineProps<{
  value?: unknown;
}>();

const { t } = useI18n();

const model = computed(() => getConversationTimestampModel(props.value));
const datetime = computed(() => model.value?.datetime ?? "");
const label = computed(() => {
  const next = model.value;
  if (!next) return "";
  if (next.kind === "today") return t("chat.conversationTime.today", { time: next.time });
  if (next.kind === "yesterday") return t("chat.conversationTime.yesterday", { time: next.time });
  if (next.kind === "thisYear") {
    return t("chat.conversationTime.thisYear", { month: next.month, day: next.day, time: next.time });
  }
  return t("chat.conversationTime.otherYear", {
    year: next.year,
    month: next.month,
    day: next.day,
    time: next.time,
  });
});
</script>
