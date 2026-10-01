<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="visible"
        class="fixed inset-0 z-[1000] flex items-center justify-center bg-black/50 backdrop-blur-sm"
        @click.self="handleClose"
      >
        <div
          class="settings-modal bg-card relative flex h-[85vh] max-h-[750px] w-[90vw] max-w-[1000px] flex-col overflow-hidden rounded-xl shadow-[0_8px_32px_rgba(0,0,0,0.12)]"
        >
          <!-- 关闭按钮 -->
          <button
            type="button"
            data-slot="modal-close"
            class="bg-muted text-muted-foreground hover:text-foreground absolute top-5 right-5 z-10 flex h-8 w-8 cursor-pointer items-center justify-center rounded-md border-none transition-all duration-200"
            @click="handleClose"
            :aria-label="$t('general.close')"
          >
            <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
              <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
            </svg>
          </button>

          <div class="flex h-full w-full overflow-hidden">
            <!-- 左侧导航 -->
            <div
              class="border-border flex w-[208px] shrink-0 flex-col overflow-hidden border-r bg-[var(--td-bg-color-settings-modal)]"
            >
              <div class="border-border shrink-0 border-b px-3.5 pt-4 pb-3">
                <h2 class="text-foreground m-0 text-base font-semibold">
                  {{ editorMode === "create" ? $t("knowledgeEditor.titleCreate") : $t("knowledgeEditor.titleEdit") }}
                </h2>
              </div>
              <div class="min-h-0 flex-1 overflow-y-auto p-2 pb-3" data-guide="kb-editor-sidebar">
                <template v-for="group in navGroups" :key="group.key">
                  <div
                    class="text-placeholder pt-2 pr-3.5 pb-0.5 pl-3.5 text-xs font-semibold tracking-[0.02em] first:pt-0.5"
                  >
                    {{ group.label }}
                  </div>
                  <div
                    v-for="(item, index) in group.items"
                    :key="index"
                    class="mb-0.5 flex cursor-pointer items-center rounded-md px-3 py-1.5 text-sm transition-all duration-200 select-none"
                    :class="
                      currentSection === item.key
                        ? 'bg-muted text-primary font-medium'
                        : 'text-foreground hover:bg-accent'
                    "
                    :data-guide="`kb-editor-nav-${item.key}`"
                    @click="currentSection = item.key"
                  >
                    <component
                      :is="navIcon(item.icon)"
                      class="mr-[9px] flex size-4 shrink-0 items-center justify-center"
                    />
                    <span class="flex-1">{{ item.label }}</span>
                    <span
                      v-if="item.badge"
                      class="bg-muted text-muted-foreground ml-0.5 rounded-lg px-1.5 text-[11px] leading-4 font-medium"
                    >
                      {{ item.badge }}
                    </span>
                  </div>
                </template>
              </div>
            </div>

            <!-- 右侧内容区域 -->
            <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
              <div class="min-h-0 flex-1 overflow-y-auto px-8 py-6">
                <!-- 基本信息 -->
                <div v-show="currentSection === 'basic'" class="mb-8 last:mb-0">
                  <div v-if="formData">
                    <div class="mb-4">
                      <h3 class="text-foreground m-0 mb-1.5 [font-family:var(--app-font-family)] text-xl font-semibold">
                        {{ $t("knowledgeEditor.basic.title") }}
                      </h3>
                      <p class="text-placeholder m-0 [font-family:var(--app-font-family)] text-sm leading-[22px]">
                        {{ $t("knowledgeEditor.basic.description") }}
                      </p>
                    </div>
                    <div>
                      <div v-if="editorMode === 'edit' && activeKbId" class="mb-4 last:mb-0">
                        <label
                          class="text-foreground mb-2 block [font-family:var(--app-font-family)] text-[15px] font-medium"
                        >
                          {{ $t("knowledgeEditor.basic.kbId") }}
                        </label>
                        <p class="text-placeholder mt-1.5 text-xs">
                          {{
                            isPostCreateSession
                              ? $t("knowledgeEditor.postCreateHint.followUpDesc")
                              : $t("knowledgeEditor.basic.kbIdDesc")
                          }}
                        </p>
                        <div
                          class="border-border bg-muted mt-2 flex w-full max-w-[480px] items-center gap-1 rounded-md border py-1.5 pr-2 pl-3"
                        >
                          <code
                            class="text-foreground m-0 min-w-0 flex-1 truncate border-none bg-transparent p-0 [font-family:var(--app-font-family-mono)] text-[13px] leading-normal"
                            :title="activeKbId"
                          >
                            {{ activeKbId }}
                          </code>
                          <Tooltip>
                            <TooltipTrigger as-child>
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                class="text-muted-foreground hover:text-primary shrink-0"
                                :aria-label="$t('common.copy')"
                                @click="copyKbId"
                              >
                                <CopyIcon />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent side="top">{{ $t("common.copy") }}</TooltipContent>
                          </Tooltip>
                        </div>
                      </div>

                      <div class="mb-4 last:mb-0">
                        <label
                          class="text-foreground after:text-destructive mb-2 block [font-family:var(--app-font-family)] text-[15px] font-medium after:ml-1 after:content-['*']"
                        >
                          {{ $t("knowledgeEditor.basic.typeLabel") }}
                        </label>
                        <SegmentedRadio
                          :model-value="formData.type"
                          :options="[
                            { value: 'document', label: $t('knowledgeEditor.basic.typeDocument') },
                            { value: 'faq', label: $t('knowledgeEditor.basic.typeFAQ') },
                          ]"
                          :disabled="editorMode === 'edit'"
                          data-guide="kb-create-type"
                          @update:model-value="(val) => (formData.type = val)"
                        />
                        <p class="text-placeholder mt-1.5 text-xs">{{ $t("knowledgeEditor.basic.typeDescription") }}</p>
                      </div>

                      <!-- 索引策略 (紧跟类型选择) -->
                      <div v-if="!isFAQ" class="mb-4 last:mb-0">
                        <label
                          class="text-foreground after:text-destructive mb-2 block [font-family:var(--app-font-family)] text-[15px] font-medium after:ml-1 after:content-['*']"
                        >
                          {{ $t("knowledgeEditor.indexing.title") }}
                        </label>
                        <p class="text-placeholder mt-1.5 text-xs">{{ $t("knowledgeEditor.indexing.description") }}</p>
                        <div
                          class="mt-2.5 grid [grid-template-columns:repeat(auto-fit,minmax(260px,1fr))] gap-3"
                          data-guide="kb-create-indexing"
                        >
                          <div
                            class="flex flex-col gap-1.5 rounded-lg border px-3.5 py-3 transition-colors duration-200 select-none"
                            :class="[
                              formData.indexingStrategy.vectorEnabled
                                ? 'border-primary bg-[var(--td-brand-color-light)]'
                                : 'border-border bg-card',
                              isIndexingLocked
                                ? 'cursor-not-allowed opacity-70'
                                : 'hover:border-primary cursor-pointer',
                            ]"
                            @click="toggleVectorIndexing"
                          >
                            <div class="flex items-center gap-2">
                              <Checkbox
                                :model-value="formData.indexingStrategy.vectorEnabled"
                                :disabled="isIndexingLocked"
                                class="pointer-events-none"
                              />
                              <span class="text-foreground text-sm font-medium">{{
                                $t("knowledgeEditor.indexing.searchTitle")
                              }}</span>
                            </div>
                            <p class="text-placeholder m-0 pl-6 text-xs leading-[18px]">
                              {{ $t("knowledgeEditor.indexing.searchDesc") }}
                            </p>
                          </div>
                          <div
                            class="flex flex-col gap-1.5 rounded-lg border px-3.5 py-3 transition-colors duration-200 select-none"
                            :class="[
                              formData.indexingStrategy.wikiEnabled
                                ? 'border-primary bg-[var(--td-brand-color-light)]'
                                : 'border-border bg-card',
                              isIndexingLocked
                                ? 'cursor-not-allowed opacity-70'
                                : 'hover:border-primary cursor-pointer',
                            ]"
                            @click="toggleWikiIndexing"
                          >
                            <div class="flex items-center gap-2">
                              <Checkbox
                                :model-value="formData.indexingStrategy.wikiEnabled"
                                :disabled="isIndexingLocked"
                                class="pointer-events-none"
                              />
                              <span class="text-foreground inline-flex items-center gap-1.5 text-sm font-medium">
                                {{ $t("knowledgeEditor.indexing.wikiTitle") }}
                                <span
                                  class="text-primary inline-flex h-4 items-center rounded-[3px] bg-[var(--td-brand-color-light)] px-1.5 text-[10px] leading-none font-semibold tracking-[0.4px]"
                                >
                                  NEW
                                </span>
                              </span>
                            </div>
                            <p class="text-placeholder m-0 pl-6 text-xs leading-[18px]">
                              {{ $t("knowledgeEditor.indexing.wikiDesc") }}
                            </p>
                          </div>
                        </div>
                        <p v-if="isIndexingLocked" class="text-warning mt-2 text-xs">
                          {{ $t("knowledgeEditor.indexing.lockedTip") }}
                        </p>
                      </div>

                      <!-- Wiki 提取粒度 (仅当 Wiki 启用时显示) -->
                      <div v-if="!isFAQ && formData.indexingStrategy.wikiEnabled" class="mb-4 last:mb-0">
                        <label
                          class="text-foreground mb-2 block [font-family:var(--app-font-family)] text-[15px] font-medium"
                        >
                          {{ $t("knowledgeEditor.wiki.extractionGranularityLabel") }}
                        </label>
                        <p class="text-placeholder mt-1.5 text-xs">
                          {{ $t("knowledgeEditor.wiki.extractionGranularityTip") }}
                        </p>
                        <SegmentedRadio
                          :model-value="resolvedGranularity"
                          :options="[
                            { value: 'focused', label: $t('knowledgeEditor.wiki.granularityFocused') },
                            { value: 'standard', label: $t('knowledgeEditor.wiki.granularityStandard') },
                            { value: 'exhaustive', label: $t('knowledgeEditor.wiki.granularityExhaustive') },
                          ]"
                          class="mt-1"
                          @update:model-value="handleGranularityChange"
                        />
                        <p class="text-muted-foreground mt-2 text-xs leading-[1.6] break-words whitespace-normal">
                          {{ granularityHint }}
                        </p>
                      </div>

                      <div v-if="!isFAQ && formData.indexingStrategy.wikiEnabled" class="mb-4 last:mb-0">
                        <label
                          class="text-foreground mb-2 block [font-family:var(--app-font-family)] text-[15px] font-medium"
                        >
                          {{ $t("knowledgeEditor.wiki.contentInstructionsLabel") }}
                        </label>
                        <p class="text-placeholder mt-1.5 text-xs">
                          {{ $t("knowledgeEditor.wiki.contentInstructionsTip") }}
                        </p>
                        <Textarea
                          v-model="formData.wikiConfig.contentInstructions"
                          :placeholder="$t('knowledgeEditor.wiki.contentInstructionsPlaceholder')"
                          :maxlength="4000"
                          class="max-h-[186px] min-h-[76px]"
                        />
                      </div>

                      <div v-if="!isFAQ && formData.indexingStrategy.wikiEnabled" class="mb-4 last:mb-0">
                        <label
                          class="text-foreground mb-2 block [font-family:var(--app-font-family)] text-[15px] font-medium"
                        >
                          {{ $t("knowledgeEditor.wiki.extractionInstructionsLabel") }}
                        </label>
                        <p class="text-placeholder mt-1.5 text-xs">
                          {{ $t("knowledgeEditor.wiki.extractionInstructionsTip") }}
                        </p>
                        <Textarea
                          v-model="formData.wikiConfig.extractionInstructions"
                          :placeholder="$t('knowledgeEditor.wiki.extractionInstructionsPlaceholder')"
                          :maxlength="4000"
                          class="max-h-[186px] min-h-[76px]"
                        />
                      </div>

                      <div class="mb-4 last:mb-0" data-guide="kb-create-name">
                        <label
                          class="text-foreground after:text-destructive mb-2 block [font-family:var(--app-font-family)] text-[15px] font-medium after:ml-1 after:content-['*']"
                        >
                          {{ $t("knowledgeEditor.basic.nameLabel") }}
                        </label>
                        <Input
                          v-model="formData.name"
                          :placeholder="$t('knowledgeEditor.basic.namePlaceholder')"
                          :maxlength="50"
                        />
                      </div>
                      <div class="mb-4 last:mb-0">
                        <label
                          class="text-foreground mb-2 block [font-family:var(--app-font-family)] text-[15px] font-medium"
                        >
                          {{ $t("knowledgeEditor.basic.descriptionLabel") }}
                        </label>
                        <Textarea
                          v-model="formData.description"
                          :placeholder="$t('knowledgeEditor.basic.descriptionPlaceholder')"
                          :maxlength="200"
                          class="max-h-[142px] min-h-[76px]"
                        />
                      </div>
                      <!-- Periodic review: how long a document may go without
                           anybody confirming or changing it before its owner
                           is asked to look. Off unless someone commits to it. -->
                      <div class="mb-4 last:mb-0">
                        <label
                          for="kb-review-interval"
                          class="text-foreground mb-2 block [font-family:var(--app-font-family)] text-[15px] font-medium"
                        >
                          {{ $t("knowledgeEditor.basic.reviewIntervalLabel") }}
                        </label>
                        <div class="flex items-center gap-2">
                          <Input
                            id="kb-review-interval"
                            v-model.number="formData.reviewIntervalDays"
                            type="number"
                            min="0"
                            :max="MAX_REVIEW_INTERVAL_DAYS"
                            step="1"
                            class="w-28"
                            data-testid="kb-review-interval"
                          />
                          <span class="text-muted-foreground text-sm">{{
                            $t("knowledgeEditor.basic.reviewIntervalUnit")
                          }}</span>
                        </div>
                        <p class="text-placeholder m-0 mt-1.5 text-xs">
                          {{ $t("knowledgeEditor.basic.reviewIntervalHint") }}
                        </p>
                      </div>

                      <!-- Wiki 合成模型移至模型配置页 -->
                    </div>
                  </div>
                </div>

                <!-- 模型配置 -->
                <div v-show="currentSection === 'models'" class="mb-8 last:mb-0">
                  <KBModelConfig
                    ref="modelConfigRef"
                    v-if="formData"
                    :config="formData.modelConfig"
                    :has-files="hasFiles"
                    :wiki-enabled="formData.indexingStrategy?.wikiEnabled"
                    :rag-enabled="formData.indexingStrategy?.vectorEnabled || formData.indexingStrategy?.keywordEnabled"
                    :all-models="allModels"
                    @update:config="handleModelConfigUpdate"
                  />
                </div>

                <!-- VectorStore 绑定 -->
                <div v-show="currentSection === 'vectorStore'" class="mb-8 last:mb-0">
                  <KBVectorStoreSettings
                    v-if="formData"
                    :mode="editorMode"
                    :vector-store-id="formData.vectorStoreId"
                    :bound-source="formData.vectorStoreInfo?.source"
                    :bound-name="formData.vectorStoreInfo?.name"
                    :bound-engine-type="formData.vectorStoreInfo?.engineType"
                    :bound-status="formData.vectorStoreInfo?.status"
                    @update:vector-store-id="handleVectorStoreIdUpdate"
                  />
                </div>

                <!-- FAQ 配置 -->
                <div v-if="isFAQ && formData" v-show="currentSection === 'faq'" class="mb-8 last:mb-0">
                  <div>
                    <div class="mb-4">
                      <h3 class="text-foreground m-0 mb-1.5 [font-family:var(--app-font-family)] text-xl font-semibold">
                        {{ $t("knowledgeEditor.faq.title") }}
                      </h3>
                      <p class="text-placeholder m-0 [font-family:var(--app-font-family)] text-sm leading-[22px]">
                        {{ $t("knowledgeEditor.faq.description") }}
                      </p>
                    </div>
                    <div>
                      <div class="mb-4 last:mb-0">
                        <label
                          class="text-foreground after:text-destructive mb-2 block [font-family:var(--app-font-family)] text-[15px] font-medium after:ml-1 after:content-['*']"
                        >
                          {{ $t("knowledgeEditor.faq.indexModeLabel") }}
                        </label>
                        <SegmentedRadio
                          :model-value="formData.faqConfig.indexMode"
                          :options="[
                            { value: 'question_only', label: $t('knowledgeEditor.faq.modes.questionOnly') },
                            { value: 'question_answer', label: $t('knowledgeEditor.faq.modes.questionAnswer') },
                          ]"
                          @update:model-value="(val) => (formData.faqConfig.indexMode = val)"
                        />
                        <p class="text-placeholder mt-1.5 text-xs">
                          {{ $t("knowledgeEditor.faq.indexModeDescription") }}
                        </p>
                      </div>
                      <div class="mb-4 last:mb-0">
                        <label
                          class="text-foreground after:text-destructive mb-2 block [font-family:var(--app-font-family)] text-[15px] font-medium after:ml-1 after:content-['*']"
                        >
                          {{ $t("knowledgeEditor.faq.questionIndexModeLabel") }}
                        </label>
                        <SegmentedRadio
                          :model-value="formData.faqConfig.questionIndexMode"
                          :options="[
                            { value: 'combined', label: $t('knowledgeEditor.faq.modes.combined') },
                            { value: 'separate', label: $t('knowledgeEditor.faq.modes.separate') },
                          ]"
                          @update:model-value="(val) => (formData.faqConfig.questionIndexMode = val)"
                        />
                        <p class="text-placeholder mt-1.5 text-xs">
                          {{ $t("knowledgeEditor.faq.questionIndexModeDescription") }}
                        </p>
                      </div>
                      <div class="bg-muted text-muted-foreground mt-5 rounded-lg p-3 px-4 text-[13px] leading-5">
                        <p>{{ $t("knowledgeEditor.faq.entryGuide") }}</p>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 解析引擎 -->
                <div v-if="!isFAQ && formData && currentSection === 'parser'" class="mb-8 last:mb-0">
                  <KBParserSettings
                    :parser-engine-rules="formData.chunkingConfig.parserEngineRules"
                    @update:parser-engine-rules="handleParserEngineRulesUpdate"
                  />
                </div>

                <!-- 存储引擎 -->
                <div v-if="!isFAQ && formData && currentSection === 'storage'" class="mb-8 last:mb-0">
                  <KBStorageSettings
                    :storage-backend-id="formData.storageBackendId"
                    :bound-storage-backend-id="initialStorageBackendId"
                    :has-files="editorMode === 'edit' && hasFiles"
                    @update:storage-backend-id="handleStorageBackendUpdate"
                  />
                </div>

                <!-- 分块设置 -->
                <div v-if="!isFAQ" v-show="currentSection === 'chunking'" class="mb-8 last:mb-0">
                  <KBChunkingSettings
                    v-if="formData"
                    :config="formData.chunkingConfig"
                    @update:config="handleChunkingConfigUpdate"
                  />
                </div>

                <!-- 多模态配置 -->
                <div v-if="!isFAQ" v-show="currentSection === 'multimodal'" class="mb-8 last:mb-0">
                  <div v-if="formData" class="w-full">
                    <div class="mb-5">
                      <h2 class="text-foreground m-0 mb-1.5 text-xl font-semibold">
                        {{ $t("knowledgeEditor.multimodal.title") }}
                      </h2>
                      <p class="text-muted-foreground m-0 text-sm leading-normal">
                        {{ $t("knowledgeEditor.multimodal.description") }}
                      </p>
                    </div>

                    <div class="flex flex-col">
                      <!-- 多模态开关 -->
                      <div
                        class="border-border flex items-start justify-between py-4 [&:not(:last-child)]:border-b"
                        data-guide="kb-create-multimodal-toggle"
                      >
                        <div class="max-w-[65%] flex-1 pr-6">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            $t("knowledgeEditor.advanced.multimodal.label")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ $t("knowledgeEditor.advanced.multimodal.description") }}
                          </p>
                        </div>
                        <div class="flex shrink-0 items-center justify-end" style="min-width: 280px">
                          <Switch
                            :model-value="formData.multimodalConfig.enabled"
                            @update:model-value="
                              (val: boolean) => {
                                formData.multimodalConfig.enabled = val;
                                handleMultimodalToggle();
                              }
                            "
                          />
                        </div>
                      </div>

                      <!-- VLLM 模型选择（多模态启用时） -->
                      <div
                        v-if="formData.multimodalConfig.enabled"
                        class="border-border flex items-start justify-between py-4 [&:not(:last-child)]:border-b"
                        data-guide="kb-create-multimodal-vllm"
                      >
                        <div class="max-w-[65%] flex-1 pr-6">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">
                            {{ $t("knowledgeEditor.advanced.multimodal.vllmLabel") }}
                            <span class="text-destructive ml-0.5 font-medium">*</span>
                          </label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ $t("knowledgeEditor.advanced.multimodal.vllmDescription") }}
                          </p>
                        </div>
                        <div class="flex shrink-0 items-center justify-end" style="min-width: 280px">
                          <ModelSelector
                            model-type="VLLM"
                            :selected-model-id="formData.multimodalConfig.vllmModelId"
                            :all-models="allModels"
                            @update:selected-model-id="handleMultimodalVLLMChange"
                            @add-model="handleAddVLLMModel"
                            :placeholder="$t('knowledgeEditor.advanced.multimodal.vllmPlaceholder')"
                          />
                        </div>
                      </div>

                      <div
                        v-if="formData.multimodalConfig.enabled"
                        class="border-border flex items-start justify-between py-4 [&:not(:last-child)]:border-b"
                      >
                        <div class="max-w-[65%] flex-1 pr-6">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            $t("knowledgeEditor.advanced.multimodal.descriptionLanguageLabel")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ $t("knowledgeEditor.advanced.multimodal.descriptionLanguageDescription") }}
                          </p>
                        </div>
                        <div class="flex shrink-0 items-center justify-end" style="min-width: 280px">
                          <!-- The old select was clearable back to "follow the document". Reka
                               forbids an empty item value, so that choice is an explicit item
                               with a sentinel value, mapped back to the empty string. -->
                          <Select
                            :model-value="formData.multimodalConfig.descriptionLanguage || undefined"
                            @update:model-value="
                              (val) =>
                                (formData.multimodalConfig.descriptionLanguage =
                                  val === AUTO_DESCRIPTION_LANGUAGE ? '' : String(val ?? ''))
                            "
                          >
                            <SelectTrigger class="w-[280px]">
                              <SelectValue
                                :placeholder="$t('knowledgeEditor.advanced.multimodal.descriptionLanguageAuto')"
                              />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem :value="AUTO_DESCRIPTION_LANGUAGE">{{
                                $t("knowledgeEditor.advanced.multimodal.descriptionLanguageAuto")
                              }}</SelectItem>
                              <SelectItem value="Chinese">{{ $t("language.zhCN") }}</SelectItem>
                              <SelectItem value="English">{{ $t("language.enUS") }}</SelectItem>
                              <SelectItem value="Korean">{{ $t("language.koKR") }}</SelectItem>
                              <SelectItem value="Russian">{{ $t("language.ruRU") }}</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                      </div>

                      <div
                        v-if="formData.multimodalConfig.enabled"
                        class="border-border flex items-start justify-between py-4 [&:not(:last-child)]:border-b"
                      >
                        <div class="max-w-[65%] flex-1 pr-6">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            $t("knowledgeEditor.advanced.multimodal.customInstructionsLabel")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ $t("knowledgeEditor.advanced.multimodal.customInstructionsDescription") }}
                          </p>
                        </div>
                        <div class="flex shrink-0 items-center justify-end" style="min-width: 280px">
                          <Textarea
                            v-model="formData.multimodalConfig.customInstructions"
                            class="max-h-[186px] min-h-[76px]"
                            :placeholder="$t('knowledgeEditor.advanced.multimodal.customInstructionsPlaceholder')"
                            :maxlength="4000"
                          />
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 音频处理（ASR）设置 -->
                <div v-if="!isFAQ" v-show="currentSection === 'asr'" class="mb-8 last:mb-0">
                  <div v-if="formData" class="w-full">
                    <div class="mb-5">
                      <h2 class="text-foreground m-0 mb-1.5 text-xl font-semibold">
                        {{ $t("knowledgeEditor.asr.title") }}
                      </h2>
                      <p class="text-muted-foreground m-0 text-sm leading-normal">
                        {{ $t("knowledgeEditor.asr.description") }}
                      </p>
                    </div>

                    <div class="flex flex-col">
                      <!-- ASR 开关 -->
                      <div class="border-border flex items-start justify-between py-4 [&:not(:last-child)]:border-b">
                        <div class="max-w-[65%] flex-1 pr-6">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            $t("knowledgeEditor.asr.label")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ $t("knowledgeEditor.asr.desc") }}
                          </p>
                        </div>
                        <div class="flex shrink-0 items-center justify-end" style="min-width: 280px">
                          <Switch
                            :model-value="formData.asrConfig.enabled"
                            @update:model-value="(val: boolean) => (formData.asrConfig.enabled = val)"
                          />
                        </div>
                      </div>

                      <!-- ASR 模型选择 -->
                      <div
                        v-if="formData.asrConfig.enabled"
                        class="border-border flex items-start justify-between py-4 [&:not(:last-child)]:border-b"
                      >
                        <div class="max-w-[65%] flex-1 pr-6">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">
                            {{ $t("knowledgeEditor.asr.modelLabel") }}
                            <span class="text-destructive ml-0.5 font-medium">*</span>
                          </label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ $t("knowledgeEditor.asr.modelDescription") }}
                          </p>
                        </div>
                        <div class="flex shrink-0 items-center justify-end" style="min-width: 280px">
                          <ModelSelector
                            model-type="ASR"
                            :selected-model-id="formData.asrConfig.modelId"
                            :all-models="allModels"
                            @update:selected-model-id="
                              (val: string) => {
                                if (formData) formData.asrConfig.modelId = val;
                              }
                            "
                            @add-model="handleAddASRModel"
                            :placeholder="$t('knowledgeEditor.asr.modelPlaceholder')"
                          />
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 知识图谱 -->
                <div v-if="!isFAQ && currentSection === 'graph'" class="mb-8 last:mb-0">
                  <GraphSettings
                    v-if="formData"
                    :graph-extract="formData.nodeExtractConfig"
                    :model-id="formData.modelConfig.llmModelId"
                    :all-models="allModels"
                    @update:graphExtract="handleNodeExtractUpdate"
                  />
                </div>

                <!-- 高级设置 -->
                <div v-if="!isFAQ" v-show="currentSection === 'advanced'" class="mb-8 last:mb-0">
                  <KBAdvancedSettings
                    ref="advancedSettingsRef"
                    v-if="formData"
                    :question-generation="formData.questionGenerationConfig"
                    :auto-tag="formData.autoTagConfig"
                    :rag-enabled="formData.indexingStrategy?.vectorEnabled || formData.indexingStrategy?.keywordEnabled"
                    :all-models="allModels"
                    :table-metadata-instructions="formData.chunkingConfig.tableMetadataInstructions"
                    @update:question-generation="handleQuestionGenerationUpdate"
                    @update:auto-tag="
                      (value) => {
                        if (formData) formData.autoTagConfig = value;
                      }
                    "
                    @update:table-metadata-instructions="
                      (value: string) => {
                        if (formData) formData.chunkingConfig.tableMetadataInstructions = value;
                      }
                    "
                  />
                </div>

                <!-- 数据源管理（仅编辑模式） -->
                <div
                  v-if="editorMode === 'edit' && activeKbId && currentSection === 'datasource'"
                  class="mb-8 last:mb-0"
                >
                  <DataSourceSettings :kb-id="activeKbId" @count="dsCount = $event" />
                </div>

                <!-- 共享设置（仅编辑模式） -->
                <div v-if="editorMode === 'edit' && activeKbId && currentSection === 'share'" class="mb-8 last:mb-0">
                  <KBShareSettings :kb-id="activeKbId" :can-share="canShareKB" />
                </div>

                <!-- 活动记录（仅编辑模式，KB 所属租户内 Owner/Admin） -->
                <div
                  v-if="editorMode === 'edit' && activeKbId && canViewActivity && currentSection === 'activity'"
                  class="mb-8 last:mb-0"
                >
                  <KnowledgeBaseActivitySettings :kb-id="activeKbId" :active="currentSection === 'activity'" />
                </div>
              </div>

              <!-- 保存按钮 -->
              <div class="border-border flex shrink-0 items-center justify-end gap-4 border-t px-10 py-3">
                <p
                  v-if="isPostCreateSession"
                  class="text-muted-foreground m-0 mr-auto flex min-w-0 flex-1 items-start gap-1.5 text-[13px] leading-5"
                >
                  <CircleCheckIcon class="text-success mt-0.5 size-3.5 shrink-0" />
                  <span>
                    <strong class="text-foreground mr-1 font-medium">{{
                      $t("knowledgeEditor.postCreateHint.title")
                    }}</strong>
                    {{ $t("knowledgeEditor.postCreateHint.footer") }}
                  </span>
                </p>
                <div class="flex shrink-0 gap-3">
                  <Button variant="outline" @click="handleClose">
                    {{ $t("common.cancel") }}
                  </Button>
                  <Button data-guide="kb-create-submit" :disabled="saving" @click="handleSubmit">
                    <Loader2Icon v-if="saving" class="animate-spin" />
                    {{ saveButtonLabel }}
                  </Button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>

  <KbCreateContextualGuide
    :when="visible && editorMode === 'create'"
    :is-faq="isFAQ"
    :needs-embedding="kbCreateNeedsEmbedding"
  />
