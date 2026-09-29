<template>
  <div class="relative mr-4 box-border flex h-full min-h-0 flex-1">
    <ListSpaceSidebar
      mode="organization"
      v-model="spaceSelection"
      :count-all="organizations.length"
      :count-created="createdCount"
      :count-joined="joinedCount"
    />
    <div class="flex min-w-0 flex-1 flex-col px-7 pt-5">
      <div class="mb-4 flex shrink-0 items-center justify-between">
        <div class="flex flex-col gap-1">
          <div class="flex items-center gap-2">
            <h2
              class="text-foreground m-0 font-[family-name:var(--app-font-family)] text-[24px] leading-8 font-semibold"
            >
              {{ $t("organization.title") }}
            </h2>
            <div class="flex shrink-0 items-center gap-2">
              <!-- A disabled button swallows pointer events, so the tooltip hangs on a wrapper. -->
              <Tooltip>
                <TooltipTrigger as-child>
                  <span class="inline-flex">
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      :class="headerActionButtonClass"
                      :disabled="!canManageOrg"
                      :aria-label="$t('organization.joinOrg')"
                      @click="handleJoinOrganization"
                    >
                      <LogInIcon class="text-primary size-4" />
                    </Button>
                  </span>
                </TooltipTrigger>
                <TooltipContent side="bottom">{{
                  canManageOrg ? $t("organization.joinOrg") : noPermissionTip
                }}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger as-child>
                  <span class="inline-flex">
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      :class="headerActionButtonClass"
                      :disabled="!canManageOrg"
                      :aria-label="$t('organization.createOrg')"
                      @click="handleCreateOrganization"
                    >
                      <img src="@/assets/img/organization-green.svg" class="size-4" alt="" aria-hidden="true" />
                    </Button>
                  </span>
                </TooltipTrigger>
                <TooltipContent side="bottom">{{
                  canManageOrg ? $t("organization.createOrg") : noPermissionTip
                }}</TooltipContent>
              </Tooltip>
            </div>
          </div>
          <p class="text-muted-foreground m-0 font-[family-name:var(--app-font-family)] text-[14px] leading-5">
            {{ $t("organization.subtitle") }}
          </p>
        </div>
      </div>
      <div class="min-w-0 flex-1 overflow-x-hidden overflow-y-auto py-2">
        <!-- 骨架屏占位 -->
        <div v-if="loading && filteredOrganizations.length === 0" :class="cardGridClass">
          <div v-for="n in 4" :key="'skel-' + n" :class="[cardBaseClass, 'cursor-default']">
            <div class="flex items-center gap-4">
              <Skeleton class="size-9 rounded-full" />
              <Skeleton class="h-5 w-1/2" />
            </div>
            <div class="mt-3 flex flex-1 flex-col gap-3">
              <Skeleton class="h-3.5 w-full" />
              <Skeleton class="h-3.5 w-[70%]" />
            </div>
            <div class="mt-auto flex gap-4">
              <Skeleton class="h-[22px] w-[60px] rounded-sm" />
              <Skeleton class="h-[22px] w-[60px] rounded-sm" />
            </div>
          </div>
        </div>

        <!-- 卡片网格 -->
        <div v-if="filteredOrganizations.length > 0" :class="cardGridClass">
          <template v-for="(org, index) in filteredOrganizations" :key="org.id">
            <!-- 我创建的：仅在 all 视图下出现；created/joined 子视图自身已经
                 隐含了语义，再加标题反而冗余。-->
            <div
              v-if="spaceSelection === 'all' && org.is_owner && index === 0"
              :class="sectionHeaderClass"
              role="button"
              tabindex="0"
              @click="toggleOrgSection('created')"
              @keydown.enter.prevent="toggleOrgSection('created')"
              @keydown.space.prevent="toggleOrgSection('created')"
            >
              <UserIcon class="size-3.5" />
              <span>{{ $t("organization.createdByMe") }}</span>
              <span :class="sectionCountClass">{{ orgSectionCounts.created }}</span>
              <component
                :is="isOrgSectionCollapsed('created') ? ChevronRightIcon : ChevronDownIcon"
                :class="sectionToggleClass"
              />
            </div>
            <!-- 我加入的：第一张非 owner 卡片前打标题（all 视图下） -->
            <div
              v-if="
                spaceSelection === 'all' && !org.is_owner && (index === 0 || filteredOrganizations[index - 1].is_owner)
              "
              :class="sectionHeaderClass"
              role="button"
              tabindex="0"
              @click="toggleOrgSection('joined')"
              @keydown.enter.prevent="toggleOrgSection('joined')"
              @keydown.space.prevent="toggleOrgSection('joined')"
            >
              <UsersIcon class="size-3.5" />
              <span>{{ $t("organization.joinedByMe") }}</span>
              <span :class="sectionCountClass">{{ orgSectionCounts.joined }}</span>
              <component
                :is="isOrgSectionCollapsed('joined') ? ChevronRightIcon : ChevronDownIcon"
                :class="sectionToggleClass"
              />
            </div>
            <!--
              与知识库 / 智能体列表统一：紧凑 + 1px 描边。The ::before glow in the
              top-right corner is drawn with before: utilities; a joined space
              lights up a little less on hover than one you own.
            -->
            <div
              v-show="!isOrgRowHidden(org)"
              :class="[
                cardBaseClass,
                'group cursor-pointer transition-[border-color,box-shadow,transform] duration-250 ease-in-out',
                cardGlowClass,
                org.is_owner
                  ? 'hover:border-[rgba(7,192,95,0.5)] hover:shadow-[0_6px_20px_rgba(7,192,95,0.12)]'
                  : 'hover:border-[rgba(7,192,95,0.4)] hover:shadow-[0_4px_16px_rgba(7,192,95,0.08)]',
              ]"
              @click="handleCardClick(org)"
            >
              <!-- 装饰：协作网络感图形 -->
              <div
                class="pointer-events-none absolute top-2 right-3.5 z-0 flex items-start justify-end text-[rgba(7,192,95,0.35)] transition-colors duration-300 group-hover:text-[rgba(7,192,95,0.55)]"
              >
                <svg
                  class="block h-10 w-14"
                  width="56"
                  height="40"
                  viewBox="0 0 56 40"
                  fill="none"
                  xmlns="http://www.w3.org/2000/svg"
                  aria-hidden="true"
                >
                  <circle cx="10" cy="12" r="4" stroke="currentColor" stroke-width="1.5" fill="none" opacity="0.5" />
                  <circle cx="28" cy="8" r="5" stroke="currentColor" stroke-width="1.8" fill="none" opacity="0.7" />
                  <circle cx="46" cy="14" r="4" stroke="currentColor" stroke-width="1.5" fill="none" opacity="0.5" />
                  <path
                    d="M14 13 L24 10 M32 10 L42 13"
                    stroke="currentColor"
                    stroke-width="1.2"
                    stroke-linecap="round"
                    opacity="0.4"
                  />
                  <circle cx="28" cy="28" r="6" stroke="currentColor" stroke-width="1.2" fill="none" opacity="0.35" />
                  <path
                    d="M28 14 L28 22 M20 18 L26 24 M36 18 L30 24"
                    stroke="currentColor"
                    stroke-width="1"
                    stroke-linecap="round"
                    opacity="0.3"
                  />
                </svg>
              </div>

              <!-- 卡片头部 -->
              <div class="relative z-[2] mb-1.5 flex items-center justify-between">
                <div class="flex min-w-0 flex-1 items-center gap-2">
                  <div class="flex shrink-0 items-center justify-center">
                    <SpaceAvatar :name="org.name" :avatar="org.avatar" size="small" />
                  </div>
                  <div class="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span
                      class="text-foreground truncate font-[family-name:var(--app-font-family)] text-[15px] leading-[22px] font-semibold tracking-[0.01em]"
                      :title="org.name"
                      >{{ org.name }}</span
                    >
                  </div>
                </div>
                <DropdownMenu
                  :open="!!organizationMenuVisibility[org.id]"
                  @update:open="(open: boolean) => onOrgMenuOpenChange(org, open)"
                >
                  <DropdownMenuTrigger as-child>
                    <!-- Hidden until the card is hovered, and held visible while its menu is open. -->
                    <button
                      type="button"
                      data-slot="org-card-more"
                      class="hover:bg-accent flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-lg transition-all duration-200 hover:opacity-100!"
                      :class="
                        organizationMenuVisibility[org.id]
                          ? 'bg-accent opacity-100'
                          : 'opacity-0 group-hover:opacity-60'
                      "
                      :aria-label="$t('docs.tree.moreActions')"
                      @click.stop
                    >
                      <img class="size-4" src="@/assets/img/more.png" alt="" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" :class="cardMenuClass" @click.stop>
                    <DropdownMenuItem :class="cardMenuItemClass" @select="handleSettings(org)">
                      <SettingsIcon class="text-muted-foreground" />
                      <span>{{ $t("organization.settings.editTitle") }}</span>
                    </DropdownMenuItem>
                    <template v-if="!org.is_owner || canManageOrg">
                      <DropdownMenuSeparator class="mx-2" />
                    </template>
                    <DropdownMenuItem
                      v-if="!org.is_owner"
                      variant="destructive"
                      :class="cardMenuItemClass"
                      @select="handleLeave(org)"
                    >
                      <LogOutIcon />
                      <span>{{ $t("organization.leave") }}</span>
                    </DropdownMenuItem>
                    <DropdownMenuItem
                      v-if="org.is_owner && canManageOrg"
                      variant="destructive"
                      :class="cardMenuItemClass"
                      @select="handleDelete(org)"
                    >
                      <Trash2Icon />
                      <span>{{ $t("common.delete") }}</span>
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>

              <!-- 卡片内容 -->
              <div class="relative z-[1] mb-1.5 flex min-h-0 flex-1 flex-col gap-1.5 overflow-hidden">
                <!-- 三个列表卡片统一：描述字体 -->
                <div
                  class="text-muted-foreground line-clamp-2 font-[family-name:var(--app-font-family)] text-[12px] leading-[17px]"
                >
                  {{ org.description || $t("organization.noDescription") }}
                </div>
              </div>

              <!-- 卡片底部（与知识库卡片风格统一：小标签、无日期、智能体用主题色） -->
              <div
                class="border-border relative z-[1] mt-auto flex items-center justify-between border-t-[0.5px] pt-1.5"
              >
                <div class="flex min-w-0 flex-1 items-center gap-1.5">
                  <div class="flex items-center gap-1">
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <div :class="[featureBadgeClass, featureBadgeMemberClass]">
                          <UserIcon class="size-3.5 shrink-0" />
                          <span class="leading-none">{{ org.member_count || 0 }}</span>
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("organization.memberCount") }}</TooltipContent>
                    </Tooltip>
                    <Tooltip>
                      <TooltipTrigger as-child>
                        <div :class="[featureBadgeClass, featureBadgeKbClass]">
                          <FolderIcon class="size-3.5 shrink-0" />
                          <span class="leading-none">{{ org.share_count ?? 0 }}</span>
                        </div>
                      </TooltipTrigger>
                      <TooltipContent side="top">{{ $t("organization.invite.knowledgeBases") }}</TooltipContent>
                    </Tooltip>
                  </div>
                  <!-- 待审核角标：与 feature-badge 同高 -->
                  <Tooltip v-if="(org.pending_join_request_count ?? 0) > 0">
                    <TooltipTrigger as-child>
                      <span
                        class="text-warning inline-flex h-[22px] items-center rounded-md bg-[rgba(250,173,20,0.12)] px-1.5 text-[12px] font-medium whitespace-nowrap"
                        >{{ org.pending_join_request_count }} {{ $t("organization.settings.pendingReview") }}</span
                      >
                    </TooltipTrigger>
                    <TooltipContent side="top">{{
                      $t("organization.settings.pendingJoinRequestsBadge")
                    }}</TooltipContent>
                  </Tooltip>
                </div>
                <!-- 右下角：创建者/角色 合并标签（带图标） -->
                <div v-if="showOrgRelationTag(org)" class="flex shrink-0 items-center">
                  <div
                    class="inline-flex h-[22px] items-center gap-1 rounded-md px-1.5 font-[family-name:var(--app-font-family)] text-[12px] font-medium"
                    :class="relationTagClass(org)"
                  >
                    <component :is="org.is_owner ? UserPlusIcon : UsersIcon" class="size-3.5 shrink-0" />
                    <span>{{
                      org.is_owner
                        ? $t("organization.owner")
                        : org.my_role
                          ? $t(`organization.role.${org.my_role}`)
                          : $t("organization.joinedByMe")
                    }}</span>
                  </div>
                </div>
              </div>
            </div>
          </template>
        </div>

        <!-- 空状态（按筛选显示不同文案） -->
        <div v-else-if="!loading" class="flex flex-1 flex-col items-center justify-center px-5 py-[60px]">
          <img class="mb-5 size-[162px]" src="@/assets/img/upload.svg" alt="" />
          <span
            class="text-placeholder mb-2 font-[family-name:var(--app-font-family)] text-[16px] leading-[26px] font-semibold"
            >{{ emptyStateTitle }}</span
          >
          <span
            class="mb-0 font-[family-name:var(--app-font-family)] text-[14px] leading-[22px] text-[var(--td-text-color-disabled)]"
            >{{ emptyStateDesc }}</span
          >
          <div class="mt-5 flex items-center gap-3">
            <Tooltip :disabled="canManageOrg">
              <TooltipTrigger as-child>
                <span class="inline-flex">
                  <Button
                    variant="outline"
                    class="text-primary hover:border-primary hover:text-primary border-[rgba(7,192,95,0.5)] font-medium transition-all duration-200 hover:bg-[rgba(7,192,95,0.08)]"
                    :disabled="!canManageOrg"
                    @click="handleJoinOrganization"
                  >
                    <LogInIcon />
                    {{ $t("organization.joinOrg") }}
                  </Button>
                </span>
              </TooltipTrigger>
              <TooltipContent side="top">{{ noPermissionTip }}</TooltipContent>
            </Tooltip>
            <Tooltip :disabled="canManageOrg">
              <TooltipTrigger as-child>
                <span class="inline-flex">
                  <Button
                    class="hover:bg-primary font-medium shadow-[0_2px_8px_rgba(7,192,95,0.25)] transition-all duration-250 hover:shadow-[0_4px_14px_rgba(7,192,95,0.35)]"
                    :disabled="!canManageOrg"
                    @click="handleCreateOrganization"
                  >
                    <img
                      src="@/assets/img/organization-green.svg"
                      class="size-4 brightness-0 invert"
                      alt=""
                      aria-hidden="true"
                    />
                    {{ $t("organization.createOrg") }}
                  </Button>
                </span>
              </TooltipTrigger>
              <TooltipContent side="top">{{ noPermissionTip }}</TooltipContent>
            </Tooltip>
          </div>
        </div>
      </div>
    </div>

    <!-- Organization Settings Modal (用于创建和编辑组织) -->
    <OrganizationSettingsModal
      :visible="showSettingsModal"
      :org-id="settingsOrgId"
      :mode="settingsMode"
      @update:visible="showSettingsModal = $event"
    />

    <!-- Delete Confirm Dialog -->
    <Dialog :open="deleteVisible" @update:open="(open: boolean) => (deleteVisible = open)">
      <DialogContent :show-close-button="false" :class="confirmDialogClass">
        <div class="mb-2 flex items-center">
          <img class="mr-2 size-5" src="@/assets/img/circle.png" alt="" />
          <DialogTitle :class="confirmDialogTitleClass">{{ $t("organization.deleteConfirmTitle") }}</DialogTitle>
        </div>
        <DialogDescription :class="confirmDialogTextClass">
          {{ $t("organization.deleteConfirmMessage", { name: deletingOrg?.name ?? "" }) }}
        </DialogDescription>
        <div class="flex h-[22px] w-full justify-end">
          <button
            type="button"
            data-slot="confirm-cancel"
            :class="confirmDialogActionClass"
            @click="deleteVisible = false"
          >
            {{ $t("common.cancel") }}
          </button>
          <button
            type="button"
            data-slot="confirm-delete"
            :class="[confirmDialogActionClass, 'text-destructive ml-10']"
            @click="confirmDelete"
          >
            {{ $t("common.delete") }}
          </button>
        </div>
      </DialogContent>
    </Dialog>

    <!-- Leave Confirm Dialog -->
    <Dialog :open="leaveVisible" @update:open="(open: boolean) => (leaveVisible = open)">
      <DialogContent :show-close-button="false" :class="confirmDialogClass">
        <div class="mb-2 flex items-center">
          <img class="mr-2 size-5" src="@/assets/img/circle.png" alt="" />
          <DialogTitle :class="confirmDialogTitleClass">{{ $t("organization.leaveConfirmTitle") }}</DialogTitle>
        </div>
        <DialogDescription :class="confirmDialogTextClass">
          {{ $t("organization.leaveConfirmMessage", { name: leavingOrg?.name ?? "" }) }}
        </DialogDescription>
        <div class="flex h-[22px] w-full justify-end">
          <button
            type="button"
            data-slot="confirm-cancel"
            :class="confirmDialogActionClass"
            @click="leaveVisible = false"
          >
            {{ $t("common.cancel") }}
          </button>
          <button
            type="button"
            data-slot="confirm-leave"
            :class="[confirmDialogActionClass, 'text-destructive ml-10']"
            @click="confirmLeave"
          >
            {{ $t("organization.leave") }}
          </button>
        </div>
      </DialogContent>
    </Dialog>

    <!-- 加入组织 / 邀请预览弹框（菜单与邀请链接共用同一弹框） -->
    <Teleport to="body">
      <!-- The overlay fades; the panel inside scales up from 95% as it does. -->
      <Transition
        enter-active-class="transition-all duration-300 ease-in-out"
        leave-active-class="transition-all duration-300 ease-in-out"
        enter-from-class="opacity-0 [&>div]:scale-95"
        leave-to-class="opacity-0 [&>div]:scale-95"
      >
        <!--
          data-slot opts the hand-written buttons in this modal into the
          element resets of tailwind.css. z-[2000] keeps it above the page;
          every popup opened from inside it therefore carries z-[2100].
        -->
        <div
          v-if="showInvitePreview"
          data-slot="invite-preview-overlay"
          class="fixed inset-0 z-[2000] flex items-center justify-center bg-black/50 p-5 backdrop-blur-[4px]"
          @click.self="closeInvitePreview"
        >
          <div
            class="bg-card relative flex max-h-[90vh] w-full flex-col overflow-hidden rounded-xl shadow-[0_8px_32px_rgba(0,0,0,0.12)] transition-transform duration-300 ease-in-out"
            :class="
              !invitePreviewData && !invitePreviewLoading && joinStep === 'search' ? 'max-w-[560px]' : 'max-w-[480px]'
            "
          >
            <div
              class="border-border bg-card flex shrink-0 items-center justify-between gap-3 border-b py-4 pr-12 pl-5"
            >
              <!-- 预览详情且来自搜索时显示返回按钮 -->
              <button
                v-if="invitePreviewData && !inviteCode"
                type="button"
                :class="[modalIconButtonClass, 'hover:text-primary']"
                @click="backFromPreview"
                :aria-label="$t('organization.join.backToSearch')"
              >
                <ChevronLeftIcon class="size-4" />
              </button>
              <h2 class="text-foreground m-0 min-w-0 flex-1 text-[16px] font-semibold">
                {{ invitePreviewData ? $t("organization.invite.previewTitle") : $t("organization.joinOrg") }}
              </h2>
              <button
                type="button"
                :class="[modalIconButtonClass, 'hover:text-foreground absolute top-4 right-4 z-10']"
                @click="closeInvitePreview"
                :aria-label="$t('common.close')"
              >
                <XIcon class="size-5" />
              </button>
            </div>

            <!-- 步骤1/2/Loading 共用高度过渡容器 -->
            <div
              class="h-auto flex-[0_0_auto] overflow-hidden transition-[min-height,max-height] duration-350 ease-[cubic-bezier(0.4,0,0.2,1)]"
              :style="inviteBodyWrapStyle"
            >
              <div ref="inviteBodyInnerRef" class="block">
                <!-- 步骤1：输入邀请码 或 搜索空间 -->
                <div
                  v-if="!invitePreviewLoading && !invitePreviewData"
                  :class="[modalBodyClass, scrollbarClass, 'px-6 pt-5']"
                >
                  <div class="mb-5 flex flex-wrap gap-2">
                    <button
                      type="button"
                      :class="joinModePillClass(joinStep === 'invite')"
                      @click="joinStep = 'invite'"
                    >
                      {{ $t("organization.join.byInviteCode") }}
                    </button>
                    <button
                      type="button"
                      :class="joinModePillClass(joinStep === 'search')"
                      @click="handleSearchTabClick"
                    >
                      {{ $t("organization.join.searchSpaces") }}
                    </button>
                  </div>

                  <!-- Tab 内容容器 - 平滑高度过渡 -->
                  <div
                    ref="tabContentWrapperRef"
                    class="overflow-hidden transition-[height] duration-300 ease-[cubic-bezier(0.4,0,0.2,1)]"
                  >
                    <!-- 输入邀请码 -->
                    <div v-if="joinStep === 'invite'" class="w-full">
                      <template v-if="!invitePreviewError">
                        <div class="mb-5">
                          <label for="org-join-invite-code" :class="joinFormLabelClass">{{
                            $t("organization.inviteCode")
                          }}</label>
                          <p :class="joinFormDescClass">{{ $t("organization.invite.inputDesc") }}</p>
                          <div class="relative">
                            <Input
                              id="org-join-invite-code"
                              v-model="joinInputCode"
                              :placeholder="$t('organization.inviteCodePlaceholder')"
                              :maxlength="32"
                              class="pr-8"
                              @keyup.enter="doPreviewFromInput"
                            />
                            <button
                              v-if="joinInputCode"
                              type="button"
                              :class="inputClearButtonClass"
                              :aria-label="$t('common.clear')"
                              @click="joinInputCode = ''"
                            >
                              <CircleXIcon class="size-3.5" />
                            </button>
                          </div>
                          <p class="text-placeholder mt-2 mb-0 text-[12px] leading-[1.45]">
                            {{ $t("organization.editor.inviteCodeTip") }}
                          </p>
                        </div>
                      </template>
                      <template v-else>
                        <div
                          class="text-destructive mb-3 flex items-center gap-2 rounded-lg bg-[var(--td-error-color-light)] px-3 py-2.5 text-[13px]"
                        >
                          <CircleAlertIcon class="size-5 shrink-0" />
                          <span>{{ invitePreviewError }}</span>
                        </div>
                        <div class="mb-5">
                          <label for="org-join-invite-code-retry" :class="joinFormLabelClass">{{
                            $t("organization.inviteCode")
                          }}</label>
                          <div class="relative">
                            <Input
                              id="org-join-invite-code-retry"
                              v-model="joinInputCode"
                              :placeholder="$t('organization.inviteCodePlaceholder')"
                              :maxlength="32"
                              class="pr-8"
                              @keyup.enter="doPreviewFromInput"
                            />
                            <button
                              v-if="joinInputCode"
                              type="button"
                              :class="inputClearButtonClass"
                              :aria-label="$t('common.clear')"
                              @click="joinInputCode = ''"
                            >
                              <CircleXIcon class="size-3.5" />
                            </button>
                          </div>
                        </div>
                      </template>
                      <div :class="[modalFooterClass, 'mt-1 px-0 pt-4']">
                        <Button variant="outline" @click="closeInvitePreview">
                          {{ $t("common.cancel") }}
                        </Button>
                        <Button :disabled="invitePreviewLoading" @click="doPreviewFromInput">
                          <Loader2Icon v-if="invitePreviewLoading" class="animate-spin" />
                          {{ $t("organization.invite.previewAction") }}
                        </Button>
                      </div>
                    </div>

                    <!-- 搜索可加入空间 -->
                    <div v-else-if="joinStep === 'search'" class="w-full">
                      <div class="mb-3">
                        <label for="org-join-search" :class="joinFormLabelClass">{{
                          $t("organization.join.searchSpaces")
                        }}</label>
                        <p :class="joinFormDescClass">{{ $t("organization.join.searchSpacesDesc") }}</p>
                        <div class="relative">
                          <SearchIcon
                            class="text-placeholder pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
                          />
                          <Input
                            id="org-join-search"
                            v-model="searchQuery"
                            :placeholder="$t('organization.join.searchSpacesPlaceholder')"
                            class="pr-8 pl-8"
                            @input="doSearchSearchableDebounced"
                            @keyup.enter="doSearchSearchable"
                          />
                          <button
                            v-if="searchQuery"
                            type="button"
                            :class="inputClearButtonClass"
                            :aria-label="$t('common.clear')"
                            @click="clearSearchQuery"
                          >
                            <CircleXIcon class="size-3.5" />
                          </button>
                        </div>
                      </div>
                      <!-- 搜索空间列表容器（与主列表一致：卡片间距） -->
                      <div
                        :class="[
                          scrollbarClass,
                          'border-border bg-card relative mb-4 max-h-[320px] min-h-[120px] overflow-y-auto rounded-[10px] border',
                        ]"
                      >
                        <div v-if="searchableList.length === 0 && !searchLoading" class="px-4 py-6">
                          <Empty class="p-0">
                            <EmptyDescription>{{
                              searchQuery
                                ? $t("organization.join.noSearchResult")
                                : $t("organization.join.noSearchableSpaces")
                            }}</EmptyDescription>
                          </Empty>
                        </div>
                        <div v-else class="flex flex-col">
                          <div
                            v-for="org in searchableList"
                            :key="org.id"
                            class="border-border flex items-center justify-between gap-3 border-b px-3.5 py-3 transition-colors duration-150 last:border-b-0"
                            :class="isOrgFull(org) ? 'cursor-default opacity-72' : 'hover:bg-accent cursor-pointer'"
                            @click="!isOrgFull(org) && previewSearchableOrg(org)"
                          >
                            <div class="flex min-w-0 flex-1 items-center gap-2.5">
                              <SpaceAvatar :name="org.name" :avatar="org.avatar" size="small" />
                              <div class="flex min-w-0 flex-col gap-0.5">
                                <span class="text-foreground truncate text-[14px] font-medium" :title="org.name">{{
                                  org.name
                                }}</span>
                                <span class="text-muted-foreground truncate text-[12px]">{{
                                  org.description || $t("organization.noDescription")
                                }}</span>
                              </div>
                            </div>
                            <div class="flex shrink-0 items-center gap-2">
                              <span class="text-muted-foreground inline-flex items-center gap-1 text-[12px]">
                                <UserIcon class="size-3" />
                                <template v-if="org.member_limit > 0"
                                  >{{ org.member_count }}/{{ org.member_limit }}</template
                                >
                                <template v-else>{{ org.member_count }}</template>
                              </span>
                              <span v-if="org.require_approval" :class="[lightTagClass, lightTagWarningClass]">
                                {{ $t("organization.invite.needApproval") }}
                              </span>
                              <span v-if="isOrgFull(org)" :class="[lightTagClass, 'bg-secondary text-foreground']">
                                {{ $t("organization.join.memberLimitReached") }}
                              </span>
                              <Button
                                v-if="!isOrgFull(org)"
                                variant="outline"
                                size="sm"
                                class="border-primary text-primary hover:text-primary"
                                @click.stop="previewSearchableOrg(org)"
                              >
                                {{ $t("organization.invite.previewAction") }}
                              </Button>
                            </div>
                          </div>
                        </div>
                        <!-- The old t-loading wrapper: a veil with a spinner over whatever the list shows. -->
                        <div
                          v-if="searchLoading"
                          class="bg-card/60 absolute inset-0 flex items-center justify-center"
                          aria-busy="true"
                        >
                          <Loader2Icon class="text-primary size-5 animate-spin" />
                        </div>
                      </div>
                      <div :class="[modalFooterClass, 'mt-1 px-0 pt-4']">
                        <Button variant="outline" @click="closeInvitePreview">
                          {{ $t("common.cancel") }}
                        </Button>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Loading -->
                <div
                  v-else-if="invitePreviewLoading"
                  :class="[
                    modalBodyClass,
                    scrollbarClass,
                    'flex flex-col items-center justify-center gap-5 px-7 py-16',
                  ]"
                >
                  <Loader2Icon class="text-primary size-8 animate-spin" />
                  <span class="text-muted-foreground font-[family-name:var(--app-font-family)] text-[14px]">{{
                    $t("organization.invite.loading")
                  }}</span>
                </div>

                <!-- 步骤2：空间详情预览 -->
                <div v-else-if="invitePreviewData" :class="[modalBodyClass, scrollbarClass, 'px-6 pt-2']">
                  <div class="flex flex-col items-center pt-2 pb-5 text-center">
                    <div class="mb-3">
                      <SpaceAvatar :name="invitePreviewData.name" :avatar="invitePreviewData.avatar" size="large" />
                    </div>
                    <h3 class="text-foreground m-0 mb-1.5 max-w-full truncate text-[18px] leading-[1.35] font-semibold">
                      {{ invitePreviewData.name }}
                    </h3>
                    <p class="text-muted-foreground m-0 mb-3.5 line-clamp-2 max-w-[360px] text-[13px] leading-normal">
                      {{ invitePreviewData.description || $t("organization.noDescription") }}
                    </p>
                    <div class="mb-3 flex items-center justify-center gap-1">
                      <Tooltip>
                        <TooltipTrigger as-child>
                          <div :class="[featureBadgeClass, featureBadgeMemberClass]">
                            <UserIcon class="size-3.5 shrink-0" />
                            <span class="leading-none">{{ invitePreviewData.member_count }}</span>
                          </div>
                        </TooltipTrigger>
                        <TooltipContent side="top" class="z-[2100]">{{
                          $t("organization.memberCount")
                        }}</TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger as-child>
                          <div :class="[featureBadgeClass, featureBadgeKbClass]">
                            <FolderIcon class="size-3.5 shrink-0" />
                            <span class="leading-none">{{ invitePreviewData.share_count }}</span>
                          </div>
                        </TooltipTrigger>
                        <TooltipContent side="top" class="z-[2100]">{{
                          $t("organization.invite.knowledgeBases")
                        }}</TooltipContent>
                      </Tooltip>
                    </div>
                    <button
                      type="button"
                      class="group/id bg-secondary text-placeholder hover:text-muted-foreground inline-flex max-w-full cursor-pointer items-center gap-1.5 rounded-full px-2.5 py-1 text-[12px] transition-colors duration-150 hover:bg-[color-mix(in_srgb,var(--td-brand-color)_8%,var(--td-bg-color-secondarycontainer))]"
                      @click="copyPreviewSpaceId"
                    >
                      <span>{{ $t("organization.join.spaceId") }}</span>
                      <code
                        class="text-muted-foreground group-hover/id:text-primary bg-transparent p-0 font-[family-name:var(--app-font-family-mono)] text-[11px]"
                        >{{ shortPreviewSpaceId }}</code
                      >
                      <CopyIcon class="text-placeholder group-hover/id:text-primary size-3.5 shrink-0" />
                    </button>
                  </div>

                  <div
                    v-if="invitePreviewData.is_already_member"
                    class="text-primary flex items-center justify-center gap-2 pt-3 pb-1 text-[14px] font-medium"
                  >
                    <CircleCheckIcon class="size-[18px]" />
                    <span>{{ $t("organization.invite.alreadyMember") }}</span>
                  </div>

                  <div v-else class="border-border border-t pt-4">
                    <div class="flex min-h-7 items-center justify-between gap-3">
                      <span class="text-foreground text-[14px] font-medium">{{
                        $t("organization.invite.approvalLabel")
                      }}</span>
                      <span
                        :class="[
                          lightTagClass,
                          invitePreviewData.require_approval ? lightTagWarningClass : lightTagSuccessClass,
                        ]"
                      >
                        {{
                          invitePreviewData.require_approval
                            ? $t("organization.invite.needApproval")
                            : $t("organization.invite.noApproval")
                        }}
                      </span>
                    </div>
                    <p
                      v-if="!invitePreviewData.require_approval"
                      class="text-muted-foreground mt-2 mb-0 text-[13px] leading-normal"
                    >
                      {{ $t("organization.invite.defaultRoleAfterJoin", { role: $t("organization.role.viewer") }) }}
                    </p>
                    <template v-else>
                      <p class="mt-2 mb-0 text-[13px] leading-normal text-[var(--td-warning-color-active)]">
                        {{ $t("organization.invite.requireApprovalTip") }}
                      </p>
                      <div class="border-border mt-3.5 flex flex-col gap-3 border-t border-dashed pt-3.5">
                        <div class="mb-3 last:mb-0">
                          <label :class="joinFormLabelClass">{{ $t("organization.invite.requestRole") }}</label>
                          <Select :model-value="inviteRequestRole" @update:model-value="setInviteRequestRole">
                            <SelectTrigger class="w-full">
                              <SelectValue :placeholder="$t('organization.invite.selectRole')" />
                            </SelectTrigger>
                            <SelectContent position="popper" class="z-[2100]">
                              <SelectItem v-for="opt in orgRoleOptions" :key="opt.value" :value="opt.value">
                                {{ opt.label }}
                              </SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                        <div class="mb-3 last:mb-0">
                          <label for="org-join-request-message" :class="joinFormLabelClass">{{
                            $t("organization.invite.applicationNote")
                          }}</label>
                          <!-- Grows with its content between two and four lines, as the old autosize did. -->
                          <Textarea
                            id="org-join-request-message"
                            v-model="inviteRequestMessage"
                            :placeholder="$t('organization.invite.messagePlaceholder')"
                            :maxlength="500"
                            class="max-h-[104px] min-h-[60px] resize-none overflow-y-auto"
                          />
                        </div>
                      </div>
                    </template>
                  </div>

                  <div :class="[modalFooterClass, '-mx-6 mt-4 px-6 pt-3']">
                    <Button variant="outline" @click="backFromPreview">
                      {{ !inviteCode ? $t("organization.join.backToSearch") : $t("common.cancel") }}
                    </Button>
                    <Button
                      v-if="!invitePreviewData.is_already_member"
                      :disabled="inviteJoining"
                      @click="confirmJoinOrganization"
                    >
                      <Loader2Icon v-if="inviteJoining" class="animate-spin" />
                      {{
                        invitePreviewData.require_approval
                          ? $t("organization.invite.submitRequest")
                          : $t("organization.invite.primaryJoin")
                      }}
                    </Button>
                    <Button v-else @click="viewOrganizationFromPreview">
                      {{ $t("organization.invite.viewOrganization") }}
                    </Button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, computed, watch, nextTick } from "vue";
