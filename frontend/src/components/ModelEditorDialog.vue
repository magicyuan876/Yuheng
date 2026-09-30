<template>
  <SettingDrawer
    :visible="dialogVisible"
    :title="isEdit ? $t('model.editor.editTitle') : $t('model.editor.addTitle')"
    :description="getModalDescription()"
    :icon="modelTypeIcon"
    :confirm-loading="saving"
    @update:visible="(v: boolean) => (dialogVisible = v)"
    @confirm="handleConfirm"
    @cancel="handleCancel"
  >
    <!--
      Footer-left slot: connection-test button lives here so it sits next to
      Save/Cancel — primary actions all aligned along the bottom of the
      drawer. Avoids the "test, then scroll back down to save" dance.
      Mirrors the pattern used in WebSearchSettings' provider drawer.
    -->
    <template v-if="formData.source === 'remote'" #footer-left>
      <Button
        variant="outline"
        :disabled="checking || !formData.modelName || !formData.baseUrl"
        @click="checkRemoteAPI"
      >
        <Loader2Icon v-if="checking" class="animate-spin" />
        <CircleCheckIcon v-else-if="remoteChecked && remoteAvailable" class="text-primary" />
        <CircleXIcon v-else-if="remoteChecked && !remoteAvailable" class="text-destructive" />
        {{ checking ? $t("model.editor.testing") : $t("model.editor.testConnection") }}
      </Button>
      <!--
        The test message truncates so a long backend error doesn't push
        Save/Cancel off-screen; the full text is in the title attribute.
      -->
      <span
        v-if="remoteChecked"
        class="min-w-0 flex-1 truncate text-xs leading-[1.4]"
        :class="remoteAvailable ? 'text-[var(--td-brand-color-active)]' : 'text-destructive'"
        :title="remoteMessage"
      >
        {{ remoteMessage }}
      </span>
    </template>

    <!--
      A plain container, not a <form>: every field is validated by hand in
      handleConfirm, and a form element would add an implicit Enter-to-submit
      the drawer never had.
    -->
    <div>
      <section v-if="!isEdit" class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ $t("model.editor.sectionType") }}</h4>
        <div class="flex flex-wrap gap-2" role="radiogroup" :aria-label="$t('model.editor.typeLabel')">
          <button
            v-for="opt in modelTypeChoices"
            :key="opt.value"
            type="button"
            data-slot="model-type-option"
            class="focus-visible:outline-primary inline-flex min-h-8 items-center gap-1.5 rounded-lg border px-3 py-1.5 text-[13px] leading-[1.4] transition-[border-color,color,background-color] duration-150 focus-visible:outline-2 focus-visible:outline-offset-2"
            :class="
              activeModelType === opt.value
                ? 'border-primary bg-primary/10 text-primary font-medium'
                : 'border-border bg-card text-muted-foreground hover:text-foreground hover:border-[var(--td-brand-color-3,var(--td-brand-color))]'
            "
            role="radio"
            :aria-checked="activeModelType === opt.value"
            @click="selectModelType(opt.value)"
          >
            <component :is="opt.icon" class="size-[15px] shrink-0" />
            <span class="whitespace-nowrap">{{ opt.label }}</span>
          </button>
        </div>
      </section>

      <!--
        Section 1 — 模型来源 + 模型名称（来源直接决定下方字段，所以放一节）
      -->
      <section class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ $t("model.editor.sectionSource") }}</h4>

        <div>
          <!--
            Section title already says 「模型来源」，所以这里不再重复 label，
            直接把分段控件作为 section 的首个内容呈现，避免「双标题」感。

            模型来源分段：紧凑单行 pill 形 segmented。容器自身是浅底圆角条，
            选中按钮通过实色背景 + 主题色描边浮出，未选中态接近透明，节省纵向空间。
          -->
          <div
            class="border-border inline-flex items-center gap-1 rounded-lg border bg-[var(--td-bg-color-component)] p-[3px]"
            role="radiogroup"
            :aria-label="$t('model.editor.sourceLabel')"
          >
            <button
              type="button"
              data-slot="source-option"
              :class="[sourceOptionClass, formData.source === 'remote' ? sourceActiveClass : sourceIdleClass]"
              role="radio"
              :aria-checked="formData.source === 'remote'"
              @click="formData.source = 'remote'"
            >
              <CloudIcon class="size-3.5 shrink-0" />
              <span class="whitespace-nowrap">{{ $t("model.editor.sourceRemote") }}</span>
            </button>
            <button
              type="button"
              data-slot="source-option"
              :class="[
                sourceOptionClass,
                formData.source === 'local' ? sourceActiveClass : sourceIdleClass,
                { 'cursor-not-allowed opacity-45 hover:bg-transparent': localSourceDisabled },
              ]"
              :disabled="localSourceDisabled"
              role="radio"
              :aria-checked="formData.source === 'local'"
              @click="formData.source = 'local'"
            >
              <ServerIcon class="size-3.5 shrink-0" />
              <span class="whitespace-nowrap">{{ $t("model.editor.sourceLocal") }}</span>
            </button>
          </div>

          <!-- ReRank模型不支持Ollama的提示信息（使用主题绿色风格，与主页面保持一致） -->
          <div
            v-if="activeModelType === 'rerank'"
            class="border-l-primary mt-3 flex items-center gap-2 rounded-lg border border-l-[3px] border-[var(--td-success-color-focus)] bg-[var(--td-success-color-light)] px-3 py-2.5 text-[13px]"
          >
            <InfoIcon class="text-primary mr-0.5 size-4 shrink-0" />
            <span class="text-success flex-1 leading-normal">{{ $t("model.editor.ollamaNotSupportRerank") }}</span>
          </div>

          <!-- Ollama不可用时的提示信息 -->
          <div
            v-else-if="shouldShowOllamaUnavailableTip(formData.source, activeModelType, ollamaServiceStatus)"
            class="mt-3 flex items-center gap-2 rounded-lg border border-[var(--td-error-color-focus)] bg-[var(--td-error-color-light)] px-3 py-2.5 text-[13px]"
          >
            <CircleAlertIcon class="text-destructive mr-0.5 size-4 shrink-0" />
            <span class="text-destructive flex-1 leading-normal">{{ $t("model.editor.ollamaUnavailable") }}</span>
            <Button
              variant="ghost"
              class="text-primary h-auto gap-px rounded-[4px] py-1 pr-1.5 pl-2.5 text-[13px] leading-[1.4] font-medium whitespace-nowrap hover:bg-[rgba(7,192,95,0.08)] hover:text-[var(--td-brand-color-active)] active:bg-[rgba(7,192,95,0.12)]"
              @click="goToOllamaSettings"
            >
              <ArrowUpRightIcon class="size-3.5" />
              {{ $t("model.editor.goToOllamaSettings") }}
            </Button>
          </div>
        </div>

        <!-- Ollama 本地模型选择器 -->
        <div v-if="formData.source === 'local'">
          <label :class="[labelClass, requiredClass]">{{ $t("model.modelName") }}</label>
          <div class="flex items-center gap-2">
            <!--
              The list is filtered here rather than by the select, because the
              keyword also decides whether to offer "download <keyword>".
              Opening the list loads it, as focusing the old select did.
            -->
            <SearchableSelect
              v-model="formData.modelName"
              v-model:keyword="searchKeyword"
              class="flex-1"
              :options="ollamaOptions"
              :filter="false"
              :loading="loadingOllamaModels"
              :progress="downloading ? downloadProgress : null"
              :placeholder="$t('model.searchPlaceholder')"
              @open-change="onOllamaSelectOpenChange"
            >
              <template #option="{ option }">
                <template v-if="option.download">
                  <DownloadIcon class="text-primary size-3.5 shrink-0" />
                  <span class="text-primary flex-1 font-medium">{{ option.label }}</span>
                </template>
                <template v-else>
                  <CircleCheckIcon class="text-primary size-3.5 shrink-0" />
                  <span class="text-foreground flex-1">{{ option.label }}</span>
                  <span class="text-placeholder ml-auto text-xs">{{ option.size }}</span>
                </template>
              </template>

              <!-- 下载进度后缀 -->
              <template v-if="downloading" #suffix>
                <span class="text-primary flex items-center gap-1 px-1">
                  <Loader2Icon class="size-3.5 animate-spin" />
                  <span class="text-xs font-medium">{{ downloadProgress.toFixed(1) }}%</span>
                </span>
              </template>
            </SearchableSelect>

            <!-- 刷新按钮 -->
            <Button
              variant="ghost"
              size="sm"
              class="shrink-0"
              :disabled="loadingOllamaModels"
              @click="refreshOllamaModels"
            >
              <Loader2Icon v-if="loadingOllamaModels" class="animate-spin" />
              <RefreshCwIcon v-else />
              {{ $t("model.editor.refreshList") }}
            </Button>
          </div>
        </div>
      </section>

      <!-- Remote API 配置 -->
      <template v-if="formData.source === 'remote'">
        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t("model.editor.sectionProvider") }}</h4>

          <!-- 厂商选择器 -->
          <div>
            <label :class="labelClass">{{ $t("model.editor.providerLabel") }}</label>
            <Select
              :model-value="formData.provider"
              @update:model-value="
                (v) => {
                  formData.provider = String(v ?? '');
                  handleProviderChange(formData.provider);
                }
              "
            >
              <SelectTrigger class="w-full text-[13px]">
                <!-- The box shows the name only; the options below add a description line. -->
                <SelectValue :placeholder="$t('model.editor.providerPlaceholder')">
                  {{ selectedProviderLabel }}
                </SelectValue>
              </SelectTrigger>
              <!-- z-[5500]: above the drawer, which sits at z-[2500]. -->
              <SelectContent position="popper" class="z-[5500] max-h-[360px] p-1">
                <SelectItem v-for="opt in providerOptions" :key="opt.value" :value="opt.value" :class="richOptionClass">
                  <span class="flex w-full min-w-0 flex-col gap-0.5">
                    <span
                      class="text-foreground group-data-[state=checked]/option:text-primary text-[13px] leading-5 font-medium"
                    >
                      {{ opt.label }}
                    </span>
                    <span class="text-placeholder truncate text-xs leading-[18px]">{{ opt.description }}</span>
                  </span>
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <!-- 模型名称 -->
          <div>
            <label :class="[labelClass, requiredClass]">{{ $t("model.modelName") }}</label>
            <Input
              :model-value="formData.modelName"
              :class="inputClass"
              :placeholder="getModelNamePlaceholder()"
              @update:model-value="(v) => (formData.modelName = String(v))"
            />
          </div>

          <div>
            <label :class="labelClass">{{ $t("model.editor.displayNameLabel") }}</label>
            <Input
              :model-value="formData.displayName"
              :class="inputClass"
              :placeholder="$t('model.editor.displayNamePlaceholder')"
              @update:model-value="(v) => (formData.displayName = String(v))"
            />
            <p :class="descClass">{{ $t("model.editor.displayNameDesc") }}</p>
          </div>

          <div>
            <label :class="[labelClass, requiredClass]">{{ $t("model.editor.baseUrlLabel") }}</label>
            <Input
              :model-value="formData.baseUrl"
              :class="inputClass"
              :placeholder="getBaseUrlPlaceholder()"
              @update:model-value="(v) => (formData.baseUrl = String(v))"
            />
          </div>

          <div>
            <label :class="labelClass">{{
              isSignedRerank ? signedRerankAccessKeyLabel : $t("model.editor.apiKeyOptional")
            }}</label>
            <!--
              Edit mode: credentials live behind the /credentials subresource
              of the model — managed by the shared CredentialResource card,
              which renders an INPUT-LOOKING row (32px tall, same border
              + radius as an input) so it sits flush with the Base URL field
              above and the 自定义请求头 controls below — no more
              "card inside a card" feel.
              Create mode: the resource doesn't exist yet, so we render a
              plain password input with a leading lock icon and a trailing
              show/hide eye toggle. Both icons use the placeholder colour; the
              eye turns to the text colour on hover so it doesn't steal focus.
            -->
            <CredentialResource
              v-if="isEdit && props.modelData?.id"
              :api="credentialApi"
              :fields="credentialFields"
              :meta="credentialMeta"
            />
            <div v-else class="relative">
              <LockIcon
                class="text-placeholder pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
              />
              <Input
                :model-value="formData.apiKey"
                :type="showApiKey ? 'text' : 'password'"
                :placeholder="isSignedRerank ? signedRerankAccessKeyPlaceholder : apiKeyPlaceholder"
                :class="[inputClass, 'pr-9 pl-8']"
                autocomplete="off"
                spellcheck="false"
                @update:model-value="(v) => (formData.apiKey = String(v))"
              />
              <button
                type="button"
                data-slot="api-key-toggle"
                class="text-placeholder hover:text-foreground absolute top-1/2 right-2.5 flex -translate-y-1/2 items-center transition-colors duration-150"
                :aria-label="showApiKey ? 'Hide' : 'Show'"
                @click.stop="showApiKey = !showApiKey"
              >
                <EyeOffIcon v-if="showApiKey" class="size-4" />
                <EyeIcon v-else class="size-4" />
              </button>
            </div>
            <p v-if="isSignedRerank" :class="descClass">{{ signedRerankCredentialHint }}</p>
          </div>

          <!-- AK/SK Rerank 创建模式：SecretKey（编辑模式由 CredentialResource 管理） -->
          <div v-if="isSignedRerank && !isEdit">
            <label :class="[labelClass, requiredClass]">{{ signedRerankSecretKeyLabel }}</label>
            <div class="relative">
              <LockIcon
                class="text-placeholder pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
              />
              <Input
                :model-value="formData.appSecret"
                type="password"
                :placeholder="signedRerankSecretKeyPlaceholder"
                :class="[inputClass, 'pl-8']"
                autocomplete="off"
                spellcheck="false"
                @update:model-value="(v) => (formData.appSecret = String(v))"
              />
            </div>
          </div>

          <div v-if="isLkeapRerank">
            <label :class="labelClass">{{ $t("model.editor.lkeap.regionLabel") }}</label>
            <Input
              :model-value="formData.lkeapRegion"
              :class="inputClass"
              :placeholder="$t('model.editor.lkeap.regionPlaceholder')"
              @update:model-value="(v) => (formData.lkeapRegion = String(v))"
            />
            <p :class="descClass">{{ $t("model.editor.lkeap.regionDesc") }}</p>
          </div>

          <!-- 自定义 HTTP Header（类似 OpenAI Python SDK 的 extra_headers） -->
          <div>
            <div class="mb-1.5 flex items-center justify-between">
              <label :class="[labelClass, 'mb-0']">{{ $t("model.editor.customHeadersLabel") }}</label>
              <Button variant="ghost" size="sm" class="text-primary hover:text-primary" @click="addCustomHeader">
                <PlusIcon />
                {{ $t("model.editor.customHeadersAdd") }}
              </Button>
            </div>
            <p :class="[descClass, 'mt-0 mb-2.5']">{{ $t("model.editor.customHeadersDesc") }}</p>
            <div v-if="formData.customHeaders && formData.customHeaders.length > 0" class="flex flex-col gap-2">
              <div v-for="(item, idx) in formData.customHeaders" :key="idx" class="flex items-center gap-2">
                <Input
                  :model-value="item.key"
                  :placeholder="$t('model.editor.customHeadersKeyPlaceholder')"
                  :class="[inputClass, 'w-auto flex-[0_0_38%]']"
                  @update:model-value="(v) => (item.key = String(v))"
                />
                <Input
                  :model-value="item.value"
                  :placeholder="$t('model.editor.customHeadersValuePlaceholder')"
                  :class="[inputClass, 'flex-1']"
                  @update:model-value="(v) => (item.value = String(v))"
                />
                <!--
                  Ghost icon button — matches the model-card "more" affordance:
                  quiet until hover/focus, then a subtle background pops in.
                  Avoids painting a permanent red splotch next to every row.
                -->
                <Button
                  variant="ghost"
                  size="icon"
                  class="text-placeholder hover:text-destructive shrink-0 rounded-[6px] transition-all duration-200 hover:bg-[var(--td-error-color-light)]"
                  :aria-label="$t('common.delete')"
                  @click="removeCustomHeader(idx)"
                >
                  <XIcon />
                </Button>
              </div>
            </div>
          </div>

          <!--
            Connection test action moved to the drawer footer (footer-left
            slot above) so primary actions live in one row at the bottom.
          -->
        </section>
      </template>

      <!-- Section 3 — 高级选项（仅在有内容时渲染，避免空 section 出现底部分隔线） -->
      <section v-if="['embedding', 'chat', 'vllm'].includes(activeModelType)" class="setting-drawer__section">
        <h4 class="setting-drawer__section-title">{{ $t("model.editor.sectionAdvanced") }}</h4>

        <!-- Embedding 专用：维度 -->
        <div v-if="activeModelType === 'embedding'">
          <label :class="labelClass">{{ $t("model.editor.dimensionLabel") }}</label>
          <div class="flex items-center gap-2">
            <Input
              type="number"
              :model-value="formData.dimension ?? ''"
              :min="128"
              :max="4096"
              :class="[inputClass, 'flex-1']"
              :placeholder="$t('model.editor.dimensionPlaceholder')"
              :disabled="!formData.supportsDimensionOverride || (formData.source === 'local' && checking)"
              @update:model-value="(v) => (formData.dimension = optionalNumber(v))"
            />
            <!-- Ollama 本地模型：自动检测维度按钮 -->
            <Button
              v-if="formData.source === 'local' && formData.modelName"
              variant="ghost"
              size="sm"
              class="shrink-0"
              :disabled="checking"
              @click="checkOllamaDimension"
            >
              <Loader2Icon v-if="checking" class="animate-spin" />
              <RefreshCwIcon v-else />
              {{ $t("model.editor.checkDimension") }}
            </Button>
          </div>
          <p
            v-if="dimensionChecked && dimensionMessage"
            class="mt-2 mb-0 text-[13px] leading-normal"
            :class="dimensionSuccess ? 'text-primary' : 'text-destructive'"
          >
            {{ dimensionMessage }}
          </p>
        </div>

        <div v-if="activeModelType === 'embedding'">
          <label :class="labelClass">{{ $t("model.editor.dimensionOverrideLabel") }}</label>
          <div class="flex items-center gap-2">
            <Switch v-model="formData.supportsDimensionOverride" />
            <span :class="inlineDescClass">{{ $t("model.editor.dimensionOverrideDesc") }}</span>
          </div>
        </div>

        <!-- Chat: supports vision toggle (VLLM models are inherently multimodal) -->
        <div v-if="activeModelType === 'chat'">
          <label :class="labelClass">{{ $t("model.editor.supportsVisionLabel") }}</label>
          <div class="flex items-center gap-2">
            <Switch v-model="formData.supportsVision" />
            <span :class="inlineDescClass">{{ $t("model.editor.supportsVisionDesc") }}</span>
          </div>
        </div>

        <!-- Chat + 远程 API：思考模式参数格式 -->
        <div v-if="showThinkingControlField">
          <label :class="labelClass">{{ $t("model.editor.thinkingControlLabel") }}</label>
          <Select
            :key="`thinking-${formData.id}-${formData.thinkingControl}`"
            :model-value="formData.thinkingControl"
            @update:model-value="
              (v) => {
                formData.thinkingControl = String(v ?? '');
                onThinkingControlManualPick();
              }
            "
          >
            <SelectTrigger class="w-full text-[13px]">
              <SelectValue>{{ selectedThinkingControlLabel }}</SelectValue>
            </SelectTrigger>
            <SelectContent
              position="popper"
              class="z-[5500] w-auto max-w-[min(28rem,calc(100vw-2rem))] min-w-[22rem] p-1"
            >
              <SelectItem
                v-for="opt in thinkingControlOptions"
                :key="opt.value"
                :value="opt.value"
                :class="richOptionClass"
              >
                <span class="flex min-w-0 flex-col gap-0.5 leading-[1.35] whitespace-normal">
                  <span class="text-foreground text-[13px]">{{ opt.label }}</span>
                  <span class="text-placeholder text-xs break-words">{{ opt.hint }}</span>
                </span>
              </SelectItem>
            </SelectContent>
          </Select>
          <p :class="descClass">{{ $t("model.editor.thinkingControlDesc") }}</p>
        </div>

        <!--
          Background concurrency cap for this model. Only chat / embedding / vllm
          are gated by the governor (see internal/models/limiter), so we surface
          it just for those three. 0 = fall back to the global default.
        -->
        <div>
          <label :class="labelClass">{{ $t("model.editor.maxConcurrencyLabel") }}</label>
          <Input
            type="number"
            :model-value="formData.maxConcurrency ?? ''"
            :min="0"
            :max="4096"
            :class="inputClass"
            :placeholder="$t('model.editor.maxConcurrencyPlaceholder')"
            @update:model-value="(v) => (formData.maxConcurrency = optionalNumber(v))"
          />
          <p :class="descClass">{{ $t("model.editor.maxConcurrencyDesc") }}</p>
        </div>
      </section>
    </div>
  </SettingDrawer>
