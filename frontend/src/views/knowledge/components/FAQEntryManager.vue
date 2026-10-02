<template>
  <div class="flex h-full flex-col">
    <div class="flex min-h-0 flex-1 flex-col gap-5">
      <!-- Header -->
      <div class="flex shrink-0 flex-wrap items-start justify-between gap-3">
        <div class="flex flex-col gap-1">
          <div class="flex w-full flex-wrap items-center gap-2">
            <h2 class="text-foreground m-0 flex items-center gap-1.5 text-xl leading-8 font-semibold">
              <button
                type="button"
                data-slot="breadcrumb-link"
                class="text-muted-foreground enabled:hover:bg-card enabled:hover:text-success disabled:text-placeholder -mx-2 -my-1 inline-flex items-center gap-1 rounded-md px-2 py-1 transition-all duration-[120ms] disabled:cursor-not-allowed"
                @click="handleNavigateToKbList"
              >
                {{ $t("menu.knowledgeBase") }}
              </button>
              <ChevronRightIcon class="text-placeholder size-3.5" />
              <KBSwitcherDropdown
                v-if="knowledgeList.length"
                :kb-list="knowledgeList"
                :current-kb-id="props.kbId"
                @select="(id) => handleKnowledgeDropdownSelect({ value: id })"
              >
                <button
                  type="button"
                  data-slot="breadcrumb-link"
                  class="group/crumb text-muted-foreground enabled:hover:bg-card enabled:hover:text-success disabled:text-placeholder -mx-2 -my-1 inline-flex items-center gap-1 rounded-md py-1 pr-1.5 pl-2 transition-all duration-[120ms] disabled:cursor-not-allowed"
                  :disabled="!props.kbId"
                >
                  <template v-if="!kbInfo">
                    <Skeleton class="h-5 w-[120px]" />
                  </template>
                  <template v-else>
                    <span>{{ kbInfo.name }}</span>
                    <ChevronDownIcon
                      class="size-3.5 transition-transform duration-[120ms] group-enabled/crumb:group-hover/crumb:translate-y-px"
                    />
                  </template>
                </button>
              </KBSwitcherDropdown>
              <button
                v-else
                type="button"
                data-slot="breadcrumb-link"
                class="text-muted-foreground enabled:hover:bg-card enabled:hover:text-success disabled:text-placeholder -mx-2 -my-1 inline-flex items-center gap-1 rounded-md px-2 py-1 transition-all duration-[120ms] disabled:cursor-not-allowed"
                :disabled="!props.kbId"
                @click="handleNavigateToCurrentKB"
              >
                <template v-if="!kbInfo">
                  <Skeleton class="h-5 w-[120px]" />
                </template>
                <template v-else>
                  {{ kbInfo.name }}
                </template>
              </button>
              <ChevronRightIcon class="text-placeholder size-3.5" />
              <span class="text-foreground font-semibold">{{ $t("knowledgeEditor.faq.title") }}</span>
            </h2>
            <div class="inline-flex shrink-0 items-center gap-1.5">
              <KBInfoPopover v-if="kbInfo" :kb-info="kbInfo" />
              <Tooltip v-if="canManage">
                <TooltipTrigger as-child>
                  <button
                    type="button"
                    data-slot="kb-settings-button"
                    class="bg-secondary text-muted-foreground enabled:hover:text-primary inline-flex size-[30px] items-center justify-center rounded-full p-0 transition-all duration-200 enabled:hover:bg-[var(--td-success-color-light)] disabled:cursor-not-allowed disabled:opacity-40"
                    @click="handleOpenKBSettings"
                  >
                    <SettingsIcon class="size-4" />
                  </button>
                </TooltipTrigger>
                <TooltipContent side="top">{{ $t("knowledgeBase.settings") }}</TooltipContent>
              </Tooltip>
              <!-- 导入结果：默认仅图标，hover / 点击展开详情 -->
              <div v-if="showImportResultBadge" class="group/import relative shrink-0">
                <button
                  type="button"
                  data-slot="import-result-trigger"
                  class="text-success m-0 inline-flex items-center justify-center p-0.5 leading-none transition-opacity duration-150 hover:opacity-75"
                  :aria-label="$t('faqManager.import.recentResult')"
                  @click.stop="importResultExpanded = !importResultExpanded"
                >
                  <CircleCheckIcon class="size-4" />
                </button>
                <!--
                  The panel opens on hover, on keyboard focus inside the host,
                  or when the trigger was clicked (importResultExpanded), and
                  stays in the DOM so the fade can run both ways.
                -->
                <div
                  class="absolute top-[calc(100%+8px)] left-0 z-[200] transition-[opacity,transform,visibility] duration-150 ease-in-out"
                  :class="
                    importResultExpanded
                      ? 'pointer-events-auto visible translate-y-0 opacity-100'
                      : 'pointer-events-none invisible -translate-y-1 opacity-0 group-focus-within/import:pointer-events-auto group-focus-within/import:visible group-focus-within/import:translate-y-0 group-focus-within/import:opacity-100 group-hover/import:pointer-events-auto group-hover/import:visible group-hover/import:translate-y-0 group-hover/import:opacity-100'
                  "
                >
                  <div
                    class="border-border bg-secondary text-muted-foreground inline-flex w-fit max-w-full items-center gap-2 rounded-md border px-2.5 py-2 text-xs leading-[1.4] whitespace-nowrap shadow-[0_4px_16px_rgba(0,0,0,0.1)]"
                  >
                    <span class="max-w-[360px] min-w-0 flex-[0_1_auto] truncate">{{ importResultSummary }}</span>
                    <span
                      class="inline-flex h-5 shrink-0 items-center rounded-sm px-1.5 text-xs"
                      :class="
                        importResult!.import_mode === 'append'
                          ? 'text-primary bg-[var(--td-brand-color-light)]'
                          : 'text-warning bg-[var(--td-warning-color-light)]'
                      "
                    >
                      {{
                        importResult!.import_mode === "append"
                          ? $t("faqManager.import.appendMode")
                          : $t("faqManager.import.replaceMode")
                      }}
                    </span>
                    <Button
                      v-if="importResult!.failed_entries_url && importResult!.failed_count > 0"
                      variant="ghost"
                      size="xs"
                      class="text-destructive hover:text-destructive h-auto shrink-0 px-1 text-xs"
                      @click="downloadFailedEntries"
                    >
                      {{ $t("faqManager.import.downloadReasons") }}
                    </Button>
                    <span class="text-placeholder shrink-0 text-xs whitespace-nowrap">{{
                      formatImportTime(importResult!.imported_at)
                    }}</span>
                    <button
                      type="button"
                      data-slot="import-result-close"
                      class="text-placeholder hover:text-muted-foreground m-0 inline-flex size-5 shrink-0 items-center justify-center rounded-sm p-0 transition-colors duration-150 hover:bg-black/6"
                      :aria-label="$t('common.close')"
                      @click="closeImportResult"
                    >
                      <XIcon class="size-3.5" />
                    </button>
                  </div>
                </div>
              </div>
              <!-- 导入进行中 -->
              <div
                v-else-if="isImportInProgress && importState.taskStatus"
                class="text-muted-foreground inline-flex w-fit max-w-[min(420px,40vw)] min-w-0 flex-[0_1_auto] items-center gap-2 rounded-md border py-1 pr-2 pl-2.5 text-xs leading-[1.4]"
                :class="
                  importState.taskStatus.status === 'failed'
                    ? 'border-[rgba(227,77,89,0.3)] bg-[rgba(227,77,89,0.06)]'
                    : 'border-border bg-secondary'
                "
              >
                <component
                  :is="importProgressIcon"
                  class="size-4 shrink-0"
                  :class="{
                    'text-primary animate-spin': importState.taskStatus.status === 'running',
                    'text-success': importState.taskStatus.status === 'success',
                    'text-destructive': importState.taskStatus.status === 'failed',
                    'text-placeholder': !['running', 'success', 'failed'].includes(importState.taskStatus.status),
                  }"
                />
                <span
                  class="max-w-[220px] min-w-0 flex-[0_1_auto] truncate"
                  :class="{ 'text-destructive': importState.taskStatus.status === 'failed' }"
                  >{{ importProgressText }}</span
                >
                <div class="h-1 w-[72px] shrink-0 overflow-hidden rounded-[2px] bg-black/8">
                  <div
                    class="h-full rounded-[2px] transition-[width] duration-300 ease-in-out"
                    :class="{
                      'bg-success': importState.taskStatus.status === 'success',
                      'bg-destructive': importState.taskStatus.status === 'failed',
                      'bg-primary':
                        importState.taskStatus.status !== 'success' && importState.taskStatus.status !== 'failed',
                    }"
                    :style="{ width: `${importState.taskStatus.progress}%` }"
                  />
                </div>
                <span class="text-placeholder shrink-0 text-xs tabular-nums"
                  >{{ importState.taskStatus.processed }}/{{ importState.taskStatus.total }}</span
                >
              </div>
            </div>
          </div>
          <p class="text-placeholder m-0 text-sm leading-5 font-normal">
            {{ $t("knowledgeEditor.faq.subtitle") }}
          </p>
        </div>
      </div>

      <div class="flex min-h-0 flex-1">
        <div class="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
          <!-- 搜索栏与标签筛选 -->
          <div class="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-2 pb-3">
            <div class="relative min-w-0 flex-[1_1_220px] max-[767px]:basis-full">
              <SearchIcon
                class="text-placeholder pointer-events-none absolute top-1/2 left-2 size-4 -translate-y-1/2"
              />
              <Input
                :model-value="entrySearchKeyword"
                :placeholder="$t('knowledgeEditor.faq.searchPlaceholder')"
                class="bg-secondary hover:border-primary hover:bg-card focus-visible:border-primary focus-visible:bg-card dark:bg-secondary dark:hover:bg-card dark:focus-visible:bg-card h-8 rounded-md border-transparent pr-7 pl-7 text-[13px] shadow-none focus-visible:ring-0 md:text-[13px]"
                @update:model-value="(v) => (entrySearchKeyword = String(v).trim())"
                @keydown.enter="(e: KeyboardEvent) => !e.isComposing && loadEntries()"
              />
              <button
                v-if="entrySearchKeyword"
                type="button"
                data-slot="input-clear"
                class="text-placeholder hover:text-muted-foreground absolute top-1/2 right-2 inline-flex -translate-y-1/2 items-center"
                :aria-label="$t('common.clear')"
                @click="clearEntrySearch"
              >
                <CircleXIcon class="size-4" />
              </button>
            </div>
            <div class="flex min-w-0 flex-none items-center gap-3 max-[767px]:flex-[1_1_auto]">
              <Popover v-model:open="tagFilterPanelVisible">
                <div class="w-[140px] shrink-0">
                  <PopoverTrigger as-child>
                    <button
                      type="button"
                      data-slot="tag-filter-trigger"
                      class="bg-secondary inline-flex h-8 w-full items-center rounded-[var(--td-radius-default)] border border-transparent px-2 text-sm leading-none transition-[background,border-color] duration-200"
                      :class="isTagFilterPlaceholder ? 'text-placeholder' : 'text-foreground'"
                      :aria-label="$t('knowledgeBase.tagFilterTitle')"
                      :title="activeTagFilterTitle"
                      @mouseenter="tagFilterTriggerHover = true"
                      @mouseleave="tagFilterTriggerHover = false"
                    >
                      <span
                        class="text-placeholder mr-[var(--td-comp-margin-s)] inline-flex shrink-0 items-center"
                        aria-hidden="true"
                      >
                        <TagIcon class="size-4" />
                      </span>
                      <span class="min-w-0 flex-1 truncate text-left">{{ activeTagFilterLabel }}</span>
                      <span class="ml-[var(--td-comp-margin-s)] inline-flex shrink-0 items-center">
                        <span
                          v-if="showTagFilterClear"
                          class="text-placeholder hover:text-muted-foreground inline-flex items-center"
                          :aria-label="$t('common.clear')"
                          @click.stop="clearTagFilter"
                          @mousedown.stop
                          @pointerdown.stop
                        >
                          <CircleXIcon class="size-4" />
                        </span>
                        <ChevronDownIcon
                          v-else
                          class="size-4 shrink-0 transition-[transform,color] duration-200"
                          :class="tagFilterPanelVisible ? 'text-primary rotate-180' : 'text-placeholder'"
                        />
                      </span>
                    </button>
                  </PopoverTrigger>
                </div>
                <PopoverContent
                  align="start"
                  class="border-border z-[5500] w-80 max-w-[min(320px,calc(100vw-32px))] gap-0 rounded-lg border-[0.5px] p-0 shadow-[0_0_0_0.5px_rgba(0,0,0,0.03),0_2px_4px_rgba(0,0,0,0.04),0_8px_24px_rgba(0,0,0,0.1)] ring-0"
                >
                  <div
                    class="text-foreground box-border flex max-h-[min(70vh,480px)] w-full flex-col px-3.5 py-3 text-xs"
                    @click.stop
                  >
                    <div class="mb-2.5 flex items-center justify-between">
                      <div class="flex items-baseline gap-1.5 text-sm font-semibold">
                        <span>{{ $t("knowledgeBase.tagFilterTitle") }}</span>
                        <span class="text-placeholder text-xs font-normal">({{ sidebarCategoryCount }})</span>
                      </div>
                    </div>
                    <div class="relative mb-2.5">
                      <SearchIcon
                        class="text-placeholder pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2"
                      />
                      <Input
                        :model-value="tagSearchQuery"
                        :placeholder="$t('knowledgeBase.tagSearchPlaceholder')"
                        class="border-border bg-card hover:border-primary focus-visible:border-primary focus-visible:ring-primary/10 h-6 rounded-lg pr-6 pl-7 text-sm md:text-sm"
                        @update:model-value="(v) => (tagSearchQuery = String(v).trim())"
                      />
                      <button
                        v-if="tagSearchQuery"
                        type="button"
                        data-slot="input-clear"
                        class="text-placeholder hover:text-muted-foreground absolute top-1/2 right-1.5 inline-flex -translate-y-1/2 items-center"
                        :aria-label="$t('common.clear')"
                        @click="tagSearchQuery = ''"
                      >
                        <CircleXIcon class="size-3.5" />
                      </button>
                    </div>
                    <div class="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto">
                      <template v-if="tagLoading && !sidebarTags.length">
                        <div class="flex flex-wrap gap-1.5">
                          <Skeleton v-for="n in 8" :key="'skel-tag-' + n" class="h-6 w-14 rounded-sm" />
                        </div>
                      </template>
                      <template v-else>
                        <div class="flex flex-wrap gap-1.5">
                          <button
                            v-for="tag in sidebarTags"
                            :key="tag.id"
                            type="button"
                            data-slot="tag-filter-chip"
                            class="inline-flex h-6 items-center gap-1 rounded-sm border px-2 text-[11px]"
                            :class="
                              isTagFilterActive(tag.id)
                                ? 'text-primary bg-primary/6 border-[color-mix(in_srgb,var(--td-brand-color)_35%,var(--td-component-stroke))]'
                                : 'border-border text-muted-foreground bg-transparent'
                            "
                            :title="`${tag.name} (${tag.chunk_count || 0})`"
                            @click="handleTagRowClick(tag.id)"
                          >
                            <span class="max-w-[120px] truncate">{{ tag.name }}</span>
                            <span class="text-placeholder text-[10px]">{{ tag.chunk_count || 0 }}</span>
                          </button>
                        </div>
                        <div v-if="!sidebarTags.length" class="text-placeholder px-1.5 py-2.5 text-center text-[11px]">
                          {{ $t("knowledgeBase.tagEmptyResult") }}
                        </div>
                        <div v-if="tagHasMore" class="flex justify-center pt-0.5">
                          <Button variant="ghost" size="xs" :disabled="tagLoadingMore" @click.stop="loadTags()">
                            <Loader2Icon v-if="tagLoadingMore" class="animate-spin" />
                            {{ $t("tenant.loadMore") }}
                          </Button>
                        </div>
                      </template>
                    </div>
                    <div v-if="canEdit" class="border-border mt-2.5 border-t pt-2.5">
                      <Button variant="ghost" size="xs" @click="openTagManageDrawer">
                        {{ $t("knowledgeBase.tagManageLink") }}
                      </Button>
                    </div>
                  </div>
                </PopoverContent>
              </Popover>
            </div>
            <div class="ml-auto flex flex-none items-center gap-1">
              <!-- 新建：新建条目 / 导入 -->
              <DropdownMenu v-if="faqCreateOptions.length">
                <Tooltip>
                  <TooltipTrigger as-child>
                    <DropdownMenuTrigger as-child>
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        class="text-muted-foreground hover:bg-secondary hover:text-primary dark:hover:bg-secondary"
                        :aria-label="$t('knowledgeEditor.faq.createGroup')"
                      >
                        <PlusIcon class="size-4" />
                      </Button>
                    </DropdownMenuTrigger>
                  </TooltipTrigger>
                  <TooltipContent side="top">{{ $t("knowledgeEditor.faq.createGroup") }}</TooltipContent>
                </Tooltip>
                <DropdownMenuContent align="end" class="w-auto">
                  <DropdownMenuItem
                    v-for="option in faqCreateOptions"
                    :key="option.value"
                    @select="handleFaqAction({ value: option.value })"
                  >
                    <component :is="option.icon" class="size-4" />
                    {{ option.content }}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
              <!-- 导出 -->
              <DropdownMenu>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <DropdownMenuTrigger as-child>
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        class="text-muted-foreground hover:bg-secondary hover:text-primary dark:hover:bg-secondary"
                        :disabled="exportLoading"
                        :aria-label="$t('knowledgeEditor.faqExport.exportButton')"
                      >
                        <Loader2Icon v-if="exportLoading" class="size-4 animate-spin" />
                        <DownloadIcon v-else class="size-4" />
                      </Button>
                    </DropdownMenuTrigger>
                  </TooltipTrigger>
                  <TooltipContent side="top">{{ $t("knowledgeEditor.faqExport.exportButton") }}</TooltipContent>
                </Tooltip>
                <DropdownMenuContent align="end" class="w-auto">
                  <DropdownMenuItem
                    v-for="option in faqExportOptions"
                    :key="option.value"
                    @select="handleFaqAction({ value: option.value })"
                  >
                    {{ option.content }}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
              <!-- 检索 -->
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button
                    variant="ghost"
                    size="icon-xs"
                    class="text-muted-foreground hover:bg-secondary hover:text-primary dark:hover:bg-secondary"
                    :aria-label="$t('knowledgeEditor.faq.searchTest')"
                    @click="handleFaqAction({ value: 'search' })"
                  >
                    <SearchIcon class="size-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="top">{{ $t("knowledgeEditor.faq.searchTest") }}</TooltipContent>
              </Tooltip>
            </div>
          </div>
          <!-- Card List Container with Scroll -->
          <div
            ref="scrollContainer"
            class="flex-1 overflow-x-hidden overflow-y-auto pr-1"
            :class="{ 'pb-[76px]': selectedRowKeys.length > 0 && canSelectEntries }"
            @scroll="handleScroll"
          >
            <!-- FAQ 骨架屏 -->
            <div
              v-if="loading && entries.length === 0"
              class="animate-in fade-in slide-in-from-bottom-[6px] grid w-full grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-3 duration-[320ms] ease-out"
            >
              <div
                v-for="n in 6"
                :key="'faq-skel-' + n"
                class="border-border bg-card box-border flex h-auto min-w-0 flex-col gap-1.5 overflow-hidden rounded-[10px] border p-2.5 shadow-[0_1px_3px_rgba(0,0,0,0.05)]"
              >
                <div class="border-border border-b pb-2.5">
                  <Skeleton class="h-4 w-4/5" />
                </div>
                <div class="flex flex-col gap-2 py-2">
                  <Skeleton class="h-[13px] w-full" />
                  <Skeleton class="h-[13px] w-[90%]" />
                  <Skeleton class="h-[13px] w-[60%]" />
                </div>
                <div class="border-border flex gap-2 border-t pt-2">
                  <Skeleton class="h-[18px] w-[50px] rounded-sm" />
                  <Skeleton class="h-[18px] w-[60px] rounded-sm" />
                </div>
              </div>
            </div>
            <!-- Card List -->
            <template v-else-if="entries.length > 0">
              <!--
                Cards are laid out as a masonry by arrangeCards(), which finds
                them through the `faq-card` class and positions each one
                absolutely; that class is a hook for the script, not styling.
              -->
              <div
                ref="cardListRef"
                class="animate-in fade-in slide-in-from-bottom-[6px] relative w-full min-w-0 duration-[320ms] ease-out"
              >
                <div
                  v-for="entry in entries"
                  :key="entry.id"
                  class="faq-card box-border flex h-fit max-w-full min-w-0 flex-col gap-1.5 overflow-hidden rounded-[10px] border p-2.5 transition-[border-color,box-shadow,background-color] duration-200"
                  :class="[
                    selectedRowKeys.includes(entry.id)
                      ? 'border-primary bg-[var(--td-success-color-light)] shadow-[0_2px_8px_rgba(7,192,95,0.15)]'
                      : 'border-border bg-card shadow-[0_1px_3px_rgba(0,0,0,0.05)]',
                    canSelectEntries
                      ? 'hover:border-primary cursor-pointer hover:shadow-[0_2px_8px_rgba(7,192,95,0.1)]'
                      : 'cursor-default',
                  ]"
                  @click="handleCardSelect(entry.id, !selectedRowKeys.includes(entry.id))"
                >
                  <!-- Card Header -->
                  <div class="border-border relative flex flex-col gap-2 border-b pb-2.5">
                    <div class="flex items-start gap-2.5">
                      <div
                        class="text-foreground line-clamp-2 min-w-0 flex-1 overflow-hidden text-[15px] leading-normal font-semibold break-words"
                        :title="entry.standard_question"
                      >
                        {{ entry.standard_question }}
                      </div>
                      <div class="ml-auto flex shrink-0 items-center gap-1.5">
                        <DropdownMenu
                          v-if="canManage"
                          :open="!!entry.showMore"
                          @update:open="(open: boolean) => (entry.showMore = open)"
                        >
                          <DropdownMenuTrigger as-child>
                            <button
                              type="button"
                              data-slot="card-more-button"
                              class="hover:bg-secondary flex size-7 shrink-0 items-center justify-center rounded-md opacity-60 hover:opacity-100"
                              @click.stop
                            >
                              <img class="size-4" src="@/assets/img/more.png" alt="" />
                            </button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end" class="w-auto" @click.stop>
                            <DropdownMenuItem @select="handleMenuEdit(entry)">
                              <PencilIcon />
                              <span>{{ $t("common.edit") }}</span>
                            </DropdownMenuItem>
                            <DropdownMenuItem variant="destructive" @select="handleMenuDelete(entry)">
                              <Trash2Icon />
                              <span>{{ $t("common.delete") }}</span>
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    </div>
                  </div>

                  <!-- Card Body -->
                  <div class="flex min-w-0 flex-1 flex-col gap-1.5 overflow-hidden [contain:layout]">
                    <!-- Similar Questions Section -->
                    <div v-if="entry.similar_questions?.length" class="flex min-w-0 flex-col gap-1.5 overflow-hidden">
                      <div
                        :class="sectionLabelClass"
                        class="before:bg-primary"
                        @click.stop="entry.similarCollapsed = !entry.similarCollapsed"
                      >
                        <span>{{ $t("knowledgeEditor.faq.similarQuestions") }}</span>
                        <span class="text-placeholder ml-1 font-normal"> ({{ entry.similar_questions.length }}) </span>
                        <component
                          :is="entry.similarCollapsed ? ChevronRightIcon : ChevronDownIcon"
                          class="text-placeholder ml-auto size-[13px] shrink-0"
                        />
                      </div>
                      <Transition v-bind="slideDownTransition">
                        <div v-if="!entry.similarCollapsed" :class="cardTagsClass">
                          <FAQTagTooltip
                            v-for="question in entry.similar_questions"
                            :key="question"
                            :content="question"
                            type="similar"
                            placement="top"
                          >
                            <span :class="questionTagClass">
                              <span class="block min-w-0 truncate leading-[1.4]">{{ question }}</span>
                            </span>
                          </FAQTagTooltip>
                        </div>
                      </Transition>
                    </div>

                    <!-- Negative Questions Section -->
                    <div v-if="entry.negative_questions?.length" class="flex min-w-0 flex-col gap-1.5 overflow-hidden">
                      <div
                        :class="sectionLabelClass"
                        class="before:bg-warning"
                        @click.stop="entry.negativeCollapsed = !entry.negativeCollapsed"
                      >
                        <span>{{ $t("knowledgeEditor.faq.negativeQuestions") }}</span>
                        <span class="text-placeholder ml-1 font-normal"> ({{ entry.negative_questions.length }}) </span>
                        <component
                          :is="entry.negativeCollapsed ? ChevronRightIcon : ChevronDownIcon"
                          class="text-placeholder ml-auto size-[13px] shrink-0"
                        />
                      </div>
                      <Transition v-bind="slideDownTransition">
                        <div v-if="!entry.negativeCollapsed" :class="cardTagsClass">
                          <FAQTagTooltip
                            v-for="question in entry.negative_questions"
                            :key="question"
                            :content="question"
                            type="negative"
                            placement="top"
                          >
                            <span :class="questionTagClass">
                              <span class="block min-w-0 truncate leading-[1.4]">{{ question }}</span>
                            </span>
                          </FAQTagTooltip>
                        </div>
                      </Transition>
                    </div>

                    <!-- Answers Section -->
                    <div class="flex min-w-0 flex-col gap-1.5 overflow-hidden">
                      <div
                        :class="sectionLabelClass"
                        class="before:bg-primary"
                        @click.stop="entry.answersCollapsed = !entry.answersCollapsed"
                      >
                        <span>{{ $t("knowledgeEditor.faq.answers") }}</span>
                        <span v-if="entry.answers?.length" class="text-placeholder ml-1 font-normal">
                          ({{ entry.answers.length }})
                        </span>
                        <component
                          :is="entry.answersCollapsed ? ChevronRightIcon : ChevronDownIcon"
                          class="text-placeholder ml-auto size-[13px] shrink-0"
                        />
                      </div>
                      <Transition v-bind="slideDownTransition">
                        <div v-if="!entry.answersCollapsed" :class="cardTagsClass">
                          <FAQTagTooltip
                            v-for="answer in entry.answers"
                            :key="answer"
                            :content="answer"
                            type="answer"
                            placement="top"
                          >
                            <span :class="questionTagClass">
                              <span class="block min-w-0 truncate leading-[1.4]">{{ answer }}</span>
                            </span>
                          </FAQTagTooltip>
                        </div>
                      </Transition>
                    </div>
                  </div>

                  <!-- Card Footer -->
                  <div
                    class="border-border -mx-2.5 -mb-2.5 flex flex-nowrap items-center justify-between gap-1.5 border-t bg-[rgba(48,50,54,0.02)] px-3 py-2"
                  >
                    <div class="flex min-w-0 flex-1 items-center justify-start" @click.stop>
                      <DropdownMenu v-if="canEdit && tagList.length">
                        <DropdownMenuTrigger as-child>
                          <button type="button" data-slot="faq-tag-chip" :class="tagChipClass">
                            <span class="max-w-[100px] truncate">{{
                              getTagName(entry.tag_id) || $t("knowledgeBase.untagged")
                            }}</span>
                          </button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="start" class="max-h-72 w-auto">
                          <DropdownMenuItem
                            v-for="option in tagDropdownOptions"
                            :key="option.value"
                            @select="handleEntryTagChange(entry.id, option.value)"
                          >
                            {{ option.content }}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                      <span v-else :class="tagChipClass">
                        <span class="max-w-[100px] truncate">{{
                          getTagName(entry.tag_id) || $t("knowledgeBase.untagged")
                        }}</span>
                      </span>
                    </div>
                    <div class="ml-auto flex shrink-0 items-center gap-[5px]" @click.stop>
                      <!-- 暂时隐藏推荐开关
                      <Tooltip>
                        <TooltipTrigger as-child>
                          <div class="hover:bg-accent inline-flex cursor-pointer items-center gap-1.5 rounded-sm px-1 py-0.5">
                            <Switch
                              :key="`${entry.id}-recommended-${entry.is_recommended}`"
                              size="sm"
                              :model-value="entry.is_recommended"
                              :disabled="!!entryRecommendedLoading[entry.id]"
                              @click.stop
                              @update:model-value="(value: boolean) => handleEntryRecommendedChange(entry, value)"
                            />
                            <span class="text-muted-foreground text-[11px]">{{ $t('knowledgeEditor.faq.recommended') }}</span>
                          </div>
                        </TooltipTrigger>
                        <TooltipContent side="top">
                          {{ entry.is_recommended ? $t('knowledgeEditor.faq.recommendedEnabled') : $t('knowledgeEditor.faq.recommendedDisabled') }}
                        </TooltipContent>
                      </Tooltip>
                      -->
                      <Tooltip>
                        <TooltipTrigger as-child>
                          <div
                            class="hover:bg-accent inline-flex cursor-pointer items-center gap-1.5 rounded-sm px-1 py-0.5 transition-all duration-200"
                          >
                            <Loader2Icon
                              v-if="entryStatusLoading[entry.id]"
                              class="text-muted-foreground size-3 animate-spin"
                            />
                            <Switch
                              :key="`${entry.id}-${entry.is_enabled}`"
                              size="sm"
                              class="shrink-0"
                              :model-value="entry.is_enabled"
                              :disabled="!!entryStatusLoading[entry.id] || !canEdit"
                              @click.stop
                              @update:model-value="(value: boolean) => handleEntryStatusChange(entry, value)"
                            />
                          </div>
                        </TooltipTrigger>
                        <TooltipContent side="top">
                          {{
                            entry.is_enabled
                              ? $t("knowledgeEditor.faq.statusEnabled")
                              : $t("knowledgeEditor.faq.statusDisabled")
                          }}
                        </TooltipContent>
                      </Tooltip>
                    </div>
                  </div>
                </div>
              </div>
            </template>
            <template v-else-if="!loading">
              <div class="flex min-h-[400px] items-center justify-center px-5 py-[60px]">
                <div class="flex max-w-[400px] flex-col items-center gap-4 text-center">
                  <FilePlusIcon class="size-12 text-[var(--td-text-color-disabled)] opacity-60" />
                  <div class="text-foreground text-lg leading-7 font-semibold">
                    {{ $t("knowledgeEditor.faq.emptyTitle") }}
                  </div>
                  <div class="text-muted-foreground text-sm leading-[22px] font-normal">
                    {{ $t("knowledgeEditor.faq.emptyDesc") }}
                  </div>
                </div>
              </div>
            </template>
            <div
              v-if="loadingMore"
              class="text-muted-foreground flex items-center justify-center gap-2 px-4 py-6 text-[13px]"
            >
              <Loader2Icon class="text-primary size-4 animate-spin" />
              <span>{{ $t("common.loading") }}</span>
            </div>
            <div
              v-if="hasMore === false && entries.length > 0"
              class="text-placeholder flex items-center justify-center px-4 py-6 text-[13px] italic"
            >
              {{ $t("common.noMoreData") }}
            </div>
          </div>
          <div
            class="pointer-events-none absolute right-0 bottom-3 left-0 z-[6] flex justify-center px-4 [&>*]:pointer-events-auto"
          >
            <FAQBatchBar
              :count="selectedRowKeys.length"
              :enabled-count="selectedEnabledCount"
              :disabled-count="selectedDisabledCount"
              :can-edit="canEdit"
              :can-manage="canManage"
              :tag-loading="batchTagLoading"
              :status-action="batchStatusAction"
              :delete-loading="batchDeleteLoading"
              @cancel="clearFAQSelection"
              @batch-tag="openBatchTagDialog"
              @enable="handleBatchStatusChange(true)"
              @disable="handleBatchStatusChange(false)"
              @delete="handleBatchDelete"
            />
          </div>
        </div>
      </div>
    </div>
    <!-- Editor Drawer -->
    <Drawer :open="editorVisible" swipe-direction="right" @update:open="handleEditorOpenChange">
      <DrawerContent class="w-[520px] max-w-full rounded-none border-0 sm:max-w-none">
        <div class="border-border flex shrink-0 items-center justify-between gap-3 border-b px-6 py-5">
          <DrawerTitle class="text-foreground text-lg font-semibold">
            {{
              editorMode === "create" ? $t("knowledgeEditor.faq.editorCreate") : $t("knowledgeEditor.faq.editorEdit")
            }}
          </DrawerTitle>
          <Button
            variant="ghost"
            size="icon-sm"
            class="text-muted-foreground"
            :aria-label="$t('common.close')"
            @click="handleEditorOpenChange(false)"
          >
            <XIcon />
          </Button>
        </div>
        <div class="flex min-h-0 flex-1 flex-col p-5">
          <div
            class="min-h-0 flex-1 overflow-x-hidden overflow-y-auto [&::-webkit-scrollbar]:w-1.5 [&::-webkit-scrollbar-thumb]:rounded-[3px] [&::-webkit-scrollbar-thumb]:bg-[var(--td-bg-color-component-disabled)] [&::-webkit-scrollbar-thumb:hover]:bg-[var(--td-brand-color)] [&::-webkit-scrollbar-track]:rounded-[3px] [&::-webkit-scrollbar-track]:bg-[var(--td-bg-color-secondarycontainer)]"
          >
            <!--
              Submitting is the footer button's job alone. The native submit a
              stray Enter would cause is swallowed, as the TDesign form did, so
              Enter in the question field cannot save a half-written entry.
            -->
            <form class="w-full" novalidate @submit.prevent>
              <div class="flex flex-col">
                <!-- 标准问 -->
                <div
                  :class="[editorRowClass, stripeClass]"
                  class="border-border before:bg-primary border-b pt-0 pb-5 before:top-0 before:h-[calc(100%-20px)]"
                >
                  <div class="w-full">
                    <label :class="requiredLabelClass" for="faq-standard-question">
                      {{ $t("knowledgeEditor.faq.standardQuestion") }}
                      <span class="text-destructive text-sm font-semibold">*</span>
                    </label>
                    <p :class="editorDescClass">{{ $t("knowledgeEditor.faq.standardQuestionDesc") }}</p>
                  </div>
                  <div class="flex w-full flex-col items-start">
                    <Input
                      id="faq-standard-question"
                      :model-value="editorForm.standard_question"
                      :maxlength="200"
                      :class="editorInputClass"
                      :aria-invalid="editorErrors.standard_question ? true : undefined"
                      @update:model-value="(v) => (editorForm.standard_question = String(v))"
                    />
                    <p v-if="editorErrors.standard_question" class="text-destructive mt-1 mb-0 text-xs">
                      {{ editorErrors.standard_question }}
                    </p>
                  </div>
                </div>

                <!-- 相似问 -->
                <div
                  :class="[editorRowClass, stripeClass]"
                  class="border-border before:bg-primary border-b py-5 before:top-5 before:h-[calc(100%-40px)]"
                >
                  <div class="w-full">
                    <label :class="optionalLabelClass">{{ $t("knowledgeEditor.faq.similarQuestions") }}</label>
                    <p :class="editorDescClass">{{ $t("knowledgeEditor.faq.similarQuestionsDesc") }}</p>
                  </div>
                  <div class="flex w-full flex-col items-start">
                    <div class="flex w-full items-center gap-2">
                      <Input
                        :model-value="similarInput"
                        :placeholder="$t('knowledgeEditor.faq.similarPlaceholder')"
                        :class="editorInputClass"
                        class="min-w-0 flex-1"
                        @update:model-value="(v) => (similarInput = String(v))"
                        @keydown.enter.prevent="(e: KeyboardEvent) => !e.isComposing && addSimilar()"
                      />
                      <Button
                        type="button"
                        :class="addItemButtonClass"
                        :disabled="!similarInput.trim() || editorForm.similar_questions.length >= 10"
                        @click="addSimilar"
                      >
                        <PlusIcon class="size-4" />
                      </Button>
                    </div>
                    <div v-if="editorForm.similar_questions.length > 0" class="mt-2 flex w-full flex-col gap-2">
                      <div
                        v-for="(question, index) in editorForm.similar_questions"
                        :key="index"
                        :class="itemRowClass"
                        class="bg-card border-border hover:bg-secondary hover:border-primary items-center py-2.5 hover:shadow-[0_2px_8px_rgba(7,192,95,0.12)]"
                      >
                        <div :class="itemContentClass">{{ question }}</div>
                        <button
                          type="button"
                          data-slot="remove-item-button"
                          :class="removeItemButtonClass"
                          @click="removeSimilar(index)"
                        >
                          <XIcon class="size-4" />
                        </button>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 反例 -->
                <div
                  :class="[editorRowClass, stripeClass]"
                  class="border-border before:bg-warning border-b py-5 before:top-5 before:h-[calc(100%-40px)]"
                >
                  <div class="w-full">
                    <label :class="optionalLabelClass">{{ $t("knowledgeEditor.faq.negativeQuestions") }}</label>
                    <p :class="editorDescClass">{{ $t("knowledgeEditor.faq.negativeQuestionsDesc") }}</p>
                  </div>
                  <div class="flex w-full flex-col items-start">
                    <div class="flex w-full items-center gap-2">
                      <Input
                        :model-value="negativeInput"
                        :placeholder="$t('knowledgeEditor.faq.negativePlaceholder')"
                        :class="editorInputClass"
                        class="min-w-0 flex-1"
                        @update:model-value="(v) => (negativeInput = String(v))"
                        @keydown.enter.prevent="(e: KeyboardEvent) => !e.isComposing && addNegative()"
                      />
                      <Button
                        type="button"
                        :class="addItemButtonClass"
                        :disabled="!negativeInput.trim() || editorForm.negative_questions.length >= 10"
                        @click="addNegative"
                      >
                        <PlusIcon class="size-4" />
                      </Button>
                    </div>
                    <div v-if="editorForm.negative_questions.length > 0" class="mt-2 flex w-full flex-col gap-2">
                      <div
                        v-for="(question, index) in editorForm.negative_questions"
                        :key="index"
                        :class="itemRowClass"
                        class="hover:border-warning items-center border-[var(--td-warning-color-focus)] bg-[var(--td-warning-color-light)] py-2.5 hover:bg-[var(--td-warning-color-light)] hover:shadow-[0_2px_8px_rgba(251,191,36,0.15)]"
                      >
                        <div :class="itemContentClass">{{ question }}</div>
                        <button
                          type="button"
                          data-slot="remove-item-button"
                          :class="removeItemButtonClass"
                          @click="removeNegative(index)"
                        >
                          <XIcon class="size-4" />
                        </button>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 答案 -->
                <div
                  :class="[editorRowClass, stripeClass]"
                  class="before:bg-primary py-5 before:top-5 before:h-[calc(100%-40px)]"
                >
                  <div class="w-full">
                    <label :class="requiredLabelClass" for="faq-answer-input">
                      {{ $t("knowledgeEditor.faq.answers") }}
                      <span class="text-destructive text-sm font-semibold">*</span>
                    </label>
                    <p :class="editorDescClass">{{ $t("knowledgeEditor.faq.answersDesc") }}</p>
                  </div>
                  <div class="flex w-full flex-col items-start">
                    <div class="flex w-full flex-col gap-2">
                      <div class="flex w-full items-start gap-2">
                        <!--
                          The old control grew from three to six rows with its
                          content; field-sizing does the same without script,
                          bounded by the min/max heights (80px was the old floor).
                        -->
                        <Textarea
                          id="faq-answer-input"
                          :model-value="answerInput"
                          :placeholder="$t('knowledgeEditor.faq.answerPlaceholder')"
                          :class="editorInputClass"
                          class="field-sizing-content max-h-[147px] min-h-20 min-w-0 flex-1 resize-y py-1.5 leading-[1.6]"
                          :aria-invalid="editorErrors.answers ? true : undefined"
                          @update:model-value="(v) => (answerInput = String(v))"
                          @keydown.ctrl.enter="addAnswer"
                          @keydown.meta.enter="addAnswer"
                        />
                        <Button
                          type="button"
                          :class="addItemButtonClass"
                          :disabled="!answerInput.trim() || editorForm.answers.length >= 5"
                          @click="addAnswer"
                        >
                          <PlusIcon class="size-4" />
                        </Button>
                      </div>
                      <div class="text-muted-foreground pr-10 text-right text-[13px] leading-none font-medium">
                        {{ editorForm.answers.length }}/5
                      </div>
                    </div>
                    <p v-if="editorErrors.answers" class="text-destructive mt-1 mb-0 text-xs">
                      {{ editorErrors.answers }}
                    </p>
                    <div v-if="editorForm.answers.length > 0" class="mt-2 flex w-full flex-col gap-2">
                      <div
                        v-for="(answer, index) in editorForm.answers"
                        :key="index"
                        :class="itemRowClass"
                        class="bg-card border-border hover:bg-secondary hover:border-primary items-start py-3 hover:shadow-[0_2px_8px_rgba(7,192,95,0.12)]"
                      >
                        <div :class="itemContentClass">{{ answer }}</div>
                        <button
                          type="button"
                          data-slot="remove-item-button"
                          :class="removeItemButtonClass"
                          @click="removeAnswer(index)"
                        >
                          <XIcon class="size-4" />
                        </button>
                      </div>
                    </div>
                  </div>
                </div>

                <div :class="editorRowClass" class="py-5">
                  <div class="w-full">
                    <label class="text-foreground mb-1 block text-[15px] font-medium">
                      {{ $t("knowledgeBase.tagLabel") }}
                    </label>
                    <p :class="editorDescClass">{{ $t("knowledgeEditor.faq.tagDesc") }}</p>
                  </div>
                  <div class="relative flex w-full flex-col items-start">
                    <Select
                      :model-value="editorForm.tag_id != null ? String(editorForm.tag_id) : undefined"
                      @update:model-value="(v) => (editorForm.tag_id = v == null || v === '' ? undefined : Number(v))"
                    >
                      <SelectTrigger
                        class="border-border bg-card hover:border-primary focus-visible:border-primary focus-visible:ring-primary/10 dark:bg-card h-8 w-full rounded-lg py-1 pr-8 pl-3"
                      >
                        <SelectValue :placeholder="$t('knowledgeEditor.faq.tagPlaceholder')" />
                      </SelectTrigger>
                      <SelectContent position="popper">
                        <SelectItem v-for="option in tagSelectOptions" :key="option.value" :value="option.value">
                          {{ option.label }}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                    <!-- The select had a clear affordance; Reka's has none, so it sits over the trigger. -->
                    <button
                      v-if="editorForm.tag_id != null"
                      type="button"
                      data-slot="select-clear"
                      class="text-placeholder hover:text-muted-foreground absolute top-1/2 right-8 inline-flex -translate-y-1/2 items-center"
                      :aria-label="$t('common.clear')"
                      @click="editorForm.tag_id = undefined"
                    >
                      <CircleXIcon class="size-4" />
                    </button>
                  </div>
                </div>
              </div>
            </form>
          </div>
        </div>

        <div class="border-border flex shrink-0 justify-end gap-3 border-t px-6 py-4">
          <Button variant="outline" @click="handleEditorOpenChange(false)">
            {{ $t("common.cancel") }}
          </Button>
          <Button :disabled="savingEntry" @click="handleSubmitEntry">
            <Loader2Icon v-if="savingEntry" class="animate-spin" />
            {{ editorMode === "create" ? $t("knowledgeEditor.faq.editorCreate") : $t("common.save") }}
          </Button>
        </div>
      </DrawerContent>
    </Drawer>

    <!-- Import Dialog -->
    <Teleport to="body">
      <Transition v-bind="modalTransition">
        <div v-if="importVisible" :class="modalOverlayClass" @click.self="importVisible = false">
          <div
            class="bg-card relative flex max-h-[90vh] w-full max-w-[600px] flex-col overflow-hidden rounded-xl shadow-[0_6px_28px_rgba(15,23,42,0.08)]"
          >
            <!-- 关闭按钮 -->
            <button
              type="button"
              data-slot="modal-close"
              :class="modalCloseClass"
              :aria-label="$t('general.close')"
              @click="importVisible = false"
            >
              <XIcon class="size-5" />
            </button>

            <div class="flex h-full flex-col overflow-hidden">
              <div class="border-border shrink-0 border-b px-6 pt-6 pb-4">
                <h2 class="text-foreground m-0 text-lg font-semibold">{{ $t("knowledgeEditor.faqImport.title") }}</h2>
              </div>

              <div
                class="max-h-[calc(90vh-140px)] min-h-0 flex-1 overflow-x-hidden overflow-y-auto p-6 [&::-webkit-scrollbar]:w-1.5 [&::-webkit-scrollbar-thumb]:rounded-[3px] [&::-webkit-scrollbar-thumb]:bg-[var(--td-bg-color-component-disabled)] [&::-webkit-scrollbar-thumb:hover]:bg-[var(--td-brand-color)] [&::-webkit-scrollbar-track]:rounded-[3px] [&::-webkit-scrollbar-track]:bg-[var(--td-bg-color-secondarycontainer)]"
              >
                <!-- 导入模式选择 -->
                <div class="mb-6 last:mb-0">
                  <label :class="importLabelClass">{{ $t("knowledgeEditor.faqImport.modeLabel") }}</label>
                  <div
                    role="radiogroup"
                    class="border-border inline-flex overflow-hidden rounded-md border"
                    :aria-label="$t('knowledgeEditor.faqImport.modeLabel')"
                  >
                    <button
                      v-for="mode in importModes"
                      :key="mode.value"
                      type="button"
                      role="radio"
                      data-slot="import-mode"
                      :aria-checked="importState.mode === mode.value"
                      class="border-border -ml-px h-8 border-l px-4 text-sm transition-colors first:ml-0 first:border-l-0"
                      :class="
                        importState.mode === mode.value
                          ? 'bg-primary/10 text-primary'
                          : 'bg-card text-foreground hover:text-primary'
                      "
                      @click="importState.mode = mode.value"
                    >
                      {{ mode.label }}
                    </button>
                  </div>
                </div>

                <!-- 文件上传区域 -->
                <div class="mb-6 last:mb-0">
                  <div class="mb-2.5 flex items-center justify-between gap-3">
                    <label :class="importLabelClass">{{ $t("knowledgeEditor.faqImport.fileLabel") }}</label>
                    <DropdownMenu>
                      <DropdownMenuTrigger as-child>
                        <Button
                          variant="outline"
                          size="sm"
                          class="border-border bg-card text-foreground hover:border-primary hover:text-primary dark:bg-card h-auto gap-1.5 rounded-md px-3.5 py-1.5 text-[13px] font-medium whitespace-nowrap hover:bg-[var(--td-success-color-light)] active:bg-[var(--td-success-color-light)]"
                        >
                          <DownloadIcon class="size-4" />
                          <span>{{ $t("knowledgeEditor.faqImport.downloadExample") }}</span>
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end" class="z-[1100] w-auto">
                        <DropdownMenuItem
                          v-for="option in downloadExampleOptions"
                          :key="option.value"
                          @select="handleDownloadExample({ value: option.value })"
                        >
                          {{ option.content }}
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                  <div class="w-full">
                    <input
                      ref="fileInputRef"
                      type="file"
                      accept=".json,.csv,.xlsx,.xls"
                      class="pointer-events-none absolute size-0 overflow-hidden opacity-0"
                      @change="handleFileChange"
                    />
                    <div
                      class="group/upload relative box-border flex min-h-[120px] w-full cursor-pointer items-center justify-center rounded-lg border-2 transition-all duration-300 hover:border-[var(--td-brand-color)] hover:bg-[var(--td-success-color-light)]"
                      :class="
                        importState.file
                          ? 'border-primary border-solid bg-[var(--td-success-color-light)]'
                          : 'border-border bg-secondary border-dashed'
                      "
                      @click="fileInputRef?.click()"
                      @dragover.prevent
                      @dragenter.prevent
                      @drop.prevent="handleFileDrop"
                    >
                      <div class="flex flex-col items-center gap-3 text-center">
                        <UploadIcon
                          class="text-primary size-8 transition-transform duration-200 group-hover/upload:-translate-y-0.5"
                        />
                        <div class="flex flex-col gap-1">
                          <span v-if="!importState.file" class="text-foreground text-sm font-medium">
                            {{ $t("knowledgeEditor.faqImport.clickToUpload") }}
                          </span>
                          <span v-else class="text-primary text-sm font-medium break-all">
                            {{ importState.file.name }}
                          </span>
                          <span v-if="!importState.file" class="text-muted-foreground text-xs">
                            {{ $t("knowledgeEditor.faqImport.dragDropTip") }}
                          </span>
                        </div>
                      </div>
                    </div>
                    <p class="mt-2 mb-3 text-xs leading-[18px] text-[var(--td-text-color-disabled)]">
                      {{ $t("knowledgeEditor.faqImport.fileTip") }}
                    </p>
                  </div>
                </div>

                <!-- 预览区域 -->
                <div v-if="importState.preview.length" class="border-border bg-secondary mt-5 rounded-lg border p-4">
                  <div class="border-border mb-3 flex items-center gap-2 border-b pb-3">
                    <FileSearchIcon class="text-primary size-4 shrink-0" />
                    <span class="text-foreground text-sm font-medium">
                      {{ $t("knowledgeEditor.faqImport.previewCount", { count: importState.preview.length }) }}
                    </span>
                  </div>
                  <div class="mb-2 flex flex-col gap-2">
                    <div
                      v-for="(item, index) in importState.preview.slice(0, 5)"
                      :key="index"
                      class="border-border bg-card hover:border-primary flex items-start gap-3 rounded-md border px-3 py-2.5 transition-all duration-200 hover:shadow-[0_2px_4px_rgba(7,192,95,0.08)]"
                    >
                      <span
                        class="text-primary-foreground flex size-5 shrink-0 items-center justify-center rounded-sm bg-[linear-gradient(135deg,var(--td-brand-color)_0%,var(--td-brand-color-active)_100%)] text-xs font-semibold"
                        >{{ index + 1 }}</span
                      >
                      <span class="text-foreground flex-1 text-[13px] leading-normal break-words">{{
                        item.standard_question
                      }}</span>
                    </div>
                  </div>
                  <p
                    v-if="importState.preview.length > 5"
                    class="border-border text-muted-foreground mt-2 mb-0 border-t pt-2 text-center text-xs"
                  >
                    {{ $t("knowledgeEditor.faqImport.previewMore", { count: importState.preview.length - 5 }) }}
                  </p>
                </div>
              </div>

              <div class="border-border flex shrink-0 justify-end gap-3 border-t px-6 py-4">
                <Button
                  variant="outline"
                  :disabled="importState.importing && importState.taskStatus?.status === 'running'"
                  @click="handleCancelImport"
                >
                  {{ $t("common.cancel") }}
                </Button>
                <Button
                  :disabled="
                    importState.taskStatus?.status === 'running' || (importState.importing && !importState.taskId)
                  "
                  @click="handleImport"
                >
                  <Loader2Icon v-if="importState.importing && !importState.taskId" class="animate-spin" />
                  {{
                    importState.taskStatus?.status === "success"
                      ? $t("common.close")
                      : importState.taskStatus?.status === "failed"
                        ? $t("common.retry")
                        : $t("knowledgeEditor.faqImport.importButton")
                  }}
                </Button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- Batch Tag Dialog -->
    <Teleport to="body">
      <Transition v-bind="modalTransition">
        <div v-if="batchTagDialogVisible" :class="modalOverlayClass" @click.self="batchTagDialogVisible = false">
          <div
            class="bg-card relative flex w-full max-w-[480px] flex-col overflow-hidden rounded-xl shadow-[0_6px_28px_rgba(15,23,42,0.08)]"
          >
            <!-- 关闭按钮 -->
            <button
              type="button"
              data-slot="modal-close"
              :class="modalCloseClass"
              :aria-label="$t('general.close')"
              @click="batchTagDialogVisible = false"
            >
              <XIcon class="size-5" />
            </button>

            <div class="flex flex-col p-6">
              <div class="mb-6 pr-10">
                <h2 class="text-foreground m-0 text-xl leading-[1.4] font-semibold">
                  {{ $t("knowledgeEditor.faq.batchUpdateTag") }}
                </h2>
              </div>

              <div class="min-h-0 flex-1">
                <div
                  class="text-primary mb-5 flex items-start gap-2 rounded-lg border border-[var(--td-brand-color-focus)] bg-[var(--td-brand-color-light)] px-4 py-3 text-sm leading-normal"
                >
                  <InfoIcon class="text-primary mt-0.5 size-4 shrink-0" />
                  <span>{{ $t("knowledgeEditor.faq.batchUpdateTagTip", { count: selectedRowKeys.length }) }}</span>
                </div>
                <div class="flex flex-col">
                  <label class="text-foreground mb-2 text-sm font-medium">{{ $t("knowledgeBase.tagLabel") }}</label>
                  <!--
                    The old select was filterable, which Reka's Select is not;
                    a popover with a search box and the options keeps typing to
                    filter a long tag list.
                  -->
                  <Popover v-model:open="batchTagPickerOpen">
                    <div class="relative w-full">
                      <PopoverTrigger as-child>
                        <button
                          type="button"
                          data-slot="batch-tag-trigger"
                          class="border-border bg-card hover:border-primary flex h-8 w-full items-center justify-between gap-1.5 rounded-lg border py-1 pr-2 pl-3 text-sm transition-colors"
                          :class="{ 'border-primary': batchTagPickerOpen }"
                        >
                          <span
                            class="min-w-0 truncate"
                            :class="batchTagLabel ? 'text-foreground' : 'text-placeholder'"
                          >
                            {{ batchTagLabel || $t("knowledgeBase.tagPlaceholder") }}
                          </span>
                          <ChevronDownIcon class="text-muted-foreground size-4 shrink-0" />
                        </button>
                      </PopoverTrigger>
                      <button
                        v-if="batchTagValue"
                        type="button"
                        data-slot="select-clear"
                        class="text-placeholder hover:text-muted-foreground absolute top-1/2 right-8 inline-flex -translate-y-1/2 items-center"
                        :aria-label="$t('common.clear')"
                        @click="batchTagValue = ''"
                      >
                        <CircleXIcon class="size-4" />
                      </button>
                    </div>
                    <PopoverContent
                      align="start"
                      class="z-[1100] w-(--reka-popover-trigger-width) max-w-none gap-1 p-1"
                    >
                      <Input
                        :model-value="batchTagQuery"
                        :placeholder="$t('knowledgeBase.tagSearchPlaceholder')"
                        class="h-7 text-sm md:text-sm"
                        @update:model-value="(v) => (batchTagQuery = String(v))"
                      />
                      <div class="max-h-60 overflow-y-auto">
                        <button
                          v-for="option in filteredBatchTagOptions"
                          :key="option.value"
                          type="button"
                          data-slot="batch-tag-option"
                          class="hover:bg-accent flex w-full items-center justify-between gap-2 rounded-md px-2 py-1.5 text-left text-sm"
                          :class="{ 'text-primary': batchTagValue === String(option.value) }"
                          @click="selectBatchTag(String(option.value))"
                        >
                          <span class="min-w-0 truncate">{{ option.label }}</span>
                          <CheckIcon v-if="batchTagValue === String(option.value)" class="size-4 shrink-0" />
                        </button>
                        <div
                          v-if="!filteredBatchTagOptions.length"
                          class="text-muted-foreground px-3 py-2 text-center text-sm"
                        >
                          {{ $t("knowledgeBase.noTags") }}
                        </div>
                      </div>
                    </PopoverContent>
                  </Popover>
                </div>
              </div>

              <div class="border-border mt-6 flex justify-end gap-3 border-t pt-5">
                <Button variant="outline" @click="batchTagDialogVisible = false">
                  {{ $t("common.cancel") }}
                </Button>
                <Button :disabled="batchActionLoading" @click="handleBatchTag">
                  <Loader2Icon v-if="batchTagLoading" class="animate-spin" />
                  {{ $t("common.confirm") }}
                </Button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- Search Test Drawer -->
    <Drawer :open="searchDrawerVisible" swipe-direction="right" @update:open="(open) => (searchDrawerVisible = open)">
      <DrawerContent class="w-[420px] max-w-full rounded-none border-0 sm:max-w-none">
        <div class="border-border flex shrink-0 items-center justify-between gap-3 border-b px-6 py-5">
          <DrawerTitle class="text-foreground text-lg font-semibold">
            {{ $t("knowledgeEditor.faq.searchTestTitle") }}
          </DrawerTitle>
          <Button
            variant="ghost"
            size="icon-sm"
            class="text-muted-foreground"
            :aria-label="$t('common.close')"
            @click="searchDrawerVisible = false"
          >
            <XIcon />
          </Button>
        </div>
        <div class="flex min-h-0 flex-1 flex-col p-5">
          <div
            class="flex min-h-0 flex-1 [scrollbar-width:none] flex-col gap-4 overflow-y-auto [&::-webkit-scrollbar]:hidden"
          >
            <!-- Enter in the query and the button both search; the native submit does nothing. -->
            <form class="shrink-0" novalidate @submit.prevent>
              <div class="flex flex-col">
                <!-- 查询文本 -->
                <div :class="searchRowClass" class="border-border border-b pt-0 pb-4">
                  <div :class="searchInfoClass">
                    <label :class="searchLabelClass" for="faq-search-query">
                      {{ $t("knowledgeEditor.faq.queryLabel") }}
                    </label>
                    <p :class="searchDescClass">{{ $t("knowledgeEditor.faq.queryPlaceholder") }}</p>
                  </div>
                  <div class="flex w-full flex-col items-start">
                    <Input
                      id="faq-search-query"
                      :model-value="searchForm.query"
                      :placeholder="$t('knowledgeEditor.faq.queryPlaceholder')"
                      :class="editorInputClass"
                      @update:model-value="(v) => (searchForm.query = String(v))"
                      @keydown.enter.prevent="(e: KeyboardEvent) => !e.isComposing && handleSearch()"
                    />
                  </div>
                </div>

                <!-- 相似度阈值 -->
                <div :class="searchRowClass" class="border-border border-b py-4">
                  <div :class="searchInfoClass">
                    <label :class="searchLabelClass">{{ $t("knowledgeEditor.faq.similarityThresholdLabel") }}</label>
                    <p :class="searchDescClass">{{ $t("knowledgeEditor.faq.vectorThresholdDesc") }}</p>
                  </div>
                  <div class="flex w-full flex-col items-start">
                    <div class="flex w-full items-center gap-3 py-0.5">
                      <Slider
                        class="min-w-0 flex-1"
                        :model-value="[searchForm.vectorThreshold]"
                        :min="0"
                        :max="1"
                        :step="0.1"
                        @update:model-value="(v) => v && (searchForm.vectorThreshold = v[0])"
                      />
                      <div :class="sliderValueClass">{{ searchForm.vectorThreshold.toFixed(2) }}</div>
                    </div>
                  </div>
                </div>

                <!-- 匹配数量 -->
                <div :class="searchRowClass" class="border-border border-b py-4">
                  <div :class="searchInfoClass">
                    <label :class="searchLabelClass">{{ $t("knowledgeEditor.faq.matchCountLabel") }}</label>
                    <p :class="searchDescClass">{{ $t("knowledgeEditor.faq.matchCountDesc") }}</p>
                  </div>
                  <div class="flex w-full flex-col items-start">
                    <div class="flex w-full items-center gap-3 py-0.5">
                      <Slider
                        class="min-w-0 flex-1"
                        :model-value="[searchForm.matchCount]"
                        :min="1"
                        :max="50"
                        :step="1"
                        @update:model-value="(v) => v && (searchForm.matchCount = v[0])"
                      />
                      <div :class="sliderValueClass">{{ searchForm.matchCount }}</div>
                    </div>
                  </div>
                </div>

                <!-- 搜索按钮 -->
                <div :class="searchRowClass" class="pt-4 pb-0">
                  <div class="flex w-full flex-col items-start">
                    <Button
                      type="button"
                      class="h-9 w-full rounded-lg text-sm font-medium transition-all duration-200 hover:-translate-y-px hover:shadow-[0_4px_12px_rgba(7,192,95,0.3)] active:translate-y-0"
                      :disabled="searching"
                      @click="handleSearch"
                    >
                      <Loader2Icon v-if="searching" class="animate-spin" />
                      {{ searching ? $t("knowledgeEditor.faq.searching") : $t("knowledgeEditor.faq.searchButton") }}
                    </Button>
                  </div>
                </div>
              </div>
            </form>

            <!-- Search Results -->
            <div v-if="searchResults.length > 0 || hasSearched" class="box-border flex w-full flex-col pt-5">
              <div class="text-foreground mb-4 flex shrink-0 items-center justify-start gap-2 text-sm font-semibold">
                <span>{{ $t("knowledgeEditor.faq.searchResults") }} ({{ searchResults.length }})</span>
              </div>
              <div
                v-if="searchResults.length === 0"
                class="border-border bg-card text-muted-foreground flex items-center justify-center rounded-lg border border-dashed px-4 py-12 text-center text-sm"
              >
                {{ $t("knowledgeEditor.faq.noResults") }}
              </div>
              <div v-else class="flex flex-col gap-3">
                <div
                  v-for="(result, index) in searchResults"
                  :key="result.id"
                  class="border-border bg-card hover:border-primary relative box-border w-full min-w-0 overflow-visible rounded-lg border p-3.5 shadow-[0_1px_2px_rgba(0,0,0,0.04)] transition-[border-color,box-shadow] duration-200 hover:shadow-[0_2px_8px_rgba(7,192,95,0.12)]"
                >
                  <div
                    class="hover:bg-card relative -m-1 flex cursor-pointer flex-col gap-2 rounded-md p-1 select-none"
                    :class="{ 'border-border mb-3 border-b pb-3': result.expanded }"
                    @click="toggleResult(result)"
                  >
                    <div class="flex w-full items-start gap-2.5">
                      <div class="flex min-w-0 flex-1 flex-col gap-1">
                        <div
                          class="text-foreground flex items-start gap-1.5 text-sm leading-[1.6] font-semibold break-words"
                        >
                          <span class="text-primary shrink-0 font-semibold">{{ index + 1 }}.</span>
                          {{ result.standard_question }}
                        </div>
                        <div
                          v-if="result.matched_question && result.matched_question !== result.standard_question"
                          class="flex items-start gap-1 pl-5 text-xs leading-normal"
                        >
                          <span class="text-warning shrink-0 font-medium"
                            >{{ $t("knowledgeEditor.faq.matchedQuestion") }}:</span
                          >
                          <span
                            class="rounded-sm bg-[linear-gradient(90deg,rgba(251,191,36,0.15)_0%,rgba(251,191,36,0.05)_100%)] px-1.5 py-px break-words text-[var(--td-warning-color-active)]"
                            >{{ result.matched_question }}</span
                          >
                        </div>
                      </div>
                      <div class="ml-auto flex shrink-0 flex-wrap gap-2">
                        <span
                          class="border-border bg-secondary text-foreground inline-flex items-center rounded-md border px-2 py-1 text-xs leading-none"
                        >
                          {{ (result.score || 0).toFixed(3) }}
                        </span>
                      </div>
                      <component
                        :is="result.expanded ? ChevronUpIcon : ChevronDownIcon"
                        class="text-muted-foreground hover:text-primary size-[18px] shrink-0 cursor-pointer transition-transform duration-200"
                      />
                    </div>
                  </div>
                  <Transition v-bind="slideDownTransition">
                    <div v-if="result.expanded" class="border-border relative flex w-full flex-col gap-3 border-t pt-3">
                      <div v-if="result.answers?.length" class="flex flex-col gap-2">
                        <div :class="resultSectionLabelClass">{{ $t("knowledgeEditor.faq.answers") }}</div>
                        <div class="flex w-full min-w-0 flex-wrap gap-1">
                          <Tooltip v-for="answer in result.answers" :key="answer">
                            <TooltipTrigger as-child>
                              <span
                                class="text-success inline-block max-w-full min-w-0 rounded-sm bg-[var(--td-success-color-light)] px-2 py-0.5 text-xs leading-[1.4] break-words whitespace-normal"
                              >
                                {{ answer }}
                              </span>
                            </TooltipTrigger>
                            <TooltipContent side="top" class="whitespace-pre-wrap">{{ answer }}</TooltipContent>
                          </Tooltip>
                        </div>
                      </div>
                      <div v-if="result.similar_questions?.length" class="flex flex-col gap-2">
                        <div :class="resultSectionLabelClass">{{ $t("knowledgeEditor.faq.similarQuestions") }}</div>
                        <div class="flex w-full min-w-0 flex-wrap gap-1">
                          <Tooltip v-for="question in result.similar_questions" :key="question">
                            <TooltipTrigger as-child>
                              <span
                                class="border-border bg-card text-placeholder inline-block max-w-full min-w-0 rounded-[5px] border px-2 py-[3px] text-[11px] leading-[1.4] break-words whitespace-normal"
                              >
                                {{ question }}
                              </span>
                            </TooltipTrigger>
                            <TooltipContent side="top" class="whitespace-pre-wrap">{{ question }}</TooltipContent>
                          </Tooltip>
                        </div>
                      </div>
                    </div>
                  </Transition>
                </div>
              </div>
            </div>
          </div>
        </div>
      </DrawerContent>
    </Drawer>

    <KbTagManageDrawer
      v-model:visible="tagManageDrawerVisible"
      :kb-id="props.kbId"
      :is-faq="true"
      @changed="onTagManageChanged"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, watch, onMounted, computed, nextTick, onUnmounted } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import {
  listFAQEntries,
  upsertFAQEntries,
  createFAQEntry,
  updateFAQEntry,
  updateFAQEntryFieldsBatch,
  deleteFAQEntries,
  searchFAQEntries,
  exportFAQEntries,
  listKnowledgeTags,
  updateFAQEntryTagBatch,
  getKnowledgeBaseById,
  listKnowledgeBases,
  getFAQImportProgress,
  updateFAQImportResultDisplayStatus,
} from "@/api/knowledge-base";
import * as XLSX from "xlsx";
import Papa from "papaparse";
import FAQTagTooltip from "@/components/FAQTagTooltip.vue";
import {
  CheckIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  ChevronUpIcon,
  CircleAlertIcon,
  CircleCheckIcon,
  CircleXIcon,
  ClockIcon,
  DownloadIcon,
  FilePlusIcon,
  FileSearchIcon,
  InfoIcon,
  Loader2Icon,
  PencilIcon,
  PlusIcon,
  SearchIcon,
  SettingsIcon,
  TagIcon,
  Trash2Icon,
  UploadIcon,
  XIcon,
  type LucideIcon,
} from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Drawer, DrawerContent, DrawerTitle } from "@/components/ui/drawer";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import KBInfoPopover from "@/components/KBInfoPopover.vue";
import KBSwitcherDropdown from "@/components/KBSwitcherDropdown.vue";
import FAQBatchBar from "./FAQBatchBar.vue";
import KbTagManageDrawer from "./KbTagManageDrawer.vue";
import { useUIStore } from "@/stores/ui";