</template>

<script setup lang="ts">
import { DEFAULT_CHUNK_OVERLAP, DEFAULT_CHUNK_SIZE, chunkOverlapOrDefault } from "@/config/chunking";
import { ref, computed, watch, onMounted, onBeforeUnmount } from "vue";
import KbCreateContextualGuide from "@/components/KbCreateContextualGuide.vue";
import { KB_EDITOR_FOCUS_SECTION_EVENT, markContextualGuideDone } from "@/config/contextualGuides";
import { MessagePlugin, DialogPlugin } from "tdesign-vue-next";
import {
  CircleCheckIcon,
  CircleHelpIcon,
  CloudDownloadIcon,
  CloudIcon,
  CopyIcon,
  DatabaseIcon,
  FileSearchIcon,
  HistoryIcon,
  ImageIcon,
  InfoIcon,
  Loader2Icon,
  NetworkIcon,
  SettingsIcon,
  Share2Icon,
  SlidersHorizontalIcon,
  AudioLinesIcon,
  type LucideIcon,
} from "@lucide/vue";
import {
  createKnowledgeBase,
  getKnowledgeBaseById,
  listKnowledgeFiles,
  updateKnowledgeBase,
  rebuildKBIndex,
} from "@/api/knowledge-base";
import { updateKBConfig, type KBModelConfigRequest } from "@/api/initialization";
import { useChatResourcesStore } from "@/stores/chatResources";
import { selectInitialModelId } from "@/utils/modelDefaults";
import { copyWithToast } from "@/utils/clipboard";
import { useUIStore } from "@/stores/ui";
import { useAuthStore } from "@/stores/auth";
import KBModelConfig from "./settings/KBModelConfig.vue";
import KBParserSettings from "./settings/KBParserSettings.vue";
import KBStorageSettings from "./settings/KBStorageSettings.vue";
import KBChunkingSettings from "./settings/KBChunkingSettings.vue";
import KBVectorStoreSettings from "./settings/KBVectorStoreSettings.vue";
import KBAdvancedSettings from "./settings/KBAdvancedSettings.vue";
import ModelSelector from "@/components/ModelSelector.vue";
import GraphSettings from "./settings/GraphSettings.vue";
import KBShareSettings from "./settings/KBShareSettings.vue";
import DataSourceSettings from "./settings/DataSourceSettings.vue";
import KnowledgeBaseActivitySettings from "./settings/KnowledgeBaseActivitySettings.vue";
import { useI18n } from "vue-i18n";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import SegmentedRadio from "./components/SegmentedRadio.vue";