</template>

<script setup lang="ts">
import { ref, watch, computed, onUnmounted, nextTick } from "vue";
import type { Component } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import {
  checkRemoteModel,
  testEmbeddingModel,
  checkRerankModel,
  checkASRModel,
  listOllamaModels,
  downloadOllamaModel,
  getDownloadProgress,
  checkOllamaStatus,
  listModelProviders,
  type OllamaModelInfo,
  type ModelProviderOption,
} from "@/api/initialization";
import { putModelCredentials, deleteModelCredentialField, type ModelCredentialField } from "@/api/model";
import { useI18n } from "vue-i18n";
import { useUIStore } from "@/stores/ui";
import { defaultThinkingControl, resolveThinkingControl, type ThinkingControlValue } from "@/utils/thinkingControl";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import CredentialResource, {
  type CredentialFieldDef,
  type CredentialResourceApi,
} from "@/components/credentials/CredentialResource.vue";
import { shouldShowOllamaUnavailableTip } from "@/components/modelEditorSourceState";
import SearchableSelect from "@/components/SearchableSelect.vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import {
  ArrowUpRightIcon,
  BubblesIcon,
  CircleAlertIcon,
  CircleCheckIcon,
  CircleXIcon,
  CloudIcon,
  DownloadIcon,
  EyeIcon,
  EyeOffIcon,
  FilterIcon,
  ImageIcon,
  InfoIcon,
  Loader2Icon,
  LockIcon,
  MessageSquareIcon,
  PlusIcon,
  RefreshCwIcon,
  ServerIcon,
  SettingsIcon,
  Volume2Icon,
  XIcon,
} from "@lucide/vue";