import { useRoute, useRouter } from "vue-router";
import { MessagePlugin } from "tdesign-vue-next";
import { useOrganizationStore } from "@/stores/organization";
import { useAuthStore } from "@/stores/auth";
import type { Organization, OrganizationPreview, SearchableOrganizationItem } from "@/api/organization";
import { previewOrganization, submitJoinRequest } from "@/api/organization";
import { useI18n } from "vue-i18n";
import { copyWithToast } from "@/utils/clipboard";
import OrganizationSettingsModal from "./OrganizationSettingsModal.vue";
import SpaceAvatar from "@/components/SpaceAvatar.vue";
import ListSpaceSidebar from "@/components/ListSpaceSidebar.vue";
import { shouldShowOrgRelationTag } from "@/utils/card-list-badge";
import {
  ChevronDownIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  CircleAlertIcon,
  CircleCheckIcon,
  CircleXIcon,
  CopyIcon,
  FolderIcon,
  Loader2Icon,
  LogInIcon,
  LogOutIcon,
  SearchIcon,
  SettingsIcon,
  Trash2Icon,
  UserIcon,
  UserPlusIcon,
  UsersIcon,
  XIcon,
} from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Empty, EmptyDescription } from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

type OrgWithUI = Organization;

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const orgStore = useOrganizationStore();
const authStore = useAuthStore();