interface FAQEntry {
  id: number;
  chunk_id: string;
  knowledge_id: string;
  knowledge_base_id: string;
  tag_id?: number;
  is_enabled: boolean;
  is_recommended: boolean;
  standard_question: string;
  similar_questions: string[];
  negative_questions: string[];
  answers: string[];
  updated_at: string;
  showMore?: boolean;
  score?: number;
  match_type?: string;
  matched_question?: string;
  expanded?: boolean;
  similarCollapsed?: boolean;
  negativeCollapsed?: boolean;
  answersCollapsed?: boolean;
}

interface FAQEntryPayload {
  standard_question: string;
  similar_questions: string[];
  negative_questions: string[];
  answers: string[];
  tag_id?: number;
  tag_name?: string;
  is_enabled?: boolean;
  is_recommended?: boolean;
}

const props = defineProps<{
  kbId: string;
}>();

const { t } = useI18n();
const router = useRouter();
const uiStore = useUIStore();
const authStore = useAuthStore();

// Permission control: check if current user owns this KB or has edit/manage permission.
//
// isOwner used to compare kbInfo.tenant_id against the user's effective tenant id,
// which silently treated "any KB visible to me in my current tenant" as "I created
// it" — Viewer / Contributor ended up showing every FAQ CRUD entry on every KB
// and 403'ing when they clicked. Mirror the rule we settled on in
// KnowledgeBase.vue: explicit creator_id match, with the Admin+ role fallback
// inside canEdit / canManage. A KB with an empty creator_id (created through an
// API key) is workspace-owned (Admin+ may manage).
const isOwner = computed(() => {
  if (!kbInfo.value) return false;
  const creatorId = (kbInfo.value as any).creator_id || "";
  const userId = authStore.user?.id || "";
  if (!creatorId) return false;
  return creatorId === userId;
});

