<template>
  <div class="relative m-0 box-border flex h-full min-h-0 flex-1">
    <ListSpaceSidebar
      v-model="spaceSelection"
      :count-all="allKnowledgeBases"
      :count-mine="kbs.length"
      :count-by-org="effectiveSharedCountByOrg"
      :count-favorites="kbFavoritesCount"
      :count-recents="kbRecentsCount"
    />
    <div class="flex min-w-0 flex-1 flex-col pt-5 pr-0 pb-0 pl-7">
      <div class="mb-4 flex items-center justify-between pr-7">
        <div class="flex flex-col gap-1">
          <div class="flex items-center gap-2">
            <h2 class="text-foreground m-0 [font-family:var(--app-font-family)] text-2xl leading-8 font-semibold">
              {{ $t("knowledgeBase.title") }}
            </h2>
            <Tooltip v-if="authStore.hasRole('contributor')">
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="sm"
                  class="bg-muted text-muted-foreground hover:text-foreground border-border [&_svg]:text-primary inline-flex size-7 cursor-pointer items-center justify-center rounded-md border shadow-[inset_0_1px_0_color-mix(in_srgb,var(--td-bg-color-container)_72%,transparent)] transition-colors"
                  data-guide="kb-list-create"
                  :aria-label="$t('knowledgeList.create')"
                  @click="handleCreateKnowledgeBase"
                >
                  <FolderPlusIcon class="size-[16px]"
                /></Button>
              </TooltipTrigger>
              <TooltipContent side="bottom">{{ $t("knowledgeList.create") }}</TooltipContent>
            </Tooltip>
          </div>
          <p class="text-placeholder m-0 [font-family:var(--app-font-family)] text-sm leading-5 font-normal">
            {{ $t("knowledgeList.subtitle") }}
          </p>
        </div>
      </div>
      <div class="min-w-0 flex-1 overflow-x-hidden overflow-y-auto pt-0 pr-7 pb-2 pl-0">
        <!-- creator filter intentionally removed from chrome: every card
             already shows its creator via ResourceOriginBadge / avatar, so
             a dedicated horizontal switch added more noise than signal.
             The backend `?creator=mine|others` param and the URL-state
             field are kept so a future "filter by member" entry point
             (e.g. clicking an avatar) can deep-link without re-plumbing. -->

        <!-- 未初始化知识库提示 -->
        <div
          v-if="hasUninitializedKbs"
          class="text-warning mb-5 flex items-center gap-2 rounded-md border border-[var(--td-warning-color-focus)] px-4 py-3 [font-family:var(--app-font-family)] text-sm [background:var(--td-warning-color-light)] [&_svg]:shrink-0"
        >
          <InfoIcon class="size-[16px]" />
          <span>{{ $t("knowledgeList.uninitializedBanner") }}</span>
        </div>

        <!-- 上传进度提示 -->
        <div v-if="uploadSummaries.length" class="mb-5 flex flex-col gap-3">
          <div
            v-for="summary in uploadSummaries"
            :key="summary.kbId"
            class="border-border bg-card flex items-center gap-3 rounded-lg border px-4 py-3"
          >
            <div class="text-primary flex items-center justify-center">
              <component
                :is="kbListIcon(summary.completed === summary.total ? 'check-circle-filled' : 'upload')"
                class="size-[20px]"
              />
            </div>
            <div class="flex-1">
              <div
                class="text-foreground mb-0.5 [font-family:var(--app-font-family)] text-sm leading-[22px] font-semibold"
              >
                {{
                  summary.completed === summary.total
                    ? $t("knowledgeList.uploadProgress.completedTitle", { name: summary.kbName })
                    : $t("knowledgeList.uploadProgress.uploadingTitle", { name: summary.kbName })
                }}
              </div>
              <div class="text-muted-foreground [font-family:var(--app-font-family)] text-xs leading-[18px]">
                {{
                  summary.completed === summary.total
                    ? $t("knowledgeList.uploadProgress.completedDetail", { total: summary.total })
                    : $t("knowledgeList.uploadProgress.detail", { completed: summary.completed, total: summary.total })
                }}
              </div>
              <div class="text-placeholder mt-0.5 [font-family:var(--app-font-family)] text-xs leading-[18px]">
                {{
                  summary.completed === summary.total
                    ? $t("knowledgeList.uploadProgress.refreshing")
                    : $t("knowledgeList.uploadProgress.keepPageOpen")
                }}
              </div>
              <div
                v-if="summary.hasError"
                class="text-destructive mt-1 [font-family:var(--app-font-family)] text-xs leading-[18px]"
              >
                {{ $t("knowledgeList.uploadProgress.errorTip") }}
              </div>
              <div class="bg-muted mt-2.5 h-1.5 w-full overflow-hidden rounded-full">
                <div
                  class="h-full transition-[width] duration-200 [background:linear-gradient(90deg,var(--td-brand-color-active)_0%,var(--td-brand-color)_100%)]"
                  :style="{ width: summary.progress + '%' }"
                ></div>
              </div>
            </div>
          </div>
        </div>

        <!-- 骨架屏占位 -->
        <div
          v-if="loading && kbs.length === 0"
          class="kb-card-grid-fade-in grid grid-cols-1 gap-3 min-[900px]:grid-cols-2 min-[1250px]:grid-cols-3 min-[1600px]:grid-cols-4 min-[1900px]:grid-cols-5 min-[2200px]:grid-cols-6"
        >
          <div
            v-for="n in 6"
            :key="'skel-' + n"
            class="border-border bg-card relative box-border flex h-[136px] min-h-[136px] cursor-default flex-col overflow-hidden rounded-[8px] border px-3.5 py-3 shadow-[0_1px_3px_rgba(0,0,0,0.04)] transition-all duration-[250ms] ease-in-out hover:border-[var(--td-brand-color)] hover:shadow-[0_4px_12px_rgba(7,192,95,0.12)]"
          >
            <div class="relative z-[1] mb-3 flex items-center justify-between gap-1">
              <Skeleton class="h-[20px] w-[60%]" />
            </div>
            <div class="relative z-[1] mb-1.5 flex min-h-0 flex-1 flex-col gap-1.5 overflow-hidden">
              <Skeleton class="h-3.5 w-full" />
              <Skeleton class="h-3.5 w-[80%]" />
            </div>
            <div class="border-border relative z-[1] mt-auto flex items-center gap-2 border-t-[0.5px] pt-1.5">
              <Skeleton class="size-7 rounded-none" />
              <Skeleton class="size-7 rounded-none" />
            </div>
          </div>
        </div>

        <!-- 卡片网格：全部 / 收藏 / 最近 — 共用同一份卡片模板，
             仅依赖 filteredKnowledgeBases 切片即可切换视图 -->
        <div
          v-if="
            (spaceSelection === 'all' || spaceSelection === 'favorites' || spaceSelection === 'recents') &&
            filteredKnowledgeBases.length > 0
          "
          class="kb-card-grid-fade-in grid grid-cols-1 gap-3 min-[900px]:grid-cols-2 min-[1250px]:grid-cols-3 min-[1600px]:grid-cols-4 min-[1900px]:grid-cols-5 min-[2200px]:grid-cols-6"
        >
          <!-- 置顶分组标题 -->
          <div
            v-if="filteredKnowledgeBases[0] && filteredKnowledgeBases[0].isMine && filteredKnowledgeBases[0].is_pinned"
            :class="KB_SECTION_HEADER_CLASS"
            role="button"
            tabindex="0"
            @click="toggleKbSection('pinned')"
            @keydown.enter.prevent="toggleKbSection('pinned')"
            @keydown.space.prevent="toggleKbSection('pinned')"
          >
            <PinIcon class="size-[14px] fill-current" />
            <span>{{ $t("knowledgeList.sections.pinned") }}</span>
            <span
              class="bg-muted text-muted-foreground ml-0.5 rounded-[8px] px-1.5 text-[11px] leading-4 font-medium"
              >{{ filteredKbSectionCounts.pinned }}</span
            >
            <component
              :is="kbListIcon(isKbSectionCollapsed('pinned') ? 'chevron-right' : 'chevron-down')"
              class="ml-1 size-[14px] opacity-70 transition-opacity duration-150 group-hover:opacity-100"
            />
          </div>
          <!-- 全部：我的知识库 + 共享给我的知识库。
               「已置顶」分组由顶部 header 接管。其余分段（我创建 / 本空间 ·
               仅查看 / 共享给我）各自打自己的标题；原本的「其他」过渡标题
               在 per-user 置顶模型下已无意义，删除以免和具体子段标题叠加。 -->
          <template v-for="(kb, index) in filteredKnowledgeBases" :key="kb.id">
            <!-- 我创建的：第一张「我创建」非置顶卡片前打标题，统一展示
                 不管上方是否存在「已置顶」段。与「本空间 · 仅查看」同样
                 仅在 contributor 视图下出现——admin/owner 视图原本就没有
                 任何分段标题，单独冒一个反而失衡。 -->
            <div
              v-if="
                showShareGroupHeaders &&
                kb.isMine &&
                isMyKb(kb as KB) &&
                !kb.is_pinned &&
                (index === 0 || (filteredKnowledgeBases[index - 1] as any).is_pinned)
              "
              :class="KB_SECTION_HEADER_CLASS"
              role="button"
              tabindex="0"
              @click="toggleKbSection('mine')"
              @keydown.enter.prevent="toggleKbSection('mine')"
              @keydown.space.prevent="toggleKbSection('mine')"
            >
              <UserIcon class="size-[14px]" />
              <span>{{ $t("knowledgeList.sections.mine") }}</span>
              <span
                class="bg-muted text-muted-foreground ml-0.5 rounded-[8px] px-1.5 text-[11px] leading-4 font-medium"
                >{{ filteredKbSectionCounts.mine }}</span
              >
              <component
                :is="kbListIcon(isKbSectionCollapsed('mine') ? 'chevron-right' : 'chevron-down')"
                class="ml-1 size-[14px] opacity-70 transition-opacity duration-150 group-hover:opacity-100"
              />
            </div>
            <!-- 本空间 · 仅查看：本空间里同事创建、对当前 contributor 不可编辑。
                 当前卡片必须是非置顶（否则归在「已置顶」），且前一张要么
                 不存在、要么是「共享给我」、要么是我创建、要么是置顶卡片
                 （置顶→非置顶的过渡同样要打这个标题）。 -->
            <div
              v-if="
                showShareGroupHeaders &&
                kb.isMine &&
                !isMyKb(kb as KB) &&
                !kb.is_pinned &&
                (index === 0 ||
                  !filteredKnowledgeBases[index - 1].isMine ||
                  isMyKb(filteredKnowledgeBases[index - 1] as KB) ||
                  (filteredKnowledgeBases[index - 1] as any).is_pinned)
              "
              :class="KB_SECTION_HEADER_CLASS"
              role="button"
              tabindex="0"
              @click="toggleKbSection('tenantOthers')"
              @keydown.enter.prevent="toggleKbSection('tenantOthers')"
              @keydown.space.prevent="toggleKbSection('tenantOthers')"
            >
              <component :is="kbListIcon(tenantSectionIconName)" class="size-[14px]" />
              <span>{{ $t(tenantSectionLabelKey) }}</span>
              <span
                class="bg-muted text-muted-foreground ml-0.5 rounded-[8px] px-1.5 text-[11px] leading-4 font-medium"
                >{{ filteredKbSectionCounts.tenantOthers }}</span
              >
              <component
                :is="kbListIcon(isKbSectionCollapsed('tenantOthers') ? 'chevron-right' : 'chevron-down')"
                class="ml-1 size-[14px] opacity-70 transition-opacity duration-150 group-hover:opacity-100"
              />
            </div>
            <!-- 共享给我 · 可编辑：从「我的（含同事）」首次过渡到共享 + 可编辑 -->
            <div
              v-if="
                showShareGroupHeaders &&
                !kb.isMine &&
                isSharedKbEditable((kb as any).permission) &&
                (index === 0 || filteredKnowledgeBases[index - 1].isMine)
              "
              :class="KB_SECTION_HEADER_CLASS"
              role="button"
              tabindex="0"
              @click="toggleKbSection('sharedEditable')"
              @keydown.enter.prevent="toggleKbSection('sharedEditable')"
              @keydown.space.prevent="toggleKbSection('sharedEditable')"
            >
              <UserPlusIcon class="size-[14px]" />
              <PenLineIcon class="-ml-1 size-[12px] opacity-75" />
              <span>{{ $t("knowledgeList.sections.sharedEditable") }}</span>
              <span
                class="bg-muted text-muted-foreground ml-0.5 rounded-[8px] px-1.5 text-[11px] leading-4 font-medium"
                >{{ filteredKbSectionCounts.sharedEditable }}</span
              >
              <component
                :is="kbListIcon(isKbSectionCollapsed('sharedEditable') ? 'chevron-right' : 'chevron-down')"
                class="ml-1 size-[14px] opacity-70 transition-opacity duration-150 group-hover:opacity-100"
              />
            </div>
            <!-- 共享给我 · 仅查看：从「可编辑共享 / 我的」过渡到 viewer 共享 -->
            <div
              v-if="
                showShareGroupHeaders &&
                !kb.isMine &&
                !isSharedKbEditable((kb as any).permission) &&
                (index === 0 ||
                  filteredKnowledgeBases[index - 1].isMine ||
                  isSharedKbEditable((filteredKnowledgeBases[index - 1] as any).permission))
              "
              :class="KB_SECTION_HEADER_CLASS"
              role="button"
              tabindex="0"
              @click="toggleKbSection('sharedReadonly')"
              @keydown.enter.prevent="toggleKbSection('sharedReadonly')"
              @keydown.space.prevent="toggleKbSection('sharedReadonly')"
            >
              <UserPlusIcon class="size-[14px]" />
              <EyeIcon class="-ml-1 size-[12px] opacity-75" />
              <span>{{ $t("knowledgeList.sections.sharedReadonly") }}</span>
              <span
                class="bg-muted text-muted-foreground ml-0.5 rounded-[8px] px-1.5 text-[11px] leading-4 font-medium"
                >{{ filteredKbSectionCounts.sharedReadonly }}</span
              >
              <component
                :is="kbListIcon(isKbSectionCollapsed('sharedReadonly') ? 'chevron-right' : 'chevron-down')"
                class="ml-1 size-[14px] opacity-70 transition-opacity duration-150 group-hover:opacity-100"
              />
            </div>
            <!-- 我的知识库卡片 -->
            <div
              v-if="kb.isMine"
              v-show="!isKbSectionCollapsed(kbSectionOf(kb))"
              :class="
                kbCardClass(kb.type, {
                  uninitialized: !isInitialized(kb),
                  highlighted: highlightedKbId !== null && highlightedKbId === kb.id,
                })
              "
              :ref="
                (el) => {
                  if (highlightedKbId !== null && highlightedKbId === kb.id && el)
                    highlightedCardRef = el as HTMLElement;
                }
              "
              @click="handleCardClick(kb)"
            >
              <!-- 收藏按钮：右上角浮动；通过 .card-header 的 padding-right
                   给「更多」按钮腾出空间，避免两个按钮叠在一起。 -->
              <button
                type="button"
                data-slot="kb-favorite-star"
                class="hover:bg-muted absolute top-0 right-0 z-[3] flex size-6 cursor-pointer items-center justify-center rounded-md transition-[opacity,background-color,color] duration-150 hover:text-[var(--td-warning-color,#e37318)]"
                :class="
                  isKbFavorited(kb.id)
                    ? 'text-[var(--td-warning-color,#e37318)] opacity-100'
                    : 'text-muted-foreground opacity-0 group-hover/kb:opacity-100'
                "
                @click.stop="toggleFavoriteKb(kb.id, $event)"
              >
                <component
                  :is="kbListIcon(isKbFavorited(kb.id) ? 'star-filled' : 'star')"
                  class="size-[14px]"
                  :class="kbListIconFill(isKbFavorited(kb.id) ? 'star-filled' : 'star')"
                />
              </button>
              <!-- 卡片头部 -->
              <div class="relative z-[1] mb-1.5 flex items-center justify-between gap-1">
                <span
                  class="text-foreground flex min-w-0 flex-1 items-center gap-1.5 truncate [font-family:var(--app-font-family)] text-[15px] leading-[22px] font-semibold tracking-[0.01em]"
                  :title="kb.name"
                >
                  <KbWikiBadge v-if="isWikiKb(kb)" />
                  <span class="min-w-0 truncate">{{ kb.name }}</span>
                </span>
                <!-- The card menu always exists when the card is visible: pin
                     is now per-user and available to anyone who can see the KB
                     (backend route only requires KB read access). Settings /
                     Delete are mutations, so they stay behind canManageKBCard. -->
                <Popover>
                  <PopoverTrigger as-child>
                    <!-- more-wrap / more-icon are hook classes: useMarqueeSelect ignores
                         drags that start on .more-wrap, and theme.css inverts
                         .more-icon in dark mode. -->
                    <div
                      class="more-wrap hover:bg-accent flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md opacity-0 transition-all duration-200 group-hover/kb:opacity-60 hover:opacity-100!"
                      @click.stop
                    >
                      <img class="more-icon size-4" src="@/assets/img/more.png" alt="" />
                    </div>
                  </PopoverTrigger>
                  <PopoverContent align="end" :class="CARD_MENU_CONTENT_CLASS">
                    <div class="flex flex-col gap-px" @click.stop>
                      <div :class="CARD_MENU_ITEM_CLASS" @click.stop="handleTogglePinById(kb.id)">
                        <component
                          :is="kbListIcon(kb.is_pinned ? 'pin-filled' : 'pin')"
                          :class="[CARD_MENU_ICON_CLASS, kbListIconFill(kb.is_pinned ? 'pin-filled' : 'pin')]"
                        />
                        <span>{{ kb.is_pinned ? $t("knowledgeList.pin.unpin") : $t("knowledgeList.pin.pin") }}</span>
                      </div>
                      <div
                        v-if="canDuplicateKBCard(kb)"
                        :class="CARD_MENU_ITEM_CLASS"
                        @click.stop="handleDuplicateById(kb.id)"
                      >
                        <CopyIcon :class="CARD_MENU_ICON_CLASS" />
                        <span>{{ $t("knowledgeList.menu.duplicate") }}</span>
                      </div>
                      <template v-if="canManageKBCard(kb)">
                        <div :class="CARD_MENU_ITEM_CLASS" @click.stop="handleSettingsById(kb.id)">
                          <SettingsIcon :class="CARD_MENU_ICON_CLASS" />
                          <span>{{ $t("knowledgeBase.settings") }}</span>
                        </div>
                        <div :class="CARD_MENU_DANGER_ITEM_CLASS" @click.stop="handleDeleteById(kb.id)">
                          <Trash2Icon class="size-4 shrink-0 transition-all duration-150" />
                          <span>{{ $t("common.delete") }}</span>
                        </div>
                      </template>
                    </div>
                  </PopoverContent>
                </Popover>
              </div>

              <!-- 卡片内容 -->
              <div class="relative z-[1] mb-1.5 flex min-h-0 flex-1 flex-col gap-1.5 overflow-hidden">
                <div
                  class="text-muted-foreground line-clamp-2 [font-family:var(--app-font-family)] text-xs leading-[17px] font-normal"
                >
                  {{ kb.description || $t("knowledgeBase.noDescription") }}
                </div>
              </div>

              <!-- 卡片底部 -->
              <div
                class="border-border relative z-[1] mt-auto flex items-center justify-between border-t-[0.5px] pt-1.5"
              >
                <div class="flex min-w-0 flex-1 items-center gap-2">
                  <div class="flex items-center gap-1">
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <div
                          class="flex h-[22px] w-auto cursor-default items-center justify-center gap-0.75 rounded-[5px] px-1.5 transition-colors duration-200"
                          :class="kbTypeBadgeClass(kb.type)"
                        >
                          <component
                            :is="kbListIcon(kb.type === 'faq' ? 'chat-bubble-help' : 'folder')"
                            class="size-[14px]"
                          />
                          <span class="text-[11px] font-medium">{{
                            kb.type === "faq" ? kb.chunk_count || 0 : kb.knowledge_count || 0
                          }}</span>
                          <Loader2Icon v-if="kb.isProcessing" class="size-[12px] animate-spin" />
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{
                        kb.type === "faq"
                          ? $t("knowledgeEditor.basic.typeFAQ")
                          : $t("knowledgeEditor.basic.typeDocument")
                      }}</TooltipContent>
                    </Tooltip>
                    <Tooltip v-if="kb.extract_config?.enabled">
                      <TooltipTrigger as-child>
                        <div
                          class="text-primary flex h-[22px] w-[22px] cursor-default items-center justify-center rounded-[5px] bg-[rgba(124,77,255,0.08)] transition-colors duration-200 hover:bg-[rgba(124,77,255,0.12)]"
                        >
                          <NetworkIcon class="size-[14px]" />
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("knowledgeList.features.knowledgeGraph") }}</TooltipContent>
                    </Tooltip>
                    <Tooltip v-if="kb.vlm_config?.enabled">
                      <TooltipTrigger as-child>
                        <div
                          class="text-warning flex h-[22px] w-[22px] cursor-default items-center justify-center rounded-[5px] bg-[rgba(255,152,0,0.08)] transition-colors duration-200 hover:bg-[rgba(255,152,0,0.12)]"
                        >
                          <ImageIcon class="size-[14px]" />
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("knowledgeList.features.multimodal") }}</TooltipContent>
                    </Tooltip>
                    <Tooltip v-if="kb.question_generation_config?.enabled">
                      <TooltipTrigger as-child>
                        <div
                          class="text-success flex h-[22px] w-[22px] cursor-default items-center justify-center rounded-[5px] bg-[rgba(0,150,136,0.08)] transition-colors duration-200 hover:bg-[rgba(0,150,136,0.12)]"
                        >
                          <CircleHelpIcon class="size-[14px]" />
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("knowledgeList.features.questionGeneration") }}</TooltipContent>
                    </Tooltip>
                    <Tooltip v-if="kb.share_count && kb.share_count > 0">
                      <TooltipTrigger as-child>
                        <div
                          class="text-primary flex h-[22px] w-[22px] cursor-default items-center justify-center rounded-[5px] bg-[rgba(0,82,217,0.08)] transition-colors duration-200 hover:bg-[rgba(0,82,217,0.12)]"
                        >
                          <Share2Icon class="size-[14px]" />
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{
                        $t("knowledgeList.sharedToOrgs", { count: kb.share_count })
                      }}</TooltipContent>
                    </Tooltip>
                  </div>
                </div>
                <div v-if="showKbOriginBadge(kb)" class="flex shrink-0 items-center gap-2">
                  <ResourceOriginBadge :variant="kbOriginVariant(kb)" :creator-name="kb.creator_name" />
                </div>
              </div>
            </div>

            <!-- 共享知识库卡片 -->
            <div
              v-else
              v-show="!isKbSectionCollapsed(kbSectionOf(kb))"
              :class="kbCardClass(kb.type)"
              @click="handleSharedKbClickFromAll(kb)"
            >
              <button
                type="button"
                data-slot="kb-favorite-star"
                class="hover:bg-muted absolute top-0 right-0 z-[3] flex size-6 cursor-pointer items-center justify-center rounded-md transition-[opacity,background-color,color] duration-150 hover:text-[var(--td-warning-color,#e37318)]"
                :class="
                  isKbFavorited(kb.id)
                    ? 'text-[var(--td-warning-color,#e37318)] opacity-100'
                    : 'text-muted-foreground opacity-0 group-hover/kb:opacity-100'
                "
                @click.stop="toggleFavoriteKb(kb.id, $event)"
              >
                <component
                  :is="kbListIcon(isKbFavorited(kb.id) ? 'star-filled' : 'star')"
                  class="size-[14px]"
                  :class="kbListIconFill(isKbFavorited(kb.id) ? 'star-filled' : 'star')"
                />
              </button>
              <!-- 卡片头部 -->
              <div class="relative z-[1] mb-1.5 flex items-center justify-between gap-1">
                <span
                  class="text-foreground flex min-w-0 flex-1 items-center gap-1.5 truncate [font-family:var(--app-font-family)] text-[15px] leading-[22px] font-semibold tracking-[0.01em]"
                  :title="kb.name"
                >
                  <KbWikiBadge v-if="isWikiKb(kb)" />
                  <span class="min-w-0 truncate">{{ kb.name }}</span>
                </span>
                <Tooltip>
                  <TooltipTrigger as-child>
                    <button
                      type="button"
                      data-slot="shared-detail-trigger"
                      class="text-primary inline-flex cursor-pointer items-center gap-1 rounded-md px-2 py-1 [font-family:var(--app-font-family)] text-[13px] transition-colors duration-200 hover:bg-[rgba(7,192,95,0.08)] [&_svg]:shrink-0"
                      @click.stop="openSharedDetailFromAll(kb)"
                      :aria-label="$t('knowledgeList.menu.viewDetails')"
                    >
                      <InfoIcon class="size-[16px]" />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent side="top">{{ $t("knowledgeList.menu.viewDetails") }}</TooltipContent>
                </Tooltip>
              </div>

              <!-- 卡片内容 -->
              <div class="relative z-[1] mb-1.5 flex min-h-0 flex-1 flex-col gap-1.5 overflow-hidden">
                <div
                  class="text-muted-foreground line-clamp-2 [font-family:var(--app-font-family)] text-xs leading-[17px] font-normal"
                >
                  {{ kb.description || $t("knowledgeBase.noDescription") }}
                </div>
              </div>

              <!-- 卡片底部 -->
              <div
                class="border-border relative z-[1] mt-auto flex items-center justify-between border-t-[0.5px] pt-1.5"
              >
                <div class="flex min-w-0 flex-1 items-center gap-2">
                  <div class="flex items-center gap-1">
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <div
                          class="flex h-[22px] w-auto cursor-default items-center justify-center gap-0.75 rounded-[5px] px-1.5 transition-colors duration-200"
                          :class="kbTypeBadgeClass(kb.type)"
                        >
                          <component
                            :is="kbListIcon(kb.type === 'faq' ? 'chat-bubble-help' : 'folder')"
                            class="size-[14px]"
                          />
                          <span class="text-[11px] font-medium">{{
                            kb.type === "faq" ? kb.chunk_count || "-" : kb.knowledge_count || "-"
                          }}</span>
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{
                        kb.type === "faq"
                          ? $t("knowledgeEditor.basic.typeFAQ")
                          : $t("knowledgeEditor.basic.typeDocument")
                      }}</TooltipContent>
                    </Tooltip>
                    <Tooltip v-if="kb.extract_config?.enabled">
                      <TooltipTrigger as-child>
                        <div
                          class="text-primary flex h-[22px] w-[22px] cursor-default items-center justify-center rounded-[5px] bg-[rgba(124,77,255,0.08)] transition-colors duration-200 hover:bg-[rgba(124,77,255,0.12)]"
                        >
                          <NetworkIcon class="size-[14px]" />
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("knowledgeList.features.knowledgeGraph") }}</TooltipContent>
                    </Tooltip>
                    <Tooltip
                      v-if="
                        kb.vlm_config?.enabled ||
                        (kb.storage_provider_config?.provider && kb.storage_provider_config.provider !== 'local')
                      "
                    >
                      <TooltipTrigger as-child>
                        <div
                          class="text-warning flex h-[22px] w-[22px] cursor-default items-center justify-center rounded-[5px] bg-[rgba(255,152,0,0.08)] transition-colors duration-200 hover:bg-[rgba(255,152,0,0.12)]"
                        >
                          <ImageIcon class="size-[14px]" />
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("knowledgeList.features.multimodal") }}</TooltipContent>
                    </Tooltip>
                    <Tooltip v-if="kb.question_generation_config?.enabled">
                      <TooltipTrigger as-child>
                        <div
                          class="text-success flex h-[22px] w-[22px] cursor-default items-center justify-center rounded-[5px] bg-[rgba(0,150,136,0.08)] transition-colors duration-200 hover:bg-[rgba(0,150,136,0.12)]"
                        >
                          <CircleHelpIcon class="size-[14px]" />
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("knowledgeList.features.questionGeneration") }}</TooltipContent>
                    </Tooltip>
                  </div>
                </div>
                <div class="flex shrink-0 items-center gap-2">
                  <Tooltip>
                    <TooltipTrigger as-child>
                      <div
                        class="text-muted-foreground inline-flex max-w-[140px] items-center gap-[5px] rounded-md bg-[rgba(7,192,95,0.06)] px-2 py-[3px] [font-family:var(--app-font-family)] text-xs leading-[1.4] transition-colors duration-150"
                      >
                        <img
                          src="@/assets/img/organization-green.svg"
                          class="size-3.5 shrink-0 align-middle"
                          alt=""
                          aria-hidden="true"
                        />
                        <span class="truncate font-medium">{{ kb.org_name }}</span>
                      </div>
                    </TooltipTrigger>
                    <TooltipContent side="top">{{ kb.org_name }}</TooltipContent>
                  </Tooltip>
                </div>
              </div>
            </div>
          </template>
        </div>

        <div
          v-if="spaceSelection === 'mine' && sortedMineKbs.length > 0"
          class="kb-card-grid-fade-in grid grid-cols-1 gap-3 min-[900px]:grid-cols-2 min-[1250px]:grid-cols-3 min-[1600px]:grid-cols-4 min-[1900px]:grid-cols-5 min-[2200px]:grid-cols-6"
        >
          <!-- 置顶分组标题 -->
          <div
            v-if="sortedMineKbs[0] && sortedMineKbs[0].is_pinned"
            :class="KB_SECTION_HEADER_CLASS"
            role="button"
            tabindex="0"
            @click="toggleKbSection('pinned')"
            @keydown.enter.prevent="toggleKbSection('pinned')"
            @keydown.space.prevent="toggleKbSection('pinned')"
          >
            <PinIcon class="size-[14px] fill-current" />
            <span>{{ $t("knowledgeList.sections.pinned") }}</span>
            <span
              class="bg-muted text-muted-foreground ml-0.5 rounded-[8px] px-1.5 text-[11px] leading-4 font-medium"
              >{{ mineKbSectionCounts.pinned }}</span
            >
            <component
              :is="kbListIcon(isKbSectionCollapsed('pinned') ? 'chevron-right' : 'chevron-down')"
              class="ml-1 size-[14px] opacity-70 transition-opacity duration-150 group-hover:opacity-100"
            />
          </div>
          <!-- 我的知识库。「已置顶」由顶部 header 接管；其余各分段各打各的
               标题——见「全部」tab 同处注释。 -->
          <template v-for="(kb, index) in sortedMineKbs" :key="kb.id">
            <!-- 我创建的：第一张非置顶的我创建卡片前打标题，无论上方是否
                 有「已置顶」段都要显示，和「本空间 · 仅查看」对齐——见
                 「全部」tab 同处注释。 -->
            <div
              v-if="
                showShareGroupHeaders &&
                isMyKb(kb) &&
                !kb.is_pinned &&
                (index === 0 || sortedMineKbs[index - 1].is_pinned)
              "
              :class="KB_SECTION_HEADER_CLASS"
              role="button"
              tabindex="0"
              @click="toggleKbSection('mine')"
              @keydown.enter.prevent="toggleKbSection('mine')"
              @keydown.space.prevent="toggleKbSection('mine')"
            >
              <UserIcon class="size-[14px]" />
              <span>{{ $t("knowledgeList.sections.mine") }}</span>
              <span
                class="bg-muted text-muted-foreground ml-0.5 rounded-[8px] px-1.5 text-[11px] leading-4 font-medium"
                >{{ mineKbSectionCounts.mine }}</span
              >
              <component
                :is="kbListIcon(isKbSectionCollapsed('mine') ? 'chevron-right' : 'chevron-down')"
                class="ml-1 size-[14px] opacity-70 transition-opacity duration-150 group-hover:opacity-100"
              />
            </div>
            <!-- 本空间 · 仅查看：当前非置顶的同事 KB，且前一张要么不存在、
                 要么是我创建、要么是置顶卡片（置顶→非置顶过渡）。 -->
            <div
              v-if="
                showShareGroupHeaders &&
                !isMyKb(kb) &&
                !kb.is_pinned &&
                (index === 0 || isMyKb(sortedMineKbs[index - 1]) || sortedMineKbs[index - 1].is_pinned)
              "
              :class="KB_SECTION_HEADER_CLASS"
              role="button"
              tabindex="0"
              @click="toggleKbSection('tenantOthers')"
              @keydown.enter.prevent="toggleKbSection('tenantOthers')"
              @keydown.space.prevent="toggleKbSection('tenantOthers')"
            >
              <component :is="kbListIcon(tenantSectionIconName)" class="size-[14px]" />
              <span>{{ $t(tenantSectionLabelKey) }}</span>
              <span
                class="bg-muted text-muted-foreground ml-0.5 rounded-[8px] px-1.5 text-[11px] leading-4 font-medium"
                >{{ mineKbSectionCounts.tenantOthers }}</span
              >
              <component
                :is="kbListIcon(isKbSectionCollapsed('tenantOthers') ? 'chevron-right' : 'chevron-down')"
                class="ml-1 size-[14px] opacity-70 transition-opacity duration-150 group-hover:opacity-100"
              />
            </div>
            <div
              v-show="!isKbSectionCollapsed(kbSectionOf(kb))"
              :class="
                kbCardClass(kb.type, {
                  uninitialized: !isInitialized(kb),
                  highlighted: highlightedKbId !== null && highlightedKbId === kb.id,
                })
              "
              :ref="
                (el) => {
                  if (highlightedKbId !== null && highlightedKbId === kb.id && el)
                    highlightedCardRef = el as HTMLElement;
                }
              "
              @click="handleCardClick(kb)"
            >
              <button
                type="button"
                data-slot="kb-favorite-star"
                class="hover:bg-muted absolute top-0 right-0 z-[3] flex size-6 cursor-pointer items-center justify-center rounded-md transition-[opacity,background-color,color] duration-150 hover:text-[var(--td-warning-color,#e37318)]"
                :class="
                  isKbFavorited(kb.id)
                    ? 'text-[var(--td-warning-color,#e37318)] opacity-100'
                    : 'text-muted-foreground opacity-0 group-hover/kb:opacity-100'
                "
                @click.stop="toggleFavoriteKb(kb.id, $event)"
              >
                <component
                  :is="kbListIcon(isKbFavorited(kb.id) ? 'star-filled' : 'star')"
                  class="size-[14px]"
                  :class="kbListIconFill(isKbFavorited(kb.id) ? 'star-filled' : 'star')"
                />
              </button>
              <!-- 卡片头部 -->
              <div class="relative z-[1] mb-1.5 flex items-center justify-between gap-1">
                <span
                  class="text-foreground flex min-w-0 flex-1 items-center gap-1.5 truncate [font-family:var(--app-font-family)] text-[15px] leading-[22px] font-semibold tracking-[0.01em]"
                  :title="kb.name"
                >
                  <KbWikiBadge v-if="isWikiKb(kb)" />
                  <span class="min-w-0 truncate">{{ kb.name }}</span>
                </span>
                <!-- See the matching block in the "all" tab template for why
                     this is no longer gated by canManageKBCard. -->
                <Popover
                  :open="!!kb.showMore"
                  @update:open="
                    (v) => {
                      kb.showMore = v;
                      onVisibleChange(v);
                    }
                  "
                >
                  <PopoverTrigger as-child>
                    <div
                      class="more-wrap hover:bg-accent flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md transition-all duration-200 hover:opacity-100!"
                      :class="
                        currentMoreIndex === index ? 'bg-accent opacity-100!' : 'opacity-0 group-hover/kb:opacity-60'
                      "
                      @click.stop="openMore(index)"
                    >
                      <img class="more-icon size-4" src="@/assets/img/more.png" alt="" />
                    </div>
                  </PopoverTrigger>
                  <PopoverContent align="end" :class="CARD_MENU_CONTENT_CLASS">
                    <div class="flex flex-col gap-px" @click.stop>
                      <div :class="CARD_MENU_ITEM_CLASS" @click.stop="handleTogglePin(kb)">
                        <component
                          :is="kbListIcon(kb.is_pinned ? 'pin-filled' : 'pin')"
                          :class="[CARD_MENU_ICON_CLASS, kbListIconFill(kb.is_pinned ? 'pin-filled' : 'pin')]"
                        />
                        <span>{{ kb.is_pinned ? $t("knowledgeList.pin.unpin") : $t("knowledgeList.pin.pin") }}</span>
                      </div>
                      <div
                        v-if="canDuplicateKBCard(kb)"
                        :class="CARD_MENU_ITEM_CLASS"
                        @click.stop="handleDuplicate(kb)"
                      >
                        <CopyIcon :class="CARD_MENU_ICON_CLASS" />
                        <span>{{ $t("knowledgeList.menu.duplicate") }}</span>
                      </div>
                      <template v-if="canManageKBCard(kb)">
                        <div :class="CARD_MENU_ITEM_CLASS" @click.stop="handleSettings(kb)">
                          <SettingsIcon :class="CARD_MENU_ICON_CLASS" />
                          <span>{{ $t("knowledgeBase.settings") }}</span>
                        </div>
                        <div :class="CARD_MENU_DANGER_ITEM_CLASS" @click.stop="handleDelete(kb)">
                          <Trash2Icon class="size-4 shrink-0 transition-all duration-150" />
                          <span>{{ $t("common.delete") }}</span>
                        </div>
                      </template>
                    </div>
                  </PopoverContent>
                </Popover>
              </div>

              <!-- 卡片内容 -->
              <div class="relative z-[1] mb-1.5 flex min-h-0 flex-1 flex-col gap-1.5 overflow-hidden">
                <div
                  class="text-muted-foreground line-clamp-2 [font-family:var(--app-font-family)] text-xs leading-[17px] font-normal"
                >
                  {{ kb.description || $t("knowledgeBase.noDescription") }}
                </div>
              </div>

              <!-- 卡片底部 -->
              <div
                class="border-border relative z-[1] mt-auto flex items-center justify-between border-t-[0.5px] pt-1.5"
              >
                <div class="flex min-w-0 flex-1 items-center gap-2">
                  <div class="flex items-center gap-1">
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <div
                          class="flex h-[22px] w-auto cursor-default items-center justify-center gap-0.75 rounded-[5px] px-1.5 transition-colors duration-200"
                          :class="kbTypeBadgeClass(kb.type)"
                        >
                          <component
                            :is="kbListIcon(kb.type === 'faq' ? 'chat-bubble-help' : 'folder')"
                            class="size-[14px]"
                          />
                          <span class="text-[11px] font-medium">{{
                            kb.type === "faq" ? kb.chunk_count || 0 : kb.knowledge_count || 0
                          }}</span>
                          <Loader2Icon v-if="kb.isProcessing" class="size-[12px] animate-spin" />
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{
                        kb.type === "faq"
                          ? $t("knowledgeEditor.basic.typeFAQ")
                          : $t("knowledgeEditor.basic.typeDocument")
                      }}</TooltipContent>
                    </Tooltip>
                    <Tooltip v-if="kb.extract_config?.enabled">
                      <TooltipTrigger as-child>
                        <div
                          class="text-primary flex h-[22px] w-[22px] cursor-default items-center justify-center rounded-[5px] bg-[rgba(124,77,255,0.08)] transition-colors duration-200 hover:bg-[rgba(124,77,255,0.12)]"
                        >
                          <NetworkIcon class="size-[14px]" />
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("knowledgeList.features.knowledgeGraph") }}</TooltipContent>
                    </Tooltip>
                    <Tooltip
                      v-if="
                        kb.vlm_config?.enabled ||
                        (kb.storage_provider_config?.provider && kb.storage_provider_config.provider !== 'local')
                      "
                    >
                      <TooltipTrigger as-child>
                        <div
                          class="text-warning flex h-[22px] w-[22px] cursor-default items-center justify-center rounded-[5px] bg-[rgba(255,152,0,0.08)] transition-colors duration-200 hover:bg-[rgba(255,152,0,0.12)]"
                        >
                          <ImageIcon class="size-[14px]" />
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("knowledgeList.features.multimodal") }}</TooltipContent>
                    </Tooltip>
                    <Tooltip v-if="kb.question_generation_config?.enabled">
                      <TooltipTrigger as-child>
                        <div
                          class="text-success flex h-[22px] w-[22px] cursor-default items-center justify-center rounded-[5px] bg-[rgba(0,150,136,0.08)] transition-colors duration-200 hover:bg-[rgba(0,150,136,0.12)]"
                        >
                          <CircleHelpIcon class="size-[14px]" />
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("knowledgeList.features.questionGeneration") }}</TooltipContent>
                    </Tooltip>
                    <!-- 共享状态图标 -->
                    <Tooltip v-if="(kb.share_count ?? 0) > 0">
                      <TooltipTrigger as-child>
                        <div
                          class="text-primary flex h-[22px] w-[22px] cursor-default items-center justify-center rounded-[5px] bg-[rgba(0,82,217,0.08)] transition-colors duration-200 hover:bg-[rgba(0,82,217,0.12)]"
                        >
                          <Share2Icon class="size-[14px]" />
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{
                        $t("knowledgeList.sharedToOrgs", { count: kb.share_count ?? 0 })
                      }}</TooltipContent>
                    </Tooltip>
                  </div>
                </div>
                <div v-if="showKbOriginBadge(kb)" class="flex shrink-0 items-center gap-2">
                  <ResourceOriginBadge :variant="kbOriginVariant(kb)" :creator-name="kb.creator_name" />
                </div>
              </div>
            </div>
          </template>
        </div>

        <!-- 协作 / 共享给我 聚合视图已移除：共享 KB 走「全部」或具体空间下展示 -->

        <!-- 按空间筛选：该空间内全部知识库（含我共享的） -->
        <div
          v-if="spaceSelectionOrgId && spaceKbsLoading"
          class="bg-card flex min-h-[200px] items-center justify-center p-3"
        >
          <Loader2Icon class="text-primary size-5 animate-spin" />
        </div>
        <div
          v-else-if="spaceSelectionOrgId && sortedSpaceKbsList.length > 0"
          class="kb-card-grid-fade-in grid grid-cols-1 gap-3 min-[900px]:grid-cols-2 min-[1250px]:grid-cols-3 min-[1600px]:grid-cols-4 min-[1900px]:grid-cols-5 min-[2200px]:grid-cols-6"
        >
          <template
            v-for="(shared, index) in sortedSpaceKbsList"
            :key="
              'shared-' +
              (shared.share_id || `agent-${shared.knowledge_base?.id}-${shared.source_from_agent?.agent_id || ''}`)
            "
          >
            <!-- 我共享的：本空间下我自己创建并共享进来的条目，只在第一条 is_mine 上挂标题 -->
            <div
              v-if="showShareGroupHeaders && shared.is_mine && index === 0"
              :class="KB_SECTION_HEADER_CLASS"
              role="button"
              tabindex="0"
              @click="toggleKbSection('sharedByMe')"
              @keydown.enter.prevent="toggleKbSection('sharedByMe')"
              @keydown.space.prevent="toggleKbSection('sharedByMe')"
            >
              <Share2Icon class="size-[14px]" />
              <span>{{ $t("knowledgeList.sections.sharedByMe") }}</span>
              <span
                class="bg-muted text-muted-foreground ml-0.5 rounded-[8px] px-1.5 text-[11px] leading-4 font-medium"
                >{{ spaceKbSectionCounts.sharedByMe }}</span
              >
              <component
                :is="kbListIcon(isKbSectionCollapsed('sharedByMe') ? 'chevron-right' : 'chevron-down')"
                class="ml-1 size-[14px] opacity-70 transition-opacity duration-150 group-hover:opacity-100"
              />
            </div>
            <!-- 共享给我 · 可编辑：从「我的」首次进入「共享 + 可编辑」 -->
            <div
              v-if="
                showShareGroupHeaders &&
                !shared.is_mine &&
                isSharedKbEditable(shared.permission) &&
                (index === 0 || sortedSpaceKbsList[index - 1].is_mine)
              "
              :class="KB_SECTION_HEADER_CLASS"
              role="button"
              tabindex="0"
              @click="toggleKbSection('sharedEditable')"
              @keydown.enter.prevent="toggleKbSection('sharedEditable')"
              @keydown.space.prevent="toggleKbSection('sharedEditable')"
            >
              <UserPlusIcon class="size-[14px]" />
              <PenLineIcon class="-ml-1 size-[12px] opacity-75" />
              <span>{{ $t("knowledgeList.sections.sharedEditable") }}</span>
              <span
                class="bg-muted text-muted-foreground ml-0.5 rounded-[8px] px-1.5 text-[11px] leading-4 font-medium"
                >{{ spaceKbSectionCounts.sharedEditable }}</span
              >
              <component
                :is="kbListIcon(isKbSectionCollapsed('sharedEditable') ? 'chevron-right' : 'chevron-down')"
                class="ml-1 size-[14px] opacity-70 transition-opacity duration-150 group-hover:opacity-100"
              />
            </div>
            <!-- 共享给我 · 仅查看：从「可编辑共享 / 我的」首次进入「viewer」 -->
            <div
              v-if="
                showShareGroupHeaders &&
                !shared.is_mine &&
                !isSharedKbEditable(shared.permission) &&
                (index === 0 ||
                  sortedSpaceKbsList[index - 1].is_mine ||
                  isSharedKbEditable(sortedSpaceKbsList[index - 1].permission))
              "
              :class="KB_SECTION_HEADER_CLASS"
              role="button"
              tabindex="0"
              @click="toggleKbSection('sharedReadonly')"
              @keydown.enter.prevent="toggleKbSection('sharedReadonly')"
              @keydown.space.prevent="toggleKbSection('sharedReadonly')"
            >
              <UserPlusIcon class="size-[14px]" />
              <EyeIcon class="-ml-1 size-[12px] opacity-75" />
              <span>{{ $t("knowledgeList.sections.sharedReadonly") }}</span>
              <span
                class="bg-muted text-muted-foreground ml-0.5 rounded-[8px] px-1.5 text-[11px] leading-4 font-medium"
                >{{ spaceKbSectionCounts.sharedReadonly }}</span
              >
              <component
                :is="kbListIcon(isKbSectionCollapsed('sharedReadonly') ? 'chevron-right' : 'chevron-down')"
                class="ml-1 size-[14px] opacity-70 transition-opacity duration-150 group-hover:opacity-100"
              />
            </div>
            <div
              v-show="!isSpaceKbCollapsed(shared)"
              :class="kbCardClass(shared.knowledge_base.type)"
              @click="handleSharedKbClick(shared)"
            >
              <!-- 卡片头部 -->
              <div class="relative z-[1] mb-1.5 flex items-center justify-between gap-1">
                <span
                  class="text-foreground flex min-w-0 flex-1 items-center gap-1.5 truncate [font-family:var(--app-font-family)] text-[15px] leading-[22px] font-semibold tracking-[0.01em]"
                  :title="shared.knowledge_base.name"
                >
                  <KbWikiBadge v-if="isWikiKb(shared.knowledge_base)" />
                  <span class="min-w-0 truncate">{{ shared.knowledge_base.name }}</span>
                </span>
                <Tooltip v-if="!shared.is_mine">
                  <TooltipTrigger as-child>
                    <button
                      type="button"
                      data-slot="shared-detail-trigger"
                      class="text-primary inline-flex cursor-pointer items-center gap-1 rounded-md px-2 py-1 [font-family:var(--app-font-family)] text-[13px] transition-colors duration-200 hover:bg-[rgba(7,192,95,0.08)] [&_svg]:shrink-0"
                      @click.stop="openSharedDetail(shared)"
                      :aria-label="$t('knowledgeList.menu.viewDetails')"
                    >
                      <InfoIcon class="size-[16px]" />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent side="top">{{ $t("knowledgeList.menu.viewDetails") }}</TooltipContent>
                </Tooltip>
              </div>

              <!-- 卡片内容 -->
              <div class="relative z-[1] mb-1.5 flex min-h-0 flex-1 flex-col gap-1.5 overflow-hidden">
                <div
                  class="text-muted-foreground line-clamp-2 [font-family:var(--app-font-family)] text-xs leading-[17px] font-normal"
                >
                  {{ shared.knowledge_base.description || $t("knowledgeBase.noDescription") }}
                </div>
              </div>

              <!-- 卡片底部 -->
              <div
                class="border-border relative z-[1] mt-auto flex items-center justify-between border-t-[0.5px] pt-1.5"
              >
                <div class="flex min-w-0 flex-1 items-center gap-2">
                  <div class="flex items-center gap-1">
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <div
                          class="flex h-[22px] w-auto cursor-default items-center justify-center gap-0.75 rounded-[5px] px-1.5 transition-colors duration-200"
                          :class="kbTypeBadgeClass(shared.knowledge_base.type)"
                        >
                          <component
                            :is="kbListIcon(shared.knowledge_base.type === 'faq' ? 'chat-bubble-help' : 'folder')"
                            class="size-[14px]"
                          />
                          <span class="text-[11px] font-medium">{{
                            shared.knowledge_base.type === "faq"
                              ? (shared.knowledge_base.chunk_count ?? "-")
                              : (shared.knowledge_base.knowledge_count ?? "-")
                          }}</span>
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{
                        shared.knowledge_base.type === "faq"
                          ? $t("knowledgeEditor.basic.typeFAQ")
                          : $t("knowledgeEditor.basic.typeDocument")
                      }}</TooltipContent>
                    </Tooltip>
                  </div>
                </div>
              </div>
            </div>
          </template>
        </div>

        <!-- 全部空状态：保留「新建知识库」CTA，因为是空间没有任何 KB 的真空场景 -->
        <div
          v-if="spaceSelection === 'all' && filteredKnowledgeBases.length === 0 && !loading"
          class="flex flex-1 flex-col items-center justify-center px-5 py-15"
        >
          <img class="mb-5 size-[162px]" src="@/assets/img/upload.svg" alt="" />
          <span
            class="text-placeholder mb-2 [font-family:var(--app-font-family)] text-base leading-[26px] font-semibold"
            >{{ $t("knowledgeList.empty.title") }}</span
          >
          <span
            class="m-0 [font-family:var(--app-font-family)] text-sm leading-[22px] font-normal text-[var(--td-text-color-disabled)]"
            >{{ $t("knowledgeList.empty.description") }}</span
          >
          <Button
            v-if="authStore.hasRole('contributor')"
            class="text-primary-foreground mt-5 border-none [background:linear-gradient(135deg,var(--td-brand-color)_0%,#00a67e_100%)] hover:[background:linear-gradient(135deg,var(--td-brand-color)_0%,var(--td-brand-color-active)_100%)]"
            data-guide="kb-list-create"
            @click="handleCreateKnowledgeBase"
          >
            <FolderPlusIcon /> {{ $t("knowledgeList.create") }}</Button
          >
        </div>

        <!-- 收藏空状态：不放创建按钮——「没有收藏」 ≠ 「没有知识库」，
             正确引导是「去星标一下」，不是「再建一个」。 -->
        <div
          v-if="spaceSelection === 'favorites' && filteredKnowledgeBases.length === 0 && !loading"
          class="flex flex-1 flex-col items-center justify-center px-5 py-15"
        >
          <StarIcon class="size-[48px]" />
          <span
            class="text-placeholder mb-2 [font-family:var(--app-font-family)] text-base leading-[26px] font-semibold"
            >{{ $t("knowledgeList.empty.favoritesTitle") }}</span
          >
          <span
            class="m-0 [font-family:var(--app-font-family)] text-sm leading-[22px] font-normal text-[var(--td-text-color-disabled)]"
            >{{ $t("knowledgeList.empty.favoritesDescription") }}</span
          >
        </div>

        <!-- 最近空状态：同理，引导是「去打开一个」。 -->
        <div
          v-if="spaceSelection === 'recents' && filteredKnowledgeBases.length === 0 && !loading"
          class="flex flex-1 flex-col items-center justify-center px-5 py-15"
        >
          <HistoryIcon class="size-[48px]" />
          <span
            class="text-placeholder mb-2 [font-family:var(--app-font-family)] text-base leading-[26px] font-semibold"
            >{{ $t("knowledgeList.empty.recentsTitle") }}</span
          >
          <span
            class="m-0 [font-family:var(--app-font-family)] text-sm leading-[22px] font-normal text-[var(--td-text-color-disabled)]"
            >{{ $t("knowledgeList.empty.recentsDescription") }}</span
          >
        </div>

        <!-- 我的知识库空状态 -->
        <div
          v-if="spaceSelection === 'mine' && kbs.length === 0 && !loading"
          class="flex flex-1 flex-col items-center justify-center px-5 py-15"
        >
          <img class="mb-5 size-[162px]" src="@/assets/img/upload.svg" alt="" />
          <span
            class="text-placeholder mb-2 [font-family:var(--app-font-family)] text-base leading-[26px] font-semibold"
            >{{ $t("knowledgeList.empty.title") }}</span
          >
          <span
            class="m-0 [font-family:var(--app-font-family)] text-sm leading-[22px] font-normal text-[var(--td-text-color-disabled)]"
            >{{ $t("knowledgeList.empty.description") }}</span
          >
          <Button
            v-if="authStore.hasRole('contributor')"
            class="text-primary-foreground mt-5 border-none [background:linear-gradient(135deg,var(--td-brand-color)_0%,#00a67e_100%)] hover:[background:linear-gradient(135deg,var(--td-brand-color)_0%,var(--td-brand-color-active)_100%)]"
            data-guide="kb-list-create"
            @click="handleCreateKnowledgeBase"
          >
            <FolderPlusIcon /> {{ $t("knowledgeList.create") }}</Button
          >
        </div>

        <!-- 空间下知识库空状态 -->
        <div
          v-if="spaceSelectionOrgId && !spaceKbsLoading && spaceKbsList.length === 0"
          class="flex flex-1 flex-col items-center justify-center px-5 py-15"
        >
          <img class="mb-5 size-[162px]" src="@/assets/img/upload.svg" alt="" />
          <span
            class="text-placeholder mb-2 [font-family:var(--app-font-family)] text-base leading-[26px] font-semibold"
            >{{ $t("knowledgeList.empty.sharedTitle") }}</span
          >
          <span
            class="m-0 [font-family:var(--app-font-family)] text-sm leading-[22px] font-normal text-[var(--td-text-color-disabled)]"
            >{{ $t("knowledgeList.empty.sharedDescription") }}</span
          >
        </div>
      </div>
    </div>

    <!-- 删除确认对话框 -->
    <Dialog :open="deleteVisible" @update:open="(v: boolean) => (deleteVisible = v)">
      <DialogContent class="block gap-0 overflow-hidden rounded-md p-4 sm:max-w-[464px]" :show-close-button="false">
        <div class="mb-2 flex items-center">
          <img class="mr-2 size-5" src="@/assets/img/circle.png" alt="" />
          <DialogTitle
            class="text-foreground m-0 [font-family:var(--app-font-family)] text-base leading-6 font-semibold"
          >
            {{ $t("knowledgeList.delete.confirmTitle") }}
          </DialogTitle>
        </div>
        <DialogDescription
          class="text-placeholder mt-0 mb-[21px] ml-[29px] inline-block [font-family:var(--app-font-family)] text-sm leading-[22px] font-normal"
        >
          {{ $t("knowledgeList.delete.confirmMessage", { name: deletingKb?.name ?? "" }) }}
        </DialogDescription>
        <div class="flex h-[22px] w-full justify-end">
          <span
            class="text-foreground cursor-pointer [font-family:var(--app-font-family)] text-sm leading-[22px] font-normal hover:opacity-80"
            @click="deleteVisible = false"
            >{{ $t("common.cancel") }}</span
          >
          <span
            class="text-destructive ml-10 cursor-pointer [font-family:var(--app-font-family)] text-sm leading-[22px] font-normal hover:opacity-80"
            @click="confirmDelete"
            >{{ $t("knowledgeList.delete.confirmButton") }}</span
          >
        </div>
      </DialogContent>
    </Dialog>

    <!-- 知识库编辑器（创建/编辑统一组件） -->
    <KnowledgeBaseEditorModal
      :visible="uiStore.showKBEditorModal"
      :mode="uiStore.kbEditorMode"
      :kb-id="uiStore.currentKBId || undefined"
      :initial-type="uiStore.kbEditorType"
      @update:visible="(val) => (val ? null : uiStore.closeKBEditor())"
      @success="handleKBEditorSuccess"
    />

    <!-- 共享知识库对话框 -->
    <ShareKnowledgeBaseDialog
      v-model:visible="shareDialogVisible"
      :knowledge-base-id="sharingKbId"
      :knowledge-base-name="sharingKbName"
      @shared="handleShareSuccess"
    />

    <!-- 右侧：共享知识库详情面板 -->
    <Teleport to="body">
      <Transition name="shared-detail-drawer">
        <div
          v-if="sharedDetailPanelVisible && currentSharedKbForDetail"
          class="fixed inset-0 z-[1000] flex justify-end bg-black/40"
          @click.self="closeSharedDetailPanel"
        >
          <!-- shared-detail-slide is the hook the global transition block slides in. -->
          <div
            class="shared-detail-slide bg-card flex h-full w-[360px] max-w-[90vw] flex-col [font-family:var(--app-font-family)] shadow-[-4px_0_24px_rgba(0,0,0,0.12)]"
          >
            <div class="border-border flex shrink-0 items-center justify-between border-b px-6 py-5">
              <h3 class="text-foreground m-0 text-lg font-semibold">{{ $t("knowledgeList.detail.title") }}</h3>
              <button
                type="button"
                data-slot="drawer-close"
                class="bg-muted text-muted-foreground hover:text-foreground flex size-8 cursor-pointer items-center justify-center rounded-md border-none transition-colors duration-200"
                @click="closeSharedDetailPanel"
                :aria-label="$t('general.close')"
              >
                <XIcon class="size-[20px]" />
              </button>
            </div>
            <div class="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto p-6">
              <div class="flex flex-col gap-1.5">
                <span class="text-muted-foreground text-xs leading-[1.4]">{{ $t("knowledgeBase.name") }}</span>
                <span class="text-foreground text-sm leading-normal break-words">{{
                  currentSharedKbForDetail.knowledge_base.name
                }}</span>
              </div>
              <div class="flex flex-col gap-1.5">
                <span class="text-muted-foreground text-xs leading-[1.4]">{{
                  $t("knowledgeList.detail.sourceType")
                }}</span>
                <span class="text-foreground text-sm leading-normal font-medium break-words">
                  {{
                    currentSharedKbForDetail.source_from_agent
                      ? $t("knowledgeList.detail.sourceTypeAgent")
                      : $t("knowledgeList.detail.sourceTypeKbShare")
                  }}
                </span>
              </div>
              <div class="flex flex-col gap-1.5">
                <span class="text-muted-foreground text-xs leading-[1.4]">{{
                  currentSharedKbForDetail.source_from_agent
                    ? $t("knowledgeList.detail.sourceFromAgent")
                    : $t("knowledgeList.detail.sourceOrg")
                }}</span>
                <span class="text-foreground inline-flex items-center gap-1.5 text-sm leading-normal break-words">
                  <img src="@/assets/img/organization-green.svg" class="size-3.5 shrink-0" alt="" aria-hidden="true" />
                  {{
                    currentSharedKbForDetail.source_from_agent
                      ? currentSharedKbForDetail.source_from_agent.agent_name
                      : currentSharedKbForDetail.org_name
                  }}
                </span>
              </div>
              <div v-if="currentSharedKbForDetail.source_from_agent" class="flex flex-col gap-1.5">
                <span class="text-muted-foreground text-xs leading-[1.4]">{{
                  $t("knowledgeList.detail.agentKbStrategy")
                }}</span>
                <span class="text-foreground text-sm leading-normal break-words">
                  {{ agentKbStrategyText(currentSharedKbForDetail.source_from_agent?.kb_selection_mode ?? "") }}
                </span>
              </div>
              <div class="flex flex-col gap-1.5">
                <span class="text-muted-foreground text-xs leading-[1.4]">{{
                  $t("knowledgeList.detail.sharedAt")
                }}</span>
                <span class="text-foreground text-sm leading-normal break-words">{{
                  formatStringDate(new Date(currentSharedKbForDetail.shared_at))
                }}</span>
              </div>
              <div class="flex flex-col gap-1.5">
                <span class="text-muted-foreground text-xs leading-[1.4]">{{
                  $t("knowledgeList.detail.myPermission")
                }}</span>
                <!-- A small solid tag, as t-tag's default (dark) variant drew it:
                     brand for admin, warning for editor, neutral otherwise. -->
                <Badge
                  class="h-[22px] rounded-[3px] px-2 font-normal"
                  :class="
                    currentSharedKbForDetail.permission === 'admin'
                      ? 'bg-primary text-primary-foreground'
                      : currentSharedKbForDetail.permission === 'editor'
                        ? 'bg-warning text-primary-foreground'
                        : 'text-foreground bg-[var(--td-bg-color-component)]'
                  "
                >
                  {{ $t(`organization.role.${currentSharedKbForDetail.permission}`) }}
                </Badge>
              </div>
            </div>
            <div class="border-border bg-card flex shrink-0 justify-end gap-3 border-t px-6 py-4">
              <Button variant="outline" @click="closeSharedDetailPanel">{{ $t("common.close") }}</Button>
              <Button class="gap-1" @click="goToSharedKbFromPanel">
                <EyeIcon />
                {{ $t("knowledgeList.detail.goToKb") }}</Button
              >
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <ContextualGuide tour="kbList" :when="showKbListContextualGuide" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed, watch, nextTick } from "vue";
import { useRouter, useRoute } from "vue-router";
import { MessagePlugin } from "tdesign-vue-next";
import { deleteKnowledgeBase, duplicateKnowledgeBase, togglePinKnowledgeBase } from "@/api/knowledge-base";
import { useChatResourcesStore } from "@/stores/chatResources";
import { formatStringDate } from "@/utils/index";
import { useUIStore } from "@/stores/ui";
import { useAuthStore } from "@/stores/auth";
import { useOrganizationStore } from "@/stores/organization";
import {
  listOrganizationSharedKnowledgeBases,
  type SharedKnowledgeBase,
  type OrganizationSharedKnowledgeBaseItem,
  type SourceFromAgentInfo,
} from "@/api/organization";
import { mergeAllScopeKnowledgeBases, type OwnedKnowledgeBase, type SharedKnowledgeBaseLike } from "./kbListMerge";
import KnowledgeBaseEditorModal from "./KnowledgeBaseEditorModal.vue";
import KbWikiBadge from "./components/KbWikiBadge.vue";
import ShareKnowledgeBaseDialog from "@/components/ShareKnowledgeBaseDialog.vue";
import ListSpaceSidebar from "@/components/ListSpaceSidebar.vue";
import ResourceOriginBadge from "@/components/ResourceOriginBadge.vue";
import { shouldShowResourceOriginBadge } from "@/utils/card-list-badge";
import ContextualGuide from "@/components/ContextualGuide.vue";
import { isContextualGuideDone, markContextualGuideDone } from "@/config/contextualGuides";
import { useTenantModelReadiness } from "@/composables/useTenantModelReadiness";
import { useI18n } from "vue-i18n";
import { useListUrlState } from "@/composables/useListUrlState";
import { useResourcePins } from "@/composables/useResourcePins";
import {
  ChevronDownIcon,
  ChevronRightIcon,
  CircleCheckIcon,
  CircleHelpIcon,
  CopyIcon,
  EyeIcon,
  FolderIcon,
  FolderPlusIcon,
  HistoryIcon,
  ImageIcon,
  InfoIcon,
  Loader2Icon,
  NetworkIcon,
  PenLineIcon,
  PinIcon,
  SettingsIcon,
  Share2Icon,
  StarIcon,
  Trash2Icon,
  UploadIcon,
  UserIcon,
  UserPlusIcon,
  UsersIcon,
  XIcon,
  type LucideIcon,
} from "@lucide/vue";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Skeleton } from "@/components/ui/skeleton";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