interface CustomHeaderItem {
  key: string;
  value: string;
}

interface ModelFormData {
  id: string;
  name: string;
  source: "local" | "remote";
  provider?: string; // Provider identifier: openai, aliyun, zhipu, generic, etc.
  modelName: string;
  displayName?: string;
  baseUrl?: string;
  apiKey?: string;
  dimension?: number;
  supportsDimensionOverride?: boolean;
  interfaceType?: "ollama" | "openai";
  isDefault: boolean;
  supportsVision?: boolean;
  /** 后台任务对该模型的并发上限；0/undefined 表示沿用全局默认。仅 chat/embedding/vllm 生效。 */
  maxConcurrency?: number;
  /** extra_config.thinking_control — how agent thinking on/off maps to API fields. */
  thinkingControl?: string;
  // 自定义 HTTP 请求头（类似 OpenAI Python SDK 的 extra_headers）
  customHeaders?: CustomHeaderItem[];
  /** LKEAP Rerank：腾讯云 SecretKey（创建时写入 app_secret） */
  appSecret?: string;
  /** LKEAP Rerank：地域，如 ap-guangzhou */
  lkeapRegion?: string;
}

type EditorModelType = "chat" | "embedding" | "rerank" | "vllm" | "asr";

interface Props {
  visible: boolean;
  modelType: EditorModelType;
  modelData?: ModelFormData | null;
}

const { t, te } = useI18n();
const uiStore = useUIStore();

const props = withDefaults(defineProps<Props>(), {
  visible: false,
  modelData: null,
});

const emit = defineEmits<{
  "update:visible": [value: boolean];
  confirm: [data: ModelFormData & { modelType?: EditorModelType }];
}>();

const draftModelType = ref<EditorModelType>(props.modelType);

const isEdit = computed(() => !!props.modelData);

const activeModelType = computed(() => (isEdit.value ? props.modelType : draftModelType.value));

// The same icons as the model cards in ModelSettings, so the type picked
// here reads the same as the card the model ends up on.
const MODEL_TYPE_ICONS: Record<EditorModelType, Component> = {
  chat: MessageSquareIcon,
  embedding: BubblesIcon,
  rerank: FilterIcon,
  vllm: ImageIcon,
  asr: Volume2Icon,
};