// 后端 /api/v1/organizations 下的写操作（创建、加入、申请加入、邀请、审批、改设置等）
// 在路由层都要求当前空间角色 ≥ admin。前端只用于 UI 渲染，安全边界仍在服务端。
const canManageOrg = computed(() => authStore.hasRole("admin") || authStore.canAccessAllTenants);
const noPermissionTip = computed(() => t("organization.rbac.needTenantAdminTip"));

// 申请加入时可选角色（仅需审核时使用）
const orgRoleOptions = [
  { label: t("organization.role.viewer"), value: "viewer" },
  { label: t("organization.role.editor"), value: "editor" },
  { label: t("organization.role.admin"), value: "admin" },
];
type OrgRequestRole = "viewer" | "editor" | "admin";
const inviteRequestRole = ref<OrgRequestRole>("viewer");
const inviteRequestMessage = ref("");

// The select reports an untyped value; only the three offered roles are accepted.
function setInviteRequestRole(value: unknown) {
  const match = orgRoleOptions.find((o) => o.value === value);
  if (match) inviteRequestRole.value = match.value as OrgRequestRole;
}

// ---- Shared class strings ------------------------------------------------
// Pieces of the look that the template repeats — the card grid, the card
// shell, the badges, the modal chrome — named once so the copies agree.

