<template>
  <div class="model-selector relative w-full">
    <Popover v-model:open="open">
      <PopoverTrigger as-child>
        <!--
          A button styled as the select box. The search field lives inside
          the popover, which keeps the trigger a single focusable control
          and the list reachable from it by keyboard.
        -->
        <button
          type="button"
          data-slot="model-selector-trigger"
          :disabled="disabled"
          :aria-invalid="status === 'error' || undefined"
          class="border-input dark:bg-input/30 focus-visible:border-ring focus-visible:ring-ring/50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 flex h-8 w-full min-w-0 items-center gap-1.5 rounded-lg border bg-transparent pr-2 pl-2.5 text-left text-sm transition-colors outline-none focus-visible:ring-3 disabled:cursor-not-allowed disabled:opacity-50"
          :class="{
            'border-warning': status === 'warning',
            'border-success': status === 'success',
            'border-ring': open,
          }"
        >
          <span v-if="selectedLabel" class="text-foreground min-w-0 flex-1 truncate">{{ selectedLabel }}</span>
          <span v-else class="text-placeholder min-w-0 flex-1 truncate">{{ placeholderText }}</span>
          <Loader2Icon v-if="loading" class="text-muted-foreground size-4 shrink-0 animate-spin" />
          <ChevronDownIcon
            v-else
            class="text-muted-foreground size-4 shrink-0 transition-transform duration-150"
            :class="{ 'rotate-180': open, invisible: showClear }"
          />
        </button>
      </PopoverTrigger>

      <!--
        z-[5500] matches TDesign's popup layer: this selector is still used
        inside TDesign dialogs (z-index 2500), and a list at the default z-50
        would open behind them.
      -->
      <PopoverContent
        align="start"
        class="z-[5500] w-(--reka-popover-trigger-width) min-w-[240px] gap-1 p-1"
        @open-auto-focus="onOpenAutoFocus"
      >
        <div class="relative">
          <SearchIcon
            class="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2"
          />
          <Input
            ref="searchInputRef"
            v-model="keyword"
            :placeholder="t('menu.search')"
            class="h-8 pl-8 text-[13px] md:text-[13px]"
            role="combobox"
            aria-autocomplete="list"
            :aria-expanded="open"
            @keydown="onSearchKeydown"
          />
        </div>

        <div role="listbox" class="flex max-h-[300px] flex-col gap-px overflow-y-auto">
          <!-- 已有的模型选项 -->
          <button
            v-for="(model, index) in filteredModels"
            :key="model.id"
            type="button"
            role="option"
            data-slot="model-selector-option"
            :aria-selected="model.id === selectedModelId"
            class="model-option flex h-8 w-full shrink-0 items-center gap-2 rounded-md px-2 text-left"
            :class="{
              'bg-accent': index === highlightedIndex,
              'text-primary': model.id === selectedModelId,
            }"
            @mouseenter="highlightedIndex = index"
            @click="handleModelChange(model.id)"
          >
            <CircleCheckIcon class="model-icon text-primary size-3.5 shrink-0" />
            <span class="model-name min-w-0 flex-[0_1_auto] truncate text-[13px]">{{ modelDisplayName(model) }}</span>
            <span v-if="model.display_name" class="model-raw-name text-placeholder min-w-0 flex-1 truncate text-xs">
              {{ model.name }}
            </span>
            <span
              v-if="model.is_builtin"
              class="bg-primary text-primary-foreground inline-flex h-5 shrink-0 items-center rounded-sm px-1.5 text-xs"
            >
              {{ $t("model.builtinTag") }}
            </span>
            <span
              v-if="model.is_default"
              class="bg-success text-primary-foreground inline-flex h-5 shrink-0 items-center rounded-sm px-1.5 text-xs"
            >
              {{ $t("model.defaultTag") }}
            </span>
          </button>

          <div
            v-if="filteredModels.length === 0"
            class="text-muted-foreground flex h-8 shrink-0 items-center justify-center text-[13px]"
          >
            {{ loading ? t("common.loading") : t("common.noResult") }}
          </div>

          <!-- 添加模型选项（在底部） -->
          <!-- canManageModels：集中管控模式下模型配置归系统管理员，其他人看到这个
               入口点进去会落到一个隐藏的设置页。模型列表本身不受影响，平台模型照常可选。 -->
          <button
            v-if="!disabled && canManageModels"
            type="button"
            role="option"
            data-slot="model-selector-option"
            :aria-selected="false"
            class="add-model-option model-option add flex h-8 w-full shrink-0 items-center gap-2 rounded-md px-2 text-left"
            :class="{ 'bg-accent': highlightedIndex === filteredModels.length }"
            @mouseenter="highlightedIndex = filteredModels.length"
            @click="handleModelChange(ADD_MODEL_VALUE)"
          >
            <PlusIcon class="add-icon text-primary size-3.5 shrink-0" />
            <span class="model-name text-primary min-w-0 truncate text-[13px] font-medium">
              {{ $t("model.addModelInSettings") }}
            </span>
          </button>
        </div>
      </PopoverContent>
    </Popover>

    <!--
      The clear button sits over the chevron rather than inside the trigger:
      a button nested in a button is invalid markup and would open the list
      on every click.
    -->
    <button
      v-if="showClear"
      type="button"
      data-slot="model-selector-clear"
      :aria-label="t('common.clear')"
      class="text-muted-foreground hover:text-foreground absolute top-1/2 right-2 flex size-4 -translate-y-1/2 items-center justify-center"
      @click="handleModelChange('')"
    >
      <CircleXIcon class="size-4" />
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, nextTick } from "vue";
import { listModels, type ModelConfig } from "@/api/model";
import { usePlatformInfraAccess } from "@/composables/usePlatformInfraAccess";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import { filterModelsByType } from "./modelSelectorFilter";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { ChevronDownIcon, CircleCheckIcon, CircleXIcon, Loader2Icon, PlusIcon, SearchIcon } from "@lucide/vue";