// Can edit: the KB creator (any role) or workspace Admin+ — the same rule as
// KnowledgeBase.vue, and the one the backend enforces.
const canEdit = computed(() => isOwner.value || authStore.hasRole("admin"));

// Can manage (delete, settings): the same two subjects, kept as its own gate
// so the two can diverge again without touching every consumer.
const canManage = computed(() => isOwner.value || authStore.hasRole("admin"));

const canSelectEntries = computed(() => canEdit.value || canManage.value);

const faqExportOptions = computed(() => [
  { content: t("knowledgeEditor.faqExport.exportCSV"), value: "export_csv" },
  { content: t("knowledgeEditor.faqExport.exportJSON"), value: "export_json" },
]);

// FAQ 操作：新建组（新建条目 + 导入）
const faqCreateOptions = computed(() => {
  if (!canEdit.value) return [];
  return [
    {
      content: t("knowledgeEditor.faq.editorCreate"),
      value: "create",
      icon: PlusIcon,
    },
    {
      content: t("knowledgeEditor.faqImport.importButton"),
      value: "import",
      icon: UploadIcon,
    },
  ];
});

// 处理 FAQ 操作
const handleFaqAction = (data: { value: string }) => {
  switch (data.value) {
    case "create":
      openEditor();
      break;
    case "import":
      openImportDialog();
      break;
    case "search":
      searchDrawerVisible.value = true;
      break;
    case "export_csv":
      handleExportCSV();
      break;
    case "export_json":
      handleExportJSON();
      break;
    case "export":
      handleExportCSV();
      break;
  }
};

