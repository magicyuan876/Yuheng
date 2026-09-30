<template>
  <div class="w-full">
    <div v-if="!embedded" class="mb-5">
      <h2 class="text-foreground mt-0 mb-1.5 text-xl font-semibold">{{ t("graphSettings.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-normal">{{ t("graphSettings.description") }}</p>

      <!-- t-alert theme="warning": warning-coloured icon, message in the normal text colour. -->
      <Alert v-if="!isGraphDatabaseEnabled" class="border-warning/40 bg-warning/10 text-warning mt-4">
        <CircleAlertIcon />
        <AlertTitle class="text-foreground font-normal">
          <div>{{ t("graphSettings.disabledWarning") }}</div>
          <button
            type="button"
            data-slot="link-button"
            class="text-primary hover:text-primary/80 text-sm"
            @click="handleOpenGraphGuide"
          >
            {{ t("graphSettings.howToEnable") }}
          </button>
        </AlertTitle>
      </Alert>
    </div>
    <Alert v-else-if="!isGraphDatabaseEnabled" class="border-warning/40 bg-warning/10 text-warning mb-3">
      <CircleAlertIcon />
      <AlertTitle class="text-foreground font-normal">{{ t("graphSettings.disabledWarning") }}</AlertTitle>
    </Alert>

    <div v-if="isGraphDatabaseEnabled" class="flex flex-col">
      <!-- 启用实体关系提取 -->
      <div
        class="border-border flex items-start justify-between [&:not(:last-child)]:border-b"
        :class="embedded ? 'flex-col items-stretch gap-2 py-3' : 'py-4'"
      >
        <div :class="embedded ? 'max-w-none pr-0' : 'max-w-[40%] shrink-0 basis-2/5 pr-6'">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ t("graphSettings.enableLabel") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ t("graphSettings.enableDescription") }}
          </p>
        </div>
        <div :class="embedded ? 'self-start' : 'flex max-w-[55%] shrink-0 basis-[55%] items-center justify-end'">
          <Switch
            :model-value="localGraphExtract.enabled"
            @update:model-value="
              (val: boolean) => {
                localGraphExtract.enabled = val;
                handleEnabledChange();
              }
            "
          />
        </div>
      </div>

      <div
        v-if="localGraphExtract.enabled"
        class="border-border flex flex-col gap-3 py-4 [&:not(:last-child)]:border-b"
      >
        <div class="max-w-[40%] pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            t("graphSettings.customInstructionsLabel")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ t("graphSettings.customInstructionsDescription") }}
          </p>
        </div>
        <div class="flex w-full max-w-full flex-col items-start gap-3">
          <Textarea
            v-model="localGraphExtract.customInstructions"
            :placeholder="t('graphSettings.customInstructionsPlaceholder')"
            :maxlength="4000"
            :rows="3"
            class="max-h-[176px] min-h-[76px]"
            @update:model-value="handleConfigChange"
          />
        </div>
      </div>

      <!-- 关系类型配置 -->
      <div
        v-if="localGraphExtract.enabled"
        class="border-border flex flex-col gap-3 py-4 [&:not(:last-child)]:border-b"
      >
        <div class="max-w-[40%] pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ t("graphSettings.tagsLabel") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ t("graphSettings.tagsDescription") }}
          </p>
        </div>
        <div class="flex w-full max-w-full flex-col items-start gap-3">
          <div class="flex w-full items-start gap-3">
            <Button
              v-if="canRunGraphExtract"
              variant="secondary"
              :disabled="!modelStatus.llm.available || tagFabring"
              @click="handleFabriTag"
            >
              <Loader2Icon v-if="tagFabring" class="animate-spin" />
              {{ t("graphSettings.generateRandomTags") }}
            </Button>
            <!--
              t-select multiple + creatable + clearable with no options of its own:
              the values show as removable chips, typing and Enter adds one, and
              a clear button empties the list.
            -->
            <div
              class="border-input focus-within:border-ring focus-within:ring-ring/50 dark:bg-input/30 flex min-h-8 min-w-[400px] flex-1 flex-wrap items-center gap-1 rounded-lg border px-1.5 py-1 focus-within:ring-3"
            >
              <span
                v-for="tag in localGraphExtract.tags"
                :key="tag"
                class="bg-muted text-foreground inline-flex h-6 items-center gap-1 rounded-md pr-1 pl-2 text-xs"
              >
                {{ tag }}
                <button
                  type="button"
                  data-slot="icon-button"
                  class="text-muted-foreground hover:text-foreground inline-flex"
                  :aria-label="t('common.delete')"
                  @click="removeTag(tag)"
                >
                  <XIcon class="size-3" />
                </button>
              </span>
              <input
                v-model="newTagInput"
                data-slot="tag-input"
                class="placeholder:text-placeholder h-6 min-w-[120px] flex-1 px-1 text-sm outline-none"
                :placeholder="localGraphExtract.tags.length ? '' : t('graphSettings.tagsPlaceholder')"
                @keydown.enter.prevent="addTag"
              />
              <button
                v-if="localGraphExtract.tags.length"
                type="button"
                data-slot="icon-button"
                class="text-placeholder hover:text-muted-foreground inline-flex px-1"
                :aria-label="t('common.clear')"
                @click="
                  localGraphExtract.tags = [];
                  handleTagsChange();
                "
              >
                <CircleXIcon class="size-4" />
              </button>
            </div>
          </div>
          <div v-if="!modelStatus.llm.available" class="text-muted-foreground flex items-center gap-1.5 text-[13px]">
            <InfoIcon class="text-primary size-4" />
            <span>{{ t("graphSettings.completeModelConfig") }}</span>
          </div>
        </div>
      </div>

      <!-- 示例文本 -->
      <div
        v-if="localGraphExtract.enabled"
        class="border-border flex flex-col gap-3 py-4 [&:not(:last-child)]:border-b"
      >
        <div class="max-w-[40%] pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            t("graphSettings.sampleTextLabel")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ t("graphSettings.sampleTextDescription") }}
          </p>
        </div>
        <div class="flex w-full max-w-full flex-col items-start gap-3">
          <div class="flex w-full flex-col items-start gap-3">
            <Button
              v-if="canRunGraphExtract"
              variant="secondary"
              :disabled="!modelStatus.llm.available || textFabring"
              @click="handleFabriText"
            >
              <Loader2Icon v-if="textFabring" class="animate-spin" />
              {{ t("graphSettings.generateRandomText") }}
            </Button>
            <div class="w-full">
              <Textarea
                v-model="localGraphExtract.text"
                :placeholder="t('graphSettings.sampleTextPlaceholder')"
                :rows="6"
                :maxlength="5000"
                class="max-h-[256px] min-h-[136px]"
                @update:model-value="handleTextChange"
              />
              <!-- t-textarea's show-word-limit. -->
              <p class="text-placeholder m-0 mt-1 text-right text-xs">{{ localGraphExtract.text.length }}/5000</p>
            </div>
          </div>
          <div v-if="!modelStatus.llm.available" class="text-muted-foreground flex items-center gap-1.5 text-[13px]">
            <InfoIcon class="text-primary size-4" />
            <span>{{ t("graphSettings.completeModelConfig") }}</span>
          </div>
        </div>
      </div>

      <!-- 实体列表 -->
      <div
        v-if="localGraphExtract.enabled && localGraphExtract.nodes.length > 0"
        class="border-border flex flex-col gap-3 py-4 [&:not(:last-child)]:border-b"
      >
        <div class="max-w-[40%] pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            t("graphSettings.entityListLabel")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ t("graphSettings.entityListDescription") }}
          </p>
        </div>
        <div class="flex w-full max-w-full flex-col items-start gap-3">
          <div class="flex w-full flex-col gap-4">
            <div
              v-for="(node, nodeIndex) in localGraphExtract.nodes"
              :key="nodeIndex"
              class="border-border bg-card rounded-lg border p-4"
            >
              <div class="mb-3 flex items-center gap-3">
                <UserIcon class="text-primary size-5" />
                <Input
                  v-model="node.name"
                  :placeholder="t('graphSettings.nodeNamePlaceholder')"
                  class="flex-1"
                  @update:model-value="handleNodesChange"
                />
                <Button variant="outline" size="icon-sm" @click="removeNode(nodeIndex)">
                  <Trash2Icon />
                </Button>
              </div>
              <div class="flex flex-col gap-2 pl-8">
                <div v-for="(attribute, attrIndex) in node.attributes" :key="attrIndex" class="flex items-center gap-2">
                  <Input
                    v-model="node.attributes[attrIndex]"
                    :placeholder="t('graphSettings.attributePlaceholder')"
                    class="flex-1"
                    @update:model-value="handleNodesChange"
                  />
                  <Button variant="outline" size="icon-sm" @click="removeAttribute(nodeIndex, attrIndex)">
                    <XIcon />
                  </Button>
                </div>
                <Button variant="outline" size="sm" class="self-start" @click="addAttribute(nodeIndex)">
                  {{ t("graphSettings.addAttribute") }}
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 添加实体按钮 -->
      <div
        v-if="localGraphExtract.enabled"
        class="border-border flex items-start justify-between [&:not(:last-child)]:border-b"
        :class="embedded ? 'flex-col items-stretch gap-2 py-3' : 'py-4'"
      >
        <div :class="embedded ? 'max-w-none pr-0' : 'max-w-[40%] shrink-0 basis-2/5 pr-6'">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            t("graphSettings.manageEntitiesLabel")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ t("graphSettings.manageEntitiesDescription") }}
          </p>
        </div>
        <div :class="embedded ? 'self-start' : 'flex max-w-[55%] shrink-0 basis-[55%] items-center justify-end'">
          <Button @click="addNode">{{ t("graphSettings.addEntity") }}</Button>
        </div>
      </div>

      <!-- 关系列表 -->
      <div
        v-if="localGraphExtract.enabled && localGraphExtract.relations.length > 0"
        class="border-border flex flex-col gap-3 py-4 [&:not(:last-child)]:border-b"
      >
        <div class="max-w-[40%] pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            t("graphSettings.relationListLabel")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ t("graphSettings.relationListDescription") }}
          </p>
        </div>
        <div class="flex w-full max-w-full flex-col items-start gap-3">
          <div class="flex w-full flex-col gap-3">
            <div
              v-for="(relation, index) in localGraphExtract.relations"
              :key="index"
              class="border-border bg-card flex items-center gap-3 rounded-lg border p-3"
            >
              <Select v-model="relation.node1" @update:model-value="handleRelationsChange">
                <SelectTrigger class="min-w-[150px] flex-1">
                  <SelectValue :placeholder="t('graphSettings.selectEntity')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="node in namedNodes" :key="node.name" :value="node.name">
                    {{ node.name }}
                  </SelectItem>
                </SelectContent>
              </Select>
              <ArrowRightIcon class="text-muted-foreground size-4" />
              <Popover>
                <PopoverTrigger as-child>
                  <Button variant="outline" class="min-w-[150px] flex-1 justify-between font-normal">
                    <span class="truncate">{{ relation.type || t("graphSettings.selectRelationType") }}</span>
                    <ChevronDownIcon class="size-4 opacity-50" />
                  </Button>
                </PopoverTrigger>
                <PopoverContent class="w-[220px] p-1">
                  <div class="flex max-h-[240px] flex-col gap-px overflow-y-auto">
                    <button
                      v-for="tag in localGraphExtract.tags"
                      :key="tag"
                      type="button"
                      data-slot="option-button"
                      class="flex items-center rounded-md px-2.5 py-1.5 text-left text-[13px] transition-colors"
                      :class="
                        relation.type === tag
                          ? 'bg-primary/10 text-primary font-medium'
                          : 'text-foreground hover:bg-muted'
                      "
                      @click="
                        relation.type = tag;
                        handleRelationsChange();
                      "
                    >
                      {{ tag }}
                    </button>
                  </div>
                  <div class="border-border mt-1 flex items-center gap-1.5 border-t p-1.5">
                    <Input
                      :model-value="relationTypeDrafts[index] || ''"
                      class="h-7 flex-1 text-xs"
                      :placeholder="t('graphSettings.selectRelationType')"
                      @update:model-value="(v) => (relationTypeDrafts[index] = String(v))"
                      @keydown.enter.prevent="
                        if (relationTypeDrafts[index]?.trim()) {
                          relation.type = relationTypeDrafts[index].trim();
                          relationTypeDrafts[index] = '';
                          handleRelationsChange();
                        }
                      "
                    />
                    <Button
                      size="icon-xs"
                      variant="ghost"
                      :disabled="!relationTypeDrafts[index]?.trim()"
                      @click="
                        relation.type = relationTypeDrafts[index].trim();
                        relationTypeDrafts[index] = '';
                        handleRelationsChange();
                      "
                    >
                      <PlusIcon />
                    </Button>
                  </div>
                </PopoverContent>
              </Popover>
              <ArrowRightIcon class="text-muted-foreground size-4" />
              <Select v-model="relation.node2" @update:model-value="handleRelationsChange">
                <SelectTrigger class="min-w-[150px] flex-1">
                  <SelectValue :placeholder="t('graphSettings.selectEntity')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="node in namedNodes" :key="node.name" :value="node.name">
                    {{ node.name }}
                  </SelectItem>
                </SelectContent>
              </Select>
              <Button variant="outline" size="icon-sm" @click="removeRelation(index)">
                <Trash2Icon />
              </Button>
            </div>
          </div>
        </div>
      </div>

      <!-- 添加关系按钮 -->
      <div
        v-if="localGraphExtract.enabled"
        class="border-border flex items-start justify-between [&:not(:last-child)]:border-b"
        :class="embedded ? 'flex-col items-stretch gap-2 py-3' : 'py-4'"
      >
        <div :class="embedded ? 'max-w-none pr-0' : 'max-w-[40%] shrink-0 basis-2/5 pr-6'">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            t("graphSettings.manageRelationsLabel")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ t("graphSettings.manageRelationsDescription") }}
          </p>
        </div>
        <div :class="embedded ? 'self-start' : 'flex max-w-[55%] shrink-0 basis-[55%] items-center justify-end'">
          <Button @click="addRelation">{{ t("graphSettings.addRelation") }}</Button>
        </div>
      </div>

      <!-- 提取操作按钮 -->
      <div
        v-if="localGraphExtract.enabled"
        class="border-border flex items-start justify-between [&:not(:last-child)]:border-b"
        :class="embedded ? 'flex-col items-stretch gap-2 py-3' : 'py-4'"
      >
        <div :class="embedded ? 'max-w-none pr-0' : 'max-w-[40%] shrink-0 basis-2/5 pr-6'">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
            t("graphSettings.extractActionsLabel")
          }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
            {{ t("graphSettings.extractActionsDescription") }}
          </p>
        </div>
        <div :class="embedded ? 'self-start' : 'flex max-w-[55%] shrink-0 basis-[55%] items-center justify-end'">
          <div class="flex flex-wrap gap-3">
            <Button
              v-if="canRunGraphExtract"
              :disabled="!modelStatus.llm.available || !localGraphExtract.text || extracting"
              @click="handleExtract"
            >
              <Loader2Icon v-if="extracting" class="animate-spin" />
              {{ extracting ? t("graphSettings.extracting") : t("graphSettings.startExtraction") }}
            </Button>
            <Button variant="secondary" @click="defaultExtractExample">
              {{ t("graphSettings.defaultExample") }}
            </Button>
            <Button variant="secondary" @click="clearExtractExample">
              {{ t("graphSettings.clearExample") }}
            </Button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { docsUrl } from "@/config/externalLinks";