const uiStore = useUIStore();
const authStore = useAuthStore();
const chatResources = useChatResourcesStore();
const { t } = useI18n();

// Props
const props = defineProps<{
  visible: boolean;
  mode: "create" | "edit";
  kbId?: string;
  initialType?: "document" | "faq";
}>();

// Emits
const emit = defineEmits<{
  (e: "update:visible", value: boolean): void;
  (e: "success", kbId: string): void;
}>();

/** 首次保存创建成功后留在弹窗内，继续配置共享等设置 */
const savedKbId = ref<string | null>(null);
const editorMode = computed(() => (savedKbId.value ? "edit" : props.mode));
const activeKbId = computed(() => savedKbId.value ?? props.kbId);
const isPostCreateSession = computed(() => !!savedKbId.value);
const saveButtonLabel = computed(() =>
  editorMode.value === "create" ? t("knowledgeEditor.buttons.create") : t("knowledgeEditor.buttons.saveAndClose"),
);

// The description-language select's "follow the document" choice. Reka's
// SelectItem cannot carry an empty value, so this stands in for "" in the UI.
const AUTO_DESCRIPTION_LANGUAGE = "__auto__";

const copyKbId = async () => {
  await copyWithToast(activeKbId.value, "common.copied");
};

const currentSection = ref<string>("basic");

