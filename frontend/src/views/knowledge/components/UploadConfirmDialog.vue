<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="dialogVisible"
        class="fixed inset-0 z-[3000] flex items-center justify-center bg-black/50 backdrop-blur-xs"
      >
        <div
          class="bg-card relative flex h-[85vh] max-h-[750px] w-[92vw] max-w-[1160px] flex-col overflow-hidden rounded-xl shadow-[0_8px_32px_rgba(0,0,0,0.12)]"
          role="dialog"
          :aria-label="dialogTitle"
        >
          <button
            data-slot="close-button"
            class="bg-muted text-muted-foreground hover:text-foreground absolute top-5 right-5 z-10 flex size-8 items-center justify-center rounded-md"
            type="button"
            :aria-label="t('general.close')"
            @click="handleCancel"
          >
            <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
              <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
            </svg>
          </button>

          <div class="flex min-h-0 flex-1 overflow-hidden max-[800px]:flex-col">
            <aside
              class="border-border flex w-[220px] shrink-0 flex-col border-r bg-[var(--td-bg-color-settings-modal,var(--td-bg-color-secondarycontainer))] max-[800px]:max-h-[140px] max-[800px]:w-auto max-[800px]:border-r-0 max-[800px]:border-b"
            >
              <div class="border-border box-border flex min-h-14 shrink-0 flex-col items-stretch gap-2.5 border-b p-3">
                <div class="flex w-full min-w-0 items-center justify-between gap-2">
                  <h2 class="text-foreground m-0 min-w-0 flex-1 pr-0 text-base leading-[1.35] font-semibold">
                    {{ dialogTitle }}
                  </h2>
                  <div v-if="mode === 'file'" class="flex shrink-0 items-center gap-1.5">
                    <span
                      class="text-muted-foreground box-border h-5 min-w-5 shrink-0 rounded-[10px] bg-[var(--td-bg-color-component)] px-1.5 text-center text-[11px] leading-5 font-semibold"
                    >
                      {{ batchItemCount }}
                    </span>
                    <KbUploadSourceDropdown
                      :accept-file-types="acceptFileTypes"
                      :supported-file-types="supportedFileTypes"
                      :tooltip="t('uploadConfirm.continueAdd')"
                      placement="bottom-left"
                      @files="appendFiles"
                      @url="appendUrl"
                    />
                  </div>
                </div>

                <div v-if="mode === 'file'" class="shrink-0">
                  <Popover v-model:open="destinationPickerVisible">
                    <PopoverTrigger as-child>
                      <button
                        type="button"
                        data-slot="destination-crumb"
                        class="group text-muted-foreground hover:text-primary inline-flex max-w-full min-w-0 items-center gap-1 [font-family:var(--app-font-family)] text-xs leading-[18px] transition-colors"
                        :title="destinationFullLabel"
                        :aria-label="t('uploadConfirm.destinationChange')"
                        :aria-expanded="destinationPickerVisible"
                      >
                        <span class="shrink-0">{{ t("uploadConfirm.destinationLabel") }}</span>
                        <span class="text-foreground group-hover:text-primary truncate font-medium">{{
                          destinationBreadcrumb
                        }}</span>
                        <ChevronDownIcon class="text-placeholder group-hover:text-primary size-3 shrink-0" />
                      </button>
                    </PopoverTrigger>
                    <PopoverContent
                      align="start"
                      :side-offset="6"
                      class="w-auto min-w-[208px] gap-0 rounded-[10px] p-1"
                    >
                      <div @click.stop>
                        <FolderPickerMenu
                          :options="pickerFolderOptions"
                          :current-path="localTargetFolder"
                          allow-reselect
                          @create="onDestinationCreated"
                          @confirm="onDestinationPicked"
                        />
                      </div>
                    </PopoverContent>
                  </Popover>
                </div>
              </div>

              <div class="flex min-h-0 flex-1 flex-col overflow-hidden px-2 pt-1.5 pb-3">
                <div v-if="mode === 'manual' && manualPreview" class="min-h-0 flex-1 overflow-y-auto p-0">
                  <p
                    class="bg-accent text-foreground m-0 rounded-md px-2.5 py-2 text-[13px] leading-[1.4] font-medium break-words"
                    :title="manualPreview.title"
                  >
                    {{ manualPreview.title }}
                  </p>
                  <p class="text-placeholder mx-0.5 mt-1.5 mb-0 text-[11px]">
                    {{ t("uploadConfirm.manualCharCount", { count: manualCharCount }) }}
                  </p>
                </div>
                <div v-else-if="mode === 'reparse' && reparsePreview" class="min-h-0 flex-1 overflow-y-auto p-0">
                  <p
                    class="bg-accent text-foreground m-0 rounded-md px-2.5 py-2 text-[13px] leading-[1.4] font-medium break-words"
                    :title="reparsePreview.fileName"
                  >
                    {{ reparsePreview.fileName || t("uploadConfirm.reparseSource") }}
                  </p>
                  <p class="text-placeholder mx-0.5 mt-1.5 mb-0 text-[11px]">{{ t("uploadConfirm.reparseHint") }}</p>
                </div>
                <ul
                  v-else-if="mode === 'file' && batchItemCount > 0"
                  class="m-0 min-h-0 flex-1 list-none overflow-y-auto p-0"
                >
                  <li
                    v-for="(url, index) in localUrls"
                    :key="`url-${url}-${index}`"
                    class="group hover:bg-accent mb-0.5 flex items-center gap-2 rounded-md py-1.5 pr-1.5 pl-2 transition-colors last:mb-0"
                  >
                    <span class="flex h-6 w-6 shrink-0 items-center justify-center">
                      <LinkIcon class="text-muted-foreground group-hover:text-primary size-4" />
                    </span>
                    <div class="min-w-0 flex-1">
                      <span class="text-foreground block truncate text-xs leading-[1.35] font-medium" :title="url">{{
                        url
                      }}</span>
                      <span class="text-placeholder mt-px block truncate text-[11px] leading-[1.3]">
                        {{ t("uploadConfirm.urlItemLabel") }}
                      </span>
                    </div>
                    <button
                      type="button"
                      data-slot="file-remove"
                      class="text-placeholder hover:text-foreground flex size-[22px] shrink-0 items-center justify-center rounded opacity-45 transition-all group-hover:opacity-100 hover:bg-[var(--td-bg-color-component)] focus-visible:opacity-100"
                      :aria-label="t('common.remove')"
                      @click="removeUrl(index)"
                    >
                      <XIcon class="size-3.5" />
                    </button>
                  </li>
                  <li
                    v-for="(file, index) in localFiles"
                    :key="`${file.name}-${index}`"
                    class="group hover:bg-accent mb-0.5 flex items-center gap-2 rounded-md py-1.5 pr-1.5 pl-2 transition-colors last:mb-0"
                  >
                    <span class="flex h-6 w-6 shrink-0 items-center justify-center">
                      <component
                        :is="fileTypeIcon(getFileIcon(file.name))"
                        class="text-muted-foreground group-hover:text-primary size-4"
                      />
                    </span>
                    <div class="min-w-0 flex-1">
                      <span
                        class="text-foreground block truncate text-xs leading-[1.35] font-medium"
                        :title="fileDisplayTitle(file)"
                      >
                        {{ file.name }}
                      </span>
                      <span class="text-placeholder mt-px block truncate text-[11px] leading-[1.3]">
                        <template v-if="fileRelativeDir(file)">
                          <span class="text-muted-foreground" :title="fileRelativeDir(file)">{{
                            fileRelativeDir(file)
                          }}</span>
                          <span class="mx-1">·</span>
                        </template>
                        {{ formatFileSize(file.size) }}
                      </span>
                    </div>
                    <button
                      type="button"
                      data-slot="file-remove"
                      class="text-placeholder hover:text-foreground flex size-[22px] shrink-0 items-center justify-center rounded opacity-45 transition-all group-hover:opacity-100 hover:bg-[var(--td-bg-color-component)] focus-visible:opacity-100"
                      :aria-label="t('common.remove')"
                      @click="removeFile(index)"
                    >
                      <XIcon class="size-3.5" />
                    </button>
                  </li>
                </ul>
                <div v-else-if="mode === 'file'" class="text-placeholder flex-1 px-2 py-4 text-center text-xs">
                  {{ t("uploadConfirm.noItems") }}
                </div>
              </div>
            </aside>

            <aside
              class="border-border flex min-h-0 w-[216px] shrink-0 flex-col border-r bg-[var(--td-bg-color-settings-modal,var(--td-bg-color-secondarycontainer))] max-[800px]:w-auto max-[800px]:border-r-0 max-[800px]:border-b"
            >
              <div class="border-border box-border flex min-h-14 w-full shrink-0 items-center border-b p-3">
                <h2 class="text-foreground m-0 text-base leading-[1.35] font-semibold">
                  {{ t("uploadConfirm.parseConfig") }}
                </h2>
              </div>
              <nav
                class="min-h-0 flex-1 overflow-y-auto px-1.5 pt-2.5 pb-3 max-[800px]:flex max-[800px]:flex-none max-[800px]:flex-nowrap max-[800px]:gap-1 max-[800px]:overflow-x-auto max-[800px]:p-2"
                :aria-label="t('uploadConfirm.configNav')"
              >
                <button
                  v-for="item in navItems"
                  :key="item.key"
                  type="button"
                  data-slot="section-nav-item"
                  :data-active="activeSection === item.key || undefined"
                  class="mb-0.5 flex w-full items-start rounded-md px-2.5 py-[9px] text-left text-sm transition-all select-none max-[800px]:mb-0 max-[800px]:w-auto max-[800px]:shrink-0 max-[800px]:whitespace-nowrap"
                  :class="
                    activeSection === item.key
                      ? 'bg-muted text-primary font-medium'
                      : 'text-foreground hover:bg-accent bg-transparent'
                  "
                  @click="activeSection = item.key"
                >
                  <component
                    :is="sectionIcon(item.icon)"
                    class="mt-0.5 mr-2 flex size-4 shrink-0 items-center justify-center"
                  />
                  <span class="flex min-w-0 flex-1 flex-col gap-0.75">
                    <!-- The active row's colours win over the issue and tone colours, as they did in the old CSS. -->
                    <span
                      class="truncate text-[13px] leading-[1.35] font-medium"
                      :class="{ 'text-destructive': item.issue && activeSection !== item.key }"
                    >
                      {{ item.label }}
                    </span>
                    <span
                      class="truncate text-xs leading-[1.35]"
                      :class="
                        activeSection === item.key
                          ? 'text-muted-foreground'
                          : {
                              'text-placeholder': item.statusTone !== 'warning' && item.statusTone !== 'error',
                              'text-warning': item.statusTone === 'warning',
                              'text-destructive': item.statusTone === 'error',
                            }
                      "
                      :title="item.statusFull"
                      >{{ item.status }}</span
                    >
                  </span>
                  <span
                    v-if="item.issue"
                    class="bg-destructive ml-1.5 size-1.5 shrink-0 rounded-full"
                    aria-hidden="true"
                  />
                </button>
              </nav>
            </aside>

            <div class="flex min-w-0 flex-1 flex-col overflow-hidden">
              <div class="bg-card min-h-0 min-w-0 flex-1 overflow-y-auto px-8 pt-[22px] pb-7 max-[800px]:p-4">
                <div v-show="activeSection === 'tags'" class="mb-8 last:mb-0">
                  <div>
                    <div class="mb-4">
                      <h2 class="text-foreground m-0 mb-1.5 text-xl font-semibold">{{ t("uploadConfirm.tabTags") }}</h2>
                      <p class="text-placeholder m-0 text-sm leading-[22px]">
                        {{ t("uploadConfirm.tagsDescription") }}
                      </p>
                    </div>
                    <div class="flex flex-col">
                      <div class="flex flex-col gap-3 py-4">
                        <div class="w-full max-w-none pr-0">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("uploadConfirm.tagsPlaceholder")
                          }}</label>
                        </div>
                        <div class="block w-full max-w-none">
                          <!--
                            The old multiple + filterable + clearable select: a
                            popover with a filter field over a checkbox list, so
                            picking several tags does not close it after each one.
                          -->
                          <Popover
                            v-model:open="tagPickerOpen"
                            @update:open="(open: boolean) => open && (tagQuery = '')"
                          >
                            <div class="relative w-full">
                              <PopoverTrigger as-child>
                                <Button
                                  variant="outline"
                                  class="w-full justify-between pr-8 font-normal"
                                  :aria-label="t('uploadConfirm.tagsPlaceholder')"
                                >
                                  <span class="truncate" :class="{ 'text-placeholder': !selectedTagIds.length }">
                                    {{ selectedTagIds.length ? selectedTagNames : t("uploadConfirm.tagsPlaceholder") }}
                                  </span>
                                  <Loader2Icon v-if="tagsLoading" class="size-4 shrink-0 animate-spin opacity-50" />
                                  <ChevronDownIcon
                                    v-else-if="!selectedTagIds.length"
                                    class="size-4 shrink-0 opacity-50"
                                  />
                                </Button>
                              </PopoverTrigger>
                              <button
                                v-if="selectedTagIds.length && !tagsLoading"
                                type="button"
                                data-slot="select-clear"
                                class="text-placeholder hover:text-muted-foreground absolute top-1/2 right-2.5 -translate-y-1/2"
                                :aria-label="t('common.clear')"
                                @click="selectedTagIds = []"
                              >
                                <CircleXIcon class="size-4" aria-hidden="true" />
                              </button>
                            </div>
                            <PopoverContent
                              align="start"
                              class="w-(--reka-popover-trigger-width) min-w-[240px] gap-1 p-1"
                            >
                              <Input
                                v-model="tagQuery"
                                :placeholder="t('knowledgeBase.tagEditSearch')"
                                class="h-7 text-xs md:text-xs"
                              />
                              <div class="max-h-64 overflow-y-auto" role="listbox" aria-multiselectable="true">
                                <label
                                  v-for="tag in filteredTags"
                                  :key="tag.id"
                                  class="hover:bg-accent flex cursor-pointer items-center gap-2 rounded-md px-1.5 py-1 text-sm"
                                  role="option"
                                  :aria-selected="selectedTagIds.includes(tag.id)"
                                >
                                  <Checkbox
                                    :model-value="selectedTagIds.includes(tag.id)"
                                    @update:model-value="(checked) => toggleTag(tag.id, checked === true)"
                                  />
                                  <span class="min-w-0 truncate">{{ tag.name }}</span>
                                </label>
                                <p
                                  v-if="!filteredTags.length"
                                  class="text-placeholder m-0 px-1.5 py-2 text-center text-xs"
                                >
                                  {{ t("knowledgeBase.tagEmptyResult") }}
                                </p>
                              </div>
                            </PopoverContent>
                          </Popover>
                          <p v-if="tagsLoadFailed" class="text-destructive mx-0 mt-1.5 mb-0 text-xs leading-normal">
                            {{ t("uploadConfirm.tagsLoadFailed") }}
                          </p>
                          <p
                            v-else-if="!tagsLoading && availableTags.length === 0"
                            class="text-placeholder mx-0 mt-1.5 mb-0 text-xs leading-normal"
                          >
                            {{ t("uploadConfirm.tagsEmpty") }}
                          </p>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <div v-show="activeSection === 'parser'" class="mb-8 last:mb-0">
                  <KBParserSettings
                    :relevant-extensions="batchFileExts"
                    :parser-engine-rules="uiState.chunkingConfig.parserEngineRules"
                    @update:parser-engine-rules="handleParserEngineRulesUpdate"
                  />
                  <div v-if="hasPdf" class="w-full">
                    <div class="flex flex-col">
                      <div
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("uploadConfirm.pdfForceScanned.label")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ t("uploadConfirm.pdfForceScanned.description") }}
                          </p>
                        </div>
                        <div :class="controlCol">
                          <Switch
                            :model-value="uiState.pdfForceScanned"
                            @update:model-value="(val: boolean) => (uiState.pdfForceScanned = val)"
                          />
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <div v-show="activeSection === 'chunking'" class="mb-8 last:mb-0">
                  <div>
                    <div class="mb-4">
                      <h2 class="text-foreground m-0 mb-1.5 text-xl font-semibold">
                        {{ t("knowledgeEditor.chunking.title") }}
                      </h2>
                      <p class="text-placeholder m-0 text-sm leading-[22px]">
                        {{ t("knowledgeEditor.chunking.description") }}
                      </p>
                    </div>
                    <div class="flex flex-col">
                      <div
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.chunking.strategyLabel")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ t("knowledgeEditor.chunking.strategyDescription") }}
                          </p>
                        </div>
                        <div :class="controlCol">
                          <Select
                            :model-value="uiState.chunkingConfig.strategy"
                            @update:model-value="(v) => (uiState.chunkingConfig.strategy = String(v ?? ''))"
                          >
                            <SelectTrigger class="w-[280px]">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem v-for="opt in chunkingStrategyOptions" :key="opt.value" :value="opt.value">
                                {{ opt.label }}
                              </SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                      </div>
                      <div
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.chunking.sizeLabel")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ t("knowledgeEditor.chunking.sizeDescription") }}
                          </p>
                        </div>
                        <div :class="controlCol">
                          <Input
                            v-model.number="uiState.chunkingConfig.chunkSize"
                            type="number"
                            @change="clampNumber(uiState.chunkingConfig, 'chunkSize', 100, 4000)"
                            :min="100"
                            :max="4000"
                            :step="50"
                            class="w-[200px]"
                          />
                        </div>
                      </div>
                      <div
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.chunking.overlapLabel")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ t("knowledgeEditor.chunking.overlapDescription") }}
                          </p>
                        </div>
                        <div :class="controlCol">
                          <Input
                            v-model.number="uiState.chunkingConfig.chunkOverlap"
                            type="number"
                            @change="clampNumber(uiState.chunkingConfig, 'chunkOverlap', 0, 500)"
                            :min="0"
                            :max="500"
                            :step="20"
                            class="w-[200px]"
                          />
                        </div>
                      </div>
                    </div>

                    <button
                      type="button"
                      data-slot="more-options-toggle"
                      class="text-primary mt-1 inline-flex items-center gap-1.5 px-0 py-1.5 text-[13px]"
                      :aria-expanded="chunkingMoreOpen"
                      @click="chunkingMoreOpen = !chunkingMoreOpen"
                    >
                      <ChevronDownIcon
                        class="size-[13px] transition-transform duration-[180ms]"
                        :class="{ 'rotate-180': chunkingMoreOpen }"
                      />
                      <span>{{ t("uploadConfirm.moreOptions") }}</span>
                    </button>

                    <div v-if="chunkingMoreOpen" class="mt-1 flex flex-col">
                      <div
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.chunking.separatorsLabel")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ t("knowledgeEditor.chunking.separatorsDescription") }}
                          </p>
                        </div>
                        <div :class="controlCol">
                          <DropdownMenu>
                            <DropdownMenuTrigger as-child>
                              <Button variant="outline" class="w-[280px] justify-between font-normal">
                                <span class="truncate">
                                  {{
                                    uiState.chunkingConfig.separators.length
                                      ? uiState.chunkingConfig.separators.map((s) => separatorLabel(s)).join(", ")
                                      : ""
                                  }}
                                </span>
                                <ChevronDownIcon class="size-4 opacity-50" />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent class="w-[280px]">
                              <DropdownMenuCheckboxItem
                                v-for="opt in separatorOptions"
                                :key="opt.value"
                                :model-value="uiState.chunkingConfig.separators.includes(opt.value)"
                                @select.prevent
                                @update:model-value="(checked) => toggleSeparator(opt.value, checked === true)"
                              >
                                {{ opt.label }}
                              </DropdownMenuCheckboxItem>
                              <DropdownMenuSeparator />
                              <div class="flex items-center gap-1.5 p-1.5">
                                <!--
                                  keydown.stop: the menu's typeahead would otherwise
                                  take every typed character and move focus to an item.
                                -->
                                <Input
                                  v-model="customSeparator"
                                  :aria-label="t('knowledgeEditor.chunking.separatorsLabel')"
                                  class="h-7 flex-1 text-xs md:text-xs"
                                  @keydown.stop
                                  @keydown.enter.prevent="addCustomSeparator"
                                />
                                <Button
                                  size="icon-xs"
                                  variant="ghost"
                                  :disabled="!customSeparator"
                                  @click="addCustomSeparator"
                                >
                                  <PlusIcon />
                                </Button>
                              </div>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </div>
                      </div>
                      <div
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.chunking.tokenLimitLabel")
                          }}</label>
                        </div>
                        <div :class="controlCol">
                          <Input
                            v-model.number="uiState.chunkingConfig.tokenLimit"
                            type="number"
                            @change="clampNumber(uiState.chunkingConfig, 'tokenLimit', 0, 8192)"
                            :min="0"
                            :max="8192"
                            :step="64"
                            class="w-[200px]"
                          />
                        </div>
                      </div>
                      <div
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.chunking.languagesLabel")
                          }}</label>
                        </div>
                        <div :class="controlCol">
                          <DropdownMenu>
                            <DropdownMenuTrigger as-child>
                              <Button variant="outline" class="w-[280px] justify-between font-normal">
                                <span class="truncate">
                                  {{
                                    uiState.chunkingConfig.languages?.length
                                      ? uiState.chunkingConfig.languages.map((l) => languageLabel(l)).join(", ")
                                      : ""
                                  }}
                                </span>
                                <ChevronDownIcon class="size-4 opacity-50" />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent class="w-[280px]">
                              <DropdownMenuCheckboxItem
                                v-for="opt in languageOptions"
                                :key="opt.value"
                                :model-value="(uiState.chunkingConfig.languages || []).includes(opt.value)"
                                @select.prevent
                                @update:model-value="(checked) => toggleLanguage(opt.value, checked === true)"
                              >
                                {{ opt.label }}
                              </DropdownMenuCheckboxItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </div>
                      </div>
                      <div
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.chunking.parentChildLabel")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ t("knowledgeEditor.chunking.parentChildDescription") }}
                          </p>
                        </div>
                        <div :class="controlCol">
                          <Switch
                            :model-value="uiState.chunkingConfig.enableParentChild"
                            @update:model-value="(val: boolean) => (uiState.chunkingConfig.enableParentChild = val)"
                          />
                        </div>
                      </div>
                      <div
                        v-if="uiState.chunkingConfig.enableParentChild"
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.chunking.parentChunkSizeLabel")
                          }}</label>
                        </div>
                        <div :class="controlCol">
                          <Input
                            v-model.number="uiState.chunkingConfig.parentChunkSize"
                            type="number"
                            @change="clampNumber(uiState.chunkingConfig, 'parentChunkSize', 512, 8192)"
                            :min="512"
                            :max="8192"
                            :step="64"
                            class="w-[200px]"
                          />
                        </div>
                      </div>
                      <div
                        v-if="uiState.chunkingConfig.enableParentChild"
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.chunking.childChunkSizeLabel")
                          }}</label>
                        </div>
                        <div :class="controlCol">
                          <Input
                            v-model.number="uiState.chunkingConfig.childChunkSize"
                            type="number"
                            @change="clampNumber(uiState.chunkingConfig, 'childChunkSize', 64, 2048)"
                            :min="64"
                            :max="2048"
                            :step="32"
                            class="w-[200px]"
                          />
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <div v-show="activeSection === 'multimodal'" class="mb-8 last:mb-0" data-section="multimodal">
                  <div class="w-full">
                    <div class="mb-5">
                      <h2 class="text-foreground m-0 mb-1.5 text-xl font-semibold">
                        {{ t("knowledgeEditor.multimodal.title") }}
                      </h2>
                      <p class="text-muted-foreground m-0 text-sm leading-normal">
                        {{ t("knowledgeEditor.multimodal.description") }}
                      </p>
                    </div>
                    <div
                      v-if="issueSectionKeys.has('multimodal')"
                      class="border-border bg-muted text-muted-foreground mb-4 flex items-start gap-2 rounded-lg border px-3 py-2.5 text-[13px] leading-normal"
                    >
                      <InfoIcon class="text-primary mt-px size-4 shrink-0" />
                      <span>{{ t("uploadConfirm.multimodalSetupHint") }}</span>
                    </div>
                    <div class="flex flex-col">
                      <div
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.advanced.multimodal.label")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ t("knowledgeEditor.advanced.multimodal.description") }}
                          </p>
                        </div>
                        <div :class="controlCol">
                          <Switch
                            :model-value="uiState.multimodalConfig.enabled"
                            @update:model-value="(val: boolean) => (uiState.multimodalConfig.enabled = val)"
                          />
                        </div>
                      </div>
                      <div
                        v-if="uiState.multimodalConfig.enabled"
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">
                            {{ t("knowledgeEditor.advanced.multimodal.vllmLabel") }}
                            <span class="text-destructive ml-0.5 font-medium">*</span>
                          </label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ t("knowledgeEditor.advanced.multimodal.vllmDescription") }}
                          </p>
                        </div>
                        <div :class="controlCol">
                          <ModelSelector
                            model-type="VLLM"
                            :selected-model-id="uiState.multimodalConfig.vllmModelId"
                            :all-models="allModels"
                            :status="showMultimodalModelError ? 'error' : 'default'"
                            :placeholder="t('knowledgeEditor.advanced.multimodal.vllmPlaceholder')"
                            @update:selected-model-id="handleMultimodalVLLMChange"
                            @add-model="handleAddVLLMModel"
                          />
                        </div>
                      </div>
                      <div
                        v-if="uiState.multimodalConfig.enabled"
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.advanced.multimodal.descriptionLanguageLabel")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ t("knowledgeEditor.advanced.multimodal.descriptionLanguageDescription") }}
                          </p>
                        </div>
                        <div :class="controlCol">
                          <!--
                            The old select was clearable back to "automatic"; a
                            Select item cannot carry an empty value, so automatic
                            is a sentinel that maps back to ''.
                          -->
                          <Select
                            :model-value="uiState.multimodalConfig.descriptionLanguage || AUTO_LANGUAGE"
                            @update:model-value="
                              (val) =>
                                (uiState.multimodalConfig.descriptionLanguage =
                                  val === AUTO_LANGUAGE ? '' : String(val ?? ''))
                            "
                          >
                            <SelectTrigger class="w-[280px]">
                              <SelectValue
                                :placeholder="t('knowledgeEditor.advanced.multimodal.descriptionLanguageAuto')"
                              />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem :value="AUTO_LANGUAGE">{{
                                t("knowledgeEditor.advanced.multimodal.descriptionLanguageAuto")
                              }}</SelectItem>
                              <SelectItem value="Chinese">{{ t("language.zhCN") }}</SelectItem>
                              <SelectItem value="English">{{ t("language.enUS") }}</SelectItem>
                              <SelectItem value="Korean">{{ t("language.koKR") }}</SelectItem>
                              <SelectItem value="Russian">{{ t("language.ruRU") }}</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                      </div>
                      <div
                        v-if="uiState.multimodalConfig.enabled"
                        class="border-border flex flex-col gap-3 py-4 [&:not(:last-child)]:border-b"
                      >
                        <div class="w-full max-w-none pr-0">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.advanced.multimodal.customInstructionsLabel")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ t("knowledgeEditor.advanced.multimodal.customInstructionsDescription") }}
                          </p>
                        </div>
                        <div class="block w-full max-w-none">
                          <Textarea
                            v-model="uiState.multimodalConfig.customInstructions"
                            :placeholder="t('knowledgeEditor.advanced.multimodal.customInstructionsPlaceholder')"
                            :maxlength="4000"
                            :rows="3"
                            class="max-h-48 min-h-[76px] overflow-y-auto"
                          />
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <div v-show="activeSection === 'asr'" class="mb-8 last:mb-0" data-section="asr">
                  <div class="w-full">
                    <div class="mb-5">
                      <h2 class="text-foreground m-0 mb-1.5 text-xl font-semibold">
                        {{ t("knowledgeEditor.asr.title") }}
                      </h2>
                      <p class="text-muted-foreground m-0 text-sm leading-normal">
                        {{ t("knowledgeEditor.asr.description") }}
                      </p>
                    </div>
                    <div
                      v-if="issueSectionKeys.has('asr')"
                      class="border-border bg-muted text-muted-foreground mb-4 flex items-start gap-2 rounded-lg border px-3 py-2.5 text-[13px] leading-normal"
                    >
                      <InfoIcon class="text-primary mt-px size-4 shrink-0" />
                      <span>{{ t("uploadConfirm.asrSetupHint") }}</span>
                    </div>
                    <div class="flex flex-col">
                      <div
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.asr.label")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ t("knowledgeEditor.asr.desc") }}
                          </p>
                        </div>
                        <div :class="controlCol">
                          <Switch
                            :model-value="uiState.asrConfig.enabled"
                            @update:model-value="(val: boolean) => (uiState.asrConfig.enabled = val)"
                          />
                        </div>
                      </div>
                      <div
                        v-if="uiState.asrConfig.enabled"
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">
                            {{ t("knowledgeEditor.asr.modelLabel") }}
                            <span class="text-destructive ml-0.5 font-medium">*</span>
                          </label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ t("knowledgeEditor.asr.modelDescription") }}
                          </p>
                        </div>
                        <div :class="controlCol">
                          <ModelSelector
                            model-type="ASR"
                            :selected-model-id="uiState.asrConfig.modelId"
                            :all-models="allModels"
                            :status="showAsrModelError ? 'error' : 'default'"
                            :placeholder="t('knowledgeEditor.asr.modelPlaceholder')"
                            @update:selected-model-id="
                              (val: string) => {
                                uiState.asrConfig.modelId = val;
                              }
                            "
                            @add-model="handleAddASRModel"
                          />
                        </div>
                      </div>
                      <div
                        v-if="uiState.asrConfig.enabled"
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.asr.languageLabel")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ t("knowledgeEditor.asr.languageDescription") }}
                          </p>
                        </div>
                        <div :class="controlCol">
                          <div class="relative w-[280px]">
                            <Input
                              v-model="uiState.asrConfig.language"
                              :placeholder="t('knowledgeEditor.asr.languagePlaceholder')"
                              class="pr-8"
                            />
                            <button
                              v-if="uiState.asrConfig.language"
                              type="button"
                              data-slot="input-clear"
                              class="text-placeholder hover:text-muted-foreground absolute top-1/2 right-2 -translate-y-1/2"
                              :aria-label="t('common.clear')"
                              @click="uiState.asrConfig.language = ''"
                            >
                              <CircleXIcon class="size-4" aria-hidden="true" />
                            </button>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <div v-show="activeSection === 'question'" class="mb-8 last:mb-0">
                  <div class="w-full">
                    <div class="mb-5">
                      <h2 class="text-foreground m-0 mb-1.5 text-xl font-semibold">
                        {{ t("knowledgeEditor.advanced.questionGeneration.label") }}
                      </h2>
                      <p class="text-muted-foreground m-0 text-sm leading-normal">
                        {{ t("knowledgeEditor.advanced.questionGeneration.description") }}
                      </p>
                    </div>
                    <div class="flex flex-col">
                      <div
                        class="border-border flex items-start justify-between py-4 max-[800px]:flex-col max-[800px]:gap-3 [&:not(:last-child)]:border-b"
                      >
                        <div :class="infoCol">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.advanced.questionGeneration.label")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ t("knowledgeEditor.advanced.questionGeneration.countDescription") }}
                          </p>
                        </div>
                        <div :class="[controlCol, 'gap-3']">
                          <Input
                            v-if="uiState.questionGenerationConfig.enabled"
                            v-model.number="uiState.questionGenerationConfig.questionCount"
                            type="number"
                            @change="clampNumber(uiState.questionGenerationConfig, 'questionCount', 1, 10)"
                            :min="1"
                            :max="10"
                            :step="1"
                            class="w-[88px]"
                          />
                          <Switch
                            :model-value="uiState.questionGenerationConfig.enabled"
                            @update:model-value="(val: boolean) => (uiState.questionGenerationConfig.enabled = val)"
                          />
                        </div>
                      </div>
                      <div
                        v-if="uiState.questionGenerationConfig.enabled"
                        class="border-border flex flex-col gap-3 py-4 [&:not(:last-child)]:border-b"
                      >
                        <div class="w-full max-w-none pr-0">
                          <label class="text-foreground mb-1 block text-[15px] font-medium">{{
                            t("knowledgeEditor.advanced.questionGeneration.instructionsLabel")
                          }}</label>
                          <p class="text-muted-foreground m-0 text-[13px] leading-normal">
                            {{ t("knowledgeEditor.advanced.questionGeneration.instructionsDescription") }}
                          </p>
                        </div>
                        <div class="block w-full max-w-none">
                          <Textarea
                            v-model="uiState.questionGenerationConfig.customInstructions"
                            :placeholder="t('knowledgeEditor.advanced.questionGeneration.instructionsPlaceholder')"
                            :maxlength="4000"
                            :rows="3"
                            class="max-h-48 min-h-[76px] overflow-y-auto"
                          />
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <div v-if="isGraphSectionAvailable" v-show="activeSection === 'graph'" class="mb-8 last:mb-0">
                  <GraphSettings
                    :graph-extract="uiState.nodeExtractConfig"
                    :model-id="llmModelId"
                    :all-models="allModels"
                    @update:graphExtract="handleNodeExtractUpdate"
                  />
                </div>
              </div>

              <footer class="border-border bg-card flex shrink-0 justify-end gap-3 border-t px-5 py-3.5">
                <Button variant="outline" @click="handleCancel">
                  {{ t("uploadConfirm.cancel") }}
                </Button>
                <Button :disabled="!canConfirm" @click="handleConfirm">
                  {{ confirmButtonText }}
                </Button>
              </footer>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import {
  AudioLinesIcon,
  ChevronDownIcon,
  CircleXIcon,
  CopyIcon,
  FileSearchIcon,
  ImageIcon,
  InfoIcon,
  LinkIcon,
  Loader2Icon,
  MessageSquareIcon,
  NetworkIcon,
  PlusIcon,
  TagIcon,
  XIcon,
  type LucideIcon,
} from "@lucide/vue";
import ModelSelector from "@/components/ModelSelector.vue";
import KBParserSettings from "../settings/KBParserSettings.vue";
import GraphSettings from "../settings/GraphSettings.vue";
import { useChatResourcesStore } from "@/stores/chatResources";
import { useEditorResourcesStore } from "@/stores/editorResources";
import { useUIStore } from "@/stores/ui";
import { formatFileSize, getFileIcon } from "@/utils/files";
import { fileTypeIcon } from "../utils/fileTypeIcons";
import { getUploadFileKey } from "../utils/uploadSources";
import { listKnowledgeTags } from "@/api/knowledge-base";
import KbUploadSourceDropdown from "./KbUploadSourceDropdown.vue";
import FolderPickerMenu, { type FolderOption } from "./FolderPickerMenu.vue";
import { folderOptionFromPath, sortFolderOptions } from "../folderTree";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import type { KnowledgeProcessOverrides } from "@/types/knowledgeProcess";
import type {
  UploadConfirmManualSource,
  UploadConfirmMode,
  UploadConfirmReparseSource,
  UploadConfirmResult,
} from "@/stores/uploadConfirm";

