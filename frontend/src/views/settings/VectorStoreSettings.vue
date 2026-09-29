<template>
  <div class="w-full">
    <div class="mb-8">
      <h2 class="text-foreground mt-0 mb-2 text-xl font-semibold">{{ t("vectorStoreSettings.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-normal">{{ t("vectorStoreSettings.description") }}</p>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="flex justify-center py-12">
      <Loader2Icon class="animate-spin" />
    </div>

    <template v-else>
      <div class="flex flex-col">
        <h3 class="text-foreground m-0 mb-4 text-base font-semibold">{{ t("vectorStoreSettings.storesTitle") }}</h3>

        <!-- 与其它 settings 列表同形：左侧 engine 徽章 + 标题 + env pill + 副标题 + 测试动作。
             env 来源是只读的 (engine_type / connection_config 由 .env 写入），所以没有更多菜单；
             user 来源沿用三点菜单的编辑 / 删除入口；测试结果作为卡片底部的彩色条出现。 -->
        <div v-if="stores.length === 0 && !authStore.hasRole('admin')" class="py-16 text-center">
          <Empty>
            <EmptyDescription class="text-placeholder mb-4 text-sm">{{
              t("vectorStoreSettings.emptyDesc")
            }}</EmptyDescription>
          </Empty>
        </div>
        <div v-else class="grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-3">
          <div
            v-for="store in [...envStores, ...userStores]"
            :key="store.id"
            class="store-card border-border min-w-0 rounded-[10px] border py-3.5 pr-3.5 pl-3 transition-[border-color,box-shadow] duration-[180ms] ease-out"
            :class="[
              `store-card--${store.engine_type}`,
              {
                'bg-secondary': store.source === 'env',
                'bg-card': store.source !== 'env',
                'cursor-pointer hover:border-[var(--td-brand-color-3,var(--td-brand-color))] hover:shadow-[0_4px_14px_rgba(15,23,42,0.06)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--td-brand-color)]':
                  isStoreCardClickable(store),
              },
            ]"
            :role="isStoreCardClickable(store) ? 'button' : undefined"
            :tabindex="isStoreCardClickable(store) ? 0 : undefined"
            @click="onStoreCardClick($event, store)"
            @keydown.enter="onStoreCardClick($event, store)"
          >
            <div class="flex min-w-0 items-start gap-3">
              <div
                class="store-card__badge mt-px flex size-9 shrink-0 items-center justify-center rounded-[9px] bg-[rgba(0,82,217,0.1)] text-[15px] font-semibold tracking-[0.02em] text-[#0052d9]"
                :class="badgeClass(store.engine_type)"
                :style="badgeStyle(store.engine_type)"
                :aria-label="store.engine_type"
              >
                <img
                  v-if="resolveLogo(store.engine_type)?.mode === 'color'"
                  :src="resolveLogo(store.engine_type)!.url"
                  :alt="store.engine_type"
                  class="block size-6 object-contain"
                />
                <template v-else-if="!resolveLogo(store.engine_type)">{{ engineInitial(store.engine_type) }}</template>
              </div>
              <div class="flex min-w-0 flex-1 flex-col gap-1">
                <div class="flex min-w-0 items-center gap-1.5">
                  <h3
                    class="text-foreground m-0 min-w-0 flex-1 truncate text-sm leading-[1.4] font-semibold"
                    :title="store.name"
                  >
                    {{ store.name }}
                  </h3>
                  <span
                    v-if="store.source === 'env'"
                    class="shrink-0 rounded-[3px] bg-[var(--td-warning-color-1,#fef3e6)] px-1.5 py-px text-[11px] leading-4 font-medium text-[var(--td-warning-color-7,#b85c00)]"
                  >
                    {{ t("vectorStoreSettings.envTag") }}
                  </span>
                  <Badge
                    v-if="store.is_builtin"
                    variant="outline"
                    class="border-primary/40 bg-primary/10 text-primary"
                    :title="t('platformSharing.badgeHint')"
                    >{{ t("platformSharing.badge") }}</Badge
                  >
                  <!--
                    测试连接已挪到编辑抽屉的 footer，外层菜单不再有"测试"入口。
                    env 来源（.env 写入）也不需要 dropdown — 没有可执行的动作。
                  -->
                  <!-- `store-card__actions` is a hook: onStoreCardClick ignores keys pressed inside it. -->
                  <div
                    v-if="authStore.hasRole('admin') && storeActionsFor(store).length > 0"
                    class="store-card__actions shrink-0"
                    @click.stop
                  >
                    <DropdownMenu>
                      <DropdownMenuTrigger as-child>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          class="store-card__more text-placeholder hover:text-foreground focus-visible:text-foreground shrink-0 p-0.5 opacity-0 transition-opacity duration-150 hover:bg-[var(--td-bg-color-secondarycontainer)] focus-visible:bg-[var(--td-bg-color-secondarycontainer)]"
                          :aria-label="t('docs.tree.moreActions')"
                        >
                          <EllipsisIcon />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem
                          v-for="action in storeActionsFor(store)"
                          :key="action.value"
                          @select="handleAction(action, store)"
                        >
                          <span :class="action.theme === 'error' ? 'text-destructive' : ''">{{ action.content }}</span>
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </div>
                <div class="text-muted-foreground flex min-w-0 flex-wrap items-center gap-1 text-xs leading-[1.4]">
                  <span class="font-medium">{{ store.engine_type }}</span>
                  <template v-if="getStoreEndpoint(store)">
                    <span class="text-placeholder">·</span>
                    <span
                      class="text-placeholder min-w-0 truncate font-[ui-monospace,SFMono-Regular,'SF_Mono',Menlo,Consolas,monospace] text-[11px] whitespace-nowrap"
                      :title="getStoreEndpoint(store)"
                      >{{ getStoreEndpoint(store) }}</span
                    >
                  </template>
                </div>
              </div>
            </div>
          </div>
          <button
            v-if="authStore.hasRole('admin')"
            type="button"
            data-slot="add-store-card"
            class="border-border text-placeholder hover:border-primary hover:text-primary hover:bg-primary/6 focus-visible:border-primary focus-visible:text-primary focus-visible:bg-primary/6 focus-visible:outline-primary flex h-full min-h-[68px] w-full cursor-pointer flex-col items-center justify-center gap-2 rounded-[10px] border border-dashed bg-transparent text-center transition-all duration-[180ms] ease-out focus-visible:outline-2 focus-visible:outline-offset-2"
            @click="openAddDialog"
          >
            <span
              class="bg-primary/10 text-primary flex size-8 items-center justify-center rounded-lg"
              aria-hidden="true"
            >
              <PlusIcon class="size-[18px]" />
            </span>
            <span class="text-[13px] leading-[1.4] font-medium">{{ t("vectorStoreSettings.addStore") }}</span>
          </button>
        </div>
      </div>
    </template>

    <!-- Add/Edit Drawer — 与 ModelEditorDialog/Storage/Parser/WebSearch 同款 -->
    <SettingDrawer
      v-model:visible="showDialog"
      :title="editingStore ? t('vectorStoreSettings.editStore') : t('vectorStoreSettings.addStore')"
      :class="drawerClass"
      :confirm-loading="saving"
      @confirm="onDrawerConfirm"
      @cancel="showDialog = false"
    >
      <!--
        Header icon — 与列表 .store-card__badge 同款 logo/mono/fallback。
        per-engine 配色由非 scoped 块的 .vectorstore-drawer--{engine} 注入。
      -->
      <template v-if="form.engine_type" #headerIcon>
        <img
          v-if="drawerLogo?.mode === 'color'"
          :src="drawerLogo.url"
          :alt="form.engine_type"
          class="block size-6 object-contain"
        />
        <span
          v-else-if="drawerLogo?.mode === 'mono'"
          class="header-icon__mono inline-block size-[22px]"
          :style="drawerLogoStyle"
        />
        <span v-else class="text-[15px] font-semibold tracking-[0.02em]">{{ engineInitial(form.engine_type) }}</span>
      </template>

      <!-- 副标题：engine display_name -->
      <template v-if="selectedType" #subtitle>
        <span>{{ selectedType.display_name || form.engine_type }}</span>
      </template>

      <!--
        Test connection (footer-left). create 模式：实时验证当前表单的连接信息；
        edit 模式：用存储的连接配置（连接配置在编辑模式不可改 — engine 是 immutable）。
        始终显示按钮，由 canTestConnection 控制 disabled。
      -->
      <template #footer-left>
        <Button variant="outline" :disabled="testing || !canTestConnection" @click="onDrawerTest">
          <CircleCheckIcon v-if="!testing && lastTestOk === true" class="text-primary size-4 shrink-0" />
          <CircleXIcon v-else-if="!testing && lastTestOk === false" class="text-destructive size-4 shrink-0" />
          <Loader2Icon v-if="testing" class="animate-spin" />
          {{ testing ? t("vectorStoreSettings.testing") : t("vectorStoreSettings.testConnection") }}
        </Button>
      </template>

      <!-- `setting-drawer__section` / `__section-title` are styled by SettingDrawer
           (spacing, dividers, the brand bar before each title); the fields sit
           directly in the section, which spaces them. -->
      <div>
        <!--
          Edit 模式特殊提示：engine_type / connection_config / index_config
          创建后不可改，仅 name 可编辑。用 inline-alert 而不是大块 banner，
          视觉与其他抽屉的提示一致。
        -->
        <section v-if="editingStore" class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">
            {{ t("vectorStoreSettings.basicSection", "基本信息") }}
          </h4>

          <div class="text-foreground flex flex-wrap items-center gap-2 text-[13px] leading-normal whitespace-pre-line">
            <InfoIcon class="text-primary size-[15px] shrink-0" />
            <span class="min-w-0 flex-1">{{ t("vectorStoreSettings.immutableNotice") }}</span>
          </div>

          <div>
            <label
              class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
            >
              {{ t("vectorStoreSettings.nameLabel") }}
            </label>
            <SettingsInput
              v-model="form.name"
              :placeholder="t('vectorStoreSettings.namePlaceholder')"
              input-class="text-[13px] md:text-[13px]"
            />
            <p v-if="fieldErrors.name" class="text-destructive m-0 mt-1 text-xs">{{ fieldErrors.name }}</p>
          </div>

          <!-- 只读字段以 inline list 展示（轻量 readonly 行） -->
          <div class="bg-secondary rounded-lg px-3 py-2.5">
            <div class="border-border flex items-baseline gap-2 border-b py-1 text-xs leading-[1.4] last:border-b-0">
              <span class="text-placeholder min-w-[80px] text-[11px] whitespace-nowrap">{{
                t("vectorStoreSettings.engineTypeLabel")
              }}</span>
              <span
                class="text-foreground font-[ui-monospace,SFMono-Regular,'SF_Mono',Menlo,Consolas,monospace] break-all"
              >
                {{ selectedType?.display_name || editingStore.engine_type }}
              </span>
            </div>
            <template v-if="selectedType">
              <template v-for="field in selectedType.connection_fields" :key="field.name">
                <div
                  v-if="field.sensitive || form.connection_config[field.name]"
                  class="border-border flex items-baseline gap-2 border-b py-1 text-xs leading-[1.4] last:border-b-0"
                >
                  <span class="text-placeholder min-w-[80px] text-[11px] whitespace-nowrap">{{
                    fieldLabel(field.name)
                  }}</span>
                  <span
                    class="text-foreground font-[ui-monospace,SFMono-Regular,'SF_Mono',Menlo,Consolas,monospace] break-all"
                  >
                    {{ field.sensitive ? "********" : form.connection_config[field.name] }}
                  </span>
                </div>
              </template>
            </template>
            <template v-if="selectedType?.index_fields?.length">
              <template v-for="field in selectedType.index_fields" :key="field.name">
                <div
                  v-if="form.index_config[field.name]"
                  class="border-border flex items-baseline gap-2 border-b py-1 text-xs leading-[1.4] last:border-b-0"
                >
                  <span class="text-placeholder min-w-[80px] text-[11px] whitespace-nowrap">{{
                    fieldLabel(field.name)
                  }}</span>
                  <span
                    class="text-foreground font-[ui-monospace,SFMono-Regular,'SF_Mono',Menlo,Consolas,monospace] break-all"
                  >
                    {{ form.index_config[field.name] }}
                  </span>
                </div>
              </template>
            </template>
          </div>
        </section>

        <!-- Create 模式：基本信息 + 连接配置 + 高级索引 三段 -->
        <template v-else>
          <!-- Section 1 — 基本信息：engine 类型 + 名称 -->
          <section class="setting-drawer__section">
            <h4 class="setting-drawer__section-title">
              {{ t("vectorStoreSettings.basicSection", "基本信息") }}
            </h4>

            <div>
              <label
                class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
              >
                {{ t("vectorStoreSettings.engineTypeLabel") }}
              </label>
              <Select v-model="form.engine_type" @update:model-value="onEngineTypeChange">
                <SelectTrigger class="w-full text-[13px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="st in storeTypes" :key="st.type" :value="st.type">
                    {{ st.display_name }}
                  </SelectItem>
                </SelectContent>
              </Select>
              <p v-if="fieldErrors.engine_type" class="text-destructive m-0 mt-1 text-xs">
                {{ fieldErrors.engine_type }}
              </p>
            </div>

            <div>
              <label
                class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
              >
                {{ t("vectorStoreSettings.nameLabel") }}
              </label>
              <SettingsInput
                v-model="form.name"
                :placeholder="t('vectorStoreSettings.namePlaceholder')"
                input-class="text-[13px] md:text-[13px]"
              />
              <p v-if="fieldErrors.name" class="text-destructive m-0 mt-1 text-xs">{{ fieldErrors.name }}</p>
            </div>
          </section>

          <!-- Section 2 — 连接配置（engine type 决定具体字段） -->
          <section v-if="selectedType" class="setting-drawer__section">
            <h4 class="setting-drawer__section-title">{{ t("vectorStoreSettings.connectionInfo") }}</h4>

            <div v-for="field in selectedType.connection_fields" :key="field.name">
              <label
                class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium"
                :class="{
                  'before:text-destructive before:mr-1 before:leading-none before:font-medium before:content-[\'*\']':
                    field.required,
                }"
                >{{ fieldLabel(field.name) }}</label
              >

              <!-- boolean 字段：switch + 行内描述 / TLS 警告 -->
              <template v-if="field.type === 'boolean'">
                <div class="flex items-center gap-2">
                  <Switch
                    :model-value="!!form.connection_config[field.name]"
                    @update:model-value="(v: boolean) => setConnectionField(field.name, v)"
                  />
                </div>
                <p
                  v-if="field.name === 'insecure_skip_verify' && form.connection_config[field.name]"
                  class="text-destructive m-0 mt-1 text-xs leading-normal"
                >
                  {{ t("vectorStoreSettings.insecureSkipVerifyWarning") }}
                </p>
              </template>

              <!-- 敏感字段（password / api key 等）：lock prefix + password -->
              <SettingsInput
                v-else-if="field.type === 'string' && field.sensitive"
                v-model="form.connection_config[field.name]"
                type="password"
                placeholder="********"
                :prefix-icon="LockIcon"
                input-class="text-[13px] md:text-[13px]"
              />

              <!-- 数字字段：用 number input，与 MCP 高级配置同款；无单位提示 -->
              <Input
                v-else-if="field.type === 'number'"
                v-model="connectionNumberTextProxy[field.name].value"
                type="number"
                :placeholder="field.default != null ? String(field.default) : ' '"
                class="number-input text-[13px] md:text-[13px]"
              />

              <!-- 普通字符串 -->
              <Input
                v-else
                v-model="form.connection_config[field.name]"
                :placeholder="field.default?.toString() || ''"
                class="text-[13px] md:text-[13px]"
              />
              <p v-if="fieldErrors[`connection_config.${field.name}`]" class="text-destructive m-0 mt-1 text-xs">
                {{ fieldErrors[`connection_config.${field.name}`] }}
              </p>
            </div>
          </section>

          <!-- Section 3 — 高级索引（仅 selectedType 有 index_fields 时显示） -->
          <section v-if="selectedType?.index_fields?.length" class="setting-drawer__section">
            <h4 class="setting-drawer__section-title">
              {{ t("vectorStoreSettings.advancedIndexConfig") }}
            </h4>

            <!-- 折叠/展开开关：保留之前的可选展示行为，但样式更轻量 -->
            <button
              type="button"
              data-slot="advanced-toggle"
              class="text-muted-foreground hover:text-primary inline-flex cursor-pointer items-center gap-1 self-start bg-transparent px-0 py-1 text-[13px] select-none"
              @click="showAdvanced = !showAdvanced"
            >
              <ChevronDownIcon v-if="showAdvanced" class="size-3.5" />
              <ChevronRightIcon v-else class="size-3.5" />
              <span>{{ showAdvanced ? t("common.collapse", "收起") : t("common.expand", "展开") }}</span>
            </button>

            <template v-if="showAdvanced">
              <div v-for="field in selectedType.index_fields" :key="field.name">
                <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
                  {{ fieldLabel(field.name) }}
                </label>

                <!-- 枚举 → 下拉 -->
                <Select v-if="field.enum && field.enum.length" v-model="form.index_config[field.name]">
                  <SelectTrigger class="w-full text-[13px]">
                    <SelectValue :placeholder="field.default?.toString() || ''" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="opt in field.enum" :key="opt" :value="opt">{{ opt }}</SelectItem>
                  </SelectContent>
                </Select>

                <!-- 数字 → number input -->
                <Input
                  v-else-if="field.type === 'number'"
                  v-model="indexNumberTextProxy[field.name].value"
                  type="number"
                  :placeholder="field.default?.toString()"
                  :min="field.min ?? 1"
                  :max="field.max ?? (isReplicaField(field.name) ? 10 : 64)"
                  class="number-input text-[13px] md:text-[13px]"
                />

                <!-- 字符串 -->
                <Input
                  v-else
                  v-model="form.index_config[field.name]"
                  :placeholder="field.default?.toString() || ''"
                  :maxlength="128"
                  class="text-[13px] md:text-[13px]"
                />
                <p v-if="fieldErrors[`index_config.${field.name}`]" class="text-destructive m-0 mt-1 text-xs">
                  {{ fieldErrors[`index_config.${field.name}`] }}
                </p>
              </div>
            </template>
          </section>
        </template>
      </div>
    </SettingDrawer>

    <!-- 平台共享 / 取消共享确认 -->
    <Dialog v-model:open="sharingDialogVisible">
      <DialogContent class="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>
            {{ sharingDialog.shared ? t("platformSharing.shareAction") : t("platformSharing.unshareAction") }}
          </DialogTitle>
          <DialogDescription>
            {{
              sharingDialog.store
                ? sharingDialog.shared
                  ? t("platformSharing.confirmShare", { name: sharingDialog.store.name })
                  : t("platformSharing.confirmUnshare", { name: sharingDialog.store.name })
                : ""
            }}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose as-child>
            <Button variant="outline">{{ t("common.cancel") }}</Button>
          </DialogClose>
          <Button
            :variant="sharingDialog.shared ? 'default' : 'destructive'"
            :class="
              sharingDialog.shared
                ? ''
                : 'bg-destructive text-primary-foreground hover:bg-destructive/90 dark:bg-destructive'
            "
            :disabled="sharingDialog.pending"
            @click="doToggleSharing"
          >
            <Loader2Icon v-if="sharingDialog.pending" class="animate-spin" />
            {{ t("common.confirm") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- 删除确认 -->
    <Dialog v-model:open="deleteDialogVisible">
      <DialogContent class="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>{{ t("vectorStoreSettings.deleteConfirm") }}</DialogTitle>
        </DialogHeader>
        <DialogFooter>
          <DialogClose as-child>
            <Button variant="outline">{{ t("common.cancel") }}</Button>
          </DialogClose>
          <!-- The old DialogPlugin.confirm used its default (primary) confirm button here. -->
          <Button :disabled="deleteDialogPending" @click="doDelete">
            <Loader2Icon v-if="deleteDialogPending" class="animate-spin" />
            {{ t("common.delete") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, reactive, type WritableComputedRef } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import {
  listVectorStores,
  listVectorStoreTypes,
  createVectorStore,
  updateVectorStore,
  deleteVectorStore as deleteVectorStoreAPI,
  setVectorStoreSharing,
  testVectorStoreRaw,
  type VectorStoreEntity,
  type VectorStoreTypeInfo,
} from "@/api/vector-store";
import { useAuthStore } from "@/stores/auth";
import { providerLogo } from "./providerLogos";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import SettingsInput from "./SettingsInput.vue";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Empty, EmptyDescription } from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import {
  ChevronDownIcon,
  ChevronRightIcon,
  CircleCheckIcon,
  CircleXIcon,
  EllipsisIcon,
  InfoIcon,
  Loader2Icon,
  LockIcon,
  PlusIcon,
} from "@lucide/vue";

const { t } = useI18n();
const authStore = useAuthStore();

// ===== State =====
const stores = ref<VectorStoreEntity[]>([]);
const storeTypes = ref<VectorStoreTypeInfo[]>([]);
const loading = ref(false);
const showDialog = ref(false);
const editingStore = ref<VectorStoreEntity | null>(null);
const testing = ref(false);
const saving = ref(false);
const showAdvanced = ref(false);

const form = ref<{
  name: string;
  engine_type: string;
  connection_config: Record<string, any>;
  index_config: Record<string, any>;
}>({
  name: "",
  engine_type: "",
  connection_config: {},
  index_config: {},
});

// Tri-state hint icon next to the test button: null=neutral, true=just
// succeeded, false=just failed. Cleared when the user changes any
// connection-relevant field so a stale ✓/✗ doesn't follow a config the
// user is still editing.
const lastTestOk = ref<boolean | null>(null);

watch(
  () => [form.value.engine_type, form.value.connection_config],
  () => {
    lastTestOk.value = null;
  },
  { deep: true },
);

// Per-field validation messages, replacing the old t-form rules display.
const fieldErrors = reactive<Record<string, string>>({});

const clearFieldErrors = () => {
  for (const k of Object.keys(fieldErrors)) delete fieldErrors[k];
};

// The same checks the t-form rules enforced: name/engine required, required
// connection fields, index string fields match the name pattern (empty OK).
function validateForm(): boolean {
  clearFieldErrors();
  let ok = true;
  const fail = (key: string, message: string) => {
    fieldErrors[key] = message;
    ok = false;
  };

  if (!form.value.name.trim()) {
    fail("name", t("vectorStoreSettings.validation.nameRequired"));
  }

  if (!editingStore.value) {
    if (!form.value.engine_type) {
      fail("engine_type", t("vectorStoreSettings.validation.engineTypeRequired"));
    }
    const st = selectedType.value;
    if (st) {
      for (const field of st.connection_fields) {
        if (!field.required) continue;
        const v = form.value.connection_config[field.name];
        if (v == null || v === "" || (typeof v === "string" && v.trim() === "")) {
          fail(
            `connection_config.${field.name}`,
            t("vectorStoreSettings.validation.fieldRequired", { field: fieldLabel(field.name) }),
          );
        }
      }
      for (const field of st.index_fields || []) {
        if (field.type !== "string") continue;
        const val = form.value.index_config[field.name];
        if (val && !indexNamePattern.test(String(val))) {
          fail(`index_config.${field.name}`, t("vectorStoreSettings.validation.indexNamePattern"));
        }
      }
    }
  }
  return ok;
}

// ===== Computed =====
const envStores = computed(() => stores.value.filter((s) => s.source === "env"));
const userStores = computed(() => stores.value.filter((s) => s.source === "user"));
const selectedType = computed(() => storeTypes.value.find((st) => st.type === form.value.engine_type));

// Drawer header logo — 与列表 .store-card__badge 同源（providerLogo()），让
// 列表卡 → 抽屉 hand-off 视觉连贯。
const drawerLogo = computed(() => {
  if (!form.value.engine_type) return null;
  return providerLogo("vectorstore", form.value.engine_type);
});

const drawerLogoStyle = computed((): Record<string, string> => {
  const logo = drawerLogo.value;
  if (!logo || logo.mode !== "mono") return {};
  return { "--logo-url": `url("${logo.url}")` };
});

// per-engine class on drawer for non-scoped header-icon coloring rules.
const drawerClass = computed(() => {
  return form.value.engine_type
    ? `vectorstore-drawer vectorstore-drawer--${form.value.engine_type}`
    : "vectorstore-drawer";
});

// 测试连接是否可点。create 模式：必须填全所有 required 连接字段；
// edit 模式：engine 不可改、连接配置只读，禁用测试（要重新建条目，不在抽屉里测）。
const canTestConnection = computed(() => {
  if (editingStore.value) return false;
  const st = selectedType.value;
  if (!st) return false;
  for (const f of st.connection_fields) {
    if (!f.required) continue;
    const v = form.value.connection_config[f.name];
    if (v == null || v === "" || (typeof v === "string" && v.trim() === "")) return false;
  }
  return true;
});

// Per-store dropdown options. env 来源由 .env 写入，UI 不允许 edit / delete；
// 测试连接已挪到编辑抽屉的 footer，外层菜单不再露出"测试"项。env 来源没有
// 编辑/删除入口 → 整个 dropdown 都不需要展示。
const storeActionsFor = (store: VectorStoreEntity) => {
  if (store.source === "env") return [];
  // 平台共享的向量库对普通空间管理员只读：列出来是为了让他们建库时能选，
  // 但连接配置是平台的，不该由某个空间改。
  if (store.is_builtin && !authStore.isSystemAdmin) return [];
  const actions: Array<{ content: string; value: string; theme?: "error" }> = [
    { content: t("common.edit"), value: "edit" },
  ];
  if (authStore.isSystemAdmin) {
    actions.push({
      content: store.is_builtin ? t("platformSharing.unshareAction") : t("platformSharing.shareAction"),
      value: "sharing",
    });
  }
  // 共享中的向量库必须先取消共享再删 —— 取消共享那一步才会做跨空间引用检查。
  if (!store.is_builtin) {
    actions.push({ content: t("common.delete"), value: "delete", theme: "error" });
  }
  return actions;
};

// Index/collection name pattern: must start with letter, alphanumeric + _ + - only, max 128
const indexNamePattern = /^[a-zA-Z][a-zA-Z0-9_-]{0,127}$/;

// ===== Methods =====
const fieldLabel = (name: string): string => {
  const key = `vectorStoreSettings.fields.${name}`;
  const translated = t(key);
  // If i18n key not found, vue-i18n returns the key itself — fall back to field name
  return translated === key ? name : translated;
};

// Distinguish replica fields (max 10) from shard fields (max 64) for input bounds
const replicaFieldNames = ["number_of_replicas", "replication_factor", "replica_number"];
const isReplicaField = (name: string): boolean => replicaFieldNames.includes(name);

const getStoreEndpoint = (store: VectorStoreEntity): string => {
  const cc = store.connection_config || {};
  return cc.addr || cc.host || "";
};

// 卡片徽章首字母。engine_type 都是英文 ASCII，直接 charAt。
const engineInitial = (engineType: string): string => {
  return (engineType || "?").charAt(0).toUpperCase();
};

// 当 engine 有 logo 资源时，把 SVG URL 透传给 CSS（::before 用 mask-image
// 渲染），并把卡片底色切回中性白；没有 logo 时返回空对象，沿用每个 engine
// 的品牌色 monogram 样式。color 模式不需要 mask 染色，所以 url 不上报。
const resolveLogo = (engineType: string) => providerLogo("vectorstore", engineType);

const badgeClass = (engineType: string) => {
  const m = resolveLogo(engineType)?.mode;
  return {
    "store-card__badge--logo": !!m,
    "store-card__badge--color": m === "color",
    "store-card__badge--mono": m === "mono",
  };
};

const badgeStyle = (engineType: string): Record<string, string> => {
  const logo = resolveLogo(engineType);
  return logo?.mode === "mono" ? { "--logo-url": `url("${logo.url}")` } : {};
};

// boolean 连接字段的回写：the Switch reports its new value through update:modelValue.
const setConnectionField = (name: string, value: boolean) => {
  form.value.connection_config[name] = value;
};

const onEngineTypeChange = () => {
  form.value.connection_config = {};
  form.value.index_config = {};
  showAdvanced.value = false;
  clearFieldErrors();
  // Drop cached number-text proxies so a switch to a different engine
  // doesn't keep stale entries pointing at the old field set.
  for (const k of Object.keys(connectionNumberText)) delete connectionNumberText[k];
  for (const k of Object.keys(indexNumberText)) delete indexNumberText[k];
};

// ---- Number-input text proxies (lazy per field name) ----
// type=number 输入会因为 v-model 把空字符串 coerce 成 0 / NaN，导致
// "用户清空 → 自动塞回 0" 的烦躁交互。我们用 WritableComputedRef 包一层：
// 读取时把数字转成字符串展示；写入时空串 → 删除字段（让 placeholder 显示
// 出来），非空 → 转 int。Proxy 按字段名按需创建并缓存，避免重复 computed。
const connectionNumberText: Record<string, WritableComputedRef<string>> = {};
const indexNumberText: Record<string, WritableComputedRef<string>> = {};

function ensureNumberProxy(
  bag: Record<string, WritableComputedRef<string>>,
  store: Record<string, any>,
  key: string,
): WritableComputedRef<string> {
  if (bag[key]) return bag[key];
  bag[key] = computed<string>({
    get: () => {
      const v = store[key];
      return v == null || v === "" ? "" : String(v);
    },
    set: (raw: string) => {
      const s = String(raw ?? "").trim();
      if (!s) {
        delete store[key];
        return;
      }
      const n = Number(s);
      store[key] = Number.isFinite(n) ? n : s;
    },
  });
  return bag[key];
}

// Vue templates can't call ensureNumberProxy on every render without the
// keys multiplying — wrap in a Proxy so `connectionNumberText[name].value`
// from the template lazily creates the proxy on first read.
const connectionNumberTextProxy = new Proxy(connectionNumberText, {
  get: (target, name: string) => ensureNumberProxy(target, form.value.connection_config, name),
});
const indexNumberTextProxy = new Proxy(indexNumberText, {
  get: (target, name: string) => ensureNumberProxy(target, form.value.index_config, name),
});

const loadStores = async () => {
  try {
    const response = await listVectorStores();
    if (response.data && Array.isArray(response.data)) {
      stores.value = response.data;
    }
  } catch (error) {
    console.error("Failed to load vector stores:", error);
  }
};

const loadStoreTypes = async () => {
  try {
    storeTypes.value = await listVectorStoreTypes();
  } catch (error) {
    console.error("Failed to load vector store types:", error);
  }
};

const openAddDialog = () => {
  editingStore.value = null;
  showAdvanced.value = false;
  clearFieldErrors();
  form.value = {
    name: "",
    engine_type: storeTypes.value[0]?.type || "",
    connection_config: {},
    index_config: {},
  };
  lastTestOk.value = null;
  showDialog.value = true;
};

// env 来源由 .env 注入，与列表菜单一致：不可点击编辑
const isStoreCardClickable = (store: VectorStoreEntity) => authStore.hasRole("admin") && store.source !== "env";

const onStoreCardClick = (event: Event, store: VectorStoreEntity) => {
  if (!isStoreCardClickable(store)) return;
  if (event.type === "keydown") {
    const ke = event as KeyboardEvent;
    if (ke.key !== "Enter" && ke.key !== " ") return;
    ke.preventDefault();
  }
  const target = event.target as HTMLElement | null;
  if (target?.closest(".store-card__actions")) return;
  editStore(store);
};

const editStore = (store: VectorStoreEntity) => {
  if (store.source === "env") {
    return;
  }
  editingStore.value = store;
  showAdvanced.value = false;
  clearFieldErrors();
  form.value = {
    name: store.name,
    engine_type: store.engine_type,
    connection_config: { ...store.connection_config },
    index_config: { ...store.index_config },
  };
  lastTestOk.value = null;
  showDialog.value = true;
};

// SettingDrawer 的"保存"按钮触发：手动校验后写后端。
// edit 模式只能改 name；create 模式提交完整 connection / index 配置。
const onDrawerConfirm = async () => {
  if (!validateForm()) {
    MessagePlugin.warning(t("vectorStoreSettings.toasts.errorGeneric") as string);
    return;
  }

  saving.value = true;
  try {
    if (editingStore.value) {
      await updateVectorStore(editingStore.value.id!, { name: form.value.name.trim() });
      MessagePlugin.success(t("vectorStoreSettings.toasts.storeUpdated"));
    } else {
      const data: Partial<VectorStoreEntity> = {
        name: form.value.name.trim(),
        engine_type: form.value.engine_type,
        connection_config: { ...form.value.connection_config },
        index_config: showAdvanced.value ? { ...form.value.index_config } : {},
      };
      await createVectorStore(data);
      MessagePlugin.success(t("vectorStoreSettings.toasts.storeCreated"));
    }
    showDialog.value = false;
    await loadStores();
  } catch (error: any) {
    const msg = error?.message || t("vectorStoreSettings.toasts.errorGeneric");
    if (msg.toLowerCase().includes("already exists") || msg.toLowerCase().includes("duplicate")) {
      MessagePlugin.error(t("vectorStoreSettings.toasts.duplicateName"));
    } else {
      MessagePlugin.error(msg);
    }
  } finally {
    saving.value = false;
  }
};

const handleAction = (action: { value: string }, store: VectorStoreEntity) => {
  // test 已挪到抽屉，外层菜单不再处理 'test' 值。
  if (action.value === "edit") {
    editStore(store);
  } else if (action.value === "delete") {
    confirmDelete(store);
  } else if (action.value === "sharing") {
    confirmSharing(store);
  }
};

// 切换平台共享。取消共享时后端会拒绝仍被其他空间知识库绑定的向量库，
// 错误文案里带着引用数量，直接透传比一句泛化的失败提示有用。
const sharingDialogVisible = ref(false);
const sharingDialog = reactive<{ store: VectorStoreEntity | null; shared: boolean; pending: boolean }>({
  store: null,
  shared: false,
  pending: false,
});

const confirmSharing = (store: VectorStoreEntity) => {
  sharingDialog.store = store;
  sharingDialog.shared = !store.is_builtin;
  sharingDialog.pending = false;
  sharingDialogVisible.value = true;
};

async function doToggleSharing() {
  const store = sharingDialog.store;
  if (!store || sharingDialog.pending) return;
  const shared = sharingDialog.shared;
  sharingDialog.pending = true;
  try {
    await setVectorStoreSharing(store.id!, shared);
    MessagePlugin.success(shared ? t("platformSharing.sharedToast") : t("platformSharing.unsharedToast"));
    sharingDialogVisible.value = false;
    await loadStores();
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("platformSharing.failedToast"));
  } finally {
    sharingDialog.pending = false;
  }
}

const deleteDialogVisible = ref(false);
const deleteDialogPending = ref(false);
const deleteTarget = ref<VectorStoreEntity | null>(null);

const confirmDelete = (store: VectorStoreEntity) => {
  deleteTarget.value = store;
  deleteDialogPending.value = false;
  deleteDialogVisible.value = true;
};

async function doDelete() {
  const store = deleteTarget.value;
  if (!store || deleteDialogPending.value) return;
  deleteDialogPending.value = true;
  try {
    await deleteVectorStoreAPI(store.id!);
    MessagePlugin.success(t("vectorStoreSettings.toasts.storeDeleted"));
    deleteDialogVisible.value = false;
    await loadStores();
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("vectorStoreSettings.toasts.errorGeneric"));
  } finally {
    deleteDialogPending.value = false;
  }
}

// 测试连接（在抽屉内触发）。create 模式下用当前表单数据，调
// /test/raw 端点。edit 模式按钮 disabled，所以这里只处理 create 路径。
const onDrawerTest = async () => {
  if (editingStore.value) return;
  testing.value = true;
  try {
    const data = {
      engine_type: form.value.engine_type,
      connection_config: { ...form.value.connection_config },
    };
    const res = await testVectorStoreRaw(data);
    lastTestOk.value = !!res.success;
    if (res.success) {
      MessagePlugin.success(t("vectorStoreSettings.toasts.testSuccess"));
    } else {
      MessagePlugin.error(res.error || t("vectorStoreSettings.toasts.testFailed"));
    }
  } catch (error: any) {
    lastTestOk.value = false;
    MessagePlugin.error(error?.message || t("vectorStoreSettings.toasts.testFailed"));
  } finally {
    testing.value = false;
  }
};

// ===== Init =====
onMounted(async () => {
  loading.value = true;
  try {
    await Promise.all([loadStoreTypes(), loadStores()]);
  } finally {
    loading.value = false;
  }
});
</script>

<style scoped>
/*
 * Utilities cover layout; these rules reach into generated markup or repeat
 * per-engine tints that are clearer as CSS:
 *  - mono logos are CSS-masked icons driven by a per-card --logo-url;
 *  - the per-engine badge tints (11 vector backends);
 *  - the "more" button fading in on hover / keyboard focus;
 *  - hiding the native number-input spinners.
 */
.header-icon__mono {
  background-color: currentColor;
  -webkit-mask-image: var(--logo-url);
  -webkit-mask-position: center;
  -webkit-mask-repeat: no-repeat;
  -webkit-mask-size: contain;
  mask-image: var(--logo-url);
  mask-position: center;
  mask-repeat: no-repeat;
  mask-size: contain;
}

.store-card .store-card__badge--logo {
  background: var(--td-bg-color-container, #fff);
  box-shadow: inset 0 0 0 1px var(--td-component-stroke);
}

.store-card .store-card__badge--mono::before {
  content: "";
  width: 22px;
  height: 22px;
  background-color: currentColor;
  -webkit-mask-image: var(--logo-url);
  -webkit-mask-position: center;
  -webkit-mask-repeat: no-repeat;
  -webkit-mask-size: contain;
  mask-image: var(--logo-url);
  mask-position: center;
  mask-repeat: no-repeat;
  mask-size: contain;
}

/* 各 vector engine 配色（覆盖 11 类常见后端，未列出的回落到默认蓝） */
.store-card--qdrant .store-card__badge {
  background: rgba(225, 38, 38, 0.12);
  color: #e12626;
}
.store-card--milvus .store-card__badge {
  background: rgba(0, 137, 255, 0.12);
  color: #0089ff;
}
.store-card--weaviate .store-card__badge {
  background: rgba(7, 192, 95, 0.12);
  color: #07a050;
}
.store-card--elasticsearch .store-card__badge,
.store-card--elasticfaiss .store-card__badge {
  background: rgba(255, 153, 0, 0.12);
  color: #d97706;
}
.store-card--postgres .store-card__badge {
  background: rgba(0, 82, 217, 0.1);
  color: #0052d9;
}
.store-card--opensearch .store-card__badge {
  background: rgba(98, 53, 187, 0.12);
  color: #6235bb;
}
.store-card--infinity .store-card__badge {
  background: rgba(98, 53, 187, 0.12);
  color: #6235bb;
}
.store-card--tencent_vectordb .store-card__badge {
  background: rgba(0, 82, 217, 0.1);
  color: #0052d9;
}
.store-card--doris .store-card__badge {
  background: rgba(255, 90, 0, 0.12);
  color: #e55a00;
}

.store-card:hover .store-card__more,
.store-card:focus-within .store-card__more,
.store-card__actions:focus-within .store-card__more {
  opacity: 1;
}

/* `number-input` sits on the <input> itself (the Input's root element). */
.number-input::-webkit-outer-spin-button,
.number-input::-webkit-inner-spin-button {
  -webkit-appearance: none;
  appearance: none;
  margin: 0;
}

.number-input[type="number"] {
  -moz-appearance: textfield;
  appearance: textfield;
}
</style>

<!--
  Non-scoped block: per-engine header-icon coloring + color-logo background
  tweak. Same pattern as Storage/Parser/WebSearch drawers — these rules
  must be global so they reach the teleported drawer panel even if its
  scoped data-attribute is dropped in some builds. Each rule mirrors the
  matching .store-card--{engine} .store-card__badge from the scoped block
  above so list-card → drawer hand-off stays visually continuous.
-->
<style>
/* 彩色 logo 时给 header-icon 容器一个白底 + 1px 边 */
.vectorstore-drawer .setting-drawer__header-icon:has(.header-icon__img) {
  background: var(--td-bg-color-container, #fff);
  box-shadow: inset 0 0 0 1px var(--td-component-stroke);
}

.vectorstore-drawer--qdrant .setting-drawer__header-icon {
  background: rgba(225, 38, 38, 0.12);
  color: #e12626;
}
.vectorstore-drawer--milvus .setting-drawer__header-icon {
  background: rgba(0, 137, 255, 0.12);
  color: #0089ff;
}
.vectorstore-drawer--weaviate .setting-drawer__header-icon {
  background: rgba(7, 192, 95, 0.12);
  color: #07a050;
}
.vectorstore-drawer--elasticsearch .setting-drawer__header-icon,
.vectorstore-drawer--elasticfaiss .setting-drawer__header-icon {
  background: rgba(255, 153, 0, 0.12);
  color: #d97706;
}
.vectorstore-drawer--postgres .setting-drawer__header-icon {
  background: rgba(0, 82, 217, 0.1);
  color: #0052d9;
}
.vectorstore-drawer--opensearch .setting-drawer__header-icon {
  background: rgba(98, 53, 187, 0.12);
  color: #6235bb;
}
.vectorstore-drawer--infinity .setting-drawer__header-icon {
  background: rgba(98, 53, 187, 0.12);
  color: #6235bb;
}
.vectorstore-drawer--tencent_vectordb .setting-drawer__header-icon {
  background: rgba(0, 82, 217, 0.1);
  color: #0052d9;
}
.vectorstore-drawer--doris .setting-drawer__header-icon {
  background: rgba(255, 90, 0, 0.12);
  color: #e55a00;
}
</style>