const onKbEditorFocusSection = (event: Event) => {
  const section = (event as CustomEvent<{ section?: string }>).detail?.section;
  if (section) {
    currentSection.value = section;
  }
};

onMounted(() => {
  window.addEventListener(KB_EDITOR_FOCUS_SECTION_EVENT, onKbEditorFocusSection);
});

onBeforeUnmount(() => {
  window.removeEventListener(KB_EDITOR_FOCUS_SECTION_EVENT, onKbEditorFocusSection);
});
const saving = ref(false);
const loading = ref(false);
const allModels = ref<any[]>([]);
const hasFiles = ref(false);
/** The storage binding the knowledge base was loaded with, to show what a rebind means. */
const initialStorageBackendId = ref("");
const initialIndexingStrategy = ref<any>(null);
const dsCount = ref(0);
// Identifier of the user who created this KB. Empty for a KB created
// through an API key; such a KB has no "owner" and only tenant Admin+ can
// mutate its share settings.
const kbCreatorId = ref<string>("");
const kbTenantId = ref<number>(0);

// Backend gate for /knowledge-bases/:id/shares (POST/PUT/DELETE) is
// g.OwnedKBOrAdmin(): only the KB creator or tenant Admin+ may mutate
// shares. Org-admins on a shared KB do NOT pass this guard, so they
// would only see 403s if we let them try. Mirror the matrix here so
// the buttons disappear instead of failing.
const canShareKB = computed(() => {
  if (!activeKbId.value) return false;
  const userId = authStore.user?.id || "";
  if (kbCreatorId.value && userId && kbCreatorId.value === userId) return true;
  return authStore.hasRole("admin");
});

const isKbOwner = computed(() => {
  const userId = authStore.user?.id || "";
  return Boolean(kbCreatorId.value && userId && kbCreatorId.value === userId);
});

const canViewActivity = computed(() => {
  if (editorMode.value !== "edit" || !activeKbId.value) return false;
  if (Number(kbTenantId.value || 0) !== Number(authStore.currentTenantId || 0)) return false;
  return isKbOwner.value || authStore.hasRole("admin");
});
// 用户是否在分块设置中手动改过任何值。一旦为 true，就不再根据索引策略自动调整默认分块参数。
const chunkingDirty = ref(false);

// 仅 Wiki 索引模式下的分块预设：更大 chunk、无 overlap、关闭父子分块。
// 该预设只在「创建模式」下、且用户尚未手动调整分块参数时生效，避免覆盖既有 KB 的配置。
const WIKI_ONLY_CHUNKING_PRESET = {
  chunkSize: 2048,
  chunkOverlap: 0,
  enableParentChild: false,
} as const;