// One column, then more as the page widens, at the breakpoints the list
// always used. The grid fades up into place when it first renders.
const cardGridClass =
  "grid grid-cols-1 gap-3 animate-in fade-in slide-in-from-bottom-1.5 duration-320 ease-out " +
  "min-[900px]:grid-cols-2 min-[1250px]:grid-cols-3 min-[1600px]:grid-cols-4 min-[1900px]:grid-cols-5 " +
  "min-[2200px]:grid-cols-6";
const cardBaseClass =
  "relative box-border flex h-[136px] min-h-[136px] flex-col overflow-hidden rounded-lg border border-border " +
  "bg-card px-3.5 py-3 shadow-[0_1px_3px_rgba(0,0,0,0.04)]";
// A faint brand-coloured glow in the card's top-right corner.
const cardGlowClass =
  "before:pointer-events-none before:absolute before:top-0 before:right-0 before:z-0 before:h-20 before:w-[120px] " +
  "before:bg-[radial-gradient(ellipse_60%_50%_at_100%_0%,rgba(7,192,95,0.06)_0%,transparent_70%)] before:content-['']";
// 共享空间分组标题——与 KB / Agent 列表口径完全一致（图标 + 名称 + 数量 + 折叠 chevron）。
// The row itself only paints the sticky background; clicks land through its
// children, so a click in the blank space right of the title does not fold it.
const sectionHeaderClass =
  "group/section pointer-events-none sticky top-0 z-[5] col-span-full flex cursor-pointer items-center gap-1.5 " +
  "bg-card py-1.5 pr-1 pl-0 font-[family-name:var(--app-font-family)] text-[13px] leading-5 font-semibold " +
  "text-muted-foreground outline-none select-none hover:text-foreground *:pointer-events-auto " +
  "shadow-[0_-8px_0_0_var(--td-bg-color-container),0_4px_0_0_var(--td-bg-color-container)] " +
  "focus-visible:shadow-[0_0_0_2px_var(--td-brand-color-focus)]";