const kbListIcons: Record<string, LucideIcon> = {
  "star-filled": StarIcon,
  star: StarIcon,
  "pin-filled": PinIcon,
  pin: PinIcon,
  "chevron-right": ChevronRightIcon,
  "chevron-down": ChevronDownIcon,
  "chat-bubble-help": CircleHelpIcon,
  folder: FolderIcon,
  "check-circle-filled": CircleCheckIcon,
  upload: UploadIcon,
  // tenantSectionIconName's two values.
  usergroup: UsersIcon,
  browse: EyeIcon,
};

function kbListIcon(name: string): LucideIcon {
  return kbListIcons[name] ?? InfoIcon;
}

// TDesign had filled variants of the star and pin; lucide draws outlines, so
// the filled state is the same outline with its fill turned on.
function kbListIconFill(name: string): string {
  return name === "star-filled" || name === "pin-filled" ? "fill-current" : "";
}

// Classes of a knowledge-base card, mine or shared (the old .kb-card and its
// type modifiers). Every type gets the brand border and a tinted lift on hover,
// a faint diagonal gradient, and a tinted quarter-circle in the top-right
// corner drawn by ::after beneath the card's content.
function kbCardClass(type: string | undefined, opts: { uninitialized?: boolean; highlighted?: boolean } = {}) {
  const faq = type === "faq";
  return [
    "group/kb border-border relative box-border flex h-[136px] min-h-[136px] cursor-pointer flex-col overflow-hidden rounded-[8px] border px-3.5 py-3 shadow-[0_1px_3px_rgba(0,0,0,0.04)] transition-all duration-[250ms] ease-in-out hover:border-[var(--td-brand-color)]",
    "after:pointer-events-none after:absolute after:top-0 after:right-0 after:z-0 after:size-[60px] after:rounded-[0_12px_0_100%] after:content-['']",
    faq
      ? "[background:linear-gradient(135deg,var(--td-bg-color-container)_0%,rgba(0,82,217,0.04)_100%)] hover:shadow-[0_4px_12px_rgba(0,82,217,0.12)] hover:[background:linear-gradient(135deg,var(--td-bg-color-container)_0%,rgba(0,82,217,0.08)_100%)] after:[background:linear-gradient(135deg,rgba(0,82,217,0.08)_0%,transparent_100%)]"
      : "[background:linear-gradient(135deg,var(--td-bg-color-container)_0%,rgba(7,192,95,0.04)_100%)] hover:shadow-[0_4px_12px_rgba(7,192,95,0.12)] hover:[background:linear-gradient(135deg,var(--td-bg-color-container)_0%,rgba(7,192,95,0.08)_100%)] after:[background:linear-gradient(135deg,rgba(7,192,95,0.08)_0%,transparent_100%)]",
    opts.uninitialized ? "opacity-90" : "",
    opts.highlighted ? "highlight-flash" : "",
  ];
}