// Non-Wiki-only fallback. Mirrors chunker.DefaultChunkSize and
// DefaultChunkOverlap on the backend so a freshly created KB uses
// the same numbers whether the editor sets them or the splitter
// falls back to its package defaults.
const DEFAULT_CHUNKING_PRESET = {
  chunkSize: DEFAULT_CHUNK_SIZE,
  chunkOverlap: DEFAULT_CHUNK_OVERLAP,
  enableParentChild: true,
} as const;

const navIcons: Record<string, LucideIcon> = {
  "info-circle": InfoIcon,
  "control-platform": SlidersHorizontalIcon,
  "data-base": DatabaseIcon,
  "help-circle": CircleHelpIcon,
  "file-search": FileSearchIcon,
  image: ImageIcon,
  sound: AudioLinesIcon,
  cloud: CloudIcon,
  "file-copy": CopyIcon,
  "chart-bubble": NetworkIcon,
  setting: SettingsIcon,
  "cloud-download": CloudDownloadIcon,
  share: Share2Icon,
  history: HistoryIcon,
};

function navIcon(name: string): LucideIcon {
  return navIcons[name] ?? InfoIcon;
}

const navItems = computed(() => {
  const items: { key: string; icon: string; label: string; badge?: number }[] = [
    { key: "basic", icon: "info-circle", label: t("knowledgeEditor.sidebar.basic") },
    { key: "models", icon: "control-platform", label: t("knowledgeEditor.sidebar.models") },
    // VectorStore binding section — present in both create and edit
    // modes. Create mode shows a dropdown; edit mode shows the bound
    // store read-only with an immutability hint.
    { key: "vectorStore", icon: "data-base", label: t("knowledgeEditor.sidebar.vectorStore") },
  ];
  if (formData.value?.type === "faq") {
    items.push({ key: "faq", icon: "help-circle", label: t("knowledgeEditor.sidebar.faq") });
  } else {
    items.push(
      { key: "parser", icon: "file-search", label: t("settings.parserEngine") },
      { key: "multimodal", icon: "image", label: t("knowledgeEditor.sidebar.multimodal") },
      { key: "asr", icon: "sound", label: t("knowledgeEditor.sidebar.asr") },
      { key: "storage", icon: "cloud", label: t("knowledgeEditor.sidebar.storage") },
      { key: "chunking", icon: "file-copy", label: t("knowledgeEditor.sidebar.chunking") },
      { key: "graph", icon: "chart-bubble", label: t("knowledgeEditor.sidebar.graph") },
      { key: "advanced", icon: "setting", label: t("knowledgeEditor.sidebar.advanced") },
    );
    if (editorMode.value === "edit" && activeKbId.value) {
      items.push({
        key: "datasource",
        icon: "cloud-download",
        label: t("knowledgeEditor.sidebar.datasource"),
        badge: dsCount.value || undefined,
      });
    }
  }
  if (editorMode.value === "edit" && activeKbId.value) {
    items.push({ key: "share", icon: "share", label: t("knowledgeEditor.sidebar.share") });
  }
  if (canViewActivity.value) {
    items.push({ key: "activity", icon: "history", label: t("knowledgeEditor.sidebar.activity") });
  }
  return items;
});

// 左侧导航分组（与 AgentEditorModal 对齐）
const navGroups = computed(() => {
  const itemMap = new Map(navItems.value.map((item) => [item.key, item]));
  const pickItems = (keys: string[]) => keys.map((key) => itemMap.get(key)).filter(Boolean) as typeof navItems.value;
  return [
    {
      key: "basic",
      label: t("knowledgeEditor.navGroups.basic"),
      items: pickItems(["basic", "models", "vectorStore", "faq"]),
    },
    {
      key: "processing",
      label: t("knowledgeEditor.navGroups.processing"),
      items: pickItems(["parser", "chunking", "multimodal", "asr", "graph", "advanced"]),
    },
    {
      key: "data",
      label: t("knowledgeEditor.navGroups.data"),
      items: pickItems(["storage", "datasource"]),
    },
    {
      key: "integration",
      label: t("knowledgeEditor.navGroups.integration"),
      items: pickItems(["share"]),
    },
    {
      key: "management",
      label: t("knowledgeEditor.navGroups.management"),
      items: pickItems(["activity"]),
    },
  ].filter((group) => group.items.length > 0);
});

// 模型配置引用
const modelConfigRef = ref<InstanceType<typeof KBModelConfig>>();
const advancedSettingsRef = ref<InstanceType<typeof KBAdvancedSettings>>();

// 表单数据
const formData = ref<any>(null);
const isFAQ = computed(() => formData.value?.type === "faq");

const kbCreateNeedsEmbedding = computed(() => {
  if (!formData.value || formData.value.type === "faq") return false;
  const s = formData.value.indexingStrategy;
  return Boolean(s?.vectorEnabled || s?.keywordEnabled);
});

const applyDefaultModelsIfEmpty = () => {
  if (!formData.value || editorMode.value !== "create") return;
  const chatModelId = selectInitialModelId(allModels.value, "KnowledgeQA");
  const embeddingModelId = selectInitialModelId(allModels.value, "Embedding");
  if (!formData.value.modelConfig.llmModelId && chatModelId) {
    formData.value.modelConfig.llmModelId = chatModelId;
  }
  if (!formData.value.modelConfig.embeddingModelId && embeddingModelId) {
    formData.value.modelConfig.embeddingModelId = embeddingModelId;
  }
};

watch(
  () => formData.value?.type,
  (newType, oldType) => {
    if (!formData.value) return;
    if (newType === "faq") {
      if (!formData.value.faqConfig) {
        formData.value.faqConfig = { indexMode: "question_only", questionIndexMode: "separate" };
      }
      if (!["basic", "models", "faq"].includes(currentSection.value)) {
        currentSection.value = "faq";
      }
    } else if (oldType === "faq" && currentSection.value === "faq") {
      currentSection.value = "basic";
    }
  },
);

// 初始化表单数据
const initFormData = (type: "document" | "faq" = "document") => {
  return {
    type,
    name: "",
    description: "",
    faqConfig: {
      indexMode: "question_only",
      questionIndexMode: "separate",
    },
    modelConfig: {
      llmModelId: "",
      embeddingModelId: "",
      wikiSynthesisModelId: "",
    },
    chunkingConfig: {
      chunkSize: DEFAULT_CHUNK_SIZE,
      chunkOverlap: DEFAULT_CHUNK_OVERLAP,
      separators: ["\n\n", "\n", "。", "！", "？", ";", "；"],
      parserEngineRules: undefined as any,
      enableParentChild: true,
      parentChunkSize: 4096,
      childChunkSize: 384,
      // New KBs default to the adaptive auto-strategy. User can change in the UI.
      strategy: "auto" as string,
      tokenLimit: 0,
      languages: [] as string[],
      tableMetadataInstructions: "",
    },
    // Empty means "the workspace default": the server binds a new knowledge
    // base to it, and the storage panel preselects it once it has loaded.
    storageBackendId: "" as string,
    multimodalConfig: {
      enabled: false,
      vllmModelId: "",
      descriptionLanguage: "",
      customInstructions: "",
    },
    asrConfig: {
      enabled: false,
      modelId: "",
      language: "",
    },
    nodeExtractConfig: {
      enabled: false,
      text: "",
      tags: [] as string[],
      nodes: [] as Array<{
        name: string;
        attributes: string[];
      }>,
      relations: [] as Array<{
        node1: string;
        node2: string;
        type: string;
      }>,
      customInstructions: "",
    },
    questionGenerationConfig: {
      enabled: true,
      questionCount: 3,
      customInstructions: "",
    },
    autoTagConfig: {
      enabled: false,
      modelId: "",
      maxTags: 3,
      skipIfTagged: true,
    },
    reviewIntervalDays: 0,
    wikiConfig: {
      synthesisModelId: "",
      maxPagesPerIngest: 0,
      extractionGranularity: "standard" as "focused" | "standard" | "exhaustive",
      contentInstructions: "",
      extractionInstructions: "",
    },
    indexingStrategy: {
      vectorEnabled: true,
      keywordEnabled: true,
      wikiEnabled: false,
      graphEnabled: false,
    },
    // Vector-store binding. Empty string means "use the env-configured
    // store"; create mode defaults to that, edit mode loads the
    // existing binding from the KB response below.
    vectorStoreId: "" as string,
    vectorStoreInfo: {
      source: undefined as string | undefined,
      name: undefined as string | undefined,
      engineType: undefined as string | undefined,
      status: undefined as string | undefined,
    },
  };
};

// 加载所有模型
const loadAllModels = async (force = false) => {
  try {
    await chatResources.ensureModels(force);
    allModels.value = chatResources.allModels || [];
  } catch (error) {
    console.error("Failed to load model list:", error);
    MessagePlugin.error(t("knowledgeEditor.messages.loadModelsFailed"));
    allModels.value = [];
  }
};