const sectionCountClass =
  "ml-0.5 rounded-lg bg-secondary px-1.5 text-[11px] leading-4 font-medium text-muted-foreground";
const sectionToggleClass = "ml-1 size-3.5 opacity-70 transition-opacity duration-150 group-hover/section:opacity-100";
// 与知识库卡片统一的底部标签：小尺寸、统一圆角
const featureBadgeClass =
  "inline-flex h-5 cursor-default items-center justify-center gap-[3px] rounded-[5px] px-[5px] " +
  "font-[family-name:var(--app-font-family)] text-[11px] font-medium transition-colors duration-200";
const featureBadgeMemberClass = "bg-[rgba(100,116,139,0.08)] text-muted-foreground hover:bg-[rgba(100,116,139,0.12)]";
const featureBadgeKbClass = "bg-[rgba(7,192,95,0.08)] text-primary hover:bg-[rgba(7,192,95,0.12)]";
const headerActionButtonClass =
  "size-7 rounded-md border border-border bg-secondary text-muted-foreground " +
  "shadow-[inset_0_1px_0_color-mix(in_srgb,var(--td-bg-color-container)_72%,transparent)] " +
  "hover:bg-secondary hover:text-foreground dark:hover:bg-secondary";
const cardMenuClass =
  "w-auto min-w-[148px] rounded-[10px] border-[0.5px] border-border p-1 ring-0 backdrop-blur-[20px] " +
  "backdrop-saturate-[180%] shadow-[0_0_0_0.5px_rgba(0,0,0,0.03),0_2px_4px_rgba(0,0,0,0.04),0_8px_24px_rgba(0,0,0,0.1)]";