import { ref, watch, onMounted, computed } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import {
  ArrowRightIcon,
  ChevronDownIcon,
  CircleAlertIcon,
  CircleXIcon,
  InfoIcon,
  Loader2Icon,
  PlusIcon,
  Trash2Icon,
  UserIcon,
  XIcon,
} from "@lucide/vue";
import { extractTextRelations, fabriText, fabriTag, type Node, type Relation } from "@/api/initialization";
import { useEditorResourcesStore } from "@/stores/editorResources";
import { useAuthStore } from "@/stores/auth";
import { Alert, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";

const { t } = useI18n();
const authStore = useAuthStore();

// canRunGraphExtract 对应后端 POST /initialization/extract/{fabri-tag,fabri-text,
// text-relation} 的 g.Admin() 守卫——这三个都是会调用大模型 + 写库的 admin
// 工具。Contributor 看到按钮点了只会撞 403。
const canRunGraphExtract = computed(() => authStore.hasRole("admin"));

interface GraphExtractConfig {
  enabled: boolean;
  text: string;
  tags: string[];
  nodes: Node[];
  relations: Relation[];
  customInstructions?: string;
}

interface Props {
  graphExtract: GraphExtractConfig;
  modelId: string;
  allModels?: any[];
  embedded?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  embedded: false,
});