// The type badge at the bottom-left of a card: wider than the square feature
// badges because it carries the document or entry count.
function kbTypeBadgeClass(type: string | undefined) {
  return type === "faq"
    ? "text-primary bg-[rgba(0,82,217,0.08)] hover:bg-[rgba(0,82,217,0.12)]"
    : "bg-[rgba(7,192,95,0.08)] text-[var(--td-brand-color-active)] hover:bg-[rgba(7,192,95,0.12)]";
}

// The card "more" menu, as dropdown-menu.css styled the old card-more-popup and
// its .popup-menu items (including the dark-mode glass panel).
const CARD_MENU_CONTENT_CLASS =
  "border-border mt-1 w-auto min-w-[148px] gap-px overflow-hidden rounded-[10px] border-[0.5px] p-1 shadow-[0_0_0_0.5px_rgba(0,0,0,0.03),0_2px_4px_rgba(0,0,0,0.04),0_8px_24px_rgba(0,0,0,0.1)] ring-0 backdrop-blur-[20px] backdrop-saturate-[1.8] dark:border-[rgba(255,255,255,0.08)] dark:bg-[rgba(36,36,36,0.92)] dark:shadow-[0_0_0_0.5px_rgba(255,255,255,0.05),0_2px_4px_rgba(0,0,0,0.12),0_8px_32px_rgba(0,0,0,0.28)]";