const loading = ref(true);
const loadingMore = ref(false);
const entries = ref<FAQEntry[]>([]);
const entryStatusLoading = reactive<Record<number, boolean>>({});
const selectedRowKeys = ref<number[]>([]);
const batchDeleteLoading = ref(false);
const batchTagLoading = ref(false);
const batchStatusAction = ref<"enable" | "disable" | null>(null);
const selectedEntries = computed(() => {
  const selectedIds = new Set(selectedRowKeys.value);
  return entries.value.filter((entry) => selectedIds.has(entry.id));
});
const selectedEnabledCount = computed(() => selectedEntries.value.filter((entry) => entry.is_enabled !== false).length);
const selectedDisabledCount = computed(() => selectedEntries.value.length - selectedEnabledCount.value);
const batchActionLoading = computed(
  () => batchDeleteLoading.value || batchTagLoading.value || batchStatusAction.value != null,
);
const scrollContainer = ref<HTMLElement | null>(null);
const cardListRef = ref<HTMLElement | null>(null);
const hasMore = ref(true);
const pageSize = 20;
let currentPage = 1;
const entrySearchKeyword = ref("");
let entrySearchDebounce: number | null = null;

const tagList = ref<any[]>([]);
const tagLoading = ref(false);
const selectedTagIds = ref<string[]>([]);
const tagFilterPanelVisible = ref(false);
const tagFilterTriggerHover = ref(false);
const tagFilterCleared = ref(false);
const tagManageDrawerVisible = ref(false);
const overallFAQTotal = ref(0);
const tagSearchQuery = ref("");
const TAG_PAGE_SIZE = 20;
const tagPage = ref(1);
const tagHasMore = ref(false);
const tagLoadingMore = ref(false);
const tagTotal = ref(0);
let tagSearchDebounce: number | null = null;