const IMAGE_EXTENSIONS = ["jpg", "jpeg", "png", "gif", "bmp", "webp"];
const AUDIO_EXTENSIONS = ["mp3", "wav", "m4a", "flac", "ogg"];
const VIDEO_EXTENSIONS = ["mp4", "mov", "avi", "mkv", "webm", "wmv", "flv", "m4v"];

type ConfigSectionKey = "tags" | "parser" | "chunking" | "multimodal" | "asr" | "question" | "graph";
type IssueSectionKey = "multimodal" | "asr";

const sectionIcons: Record<string, LucideIcon> = {
  tag: TagIcon,
  "file-search": FileSearchIcon,
  "file-copy": CopyIcon,
  image: ImageIcon,
  sound: AudioLinesIcon,
  chat: MessageSquareIcon,
  "chart-bubble": NetworkIcon,
};

function sectionIcon(name: string): LucideIcon {
  return sectionIcons[name] ?? InfoIcon;
}

// A settings row: the label column takes 40%, the control 56% with a 280px
// floor (the old `.upload-confirm-content` override, which applied to every
// row); below 800px the row stacks and both take the full width.
const infoCol = "max-w-[40%] flex-[0_0_40%] pr-6 max-[800px]:max-w-none max-[800px]:basis-auto max-[800px]:pr-0";
const controlCol =
  "flex max-w-[56%] min-w-[280px] flex-[1_1_56%] items-center justify-end max-[800px]:max-w-none max-[800px]:basis-auto";