const CARD_MENU_ITEM_CLASS =
  "group/item text-foreground hover:bg-accent relative flex cursor-pointer items-center gap-2.5 rounded-md px-3 py-2 text-sm leading-5 font-normal transition-all duration-150 ease-[cubic-bezier(0.2,0,0,1)] active:scale-[0.98] active:bg-[var(--td-bg-color-container-active)]";
const CARD_MENU_ICON_CLASS =
  "text-muted-foreground group-hover/item:text-foreground size-4 shrink-0 transition-all duration-150 ease-[cubic-bezier(0.2,0,0,1)]";
const CARD_MENU_DANGER_ITEM_CLASS =
  "group/item relative mt-1 flex cursor-pointer items-center gap-2.5 rounded-md px-3 py-2 text-sm leading-5 font-normal text-[var(--td-error-color-6)] transition-all duration-150 ease-[cubic-bezier(0.2,0,0,1)] before:absolute before:-top-[3px] before:right-2 before:left-2 before:h-px before:bg-[var(--td-component-stroke)] before:content-[''] hover:bg-[var(--td-error-color-1)] active:scale-[0.98] active:bg-[var(--td-error-color-2)]";

// The sticky group header above a run of cards (the old .kb-section-header):
// the row only paints the background that keeps cards from showing through
// while it is stuck; clicks land on its children, so a click in the empty
// space to the right does not fold the group. Keyboard focus is unaffected.
const KB_SECTION_HEADER_CLASS =
  "group text-muted-foreground hover:text-foreground bg-card pointer-events-none sticky top-0 z-[5] [grid-column:1/-1] flex cursor-pointer items-center gap-1.5 py-1.5 pr-1 pl-0 [font-family:var(--app-font-family)] text-[13px] leading-5 font-semibold [box-shadow:0_-8px_0_0_var(--td-bg-color-container),0_4px_0_0_var(--td-bg-color-container)] outline-none select-none focus-visible:[box-shadow:0_0_0_2px_var(--td-brand-color-focus,rgba(0,82,217,0.2))] [&>*]:pointer-events-auto";

