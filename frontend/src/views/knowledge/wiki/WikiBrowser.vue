<template>
  <div class="bg-card flex h-full min-h-0">
    <!-- Graph view (full screen) -->
    <template v-if="view === 'graph'">
      <div class="relative size-full flex-1 overflow-hidden">
        <div ref="graphRef" class="size-full min-h-[500px]"></div>

        <!-- Graph Search Overlay -->
        <div v-if="graphReady" class="absolute top-4 left-4 z-10 flex w-[320px] flex-col gap-3">
          <div class="flex w-full items-center gap-2">
            <!-- A combobox rather than a Select: the list is fed by a remote
                 search as the user types, and Enter has to work before any
                 suggestion has arrived (handleGraphSearchEnter). -->
            <div class="relative min-w-0 flex-1 rounded-sm shadow-[var(--td-shadow-1)]">
              <SearchIcon
                class="text-placeholder pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
              />
              <Input
                :model-value="graphSearchKeyword"
                role="combobox"
                aria-autocomplete="list"
                :aria-expanded="graphSearchOpen"
                class="bg-card dark:bg-card h-8 pl-8 opacity-95"
                :placeholder="$t('knowledgeEditor.wikiBrowser.searchPlaceholder')"
                @update:model-value="onGraphSearchInput"
                @focus="openGraphSearch"
                @click="openGraphSearch"
                @blur="closeGraphSearch"
                @keydown="onGraphSearchKeydown"
              />
              <div
                v-if="graphSearchOpen"
                role="listbox"
                class="bg-popover text-popover-foreground ring-foreground/10 absolute top-full right-0 left-0 z-[100] mt-1 max-h-[300px] overflow-y-auto rounded-md p-1 shadow-md ring-1"
              >
                <div
                  v-if="graphSearchLoading"
                  class="text-muted-foreground flex items-center justify-center gap-2 py-3 text-sm"
                >
                  <Loader2Icon class="text-primary size-4 animate-spin" />
                  {{ $t("common.loading") }}
                </div>
                <div
                  v-else-if="graphSearchEffectiveOptions.length === 0"
                  class="text-placeholder py-3 text-center text-sm"
                >
                  {{ $t("common.noData") }}
                </div>
                <template v-else>
                  <!-- mousedown.prevent keeps focus in the input, so the blur
                       that closes the list does not fire before the click. -->
                  <div
                    v-for="(opt, i) in graphSearchEffectiveOptions"
                    :key="opt.value"
                    role="option"
                    :aria-selected="i === graphSearchActiveIndex"
                    class="text-foreground flex h-7 cursor-pointer items-center rounded-sm px-2 text-sm"
                    :class="{ 'bg-accent': i === graphSearchActiveIndex }"
                    @mousedown.prevent
                    @mouseenter="graphSearchActiveIndex = i"
                    @click="pickGraphSearchOption(opt)"
                  >
                    <span class="truncate">{{ opt.label }}</span>
                  </div>
                </template>
              </div>
            </div>
            <Popover>
              <PopoverTrigger as-child>
                <div
                  class="text-placeholder hover:text-primary inline-flex size-8 shrink-0 cursor-pointer items-center justify-center transition-colors select-none"
                  role="button"
                  tabindex="0"
                  :title="$t('knowledgeEditor.wikiBrowser.helpButtonTitle')"
                >
                  <CircleHelpIcon class="size-[18px]" />
                </div>
              </PopoverTrigger>
              <PopoverContent side="bottom" align="end" class="w-auto p-3">
                <PopoverArrow class="fill-popover" />
                <div class="max-w-[320px] min-w-[240px]">
                  <div class="text-placeholder mb-2 text-[11px] leading-[14px] tracking-[0.04em] uppercase select-none">
                    {{ $t("knowledgeEditor.wikiBrowser.helpTitle") }}
                  </div>
                  <div class="flex flex-col gap-1.5">
                    <div
                      v-for="row in graphHelpRows"
                      :key="row.action"
                      class="grid grid-cols-[110px_1fr] gap-3 text-xs leading-4"
                    >
                      <span class="text-foreground font-medium whitespace-nowrap">{{ row.action }}</span>
                      <span class="text-muted-foreground">{{ row.desc }}</span>
                    </div>
                  </div>
                </div>
              </PopoverContent>
            </Popover>
          </div>
          <Button
            variant="outline"
            size="sm"
            class="bg-card shadow-[var(--td-shadow-1)]"
            @click="showLintDrawer = true"
          >
            <ShieldCheckIcon />
            {{ $t("knowledgeEditor.wikiBrowser.lintOpen") }}
          </Button>
        </div>

        <!-- Legend Overlay -->
        <div
          v-if="graphReady"
          class="bg-card border-border absolute top-4 z-10 flex flex-col gap-3 rounded-md border px-3 py-2.5 opacity-95 shadow-[var(--td-shadow-1)] transition-[right] duration-300 ease-[cubic-bezier(0.645,0.045,0.355,1)]"
          :class="graphDrawerVisible ? 'right-[496px]' : 'right-4'"
        >
          <div class="flex flex-col gap-2">
            <div
              class="hover:text-foreground flex cursor-pointer items-center gap-2 text-[11px] transition-all"
              :class="
                graphFilterTypes.has('summary') ? 'text-muted-foreground' : 'text-placeholder line-through opacity-50'
              "
              @click="toggleGraphFilterType('summary')"
            >
              <span class="inline-block size-2.5 shrink-0 rounded-full bg-[#0052d9]"></span>
              {{ $t("knowledgeEditor.wikiBrowser.filterSummary") }}
            </div>
            <div
              class="hover:text-foreground flex cursor-pointer items-center gap-2 text-[11px] transition-all"
              :class="
                graphFilterTypes.has('entity') ? 'text-muted-foreground' : 'text-placeholder line-through opacity-50'
              "
              @click="toggleGraphFilterType('entity')"
            >
              <span class="inline-block size-2.5 shrink-0 rounded-full bg-[#2ba471]"></span>
              {{ $t("knowledgeEditor.wikiBrowser.filterEntity") }}
            </div>
            <div
              class="hover:text-foreground flex cursor-pointer items-center gap-2 text-[11px] transition-all"
              :class="
                graphFilterTypes.has('concept') ? 'text-muted-foreground' : 'text-placeholder line-through opacity-50'
              "
              @click="toggleGraphFilterType('concept')"
            >
              <span class="inline-block size-2.5 shrink-0 rounded-full bg-[#e37318]"></span>
              {{ $t("knowledgeEditor.wikiBrowser.filterConcept") }}
            </div>
            <div
              class="hover:text-foreground flex cursor-pointer items-center gap-2 text-[11px] transition-all"
              :class="
                graphFilterTypes.has('synthesis') ? 'text-muted-foreground' : 'text-placeholder line-through opacity-50'
              "
              @click="toggleGraphFilterType('synthesis')"
            >
              <span class="inline-block size-2.5 shrink-0 rounded-full bg-[#0594fa]"></span>
              {{ $t("knowledgeEditor.wikiBrowser.filterSynthesis") }}
            </div>
            <div
              class="hover:text-foreground flex cursor-pointer items-center gap-2 text-[11px] transition-all"
              :class="
                graphFilterTypes.has('comparison')
                  ? 'text-muted-foreground'
                  : 'text-placeholder line-through opacity-50'
              "
              @click="toggleGraphFilterType('comparison')"
            >
              <span class="inline-block size-2.5 shrink-0 rounded-full bg-[#d54941]"></span>
              {{ $t("knowledgeEditor.wikiBrowser.filterComparison") }}
            </div>
            <div v-if="graphFamiliarCount > 0" class="text-muted-foreground flex items-center gap-2 text-[11px]">
              <span
                class="box-border inline-block size-2.5 shrink-0 rounded-full border-2 border-[#0052d9] bg-transparent"
              ></span>
              {{ $t("knowledgeEditor.wikiBrowser.legendFamiliar") }}
            </div>
          </div>
          <div class="bg-border -mx-3 h-px"></div>
          <div class="flex flex-col gap-2">
            <div :class="legendActionClass" @click="fitGraphToView" title="Fit to View">
              <span :class="legendActionIconClass"><FocusIcon class="size-[13px]" /></span>
              <span>{{ $t("knowledgeEditor.wikiBrowser.fitView") || "适应屏幕" }}</span>
            </div>
            <div :class="legendActionClass" @click="toggleArrows">
              <span :class="legendActionIconClass">
                <EyeOffIcon v-if="showArrows" class="size-[13px]" />
                <EyeIcon v-else class="size-[13px]" />
              </span>
              <span>{{
                showArrows ? $t("knowledgeEditor.wikiBrowser.hideArrows") : $t("knowledgeEditor.wikiBrowser.showArrows")
              }}</span>
            </div>
            <div
              v-if="graphMode === 'ego' && graphFrontierCount > 0"
              :class="legendActionClass"
              @click="growFrontier"
              :title="$t('knowledgeEditor.wikiBrowser.growFrontierTitle', { count: graphFrontierCount })"
            >
              <span :class="legendActionIconClass"><ChartScatterIcon class="size-[13px]" /></span>
              <span>{{ $t("knowledgeEditor.wikiBrowser.growFrontier", { count: graphFrontierCount }) }}</span>
            </div>
            <div v-if="graphMode === 'ego'" :class="legendActionClass" @click="loadGraph">
              <span :class="legendActionIconClass"><Undo2Icon class="size-[13px]" /></span>
              <span>{{ $t("knowledgeEditor.wikiBrowser.backToOverview") }}</span>
            </div>
          </div>
          <template v-if="graphStatusCard">
            <div
              class="border-border flex max-w-[240px] flex-col gap-1 border-t pt-2 select-none [--tw-border-style:dashed]"
            >
              <div class="text-placeholder flex items-center gap-1 text-[11px] leading-[14px]">
                <component :is="graphStatusCard.icon" class="size-3" />
                <span class="font-medium">{{ graphStatusCard.title }}</span>
              </div>
              <div class="text-foreground truncate text-xs leading-4" :title="graphStatusCard.primary">
                {{ graphStatusCard.primary }}
              </div>
              <div v-if="graphStatusCard.secondary" class="text-muted-foreground text-[11px] leading-[14px]">
                {{ graphStatusCard.secondary }}
              </div>
            </div>
          </template>
        </div>

        <div
          v-if="!graphReady"
          class="bg-card absolute inset-0 z-20 flex flex-col items-center justify-center px-5 py-[60px] text-center"
        >
          <Loader2Icon v-if="graphLoading" class="text-primary size-6 animate-spin" />
          <div v-else class="bg-muted text-placeholder mb-4 flex size-16 items-center justify-center rounded-full">
            <ChartScatterIcon class="size-12" />
          </div>
          <p class="text-placeholder m-0 text-[13px]">
            {{
              graphLoading
                ? $t("knowledgeEditor.wikiBrowser.graphEmpty")
                : $t("knowledgeEditor.wikiBrowser.graphNoData")
            }}
          </p>
        </div>

        <!-- Graph page detail drawer. A plain fixed panel rather than a modal
             drawer: the graph behind it must stay interactive (clicking another
             node swaps the drawer's page), so there is no overlay, no focus
             trap and no dismiss-on-outside-click. -->
        <Teleport to="body">
          <Transition
            enter-active-class="transition-transform duration-300 ease-out"
            leave-active-class="transition-transform duration-200 ease-in"
            enter-from-class="translate-x-full"
            leave-to-class="translate-x-full"
          >
            <aside
              v-if="graphDrawerVisible"
              role="dialog"
              :aria-label="graphDrawerPage?.title || ''"
              class="bg-card text-foreground fixed top-0 right-0 bottom-0 z-[1500] flex w-[480px] max-w-full flex-col shadow-[-4px_0_16px_rgba(0,0,0,0.08)]"
            >
              <header class="border-border flex h-14 shrink-0 items-center gap-2 border-b pr-3 pl-6">
                <div class="min-w-0 flex-1 truncate text-base font-semibold">{{ graphDrawerPage?.title || "" }}</div>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :aria-label="$t('common.close')"
                  @click="graphDrawerVisible = false"
                >
                  <XIcon />
                </Button>
              </header>
              <div class="min-h-0 flex-1 overflow-y-auto p-6">
                <template v-if="graphDrawerPage">
                  <div class="mb-2 flex min-w-0 flex-wrap items-center gap-2.5">
                    <span
                      :class="[
                        'inline-flex h-[22px] items-center rounded-sm border px-2 text-xs',
                        getTypeTagClass(graphDrawerPage.page_type),
                      ]"
                    >
                      {{ getTypeLabel(graphDrawerPage.page_type) }}
                    </span>
                    <span class="text-placeholder text-[13px]">{{
                      $t("knowledgeEditor.wikiBrowser.version", {
                        ver: graphDrawerPage.version,
                      })
                    }}</span>
                    <Button
                      v-if="graphMode === 'ego' && graphCenter !== graphDrawerPage.slug"
                      size="sm"
                      variant="outline"
                      class="ml-auto"
                      :disabled="!graphDrawerCanBloom"
                      @click="loadBloomNeighbors(graphDrawerPage.slug)"
                    >
                      {{ $t("knowledgeEditor.wikiBrowser.bloomNeighbors") }}
                    </Button>
                    <Button
                      v-if="graphMode !== 'ego' || graphCenter !== graphDrawerPage.slug"
                      size="sm"
                      variant="outline"
                      class="border-primary text-primary hover:text-primary hover:bg-primary/10 dark:border-primary"
                      :class="{ 'ml-auto': !(graphMode === 'ego' && graphCenter !== graphDrawerPage.slug) }"
                      @click="loadEgoGraph(graphDrawerPage.slug)"
                    >
                      {{ $t("knowledgeEditor.wikiBrowser.expandNeighbors") }}
                    </Button>
                  </div>
                  <div v-if="graphDrawerNeighborHint" class="text-muted-foreground mb-4 text-xs leading-4 select-none">
                    {{ graphDrawerNeighborHint }}
                  </div>
                  <div
                    ref="drawerBodyRef"
                    class="wiki-reader-body"
                    v-html="graphDrawerContent"
                    @click="handleGraphDrawerClick"
                  ></div>
                </template>
              </div>
            </aside>
          </Transition>
        </Teleport>
      </div>
    </template>

    <!-- Browser view (left list + right reader) -->
    <template v-else>
      <!-- Left Panel: Page List -->
      <aside class="border-border bg-card flex w-[280px] min-w-[240px] shrink-0 flex-col border-r">
        <div class="-ml-2 flex flex-col gap-2 pr-2.5 pb-2 pl-2">
          <div
            v-if="stats && (stats.pending_tasks > 0 || stats.is_active)"
            class="bg-muted text-muted-foreground flex items-center gap-2 rounded-md px-3 py-2 text-[13px]"
          >
            <Loader2Icon class="text-primary size-4 shrink-0 animate-spin" />
            <span class="leading-[1.2]">{{
              $t("knowledgeEditor.wikiBrowser.queueStatus", {
                count: stats.pending_tasks || 0,
              })
            }}</span>
          </div>
          <!-- Structural check of the whole wiki (lint) -->
          <Button variant="outline" size="sm" class="justify-start" @click="showLintDrawer = true">
            <ShieldCheckIcon />
            {{ $t("knowledgeEditor.wikiBrowser.lintOpen") }}
          </Button>
          <div class="relative">
            <SearchIcon
              class="text-placeholder pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
            />
            <Input
              :model-value="searchQuery"
              class="pr-8 pl-8"
              :placeholder="$t('knowledgeEditor.wikiBrowser.searchPlaceholder')"
              @update:model-value="(v) => (searchQuery = String(v))"
              @keydown.enter="doSearch"
            />
            <button
              v-if="searchQuery"
              type="button"
              data-slot="search-clear"
              class="text-placeholder hover:text-muted-foreground absolute top-1/2 right-2 inline-flex size-4 -translate-y-1/2 items-center justify-center"
              :aria-label="$t('common.clear')"
              @click="clearSidebarSearch"
            >
              <CircleXIcon class="size-4" />
            </button>
          </div>
        </div>

        <div class="-ml-2 flex-1 overflow-y-auto pr-2 pb-3 pl-2" ref="pageListRef">
          <!-- Search mode: flat list of hits, no group chrome. Clearing
               the search snaps back to the bucketed view below. -->
          <template v-if="searchResults !== null">
            <div
              v-for="page in searchResults"
              :key="page.id"
              :class="[
                'hover:bg-accent box-border min-h-[30px] cursor-pointer overflow-hidden rounded-[6px] py-1.5 pr-2.5 pl-2.5 transition-colors select-none',
                { 'bg-accent': selectedPage?.id === page.id },
              ]"
              @click="selectPage(page)"
            >
              <div
                class="min-w-0 flex-1 truncate text-[13px]"
                :class="selectedPage?.id === page.id ? 'text-primary' : 'text-foreground'"
              >
                {{ page.title }}
              </div>
              <div class="text-muted-foreground mb-1.5 line-clamp-2 text-xs leading-[1.5]">{{ page.summary }}</div>
              <div class="text-placeholder flex items-center justify-between text-[11px]">
                <span>{{ formatDate(page.updated_at) }}</span>
              </div>
            </div>
            <div
              v-if="searchResults.length === 0 && !loading"
              class="flex flex-col items-center justify-center px-5 py-[60px] text-center"
            >
              <p class="text-placeholder m-0 text-[13px]">
                {{ $t("knowledgeEditor.wikiBrowser.searchNoResults") || "没有找到匹配的页面" }}
              </p>
            </div>
          </template>

          <template v-else>
            <!-- Index overview (pinned at top). Rendered lazily from a
                 structured API response — never loads the full directory
                 as markdown. -->
            <div
              v-if="indexAvailable"
              :class="[
                'hover:bg-accent flex min-h-[30px] cursor-pointer items-center gap-2 rounded-[6px] pr-2.5 pl-2.5 transition-colors',
                { 'bg-accent': activeSystemView === 'index' },
              ]"
              @click="openIndexView"
            >
              <TableOfContentsIcon
                class="size-4 shrink-0"
                :class="activeSystemView === 'index' ? 'text-primary' : 'text-muted-foreground'"
              />
              <span
                class="text-sm leading-5 font-normal"
                :class="activeSystemView === 'index' ? 'text-primary' : 'text-foreground'"
                >{{ $t("knowledgeEditor.wikiBrowser.indexTitle") }}</span
              >
            </div>

            <div class="bg-border my-1.5 h-px" v-if="indexAvailable"></div>

            <!-- Tab bar + tree share one horizontal inset so the "new folder"
                 action lines up with the folder rows below. -->
            <div v-if="visibleTabs.length > 0 || activeGroup">
              <div v-if="visibleTabs.length > 0" class="bg-card sticky top-0 z-10 flex items-center gap-2 pt-1 pb-1.5">
                <div
                  class="flex min-w-0 flex-1 [scrollbar-width:none] gap-4 overflow-x-auto [&::-webkit-scrollbar]:hidden"
                >
                  <div
                    v-for="tab in visibleTabs"
                    :key="tab.type"
                    class="relative inline-flex shrink-0 cursor-pointer items-center gap-[5px] px-0.5 pt-[7px] pb-2 text-[13px] whitespace-nowrap transition-colors"
                    :class="
                      activeTab === tab.type
                        ? 'text-primary after:bg-primary font-semibold after:absolute after:right-0.5 after:bottom-px after:left-0.5 after:h-0.5 after:rounded-t-[2px]'
                        : 'text-muted-foreground hover:text-foreground'
                    "
                    @click="setActiveTab(tab.type)"
                  >
                    <span>{{ tab.label }}</span>
                    <span
                      class="text-[11px] leading-none"
                      :class="activeTab === tab.type ? 'text-primary font-medium' : 'text-placeholder'"
                      >{{ tab.total }}</span
                    >
                  </div>
                </div>
                <div class="flex shrink-0 items-center gap-1.5">
                  <div
                    class="bg-card border-border inline-flex items-center rounded-md border p-0.5"
                    role="group"
                    :aria-label="$t('knowledgeEditor.wikiBrowser.viewModeToggle')"
                  >
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <button
                          type="button"
                          data-slot="wiki-view-toggle"
                          class="inline-flex h-[22px] w-6 items-center justify-center rounded-[4px] transition-colors duration-[120ms]"
                          :class="
                            sidebarViewMode === 'tree'
                              ? 'text-primary bg-card shadow-[0_1px_2px_rgba(0,0,0,0.06)]'
                              : 'text-muted-foreground hover:text-foreground'
                          "
                          :aria-pressed="sidebarViewMode === 'tree'"
                          :aria-label="$t('knowledgeEditor.wikiBrowser.viewTree')"
                          :disabled="sidebarViewSwitching"
                          @click="switchSidebarViewMode('tree')"
                        >
                          <ListTreeIcon class="size-[15px]" />
                        </button>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("knowledgeEditor.wikiBrowser.viewTree") }}</TooltipContent>
                    </Tooltip>
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <button
                          type="button"
                          data-slot="wiki-view-toggle"
                          class="inline-flex h-[22px] w-6 items-center justify-center rounded-[4px] transition-colors duration-[120ms]"
                          :class="
                            sidebarViewMode === 'list'
                              ? 'text-primary bg-card shadow-[0_1px_2px_rgba(0,0,0,0.06)]'
                              : 'text-muted-foreground hover:text-foreground'
                          "
                          :aria-pressed="sidebarViewMode === 'list'"
                          :aria-label="$t('knowledgeEditor.wikiBrowser.viewList')"
                          :disabled="sidebarViewSwitching"
                          @click="switchSidebarViewMode('list')"
                        >
                          <ListIcon class="size-[15px]" />
                        </button>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("knowledgeEditor.wikiBrowser.viewList") }}</TooltipContent>
                    </Tooltip>
                  </div>
                  <Tooltip v-if="props.canEdit">
                    <TooltipTrigger as-child>
                      <button
                        type="button"
                        data-slot="wiki-tab-bar-action"
                        :class="tabBarActionClass"
                        :disabled="sidebarViewMode !== 'tree' || sidebarViewSwitching || sidebarTabSwitching"
                        :aria-label="$t('knowledgeEditor.wikiBrowser.newRootFolder')"
                        @click.stop="startCreateRootFolder"
                      >
                        <FolderPlusIcon class="size-[15px]" />
                      </button>
                    </TooltipTrigger>
                    <TooltipContent side="top">{{ $t("knowledgeEditor.wikiBrowser.newRootFolder") }}</TooltipContent>
                  </Tooltip>
                  <Tooltip v-if="props.canEdit">
                    <TooltipTrigger as-child>
                      <button
                        type="button"
                        data-slot="wiki-tab-bar-action"
                        :class="tabBarActionClass"
                        :aria-label="$t('knowledgeEditor.wikiBrowser.newPageBtn')"
                        @click.stop="openCreatePageDialog"
                      >
                        <FilePlusIcon class="size-[15px]" />
                      </button>
                    </TooltipTrigger>
                    <TooltipContent side="top">{{ $t("knowledgeEditor.wikiBrowser.newPageBtn") }}</TooltipContent>
                  </Tooltip>
                </div>
              </div>

              <!-- Active-tab list -->
              <template v-if="activeGroup && sidebarViewMode === 'tree'">
                <!-- The list container is itself the "move to root" drop target;
                   folder rows stop propagation so their own drop wins. This
                   avoids inserting/removing a drop bar during the drag, which
                   was cancelling the native drag after the first success. -->
                <div
                  ref="treeListRef"
                  class="pb-1"
                  :class="{ 'rounded-md shadow-[inset_0_0_0_1px_var(--td-brand-color)]': dropTargetKey === '__root__' }"
                  @dragover.prevent="onRootDragOver"
                  @dragleave="onDirectoryDragLeave('__root__')"
                  @drop.prevent="onDropOnDirectory($event, '', [])"
                >
                  <div
                    v-if="creatingRootFolder"
                    class="text-muted-foreground box-border flex h-[34px] cursor-default items-center gap-1.5 overflow-hidden rounded-[6px] pr-2.5 pl-2.5 select-none"
                    @click.stop
                  >
                    <input
                      ref="creatingRootFolderInputRef"
                      v-model="creatingRootFolderName"
                      data-slot="wiki-folder-name-input"
                      :class="folderNameInputClass"
                      :placeholder="$t('knowledgeEditor.wikiBrowser.folderNamePlaceholder')"
                      @keydown.enter="submitCreateRootFolder"
                      @keydown.esc="cancelCreateRootFolder"
                    />
                    <div class="ml-auto flex shrink-0 items-center justify-end gap-0.5">
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        class="text-muted-foreground hover:bg-muted hover:text-primary rounded-[4px]"
                        @click.stop="submitCreateRootFolder"
                      >
                        <CheckIcon class="size-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        class="text-muted-foreground hover:bg-muted hover:text-destructive rounded-[4px]"
                        @click.stop="cancelCreateRootFolder"
                      >
                        <XIcon class="size-4" />
                      </Button>
                    </div>
                  </div>
                  <template v-for="item in activeTreeRows" :key="item.rowKey">
                    <!-- group/wiki-dir lets the row's folder-actions trigger
                         (WikiFolderActions) reveal itself on row hover. -->
                    <div
                      v-if="item.kind === 'directory'"
                      class="group/wiki-dir text-muted-foreground hover:bg-accent hover:text-foreground box-border flex h-[34px] cursor-pointer items-center gap-1.5 overflow-hidden rounded-[6px] pr-2.5 pl-[calc(var(--wiki-tree-depth,0)*14px+10px)] transition-colors select-none"
                      :class="{
                        'bg-[var(--td-brand-color-light)] shadow-[inset_0_0_0_1px_var(--td-brand-color)]':
                          dropTargetKey === item.pathKey,
                      }"
                      :style="{ '--wiki-tree-depth': item.depth }"
                      :draggable="editingFolderId !== item.folderId"
                      @click="toggleDirectory(item.pathKey)"
                      @dragstart="onFolderDragStart($event, item.folderId, item.path)"
                      @dragend="onPageDragEnd"
                      @dragover.prevent.stop="onDirectoryDragOver($event, item.pathKey)"
                      @dragleave.stop="onDirectoryDragLeave(item.pathKey)"
                      @drop.prevent.stop="onDropOnDirectory($event, item.folderId, item.path)"
                    >
                      <ChevronRightIcon v-if="item.collapsed" class="text-placeholder size-[15px] flex-none" />
                      <ChevronDownIcon v-else class="text-placeholder size-[15px] flex-none" />
                      <input
                        v-if="editingFolderId === item.folderId"
                        v-model="editingName"
                        data-slot="wiki-folder-name-input"
                        :class="folderNameInputClass"
                        :placeholder="$t('knowledgeEditor.wikiBrowser.folderNamePlaceholder')"
                        @click.stop
                        @keydown.enter="commitRenameFolder(item.folderId, item.label)"
                        @keydown.esc="cancelRenameFolder"
                        @blur="commitRenameFolder(item.folderId, item.label)"
                      />
                      <template v-else>
                        <span class="text-foreground min-w-0 flex-1 truncate text-[13px] font-semibold">{{
                          item.label
                        }}</span>
                        <div class="ml-auto flex shrink-0 items-center justify-end gap-0.5">
                          <span
                            class="text-placeholder min-w-4 flex-none text-right text-[11px] leading-[18px] tabular-nums"
                            >{{ item.count }}</span
                          >
                          <WikiFolderActions
                            v-if="item.folderId"
                            :name="item.label"
                            :page-count="item.count"
                            :has-children="item.hasChildren"
                            @create="(name: string) => createFolder(item.folderId, item.path, name)"
                            @rename="() => startRenameFolder(item.folderId, item.label)"
                            @delete="() => deleteFolder(item.folderId)"
                          />
                        </div>
                      </template>
                    </div>
                    <div
                      v-else-if="item.kind === 'load-more'"
                      class="text-primary my-px box-border flex h-[30px] cursor-pointer items-center gap-1.5 rounded-[6px] pl-[calc(var(--wiki-tree-depth,0)*14px)] text-xs hover:bg-[var(--td-brand-color-light)]"
                      :data-loadmore-key="item.rowKey"
                      :style="{ '--wiki-tree-depth': item.depth }"
                      @click="loadPagesForType(item.type, { categoryPath: item.path })"
                    >
                      <Loader2Icon v-if="item.loading" class="size-4 animate-spin" />
                      <template v-else>
                        <ChevronDownIcon class="size-3" />
                        <span>{{ $t("knowledgeEditor.wikiBrowser.loadMoreShort") }}</span>
                      </template>
                    </div>
                    <div
                      v-else
                      :class="[
                        'hover:bg-accent box-border flex h-[34px] min-h-[34px] cursor-pointer items-center gap-1.5 overflow-hidden rounded-[6px] py-1 pr-2.5 pl-[calc(var(--wiki-tree-depth,0)*14px+10px)] transition-colors select-none',
                        { 'bg-accent': selectedPage?.id === item.page.id },
                      ]"
                      :style="{ '--wiki-tree-depth': item.depth }"
                      :title="item.page.title"
                      draggable="true"
                      @click="selectPage(item.page)"
                      @dragstart="onPageDragStart($event, item.page)"
                      @dragend="onPageDragEnd"
                    >
                      <component
                        :is="getPageIcon(item.page)"
                        class="size-[15px] flex-none"
                        :class="selectedPage?.id === item.page.id ? 'text-primary' : 'text-placeholder'"
                      />
                      <span
                        class="min-w-0 flex-1 truncate text-sm leading-5 font-normal transition-colors"
                        :class="selectedPage?.id === item.page.id ? 'text-primary' : 'text-foreground'"
                        >{{ item.page.title }}</span
                      >
                    </div>
                  </template>
                </div>
                <!-- Sentinel: when this hits the viewport, fetch the next
                   page. The tree list itself is intentionally plain DOM:
                   rows are already paged, and avoiding dynamic row
                   measurement keeps expand/collapse visually stable. -->
                <div
                  v-if="activeGroup.hasMore"
                  ref="groupSentinelRef"
                  class="h-px w-full"
                  :data-type="activeGroup.type"
                ></div>
                <div v-if="activeGroup.loading" class="flex justify-center py-1.5">
                  <Loader2Icon class="text-primary size-4 animate-spin" />
                </div>
              </template>

              <!-- Classic flat list. It owns a separate paged dataset because
                   tree mode requests one directory at a time. -->
              <template v-else-if="activeGroup">
                <RecycleScroller
                  class="my-1 min-h-[60px]"
                  :items="activeFlatPages"
                  :item-size="WIKI_PAGE_ITEM_HEIGHT"
                  key-field="id"
                  :buffer="400"
                  page-mode
                  v-slot="{ item }"
                >
                  <div
                    :class="[
                      'hover:bg-accent box-border h-[98px] cursor-pointer overflow-hidden rounded-[6px] py-2 pr-2.5 pl-2.5 transition-colors select-none',
                      { 'bg-accent': selectedPage?.id === item.id },
                    ]"
                    @click="selectPage(item)"
                  >
                    <div
                      class="mb-1 block min-w-0 truncate text-sm leading-5 font-normal transition-colors"
                      :class="selectedPage?.id === item.id ? 'text-primary' : 'text-foreground'"
                    >
                      {{ item.title }}
                    </div>
                    <div class="text-muted-foreground mb-1.5 line-clamp-2 text-xs leading-[1.5]">
                      {{ item.summary }}
                    </div>
                    <div class="text-placeholder flex items-center justify-between text-[11px]">
                      <span>{{ formatDate(item.updated_at) }}</span>
                    </div>
                  </div>
                </RecycleScroller>
                <div
                  v-if="activeFlatState?.hasMore"
                  ref="groupSentinelRef"
                  class="h-px w-full"
                  :data-type="activeGroup.type"
                ></div>
                <div v-if="activeFlatState?.loading" class="flex justify-center py-1.5">
                  <Loader2Icon class="text-primary size-4 animate-spin" />
                </div>
              </template>
            </div>

            <!-- Empty state -->
            <div
              v-if="!hasContentPages && !loading"
              class="flex flex-col items-center justify-center px-5 py-[60px] text-center"
            >
              <div class="bg-muted text-placeholder mb-4 flex size-16 items-center justify-center rounded-full">
                <FileQuestionMarkIcon class="size-9" />
              </div>
              <p class="text-muted-foreground m-0 mb-1 text-sm font-medium">
                {{ $t("knowledgeEditor.wikiBrowser.emptyTitle") }}
              </p>
              <p class="text-placeholder m-0 text-[13px]">{{ $t("knowledgeEditor.wikiBrowser.emptyDesc") }}</p>
            </div>
          </template>
        </div>
      </aside>

      <!-- Right Panel: Reader -->
      <div class="flex min-w-0 flex-1 flex-col overflow-hidden">
        <div class="flex-1 overflow-y-auto px-6 pb-4">
          <div class="w-full">
            <template v-if="selectedPage">
              <!-- Navigation -->
              <div v-if="navHistory.length || navFromSystemView" class="mb-2">
                <a
                  href="#"
                  class="text-muted-foreground hover:bg-accent hover:text-primary -ml-2 inline-flex items-center gap-1 rounded-[4px] px-2 py-1 text-[13px] no-underline transition-all"
                  @click.prevent="goBack"
                >
                  <ArrowLeftIcon class="size-3.5" />
                  <span>{{ backLabel }}</span>
                </a>
              </div>

              <!-- Page header -->
              <div class="mb-6">
                <div class="flex items-start justify-between gap-5">
                  <div class="min-w-0 flex-1">
                    <h2
                      v-if="!editingPage"
                      class="text-foreground m-0 flex min-w-0 items-center gap-2 text-[26px] leading-[1.3] font-semibold"
                    >
                      <span class="min-w-0">{{ selectedPage.title }}</span>
                    </h2>
                    <Input
                      v-else
                      :model-value="editForm.title"
                      class="bg-card dark:bg-card focus-visible:border-primary h-auto min-h-11 rounded-lg px-3 py-2.5 text-[22px] leading-[1.35] font-semibold focus-visible:ring-2 focus-visible:ring-[rgba(7,192,95,0.1)] md:text-[22px]"
                      :placeholder="$t('knowledgeEditor.wikiBrowser.editTitlePlaceholder')"
                      @update:model-value="(v) => (editForm.title = String(v))"
                    />
                    <div class="mt-2 inline-flex shrink-0 flex-wrap items-center gap-1.5">
                      <span v-if="editingPage" :class="[badgeClass, 'text-warning bg-[var(--td-warning-color-1)]']">
                        {{ $t("knowledgeEditor.wikiBrowser.editingBadge") }}
                      </span>
                      <span :class="[badgeClass, 'bg-muted text-muted-foreground']">
                        <component :is="getPageIcon(selectedPage)" class="size-[13px] shrink-0" />
                        {{ getTypeLabel(selectedPage.page_type) }}
                      </span>
                      <span
                        :class="[
                          badgeClass,
                          'bg-muted text-muted-foreground font-[family-name:var(--td-font-family-mono,monospace)] tabular-nums',
                        ]"
                      >
                        {{ $t("knowledgeEditor.wikiBrowser.version", { ver: selectedPage.version }) }}
                      </span>
                      <Tooltip v-if="editSourceVisible(selectedPage.last_edit_source)">
                        <TooltipTrigger as-child>
                          <span :class="[badgeClass, 'bg-muted text-muted-foreground cursor-default']">
                            <component
                              :is="editSourceIcon(selectedPage.last_edit_source)"
                              class="size-[13px] shrink-0"
                            />
                            {{ editSourceLabel(selectedPage.last_edit_source) }}
                          </span>
                        </TooltipTrigger>
                        <TooltipContent>{{ editSourceLabel(selectedPage.last_edit_source) }}</TooltipContent>
                      </Tooltip>
                      <span
                        v-for="alias in selectedPage.aliases || []"
                        :key="alias"
                        :class="[badgeClass, 'bg-muted text-muted-foreground max-w-[240px] truncate']"
                        :title="`${$t('knowledgeEditor.wikiBrowser.aliases')} ${alias}`"
                      >
                        <LinkIcon class="size-[13px] shrink-0" />
                        {{ alias }}
                      </span>
                    </div>
                    <p
                      v-if="!editingPage && selectedPage.summary"
                      class="text-muted-foreground mt-2.5 mb-0 text-[15px] leading-[1.65]"
                    >
                      {{ selectedPage.summary }}
                    </p>
                    <!-- field-sizing-content grows the box with its text; the
                         min/max heights reproduce the old 2–4 row autosize. -->
                    <Textarea
                      v-if="editingPage"
                      :model-value="editForm.summary"
                      class="bg-card dark:bg-card focus-visible:border-primary mt-2.5 max-h-[110px] min-h-[65px] resize-y overflow-y-auto rounded-lg px-3 py-2.5 text-sm leading-[1.6] focus-visible:ring-2 focus-visible:ring-[rgba(7,192,95,0.1)] md:text-sm"
                      :placeholder="$t('knowledgeEditor.wikiBrowser.editSummaryPlaceholder')"
                      @update:model-value="(v) => (editForm.summary = String(v))"
                    />
                  </div>
                  <div class="flex shrink-0 flex-col items-end gap-2">
                    <div
                      class="flex shrink-0 items-center gap-0.5"
                      role="toolbar"
                      :aria-label="$t('knowledgeEditor.wikiBrowser.pageActions')"
                    >
                      <template v-if="editingPage">
                        <Button size="sm" :disabled="savingPage" @click="savePageEdit()">
                          <Loader2Icon v-if="savingPage" class="animate-spin" />
                          {{ $t("common.save") }}
                        </Button>
                        <Button variant="ghost" size="sm" :disabled="savingPage" @click="cancelEditPage">
                          {{ $t("common.cancel") }}
                        </Button>
                      </template>
                      <template v-else>
                        <Tooltip v-if="props.canEdit">
                          <TooltipTrigger as-child>
                            <button
                              type="button"
                              data-slot="wiki-action-btn"
                              :class="[readerActionClass, 'hover:text-foreground']"
                              :aria-label="$t('knowledgeEditor.wikiBrowser.editBtn')"
                              @click="startEditPage"
                            >
                              <PencilIcon class="size-4" />
                            </button>
                          </TooltipTrigger>
                          <TooltipContent side="top">{{ $t("knowledgeEditor.wikiBrowser.editBtn") }}</TooltipContent>
                        </Tooltip>
                        <Tooltip>
                          <TooltipTrigger as-child>
                            <button
                              type="button"
                              data-slot="wiki-action-btn"
                              :class="[readerActionClass, 'hover:text-foreground']"
                              :aria-label="$t('knowledgeEditor.wikiBrowser.historyBtn')"
                              @click="openRevisionDrawer"
                            >
                              <HistoryIcon class="size-4" />
                            </button>
                          </TooltipTrigger>
                          <TooltipContent side="top">{{ $t("knowledgeEditor.wikiBrowser.historyBtn") }}</TooltipContent>
                        </Tooltip>
                        <Tooltip>
                          <TooltipTrigger as-child>
                            <button
                              type="button"
                              data-slot="wiki-action-btn"
                              :class="[readerActionClass, 'hover:text-foreground']"
                              :aria-label="$t('knowledgeEditor.wikiBrowser.viewInGraph')"
                              @click="emit('view-graph', selectedPage.slug)"
                            >
                              <ChartScatterIcon class="size-4" />
                            </button>
                          </TooltipTrigger>
                          <TooltipContent side="top">{{
                            $t("knowledgeEditor.wikiBrowser.viewInGraph")
                          }}</TooltipContent>
                        </Tooltip>
                        <!-- Delete confirmation, anchored to the button like the
                             popconfirm it replaces. -->
                        <Popover v-if="props.canEdit" v-model:open="deletePageConfirmOpen">
                          <Tooltip>
                            <TooltipTrigger as-child>
                              <PopoverTrigger as-child>
                                <button
                                  type="button"
                                  data-slot="wiki-action-btn"
                                  :class="[readerActionClass, 'hover:text-destructive']"
                                  :aria-label="$t('knowledgeEditor.wikiBrowser.deletePageBtn')"
                                >
                                  <Trash2Icon class="size-4" />
                                </button>
                              </PopoverTrigger>
                            </TooltipTrigger>
                            <TooltipContent side="top">{{
                              $t("knowledgeEditor.wikiBrowser.deletePageBtn")
                            }}</TooltipContent>
                          </Tooltip>
                          <PopoverContent side="bottom" align="end" class="w-auto max-w-[320px] gap-3 p-3">
                            <div class="flex items-start gap-2">
                              <CircleAlertIcon class="text-destructive mt-0.5 size-4 shrink-0" />
                              <span class="text-foreground leading-[1.6] break-words">{{
                                $t("knowledgeEditor.wikiBrowser.deletePageConfirm", { title: selectedPage.title })
                              }}</span>
                            </div>
                            <div class="flex justify-end gap-2">
                              <Button size="sm" variant="outline" @click="deletePageConfirmOpen = false">
                                {{ $t("common.cancel") }}
                              </Button>
                              <Button size="sm" variant="destructive" @click="onConfirmDeletePage">
                                {{ $t("common.confirm") }}
                              </Button>
                            </div>
                          </PopoverContent>
                        </Popover>
                      </template>
                    </div>
                    <div v-if="!editingPage" class="flex max-w-[220px] flex-col items-end gap-1">
                      <span
                        class="text-placeholder inline-flex items-center gap-1 overflow-hidden text-xs leading-[1.4] text-ellipsis whitespace-nowrap"
                      >
                        <ClockIcon class="size-3.5 shrink-0" />
                        {{ formatDate(selectedPage.updated_at) }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Content -->
              <div
                v-if="!editingPage"
                ref="readerBodyRef"
                class="wiki-reader-body"
                v-html="renderedContent"
                @click="handleContentClick"
              ></div>

              <!-- Inline markdown editor (canEdit only). Saves are guarded by
                   the page version captured at edit start; the backend answers
                   409 when someone (or the pipeline) edited in between. -->
              <div v-else class="mt-4 flex flex-col gap-3">
                <!-- min/max heights reproduce the old 16–40 row autosize. -->
                <Textarea
                  :model-value="editForm.content"
                  class="bg-card dark:bg-card focus-visible:border-primary max-h-[976px] min-h-[405px] resize-y overflow-y-auto rounded-lg px-3.5 py-3 font-[family-name:var(--td-font-family-mono,monospace)] text-sm leading-[1.7] focus-visible:ring-2 focus-visible:ring-[rgba(7,192,95,0.1)] md:text-sm"
                  :placeholder="$t('knowledgeEditor.wikiBrowser.editContentPlaceholder')"
                  @update:model-value="(v) => (editForm.content = String(v))"
                />
                <Alert
                  v-if="editConflictVersion !== null"
                  class="border-transparent bg-[var(--td-warning-color-light)]"
                >
                  <CircleAlertIcon class="text-warning" />
                  <AlertDescription class="text-foreground flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
                    <span>{{ t("knowledgeEditor.wikiBrowser.editConflictHint", { ver: editConflictVersion }) }}</span>
                    <span class="inline-flex items-center gap-3">
                      <Button variant="link" class="h-auto p-0" @click="reloadLatestIntoEditor">
                        {{ $t("knowledgeEditor.wikiBrowser.editConflictReload") }}
                      </Button>
                      <Button variant="link" class="text-warning h-auto p-0" @click="overwriteSavePage">
                        {{ $t("knowledgeEditor.wikiBrowser.editConflictOverwrite") }}
                      </Button>
                    </span>
                  </AlertDescription>
                </Alert>
              </div>

              <!-- Page footer: backlinks + sources -->
              <footer
                v-if="!editingPage && (selectedPage.in_links?.length || parsedSourceRefs.length)"
                class="border-border mt-8 flex flex-col gap-2.5 border-t pt-[18px]"
              >
                <div v-if="selectedPage.in_links?.length" class="flex items-baseline gap-4 text-[13px] leading-[1.65]">
                  <span class="text-placeholder flex-[0_0_64px] text-xs">{{
                    $t("knowledgeEditor.wikiBrowser.linkedFrom")
                  }}</span>
                  <span class="flex min-w-0 flex-1 flex-wrap items-center gap-x-5 gap-y-2">
                    <a
                      v-for="link in selectedPage.in_links"
                      :key="'in-' + link"
                      href="#"
                      :class="footerLinkClass"
                      @click.prevent="navigateToSlug(link)"
                    >
                      {{ slugDisplayName(link) }}
                    </a>
                  </span>
                </div>
                <div v-if="parsedSourceRefs.length" class="flex items-baseline gap-4 text-[13px] leading-[1.65]">
                  <span class="text-placeholder flex-[0_0_64px] text-xs">{{
                    $t("knowledgeEditor.wikiBrowser.sources")
                  }}</span>
                  <span class="flex min-w-0 flex-1 flex-wrap items-center gap-x-5 gap-y-2">
                    <a
                      v-for="ref in parsedSourceRefs"
                      :key="ref.id"
                      href="#"
                      :class="footerLinkClass"
                      @click.prevent="emit('open-source-doc', ref.id)"
                    >
                      {{ ref.title }}
                    </a>
                  </span>
                </div>
              </footer>
            </template>

            <!-- System view: index overview rendered as markdown. Starts
                 with intro only; an IntersectionObserver-driven sentinel
                 at the bottom auto-appends the next directory section
                 (Summary → Entity → Concept → …) as the user scrolls
                 near the end. [[wiki-link]] clicks inside the rendered
                 body are handled by handleContentClick just like a
                 regular wiki page. -->
            <template v-else-if="activeSystemView === 'index'">
              <div class="mb-6">
                <h2 class="text-foreground m-0 flex min-w-0 items-center gap-2 text-[26px] leading-[1.3] font-semibold">
                  {{ $t("knowledgeEditor.wikiBrowser.indexTitle") }}
                </h2>
                <div class="flex min-w-0 flex-wrap items-center gap-2.5">
                  <span
                    :class="[
                      'inline-flex h-[22px] items-center rounded-sm border px-2 text-xs',
                      getTypeTagClass('index'),
                    ]"
                  >
                    {{ $t("knowledgeEditor.wikiBrowser.indexOverviewTag") }}
                  </span>
                </div>
              </div>
              <div
                v-if="indexLoading && !indexMarkdown"
                class="flex flex-col items-center justify-center px-5 py-[60px] text-center"
              >
                <p class="text-muted-foreground m-0 mb-1 text-sm font-medium">
                  {{ $t("knowledgeEditor.wikiBrowser.loading") }}
                </p>
              </div>
              <template v-else-if="indexMarkdown">
                <div
                  ref="indexBodyRef"
                  class="wiki-reader-body"
                  v-html="renderedIndexMarkdown"
                  @click="handleContentClick"
                ></div>
                <div
                  v-if="indexHasMore"
                  ref="indexSentinelRef"
                  class="text-placeholder flex min-h-8 items-center justify-center pt-4 pb-6 text-[13px]"
                >
                  <span v-if="indexLoading" class="opacity-70">
                    {{ $t("knowledgeEditor.wikiBrowser.loading") }}
                  </span>
                </div>
              </template>
              <div
                v-else-if="!indexLoading"
                class="flex flex-col items-center justify-center px-5 py-[60px] text-center"
              >
                <p class="text-muted-foreground m-0 mb-1 text-sm font-medium">
                  {{ $t("knowledgeEditor.wikiBrowser.indexEmpty") }}
                </p>
              </div>
            </template>

            <!-- No page selected -->
            <div v-else class="flex flex-col items-center justify-center px-5 py-[60px] text-center">
              <div class="bg-muted text-placeholder mb-4 flex size-16 items-center justify-center rounded-full">
                <EyeIcon class="size-12" />
              </div>
              <p class="text-muted-foreground m-0 mb-1 text-sm font-medium" v-if="hasContentPages">
                {{ $t("knowledgeEditor.wikiBrowser.selectPageHint") }}
              </p>
              <template v-else>
                <p class="text-muted-foreground m-0 mb-1 text-sm font-medium">
                  {{ $t("knowledgeEditor.wikiBrowser.emptyTitle") }}
                </p>
                <p class="text-placeholder m-0 text-[13px]">{{ $t("knowledgeEditor.wikiBrowser.emptyDesc") }}</p>
              </template>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- Image Preview -->
    <Teleport to="body">
      <picturePreview
        v-if="imagePreviewVisible"
        :reviewImg="imagePreviewVisible"
        :reviewUrl="imagePreviewUrl"
        @closePreImg="closeImagePreview"
      />
    </Teleport>

    <!-- Lint report: the wiki's structural problems, computed on request -->
    <SettingDrawer
      v-model:visible="showLintDrawer"
      :title="$t('knowledgeEditor.wikiBrowser.lintTitle')"
      width="520px"
      :resizable="false"
      hide-footer
    >
      <div class="flex min-h-0 flex-1 flex-col gap-3 px-3 py-2" data-testid="wiki-lint">
        <div v-if="lintLoading" class="text-muted-foreground flex items-center gap-2 p-6 text-sm">
          <Loader2Icon class="size-4 animate-spin" />
          {{ $t("knowledgeEditor.wikiBrowser.lintRunning") }}
        </div>
        <Alert v-else-if="lintError" variant="destructive">
          <AlertDescription>{{ $t("knowledgeEditor.wikiBrowser.lintFailed") }}</AlertDescription>
        </Alert>
        <template v-else-if="lintReport">
          <div class="border-border bg-muted flex items-center gap-3 rounded-md border px-4 py-3">
            <span class="text-foreground text-2xl font-semibold tabular-nums">{{ lintReport.health_score }}</span>
            <div class="flex min-w-0 flex-1 flex-col">
              <span class="text-foreground text-sm font-medium">{{ $t("knowledgeEditor.wikiBrowser.lintScore") }}</span>
              <span class="text-placeholder text-xs">{{
                lintIssues.length === 0
                  ? $t("knowledgeEditor.wikiBrowser.lintClean")
                  : $t("knowledgeEditor.wikiBrowser.lintCount", { count: lintIssues.length })
              }}</span>
            </div>
            <Button v-if="props.canEdit && fixableCount > 0" size="sm" :disabled="autoFixing" @click="runAutoFix">
              <Loader2Icon v-if="autoFixing" class="animate-spin" />
              <WrenchIcon v-else />
              {{ $t("knowledgeEditor.wikiBrowser.lintAutoFix", { count: fixableCount }) }}
            </Button>
          </div>
          <div class="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto">
            <div
              v-for="(issue, idx) in lintIssues"
              :key="`${issue.type}:${issue.page_slug}:${issue.target_slug ?? ''}:${idx}`"
              :class="lintItemClass"
            >
              <div class="flex flex-wrap items-center gap-2">
                <span :class="lintSeverityClass(issue.severity)">{{ lintTypeLabel(issue.type) }}</span>
                <span v-if="issue.auto_fixable" class="text-placeholder text-xs">{{
                  $t("knowledgeEditor.wikiBrowser.lintFixable")
                }}</span>
              </div>
              <button
                type="button"
                class="text-primary w-fit cursor-pointer text-left text-sm font-medium hover:underline"
                @click="openLintPage(issue.page_slug)"
              >
                {{ slugDisplayName(issue.page_slug) }}
              </button>
              <div :class="lintDescClass">{{ issue.description }}</div>
            </div>
          </div>
        </template>
      </div>
    </SettingDrawer>

    <!-- Revision history drawer -->
    <WikiRevisionDrawer
      v-model:visible="showRevisionDrawer"
      :kb-id="props.knowledgeBaseId"
      :slug="selectedPage?.slug || ''"
      :current-page="selectedPage"
      :can-edit="props.canEdit"
      @reverted="onPageReverted"
    />

    <!-- Create page dialog -->
    <Dialog :open="showCreatePageDialog" @update:open="(v) => (showCreatePageDialog = v)">
      <DialogContent class="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>{{ $t("knowledgeEditor.wikiBrowser.newPageTitle") }}</DialogTitle>
        </DialogHeader>
        <div class="flex flex-col gap-3.5">
          <div class="flex flex-col gap-1.5">
            <Label for="wiki-create-page-title" class="text-muted-foreground text-[13px] font-normal">{{
              $t("knowledgeEditor.wikiBrowser.newPageTitleLabel")
            }}</Label>
            <Input
              id="wiki-create-page-title"
              :model-value="createPageForm.title"
              :placeholder="$t('knowledgeEditor.wikiBrowser.newPageTitlePlaceholder')"
              @update:model-value="onCreatePageTitleInput"
            />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="wiki-create-page-slug" class="text-muted-foreground text-[13px] font-normal">{{
              $t("knowledgeEditor.wikiBrowser.newPageSlugLabel")
            }}</Label>
            <Input
              id="wiki-create-page-slug"
              :model-value="createPageForm.slug"
              :placeholder="$t('knowledgeEditor.wikiBrowser.newPageSlugPlaceholder')"
              @update:model-value="onCreatePageSlugInput"
            />
            <div class="text-placeholder text-xs">{{ $t("knowledgeEditor.wikiBrowser.newPageSlugHint") }}</div>
          </div>
          <div class="flex flex-col gap-1.5">
            <Label class="text-muted-foreground text-[13px] font-normal">{{
              $t("knowledgeEditor.wikiBrowser.newPageTypeLabel")
            }}</Label>
            <Select
              :model-value="createPageForm.pageType"
              @update:model-value="(v) => (createPageForm.pageType = String(v ?? ''))"
            >
              <SelectTrigger class="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="concept">{{ $t("knowledgeEditor.wikiBrowser.filterConcept") }}</SelectItem>
                <SelectItem value="entity">{{ $t("knowledgeEditor.wikiBrowser.filterEntity") }}</SelectItem>
                <SelectItem value="synthesis">{{ $t("knowledgeEditor.wikiBrowser.filterSynthesis") }}</SelectItem>
                <SelectItem value="comparison">{{ $t("knowledgeEditor.wikiBrowser.filterComparison") }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="wiki-create-page-content" class="text-muted-foreground text-[13px] font-normal">{{
              $t("knowledgeEditor.wikiBrowser.newPageContentLabel")
            }}</Label>
            <!-- min/max heights reproduce the old 6–16 row autosize. -->
            <Textarea
              id="wiki-create-page-content"
              :model-value="createPageForm.content"
              class="max-h-[362px] min-h-[142px] overflow-y-auto"
              :placeholder="$t('knowledgeEditor.wikiBrowser.editContentPlaceholder')"
              @update:model-value="(v) => (createPageForm.content = String(v))"
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="showCreatePageDialog = false">{{ $t("common.cancel") }}</Button>
          <Button :disabled="creatingPage" @click="submitCreatePage">
            <Loader2Icon v-if="creatingPage" class="animate-spin" />
            {{ $t("common.confirm") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- In-place move confirmation, anchored at the drop point. Confirming runs
         the actual move API; cancelling discards the staged move. -->
    <teleport to="body">
      <div v-if="pendingMove" class="fixed inset-0 z-[3500]" @click="cancelPendingMove">
        <!-- x/y are the drop coords; the translate nudges the card just
             below-right of the cursor, and the max-width keeps it on-screen
             for edge drops. -->
        <div
          class="bg-card border-border fixed max-w-[min(392px,calc(100vw-24px))] min-w-[300px] translate-x-2 translate-y-2 rounded-xl border-[0.5px] px-4 py-3.5 shadow-[0_0_0_0.5px_rgba(0,0,0,0.03),0_2px_4px_rgba(0,0,0,0.04),0_8px_24px_rgba(0,0,0,0.1)] backdrop-blur-[20px] backdrop-saturate-[180%] dark:border-[rgba(255,255,255,0.08)] dark:bg-[rgba(36,36,36,0.92)] dark:shadow-[0_0_0_0.5px_rgba(255,255,255,0.05),0_2px_4px_rgba(0,0,0,0.12),0_8px_32px_rgba(0,0,0,0.28)]"
          :style="{ left: `${pendingMove.x}px`, top: `${pendingMove.y}px` }"
          @click.stop
        >
          <div class="text-foreground mb-3 text-[15px] leading-[1.35] font-semibold">
            {{ $t("knowledgeEditor.wikiBrowser.moveConfirmTitle") }}
          </div>
          <div class="text-foreground pb-1 text-sm leading-[1.6] break-words">
            {{ $t("knowledgeEditor.wikiBrowser.moveConfirm", { target: pendingMove.targetLabel }) }}
          </div>
          <div class="mt-4 flex justify-end gap-2">
            <Button variant="outline" @click="cancelPendingMove">
              {{ $t("common.cancel") }}
            </Button>
            <Button @click="confirmPendingMove">
              {{ $t("common.confirm") }}
            </Button>
          </div>
        </div>
      </div>
    </teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, reactive, onMounted, onUnmounted, watch, nextTick, type Component } from "vue";
import { useRoute } from "vue-router";
import { useI18n } from "vue-i18n";
import { marked } from "marked";
import { MessagePlugin } from "tdesign-vue-next";
import { RecycleScroller } from "vue-virtual-scroller";
import { PopoverArrow } from "reka-ui";
import {
  ArrowLeftIcon,
  BlendIcon,
  ChartScatterIcon,
  CheckIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  CircleAlertIcon,
  CircleHelpIcon,
  CircleXIcon,
  ClockIcon,
  EyeIcon,
  EyeOffIcon,
  FileCodeIcon,
  FileIcon,
  FilePlusIcon,
  FileQuestionMarkIcon,
  FocusIcon,
  FolderPlusIcon,
  HistoryIcon,
  LayoutGridIcon,
  LightbulbIcon,
  LinkIcon,
  ListIcon,
  ListTreeIcon,
  Loader2Icon,
  PencilIcon,
  SearchIcon,
  ShieldCheckIcon,
  TableOfContentsIcon,
  TagIcon,
  Trash2Icon,
  Undo2Icon,
  UserIcon,
  WrenchIcon,
  XIcon,
} from "@lucide/vue";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import { hydrateProtectedFileImages, sanitizeMarkdownHTML } from "@/utils/security";
import type { ProtectedFileAccessContext } from "@/utils/protectedFileAccess";
import picturePreview from "@/components/picture-preview.vue";
import WikiFolderActions from "./WikiFolderActions.vue";
import WikiRevisionDrawer from "./WikiRevisionDrawer.vue";
import { expandedWikiDirectoryPaths, expandWikiDirectoryPath } from "./wikiDirectoryState";
import { getKnowledgeDetails } from "@/api/knowledge-base";
import {
  listWikiPages,
  listWikiFolders,
  createWikiFolder,
  updateWikiFolder,
  deleteWikiFolder,
  moveWikiPage,
  createWikiPage,
  updateWikiPage,
  deleteWikiPage,
  getWikiPage,
  getWikiIndex,
  getWikiGraph,
  getWikiStats,
  searchWikiPages,
  lintWiki,
  autoFixWiki,
  type WikiPage,
  type WikiFolderNode,
  type WikiGraphData,
  type WikiStats,
  type WikiLintIssue,
  type WikiLintReport,
  type WikiIndexGroup,
  type WikiIndexEntryDTO,
} from "@/api/wiki";

const route = useRoute();

const { t } = useI18n();

const props = defineProps<{
  knowledgeBaseId: string;
  view?: "browser" | "graph";
  // canEdit 由父组件 KnowledgeBase.vue 透传（与 canEdit computed 同源）。
  // 控制写操作按钮（编辑、移动、Lint 自动修复等）的可见性，
  // 对应后端 g.OwnedWikiKBOrAdmin() 守卫（KB creator OR Admin+）。
  // 父组件没传时按 false 兜底，避免漏 gate。
  canEdit?: boolean;
  // Opens the lint report as soon as the browser mounts. The knowledge-health
  // view links here for the wiki's structural check instead of running the
  // same (whole-wiki) lint a second time.
  openLintOnMount?: boolean;
}>();

const emit = defineEmits<{
  (e: "open-source-doc", knowledgeId: string): void;
  (e: "status-change", payload: { pendingTasks: number; isActive: boolean }): void;
  (e: "view-graph", slug: string): void;
}>();

// Files referenced by wiki content are fetched through the knowledge-base
// file proxy, which authorises by access to this KB (see
// ProtectedFileAccessContext).
const kbFileAccess = computed<ProtectedFileAccessContext>(() => ({
  mode: "knowledgeBase",
  kbId: props.knowledgeBaseId,
}));
const pages = ref<WikiPage[]>([]);
const selectedPage = ref<WikiPage | null>(null);

// Per-type pagination state for the sidebar. 4万-page wikis used to load
// the entire page list into `pages.value` at startup (50 pages of 500 =
// 25k rows of JSON fetched even when the user only wants to glance at
// one type). Instead we now keep one bucket per page_type and lazy-load
// them on demand:
//
//   * Each bucket tracks loaded items, next page cursor, total count
//     (from the backend), and whether a fetch is currently in flight.
//   * Tabs (summary/entity/concept/…) only request their bucket when
//     the user expands that group, and more items are pulled when the
//     virtualized scroller nears the bottom.
//
// `pages.value` is still kept and contains the union of all loaded
// items, purely as a fallback lookup table for `slugDisplayName()` and
// similar "I saw this title somewhere" paths.
interface PageTypeBucket {
  items: WikiPage[];
  nextPage: number; // page cursor for the next fetch, 1-based
  total: number; // KB-wide count reported by the backend for this type
  loading: boolean;
  initialized: boolean; // true once the first page has been fetched
  categoryPaths: Array<{ path: string[]; count: number }>;
  // folderIdByPath maps a materialized folder path ("AI/LLM") to its stable
  // wiki_folders id, populated as folder levels load. Drag-and-drop and the
  // directory rows resolve a folder's id through this map.
  folderIdByPath: Record<string, string>;
  categoriesLoading: boolean;
  categoriesInitialized: boolean;
  // categoryPages tracks the directory-skeleton pagination per parent level,
  // keyed by directoryPathKey(type, parentPath). Each level is paged in on
  // demand so a folder with thousands of siblings doesn't arrive in one shot.
  categoryPages: Record<string, DirectoryCategoryState>;
  directoryPages: Record<string, DirectoryPageState>;
  flatItems: WikiPage[];
  flatNextPage: number;
  flatTotal: number;
  flatLoading: boolean;
  flatInitialized: boolean;
}
interface DirectoryPageState {
  nextPage: number;
  total: number;
  loading: boolean;
  initialized: boolean;
}
interface DirectoryCategoryState {
  nextPage: number; // next directory page to fetch, 1-based
  totalPages: number;
  loading: boolean;
  initialized: boolean;
}
type WikiTreeRow =
  | {
      kind: "directory";
      rowKey: string;
      pathKey: string;
      folderId: string;
      path: string[];
      label: string;
      depth: number;
      count: number;
      hasChildren: boolean;
      collapsed: boolean;
    }
  | { kind: "load-more"; rowKey: string; type: string; path: string[]; depth: number; loading: boolean }
  | { kind: "page"; rowKey: string; page: WikiPage; depth: number };
interface WikiTreeDirectory {
  label: string;
  path: string[];
  dirs: Map<string, WikiTreeDirectory>;
  pages: WikiPage[];
  count: number;
}
const pagesByType = ref<Record<string, PageTypeBucket>>({});
const collapsedDirectories = ref<Set<string>>(new Set());
const touchedDirectories = ref<Set<string>>(new Set());
// Index view state. The reader renders an incrementally-built markdown
// string rather than a structured list — opening the view loads intro
// only, and "Load more" appends one directory section at a time in a
// fixed order (Summary → Entity → Concept → Synthesis → Comparison).
// Once the last section exhausts its pages, indexHasMore flips off.
//
// We deliberately avoid keeping a parallel structured list + a parallel
// markdown buffer; the markdown is the single source of truth the reader
// renders, and [[wiki-link]] clicks flow through the same
// handleContentClick as regular page bodies.
const indexMarkdown = ref("");
const indexLoading = ref(false);
const indexAvailable = ref(false);
// Per-section pagination cursor. Empty string = not yet loaded; empty
// cursor AFTER a load = that section is exhausted. `indexSectionIdx`
// tracks which section in INDEX_SECTION_ORDER is "next to load" — we
// advance to the following section only when the current one runs out.
const indexSections = ref<Record<string, { loaded: boolean; cursor: string; total: number }>>({});
const indexSectionIdx = ref(0);
const indexBodyRef = ref<HTMLElement | null>(null);
const indexSentinelRef = ref<HTMLElement | null>(null);
let indexObserver: IntersectionObserver | null = null;

// Order matters: Summary first (these are the document-level pages the
// user most often wants to see), then the LLM-derived ones. Matches the
// plan's "intro then Summary → Entity → Concept → …" progression.
const INDEX_SECTION_ORDER = ["summary", "entity", "concept", "synthesis", "comparison"] as const;
// activeSystemView lets the reader toggle between a regular wiki page and the
// virtual index overview. Entering the index clears the selected page, and
// picking a page clears the system view flag.
const activeSystemView = ref<"" | "index">("");

// When the user types into the search box we leave pagination mode and
// show a flat result list instead. Bucketed state is preserved behind
// the scenes so clearing the query can snap back without re-fetching.
const searchResults = ref<WikiPage[] | null>(null);
// The lint report is fetched each time its drawer opens: it is computed from
// every page of the wiki, stored nowhere, and a stale copy would list
// problems that were fixed a minute ago.
const showLintDrawer = ref(false);
const lintReport = ref<WikiLintReport | null>(null);
const lintLoading = ref(false);
const lintError = ref(false);
const autoFixing = ref(false);
const lintIssues = computed<WikiLintIssue[]>(() => lintReport.value?.issues ?? []);
const fixableCount = computed(() => lintIssues.value.filter((i) => i.auto_fixable).length);
const stats = ref<WikiStats | null>(null);
const graphData = ref<WikiGraphData | null>(null);
const searchQuery = ref("");
const graphSearchValue = ref("");
const graphRef = ref<HTMLElement | null>(null);
const readerBodyRef = ref<HTMLElement | null>(null);
const drawerBodyRef = ref<HTMLElement | null>(null);
const loading = ref(false);
const graphLoading = ref(false);
const graphReady = ref(false);
const showArrows = ref(true);

// Graph filtering
const graphFilterTypes = ref<Set<string>>(
  new Set(["summary", "entity", "concept", "synthesis", "comparison", "index"]),
);

// Graph slicing state. The backend caps an overview fetch at 500 nodes —
// tens-of-thousands-page wikis would otherwise crash the browser trying to
// render 100k SVG elements. `graphMode` tracks whether we're on the
// overview landing or drilled into an ego neighborhood so the UI can offer
// "back to overview" and show the truncation hint.
const graphMode = ref<"overview" | "ego">("overview");
const graphCenter = ref<string>("");
const GRAPH_OVERVIEW_LIMIT = 500;
const GRAPH_EGO_LIMIT = 500;
const GRAPH_EGO_DEFAULT_DEPTH = 1;

async function runLint() {
  lintLoading.value = true;
  lintError.value = false;
  try {
    lintReport.value = await lintWiki(props.knowledgeBaseId);
  } catch (e) {
    console.error("Wiki lint failed:", e);
    lintReport.value = null;
    lintError.value = true;
  } finally {
    lintLoading.value = false;
  }
}

watch(showLintDrawer, (open) => {
  if (open) void runLint();
});

// Registered after the watcher above so that opening on mount goes through it.
watch(
  () => props.openLintOnMount,
  (open) => {
    if (open) showLintDrawer.value = true;
  },
  { immediate: true },
);

async function runAutoFix() {
  autoFixing.value = true;
  try {
    const res = await autoFixWiki(props.knowledgeBaseId);
    MessagePlugin.success(t("knowledgeEditor.wikiBrowser.lintFixed", { count: res?.fixed ?? 0 }));
  } catch (e) {
    console.error("Wiki auto-fix failed:", e);
    MessagePlugin.error(t("knowledgeEditor.wikiBrowser.lintFixFailed"));
  } finally {
    autoFixing.value = false;
  }
  // What was fixed changed the pages; the report and the counts follow.
  await runLint();
  loadStats();
  if (selectedPage.value) void refreshSelectedPage();
}

async function openLintPage(slug: string) {
  showLintDrawer.value = false;
  if (props.view === "graph") {
    handleGraphSearchSelect(slug);
  } else {
    await navigateToSlug(slug);
  }
}

// toggleGraphFilterType flips a page_type in the active allow-list and
// refetches the graph from the server. Client-side DOM hiding used to
// suffice when the canvas contained every page, but once we cap the
// overview at top-500 by link_count, hiding the "summary" type just
// blanks out most of the canvas without surfacing the next 500 nodes
// that would qualify under the narrowed filter. Re-asking the server
// keeps the top-N always relevant to what the user said they wanted to
// see, at the cost of one network round-trip per toggle.
async function toggleGraphFilterType(type: string) {
  const newSet = new Set(graphFilterTypes.value);
  if (newSet.has(type)) {
    newSet.delete(type);
  } else {
    newSet.add(type);
  }
  graphFilterTypes.value = newSet;

  // Dismiss any highlight/drawer that no longer matches the new filter
  // before we repaint, otherwise the old selection can linger against
  // freshly-rendered elements that were never built for it.
  graphHighlightSlug.value = null;
  if (
    graphSelectedSlug.value &&
    !newSet.has(graphData.value?.nodes.find((n) => n.slug === graphSelectedSlug.value)?.page_type || "")
  ) {
    graphSelectedSlug.value = null;
    graphDrawerVisible.value = false;
  }

  if (graphMode.value === "ego" && graphCenter.value) {
    await loadEgoGraph(graphCenter.value);
  } else {
    await loadGraph();
  }
}

// Fit graph to view
function fitGraphToView() {
  if (!graphReady.value || !graphPanZoomRef || !graphRef.value || graphNodes.length === 0) return;

  const container = graphRef.value;
  const width = container.clientWidth;
  const height = container.clientHeight;

  // Find bounding box of all VISIBLE nodes
  let minX = Infinity,
    minY = Infinity,
    maxX = -Infinity,
    maxY = -Infinity;
  let visibleCount = 0;

  for (const node of graphNodes) {
    // Every node in graphNodes is a visible candidate now that filtering
    // is server-side — no need to recheck the client-side allow-list.
    minX = Math.min(minX, node.x);
    minY = Math.min(minY, node.y);
    maxX = Math.max(maxX, node.x);
    maxY = Math.max(maxY, node.y);
    visibleCount++;
  }

  if (visibleCount === 0) return; // No visible nodes

  // Calculate center of the bounding box
  const cx = (minX + maxX) / 2;
  const cy = (minY + maxY) / 2;

  // Calculate scale to fit the bounding box (with some padding)
  const padding = 60;
  const boxWidth = Math.max(maxX - minX, 100) + padding * 2;
  const boxHeight = Math.max(maxY - minY, 100) + padding * 2;

  const scaleX = width / boxWidth;
  const scaleY = height / boxHeight;
  const targetScale = Math.max(0.2, Math.min(2, Math.min(scaleX, scaleY))); // Limit scale between 0.2 and 2

  // Offset center if drawer is open
  const targetCx = width / 2 - (graphDrawerVisible.value ? 240 : 0);
  const targetCy = height / 2;

  // Target translation
  const targetTx = targetCx - cx * targetScale;
  const targetTy = targetCy - cy * targetScale;

  graphPanZoomRef.flyTo(targetTx, targetTy, targetScale, 600);
}

const graphDrawerVisible = ref(false);
const graphDrawerPage = ref<WikiPage | null>(null);
const navHistory = ref<WikiPage[]>([]);
// navFromSystemView remembers that the user was viewing the Index when they
// clicked into a slug, so goBack can restore it
// once the page-level history stack is empty. We keep this parallel to
// navHistory rather than widening its element type — navHistory is
// consumed everywhere as `WikiPage[]` and that contract stays cleaner
// if the system-view sentinel lives in its own ref.
const navFromSystemView = ref<"" | "index">("");

// typeOrder drives the order of groups in the sidebar. Keep in sync
// with WIKI_PAGE_TYPES on the backend; unknown types fall through to
// the "other" bucket at the bottom of groupedPages.

// Entity and concept pages look and behave alike, so the sidebar merges all
// non-summary content types under a single "knowledge" tab and distinguishes
// the individual page types by icon instead. Summary keeps its own tab and is
// shown after the knowledge tab.
const KNOWLEDGE_TAB = "knowledge";
const KNOWLEDGE_TYPES = ["entity", "concept", "synthesis", "comparison"];
// CONTENT_TABS are the sidebar tabs in display order: the merged knowledge tab
// first, then summary. Each tab maps to its own bucket keyed by the tab id.
const CONTENT_TABS = [KNOWLEDGE_TAB, "summary"];

// tabPageTypes maps a sidebar tab onto the comma-separated page_type filter the
// backend expects. The knowledge tab folds every non-summary content type into
// one request so the server returns a single merged list and directory
// skeleton — no per-type fan-out on the client.
function tabPageTypes(tab: string): string {
  return tab === KNOWLEDGE_TAB ? KNOWLEDGE_TYPES.join(",") : tab;
}

// Pick the default sidebar tab in CONTENT_TABS display order (knowledge
// before summary), not whatever tab happened to finish loading first.
function preferredDefaultTab(tabs: Array<{ type: string }>): string {
  for (const tab of CONTENT_TABS) {
    if (tabs.some((t) => t.type === tab)) return tab;
  }
  return tabs[0]?.type || "";
}

// groupedPages projects the bucketed state into the shape the sidebar
// template expects: one {type, label, items, total, loading, hasMore}
// per displayed group. Groups with zero total are hidden (nothing to
// show) but groups with total > 0 but items.length === 0 still render
// so the collapse header can trigger a lazy fetch.
const groupedPages = computed(() => {
  type Group = {
    type: string;
    label: string;
    pages: WikiPage[];
    total: number;
    loading: boolean;
    hasMore: boolean;
  };
  const out: Group[] = [];
  const seen = new Set<string>();
  // Stats are reported per real page_type; the knowledge tab sums its members
  // so the count is available before the first page request completes.
  const statTotal = (tab: string) => {
    const byType = stats.value?.pages_by_type;
    if (!byType) return 0;
    if (tab === KNOWLEDGE_TAB) return KNOWLEDGE_TYPES.reduce((sum, t) => sum + (byType[t] || 0), 0);
    return byType[tab] || 0;
  };
  const push = (tab: string) => {
    const bucket = pagesByType.value[tab];
    if (!bucket) return;
    const total = bucket.total || statTotal(tab) || bucket.categoryPaths.length;
    if (total === 0) return;
    out.push({
      type: tab,
      label: getTypeLabel(tab),
      pages: bucket.items,
      total,
      loading: bucket.loading,
      hasMore: bucket.total > 0 && bucket.items.length < bucket.total,
    });
    seen.add(tab);
  };
  for (const tab of CONTENT_TABS) push(tab);
  // Any tabs present in the buckets but not handled above go last in insertion
  // order so the sidebar doesn't suddenly hide a future tab.
  for (const tab of Object.keys(pagesByType.value)) {
    if (seen.has(tab)) continue;
    if (tab === "index") continue;
    push(tab);
  }
  return out;
});

// hasContentPages is the sidebar's empty-state gate. The old version
// looked at `contentPages.length === 0`, which forced a full load to
// decide whether the wiki was truly empty. Now we check bucket totals
// reported by the backend — zero everywhere means no content pages.
const hasContentPages = computed(() => {
  for (const bucket of Object.values(pagesByType.value)) {
    if (bucket.total > 0 || bucket.categoryPaths.length > 0) return true;
  }
  return false;
});

// Pipeline ingest stores source_refs as bare knowledge IDs (see wiki_ingest_batch)
// so filenames do not leak into LLM citation strings. Resolve titles for display.
const sourceRefTitleCache = reactive<Record<string, string>>({});
let sourceRefTitleRequestSeq = 0;

function parseSourceRefEntry(ref: string): { id: string; title: string } {
  const pipeIdx = ref.indexOf("|");
  if (pipeIdx > 0) {
    return { id: ref.substring(0, pipeIdx), title: ref.substring(pipeIdx + 1) };
  }
  const cached = sourceRefTitleCache[ref];
  if (cached) {
    return { id: ref, title: cached };
  }
  return {
    id: ref,
    title: ref.length > 20 ? ref.substring(0, 8) + "..." : ref,
  };
}

const parsedSourceRefs = computed(() => {
  if (!selectedPage.value?.source_refs?.length) return [];
  return selectedPage.value.source_refs.map(parseSourceRefEntry);
});

async function hydrateSourceRefTitles(refs: string[]) {
  const ids = refs.filter((ref) => ref.indexOf("|") < 0 && !sourceRefTitleCache[ref]);
  if (!ids.length) return;
  const seq = ++sourceRefTitleRequestSeq;
  for (const id of ids) {
    try {
      const res = await getKnowledgeDetails(id);
      if (seq !== sourceRefTitleRequestSeq) return;
      const data = res.data;
      const title = data?.title || data?.file_name;
      if (title) sourceRefTitleCache[id] = title;
    } catch {
      // Keep truncated-ID fallback when the doc was deleted or is inaccessible.
    }
  }
}

watch(
  () => selectedPage.value?.source_refs,
  (refs) => {
    if (refs?.length) hydrateSourceRefTitles(refs);
  },
  { immediate: true },
);

// Rendered content for graph drawer
const graphDrawerContent = computed(() => {
  if (!graphDrawerPage.value) return "";
  return renderMarkdown(graphDrawerPage.value.content);
});

// graphDrawerNeighborStatus describes, for the currently open drawer page,
// how the canvas relates to the KB-wide neighborhood of the node. The
// accounting is subtler than a simple "shown vs link_count" because three
// different situations produce different interpretations of a gap:
//
//   ego center — the backend already returned every neighbor reachable
//     through BFS at depth 1+. Any difference between `link_count` and
//     the visible degree is pages that couldn't be traversed (dead refs,
//     type-filtered pages, soft-deleted neighbors), NOT pages we can
//     still fetch. Expanding or blooming from the center does nothing
//     useful, so we flag it as fullyExplored and disable the buttons.
//
//   ego non-center — difference IS "neighbors not yet loaded". The user
//     can bloom to pull them in. This is the main signal for the dashed
//     expansion ring.
//
//   overview — difference is "neighbors that didn't make top-500", and
//     the fix isn't bloom (overview doesn't bloom) but pivoting to ego.
//     We still disable Bloom (it's an ego-only op) but leave Expand
//     enabled so the user can drill down.
const graphDrawerNeighborStatus = computed(() => {
  const page = graphDrawerPage.value;
  if (!page) return null;
  const data = graphData.value;
  if (!data) return null;
  const node = data.nodes.find((n) => n.slug === page.slug);
  if (!node) {
    // Drawer is open on a page that isn't currently on the canvas (e.g.
    // the user just clicked a wiki-link that triggered an ego pivot and
    // we're between data update and re-render). Treat as unknown so the
    // button stays enabled — the pivot will populate neighbors shortly.
    return null;
  }
  // Undirected degree within the current subgraph. Both incoming and
  // outgoing edges count toward a visible neighbor, matching how
  // link_count is computed server-side (in+out).
  const neighbors = new Set<string>();
  for (const e of data.edges) {
    if (e.source === page.slug) neighbors.add(e.target);
    else if (e.target === page.slug) neighbors.add(e.source);
  }
  const visible = neighbors.size;
  const total = node.link_count || 0;
  // hidden can go negative in a rare corner case — a neighbor might be
  // visible via an edge that the link_count counter didn't know about
  // (e.g. a broken-link cleanup happened after the snapshot). Clamp.
  const hidden = Math.max(0, total - visible);
  const isEgoCenter = data.meta?.mode === "ego" && data.meta.center === page.slug;
  const isOverview = data.meta?.mode === "overview";
  return {
    visible,
    total,
    hidden,
    isEgoCenter,
    isOverview,
    // fullyExplored drives the disabled state of the expand/bloom buttons.
    // True when either there's genuinely nothing to load, or when we're
    // on the ego center and any remaining gap is unreachable (dead refs
    // / filtered out).
    fullyExplored: total === 0 || visible >= total || isEgoCenter,
  };
});

const graphDrawerNeighborHint = computed(() => {
  const status = graphDrawerNeighborStatus.value;
  if (!status) return "";
  if (status.total === 0) {
    return t("knowledgeEditor.wikiBrowser.neighborsNone");
  }
  if (status.visible >= status.total) {
    // All neighbors already visible. Expand still does something useful
    // though — it pivots the canvas to just this node's neighborhood,
    // giving the user a focused N-node view instead of wading through
    // the 500-node overview. Say so rather than sounding like a dead end.
    return t("knowledgeEditor.wikiBrowser.neighborsAllShown", { total: status.total });
  }
  if (status.isEgoCenter) {
    // hidden > 0 but can't be loaded — distinguish from "未加载".
    return t("knowledgeEditor.wikiBrowser.neighborsCenterUnreachable", {
      visible: status.visible,
      total: status.total,
      hidden: status.hidden,
    });
  }
  if (status.isOverview) {
    // hidden means "not in the top-500 subgraph"; bloom doesn't help
    // here, expand/pivot does.
    return t("knowledgeEditor.wikiBrowser.neighborsOverviewHidden", {
      visible: status.visible,
      total: status.total,
      hidden: status.hidden,
    });
  }
  return t("knowledgeEditor.wikiBrowser.neighborsProgress", {
    visible: status.visible,
    total: status.total,
    hidden: status.hidden,
  });
});

// graphDrawerCanBloom is true when clicking Bloom would actually add
// new nodes to the canvas. Bloom is additive so we only disable it when
// there's nothing to add: either the node is the ego center (BFS already
// gave us everything reachable) or every one of its neighbors is already
// on screen.
const graphDrawerCanBloom = computed(() => {
  const status = graphDrawerNeighborStatus.value;
  if (!status) return true;
  if (status.isEgoCenter) return false;
  return status.hidden > 0;
});

// graphFrontierCount powers the legend's "Grow frontier (N)" button. It
// counts nodes on the current ego canvas that the user can still expand
// outward from — matches the filter used by growFrontier() itself so
// the button count can never disagree with what the click actually
// expands. Hidden when 0 so the button disappears once the local
// neighborhood is fully explored (or only Index/Log super-nodes remain).
const graphFrontierCount = computed(() => {
  const data = graphData.value;
  if (!data || data.meta?.mode !== "ego") return 0;
  const visibleDegree = new Map<string, number>();
  for (const e of data.edges) {
    visibleDegree.set(e.source, (visibleDegree.get(e.source) ?? 0) + 1);
    visibleDegree.set(e.target, (visibleDegree.get(e.target) ?? 0) + 1);
  }
  let count = 0;
  const centerSlug = data.meta?.center || "";
  for (const n of data.nodes) {
    if (isFrontierCandidate(n, centerSlug, visibleDegree.get(n.slug) ?? 0)) {
      count += 1;
    }
  }
  return count;
});

const graphFamiliarCount = computed(() => graphData.value?.meta?.familiar_count || 0);

// graphStatusCard drives the little summary panel below the legend.
//
// The old design ("以 A 为中心 · 1 跳 · 7 个节点" / "showing 500 / 40000,
// click a node to expand neighbors") crammed four pieces of info into a
// single line of running prose — the most important bit (what page the
// user is focused on) got lost between the jargon ("1 跳") and the
// imperative tail ("click a node...").
//
// The card version separates the three jobs into visible slots:
//   header  → icon + short mode name, tells the user "am I looking at
//             the whole wiki or at one page's neighborhood"
//   primary → the noun that identifies the current view (page title in
//             ego mode, "X / Y 个节点" in overview)
//   secondary → optional subline with type badge / hint / progress
//
// We also resolve `meta.center` (a slug) to the actual page title via
// graphData.nodes so users see "北京市昌职…" instead of "entity/beijing-..."
// — a common complaint with the old hint.
// graphHelpRows is the content of the ? popup. Keeping it in a computed
// rather than the template lets us i18n each action/description in one
// place and also makes it trivially extensible — new shortcuts land as
// one row addition each rather than a full template rewrite. The order
// below is "most common → rarest"; users don't typically read past the
// first few rows.
const graphHelpRows = computed(() => [
  { action: t("knowledgeEditor.wikiBrowser.helpClickAction"), desc: t("knowledgeEditor.wikiBrowser.helpClickDesc") },
  {
    action: t("knowledgeEditor.wikiBrowser.helpDblClickAction"),
    desc: t("knowledgeEditor.wikiBrowser.helpDblClickDesc"),
  },
  {
    action: t("knowledgeEditor.wikiBrowser.helpShiftClickAction"),
    desc: t("knowledgeEditor.wikiBrowser.helpShiftClickDesc"),
  },
  {
    action: t("knowledgeEditor.wikiBrowser.helpHoverPlusAction"),
    desc: t("knowledgeEditor.wikiBrowser.helpHoverPlusDesc"),
  },
  { action: t("knowledgeEditor.wikiBrowser.helpDragAction"), desc: t("knowledgeEditor.wikiBrowser.helpDragDesc") },
  { action: t("knowledgeEditor.wikiBrowser.helpPanAction"), desc: t("knowledgeEditor.wikiBrowser.helpPanDesc") },
  { action: t("knowledgeEditor.wikiBrowser.helpZoomAction"), desc: t("knowledgeEditor.wikiBrowser.helpZoomDesc") },
]);

const graphStatusCard = computed((): { icon: Component; title: string; primary: string; secondary: string } | null => {
  const data = graphData.value;
  if (!data?.meta) return null;
  const meta = data.meta;
  if (meta.mode === "ego" && meta.center) {
    const centerNode = data.nodes.find((n) => n.slug === meta.center);
    const centerTitle = centerNode?.title || meta.center;
    const typeLabel = centerNode ? getTypeLabel(centerNode.page_type) : "";
    // Subtract 1 so the count means "related nodes" (excluding the
    // center itself) — matches how users count "connections". If the
    // count is 0 the center is an isolated page.
    const relatedCount = Math.max(0, meta.returned - 1);
    const secondaryParts: string[] = [];
    if (typeLabel) secondaryParts.push(typeLabel);
    secondaryParts.push(t("knowledgeEditor.wikiBrowser.cardRelatedNodes", { count: relatedCount }));
    return {
      icon: FocusIcon,
      title: t("knowledgeEditor.wikiBrowser.cardEgoTitle"),
      primary: centerTitle,
      secondary: secondaryParts.join(" · "),
    };
  }
  if (meta.mode === "overview") {
    const secondary = meta.truncated
      ? t("knowledgeEditor.wikiBrowser.cardOverviewHintTruncated")
      : t("knowledgeEditor.wikiBrowser.cardOverviewHintFull");
    return {
      icon: ChartScatterIcon,
      title: t("knowledgeEditor.wikiBrowser.cardOverviewTitle"),
      primary: t("knowledgeEditor.wikiBrowser.cardOverviewPrimary", {
        returned: meta.returned,
        total: meta.total,
      }),
      secondary,
    };
  }
  return null;
});

const imagePreviewVisible = ref(false);
const imagePreviewUrl = ref("");

function closeImagePreview() {
  imagePreviewVisible.value = false;
  imagePreviewUrl.value = "";
}

watch(graphDrawerContent, async () => {
  await nextTick();
  if (drawerBodyRef.value) {
    await hydrateProtectedFileImages(drawerBodyRef.value, kbFileAccess.value);
  }
});

function renderMarkdown(content: string): string {
  // Pre-process wiki links [[slug|name]] to custom HTML tags
  const preprocessed = content.replace(/\[\[([^\]]+)\]\]/g, (_, inner: string) => {
    const pipeIdx = inner.indexOf("|");
    const slug = pipeIdx > 0 ? inner.substring(0, pipeIdx).trim() : inner.trim();
    const display = pipeIdx > 0 ? inner.substring(pipeIdx + 1).trim() : slugDisplayName(slug);
    return `<a href="#" class="wiki-content-link" data-slug="${slug}">${display}</a>`;
  });

  const html = marked.parse(preprocessed, { breaks: true, async: false }) as string;
  return sanitizeMarkdownHTML(html);
}

async function openGraphDrawer(slug: string) {
  try {
    const res = await getWikiPage(props.knowledgeBaseId, slug);
    graphDrawerPage.value = res;
    graphDrawerVisible.value = true;
  } catch (e) {
    console.error(`Failed to load page ${slug}:`, e);
  }
}

function handleGraphDrawerClick(e: MouseEvent) {
  const target = e.target as HTMLElement;
  if (target.classList.contains("wiki-content-link")) {
    e.preventDefault();
    const slug = target.getAttribute("data-slug");
    if (slug) handleGraphSearchSelect(slug);
  } else if (target.tagName.toLowerCase() === "img") {
    e.preventDefault();
    imagePreviewUrl.value = target.getAttribute("src") || "";
    if (imagePreviewUrl.value) {
      imagePreviewVisible.value = true;
    }
  }
}

// activeTab drives which page_type's list is visible in the sidebar.
// Pre-tabbed UX stacked collapsible groups, but on a 40k-page KB the
// expanded groups left multiple virtualized viewports and scroll events got
// ambiguous — "which list am I scrolling?" The tabbed version removes
// that ambiguity by mounting exactly one scroller at a time.
const activeTab = ref<string>("");
type SidebarViewMode = "tree" | "list";
const SIDEBAR_VIEW_MODE_KEY = "yuheng.wiki.sidebar.viewMode";
function initialSidebarViewMode(): SidebarViewMode {
  try {
    return localStorage.getItem(SIDEBAR_VIEW_MODE_KEY) === "list" ? "list" : "tree";
  } catch {
    return "tree";
  }
}
const sidebarViewMode = ref<SidebarViewMode>(initialSidebarViewMode());
// Suppress visibleTabs watcher during the first sidebar load. Knowledge and
// summary buckets load in parallel; whichever API returns first used to win
// the race and stick on summary even though knowledge is the intended default.
let initialSidebarLoad = true;
// The outer scroll container. We reset it on tab switches so the retained
// scroll position from the old tab doesn't immediately expose the new
// tab's sentinel and cascade load-more calls.
const pageListRef = ref<HTMLElement | null>(null);
const sidebarViewSwitching = ref(false);
const sidebarTabSwitching = ref(false);

watch(sidebarViewMode, (mode) => {
  try {
    localStorage.setItem(SIDEBAR_VIEW_MODE_KEY, mode);
  } catch {
    /* ignore */
  }
  cancelCreateRootFolder();
  if (pageListRef.value) pageListRef.value.scrollTop = 0;
});

async function switchSidebarViewMode(mode: SidebarViewMode) {
  if (mode === sidebarViewMode.value || sidebarViewSwitching.value) return;
  sidebarViewSwitching.value = true;
  try {
    // The flat list has its own unscoped pagination stream. Fetch its first
    // page before swapping containers so the user never lands on an empty
    // RecycleScroller while the request is still in flight.
    if (mode === "list" && activeTab.value) {
      const ready = await loadFlatPagesForType(activeTab.value);
      if (!ready && activeFlatPages.value.length === 0) return;
    }
    sidebarViewMode.value = mode;
  } finally {
    sidebarViewSwitching.value = false;
  }
}

function waitForTabData(type: string, mode: SidebarViewMode): Promise<void> {
  const isBusy = () => {
    const bucket = pagesByType.value[type];
    if (!bucket) return false;
    return mode === "list" ? bucket.flatLoading : bucket.loading || bucket.categoriesLoading;
  };
  if (!isBusy()) return Promise.resolve();

  return new Promise((resolve) => {
    const stop = watch(
      isBusy,
      (busy) => {
        if (!busy) {
          stop();
          resolve();
        }
      },
      { flush: "post" },
    );
    // The request may finish between the initial check and watcher setup.
    if (!isBusy()) {
      stop();
      resolve();
    }
  });
}

async function setActiveTab(type: string) {
  if (activeTab.value === type || sidebarTabSwitching.value) return;
  sidebarTabSwitching.value = true;
  ensureBucket(type);
  try {
    // Initial sidebar requests run in parallel. A tab can become visible from
    // stats before its page/folder requests finish, so prepare the target
    // bucket before replacing the current content instead of briefly mounting
    // an empty tree/list on the first click after refresh.
    if (sidebarViewMode.value === "list") {
      await loadFlatPagesForType(type);
    } else {
      await Promise.all([loadPagesForType(type), loadCategoriesForType(type)]);
    }
    await waitForTabData(type, sidebarViewMode.value);

    cancelCreateRootFolder();
    activeTab.value = type;
  } finally {
    sidebarTabSwitching.value = false;
  }
  // Snap back to the top before the new list renders so the sentinel
  // has to be scrolled to, not simply appear at a retained scroll depth.
  if (pageListRef.value) pageListRef.value.scrollTop = 0;
}

// visibleTabs mirrors groupedPages but is meant for rendering the
// horizontal tab bar: only non-empty types survive, in typeOrder with
// any unknown types appended after.
const visibleTabs = computed(() => groupedPages.value.map((g) => ({ type: g.type, label: g.label, total: g.total })));

// activeGroup resolves activeTab into the current group descriptor,
// or null when the active type has been deselected (e.g. after a
// filter toggle zeroed out every bucket).
const activeGroup = computed(() => {
  if (!activeTab.value) return null;
  return groupedPages.value.find((g) => g.type === activeTab.value) || null;
});

const activeFlatPages = computed(() => {
  if (!activeTab.value) return [];
  return pagesByType.value[activeTab.value]?.flatItems || [];
});

const activeFlatState = computed(() => {
  if (!activeTab.value) return null;
  const bucket = pagesByType.value[activeTab.value];
  if (!bucket) return null;
  return {
    loading: bucket.flatLoading,
    hasMore: !bucket.flatInitialized || bucket.flatItems.length < bucket.flatTotal,
  };
});

function pageCategoryPath(page: WikiPage): string[] {
  const raw = Array.isArray(page.category_path) ? page.category_path : [];
  return raw.map((part) => String(part || "").trim()).filter(Boolean);
}

function directoryPathKey(type: string, parts: string[]): string {
  return `${type}:${parts.join("/")}`;
}

function toggleDirectory(pathKey: string) {
  const next = new Set(collapsedDirectories.value);
  if (next.has(pathKey)) {
    next.delete(pathKey);
    const parsed = parseDirectoryPathKey(pathKey);
    if (parsed) {
      loadCategoriesForType(parsed.type, { parentPath: parsed.path });
      loadPagesForType(parsed.type, { categoryPath: parsed.path });
    }
  } else {
    next.add(pathKey);
  }
  collapsedDirectories.value = next;

  const touched = new Set(touchedDirectories.value);
  touched.add(pathKey);
  touchedDirectories.value = touched;
}

function parseDirectoryPathKey(pathKey: string): { type: string; path: string[] } | null {
  const idx = pathKey.indexOf(":");
  if (idx < 0) return null;
  const type = pathKey.slice(0, idx);
  const path = pathKey
    .slice(idx + 1)
    .split("/")
    .map((part) => part.trim())
    .filter(Boolean);
  return { type, path };
}

function clearDirectoryStateForType(type: string) {
  const prefix = `${type}:`;
  collapsedDirectories.value = new Set([...collapsedDirectories.value].filter((key) => !key.startsWith(prefix)));
  touchedDirectories.value = new Set([...touchedDirectories.value].filter((key) => !key.startsWith(prefix)));
}

function initializeDefaultCollapsedDirectories(type: string, pagesToInspect: WikiPage[]) {
  if (pagesToInspect.length === 0) return;

  const next = new Set(collapsedDirectories.value);
  const touched = touchedDirectories.value;
  let changed = false;

  for (const page of pagesToInspect) {
    const path = pageCategoryPath(page);
    for (let i = 0; i < path.length; i++) {
      const key = directoryPathKey(type, path.slice(0, i + 1));
      if (touched.has(key) || next.has(key)) continue;
      next.add(key);
      changed = true;
    }
  }

  if (changed) {
    collapsedDirectories.value = next;
  }
}

const activeTreeRows = computed<WikiTreeRow[]>(() => {
  const group = activeGroup.value;
  if (!group) return [];
  const groupType = group.type;

  const rows: WikiTreeRow[] = [];
  const root: WikiTreeDirectory = {
    label: "",
    path: [],
    dirs: new Map(),
    pages: [],
    count: 0,
  };

  function ensureDirectory(parent: WikiTreeDirectory, label: string): WikiTreeDirectory {
    const existing = parent.dirs.get(label);
    if (existing) return existing;

    const dir: WikiTreeDirectory = {
      label,
      path: [...parent.path, label],
      dirs: new Map(),
      pages: [],
      count: 0,
    };
    parent.dirs.set(label, dir);
    return dir;
  }

  // Each tab maps to a single bucket; the knowledge tab's bucket already holds
  // the server-merged pages and directory skeleton (page_type=entity,concept,…).
  const bucket = pagesByType.value[groupType];
  const hasCategorySkeleton = (bucket?.categoryPaths.length || 0) > 0;
  for (const category of bucket?.categoryPaths || []) {
    const categoryPath = category.path;
    let cursor = root;
    for (let i = 0; i < categoryPath.length; i++) {
      const part = categoryPath[i];
      cursor = ensureDirectory(cursor, part);
      if (i === categoryPath.length - 1) {
        cursor.count = category.count;
      }
    }
  }

  for (const page of group.pages) {
    const path = pageCategoryPath(page);
    let cursor = root;
    if (!hasCategorySkeleton) cursor.count += 1;

    for (const part of path) {
      cursor = ensureDirectory(cursor, part);
      if (!hasCategorySkeleton) cursor.count += 1;
    }

    cursor.pages.push(page);
  }

  function appendDirectory(dir: WikiTreeDirectory, depth: number) {
    const key = directoryPathKey(groupType, dir.path);
    const collapsed = collapsedDirectories.value.has(key);
    rows.push({
      kind: "directory",
      rowKey: `dir:${key}`,
      pathKey: key,
      folderId: bucket?.folderIdByPath[dir.path.join("/")] || "",
      path: dir.path,
      label: dir.label,
      depth,
      count: dir.count,
      hasChildren: dir.dirs.size > 0,
      collapsed,
    });
    if (collapsed) return;

    for (const child of dir.dirs.values()) {
      appendDirectory(child, depth + 1);
    }
    for (const page of dir.pages) {
      rows.push({
        kind: "page",
        rowKey: `page:${page.id}`,
        page,
        depth: depth + 1,
      });
    }
    const state = bucket?.directoryPages[key];
    const hasMore = state && state.total > 0 && (state.nextPage - 1) * WIKI_SIDEBAR_PAGE_SIZE < state.total;
    if (state?.loading || hasMore) {
      rows.push({
        kind: "load-more",
        rowKey: `load-more:${key}`,
        type: groupType,
        path: dir.path,
        depth: depth + 1,
        loading: !!state?.loading,
      });
    }
  }

  for (const dir of root.dirs.values()) {
    appendDirectory(dir, 0);
  }
  for (const page of root.pages) {
    rows.push({
      kind: "page",
      rowKey: `page:${page.id}`,
      page,
      depth: 0,
    });
  }
  return rows;
});

// Keep activeTab in sync with what's available. When loadPages first
// populates buckets, pick the first non-empty tab. When a user deletes
// the last page of the active type we transparently switch to the next
// available one so the sidebar never shows "tab selected with no list".
watch(visibleTabs, (tabs) => {
  if (initialSidebarLoad) return;
  if (tabs.length === 0) {
    activeTab.value = "";
    return;
  }
  if (!tabs.some((t) => t.type === activeTab.value)) {
    activeTab.value = preferredDefaultTab(tabs);
  }
});

// IntersectionObserver-driven infinite scroll for the active tab. We
// observe a 1px sentinel placed after the list; when it enters the
// viewport we pull the next page for the active bucket. Guards in
// loadPagesForType prevent double-fetching.
const groupSentinelRef = ref<HTMLElement | null>(null);
let groupSentinelObserver: IntersectionObserver | null = null;
watch(
  [groupSentinelRef, sidebarViewMode],
  ([el]) => {
    if (groupSentinelObserver) {
      groupSentinelObserver.disconnect();
      groupSentinelObserver = null;
    }
    if (!el) return;
    groupSentinelObserver = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          const type = (entry.target as HTMLElement).dataset.type;
          if (!type) continue;
          if (sidebarViewMode.value === "list") loadFlatPagesForType(type);
          else loadPagesForType(type);
        }
      },
      { rootMargin: "200px" },
    );
    groupSentinelObserver.observe(el);
  },
  { flush: "post" },
);