const showTagFilterClear = computed(() => selectedTagIds.value.length > 0 && tagFilterTriggerHover.value);

const isTagFilterPlaceholder = computed(() => selectedTagIds.value.length === 0 && tagFilterCleared.value);

const tagMap = computed<Record<string, any>>(() => {
  const map: Record<string, any> = {};
  tagList.value.forEach((tag) => {
    map[tag.id] = tag;
  });
  return map;
});

// tagMapBySeqId uses seq_id as key for looking up by entry.tag_id
const tagMapBySeqId = computed<Record<number, any>>(() => {
  const map: Record<number, any> = {};
  tagList.value.forEach((tag) => {
    map[tag.seq_id] = tag;
  });
  return map;
});

const regularTags = computed(() => tagList.value);
const tagDropdownOptions = computed(() =>
  regularTags.value.map((tag: any) => ({ content: tag.name, value: String(tag.seq_id) })),
);
// Reka's SelectItem only takes string values, so the seq_id is carried as a
// string here and turned back into a number where the form stores it.
const tagSelectOptions = computed(() =>
  regularTags.value.map((tag: any) => ({ label: tag.name as string, value: String(tag.seq_id) })),
);

const sidebarCategoryCount = computed(() => tagTotal.value || tagList.value.length);
const sidebarTags = computed(() => {
  const list = tagList.value;
  const selectedIds = selectedTagIds.value;
  if (!selectedIds.length) {
    return list;
  }
  const missingSelected = selectedIds
    .filter((id) => !list.some((tag) => tag.id === id))
    .map((id) => tagMap.value[id])
    .filter(Boolean);
  if (!missingSelected.length) {
    return list;
  }
  return [...missingSelected, ...list];
});

const activeTagFilterLabel = computed(() => {
  if (selectedTagIds.value.length === 0) {
    return tagFilterCleared.value ? t("knowledgeBase.tagFilterPlaceholder") : t("knowledgeBase.allTags");
  }
  if (selectedTagIds.value.length === 1) {
    const id = selectedTagIds.value[0];
    return tagMap.value[id]?.name || t("knowledgeBase.allTags");
  }
  return t("knowledgeBase.tagFilterMulti", { count: selectedTagIds.value.length });
});

const activeTagFilterTitle = computed(() => {
  if (selectedTagIds.value.length === 0) {
    return t("knowledgeBase.tagFilterTitle");
  }
  const names = selectedTagIds.value.map((id) => tagMap.value[id]?.name).filter(Boolean);
  return names.length > 0 ? names.join("、") : t("knowledgeBase.tagFilterTitle");
});

const isTagFilterActive = (tagId: string) => selectedTagIds.value.includes(tagId);

const kbInfo = ref<any>(null);
const knowledgeList = ref<Array<{ id: string; name: string; type?: string }>>([]);

const loadKnowledgeInfo = async (kbId: string) => {
  if (!kbId) {
    kbInfo.value = null;
    return;
  }
  try {
    const res: any = await getKnowledgeBaseById(kbId);
    kbInfo.value = res?.data || null;
    return kbInfo.value;
  } catch (error) {
    console.error("Failed to load knowledge base info:", error);
    kbInfo.value = null;
    return null;
  }
};

const loadKnowledgeList = async () => {
  try {
    const res: any = await listKnowledgeBases();
    knowledgeList.value = (res?.data || []).map((item: any) => ({
      id: String(item.id),
      name: item.name,
      type: item.type,
    }));
  } catch (error) {
    console.error("Failed to load knowledge bases:", error);
  }
};

const editorVisible = ref(false);
const editorMode = ref<"create" | "edit">("create");
const currentEntryId = ref<number | null>(null);
const editorForm = reactive<FAQEntryPayload>({
  standard_question: "",
  similar_questions: [],
  negative_questions: [],
  answers: [],
  tag_id: undefined,
});
const savingEntry = ref(false);

// Inline validation for the editor. The TDesign form this replaced declared
// rules for these two fields but no form items bound to them, so its
// validate() resolved true and an empty entry went straight to the API; the
// checks now run for real, with the same messages, under the fields.
const editorErrors = reactive<{ standard_question: string; answers: string }>({
  standard_question: "",
  answers: "",
});

const validateEditor = () => {
  editorErrors.standard_question = editorForm.standard_question.trim()
    ? ""
    : t("knowledgeEditor.messages.nameRequired");
  editorErrors.answers = editorForm.answers.length > 0 ? "" : t("knowledgeEditor.faq.answerRequired");
  return !editorErrors.standard_question && !editorErrors.answers;
};

const clearEditorErrors = () => {
  editorErrors.standard_question = "";
  editorErrors.answers = "";
};

// 输入框状态
const answerInput = ref("");
const similarInput = ref("");
const negativeInput = ref("");

const importVisible = ref(false);

const importModes = computed(() => [
  { value: "append" as const, label: t("knowledgeEditor.faqImport.appendMode") },
  { value: "replace" as const, label: t("knowledgeEditor.faqImport.replaceMode") },
]);
const fileInputRef = ref<HTMLInputElement | null>(null);
const importState = reactive({
  mode: "append" as "append" | "replace",
  file: null as File | null,
  preview: [] as FAQEntryPayload[],
  importing: false,
  taskId: null as string | null,
  taskStatus: null as {
    status: string;
    progress: number;
    total: number;
    processed: number;
    message?: string;
    error?: string;
  } | null,
  pollingInterval: null as ReturnType<typeof setInterval> | null,
});

// FAQ导入结果状态（持久化的）
type FAQImportResultView = {
  total_entries: number;
  success_count: number;
  failed_count: number;
  skipped_count: number;
  partial_failed_count: number;
  merged_count: number;
  added_count: number;
  import_mode: string;
  imported_at: string;
  task_id: string;
  processing_time: number;
  message?: string;
  failed_entries_url?: string;
  success_entries?: Array<{
    index: number;
    seq_id: number;
    tag_id?: number;
    tag_name?: string;
    standard_question: string;
  }>;
  display_status: string;
};

const importResult = ref<FAQImportResultView | null>(null);
const importResultExpanded = ref(false);

const showImportResultBadge = computed(
  () => !!importResult.value && importResult.value.display_status === "open" && !importState.taskId,
);

const isImportInProgress = computed(() => {
  const status = importState.taskStatus?.status;
  return !!importState.taskId && (status === "running" || status === "pending");
});

const importResultSummary = computed(() => {
  const result = importResult.value;
  if (!result) return "";
  if (result.message?.trim()) {
    return result.message.trim();
  }
  const parts: string[] = [];
  parts.push(`${t("faqManager.import.totalData")} ${result.total_entries}`);
  if (result.merged_count > 0) {
    if (result.added_count > 0) {
      parts.push(`${t("faqManager.import.added")} ${result.added_count}`);
    }
    parts.push(`${t("faqManager.import.merged")} ${result.merged_count}`);
  } else if (result.success_count > 0) {
    parts.push(`${t("faqManager.import.success")} ${result.success_count}`);
  }
  if (result.partial_failed_count > 0) {
    parts.push(`${t("faqManager.import.partialFailed")} ${result.partial_failed_count}`);
  }
  if (result.failed_count > 0) {
    parts.push(`${t("faqManager.import.failed")} ${result.failed_count}`);
  }
  if (result.skipped_count > 0) {
    parts.push(`${t("faqManager.import.skipped")} ${result.skipped_count}`);
  }
  return parts.join(" · ");
});

const importProgressTitle = computed(() => {
  const status = importState.taskStatus?.status;
  if (status === "running") return t("faqManager.import.importing");
  if (status === "success") return t("faqManager.import.importDone");
  if (status === "failed") return t("faqManager.import.importFailed");
  return t("faqManager.import.waiting");
});

const importProgressIcon = computed<LucideIcon>(() => {
  const status = importState.taskStatus?.status;
  if (status === "running") return Loader2Icon;
  if (status === "success") return CircleCheckIcon;
  if (status === "failed") return CircleAlertIcon;
  return ClockIcon;
});

const importProgressText = computed(() => {
  const status = importState.taskStatus;
  if (!status) return "";
  if (status.error) return status.error;
  if (status.message?.trim()) return status.message.trim();
  return importProgressTitle.value;
});

// Search test state
const searchDrawerVisible = ref(false);
const searching = ref(false);
const hasSearched = ref(false);
const searchResults = ref<FAQEntry[]>([]);
const searchForm = reactive({
  query: "",
  vectorThreshold: 0.7,
  matchCount: 10,
});

const getTagName = (tagId?: number) => {
  if (!tagId) return t("knowledgeBase.untagged");
  return tagMapBySeqId.value[tagId]?.name || t("knowledgeBase.untagged");
};

const handleTagFilterChange = (tagIds: string[]) => {
  selectedTagIds.value = tagIds;
  uiStore.clearSelectedTagIds();
  tagIds.forEach((id) => uiStore.toggleSelectedTagId(id));
};

const handleTagRowClick = (tagId: string) => {
  const next = new Set(selectedTagIds.value);
  if (next.has(tagId)) {
    next.delete(tagId);
  } else {
    next.add(tagId);
  }
  if (next.size > 0) {
    tagFilterCleared.value = false;
  }
  handleTagFilterChange([...next]);
};

const clearTagFilter = () => {
  tagFilterCleared.value = true;
  handleTagFilterChange([]);
};

const openTagManageDrawer = () => {
  tagFilterPanelVisible.value = false;
  tagManageDrawerVisible.value = true;
};

const onTagManageChanged = (payload?: { deletedTagId?: string }) => {
  if (!props.kbId) return;
  void loadTags(true);
  if (payload?.deletedTagId && selectedTagIds.value.includes(payload.deletedTagId)) {
    selectedTagIds.value = selectedTagIds.value.filter((id) => id !== payload.deletedTagId);
    handleTagFilterChange([...selectedTagIds.value]);
  }
  currentPage = 1;
  entries.value = [];
  selectedRowKeys.value = [];
  void loadEntries();
};