const modelTypeChoices = computed(() => [
  { value: "chat" as const, label: t("modelSettings.typeShort.chat"), icon: MODEL_TYPE_ICONS.chat },
  { value: "embedding" as const, label: t("modelSettings.typeShort.embedding"), icon: MODEL_TYPE_ICONS.embedding },
  { value: "rerank" as const, label: t("modelSettings.typeShort.rerank"), icon: MODEL_TYPE_ICONS.rerank },
  { value: "vllm" as const, label: t("modelSettings.typeShort.vllm"), icon: MODEL_TYPE_ICONS.vllm },
  { value: "asr" as const, label: t("modelSettings.typeShort.asr"), icon: MODEL_TYPE_ICONS.asr },
]);

// API 返回的 Provider 列表
const apiProviderOptions = ref<ModelProviderOption[]>([]);
const loadingProviders = ref(false);

// 硬编码的后备 Provider 配置 (当 API 不可用时使用)
const fallbackProviderOptions = computed(() => [
  {
    value: "openai",
    label: t("model.editor.providers.openai.label"),
    defaultUrls: {
      chat: "https://api.openai.com/v1",
      embedding: "https://api.openai.com/v1",
      rerank: "https://api.openai.com/v1",
      vllm: "https://api.openai.com/v1",
      asr: "https://api.openai.com/v1",
    },
    description: t("model.editor.providers.openai.description"),
    modelTypes: ["chat", "embedding", "vllm", "asr"],
  },
  {
    value: "azure_openai",
    label: t("model.editor.providers.azure_openai.label"),
    defaultUrls: {
      chat: "https://{resource}.openai.azure.com",
      embedding: "https://{resource}.openai.azure.com",
      vllm: "https://{resource}.openai.azure.com",
      asr: "https://{resource}.openai.azure.com",
    },
    description: t("model.editor.providers.azure_openai.description"),
    modelTypes: ["chat", "embedding", "vllm", "asr"],
  },
  {
    value: "aliyun",
    label: t("model.editor.providers.aliyun.label"),
    defaultUrls: {
      chat: "https://dashscope.aliyuncs.com/compatible-mode/v1",
      embedding: "https://dashscope.aliyuncs.com/compatible-mode/v1",
      rerank: "https://dashscope.aliyuncs.com/api/v1/services/rerank/text-rerank/text-rerank",
      vllm: "https://dashscope.aliyuncs.com/compatible-mode/v1",
    },
    description: t("model.editor.providers.aliyun.description"),
    modelTypes: ["chat", "embedding", "rerank", "vllm"],
  },
  {
    value: "zhipu",
    label: t("model.editor.providers.zhipu.label"),
    defaultUrls: {
      chat: "https://open.bigmodel.cn/api/paas/v4",
      embedding: "https://open.bigmodel.cn/api/paas/v4/embeddings",
      vllm: "https://open.bigmodel.cn/api/paas/v4",
    },
    description: t("model.editor.providers.zhipu.description"),
    modelTypes: ["chat", "embedding", "vllm"],
  },
  {
    value: "openrouter",
    label: t("model.editor.providers.openrouter.label"),
    defaultUrls: {
      chat: "https://openrouter.ai/api/v1",
      embedding: "https://openrouter.ai/api/v1",
    },
    description: t("model.editor.providers.openrouter.description"),
    modelTypes: ["chat", "embedding"],
  },
  {
    value: "requesty",
    label: t("model.editor.providers.requesty.label"),
    defaultUrls: {
      chat: "https://router.requesty.ai/v1",
      embedding: "https://router.requesty.ai/v1",
    },
    description: t("model.editor.providers.requesty.description"),
    modelTypes: ["chat", "embedding"],
  },
  {
    value: "gemini",
    label: t("model.editor.providers.gemini.label"),
    defaultUrls: {
      chat: "https://generativelanguage.googleapis.com/v1beta/openai",
      embedding: "https://generativelanguage.googleapis.com/v1beta",
    },
    description: t("model.editor.providers.gemini.description"),
    modelTypes: ["chat", "embedding"],
  },
  {
    value: "siliconflow",
    label: t("model.editor.providers.siliconflow.label"),
    defaultUrls: {
      chat: "https://api.siliconflow.cn/v1",
      embedding: "https://api.siliconflow.cn/v1",
      rerank: "https://api.siliconflow.cn/v1",
    },
    description: t("model.editor.providers.siliconflow.description"),
    modelTypes: ["chat", "embedding", "rerank"],
  },
  {
    value: "jina",
    label: t("model.editor.providers.jina.label"),
    defaultUrls: {
      embedding: "https://api.jina.ai/v1",
      rerank: "https://api.jina.ai/v1",
    },
    description: t("model.editor.providers.jina.description"),
    modelTypes: ["embedding", "rerank"],
  },
  {
    value: "nvidia",
    label: t("model.editor.providers.nvidia.label"),
    defaultUrls: {
      chat: "https://integrate.api.nvidia.com/v1",
      embedding: "https://integrate.api.nvidia.com/v1",
      rerank: "https://ai.api.nvidia.com/v1/retrieval/nvidia/reranking",
      vllm: "https://integrate.api.nvidia.com/v1",
    },
    description: t("model.editor.providers.nvidia.description"),
    modelTypes: ["chat", "embedding", "rerank", "vllm"],
  },
  {
    value: "novita",
    label: t("model.editor.providers.novita.label"),
    defaultUrls: {
      chat: "https://api.novita.ai/openai/v1",
      embedding: "https://api.novita.ai/openai/v1",
      vllm: "https://api.novita.ai/openai/v1",
    },
    description: t("model.editor.providers.novita.description"),
    modelTypes: ["chat", "embedding", "vllm"],
  },
  {
    value: "generic",
    label: t("model.editor.providers.generic.label"),
    defaultUrls: {},
    description: t("model.editor.providers.generic.description"),
    modelTypes: ["chat", "embedding", "rerank", "vllm", "asr"],
  },
]);

// 从 API 获取 Provider 列表
const loadProviders = async () => {
  loadingProviders.value = true;
  try {
    const providers = await listModelProviders(activeModelType.value);
    if (providers.length > 0) {
      apiProviderOptions.value = providers;
    }
  } catch (error) {
    console.error("Failed to load providers from API, using fallback", error);
  } finally {
    loadingProviders.value = false;
  }
};

// 根据当前模型类型过滤的 Provider 列表
// API 返回的 defaultUrls/modelTypes 数据优先，但 label/description 使用 i18n
/**
 * 不在下拉中提供的服务商：用户无法新建该类型模型，已存在的模型仍可正常使用。
 * 目前为空 —— 保留这个机制，以便将来需要下架某个服务商时无需改动过滤逻辑。
 */
const HIDDEN_PROVIDERS = new Set<string>();

const providerOptions = computed(() => {
  // API 数据可用时，用 API 的结构数据 + i18n 的显示文本
  if (apiProviderOptions.value.length > 0) {
    return apiProviderOptions.value
      .filter((p) => !HIDDEN_PROVIDERS.has(p.value))
      .map((p) => ({
        ...p,
        label: te(`model.editor.providers.${p.value}.label`) ? t(`model.editor.providers.${p.value}.label`) : p.label,
        description: te(`model.editor.providers.${p.value}.description`)
          ? t(`model.editor.providers.${p.value}.description`)
          : p.description,
      }));
  }
  // 回退到硬编码值，按 modelTypes 过滤
  return fallbackProviderOptions.value.filter(
    (p) => !HIDDEN_PROVIDERS.has(p.value) && p.modelTypes.includes(activeModelType.value),
  );
});

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit("update:visible", val),
});

const showThinkingControlField = computed(() => activeModelType.value === "chat" && formData.value.source === "remote");

const resolvedThinkingControl = (): ThinkingControlValue =>
  defaultThinkingControl(formData.value.provider || "", formData.value.modelName || "");

/** 用户是否手动改过思考参数格式（改过则不再自动覆盖，直到换服务商） */
const thinkingControlManual = ref(false);
/** 正在从 modelData 灌入表单，忽略厂商/来源控件的程序化 change 副作用 */
const hydratingForm = ref(false);

const onThinkingControlManualPick = () => {
  thinkingControlManual.value = true;
};