const router = useRouter();
const route = useRoute();
const uiStore = useUIStore();
const authStore = useAuthStore();
const { loaded: modelsReadyLoaded, isReadyForDocumentKb } = useTenantModelReadiness();
const orgStore = useOrganizationStore();
const chatResources = useChatResourcesStore();
const { t } = useI18n();

// 左侧空间选择：默认根据当前角色决定。
// Viewer 在该空间里通常 0 KB owned，"我的"会显示空状态、又把共享 KB 藏起来，
// 体验非常误导；所以 Viewer 默认落到 "all"（我的 + 共享给我都显示）。
// Contributor 及以上一进来主要管理自己创建的 KB，仍默认 "mine"。
//
// State lives in `?scope=` so links are shareable/bookmarkable; the
// composable handles two-way sync with the URL. We keep "mine" as the
// stored value (not "workspace") for back-compat with any external link
// that might point at the old query — its display label is rebranded
// via ListSpaceSidebar's workspaceLabel computed.
const defaultScope: "all" | "mine" = authStore.hasRole("contributor") ? "mine" : "all";
const { scope: spaceSelection, creator: creatorFilter } = useListUrlState({
  defaultScope,
  defaultCreator: "all",
});

// Per-user favorites + recents (localStorage-backed). isFavorite & touchRecent
// are wired into card render and click handlers below.
const pins = useResourcePins();
const kbFavoritesCount = computed(() => pins.favorites.value.filter((e) => e.type === "kb").length);
const kbRecentsCount = computed(() => pins.recents.value.filter((e) => e.type === "kb").length);