interface Props {
  modelType: "KnowledgeQA" | "Embedding" | "Rerank" | "VLLM" | "ASR";
  selectedModelId?: string;
  disabled?: boolean;
  placeholder?: string;
  status?: "default" | "success" | "warning" | "error";
  clearable?: boolean;
  // 可选：外部传入的所有模型列表，如果提供则不调用API
  allModels?: ModelConfig[];
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
  placeholder: "",
  status: "default",
  clearable: false,
});

const emit = defineEmits<{
  "update:selectedModelId": [value: string];
  "add-model": [];
}>();

const canManageModels = usePlatformInfraAccess("models");
const models = ref<ModelConfig[]>([]);
const loading = ref(false);
const { t } = useI18n();

// The sentinel value the "add model" entry selects. It never reaches the
// parent: handleModelChange turns it into the add-model event.
const ADD_MODEL_VALUE = "__add_model__";

const open = ref(false);
const keyword = ref("");
const highlightedIndex = ref(-1);
const searchInputRef = ref<InstanceType<typeof Input> | null>(null);

const selectedModel = computed(() => models.value.find((model) => model.id === props.selectedModelId));

// Like the old TDesign select, a value with no matching option (a model
// that was deleted, or a list still loading) shows the raw id rather than
// pretending nothing is selected.
const selectedLabel = computed(() => {
  if (!props.selectedModelId) return "";
  return selectedModel.value ? modelDisplayName(selectedModel.value) : props.selectedModelId;
});

const showClear = computed(() => props.clearable && !props.disabled && !!props.selectedModelId);

// The search matches the name shown and the raw model name, case-blind,
// so a model found by either spelling in the list is found here too.
const filteredModels = computed(() => {
  const q = keyword.value.trim().toLowerCase();
  if (!q) return models.value;
  return models.value.filter(
    (model) => modelDisplayName(model).toLowerCase().includes(q) || model.name.toLowerCase().includes(q),
  );
});

// The add entry counts as the last row for keyboard navigation.
const optionCount = computed(() => filteredModels.value.length + (!props.disabled && canManageModels.value ? 1 : 0));

watch(open, (value) => {
  if (!value) return;
  keyword.value = "";
  const index = filteredModels.value.findIndex((model) => model.id === props.selectedModelId);
  highlightedIndex.value = index;
});

watch(keyword, () => {
  highlightedIndex.value = filteredModels.value.length > 0 ? 0 : -1;
});

// Focus lands in the search field when the list opens, so typing filters
// straight away as it did in the old select.
function onOpenAutoFocus(event: Event) {
  event.preventDefault();
  void nextTick(() => {
    const el = searchInputRef.value?.$el;
    if (el instanceof HTMLElement) el.focus();
  });
}

function onSearchKeydown(event: KeyboardEvent) {
  const count = optionCount.value;
  if (event.key === "ArrowDown") {
    event.preventDefault();
    if (count > 0) highlightedIndex.value = (highlightedIndex.value + 1) % count;
  } else if (event.key === "ArrowUp") {
    event.preventDefault();
    if (count > 0) highlightedIndex.value = (highlightedIndex.value - 1 + count) % count;
  } else if (event.key === "Enter") {
    event.preventDefault();
    const index = highlightedIndex.value;
    if (index < 0) return;
    const model = filteredModels.value[index];
    if (model) handleModelChange(model.id);
    else if (index === filteredModels.value.length) handleModelChange(ADD_MODEL_VALUE);
  }
}

const placeholderText = computed(() => {
  return props.placeholder || t("model.selectModelPlaceholder");
});

const modelDisplayName = (model: ModelConfig) => {
  const displayName = model.display_name?.trim();
  return displayName || model.name;
};

// 监听 allModels / modelType 变化，自动过滤当前类型的模型
watch(
  () => [props.allModels, props.modelType] as const,
  ([newModels]) => {
    if (newModels && Array.isArray(newModels)) {
      models.value = filterModelsByType(newModels, props.modelType);
    }
  },
  { immediate: true },
);

// 加载模型列表（仅在未提供 allModels 时调用）
const loadModels = async () => {
  // 如果外部提供了 allModels，则不需要加载
  if (props.allModels) {
    return;
  }

  loading.value = true;
  try {
    const result = await listModels();
    // 前端按类型筛选模型
    if (result && Array.isArray(result)) {
      models.value = filterModelsByType(result, props.modelType);
    } else {
      models.value = [];
    }
  } catch (error) {
    console.error(t("model.loadFailed"), error);
    MessagePlugin.error(t("model.loadFailed"));
    models.value = [];
  } finally {
    loading.value = false;
  }
};

// 处理模型选择变化
const handleModelChange = (value?: string) => {
  open.value = false;
  // 如果选择的是添加模型选项，触发添加事件而不更新选中值
  if (value === ADD_MODEL_VALUE) {
    emit("add-model");
    return;
  }
  emit("update:selectedModelId", value || "");
};

// 暴露刷新方法给父组件
defineExpose({
  refresh: loadModels,
});

onMounted(() => {
  // 只有在没有提供 allModels 时才加载
  if (!props.allModels) {
    loadModels();
  }
});
</script>