// 加载知识库数据（编辑模式）
const loadKBData = async (kbIdOverride?: string) => {
  const kbId = kbIdOverride ?? activeKbId.value;
  if (editorMode.value !== "edit" || !kbId) return;

  loading.value = true;
  try {
    const [kbInfo, filesResult] = await Promise.all([
      getKnowledgeBaseById(kbId),
      listKnowledgeFiles(kbId, { page: 1, page_size: 1 }),
    ]);

    if (!kbInfo || !kbInfo.data) {
      throw new Error(t("knowledgeEditor.messages.notFound"));
    }

    const kb = kbInfo.data;
    hasFiles.value = (filesResult as any)?.total > 0;
    kbCreatorId.value = (kb as any).creator_id || "";
    kbTenantId.value = Number((kb as any).tenant_id || 0);

    // 设置表单数据
    const kbType = (kb.type as "document" | "faq") || "document";
    formData.value = {
      type: kbType,
      name: kb.name || "",
      description: kb.description || "",
      faqConfig: {
        indexMode: kb.faq_config?.index_mode || "question_only",
        questionIndexMode: kb.faq_config?.question_index_mode || "separate",
      },
      modelConfig: {
        llmModelId: kb.summary_model_id || "",
        embeddingModelId: kb.embedding_model_id || "",
        wikiSynthesisModelId: kb.wiki_config?.synthesis_model_id || "",
      },
      chunkingConfig: {
        chunkSize: kb.chunking_config?.chunk_size || DEFAULT_CHUNK_SIZE,
        // Absent means "never chosen"; 0 is a real setting (no overlap).
        chunkOverlap: chunkOverlapOrDefault(kb.chunking_config?.chunk_overlap),
        separators: kb.chunking_config?.separators || ["\n\n", "\n", "。", "！", "？", ";", "；"],
        parserEngineRules: kb.chunking_config?.parser_engine_rules || undefined,
        enableParentChild: kb.chunking_config?.enable_parent_child || false,
        parentChunkSize: kb.chunking_config?.parent_chunk_size || 4096,
        childChunkSize: kb.chunking_config?.child_chunk_size || 384,
        // An unset strategy renders as empty, which the backend treats as the
        // classic recursive splitter; the user picks a tier to opt in.
        strategy: kb.chunking_config?.strategy || "",
        tokenLimit: kb.chunking_config?.token_limit || 0,
        languages: kb.chunking_config?.languages || [],
        tableMetadataInstructions: kb.chunking_config?.table_metadata_instructions || "",
      },
      storageBackendId: (kb.storage_backend_id || "") as string,
      multimodalConfig: {
        enabled: !!kb.vlm_config?.enabled,
        vllmModelId: kb.vlm_config?.model_id || "",
        descriptionLanguage: kb.vlm_config?.description_language || "",
        customInstructions: kb.vlm_config?.custom_instructions || "",
      },
      asrConfig: {
        enabled: !!kb.asr_config?.enabled,
        modelId: kb.asr_config?.model_id || "",
        language: kb.asr_config?.language || "",
      },
      nodeExtractConfig: {
        enabled: kb.extract_config?.enabled || false,
        text: kb.extract_config?.text || "",
        tags: kb.extract_config?.tags || [],
        nodes: (kb.extract_config?.nodes || []).map((node: any) => ({
          name: node.name,
          attributes: node.attributes || [],
        })),
        relations: kb.extract_config?.relations || [],
        customInstructions: kb.extract_config?.custom_instructions || "",
      },
      questionGenerationConfig: {
        enabled: kb.question_generation_config?.enabled || false,
        questionCount: kb.question_generation_config?.question_count || 3,
        customInstructions: kb.question_generation_config?.custom_instructions || "",
      },
      autoTagConfig: {
        enabled: kb.auto_tag_config?.enabled || false,
        modelId: kb.auto_tag_config?.model_id || "",
        maxTags: kb.auto_tag_config?.max_tags || 3,
        // Absent on knowledge bases saved before the toggle existed; the
        // backend treats that as "skip", so mirror it here.
        skipIfTagged: kb.auto_tag_config?.skip_if_tagged ?? true,
      },
      reviewIntervalDays: kb.review_interval_days || 0,
      wikiConfig: {
        synthesisModelId: kb.wiki_config?.synthesis_model_id || "",
        maxPagesPerIngest: kb.wiki_config?.max_pages_per_ingest || 0,
        extractionGranularity: (kb.wiki_config?.extraction_granularity === "focused" ||
        kb.wiki_config?.extraction_granularity === "exhaustive"
          ? kb.wiki_config.extraction_granularity
          : "standard") as "focused" | "standard" | "exhaustive",
        contentInstructions: kb.wiki_config?.content_instructions || "",
        extractionInstructions: kb.wiki_config?.extraction_instructions || "",
      },
      indexingStrategy: {
        vectorEnabled: kb.indexing_strategy?.vector_enabled ?? true,
        keywordEnabled: kb.indexing_strategy?.keyword_enabled ?? true,
        wikiEnabled: kb.indexing_strategy?.wiki_enabled ?? false,
        graphEnabled: kb.indexing_strategy?.graph_enabled ?? false,
      },
      // Vector-store binding. vectorStoreId is editor-only state; it
      // is only included in the create request, never the update
      // request, because the binding is immutable after creation.
      // vectorStoreInfo carries the read-only display fields that the
      // edit view renders below; they come straight from the KB
      // response.
      vectorStoreId: "",
      vectorStoreInfo: {
        source: kb.vector_store_source,
        name: kb.vector_store_name,
        engineType: kb.vector_store_engine_type,
        status: kb.vector_store_status,
      },
    };
    initialStorageBackendId.value = formData.value.storageBackendId;
    initialIndexingStrategy.value = { ...formData.value.indexingStrategy };
  } catch (error) {
    console.error("Failed to load knowledge base data:", error);
    MessagePlugin.error(t("knowledgeEditor.messages.loadDataFailed"));
    handleClose();
  } finally {
    loading.value = false;
  }
};

// 处理配置更新
const handleModelConfigUpdate = (config: any) => {
  if (formData.value) {
    formData.value.modelConfig = { ...config };
  }
};

// 粒度选择器：从 formData.wikiConfig 读出并规范化，未知值回退到 'standard'，
// 与后端 WikiExtractionGranularity.Normalize() 的契约保持一致。
const resolvedGranularity = computed<"focused" | "standard" | "exhaustive">(() => {
  const g = formData.value?.wikiConfig?.extractionGranularity;
  if (g === "focused" || g === "standard" || g === "exhaustive") {
    return g;
  }
  return "standard";
});

const granularityHint = computed<string>(() => {
  switch (resolvedGranularity.value) {
    case "focused":
      return t("knowledgeEditor.wiki.granularityFocusedHint");
    case "exhaustive":
      return t("knowledgeEditor.wiki.granularityExhaustiveHint");
    default:
      return t("knowledgeEditor.wiki.granularityStandardHint");
  }
});

const handleGranularityChange = (value: unknown) => {
  if (!formData.value) return;
  const next: "focused" | "standard" | "exhaustive" =
    value === "focused" || value === "exhaustive" ? value : "standard";
  formData.value.wikiConfig = {
    ...formData.value.wikiConfig,
    extractionGranularity: next,
  };
};

const isIndexingLocked = computed(() => editorMode.value === "edit" && hasFiles.value);

const toggleVectorIndexing = () => {
  if (!formData.value) return;
  if (isIndexingLocked.value) return;
  const next = !formData.value.indexingStrategy.vectorEnabled;
  formData.value.indexingStrategy.vectorEnabled = next;
  formData.value.indexingStrategy.keywordEnabled = next;
};

const toggleWikiIndexing = () => {
  if (!formData.value) return;
  if (isIndexingLocked.value) return;
  formData.value.indexingStrategy.wikiEnabled = !formData.value.indexingStrategy.wikiEnabled;
};

const handleChunkingConfigUpdate = (config: any) => {
  if (formData.value) {
    formData.value.chunkingConfig = { ...config };
    // 用户已经手动触达分块设置，后续索引策略切换不再覆盖这些值
    chunkingDirty.value = true;
  }
};

// 判断当前是否为「仅 Wiki 索引」：只开了 Wiki，关了向量/关键词检索
const isWikiOnlyStrategy = computed(() => {
  const s = formData.value?.indexingStrategy;
  if (!s) return false;
  return !!s.wikiEnabled && !s.vectorEnabled && !s.keywordEnabled;
});

// 仅在创建模式、用户未改过分块设置时，随索引策略自动应用/撤销 Wiki-only 预设。
// 编辑模式严格保持后端已有配置不变，避免误改。
watch(isWikiOnlyStrategy, (wikiOnly) => {
  if (editorMode.value !== "create") return;
  if (!formData.value) return;
  if (chunkingDirty.value) return;
  const preset = wikiOnly ? WIKI_ONLY_CHUNKING_PRESET : DEFAULT_CHUNKING_PRESET;
  formData.value.chunkingConfig = {
    ...formData.value.chunkingConfig,
    ...preset,
  };
});

const handleParserEngineRulesUpdate = (rules: any[]) => {
  if (formData.value) {
    formData.value.chunkingConfig.parserEngineRules = rules?.length ? rules : undefined;
  }
};

const handleMultimodalToggle = () => {
  if (formData.value && !formData.value.multimodalConfig.enabled) {
    formData.value.multimodalConfig.vllmModelId = "";
  }
};