const cardMenuItemClass = "cursor-pointer gap-2.5 px-3 py-2 text-[14px] leading-5";
// 删除/离开确认对话框：a compact card with no header bar or footer.
const confirmDialogClass = "gap-0 rounded-md p-4 sm:max-w-[464px]";
const confirmDialogTitleClass =
  "m-0 font-[family-name:var(--app-font-family)] text-[16px] leading-6 font-semibold text-foreground";
const confirmDialogTextClass =
  "mb-[21px] ml-[29px] inline-block font-[family-name:var(--app-font-family)] text-[14px] leading-[22px] text-placeholder";
const confirmDialogActionClass =
  "cursor-pointer font-[family-name:var(--app-font-family)] text-[14px] leading-[22px] text-foreground hover:opacity-80";
// The join / preview modal.
const modalIconButtonClass =
  "flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-lg text-muted-foreground " +
  "transition-colors duration-200 hover:bg-secondary";
const modalBodyClass = "min-h-0 flex-1 overflow-x-hidden overflow-y-auto max-h-[calc(90vh-120px)]";
const modalFooterClass = "flex shrink-0 justify-end gap-3 border-t border-border bg-card px-6 pt-3 pb-5";
// A thin scrollbar whose thumb turns brand-coloured under the pointer.
const scrollbarClass =
  "[&::-webkit-scrollbar]:w-1.5 [&::-webkit-scrollbar-track]:rounded-[3px] [&::-webkit-scrollbar-track]:bg-secondary " +
  "[&::-webkit-scrollbar-thumb]:rounded-[3px] [&::-webkit-scrollbar-thumb]:bg-[var(--td-bg-color-component-disabled)] " +
  "[&::-webkit-scrollbar-thumb:hover]:bg-primary";
const joinFormLabelClass = "mb-1 block text-[14px] font-medium text-foreground";
const joinFormDescClass = "m-0 mb-2.5 text-[13px] leading-normal text-muted-foreground";
const inputClearButtonClass =
  "absolute top-1/2 right-2.5 flex -translate-y-1/2 cursor-pointer text-placeholder hover:text-muted-foreground";
/** TDesign's small light tag. */
const lightTagClass =
  "inline-flex h-5 items-center rounded-[var(--td-radius-small)] px-1.5 text-[12px] whitespace-nowrap";
const lightTagWarningClass = "bg-[var(--td-warning-color-light)] text-warning";
const lightTagSuccessClass = "bg-[var(--td-success-color-light)] text-success";

function joinModePillClass(active: boolean): string {
  const base =
    "inline-flex cursor-pointer items-center rounded-md px-3.5 py-1.5 text-[13px] leading-[1.4] outline-none " +
    "transition-colors duration-150";
  return active
    ? `${base} bg-primary/12 font-medium text-primary`
    : `${base} bg-secondary text-muted-foreground hover:text-primary focus-visible:text-primary ` +
        "hover:bg-[color-mix(in_srgb,var(--td-brand-color)_8%,var(--td-bg-color-secondarycontainer))] " +
        "focus-visible:bg-[color-mix(in_srgb,var(--td-brand-color)_8%,var(--td-bg-color-secondarycontainer))]";
}

// 右下角：创建者/角色 合并标签 — owner, admin and editor read in the brand
// colour on different tints; viewer (and an unknown role) stays grey.
function relationTagClass(org: { is_owner?: boolean; my_role?: string }): string {
  if (org.is_owner) return "bg-[rgba(124,77,255,0.1)] text-primary";
  if (org.my_role === "admin") return "bg-[rgba(7,192,95,0.12)] text-primary";
  if (org.my_role === "editor") return "bg-[rgba(7,192,95,0.08)] text-primary";
  return "bg-[rgba(107,114,128,0.08)] text-muted-foreground";
}

// State
const showSettingsModal = ref(false);
const settingsOrgId = ref("");
const settingsMode = ref<"create" | "edit">("edit");
const deleteVisible = ref(false);
const leaveVisible = ref(false);
const deletingOrg = ref<Organization | null>(null);
const leavingOrg = ref<Organization | null>(null);

// 邀请预览相关状态（与邀请链接共用同一弹框）
const showInvitePreview = ref(false);
const invitePreviewLoading = ref(false);
const inviteJoining = ref(false);
const inviteCode = ref("");
const joinInputCode = ref(""); // 从菜单打开时输入的邀请码
const invitePreviewData = ref<OrganizationPreview | null>(null);
const invitePreviewError = ref("");

// 加入方式：邀请码 / 搜索空间
const joinStep = ref<"invite" | "search">("invite");
const searchQuery = ref("");
const searchableList = computed(() => orgStore.searchableOrganizations);
const searchLoading = ref(false);
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null;
// 搜索结果缓存：避免重复点击时重复请求导致高度跳动
const organizationMenuVisibility = reactive<Record<string, boolean>>({});

// Tab 内容容器 ref，用于高度过渡
const tabContentWrapperRef = ref<HTMLElement | null>(null);

// 加入弹框整体 body 高度过渡（输入邀请码 / 搜索空间 / 查看详情）
const inviteBodyInnerRef = ref<HTMLElement | null>(null);
const inviteBodyHeightPx = ref<number>(0);
let inviteBodyResizeObserver: ResizeObserver | null = null;

const inviteBodyWrapStyle = computed(() => {
  const px = inviteBodyHeightPx.value;
  if (px <= 0) return {};
  return { maxHeight: `${px}px`, minHeight: `${px}px` };
});

// 预览中空间 ID 的简短显示（前 8 位 + …）
const shortPreviewSpaceId = computed(() => {
  const id = invitePreviewData.value?.id;
  if (!id) return "";
  return id.length > 8 ? `${id.slice(0, 8)}…` : id;
});

// 根据当前 body 内容更新高度（用于过渡动画）
function updateInviteBodyHeight() {
  const el = inviteBodyInnerRef.value;
  if (!el || !showInvitePreview.value) return;
  const h = el.scrollHeight;
  // 避免把高度写成 0 导致闪缩，仅在得到有效高度时更新
  if (h > 0) inviteBodyHeightPx.value = h;
}

// 观察加入弹框 body 内容高度，用于步骤切换时的高度过渡动画
function setupInviteBodyResizeObserver() {
  if (inviteBodyResizeObserver) return;
  const el = inviteBodyInnerRef.value;
  if (!el || !showInvitePreview.value) return;
  inviteBodyResizeObserver = new ResizeObserver((entries) => {
    const entry = entries[0];
    if (!entry) return;
    const h = entry.contentRect.height;
    // 避免切换瞬间读到 0 导致闪缩
    if (h > 0 || inviteBodyHeightPx.value <= 0) inviteBodyHeightPx.value = h;
  });
  inviteBodyResizeObserver.observe(el);
  inviteBodyHeightPx.value = el.scrollHeight;
}

function teardownInviteBodyResizeObserver() {
  if (inviteBodyResizeObserver) {
    inviteBodyResizeObserver.disconnect();
    inviteBodyResizeObserver = null;
  }
  inviteBodyHeightPx.value = 0;
}

watch(
  [showInvitePreview, inviteBodyInnerRef],
  ([show, inner]) => {
    if (!show) {
      teardownInviteBodyResizeObserver();
      return;
    }
    if (inner) {
      nextTick(() => {
        setupInviteBodyResizeObserver();
      });
    }
  },
  { flush: "post" },
);

// 步骤切换时在布局完成后读取新内容高度，保证高度过渡动画可见
watch(
  [() => invitePreviewLoading.value, () => invitePreviewData.value],
  () => {
    if (!showInvitePreview.value || !inviteBodyInnerRef.value) return;
    nextTick(() => {
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          updateInviteBodyHeight();
        });
      });
    });
  },
  { flush: "post" },
);

// 更新容器高度的辅助函数
const updateTabContentHeight = () => {
  if (!tabContentWrapperRef.value) return;

  // 先移除固定高度，获取自然高度
  tabContentWrapperRef.value.style.height = "auto";
  const naturalHeight = tabContentWrapperRef.value.scrollHeight;

  // 设置固定高度以触发过渡
  tabContentWrapperRef.value.style.height = `${naturalHeight}px`;
};