// Select items cannot carry an empty value; this stands for "automatic".
const AUTO_LANGUAGE = "__auto__";

interface ChunkingUIConfig {
  chunkSize: number;
  chunkOverlap: number;
  separators: string[];
  parserEngineRules?: Array<{
    file_types: string[];
    engine: string;
    xlsx_first_row_as_header?: boolean;
  }>;
  enableParentChild: boolean;
  parentChunkSize: number;
  childChunkSize: number;
  strategy?: string;
  tokenLimit?: number;
  languages?: string[];
  tableMetadataInstructions?: string;
}

interface UploadUIState {
  chunkingConfig: ChunkingUIConfig;
  multimodalConfig: {
    enabled: boolean;
    vllmModelId: string;
    descriptionLanguage?: string;
    customInstructions?: string;
  };
  asrConfig: { enabled: boolean; modelId: string; language: string };
  questionGenerationConfig: { enabled: boolean; questionCount: number; customInstructions?: string };
  nodeExtractConfig: {
    enabled: boolean;
    text: string;
    tags: string[];
    nodes: Array<{ name: string; attributes: string[] }>;
    relations: Array<{ node1: string; node2: string; type: string }>;
    customInstructions?: string;
  };
  graphEnabled: boolean;
  pdfForceScanned: boolean;
}