const emit = defineEmits<{
  "update:graphExtract": [value: GraphExtractConfig];
}>();

const modelStatus = computed(() => ({
  llm: {
    available: !!props.modelId,
  },
}));

// 本地状态
const localGraphExtract = ref<GraphExtractConfig>({
  ...props.graphExtract,
  nodes: props.graphExtract.nodes || [],
  relations: props.graphExtract.relations || [],
  customInstructions: props.graphExtract.customInstructions || "",
});

// 加载状态
const tagFabring = ref(false);
const textFabring = ref(false);
const extracting = ref(false);

// 关系类型自定义输入草稿（按关系行索引）
const relationTypeDrafts = ref<Record<number, string>>({});
const newTagInput = ref("");

// Entities a relation can point at. A freshly added entity has no name yet,
// and Reka's SelectItem rejects an empty value, so unnamed ones are left out.
const namedNodes = computed(() => localGraphExtract.value.nodes.filter((node) => node.name));

// 系统信息
const systemInfo = ref<any>(null);

// 计算图数据库是否启用
const isGraphDatabaseEnabled = computed(() => {
  return systemInfo.value?.graph_database_engine && systemInfo.value.graph_database_engine !== "Not Enabled";
});

// Watch for prop changes
watch(
  () => props.graphExtract,
  (newVal) => {
    localGraphExtract.value = {
      ...newVal,
      nodes: newVal.nodes || [],
      relations: newVal.relations || [],
      customInstructions: newVal.customInstructions || "",
    };
  },
  { deep: true },
);