const WIKI_PAGE_ITEM_HEIGHT = 100;

// Auto-load for the "load more" rows that live INSIDE the tree (more
// pages within an expanded directory, or more sub-folders at a level).
// The bottom group sentinel only covers the root page list; these rows
// can sit in the MIDDLE of the list, so we observe each of them. When a
// row scrolls into view it fires its loader instead of waiting for a
// manual click. Re-attaching on every activeTreeRows change also drains
// the case the user asked about — "current page loaded but more remain,
// and the row never reached the bottom": observe() re-fires for any row
// still intersecting after the previous batch rendered, and the loader
// guards stop the cascade once the directory is exhausted or the row is
// pushed out of view.
const treeListRef = ref<HTMLElement | null>(null);
let loadMoreObserver: IntersectionObserver | null = null;

function dispatchLoadMore(rowKey: string) {
  const row = activeTreeRows.value.find((r) => r.rowKey === rowKey);
  if (!row) return;
  if (row.kind === "load-more") {
    loadPagesForType(row.type, { categoryPath: row.path });
  }
}

watch(
  [activeTreeRows, treeListRef],
  () => {
    if (loadMoreObserver) {
      loadMoreObserver.disconnect();
      loadMoreObserver = null;
    }
    const container = treeListRef.value;
    if (!container) return;
    loadMoreObserver = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          const key = (entry.target as HTMLElement).dataset.loadmoreKey;
          if (key) dispatchLoadMore(key);
        }
      },
      { rootMargin: "200px" },
    );
    container.querySelectorAll<HTMLElement>("[data-loadmore-key]").forEach((el) => loadMoreObserver!.observe(el));
  },
  { flush: "post" },
);