const props = withDefaults(
  defineProps<{
    visible: boolean;
    kbInfo: any;
    mode?: UploadConfirmMode;
    files?: File[];
    urls?: string[];
    tagIds?: string[];
    manualPreview?: UploadConfirmManualSource | null;
    reparsePreview?: UploadConfirmReparseSource | null;
    tagId?: string;
    acceptFileTypes?: string;
    supportedFileTypes?: string[];
    /** Folder the batch will be uploaded into; '' means the knowledge base root. */
    targetFolder?: string;
    /** Existing folders offered as upload destinations. */
    folderOptions?: FolderOption[];
  }>(),
  {
    mode: "file",
    files: () => [],
    urls: () => [],
    tagIds: () => [],
    manualPreview: null,
    reparsePreview: null,
    acceptFileTypes: "",
    supportedFileTypes: () => [],
    targetFolder: "",
    folderOptions: () => [],
  },
);

const emit = defineEmits<{
  "update:visible": [value: boolean];
  confirm: [payload: UploadConfirmResult];
  cancel: [];
}>();

const { t } = useI18n();
const chatResources = useChatResourcesStore();
const editorResources = useEditorResourcesStore();
const uiStore = useUIStore();

const allModels = ref<any[]>([]);
const localFiles = ref<File[]>([]);
const localUrls = ref<string[]>([]);
const availableTags = ref<Array<{ id: string; name: string }>>([]);
const selectedTagIds = ref<string[]>([]);
const tagsLoading = ref(false);
const tagsLoadFailed = ref(false);
const chunkingMoreOpen = ref(false);
const activeSection = ref<ConfigSectionKey>("tags");
const uiState = ref<UploadUIState>(createDefaultUIState());
const customSeparator = ref("");
const tagPickerOpen = ref(false);
const tagQuery = ref("");