// 监听 joinStep 变化，动态调整容器高度以实现平滑过渡
watch(
  joinStep,
  () => {
    if (!tabContentWrapperRef.value) return;

    // 先设置当前高度
    const currentHeight = tabContentWrapperRef.value.scrollHeight;
    tabContentWrapperRef.value.style.height = `${currentHeight}px`;

    // 等待下一帧，让新内容渲染
    requestAnimationFrame(() => {
      updateTabContentHeight();

      // 过渡完成后，移除固定高度，让容器自适应
      setTimeout(() => {
        if (tabContentWrapperRef.value) {
          tabContentWrapperRef.value.style.height = "auto";
        }
      }, 300); // 与 CSS transition 时长一致
    });
  },
  { flush: "post" },
);

// 监听搜索列表变化，更新高度
watch([searchableList, searchLoading], () => {
  if (joinStep.value === "search") {
    nextTick(() => {
      updateTabContentHeight();
    });
  }
});

// 监听菜单快捷操作事件
const handleOrganizationDialogEvent = ((event: CustomEvent<{ type: "create" | "join" }>) => {
  if (!canManageOrg.value) {
    MessagePlugin.warning(
      event.detail?.type === "create" ? t("organization.rbac.cannotCreate") : t("organization.rbac.cannotJoin"),
    );
    return;
  }
  if (event.detail?.type === "create") {
    // 创建组织使用 SettingsModal
    settingsOrgId.value = "";
    settingsMode.value = "create";
    showSettingsModal.value = true;
  } else if (event.detail?.type === "join") {
    // 加入组织使用与邀请链接相同的预览弹框，先显示输入邀请码步骤
    joinInputCode.value = "";
    inviteCode.value = "";
    invitePreviewData.value = null;
    invitePreviewError.value = "";
    invitePreviewLoading.value = false;
    joinStep.value = "invite";
    searchQuery.value = "";
    orgStore.clearSearchableOrganizations();
    // 注意：不清空缓存，保留搜索结果以便下次快速显示
    showInvitePreview.value = true;
  }
}) as EventListener;

// 左侧筛选：'all' | 'created' | 'joined'
const spaceSelection = ref<"all" | "created" | "joined">("all");

// Computed
const loading = computed(() => orgStore.loading);
const organizations = computed<OrgWithUI[]>(() => orgStore.organizations);

const createdCount = computed(() => organizations.value.filter((o) => o.is_owner).length);
const joinedCount = computed(() => organizations.value.filter((o) => !o.is_owner).length);

const filteredOrganizations = computed(() => {
  if (spaceSelection.value === "created") return organizations.value.filter((o) => o.is_owner);
  if (spaceSelection.value === "joined") return organizations.value.filter((o) => !o.is_owner);
  // 「全部」视图下把我创建的 owner 排在前面、我加入的排在后面，方便上面的
  // 分组标题在过渡处一次性打出来——和 KB / Agent 列表口径一致。
  return [...organizations.value].sort((a, b) => {
    if (a.is_owner === b.is_owner) return 0;
    return a.is_owner ? -1 : 1;
  });
});

type OrgSectionKey = "created" | "joined";
const collapsedOrgSections = ref<Set<OrgSectionKey>>(new Set());
const isOrgSectionCollapsed = (key: OrgSectionKey) => collapsedOrgSections.value.has(key);
const toggleOrgSection = (key: OrgSectionKey) => {
  const next = new Set(collapsedOrgSections.value);
  if (next.has(key)) next.delete(key);
  else next.add(key);
  collapsedOrgSections.value = next;
};
const orgSectionOf = (org: { is_owner?: boolean }): OrgSectionKey => (org.is_owner ? "created" : "joined");
const isOrgRowHidden = (org: { is_owner?: boolean }) =>
  spaceSelection.value === "all" && isOrgSectionCollapsed(orgSectionOf(org));
const orgSectionCounts = computed<Record<OrgSectionKey, number>>(() => {
  const c: Record<OrgSectionKey, number> = { created: 0, joined: 0 };
  filteredOrganizations.value.forEach((o) => {
    c[orgSectionOf(o)]++;
  });
  return c;
});

function showOrgRelationTag(org: { is_owner?: boolean; my_role?: string }): boolean {
  return shouldShowOrgRelationTag({
    spaceSelection: spaceSelection.value,
    isOwner: !!org.is_owner,
    myRole: org.my_role,
  });
}

const emptyStateTitle = computed(() => {
  if (spaceSelection.value === "created") return t("organization.emptyCreated");
  if (spaceSelection.value === "joined") return t("organization.emptyJoined");
  return t("organization.empty");
});

const emptyStateDesc = computed(() => {
  if (spaceSelection.value === "created") return t("organization.emptyCreatedDesc");
  if (spaceSelection.value === "joined") return t("organization.emptyJoinedDesc");
  return t("organization.emptyDesc");
});

// Methods

const onVisibleChange = (visible: boolean, org: OrgWithUI) => {
  if (!visible) {
    organizationMenuVisibility[org.id] = false;
  }
};

// The card menu is controlled through organizationMenuVisibility, which
// handleCardClick reads so that a click meant to dismiss the menu does not
// also open the space's settings.
function onOrgMenuOpenChange(org: OrgWithUI, open: boolean) {
  organizationMenuVisibility[org.id] = open;
  onVisibleChange(open, org);
}

// 创建组织
function handleCreateOrganization() {
  if (!canManageOrg.value) {
    MessagePlugin.warning(t("organization.rbac.cannotCreate"));
    return;
  }
  settingsOrgId.value = "";
  settingsMode.value = "create";
  showSettingsModal.value = true;
}

// 加入组织
function handleJoinOrganization() {
  if (!canManageOrg.value) {
    MessagePlugin.warning(t("organization.rbac.cannotJoin"));
    return;
  }
  joinInputCode.value = "";
  inviteCode.value = "";
  invitePreviewData.value = null;
  invitePreviewError.value = "";
  invitePreviewLoading.value = false;
  joinStep.value = "invite";
  searchQuery.value = "";
  orgStore.clearSearchableOrganizations();
  showInvitePreview.value = true;
}

function handleCardClick(org: OrgWithUI) {
  // 如果弹窗正在显示，不触发设置
  if (organizationMenuVisibility[org.id]) {
    return;
  }
  settingsOrgId.value = org.id;
  settingsMode.value = "edit";
  showSettingsModal.value = true;
}

function handleSettings(org: OrgWithUI) {
  organizationMenuVisibility[org.id] = false;
  settingsOrgId.value = org.id;
  settingsMode.value = "edit";
  showSettingsModal.value = true;
}

function handleLeave(org: OrgWithUI) {
  organizationMenuVisibility[org.id] = false;
  leavingOrg.value = org;
  leaveVisible.value = true;
}

async function confirmLeave() {
  if (!leavingOrg.value) return;
  const success = await orgStore.leave(leavingOrg.value.id);
  if (success) {
    MessagePlugin.success(t("organization.leaveSuccess"));
    leaveVisible.value = false;
    leavingOrg.value = null;
  } else {
    MessagePlugin.error(orgStore.error || t("organization.leaveFailed"));
  }
}

function handleDelete(org: OrgWithUI) {
  organizationMenuVisibility[org.id] = false;
  deletingOrg.value = org;
  deleteVisible.value = true;
}

async function confirmDelete() {
  if (!deletingOrg.value) return;
  if (!canManageOrg.value) {
    MessagePlugin.warning(t("organization.rbac.cannotManage"));
    return;
  }
  const success = await orgStore.remove(deletingOrg.value.id);
  if (success) {
    MessagePlugin.success(t("organization.deleteSuccess"));
    deleteVisible.value = false;
    deletingOrg.value = null;
  } else {
    MessagePlugin.error(orgStore.error || t("organization.deleteFailed"));
  }
}

// 处理邀请链接预览
async function handleInvitePreview(code: string) {
  inviteCode.value = code;
  invitePreviewLoading.value = true;
  invitePreviewError.value = "";
  invitePreviewData.value = null;
  showInvitePreview.value = true;

  try {
    const result = await previewOrganization(code);
    if (result.success && result.data) {
      invitePreviewData.value = result.data;
      // 如果已经是成员，显示提示
      if (result.data.is_already_member) {
        invitePreviewError.value = t("organization.invite.alreadyMember");
      }
    } else {
      invitePreviewError.value = result.message || t("organization.invite.invalidCode");
    }
  } catch (e: any) {
    invitePreviewError.value = e?.message || t("organization.invite.previewFailed");
  } finally {
    invitePreviewLoading.value = false;
  }
}