// getTypeTagClass colours the small outlined page-type tag. It keeps the
// palette of the TDesign tag themes it replaces (summary and synthesis in
// the brand colour, entity green, concept orange, comparison red), each as
// a tinted fill with a border and text of the same hue.
function getTypeTagClass(type: string): string {
  const map: Record<string, string> = {
    summary: "border-primary/40 bg-primary/10 text-primary",
    entity: "border-success/40 bg-success/10 text-success",
    concept: "border-warning/40 bg-warning/10 text-warning",
    synthesis: "border-primary/40 bg-primary/10 text-primary",
    comparison: "border-destructive/40 bg-destructive/10 text-destructive",
  };
  return map[type] || "border-border bg-muted text-foreground";
}

// Lint tags are the filled-light variant: a tint, no border, coloured by
// severity and labelled by kind.
const LINT_TAG_BASE = "inline-flex h-[22px] items-center rounded-sm px-2 text-xs";

function lintSeverityClass(severity: WikiLintIssue["severity"]): string {
  switch (severity) {
    case "error":
      return `${LINT_TAG_BASE} bg-[var(--td-error-color-light)] text-destructive`;
    case "warning":
      return `${LINT_TAG_BASE} bg-[var(--td-warning-color-light)] text-warning`;
    default:
      return `${LINT_TAG_BASE} bg-muted text-foreground`;
  }
}