const loadTags = async (reset = false) => {
  if (!props.kbId) {
    tagList.value = [];
    tagTotal.value = 0;
    tagHasMore.value = false;
    tagPage.value = 1;
    return;
  }

  if (reset) {
    tagPage.value = 1;
    tagList.value = [];
    tagTotal.value = 0;
    tagHasMore.value = false;
  } else if (tagLoading.value || tagLoadingMore.value) {
    return;
  }

  const currentTagPage = tagPage.value || 1;
  tagLoading.value = currentTagPage === 1;
  tagLoadingMore.value = currentTagPage > 1;

  try {
    const res: any = await listKnowledgeTags(props.kbId, {
      page: currentTagPage,
      page_size: TAG_PAGE_SIZE,
      keyword: tagSearchQuery.value || undefined,
    });
    const pageData = (res?.data || {}) as {
      data?: any[];
      total?: number;
    };
    const pageTags = (pageData.data || []).map((tag: any) => ({
      ...tag,
      id: String(tag.id),
    }));

    if (currentTagPage === 1) {
      tagList.value = pageTags;
    } else {
      tagList.value = [...tagList.value, ...pageTags];
    }

    tagTotal.value = pageData.total || tagList.value.length;
    tagHasMore.value = tagList.value.length < tagTotal.value;
    if (tagHasMore.value) {
      tagPage.value = currentTagPage + 1;
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.operationFailed"));
  } finally {
    tagLoading.value = false;
    tagLoadingMore.value = false;
  }
};

const handleEntryTagChange = async (entryId: number, value?: string) => {
  if (!props.kbId) return;
  const targetEntry = entries.value.find((item) => item.id === entryId);
  const previousTagId = targetEntry ? targetEntry.tag_id : undefined;
  const normalizedValue = value ? Number(value) : null;
  if (normalizedValue === previousTagId) {
    return;
  }
  try {
    await updateFAQEntryTagBatch(props.kbId, { updates: { [entryId]: normalizedValue } });
    MessagePlugin.success(t("knowledgeEditor.messages.updateSuccess"));
    await loadEntries();
    await loadTags(true);
  } catch (error: any) {
    if (targetEntry) {
      targetEntry.tag_id = previousTagId;
    }
    MessagePlugin.error(error?.message || t("common.operationFailed"));
  }
};

const handleNavigateToKbList = () => {
  router.push("/platform/knowledge-bases");
};

const handleNavigateToCurrentKB = () => {
  if (!props.kbId) return;
  router.push(`/platform/knowledge-bases/${props.kbId}`);
};

const handleOpenKBSettings = () => {
  if (!props.kbId) {
    MessagePlugin.warning(t("knowledgeEditor.messages.missingId"));
    return;
  }
  uiStore.openKBSettings(props.kbId);
};

const handleKnowledgeDropdownSelect = (data: { value: string }) => {
  if (!data?.value || data.value === props.kbId) return;
  router.push(`/platform/knowledge-bases/${data.value}`);
};

const handleEntryStatusChange = async (entry: FAQEntry, value: boolean) => {
  if (!props.kbId) {
    return;
  }
  const entryIndex = entries.value.findIndex((e) => e.id === entry.id);
  if (entryIndex === -1) {
    return;
  }
  // 从数组中获取实际的对象引用，确保使用最新的数据
  const actualEntry = entries.value[entryIndex];
  const previous = actualEntry.is_enabled;
  if (previous === value) {
    return;
  }
  // 直接更新属性，Vue 3 的响应式系统应该能够检测到
  actualEntry.is_enabled = value;
  entryStatusLoading[entry.id] = true;
  try {
    await updateFAQEntryFieldsBatch(props.kbId, { by_id: { [entry.id]: { is_enabled: value } } });
    MessagePlugin.success(
      t(value ? "knowledgeEditor.faq.statusEnableSuccess" : "knowledgeEditor.faq.statusDisableSuccess"),
    );
  } catch (error: any) {
    // 失败时回滚
    actualEntry.is_enabled = previous;
    MessagePlugin.error(error?.message || t("knowledgeEditor.faq.statusUpdateFailed"));
  } finally {
    entryStatusLoading[entry.id] = false;
  }
};

const loadEntries = async (append = false) => {
  if (!props.kbId) return;
  if (append) {
    loadingMore.value = true;
  } else {
    loading.value = true;
    currentPage = 1;
    entries.value = [];
    selectedRowKeys.value = [];
    Object.keys(entryStatusLoading).forEach((key) => {
      delete entryStatusLoading[Number(key)];
    });
  }

  try {
    // If overallFAQTotal is not initialized, fetch it first (without a tag filter)
    if (overallFAQTotal.value === 0 && !append) {
      const totalRes = await listFAQEntries(props.kbId, {
        page: 1,
        page_size: 1,
      });
      const totalData = (totalRes.data || {}) as { total: number };
      overallFAQTotal.value = totalData.total || 0;
    }

    const res = await listFAQEntries(props.kbId, {
      page: currentPage,
      page_size: pageSize,
      tag_ids: selectedTagIds.value.length > 0 ? selectedTagIds.value.join(",") : undefined,
      keyword: entrySearchKeyword.value ? entrySearchKeyword.value.trim() : undefined,
    });
    const pageData = (res.data || {}) as {
      data: FAQEntry[];
      total: number;
    };
    const newEntries = (pageData.data || []).map((entry) => ({
      ...entry,
      showMore: false,
      similarCollapsed: true, // 相似问默认折叠
      negativeCollapsed: true, // 反例默认折叠
      answersCollapsed: true, // 答案默认折叠
      is_enabled: entry.is_enabled !== false,
    }));

    if (append) {
      entries.value = [...entries.value, ...newEntries];
    } else {
      entries.value = newEntries;
    }
    // 判断是否还有更多数据
    hasMore.value = entries.value.length < (pageData.total || 0);
    currentPage++;

    // 等待 DOM 更新后重新布局
    await nextTick();
    arrangeCards();
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.operationFailed"));
  } finally {
    loading.value = false;
    loadingMore.value = false;

    // 检查是否需要继续加载以填满可视区域
    // 延迟执行以确保 arrangeCards 的 requestAnimationFrame 完成
    setTimeout(() => {
      checkAndLoadMore();
    }, 350);
  }
};

const handleScroll = () => {
  if (!scrollContainer.value || loadingMore.value || !hasMore.value) return;

  const container = scrollContainer.value;
  const scrollTop = container.scrollTop;
  const scrollHeight = container.scrollHeight;
  const clientHeight = container.clientHeight;

  // 当滚动到距离底部 200px 时加载更多
  if (scrollTop + clientHeight >= scrollHeight - 200) {
    loadEntries(true);
  }
};

// 检查内容是否填满可视区域，如果没有且还有更多数据，继续加载
const checkAndLoadMore = () => {
  if (!scrollContainer.value) return;
  if (loadingMore.value || loading.value) return;
  if (!hasMore.value) return;

  const container = scrollContainer.value;
  const scrollHeight = container.scrollHeight;
  const clientHeight = container.clientHeight;

  // 如果内容高度小于容器高度 + 50px 的缓冲，说明可能没有滚动条或接近底部，需要继续加载
  if (scrollHeight <= clientHeight + 50) {
    loadEntries(true);
  }
};

const handleCardSelect = (entryId: number, checked: boolean) => {
  if (!canSelectEntries.value || batchActionLoading.value) return;
  if (checked) {
    if (!selectedRowKeys.value.includes(entryId)) {
      selectedRowKeys.value.push(entryId);
    }
  } else {
    const index = selectedRowKeys.value.indexOf(entryId);
    if (index > -1) {
      selectedRowKeys.value.splice(index, 1);
    }
  }
};

const clearFAQSelection = () => {
  if (batchActionLoading.value) return;
  selectedRowKeys.value = [];
};

const resetEditorForm = () => {
  editorForm.standard_question = "";
  editorForm.similar_questions = [];
  editorForm.negative_questions = [];
  editorForm.answers = [];
  editorForm.tag_id = undefined;
  answerInput.value = "";
  similarInput.value = "";
  negativeInput.value = "";
};

const openEditor = (entry?: FAQEntry) => {
  clearEditorErrors();
  if (entry) {
    editorMode.value = "edit";
    currentEntryId.value = entry.id;
    editorForm.standard_question = entry.standard_question;
    editorForm.similar_questions = [...(entry.similar_questions || [])];
    editorForm.negative_questions = [...(entry.negative_questions || [])];
    editorForm.answers = [...(entry.answers || [])];
    editorForm.tag_id = entry.tag_id || undefined;
  } else {
    editorMode.value = "create";
    currentEntryId.value = null;
    resetEditorForm();
  }
  answerInput.value = "";
  similarInput.value = "";
  negativeInput.value = "";
  editorVisible.value = true;
};

const handleEditorClose = () => {
  // 关闭时重置表单
  resetEditorForm();
  answerInput.value = "";
  similarInput.value = "";
  negativeInput.value = "";
  clearEditorErrors();
};

// Every user-initiated close (the X, Cancel, Escape, a click outside) resets
// the form, as the old drawer's close event did. A close after a successful
// save sets editorVisible directly and does not pass through here; the next
// openEditor() resets the form anyway.
const handleEditorOpenChange = (open: boolean) => {
  if (open) return;
  editorVisible.value = false;
  handleEditorClose();
};

// A field that becomes valid drops its message at once, rather than waiting
// for the next submit, which is how the old form behaved on change.
watch(
  () => editorForm.standard_question,
  (value) => {
    if (editorErrors.standard_question && value.trim()) editorErrors.standard_question = "";
  },
);
watch(
  () => editorForm.answers.length,
  (count) => {
    if (editorErrors.answers && count > 0) editorErrors.answers = "";
  },
);

// 添加答案
const addAnswer = () => {
  const trimmed = answerInput.value.trim();
  if (trimmed && editorForm.answers.length < 5 && !editorForm.answers.includes(trimmed)) {
    editorForm.answers.push(trimmed);
    answerInput.value = "";
  }
};

// 删除答案
const removeAnswer = (index: number) => {
  editorForm.answers.splice(index, 1);
};

// 添加相似问
const addSimilar = () => {
  const trimmed = similarInput.value.trim();
  if (trimmed && editorForm.similar_questions.length < 10 && !editorForm.similar_questions.includes(trimmed)) {
    editorForm.similar_questions.push(trimmed);
    similarInput.value = "";
  }
};

// 删除相似问
const removeSimilar = (index: number) => {
  editorForm.similar_questions.splice(index, 1);
};

// 添加反例
const addNegative = () => {
  const trimmed = negativeInput.value.trim();
  if (trimmed && editorForm.negative_questions.length < 10 && !editorForm.negative_questions.includes(trimmed)) {
    editorForm.negative_questions.push(trimmed);
    negativeInput.value = "";
  }
};

// 删除反例
const removeNegative = (index: number) => {
  editorForm.negative_questions.splice(index, 1);
};

const handleSubmitEntry = async () => {
  if (!validateEditor()) return;

  savingEntry.value = true;
  try {
    const payload: FAQEntryPayload = {
      standard_question: editorForm.standard_question,
      similar_questions: [...editorForm.similar_questions],
      negative_questions: [...editorForm.negative_questions],
      answers: [...editorForm.answers],
      tag_id: editorForm.tag_id || undefined,
    };
    if (editorMode.value === "create") {
      await createFAQEntry(props.kbId, payload);
      MessagePlugin.success(t("knowledgeEditor.messages.createSuccess"));
    } else if (currentEntryId.value) {
      await updateFAQEntry(props.kbId, currentEntryId.value, payload);
      MessagePlugin.success(t("knowledgeEditor.messages.updateSuccess"));
    }
    editorVisible.value = false;
    await loadEntries();
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.operationFailed"));
  } finally {
    savingEntry.value = false;
  }
};

const handleBatchDelete = async () => {
  if (!canManage.value || !selectedRowKeys.value.length || !props.kbId || batchActionLoading.value) return;
  const selectedIds = [...selectedRowKeys.value];
  batchDeleteLoading.value = true;
  try {
    await deleteFAQEntries(props.kbId, selectedIds);
    MessagePlugin.success(t("knowledgeEditor.faq.batchDeleteSuccess", { count: selectedIds.length }));
    selectedRowKeys.value = [];
    await loadEntries();
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.operationFailed"));
  } finally {
    batchDeleteLoading.value = false;
  }
};

// 批量状态更新对话框
const batchTagDialogVisible = ref(false);
const batchTagValue = ref<string>("");

const openBatchTagDialog = () => {
  if (!canEdit.value || !selectedRowKeys.value.length || batchActionLoading.value) return;
  batchTagValue.value = "";
  batchTagQuery.value = "";
  batchTagDialogVisible.value = true;
};

// The tag picker in the batch dialog: a popover with a filter box standing in
// for the filterable select it replaced.
const batchTagPickerOpen = ref(false);
const batchTagQuery = ref("");
const filteredBatchTagOptions = computed(() => {
  const query = batchTagQuery.value.trim().toLowerCase();
  if (!query) return tagSelectOptions.value;
  return tagSelectOptions.value.filter((option) => option.label.toLowerCase().includes(query));
});
const batchTagLabel = computed(
  () => tagSelectOptions.value.find((option) => option.value === batchTagValue.value)?.label ?? "",
);
const selectBatchTag = (value: string) => {
  batchTagValue.value = value;
  batchTagPickerOpen.value = false;
  batchTagQuery.value = "";
};

const handleBatchTag = async () => {
  if (!canEdit.value || !selectedRowKeys.value.length || !props.kbId || batchActionLoading.value) return;
  const selectedIds = [...selectedRowKeys.value];
  batchTagLoading.value = true;
  try {
    const updates: Record<number, number | null> = {};
    selectedIds.forEach((id) => {
      updates[id] = batchTagValue.value ? Number(batchTagValue.value) : null;
    });
    await updateFAQEntryTagBatch(props.kbId, { updates });
    MessagePlugin.success(t("knowledgeEditor.messages.updateSuccess"));
    batchTagDialogVisible.value = false;
    selectedRowKeys.value = [];
    await loadEntries();
    await loadTags(true);
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.operationFailed"));
  } finally {
    batchTagLoading.value = false;
  }
};

const handleBatchStatusChange = async (isEnabled: boolean) => {
  if (!canEdit.value || !selectedRowKeys.value.length || !props.kbId || batchActionLoading.value) return;
  const selectedIds = [...selectedRowKeys.value];
  batchStatusAction.value = isEnabled ? "enable" : "disable";
  try {
    const by_id: Record<number, { is_enabled: boolean }> = {};
    selectedIds.forEach((id) => {
      by_id[id] = { is_enabled: isEnabled };
    });
    await updateFAQEntryFieldsBatch(props.kbId, { by_id });
    MessagePlugin.success(
      t(isEnabled ? "knowledgeEditor.faq.statusEnableSuccess" : "knowledgeEditor.faq.statusDisableSuccess"),
    );
    selectedRowKeys.value = [];
    await loadEntries();
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.operationFailed"));
  } finally {
    batchStatusAction.value = null;
  }
};

const handleMenuEdit = (entry: FAQEntry) => {
  entry.showMore = false;
  openEditor(entry);
};

const handleMenuDelete = async (entry: FAQEntry) => {
  entry.showMore = false;
  try {
    await deleteFAQEntries(props.kbId, [entry.id]);
    MessagePlugin.success(t("knowledgeEditor.faqImport.deleteSuccess"));
    await loadEntries();
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.operationFailed"));
  }
};

const openImportDialog = () => {
  // 如果正在导入，不允许打开导入对话框
  if (importState.taskStatus?.status === "running") {
    MessagePlugin.warning(t("faqManager.import.importInProgress"));
    return;
  }
  stopPolling();
  importVisible.value = true;
  importState.file = null;
  importState.preview = [];
  importState.mode = "append";
  // 注意：不清除taskId和taskStatus，以便在关闭对话框后仍能看到进度
  importState.importing = false;
};

const processFile = async (file: File) => {
  importState.file = file;

  try {
    let parsed: FAQEntryPayload[] = [];
    if (file.name.endsWith(".json")) {
      parsed = await parseJSONFile(file);
    } else if (file.name.endsWith(".csv")) {
      parsed = await parseCSVFile(file);
    } else if (file.name.endsWith(".xlsx") || file.name.endsWith(".xls")) {
      parsed = await parseExcelFile(file);
    } else {
      MessagePlugin.warning(t("knowledgeEditor.faqImport.unsupportedFormat"));
      importState.preview = [];
      return;
    }
    importState.preview = parsed;
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("knowledgeEditor.faqImport.parseFailed"));
    importState.preview = [];
  }
};

const handleFileChange = async (event: Event) => {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0];
  if (!file) return;
  await processFile(file);
};

const handleFileDrop = async (event: DragEvent) => {
  const file = event.dataTransfer?.files[0];
  if (!file) return;
  await processFile(file);
};

const parseJSONFile = async (file: File): Promise<FAQEntryPayload[]> => {
  const text = await file.text();
  const data = JSON.parse(text);
  if (!Array.isArray(data)) {
    throw new Error(t("knowledgeEditor.faqImport.invalidJSON"));
  }
  return data.map(normalizePayload);
};

const parseCSVFile = async (file: File): Promise<FAQEntryPayload[]> => {
  const text = await file.text();

  // 使用 papaparse 解析 CSV，自动处理引号、转义、分隔符等
  return new Promise((resolve, reject) => {
    Papa.parse(text, {
      header: true,
      skipEmptyLines: true,
      delimiter: "", // 自动检测分隔符（逗号或制表符）
      quoteChar: '"',
      escapeChar: '"',
      transformHeader: (header: string) => {
        // 移除字段名中的括号和说明，只保留核心字段名
        const cleaned = header
          .trim()
          .replace(/\([^)]*\)/g, "") // 移除括号及内容
          .trim();
        // 对于中文字段名，不转换为小写；对于英文字段名，转换为小写
        return /[\u4e00-\u9fa5]/.test(cleaned) ? cleaned : cleaned.toLowerCase();
      },
      complete: (results) => {
        try {
          const payloads: FAQEntryPayload[] = [];
          results.data.forEach((row: any) => {
            const record: Record<string, string> = {};
            // 将行数据转换为记录对象
            Object.keys(row).forEach((key) => {
              record[key] = String(row[key] || "").trim();
            });

            const isDisabled = parseBooleanField(record["是否停用"], false);
            payloads.push(
              normalizePayload({
                standard_question: record["问题"] || record["standard_question"] || record["question"] || "",
                answers: splitByDelimiter(record["机器人回答"] || record["answers"]),
                similar_questions: splitByDelimiter(record["相似问题"] || record["similar_questions"]),
                negative_questions: splitByDelimiter(record["反例问题"] || record["negative_questions"]),
                tag_id: record["tag_id"] ? Number(record["tag_id"]) : undefined,
                tag_name: record["标签"] || record["分类"] || record["tag_name"] || "",
                is_enabled: isDisabled !== undefined ? !isDisabled : undefined, // 是否停用：FALSE表示启用，TRUE表示停用，所以取反
              }),
            );
          });
          resolve(payloads);
        } catch (error) {
          reject(error);
        }
      },
      error: (error: Error) => {
        reject(new Error(`CSV parse failed: ${error.message}`));
      },
    });
  });
};

