<template>
  <div class="inline-flex" :class="{ 'absolute right-2 bottom-2 z-10': position === 'corner' }">
    <div class="inline-flex items-center gap-1">
      <!-- 恢复默认按钮 -->
      <Button
        variant="ghost"
        size="sm"
        class="text-placeholder hover:text-primary h-[26px] gap-[3px] px-1.5 text-xs"
        :disabled="resettingDefault"
        @click="handleResetToDefault"
      >
        <Loader2Icon v-if="resettingDefault" class="size-3.5 animate-spin" />
        <Undo2Icon v-else class="size-3.5" />
        <span>{{ $t("promptTemplate.resetDefault") }}</span>
      </Button>
      <!--
        选择模板按钮. The popover stays controlled (open + update:open) so a
        picked template can close it and the first opening can load the list.
        TDesign's placement="top-right" is side="top" + align="end".
      -->
      <Popover v-if="showTemplatePicker" :open="popupVisible" @update:open="handleVisibleChange">
        <PopoverTrigger as-child>
          <Button
            variant="outline"
            size="sm"
            class="border-border bg-card text-muted-foreground hover:border-primary hover:bg-secondary hover:text-primary dark:border-border dark:bg-card dark:hover:bg-secondary h-[26px] gap-1 px-2 text-xs"
            :disabled="loading"
          >
            <Loader2Icon v-if="loading" class="size-3.5 animate-spin" />
            <LayoutGridIcon v-else class="size-3.5" />
            <span>{{ $t("promptTemplate.useTemplate") }}</span>
          </Button>
        </PopoverTrigger>
        <PopoverContent side="top" align="end" class="w-auto gap-0 rounded-md px-2 py-0.5">
          <div class="flex max-h-[400px] w-[420px] flex-col overflow-hidden">
            <div class="border-border shrink-0 border-b border-solid px-4 py-3">
              <span class="text-foreground text-sm font-medium">{{ $t("promptTemplate.selectTemplate") }}</span>
            </div>
            <div v-if="loading" class="text-placeholder flex justify-center px-4 py-10 text-center text-[13px]">
              <Loader2Icon class="text-primary size-5 animate-spin" />
            </div>
            <div v-else-if="templates.length === 0" class="text-placeholder px-4 py-10 text-center text-[13px]">
              {{ $t("promptTemplate.noTemplates") }}
            </div>
            <div v-else class="flex-1 overflow-y-auto p-2">
              <div
                v-for="template in templates"
                :key="template.id"
                class="hover:bg-secondary mb-1 cursor-pointer rounded-lg p-3 transition-all duration-200 ease-in-out last:mb-0"
                @click="selectTemplate(template)"
              >
                <div class="mb-1.5 flex flex-wrap items-center gap-2">
                  <span class="text-foreground text-sm font-medium">{{ template.name }}</span>
                  <span
                    v-if="template.default"
                    :class="[tagClass, 'text-warning bg-[var(--td-warning-color-light)] font-medium']"
                  >
                    {{ $t("promptTemplate.default") }}
                  </span>
                  <span
                    v-if="template.has_knowledge_base"
                    :class="[tagClass, 'text-primary bg-[var(--td-brand-color-light)]']"
                  >
                    <FolderIcon class="size-3" />
                    {{ $t("promptTemplate.withKnowledgeBase") }}
                  </span>
                  <span
                    v-if="template.has_web_search"
                    :class="[tagClass, 'text-primary bg-[var(--td-success-color-light)]']"
                  >
                    <GlobeIcon class="size-3" />
                    {{ $t("promptTemplate.withWebSearch") }}
                  </span>
                </div>
                <p class="text-muted-foreground m-0 line-clamp-2 text-xs leading-normal">{{ template.description }}</p>
              </div>
            </div>
          </div>
        </PopoverContent>
      </Popover>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { FolderIcon, GlobeIcon, LayoutGridIcon, Loader2Icon, Undo2Icon } from "@lucide/vue";
import { getPromptTemplates, type PromptTemplate, type PromptTemplatesConfig } from "@/api/system";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";