const handleMultimodalVLLMChange = (modelId: string) => {
  if (formData.value) {
    formData.value.multimodalConfig.vllmModelId = modelId;
  }
};

const handleAddVLLMModel = () => {
  uiStore.openSettings("models", "vllm");
};

const handleAddASRModel = () => {
  uiStore.openSettings("models", "asr");
};

const handleStorageBackendUpdate = (value: string) => {
  if (formData.value) {
    formData.value.storageBackendId = value;
  }
};

const handleVectorStoreIdUpdate = (id: string) => {
  if (formData.value) {
    // Empty string here means "use system default" (env-store fallback).
    // The create-payload assembly below converts this back to `omit` so
    // the backend stores NULL — keeping the wire shape identical to
    // pre-Phase-2 clients.
    formData.value.vectorStoreId = id || "";
  }
};

const handleQuestionGenerationUpdate = (config: any) => {
  if (formData.value) {
    formData.value.questionGenerationConfig = { ...config };
  }
};

const handleNodeExtractUpdate = (config: any) => {
  if (formData.value) {
    formData.value.nodeExtractConfig = { ...config };
  }
};

// 验证表单
const validateForm = (): boolean => {
  if (!formData.value) return false;

  // 验证基本信息
  if (!formData.value.name || !formData.value.name.trim()) {
    MessagePlugin.warning(t("knowledgeEditor.messages.nameRequired"));
    currentSection.value = "basic";
    return false;
  }

  // 验证索引策略 — 文档类型至少需要开启一种
  if (formData.value.type !== "faq") {
    const s = formData.value.indexingStrategy;
    if (s && !s.vectorEnabled && !s.keywordEnabled && !s.wikiEnabled && !s.graphEnabled) {
      MessagePlugin.warning(t("knowledgeEditor.indexing.atLeastOne"));
      currentSection.value = "basic";
      return false;
    }
  }

  // 验证模型配置 - embedding 模型仅在检索索引启用时必须
  const needsEmbedding =
    formData.value.indexingStrategy?.vectorEnabled || formData.value.indexingStrategy?.keywordEnabled;
  if (needsEmbedding && !formData.value.modelConfig.embeddingModelId) {
    MessagePlugin.warning(t("knowledgeEditor.indexing.embeddingRequired"));
    currentSection.value = "models";
    return false;
  }

  if (!formData.value.modelConfig.llmModelId) {
    MessagePlugin.warning(t("knowledgeEditor.messages.summaryRequired"));
    currentSection.value = "models";
    return false;
  }

  // 验证多模态配置（如果启用）
  if (formData.value.multimodalConfig.enabled && !formData.value.multimodalConfig.vllmModelId) {
    MessagePlugin.warning(t("knowledgeEditor.messages.multimodalInvalid"));
    currentSection.value = "multimodal";
    return false;
  }

  if (formData.value.type === "faq" && !formData.value.faqConfig?.indexMode) {
    MessagePlugin.warning(t("knowledgeEditor.messages.indexModeRequired"));
    currentSection.value = "faq";
    return false;
  }

  return true;
};

// 构建提交数据
/** The review period as the server takes it: whole days, 0 (off) to ten
 * years, whatever the number field holds while someone is typing. */
const MAX_REVIEW_INTERVAL_DAYS = 3650;
const normalizeReviewInterval = (value: unknown): number => {
  const days = Math.floor(Number(value));
  return Number.isFinite(days) ? Math.min(MAX_REVIEW_INTERVAL_DAYS, Math.max(0, days)) : 0;
};

const buildSubmitData = () => {
  if (!formData.value) return null;

  const data: any = {
    name: formData.value.name,
    description: formData.value.description,
    type: formData.value.type,
    chunking_config: {
      chunk_size: formData.value.chunkingConfig.chunkSize,
      chunk_overlap: formData.value.chunkingConfig.chunkOverlap,
      separators: formData.value.chunkingConfig.separators,
      enable_parent_child: formData.value.chunkingConfig.enableParentChild,
      parent_chunk_size: formData.value.chunkingConfig.parentChunkSize,
      child_chunk_size: formData.value.chunkingConfig.childChunkSize,
      // Adaptive chunking fields are always sent (empty/zero values
      // included) so the user can clear them — backend uses pointer DTOs
      // to distinguish "not in payload" from "explicitly empty".
      strategy: formData.value.chunkingConfig.strategy ?? "",
      token_limit: formData.value.chunkingConfig.tokenLimit ?? 0,
      languages: formData.value.chunkingConfig.languages ?? [],
      table_metadata_instructions: formData.value.chunkingConfig.tableMetadataInstructions || "",
      ...(formData.value.chunkingConfig.parserEngineRules?.length
        ? { parser_engine_rules: formData.value.chunkingConfig.parserEngineRules }
        : {}),
    },
    embedding_model_id: formData.value.modelConfig.embeddingModelId,
    summary_model_id: formData.value.modelConfig.llmModelId,
  };

  // Vector-store binding. Only attach the field when the user actively
  // selected a non-default store. The server treats an empty string as
  // NULL, but keeping the field absent on the wire matches what a
  // client that doesn't know about this binding would send — which
  // makes A/B response diffs easier to read.
  if (formData.value.vectorStoreId) {
    data.vector_store_id = formData.value.vectorStoreId;
  }

  // 添加多模态配置
  data.vlm_config = {
    enabled: formData.value.multimodalConfig.enabled,
    model_id: formData.value.multimodalConfig.enabled ? formData.value.multimodalConfig.vllmModelId || "" : "",
    description_language: formData.value.multimodalConfig.descriptionLanguage || "",
    custom_instructions: formData.value.multimodalConfig.customInstructions || "",
  };

  // 添加ASR语音识别配置
  data.asr_config = {
    enabled: formData.value.asrConfig?.enabled || false,
    model_id: formData.value.asrConfig?.enabled ? formData.value.asrConfig?.modelId || "" : "",
    language: formData.value.asrConfig?.language || "",
  };

  // The storage binding is a backend id; without one the server binds the
  // workspace default.
  if (formData.value.storageBackendId) {
    data.storage_backend_id = formData.value.storageBackendId;
  }

  // 添加知识图谱配置 — now synced via indexingStrategy.graphEnabled
  // extract_config is sent below along with indexing_strategy

  // 添加问题生成配置
  if (formData.value.questionGenerationConfig?.enabled) {
    data.question_generation_config = {
      enabled: true,
      question_count: formData.value.questionGenerationConfig.questionCount || 3,
      custom_instructions: formData.value.questionGenerationConfig.customInstructions || "",
    };
  } else {
    data.question_generation_config = {
      enabled: false,
      question_count: 3,
      custom_instructions: formData.value.questionGenerationConfig?.customInstructions || "",
    };
  }

  data.review_interval_days = normalizeReviewInterval(formData.value.reviewIntervalDays);

  data.auto_tag_config = {
    enabled: formData.value.autoTagConfig?.enabled || false,
    model_id: formData.value.autoTagConfig?.modelId || "",
    max_tags: formData.value.autoTagConfig?.maxTags || 3,
    skip_if_tagged: formData.value.autoTagConfig?.skipIfTagged ?? true,
  };

  if (formData.value.type === "faq") {
    data.faq_config = {
      index_mode: formData.value.faqConfig?.indexMode || "question_only",
      question_index_mode: formData.value.faqConfig?.questionIndexMode || "separate",
    };
  }

  // Wiki enablement is carried solely by indexing_strategy.wiki_enabled.
  // wiki_config only holds wiki-specific tunables.
  if (formData.value.type !== "faq") {
    data.wiki_config = {
      synthesis_model_id: formData.value.modelConfig?.wikiSynthesisModelId || "",
      max_pages_per_ingest: formData.value.wikiConfig?.maxPagesPerIngest || 0,
      extraction_granularity: formData.value.wikiConfig?.extractionGranularity || "standard",
      content_instructions: formData.value.wikiConfig?.contentInstructions || "",
      extraction_instructions: formData.value.wikiConfig?.extractionInstructions || "",
    };
  }

  // Send indexing strategy
  if (formData.value.type !== "faq") {
    data.indexing_strategy = {
      vector_enabled: formData.value.indexingStrategy?.vectorEnabled ?? true,
      keyword_enabled: formData.value.indexingStrategy?.keywordEnabled ?? true,
      wiki_enabled: formData.value.indexingStrategy?.wikiEnabled ?? false,
      graph_enabled: formData.value.indexingStrategy?.graphEnabled ?? false,
    };
  }

  // Always persist extract_config so the toggle state from GraphSettings is saved,
  // regardless of whether the graph indexing strategy is currently enabled.
  if (formData.value.nodeExtractConfig) {
    data.extract_config = {
      enabled: !!formData.value.nodeExtractConfig.enabled,
      text: formData.value.nodeExtractConfig.text || "",
      tags: formData.value.nodeExtractConfig.tags || [],
      nodes: formData.value.nodeExtractConfig.nodes || [],
      relations: formData.value.nodeExtractConfig.relations || [],
      custom_instructions: formData.value.nodeExtractConfig.customInstructions || "",
    };
  }

  return data;
};