function lintTypeLabel(type: WikiLintIssue["type"]): string {
  switch (type) {
    case "orphan_page":
      return t("knowledgeEditor.wikiBrowser.lintOrphan");
    case "broken_link":
      return t("knowledgeEditor.wikiBrowser.lintBrokenLink");
    case "stale_ref":
      return t("knowledgeEditor.wikiBrowser.lintStaleRef");
    case "missing_cross_ref":
      return t("knowledgeEditor.wikiBrowser.lintMissingCrossRef");
    case "empty_content":
      return t("knowledgeEditor.wikiBrowser.lintEmpty");
    case "duplicate_slug":
      return t("knowledgeEditor.wikiBrowser.lintDuplicateSlug");
    default:
      return type;
  }
}

// Class strings shared by several repeated elements of the template. They
// live here so each variant is written once; Tailwind scans this file, so
// the utilities are generated all the same.
const lintItemClass =
  "border-border bg-card flex flex-col gap-2 rounded-md border p-4 transition-[box-shadow,border-color] duration-200 hover:border-[var(--td-brand-color-light)]";
const lintDescClass =
  "text-foreground max-h-[150px] overflow-y-auto pr-1 text-[13px] leading-[1.6] break-words whitespace-pre-wrap [&::-webkit-scrollbar]:w-1 [&::-webkit-scrollbar-thumb]:rounded [&::-webkit-scrollbar-thumb]:bg-[var(--td-scrollbar-color)] [&::-webkit-scrollbar-track]:bg-transparent";
const legendActionClass =
  "group/legend-action text-muted-foreground hover:text-primary flex cursor-pointer items-center gap-1.5 text-[11px] leading-[14px] transition-all select-none";
const legendActionIconClass =
  "text-placeholder group-hover/legend-action:text-primary inline-flex size-3.5 shrink-0 items-center justify-center leading-none transition-colors";
const tabBarActionClass =
  "text-muted-foreground hover:bg-accent hover:text-primary disabled:text-placeholder inline-flex size-[26px] shrink-0 items-center justify-center rounded-md transition-colors disabled:cursor-not-allowed disabled:bg-transparent disabled:opacity-45";
const folderNameInputClass =
  "wiki-directory-rename-input bg-card text-foreground border-primary h-6 min-w-0 flex-1 rounded-[4px] border px-1.5 text-[13px] outline-none";
const badgeClass = "inline-flex items-center gap-1 rounded-[4px] px-2 py-0.5 text-xs leading-[1.4]";
const readerActionClass =
  "text-placeholder inline-flex size-8 shrink-0 items-center justify-center rounded-md transition-colors";
const footerLinkClass =
  "wiki-content-link text-primary border-primary cursor-pointer border-b font-medium no-underline [--tw-border-style:dashed] hover:[--tw-border-style:solid]";

function getTypeLabel(type: string): string {
  const map: Record<string, string> = {
    knowledge: t("knowledgeEditor.wikiBrowser.filterKnowledge"),
    summary: t("knowledgeEditor.wikiBrowser.filterSummary"),
    entity: t("knowledgeEditor.wikiBrowser.filterEntity"),
    concept: t("knowledgeEditor.wikiBrowser.filterConcept"),
    synthesis: t("knowledgeEditor.wikiBrowser.filterSynthesis"),
    comparison: t("knowledgeEditor.wikiBrowser.filterComparison"),
    index: "Index",
  };
  return map[type] || type;
}

// getPageIcon picks a distinct icon per page_type so the merged knowledge
// tab can still tell entities, concepts, etc. apart at a glance.
function getPageIcon(page: WikiPage): Component {
  const map: Record<string, Component> = {
    entity: TagIcon,
    concept: LightbulbIcon,
    synthesis: BlendIcon,
    comparison: LayoutGridIcon,
    summary: FileIcon,
  };
  return map[page.page_type] || FileIcon;
}

const renderedContent = computed(() => {
  if (!selectedPage.value) return "";
  const body = stripDuplicateLeadingTitle(selectedPage.value.content || "", selectedPage.value.title);
  return renderMarkdown(body);
});