const parseExcelFile = async (file: File): Promise<FAQEntryPayload[]> => {
  const data = await file.arrayBuffer();
  const workbook = XLSX.read(data, { type: "array" });
  const sheetName = workbook.SheetNames[0];
  const worksheet = workbook.Sheets[sheetName];
  // 使用 raw: false 确保正确处理引号和转义
  const json = XLSX.utils.sheet_to_json<Record<string, string>>(worksheet, {
    defval: "",
    raw: false, // 确保字符串值被正确解析
  });
  return json.map((row) => {
    // 获取原始表头（去除括号说明）
    const normalizedRow: Record<string, string> = {};
    Object.keys(row).forEach((key) => {
      const normalizedKey = key
        .trim()
        .replace(/\([^)]*\)/g, "") // 移除括号及内容
        .trim();
      // 对于中文字段名，不转换为小写；对于英文字段名，转换为小写
      const finalKey = /[\u4e00-\u9fa5]/.test(normalizedKey) ? normalizedKey : normalizedKey.toLowerCase();
      // 确保值是字符串类型
      normalizedRow[finalKey] = String(row[key] || "").trim();
    });

    const isDisabled = parseBooleanField(normalizedRow["是否停用"], false);
    return normalizePayload({
      standard_question: normalizedRow["问题"] || normalizedRow["standard_question"] || normalizedRow["question"] || "",
      answers: splitByDelimiter(normalizedRow["机器人回答"] || normalizedRow["answers"]),
      similar_questions: splitByDelimiter(normalizedRow["相似问题"] || normalizedRow["similar_questions"]),
      negative_questions: splitByDelimiter(normalizedRow["反例问题"] || normalizedRow["negative_questions"]),
      tag_id: normalizedRow["tag_id"] ? Number(normalizedRow["tag_id"]) : undefined,
      tag_name: normalizedRow["标签"] || normalizedRow["分类"] || normalizedRow["tag_name"] || "",
      is_enabled: isDisabled !== undefined ? !isDisabled : undefined, // 是否停用：FALSE表示启用，TRUE表示停用，所以取反
    });
  });
};

const splitByDelimiter = (value?: string) => {
  if (!value) return [];
  // 只使用 ## 作为分隔符，避免错误分割包含逗号、分号等内容
  const trimmedValue = value.trim();
  if (!trimmedValue) return [];

  // 如果包含 ## 分隔符，按 ## 分割
  if (trimmedValue.includes("##")) {
    return trimmedValue
      .split("##")
      .map((item) => item.trim())
      .filter(Boolean);
  }

  // 如果没有 ## 分隔符，整个值作为一个答案
  return [trimmedValue];
};

// 解析布尔字段（支持多种格式：TRUE/FALSE, true/false, 是/否, 1/0等）
const parseBooleanField = (value?: string, defaultValue: boolean = true): boolean | undefined => {
  if (!value) return undefined;
  const normalized = value.trim().toUpperCase();
  if (normalized === "TRUE" || normalized === "1" || normalized === "是" || normalized === "YES") {
    return true;
  }
  if (normalized === "FALSE" || normalized === "0" || normalized === "否" || normalized === "NO") {
    return false;
  }
  return defaultValue;
};

const normalizePayload = (payload: Partial<FAQEntryPayload>): FAQEntryPayload => ({
  standard_question: payload.standard_question || "",
  answers: payload.answers?.filter(Boolean) || [],
  similar_questions: payload.similar_questions?.filter(Boolean) || [],
  negative_questions: payload.negative_questions?.filter(Boolean) || [],
  tag_id: payload.tag_id || undefined,
  tag_name: payload.tag_name || "",
  is_enabled: payload.is_enabled !== undefined ? payload.is_enabled : undefined,
});

const stopPolling = () => {
  if (importState.pollingInterval) {
    clearInterval(importState.pollingInterval);
    importState.pollingInterval = null;
  }
};

const startPolling = (taskId: string) => {
  stopPolling();
  // 保存taskId到localStorage，以便刷新后恢复
  saveTaskIdToStorage(taskId);

  // 记录上次已处理数量，用于判断是否需要刷新列表
  let lastProcessed = 0;

  importState.pollingInterval = setInterval(async () => {
    try {
      const res: any = await getFAQImportProgress(taskId);
      const progressData = res?.data;
      if (progressData) {
        // 从Redis进度数据中提取状态
        // status: "pending" -> "pending", "processing" -> "running", "completed" -> "success", "failed" -> "failed"
        let status = progressData.status;
        if (status === "processing") {
          status = "running";
        } else if (status === "completed") {
          status = "success";
        }

        const progress = progressData.progress || 0;
        const total = progressData.total || 0;
        const processed = progressData.processed || 0;
        const error = progressData.error || "";
        const message = progressData.message || "";

        importState.taskStatus = {
          status: status,
          progress: progress,
          total: total,
          processed: processed,
          message: message,
          error: error,
        };

        // 进度更新时刷新FAQ列表（每增加一些条目就刷新一次）
        if (processed > lastProcessed) {
          lastProcessed = processed;
          await loadEntries();
          await loadTags(true);
        }

        // 任务完成或失败，停止轮询（但不自动关闭进度条，让用户手动关闭）
        if (status === "success" || status === "failed") {
          stopPolling();
          if (status === "success") {
            // 保存已完成的 taskId 用于后续加载结果
            if (importState.taskId) {
              saveLastCompletedTaskId(importState.taskId);
            }
            MessagePlugin.success(progressData.message || t("knowledgeEditor.faqImport.importSuccess"));
            // 清除筛选条件，确保用户能看到所有新导入的数据
            selectedTagIds.value = [];
            tagFilterCleared.value = false;
            uiStore.clearSelectedTagIds();
            entrySearchKeyword.value = "";
            overallFAQTotal.value = 0; // Reset to trigger re-fetch
            await loadEntries();
            await loadTags(true);
            await loadImportResult(); // 加载最新的导入结果统计
            // 任务完成后，3秒后自动关闭进度条
            setTimeout(() => {
              if (importState.taskStatus?.status === "success") {
                handleCloseProgress();
              }
            }, 3000);
          } else {
            MessagePlugin.error(error || t("common.operationFailed"));
            // 失败时不自动关闭，让用户看到错误信息
          }
        }
      }
    } catch (error: any) {
      console.error("Failed to poll task status:", error);
      // 如果任务不存在或已过期，清除存储
      if (error?.response?.status === 404 || error?.message?.includes("not found")) {
        clearTaskIdFromStorage();
        stopPolling();
        importState.taskId = null;
        importState.taskStatus = null;
      }
    }
  }, 3000); // 每3秒轮询一次
};

const handleCancelImport = () => {
  stopPolling();
  importState.importing = false;
  importState.taskId = null;
  importState.taskStatus = null;
  importVisible.value = false;
  // 注意：不清除localStorage，因为任务可能还在进行中
};

const handleCloseProgress = () => {
  stopPolling();
  importState.taskId = null;
  importState.taskStatus = null;
  clearTaskIdFromStorage();
};

// localStorage相关函数
const getStorageKey = () => {
  return `faq_import_task_${props.kbId}`;
};

const saveTaskIdToStorage = (taskId: string) => {
  if (!props.kbId) return;
  try {
    localStorage.setItem(getStorageKey(), taskId);
  } catch (error) {
    console.error("Failed to save taskId to localStorage:", error);
  }
};

const getTaskIdFromStorage = (): string | null => {
  if (!props.kbId) return null;
  try {
    return localStorage.getItem(getStorageKey());
  } catch (error) {
    console.error("Failed to get taskId from localStorage:", error);
    return null;
  }
};

const clearTaskIdFromStorage = () => {
  if (!props.kbId) return;
  try {
    localStorage.removeItem(getStorageKey());
  } catch (error) {
    console.error("Failed to clear taskId from localStorage:", error);
  }
};

// 恢复导入任务状态（用于刷新后恢复）
const restoreImportTask = async () => {
  if (!props.kbId) return;

  const savedTaskId = getTaskIdFromStorage();
  if (!savedTaskId) return;

  try {
    // 查询Redis中的进度状态
    const res: any = await getFAQImportProgress(savedTaskId);
    const progressData = res?.data;

    if (progressData) {
      // 从Redis进度数据中提取状态
      let status = progressData.status;
      if (status === "processing") {
        status = "running";
      } else if (status === "completed") {
        status = "success";
      }

      const progress = progressData.progress || 0;
      const total = progressData.total || 0;
      const processed = progressData.processed || 0;
      const error = progressData.error || "";

      importState.taskId = savedTaskId;
      importState.taskStatus = {
        status: status,
        progress: progress,
        total: total,
        processed: processed,
        message: progressData.message || "",
        error: error,
      };

      // 如果任务还在进行中，恢复轮询
      if (status === "pending" || status === "running") {
        startPolling(savedTaskId);
      } else {
        // 任务已完成或失败，清除存储
        clearTaskIdFromStorage();
      }
    } else {
      // 任务不存在，清除存储
      clearTaskIdFromStorage();
    }
  } catch (error: any) {
    console.error("Failed to restore import task:", error);
    // 如果任务不存在或已过期，清除存储
    if (error?.response?.status === 404 || error?.message?.includes("not found")) {
      clearTaskIdFromStorage();
    }
  }
};

// localStorage key for last completed task
const getLastCompletedTaskKey = () => {
  return `faq_import_last_completed_${props.kbId}`;
};

const saveLastCompletedTaskId = (taskId: string) => {
  if (!props.kbId) return;
  try {
    localStorage.setItem(getLastCompletedTaskKey(), taskId);
  } catch (error) {
    console.error("Failed to save last completed taskId:", error);
  }
};

const getLastCompletedTaskId = (): string | null => {
  if (!props.kbId) return null;
  try {
    return localStorage.getItem(getLastCompletedTaskKey());
  } catch (error) {
    return null;
  }
};

// 加载持久化的导入结果统计
const loadImportResult = async () => {
  if (!props.kbId) return;

  const lastTaskId = getLastCompletedTaskId();
  if (!lastTaskId) {
    importResult.value = null;
    return;
  }

  try {
    const res: any = await getFAQImportProgress(lastTaskId);
    const data = res?.data;
    if (data && data.status === "completed") {
      // 检查后端返回的 display_status，如果是 close 则不显示
      if (data.display_status === "close") {
        importResult.value = null;
        return;
      }
      // Map progress fields to importResult format
      importResult.value = {
        total_entries: data.total,
        success_count: data.success_count || 0,
        failed_count: data.failed_count || 0,
        skipped_count: data.skipped_count || 0,
        partial_failed_count: data.partial_failed_count || 0,
        merged_count: data.merged_count || 0,
        added_count: data.added_count || 0,
        message: data.message || "",
        import_mode: data.import_mode || "append",
        imported_at: data.imported_at,
        task_id: data.task_id,
        failed_entries_url: data.failed_entries_url,
        success_entries: data.success_entries,
        display_status: data.display_status || "open",
        processing_time: data.processing_time || 0,
      };
    } else {
      importResult.value = null;
    }
  } catch (error) {
    console.error("Failed to load FAQ import result:", error);
    importResult.value = null;
  }
};

// 关闭导入结果统计卡片
const closeImportResult = async () => {
  importResultExpanded.value = false;
  if (!props.kbId) return;
  try {
    await updateFAQImportResultDisplayStatus(props.kbId, "close");
    if (importResult.value) {
      importResult.value.display_status = "close";
    }
  } catch (error) {
    console.error("Failed to close import result:", error);
  }
};

// 下载失败条目原因
const downloadFailedEntries = () => {
  if (!importResult.value?.failed_entries_url) {
    MessagePlugin.warning(t("faqManager.import.noFailedRecords"));
    return;
  }
  // 直接打开下载链接
  window.open(importResult.value.failed_entries_url, "_blank");
};

// 格式化导入时间
const formatImportTime = (timeStr?: string) => {
  if (!timeStr) return "";
  try {
    const date = new Date(timeStr);
    return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")} ${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
  } catch (e) {
    return timeStr;
  }
};

const handleImport = async () => {
  if (!importState.file || !importState.preview.length) {
    MessagePlugin.warning(t("knowledgeEditor.faqImport.selectFile"));
    return;
  }

  // 如果任务已完成或失败，关闭对话框
  if (importState.taskStatus?.status === "success" || importState.taskStatus?.status === "failed") {
    if (importState.taskStatus.status === "success") {
      handleCancelImport();
    } else {
      // 失败时重试
      importState.taskId = null;
      importState.taskStatus = null;
      importState.importing = false;
    }
    return;
  }

  importState.importing = true;
  try {
    const res: any = await upsertFAQEntries(props.kbId, {
      entries: importState.preview,
      mode: importState.mode,
    });

    // The import always runs as a background task; its progress shows on
    // top of the entry list once the dialog closes.
    const taskId: string = res.data.task_id;
    importState.taskId = taskId;
    importState.taskStatus = {
      status: "pending",
      progress: 0,
      total: importState.preview.length,
      processed: 0,
      message: t("faqManager.import.progressHint"),
    };
    startPolling(taskId);
    importVisible.value = false;
    // Clear the dialog's input but keep taskId / taskStatus for the progress display.
    importState.file = null;
    importState.preview = [];
    importState.importing = false;
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.operationFailed"));
    importState.importing = false;
    stopPolling();
  }
};

// 组件卸载时清理轮询
onUnmounted(() => {
  stopPolling();
});

// 下载示例文件选项
const downloadExampleOptions = computed(() => [
  { content: t("knowledgeEditor.faqImport.downloadExampleJSON"), value: "json" },
  { content: t("knowledgeEditor.faqImport.downloadExampleCSV"), value: "csv" },
  { content: t("knowledgeEditor.faqImport.downloadExampleExcel"), value: "excel" },
]);

// 示例数据
const exampleData: FAQEntryPayload[] = [
  {
    standard_question: "什么是 Yuheng 知识库？",
    answers: ["Yuheng 知识库是一个智能知识库管理系统", "它支持多种知识库类型和导入方式"],
    similar_questions: ["知识库平台是什么？", "介绍一下 Yuheng 知识库"],
    negative_questions: ["这不是知识库平台", "与知识库平台无关"],
    tag_name: "产品介绍",
  },
  {
    standard_question: "如何创建知识库？",
    answers: ['点击"新建知识库"按钮', "选择知识库类型并填写相关信息", "完成创建后即可开始使用"],
    similar_questions: ["怎么创建知识库？", "如何新建知识库？"],
    negative_questions: [],
    tag_name: "使用指南",
  },
];

// 下载示例文件
const handleDownloadExample = (data: { value: string }) => {
  const { value } = data;
  switch (value) {
    case "json":
      downloadJSONExample();
      break;
    case "csv":
      downloadCSVExample();
      break;
    case "excel":
      downloadExcelExample();
      break;
  }
};