// 提交表单
const handleSubmit = async () => {
  if (!validateForm()) {
    return;
  }

  doSubmit();
};

const doSubmit = async () => {
  saving.value = true;
  try {
    const data = buildSubmitData();
    if (!data) {
      throw new Error(t("knowledgeEditor.messages.buildDataFailed"));
    }

    if (editorMode.value === "create") {
      // 创建模式：一次性创建知识库及所有配置
      const result: any = await createKnowledgeBase(data);
      if (!result.success || !result.data?.id) {
        throw new Error(result.message || t("knowledgeEditor.messages.createFailed"));
      }
      const createdKbId = result.data.id as string;
      savedKbId.value = createdKbId;
      currentSection.value = "basic";
      await loadKBData(createdKbId);
      MessagePlugin.success(t("knowledgeEditor.messages.createSuccess"));
      markContextualGuideDone("kbCreate");
      emit("success", createdKbId);
    } else {
      // 编辑模式：分别更新基本信息和配置
      const kbId = activeKbId.value;
      if (!kbId) {
        throw new Error(t("knowledgeEditor.messages.missingId"));
      }

      // 1. 更新基本信息（名称、描述）和 FAQ/Wiki 配置
      const updateConfig: any = {};
      if (formData.value.type === "faq" && formData.value.faqConfig) {
        updateConfig.faq_config = {
          index_mode: formData.value.faqConfig.indexMode || "question_only",
          question_index_mode: formData.value.faqConfig.questionIndexMode || "separate",
        };
      }
      if (formData.value.wikiConfig && formData.value.type !== "faq") {
        updateConfig.wiki_config = {
          synthesis_model_id: formData.value.modelConfig?.wikiSynthesisModelId || "",
          max_pages_per_ingest: formData.value.wikiConfig.maxPagesPerIngest || 0,
          extraction_granularity: formData.value.wikiConfig.extractionGranularity || "standard",
          content_instructions: formData.value.wikiConfig.contentInstructions || "",
          extraction_instructions: formData.value.wikiConfig.extractionInstructions || "",
        };
      }
      if (formData.value.type !== "faq") {
        updateConfig.auto_tag_config = data.auto_tag_config;
        updateConfig.indexing_strategy = {
          vector_enabled: formData.value.indexingStrategy?.vectorEnabled ?? true,
          keyword_enabled: formData.value.indexingStrategy?.keywordEnabled ?? true,
          wiki_enabled: formData.value.indexingStrategy?.wikiEnabled ?? false,
          graph_enabled: formData.value.indexingStrategy?.graphEnabled ?? false,
        };
      }
      updateConfig.review_interval_days = data.review_interval_days;
      await updateKnowledgeBase(kbId, {
        name: data.name,
        description: data.description,
        config: updateConfig,
      });

      // 2. 更新完整配置（模型、分块、多模态、存储引擎、知识图谱等）
      const config: KBModelConfigRequest = {
        llmModelId: data.summary_model_id,
        embeddingModelId: data.embedding_model_id,
        vlm_config: data.vlm_config,
        asr_config: data.asr_config,
        documentSplitting: {
          chunkSize: data.chunking_config.chunk_size,
          chunkOverlap: data.chunking_config.chunk_overlap,
          separators: data.chunking_config.separators,
          parserEngineRules: data.chunking_config.parser_engine_rules || undefined,
          enableParentChild: data.chunking_config.enable_parent_child || false,
          parentChunkSize: data.chunking_config.parent_chunk_size || 4096,
          childChunkSize: data.chunking_config.child_chunk_size || 384,
          // Always send strategy / tokenLimit / languages — backend treats
          // empty/0/[] as a valid clear, so we must include them in the
          // payload to let users reset back to defaults.
          strategy: formData.value?.chunkingConfig.strategy ?? "",
          tokenLimit: formData.value?.chunkingConfig.tokenLimit ?? 0,
          languages: formData.value?.chunkingConfig.languages ?? [],
          tableMetadataInstructions: formData.value?.chunkingConfig.tableMetadataInstructions ?? "",
        },
        multimodal: {
          enabled: !!data.vlm_config?.enabled,
        },
        storageBackendId: formData.value?.storageBackendId || "",
        nodeExtract: {
          enabled: data.extract_config?.enabled || false,
          text: data.extract_config?.text || "",
          tags: data.extract_config?.tags || [],
          nodes: data.extract_config?.nodes || [],
          relations: data.extract_config?.relations || [],
          customInstructions: data.extract_config?.custom_instructions || "",
        },
        questionGeneration: {
          enabled: data.question_generation_config?.enabled || false,
          questionCount: data.question_generation_config?.question_count || 3,
          customInstructions: data.question_generation_config?.custom_instructions || "",
        },
      };

      await updateKBConfig(kbId, config);
      MessagePlugin.success(t("knowledgeEditor.messages.updateSuccess"));

      // Check if indexing strategy changed and offer rebuild
      if (hasFiles.value && initialIndexingStrategy.value && formData.value) {
        const curr = formData.value.indexingStrategy;
        const prev = initialIndexingStrategy.value;
        const strategyChanged =
          curr.vectorEnabled !== prev.vectorEnabled ||
          curr.keywordEnabled !== prev.keywordEnabled ||
          curr.wikiEnabled !== prev.wikiEnabled ||
          curr.graphEnabled !== prev.graphEnabled;
        if (strategyChanged) {
          const dialog = DialogPlugin.confirm({
            header: t("knowledgeEditor.indexing.rebuildConfirmTitle"),
            body: t("knowledgeEditor.indexing.rebuildConfirmBody", { count: "..." }),
            confirmBtn: t("common.confirm"),
            cancelBtn: t("common.cancel"),
            onConfirm: async () => {
              dialog.destroy();
              try {
                const result: any = await rebuildKBIndex(kbId);
                const count = result?.data?.document_count ?? 0;
                MessagePlugin.success(t("knowledgeEditor.indexing.rebuildSuccess", { count }));
              } catch (e) {
                console.error("Rebuild index failed:", e);
              }
            },
            onCancel: () => {
              dialog.destroy();
              MessagePlugin.info(t("knowledgeEditor.indexing.rebuildSkip"));
            },
          });
        }
      }

      emit("success", kbId);
      handleClose();
    }
  } catch (error: any) {
    console.error("Knowledge base operation failed:", error);
    // Vector-store-binding error codes from the server. Both indicate
    // the selected store cannot be used: 2200 is "the binding itself
    // is invalid" (e.g. unknown id, foreign tenant), 2201 is "the
    // store is currently unreachable". For either, swap in a localized
    // message and jump the user back to the Vector Store section so
    // they can pick a different store or fall back to the system
    // default.
    const code = error?.response?.data?.error?.code ?? error?.code;
    if (code === 2200) {
      MessagePlugin.error(t("knowledgeEditor.errors.vectorStoreBindingInvalid"));
      currentSection.value = "vectorStore";
    } else if (code === 2201) {
      MessagePlugin.error(t("knowledgeEditor.errors.vectorStoreUnavailable"));
      currentSection.value = "vectorStore";
    } else {
      MessagePlugin.error(error?.message || t("common.operationFailed"));
    }
  } finally {
    saving.value = false;
  }
};

// 重置所有状态
const resetState = () => {
  savedKbId.value = null;
  currentSection.value = "basic";
  formData.value = null;
  hasFiles.value = false;
  initialStorageBackendId.value = "";
  initialIndexingStrategy.value = null;
  saving.value = false;
  loading.value = false;
  chunkingDirty.value = false;
  kbCreatorId.value = "";
  kbTenantId.value = 0;
};

// 关闭弹窗
const handleClose = () => {
  emit("update:visible", false);
  setTimeout(() => {
    resetState();
  }, 300);
};

// 监听弹窗打开/关闭
watch(
  () => props.visible,
  async (newVal) => {
    if (newVal) {
      // 打开弹窗时，先重置状态
      resetState();

      // 检查是否有初始 section，如果有则跳转
      if (uiStore.kbEditorInitialSection) {
        currentSection.value = uiStore.kbEditorInitialSection;
      }

      await loadAllModels();

      // 根据模式加载数据
      if (props.mode === "edit" && props.kbId) {
        await loadKBData();
      } else {
        // 创建模式：初始化空表单（存储留空即使用空间默认存储实例）
        formData.value = initFormData(props.initialType || "document");
        hasFiles.value = false;
        applyDefaultModelsIfEmpty();
      }
    } else {
      // 关闭弹窗时，延迟重置状态（等待动画结束）
      setTimeout(() => {
        resetState();
        currentSection.value = "basic"; // 重置为默认 section
      }, 300);
    }
  },
);

// 监听全局设置弹窗关闭后刷新模型列表
watch(
  () => uiStore.showSettingsModal,
  async (visible, previous) => {
    if (!visible && previous && props.visible) {
      await loadAllModels(true);
    }
  },
);
</script>

<!-- Vue <Transition> hooks for the modal fade; the only style block in the file.
     The enter/leave classes are applied by Vue at runtime, so they cannot be utilities. -->
<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: all 0.3s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.modal-enter-from .settings-modal,
.modal-leave-to .settings-modal {
  transform: scale(0.95);
}
</style>