function stripDuplicateLeadingTitle(content: string, title: string): string {
  if (!content || !title) return content;
  const lines = content.split("\n");
  let i = 0;
  while (i < lines.length && lines[i].trim() === "") i++;
  if (i >= lines.length) return content;
  const headingMatch = lines[i].match(/^#\s+(.+?)\s*$/);
  if (!headingMatch) return content;
  const heading = headingMatch[1].trim();
  const pageTitle = title.trim();
  if (heading !== pageTitle && heading.toLowerCase() !== pageTitle.toLowerCase()) return content;
  i++;
  while (i < lines.length && lines[i].trim() === "") i++;
  return lines.slice(i).join("\n");
}

// Label shown next to the back arrow on page headers. Prefers the
// nearest page-history entry when available so the user sees where
// they'll land; falls back to the Index label when the current
// page was opened directly from a system view.
const backLabel = computed(() => {
  if (navHistory.value.length > 0) {
    return navHistory.value[navHistory.value.length - 1].title;
  }
  if (navFromSystemView.value === "index") {
    return t("knowledgeEditor.wikiBrowser.indexTitle");
  }
  return "";
});

// Rendered markdown for the incremental index view. Re-runs every time
// indexMarkdown grows (initial intro load or a loadMore section append).
const renderedIndexMarkdown = computed(() => {
  if (!indexMarkdown.value) return "";
  return renderMarkdown(indexMarkdown.value);
});

// True while another section or page is available to load. Starts true
// after the first fetch (intro only loaded, sections untouched), becomes
// false once the last section in INDEX_SECTION_ORDER is exhausted.
const indexHasMore = computed(() => {
  if (!indexAvailable.value) return false;
  if (indexSectionIdx.value >= INDEX_SECTION_ORDER.length) return false;
  return true;
});

watch(renderedContent, async () => {
  await nextTick();
  if (readerBodyRef.value) {
    await hydrateProtectedFileImages(readerBodyRef.value, kbFileAccess.value);
  }
});

// Index body may contain image markdown from an LLM-generated intro;
// hydrate the same way as regular page content so protected URLs
// resolve. Also re-applied after every loadMore append.
watch(renderedIndexMarkdown, async () => {
  await nextTick();
  if (indexBodyRef.value) {
    await hydrateProtectedFileImages(indexBodyRef.value, kbFileAccess.value);
  }
});

function handleContentClick(e: MouseEvent) {
  const target = e.target as HTMLElement;
  if (target.classList.contains("wiki-content-link")) {
    e.preventDefault();
    const slug = target.getAttribute("data-slug");
    if (slug) navigateToSlug(slug);
  } else if (target.tagName.toLowerCase() === "img") {
    e.preventDefault();
    imagePreviewUrl.value = target.getAttribute("src") || "";
    if (imagePreviewUrl.value) {
      imagePreviewVisible.value = true;
    }
  }
}

// WIKI_SIDEBAR_PAGE_SIZE is the per-type fetch batch. Small enough that
// the initial paint is snappy even on a big KB, large enough that the
// virtualized scroller normally gets everything it needs in one request
// for common wikis. Later pages are pulled on scroll.
const WIKI_SIDEBAR_PAGE_SIZE = 100;

function emptyBucket(): PageTypeBucket {
  return {
    items: [],
    nextPage: 1,
    total: 0,
    loading: false,
    initialized: false,
    categoryPaths: [],
    folderIdByPath: {},
    categoriesLoading: false,
    categoriesInitialized: false,
    categoryPages: {},
    directoryPages: {},
    flatItems: [],
    flatNextPage: 1,
    flatTotal: 0,
    flatLoading: false,
    flatInitialized: false,
  };
}

async function loadFlatPagesForType(type: string, reset = false): Promise<boolean> {
  const bucket = ensureBucket(type);
  if (bucket.flatLoading) return bucket.flatItems.length > 0;
  if (!reset && bucket.flatInitialized && bucket.flatItems.length >= bucket.flatTotal) return true;

  bucket.flatLoading = true;
  try {
    const requestPage = reset ? 1 : bucket.flatNextPage;
    const res = await listWikiPages(props.knowledgeBaseId, {
      page_type: tabPageTypes(type),
      page: requestPage,
      page_size: WIKI_SIDEBAR_PAGE_SIZE,
      sort_by: "wiki_path",
      sort_order: "asc",
    });
    const batch: WikiPage[] = res.pages ?? [];
    if (reset) {
      bucket.flatItems = batch;
      bucket.flatNextPage = 2;
    } else {
      const seen = new Set(bucket.flatItems.map((page) => page.id));
      for (const page of batch) {
        if (!seen.has(page.id)) bucket.flatItems.push(page);
      }
      bucket.flatNextPage += 1;
    }
    bucket.flatTotal = res.total || 0;
    bucket.flatInitialized = true;

    const seenPages = new Set(pages.value.map((page) => page.id));
    for (const page of batch) {
      if (!seenPages.has(page.id)) pages.value.push(page);
    }
    return true;
  } catch (e) {
    console.error(`Failed to load flat wiki pages of type ${type}:`, e);
    return false;
  } finally {
    bucket.flatLoading = false;
  }
}

function ensureBucket(type: string): PageTypeBucket {
  if (!pagesByType.value[type]) {
    pagesByType.value[type] = emptyBucket();
  }
  return pagesByType.value[type];
}

// loadCategoriesForType pulls the child folders of one directory level from the
// authoritative wiki_folders tree. Empty folders are returned only for the
// merged knowledge tab (multi page_type); the summary tab omits them.
// bucket.folderIdByPath; the root level uses id "". Each level's children are
// returned in one shot (the tree is navigation-sized), so there is no
// per-level "load more folders" pagination anymore.
async function loadCategoriesForType(type: string, opts: { reset?: boolean; parentPath?: string[] } = {}) {
  const bucket = ensureBucket(type);
  const parentPath = opts.parentPath || [];
  const parentKey = directoryPathKey(type, parentPath);
  const isRoot = parentPath.length === 0;

  const parentId = isRoot ? "" : bucket.folderIdByPath[parentPath.join("/")];
  // A deeper level whose parent folder id we have not yet recorded cannot be
  // resolved against the (id-based) folders endpoint — skip until it loads.
  if (!isRoot && parentId === undefined) return;

  let state = bucket.categoryPages[parentKey];
  if (state?.loading) return;
  if (!opts.reset && state && state.initialized) return; // a level is loaded in a single request
  if (!state) {
    state = { nextPage: 1, totalPages: 1, loading: false, initialized: false };
  }

  const setState = (next: DirectoryCategoryState) => {
    bucket.categoryPages = { ...bucket.categoryPages, [parentKey]: next };
  };
  setState({ ...state, loading: true });
  if (isRoot) bucket.categoriesLoading = true;
  try {
    const res = await listWikiFolders(props.knowledgeBaseId, parentId || "", tabPageTypes(type));
    const folders: WikiFolderNode[] = res.folders ?? [];
    const incoming = folders
      .map((folder) => ({
        path: String(folder.path || "")
          .split("/")
          .map((part) => part.trim())
          .filter(Boolean),
        count: Number(folder.page_count) || 0,
        id: String(folder.id || ""),
      }))
      .filter((entry) => entry.path.length > 0)
      // Summary is a single page_type; empty folders belong in the merged
      // knowledge view only (backend filters too — belt-and-suspenders).
      .filter((entry) => type !== "summary" || entry.count > 0);

    const existing = new Map(
      (opts.reset ? [] : bucket.categoryPaths).map((entry) => [directoryPathKey(type, entry.path), entry]),
    );
    const folderIds = opts.reset ? {} : { ...bucket.folderIdByPath };
    for (const entry of incoming) {
      existing.set(directoryPathKey(type, entry.path), { path: entry.path, count: entry.count });
      folderIds[entry.path.join("/")] = entry.id;
    }
    bucket.categoryPaths = Array.from(existing.values());
    bucket.folderIdByPath = folderIds;
    if (opts.reset) bucket.categoryPages = {};
    setState({ nextPage: 2, totalPages: 1, loading: false, initialized: true });
    if (isRoot) bucket.categoriesInitialized = true;
    initializeDefaultCollapsedDirectories(
      type,
      incoming.map(
        (entry) =>
          ({
            category_path: entry.path,
          }) as WikiPage,
      ),
    );
  } catch (e) {
    console.error(`Failed to load wiki folders of type ${type}:`, e);
    setState({ ...state, loading: false });
  } finally {
    if (isRoot) bucket.categoriesLoading = false;
  }
}

// reloadDirectoryForType resets and reloads both the folder skeleton and the
// pages for one tab. Used after a structural mutation (move page, create /
// rename / delete folder) so the tree reflects the new layout authoritatively
// instead of guessing at the optimistic delta.
async function reloadDirectoryForType(type: string, opts: { preserveDirectoryState?: boolean } = {}) {
  const refreshFlatList = sidebarViewMode.value === "list" || ensureBucket(type).flatInitialized;
  const expandedPaths = opts.preserveDirectoryState
    ? expandedWikiDirectoryPaths(type, collapsedDirectories.value, touchedDirectories.value)
    : [];
  if (!opts.preserveDirectoryState) clearDirectoryStateForType(type);
  await loadPagesForType(type, {
    reset: true,
    preserveDirectoryState: opts.preserveDirectoryState,
  });
  await loadCategoriesForType(type, { reset: true });
  // Folder data is fetched one level at a time. Reload preserved paths in
  // parent-first order so each child request can resolve its parent folder id.
  for (const path of expandedPaths) {
    await Promise.all([
      loadPagesForType(type, { categoryPath: path }),
      loadCategoriesForType(type, { parentPath: path }),
    ]);
  }
  if (refreshFlatList) await loadFlatPagesForType(type, true);
}

// --- Drag-and-drop: move a page or folder into a folder ------------------
// draggedItem holds whatever is being dragged — a page (move into folder) or a
// folder (reparent). dropTargetKey is the row highlighted as the hover target
// ("__root__" for the toolbar drop zone). Both clear on dragend / drop so a
// cancelled drag leaves no sticky highlight.
type DraggedItem = { kind: "page"; page: WikiPage } | { kind: "folder"; folderId: string; path: string[] };
const draggedItem = ref<DraggedItem | null>(null);
const dropTargetKey = ref<string>("");

// primeDragData sets dataTransfer so the native HTML5 drag actually starts.
// Firefox refuses to initiate a drag unless some data is set, and without an
// explicit effectAllowed/dropEffect the cursor shows "no-drop" and the drop
// event never fires — which is why the move felt impossible to trigger.
function primeDragData(e: DragEvent) {
  if (!e.dataTransfer) return;
  e.dataTransfer.effectAllowed = "move";
  try {
    e.dataTransfer.setData("text/plain", "");
  } catch {
    // Some browsers throw if setData is called outside a real dragstart; ignore.
  }
}

function onPageDragStart(e: DragEvent, page: WikiPage) {
  draggedItem.value = { kind: "page", page };
  primeDragData(e);
}

function onFolderDragStart(e: DragEvent, folderId: string, path: string[]) {
  if (!folderId) return;
  draggedItem.value = { kind: "folder", folderId, path };
  primeDragData(e);
}

function onPageDragEnd() {
  draggedItem.value = null;
  dropTargetKey.value = "";
}

function onDirectoryDragOver(e: DragEvent, pathKey: string) {
  if (!draggedItem.value) return;
  if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
  dropTargetKey.value = pathKey;
}

// onRootDragOver fires on the list container itself. Folder rows stop their
// dragover from bubbling here, so reaching this handler means the cursor is
// over empty list space (or a page row) — i.e. a "move to root" target.
function onRootDragOver(e: DragEvent) {
  if (!draggedItem.value) return;
  if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
  dropTargetKey.value = "__root__";
}

function onDirectoryDragLeave(pathKey: string) {
  if (dropTargetKey.value === pathKey) dropTargetKey.value = "";
}

// A move is staged here on drop and only executed once the user confirms via
// the in-place confirmation popup (anchored at the drop point). This avoids
// silently mutating the tree on an accidental drag.
const pendingMove = ref<{ item: DraggedItem; folderId: string; targetLabel: string; x: number; y: number } | null>(
  null,
);

function onDropOnDirectory(e: DragEvent, folderId: string, path: string[]) {
  const item = draggedItem.value;
  draggedItem.value = null;
  dropTargetKey.value = "";
  if (!item) return;
  const targetPath = path.join("/");

  if (item.kind === "page") {
    // No-op when the page already lives in this folder.
    if ((pageCategoryPath(item.page) || []).join("/") === targetPath) return;
  } else {
    // Folder reparent. Reject no-op (same parent) and the illegal move of a
    // folder into itself or one of its descendants.
    const sourcePath = item.path.join("/");
    if (sourcePath === targetPath) return;
    if (folderId === item.folderId) return;
    if (targetPath === sourcePath || targetPath.startsWith(`${sourcePath}/`)) {
      MessagePlugin.warning(t("knowledgeEditor.wikiBrowser.moveFolderIntoSelf"));
      return;
    }
  }

  const targetLabel = path.length > 0 ? path[path.length - 1] : t("knowledgeEditor.wikiBrowser.rootFolderLabel");
  pendingMove.value = { item, folderId, targetLabel, x: e.clientX, y: e.clientY };
}

function cancelPendingMove() {
  pendingMove.value = null;
}

async function confirmPendingMove() {
  const move = pendingMove.value;
  pendingMove.value = null;
  if (!move) return;
  const { item, folderId } = move;

  if (item.kind === "page") {
    try {
      await moveWikiPage(props.knowledgeBaseId, item.page.slug, folderId);
      MessagePlugin.success(t("knowledgeEditor.wikiBrowser.movePageSuccess"));
      await reloadDirectoryForType(activeTab.value);
    } catch (e) {
      console.error("Failed to move wiki page:", e);
      MessagePlugin.error(t("knowledgeEditor.wikiBrowser.movePageFailed"));
    }
    return;
  }

  try {
    await updateWikiFolder(props.knowledgeBaseId, item.folderId, { parent_id: folderId, move_parent: true });
    MessagePlugin.success(t("knowledgeEditor.wikiBrowser.moveFolderSuccess"));
    await reloadDirectoryForType(activeTab.value);
  } catch (e: any) {
    console.error("Failed to move wiki folder:", e);
    MessagePlugin.error(e?.message || t("knowledgeEditor.wikiBrowser.moveFolderFailed"));
  }
}

// --- Folder create / rename / delete -------------------------------------
// These are invoked by the in-place WikiFolderActions popup (no full-page
// dialog). The popup owns the input / confirm surface; these handlers only do
// the API call, surface a toast, and reload the affected tab so the tree
// reflects the new layout authoritatively.
async function createFolder(parentId: string, parentPath: string[], name: string) {
  const type = activeTab.value;
  const scrollTop = pageListRef.value?.scrollTop;
  try {
    await createWikiFolder(props.knowledgeBaseId, parentId, name);
    MessagePlugin.success(t("knowledgeEditor.wikiBrowser.createFolderSuccess"));
    // Keep the current path expanded so refreshing the authoritative tree does
    // not collapse it and clamp the sidebar scroll position back to the root.
    if (parentPath.length > 0) {
      const state = expandWikiDirectoryPath(type, parentPath, collapsedDirectories.value, touchedDirectories.value);
      collapsedDirectories.value = state.collapsed;
      touchedDirectories.value = state.touched;
    }
    await reloadDirectoryForType(type, { preserveDirectoryState: true });
    await nextTick();
    if (activeTab.value === type && scrollTop !== undefined && pageListRef.value) {
      pageListRef.value.scrollTop = scrollTop;
    }
  } catch (e: any) {
    console.error("Failed to create wiki folder:", e);
    MessagePlugin.error(e?.message || t("knowledgeEditor.wikiBrowser.createFolderFailed"));
  }
}

// Inline rename: the directory row swaps its label for a text input instead
// of opening a popup. editingFolderId marks the active row; editingName backs
// the input. Committed on Enter / blur, abandoned on Escape.
const editingFolderId = ref("");
const editingName = ref("");

const creatingRootFolder = ref(false);
const creatingRootFolderName = ref("");
const creatingRootFolderInputRef = ref<HTMLInputElement | null>(null);

function startCreateRootFolder() {
  if (creatingRootFolder.value) return;
  creatingRootFolder.value = true;
  creatingRootFolderName.value = "";
  nextTick(() => {
    creatingRootFolderInputRef.value?.focus();
  });
}

function cancelCreateRootFolder() {
  creatingRootFolder.value = false;
  creatingRootFolderName.value = "";
}

async function submitCreateRootFolder() {
  const name = creatingRootFolderName.value.trim();
  if (!name) return;
  cancelCreateRootFolder();
  await createFolder("", [], name);
}

function startRenameFolder(folderId: string, currentName: string) {
  if (!folderId) return;
  editingFolderId.value = folderId;
  editingName.value = currentName;
  // Only one rename input exists at a time; focus + select it once rendered.
  nextTick(() => {
    const el = document.querySelector(".wiki-directory-rename-input") as HTMLInputElement | null;
    el?.focus();
    el?.select();
  });
}

function cancelRenameFolder() {
  editingFolderId.value = "";
  editingName.value = "";
}

async function commitRenameFolder(folderId: string, originalName: string) {
  if (editingFolderId.value !== folderId) return;
  const name = editingName.value.trim();
  cancelRenameFolder();
  if (!name || name === originalName) return;
  try {
    await updateWikiFolder(props.knowledgeBaseId, folderId, { name });
    MessagePlugin.success(t("knowledgeEditor.wikiBrowser.renameFolderSuccess"));
    await reloadDirectoryForType(activeTab.value);
  } catch (e: any) {
    console.error("Failed to rename wiki folder:", e);
    MessagePlugin.error(e?.message || t("knowledgeEditor.wikiBrowser.renameFolderFailed"));
  }
}

async function deleteFolder(folderId: string) {
  if (!folderId) return;
  try {
    await deleteWikiFolder(props.knowledgeBaseId, folderId);
    MessagePlugin.success(t("knowledgeEditor.wikiBrowser.deleteFolderSuccess"));
    await reloadDirectoryForType(activeTab.value);
  } catch (e: any) {
    console.error("Failed to delete wiki folder:", e);
    MessagePlugin.error(e?.message || t("knowledgeEditor.wikiBrowser.deleteFolderFailed"));
  }
}

// loadPagesForType fetches the next page for a single type bucket. The
// first call seeds `total` from the backend so subsequent `hasMore`
// checks work without another round-trip. Guard against concurrent
// invocations for the same type (e.g. scroll event fires rapidly while
// a network request is still in flight).
async function loadPagesForType(
  type: string,
  opts: { reset?: boolean; categoryPath?: string[]; preserveDirectoryState?: boolean } = {},
) {
  const bucket = ensureBucket(type);
  const categoryPath = opts.categoryPath || [];
  const scopedToCategory = categoryPath.length > 0;
  const scopedPathKey = scopedToCategory ? directoryPathKey(type, categoryPath) : "";
  if (scopedToCategory && !bucket.directoryPages[scopedPathKey]) {
    bucket.directoryPages = {
      ...bucket.directoryPages,
      [scopedPathKey]: { nextPage: 1, total: 0, loading: false, initialized: false },
    };
  }
  const scopedState = scopedToCategory ? bucket.directoryPages[scopedPathKey] : null;
  if (scopedState?.loading || (!scopedToCategory && bucket.loading)) return;
  if (scopedToCategory) {
    const state = scopedState;
    if (!state) return;
    if (state.initialized && state.total > 0 && (state.nextPage - 1) * WIKI_SIDEBAR_PAGE_SIZE >= state.total) return;
  } else if (!opts.reset && bucket.initialized && bucket.items.length >= bucket.total) {
    return;
  }

  if (scopedState) {
    bucket.directoryPages = {
      ...bucket.directoryPages,
      [scopedPathKey]: { ...scopedState, loading: true },
    };
  } else bucket.loading = true;
  try {
    const currentScopedState = scopedToCategory ? bucket.directoryPages[scopedPathKey] : null;
    const requestPage = opts.reset ? 1 : currentScopedState ? currentScopedState.nextPage : bucket.nextPage;
    const res = await listWikiPages(props.knowledgeBaseId, {
      page_type: tabPageTypes(type),
      page: requestPage,
      page_size: WIKI_SIDEBAR_PAGE_SIZE,
      sort_by: "wiki_path",
      sort_order: "asc",
      category_path: categoryPath.join("/"),
      category_depth: categoryPath.length,
    });
    const batch: WikiPage[] = res.pages ?? [];
    const reportedTotal = res.total || 0;

    if (!scopedToCategory) {
      if (opts.reset) {
        bucket.items = batch;
        bucket.nextPage = 2;
        bucket.directoryPages = {};
        if (!opts.preserveDirectoryState) clearDirectoryStateForType(type);
      } else {
        const seenItems = new Set(bucket.items.map((p) => p.id));
        for (const p of batch) {
          if (!seenItems.has(p.id)) bucket.items.push(p);
        }
        bucket.nextPage += 1;
      }
      bucket.total = reportedTotal;
      bucket.initialized = true;
    } else if (currentScopedState) {
      const seenItems = new Set(bucket.items.map((p) => p.id));
      for (const p of batch) {
        if (!seenItems.has(p.id)) bucket.items.push(p);
      }
      bucket.directoryPages = {
        ...bucket.directoryPages,
        [scopedPathKey]: {
          ...currentScopedState,
          total: reportedTotal,
          nextPage: currentScopedState.nextPage + 1,
          initialized: true,
          loading: false,
        },
      };
    }
    initializeDefaultCollapsedDirectories(type, batch);

    // Mirror the newly arrived rows into the flat pages list so
    // slugDisplayName and friends keep working.
    if (batch.length > 0) {
      const seen = new Set(pages.value.map((p) => p.id));
      for (const p of batch) {
        if (!seen.has(p.id)) pages.value.push(p);
      }
    }
  } catch (e) {
    console.error(`Failed to load wiki pages of type ${type}:`, e);
  } finally {
    if (scopedState) {
      const latest = bucket.directoryPages[scopedPathKey];
      if (latest?.loading) {
        bucket.directoryPages = {
          ...bucket.directoryPages,
          [scopedPathKey]: { ...latest, loading: false, initialized: true },
        };
      }
    } else bucket.loading = false;
  }

  await nextTick();
}

// loadIndex probes the wiki index so the sidebar knows to show the pinned
// Index entry. We ask the backend for intro only (zero
// group types) — a bounded response regardless of KB size. Sections are
// fetched lazily after the user actually opens the Index view; see
// loadMoreIndexSection.
async function loadIndex() {
  try {
    // We only need intro on the initial probe — the directory groups
    // are fetched lazily once the user opens the Index view. Passing
    // an unknown type filter yields a cheap single count(*) + 0 rows
    // on the backend instead of scanning every directory group, and
    // the frontend discards the resulting empty group unconditionally.
    const idxRes = await getWikiIndex(props.knowledgeBaseId, { types: ["__intro_only__"], limit: 1 });
    const cleanIntro = (idxRes.intro || "").trim();
    indexMarkdown.value = cleanIntro ? cleanIntro + "\n" : "";
    indexAvailable.value = true;
    indexSections.value = {};
    indexSectionIdx.value = 0;
  } catch (e) {
    console.error("Failed to load wiki index:", e);
  }
}

// openIndexView switches the reader into the markdown-rendered index
// overview. Re-uses the intro already fetched during loadPages(); only
// re-fetches on first ever open or if a prior attempt failed.
async function openIndexView() {
  selectedPage.value = null;
  activeSystemView.value = "index";
  if (!indexMarkdown.value) {
    indexLoading.value = true;
    try {
      await loadIndex();
    } finally {
      indexLoading.value = false;
    }
  }
  // Observer is mounted/unmounted from a watch on activeSystemView
  // + indexSentinelRef below, so nothing else to do here — entering
  // the view is a render-time concern.
}

function appendIndexDirectoryLines(items: WikiIndexEntryDTO[]): string {
  let out = "";
  const emittedDirs = new Set<string>();
  for (const entry of items) {
    const path = (Array.isArray(entry.category_path) ? entry.category_path : [])
      .map((part) => String(part || "").trim())
      .filter(Boolean);
    for (let i = 0; i < path.length; i++) {
      const parts = path.slice(0, i + 1);
      const key = parts.join("/");
      if (emittedDirs.has(key)) continue;
      emittedDirs.add(key);
      out += `${"  ".repeat(i)}**${path[i]}**\n`;
    }
    const display = entry.title || entry.slug;
    const indent = "  ".repeat(path.length);
    if (entry.summary) {
      out += `${indent}[[${entry.slug}|${display}]] — ${entry.summary}\n`;
    } else {
      out += `${indent}[[${entry.slug}|${display}]]\n`;
    }
  }
  return out;
}

// loadMoreIndexSection advances the directory one step forward. The
// order is fixed (Summary → Entity → Concept → …); within a section we
// paginate with the backend's cursor, and only move to the next section
// when the current one is exhausted. Each call produces one network
// round trip that appends a markdown block to indexMarkdown.
//
// Rendering is append-only markdown rather than a structured list so
// the viewer feels like a regular wiki page — [[wiki-link]] clicks flow
// through handleContentClick just like every other page body. Entries
// are rendered as plain lines (not list items) so the reader doesn't
// carry list bullets next to every link.
async function loadMoreIndexSection() {
  if (indexLoading.value) return;
  if (indexSectionIdx.value >= INDEX_SECTION_ORDER.length) return;

  const type = INDEX_SECTION_ORDER[indexSectionIdx.value];
  const state = indexSections.value[type] || { loaded: false, cursor: "", total: 0 };
  const isFirstChunkOfSection = !state.loaded;

  indexLoading.value = true;
  try {
    const res = await getWikiIndex(props.knowledgeBaseId, {
      types: [type],
      limit: 50,
      cursor: isFirstChunkOfSection ? undefined : state.cursor || undefined,
    });
    const group = (res.groups ?? []).find((g: WikiIndexGroup) => g.type === type);

    const items: WikiIndexEntryDTO[] = group?.items || [];
    const total: number = group?.total || 0;
    const nextCursor: string = group?.next_cursor || "";

    // Only emit a section heading the first time we see entries for a
    // type. An empty section is skipped entirely so the reader doesn't
    // see "## Entity (0)" for a KB with no entities.
    let appended = "";
    if (isFirstChunkOfSection && items.length > 0) {
      const label = getTypeLabel(type);
      appended += `\n## ${label} (${total})\n\n`;
    }
    appended += appendIndexDirectoryLines(items);
    if (appended) {
      indexMarkdown.value = indexMarkdown.value + appended;
    }

    indexSections.value[type] = {
      loaded: true,
      cursor: nextCursor,
      total,
    };

    // Advance to the next section when this one has no more pages.
    // When the section is flat-out empty (total === 0), skip the
    // heading entirely and move on without emitting any markdown.
    if (!nextCursor) {
      indexSectionIdx.value += 1;
    }
  } catch (e) {
    console.error(`Failed to load more index entries for ${INDEX_SECTION_ORDER[indexSectionIdx.value]}:`, e);
  } finally {
    indexLoading.value = false;
  }

  // IntersectionObserver does NOT re-fire while the target stays
  // continuously intersecting. On small KBs (say a wiki with only
  // 3 summary pages and no entities / concepts) the sentinel sits
  // inside the viewport from the moment we finish the first section,
  // so without this nudge the remaining sections would never load.
  //
  // After every append we yield a tick (so the DOM reflows and the
  // sentinel's new rect is valid) then re-check: if it's still in
  // view and we have more to load, recurse. The recursion bottoms
  // out when either hasMore turns off or the sentinel is pushed
  // below the fold by the accumulated entries.
  await nextTick();
  if (indexHasMore.value && sentinelInView()) {
    loadMoreIndexSection();
  }
}

// sentinelInView reports whether the load sentinel's rect currently
// overlaps the viewport (± the same 200px cushion the observer uses),
// so after loading a section we know whether to drain another round
// even though the observer itself won't fire again while the target
// stays visible.
function sentinelInView(): boolean {
  const el = indexSentinelRef.value;
  if (!el) return false;
  const rect = el.getBoundingClientRect();
  const vh = window.innerHeight || document.documentElement.clientHeight;
  // 200px margin matches the observer's rootMargin so the drain
  // threshold and the scroll-triggered threshold stay consistent.
  return rect.top < vh + 200 && rect.bottom > -200;
}

// Mount/unmount the IntersectionObserver around the Index sentinel.
// We use rootMargin to pre-load when the user scrolls within ~200px
// of the sentinel, which hides the network round-trip behind the
// scroll motion.
watch([indexSentinelRef, () => activeSystemView.value], async ([el, view]) => {
  if (indexObserver) {
    indexObserver.disconnect();
    indexObserver = null;
  }
  if (view !== "index" || !el) return;
  indexObserver = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting && indexHasMore.value && !indexLoading.value) {
          loadMoreIndexSection();
        }
      }
    },
    { rootMargin: "200px" },
  );
  indexObserver.observe(el);
});

onUnmounted(() => {
  if (indexObserver) {
    indexObserver.disconnect();
    indexObserver = null;
  }
});

// loadPages is the sidebar's top-level initialization. It wires up the
// empty buckets (so groupedPages produces stable group slots even
// before any fetch completes), pulls the pinned system pages, and then
// kicks off the first page of each content type bucket in parallel —
// cheap because each bucket caps at WIKI_SIDEBAR_PAGE_SIZE rows.
//
// Historically this function looped listWikiPages({page:1..50}) and
// accumulated up to 25k rows in `pages.value`. On a 4万-page KB that
// was multiple seconds of network + serialization + O(n) group
// computation before the user saw anything.
async function loadPages() {
  loading.value = true;
  try {
    searchResults.value = null;
    for (const tab of CONTENT_TABS) ensureBucket(tab);
    await loadIndex();
    await Promise.all(
      CONTENT_TABS.map(async (tab) => {
        await loadPagesForType(tab, { reset: true });
        await loadCategoriesForType(tab, { reset: true });
      }),
    );

    // Default to the knowledge tab (first in CONTENT_TABS) once every
    // bucket has had a chance to load. On later reloads (e.g. after
    // indexing finishes) keep the user's current tab when still valid.
    if (initialSidebarLoad) {
      activeTab.value = preferredDefaultTab(visibleTabs.value);
      initialSidebarLoad = false;
    } else {
      const tabValid = activeTab.value && visibleTabs.value.some((tab) => tab.type === activeTab.value);
      if (!tabValid) {
        activeTab.value = preferredDefaultTab(visibleTabs.value);
      }
    }
    if (sidebarViewMode.value === "list" && activeTab.value) {
      await loadFlatPagesForType(activeTab.value, true);
    }

    // Auto-select based on query string or default to the index
    // overview. The index is the natural landing view — it shows
    // intro + a paginated directory of every page type.
    if (!selectedPage.value && activeSystemView.value === "") {
      if (route.query.slug && typeof route.query.slug === "string") {
        navigateToSlug(route.query.slug);
      } else if (indexAvailable.value) {
        openIndexView();
      }
    }
  } finally {
    loading.value = false;
  }
}

let statsTimer: ReturnType<typeof setInterval> | null = null;

async function loadStats() {
  try {
    const res = await getWikiStats(props.knowledgeBaseId);
    stats.value = res;

    // Notify parent so it can reflect wiki status (e.g. indexing badge in the breadcrumb)
    if (stats.value) {
      emit("status-change", {
        pendingTasks: stats.value.pending_tasks || 0,
        isActive: !!stats.value.is_active,
      });
    }

    // Poll if there are pending tasks or wiki ingest is active
    if (stats.value && (stats.value.pending_tasks > 0 || stats.value.is_active)) {
      if (!statsTimer) {
        statsTimer = setInterval(() => {
          loadStats();
        }, 5000);
      }
    } else if (statsTimer) {
      // If completed, clear timer and reload pages once to get new content
      clearInterval(statsTimer);
      statsTimer = null;
      loadPages();
      // Also refresh the currently opened page (right panel) so users see updated content
      refreshSelectedPage();
      // If currently viewing the graph, reload it as well so new nodes/edges show up
      if (props.view === "graph") {
        loadGraph();
      }
    }
  } catch (e) {
    /* ignore */
  }
}