const syncThinkingControlToForm = (force = false) => {
  if (!showThinkingControlField.value) return;
  if (!force && !isEdit.value && thinkingControlManual.value) return;
  formData.value.thinkingControl = resolvedThinkingControl();
};

const applyThinkingControlFromModelData = () => {
  if (!props.modelData || activeModelType.value !== "chat" || formData.value.source !== "remote") return;
  thinkingControlManual.value = !!props.modelData.thinkingControl;
  formData.value.thinkingControl = resolveThinkingControl(
    props.modelData.thinkingControl,
    formData.value.provider || props.modelData.provider || "",
    formData.value.modelName || props.modelData.modelName || "",
  );
};

const thinkingControlOptions = computed(() => {
  const keys = ["none", "chatTemplateKwargs", "enableThinking", "thinkingType"] as const;
  const values = ["none", "chat_template_kwargs", "enable_thinking", "thinking_type"] as const;
  return keys.map((key, i) => ({
    value: values[i],
    label: t(`model.editor.thinkingControl.${key}.label`),
    hint: t(`model.editor.thinkingControl.${key}.hint`),
  }));
});

// Header icon for the SettingDrawer — uses the same icon table as the model
// card list, so the drawer's leading badge visually matches the card the
// user just clicked on.
const modelTypeIcon = computed<Component>(() => MODEL_TYPE_ICONS[activeModelType.value] ?? SettingsIcon);

// The name shown in the provider box. The options themselves carry a
// description line, which the closed select should not repeat.
const selectedProviderLabel = computed(
  () => providerOptions.value.find((opt) => opt.value === formData.value.provider)?.label ?? formData.value.provider,
);

const selectedThinkingControlLabel = computed(
  () =>
    thinkingControlOptions.value.find((opt) => opt.value === formData.value.thinkingControl)?.label ??
    formData.value.thinkingControl,
);

const isLkeapRerank = computed(() => activeModelType.value === "rerank" && formData.value.provider === "lkeap");
const isVolcengineRerank = computed(
  () => activeModelType.value === "rerank" && formData.value.provider === "volcengine",
);
const isSignedRerank = computed(() => isLkeapRerank.value || isVolcengineRerank.value);
const signedRerankAccessKeyLabel = computed(() =>
  isVolcengineRerank.value ? t("model.editor.volcengine.accessKeyLabel") : t("model.editor.lkeap.secretIdLabel"),
);
const signedRerankAccessKeyPlaceholder = computed(() =>
  isVolcengineRerank.value
    ? t("model.editor.volcengine.accessKeyPlaceholder")
    : t("model.editor.lkeap.secretIdPlaceholder"),
);
const signedRerankSecretKeyLabel = computed(() =>
  isVolcengineRerank.value ? t("model.editor.volcengine.secretKeyLabel") : t("model.editor.lkeap.secretKeyLabel"),
);
const signedRerankSecretKeyPlaceholder = computed(() =>
  isVolcengineRerank.value
    ? t("model.editor.volcengine.secretKeyPlaceholder")
    : t("model.editor.lkeap.secretKeyPlaceholder"),
);
const signedRerankCredentialHint = computed(() =>
  isVolcengineRerank.value
    ? t("model.editor.volcengine.rerankCredentialHint")
    : t("model.editor.lkeap.rerankCredentialHint"),
);

// Credential resource binding for the shared <CredentialResource> component.
const credentialFields = computed<CredentialFieldDef<ModelCredentialField>[]>(() => {
  const fields: CredentialFieldDef<ModelCredentialField>[] = [
    {
      key: "api_key",
      label: (isSignedRerank.value ? signedRerankAccessKeyLabel.value : t("model.editor.apiKeyOptional")) as string,
    },
  ];
  if (isSignedRerank.value) {
    fields.push({ key: "app_secret", label: signedRerankSecretKeyLabel.value as string });
  }
  return fields;
});

const credentialApi = computed<CredentialResourceApi<ModelCredentialField>>(() => {
  const id = props.modelData?.id ?? "";
  return {
    save: async (patch) => {
      const meta = await putModelCredentials(id, patch);
      return meta.fields;
    },
    remove: async (field) => {
      await deleteModelCredentialField(id, field);
    },
  };
});

// Initial credential metadata. ModelSettings.toModelView
// preserves `credentials` from the main ListModels response so the card
// renders the correct "Configured" state on dialog open.
const credentialMeta = computed(
  () =>
    (props.modelData as any)?.credentials ?? {
      api_key: { configured: false },
      app_secret: { configured: false },
    },
);

// Placeholder hint for the create-mode API key input. Edit mode replaces
// this input entirely with a <CredentialResource> card.
const apiKeyPlaceholder = computed(() => t("model.editor.apiKeyPlaceholder"));

const saving = ref(false);
// Toggles the create-mode API key input between masked and plain text. Lets
// the user proofread a freshly pasted secret without losing the password
// affordance for everyday use. Reset every time the drawer closes (see
// reset block in the visible watcher) so we never leak the previous value
// across editor sessions.
const showApiKey = ref(false);
const modelChecked = ref(false);
const modelAvailable = ref(false);
const checking = ref(false);
const remoteChecked = ref(false);
const remoteAvailable = ref(false);
const remoteMessage = ref("");
const dimensionChecked = ref(false);
const dimensionSuccess = ref(false);
const dimensionMessage = ref("");

// Ollama 模型状态
const ollamaModelList = ref<OllamaModelInfo[]>([]);
const loadingOllamaModels = ref(false);
const searchKeyword = ref("");
const downloading = ref(false);
const downloadProgress = ref(0);
const currentDownloadModel = ref("");
let downloadInterval: any = null;

// Ollama 服务状态
const ollamaServiceStatus = ref<boolean | null>(null);
const checkingOllamaStatus = ref(false);

const formData = ref<ModelFormData>({
  id: "",
  name: "",
  source: "remote",
  provider: "generic",
  modelName: "",
  displayName: "",
  baseUrl: "",
  apiKey: "",
  dimension: undefined,
  supportsDimensionOverride: false,
  interfaceType: "ollama",
  isDefault: false,
  supportsVision: false,
  maxConcurrency: undefined,
  thinkingControl: defaultThinkingControl("generic", ""),
  customHeaders: [],
  appSecret: "",
  lkeapRegion: "ap-guangzhou",
});

// Validation lives in handleConfirm, which checks the model name and, for a
// remote model, the base URL, with the same messages the old TDesign form
// rules declared. Those rules were never attached to a form item, so they
// had not been running; the explicit checks were the ones users saw.

// 获取弹窗描述文字
const getModalDescription = () => {
  const key = `model.editor.description.${activeModelType.value}` as const;
  return t(key) || t("model.editor.description.default");
};

// 获取模型名称占位符
const getModelNamePlaceholder = () => {
  if (activeModelType.value === "vllm") {
    return formData.value.source === "local"
      ? t("model.editor.modelNamePlaceholder.localVllm")
      : t("model.editor.modelNamePlaceholder.remoteVllm");
  }
  if (activeModelType.value === "asr") {
    return t("model.editor.modelNamePlaceholder.remoteAsr");
  }
  return formData.value.source === "local"
    ? t("model.editor.modelNamePlaceholder.local")
    : t("model.editor.modelNamePlaceholder.remote");
};

const getBaseUrlPlaceholder = () => {
  if (activeModelType.value === "vllm") {
    return t("model.editor.baseUrlPlaceholderVllm");
  }
  if (activeModelType.value === "asr") {
    return t("model.editor.baseUrlPlaceholderAsr");
  }
  return t("model.editor.baseUrlPlaceholder");
};

// 检查Ollama服务状态
const checkOllamaServiceStatus = async () => {
  console.log("开始检查Ollama服务状态...");
  checkingOllamaStatus.value = true;
  try {
    const result = await checkOllamaStatus();
    ollamaServiceStatus.value = result.available;
    console.log("Ollama服务状态检查完成:", result.available);
  } catch (error) {
    console.error("检查Ollama服务状态失败:", error);
    ollamaServiceStatus.value = false;
  } finally {
    checkingOllamaStatus.value = false;
  }

  // Ollama 不可用时，新增场景下默认切换到 remote
  if (ollamaServiceStatus.value === false && !isEdit.value && formData.value.source === "local") {
    formData.value.source = "remote";
  }
};