interface KB {
  id: string;
  name: string;
  description?: string;
  updated_at?: string;
  created_at?: string;
  pinned_at?: string;
  embedding_model_id?: string;
  summary_model_id?: string;
  type?: "document" | "faq";
  showMore?: boolean;
  vlm_config?: { enabled?: boolean; model_id?: string };
  extract_config?: { enabled?: boolean };
  storage_provider_config?: { provider?: string };
  storage_config?: { provider?: string; bucket_name?: string }; // legacy
  question_generation_config?: { enabled?: boolean; question_count?: number };
  knowledge_count?: number;
  chunk_count?: number;
  isProcessing?: boolean;
  processing_count?: number;
  share_count?: number;
  is_pinned?: boolean;
  // creator_id is the owner-id matched against authStore.user.id when
  // gating the per-card more-menu (Settings / Delete). Empty for legacy
  // KBs created before PR 5; those fall back to the role gate.
  creator_id?: string;
  // creator_name 由后端 list 接口回填，仅用于卡片右下角来源徽章的 tooltip。
  creator_name?: string;
}

const kbs = ref<KB[]>([]);
const loading = ref(false);
const deleteVisible = ref(false);
const deletingKb = ref<KB | null>(null);
const currentMoreIndex = ref<number>(-1);
const highlightedKbId = ref<string | null>(null);
const highlightedCardRef = ref<HTMLElement | null>(null);
const uploadTasks = ref<UploadTaskState[]>([]);
const uploadCleanupTimers = new Map<string, ReturnType<typeof setTimeout>>();
let uploadRefreshTimer: ReturnType<typeof setTimeout> | null = null;
const UPLOAD_CLEANUP_DELAY = 10000;

// Share dialog state
const shareDialogVisible = ref(false);
const sharingKbId = ref("");
const sharingKbName = ref("");

// Shared knowledge bases (everything cross-tenant shared to me, including
// viewer-only). Used by the per-space views and the "all" aggregate so
// readers still see read-only shares — those are valid resources, just
// not editable.
const sharedKbs = computed<SharedKnowledgeBase[]>(() => orgStore.sharedKnowledgeBases || []);

const allKnowledgeBases = computed(() => kbs.value.length + sharedKbs.value.length);

// 当前选中的是空间 ID（非全部、非我的、非收藏/最近这类伪 scope）
// NB: keep the reserved-scope list in sync with ListSpaceSidebar's
// non-org buckets — otherwise a new pseudo-scope (e.g. "favorites")
// falls through here and triggers the per-space code paths, which
// renders an extra "no shared KB" empty state on top of the real view.
const RESERVED_SCOPES = new Set(["all", "mine", "favorites", "recents"]);
const spaceSelectionOrgId = computed(() => {
  const s = spaceSelection.value;
  return !!s && !RESERVED_SCOPES.has(s);
});

// 当前空间下共享给我的知识库（旧：仅他人共享；保留用于兼容）

// 空间视角：该空间内全部知识库（含我共享的），选中空间时请求新接口
const spaceKbsList = ref<OrganizationSharedKnowledgeBaseItem[]>([]);
const spaceKbsLoading = ref(false);

// 「工作空间」视图下的稳定排序：本空间内「我创建」在前、「同事创建」在后；
// 子段内保留服务端的置顶优先顺序。给 contributor 视图把「本空间 · 仅查看」
// 分组标题正好插在过渡处；其他角色看不到标题，纯排序变化也无害。
// Ordering for the 「本空间」 tab:
//   1. pinned KBs (mine or teammate), newest pin first
//   2. my non-pinned KBs
//   3. teammate non-pinned KBs (rendered under the「本空间 · 仅查看」header)
//
// Pin is per-user as of migration 000050, so a teammate-created KB that
// the caller has personally pinned must float into the pinned section
// even though it would otherwise live in the teammate sub-group. The
// previous version only bucketed by isMyKb and silently demoted these
// pinned-but-teammate KBs.
const sortedMineKbs = computed<KB[]>(() => {
  return [...kbs.value].sort((a, b) => {
    const ap = a.is_pinned ? 0 : 1;
    const bp = b.is_pinned ? 0 : 1;
    if (ap !== bp) return ap - bp;
    if (a.is_pinned && b.is_pinned) {
      const at = a.pinned_at ? Date.parse(a.pinned_at as string) : 0;
      const bt = b.pinned_at ? Date.parse(b.pinned_at as string) : 0;
      if (at !== bt) return bt - at;
    }
    const am = isMyKb(a) ? 0 : 1;
    const bm = isMyKb(b) ? 0 : 1;
    if (am !== bm) return am - bm;
    const ac = a.created_at ? Date.parse(a.created_at as string) : 0;
    const bc = b.created_at ? Date.parse(b.created_at as string) : 0;
    return bc - ac;
  });
});

// 空间视角下的稳定排序：我创建的（is_mine）放在前面，剩下的共享部分再按
// 可编辑 / 仅查看 排序——这样空间列表跟「全部」视图的视觉顺序一致。
const sortedSpaceKbsList = computed(() => {
  return [...spaceKbsList.value].sort((a, b) => {
    const aMine = a.is_mine ? 0 : 1;
    const bMine = b.is_mine ? 0 : 1;
    if (aMine !== bMine) return aMine - bMine;
    const aE = isSharedKbEditable(a.permission) ? 0 : 1;
    const bE = isSharedKbEditable(b.permission) ? 0 : 1;
    return aE - bE;
  });
});
const spaceCountByOrg = ref<Record<string, number>>({});

// 各空间下的共享知识库数量（用于侧栏展示）：优先用接口返回的该空间总数，否则用「共享给我」数量
const sharedCountByOrg = computed<Record<string, number>>(() => {
  const map: Record<string, number> = {};
  sharedKbs.value.forEach((s) => {
    const id = s.organization_id;
    if (!id) return;
    map[id] = (map[id] || 0) + 1;
  });
  (orgStore.organizations || []).forEach((org) => {
    if (map[org.id] === undefined) map[org.id] = 0;
  });
  return map;
});
const effectiveSharedCountByOrg = computed<Record<string, number>>(() => {
  const base = sharedCountByOrg.value;
  const merged = { ...base };
  Object.keys(spaceCountByOrg.value).forEach((orgId) => {
    merged[orgId] = spaceCountByOrg.value[orgId];
  });
  return merged;
});

// Favorites / Recents views: hydrate pin entries by id against every KB
// the user can already see in this page (own + cross-tenant shared). KBs
// the user no longer has access to (deleted / share revoked) are dropped
// silently — the pin survives until the next mutation, which keeps the
// composable simple at the cost of harmless ghost entries.
//
// Order:
//   - favorites: most recently starred first (PinEntry.ts desc)
//   - recents: most recently opened first (also ts desc, already sorted)
const kbResourceIndex = computed(() => {
  const map = new Map<string, { kb: any; isMine: boolean; shared?: SharedKnowledgeBase }>();
  for (const kb of kbs.value) {
    map.set(kb.id, { kb, isMine: true });
  }
  for (const shared of sharedKbs.value) {
    if (!shared.knowledge_base) continue;
    if (!map.has(shared.knowledge_base.id)) {
      map.set(shared.knowledge_base.id, { kb: shared.knowledge_base, isMine: false, shared });
    }
  }
  return map;
});

const favoritesList = computed(() => {
  return pins.favorites.value
    .filter((e) => e.type === "kb")
    .map((e) => {
      const entry = kbResourceIndex.value.get(e.id);
      if (!entry) return null;
      if (entry.isMine) {
        return { ...entry.kb, isMine: true as const, _pinTs: e.ts };
      }
      const s = entry.shared!;
      return {
        ...entry.kb,
        isMine: false as const,
        permission: s.permission,
        shared_at: s.shared_at,
        share_id: s.share_id,
        org_name: s.org_name,
        _pinTs: e.ts,
      } as any;
    })
    .filter((x): x is NonNullable<typeof x> => x !== null);
});

const recentsList = computed(() => {
  return pins.recents.value
    .filter((e) => e.type === "kb")
    .map((e) => {
      const entry = kbResourceIndex.value.get(e.id);
      if (!entry) return null;
      if (entry.isMine) {
        return { ...entry.kb, isMine: true as const, _pinTs: e.ts };
      }
      const s = entry.shared!;
      return {
        ...entry.kb,
        isMine: false as const,
        permission: s.permission,
        shared_at: s.shared_at,
        share_id: s.share_id,
        org_name: s.org_name,
        _pinTs: e.ts,
      } as any;
    })
    .filter((x): x is NonNullable<typeof x> => x !== null);
});

// 可编辑权限：editor / admin。viewer 进入「仅查看」组。
// 用 share-level permission（不是空间角色）做判断——跨空间拿到 viewer 的，
// 即便我在本空间是 owner 也确实改不动那个 KB；反过来跨空间拿到 editor 的，
// 哪怕我在本空间是 contributor 也确实能改。
const EDITABLE_PERMS = new Set(["admin", "editor"]);
function isSharedKbEditable(perm: string | undefined): boolean {
  return !!perm && EDITABLE_PERMS.has(perm);
}

// 是否在共享区展示「可编辑 / 仅查看」二级分组：仅对中间档（contributor / editor）
// 有意义。viewer 反正都是只读，admin / owner 视角统一管理，分组反而碎。
// 这里只是 UI 呈现，权限由后端兜底，不要把它当成安全边界。
// 分组标题对所有角色生效——置顶 / 我创建的 / 本空间 · 仅查看 / 共享给我
// 都是基于"创建者 + 来源"的客观信息，不依赖当前用户的可写权限。
// 原本只对 contributor 显示是为了在 admin/owner 那里隐藏"仅查看"这个权限
// 暗示——但实际上 admin/owner 也会想区分自己创建 vs 同事创建的卡片，所以
// 现在统一打开。如果哪天需要把权限色彩从标题里拿掉，就改 i18n 文案即可，
// 不需要再回头碰这个 computed。
const showShareGroupHeaders = computed(() => true);

// 同空间、非当前用户创建的 KB 分组标题。
// contributor / viewer 在本空间里对这些 KB 没有写权限，所以打"仅查看"；
// admin / owner 反而对整个空间都有编辑权限，"仅查看"会反复误导他们以为
// 自己改不了——这一段实际上是"工作空间里其他成员创建的 KB"，按所有权
// 而非权限来标注更准确。
const tenantSectionLabelKey = computed(() =>
  authStore.hasRole("admin") ? "knowledgeList.sections.tenantOthers" : "knowledgeList.sections.tenantReadonly",
);

// 图标和上面的文案对齐：admin/owner 看到的是"本空间 · 其他成员"，按所有权
// 划分，配 usergroup（多人）更贴；contributor/viewer 看到的是"仅查看"，
// 维持 browse（眼睛）传达"只能看不能改"的语义。
const tenantSectionIconName = computed(() => (authStore.hasRole("admin") ? "usergroup" : "browse"));

// 分组折叠：ephemeral，只在当前会话里生效，不落 localStorage/服务器。
// 之所以走"折叠集合"而不是"展开集合"，是因为默认全展开——空 Set
// 即表示初始的全展开状态，避免每次新加分段还得回头维护默认值。
type KbSectionKey = "pinned" | "mine" | "tenantOthers" | "sharedByMe" | "sharedEditable" | "sharedReadonly";
const collapsedKbSections = ref<Set<KbSectionKey>>(new Set());
const isKbSectionCollapsed = (key: KbSectionKey) => collapsedKbSections.value.has(key);
const toggleKbSection = (key: KbSectionKey) => {
  // 重新赋一个新的 Set 是为了让 ref 的 .value 身份变化触发模板重渲染；
  // 直接 .add/.delete 在 Vue 3 的 reactive Set 里也能 work，但 ref(Set) 的
  // 内层代理行为在不同版本上略有差异，整体替换最稳。
  const next = new Set(collapsedKbSections.value);
  if (next.has(key)) next.delete(key);
  else next.add(key);
  collapsedKbSections.value = next;
};
// 判断一条 KB 应该归在哪个分组——和模板里几处 v-if 用的是同一套判定，
// 抽出来是为了 v-show 卡片时复用，避免把 5 个分组的 v-if 重新拼一遍。
//
// 输入有两种形态：
//   1. filteredKnowledgeBases 的元素，会显式带 `isMine` 标志（见
//      filteredKnowledgeBases 里的 spread；跨空间 shared 拆给 isMine=false）。
//   2. sortedMineKbs 的元素就是原始 KB，无 isMine、也无 permission 字段。
// 跨空间共享条目一定带 `permission`，本空间条目永远没有，所以"无 permission"
// 是本空间的安全标识。综合：先看 isMine，再回退到 permission 是否存在。
const kbSectionOf = (kb: any): KbSectionKey => {
  if (kb?.is_pinned) return "pinned";
  const isOwnTenant = kb?.isMine === true || (kb?.isMine !== false && kb?.permission == null);
  if (isOwnTenant) return isMyKb(kb) ? "mine" : "tenantOthers";
  return isSharedKbEditable(kb?.permission) ? "sharedEditable" : "sharedReadonly";
};