// --- Manual page editing / version management ---------------------------

const editingPage = ref(false);
const savingPage = ref(false);
const editForm = ref({ title: "", summary: "", content: "" });
// Version the user started editing from — the optimistic-lock guard sent
// with the save. 409 → someone (or the pipeline) edited in between.
const editBaseVersion = ref(0);
const editConflictVersion = ref<number | null>(null);

const showRevisionDrawer = ref(false);

const showCreatePageDialog = ref(false);
const creatingPage = ref(false);
const createPageForm = ref({ title: "", slug: "", pageType: "concept", content: "" });
const createPageSlugTouched = ref(false);

// Navigating to another page (or view) silently drops an in-progress edit;
// the editor is inline, so a route-level guard would be overkill here.
watch(
  () => selectedPage.value?.slug,
  () => {
    editingPage.value = false;
    editConflictVersion.value = null;
  },
);

function startEditPage() {
  if (!selectedPage.value) return;
  editForm.value = {
    title: selectedPage.value.title,
    summary: selectedPage.value.summary || "",
    content: selectedPage.value.content || "",
  };
  editBaseVersion.value = selectedPage.value.version;
  editConflictVersion.value = null;
  editingPage.value = true;
}

function cancelEditPage() {
  editingPage.value = false;
  editConflictVersion.value = null;
}

async function savePageEdit(versionOverride?: number) {
  if (!selectedPage.value) return;
  const slug = selectedPage.value.slug;
  savingPage.value = true;
  try {
    const res = await updateWikiPage(props.knowledgeBaseId, slug, {
      title: editForm.value.title,
      summary: editForm.value.summary,
      content: editForm.value.content,
      version: versionOverride ?? editBaseVersion.value,
    });
    const updated = res;
    selectedPage.value = updated;
    editingPage.value = false;
    editConflictVersion.value = null;
    updateSidebarPageTitle(slug, updated.title);
    MessagePlugin.success(t("knowledgeEditor.wikiBrowser.editSaveSuccess"));
  } catch (e: any) {
    // The request interceptor rejects with a flattened { status, message,
    // ...body } object, so the conflict payload sits on `e` directly.
    if (e?.status === 409) {
      editConflictVersion.value = e?.current_version ?? 0;
    } else {
      MessagePlugin.error(e?.message || t("knowledgeEditor.wikiBrowser.editSaveFailed"));
    }
  } finally {
    savingPage.value = false;
  }
}

// overwriteSavePage resolves an edit conflict by re-saving on top of the
// server's current version (last write wins, but the loser's version stays
// in the revision history, so nothing is destroyed).
async function overwriteSavePage() {
  if (!selectedPage.value) return;
  try {
    const res = await getWikiPage(props.knowledgeBaseId, selectedPage.value.slug);
    await savePageEdit(res.version);
  } catch (e) {
    console.error("Failed to fetch latest version for overwrite save:", e);
    MessagePlugin.error(t("knowledgeEditor.wikiBrowser.editSaveFailed"));
  }
}

// reloadLatestIntoEditor resolves an edit conflict by discarding the local
// draft and re-opening the editor on the server's current content.
async function reloadLatestIntoEditor() {
  await refreshSelectedPage();
  startEditPage();
}

// Open state of the delete confirmation popover next to the delete button.
const deletePageConfirmOpen = ref(false);

function onConfirmDeletePage() {
  deletePageConfirmOpen.value = false;
  confirmDeletePage();
}

async function confirmDeletePage() {
  if (!selectedPage.value) return;
  const slug = selectedPage.value.slug;
  try {
    await deleteWikiPage(props.knowledgeBaseId, slug);
    MessagePlugin.success(t("knowledgeEditor.wikiBrowser.deletePageSuccess"));
    selectedPage.value = null;
    editingPage.value = false;
    loadPages();
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("knowledgeEditor.wikiBrowser.deletePageFailed"));
  }
}

function openRevisionDrawer() {
  if (!selectedPage.value) return;
  showRevisionDrawer.value = true;
}

function onPageReverted(page: WikiPage) {
  selectedPage.value = page;
  editingPage.value = false;
  updateSidebarPageTitle(page.slug, page.title);
}

function openCreatePageDialog() {
  createPageForm.value = { title: "", slug: "", pageType: "concept", content: "" };
  createPageSlugTouched.value = false;
  showCreatePageDialog.value = true;
}

// syncCreatePageSlug derives "<type>/<slugified-title>" while the user has
// not touched the slug field themselves. Only ASCII-ish titles produce a
// usable slug automatically; otherwise the user types one.
function syncCreatePageSlug() {
  if (createPageSlugTouched.value) return;
  const base = createPageForm.value.title
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9\s_-]/g, "")
    .replace(/\s+/g, "-")
    .replace(/-{2,}/g, "-")
    .replace(/^-|-$/g, "");
  createPageForm.value.slug = base ? `${createPageForm.value.pageType}/${base}` : "";
}

// The create-page inputs write their model explicitly: the Input component
// emits string | number, and the title also re-derives the slug on every
// keystroke, which the TDesign input used to do through its input event.
function onCreatePageTitleInput(value: string | number) {
  createPageForm.value.title = String(value);
  syncCreatePageSlug();
}

function onCreatePageSlugInput(value: string | number) {
  createPageForm.value.slug = String(value);
  createPageSlugTouched.value = true;
}

async function submitCreatePage() {
  const title = createPageForm.value.title.trim();
  const slug = createPageForm.value.slug.trim().replace(/^\/+|\/+$/g, "");
  if (!title || !slug) {
    MessagePlugin.warning(t("knowledgeEditor.wikiBrowser.newPageMissingFields"));
    return;
  }
  if (!/^[\p{L}\p{N}][\p{L}\p{N}\s_\-/]*$/u.test(slug)) {
    MessagePlugin.warning(t("knowledgeEditor.wikiBrowser.newPageSlugHint"));
    return;
  }
  creatingPage.value = true;
  try {
    await createWikiPage(props.knowledgeBaseId, {
      slug,
      title,
      page_type: createPageForm.value.pageType,
      content: createPageForm.value.content,
    });
    showCreatePageDialog.value = false;
    MessagePlugin.success(t("knowledgeEditor.wikiBrowser.newPageSuccess"));
    loadPages();
    navigateToSlug(slug);
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("knowledgeEditor.wikiBrowser.newPageFailed"));
  } finally {
    creatingPage.value = false;
  }
}

// updateSidebarPageTitle patches the already-loaded sidebar entries in place
// after a rename so the tree reflects the edit without a full reload.
function updateSidebarPageTitle(slug: string, title: string) {
  for (const p of pages.value) {
    if (p.slug === slug) p.title = title;
  }
  for (const bucket of Object.values(pagesByType.value)) {
    for (const p of bucket.items) {
      if (p.slug === slug) p.title = title;
    }
    for (const p of bucket.flatItems) {
      if (p.slug === slug) p.title = title;
    }
  }
}

function editSourceVisible(source?: string): boolean {
  return source === "user" || source === "revert";
}

function editSourceLabel(source?: string): string {
  switch (source) {
    case "user":
      return t("knowledgeEditor.wikiBrowser.editSourceUser");
    case "revert":
      return t("knowledgeEditor.wikiBrowser.editSourceRevert");
    default:
      return t("knowledgeEditor.wikiBrowser.editSourcePipeline");
  }
}

function editSourceIcon(source?: string): Component {
  switch (source) {
    case "user":
      return UserIcon;
    case "revert":
      return Undo2Icon;
    default:
      return FileCodeIcon;
  }
}

// Refresh the currently selected page's content without touching navigation history
async function refreshSelectedPage() {
  if (!selectedPage.value) return;
  const slug = selectedPage.value.slug;
  try {
    const res = await getWikiPage(props.knowledgeBaseId, slug);
    selectedPage.value = res;
  } catch (e) {
    console.error(`Failed to refresh wiki page ${slug}:`, e);
  }
}

// graphFilterTypesToArray returns the active allow-list as an array, or
// `undefined` when every known type is selected (in which case we want
// the backend to rank over the full page population, not a subset).
// Callers must check for "no types selected at all" separately and avoid
// the fetch — passing an empty string list to the backend is ambiguous
// there (empty == no filter == return everything, the opposite of what
// the user meant).
function graphFilterTypesToArray(): string[] | undefined {
  const all = ["summary", "entity", "concept", "synthesis", "comparison", "index"];
  if (all.every((t) => graphFilterTypes.value.has(t))) {
    return undefined;
  }
  return Array.from(graphFilterTypes.value);
}

function graphFilterSelectsNothing(): boolean {
  return graphFilterTypes.value.size === 0;
}

async function loadGraph() {
  graphLoading.value = true;
  graphReady.value = false;
  graphMode.value = "overview";
  graphCenter.value = "";
  if (graphFilterSelectsNothing()) {
    // User has deselected every type — render an empty canvas without
    // hitting the backend.
    graphData.value = { nodes: [], edges: [], meta: { mode: "overview", total: 0, returned: 0, truncated: false } };
    await nextTick();
    renderGraph();
    graphLoading.value = false;
    return;
  }
  try {
    const res = await getWikiGraph(props.knowledgeBaseId, {
      mode: "overview",
      limit: GRAPH_OVERVIEW_LIMIT,
      types: graphFilterTypesToArray(),
    });
    graphData.value = res;
    // Seed the search dropdown's empty-state with this overview snapshot
    // so opening the select without typing shows the top-500 by link_count
    // — matching what the old client-filter dropdown used to surface.
    // We re-seed on every overview load so filter toggles / KB changes
    // propagate; ego loads intentionally skip seeding so drilling into a
    // neighborhood doesn't shrink the default dropdown to a 20-node subgraph.
    setGraphSearchDefaultFromNodes(graphData.value?.nodes);
    // Returning to overview clears accumulated bloom state; the next ego
    // dive should start fresh rather than inherit an orphan generation map.
    resetBloomGenerations(graphData.value?.nodes);
    await nextTick();
    renderGraph();
    if (route.query.slug && typeof route.query.slug === "string") {
      graphSelectedSlug.value = null; // reset first to ensure watch triggers
      setTimeout(() => {
        handleGraphSearchSelect(route.query.slug as string);
      }, 300);
    }
  } catch (e) {
    console.error("Failed to load graph:", e);
  } finally {
    graphLoading.value = false;
  }
}

// loadEgoGraph fetches the neighborhood around a center slug and re-renders
// the canvas. Invoked when the user clicks "expand neighbors" in the drawer
// so they can drill into a page on a 4万+ wiki without ever having to
// download the full graph. Returning to the global top-N view is handled by
// loadGraph() again.
async function loadEgoGraph(slug: string, depth = GRAPH_EGO_DEFAULT_DEPTH) {
  if (!slug) return;
  graphLoading.value = true;
  graphReady.value = false;
  if (graphFilterSelectsNothing()) {
    graphData.value = {
      nodes: [],
      edges: [],
      meta: { mode: "ego", total: 0, returned: 0, truncated: false, center: slug, depth },
    };
    graphMode.value = "ego";
    graphCenter.value = slug;
    resetBloomGenerations(graphData.value.nodes);
    await nextTick();
    renderGraph();
    graphLoading.value = false;
    return;
  }
  try {
    const res = await getWikiGraph(props.knowledgeBaseId, {
      mode: "ego",
      center: slug,
      depth,
      limit: GRAPH_EGO_LIMIT,
      types: graphFilterTypesToArray(),
    });
    graphData.value = res;
    graphMode.value = "ego";
    graphCenter.value = slug;
    // Entering (or re-entering) a fresh ego view resets the bloom
    // generation counter — we're no longer accumulating on top of the
    // previous canvas, so every node belongs to generation 0.
    resetBloomGenerations(graphData.value?.nodes);
    await nextTick();
    renderGraph();
    // After a fresh ego render, preselect the center so the highlight /
    // drawer context matches what the user just asked for.
    graphSelectedSlug.value = slug;
  } catch (e) {
    console.error(`Failed to load ego graph for ${slug}:`, e);
  } finally {
    graphLoading.value = false;
  }
}

// ─── Bloom: additive neighbor expansion ──────────────────────────────────
//
// While loadEgoGraph replaces the canvas with a fresh ego view, bloom lets
// the user add a second (or Nth) ego around a neighbor WITHOUT losing the
// nodes already on screen. This matches how humans explore a knowledge
// graph interactively — "show me what's around A", "now also show me
// what's around B, but keep A visible for context".
//
// Three pieces of state cooperate:
//   - bloomGenerations: slug -> generation number. Generation 0 is the
//     initial ego view; each bloom increments a counter and tags the
//     newly arrived nodes with that generation. LRU eviction walks by
//     generation, oldest first.
//   - BLOOM_MAX_NODES: hard cap on rendered nodes. Past this point each
//     bloom triggers LRU eviction to keep the force simulation responsive.
//   - We reuse the existing graphData.value as the accumulator — new ego
//     responses are merged into it in place, then handed back to renderGraph
//     in preserveLayout mode.
const BLOOM_MAX_NODES = 1500;
const bloomGenerations = new Map<string, number>();
let bloomCurrentGeneration = 0;

function resetBloomGenerations(nodes: { slug: string }[] | undefined) {
  bloomGenerations.clear();
  bloomCurrentGeneration = 0;
  if (!nodes) return;
  for (const n of nodes) {
    bloomGenerations.set(n.slug, 0);
  }
}

async function loadBloomNeighbors(anchorSlug: string, depth = GRAPH_EGO_DEFAULT_DEPTH) {
  if (!anchorSlug) return;
  if (!graphData.value) return;
  if (graphMode.value !== "ego") {
    // Bloom only makes sense on top of an ego view. If we're still on the
    // overview, reuse loadEgoGraph to pivot cleanly — that's a less
    // surprising outcome than no-oping.
    await loadEgoGraph(anchorSlug, depth);
    return;
  }
  graphLoading.value = true;
  try {
    const res = await getWikiGraph(props.knowledgeBaseId, {
      mode: "ego",
      center: anchorSlug,
      depth,
      limit: GRAPH_EGO_LIMIT,
      types: graphFilterTypesToArray(),
    });
    const incoming = res;
    if (!Array.isArray(incoming.nodes)) return;

    bloomCurrentGeneration += 1;
    const merged = mergeGraphData(graphData.value, incoming, bloomCurrentGeneration);
    // Evict the oldest bloom generations if we've blown through the cap.
    // We never evict the ego center (the original anchor of the session),
    // the most recent bloom anchor, or the currently selected node — the
    // user's mental anchors must stay on screen.
    const protect = new Set<string>([graphCenter.value, anchorSlug, graphSelectedSlug.value || ""].filter(Boolean));
    evictBloomOverflow(merged, protect);

    graphData.value = merged;
    await nextTick();
    renderGraph({ preserveLayout: true, anchorSlug });
  } catch (e) {
    console.error(`Failed to bloom neighbors for ${anchorSlug}:`, e);
  } finally {
    graphLoading.value = false;
  }
}

// mergeGraphData folds `incoming` into `base` in-place-style (returns a
// new object for Vue reactivity but shares page node shape). Dedupes
// nodes by slug and edges by (source, target). New node slugs are tagged
// with `gen` so LRU knows which generation they belong to.
function mergeGraphData(base: WikiGraphData, incoming: WikiGraphData, gen: number): WikiGraphData {
  const nodeBySlug = new Map<string, WikiGraphData["nodes"][number]>();
  for (const n of base.nodes) nodeBySlug.set(n.slug, n);
  for (const n of incoming.nodes) {
    const existing = nodeBySlug.get(n.slug);
    if (!existing) {
      nodeBySlug.set(n.slug, n);
      bloomGenerations.set(n.slug, gen);
    } else if (n.familiar) {
      existing.familiar = true;
    }
  }
  const edgeKey = (e: { source: string; target: string }) => `${e.source}→${e.target}`;
  const edgeSeen = new Set<string>();
  const edges: WikiGraphData["edges"] = [];
  for (const e of base.edges) {
    const k = edgeKey(e);
    if (!edgeSeen.has(k)) {
      edgeSeen.add(k);
      edges.push(e);
    }
  }
  for (const e of incoming.edges) {
    const k = edgeKey(e);
    if (!edgeSeen.has(k)) {
      edgeSeen.add(k);
      edges.push(e);
    }
  }
  const nodes = Array.from(nodeBySlug.values());
  const familiarCount = nodes.filter((n) => n.familiar).length;
  return {
    nodes,
    edges,
    meta: {
      // Meta from the latest ego response describes the most recent
      // bloom, but we keep the overview denominator so the truncation
      // hint still reflects the KB-wide total.
      ...incoming.meta,
      returned: nodes.length,
      familiar_count: familiarCount || undefined,
    },
  };
}

// evictBloomOverflow walks generations oldest-first and drops nodes
// (plus their incident edges) until the total fits under BLOOM_MAX_NODES.
// `protect` holds slugs that must never be evicted (current center, most
// recent bloom anchor, current selection). Generation-0 nodes are
// protected too — those are the original ego view the user started with.
function evictBloomOverflow(data: WikiGraphData, protect: Set<string>) {
  if (data.nodes.length <= BLOOM_MAX_NODES) return;

  // Group slugs by generation descending-safe: we only evict gen >= 1.
  const byGen = new Map<number, string[]>();
  for (const n of data.nodes) {
    const g = bloomGenerations.get(n.slug) ?? 0;
    if (g === 0) continue;
    if (protect.has(n.slug)) continue;
    if (!byGen.has(g)) byGen.set(g, []);
    byGen.get(g)!.push(n.slug);
  }
  const gens = Array.from(byGen.keys()).sort((a, b) => a - b);

  const toRemove = new Set<string>();
  let remaining = data.nodes.length;
  for (const g of gens) {
    if (remaining <= BLOOM_MAX_NODES) break;
    for (const slug of byGen.get(g)!) {
      if (remaining <= BLOOM_MAX_NODES) break;
      toRemove.add(slug);
      remaining -= 1;
    }
  }
  if (toRemove.size === 0) return;

  data.nodes = data.nodes.filter((n) => !toRemove.has(n.slug));
  data.edges = data.edges.filter((e) => !toRemove.has(e.source) && !toRemove.has(e.target));
  for (const slug of toRemove) bloomGenerations.delete(slug);
}

// GROW_FRONTIER_CONCURRENCY is the number of parallel ego fetches we
// allow when the user asks us to expand the whole frontier at once. A
// 4万-page wiki can have ~100 frontier nodes; firing all 100 requests in
// parallel would hammer the backend and most responses would compete for
// the same DB connection pool anyway. 6 is chosen empirically: it keeps
// latency for the "whole frontier" op under ~2s for typical frontiers
// without spiking DB CPU.
const GROW_FRONTIER_CONCURRENCY = 6;

// GRAPH_SYSTEM_PAGE_TYPES are wiki page types that act as index-of-the-
// whole-KB rather than content nodes. They link out to every document
// page by design, so treating them as part of the frontier would cause
// one "Grow frontier" click to dump the entire wiki onto the canvas —
// exactly the opposite of what the user asked for ("show me more of the
// interesting neighborhood"). We keep them visible and individually
// expandable (double-click / shift-click / ⊕ all still work), but they
// don't participate in batch expansion.
const GRAPH_SYSTEM_PAGE_TYPES = new Set(["index"]);

function isFrontierCandidate(
  node: { slug: string; page_type: string; link_count: number },
  centerSlug: string,
  visibleDegree: number,
): boolean {
  if (node.slug === centerSlug) return false;
  if (GRAPH_SYSTEM_PAGE_TYPES.has(node.page_type)) return false;
  return (node.link_count || 0) > visibleDegree;
}

// growFrontier is the "one-click expand everything" operator. It finds
// every visible node that currently has an expansion ring (visible < link_count,
// not the ego center, not an Index/Log super-node), fires parallel ego
// fetches for them, merges all responses together and repaints the canvas
// preserving layout. This is the batch cousin of loadBloomNeighbors —
// one click grows the canvas along every branch instead of 100 individual
// click-by-click iterations.
async function growFrontier() {
  if (!graphData.value) return;
  if (graphMode.value !== "ego") {
    // Frontier expansion only makes sense on top of an ego layout.
    // Overview has its own pivot mechanism (expand a single node).
    return;
  }
  if (graphFilterSelectsNothing()) return;

  // Collect frontier nodes: visible degree < link_count AND not the ego
  // center AND not a system super-node. We compute visible degree inline
  // from edges so we don't depend on the stale adjacency snapshot from
  // the last render.
  const visibleDegree = new Map<string, number>();
  for (const e of graphData.value.edges) {
    visibleDegree.set(e.source, (visibleDegree.get(e.source) ?? 0) + 1);
    visibleDegree.set(e.target, (visibleDegree.get(e.target) ?? 0) + 1);
  }
  const frontier: string[] = [];
  for (const n of graphData.value.nodes) {
    if (isFrontierCandidate(n, graphCenter.value, visibleDegree.get(n.slug) ?? 0)) {
      frontier.push(n.slug);
    }
  }
  if (frontier.length === 0) return;

  graphLoading.value = true;
  try {
    // Concurrency-limited fan-out. We collect responses in order of
    // completion (doesn't matter — merge is commutative on the edge /
    // node sets) and ignore individual failures so one slow/broken node
    // doesn't sink the whole batch.
    const responses: WikiGraphData[] = [];
    let cursor = 0;
    async function worker() {
      while (cursor < frontier.length) {
        const idx = cursor++;
        const slug = frontier[idx];
        try {
          const res = await getWikiGraph(props.knowledgeBaseId, {
            mode: "ego",
            center: slug,
            depth: GRAPH_EGO_DEFAULT_DEPTH,
            limit: GRAPH_EGO_LIMIT,
            types: graphFilterTypesToArray(),
          });
          if (res.nodes) responses.push(res);
        } catch (e) {
          console.error(`growFrontier: ego fetch failed for ${slug}:`, e);
        }
      }
    }
    const workers: Promise<void>[] = [];
    const workerCount = Math.min(GROW_FRONTIER_CONCURRENCY, frontier.length);
    for (let i = 0; i < workerCount; i++) workers.push(worker());
    await Promise.all(workers);

    if (responses.length === 0) return;

    // All new arrivals belong to a single bloom generation — the user
    // performed one logical action, so LRU should evict them together.
    bloomCurrentGeneration += 1;
    const gen = bloomCurrentGeneration;
    let merged = graphData.value;
    for (const incoming of responses) {
      merged = mergeGraphData(merged, incoming, gen);
    }
    const protect = new Set<string>([graphCenter.value, graphSelectedSlug.value || ""].filter(Boolean));
    evictBloomOverflow(merged, protect);

    graphData.value = merged;
    await nextTick();
    // anchorSlug intentionally omitted — new nodes have no single natural
    // landing point, so we fall back to random canvas-center placement
    // and let the force simulation untangle them.
    renderGraph({ preserveLayout: true });
  } finally {
    graphLoading.value = false;
  }
}

async function selectPage(page: WikiPage) {
  try {
    if (selectedPage.value && selectedPage.value.id !== page.id) {
      navHistory.value.push(selectedPage.value);
    } else if (!selectedPage.value && activeSystemView.value) {
      // Jumping out of a system view (Index / Log) onto a page.
      // navHistory only holds WikiPages, so we stash the origin
      // system view separately; goBack restores it when the history
      // stack is empty.
      navFromSystemView.value = activeSystemView.value;
    }
    activeSystemView.value = "";
    const res = await getWikiPage(props.knowledgeBaseId, page.slug);
    selectedPage.value = res;
  } catch (e) {
    console.error("Failed to load wiki page:", e);
  }
}

async function navigateToSlug(slug: string) {
  try {
    if (selectedPage.value && selectedPage.value.slug !== slug) {
      navHistory.value.push(selectedPage.value);
    } else if (!selectedPage.value && activeSystemView.value) {
      // Clicking a [[slug]] from inside Index / Log — same rationale
      // as selectPage above: record the system-view origin so the
      // reader's back arrow can return to it.
      navFromSystemView.value = activeSystemView.value;
    }
    activeSystemView.value = "";
    const res = await getWikiPage(props.knowledgeBaseId, slug);
    selectedPage.value = res;
  } catch (e) {
    console.error(`Failed to navigate to ${slug}:`, e);
  }
}

function goBack() {
  const prev = navHistory.value.pop();
  if (prev) {
    selectedPage.value = prev;
    return;
  }
  // History stack is empty but we remember the page was opened from
  // a system view — restore that instead of leaving the reader empty.
  if (navFromSystemView.value) {
    const view = navFromSystemView.value;
    navFromSystemView.value = "";
    selectedPage.value = null;
    if (view === "index") openIndexView();
  }
}

async function doSearch() {
  if (!searchQuery.value.trim()) {
    searchResults.value = null;
    return;
  }
  loading.value = true;
  try {
    const res = await searchWikiPages(props.knowledgeBaseId, searchQuery.value);
    const hits: WikiPage[] = res.pages ?? [];
    searchResults.value = hits;
    // Also seed `pages.value` with hits so slugDisplayName / navigation
    // heuristics keep resolving titles correctly without re-fetching.
    const seen = new Set(pages.value.map((p) => p.id));
    for (const p of hits) {
      if (!seen.has(p.id)) pages.value.push(p);
    }
  } catch (e) {
    console.error("Wiki search failed:", e);
  } finally {
    loading.value = false;
  }
}

function toggleArrows() {
  showArrows.value = !showArrows.value;
  for (const e of graphEdgeElsRef) {
    if (showArrows.value) {
      e.line.setAttribute("marker-end", "url(#arrow-end)");
      if (e.bidir) e.line.setAttribute("marker-start", "url(#arrow-start)");
    } else {
      e.line.removeAttribute("marker-end");
      e.line.removeAttribute("marker-start");
    }
  }
}