// 打开Ollama设置窗口
const goToOllamaSettings = async () => {
  console.log("点击跳转到Ollama设置按钮");
  // 关闭当前弹窗
  emit("update:visible", false);

  // 先关闭设置弹窗（如果已打开）
  if (uiStore.showSettingsModal) {
    uiStore.closeSettings();
    // 等待 DOM 更新
    await nextTick();
  }

  // 打开设置窗口并直接跳转到Ollama设置
  console.log("调用uiStore.openSettings");
  uiStore.openSettings("ollama");
  console.log("uiStore.openSettings调用完成");
};

// 上一次打开时的 modelData id：用来判断切换模型/新增 vs. 同一次新增的连续打开
const lastOpenedModelId = ref<string | null>(null);

const selectModelType = async (type: EditorModelType) => {
  if (isEdit.value || draftModelType.value === type) return;
  draftModelType.value = type;

  if (type === "rerank") {
    formData.value.source = "remote";
  }
  if (type !== "embedding") {
    formData.value.dimension = undefined;
    formData.value.supportsDimensionOverride = false;
    dimensionChecked.value = false;
    dimensionSuccess.value = false;
    dimensionMessage.value = "";
  }
  if (type !== "chat") {
    formData.value.supportsVision = false;
    thinkingControlManual.value = false;
  }
  remoteChecked.value = false;
  remoteAvailable.value = false;
  remoteMessage.value = "";

  await loadProviders();
  const supported = providerOptions.value.some((p) => p.value === formData.value.provider);
  if (!supported) {
    formData.value.provider = "generic";
    formData.value.baseUrl = "";
  } else {
    handleProviderChange(formData.value.provider || "generic");
  }
  if (showThinkingControlField.value && !isEdit.value) {
    thinkingControlManual.value = false;
    syncThinkingControlToForm(true);
  }
};

// 监听 visible 变化，初始化表单
watch(
  () => props.visible,
  (val) => {
    if (val) {
      // 检查Ollama服务状态
      checkOllamaServiceStatus();

      // 从 API 加载 Model Provider 列表
      loadProviders();

      // 每次打开都清理上一次遗留的校验/检测结果，避免编辑别的模型时
      // 直接显示上一次的“连接成功”
      modelChecked.value = false;
      modelAvailable.value = false;
      remoteChecked.value = false;
      remoteAvailable.value = false;
      remoteMessage.value = "";
      dimensionChecked.value = false;
      dimensionSuccess.value = false;
      dimensionMessage.value = "";

      const currentId = props.modelData?.id ?? null;
      draftModelType.value = props.modelType;

      hydratingForm.value = true;
      try {
        if (props.modelData) {
          // 编辑：始终用最新的 modelData 覆盖。apiKey field is left blank — in
          // edit mode the credential is owned by the <CredentialResource> card,
          // not by this form's apiKey field.
          formData.value = {
            ...props.modelData,
            apiKey: "",
            customHeaders: Array.isArray(props.modelData.customHeaders)
              ? props.modelData.customHeaders.map((h) => ({ key: h.key, value: h.value }))
              : [],
          };
          applyThinkingControlFromModelData();
        } else if (lastOpenedModelId.value !== null || !formData.value.id) {
          // 上次是编辑某个模型，或第一次新增 → 重置成空白
          resetForm();
        }
        // 否则：连续两次"新增"打开（中间是点遮罩/ESC 关闭的）→ 保留上次填写

        lastOpenedModelId.value = currentId;

        // ReRank 模型强制使用 remote 来源（Ollama 不支持 ReRank）
        if (activeModelType.value === "rerank") {
          formData.value.source = "remote";
        }

        if (showThinkingControlField.value && !isEdit.value) {
          thinkingControlManual.value = false;
          syncThinkingControlToForm(true);
        }
      } finally {
        nextTick(() => {
          hydratingForm.value = false;
        });
      }
    }
  },
);

// 重置表单
const resetForm = () => {
  thinkingControlManual.value = false;
  formData.value = {
    id: generateId(),
    name: "", // 保留字段但不使用，保存时用 modelName
    source: "remote",
    provider: "generic",
    modelName: "",
    displayName: "",
    baseUrl: "",
    apiKey: "",
    dimension: undefined, // 默认不填，让用户手动输入或通过检测按钮获取
    supportsDimensionOverride: false,
    interfaceType: undefined,
    isDefault: false,
    supportsVision: false,
    maxConcurrency: undefined,
    thinkingControl: defaultThinkingControl("generic", ""),
    customHeaders: [],
    appSecret: "",
    lkeapRegion: "ap-guangzhou",
  };
  modelChecked.value = false;
  modelAvailable.value = false;
  remoteChecked.value = false;
  remoteAvailable.value = false;
  remoteMessage.value = "";
  dimensionChecked.value = false;
  dimensionSuccess.value = false;
  dimensionMessage.value = "";
  showApiKey.value = false;
};

// 处理厂商选择变化 (自动填充默认 URL)
const handleProviderChange = (value: string) => {
  const provider = providerOptions.value.find((opt) => opt.value === value);
  if (provider && provider.defaultUrls) {
    // 根据当前模型类型获取对应的默认 URL
    const defaultUrl = provider.defaultUrls[activeModelType.value];
    if (defaultUrl) {
      formData.value.baseUrl = defaultUrl;
    }
    if (value === "lkeap" && activeModelType.value === "rerank" && !formData.value.modelName?.trim()) {
      formData.value.modelName = "lke-reranker-base";
    }
    if (value === "volcengine" && activeModelType.value === "rerank" && !formData.value.modelName?.trim()) {
      formData.value.modelName = "doubao-seed-rerank";
    }
    // 重置校验状态
    remoteChecked.value = false;
    remoteAvailable.value = false;
    remoteMessage.value = "";
  }
  if (hydratingForm.value) return;
  if (activeModelType.value !== "chat" || formData.value.source !== "remote") return;
  if (!isEdit.value) {
    thinkingControlManual.value = false;
    syncThinkingControlToForm(true);
    return;
  }
  // 编辑时仅用户主动换厂商才跟随默认
  thinkingControlManual.value = false;
  syncThinkingControlToForm(true);
};

watch(
  () => [formData.value.source, formData.value.provider, formData.value.modelName] as const,
  ([source, provider, modelName], [prevSource, prevProvider, prevModelName]) => {
    if (hydratingForm.value || isEdit.value) return;
    if (activeModelType.value !== "chat" || source !== "remote") return;
    if (source === prevSource && provider === prevProvider && modelName === prevModelName) return;

    const providerChanged = provider !== prevProvider;

    if (providerChanged) {
      thinkingControlManual.value = false;
      syncThinkingControlToForm(true);
      return;
    }
    if (!thinkingControlManual.value) {
      syncThinkingControlToForm(true);
      return;
    }
    const prevDefault = defaultThinkingControl(prevProvider || "", prevModelName || "");
    if (formData.value.thinkingControl === prevDefault) {
      syncThinkingControlToForm(true);
    }
  },
);

// 监听来源变化，重置校验状态（已合并到下面的 watch）