// 确认加入组织（区分直接加入 vs 需要审核，支持邀请码和搜索两种方式）
async function confirmJoinOrganization() {
  if (!invitePreviewData.value || invitePreviewData.value.is_already_member) return;
  if (!canManageOrg.value) {
    MessagePlugin.warning(t("organization.rbac.cannotJoin"));
    return;
  }

  // 如果是通过搜索加入的（没有邀请码），使用搜索加入逻辑
  if (!inviteCode.value && invitePreviewData.value.id) {
    await joinBySearchOrg();
    return;
  }

  // 原有逻辑：通过邀请码加入
  if (!inviteCode.value) return;

  inviteJoining.value = true;
  try {
    // 需要审核的情况：提交申请（带申请角色与可选说明）
    if (invitePreviewData.value.require_approval) {
      const result = await submitJoinRequest({
        invite_code: inviteCode.value,
        message: inviteRequestMessage.value?.trim() || undefined,
        role: inviteRequestRole.value,
      });
      if (result.success) {
        MessagePlugin.success(t("organization.invite.requestSubmitted"));
        showInvitePreview.value = false;
        inviteCode.value = "";
        invitePreviewData.value = null;
        // 清除 URL 中的 invite_code 参数
        router.replace({ path: route.path, query: {} });
      } else {
        MessagePlugin.error(result.message || t("organization.invite.requestFailed"));
      }
    } else {
      // 直接加入
      const result = await orgStore.join(inviteCode.value);
      if (result) {
        MessagePlugin.success(t("organization.invite.joinSuccess"));
        showInvitePreview.value = false;
        inviteCode.value = "";
        invitePreviewData.value = null;
        // 清除 URL 中的 invite_code 参数
        router.replace({ path: route.path, query: {} });
      } else {
        MessagePlugin.error(orgStore.error || t("organization.invite.joinFailed"));
      }
    }
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("organization.invite.joinFailed"));
  } finally {
    inviteJoining.value = false;
  }
}

// 从输入步骤点击「预览」：用输入的邀请码拉取预览
async function doPreviewFromInput() {
  const code = joinInputCode.value?.trim();
  if (!code) {
    MessagePlugin.warning(t("organization.inviteCodeRequired"));
    return;
  }
  invitePreviewError.value = "";
  await handleInvitePreview(code);
}

// 关闭邀请预览弹框
function closeInvitePreview() {
  showInvitePreview.value = false;
  inviteCode.value = "";
  joinInputCode.value = "";
  invitePreviewData.value = null;
  invitePreviewError.value = "";
  joinStep.value = "invite";
  searchQuery.value = "";
  orgStore.clearSearchableOrganizations();
  inviteRequestRole.value = "viewer";
  inviteRequestMessage.value = "";
  router.replace({ path: route.path, query: {} });
}

// 从预览详情返回：若来自搜索则回到搜索 Tab，否则回到步骤 1
function backFromPreview() {
  const fromSearch = !inviteCode.value;
  invitePreviewData.value = null;
  inviteRequestRole.value = "viewer";
  inviteRequestMessage.value = "";
  if (fromSearch) {
    joinStep.value = "search";
  }
}

// 搜索缓存由 Store 统一管理，切换标签时直接读取缓存或请求最新结果
function handleSearchTabClick() {
  joinStep.value = "search";
  void doSearchSearchable();
}

// 搜索可加入空间
async function doSearchSearchable() {
  const currentQuery = searchQuery.value.trim();

  searchLoading.value = true;
  try {
    await orgStore.fetchSearchableOrganizations(currentQuery, { limit: 20 });
  } catch (e) {
    orgStore.clearSearchableOrganizations();
  } finally {
    searchLoading.value = false;
  }
}

function doSearchSearchableDebounced() {
  if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
  searchDebounceTimer = setTimeout(() => doSearchSearchable(), 300);
}

// The clear button empties the query and lists every searchable space again,
// the way typing the query away would.
function clearSearchQuery() {
  searchQuery.value = "";
  doSearchSearchableDebounced();
}

// 空间是否已满（超过成员上限无法加入）
function isOrgFull(org: SearchableOrganizationItem): boolean {
  return org.member_limit > 0 && org.member_count >= org.member_limit;
}

// 预览搜索到的空间（转换为预览格式）
function previewSearchableOrg(org: SearchableOrganizationItem) {
  // 将 SearchableOrganizationItem 转换为 OrganizationPreview 格式
  invitePreviewData.value = {
    id: org.id,
    name: org.name,
    description: org.description,
    avatar: org.avatar,
    member_count: org.member_count,
    share_count: org.share_count,
    is_already_member: org.is_already_member,
    require_approval: org.require_approval,
    created_at: "", // 搜索列表中没有创建时间，使用空字符串
  };
  // 清空邀请码，因为这是通过搜索加入的
  inviteCode.value = "";
}

// 查看搜索到的空间（已是成员时，打开空间设置；不关闭加入弹窗，关闭设置后仍回到搜索）

// 从预览弹框中查看空间（已是成员时；不关闭加入弹窗，关闭设置后仍回到搜索）
function viewOrganizationFromPreview() {
  if (!invitePreviewData.value) return;
  settingsOrgId.value = invitePreviewData.value.id;
  settingsMode.value = "edit";
  showSettingsModal.value = true;
}

// 复制预览中的空间 ID
async function copyPreviewSpaceId() {
  await copyWithToast(invitePreviewData.value?.id, "common.copied");
}

// 从搜索列表加入空间（通过空间 ID，无需邀请码）- 在预览确认后调用
async function joinBySearchOrg() {
  if (!invitePreviewData.value || invitePreviewData.value.is_already_member) return;
  if (!canManageOrg.value) {
    MessagePlugin.warning(t("organization.rbac.cannotJoin"));
    return;
  }

  inviteJoining.value = true;
  try {
    // 如果需要审核，传递角色和消息；否则直接加入
    const message = invitePreviewData.value.require_approval
      ? inviteRequestMessage.value?.trim() || undefined
      : undefined;
    const role = invitePreviewData.value.require_approval ? inviteRequestRole.value : undefined;
    const result = await orgStore.joinById(invitePreviewData.value.id, message, role, {
      requiresApproval: invitePreviewData.value.require_approval,
    });
    if (result.success) {
      if (invitePreviewData.value.require_approval) {
        MessagePlugin.success(t("organization.invite.requestSubmitted"));
      } else {
        MessagePlugin.success(t("organization.invite.joinSuccess"));
      }
      showInvitePreview.value = false;
      invitePreviewData.value = null;
      orgStore.clearSearchableOrganizations();
      searchQuery.value = "";
      joinStep.value = "invite";
      inviteRequestRole.value = "viewer";
      inviteRequestMessage.value = "";
    } else {
      MessagePlugin.error(result.message || t("organization.invite.joinFailed"));
    }
  } catch (e: any) {
    MessagePlugin.error(e?.message || t("organization.invite.joinFailed"));
  } finally {
    inviteJoining.value = false;
  }
}

// Lifecycle
onMounted(async () => {
  void orgStore.fetchOrganizations();
  window.addEventListener("openOrganizationDialog", handleOrganizationDialogEvent);

  // 检查 URL 中是否有邀请码
  const code = route.query.invite_code as string;
  if (code) {
    await handleInvitePreview(code);
  }

  // 检查 URL 中是否有 orgId，如果有则打开空间设置
  const orgId = route.query.orgId as string;
  if (orgId) {
    settingsOrgId.value = orgId;
    settingsMode.value = "edit";
    showSettingsModal.value = true;
    // 清除 URL 中的 orgId 参数，避免刷新时重复打开
    const newQuery = { ...route.query };
    delete newQuery.orgId;
    router.replace({ path: route.path, query: newQuery });
  }
});

onUnmounted(() => {
  window.removeEventListener("openOrganizationDialog", handleOrganizationDialogEvent);
  teardownInviteBodyResizeObserver();
});
</script>