function formatDate(dateStr: string) {
  if (!dateStr) return "";
  const d = new Date(dateStr);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}/${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

// Convert slug like "entity/acme-corp" to a readable label "acme-corp"
function slugDisplayName(slug: string): string {
  // Find the page title if loaded
  const page = pages.value.find((p) => p.slug === slug);
  if (page) return page.title;
  // Fallback: strip type prefix, replace hyphens
  const parts = slug.split("/");
  return parts.length > 1 ? parts.slice(1).join("/") : slug;
}

// ─── Graph Rendering (interactive SVG force-directed graph) ───
// Features: drag nodes, pan canvas, zoom, hover highlight, click to open drawer, legend

interface GNode {
  x: number;
  y: number;
  vx: number;
  vy: number;
  slug: string;
  title: string;
  type: string;
  linkCount: number;
  pinned: boolean;
  familiar: boolean;
}

// Persistent graph state so it survives re-renders
let graphNodes: GNode[] = [];
let graphSvg: SVGSVGElement | null = null;
let graphAnimFrame = 0;
// Shared timer for debouncing node mouseleave -> mouseenter transitions.
// Prevents flickering when the pointer quickly moves between adjacent nodes.
let graphHoverLeaveTimer: ReturnType<typeof setTimeout> | null = null;

// Used for graph search centering interaction
let graphPanZoomRef: {
  setScale: (s: number) => void;
  setTranslate: (x: number, y: number) => void;
  apply: () => void;
  flyTo: (x: number, y: number, s?: number, duration?: number) => void;
  getScale: () => number;
} | null = null;

const graphHighlightSlug = ref<string | null>(null);
const graphSelectedSlug = ref<string | null>(null);

// Color map for node types
const nodeColorMap: Record<string, string> = {
  summary: "#0052d9",
  entity: "#2ba471",
  concept: "#e37318",
  synthesis: "#0594fa",
  comparison: "#d54941",
  index: "#8c8c8c",
};

// RenderGraphOpts tweaks how renderGraph initializes node positions when
// repainting the canvas. The default (no opts) does a full layout reset —
// every node gets a fresh circular starting position and the force
// simulation runs from scratch. With `preserveLayout: true` we reuse the
// x/y/vx/vy of any node that already existed in the previous graphNodes
// list, and only new nodes get initial positions. This is what the
// "bloom neighbors" interaction needs: when the user expands a second
// ego around a neighbor, the nodes they already see don't jump to new
// positions — only the newly arrived neighbors fly in.
interface RenderGraphOpts {
  preserveLayout?: boolean;
  // anchorSlug: if set and the node is new, it is placed near the anchor
  // with a small random jitter so related nodes visually land together.
  anchorSlug?: string;
}

function renderGraph(opts: RenderGraphOpts = {}) {
  const container = graphRef.value;
  const data = graphData.value;
  if (!container) return;
  if (!data || !data.nodes?.length) {
    container.innerHTML = "";
    return;
  }
  const graph = data;

  // Stop any previous animation
  if (graphAnimFrame) {
    cancelAnimationFrame(graphAnimFrame);
    graphAnimFrame = 0;
  }
  if (graphHoverLeaveTimer) {
    clearTimeout(graphHoverLeaveTimer);
    graphHoverLeaveTimer = null;
  }

  const width = container.clientWidth || 800;
  const height = container.clientHeight || 600;

  // Snapshot prior node coordinates before we rebuild graphNodes. Used
  // when preserveLayout is true to avoid the whole canvas jumping during
  // an incremental bloom.
  const priorCoords = new Map<string, { x: number; y: number; vx: number; vy: number; pinned: boolean }>();
  if (opts.preserveLayout) {
    for (const n of graphNodes) {
      priorCoords.set(n.slug, { x: n.x, y: n.y, vx: n.vx, vy: n.vy, pinned: n.pinned });
    }
  }

  // Create SVG
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("viewBox", `0 0 ${width} ${height}`);
  svg.style.width = "100%";
  svg.style.height = "100%";
  container.innerHTML = "";
  container.appendChild(svg);
  graphSvg = svg;

  // Root group for pan/zoom transform
  const rootG = document.createElementNS("http://www.w3.org/2000/svg", "g");
  rootG.setAttribute("class", "graph-root");
  svg.appendChild(rootG);

  // Edge group (below nodes)
  const edgeG = document.createElementNS("http://www.w3.org/2000/svg", "g");
  rootG.appendChild(edgeG);

  // Node group (above edges)
  const nodeG = document.createElementNS("http://www.w3.org/2000/svg", "g");
  rootG.appendChild(nodeG);

  // Build adjacency for highlight
  const adjacency = new Map<string, Set<string>>();
  for (const edge of graph.edges) {
    if (!adjacency.has(edge.source)) adjacency.set(edge.source, new Set());
    if (!adjacency.has(edge.target)) adjacency.set(edge.target, new Set());
    adjacency.get(edge.source)!.add(edge.target);
    adjacency.get(edge.target)!.add(edge.source);
  }

  // Locate the anchor's prior coordinates so new nodes land near it in
  // bloom mode. Falls back to canvas center if the anchor is itself new
  // (e.g. ego was pivoted rather than bloomed).
  const anchorCoord = opts.anchorSlug ? priorCoords.get(opts.anchorSlug) : undefined;
  const anchorX = anchorCoord?.x ?? width / 2;
  const anchorY = anchorCoord?.y ?? height / 2;

  // Build nodes
  const nodeMap = new Map<string, GNode>();
  graphNodes = graph.nodes.map((n, i) => {
    const prior = opts.preserveLayout ? priorCoords.get(n.slug) : undefined;
    let x: number;
    let y: number;
    let vx: number;
    let vy: number;
    let pinned: boolean;
    if (prior) {
      // Reuse the node's existing position so the user's mental map of
      // the canvas stays stable across bloom iterations.
      x = prior.x;
      y = prior.y;
      vx = prior.vx;
      vy = prior.vy;
      pinned = prior.pinned;
    } else if (opts.preserveLayout && opts.anchorSlug) {
      // New node arriving during a bloom — spawn it right next to the
      // anchor with a small random kick so the force simulation pushes
      // it into place alongside its siblings.
      const jitterR = 40;
      const angle = Math.random() * Math.PI * 2;
      x = anchorX + jitterR * Math.cos(angle);
      y = anchorY + jitterR * Math.sin(angle);
      vx = 0;
      vy = 0;
      pinned = false;
    } else {
      // Full repaint — classic circular layout.
      const angle = (2 * Math.PI * i) / graph.nodes.length;
      const r = Math.min(width, height) * 0.35;
      x = width / 2 + r * Math.cos(angle) + (Math.random() - 0.5) * 50;
      y = height / 2 + r * Math.sin(angle) + (Math.random() - 0.5) * 50;
      vx = 0;
      vy = 0;
      pinned = false;
    }
    const node: GNode = {
      x,
      y,
      vx,
      vy,
      slug: n.slug,
      title: n.title,
      type: n.page_type,
      linkCount: n.link_count || 0,
      pinned,
      familiar: !!n.familiar,
    };
    nodeMap.set(n.slug, node);
    return node;
  });

  // Node radius based on link count (logarithmic scale to prevent overly large nodes)
  function nodeRadius(n: GNode) {
    return Math.max(8, Math.min(24, 8 + Math.log(n.linkCount + 1) * 4));
  }

  // Define arrow markers in SVG <defs>
  const defs = document.createElementNS("http://www.w3.org/2000/svg", "defs");

  // Single-direction arrow (at end)
  const markerEnd = document.createElementNS("http://www.w3.org/2000/svg", "marker");
  markerEnd.setAttribute("id", "arrow-end");
  markerEnd.setAttribute("viewBox", "0 0 10 6");
  markerEnd.setAttribute("refX", "10");
  markerEnd.setAttribute("refY", "3");
  markerEnd.setAttribute("markerWidth", "8");
  markerEnd.setAttribute("markerHeight", "6");
  markerEnd.setAttribute("orient", "auto");
  const arrowPath = document.createElementNS("http://www.w3.org/2000/svg", "path");
  arrowPath.setAttribute("d", "M0,0 L10,3 L0,6 L2,3 Z");
  arrowPath.setAttribute("fill", "#c0c4cc");
  markerEnd.appendChild(arrowPath);
  defs.appendChild(markerEnd);

  // Bidirectional: arrow at start (reverse)
  const markerStart = document.createElementNS("http://www.w3.org/2000/svg", "marker");
  markerStart.setAttribute("id", "arrow-start");
  markerStart.setAttribute("viewBox", "0 0 10 6");
  markerStart.setAttribute("refX", "0");
  markerStart.setAttribute("refY", "3");
  markerStart.setAttribute("markerWidth", "8");
  markerStart.setAttribute("markerHeight", "6");
  markerStart.setAttribute("orient", "auto");
  const arrowPathStart = document.createElementNS("http://www.w3.org/2000/svg", "path");
  arrowPathStart.setAttribute("d", "M10,0 L0,3 L10,6 L8,3 Z");
  arrowPathStart.setAttribute("fill", "#c0c4cc");
  markerStart.appendChild(arrowPathStart);
  defs.appendChild(markerStart);

  // Highlighted arrows
  for (const id of ["arrow-end-hl", "arrow-start-hl"]) {
    const m = document.createElementNS("http://www.w3.org/2000/svg", "marker");
    m.setAttribute("id", id);
    m.setAttribute("viewBox", "0 0 10 6");
    m.setAttribute("refX", id.includes("end") ? "10" : "0");
    m.setAttribute("refY", "3");
    m.setAttribute("markerWidth", "8");
    m.setAttribute("markerHeight", "6");
    m.setAttribute("orient", "auto");
    const p = document.createElementNS("http://www.w3.org/2000/svg", "path");
    p.setAttribute("d", id.includes("end") ? "M0,0 L10,3 L0,6 L2,3 Z" : "M10,0 L0,3 L10,6 L8,3 Z");
    p.setAttribute("fill", "#0052d9");
    m.appendChild(p);
    defs.appendChild(m);
  }

  // Drop shadow filter for nodes
  const filter = document.createElementNS("http://www.w3.org/2000/svg", "filter");
  filter.setAttribute("id", "node-shadow");
  filter.setAttribute("x", "-20%");
  filter.setAttribute("y", "-20%");
  filter.setAttribute("width", "140%");
  filter.setAttribute("height", "140%");
  filter.innerHTML = `<feDropShadow dx="0" dy="2" stdDeviation="3" flood-color="#000" flood-opacity="0.15"/>`;
  defs.appendChild(filter);

  svg.appendChild(defs);

  // Detect bidirectional edges (A→B and B→A both exist)
  const edgePairSet = new Set<string>();
  for (const edge of graph.edges) {
    edgePairSet.add(`${edge.source}→${edge.target}`);
  }

  // Create SVG elements for edges (deduplicate bidirectional into single line with double arrows)
  type EdgeEl = { line: SVGLineElement; source: string; target: string; bidir: boolean };
  const edgeEls: EdgeEl[] = [];
  const processedPairs = new Set<string>();

  for (const edge of graph.edges) {
    const pairKey = [edge.source, edge.target].sort().join("↔");
    if (processedPairs.has(pairKey)) continue;
    processedPairs.add(pairKey);

    const bidir = edgePairSet.has(`${edge.target}→${edge.source}`);

    const line = document.createElementNS("http://www.w3.org/2000/svg", "line");
    line.setAttribute("stroke", "#c0c4cc");
    line.setAttribute("stroke-width", "1.2");
    line.setAttribute("stroke-opacity", "0.4");
    line.setAttribute("marker-end", "url(#arrow-end)");
    line.style.transition = "stroke 0.2s, stroke-width 0.2s, stroke-opacity 0.2s";
    if (bidir) {
      line.setAttribute("marker-start", "url(#arrow-start)");
    }
    edgeG.appendChild(line);
    edgeEls.push({ line, source: edge.source, target: edge.target, bidir });
  }

  // Create SVG elements for nodes
  const nodeEls: {
    g: SVGGElement;
    circle: SVGCircleElement;
    text: SVGTextElement;
    activeRing: SVGCircleElement;
    node: GNode;
  }[] = [];
  for (const n of graphNodes) {
    const g = document.createElementNS("http://www.w3.org/2000/svg", "g");
    g.style.cursor = "pointer";

    const r = nodeRadius(n);

    // Expansion hint ring — dashed outer circle that appears when the
    // node has neighbors the user hasn't loaded yet. Without this signal
    // users have no way to tell a fully-explored node from one that's
    // still hiding 80 more connections just out of view, so they either
    // click "bloom" on everything (wasteful) or on nothing (miss the
    // interesting pages). adjacency here is the undirected neighbor set
    // we've already built from graph.edges; link_count is the KB-wide
    // in+out degree reported by the backend. Diff > 0 means there's
    // more to fetch.
    //
    // Exception: the ego-mode center node already received every
    // reachable neighbor from the BFS expansion, so any remaining gap
    // against link_count is dead refs / filtered pages, NOT loadable
    // neighbors. Drawing a dashed ring there would mislead users into
    // thinking there's something to click.
    const visibleNeighbors = adjacency.get(n.slug)?.size ?? 0;
    const hiddenNeighbors = Math.max(0, n.linkCount - visibleNeighbors);
    const isEgoCenter = graph.meta?.mode === "ego" && graph.meta.center === n.slug;
    const showExpansionRing = hiddenNeighbors > 0 && !isEgoCenter;
    const expansionRing = document.createElementNS("http://www.w3.org/2000/svg", "circle");
    expansionRing.setAttribute("r", String(r + 3));
    expansionRing.setAttribute("fill", "none");
    expansionRing.setAttribute("stroke", nodeColorMap[n.type] || "#8c8c8c");
    expansionRing.setAttribute("stroke-width", "1.5");
    expansionRing.setAttribute("stroke-dasharray", "3 3");
    expansionRing.setAttribute("pointer-events", "none");
    expansionRing.style.opacity = showExpansionRing ? "0.55" : "0";
    expansionRing.style.transition = "opacity 0.2s";
    expansionRing.classList.add("node-expansion-ring");
    g.appendChild(expansionRing);

    // Solid outer ring: this page was built from a document the current
    // person keeps citing. Distinct from the dashed expansion ring so
    // "I use this" and "there are more neighbors" do not look the same.
    if (n.familiar) {
      const familiarRing = document.createElementNS("http://www.w3.org/2000/svg", "circle");
      familiarRing.setAttribute("r", String(r + 7));
      familiarRing.setAttribute("fill", "none");
      familiarRing.setAttribute("stroke", "#0052d9");
      familiarRing.setAttribute("stroke-width", "2");
      familiarRing.setAttribute("pointer-events", "none");
      familiarRing.style.opacity = "0.9";
      familiarRing.classList.add("node-familiar-ring");
      g.appendChild(familiarRing);
    }

    // Pulse ring for selected state
    const activeRing = document.createElementNS("http://www.w3.org/2000/svg", "circle");
    activeRing.setAttribute("r", String(r + 5));
    activeRing.setAttribute("fill", "none");
    activeRing.setAttribute("stroke", nodeColorMap[n.type] || "#8c8c8c");
    activeRing.setAttribute("stroke-width", "2");
    activeRing.style.opacity = "0";
    activeRing.style.transition = "opacity 0.2s";
    activeRing.classList.add("node-active-ring");
    g.appendChild(activeRing);

    const circle = document.createElementNS("http://www.w3.org/2000/svg", "circle");
    circle.setAttribute("r", String(r));
    circle.setAttribute("fill", nodeColorMap[n.type] || "#8c8c8c");
    circle.setAttribute("stroke", "#fff");
    circle.setAttribute("stroke-width", "2");
    // circle.setAttribute('filter', 'url(#node-shadow)')
    circle.style.transition = "r 0.2s, stroke-width 0.2s, opacity 0.2s";
    g.appendChild(circle);

    // Text label wrapper for better readability
    const textBg = document.createElementNS("http://www.w3.org/2000/svg", "rect");
    g.appendChild(textBg); // we'll size this after we know text size

    const text = document.createElementNS("http://www.w3.org/2000/svg", "text");
    text.setAttribute("text-anchor", "middle");
    text.setAttribute("dy", String(r + 14));
    text.setAttribute("font-size", "11");
    text.setAttribute("fill", "var(--td-text-color-secondary)");
    text.setAttribute("pointer-events", "none");
    text.style.transition = "opacity 0.2s"; // Smooth fade in/out
    text.style.textShadow =
      "0 1px 3px var(--td-bg-color-container), 0 -1px 3px var(--td-bg-color-container), 1px 0 3px var(--td-bg-color-container), -1px 0 3px var(--td-bg-color-container)";
    text.textContent = n.title.length > 14 ? n.title.substring(0, 14) + "…" : n.title;
    g.appendChild(text);

    // Hover bloom button — the ⊕ badge floating off the node's upper-right.
    // Invisible by default; fades in on mouseenter when bloom would
    // actually do something (node has hidden neighbors and isn't the ego
    // center / isn't on overview). Clicking it skips the drawer round-trip
    // and pulls the neighbors straight onto the canvas.
    //
    // Stacking order note: this element has to come AFTER text so SVG's
    // painter's algorithm draws it on top; the node-shadow filter and
    // the drawer cover it otherwise.
    let bloomBtn: SVGGElement | null = null;
    const bloomBtnEligible = !isEgoCenter && graph.meta?.mode === "ego" && hiddenNeighbors > 0;
    if (bloomBtnEligible) {
      bloomBtn = document.createElementNS("http://www.w3.org/2000/svg", "g");
      bloomBtn.classList.add("node-bloom-btn");
      bloomBtn.style.opacity = "0";
      bloomBtn.style.transition = "opacity 0.15s";
      bloomBtn.style.pointerEvents = "none"; // lit up only on hover
      bloomBtn.style.cursor = "pointer";
      // Position at 45° up-right of the node center, just past the
      // expansion ring so it doesn't overlap the node glyph.
      const btnOffset = r + 6;
      const btnX = Math.SQRT1_2 * btnOffset;
      const btnY = -Math.SQRT1_2 * btnOffset;

      const btnBg = document.createElementNS("http://www.w3.org/2000/svg", "circle");
      btnBg.setAttribute("cx", String(btnX));
      btnBg.setAttribute("cy", String(btnY));
      btnBg.setAttribute("r", "8");
      btnBg.setAttribute("fill", "var(--td-bg-color-container, #fff)");
      btnBg.setAttribute("stroke", "var(--td-brand-color, #0052d9)");
      btnBg.setAttribute("stroke-width", "1.5");
      bloomBtn.appendChild(btnBg);

      // ⊕ drawn as two short lines — cross-browser-safer than a text glyph
      const btnCrossV = document.createElementNS("http://www.w3.org/2000/svg", "line");
      btnCrossV.setAttribute("x1", String(btnX));
      btnCrossV.setAttribute("x2", String(btnX));
      btnCrossV.setAttribute("y1", String(btnY - 4));
      btnCrossV.setAttribute("y2", String(btnY + 4));
      btnCrossV.setAttribute("stroke", "var(--td-brand-color, #0052d9)");
      btnCrossV.setAttribute("stroke-width", "1.8");
      btnCrossV.setAttribute("stroke-linecap", "round");
      bloomBtn.appendChild(btnCrossV);

      const btnCrossH = document.createElementNS("http://www.w3.org/2000/svg", "line");
      btnCrossH.setAttribute("x1", String(btnX - 4));
      btnCrossH.setAttribute("x2", String(btnX + 4));
      btnCrossH.setAttribute("y1", String(btnY));
      btnCrossH.setAttribute("y2", String(btnY));
      btnCrossH.setAttribute("stroke", "var(--td-brand-color, #0052d9)");
      btnCrossH.setAttribute("stroke-width", "1.8");
      btnCrossH.setAttribute("stroke-linecap", "round");
      bloomBtn.appendChild(btnCrossH);

      bloomBtn.addEventListener("click", (e) => {
        e.stopPropagation();
        loadBloomNeighbors(n.slug);
      });
      g.appendChild(bloomBtn);
    }

    // Hover highlight
    // We debounce the "leave" side so that quickly sliding the pointer from
    // one node to the next doesn't flash through the fully-unhighlighted state
    // (which is what caused the whole-graph flickering).
    g.addEventListener("mouseenter", () => {
      if (graphHoverLeaveTimer) {
        clearTimeout(graphHoverLeaveTimer);
        graphHoverLeaveTimer = null;
      }
      if (bloomBtn) {
        bloomBtn.style.opacity = "1";
        bloomBtn.style.pointerEvents = "auto";
      }
      if (!graphSelectedSlug.value) {
        if (graphHighlightSlug.value === n.slug) return;
        graphHighlightSlug.value = n.slug;
        applyHighlight(n.slug, adjacency, nodeEls, edgeEls);
      } else if (graphSelectedSlug.value !== n.slug) {
        if (graphHighlightSlug.value === n.slug) return;
        graphHighlightSlug.value = n.slug;
        applyHighlight(graphSelectedSlug.value, adjacency, nodeEls, edgeEls, n.slug);
      }
    });
    g.addEventListener("mouseleave", () => {
      if (graphHoverLeaveTimer) clearTimeout(graphHoverLeaveTimer);
      if (bloomBtn) {
        bloomBtn.style.opacity = "0";
        bloomBtn.style.pointerEvents = "none";
      }
      graphHoverLeaveTimer = setTimeout(() => {
        graphHoverLeaveTimer = null;
        if (!graphSelectedSlug.value) {
          graphHighlightSlug.value = null;
          clearHighlight(nodeEls, edgeEls);
        } else {
          graphHighlightSlug.value = null;
          applyHighlight(graphSelectedSlug.value, adjacency, nodeEls, edgeEls);
        }
      }, 60);
    });

    // Single-click behaviour + keyboard-modifier shortcuts to skip the
    // drawer round-trip for power-user navigation:
    //
    //   plain click  → select & open drawer (original behaviour)
    //   shift+click  → bloom this node's neighbors onto the canvas
    //   double-click → pivot to this node as the new ego center
    //
    // Drawer is by far the slower path (page fetch + render), so adding
    // canvas-direct expand / bloom removes a 2-3 second round-trip from
    // every exploration step. We still want shift+click to be
    // discoverable, so the drawer's buttons remain — they're the
    // keyboard-free fallback.
    //
    // Implementation note: we listen to click AND dblclick. The browser
    // fires both click events of a dblclick too, but we debounce via
    // `pendingSingleClick` — the first click sets a 220ms timer to open
    // the drawer; dblclick arriving inside that window cancels the
    // timer and runs expand instead. `event.detail` (click count) is
    // less portable across synthetic events, so we track state explicitly.
    let pendingSingleClick: ReturnType<typeof setTimeout> | null = null;
    g.addEventListener("click", (e) => {
      e.stopPropagation();

      if (e.shiftKey) {
        // Shift = Bloom. Skip the drawer entirely, skip selection — the
        // user's intent is "bring in the neighbors", not "read this page".
        // Center / isOverview cases are handled inside loadBloomNeighbors
        // (center no-ops, overview pivots to ego).
        if (pendingSingleClick) {
          clearTimeout(pendingSingleClick);
          pendingSingleClick = null;
        }
        loadBloomNeighbors(n.slug);
        return;
      }

      if (pendingSingleClick) clearTimeout(pendingSingleClick);
      pendingSingleClick = setTimeout(() => {
        pendingSingleClick = null;

        // Select and highlight
        graphSelectedSlug.value = n.slug;
        applyHighlight(n.slug, adjacency, nodeEls, edgeEls);

        // Auto pan to center the node, shifted left for drawer
        if (graphPanZoomRef) {
          const container = graphRef.value;
          if (container) {
            const width = container.clientWidth;
            const height = container.clientHeight;
            graphPanZoomRef.flyTo(
              width / 2 - n.x * graphPanZoomRef.getScale() - 240,
              height / 2 - n.y * graphPanZoomRef.getScale(),
            );
          }
        }

        // Open drawer (it will handle drawer visibility and fetching content)
        openGraphDrawer(n.slug);
      }, 220);
    });

    g.addEventListener("dblclick", (e) => {
      e.stopPropagation();
      if (pendingSingleClick) {
        clearTimeout(pendingSingleClick);
        pendingSingleClick = null;
      }
      loadEgoGraph(n.slug);
    });

    // Drag support
    setupDrag(g, n, nodeMap, edgeEls, nodeEls, nodeRadius);

    nodeG.appendChild(g);
    nodeEls.push({ g, circle, text, activeRing, node: n });
  }

  // Pan & zoom on SVG background
  setupPanZoom(svg, rootG);

  // Animated force simulation
  let alpha = 1.0;
  function tick() {
    alpha *= 0.985;
    if (alpha < 0.02) {
      graphAnimFrame = 0;
      return;
    }

    // Repulsion: Optimized using 1D spatial sorting (X-axis) to reduce O(n²) to O(n log n)
    // This allows smooth rendering even for > 1000 nodes
    const sortedNodes = [...graphNodes].sort((a, b) => a.x - b.x);
    const MAX_REPULSION_DIST = 300; // Only calculate repulsion for nodes within 300px
    const MAX_REPULSION_DIST_SQ = MAX_REPULSION_DIST * MAX_REPULSION_DIST;

    for (let i = 0; i < sortedNodes.length; i++) {
      const n1 = sortedNodes[i];
      for (let j = i + 1; j < sortedNodes.length; j++) {
        const n2 = sortedNodes[j];
        const dx = n2.x - n1.x;

        // Because nodes are sorted by X, if dx > MAX_REPULSION_DIST,
        // all subsequent n2 nodes will also be too far on the X axis, so we can break early
        if (dx > MAX_REPULSION_DIST) break;

        const dy = n2.y - n1.y;
        if (Math.abs(dy) > MAX_REPULSION_DIST) continue; // Too far on Y axis

        const distSq = dx * dx + dy * dy;
        if (distSq > MAX_REPULSION_DIST_SQ) continue;

        const dist = Math.sqrt(distSq) || 1;
        // Prevent extremely high repulsion when nodes are very close
        const force = ((200 * alpha) / Math.max(distSq, 100)) * 60;
        const fx = (dx / dist) * force;
        const fy = (dy / dist) * force;

        if (!n1.pinned) {
          n1.vx -= fx;
          n1.vy -= fy;
        }
        if (!n2.pinned) {
          n2.vx += fx;
          n2.vy += fy;
        }
      }
    }

    // Attraction along edges
    for (const edge of graph.edges) {
      const s = nodeMap.get(edge.source);
      const t = nodeMap.get(edge.target);
      if (!s || !t) continue;
      const dx = t.x - s.x;
      const dy = t.y - s.y;
      const dist = Math.sqrt(dx * dx + dy * dy) || 1;
      const force = (dist - 120) * 0.005 * alpha;
      const fx = (dx / dist) * force;
      const fy = (dy / dist) * force;
      if (!s.pinned) {
        s.vx += fx;
        s.vy += fy;
      }
      if (!t.pinned) {
        t.vx -= fx;
        t.vy -= fy;
      }
    }

    // Center gravity
    // Increase gravity slightly when there are more nodes to prevent the graph from expanding too much
    const gravityStrength = Math.min(0.01, 0.001 + graphNodes.length * 0.00002);
    for (const n of graphNodes) {
      if (n.pinned) continue;
      n.vx += (width / 2 - n.x) * gravityStrength * alpha;
      n.vy += (height / 2 - n.y) * gravityStrength * alpha;
    }

    // Apply velocity
    for (const n of graphNodes) {
      if (n.pinned) continue;
      n.vx *= 0.6;
      n.vy *= 0.6;
      // Cap velocity to prevent nodes from flying off screen during initial explosive layout
      const v = Math.sqrt(n.vx * n.vx + n.vy * n.vy);
      if (v > 20) {
        n.vx = (n.vx / v) * 20;
        n.vy = (n.vy / v) * 20;
      }
      n.x += n.vx;
      n.y += n.vy;
    }

    // Update SVG positions
    for (const { g, node } of nodeEls) {
      g.setAttribute("transform", `translate(${node.x},${node.y})`);
    }
    for (const e of edgeEls) {
      const s = nodeMap.get(e.source);
      const t = nodeMap.get(e.target);
      if (s && t) {
        setEdgePositions(e.line, s, t, nodeRadius);
      }
    }

    graphAnimFrame = requestAnimationFrame(tick);
  }

  // Initial positions before first paint
  for (const { g, node } of nodeEls) {
    g.setAttribute("transform", `translate(${node.x},${node.y})`);
  }
  for (const e of edgeEls) {
    const s = nodeMap.get(e.source);
    const t = nodeMap.get(e.target);
    if (s && t) {
      setEdgePositions(e.line, s, t, nodeRadius);
    }
  }

  // Store node and edge refs for search and arrow toggle
  graphNodeElsRef = nodeEls;
  graphEdgeElsRef = edgeEls.map((e) => ({ line: e.line, source: e.source, target: e.target, bidir: e.bidir }));
  graphAdjacencyRef = adjacency;

  graphAnimFrame = requestAnimationFrame(tick);
  graphReady.value = true;
}

// Set edge line positions, shortened to stop at node circle boundary so arrows are visible
function setEdgePositions(line: SVGLineElement, s: GNode, t: GNode, nodeRadius: (n: GNode) => number) {
  const dx = t.x - s.x;
  const dy = t.y - s.y;
  const dist = Math.sqrt(dx * dx + dy * dy) || 1;
  const ux = dx / dist;
  const uy = dy / dist;

  // Shorten each end by the node radius + arrow margin
  const rS = nodeRadius(s) + 4;
  const rT = nodeRadius(t) + 4;

  line.setAttribute("x1", String(s.x + ux * rS));
  line.setAttribute("y1", String(s.y + uy * rS));
  line.setAttribute("x2", String(t.x - ux * rT));
  line.setAttribute("y2", String(t.y - uy * rT));
}

// ─── Drag ───
function setupDrag(
  g: SVGGElement,
  node: GNode,
  nodeMap: Map<string, GNode>,
  edgeEls: { line: SVGLineElement; source: string; target: string; bidir: boolean }[],
  nodeEls: {
    g: SVGGElement;
    circle: SVGCircleElement;
    text: SVGTextElement;
    activeRing: SVGCircleElement;
    node: GNode;
  }[],
  nodeRadius: (n: GNode) => number,
) {
  let dragging = false;
  let startX = 0,
    startY = 0;

  function getPoint(e: MouseEvent | Touch) {
    const svg = graphSvg;
    if (!svg) return { x: e.clientX, y: e.clientY };
    const pt = svg.createSVGPoint();
    pt.x = e.clientX;
    pt.y = e.clientY;
    const rootG = svg.querySelector(".graph-root") as SVGGElement;
    const ctm = rootG?.getCTM()?.inverse();
    if (ctm) {
      const svgP = pt.matrixTransform(ctm);
      return { x: svgP.x, y: svgP.y };
    }
    return { x: e.clientX, y: e.clientY };
  }

  function onStart(e: MouseEvent) {
    if (e.button !== 0) return;
    e.stopPropagation();
    dragging = true;
    node.pinned = true;
    const p = getPoint(e);
    startX = p.x - node.x;
    startY = p.y - node.y;
    g.querySelector("circle")?.setAttribute("stroke", nodeColorMap[node.type] || "#8c8c8c");
    g.querySelector("circle")?.setAttribute("stroke-width", "3");
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onEnd);
  }

  function onMove(e: MouseEvent) {
    if (!dragging) return;
    const p = getPoint(e);
    node.x = p.x - startX;
    node.y = p.y - startY;
    node.vx = 0;
    node.vy = 0;
    g.setAttribute("transform", `translate(${node.x},${node.y})`);
    // Update connected edges immediately
    for (const edge of edgeEls) {
      if (edge.source === node.slug || edge.target === node.slug) {
        const sn = nodeMap.get(edge.source);
        const tn = nodeMap.get(edge.target);
        if (sn && tn) setEdgePositions(edge.line, sn, tn, nodeRadius);
      }
    }
  }

  function onEnd() {
    dragging = false;
    // Keep pinned after drag so the node stays where user placed it
    g.querySelector("circle")?.setAttribute("stroke", "#fff");
    g.querySelector("circle")?.setAttribute("stroke-width", "2");
    window.removeEventListener("mousemove", onMove);
    window.removeEventListener("mouseup", onEnd);
  }

  g.addEventListener("mousedown", onStart);
}

