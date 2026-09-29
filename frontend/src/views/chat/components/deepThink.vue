<template>
  <div
    class="deep-think border-border bg-card -mt-2 mb-2.5 box-border flex w-full flex-col overflow-hidden rounded-lg border-[0.5px] border-solid text-[12px] shadow-[0_2px_4px_color-mix(in_srgb,var(--td-brand-color)_8%,transparent)] transition-all duration-[250ms]"
  >
    <div
      class="text-foreground flex cursor-pointer items-center justify-between px-3.5 py-1.5 font-medium select-none hover:bg-[color-mix(in_srgb,var(--td-brand-color)_4%,transparent)]"
      @click="toggleFold"
    >
      <div class="flex items-center">
        <span v-if="deepSession.thinking" class="flex items-center">
          <span class="relative mr-2 flex h-4 w-4 items-center justify-center">
            <span class="indicator-dot bg-primary h-1.5 w-1.5 rounded-full"></span>
            <span
              class="indicator-ring border-primary absolute inset-0 rounded-full border-[1.5px] border-solid opacity-0"
            ></span>
          </span>
          <span class="text-foreground text-[12px] whitespace-nowrap">{{ $t("chat.thinking") }}</span>
        </span>
        <span v-else class="flex items-center">
          <img class="mr-2 h-4 w-4" src="@/assets/img/Frame3718.svg" :alt="$t('chat.deepThoughtAlt')" />
          <span class="text-foreground text-[12px] whitespace-nowrap">{{ $t("chat.deepThoughtCompleted") }}</span>
        </span>
      </div>
      <div class="text-primary px-0.5 pt-0 pb-px text-[14px]">
        <ChevronDownIcon v-if="isFold" class="h-3.5 w-3.5 transition-transform duration-200" />
        <ChevronUpIcon v-else class="h-3.5 w-3.5 transition-transform duration-200" />
      </div>
    </div>
    <div v-show="!isFold || deepSession.thinking" class="border-secondary border-t border-solid">
      <div
        ref="contentInnerRef"
        class="content-inner text-muted-foreground max-h-[200px] overflow-y-auto px-3.5 py-2 text-[12px] leading-[1.6] break-words whitespace-pre-wrap"
      >
        {{ deepSession.thinkContent }}
      </div>
    </div>
  </div>
</template>
<script setup>
import { watch, ref, onMounted, nextTick } from "vue";
import { ChevronDownIcon, ChevronUpIcon } from "@lucide/vue";

const isFold = ref(false);
const contentInnerRef = ref(null);
const props = defineProps({
  // 必填项
  deepSession: {
    type: Object,
    required: false,
  },
});

// 初始化时检查：如果 thinking 已完成（从历史记录加载），默认折叠
onMounted(() => {
  if (props.deepSession?.thinking === false) {
    isFold.value = true;
  }
});

// 监听 thinking 状态变化，自动折叠
watch(
  () => props.deepSession?.thinking,
  (newVal, oldVal) => {
    // 当 thinking 从 true 变为 false 时，自动折叠 thinking 内容
    // 只在流式输出场景下触发（oldVal 为 true）
    if (oldVal === true && newVal === false) {
      isFold.value = true;
    }
  },
);

// 监听内容变化，自动滚动到底部
watch(
  () => props.deepSession?.thinkContent,
  () => {
    // 只在 thinking 进行中时滚动
    if (props.deepSession?.thinking) {
      nextTick(() => {
        if (contentInnerRef.value) {
          contentInnerRef.value.scrollTop = contentInnerRef.value.scrollHeight;
        }
      });
    }
  },
);

const toggleFold = () => {
  // 只有 thinking 完成后才能折叠/展开
  if (!props.deepSession?.thinking) {
    isFold.value = !isFold.value;
  }
};
</script>
<style scoped>
/*
 * Stays CSS: the thinking indicator's two keyframe animations, and the
 * scrollbar pseudo-elements of the reasoning text, which utilities cannot
 * reach.
 */
.indicator-dot {
  animation: pulse-dot 1.8s ease-in-out infinite;
}

.indicator-ring {
  animation: pulse-ring 1.8s ease-out infinite;
}

@keyframes pulse-dot {
  0%,
  100% {
    transform: scale(0.85);
    opacity: 0.6;
  }
  50% {
    transform: scale(1.1);
    opacity: 1;
  }
}

@keyframes pulse-ring {
  0% {
    transform: scale(0.5);
    opacity: 0.6;
  }
  100% {
    transform: scale(1.2);
    opacity: 0;
  }
}

.content-inner::-webkit-scrollbar {
  width: 4px;
}

.content-inner::-webkit-scrollbar-thumb {
  background: rgba(0, 0, 0, 0.1);
  border-radius: 2px;
}

html[theme-mode="dark"] .content-inner::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.15);
}
</style>