const filteredTags = computed(() => {
  const query = tagQuery.value.trim().toLowerCase();
  if (!query) return availableTags.value;
  return availableTags.value.filter((tag) => tag.name.toLowerCase().includes(query));
});

const selectedTagNames = computed(() =>
  selectedTagIds.value.map((id) => availableTags.value.find((tag) => tag.id === id)?.name ?? id).join(", "),
);

/**
 * Keep a number field inside its range once the user leaves it, as
 * t-input-number did; a native number input accepts any typed value.
 */
function clampNumber<T extends object>(target: T, key: keyof T, min: number, max: number) {
  const raw = Number(target[key]);
  if (Number.isNaN(raw)) return;
  target[key] = Math.min(max, Math.max(min, raw)) as T[keyof T];
}
// Destination folder for this batch. Pre-filled from the sidebar tree, but
// editable here so browsing a folder never silently decides where files land.
const localTargetFolder = ref("");
const destinationPickerVisible = ref(false);
// Folders created in this dialog before upload; the server tree only gains them
// once files land, so keep them here across picker open/close cycles.
const pendingFolderPaths = ref<string[]>([]);

const pickerFolderOptions = computed(() => {
  const byPath = new Map<string, FolderOption>();
  (props.folderOptions || []).forEach((option) => byPath.set(option.path, option));
  pendingFolderPaths.value.forEach((path) => {
    if (!byPath.has(path)) byPath.set(path, folderOptionFromPath(path));
  });
  return sortFolderOptions([...byPath.values()]);
});

const dialogVisible = computed({
  get: () => props.visible,
  set: (value: boolean) => emit("update:visible", value),
});

// Deep paths are shown as root / segment / segment in the picker row.
const destinationBreadcrumb = computed(() => {
  if (!localTargetFolder.value) return t("knowledgeBase.folderTree.rootRow");
  const parts = localTargetFolder.value.split("/").filter(Boolean);
  return [t("knowledgeBase.folderTree.rootRow"), ...parts].join(" / ");
});

const destinationFullLabel = computed(() => localTargetFolder.value || t("knowledgeBase.folderTree.rootRow"));

/**
 * Directory a folder-upload file came from, shown under its name so a batch of
 * same-named files (README.md in five folders) stays distinguishable and the
 * resulting structure is visible before confirming.
 */
function fileRelativeDir(file: File): string {
  const relativePath = (file as File & { webkitRelativePath?: string }).webkitRelativePath;
  if (!relativePath) return "";
  return relativePath.split("/").filter(Boolean).slice(0, -1).join("/");
}

function fileDisplayTitle(file: File): string {
  const relativePath = (file as File & { webkitRelativePath?: string }).webkitRelativePath;
  return relativePath || file.name;
}

function onDestinationCreated(path: string) {
  if (!path || pendingFolderPaths.value.includes(path)) return;
  pendingFolderPaths.value = [...pendingFolderPaths.value, path];
}

function onDestinationPicked(path: string) {
  localTargetFolder.value = path;
  destinationPickerVisible.value = false;
}

function getModelName(modelId: string): string {
  if (!modelId) return t("uploadConfirm.notSet");
  const model = allModels.value.find((m: any) => m.id === modelId);
  return model?.name || modelId;
}

function truncateNavText(text: string, max = 18): string {
  if (!text) return text;
  if (text.length <= max) return text;
  return `${text.slice(0, max - 1)}…`;
}

function hasParserCustomization(): boolean {
  const rules = uiState.value.chunkingConfig.parserEngineRules;
  if (!rules?.length) return false;
  return (
    rules.some((rule) => rule.engine && rule.engine !== "builtin") ||
    rules.some((rule) => rule.xlsx_first_row_as_header)
  );
}

function getFileExt(file: File): string {
  const dot = file.name.lastIndexOf(".");
  if (dot < 0) return "";
  return file.name.substring(dot + 1).toLowerCase();
}

function getExtFromUrl(url: string): string {
  try {
    const pathname = new URL(url).pathname;
    const dot = pathname.lastIndexOf(".");
    if (dot < 0) return "";
    return pathname.substring(dot + 1).toLowerCase();
  } catch {
    return "";
  }
}