// ─── Pan & Zoom ───
function setupPanZoom(svg: SVGSVGElement, rootG: SVGGElement) {
  let scale = 1;
  let translateX = 0,
    translateY = 0;
  let panning = false;
  let panStartX = 0,
    panStartY = 0;
  let dragStartX = 0,
    dragStartY = 0;

  function applyTransform() {
    rootG.setAttribute("transform", `translate(${translateX},${translateY}) scale(${scale})`);
    updateLabelsVisibility();
  }

  function updateLabelsVisibility() {
    // Hide labels when zoomed out too much or hide less important labels
    // We only want to show labels for important nodes (high link count) when zoomed out
    for (const { text, node } of graphNodeElsRef) {
      if (node.slug === graphSelectedSlug.value || node.slug === graphHighlightSlug.value) {
        text.style.opacity = "1"; // Always show selected/highlighted
        continue;
      }

      let visibilityThreshold = 0.5; // Default: need to zoom in to at least 0.5 to see all labels

      // Highly connected nodes get their labels shown earlier
      if (node.linkCount > 10) visibilityThreshold = 0.2;
      else if (node.linkCount > 5) visibilityThreshold = 0.35;
      else if (node.linkCount > 2) visibilityThreshold = 0.45;

      if (scale < visibilityThreshold) {
        text.style.opacity = "0";
      } else {
        text.style.opacity = "1";
      }
    }
  }

  // Export methods for programmatic pan/zoom
  let animId = 0;
  graphPanZoomRef = {
    setScale: (s: number) => {
      scale = s;
    },
    setTranslate: (x: number, y: number) => {
      translateX = x;
      translateY = y;
    },
    apply: applyTransform,
    getScale: () => scale,
    flyTo: (tx: number, ty: number, s?: number, duration = 400) => {
      cancelAnimationFrame(animId);
      const startX = translateX,
        startY = translateY,
        startScale = scale;
      const targetScale = s || scale;
      const startTime = performance.now();
      const animate = (time: number) => {
        let t = (time - startTime) / duration;
        if (t > 1) t = 1;
        const ease = 1 - Math.pow(1 - t, 3); // cubic ease out
        translateX = startX + (tx - startX) * ease;
        translateY = startY + (ty - startY) * ease;
        scale = startScale + (targetScale - startScale) * ease;
        applyTransform();
        if (t < 1) animId = requestAnimationFrame(animate);
      };
      animId = requestAnimationFrame(animate);
    },
  };

  // Zoom with mouse wheel
  svg.addEventListener(
    "wheel",
    (e) => {
      e.preventDefault();
      const zoomFactor = e.deltaY > 0 ? 0.92 : 1.08;
      const newScale = Math.max(0.2, Math.min(5, scale * zoomFactor));

      // Zoom towards cursor
      const rect = svg.getBoundingClientRect();
      const cx = e.clientX - rect.left;
      const cy = e.clientY - rect.top;
      translateX = cx - (cx - translateX) * (newScale / scale);
      translateY = cy - (cy - translateY) * (newScale / scale);
      scale = newScale;
      applyTransform();
    },
    { passive: false },
  );

  // Pan with mouse drag on background
  svg.addEventListener("mousedown", (e) => {
    if (e.button !== 0) return;
    // Only pan if clicking the SVG background, not a node
    if ((e.target as Element).tagName === "svg" || (e.target as Element).tagName === "SVG") {
      panning = true;
      panStartX = e.clientX - translateX;
      panStartY = e.clientY - translateY;
      dragStartX = e.clientX;
      dragStartY = e.clientY;
      svg.style.cursor = "grabbing";
    }
  });

  window.addEventListener("mousemove", (e) => {
    if (!panning) return;
    translateX = e.clientX - panStartX;
    translateY = e.clientY - panStartY;
    applyTransform();
  });

  window.addEventListener("mouseup", (e) => {
    if (panning) {
      panning = false;
      svg.style.cursor = "default";

      // If we barely moved, consider it a click to clear selection
      const dx = e.clientX - dragStartX;
      const dy = e.clientY - dragStartY;
      if (Math.abs(dx) < 5 && Math.abs(dy) < 5) {
        if ((e.target as Element).tagName === "svg" || (e.target as Element).tagName === "SVG") {
          graphSelectedSlug.value = null;
          graphDrawerVisible.value = false;
          clearHighlight(graphNodeElsRef, graphEdgeElsRef);
        }
      }
    }
  });
}

// ─── Hover Highlight ───
function applyHighlight(
  slug: string,
  adjacency: Map<string, Set<string>>,
  nodeEls: {
    g: SVGGElement;
    circle: SVGCircleElement;
    text: SVGTextElement;
    activeRing: SVGCircleElement;
    node: GNode;
  }[],
  edgeEls: { line: SVGLineElement; source: string; target: string; bidir: boolean }[],
  hoverSlug?: string,
) {
  const neighbors = adjacency.get(slug) || new Set();
  const hoverNeighbors = hoverSlug ? adjacency.get(hoverSlug) || new Set() : new Set();

  // Helper to get consistent radius
  const getRadius = (n: GNode) => Math.max(8, Math.min(24, 8 + Math.log(n.linkCount + 1) * 4));

  for (const { g, circle, activeRing, node } of nodeEls) {
    const r = getRadius(node);
    if (node.slug === slug) {
      circle.setAttribute("r", String(r + 3));
      circle.setAttribute("stroke-width", "3");
      g.style.opacity = "1";
    } else if (hoverSlug && node.slug === hoverSlug) {
      circle.setAttribute("r", String(r + 3));
      circle.setAttribute("stroke-width", "3");
      g.style.opacity = "1";
    } else if (neighbors.has(node.slug) || (hoverSlug && hoverNeighbors.has(node.slug))) {
      circle.setAttribute("r", String(r));
      circle.setAttribute("stroke-width", "2");
      g.style.opacity = "1";
    } else {
      circle.setAttribute("r", String(r));
      circle.setAttribute("stroke-width", "2");
      g.style.opacity = "0.2";
    }

    if (node.slug === graphSelectedSlug.value) {
      activeRing.style.opacity = "1";
    } else {
      activeRing.style.opacity = "0";
    }
  }
  for (const e of edgeEls) {
    if (e.source === slug || e.target === slug || (hoverSlug && (e.source === hoverSlug || e.target === hoverSlug))) {
      e.line.setAttribute("stroke-opacity", "0.9");
      e.line.setAttribute("stroke-width", "2");

      // Determine which node is driving the highlight color
      const focusSlug = hoverSlug && (e.source === hoverSlug || e.target === hoverSlug) ? hoverSlug : slug;
      const hlColor = nodeColorMap[nodeEls.find((n) => n.node.slug === focusSlug)?.node.type || ""] || "#0052d9";

      e.line.setAttribute("stroke", hlColor);
      e.line.setAttribute("marker-end", "url(#arrow-end-hl)");
      if (e.bidir) e.line.setAttribute("marker-start", "url(#arrow-start-hl)");
    } else {
      e.line.setAttribute("stroke-opacity", "0.08");
      e.line.setAttribute("stroke-width", "1");
      e.line.setAttribute("marker-end", "url(#arrow-end)");
      if (e.bidir) e.line.setAttribute("marker-start", "url(#arrow-start)");
      else e.line.removeAttribute("marker-start");
    }
  }
}

function clearHighlight(
  nodeEls: {
    g: SVGGElement;
    circle: SVGCircleElement;
    text: SVGTextElement;
    activeRing: SVGCircleElement;
    node: GNode;
  }[],
  edgeEls: { line: SVGLineElement; source: string; target: string; bidir: boolean }[],
) {
  if (graphSelectedSlug.value) {
    applyHighlight(graphSelectedSlug.value, graphAdjacencyRef, nodeEls, edgeEls);
    return;
  }

  const getRadius = (n: GNode) => Math.max(8, Math.min(24, 8 + Math.log(n.linkCount + 1) * 4));

  for (const { g, circle, activeRing, node } of nodeEls) {
    circle.setAttribute("r", String(getRadius(node)));
    circle.setAttribute("stroke-width", "2");
    g.style.opacity = "1";
    activeRing.style.opacity = "0";
  }
  for (const e of edgeEls) {
    e.line.setAttribute("stroke", "#c0c4cc");
    e.line.setAttribute("stroke-width", "1.2");
    e.line.setAttribute("stroke-opacity", "0.4");
    e.line.setAttribute("marker-end", "url(#arrow-end)");
    if (e.bidir) e.line.setAttribute("marker-start", "url(#arrow-start)");
    else e.line.removeAttribute("marker-start");
  }
}

// graphSearchOptions drives the search select dropdown. When the input is
// empty we fall back to the overview top-500 snapshot so users can still
// browse the most-connected pages without typing — matching the old
// client-filter UX. Once the user types we switch to a remote full-text
// search against the wiki API so the dropdown can reach pages that sit
// outside the canvas (up to the whole 4万-page KB).
const graphSearchOptions = ref<{ label: string; value: string }[]>([]);
const graphSearchLoading = ref(false);
let graphSearchDebounce: ReturnType<typeof setTimeout> | null = null;
let graphSearchSeq = 0;

// graphSearchDefaultOptions is the snapshot of "global top-500 by link_count"
// used as the empty-keyword default. We populate it lazily from the first
// overview fetch and keep it across ego-mode navigations so drilling into
// a neighborhood doesn't shrink the search surface back to the ego subgraph.
const graphSearchDefaultOptions = ref<{ label: string; value: string }[]>([]);

// Expose the empty-state list to the template too, so the initial popup
// open (before the user types) renders the snapshot immediately. Using a
// computed keeps graphSearchOptions.value representing "current keyword
// results" without having to remember which list is active.
const graphSearchEffectiveOptions = computed(() => {
  return graphSearchOptions.value.length > 0 ? graphSearchOptions.value : graphSearchDefaultOptions.value;
});

// The graph search box is a hand-rolled combobox. graphSearchKeyword is the
// text in the input, graphSearchOpen whether the suggestion list shows, and
// graphSearchActiveIndex the row the arrow keys (or the pointer) highlight;
// -1 means none, and Enter then falls through to handleGraphSearchEnter.
const graphSearchKeyword = ref("");
const graphSearchOpen = ref(false);
const graphSearchActiveIndex = ref(-1);

// handleGraphSearchSelect clears graphSearchValue once a jump is done; the
// input text follows, so the box is empty and ready for the next search.
watch(graphSearchValue, (value) => {
  if (!value) graphSearchKeyword.value = "";
});

function openGraphSearch() {
  graphSearchOpen.value = true;
}

function closeGraphSearch() {
  graphSearchOpen.value = false;
  graphSearchActiveIndex.value = -1;
}

function onGraphSearchInput(value: string | number) {
  graphSearchKeyword.value = String(value);
  graphSearchOpen.value = true;
  graphSearchActiveIndex.value = -1;
  handleGraphRemoteSearch(graphSearchKeyword.value);
}

function pickGraphSearchOption(option: { label: string; value: string }) {
  graphSearchValue.value = option.value;
  graphSearchKeyword.value = option.label;
  closeGraphSearch();
  handleGraphSearchSelect(option.value);
}

function onGraphSearchKeydown(event: KeyboardEvent) {
  const options = graphSearchEffectiveOptions.value;
  if (event.key === "ArrowDown" || event.key === "ArrowUp") {
    event.preventDefault();
    graphSearchOpen.value = true;
    if (options.length === 0) return;
    const step = event.key === "ArrowDown" ? 1 : -1;
    graphSearchActiveIndex.value = (graphSearchActiveIndex.value + step + options.length) % options.length;
    return;
  }
  if (event.key === "Escape") {
    closeGraphSearch();
    return;
  }
  if (event.key === "Enter") {
    event.preventDefault();
    const highlighted = options[graphSearchActiveIndex.value];
    if (graphSearchOpen.value && highlighted) {
      pickGraphSearchOption(highlighted);
      return;
    }
    closeGraphSearch();
    handleGraphSearchEnter({ inputValue: graphSearchKeyword.value });
  }
}

function setGraphSearchDefaultFromNodes(nodes: { slug: string; title: string }[] | undefined) {
  if (!nodes) return;
  graphSearchDefaultOptions.value = nodes.map((n) => ({ label: n.title, value: n.slug }));
}

async function handleGraphRemoteSearch(keyword: string) {
  const q = (keyword || "").trim();
  if (graphSearchDebounce) {
    clearTimeout(graphSearchDebounce);
    graphSearchDebounce = null;
  }
  if (!q) {
    // No keyword — clear keyword-specific results; the computed
    // graphSearchEffectiveOptions will fall back to the top-500 snapshot.
    graphSearchOptions.value = [];
    graphSearchLoading.value = false;
    return;
  }
  graphSearchLoading.value = true;
  // Snapshot a monotonic sequence number so stale responses (user kept
  // typing while an earlier request was still in flight) don't overwrite
  // newer results with older ones.
  const seq = ++graphSearchSeq;
  graphSearchDebounce = setTimeout(async () => {
    try {
      const res = await searchWikiPages(props.knowledgeBaseId, q, 20);
      if (seq !== graphSearchSeq) return;
      const pages: WikiPage[] = res.pages ?? [];
      graphSearchOptions.value = pages.map((p) => ({ label: p.title, value: p.slug }));
    } catch (e) {
      if (seq !== graphSearchSeq) return;
      console.error("Wiki search failed:", e);
      graphSearchOptions.value = [];
    } finally {
      if (seq === graphSearchSeq) graphSearchLoading.value = false;
    }
  }, 200);
}

let graphNodeElsRef: {
  g: SVGGElement;
  circle: SVGCircleElement;
  text: SVGTextElement;
  activeRing: SVGCircleElement;
  node: GNode;
}[] = [];
let graphEdgeElsRef: { line: SVGLineElement; source: string; target: string; bidir: boolean }[] = [];
let graphAdjacencyRef = new Map<string, Set<string>>();

// handleGraphSearchSelect is the single entry point every "jump to this
// slug" path funnels through — the graph search select, drawer wiki-link
// clicks, the ?slug= query param, and the lint report's page links.
// On a 4万-page wiki, the current render contains at most GRAPH_OVERVIEW_LIMIT
// (500) nodes, so most of the wiki is NOT on screen at any given moment.
// If the requested slug is missing from the current canvas we reload the
// graph as an ego view centered on that slug, then finish the highlight
// and drawer flow once the new render is ready. This guarantees any
// navigable link can actually reach its destination regardless of where
// the target sits in the link_count ranking.
async function handleGraphSearchSelect(value: string) {
  if (!value) return;

  let node = graphNodes.find((n) => n.slug === value);
  if (!node) {
    // Target is outside the current subgraph — pivot to an ego view.
    // loadEgoGraph repopulates graphNodes as a side effect.
    await loadEgoGraph(value);
    node = graphNodes.find((n) => n.slug === value);
    if (!node) {
      // The slug truly does not exist in the KB (e.g. stale URL, deleted
      // page). loadEgoGraph will have surfaced the backend error in the
      // console; still open the drawer so the user sees the not-found
      // page body rather than a silent no-op.
      openGraphDrawer(value);
      setTimeout(() => {
        graphSearchValue.value = "";
      }, 300);
      return;
    }
  }

  // Under server-side filtering, every node currently in graphNodes has
  // already passed the active type filter — there is no longer a path
  // where we need to re-enable a filter to make the target visible.

  if (graphPanZoomRef) {
    const container = graphRef.value;
    if (container) {
      const width = container.clientWidth;
      const height = container.clientHeight;
      // Center node while maintaining current scale, shifted left by 240px to account for the 480px drawer
      const currentScale = graphPanZoomRef.getScale();
      graphPanZoomRef.flyTo(width / 2 - node.x * currentScale - 240, height / 2 - node.y * currentScale);
    }
  }

  // Trigger highlight
  graphSelectedSlug.value = value;
  graphHighlightSlug.value = value;
  if (graphNodeElsRef.length > 0) {
    applyHighlight(value, graphAdjacencyRef, graphNodeElsRef, graphEdgeElsRef);
  }

  // Open drawer automatically when searching
  openGraphDrawer(value);

  // Clear search input after selection to be ready for next search
  setTimeout(() => {
    graphSearchValue.value = "";
  }, 300);
}

async function handleGraphSearchEnter(context: { inputValue: string }) {
  const value = context.inputValue?.trim();
  if (!value) return;

  // First try the already-loaded remote suggestions — if the user picked a
  // keyword whose results are on screen, fire the first match immediately.
  const match = graphSearchOptions.value.find(
    (opt) =>
      opt.label.toLowerCase().includes(value.toLowerCase()) || opt.value.toLowerCase().includes(value.toLowerCase()),
  );
  if (match) {
    handleGraphSearchSelect(match.value);
    return;
  }

  // Fallback: user hit Enter before suggestions came back (fast typing /
  // network still pending). Run a one-shot search so Enter still navigates
  // somewhere useful rather than silently doing nothing.
  try {
    const res = await searchWikiPages(props.knowledgeBaseId, value, 1);
    const pages: WikiPage[] = res.pages ?? [];
    if (pages.length > 0) {
      handleGraphSearchSelect(pages[0].slug);
    }
  } catch (e) {
    console.error("Wiki search failed on enter:", e);
  }
}

// Load graph when switching to graph view
// Reload all pages when search query is cleared (backspace or clear button).
// `searchResults = null` snaps back to the bucketed view without refetching
// anything — the buckets still hold whatever the user scrolled in before
// they started searching.
// The sidebar search box's clear button: empty the query and drop back to
// the bucketed view at once rather than after the watcher's debounce.
function clearSidebarSearch() {
  searchQuery.value = "";
  searchResults.value = null;
}

let searchTimer: ReturnType<typeof setTimeout> | null = null;
watch(searchQuery, (val) => {
  if (searchTimer) clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    if (!val || !val.trim()) {
      searchResults.value = null;
    } else {
      doSearch();
    }
  }, 300);
});

watch(
  () => props.view,
  (v) => {
    if (v === "graph") {
      loadGraph();
    } else if (v === "browser") {
      nextTick(async () => {
        if (readerBodyRef.value && renderedContent.value) {
          await hydrateProtectedFileImages(readerBodyRef.value, kbFileAccess.value);
        }
      });
    }
  },
);

watch(
  () => route.query.slug,
  (newSlug) => {
    if (newSlug && typeof newSlug === "string") {
      if (!selectedPage.value || selectedPage.value.slug !== newSlug) {
        if (props.view === "graph") {
          handleGraphSearchSelect(newSlug);
        } else {
          navigateToSlug(newSlug);
        }
      }
    }
  },
);

onMounted(() => {
  loadPages();
  loadStats();
  if (props.view === "graph") loadGraph();
});

onUnmounted(() => {
  if (statsTimer) {
    clearInterval(statsTimer);
  }
  if (graphHoverLeaveTimer) {
    clearTimeout(graphHoverLeaveTimer);
    graphHoverLeaveTimer = null;
  }
  if (graphAnimFrame) {
    cancelAnimationFrame(graphAnimFrame);
    graphAnimFrame = 0;
  }
  if (groupSentinelObserver) {
    groupSentinelObserver.disconnect();
    groupSentinelObserver = null;
  }
  if (loadMoreObserver) {
    loadMoreObserver.disconnect();
    loadMoreObserver = null;
  }
});
</script>

<style scoped>
/*
 * Everything in this component is styled with utility classes except what
 * follows. The reader body is Markdown rendered through v-html, and the fix
 * drawer embeds the chat view, so neither can carry classes of ours: both
 * are styled as descendants through :deep(). The colours are TDesign's
 * tokens, the same ones the utilities resolve to, so dark mode follows.
 *
 * Wiki reader styles are for the knowledge-base document surface only.
 * Chat answer Markdown styles are centralized in components/css/chat-markdown.css.
 */
.wiki-reader-body {
  line-height: 1.6;
  font-size: 14px;
  color: var(--td-text-color-primary);
}

.wiki-reader-body :deep(h1) {
  font-size: 24px;
  margin: 28px 0 16px;
  font-weight: 600;
  line-height: 1.4;
}

.wiki-reader-body :deep(h2) {
  font-size: 18px;
  margin: 24px 0 12px;
  font-weight: 600;
  line-height: 1.4;
}

.wiki-reader-body :deep(h3) {
  font-size: 16px;
  margin: 20px 0 10px;
  font-weight: 600;
  line-height: 1.5;
}

.wiki-reader-body :deep(h4),
.wiki-reader-body :deep(h5),
.wiki-reader-body :deep(h6) {
  font-size: 14px;
  margin: 16px 0 8px;
  font-weight: 600;
  line-height: 1.5;
}

.wiki-reader-body :deep(p) {
  margin: 0 0 14px;
}

.wiki-reader-body :deep(ul),
.wiki-reader-body :deep(ol) {
  margin: 0 0 14px;
  padding-left: 24px;
}

.wiki-reader-body :deep(li) {
  margin-bottom: 6px;
  line-height: 1.6;
}

.wiki-reader-body :deep(li > p) {
  margin-bottom: 6px;
}

.wiki-reader-body :deep(blockquote) {
  margin: 0 0 14px;
  padding: 10px 16px;
  background: var(--td-bg-color-secondarycontainer);
  border-left: 4px solid var(--td-component-border);
  border-radius: 0 4px 4px 0;
  color: var(--td-text-color-secondary);
}

.wiki-reader-body :deep(code) {
  font-family: var(--app-font-family-mono);
  font-size: 13px;
  padding: 2px 4px;
  background: var(--td-bg-color-secondarycontainer);
  border-radius: 4px;
  color: var(--td-brand-color);
}

.wiki-reader-body :deep(pre) {
  margin: 0 0 14px;
  padding: 12px 16px;
  background: var(--td-bg-color-secondarycontainer);
  border-radius: 6px;
  overflow-x: auto;
}

.wiki-reader-body :deep(pre code) {
  padding: 0;
  background: transparent;
  color: inherit;
}

.wiki-reader-body :deep(p:has(img)) {
  text-align: center;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  margin-top: 16px;
  margin-bottom: 24px;
}

.wiki-reader-body :deep(p:has(img) img) {
  max-width: 100%;
  max-height: 400px;
  object-fit: contain;
  border-radius: 6px;
  display: block;
  margin: 0 auto 8px;
  cursor: zoom-in;
  transition: opacity 0.2s;
}

.wiki-reader-body :deep(p:has(img) img:hover) {
  opacity: 0.9;
}

.wiki-reader-body :deep(a.wiki-content-link) {
  color: var(--td-brand-color);
  text-decoration: none;
  border-bottom: 1px dashed var(--td-brand-color);
  cursor: pointer;
  font-weight: 500;
}

.wiki-reader-body :deep(a.wiki-content-link:hover) {
  border-bottom-style: solid;
  text-decoration: none !important;
}

/*
 * Markdown tables (GFM). `width: fit-content` lets a table shrink to its
 * content instead of always stretching to fill the reader column, while
 * `max-width: 100%` and horizontal scrolling still handle wide tables.
 */
.wiki-reader-body :deep(table) {
  display: block;
  width: fit-content;
  max-width: 100%;
  overflow-x: auto;
  margin: 0 0 16px;
  border-collapse: collapse;
  font-size: 13px;
  line-height: 1.55;
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-stroke);
  border-radius: 6px;
  -webkit-overflow-scrolling: touch;
}

.wiki-reader-body :deep(table thead) {
  background: var(--td-bg-color-secondarycontainer);
}

.wiki-reader-body :deep(table th),
.wiki-reader-body :deep(table td) {
  padding: 8px 12px;
  border-bottom: 1px solid var(--td-component-stroke);
  border-right: 1px solid var(--td-component-stroke);
  text-align: left;
  vertical-align: top;
  word-break: break-word;
}

.wiki-reader-body :deep(table th) {
  font-weight: 600;
  color: var(--td-text-color-primary);
  white-space: nowrap;
}

.wiki-reader-body :deep(table th:last-child),
.wiki-reader-body :deep(table td:last-child) {
  border-right: none;
}

.wiki-reader-body :deep(table tbody tr:last-child td) {
  border-bottom: none;
}

.wiki-reader-body :deep(table tbody tr:hover) {
  background: var(--td-bg-color-secondarycontainer);
}

.wiki-reader-body :deep(table code) {
  font-size: 12px;
}

/*
 * The embedded chat view in the fix drawer is built for a full page; these
 * overrides fit it to the drawer (no max-width, no outer padding, full
 * height). They need !important because the chat view's own rules are
 * written with it.
 */
.wiki-fix-chat :deep(.chat) {
  max-width: 100% !important;
  min-width: 100% !important;
  padding: 0 !important;
  height: 100% !important;
  flex: 1 !important;
  border-radius: 0 !important;
}

.wiki-fix-chat :deep(.chat_scroll_box) {
  padding: 0 !important;
}

.wiki-fix-chat :deep(.chat > .input-container) {
  padding: 16px 0 0 0 !important;
  box-sizing: border-box;
  width: 100% !important;
  max-width: 100% !important;
  margin: 0 !important;
  overflow-x: hidden;
}

.wiki-fix-chat :deep(.msg_list) {
  max-width: 100% !important;
  padding-bottom: 0 !important;
  margin: 0 !important;
}
</style>