// 空间筛选视图（sortedSpaceKbsList）的条目结构与上面不同：is_mine 直接标识
// 「我共享出来的」，其余按 permission 走 sharedEditable / sharedReadonly。
const spaceKbSectionOf = (shared: any): KbSectionKey => {
  if (shared?.is_mine) return "sharedByMe";
  return isSharedKbEditable(shared?.permission) ? "sharedEditable" : "sharedReadonly";
};
const isSpaceKbCollapsed = (shared: any): boolean => isKbSectionCollapsed(spaceKbSectionOf(shared));

// 每个分组里实际有多少张卡片——直接把分组判定函数复用一遍。组标题上展示
// "(N)" 让用户一眼知道折叠后会藏掉多少，也方便核对筛选结果。
const emptyKbCounts = (): Record<KbSectionKey, number> => ({
  pinned: 0,
  mine: 0,
  tenantOthers: 0,
  sharedByMe: 0,
  sharedEditable: 0,
  sharedReadonly: 0,
});
const filteredKbSectionCounts = computed<Record<KbSectionKey, number>>(() => {
  const c = emptyKbCounts();
  filteredKnowledgeBases.value.forEach((kb) => {
    c[kbSectionOf(kb)]++;
  });
  return c;
});
const mineKbSectionCounts = computed<Record<KbSectionKey, number>>(() => {
  const c = emptyKbCounts();
  sortedMineKbs.value.forEach((kb) => {
    c[kbSectionOf(kb)]++;
  });
  return c;
});
const spaceKbSectionCounts = computed<Record<KbSectionKey, number>>(() => {
  const c = emptyKbCounts();
  sortedSpaceKbsList.value.forEach((shared) => {
    c[spaceKbSectionOf(shared)]++;
  });
  return c;
});

// Filtered knowledge bases: 全部 = 我的 + 全部共享；我的 = 仅我的
//
// Favorites / Recents reuse the same render path as `all` — they're just
// pre-filtered, pre-ordered slices, so the existing kb-card / shared
// kb-card templates render them with zero extra markup. Order is
// preserved via the upstream array (pins order is ts-desc).
const filteredKnowledgeBases = computed(() => {
  if (spaceSelection.value === "favorites") {
    return favoritesList.value;
  }
  if (spaceSelection.value === "recents") {
    return recentsList.value;
  }
  if (spaceSelection.value === "mine") {
    return kbs.value.map((kb) => ({ ...kb, isMine: true as const }));
  }
  if (spaceSelection.value !== "all") {
    return [];
  }
  // The "All" scope merges own + shared KBs. The card template keys each
  // row by `kb.id`, so the same KB surfacing twice — owned *and* shared
  // back, or shared into the caller's view through two different orgs —
  // produced duplicate `v-for` keys and blanked the list once there were
  // ≥2 entries (#795). mergeAllScopeKnowledgeBases de-duplicates by KB id
  // (owned wins; most-privileged share kept) while preserving the existing
  // pinned → mine → teammate → shared(editable-first) ordering.
  return mergeAllScopeKnowledgeBases(
    kbs.value as unknown as OwnedKnowledgeBase[],
    sharedKbs.value as unknown as SharedKnowledgeBaseLike[],
    authStore.user?.id,
  ) as unknown as Array<
    | (KB & { isMine: true })
    | (SharedKnowledgeBase["knowledge_base"] & {
        isMine: false;
        permission: string;
        shared_at: string;
        share_id: string;
      } & any)
  >;
});

const showKbListEmpty = computed(() => {
  if (loading.value) return false;
  if (!authStore.hasRole("contributor")) return false;
  if (spaceSelection.value === "all" && filteredKnowledgeBases.value.length === 0) return true;
  if (spaceSelection.value === "mine" && kbs.value.length === 0) return true;
  return false;
});

const showKbListContextualGuide = computed(() => showKbListEmpty.value && !uiStore.showKBEditorModal);

interface UploadTaskState {
  uploadId: string;
  kbId: string;
  fileName?: string;
  progress: number;
  status: "uploading" | "success" | "error";
  error?: string;
}

interface UploadSummary {
  kbId: string;
  kbName: string;
  total: number;
  completed: number;
  progress: number;
  hasError: boolean;
}

const applyKbListData = (data: any[]) => {
  kbs.value = data.map((kb: any) => ({
    ...kb,
    updated_at: kb.updated_at ? formatStringDate(new Date(kb.updated_at)) : "",
    showMore: false,
    isProcessing: kb.is_processing || false,
    processing_count: kb.processing_count || 0,
  }));
};

const fetchList = (force = false) => {
  loading.value = true;
  // The creator filter only applies to the caller's own tenant KBs (the
  // first call). Shared KBs are inherently "not mine" so we don't filter
  // them server-side; the segmented control is also hidden whenever the
  // user is browsing the shared / per-space scopes.
  return Promise.all([
    chatResources.fetchKnowledgeBasesForList({ creator: creatorFilter.value }, force).then(applyKbListData),
    orgStore.fetchSharedKnowledgeBases({ force }),
    orgStore.fetchOrganizations({ force }),
  ])
    .finally(() => {
      loading.value = false;
    })
    .then(() => {
      // 各空间知识库数量已由 GET /organizations 的 resource_counts 带回，存于 orgStore.resourceCounts
      const counts = orgStore.resourceCounts?.knowledge_bases?.by_organization;
      if (counts) spaceCountByOrg.value = { ...counts };
    });
};

// 选中空间时请求该空间内全部知识库（含我共享的）
watch(
  spaceSelection,
  (val) => {
    // Stale URL guard: an older "协作" view used scope=shared; that view
    // was removed, so normalize back to "all" instead of letting the
    // value fall through to the per-space fetch branch (which would 404
    // on the string "shared").
    if (val === "shared") {
      spaceSelection.value = "all";
      return;
    }
    if (val === "all" || val === "mine" || val === "favorites" || val === "recents" || !val) {
      spaceKbsList.value = [];
      return;
    }
    spaceKbsLoading.value = true;
    listOrganizationSharedKnowledgeBases(val)
      .then((res) => {
        if (res.success && res.data) {
          spaceKbsList.value = res.data;
          spaceCountByOrg.value = { ...spaceCountByOrg.value, [val]: res.data.length };
        } else {
          spaceKbsList.value = [];
        }
      })
      .finally(() => {
        spaceKbsLoading.value = false;
      });
  },
  { immediate: true },
);

// Refetch when the creator filter flips. We re-pull the whole list rather
// than filtering in-memory so the server stays the single source of truth
// (and we don't need to worry about stale share_count or pagination later).
watch(creatorFilter, () => {
  fetchList(true);
});

onMounted(() => {
  fetchList().then(() => {
    // 检查路由参数中是否有需要高亮的知识库ID
    const highlightKbId = route.query.highlightKbId as string;
    if (highlightKbId) {
      triggerHighlightFlash(highlightKbId);
      // Drop the transient highlight param but preserve other state
      // (scope / creator / q) so refreshing doesn't reset the user's view.
      const { highlightKbId: _drop, ...rest } = route.query;
      router.replace({ query: rest });
    }
  });

  window.addEventListener("knowledgeFileUploadStart", handleUploadStartEvent as EventListener);
  window.addEventListener("knowledgeFileUploadProgress", handleUploadProgressEvent as EventListener);
  window.addEventListener("knowledgeFileUploadComplete", handleUploadCompleteEvent as EventListener);
  window.addEventListener("knowledgeFileUploaded", handleUploadFinishedEvent as EventListener);
});

onUnmounted(() => {
  window.removeEventListener("knowledgeFileUploadStart", handleUploadStartEvent as EventListener);
  window.removeEventListener("knowledgeFileUploadProgress", handleUploadProgressEvent as EventListener);
  window.removeEventListener("knowledgeFileUploadComplete", handleUploadCompleteEvent as EventListener);
  window.removeEventListener("knowledgeFileUploaded", handleUploadFinishedEvent as EventListener);

  uploadCleanupTimers.forEach((timer) => clearTimeout(timer));
  uploadCleanupTimers.clear();
  if (uploadRefreshTimer) {
    clearTimeout(uploadRefreshTimer);
    uploadRefreshTimer = null;
  }
});

// 监听路由变化，处理从其他页面跳转过来的高亮需求
watch(
  () => route.query.highlightKbId,
  (newKbId) => {
    if (newKbId && typeof newKbId === "string" && kbs.value.length > 0) {
      triggerHighlightFlash(newKbId);
      const { highlightKbId: _drop, ...rest } = route.query;
      router.replace({ query: rest });
    }
  },
);

const openMore = (index: number) => {
  // 只记录当前打开的索引，用于显示激活样式
  // 弹窗的开关由 v-model 自动管理
  currentMoreIndex.value = index;
};

const onVisibleChange = (visible: boolean) => {
  // 弹窗关闭时重置索引
  if (!visible) {
    currentMoreIndex.value = -1;
  }
};

const handleSettings = (kb: KB) => {
  // 手动关闭弹窗
  kb.showMore = false;
  goSettings(kb.id);
};

// canManageKBCard mirrors KnowledgeBase.vue's `canManage`, gating the
// destructive items of the per-card menu — Settings, Delete — so a
// Viewer cannot click into them for a KB they don't own. The server
// still rejects the call (PR 5 guards every such mutation with
// OwnedKBOrAdmin) but the UI shouldn't surface buttons the user has
// no authority to use.
//
// The pin item is intentionally NOT gated by this predicate any more:
// pin state is per (user, kb) as of migration 000050 and the backend
// route only requires KB read access, so anyone who can see the card
// should be able to pin it for themselves.
//
// Legacy KBs created before PR 5 have an empty creator_id; treat
// those as tenant-owned (Admin+ may manage) so existing KBs aren't
// suddenly unmanageable for everyone.
function canManageKBCard(kb: KB): boolean {
  const userId = authStore.user?.id || "";
  if (kb.creator_id && userId && kb.creator_id === userId) return true;
  return authStore.hasRole("admin");
}

function canDuplicateKBCard(kb: any): boolean {
  return authStore.hasRole("contributor") && kb.isMine !== false;
}

// isMyKb 仅用于卡片右下角徽章在「我创建」与「同空间其他成员创建」之间切换。
// 与 canManageKBCard 不同：管理权限有 admin 兜底，徽章纯粹按创建者匹配。
// creator_id 为空（PR 5 RBAC 迁移之前的老 KB）一律按 tenant 处理——避免把
// 全空间共有的旧 KB 错误地都标成「我创建」。
function isMyKb(kb: { creator_id?: string }): boolean {
  const userId = authStore.user?.id || "";
  return !!(kb.creator_id && userId && kb.creator_id === userId);
}

// kbOriginVariant 决定卡片右下角徽章的展示形态：
//   - 我自己创建的：mine（绿色 "我创建"）
//   - 同空间他人创建的：creator 变体——只显示创建者名字。用户始终在
//     某个工作空间内浏览（顶部 TenantSelector 已经标了空间身份），右下
//     角再贴一遍空间名属于重复信息；contributor / admin / owner / viewer
//     看到的徽章一致。创建者无法解析时，creator 变体自动回退到
//     resourceOrigin.tenant 文案（"本空间"），不会出现空标签。
function kbOriginVariant(kb: { creator_id?: string }): "mine" | "creator" {
  return isMyKb(kb) ? "mine" : "creator";
}

function showKbOriginBadge(kb: { creator_id?: string; creator_name?: string }): boolean {
  return shouldShowResourceOriginBadge({
    section: kbSectionOf(kb),
    variant: kbOriginVariant(kb),
    creatorName: kb.creator_name,
    showSectionHeaders: showShareGroupHeaders.value,
  });
}

// 通过 ID 处理设置（用于全部 Tab 下的知识库）
const handleSettingsById = (id: string) => {
  goSettings(id);
};

// 通过 ID 处理删除（用于全部 Tab 下的知识库）
const handleDeleteById = (id: string) => {
  const kb = kbs.value.find((k) => k.id === id);
  if (kb) {
    deletingKb.value = kb;
    deleteVisible.value = true;
  }
};

const handleTogglePin = async (kb: KB) => {
  kb.showMore = false;
  try {
    const res: any = await togglePinKnowledgeBase(kb.id);
    if (res.success) {
      MessagePlugin.success(
        res.data.is_pinned ? t("knowledgeList.pin.pinSuccess") : t("knowledgeList.pin.unpinSuccess"),
      );
      fetchList(true);
    }
  } catch {
    MessagePlugin.error(t("knowledgeList.pin.failed"));
  }
};

const handleTogglePinById = async (id: string) => {
  try {
    const res: any = await togglePinKnowledgeBase(id);
    if (res.success) {
      MessagePlugin.success(
        res.data.is_pinned ? t("knowledgeList.pin.pinSuccess") : t("knowledgeList.pin.unpinSuccess"),
      );
      fetchList(true);
    }
  } catch {
    MessagePlugin.error(t("knowledgeList.pin.failed"));
  }
};

const handleDuplicate = async (kb: KB) => {
  kb.showMore = false;
  await duplicateKB(kb.id);
};

const handleDuplicateById = async (id: string) => {
  await duplicateKB(id);
};