function inferMediaExtsFromMarkdown(content: string): string[] {
  const exts = new Set<string>();
  const patterns = [/\.(jpg|jpeg|png|gif|bmp|webp)(\?|#|\)|\s|$)/gi, /\.(mp3|wav|m4a|flac|ogg)(\?|#|\)|\s|$)/gi];
  for (const pattern of patterns) {
    let match: RegExpExecArray | null;
    while ((match = pattern.exec(content)) !== null) {
      let ext = match[1].toLowerCase();
      if (ext === "jpeg") ext = "jpg";
      exts.add(ext);
    }
  }
  return [...exts];
}

const manualCharCount = computed(() => props.manualPreview?.content?.length ?? 0);
const batchItemCount = computed(() => localFiles.value.length + localUrls.value.length);

const dialogTitle = computed(() => {
  if (props.mode === "manual") return t("uploadConfirm.titleManual");
  if (props.mode === "reparse") return t("uploadConfirm.titleReparse");
  return t("uploadConfirm.title");
});

const confirmButtonText = computed(() => {
  if (props.mode === "manual") return t("uploadConfirm.confirmManual");
  if (props.mode === "reparse") return t("uploadConfirm.confirmReparse");
  return t("uploadConfirm.confirm");
});

const batchFileExts = computed(() => {
  const set = new Set<string>();
  if (props.mode === "manual" && props.manualPreview?.content) {
    for (const ext of inferMediaExtsFromMarkdown(props.manualPreview.content)) {
      set.add(ext);
    }
  }
  if (props.mode === "reparse") {
    const ext = (props.reparsePreview?.fileType || "").toLowerCase();
    if (ext) set.add(ext);
  }
  for (const url of localUrls.value) {
    const ext = getExtFromUrl(url);
    if (ext) set.add(ext);
  }
  for (const file of localFiles.value) {
    const ext = getFileExt(file);
    if (ext) set.add(ext);
  }
  return [...set];
});

const hasPdf = computed(() => batchFileExts.value.includes("pdf"));

const chunkingStrategyOptions = computed(() => [
  { label: t("knowledgeEditor.chunking.strategies.auto.label"), value: "auto" },
  { label: t("knowledgeEditor.chunking.strategies.heading.label"), value: "heading" },
  { label: t("knowledgeEditor.chunking.strategies.heuristic.label"), value: "heuristic" },
  { label: t("knowledgeEditor.chunking.strategies.legacy.label"), value: "legacy" },
]);

const separatorOptions = computed(() => [
  { label: t("knowledgeEditor.chunking.separators.doubleNewline"), value: "\n\n" },
  { label: t("knowledgeEditor.chunking.separators.singleNewline"), value: "\n" },
  { label: t("knowledgeEditor.chunking.separators.periodCn"), value: "。" },
  { label: t("knowledgeEditor.chunking.separators.exclamationCn"), value: "！" },
  { label: t("knowledgeEditor.chunking.separators.questionCn"), value: "？" },
  { label: t("knowledgeEditor.chunking.separators.semicolonCn"), value: "；" },
  { label: t("knowledgeEditor.chunking.separators.semicolonEn"), value: ";" },
  { label: t("knowledgeEditor.chunking.separators.space"), value: " " },
]);

function separatorLabel(value: string): string {
  return separatorOptions.value.find((o) => o.value === value)?.label ?? value;
}

const languageOptions = computed(() => [
  { label: t("knowledgeEditor.chunking.languageOptions.de"), value: "de" },
  { label: t("knowledgeEditor.chunking.languageOptions.en"), value: "en" },
  { label: t("knowledgeEditor.chunking.languageOptions.zh"), value: "zh" },
]);

function languageLabel(value: string): string {
  return languageOptions.value.find((o) => o.value === value)?.label ?? value;
}

function toggleTag(id: string, checked: boolean) {
  if (checked) {
    if (!selectedTagIds.value.includes(id)) selectedTagIds.value = [...selectedTagIds.value, id];
  } else {
    selectedTagIds.value = selectedTagIds.value.filter((t) => t !== id);
  }
}

function toggleSeparator(value: string, checked: boolean) {
  const list = uiState.value.chunkingConfig.separators;
  if (checked) {
    if (!list.includes(value)) uiState.value.chunkingConfig.separators = [...list, value];
  } else {
    uiState.value.chunkingConfig.separators = list.filter((s) => s !== value);
  }
}

function addCustomSeparator() {
  const value = customSeparator.value;
  if (!value) return;
  toggleSeparator(value, true);
  customSeparator.value = "";
}

function toggleLanguage(value: string, checked: boolean) {
  const list = uiState.value.chunkingConfig.languages || [];
  if (checked) {
    if (!list.includes(value)) uiState.value.chunkingConfig.languages = [...list, value];
  } else {
    uiState.value.chunkingConfig.languages = list.filter((l) => l !== value);
  }
}

const llmModelId = computed(() => props.kbInfo?.summary_model_id || "");

const hasImages = computed(() => {
  if (props.mode === "manual" && props.manualPreview?.content) {
    const content = props.manualPreview.content;
    if (/data:image\/|!\[[^\]]*\]\([^)]+\)/i.test(content)) return true;
  }
  return batchFileExts.value.some((ext) => IMAGE_EXTENSIONS.includes(ext));
});

const hasAudio = computed(() => {
  return batchFileExts.value.some((ext) => AUDIO_EXTENSIONS.includes(ext));
});

const hasVideo = computed(() => {
  return batchFileExts.value.some((ext) => VIDEO_EXTENSIONS.includes(ext));
});

const isGraphDatabaseEnabled = computed(() => {
  const engine = editorResources.systemInfo?.graph_database_engine;
  return !!engine && engine !== "Not Enabled";
});

const isGraphSectionAvailable = computed(() => {
  return isGraphDatabaseEnabled.value && uiState.value.graphEnabled;
});

const showMultimodalModelError = computed(() => {
  return uiState.value.multimodalConfig.enabled && !uiState.value.multimodalConfig.vllmModelId;
});

const showAsrModelError = computed(() => {
  return uiState.value.asrConfig.enabled && !uiState.value.asrConfig.modelId;
});

const issueSectionKeys = computed(() => {
  const keys = new Set<IssueSectionKey>();
  // A video batch needs BOTH capabilities: ASR transcribes the audio track
  // into the timeline, multimodal captions the extracted keyframes.
  if (hasImages.value || hasVideo.value) {
    if (!uiState.value.multimodalConfig.enabled || !uiState.value.multimodalConfig.vllmModelId) {
      keys.add("multimodal");
    }
  } else if (showMultimodalModelError.value) {
    keys.add("multimodal");
  }
  if (hasAudio.value || hasVideo.value) {
    if (!uiState.value.asrConfig.enabled || !uiState.value.asrConfig.modelId) {
      keys.add("asr");
    }
  } else if (showAsrModelError.value) {
    keys.add("asr");
  }
  return keys;
});

const navItems = computed(() => {
  const items: Array<{
    key: ConfigSectionKey;
    icon: string;
    label: string;
    status: string;
    statusFull: string;
    statusTone?: "warning" | "error" | "muted";
    issue?: boolean;
  }> = [];

  const push = (key: ConfigSectionKey, icon: string, label: string, issue?: boolean) => {
    const statusMeta = getSectionNavStatus(key, issue);
    const full = statusMeta.status;
    items.push({
      key,
      icon,
      label,
      status: truncateNavText(full),
      statusFull: full,
      statusTone: statusMeta.statusTone,
      issue,
    });
  };

  if (props.mode !== "reparse") {
    push("tags", "tag", t("uploadConfirm.tabTags"));
  }
  push("parser", "file-search", t("settings.parserEngine"));
  push("chunking", "file-copy", t("knowledgeEditor.sidebar.chunking"));
  push("multimodal", "image", t("knowledgeEditor.sidebar.multimodal"), issueSectionKeys.value.has("multimodal"));
  push("asr", "sound", t("knowledgeEditor.sidebar.asr"), issueSectionKeys.value.has("asr"));
  push("question", "chat", t("knowledgeEditor.advanced.questionGeneration.label"));
  if (isGraphSectionAvailable.value) {
    push("graph", "chart-bubble", t("knowledgeEditor.sidebar.graph"));
  }
  return items;
});

