<template>
  <div class="w-full">
    <div class="mb-7">
      <h2 class="text-foreground mt-0 mb-2 text-xl font-semibold">{{ $t("settings.parser.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-[1.6]">
        {{ $t("settings.parser.description") }}
      </p>
      <!-- 作用域切换只对系统管理员出现。空间管理员没有"平台默认"这一档，
           他们看到的就是本空间的覆盖（集中管控开启时连整个入口都不会渲染）。 -->
      <div v-if="canEditPlatformScope" class="mt-4">
        <!-- Segmented control stands in for the old filled radio-button group
             (TDesign "default-filled"): a grey track, the chosen option raised
             on a white chip. -->
        <div class="inline-flex gap-0.5 rounded-md bg-[var(--td-bg-color-component)] p-0.5">
          <Button
            v-for="opt in scopeOptions"
            :key="opt.value"
            type="button"
            variant="ghost"
            size="sm"
            :aria-pressed="scope === opt.value"
            :class="
              scope === opt.value
                ? 'bg-card text-foreground hover:bg-card shadow-[0_1px_2px_rgba(15,23,42,0.06)]'
                : 'text-muted-foreground hover:text-foreground hover:bg-transparent'
            "
            @click="selectScope(opt.value)"
          >
            {{ opt.label }}
          </Button>
        </div>
        <p class="text-placeholder m-0 mt-2 text-xs leading-[1.6]">
          {{
            scope === "platform" ? $t("settings.parser.scope.platformHint") : $t("settings.parser.scope.workspaceHint")
          }}
        </p>
      </div>
    </div>

    <div v-if="loading" class="text-placeholder flex items-center justify-center gap-2 py-12 text-sm">
      <Loader2Icon class="animate-spin" />
      <span>{{ $t("settings.parser.loading") }}</span>
    </div>

    <div v-else-if="error" class="py-4">
      <!-- TDesign's error alert sat on the pale error tint with no border and
           dark text; only its icon was red. -->
      <Alert variant="destructive" class="text-foreground border-transparent bg-[var(--td-error-color-1)]">
        <CircleAlertIcon class="text-destructive!" />
        <AlertTitle>{{ error }}</AlertTitle>
        <AlertAction>
          <Button variant="outline" size="sm" @click="loadAll">{{ $t("settings.parser.retry") }}</Button>
        </AlertAction>
      </Alert>
    </div>

    <template v-else>
      <div v-if="engines.length === 0 && !hasBuiltinEngine" class="py-12 text-center">
        <p class="text-placeholder m-0 text-sm">{{ $t("settings.parser.noEngineDetected") }}</p>
      </div>

      <!-- 与其它 settings 列表同形：左侧 monogram 徽章 + 标题 + 状态徽 + 两行描述。
           整张卡片可点击，打开抽屉配置；当前抽屉对应的卡片获得品牌色描边。 -->
      <div v-else class="mt-6 grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-3">
        <!-- 当后端未返回 builtin 引擎项时，仍展示 DocReader 状态卡片 -->
        <button
          v-if="!hasBuiltinEngine"
          type="button"
          data-slot="engine-card"
          class="engine-card flex min-w-0 cursor-pointer items-start gap-3 rounded-[10px] border py-3.5 pr-3.5 pl-3 text-left transition-[border-color,box-shadow,background-color] duration-[180ms] ease-out hover:shadow-[0_4px_14px_rgba(15,23,42,0.06)]"
          :class="[
            'engine-card--builtin',
            drawerVisible && currentEngine?.Name === 'builtin'
              ? 'border-primary bg-[var(--td-brand-color-1,rgba(7,192,95,0.06))]'
              : 'bg-card border-border hover:border-[var(--td-brand-color-3,var(--td-brand-color))]',
          ]"
          @click="openDrawer({ Name: 'builtin' } as any)"
        >
          <div
            class="engine-card__badge mt-px flex size-9 shrink-0 items-center justify-center rounded-[9px] text-[15px] font-semibold tracking-[0.02em]"
          >
            {{ engineInitial("builtin") }}
          </div>
          <div class="flex min-w-0 flex-1 flex-col gap-1">
            <div class="flex min-w-0 items-center gap-1.5">
              <h3 class="text-foreground m-0 min-w-0 flex-1 truncate text-sm leading-[1.4] font-semibold">
                {{ getEngineDisplayName("builtin") }}
              </h3>
              <span
                class="bg-secondary inline-flex shrink-0 items-center gap-[5px] rounded-[10px] py-px pr-2 pl-1.5 text-[11px] leading-4 font-medium"
                :class="
                  connected ? 'text-[var(--td-success-color-7,#118053)]' : 'text-[var(--td-error-color-7,#c93e3e)]'
                "
              >
                <span
                  class="size-1.5 rounded-full"
                  :class="connected ? 'bg-[var(--td-success-color,#118053)]' : 'bg-[var(--td-error-color,#c93e3e)]'"
                />
                {{ connected ? $t("settings.parser.connected") : $t("settings.parser.disconnected") }}
              </span>
            </div>
            <p class="text-muted-foreground m-0 line-clamp-2 text-xs leading-normal">
              {{ $t("settings.parser.builtinDesc") }}
            </p>
          </div>
        </button>

        <button
          v-for="engine in sortedEngines"
          :key="engine.Name"
          type="button"
          data-slot="engine-card"
          class="engine-card flex min-w-0 cursor-pointer items-start gap-3 rounded-[10px] border py-3.5 pr-3.5 pl-3 text-left transition-[border-color,box-shadow,background-color] duration-[180ms] ease-out hover:shadow-[0_4px_14px_rgba(15,23,42,0.06)]"
          :class="[
            `engine-card--${engine.Name}`,
            // The card whose drawer is open keeps a brand outline and tint.
            drawerVisible && currentEngine?.Name === engine.Name
              ? 'border-primary bg-[var(--td-brand-color-1,rgba(7,192,95,0.06))]'
              : 'bg-card border-border hover:border-[var(--td-brand-color-3,var(--td-brand-color))]',
          ]"
          @click="openDrawer(engine)"
        >
          <div
            class="engine-card__badge mt-px flex size-9 shrink-0 items-center justify-center rounded-[9px] text-[15px] font-semibold tracking-[0.02em]"
          >
            {{ engineInitial(engine.Name) }}
          </div>
          <div class="flex min-w-0 flex-1 flex-col gap-1">
            <div class="flex min-w-0 items-center gap-1.5">
              <h3 class="text-foreground m-0 min-w-0 flex-1 truncate text-sm leading-[1.4] font-semibold">
                {{ getEngineDisplayName(engine.Name) }}
              </h3>
              <span
                v-if="engine.Available"
                class="bg-secondary inline-flex shrink-0 items-center gap-[5px] rounded-[10px] py-px pr-2 pl-1.5 text-[11px] leading-4 font-medium text-[var(--td-success-color-7,#118053)]"
              >
                <span class="size-1.5 rounded-full bg-[var(--td-success-color,#118053)]" />
                {{ $t("settings.parser.available") }}
              </span>
              <Tooltip v-else-if="engine.UnavailableReason">
                <TooltipTrigger as-child>
                  <span
                    class="bg-secondary inline-flex shrink-0 cursor-help items-center gap-[5px] rounded-[10px] py-px pr-2 pl-1.5 text-[11px] leading-4 font-medium text-[var(--td-error-color-7,#c93e3e)]"
                  >
                    <span class="size-1.5 rounded-full bg-[var(--td-error-color,#c93e3e)]" />
                    {{ $t("settings.parser.unavailable") }}
                  </span>
                </TooltipTrigger>
                <TooltipContent>{{ engine.UnavailableReason }}</TooltipContent>
              </Tooltip>
              <span
                v-else
                class="bg-secondary inline-flex shrink-0 items-center gap-[5px] rounded-[10px] py-px pr-2 pl-1.5 text-[11px] leading-4 font-medium text-[var(--td-error-color-7,#c93e3e)]"
              >
                <span class="size-1.5 rounded-full bg-[var(--td-error-color,#c93e3e)]" />
                {{ $t("settings.parser.unavailable") }}
              </span>
            </div>
            <p class="text-muted-foreground m-0 line-clamp-2 text-xs leading-normal">
              {{ getEngineDisplayDesc(engine.Name, engine.Description) }}
            </p>
          </div>
        </button>
      </div>
    </template>

    <!-- 配置抽屉 — 用 SettingDrawer 包装，保持与 ModelEditorDialog 同款视觉/交互 -->
    <SettingDrawer
      v-model:visible="drawerVisible"
      :title="drawerTitle"
      :class="
        currentEngine ? `parser-engine-drawer parser-engine-drawer--${currentEngine.Name}` : 'parser-engine-drawer'
      "
      :hide-footer="!authStore.hasRole('admin') && !needsTestButton"
      :confirm-loading="saving"
      @confirm="onSave"
      @cancel="drawerVisible = false"
    >
      <!--
        Header icon — 与列表卡片同款 monogram 徽章：首字母 + per-engine 配色，
        通过 .parser-engine-drawer--{name} .setting-drawer__header-icon 在
        非 scoped 块里覆盖背景与文字色。Parser 引擎没有真实 logo，所以这
        里只渲染字母；存储引擎那边走的是 logo 图片/mask，pattern 一致。
      -->
      <template v-if="currentEngine" #headerIcon>
        <span class="text-[15px] font-semibold tracking-[0.02em]">{{ engineInitial(currentEngine.Name) }}</span>
      </template>
      <!--
        Subtitle slot: 引擎描述 + 内联文档链接。我们把"参考资料"从一个
        独立 section 收回到头部副标题里 — 一个外链不值得占一整个 section。
      -->
      <template v-if="currentEngine" #subtitle>
        <span>{{ getEngineDisplayDesc(currentEngine.Name, currentEngine.Description) }}</span>
        <a
          v-if="engineDocLink(currentEngine.Name)"
          :href="engineDocLink(currentEngine.Name)"
          target="_blank"
          rel="noopener noreferrer"
          class="text-primary ml-1.5 inline-flex items-center gap-1 align-baseline text-xs font-medium no-underline transition-colors duration-150 hover:text-[var(--td-brand-color-active)]"
        >
          {{ engineDocLabel(currentEngine.Name) }}
          <LinkIcon class="size-3" />
        </a>
      </template>
      <!--
        Footer-left slot: 测试连接按钮 + 状态文案 — 主操作栏沿底边对齐，
        与 ModelEditorDialog 远程模型抽屉保持一致。仅在引擎有可校验的
        配置/状态时才挂载。
      -->
      <template v-if="needsTestButton" #footer-left>
        <Button variant="outline" :disabled="checking" @click="onCheck">
          <CircleCheckIcon v-if="!checking && saveSuccess && checkMessage" class="text-primary size-4 shrink-0" />
          <CircleXIcon v-else-if="!checking && checkMessage && !saveSuccess" class="text-destructive size-4 shrink-0" />
          <Loader2Icon v-if="checking" class="animate-spin" />
          {{
            checking
              ? $t("settings.parser.checking", $t("settings.parser.testConnection"))
              : $t("settings.parser.testConnection")
          }}
        </Button>
        <span
          v-if="checkMessage"
          class="min-w-0 flex-1 truncate text-xs leading-[1.4]"
          :class="saveSuccess ? 'text-[var(--td-brand-color-active)]' : 'text-destructive'"
          :title="checkMessage"
        >
          {{ checkMessage }}
        </span>
      </template>

      <!-- `setting-drawer__section` / `__section-title` are styled by SettingDrawer
           (spacing, dividers, the brand bar before each title); the fields sit
           directly in the section, which spaces them. -->
      <div v-if="currentEngine">
        <!--
          Section 1 — 支持文件类型。放在内容开头作为引擎"能干什么"的
          一目了然概览，对所有引擎都有意义；与状态/配置区分开。
        -->
        <section v-if="currentEngine.FileTypes && currentEngine.FileTypes.length" class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">
            {{ $t("settings.parser.supportedFileTypes", "支持文件类型") }}
          </h4>
          <div class="flex flex-wrap gap-1.5">
            <span
              v-for="ft in currentEngine.FileTypes"
              :key="ft"
              class="text-muted-foreground inline-flex h-[22px] items-center rounded bg-[var(--td-bg-color-component)] px-2 font-[ui-monospace,SFMono-Regular,'SF_Mono',Menlo,Consolas,monospace] text-[11px] font-medium tracking-[0.02em]"
            >
              {{ ft }}
            </span>
          </div>
        </section>

        <!--
          Section 2 — 状态信息（DocReader 连接）
          只有有内容时才渲染，避免空 section 空底部分隔线。
        -->
        <section v-if="currentEngine.Name === 'builtin'" class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">
            {{ $t("settings.parser.statusSection", "状态信息") }}
          </h4>

          <!-- builtin: DocReader 连接信息 -->
          <div class="bg-accent border-border flex flex-col gap-2 rounded-lg border px-3.5 py-3">
            <div class="flex flex-wrap items-center gap-2">
              <Badge v-if="connected" variant="secondary" class="bg-success/10 text-success">
                {{ $t("settings.parser.connected") }}
              </Badge>
              <Badge v-else variant="destructive">
                {{ $t("settings.parser.disconnected") }}
              </Badge>
              <Badge variant="secondary">{{ docreaderTransport === "http" ? "HTTP" : "gRPC" }}</Badge>
              <span
                v-if="docreaderAddrEnv"
                class="text-placeholder font-[ui-monospace,SFMono-Regular,'SF_Mono',Menlo,Consolas,monospace] text-xs"
              >
                {{ $t("settings.parser.currentAddr") }}: {{ docreaderAddrEnv }}
              </span>
            </div>
            <p class="text-placeholder m-0 text-xs leading-normal">{{ $t("settings.parser.envVarHint") }}</p>
          </div>
        </section>

        <!-- Section 3 — mineru 自建配置 -->
        <section v-if="currentEngine.Name === 'mineru'" class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t("settings.parser.configSection", "配置") }}</h4>

          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ t("settings.parser.selfHostedEndpoint") }}
            </label>
            <SettingsInput
              v-model="config.mineru_endpoint"
              :placeholder="$t('settings.parser.mineruEndpointPlaceholder')"
              clearable
              input-class="text-[13px] md:text-[13px]"
            />
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">Backend</label>
            <!-- clearable: like the old t-select, clearing unsets the value (undefined), leaving the choice to the default. -->
            <div class="group/select-clear relative">
              <Select v-model="config.mineru_model">
                <SelectTrigger class="w-full text-[13px]">
                  <SelectValue :placeholder="$t('settings.parser.defaultPipeline')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="pipeline">pipeline</SelectItem>
                  <SelectItem value="vlm-auto-engine">vlm-auto-engine</SelectItem>
                  <SelectItem value="vlm-http-client">vlm-http-client</SelectItem>
                  <SelectItem value="hybrid-auto-engine">hybrid-auto-engine</SelectItem>
                  <SelectItem value="hybrid-http-client">hybrid-http-client</SelectItem>
                </SelectContent>
              </Select>
              <SettingsSelectClear :visible="!!config.mineru_model" @clear="config.mineru_model = undefined" />
            </div>
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              vLLM {{ $t("settings.parser.serverUrl") }}
            </label>
            <SettingsInput
              v-model="config.mineru_vlm_server_url"
              :placeholder="$t('settings.parser.vlmServerUrlPlaceholder')"
              clearable
              input-class="text-[13px] md:text-[13px]"
            />
            <p class="text-placeholder m-0 mt-1 text-xs leading-normal">{{ $t("settings.parser.vlmServerUrlHint") }}</p>
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ $t("settings.parser.parseMethodLabel") }}
            </label>
            <Select v-model="config.mineru_parse_method">
              <SelectTrigger class="w-full text-[13px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="auto">{{ $t("settings.parser.parseMethodAuto") }}</SelectItem>
                <SelectItem value="ocr">{{ $t("settings.parser.parseMethodOCR") }}</SelectItem>
                <SelectItem value="txt">{{ $t("settings.parser.parseMethodText") }}</SelectItem>
              </SelectContent>
            </Select>
            <p class="text-placeholder m-0 mt-1 text-xs leading-normal">{{ $t("settings.parser.parseMethodHint") }}</p>
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ $t("settings.parser.featuresLabel", "识别选项") }}
            </label>
            <div class="flex flex-wrap gap-4 pt-2">
              <label class="flex cursor-pointer items-center gap-2 text-[13px]">
                <Checkbox
                  :model-value="config.mineru_enable_formula"
                  @update:model-value="(v: boolean | 'indeterminate') => (config.mineru_enable_formula = v === true)"
                />
                {{ $t("settings.parser.formulaRecognition") }}
              </label>
              <label class="flex cursor-pointer items-center gap-2 text-[13px]">
                <Checkbox
                  :model-value="config.mineru_enable_table"
                  @update:model-value="(v: boolean | 'indeterminate') => (config.mineru_enable_table = v === true)"
                />
                {{ $t("settings.parser.tableRecognition") }}
              </label>
            </div>
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ t("settings.parser.language") }}
            </label>
            <SettingsInput
              v-model="config.mineru_language"
              :placeholder="$t('settings.parser.languagePlaceholder')"
              clearable
              input-class="text-[13px] md:text-[13px]"
            />
          </div>
        </section>

        <!-- Section 3 — mineru_cloud 云 API 配置 -->
        <section v-if="currentEngine.Name === 'mineru_cloud'" class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t("settings.parser.configSection", "配置") }}</h4>

          <div>
            <label
              class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
            >
              API Key
            </label>
            <SettingsInput
              v-model="config.mineru_api_key"
              type="password"
              :placeholder="$t('settings.parser.mineruCloudApiKeyPlaceholder')"
              clearable
              :prefix-icon="LockIcon"
              input-class="text-[13px] md:text-[13px]"
            />
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">Model Version</label>
            <!-- clearable: like the old t-select, clearing unsets the value (undefined), leaving the choice to the default. -->
            <div class="group/select-clear relative">
              <Select v-model="config.mineru_cloud_model">
                <SelectTrigger class="w-full text-[13px]">
                  <SelectValue :placeholder="$t('settings.parser.defaultPipeline')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="pipeline">pipeline</SelectItem>
                  <SelectItem value="vlm">{{ $t("settings.parser.vlmLabel") }}</SelectItem>
                  <SelectItem value="MinerU-HTML">{{ $t("settings.parser.mineruHtmlLabel") }}</SelectItem>
                </SelectContent>
              </Select>
              <SettingsSelectClear
                :visible="!!config.mineru_cloud_model"
                @clear="config.mineru_cloud_model = undefined"
              />
            </div>
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ $t("settings.parser.featuresLabel", "识别选项") }}
            </label>
            <div class="flex flex-wrap gap-4 pt-2">
              <label class="flex cursor-pointer items-center gap-2 text-[13px]">
                <Checkbox
                  :model-value="config.mineru_cloud_enable_formula"
                  @update:model-value="
                    (v: boolean | 'indeterminate') => (config.mineru_cloud_enable_formula = v === true)
                  "
                />
                {{ $t("settings.parser.formulaRecognition") }}
              </label>
              <label class="flex cursor-pointer items-center gap-2 text-[13px]">
                <Checkbox
                  :model-value="config.mineru_cloud_enable_table"
                  @update:model-value="
                    (v: boolean | 'indeterminate') => (config.mineru_cloud_enable_table = v === true)
                  "
                />
                {{ $t("settings.parser.tableRecognition") }}
              </label>
              <label class="flex cursor-pointer items-center gap-2 text-[13px]">
                <Checkbox
                  :model-value="config.mineru_cloud_enable_ocr"
                  @update:model-value="(v: boolean | 'indeterminate') => (config.mineru_cloud_enable_ocr = v === true)"
                />
                OCR
              </label>
            </div>
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ t("settings.parser.language") }}
            </label>
            <SettingsInput
              v-model="config.mineru_cloud_language"
              :placeholder="$t('settings.parser.languagePlaceholder')"
              clearable
              input-class="text-[13px] md:text-[13px]"
            />
          </div>
        </section>

        <!-- Section 3 — mineru_tianshu 天枢自建配置 -->
        <section v-if="currentEngine.Name === 'mineru_tianshu'" class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t("settings.parser.configSection", "配置") }}</h4>

          <div>
            <label
              class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
            >
              {{ t("settings.parser.selfHostedEndpoint") }}
            </label>
            <SettingsInput
              v-model="config.mineru_tianshu_endpoint"
              :placeholder="$t('settings.parser.tianshuEndpointPlaceholder')"
              clearable
              input-class="text-[13px] md:text-[13px]"
            />
            <p class="text-placeholder m-0 mt-1 text-xs leading-normal">
              {{ $t("settings.parser.tianshuEndpointHint") }}
            </p>
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">API Key</label>
            <SettingsInput
              v-model="config.mineru_tianshu_api_key"
              type="password"
              :placeholder="$t('settings.parser.tianshuApiKeyPlaceholder')"
              clearable
              :prefix-icon="LockIcon"
              input-class="text-[13px] md:text-[13px]"
            />
            <p class="text-placeholder m-0 mt-1 text-xs leading-normal">
              {{ $t("settings.parser.tianshuApiKeyHint") }}
            </p>
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">Backend</label>
            <!-- clearable: like the old t-select, clearing unsets the value (undefined), leaving the choice to the default. -->
            <div class="group/select-clear relative">
              <Select v-model="config.mineru_tianshu_backend">
                <SelectTrigger class="w-full text-[13px]">
                  <SelectValue :placeholder="$t('settings.parser.tianshuServerDefault')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="pipeline">pipeline</SelectItem>
                  <SelectItem value="vlm-transformers">vlm-transformers</SelectItem>
                  <SelectItem value="vlm-vllm-engine">vlm-vllm-engine</SelectItem>
                  <SelectItem value="auto">auto</SelectItem>
                </SelectContent>
              </Select>
              <SettingsSelectClear
                :visible="!!config.mineru_tianshu_backend"
                @clear="config.mineru_tianshu_backend = undefined"
              />
            </div>
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ $t("settings.parser.parseMethodLabel") }}
            </label>
            <!-- clearable: like the old t-select, clearing unsets the value (undefined), leaving the choice to the default. -->
            <div class="group/select-clear relative">
              <Select v-model="config.mineru_tianshu_parse_method">
                <SelectTrigger class="w-full text-[13px]">
                  <SelectValue :placeholder="$t('settings.parser.tianshuServerDefault')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="auto">{{ $t("settings.parser.parseMethodAuto") }}</SelectItem>
                  <SelectItem value="ocr">{{ $t("settings.parser.parseMethodOCR") }}</SelectItem>
                  <SelectItem value="txt">{{ $t("settings.parser.parseMethodText") }}</SelectItem>
                </SelectContent>
              </Select>
              <SettingsSelectClear
                :visible="!!config.mineru_tianshu_parse_method"
                @clear="config.mineru_tianshu_parse_method = undefined"
              />
            </div>
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ $t("settings.parser.featuresLabel", "识别选项") }}
            </label>
            <div class="flex flex-wrap gap-4 pt-2">
              <label class="flex cursor-pointer items-center gap-2 text-[13px]">
                <Checkbox
                  :model-value="config.mineru_tianshu_enable_formula"
                  @update:model-value="
                    (v: boolean | 'indeterminate') => (config.mineru_tianshu_enable_formula = v === true)
                  "
                />
                {{ $t("settings.parser.formulaRecognition") }}
              </label>
              <label class="flex cursor-pointer items-center gap-2 text-[13px]">
                <Checkbox
                  :model-value="config.mineru_tianshu_enable_table"
                  @update:model-value="
                    (v: boolean | 'indeterminate') => (config.mineru_tianshu_enable_table = v === true)
                  "
                />
                {{ $t("settings.parser.tableRecognition") }}
              </label>
            </div>
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ t("settings.parser.language") }}
            </label>
            <SettingsInput
              v-model="config.mineru_tianshu_language"
              :placeholder="$t('settings.parser.languagePlaceholder')"
              clearable
              input-class="text-[13px] md:text-[13px]"
            />
          </div>
        </section>

        <!-- Section 3 — paddleocr_vl 自建配置 -->
        <section v-if="currentEngine.Name === 'paddleocr_vl'" class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t("settings.parser.configSection", "配置") }}</h4>

          <div>
            <label
              class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
            >
              {{ t("settings.parser.selfHostedEndpoint") }}
            </label>
            <SettingsInput
              v-model="config.paddleocr_vl_endpoint"
              :placeholder="$t('settings.parser.paddleocrVlEndpointPlaceholder')"
              clearable
              input-class="text-[13px] md:text-[13px]"
            />
            <p class="text-placeholder m-0 mt-1 text-xs leading-normal">
              {{ $t("settings.parser.paddleocrVlEndpointHint") }}
            </p>
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ $t("settings.parser.featuresLabel", "识别选项") }}
            </label>
            <div class="flex flex-wrap gap-4 pt-2">
              <label class="flex cursor-pointer items-center gap-2 text-[13px]">
                <Checkbox
                  :model-value="config.paddleocr_vl_use_seal_recognition"
                  @update:model-value="
                    (v: boolean | 'indeterminate') => (config.paddleocr_vl_use_seal_recognition = v === true)
                  "
                />
                {{ $t("settings.parser.sealRecognition") }}
              </label>
              <label class="flex cursor-pointer items-center gap-2 text-[13px]">
                <Checkbox
                  :model-value="config.paddleocr_vl_use_chart_recognition"
                  @update:model-value="
                    (v: boolean | 'indeterminate') => (config.paddleocr_vl_use_chart_recognition = v === true)
                  "
                />
                {{ $t("settings.parser.chartRecognition") }}
              </label>
            </div>
          </div>
        </section>

        <!-- Section 3 — paddleocr_vl_cloud 云 API 配置 -->
        <section v-if="currentEngine.Name === 'paddleocr_vl_cloud'" class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ $t("settings.parser.configSection", "配置") }}</h4>

          <div>
            <label
              class="text-foreground before:text-destructive mb-1.5 block text-[13px] leading-[1.4] font-medium before:mr-1 before:leading-none before:font-medium before:content-['*']"
            >
              Token
            </label>
            <SettingsInput
              v-model="config.paddleocr_vl_cloud_token"
              type="password"
              :placeholder="$t('settings.parser.paddleocrVlCloudTokenPlaceholder')"
              clearable
              :prefix-icon="LockIcon"
              input-class="text-[13px] md:text-[13px]"
            />
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">Model</label>
            <SettingsInput
              v-model="config.paddleocr_vl_cloud_model"
              placeholder="PaddleOCR-VL-1.6"
              clearable
              input-class="text-[13px] md:text-[13px]"
            />
          </div>
          <div>
            <label class="text-foreground mb-1.5 block text-[13px] leading-[1.4] font-medium">
              {{ $t("settings.parser.featuresLabel", "识别选项") }}
            </label>
            <div class="flex flex-wrap gap-4 pt-2">
              <label class="flex cursor-pointer items-center gap-2 text-[13px]">
                <Checkbox
                  :model-value="config.paddleocr_vl_cloud_use_seal_recognition"
                  @update:model-value="
                    (v: boolean | 'indeterminate') => (config.paddleocr_vl_cloud_use_seal_recognition = v === true)
                  "
                />
                {{ $t("settings.parser.sealRecognition") }}
              </label>
              <label class="flex cursor-pointer items-center gap-2 text-[13px]">
                <Checkbox
                  :model-value="config.paddleocr_vl_cloud_use_chart_recognition"
                  @update:model-value="
                    (v: boolean | 'indeterminate') => (config.paddleocr_vl_cloud_use_chart_recognition = v === true)
                  "
                />
                {{ $t("settings.parser.chartRecognition") }}
              </label>
            </div>
          </div>
        </section>
      </div>
    </SettingDrawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useI18n } from "vue-i18n";