// 处理配置变更
const handleConfigChange = () => {
  emit("update:graphExtract", localGraphExtract.value);
};

// 处理启用/禁用切换
const handleEnabledChange = () => {
  // 当关闭提取功能时，清空示例数据，但保留自定义指令以便再次启用时恢复。
  if (!localGraphExtract.value.enabled) {
    localGraphExtract.value.text = "";
    localGraphExtract.value.tags = [];
    localGraphExtract.value.nodes = [];
    localGraphExtract.value.relations = [];
  }
  handleConfigChange();
};

const handleTagsChange = () => {
  handleConfigChange();
};

const handleTextChange = () => {
  handleConfigChange();
};

const handleNodesChange = () => {
  handleConfigChange();
};

const handleRelationsChange = () => {
  handleConfigChange();
};

const addTag = () => {
  const value = newTagInput.value.trim();
  if (!value) return;
  if (!localGraphExtract.value.tags.includes(value)) {
    localGraphExtract.value.tags = [...localGraphExtract.value.tags, value];
    handleTagsChange();
  }
  newTagInput.value = "";
};

const removeTag = (tag: string) => {
  localGraphExtract.value.tags = localGraphExtract.value.tags.filter((t) => t !== tag);
  handleTagsChange();
};

// 节点操作
const addNode = () => {
  if (!localGraphExtract.value.nodes) {
    localGraphExtract.value.nodes = [];
  }
  localGraphExtract.value.nodes.push({
    name: "",
    attributes: [],
  });
  handleNodesChange();
};