const duplicateKB = async (id: string) => {
  try {
    const res: any = await duplicateKnowledgeBase(id);
    if (res?.success) {
      const newKbId = res.data?.target_id || res.data?.knowledge_base?.id;
      MessagePlugin.success(t("knowledgeList.messages.duplicateSuccess"));
      await fetchList(true);
      if (newKbId) {
        triggerHighlightFlash(newKbId);
      }
    } else {
      MessagePlugin.error(res?.message || t("knowledgeList.messages.duplicateFailed"));
    }
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("knowledgeList.messages.duplicateFailed"));
  }
};

const handleShareSuccess = () => {
  // 共享成功后可刷新列表
  fetchList(true);
};

const handleSharedKbClick = (sharedKb: SharedKnowledgeBase) => {
  pins.touchRecent("kb", sharedKb.knowledge_base.id);
  // 跳转到共享知识库详情页
  router.push(`/platform/knowledge-bases/${sharedKb.knowledge_base.id}`);
};

// 处理"全部"Tab 中的共享知识库卡片点击（直接进入知识库）
const handleSharedKbClickFromAll = (kb: any) => {
  pins.touchRecent("kb", kb.id);
  router.push(`/platform/knowledge-bases/${kb.id}`);
};

// 右侧详情面板：共享知识库详情（含直接共享与来自智能体的）
type SharedKbDetailItem = SharedKnowledgeBase & { is_mine?: boolean; source_from_agent?: SourceFromAgentInfo };
const sharedDetailPanelVisible = ref(false);
const currentSharedKbForDetail = ref<SharedKbDetailItem | null>(null);

const closeSharedDetailPanel = () => {
  sharedDetailPanelVisible.value = false;
  currentSharedKbForDetail.value = null;
};

// 打开右侧详情面板（全部 Tab 共享卡片）
const openSharedDetailFromAll = (kb: any) => {
  const sharedKb = sharedKbs.value.find((s) => s.knowledge_base.id === kb.id);
  if (sharedKb) {
    currentSharedKbForDetail.value = sharedKb;
    sharedDetailPanelVisible.value = true;
  }
};

// 打开右侧详情面板（空间 Tab：直接共享或来自智能体）
const openSharedDetail = (sharedKb: SharedKbDetailItem) => {
  currentSharedKbForDetail.value = sharedKb;
  sharedDetailPanelVisible.value = true;
};

// 智能体对知识库的策略文案（用于抽屉「来源方式」为智能体时）
const agentKbStrategyText = (mode: string) => {
  if (mode === "all") return t("knowledgeList.detail.agentKbStrategyAll");
  if (mode === "selected") return t("knowledgeList.detail.agentKbStrategySelected");
  return t("knowledgeList.detail.agentKbStrategyNone");
};

// 从右侧面板进入知识库
const goToSharedKbFromPanel = () => {
  if (currentSharedKbForDetail.value) {
    router.push(`/platform/knowledge-bases/${currentSharedKbForDetail.value.knowledge_base.id}`);
    closeSharedDetailPanel();
  }
};

const handleDelete = (kb: KB) => {
  // 手动关闭弹窗
  kb.showMore = false;
  deletingKb.value = kb;
  deleteVisible.value = true;
};

const confirmDelete = () => {
  if (!deletingKb.value) return;

  deleteKnowledgeBase(deletingKb.value.id)
    .then((res: any) => {
      if (res.success) {
        MessagePlugin.success(t("knowledgeList.messages.deleted"));
        deleteVisible.value = false;
        deletingKb.value = null;
        fetchList(true);
      } else {
        MessagePlugin.error(res.message || t("knowledgeList.messages.deleteFailed"));
      }
    })
    .catch((e: any) => {
      MessagePlugin.error(e?.message || t("knowledgeList.messages.deleteFailed"));
    });
};

const isInitialized = (kb: KB) => {
  // LLM (summary) model is always required
  if (!kb.summary_model_id || kb.summary_model_id === "") return false;
  // Embedding model only required when RAG indexing is enabled (vector or keyword)
  const strategy = (kb as any).indexing_strategy;
  const needsEmbedding = !strategy || strategy.vector_enabled || strategy.keyword_enabled;
  if (needsEmbedding && (!kb.embedding_model_id || kb.embedding_model_id === "")) return false;
  return true;
};

const isWikiKb = (kb: unknown) =>
  !!(kb as { indexing_strategy?: { wiki_enabled?: boolean } } | null | undefined)?.indexing_strategy?.wiki_enabled;

// 计算是否有未初始化的知识库
const hasUninitializedKbs = computed(() => {
  return kbs.value.some((kb) => !isInitialized(kb));
});

const getKbDisplayName = (kbId: string) => {
  const target = kbs.value.find((kb) => kb.id === kbId);
  if (target?.name) return target.name;
  return t("knowledgeList.uploadProgress.unknownKb", { id: kbId }) as string;
};

const uploadSummaries = computed<UploadSummary[]>(() => {
  if (!uploadTasks.value.length) return [];
  const grouped: Record<string, UploadTaskState[]> = {};
  uploadTasks.value.forEach((task) => {
    const kbKey = String(task.kbId);
    if (!grouped[kbKey]) grouped[kbKey] = [];
    grouped[kbKey].push(task);
  });
  return Object.entries(grouped)
    .map(([kbId, tasks]) => {
      const total = tasks.length;
      const completed = tasks.filter((task) => task.status !== "uploading").length;
      const progressSum = tasks.reduce((sum, task) => sum + (task.progress ?? 0), 0);
      const avgProgress = total === 0 ? 0 : Math.min(100, Math.max(0, Math.round(progressSum / total)));
      const hasError = tasks.some((task) => task.status === "error");
      return {
        kbId,
        kbName: getKbDisplayName(kbId),
        total,
        completed,
        progress: avgProgress,
        hasError,
      };
    })
    .sort((a, b) => a.kbName.localeCompare(b.kbName));
});

const clampProgress = (value: number) => Math.min(100, Math.max(0, Math.round(value)));

const addUploadTask = (task: UploadTaskState) => {
  uploadTasks.value = [...uploadTasks.value.filter((item) => item.uploadId !== task.uploadId), task];
};

const patchUploadTask = (uploadId: string, patch: Partial<UploadTaskState>) => {
  const index = uploadTasks.value.findIndex((task) => task.uploadId === uploadId);
  if (index === -1) return;
  const nextTasks = [...uploadTasks.value];
  nextTasks[index] = { ...nextTasks[index], ...patch };
  uploadTasks.value = nextTasks;
};

const removeUploadTask = (uploadId: string) => {
  uploadTasks.value = uploadTasks.value.filter((task) => task.uploadId !== uploadId);
  const timer = uploadCleanupTimers.get(uploadId);
  if (timer) {
    clearTimeout(timer);
    uploadCleanupTimers.delete(uploadId);
  }
};

const scheduleUploadTaskCleanup = (uploadId: string) => {
  const existing = uploadCleanupTimers.get(uploadId);
  if (existing) {
    clearTimeout(existing);
  }
  const timer = setTimeout(() => {
    removeUploadTask(uploadId);
  }, UPLOAD_CLEANUP_DELAY);
  uploadCleanupTimers.set(uploadId, timer);
};

type UploadEventDetail = {
  uploadId: string;
  kbId?: string | number;
  fileName?: string;
  progress?: number;
  status?: UploadTaskState["status"];
  error?: string;
};

const ensureUploadTaskEntry = (detail?: UploadEventDetail) => {
  if (!detail?.uploadId) return null;
  const existing = uploadTasks.value.find((task) => task.uploadId === detail.uploadId);
  if (existing) return existing;
  if (!detail.kbId) return null;
  const initialProgress = typeof detail.progress === "number" ? clampProgress(detail.progress) : 0;
  const newTask: UploadTaskState = {
    uploadId: detail.uploadId,
    kbId: String(detail.kbId),
    fileName: detail.fileName,
    progress: initialProgress,
    status: detail.status || "uploading",
    error: detail.error,
  };
  addUploadTask(newTask);
  return newTask;
};

const handleCardClick = (kb: KB) => {
  // Track this open in the per-user "recent" list before navigating —
  // matches the user mental model "this is what I last worked on".
  pins.touchRecent("kb", kb.id);
  if (isInitialized(kb)) {
    goDetail(kb.id);
  } else {
    goSettings(kb.id);
  }
};

// toggleFavoriteKb is the click handler for the star icon rendered on
// each card. Stops propagation so it doesn't bubble into the card's
// own @click which would open the KB.
const toggleFavoriteKb = (kbId: string, evt?: Event) => {
  evt?.stopPropagation();
  pins.toggleFavorite("kb", kbId);
};
const isKbFavorited = (kbId: string) => pins.isFavorite("kb", kbId);

const goDetail = (id: string) => {
  router.push(`/platform/knowledge-bases/${id}`);
};

const goSettings = (id: string) => {
  // 使用模态框打开设置
  uiStore.openKBSettings(id);
};

// 创建知识库
const handleCreateKnowledgeBase = () => {
  markContextualGuideDone("kbList");
  // 无模型时仍打开创建向导，并定位到模型配置页；用户可在向导内添加模型，无需先跳转系统设置
  const initialSection = modelsReadyLoaded.value && !isReadyForDocumentKb.value ? "models" : undefined;
  uiStore.openCreateKB("document", initialSection);
};

// 知识库编辑器成功回调（创建或编辑成功）
const handleKBEditorSuccess = (kbId: string) => {
  console.log("[KnowledgeBaseList] knowledge operation success:", kbId);
  const shouldOpenDetailForUploadGuide = !isContextualGuideDone("kbDetail");
  // 列表页编辑同样要让单 KB 详情缓存失效，否则侧栏 / 详情页 60s 内仍显示旧信息
  chatResources.invalidateKnowledgeBaseDetail(kbId);
  fetchList(true).then(() => {
    if (shouldOpenDetailForUploadGuide && kbId && !uiStore.showKBEditorModal) {
      goDetail(kbId);
    }
    // 如果是从路由参数中获取的高亮ID，触发闪烁效果
    if (route.query.highlightKbId === kbId) {
      triggerHighlightFlash(kbId);
      const { highlightKbId: _drop, ...rest } = route.query;
      router.replace({ query: rest });
    }
  });
};

// 触发高亮闪烁效果
const triggerHighlightFlash = (kbId: string) => {
  highlightedKbId.value = kbId;
  nextTick(() => {
    if (highlightedCardRef.value) {
      // 滚动到高亮的卡片
      highlightedCardRef.value.scrollIntoView({
        behavior: "smooth",
        block: "center",
      });
    }
    // 3秒后清除高亮
    setTimeout(() => {
      highlightedKbId.value = null;
    }, 3000);
  });
};

const handleUploadStartEvent = (event: Event) => {
  const detail = (event as CustomEvent<UploadEventDetail>).detail;
  if (!detail?.uploadId || !detail?.kbId) return;
  addUploadTask({
    uploadId: detail.uploadId,
    kbId: String(detail.kbId),
    fileName: detail.fileName,
    progress: typeof detail.progress === "number" ? clampProgress(detail.progress) : 0,
    status: "uploading",
  });
};

const handleUploadProgressEvent = (event: Event) => {
  const detail = (event as CustomEvent<UploadEventDetail>).detail;
  if (!detail?.uploadId || typeof detail.progress !== "number") return;
  if (!ensureUploadTaskEntry(detail)) return;
  patchUploadTask(detail.uploadId, {
    progress: clampProgress(detail.progress),
  });
};

const handleUploadCompleteEvent = (event: Event) => {
  const detail = (event as CustomEvent<UploadEventDetail>).detail;
  if (!detail?.uploadId) return;
  const progress = typeof detail.progress === "number" ? clampProgress(detail.progress) : 100;
  if (!ensureUploadTaskEntry({ ...detail, progress })) return;
  patchUploadTask(detail.uploadId, {
    status: detail.status || "success",
    progress,
    error: detail.error,
  });
  scheduleUploadTaskCleanup(detail.uploadId);
};

const handleUploadFinishedEvent = (event: Event) => {
  const detail = (event as CustomEvent<{ kbId?: string | number }>).detail;
  if (!detail?.kbId) return;
  if (uploadRefreshTimer) {
    clearTimeout(uploadRefreshTimer);
  }
  uploadRefreshTimer = setTimeout(() => {
    fetchList(true);
    uploadRefreshTimer = null;
  }, 800);
};
</script>

<!-- The card grids' fade-in and the highlight flash are keyframe animations.
     Vue renames a scoped @keyframes, so an animate-[…] utility could not reach
     them; the two classes below can. -->
<style scoped>
@keyframes contentFadeIn {
  from {
    opacity: 0;
    transform: translateY(6px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes highlightFlash {
  0% {
    border-color: var(--td-brand-color);
    box-shadow: 0 0 0 0 rgba(7, 192, 95, 0.4);
    transform: scale(1);
  }

  50% {
    border-color: var(--td-brand-color);
    box-shadow: 0 0 0 8px rgba(7, 192, 95, 0);
    transform: scale(1.02);
  }

  100% {
    border-color: var(--td-brand-color);
    box-shadow: 0 0 0 0 rgba(7, 192, 95, 0);
    transform: scale(1);
  }
}

.kb-card-grid-fade-in {
  animation: contentFadeIn 0.32s ease-out;
}

.highlight-flash {
  animation: highlightFlash 0.6s ease-in-out 3;
  border-color: var(--td-brand-color) !important;
  box-shadow: 0 0 12px rgba(7, 192, 95, 0.3) !important;
}
</style>

<!-- The drawer is teleported to <body>, so its slide-in transition lives in a
     global (unscoped) style block. -->
<style>
.shared-detail-drawer-enter-active,
.shared-detail-drawer-leave-active {
  transition: opacity 0.25s ease;
}

.shared-detail-drawer-enter-active .shared-detail-slide,
.shared-detail-drawer-leave-active .shared-detail-slide {
  transition: transform 0.25s ease;
}

.shared-detail-drawer-enter-from,
.shared-detail-drawer-leave-to {
  opacity: 0;
}

.shared-detail-drawer-enter-from .shared-detail-slide,
.shared-detail-drawer-leave-to .shared-detail-slide {
  transform: translateX(100%);
}
</style>