// 下载 JSON 示例
const downloadJSONExample = () => {
  const jsonStr = JSON.stringify(exampleData, null, 2);
  const blob = new Blob([jsonStr], { type: "application/json;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = "faq_example.json";
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
};

// 下载 CSV 示例
const downloadCSVExample = () => {
  const headers = [
    "标签(必填)",
    "问题(必填)",
    "相似问题(选填-多个用##分隔)",
    "反例问题(选填-多个用##分隔)",
    "机器人回答(必填-多个用##分隔)",
    "是否全部回复(选填-默认FALSE)",
    "是否停用(选填-默认FALSE)",
    "是否禁止被推荐(选填-默认False 可被推荐)",
  ];
  const rows = exampleData.map((item) => {
    return [
      item.tag_name || "", // 标签
      item.standard_question,
      item.similar_questions.join("##"),
      item.negative_questions.join("##"),
      item.answers.join("##"),
      "FALSE", // 是否全部回复
      "FALSE", // 是否停用
      "FALSE", // 是否禁止被推荐
    ];
  });
  const csvContent = [
    headers.join("\t"), // 使用制表符分隔
    ...rows.map((row) =>
      row
        .map((cell) => {
          // 如果包含制表符、换行符或引号，需要用引号包裹
          if (cell.includes("\t") || cell.includes("\n") || cell.includes('"')) {
            return `"${cell.replace(/"/g, '""')}"`;
          }
          return cell;
        })
        .join("\t"),
    ),
  ].join("\n");
  const blob = new Blob(["\ufeff" + csvContent], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = "faq_example.csv";
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
};

// 下载 Excel 示例
const downloadExcelExample = () => {
  const worksheet = XLSX.utils.json_to_sheet(
    exampleData.map((item) => ({
      "标签(必填)": item.tag_name || "",
      "问题(必填)": item.standard_question,
      "相似问题(选填-多个用##分隔)": item.similar_questions.join("##"),
      "反例问题(选填-多个用##分隔)": item.negative_questions.join("##"),
      "机器人回答(必填-多个用##分隔)": item.answers.join("##"),
      "是否全部回复(选填-默认FALSE)": "FALSE",
      "是否停用(选填-默认FALSE)": "FALSE",
      "是否禁止被推荐(选填-默认False 可被推荐)": "FALSE",
    })),
  );
  const workbook = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(workbook, worksheet, "FAQ");
  XLSX.writeFile(workbook, "faq_example.xlsx");
};

// 导出 FAQ 数据
const exportLoading = ref(false);
const downloadExportBlob = (blob: Blob, ext: "csv" | "json") => {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `faq_export_${new Date().toISOString().slice(0, 10)}.${ext}`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
};
const handleExportFAQ = async (format: "csv" | "json") => {
  if (!props.kbId) {
    MessagePlugin.warning(t("knowledgeBase.selectKnowledgeBase"));
    return;
  }

  exportLoading.value = true;
  try {
    const blob = await exportFAQEntries(props.kbId, format);
    downloadExportBlob(blob, format);
    MessagePlugin.success(t("knowledgeEditor.faqExport.exportSuccess"));
  } catch (error: any) {
    console.error("Export failed:", error);
    MessagePlugin.error(t("knowledgeEditor.faqExport.exportFailed"));
  } finally {
    exportLoading.value = false;
  }
};
const handleExportCSV = () => handleExportFAQ("csv");
const handleExportJSON = () => handleExportFAQ("json");

watch(
  () => props.kbId,
  async (newKbId) => {
    currentPage = 1;
    hasMore.value = true;
    selectedTagIds.value = [];
    tagFilterCleared.value = false;
    uiStore.clearSelectedTagIds();
    overallFAQTotal.value = 0;
    tagSearchQuery.value = "";

    if (!newKbId) {
      kbInfo.value = null;
      // kbId变化时，清除之前的任务状态
      stopPolling();
      importState.taskId = null;
      importState.taskStatus = null;
      clearTaskIdFromStorage();
      return;
    }

    const info = await loadKnowledgeInfo(newKbId);
    if (!info || info.type !== "faq") {
      return;
    }

    loadEntries();
    loadTags(true);
    // 恢复导入任务状态（如果存在）
    await restoreImportTask();
  },
  { immediate: true },
);

watch(selectedTagIds, (newVal, oldVal) => {
  if (oldVal === undefined) return;
  if (newVal.join(",") !== oldVal.join(",")) {
    currentPage = 1;
    entries.value = [];
    selectedRowKeys.value = [];
    loadEntries();
  }
});

watch(tagSearchQuery, (newVal, oldVal) => {
  if (newVal === oldVal) return;
  if (tagSearchDebounce) {
    window.clearTimeout(tagSearchDebounce);
  }
  tagSearchDebounce = window.setTimeout(() => {
    loadTags(true);
  }, 300);
});

// 监听FAQ搜索关键词变化
// The search box's clear button: empty the keyword and reload at once, as the
// old input's clear event did (the debounced watcher below fires as well).
const clearEntrySearch = () => {
  entrySearchKeyword.value = "";
  loadEntries();
};

watch(entrySearchKeyword, (newVal, oldVal) => {
  if (newVal === oldVal) return;
  if (entrySearchDebounce) {
    window.clearTimeout(entrySearchDebounce);
  }
  entrySearchDebounce = window.setTimeout(() => {
    loadEntries();
  }, 300);
});

const handleSearch = async () => {
  if (!searchForm.query.trim()) {
    MessagePlugin.warning(t("knowledgeEditor.faq.queryPlaceholder"));
    return;
  }

  searching.value = true;
  hasSearched.value = true;
  try {
    const res = await searchFAQEntries(props.kbId, {
      query_text: searchForm.query.trim(),
      vector_threshold: searchForm.vectorThreshold,
      match_count: searchForm.matchCount,
    });
    const results = (res.data || []).map((entry: FAQEntry) => ({
      ...entry,
      similarCollapsed: true, // 相似问默认折叠
      negativeCollapsed: true, // 反例默认折叠
      answersCollapsed: true, // 答案默认折叠
      expanded: false,
    })) as FAQEntry[];

    // 按score从大到小排序
    searchResults.value = results.sort((a, b) => (b.score || 0) - (a.score || 0));
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.operationFailed"));
    searchResults.value = [];
  } finally {
    searching.value = false;
  }
};

const toggleResult = (result: FAQEntry) => {
  result.expanded = !result.expanded;
};

// 防抖函数
let arrangeCardsTimer: ReturnType<typeof setTimeout> | null = null;
const debounceArrangeCards = (delay = 100) => {
  if (arrangeCardsTimer) {
    clearTimeout(arrangeCardsTimer);
  }
  arrangeCardsTimer = setTimeout(() => {
    arrangeCards();
    arrangeCardsTimer = null;
  }, delay);
};

// 瀑布流布局函数 - 优化版本，避免闪烁
const arrangeCards = () => {
  if (!cardListRef.value) return;

  const cards = cardListRef.value.querySelectorAll(".faq-card") as NodeListOf<HTMLElement>;
  if (cards.length === 0) return;

  // 获取容器宽度和列数
  const containerWidth = cardListRef.value.offsetWidth;
  const gap = 12; // 与 CSS gap 保持一致
  let columnCount = 1;

  // 根据容器宽度计算列数（增加每行的卡片数量）
  if (containerWidth >= 2560) columnCount = 12;
  else if (containerWidth >= 1920) columnCount = 10;
  else if (containerWidth >= 1536) columnCount = 8;
  else if (containerWidth >= 1280) columnCount = 6;
  else if (containerWidth >= 1024) columnCount = 5;
  else if (containerWidth >= 768) columnCount = 4;
  else if (containerWidth >= 640) columnCount = 3;

  const columnWidth = (containerWidth - gap * (columnCount - 1)) / columnCount;

  // 初始化每列的高度数组
  const columnHeights = new Array(columnCount).fill(0);

  // 使用 requestAnimationFrame 优化性能
  requestAnimationFrame(() => {
    // 先设置宽度，保持当前位置不变
    cards.forEach((card) => {
      // 确保卡片是绝对定位
      if (card.style.position !== "absolute") {
        card.style.position = "absolute";
      }
      // 设置宽度以便正确计算高度
      card.style.width = `${columnWidth}px`;
    });

    // 等待浏览器重新计算布局
    requestAnimationFrame(() => {
      // 计算所有卡片的高度（不改变位置）
      const cardHeights: number[] = [];
      cards.forEach((card) => {
        const height = card.offsetHeight || card.getBoundingClientRect().height;
        cardHeights.push(height);
      });

      // 计算新位置
      const newPositions: Array<{ top: number; left: number }> = [];
      cardHeights.forEach((height) => {
        const shortestColumnIndex = columnHeights.indexOf(Math.min(...columnHeights));
        const top = columnHeights[shortestColumnIndex];
        const left = shortestColumnIndex * (columnWidth + gap);

        newPositions.push({ top, left });
        columnHeights[shortestColumnIndex] += height + gap;
      });

      // 批量更新所有卡片位置，使用CSS过渡实现平滑移动
      cards.forEach((card, index) => {
        const { top, left } = newPositions[index];
        const currentTop = parseFloat(card.style.top) || 0;
        const currentLeft = parseFloat(card.style.left) || 0;

        // 如果位置发生变化，添加过渡效果
        if (Math.abs(currentTop - top) > 1 || Math.abs(currentLeft - left) > 1) {
          // 使用 will-change 提示浏览器优化
          card.style.willChange = "top, left";
          card.style.transition = "top 0.3s cubic-bezier(0.4, 0, 0.2, 1), left 0.3s cubic-bezier(0.4, 0, 0.2, 1)";
        }

        card.style.position = "absolute";
        card.style.top = `${top}px`;
        card.style.left = `${left}px`;
        card.style.width = `${columnWidth}px`;
      });

      // 设置容器高度
      const maxHeight = Math.max(...columnHeights);
      if (cardListRef.value) {
        cardListRef.value.style.height = `${maxHeight}px`;
        cardListRef.value.style.position = "relative";
      }

      // 动画完成后移除过渡和 will-change，避免影响后续交互
      setTimeout(() => {
        cards.forEach((card) => {
          card.style.transition = "";
          card.style.willChange = "";
        });
      }, 300);
    });
  });
};

// 监听窗口大小变化（使用防抖）
let resizeTimer: ReturnType<typeof setTimeout> | null = null;
const handleResize = () => {
  if (resizeTimer) {
    clearTimeout(resizeTimer);
  }
  resizeTimer = setTimeout(() => {
    arrangeCards();
    // 窗口变大时可能需要加载更多，延迟执行确保布局完成
    setTimeout(() => {
      checkAndLoadMore();
    }, 350);
    resizeTimer = null;
  }, 150);
};

onMounted(async () => {
  loadKnowledgeList();
  window.addEventListener("resize", handleResize);
  // 如果已有kbId，恢复导入任务状态
  if (props.kbId) {
    await restoreImportTask();
    await loadImportResult(); // 加载导入结果
  }
});

onUnmounted(() => {
  window.removeEventListener("resize", handleResize);
  if (arrangeCardsTimer) {
    clearTimeout(arrangeCardsTimer);
  }
  if (resizeTimer) {
    clearTimeout(resizeTimer);
  }
});

// 监听 entries 变化，重新布局
watch(
  () => entries.value.length,
  () => {
    nextTick(() => {
      arrangeCards();
    });
  },
);

// 监听折叠状态变化，重新布局（使用防抖和动画完成后的回调）
watch(
  () =>
    entries.value.map((e) => ({
      id: e.id,
      similarCollapsed: e.similarCollapsed,
      negativeCollapsed: e.negativeCollapsed,
      answersCollapsed: e.answersCollapsed,
    })),
  () => {
    // 使用 nextTick 确保 DOM 更新
    nextTick(() => {
      // 等待一个渲染帧，让高度变化生效
      requestAnimationFrame(() => {
        // 再等待一个渲染帧，确保高度计算准确
        requestAnimationFrame(() => {
          // 等待 Transition 动画完成后再布局（slide-down 动画时长约 200ms）
          // 使用防抖避免频繁调用
          debounceArrangeCards(250);
        });
      });
    });
  },
  { deep: true },
);
// ---------------------------------------------------------------------------
// Shared class lists. The template repeats these shapes many times (every card
// section, every editor row); naming them once keeps the rows identical and
// the template readable. They are plain strings, so Tailwind still finds them.
// ---------------------------------------------------------------------------

// The coloured bar before each card section label; its colour comes from the
// section (primary for questions and answers, warning for negatives).
const sectionLabelClass =
  "text-muted-foreground mb-px flex cursor-pointer items-center gap-[5px] rounded-sm py-0.5 text-[11px] font-semibold tracking-[0.5px] uppercase select-none before:h-2.5 before:w-[3px] before:shrink-0 before:rounded-[2px] before:content-[''] hover:-mx-1 hover:bg-card hover:px-1 hover:text-foreground";
const cardTagsClass =
  "flex min-h-[18px] w-full min-w-0 flex-wrap gap-[5px] overflow-hidden [contain:layout_style_paint] *:max-w-full *:min-w-0 *:flex-[0_1_auto]";
const questionTagClass =
  "border-border bg-card text-placeholder box-border inline-flex h-5 max-w-full min-w-0 items-center rounded-[5px] border px-2 text-[11px] align-middle";
const tagChipClass =
  "border-border bg-accent hover:border-primary box-border inline-flex h-5 max-w-[120px] cursor-pointer items-center rounded-sm border px-1.5 text-[11px] font-normal text-[var(--td-text-color-disabled)] transition-all duration-200 hover:bg-[var(--td-success-color-light)] hover:text-[var(--td-brand-color-active)]";

// Editor drawer rows. Vertical padding and the bottom border are set per row
// (the first and last rows differ), so they are not part of the shared list.
const editorRowClass = "flex flex-col items-start justify-between gap-3";
// Standard question, similar, negative and answer rows carry a 3px bar on the
// left; its colour, top and height are set per row.
const stripeClass =
  "relative pl-3 before:absolute before:left-0 before:w-[3px] before:rounded-r-[2px] before:content-['']";
const requiredLabelClass = "text-foreground mb-1 inline-flex items-center gap-1 text-[15px] font-semibold";
const optionalLabelClass = "text-foreground mb-1 block text-[15px] font-medium";
const editorDescClass = "text-muted-foreground m-0 text-[13px] leading-normal";
const editorInputClass =
  "border-border bg-card hover:border-primary focus-visible:border-primary focus-visible:ring-primary/10 dark:bg-card rounded-lg px-3 text-sm md:text-sm";
const addItemButtonClass =
  "border-primary hover:border-[var(--td-brand-color-active)] active:bg-[var(--td-brand-color-active)] disabled:border-border disabled:text-placeholder size-8 min-w-8 shrink-0 rounded-lg p-0 transition-all duration-200 hover:scale-105 hover:shadow-[0_2px_8px_rgba(7,192,95,0.3)] active:scale-[0.98] disabled:bg-[var(--td-bg-color-component-disabled)] disabled:opacity-60";
const itemRowClass =
  "relative box-border flex gap-2.5 rounded-lg border px-3.5 shadow-[0_1px_2px_rgba(0,0,0,0.04)] transition-all duration-200 hover:-translate-y-px";
const itemContentClass = "text-foreground flex-1 p-0 text-sm leading-[1.6] font-normal break-words whitespace-pre-wrap";
const removeItemButtonClass =
  "text-placeholder hover:text-destructive flex size-6 min-w-6 shrink-0 items-center justify-center rounded-md p-0 transition-all duration-200 hover:bg-[var(--td-error-color-light)] active:bg-[var(--td-error-color-light)]";

// Search drawer rows.
const searchRowClass = "flex flex-col items-start justify-between gap-3";
const searchInfoClass = "mb-2 w-full";
const searchLabelClass = "text-foreground mb-1 block text-sm font-medium";
const searchDescClass = "text-muted-foreground m-0 text-xs leading-[1.4]";
const sliderValueClass =
  "text-foreground bg-card min-w-[50px] shrink-0 rounded-md px-2 py-1 text-right text-sm font-medium";
const resultSectionLabelClass = "text-muted-foreground mb-1 text-xs font-semibold tracking-[0.5px] uppercase";

// Import dialog field labels; both fields are required, hence the asterisk.
const importLabelClass =
  "text-foreground m-0 block flex-1 text-sm font-medium tracking-[-0.2px] after:ml-1 after:font-semibold after:text-destructive after:content-['*']";

// The two hand-built modals (import, batch tag): a blurred backdrop and a
// panel that scales in with it. The panel is the overlay's only child, which
// is what the `[&>div]` transition classes reach.
const modalOverlayClass = "fixed inset-0 z-[1000] flex items-center justify-center bg-black/50 p-5 backdrop-blur-[4px]";
const modalCloseClass =
  "bg-secondary text-muted-foreground hover:text-foreground absolute top-5 right-5 z-10 flex size-8 items-center justify-center rounded-md transition-all duration-200";
const modalTransition = {
  enterActiveClass:
    "transition-opacity duration-200 ease-in-out [&>div]:transition-[transform,opacity] [&>div]:duration-200",
  leaveActiveClass:
    "transition-opacity duration-200 ease-in-out [&>div]:transition-[transform,opacity] [&>div]:duration-200",
  enterFromClass: "opacity-0 [&>div]:scale-95 [&>div]:opacity-0",
  leaveToClass: "opacity-0 [&>div]:scale-95 [&>div]:opacity-0",
};

// Collapsing card sections and expanding search results slide 8px and fade.
const slideDownTransition = {
  enterActiveClass:
    "overflow-hidden transition-[opacity,transform] duration-200 ease-[cubic-bezier(0.4,0,0.2,1)] will-change-[opacity,transform]",
  leaveActiveClass:
    "overflow-hidden transition-[opacity,transform] duration-200 ease-[cubic-bezier(0.4,0,0.2,1)] will-change-[opacity,transform]",
  enterFromClass: "-translate-y-2 opacity-0",
  leaveToClass: "-translate-y-2 opacity-0",
};
</script>