function getSectionNavStatus(
  key: ConfigSectionKey,
  issue?: boolean,
): { status: string; statusTone?: "warning" | "error" | "muted" } {
  switch (key) {
    case "tags":
      if (selectedTagIds.value.length === 0) {
        return { status: t("uploadConfirm.summaryNoTags"), statusTone: "muted" };
      }
      return {
        status: t("uploadConfirm.summaryTagsCount", { count: selectedTagIds.value.length }),
      };
    case "parser":
      if (uiState.value.pdfForceScanned && hasPdf.value) {
        return { status: t("uploadConfirm.summaryParserForceScanned") };
      }
      if (hasParserCustomization()) {
        return { status: t("uploadConfirm.navParserCustomized") };
      }
      return { status: t("uploadConfirm.navParserDefault"), statusTone: "muted" };
    case "chunking": {
      const chunking = uiState.value.chunkingConfig;
      const parts = [t("uploadConfirm.navChunkingSummary", { size: chunking.chunkSize })];
      if (chunking.enableParentChild) {
        parts.push(t("uploadConfirm.summaryParentChildShort"));
      }
      return { status: parts.join(" · ") };
    }
    case "multimodal": {
      if (issue) {
        return { status: t("uploadConfirm.statusNeedsSetup"), statusTone: "error" };
      }
      const mm = uiState.value.multimodalConfig;
      if (!mm.enabled) {
        return { status: t("uploadConfirm.statusOff"), statusTone: "muted" };
      }
      return {
        status: mm.vllmModelId ? getModelName(mm.vllmModelId) : t("uploadConfirm.notSet"),
        statusTone: mm.vllmModelId ? undefined : "warning",
      };
    }
    case "asr": {
      if (issue) {
        return { status: t("uploadConfirm.statusNeedsSetup"), statusTone: "error" };
      }
      const asr = uiState.value.asrConfig;
      if (!asr.enabled) {
        return { status: t("uploadConfirm.statusOff"), statusTone: "muted" };
      }
      return {
        status: asr.modelId ? getModelName(asr.modelId) : t("uploadConfirm.notSet"),
        statusTone: asr.modelId ? undefined : "warning",
      };
    }
    case "question": {
      const question = uiState.value.questionGenerationConfig;
      if (!question.enabled) {
        return { status: t("uploadConfirm.statusOff"), statusTone: "muted" };
      }
      return {
        status: t("uploadConfirm.summaryQuestionCountValue", { count: question.questionCount }),
      };
    }
    case "graph": {
      if (!uiState.value.graphEnabled || !uiState.value.nodeExtractConfig.enabled) {
        return { status: t("uploadConfirm.statusOff"), statusTone: "muted" };
      }
      const tagCount = uiState.value.nodeExtractConfig.tags?.length ?? 0;
      if (tagCount > 0) {
        return { status: t("uploadConfirm.summaryGraphTagsValue", { count: tagCount }) };
      }
      return { status: t("uploadConfirm.statusOn") };
    }
    default:
      return { status: "" };
  }
}

const canConfirm = computed(() => {
  if (props.mode === "file" && batchItemCount.value === 0) return false;
  if (props.mode === "manual" && !props.manualPreview?.content?.trim()) return false;
  if (hasImages.value) {
    if (!uiState.value.multimodalConfig.enabled || !uiState.value.multimodalConfig.vllmModelId) {
      return false;
    }
  }
  if (hasAudio.value) {
    if (!uiState.value.asrConfig.enabled || !uiState.value.asrConfig.modelId) {
      return false;
    }
  }
  if (showMultimodalModelError.value || showAsrModelError.value) {
    return false;
  }
  return true;
});

function getDefaultSection(): ConfigSectionKey {
  if (props.mode === "reparse") return "parser";
  if (issueSectionKeys.value.has("multimodal")) return "multimodal";
  if (issueSectionKeys.value.has("asr")) return "asr";
  return "tags";
}

function goToSection(key: ConfigSectionKey) {
  activeSection.value = key;
  nextTick(() => {
    document.querySelector(`[data-section="${key}"]`)?.scrollIntoView({ behavior: "smooth", block: "start" });
  });
}

function createDefaultUIState(): UploadUIState {
  return {
    chunkingConfig: {
      chunkSize: 512,
      chunkOverlap: 80,
      separators: ["\n\n", "\n", "。", "！", "？", ";", "；"],
      parserEngineRules: undefined,
      enableParentChild: true,
      parentChunkSize: 4096,
      childChunkSize: 384,
      strategy: "auto",
      tokenLimit: 0,
      languages: [],
      tableMetadataInstructions: "",
    },
    multimodalConfig: { enabled: false, vllmModelId: "", descriptionLanguage: "", customInstructions: "" },
    asrConfig: { enabled: false, modelId: "", language: "" },
    questionGenerationConfig: { enabled: true, questionCount: 3, customInstructions: "" },
    nodeExtractConfig: {
      enabled: false,
      text: "",
      tags: [],
      nodes: [],
      relations: [],
      customInstructions: "",
    },
    graphEnabled: false,
    pdfForceScanned: false,
  };
}

function initFromKbInfo(kb: any) {
  if (!kb) {
    uiState.value = createDefaultUIState();
    return;
  }

  uiState.value = {
    chunkingConfig: {
      chunkSize: kb.chunking_config?.chunk_size || 512,
      chunkOverlap: kb.chunking_config?.chunk_overlap || 80,
      separators: kb.chunking_config?.separators || ["\n\n", "\n", "。", "！", "？", ";", "；"],
      parserEngineRules: kb.chunking_config?.parser_engine_rules || undefined,
      enableParentChild: kb.chunking_config?.enable_parent_child ?? false,
      parentChunkSize: kb.chunking_config?.parent_chunk_size || 4096,
      childChunkSize: kb.chunking_config?.child_chunk_size || 384,
      strategy: kb.chunking_config?.strategy || "auto",
      tokenLimit: kb.chunking_config?.token_limit || 0,
      languages: kb.chunking_config?.languages || [],
      tableMetadataInstructions: kb.chunking_config?.table_metadata_instructions || "",
    },
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
    questionGenerationConfig: {
      enabled: kb.question_generation_config?.enabled ?? true,
      questionCount: kb.question_generation_config?.question_count || 3,
      customInstructions: kb.question_generation_config?.custom_instructions || "",
    },
    nodeExtractConfig: {
      enabled: !!kb.extract_config?.enabled && !!kb.indexing_strategy?.graph_enabled,
      text: kb.extract_config?.text || "",
      tags: kb.extract_config?.tags || [],
      nodes: (kb.extract_config?.nodes || []).map((node: any) => ({
        name: node.name,
        attributes: node.attributes || [],
      })),
      relations: kb.extract_config?.relations || [],
      customInstructions: kb.extract_config?.custom_instructions || "",
    },
    graphEnabled: kb.indexing_strategy?.graph_enabled ?? false,
    pdfForceScanned: false,
  };
}

function buildProcessOverrides(): KnowledgeProcessOverrides {
  const state = uiState.value;
  const chunking = state.chunkingConfig;

  const overrides: KnowledgeProcessOverrides = {
    parser_engine_rules: chunking.parserEngineRules,
    chunking_config: {
      chunk_size: chunking.chunkSize,
      chunk_overlap: chunking.chunkOverlap,
      separators: chunking.separators,
      enable_parent_child: chunking.enableParentChild,
      parent_chunk_size: chunking.parentChunkSize,
      child_chunk_size: chunking.childChunkSize,
      strategy: chunking.strategy,
      token_limit: chunking.tokenLimit,
      languages: chunking.languages,
      table_metadata_instructions: chunking.tableMetadataInstructions,
    },
    enable_multimodel: state.multimodalConfig.enabled,
    vlm_config: {
      enabled: state.multimodalConfig.enabled,
      model_id: state.multimodalConfig.vllmModelId,
      description_language: state.multimodalConfig.descriptionLanguage,
      custom_instructions: state.multimodalConfig.customInstructions,
    },
    asr_config: {
      enabled: state.asrConfig.enabled,
      model_id: state.asrConfig.modelId,
      language: state.asrConfig.language,
    },
    question_generation_config: {
      enabled: state.questionGenerationConfig.enabled,
      question_count: state.questionGenerationConfig.questionCount,
      custom_instructions: state.questionGenerationConfig.customInstructions,
    },
    graph_enabled: state.nodeExtractConfig.enabled && state.graphEnabled,
    extract_config: {
      enabled: state.nodeExtractConfig.enabled,
      text: state.nodeExtractConfig.text,
      tags: state.nodeExtractConfig.tags,
      nodes: state.nodeExtractConfig.nodes,
      relations: state.nodeExtractConfig.relations,
      custom_instructions: state.nodeExtractConfig.customInstructions,
    },
  };

  if (state.pdfForceScanned) {
    overrides.parser_engine_overrides = {
      pdf_force_scanned: "true",
    };
  }

  return overrides;
}