// The small pill every template tag (default / knowledge base / web search)
// shares; each adds its own tint.
const tagClass = "inline-flex items-center gap-[3px] rounded-[4px] px-1.5 py-0.5 text-[11px]";

const props = withDefaults(
  defineProps<{
    type: "systemPrompt" | "contextTemplate" | "rewrite" | "fallback" | "agentSystemPrompt" | "intentPrompt";
    hasKnowledgeBase?: boolean;
    position?: "inline" | "corner"; // inline: 行内显示, corner: 输入框右下角
    /** 用于 fallback 场景：区分固定回复和模型 prompt */
    fallbackMode?: "fixed" | "model";
    /** intent 场景：当前选中的 intent id（对应 template.id） */
    intentId?: string;
    /** 为 false 时只显示「恢复默认」，不显示「使用模板」 */
    showTemplatePicker?: boolean;
  }>(),
  {
    showTemplatePicker: true,
  },
);

const emit = defineEmits<{
  (e: "select", template: PromptTemplate): void;
  (e: "reset-default", template: PromptTemplate): void;
}>();

const popupVisible = ref(false);
const loading = ref(false);
const resettingDefault = ref(false);
const templatesConfig = ref<PromptTemplatesConfig | null>(null);

const handleVisibleChange = async (visible: boolean) => {
  popupVisible.value = visible;
  // 首次打开时加载模板
  if (visible && !templatesConfig.value) {
    await loadTemplates();
  }
};

const loadTemplates = async () => {
  if (loading.value) return;
  loading.value = true;
  try {
    const response = await getPromptTemplates();
    templatesConfig.value = response.data;
  } catch (error) {
    console.error("Failed to load prompt templates:", error);
  } finally {
    loading.value = false;
  }
};

// 根据类型获取对应的模板列表
const templates = computed<PromptTemplate[]>(() => {
  if (!templatesConfig.value) return [];

  let list: PromptTemplate[] = [];
  switch (props.type) {
    case "systemPrompt":
      list = templatesConfig.value.system_prompt || [];
      break;
    case "contextTemplate":
      list = templatesConfig.value.context_template || [];
      break;
    case "rewrite":
      list = templatesConfig.value.rewrite || [];
      break;
    case "fallback":
      list = templatesConfig.value.fallback || [];
      // Filter by fallbackMode: "model" mode shows only mode:"model" templates, otherwise shows non-model templates
      if (props.fallbackMode === "model") {
        list = list.filter((t) => t.mode === "model");
      } else if (props.fallbackMode === "fixed") {
        list = list.filter((t) => !t.mode || t.mode !== "model");
      }
      break;
    case "agentSystemPrompt":
      list = templatesConfig.value.agent_system_prompt || [];
      break;
    case "intentPrompt":
      list = templatesConfig.value.intent_prompts || [];
      break;
    default:
      list = [];
  }
  return list;
});

const selectTemplate = (template: PromptTemplate) => {
  emit("select", template);
  popupVisible.value = false;
};

// Find the default template (marked with default: true, or the first one)
const findDefaultTemplate = (list: PromptTemplate[]): PromptTemplate | null => {
  if (!list || list.length === 0) return null;
  const defaultItem = list.find((t) => t.default);
  return defaultItem || list[0];
};

const resolveResetTemplate = (): PromptTemplate | null => {
  const list = templates.value;
  if (props.type === "intentPrompt") {
    if (!props.intentId) return null;
    return list.find((t) => t.id === props.intentId) ?? null;
  }
  return findDefaultTemplate(list);
};

// Reset to default template content
const handleResetToDefault = async () => {
  if (!templatesConfig.value) {
    resettingDefault.value = true;
    try {
      const response = await getPromptTemplates();
      templatesConfig.value = response.data;
    } catch (error) {
      console.error("Failed to load prompt templates:", error);
      return;
    } finally {
      resettingDefault.value = false;
    }
  }

  const defaultTpl = resolveResetTemplate();
  if (defaultTpl) {
    emit("reset-default", defaultTpl);
  }
};

// 预加载模板（可选）
onMounted(() => {
  // 可以在这里预加载，也可以等用户点击时再加载
  // loadTemplates();
});
</script>