const removeNode = (index: number) => {
  localGraphExtract.value.nodes.splice(index, 1);
  handleNodesChange();
};

const addAttribute = (nodeIndex: number) => {
  localGraphExtract.value.nodes[nodeIndex].attributes.push("");
  handleNodesChange();
};

const removeAttribute = (nodeIndex: number, attrIndex: number) => {
  localGraphExtract.value.nodes[nodeIndex].attributes.splice(attrIndex, 1);
  handleNodesChange();
};

// 关系操作
const addRelation = () => {
  if (!localGraphExtract.value.relations) {
    localGraphExtract.value.relations = [];
  }
  localGraphExtract.value.relations.push({
    node1: "",
    node2: "",
    type: "",
  });
  handleRelationsChange();
};

const removeRelation = (index: number) => {
  localGraphExtract.value.relations.splice(index, 1);
  handleRelationsChange();
};

// 生成随机标签
const handleFabriTag = async () => {
  tagFabring.value = true;
  try {
    const response = await fabriTag({});
    localGraphExtract.value.tags = response.tags || [];
    handleTagsChange();
    MessagePlugin.success(t("graphSettings.tagsGenerated"));
  } catch (error: any) {
    console.error("Failed to generate tags:", error);
    MessagePlugin.error(t("graphSettings.tagsGenerateFailed"));
  } finally {
    tagFabring.value = false;
  }
};