function applyOverridesToState(o?: KnowledgeProcessOverrides | null) {
  if (!o) return;
  const s = uiState.value;
  const cc = o.chunking_config;
  if (cc) {
    if (cc.chunk_size != null) s.chunkingConfig.chunkSize = cc.chunk_size;
    if (cc.chunk_overlap != null) s.chunkingConfig.chunkOverlap = cc.chunk_overlap;
    if (cc.separators) s.chunkingConfig.separators = cc.separators;
    if (cc.enable_parent_child != null) s.chunkingConfig.enableParentChild = cc.enable_parent_child;
    if (cc.parent_chunk_size != null) s.chunkingConfig.parentChunkSize = cc.parent_chunk_size;
    if (cc.child_chunk_size != null) s.chunkingConfig.childChunkSize = cc.child_chunk_size;
    if (cc.strategy != null) s.chunkingConfig.strategy = cc.strategy;
    if (cc.token_limit != null) s.chunkingConfig.tokenLimit = cc.token_limit;
    if (cc.languages) s.chunkingConfig.languages = cc.languages;
    if (cc.table_metadata_instructions != null)
      s.chunkingConfig.tableMetadataInstructions = cc.table_metadata_instructions;
    if (cc.parser_engine_rules) s.chunkingConfig.parserEngineRules = cc.parser_engine_rules;
  }
  if (o.parser_engine_rules) s.chunkingConfig.parserEngineRules = o.parser_engine_rules;
  if (o.enable_multimodel != null) s.multimodalConfig.enabled = o.enable_multimodel;
  if (o.vlm_config) {
    if (o.vlm_config.enabled != null) s.multimodalConfig.enabled = o.vlm_config.enabled;
    if (o.vlm_config.model_id != null) s.multimodalConfig.vllmModelId = o.vlm_config.model_id;
    if (o.vlm_config.description_language != null)
      s.multimodalConfig.descriptionLanguage = o.vlm_config.description_language;
    if (o.vlm_config.custom_instructions != null)
      s.multimodalConfig.customInstructions = o.vlm_config.custom_instructions;
  }
  if (o.asr_config) {
    if (o.asr_config.enabled != null) s.asrConfig.enabled = o.asr_config.enabled;
    if (o.asr_config.model_id != null) s.asrConfig.modelId = o.asr_config.model_id;
    if (o.asr_config.language != null) s.asrConfig.language = o.asr_config.language;
  }
  const qg = o.question_generation_config;
  if (qg) {
    if (qg.enabled != null) s.questionGenerationConfig.enabled = qg.enabled;
    if (qg.question_count != null) s.questionGenerationConfig.questionCount = qg.question_count;
    if (qg.custom_instructions != null) s.questionGenerationConfig.customInstructions = qg.custom_instructions;
  }
  const ec = o.extract_config;
  if (ec) {
    if (ec.enabled != null) s.nodeExtractConfig.enabled = ec.enabled;
    if (ec.text != null) s.nodeExtractConfig.text = ec.text;
    if (ec.tags) s.nodeExtractConfig.tags = ec.tags;
    if (ec.nodes) s.nodeExtractConfig.nodes = ec.nodes.map((n) => ({ name: n.name, attributes: n.attributes || [] }));
    if (ec.relations) s.nodeExtractConfig.relations = ec.relations;
    if (ec.custom_instructions != null) s.nodeExtractConfig.customInstructions = ec.custom_instructions;
  }
  if (o.graph_enabled != null) s.graphEnabled = o.graph_enabled;
  s.nodeExtractConfig.enabled = s.nodeExtractConfig.enabled && s.graphEnabled;
  if (o.parser_engine_overrides && o.parser_engine_overrides.pdf_force_scanned === "true") {
    s.pdfForceScanned = true;
  } else {
    s.pdfForceScanned = false;
  }
}

async function loadModels() {
  try {
    await chatResources.ensureModels();
    allModels.value = chatResources.allModels || [];
  } catch {
    allModels.value = [];
  }
}

async function loadSystemInfo() {
  try {
    await editorResources.ensureSystemInfo();
  } catch {
    // Graph section falls back to hidden when system info is unavailable.
  }
}

async function loadTags() {
  const kbId = props.kbInfo?.id;
  availableTags.value = [];
  tagsLoadFailed.value = false;
  if (!kbId || props.mode === "reparse") return;

  tagsLoading.value = true;
  try {
    const response: any = await listKnowledgeTags(kbId, { page: 1, page_size: 1000 });
    const tags = response?.data?.data || [];
    availableTags.value = tags.map((tag: any) => ({
      id: String(tag.id),
      name: String(tag.name || ""),
    }));
  } catch {
    tagsLoadFailed.value = true;
  } finally {
    tagsLoading.value = false;
  }
}

watch(
  () => props.visible,
  (visible) => {
    if (!visible) return;
    localFiles.value = props.mode === "file" ? [...(props.files || [])] : [];
    localUrls.value = props.mode === "file" ? [...(props.urls || [])] : [];
    selectedTagIds.value = props.mode === "reparse" ? [] : [...(props.tagIds || [])];
    localTargetFolder.value = props.mode === "file" ? props.targetFolder || "" : "";
    pendingFolderPaths.value = [];
    destinationPickerVisible.value = false;
    initFromKbInfo(props.kbInfo);
    if (props.mode === "reparse") {
      applyOverridesToState(props.reparsePreview?.processOverrides);
    }
    activeSection.value = getDefaultSection();
    chunkingMoreOpen.value = false;
    loadModels();
    loadSystemInfo();
    loadTags();
  },
);

watch(isGraphSectionAvailable, (available) => {
  if (!available && activeSection.value === "graph") {
    activeSection.value = getDefaultSection();
  }
});

const appendFiles = (incoming: File[]) => {
  const existingKeys = new Set(localFiles.value.map(getUploadFileKey));
  const toAdd: File[] = [];
  let duplicateCount = 0;

  for (const file of incoming) {
    const key = getUploadFileKey(file);
    if (existingKeys.has(key)) {
      duplicateCount++;
      continue;
    }
    existingKeys.add(key);
    toAdd.push(file);
  }

  if (toAdd.length > 0) {
    localFiles.value = [...localFiles.value, ...toAdd];
    MessagePlugin.success(t("uploadConfirm.filesAdded", { count: toAdd.length }));
  } else if (duplicateCount > 0) {
    MessagePlugin.warning(t("uploadConfirm.filesAllDuplicate"));
  }
};

const appendUrl = (url: string) => {
  if (localUrls.value.includes(url)) {
    MessagePlugin.warning(t("uploadConfirm.urlDuplicate"));
    return;
  }
  localUrls.value = [...localUrls.value, url];
  MessagePlugin.success(t("uploadConfirm.urlAdded"));
};

const removeUrl = (index: number) => {
  localUrls.value = localUrls.value.filter((_, i) => i !== index);
};

const removeFile = (index: number) => {
  localFiles.value = localFiles.value.filter((_, i) => i !== index);
};

const handleParserEngineRulesUpdate = (
  rules: Array<{
    file_types: string[];
    engine: string;
    xlsx_first_row_as_header?: boolean;
  }>,
) => {
  uiState.value.chunkingConfig.parserEngineRules = rules;
};

const handleMultimodalVLLMChange = (modelId: string) => {
  uiState.value.multimodalConfig.vllmModelId = modelId;
};

const handleAddVLLMModel = () => {
  uiStore.openSettings("models", "vllm");
};

const handleAddASRModel = () => {
  uiStore.openSettings("models", "asr");
};

const handleNodeExtractUpdate = (config: UploadUIState["nodeExtractConfig"]) => {
  uiState.value.nodeExtractConfig = { ...config };
  uiState.value.graphEnabled = config.enabled;
};

const validateBeforeConfirm = (): boolean => {
  if (hasImages.value) {
    if (!uiState.value.multimodalConfig.enabled || !uiState.value.multimodalConfig.vllmModelId) {
      MessagePlugin.warning(t("uploadConfirm.vlmModelRequired"));
      uiState.value.multimodalConfig.enabled = true;
      goToSection("multimodal");
      return false;
    }
  } else if (showMultimodalModelError.value) {
    MessagePlugin.warning(t("uploadConfirm.vlmModelSelectRequired"));
    goToSection("multimodal");
    return false;
  }

  if (hasAudio.value) {
    if (!uiState.value.asrConfig.enabled || !uiState.value.asrConfig.modelId) {
      MessagePlugin.warning(t("uploadConfirm.asrModelRequired"));
      uiState.value.asrConfig.enabled = true;
      goToSection("asr");
      return false;
    }
  } else if (showAsrModelError.value) {
    MessagePlugin.warning(t("uploadConfirm.asrModelSelectRequired"));
    goToSection("asr");
    return false;
  }
  return true;
};

const handleCancel = () => {
  emit("cancel");
  emit("update:visible", false);
};

const handleConfirm = () => {
  if (props.mode === "file" && batchItemCount.value === 0) {
    MessagePlugin.warning(t("uploadConfirm.noItems"));
    return;
  }
  if (!validateBeforeConfirm()) return;

  const processConfig = buildProcessOverrides();
  if (props.mode === "manual" && props.manualPreview) {
    emit("confirm", {
      processConfig,
      mode: "manual",
      tagIds: [...selectedTagIds.value],
      manual: { ...props.manualPreview, tagIds: [...selectedTagIds.value] },
    });
  } else if (props.mode === "reparse" && props.reparsePreview) {
    emit("confirm", { processConfig, mode: "reparse", reparse: { ...props.reparsePreview } });
  } else {
    emit("confirm", {
      processConfig,
      mode: "file",
      tagIds: [...selectedTagIds.value],
      files: [...localFiles.value],
      urls: [...localUrls.value],
      targetFolder: localTargetFolder.value,
    });
  }
  emit("update:visible", false);
};
</script>

<!-- Vue <Transition> hooks for the modal fade; the only style block in the file. -->
<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