// 生成唯一ID
const generateId = () => {
  return `model_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
};

// 自定义 HTTP Header 编辑
const addCustomHeader = () => {
  if (!Array.isArray(formData.value.customHeaders)) {
    formData.value.customHeaders = [];
  }
  formData.value.customHeaders.push({ key: "", value: "" });
};

const removeCustomHeader = (idx: number) => {
  if (!Array.isArray(formData.value.customHeaders)) return;
  formData.value.customHeaders.splice(idx, 1);
};

// 过滤后的模型列表
const filteredOllamaModels = computed(() => {
  if (!searchKeyword.value) return ollamaModelList.value;
  return ollamaModelList.value.filter((model) => model.name.toLowerCase().includes(searchKeyword.value.toLowerCase()));
});

// 是否显示"下载模型"选项
const showDownloadOption = computed(() => {
  if (!searchKeyword.value.trim()) return false;
  // 检查搜索词是否已存在于模型列表中
  const exists = ollamaModelList.value.some((model) => model.name.toLowerCase() === searchKeyword.value.toLowerCase());
  return !exists;
});

// The Ollama picker's options: the downloaded models matching the keyword,
// then "download <keyword>" when the keyword names none of them. The
// download entry's value carries the prefix the modelName watcher looks for.
const ollamaOptions = computed(() => {
  const options: Array<{ value: string; label: string; size?: string; download?: boolean }> =
    filteredOllamaModels.value.map((model) => ({
      value: model.name,
      label: model.name,
      size: formatModelSize(model.size),
    }));
  if (showDownloadOption.value) {
    options.push({
      value: `__download__${searchKeyword.value}`,
      label: t("model.editor.downloadLabel", { keyword: searchKeyword.value }),
      download: true,
    });
  }
  return options;
});

// Opening the list loads the models (the old select did it on focus);
// closing it clears the keyword.
const onOllamaSelectOpenChange = (open: boolean) => {
  if (open) void loadOllamaModels();
  handleDropdownVisibleChange(open);
};

// 加载 Ollama 模型列表
const loadOllamaModels = async () => {
  // 只在选择 local 来源时加载
  if (formData.value.source !== "local") return;

  loadingOllamaModels.value = true;
  try {
    const models = await listOllamaModels();
    ollamaModelList.value = models;
  } catch (error) {
    console.error(t("model.editor.loadModelListFailed"), error);
    MessagePlugin.error(t("model.editor.loadModelListFailed"));
  } finally {
    loadingOllamaModels.value = false;
  }
};

// 刷新模型列表
const refreshOllamaModels = async () => {
  ollamaModelList.value = []; // 清空以强制重新加载
  await loadOllamaModels();
  MessagePlugin.success(t("model.editor.listRefreshed"));
};

// 监听下拉框可见性变化
const handleDropdownVisibleChange = (visible: boolean) => {
  if (!visible) {
    searchKeyword.value = "";
  }
};

// 格式化模型大小
function formatModelSize(bytes: number): string {
  if (!bytes || bytes === 0) return "";
  const gb = bytes / (1024 * 1024 * 1024);
  return gb >= 1 ? `${gb.toFixed(1)} GB` : `${(bytes / (1024 * 1024)).toFixed(0)} MB`;
}

// 检查模型状态（Ollama本地模型）

// 检查 Ollama 本地 Embedding 模型维度
const checkOllamaDimension = async () => {
  if (!formData.value.modelName || formData.value.source !== "local" || activeModelType.value !== "embedding") {
    return;
  }

  checking.value = true;
  dimensionChecked.value = false;
  dimensionMessage.value = "";

  try {
    const result = await testEmbeddingModel({
      source: "local",
      modelName: formData.value.modelName,
      dimension: formData.value.dimension,
      supportsDimensionOverride: formData.value.supportsDimensionOverride ?? false,
    });

    dimensionChecked.value = true;
    dimensionSuccess.value = result.available || false;

    if (result.available && result.dimension) {
      formData.value.dimension = result.dimension;
      dimensionMessage.value = t("model.editor.dimensionDetected", { value: result.dimension });
      MessagePlugin.success(dimensionMessage.value);
    } else {
      if (result.message) {
        console.debug("Backend dimension message:", result.message);
      }
      dimensionMessage.value = t("model.editor.dimensionFailed");
      MessagePlugin.warning(dimensionMessage.value);
    }
  } catch (error: any) {
    console.error("Ollama dimension check failed:", error);
    dimensionChecked.value = true;
    dimensionSuccess.value = false;
    dimensionMessage.value = t("model.editor.dimensionFailed");
    MessagePlugin.error(dimensionMessage.value);
  } finally {
    checking.value = false;
  }
};

// 检查 Remote API 连接（根据模型类型调用不同的接口）
const checkRemoteAPI = async () => {
  if (!formData.value.modelName || !formData.value.baseUrl) {
    MessagePlugin.warning(t("model.editor.fillModelAndUrl"));
    return;
  }

  checking.value = true;
  remoteChecked.value = false;
  remoteMessage.value = "";

  try {
    let result: any;

    // 把表单里 Key-Value 数组形式的自定义 Header 转成后端期望的 map。
    // 跟 ModelSettings.vue 保存时一致，空行自动丢弃，保证测试连接与真正保存后的
    // 生产调用使用完全相同的 Header 集合。
    const customHeaders: Record<string, string> = {};
    if (Array.isArray(formData.value.customHeaders)) {
      for (const item of formData.value.customHeaders) {
        const key = (item?.key ?? "").trim();
        const value = (item?.value ?? "").trim();
        if (key && value) customHeaders[key] = value;
      }
    }
    // 只在非空时带上字段，避免在 URL query / 日志里出现空对象
    const headerPayload = Object.keys(customHeaders).length > 0 ? { customHeaders } : {};

    // 根据模型类型调用不同的校验接口
    // 编辑模式下 apiKey 由 <CredentialResource> 独立管理、不在 formData 里。
    // 把 modelId 透传给后端，让它在 apiKey 为空时自动用存储的解密值兜底，
    // 避免出现"测试连接没带 apiKey 直接失败"的情况。
    const idPayload = isEdit.value && props.modelData?.id ? { modelId: props.modelData.id as string } : {};

    switch (activeModelType.value) {
      case "chat":
        // 对话模型（KnowledgeQA）
        result = await checkRemoteModel({
          modelName: formData.value.modelName,
          baseUrl: formData.value.baseUrl || "",
          apiKey: formData.value.apiKey || "",
          provider: formData.value.provider,
          ...idPayload,
          ...headerPayload,
        });
        break;

      case "embedding":
        // Embedding 模型
        result = await testEmbeddingModel({
          source: "remote",
          modelName: formData.value.modelName,
          baseUrl: formData.value.baseUrl || "",
          apiKey: formData.value.apiKey || "",
          dimension: formData.value.dimension,
          supportsDimensionOverride: formData.value.supportsDimensionOverride ?? false,
          provider: formData.value.provider,
          ...idPayload,
          ...headerPayload,
        });
        // 如果测试成功且返回了维度，自动填充
        if (result.available && result.dimension) {
          formData.value.dimension = result.dimension;
          MessagePlugin.info(t("model.editor.remoteDimensionDetected", { value: result.dimension }));
        }
        break;

      case "rerank": {
        const signedRerankExtra = isSignedRerank.value
          ? {
              ...(isLkeapRerank.value
                ? {
                    extraConfig: {
                      region: (formData.value.lkeapRegion || "ap-guangzhou").trim(),
                    },
                  }
                : {}),
              ...(formData.value.appSecret?.trim() ? { appSecret: formData.value.appSecret.trim() } : {}),
            }
          : {};
        result = await checkRerankModel({
          modelName: formData.value.modelName,
          baseUrl: formData.value.baseUrl || "",
          apiKey: formData.value.apiKey || "",
          provider: formData.value.provider,
          ...idPayload,
          ...headerPayload,
          ...signedRerankExtra,
        });
        break;
      }

      case "vllm":
        // VLLM 模型（多模态）
        // VLLM 使用 checkRemoteModel 进行基础连接测试
        result = await checkRemoteModel({
          modelName: formData.value.modelName,
          baseUrl: formData.value.baseUrl || "",
          apiKey: formData.value.apiKey || "",
          provider: formData.value.provider,
          ...idPayload,
          ...headerPayload,
        });
        break;

      case "asr":
        // ASR 模型（语音识别）— 使用专用的 ASR 测试接口（/v1/audio/transcriptions）
        result = await checkASRModel({
          modelName: formData.value.modelName,
          baseUrl: formData.value.baseUrl || "",
          apiKey: formData.value.apiKey || "",
          provider: formData.value.provider,
          ...idPayload,
          ...headerPayload,
        });
        break;

      default:
        MessagePlugin.error(t("model.editor.unsupportedModelType"));
        return;
    }

    remoteChecked.value = true;
    remoteAvailable.value = result.available || false;
    // 之前这里把 backend 的错误 message 只丢到 console.debug，用户只能
    // 看到通用的 "连接失败" toast，根本看不出是 401 / 404 / 模型不存在
    // 还是别的什么。改成：成功时用 i18n 通用提示；失败时直接展示后端
    // 给到的具体原因（已经在后端 classifyConnectionError 中包了一层
    // 易读的中文 hint + 原始 SDK 报错），方便排查。
    if (result.available) {
      remoteMessage.value = t("model.editor.connectionSuccess");
      MessagePlugin.success(remoteMessage.value);
    } else {
      remoteMessage.value = result.message || t("model.editor.connectionFailed");
      console.debug("Backend message:", result.message);
      MessagePlugin.error(remoteMessage.value);
    }
  } catch (error: any) {
    console.error("Remote API check failed:", error);
    remoteChecked.value = true;
    remoteAvailable.value = false;
    // 后端 4xx/5xx（如 SSRF 校验失败）会走到这里。axios 拦截器把后端
    // { error: { message: "..." } } 提到了 error.message，里面已经包含
    // 易读 hint + 原因，直接展示出来，比通用 "请检查配置" 有用得多。
    remoteMessage.value = error?.message || t("model.editor.connectionConfigError");
    MessagePlugin.error(remoteMessage.value);
  } finally {
    checking.value = false;
  }
};

// 确认保存
const handleConfirm = async () => {
  try {
    // 手动校验必填字段
    if (!formData.value.modelName || !formData.value.modelName.trim()) {
      MessagePlugin.warning(t("model.editor.validation.modelNameRequired"));
      return;
    }

    if (formData.value.modelName.trim().length > 100) {
      MessagePlugin.warning(t("model.editor.validation.modelNameMax"));
      return;
    }

    // remote 类型必须填写 baseUrl
    if (formData.value.source === "remote") {
      if (!formData.value.baseUrl || !formData.value.baseUrl.trim()) {
        MessagePlugin.warning(t("model.editor.remoteBaseUrlRequired"));
        return;
      }

      // 校验 Base URL 格式
      try {
        new URL(formData.value.baseUrl.trim());
      } catch {
        MessagePlugin.warning(t("model.editor.validation.baseUrlInvalid"));
        return;
      }
    }

    // Credential removal in edit mode is handled inline by the
    // CredentialResource card (it confirms + DELETEs to /credentials), so
    // the main save flow no longer needs to confirm or handle clear flags.

    saving.value = true;

    // 如果是新增且没有 id，生成一个
    if (!formData.value.id) {
      formData.value.id = generateId();
    }

    emit("confirm", {
      ...formData.value,
      ...(isEdit.value ? {} : { modelType: activeModelType.value }),
    });
    dialogVisible.value = false;
    // 保存成功后重置草稿，下次打开新增模型时是空白
    resetForm();
    lastOpenedModelId.value = null;
    // 移除此处的成功提示，由父组件统一处理
  } catch (error) {
    console.error("表单验证失败:", error);
  } finally {
    saving.value = false;
  }
};

// 监听模型选择变化（处理下载逻辑和自动维度检测提示）
watch(
  () => formData.value.modelName,
  async (newValue, oldValue) => {
    if (!newValue) return;

    // 处理下载逻辑
    if (newValue.startsWith("__download__")) {
      // 提取模型名称
      const modelName = newValue.replace("__download__", "");

      // 重置选择（避免显示 __download__ 前缀）
      formData.value.modelName = "";

      // 开始下载
      await startDownload(modelName);
      return;
    }

    // 如果是 embedding 模型且选择的是 Ollama 本地模型，且模型名称发生了实际变化
    if (
      activeModelType.value === "embedding" &&
      formData.value.source === "local" &&
      newValue !== oldValue &&
      oldValue !== ""
    ) {
      // 提示用户可以检测维度
      MessagePlugin.info(t("model.editor.dimensionHint"));
    }
  },
);

// 开始下载模型
const startDownload = async (modelName: string) => {
  downloading.value = true;
  downloadProgress.value = 0;
  currentDownloadModel.value = modelName;

  try {
    // 启动下载
    const result = await downloadOllamaModel(modelName);
    const taskId = result.taskId;

    MessagePlugin.success(t("model.editor.downloadStarted", { name: modelName }));

    // 轮询下载进度
    downloadInterval = setInterval(async () => {
      try {
        const progress = await getDownloadProgress(taskId);
        downloadProgress.value = progress.progress;

        if (progress.status === "completed") {
          // 下载完成
          clearInterval(downloadInterval);
          downloadInterval = null;
          downloading.value = false;

          MessagePlugin.success(t("model.editor.downloadCompleted", { name: modelName }));

          // 刷新模型列表
          await loadOllamaModels();

          // 自动选中新下载的模型
          formData.value.modelName = modelName;

          // 重置状态
          downloadProgress.value = 0;
          currentDownloadModel.value = "";
        } else if (progress.status === "failed") {
          // 下载失败
          clearInterval(downloadInterval);
          downloadInterval = null;
          downloading.value = false;
          MessagePlugin.error(progress.message || t("model.editor.downloadFailed", { name: modelName }));
          downloadProgress.value = 0;
          currentDownloadModel.value = "";
        }
      } catch (error) {
        console.error("获取下载进度失败:", error);
      }
    }, 1000); // 每秒查询一次
  } catch (error: any) {
    downloading.value = false;
    downloadProgress.value = 0;
    currentDownloadModel.value = "";
    console.error("Download start failed:", error);
    MessagePlugin.error(t("model.editor.downloadStartFailed"));
  }
};

// 组件卸载时清理定时器
onUnmounted(() => {
  if (downloadInterval) {
    clearInterval(downloadInterval);
  }
});

// 监听来源变化，清理所有状态
watch(
  () => formData.value.source,
  () => {
    // 重置校验状态
    modelChecked.value = false;
    modelAvailable.value = false;
    remoteChecked.value = false;
    remoteAvailable.value = false;
    remoteMessage.value = "";
    dimensionChecked.value = false;
    dimensionSuccess.value = false;
    dimensionMessage.value = "";

    // 清理下载状态
    searchKeyword.value = "";
    if (downloadInterval) {
      clearInterval(downloadInterval);
      downloadInterval = null;
    }
    downloading.value = false;
    downloadProgress.value = 0;
    currentDownloadModel.value = "";

    if (
      !hydratingForm.value &&
      !isEdit.value &&
      formData.value.source === "remote" &&
      activeModelType.value === "chat"
    ) {
      thinkingControlManual.value = false;
      syncThinkingControlToForm(true);
    }
  },
);

// 监听模型名称变化，清理维度检测状态
watch(
  () => formData.value.modelName,
  () => {
    dimensionChecked.value = false;
    dimensionSuccess.value = false;
    dimensionMessage.value = "";
  },
);

// The local source is off while Ollama is unreachable, and for rerank,
// which Ollama does not serve.
const localSourceDisabled = computed(() => ollamaServiceStatus.value === false || activeModelType.value === "rerank");

// A number field's value, or undefined when it is cleared, so an empty box
// means "not set" rather than the string "".
const optionalNumber = (value: string | number): number | undefined => {
  if (value === "") return undefined;
  const n = typeof value === "number" ? value : Number(value);
  return Number.isFinite(n) ? n : undefined;
};

// ---------- shared class lists ----------
// Form rows: label, required marker (a leading asterisk, as TDesign's
// required form items drew it), helper text, and inputs at 13px.
const labelClass = "text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium";
const requiredClass = "before:text-destructive before:mr-1 before:leading-none before:font-medium before:content-['*']";
const descClass = "text-placeholder mt-1 mb-0 text-xs leading-normal";
const inlineDescClass = "text-placeholder text-xs leading-normal";
const inputClass = "text-[13px] md:text-[13px]";

// The source segmented control's buttons.
const sourceOptionClass =
  "inline-flex h-7 items-center gap-1.5 rounded-md border px-3 py-[5px] text-[13px] leading-none transition-all duration-150";
const sourceActiveClass = "bg-card border-primary text-primary font-medium shadow-[0_1px_2px_rgba(15,23,42,0.04)]";
const sourceIdleClass = "text-muted-foreground hover:text-foreground hover:bg-accent border-transparent";

// Two-line select options (provider, thinking control). The selected one
// gets a light brand background and a leading brand bar instead of a full
// grey fill.
const richOptionClass =
  "group/option relative my-0.5 h-auto rounded-md py-2 pr-8 pl-2.5 transition-colors duration-150 " +
  "data-[state=checked]:bg-[var(--td-brand-color-light)] data-[state=checked]:font-medium " +
  "data-[state=checked]:before:absolute data-[state=checked]:before:top-2 data-[state=checked]:before:bottom-2 " +
  "data-[state=checked]:before:left-0 data-[state=checked]:before:w-[3px] data-[state=checked]:before:rounded-r-[2px] " +
  "data-[state=checked]:before:bg-primary data-[state=checked]:before:content-['']";

// 取消（点击底部"取消"按钮触发；点遮罩/ESC 不触发，从而保留草稿）
const handleCancel = () => {
  resetForm();
  lastOpenedModelId.value = null;
  dialogVisible.value = false;
};
</script>