import { useAuthStore } from "@/stores/auth";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import SettingsInput from "./SettingsInput.vue";
import SettingsSelectClear from "./SettingsSelectClear.vue";
import {
  getParserEngines,
  getParserEngineConfig,
  updateParserEngineConfig,
  getPlatformParserEngineConfig,
  updatePlatformParserEngineConfig,
  checkParserEngines,
  type ParserEngineInfo,
  type ParserEngineConfig,
} from "@/api/system";

import { Alert, AlertAction, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { CircleAlertIcon, CircleCheckIcon, CircleXIcon, LinkIcon, Loader2Icon, LockIcon } from "@lucide/vue";

const { t } = useI18n();
const authStore = useAuthStore();

const CONFIGURABLE_ENGINES = new Set([
  "mineru",
  "mineru_cloud",
  "mineru_tianshu",
  "paddleocr_vl",
  "paddleocr_vl_cloud",
]);

/** 各解析引擎的项目/官方文档地址 */
const ENGINE_DOC_LINKS: Record<string, string> = {
  markitdown: "https://github.com/microsoft/markitdown",
  mineru: "https://github.com/opendatalab/MinerU",
  mineru_cloud: "https://mineru.net/apiManage/docs",
  paddleocr_vl: "https://github.com/PaddlePaddle/PaddleOCR",
  paddleocr_vl_cloud: "https://aistudio.baidu.com/paddleocr",
};

/** 解析引擎配置默认值（与 DocReader/Python 侧一致） */
const DEFAULT_PARSER_CONFIG: ParserEngineConfig = {
  docreader_addr: "",
  docreader_transport: "grpc",
  mineru_endpoint: "",
  mineru_api_key: "",
  mineru_model: "pipeline",
  mineru_vlm_server_url: "",
  mineru_enable_formula: true,
  mineru_enable_table: true,
  mineru_parse_method: "auto",
  mineru_enable_ocr: true,
  mineru_language: "ch",
  mineru_cloud_model: "pipeline",
  mineru_cloud_enable_formula: true,
  mineru_cloud_enable_table: true,
  mineru_cloud_enable_ocr: true,
  mineru_cloud_language: "ch",
  paddleocr_vl_endpoint: "",
  paddleocr_vl_use_seal_recognition: true,
  paddleocr_vl_use_chart_recognition: false,
  paddleocr_vl_cloud_token: "",
  paddleocr_vl_cloud_model: "PaddleOCR-VL-1.6",
  paddleocr_vl_cloud_use_seal_recognition: true,
  paddleocr_vl_cloud_use_chart_recognition: false,
  mineru_tianshu_endpoint: "",
  mineru_tianshu_api_key: "",
  mineru_tianshu_auth_header: "",
  mineru_tianshu_backend: "",
  mineru_tianshu_language: "",
  mineru_tianshu_parse_method: "",
  mineru_tianshu_enable_formula: true,
  mineru_tianshu_enable_table: true,
};

const engines = ref<ParserEngineInfo[]>([]);
const docreaderAddrEnv = ref("");
const docreaderTransport = ref<"grpc" | "http">("grpc");
const connected = ref(false);
const loading = ref(true);
const error = ref("");

const config = ref<ParserEngineConfig>({ ...DEFAULT_PARSER_CONFIG });
const saving = ref(false);
const saveMessage = ref("");
const saveSuccess = ref(false);
const checking = ref(false);
const checkMessage = ref("");

const hasBuiltinEngine = computed(() => engines.value.some((e) => e.Name === "builtin"));

const drawerVisible = ref(false);
const currentEngine = ref<ParserEngineInfo | null>(null);
const drawerTitle = computed(() => {
  return currentEngine.value ? getEngineDisplayName(currentEngine.value.Name) : "";
});

// Whether the footer test-connection button should appear. Engines without
// configurable fields and that aren't the builtin DocReader (whose connection
// status is the whole point of the drawer) skip the test affordance — for
// e.g. simple/markitdown there's nothing to validate beyond presence.
const needsTestButton = computed(() => {
  if (!currentEngine.value) return false;
  return hasConfigFields(currentEngine.value.Name) || currentEngine.value.Name === "builtin";
});

/** 固定展示顺序，未列出的引擎排在末尾按名称排序 */
const ENGINE_ORDER: Record<string, number> = {
  builtin: 0,
  simple: 2,
  anydoc: 3,
  markitdown: 4,
  mineru: 5,
  mineru_cloud: 6,
  mineru_tianshu: 7,
  paddleocr_vl: 8,
  paddleocr_vl_cloud: 9,
};

/**
 * Engines hidden from the picker. An engine already bound to a knowledge base
 * keeps working server-side; it just cannot be newly selected here. Currently
 * empty — the mechanism is kept so retiring an engine needs no filter changes.
 */
const HIDDEN_ENGINES = new Set<string>();

const sortedEngines = computed(() => {
  return [...engines.value]
    .filter((e) => !HIDDEN_ENGINES.has(e.Name))
    .sort((a, b) => {
      const oa = ENGINE_ORDER[a.Name] ?? 100;
      const ob = ENGINE_ORDER[b.Name] ?? 100;
      if (oa !== ob) return oa - ob;
      return a.Name.localeCompare(b.Name);
    });
});

function hasConfigFields(engineName: string): boolean {
  return CONFIGURABLE_ENGINES.has(engineName);
}

function engineDocLink(name: string): string | undefined {
  return ENGINE_DOC_LINKS[name];
}

function engineDocLabel(_name: string): string {
  return t("settings.parser.docs");
}

// 卡片徽章首字母。优先用本地化名称的首字符（覆盖如「内置/简易」等中文场景），
// 兜底回到 engine name；保证英文/中文都能显示一个稳定的可读 monogram。
function engineInitial(engineName: string): string {
  const display = getEngineDisplayName(engineName);
  return (display.trim().charAt(0) || engineName.charAt(0) || "?").toUpperCase();
}

function getEngineDisplayName(engineName: string): string {
  const key = `kbSettings.parser.engines.${engineName}.name`;
  const translated = t(key);
  return translated !== key ? translated : engineName;
}

function getEngineDisplayDesc(engineName: string, fallback: string): string {
  const key = `kbSettings.parser.engines.${engineName}.desc`;
  const translated = t(key);
  return translated !== key ? translated : fallback;
}

function openDrawer(engine: ParserEngineInfo) {
  currentEngine.value = engine;
  drawerVisible.value = true;
  saveMessage.value = "";
  checkMessage.value = "";
}

async function loadEngines() {
  try {
    const res = await getParserEngines();
    engines.value = res?.data ?? [];
    docreaderAddrEnv.value = res?.docreader_addr ?? "";
    const transport = (res?.docreader_transport ?? "grpc").toLowerCase();
    docreaderTransport.value = transport === "http" ? "http" : "grpc";
    connected.value = res?.connected ?? engines.value.length > 0;
  } catch (e: any) {
    error.value = e?.message || t("settings.parser.loadFailed");
    engines.value = [];
    connected.value = false;
  }
}

// 'platform' 编辑部署级默认，'workspace' 编辑本空间覆盖。非系统管理员固定在
// workspace —— 他们根本拿不到平台端点（/system/admin/* 是系统管理员专属）。
type ParserConfigScope = "platform" | "workspace";
const scope = ref<ParserConfigScope>(authStore.isSystemAdmin ? "platform" : "workspace");
const canEditPlatformScope = computed(() => authStore.isSystemAdmin);

const scopeOptions = computed(() => [
  { value: "platform" as ParserConfigScope, label: t("settings.parser.scope.platform") },
  { value: "workspace" as ParserConfigScope, label: t("settings.parser.scope.workspace") },
]);

// 分段控件选中：写回 scope 并沿用原 onScopeChange 语义。
function selectScope(value: ParserConfigScope) {
  if (scope.value === value) return;
  scope.value = value;
  void onScopeChange();
}

async function onScopeChange() {
  saveMessage.value = "";
  checkMessage.value = "";
  await loadConfig();
}

async function loadConfig() {
  try {
    const res = scope.value === "platform" ? await getPlatformParserEngineConfig() : await getParserEngineConfig();
    const data = res?.data;
    config.value = {
      docreader_addr: data?.docreader_addr ?? DEFAULT_PARSER_CONFIG.docreader_addr ?? "",
      docreader_transport: data?.docreader_transport ?? DEFAULT_PARSER_CONFIG.docreader_transport ?? "grpc",
      mineru_endpoint: data?.mineru_endpoint ?? DEFAULT_PARSER_CONFIG.mineru_endpoint ?? "",
      mineru_api_key: data?.mineru_api_key ?? DEFAULT_PARSER_CONFIG.mineru_api_key ?? "",
      mineru_model: data?.mineru_model ?? DEFAULT_PARSER_CONFIG.mineru_model ?? "",
      mineru_vlm_server_url: data?.mineru_vlm_server_url ?? DEFAULT_PARSER_CONFIG.mineru_vlm_server_url ?? "",
      mineru_enable_formula: data?.mineru_enable_formula ?? DEFAULT_PARSER_CONFIG.mineru_enable_formula ?? true,
      mineru_enable_table: data?.mineru_enable_table ?? DEFAULT_PARSER_CONFIG.mineru_enable_table ?? true,
      mineru_parse_method: data?.mineru_parse_method ?? (data?.mineru_enable_ocr === false ? "txt" : "auto"),
      mineru_enable_ocr: data?.mineru_enable_ocr ?? DEFAULT_PARSER_CONFIG.mineru_enable_ocr ?? true,
      mineru_language: data?.mineru_language ?? DEFAULT_PARSER_CONFIG.mineru_language ?? "ch",
      mineru_cloud_model: data?.mineru_cloud_model ?? DEFAULT_PARSER_CONFIG.mineru_cloud_model ?? "",
      mineru_cloud_enable_formula:
        data?.mineru_cloud_enable_formula ?? DEFAULT_PARSER_CONFIG.mineru_cloud_enable_formula ?? true,
      mineru_cloud_enable_table:
        data?.mineru_cloud_enable_table ?? DEFAULT_PARSER_CONFIG.mineru_cloud_enable_table ?? true,
      mineru_cloud_enable_ocr: data?.mineru_cloud_enable_ocr ?? DEFAULT_PARSER_CONFIG.mineru_cloud_enable_ocr ?? true,
      mineru_cloud_language: data?.mineru_cloud_language ?? DEFAULT_PARSER_CONFIG.mineru_cloud_language ?? "ch",
      paddleocr_vl_endpoint: data?.paddleocr_vl_endpoint ?? DEFAULT_PARSER_CONFIG.paddleocr_vl_endpoint ?? "",
      paddleocr_vl_use_seal_recognition:
        data?.paddleocr_vl_use_seal_recognition ?? DEFAULT_PARSER_CONFIG.paddleocr_vl_use_seal_recognition ?? true,
      paddleocr_vl_use_chart_recognition:
        data?.paddleocr_vl_use_chart_recognition ?? DEFAULT_PARSER_CONFIG.paddleocr_vl_use_chart_recognition ?? false,
      paddleocr_vl_cloud_token: data?.paddleocr_vl_cloud_token ?? DEFAULT_PARSER_CONFIG.paddleocr_vl_cloud_token ?? "",
      paddleocr_vl_cloud_model:
        data?.paddleocr_vl_cloud_model ?? DEFAULT_PARSER_CONFIG.paddleocr_vl_cloud_model ?? "PaddleOCR-VL-1.6",
      paddleocr_vl_cloud_use_seal_recognition:
        data?.paddleocr_vl_cloud_use_seal_recognition ??
        DEFAULT_PARSER_CONFIG.paddleocr_vl_cloud_use_seal_recognition ??
        true,
      paddleocr_vl_cloud_use_chart_recognition:
        data?.paddleocr_vl_cloud_use_chart_recognition ??
        DEFAULT_PARSER_CONFIG.paddleocr_vl_cloud_use_chart_recognition ??
        false,
      mineru_tianshu_endpoint: data?.mineru_tianshu_endpoint ?? "",
      mineru_tianshu_api_key: data?.mineru_tianshu_api_key ?? "",
      mineru_tianshu_auth_header: data?.mineru_tianshu_auth_header ?? "",
      mineru_tianshu_backend: data?.mineru_tianshu_backend ?? "",
      mineru_tianshu_language: data?.mineru_tianshu_language ?? "",
      mineru_tianshu_parse_method: data?.mineru_tianshu_parse_method ?? "",
      mineru_tianshu_enable_formula: data?.mineru_tianshu_enable_formula ?? true,
      mineru_tianshu_enable_table: data?.mineru_tianshu_enable_table ?? true,
    };
  } catch {
    config.value = { ...DEFAULT_PARSER_CONFIG };
  }
}

async function loadAll() {
  loading.value = true;
  error.value = "";
  await Promise.all([loadEngines(), loadConfig()]);
  loading.value = false;
}

function buildConfigPayload(): ParserEngineConfig {
  return {
    docreader_addr: config.value.docreader_addr?.trim() ?? "",
    docreader_transport: (config.value.docreader_transport ?? "grpc").trim() || "grpc",
    mineru_endpoint: config.value.mineru_endpoint?.trim() ?? "",
    mineru_api_key: config.value.mineru_api_key?.trim() ?? "",
    mineru_model: config.value.mineru_model?.trim() ?? "",
    mineru_vlm_server_url: config.value.mineru_vlm_server_url?.trim() ?? "",
    mineru_enable_formula: config.value.mineru_enable_formula,
    mineru_enable_table: config.value.mineru_enable_table,
    mineru_parse_method: config.value.mineru_parse_method ?? "auto",
    // Keep the legacy toggle during rolling upgrades. New servers prefer parse_method.
    mineru_enable_ocr: config.value.mineru_parse_method !== "txt",
    mineru_language: config.value.mineru_language?.trim() ?? "",
    mineru_cloud_model: config.value.mineru_cloud_model?.trim() ?? "",
    mineru_cloud_enable_formula: config.value.mineru_cloud_enable_formula,
    mineru_cloud_enable_table: config.value.mineru_cloud_enable_table,
    mineru_cloud_enable_ocr: config.value.mineru_cloud_enable_ocr,
    mineru_cloud_language: config.value.mineru_cloud_language?.trim() ?? "",
    paddleocr_vl_endpoint: config.value.paddleocr_vl_endpoint?.trim() ?? "",
    paddleocr_vl_use_seal_recognition: config.value.paddleocr_vl_use_seal_recognition,
    paddleocr_vl_use_chart_recognition: config.value.paddleocr_vl_use_chart_recognition,
    paddleocr_vl_cloud_token: config.value.paddleocr_vl_cloud_token?.trim() ?? "",
    paddleocr_vl_cloud_model: config.value.paddleocr_vl_cloud_model?.trim() ?? "",
    paddleocr_vl_cloud_use_seal_recognition: config.value.paddleocr_vl_cloud_use_seal_recognition,
    paddleocr_vl_cloud_use_chart_recognition: config.value.paddleocr_vl_cloud_use_chart_recognition,
    mineru_tianshu_endpoint: config.value.mineru_tianshu_endpoint?.trim() ?? "",
    mineru_tianshu_api_key: config.value.mineru_tianshu_api_key?.trim() ?? "",
    mineru_tianshu_auth_header: config.value.mineru_tianshu_auth_header?.trim() ?? "",
    mineru_tianshu_backend: config.value.mineru_tianshu_backend?.trim() ?? "",
    mineru_tianshu_language: config.value.mineru_tianshu_language?.trim() ?? "",
    mineru_tianshu_parse_method: config.value.mineru_tianshu_parse_method?.trim() ?? "",
    mineru_tianshu_enable_formula: config.value.mineru_tianshu_enable_formula,
    mineru_tianshu_enable_table: config.value.mineru_tianshu_enable_table,
  };
}

async function onCheck() {
  // `.value`, not the ref: the ref object is always truthy, so the guard
  // below had never once fired.
  if (!connected.value) {
    checkMessage.value = t("settings.parser.ensureDocreaderConnected");
    return;
  }
  checking.value = true;
  checkMessage.value = "";
  saveMessage.value = "";
  try {
    const res = await checkParserEngines(buildConfigPayload());
    engines.value = res?.data ?? [];
    if (res?.connected !== undefined) {
      connected.value = res.connected;
    }

    if (currentEngine.value) {
      if (currentEngine.value.Name === "builtin") {
        if (connected.value) {
          checkMessage.value = t("settings.parser.checkSuccess", "测试连接成功");
          saveSuccess.value = true;
        } else {
          checkMessage.value = t("settings.parser.checkFailed", "测试连接失败");
          saveSuccess.value = false;
        }
      } else {
        const updatedEngine = engines.value.find((e) => e.Name === currentEngine.value!.Name);
        if (updatedEngine) {
          if (updatedEngine.Available) {
            checkMessage.value = t("settings.parser.checkSuccess", "测试连接成功");
            saveSuccess.value = true;
          } else {
            checkMessage.value = updatedEngine.UnavailableReason || t("settings.parser.checkFailed", "测试连接失败");
            saveSuccess.value = false;
          }
        } else {
          checkMessage.value = t("settings.parser.checkFailed", "引擎状态未知");
          saveSuccess.value = false;
        }
      }
    } else {
      checkMessage.value = t("settings.parser.checkDoneStatusUpdated", "检测已完成，状态已更新");
      saveSuccess.value = true;
    }

    setTimeout(() => {
      checkMessage.value = "";
    }, 3000);
  } catch (e: any) {
    checkMessage.value = e?.message || t("settings.parser.checkFailed", "测试连接失败");
    saveSuccess.value = false;
  } finally {
    checking.value = false;
  }
}

async function onSave() {
  saving.value = true;
  saveMessage.value = "";
  try {
    await (scope.value === "platform"
      ? updatePlatformParserEngineConfig(buildConfigPayload())
      : updateParserEngineConfig(buildConfigPayload()));
    saveSuccess.value = true;
    saveMessage.value = t("settings.parser.saveSuccess");
    drawerVisible.value = false;
    loadEngines();
  } catch (e: any) {
    saveSuccess.value = false;
    saveMessage.value = e?.message || t("settings.parser.saveFailed");
  } finally {
    saving.value = false;
  }
}

onMounted(loadAll);
</script>

<style scoped>
/* Per-engine badge tints, mirrored onto the teleported drawer header by the
   non-scoped block below. Engines not listed keep the default blue. */
.engine-card__badge {
  background: rgba(0, 82, 217, 0.1);
  color: #0052d9;
}

.engine-card--builtin .engine-card__badge,
.engine-card--builtin-legacy .engine-card__badge {
  background: rgba(7, 192, 95, 0.12);
  color: #07c05f;
}
.engine-card--simple .engine-card__badge {
  background: rgba(70, 70, 70, 0.1);
  color: #464646;
}
.engine-card--markitdown .engine-card__badge {
  background: rgba(0, 137, 255, 0.12);
  color: #0089ff;
}
.engine-card--mineru .engine-card__badge,
.engine-card--mineru_cloud .engine-card__badge,
.engine-card--paddleocr_vl .engine-card__badge,
.engine-card--paddleocr_vl_cloud .engine-card__badge {
  background: rgba(98, 53, 187, 0.12);
  color: #6235bb;
}
</style>

<!--
  Non-scoped block: per-engine header-icon coloring. Same approach as
  StorageEngineSettings — keep these rules global so they always apply
  regardless of whether the drawer panel inherits the parent's scoped
  data attributes. Each rule mirrors the matching .engine-card--{name}
  .engine-card__badge from the scoped block above.
-->
<style>
.parser-engine-drawer--builtin .setting-drawer__header-icon,
.parser-engine-drawer--builtin-legacy .setting-drawer__header-icon {
  background: rgba(7, 192, 95, 0.12);
  color: #07c05f;
}
.parser-engine-drawer--simple .setting-drawer__header-icon {
  background: rgba(70, 70, 70, 0.1);
  color: #464646;
}
.parser-engine-drawer--markitdown .setting-drawer__header-icon {
  background: rgba(0, 137, 255, 0.12);
  color: #0089ff;
}
.parser-engine-drawer--mineru .setting-drawer__header-icon,
.parser-engine-drawer--mineru_cloud .setting-drawer__header-icon,
.parser-engine-drawer--paddleocr_vl .setting-drawer__header-icon,
.parser-engine-drawer--paddleocr_vl_cloud .setting-drawer__header-icon {
  background: rgba(98, 53, 187, 0.12);
  color: #6235bb;
}
</style>