// 生成随机文本
const handleFabriText = async () => {
  if (!props.modelId) {
    MessagePlugin.warning(t("graphSettings.completeModelConfig"));
    return;
  }

  textFabring.value = true;
  try {
    const response = await fabriText({
      tags: localGraphExtract.value.tags,
      model_id: props.modelId,
    });
    localGraphExtract.value.text = response.text || "";
    handleTextChange();
    MessagePlugin.success(t("graphSettings.textGenerated"));
  } catch (error: any) {
    console.error("Failed to generate text:", error);
    MessagePlugin.error(t("graphSettings.textGenerateFailed"));
  } finally {
    textFabring.value = false;
  }
};

// 提取实体关系
const handleExtract = async () => {
  if (!props.modelId) {
    MessagePlugin.warning(t("graphSettings.completeModelConfig"));
    return;
  }

  if (!localGraphExtract.value.text) {
    MessagePlugin.warning(t("graphSettings.pleaseInputText"));
    return;
  }

  extracting.value = true;
  try {
    const response = await extractTextRelations({
      text: localGraphExtract.value.text,
      tags: localGraphExtract.value.tags,
      model_id: props.modelId,
    });
    localGraphExtract.value.nodes = response.nodes || [];
    localGraphExtract.value.relations = response.relations || [];
    handleNodesChange();
    MessagePlugin.success(t("graphSettings.extractSuccess"));
  } catch (error: any) {
    console.error("Failed to extract relations:", error);
    MessagePlugin.error(t("graphSettings.extractFailed"));
  } finally {
    extracting.value = false;
  }
};

// 默认示例
const defaultExtractExample = () => {
  localGraphExtract.value.text = `"Romeo and Juliet" is a tragedy written by William Shakespeare early in his career, and is one of the most frequently performed plays in world literature. The play follows two young lovers from feuding families in Verona, Italy — the Montagues and the Capulets. Written around 1594-1596, it was first published in quarto in 1597. The full title is "The Most Excellent and Lamentable Tragedy of Romeo and Juliet." The story has been adapted countless times for stage, film, and other media.`;
  localGraphExtract.value.tags = ["Author", "Alias"];
  localGraphExtract.value.nodes = [
    {
      name: "Romeo and Juliet",
      attributes: ["One of the most frequently performed plays", "Written around 1594-1596", "A tragedy"],
    },
    {
      name: "The Most Excellent and Lamentable Tragedy of Romeo and Juliet",
      attributes: ["Full title of Romeo and Juliet"],
    },
    { name: "William Shakespeare", attributes: ["English playwright", "Author of Romeo and Juliet"] },
    { name: "Verona", attributes: ["City in Italy", "Setting of the play"] },
  ];
  localGraphExtract.value.relations = [
    {
      node1: "Romeo and Juliet",
      node2: "The Most Excellent and Lamentable Tragedy of Romeo and Juliet",
      type: "Alias",
    },
    { node1: "Romeo and Juliet", node2: "William Shakespeare", type: "Author" },
    { node1: "Romeo and Juliet", node2: "Verona", type: "Setting" },
  ];
  handleNodesChange();
  MessagePlugin.success(t("graphSettings.exampleLoaded"));
};

// 清除示例
const clearExtractExample = () => {
  localGraphExtract.value.text = "";
  localGraphExtract.value.tags = [];
  localGraphExtract.value.nodes = [];
  localGraphExtract.value.relations = [];
  handleNodesChange();
  MessagePlugin.success(t("graphSettings.exampleCleared"));
};

const editorResources = useEditorResourcesStore();

// 加载系统信息
const loadSystemInfo = async (force = false) => {
  try {
    await editorResources.ensureSystemInfo(force);
    systemInfo.value = editorResources.systemInfo;
  } catch (error: any) {
    console.error("Failed to load system info:", error);
  }
};

const graphGuideUrl = import.meta.env.VITE_KG_GUIDE_URL || docsUrl("03-features/09-knowledge-graph");

// Open guide documentation to show how to enable graph database
const handleOpenGraphGuide = () => {
  if (!graphGuideUrl) return;
  window.open(graphGuideUrl, "_blank", "noopener");
};

// 初始化
onMounted(async () => {
  await loadSystemInfo();
});
</script>
