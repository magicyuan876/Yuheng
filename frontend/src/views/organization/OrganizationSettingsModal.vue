<template>
  <Teleport to="body">
    <Transition name="modal">
      <!--
        A hand-built overlay rather than the Dialog component: the screen locks
        the page scroll itself (see lockBackgroundScroll), keeps one inner
        scroller that it resets on navigation, and closes on an overlay click
        but not on Esc. Every floating part opened from inside it (popovers,
        selects, tooltips) is raised to z-[3050], above this overlay's 2000.
      -->
      <div
        v-if="visible"
        class="settings-overlay fixed inset-0 z-[2000] flex items-center justify-center overscroll-none bg-black/50 backdrop-blur-[4px]"
        @click.self="handleClose"
      >
        <div
          class="settings-modal bg-card relative flex h-[85vh] max-h-[750px] w-[90vw] max-w-[1100px] flex-col overflow-hidden rounded-xl shadow-[0_8px_32px_rgba(0,0,0,0.12)]"
        >
          <!-- 关闭按钮 -->
          <button
            type="button"
            data-slot="close-button"
            class="text-muted-foreground hover:bg-accent hover:text-foreground absolute top-4 right-4 z-10 flex size-8 items-center justify-center rounded-md transition-all duration-200"
            :aria-label="$t('common.close')"
            @click="handleClose"
          >
            <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
              <path d="M15 5L5 15M5 5L15 15" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
            </svg>
          </button>

          <div class="flex h-full overflow-hidden">
            <!-- 左侧导航 -->
            <div
              class="border-border flex w-52 shrink-0 flex-col overflow-hidden border-r bg-[var(--td-bg-color-settings-modal)]"
            >
              <div class="border-border shrink-0 border-b px-3.5 pt-4 pb-3">
                <h2 class="text-foreground m-0 text-base font-semibold">{{ modalTitle }}</h2>
              </div>
              <div class="settings-nav min-h-0 flex-1 overflow-y-auto px-2 pt-2 pb-3">
                <template v-for="group in navGroups" :key="group.key">
                  <div
                    class="text-placeholder px-3.5 pt-1.5 pb-0.5 text-xs font-semibold tracking-[0.02em] not-first:pt-2 first:pt-0.5"
                  >
                    {{ group.label }}
                  </div>
                  <div
                    v-for="item in group.items"
                    :key="item.key"
                    class="mb-0.5 flex cursor-pointer items-center rounded-md px-3 py-1.5 text-sm transition-all duration-200 select-none"
                    :class="{
                      'bg-muted text-primary font-medium': currentSection === item.key,
                      'text-foreground hover:bg-accent': currentSection !== item.key,
                    }"
                    @click="currentSection = item.key"
                  >
                    <component :is="item.icon" class="mr-[9px] size-4 shrink-0" />
                    <span class="flex-1">{{ item.label }}</span>
                    <span
                      v-if="item.badge != null && (item.key === 'sharedKb' ? true : item.badge > 0)"
                      class="bg-muted text-muted-foreground ml-0.5 shrink-0 rounded-lg px-1.5 text-center text-[11px] leading-4 font-medium"
                      :class="{ 'min-w-5': item.key === 'sharedKb' }"
                      >{{ item.badge }}</span
                    >
                  </div>
                </template>
              </div>
            </div>

            <!-- 右侧内容区域 -->
            <div class="bg-card flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
              <div
                ref="contentWrapperRef"
                class="content-wrapper min-h-0 flex-1 scroll-pb-6 overflow-y-auto overscroll-contain px-10 pt-7 pb-12"
              >
                <!-- 组织管理员但空间角色不足，给出只读提示 -->
                <div
                  v-if="showTenantRoleHint"
                  class="mb-4 flex items-start gap-2 rounded-lg border border-[var(--td-warning-color-focus)] bg-[var(--td-warning-color-light)] px-3 py-2.5 text-[13px] leading-normal text-[var(--td-warning-color-active)]"
                >
                  <InfoIcon class="mt-0.5 size-4 shrink-0" />
                  <span>{{ $t("organization.rbac.needTenantAdminTip") }}</span>
                </div>
                <!-- 基本信息 -->
                <div v-show="currentSection === 'basic'" class="section w-full">
                  <div class="mb-5 w-full min-w-0">
                    <h2 :class="sectionTitleClass">{{ $t("organization.editor.basicTitle") }}</h2>
                    <p :class="sectionDescriptionClass">{{ $t("organization.editor.basicDesc") }}</p>
                  </div>

                  <div class="flex flex-col">
                    <!-- 空间名称与头像：一行展示，头像点击弹出 Emoji 选择 -->
                    <div :class="settingRowClass">
                      <div :class="settingInfoClass">
                        <label :class="settingLabelClass"
                          >{{ $t("organization.name") }} <span class="text-destructive ml-0.5">*</span></label
                        >
                        <p :class="settingDescClass">{{ $t("organization.editor.nameTip") }}</p>
                      </div>
                      <div :class="settingControlClass">
                        <div class="flex w-full items-center gap-3">
                          <Popover
                            :open="avatarPopoverVisible"
                            @update:open="(v: boolean) => (avatarPopoverVisible = isAdmin && v)"
                          >
                            <PopoverTrigger as-child>
                              <div
                                class="hover:bg-accent flex shrink-0 cursor-pointer flex-col items-center gap-1 rounded-xl p-1 transition-colors duration-200"
                              >
                                <SpaceAvatar :name="formData.name || '?'" :avatar="formData.avatar" size="medium" />
                                <span v-if="isAdmin" class="text-placeholder text-[11px] leading-[1.2]">{{
                                  $t("organization.avatar")
                                }}</span>
                              </div>
                            </PopoverTrigger>
                            <PopoverContent align="start" class="z-[3050] w-auto min-w-[260px] gap-0 p-3">
                              <p class="text-muted-foreground m-0 mb-2.5 text-xs leading-[1.4]">
                                {{ $t("organization.avatarPickerHint") }}
                              </p>
                              <div class="flex max-w-[280px] flex-wrap gap-1.5">
                                <button
                                  v-for="emoji in avatarEmojiOptions"
                                  :key="emoji"
                                  type="button"
                                  data-slot="emoji-button"
                                  class="bg-card flex size-9 items-center justify-center rounded-lg border text-lg transition-colors duration-200"
                                  :class="{
                                    'border-primary bg-primary/12': formData.avatar === 'emoji:' + emoji,
                                    'border-border hover:border-primary hover:bg-primary/6':
                                      formData.avatar !== 'emoji:' + emoji,
                                  }"
                                  @click="selectAvatarEmoji(emoji)"
                                >
                                  {{ emoji }}
                                </button>
                              </div>
                              <Button
                                v-if="formData.avatar"
                                variant="ghost"
                                size="sm"
                                class="text-muted-foreground mt-2.5 self-start text-xs hover:text-[var(--td-brand-color-active)]"
                                @click="clearAvatarEmoji"
                              >
                                {{ $t("organization.avatarClear") }}
                              </Button>
                            </PopoverContent>
                          </Popover>
                          <Input
                            v-model="formData.name"
                            :placeholder="$t('organization.namePlaceholder')"
                            :disabled="!isAdmin"
                            class="min-w-0 flex-1"
                          />
                        </div>
                      </div>
                    </div>

                    <!-- 空间描述 -->
                    <div :class="settingRowClass">
                      <div :class="settingInfoClass">
                        <label :class="settingLabelClass">{{ $t("organization.description") }}</label>
                        <p :class="settingDescClass">{{ $t("organization.editor.descriptionTip") }}</p>
                      </div>
                      <div :class="settingControlClass">
                        <!-- Grows with its content between three and six rows, as the old autosize did. -->
                        <Textarea
                          v-model="formData.description"
                          :placeholder="$t('organization.descriptionPlaceholder')"
                          :maxlength="500"
                          :disabled="!isAdmin"
                          class="max-h-[8.5rem] min-h-[4.75rem]"
                        />
                      </div>
                    </div>

                    <!-- 邀请成员 (仅管理员可见) -->
                    <div v-if="isAdmin && orgId" :class="cn(settingRowClass, 'flex-col gap-3')">
                      <div class="w-full min-w-0">
                        <label :class="settingLabelClass">{{ $t("organization.settings.inviteMembers") }}</label>
                        <p :class="settingDescClass">{{ $t("organization.settings.inviteMembersDesc") }}</p>
                      </div>
                      <div class="flex w-full min-w-0 items-start justify-start">
                        <div class="bg-muted border-border w-full rounded-[10px] border p-4">
                          <!-- 邀请码 -->
                          <div>
                            <div :class="inviteHeaderClass">
                              <QrCodeIcon :class="inviteIconClass" />
                              <span :class="inviteTitleClass">{{ $t("organization.inviteCode") }}</span>
                            </div>
                            <div
                              class="bg-card border-border flex items-center justify-between rounded-lg border px-3.5 py-2.5"
                            >
                              <span
                                class="text-primary font-(family-name:--app-font-family-mono) text-base font-semibold tracking-[2px]"
                                >{{ inviteCode }}</span
                              >
                              <div class="flex gap-1">
                                <Tooltip>
                                  <TooltipTrigger as-child>
                                    <Button
                                      variant="ghost"
                                      size="icon-sm"
                                      :aria-label="$t('common.copy')"
                                      @click="copyInviteCode"
                                    >
                                      <CopyIcon />
                                    </Button>
                                  </TooltipTrigger>
                                  <TooltipContent class="z-[3050]">{{ $t("common.copy") }}</TooltipContent>
                                </Tooltip>
                                <Tooltip>
                                  <TooltipTrigger as-child>
                                    <Button
                                      variant="ghost"
                                      size="icon-sm"
                                      :disabled="refreshingCode"
                                      :aria-label="$t('organization.refreshInviteCode')"
                                      @click="refreshInviteCode"
                                    >
                                      <Loader2Icon v-if="refreshingCode" class="animate-spin" />
                                      <RefreshCwIcon v-else />
                                    </Button>
                                  </TooltipTrigger>
                                  <TooltipContent class="z-[3050]">{{
                                    $t("organization.refreshInviteCode")
                                  }}</TooltipContent>
                                </Tooltip>
                              </div>
                            </div>
                            <p v-if="inviteCode" class="text-muted-foreground m-0 mt-2 text-xs">
                              {{ remainingValidityText }}
                            </p>
                          </div>

                          <div :class="inviteDividerClass"></div>

                          <!-- 邀请链接有效期 -->
                          <div>
                            <div :class="inviteHeaderClass">
                              <ClockIcon :class="inviteIconClass" />
                              <span :class="inviteTitleClass">{{
                                $t("organization.settings.inviteLinkValidity")
                              }}</span>
                            </div>
                            <p :class="inviteDescClass">{{ $t("organization.settings.inviteLinkValidityDesc") }}</p>
                            <Select
                              :model-value="String(formData.invite_code_validity_days)"
                              :disabled="!isAdmin"
                              @update:model-value="onValiditySelect"
                            >
                              <SelectTrigger size="sm" class="min-w-[140px]">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent class="z-[3050]">
                                <SelectItem
                                  v-for="opt in inviteValidityOptions"
                                  :key="opt.value"
                                  :value="String(opt.value)"
                                >
                                  {{ opt.label }}
                                </SelectItem>
                              </SelectContent>
                            </Select>
                          </div>

                          <div :class="inviteDividerClass"></div>

                          <!-- 邀请链接 -->
                          <div>
                            <div :class="inviteHeaderClass">
                              <LinkIcon :class="inviteIconClass" />
                              <span :class="inviteTitleClass">{{ $t("organization.settings.inviteLink") }}</span>
                            </div>
                            <div
                              class="bg-card border-border flex items-center justify-between gap-3 rounded-lg border px-3.5 py-2.5"
                            >
                              <span class="text-muted-foreground flex-1 text-xs leading-[1.4] break-all">{{
                                inviteLink
                              }}</span>
                              <Tooltip>
                                <TooltipTrigger as-child>
                                  <Button
                                    variant="ghost"
                                    size="icon-sm"
                                    :aria-label="$t('common.copy')"
                                    @click="copyInviteLink"
                                  >
                                    <CopyIcon />
                                  </Button>
                                </TooltipTrigger>
                                <TooltipContent class="z-[3050]">{{ $t("common.copy") }}</TooltipContent>
                              </Tooltip>
                            </div>
                          </div>

                          <div :class="inviteDividerClass"></div>

                          <!-- 需要审核开关 -->
                          <div>
                            <div :class="inviteHeaderClass">
                              <CircleCheckIcon :class="inviteIconClass" />
                              <span :class="inviteTitleClass">{{ $t("organization.settings.requireApproval") }}</span>
                            </div>
                            <div class="flex items-center gap-3">
                              <Switch v-model="formData.require_approval" @update:model-value="handleApprovalToggle" />
                              <span class="text-placeholder text-[13px]">{{
                                $t("organization.settings.requireApprovalDesc")
                              }}</span>
                            </div>
                          </div>

                          <div :class="inviteDividerClass"></div>

                          <!-- 开放可被搜索 -->
                          <div>
                            <div :class="inviteHeaderClass">
                              <SearchIcon :class="inviteIconClass" />
                              <span :class="inviteTitleClass">{{ $t("organization.settings.searchable") }}</span>
                            </div>
                            <div class="flex items-center gap-3">
                              <Switch v-model="formData.searchable" @update:model-value="handleSearchableToggle" />
                              <span class="text-placeholder text-[13px]">{{
                                $t("organization.settings.searchableDesc")
                              }}</span>
                            </div>
                          </div>

                          <div :class="inviteDividerClass"></div>

                          <!-- 成员人数上限 -->
                          <div>
                            <div :class="inviteHeaderClass">
                              <UserPlusIcon :class="inviteIconClass" />
                              <span :class="inviteTitleClass">{{ $t("organization.settings.memberLimit") }}</span>
                            </div>
                            <p :class="inviteDescClass">{{ $t("organization.settings.memberLimitDesc") }}</p>
                            <div class="mt-2 flex items-center gap-3">
                              <Input
                                type="number"
                                :model-value="formData.member_limit"
                                :min="0"
                                :max="10000"
                                :placeholder="$t('organization.settings.memberLimitPlaceholder')"
                                class="w-[140px]"
                                @update:model-value="onMemberLimitInput"
                                @blur="clampMemberLimit"
                              />
                              <span class="text-muted-foreground text-xs">{{
                                $t("organization.settings.memberLimitHint", {
                                  count: orgInfo?.member_count ?? 0,
                                })
                              }}</span>
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 创建空间 - 权限说明 -->
                <div v-if="isCreateMode" v-show="currentSection === 'permissions'" class="section w-full">
                  <div class="mb-5 w-full min-w-0">
                    <h2 :class="sectionTitleClass">{{ $t("organization.editor.permissionsTitle") }}</h2>
                    <p :class="sectionDescriptionClass">{{ $t("organization.editor.permissionsDesc") }}</p>
                  </div>

                  <div class="flex flex-col gap-4">
                    <div
                      v-for="card in permissionCards"
                      :key="card.role"
                      class="bg-muted border-border rounded-lg border p-4"
                    >
                      <div class="mb-3 flex items-center gap-3">
                        <div
                          class="text-primary-foreground flex size-10 items-center justify-center rounded-lg"
                          :class="{
                            'bg-linear-135 from-(--td-brand-color) to-(--td-brand-color-active)': card.role === 'admin',
                            'bg-linear-135 from-(--td-warning-color) to-(--td-warning-color-active)':
                              card.role === 'editor',
                            'bg-[var(--td-bg-color-component-disabled)]': card.role === 'viewer',
                          }"
                        >
                          <component :is="orgRoleIcon(card.role)" class="size-4" />
                        </div>
                        <div class="flex items-center gap-2">
                          <span class="text-foreground text-[15px] font-semibold">{{
                            $t(`organization.role.${card.role}`)
                          }}</span>
                          <span :class="tagClass(card.tone, 'dark')">{{ $t(card.access) }}</span>
                        </div>
                      </div>
                      <ul class="m-0 list-none p-0">
                        <li
                          v-for="perm in card.perms"
                          :key="perm.key"
                          class="text-muted-foreground flex items-center gap-2 py-1.5 text-[13px]"
                        >
                          <CheckIcon v-if="perm.has" class="text-primary size-3.5 shrink-0" />
                          <XIcon v-else class="text-destructive size-3.5 shrink-0" />{{
                            $t(`organization.editor.${perm.key}`)
                          }}
                        </li>
                      </ul>
                    </div>
                  </div>
                  <div
                    class="text-primary mt-5 flex items-start gap-2 rounded-lg bg-[var(--td-brand-color-light)] px-4 py-3 text-[13px] leading-5"
                  >
                    <InfoIcon class="mt-0.5 size-4 shrink-0" />
                    <span>{{ $t("organization.editor.ownerNote") }}</span>
                  </div>
                </div>

                <!-- 成员管理 -->
                <div v-show="currentSection === 'members'" class="section w-full">
                  <div class="mb-5 w-full min-w-0">
                    <div :class="sectionHeaderRowClass">
                      <div :class="sectionTitleWrapClass">
                        <h2 :class="[sectionTitleClass, 'leading-tight whitespace-nowrap']">
                          {{ $t("organization.manageMembers") }}
                        </h2>
                        <Popover v-model:open="membersPermHint.open">
                          <PopoverTrigger as-child>
                            <button
                              type="button"
                              data-slot="hint-trigger"
                              :class="hintTriggerClass"
                              :aria-label="$t('organization.editor.permissionsTitle')"
                              :title="$t('organization.settings.permissionsIconHint')"
                              @mouseenter="membersPermHint.show"
                              @mouseleave="membersPermHint.hide"
                            >
                              <InfoIcon class="size-4" />
                            </button>
                          </PopoverTrigger>
                          <PopoverContent
                            align="start"
                            :class="[
                              hintPopoverClass,
                              'max-h-[min(400px,65vh)] w-[min(520px,calc(100vw-24px))] max-w-[min(520px,calc(100vw-24px))]',
                            ]"
                            @open-auto-focus.prevent
                            @mouseenter="membersPermHint.show"
                            @mouseleave="membersPermHint.hide"
                          >
                            <div
                              class="max-h-[min(392px,calc(65vh-8px))] overflow-x-hidden overflow-y-auto px-3.5 py-3"
                            >
                              <div class="mb-2.5 flex flex-col gap-0.5">
                                <span class="text-foreground text-[13px] font-semibold">{{
                                  $t("organization.editor.permissionsTitle")
                                }}</span>
                                <span class="text-muted-foreground text-xs leading-[1.45]">{{
                                  $t("organization.editor.permissionsDesc")
                                }}</span>
                              </div>
                              <div class="grid grid-cols-2 gap-2 max-[480px]:grid-cols-1">
                                <div
                                  v-for="role in orgRoleMatrixOrder"
                                  :key="role"
                                  class="rounded-md border px-2.5 py-2"
                                  :class="{
                                    'border-primary bg-[var(--td-brand-color-light)]': orgInfo?.my_role === role,
                                    'border-border bg-card': orgInfo?.my_role !== role,
                                  }"
                                >
                                  <div class="text-foreground mb-1.5 flex items-center gap-1 text-xs font-semibold">
                                    <component :is="orgRoleIcon(role)" class="size-3" />
                                    <span>{{ $t(`organization.role.${role}`) }}</span>
                                    <span
                                      v-if="orgInfo?.my_role === role"
                                      class="text-primary ml-auto rounded-sm bg-[var(--td-brand-color-light)] px-[5px] py-px text-[10px] font-medium"
                                      >{{ $t("common.me") }}</span
                                    >
                                  </div>
                                  <div class="flex flex-col gap-[3px]">
                                    <span
                                      v-for="(perm, idx) in orgRoleMatrix[role]"
                                      :key="idx"
                                      class="flex items-start gap-1 text-[11px] leading-[1.35]"
                                      :class="{
                                        'text-muted-foreground': perm.has,
                                        'text-[var(--td-text-color-disabled)]': !perm.has,
                                      }"
                                    >
                                      <CheckIcon v-if="perm.has" class="text-primary mt-px size-3 shrink-0" />
                                      <XIcon v-else class="mt-px size-3 shrink-0" />
                                      {{ $t(`organization.editor.${perm.key}`) }}
                                    </span>
                                  </div>
                                </div>
                              </div>
                            </div>
                          </PopoverContent>
                        </Popover>
                      </div>
                    </div>
                    <p :class="sectionDescriptionClass">{{ $t("organization.settings.membersDesc") }}</p>
                  </div>

                  <div class="flex flex-col gap-2.5">
                    <div :class="listHeaderClass">
                      <div :class="listTitleWrapClass">
                        <span :class="listTitleClass">{{ $t("organization.members.listTitle") }}</span>
                        <span :class="countBadgeClass">{{ filteredMembers.length }}</span>
                      </div>
                      <div :class="listActionsClass">
                        <div :class="listSearchClass">
                          <SearchIcon :class="searchIconClass" />
                          <Input
                            v-model="memberSearchQuery"
                            :placeholder="$t('organization.members.searchPlaceholder')"
                            :class="searchInputClass"
                          />
                          <button
                            v-if="memberSearchQuery"
                            type="button"
                            data-slot="clear-button"
                            :class="clearButtonClass"
                            :aria-label="$t('common.clear')"
                            @click="memberSearchQuery = ''"
                          >
                            <CircleXIcon class="size-3.5" />
                          </button>
                        </div>
                        <Popover v-if="canRequestUpgrade" v-model:open="upgradePopupVisible">
                          <PopoverTrigger as-child>
                            <Button
                              variant="outline"
                              size="icon-sm"
                              class="shrink-0"
                              :disabled="hasPendingUpgrade"
                              :title="
                                hasPendingUpgrade
                                  ? $t('organization.upgrade.pending')
                                  : $t('organization.upgrade.requestUpgrade')
                              "
                              :aria-label="
                                hasPendingUpgrade
                                  ? $t('organization.upgrade.pending')
                                  : $t('organization.upgrade.requestUpgrade')
                              "
                            >
                              <ArrowUpIcon />
                            </Button>
                          </PopoverTrigger>
                          <PopoverContent align="end" :class="[actionPopoverClass, 'w-[min(360px,calc(100vw-32px))]']">
                            <div :class="cn(popupTitleClass, 'mb-2.5')">
                              {{ $t("organization.upgrade.dialogTitle") }}
                            </div>
                            <p :class="cn(popupTipClass, 'mb-3')">{{ $t("organization.upgrade.dialogDesc") }}</p>

                            <div
                              class="bg-muted border-border mb-3.5 flex items-center justify-between gap-3 rounded-lg border px-3 py-2.5"
                            >
                              <span class="text-muted-foreground text-[13px]">{{
                                $t("organization.upgrade.currentRole")
                              }}</span>
                              <span :class="tagClass(roleTone(orgInfo?.my_role || 'viewer'), 'light')">
                                {{ $t(`organization.role.${orgInfo?.my_role || "viewer"}`) }}
                              </span>
                            </div>

                            <div class="flex flex-col gap-3.5">
                              <div :class="popupFieldClass">
                                <label :class="popupFieldLabelClass">{{ $t("organization.upgrade.selectRole") }}</label>
                                <div class="flex w-full flex-wrap gap-2">
                                  <button
                                    v-for="opt in upgradeRoleOptions"
                                    :key="opt.value"
                                    type="button"
                                    data-slot="role-pill"
                                    class="hover:text-primary focus-visible:text-primary inline-flex items-center rounded-md px-3.5 py-1.5 text-[13px] leading-[1.4] transition-colors duration-150 outline-none"
                                    :class="{
                                      'bg-primary/12 text-primary font-medium':
                                        upgradeForm.requested_role === opt.value,
                                      'bg-muted text-muted-foreground hover:bg-[color-mix(in_srgb,var(--td-brand-color)_8%,var(--td-bg-color-secondarycontainer))] focus-visible:bg-[color-mix(in_srgb,var(--td-brand-color)_8%,var(--td-bg-color-secondarycontainer))]':
                                        upgradeForm.requested_role !== opt.value,
                                    }"
                                    @click="upgradeForm.requested_role = opt.value as 'editor' | 'admin'"
                                  >
                                    {{ opt.label }}
                                  </button>
                                </div>
                              </div>
                              <div :class="popupFieldClass">
                                <label :class="popupFieldLabelClass">{{ $t("organization.upgrade.reason") }}</label>
                                <!-- Grows between two and four rows, as the old autosize did. -->
                                <Textarea
                                  v-model="upgradeForm.message"
                                  :placeholder="$t('organization.upgrade.reasonPlaceholder')"
                                  :maxlength="500"
                                  class="max-h-24 min-h-14"
                                />
                              </div>
                            </div>

                            <div :class="popupFooterClass">
                              <Button
                                variant="outline"
                                :disabled="upgradeSubmitting"
                                @click="upgradePopupVisible = false"
                              >
                                {{ $t("common.cancel") }}
                              </Button>
                              <Button :disabled="upgradeSubmitting" @click="handleSubmitUpgrade">
                                <Loader2Icon v-if="upgradeSubmitting" class="animate-spin" />
                                {{ $t("organization.upgrade.submitBtn") }}
                              </Button>
                            </div>
                          </PopoverContent>
                        </Popover>
                        <Popover v-if="isAdmin" v-model:open="addMemberPopupVisible">
                          <PopoverTrigger as-child>
                            <Button
                              variant="outline"
                              size="icon-sm"
                              class="border-primary text-primary hover:text-primary shrink-0"
                              :title="$t('organization.addMember.button')"
                              :aria-label="$t('organization.addMember.button')"
                            >
                              <UserPlusIcon />
                            </Button>
                          </PopoverTrigger>
                          <PopoverContent align="end" :class="[actionPopoverClass, 'w-[min(400px,calc(100vw-32px))]']">
                            <div :class="cn(popupTitleClass, 'mb-3')">
                              {{ $t("organization.addMember.dialogTitle") }}
                            </div>
                            <p :class="cn(popupTipClass, 'mb-3.5')">{{ $t("organization.addMember.tipTenant") }}</p>
                            <form class="flex flex-col" @submit.prevent>
                              <div class="mb-3.5 flex flex-col">
                                <Label class="pb-1.5">{{ $t("organization.addMember.searchTenant") }}</Label>
                                <div class="w-full min-w-0">
                                  <!--
                                    A search-as-you-type picker, standing in for TDesign's
                                    remote-filterable select: the query goes to the server
                                    (debounced by handleTenantSearch), so the list is not
                                    filtered locally.
                                  -->
                                  <div class="relative">
                                    <Popover v-model:open="tenantPickerOpen">
                                      <PopoverTrigger as-child>
                                        <Button
                                          type="button"
                                          variant="outline"
                                          class="w-full justify-between pr-2 font-normal"
                                          :class="{ 'pr-8': selectedTenantId != null }"
                                        >
                                          <span
                                            class="truncate"
                                            :class="{ 'text-placeholder': selectedTenantId == null }"
                                          >
                                            {{
                                              selectedTenantId != null
                                                ? selectedTenantLabel
                                                : $t("organization.addMember.searchTenantPlaceholder")
                                            }}
                                          </span>
                                          <ChevronDownIcon
                                            v-if="selectedTenantId == null"
                                            class="text-muted-foreground"
                                          />
                                        </Button>
                                      </PopoverTrigger>
                                      <PopoverContent
                                        align="start"
                                        class="z-[3060] w-(--reka-popover-trigger-width) gap-1 p-1"
                                      >
                                        <div class="relative p-1">
                                          <SearchIcon
                                            class="text-placeholder absolute top-1/2 left-3.5 size-3.5 -translate-y-1/2"
                                          />
                                          <Input
                                            v-model="tenantQuery"
                                            autofocus
                                            :placeholder="$t('organization.addMember.searchTenantPlaceholder')"
                                            class="pl-7"
                                            @update:model-value="(v) => handleTenantSearch(String(v))"
                                          />
                                        </div>
                                        <div v-if="tenantSearchLoading" class="flex items-center justify-center py-3">
                                          <Loader2Icon class="text-muted-foreground size-4 animate-spin" />
                                        </div>
                                        <div
                                          v-else-if="tenantSearchOptions.length === 0"
                                          class="text-muted-foreground px-2 py-3 text-center text-xs"
                                        >
                                          {{ $t("organization.addMember.searchTenantHint") }}
                                        </div>
                                        <div v-else role="listbox" class="max-h-60 overflow-y-auto">
                                          <button
                                            v-for="opt in tenantSearchOptions"
                                            :key="opt.value"
                                            type="button"
                                            role="option"
                                            data-slot="tenant-option"
                                            :aria-selected="selectedTenantId === opt.value"
                                            class="hover:bg-accent flex w-full items-center justify-between gap-2 rounded-md px-2 py-1.5 text-left text-sm"
                                            :class="{ 'text-primary': selectedTenantId === opt.value }"
                                            @click="selectTenant(opt)"
                                          >
                                            <span class="truncate">{{ opt.label }}</span>
                                            <CheckIcon v-if="selectedTenantId === opt.value" class="size-4 shrink-0" />
                                          </button>
                                        </div>
                                      </PopoverContent>
                                    </Popover>
                                    <button
                                      v-if="selectedTenantId != null"
                                      type="button"
                                      data-slot="clear-button"
                                      class="text-placeholder hover:text-muted-foreground absolute top-1/2 right-2 flex -translate-y-1/2 items-center"
                                      :aria-label="$t('common.clear')"
                                      @click="clearSelectedTenant"
                                    >
                                      <CircleXIcon class="size-4" />
                                    </button>
                                  </div>
                                  <p class="text-placeholder m-0 mt-1.5 text-xs leading-[1.45]">
                                    {{ $t("organization.addMember.searchTenantHint") }}
                                  </p>
                                </div>
                              </div>
                              <div class="mb-1 flex flex-col">
                                <Label class="pb-1.5">{{ $t("organization.addMember.selectRole") }}</Label>
                                <Select
                                  :model-value="addMemberRole"
                                  @update:model-value="(v) => (addMemberRole = normalizeJoinRequestRole(String(v)))"
                                >
                                  <SelectTrigger class="w-full">
                                    <SelectValue :placeholder="$t('organization.addMember.selectRole')" />
                                  </SelectTrigger>
                                  <SelectContent class="z-[3060]">
                                    <SelectItem v-for="opt in addMemberRoleOptions" :key="opt.value" :value="opt.value">
                                      {{ opt.label }}
                                    </SelectItem>
                                  </SelectContent>
                                </Select>
                              </div>
                            </form>
                            <div :class="popupFooterClass">
                              <Button
                                variant="outline"
                                :disabled="addMemberSubmitting"
                                @click="addMemberPopupVisible = false"
                              >
                                {{ $t("common.cancel") }}
                              </Button>
                              <Button
                                :disabled="addMemberSubmitting || selectedTenantId == null"
                                @click="handleAddMember"
                              >
                                <Loader2Icon v-if="addMemberSubmitting" class="animate-spin" />
                                {{ $t("organization.addMember.confirmBtn") }}
                              </Button>
                            </div>
                          </PopoverContent>
                        </Popover>
                      </div>
                    </div>

                    <div v-if="membersLoading && members.length === 0" :class="loadingInlineClass">
                      <Loader2Icon class="text-primary size-4 animate-spin" />
                      <span>{{ $t("organization.members.loading") }}</span>
                    </div>
                    <Empty v-else-if="filteredMembers.length === 0" :class="emptyClass">
                      <EmptyMedia>
                        <InboxIcon :class="emptyIconClass" :stroke-width="1.25" />
                      </EmptyMedia>
                      <EmptyDescription>
                        {{
                          memberSearchQuery.trim()
                            ? $t("organization.members.emptySearch", { q: memberSearchQuery })
                            : $t("organization.noMembers")
                        }}
                      </EmptyDescription>
                    </Empty>
                    <!--
                      The tables use a fixed layout so long names truncate, as the old
                      columns' ellipsis did; the minimum width is the sum of the old
                      column widths and minimum widths, below which the shell scrolls
                      sideways rather than squeezing the flexible columns to nothing.
                    -->
                    <div v-else :class="tableShellClass">
                      <Table class="min-w-[534px] table-fixed">
                        <TableHeader>
                          <TableRow :class="headRowClass">
                            <TableHead :class="[headCellClass, 'min-w-[160px]']">
                              {{ $t("organization.members.columns.member") }}
                            </TableHead>
                            <TableHead :class="[headCellClass, 'w-[132px]']">
                              {{ $t("organization.members.columns.role") }}
                            </TableHead>
                            <TableHead :class="[headCellClass, 'w-[154px]']">
                              {{ $t("organization.members.columns.joinedAt") }}
                            </TableHead>
                            <TableHead v-if="isAdmin" :class="[headCellClass, 'w-[88px]']">
                              {{ $t("organization.members.columns.operations") }}
                            </TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          <TableRow v-for="row in filteredMembers" :key="row.id" :class="bodyRowClass">
                            <TableCell :class="bodyCellClass">
                              <div class="flex min-w-0 flex-col gap-0.5 py-0.5">
                                <span
                                  class="text-foreground inline-flex max-w-full items-center gap-1.5 overflow-hidden text-sm font-medium text-ellipsis whitespace-nowrap"
                                >
                                  {{ memberPrimaryLabel(row) }}
                                  <span
                                    v-if="isOwnerMember(row)"
                                    class="text-primary inline-flex h-4 shrink-0 items-center rounded-[3px] bg-[var(--td-brand-color-light)] px-[5px] text-[10px] font-medium"
                                    >{{ $t("organization.owner") }}</span
                                  >
                                  <span
                                    v-if="row.representative_user_id === authStore.currentUserId"
                                    class="bg-primary text-primary-foreground inline-flex h-4 shrink-0 items-center rounded-[3px] px-[5px] text-[10px] font-medium"
                                    >{{ $t("common.me") }}</span
                                  >
                                </span>
                                <span
                                  v-if="memberSecondaryLabel(row)"
                                  class="text-muted-foreground truncate text-xs leading-[1.35]"
                                  >{{ memberSecondaryLabel(row) }}</span
                                >
                              </div>
                            </TableCell>
                            <TableCell :class="bodyCellClass">
                              <div class="flex min-w-0 items-center">
                                <Select
                                  v-if="isAdmin && !isOwnerMember(row)"
                                  :model-value="row.role"
                                  @update:model-value="(val) => handleRoleChange(row, String(val))"
                                >
                                  <SelectTrigger size="sm" class="w-full">
                                    <SelectValue />
                                  </SelectTrigger>
                                  <SelectContent class="z-[3050]">
                                    <SelectItem v-for="opt in roleOptions" :key="opt.value" :value="opt.value">
                                      {{ opt.label }}
                                    </SelectItem>
                                  </SelectContent>
                                </Select>
                                <span v-else :class="tagClass(roleTone(row.role), 'dark')">
                                  {{ $t(`organization.role.${row.role}`) }}
                                </span>
                              </div>
                            </TableCell>
                            <TableCell :class="bodyCellClass">{{ formatDate(row.joined_at) }}</TableCell>
                            <TableCell v-if="isAdmin" :class="bodyCellClass">
                              <OrgConfirmPopover
                                v-if="!isOwnerMember(row)"
                                :content="
                                  $t('organization.detail.removeMemberConfirm', { name: memberPrimaryLabel(row) })
                                "
                                :tooltip="$t('organization.detail.removeMember')"
                                :confirm-text="$t('common.confirm')"
                                :cancel-text="$t('common.cancel')"
                                @confirm="confirmRemoveMember(row)"
                              >
                                <Button
                                  variant="ghost"
                                  size="icon-sm"
                                  :class="dangerIconButtonClass"
                                  :aria-label="$t('organization.detail.removeMember')"
                                >
                                  <UserXIcon />
                                </Button>
                              </OrgConfirmPopover>
                            </TableCell>
                          </TableRow>
                        </TableBody>
                      </Table>
                      <div v-if="membersLoading" :class="tableLoadingClass">
                        <Loader2Icon class="text-primary size-5 animate-spin" />
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 加入申请（待审核） -->
                <div v-show="currentSection === 'joinRequests'" class="section w-full">
                  <div class="mb-5 w-full min-w-0">
                    <h2 :class="sectionTitleClass">{{ $t("organization.settings.joinRequests") }}</h2>
                    <p :class="sectionDescriptionClass">{{ $t("organization.settings.joinRequestsDesc") }}</p>
                  </div>

                  <div class="flex flex-col gap-2.5">
                    <div :class="listHeaderClass">
                      <div :class="listTitleWrapClass">
                        <span :class="listTitleClass">{{ $t("organization.joinRequests.listTitle") }}</span>
                        <span :class="countBadgeClass">{{ filteredJoinRequests.length }}</span>
                      </div>
                      <div :class="listActionsClass">
                        <div :class="listSearchClass">
                          <SearchIcon :class="searchIconClass" />
                          <Input
                            v-model="joinRequestSearchQuery"
                            :placeholder="$t('organization.joinRequests.searchPlaceholder')"
                            :class="searchInputClass"
                          />
                          <button
                            v-if="joinRequestSearchQuery"
                            type="button"
                            data-slot="clear-button"
                            :class="clearButtonClass"
                            :aria-label="$t('common.clear')"
                            @click="joinRequestSearchQuery = ''"
                          >
                            <CircleXIcon class="size-3.5" />
                          </button>
                        </div>
                      </div>
                    </div>

                    <div v-if="joinRequestsLoading && joinRequests.length === 0" :class="loadingInlineClass">
                      <Loader2Icon class="text-primary size-4 animate-spin" />
                      <span>{{ $t("organization.joinRequests.loading") }}</span>
                    </div>
                    <Empty v-else-if="filteredJoinRequests.length === 0" :class="emptyClass">
                      <EmptyMedia>
                        <InboxIcon :class="emptyIconClass" :stroke-width="1.25" />
                      </EmptyMedia>
                      <EmptyDescription>
                        {{
                          joinRequestSearchQuery.trim()
                            ? $t("organization.joinRequests.emptySearch", { q: joinRequestSearchQuery })
                            : $t("organization.settings.noPendingRequests")
                        }}
                      </EmptyDescription>
                    </Empty>
                    <div v-else :class="tableShellClass">
                      <Table class="min-w-[750px] table-fixed">
                        <TableHeader>
                          <TableRow :class="headRowClass">
                            <TableHead :class="[headCellClass, 'min-w-[160px]']">
                              {{ $t("organization.joinRequests.columns.applicant") }}
                            </TableHead>
                            <TableHead :class="[headCellClass, 'w-[88px]']">
                              {{ $t("organization.joinRequests.columns.type") }}
                            </TableHead>
                            <TableHead :class="[headCellClass, 'w-[140px]']">
                              {{ $t("organization.joinRequests.columns.requestedRole") }}
                            </TableHead>
                            <TableHead :class="[headCellClass, 'min-w-[120px]']">
                              {{ $t("organization.joinRequests.columns.message") }}
                            </TableHead>
                            <TableHead :class="[headCellClass, 'w-[154px]']">
                              {{ $t("organization.joinRequests.columns.appliedAt") }}
                            </TableHead>
                            <TableHead :class="[headCellClass, 'w-[88px]']">
                              {{ $t("organization.members.columns.operations") }}
                            </TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          <TableRow v-for="row in filteredJoinRequests" :key="row.id" :class="bodyRowClass">
                            <TableCell :class="bodyCellClass">
                              <div class="flex min-w-0 flex-col gap-0.5 py-0.5">
                                <span class="text-foreground truncate text-sm font-medium">{{
                                  joinRequestApplicantLabel(row)
                                }}</span>
                                <span
                                  v-if="joinRequestApplicantSecondary(row)"
                                  class="text-muted-foreground truncate text-xs leading-[1.35]"
                                >
                                  {{ joinRequestApplicantSecondary(row) }}
                                </span>
                              </div>
                            </TableCell>
                            <TableCell :class="bodyCellClass">
                              <span :class="tagClass(row.request_type === 'upgrade' ? 'warning' : 'primary', 'light')">
                                {{
                                  row.request_type === "upgrade"
                                    ? $t("organization.joinRequests.typeUpgrade")
                                    : $t("organization.joinRequests.typeJoin")
                                }}
                              </span>
                            </TableCell>
                            <TableCell :class="bodyCellClass">
                              <span
                                v-if="row.request_type === 'upgrade' && row.prev_role"
                                class="text-muted-foreground inline-flex items-center gap-1 text-[13px]"
                              >
                                {{ roleLabel(row.prev_role) }}
                                <ArrowRightIcon class="text-placeholder size-3 shrink-0" />
                                {{ roleLabel(row.requested_role) }}
                              </span>
                              <span v-else :class="tagClass(roleTone(row.requested_role), 'light')">
                                {{ roleLabel(row.requested_role) }}
                              </span>
                            </TableCell>
                            <TableCell :class="bodyCellClass">
                              <span
                                class="text-muted-foreground block max-w-[220px] truncate text-[13px]"
                                :title="row.message || undefined"
                              >
                                {{ row.message || "—" }}
                              </span>
                            </TableCell>
                            <TableCell :class="bodyCellClass">{{ formatDate(row.created_at) }}</TableCell>
                            <TableCell :class="bodyCellClass">
                              <div class="inline-flex items-center gap-0.5">
                                <Popover
                                  :open="approvePopupRequestId === row.id"
                                  @update:open="(v: boolean) => handleApprovePopupVisibleChange(v, row)"
                                >
                                  <Tooltip>
                                    <TooltipTrigger as-child>
                                      <PopoverTrigger as-child>
                                        <Button
                                          variant="ghost"
                                          size="icon-sm"
                                          class="text-primary hover:text-primary hover:bg-primary/10"
                                          :disabled="reviewingRequestId === row.id"
                                          :aria-label="$t('organization.settings.approve')"
                                        >
                                          <Loader2Icon v-if="reviewingRequestId === row.id" class="animate-spin" />
                                          <CheckIcon v-else />
                                        </Button>
                                      </PopoverTrigger>
                                    </TooltipTrigger>
                                    <TooltipContent side="top" class="z-[3050]">{{
                                      $t("organization.settings.approve")
                                    }}</TooltipContent>
                                  </Tooltip>
                                  <PopoverContent
                                    side="left"
                                    align="start"
                                    :class="[actionPopoverClass, 'w-[min(360px,calc(100vw-32px))]']"
                                  >
                                    <div :class="cn(popupTitleClass, 'mb-2.5')">
                                      {{ $t("organization.joinRequests.approveTitle") }}
                                    </div>
                                    <p :class="cn(popupTipClass, 'mb-3')">
                                      {{
                                        $t("organization.joinRequests.approveDesc", {
                                          name: joinRequestApplicantLabel(row),
                                        })
                                      }}
                                    </p>
                                    <div :class="popupFieldClass">
                                      <label :class="popupFieldLabelClass">{{
                                        $t("organization.settings.assignRole")
                                      }}</label>
                                      <Select
                                        :model-value="approveAssignRole"
                                        @update:model-value="
                                          (v) => (approveAssignRole = normalizeJoinRequestRole(String(v)))
                                        "
                                      >
                                        <SelectTrigger class="w-full">
                                          <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent class="z-[3060]">
                                          <SelectItem v-for="opt in orgRoleOptions" :key="opt.value" :value="opt.value">
                                            {{ opt.label }}
                                          </SelectItem>
                                        </SelectContent>
                                      </Select>
                                    </div>
                                    <div :class="popupFooterClass">
                                      <Button
                                        variant="outline"
                                        :disabled="reviewingRequestId === row.id"
                                        @click="closeApprovePopup"
                                      >
                                        {{ $t("common.cancel") }}
                                      </Button>
                                      <Button
                                        :disabled="reviewingRequestId === row.id"
                                        @click="confirmApproveRequest(row)"
                                      >
                                        <Loader2Icon v-if="reviewingRequestId === row.id" class="animate-spin" />
                                        {{ $t("organization.settings.approve") }}
                                      </Button>
                                    </div>
                                  </PopoverContent>
                                </Popover>
                                <OrgConfirmPopover
                                  :content="$t('organization.joinRequests.rejectConfirm')"
                                  :tooltip="$t('organization.settings.reject')"
                                  :confirm-text="$t('organization.settings.reject')"
                                  :cancel-text="$t('common.cancel')"
                                  @confirm="handleRejectRequest(row)"
                                >
                                  <Button
                                    variant="ghost"
                                    size="icon-sm"
                                    :class="dangerIconButtonClass"
                                    :disabled="reviewingRequestId === row.id"
                                    :aria-label="$t('organization.settings.reject')"
                                  >
                                    <Loader2Icon v-if="reviewingRequestId === row.id" class="animate-spin" />
                                    <XIcon v-else />
                                  </Button>
                                </OrgConfirmPopover>
                              </div>
                            </TableCell>
                          </TableRow>
                        </TableBody>
                      </Table>
                      <div v-if="joinRequestsLoading" :class="tableLoadingClass">
                        <Loader2Icon class="text-primary size-5 animate-spin" />
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 共享知识库 -->
                <div v-show="currentSection === 'sharedKb'" class="section w-full">
                  <div class="mb-5 w-full min-w-0">
                    <div :class="sectionHeaderRowClass">
                      <div :class="sectionTitleWrapClass">
                        <h2 :class="[sectionTitleClass, 'leading-tight whitespace-nowrap']">
                          {{ $t("organization.share.sharedKnowledgeBase") }}
                        </h2>
                        <Popover v-model:open="shareCalcHint.open">
                          <PopoverTrigger as-child>
                            <button
                              type="button"
                              data-slot="hint-trigger"
                              :class="hintTriggerClass"
                              :aria-label="$t('organization.settings.permissionCalcFormula')"
                              :title="$t('organization.settings.permissionCalcFormula')"
                              @mouseenter="shareCalcHint.show"
                              @mouseleave="shareCalcHint.hide"
                            >
                              <InfoIcon class="size-4" />
                            </button>
                          </PopoverTrigger>
                          <PopoverContent
                            align="start"
                            :class="[
                              hintPopoverClass,
                              'max-h-[min(280px,65vh)] w-[min(400px,calc(100vw-24px))] max-w-[min(400px,calc(100vw-24px))]',
                            ]"
                            @open-auto-focus.prevent
                            @mouseenter="shareCalcHint.show"
                            @mouseleave="shareCalcHint.hide"
                          >
                            <div class="max-w-[360px] px-4 py-3.5">
                              <p class="text-foreground m-0 mb-1.5 text-sm leading-[1.35] font-semibold">
                                {{ $t("organization.settings.sharePermissionLabel") }}
                              </p>
                              <p class="text-muted-foreground m-0 text-[13px] leading-[1.55]">
                                {{ $t("organization.settings.permissionCalcTip") }}
                              </p>
                            </div>
                          </PopoverContent>
                        </Popover>
                      </div>
                    </div>
                    <p :class="sectionDescriptionClass">{{ $t("organization.settings.sharedDesc") }}</p>
                  </div>

                  <div class="flex flex-col gap-2.5">
                    <div :class="listHeaderClass">
                      <div :class="listTitleWrapClass">
                        <span :class="listTitleClass">{{ $t("organization.sharedResources.kbListTitle") }}</span>
                        <span :class="countBadgeClass">{{ sharedKnowledgeBases.length }}</span>
                      </div>
                    </div>

                    <div v-if="sharesLoading && sharedKnowledgeBases.length === 0" :class="loadingInlineClass">
                      <Loader2Icon class="text-primary size-4 animate-spin" />
                      <span>{{ $t("organization.sharedResources.loading") }}</span>
                    </div>
                    <Empty v-else-if="sharedKnowledgeBases.length === 0" :class="emptyClass">
                      <EmptyMedia>
                        <InboxIcon :class="emptyIconClass" :stroke-width="1.25" />
                      </EmptyMedia>
                      <div>
                        <p class="text-foreground m-0 mb-1 text-sm font-medium">
                          {{ $t("organization.settings.noSharedKB") }}
                        </p>
                        <p class="text-muted-foreground m-0 text-[13px]">
                          {{ $t("organization.settings.noSharedKBTip") }}
                        </p>
                      </div>
                    </Empty>
                    <div v-else :class="tableShellClass">
                      <Table class="min-w-[754px] table-fixed">
                        <TableHeader>
                          <TableRow :class="headRowClass">
                            <TableHead :class="[headCellClass, 'min-w-[180px]']">
                              {{ $t("organization.sharedResources.columns.name") }}
                            </TableHead>
                            <TableHead :class="[headCellClass, 'w-[120px]']">
                              {{ $t("organization.sharedResources.columns.sharedBy") }}
                            </TableHead>
                            <TableHead :class="[headCellClass, 'w-[154px]']">
                              {{ $t("organization.sharedResources.columns.sharedAt") }}
                            </TableHead>
                            <TableHead :class="[headCellClass, 'w-[108px]']">
                              {{ $t("organization.settings.sharePermissionLabel") }}
                            </TableHead>
                            <TableHead :class="[headCellClass, 'w-[96px]']">
                              {{ $t("organization.settings.myPermissionLabel") }}
                            </TableHead>
                            <TableHead :class="[headCellClass, isAdmin ? 'w-[96px]' : 'w-16']">
                              {{ $t("organization.members.columns.operations") }}
                            </TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          <TableRow v-for="row in sharedKnowledgeBases" :key="row.id" :class="bodyRowClass">
                            <TableCell :class="bodyCellClass">
                              <span
                                class="text-foreground block truncate text-sm font-medium"
                                :title="row.knowledge_base_name"
                                >{{ row.knowledge_base_name }}</span
                              >
                            </TableCell>
                            <TableCell :class="bodyCellClass">
                              <span class="text-muted-foreground block truncate text-[13px]">{{
                                row.shared_by_username || "—"
                              }}</span>
                            </TableCell>
                            <TableCell :class="bodyCellClass">{{ formatDate(row.created_at) }}</TableCell>
                            <TableCell :class="bodyCellClass">
                              <span :class="tagClass(roleTone(row.permission), 'light')">
                                {{ sharePermissionLabel(row.permission) }}
                              </span>
                            </TableCell>
                            <TableCell :class="bodyCellClass">
                              <span :class="tagClass(roleTone(row.my_permission ?? row.permission), 'light')">
                                {{ sharePermissionLabel(row.my_permission ?? row.permission) }}
                              </span>
                            </TableCell>
                            <TableCell :class="bodyCellClass">
                              <div class="inline-flex items-center gap-0.5">
                                <Tooltip>
                                  <TooltipTrigger as-child>
                                    <Button
                                      variant="ghost"
                                      size="icon-sm"
                                      class="text-primary hover:text-primary hover:bg-primary/10"
                                      :aria-label="$t('knowledgeList.detail.goToKb')"
                                      @click.stop="handleShareClick(row)"
                                    >
                                      <EyeIcon />
                                    </Button>
                                  </TooltipTrigger>
                                  <TooltipContent side="top" class="z-[3050]">{{
                                    $t("knowledgeList.detail.goToKb")
                                  }}</TooltipContent>
                                </Tooltip>
                                <OrgConfirmPopover
                                  v-if="isAdmin"
                                  :content="
                                    $t('organization.settings.removeShareConfirm', {
                                      name: row.knowledge_base_name || row.knowledge_base_id,
                                    })
                                  "
                                  :tooltip="$t('organization.settings.removeShareFromOrg')"
                                  :confirm-text="$t('common.confirm')"
                                  :cancel-text="$t('common.cancel')"
                                  @confirm="handleRemoveShare(row)"
                                >
                                  <Button
                                    variant="ghost"
                                    size="icon-sm"
                                    :class="dangerIconButtonClass"
                                    :aria-label="$t('organization.settings.removeShareFromOrg')"
                                  >
                                    <Trash2Icon />
                                  </Button>
                                </OrgConfirmPopover>
                              </div>
                            </TableCell>
                          </TableRow>
                        </TableBody>
                      </Table>
                      <div v-if="sharesLoading" :class="tableLoadingClass">
                        <Loader2Icon class="text-primary size-5 animate-spin" />
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 共享智能体 -->
              </div>

              <!-- 底部操作按钮 -->
              <div class="border-border bg-card flex shrink-0 justify-end gap-3 border-t px-10 py-3">
                <Button variant="outline" @click="handleClose">{{ $t("common.cancel") }}</Button>
                <Button v-if="isAdmin" :disabled="submitting" @click="handleSave">
                  <Loader2Icon v-if="submitting" class="animate-spin" />
                  {{ isCreateMode ? $t("common.create") : $t("common.save") }}
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, reactive, watch, nextTick, onBeforeUnmount, type Component } from "vue";
import { useRouter } from "vue-router";
import {
  ArrowRightIcon,
  ArrowUpIcon,
  CheckIcon,
  ChevronDownIcon,
  CircleCheckIcon,
  CircleXIcon,
  ClockIcon,
  CopyIcon,
  EyeIcon,
  FolderOpenIcon,
  InboxIcon,
  InfoIcon,
  LinkIcon,
  Loader2Icon,
  PencilIcon,
  QrCodeIcon,
  RefreshCwIcon,
  SearchIcon,
  ShieldUserIcon,
  Trash2Icon,
  UserIcon,
  UserPlusIcon,
  UserXIcon,
  XIcon,
} from "@lucide/vue";
import type { AcceptableValue } from "reka-ui";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import { copyWithToast } from "@/utils/clipboard";
import {
  getOrganization,
  listOrgShares,
  listJoinRequests,
  searchTenantsForInvite,
  type OrganizationMember,
  type KnowledgeBaseShare,
  type JoinRequestResponse,
  type TenantInviteCandidate,
} from "@/api/organization";
import { useOrganizationStore } from "@/stores/organization";
import { useAuthStore } from "@/stores/auth";
import SpaceAvatar from "@/components/SpaceAvatar.vue";
import { Button } from "@/components/ui/button";
import { Empty, EmptyDescription, EmptyMedia } from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import OrgConfirmPopover from "./OrgConfirmPopover.vue";

const router = useRouter();
const authStore = useAuthStore();
const { t } = useI18n();

const orgStore = useOrganizationStore();

interface Props {
  visible: boolean;
  orgId?: string;
  mode?: "view" | "edit" | "create";
}

const props = withDefaults(defineProps<Props>(), {
  mode: "view",
});

const emit = defineEmits<{
  (e: "update:visible", value: boolean): void;
  (e: "saved"): void;
}>();

// State
const currentSection = ref("basic");
const contentWrapperRef = ref<HTMLElement | null>(null);
const orgInfo = computed(() => orgStore.currentOrganization);
const members = computed(() => orgStore.currentMembers);
const sharedKnowledgeBases = ref<KnowledgeBaseShare[]>([]);
const joinRequests = ref<JoinRequestResponse[]>([]);
const joinRequestsLoading = ref(false);
const joinRequestSearchQuery = ref("");
const reviewingRequestId = ref<string | null>(null);
const sharesLoading = ref(false);
const membersLoading = ref(false);
const memberSearchQuery = ref("");
const submitting = ref(false);
const refreshingCode = ref(false);
const inviteCode = ref("");
const inviteCodeExpiresAt = ref<string | null>(null);
const upgradePopupVisible = ref(false);
const upgradeSubmitting = ref(false);
const hasPendingUpgrade = ref(false);
const upgradeForm = ref({
  requested_role: "editor" as "admin" | "editor" | "viewer",
  message: "",
});

// 添加成员（按空间邀请）相关状态。Plan 3 之后，邀请实际上是把
// 一整个空间拉进空间；这里的「搜索结果」是空间候选列表，每条带一个
// 代表用户用于展示。`selectedTenantId` 是真正提交给后端的 tenant_id。
const addMemberPopupVisible = ref(false);
const addMemberSubmitting = ref(false);
const tenantSearchLoading = ref(false);
const tenantSearchResults = ref<TenantInviteCandidate[]>([]);
const selectedTenantId = ref<number | null>(null);
const addMemberRole = ref<"admin" | "editor" | "viewer">("viewer");

const formData = ref({
  name: "",
  description: "",
  avatar: "" as string,
  require_approval: false,
  searchable: false,
  invite_code_validity_days: 7 as number,
  member_limit: 50 as number, // 0 = unlimited
});

// 空间头像可选 Emoji（方案三：Emoji 作为头像）
const avatarEmojiOptions = [
  "🚀",
  "📁",
  "👥",
  "🏢",
  "💡",
  "📚",
  "🌟",
  "🔧",
  "📌",
  "🎯",
  "📂",
  "🔒",
  "🌐",
  "⚡",
  "🎨",
  "📊",
  "🤝",
  "💼",
  "📧",
  "🏠",
  "🔑",
  "📈",
  "✨",
  "📋",
  "🌍",
  "💬",
  "🔔",
  "📦",
  "🎉",
  "🌈",
];
const avatarPopoverVisible = ref(false);

function selectAvatarEmoji(emoji: string) {
  formData.value.avatar = "emoji:" + emoji;
  avatarPopoverVisible.value = false;
}
function clearAvatarEmoji() {
  formData.value.avatar = "";
  avatarPopoverVisible.value = false;
}

// Computed
const isCreateMode = computed(() => props.mode === "create");
// 后端组织相关变更接口（保存设置、邀请、搜索用户、改/删成员、审核加入申请、
// 升级申请、刷新邀请码、移除共享等）在路由层都要求当前空间角色 ≥ admin（见
// internal/router/router.go 的 RegisterOrganizationRoutes）。跨空间超管可绕过。
// 因此前端任何"管理类"入口必须同时满足：组织内是 admin/owner ∩ 当前空间 admin+。
const hasTenantAdmin = computed(() => authStore.hasRole("admin") || authStore.canAccessAllTenants);
const isAdmin = computed(() => {
  if (isCreateMode.value) return hasTenantAdmin.value;
  const orgAdmin = orgInfo.value?.my_role === "admin" || orgInfo.value?.is_owner;
  return !!orgAdmin && hasTenantAdmin.value;
});

// 当用户在组织内是 admin/owner 但当前空间角色不足时，展示只读提示
const showTenantRoleHint = computed(() => {
  if (isCreateMode.value) return !hasTenantAdmin.value;
  const orgAdmin = orgInfo.value?.my_role === "admin" || orgInfo.value?.is_owner;
  return !!orgAdmin && !hasTenantAdmin.value;
});

// 是否可以申请权限升级（非管理员成员可申请；后端也要求空间 admin+）
const canRequestUpgrade = computed(() => {
  if (isCreateMode.value || !props.orgId) return false;
  const myRole = orgInfo.value?.my_role;
  if (!myRole || myRole === "admin") return false;
  return hasTenantAdmin.value;
});

// 可申请的角色选项（比当前角色高的角色）
const upgradeRoleOptions = computed(() => {
  const myRole = orgInfo.value?.my_role || "viewer";
  const options = [];
  if (myRole === "viewer") {
    options.push({ label: t("organization.role.editor"), value: "editor" });
    options.push({ label: t("organization.role.admin"), value: "admin" });
  } else if (myRole === "editor") {
    options.push({ label: t("organization.role.admin"), value: "admin" });
  }
  return options;
});

// 添加成员时可选的角色
const addMemberRoleOptions = computed(() => [
  { label: t("organization.role.viewer"), value: "viewer" },
  { label: t("organization.role.editor"), value: "editor" },
  { label: t("organization.role.admin"), value: "admin" },
]);

// 空间搜索结果选项。成员单位是空间，只按空间名搜索，因此直接展示空间名；
// 空间名缺失时回退到空间 ID。
const tenantSearchOptions = computed(() =>
  tenantSearchResults.value.map((c) => ({
    label: c.tenant_name || `tenant#${c.tenant_id}`,
    value: c.tenant_id,
  })),
);

const modalTitle = computed(() => {
  if (isCreateMode.value) return t("organization.createOrg");
  return t("organization.settings.editTitle");
});

const navItems = computed(() => {
  const items: { key: string; icon: Component; label: string; badge?: number }[] = [
    { key: "basic", icon: InfoIcon, label: t("organization.editor.navBasic") },
  ];
  if (isCreateMode.value) {
    items.push({ key: "permissions", icon: ShieldUserIcon, label: t("organization.editor.navPermissions") });
  }
  // 只有在编辑已有组织时才显示成员管理、加入申请（仅管理员）、共享知识库
  if (props.orgId && !isCreateMode.value) {
    items.push({ key: "members", icon: UserIcon, label: t("organization.manageMembers") });
    if (isAdmin.value) {
      const pendingCount = orgInfo.value?.pending_join_request_count ?? 0;
      items.push({
        key: "joinRequests",
        icon: UserPlusIcon,
        label: t("organization.settings.joinRequests"),
        badge: pendingCount > 0 ? pendingCount : undefined,
      });
    }
    items.push({
      key: "sharedKb",
      icon: FolderOpenIcon,
      label: t("organization.share.sharedKnowledgeBase"),
      badge: sharedKnowledgeBases.value.length,
    });
  }
  return items;
});

const navGroups = computed(() => {
  const itemMap = new Map(navItems.value.map((item) => [item.key, item]));
  const pickItems = (keys: string[]) => keys.map((key) => itemMap.get(key)).filter(Boolean) as typeof navItems.value;
  if (isCreateMode.value) {
    return [
      {
        key: "basic",
        label: t("organization.navGroups.basic"),
        items: pickItems(["basic", "permissions"]),
      },
    ].filter((group) => group.items.length > 0);
  }
  return [
    {
      key: "basic",
      label: t("organization.navGroups.basic"),
      items: pickItems(["basic"]),
    },
    {
      key: "management",
      label: t("organization.navGroups.management"),
      items: pickItems(["members", "joinRequests"]),
    },
    {
      key: "resources",
      label: t("organization.navGroups.resources"),
      items: pickItems(["sharedKb"]),
    },
  ].filter((group) => group.items.length > 0);
});

const roleOptions = computed(() => [
  { label: t("organization.role.admin"), value: "admin" },
  { label: t("organization.role.editor"), value: "editor" },
  { label: t("organization.role.viewer"), value: "viewer" },
]);

type OrgRole = "admin" | "editor" | "viewer";
type OrgRolePerm = { key: string; has: boolean };

const orgRoleMatrixOrder: OrgRole[] = ["admin", "editor", "viewer"];

const orgRoleMatrix: Record<OrgRole, OrgRolePerm[]> = {
  admin: [
    { key: "viewerPerm1", has: true },
    { key: "editorPerm1", has: true },
    { key: "shareKBPerm", has: true },
    { key: "adminPerm1", has: true },
  ],
  editor: [
    { key: "viewerPerm1", has: true },
    { key: "editorPerm1", has: true },
    { key: "shareKBPerm", has: false },
    { key: "adminPerm1", has: false },
  ],
  viewer: [
    { key: "viewerPerm1", has: true },
    { key: "editorPerm1", has: false },
    { key: "shareKBPerm", has: false },
    { key: "adminPerm1", has: false },
  ],
};

function orgRoleIcon(role: OrgRole): Component {
  switch (role) {
    case "admin":
      return ShieldUserIcon;
    case "editor":
      return PencilIcon;
    default:
      return EyeIcon;
  }
}

// The three role cards shown while creating a space. Each lists what the
// role can (true) and cannot (false) do, in the order the old markup did.
const permissionCards: {
  role: OrgRole;
  tone: TagTone;
  access: string;
  perms: OrgRolePerm[];
}[] = [
  {
    role: "admin",
    tone: "primary",
    access: "organization.editor.fullAccess",
    perms: [
      { key: "adminPerm1", has: true },
      { key: "adminPerm2", has: true },
      { key: "adminPerm3", has: true },
      { key: "adminPerm4", has: true },
    ],
  },
  {
    role: "editor",
    tone: "warning",
    access: "organization.editor.editAccess",
    perms: [
      { key: "editorPerm1", has: true },
      { key: "editorPerm2", has: true },
      { key: "shareKBPerm", has: false },
      { key: "editorPerm3", has: false },
    ],
  },
  {
    role: "viewer",
    tone: "default",
    access: "organization.editor.viewAccess",
    perms: [
      { key: "viewerPerm1", has: true },
      { key: "shareKBPerm", has: false },
      { key: "viewerPerm2", has: false },
      { key: "viewerPerm3", has: false },
    ],
  },
];

function sharePermissionLabel(permission: string): string {
  if (permission === "editor" || permission === "admin") {
    return t("organization.share.permissionEditable");
  }
  return t("organization.share.permissionReadonly");
}

const filteredMembers = computed(() => {
  const query = memberSearchQuery.value.toLowerCase();
  if (!query) return members.value;
  return members.value.filter(
    (m) =>
      (m.tenant_name || "").toLowerCase().includes(query) ||
      (m.username || "").toLowerCase().includes(query) ||
      (m.email || "").toLowerCase().includes(query),
  );
});

const filteredJoinRequests = computed(() => {
  const query = joinRequestSearchQuery.value.trim().toLowerCase();
  if (!query) return joinRequests.value;
  return joinRequests.value.filter((req) => {
    const haystack = [req.username, req.email, req.user_id, req.message].filter(Boolean).join(" ").toLowerCase();
    return haystack.includes(query);
  });
});

function joinRequestApplicantLabel(req: JoinRequestResponse): string {
  return req.username || req.email || req.user_id;
}

function joinRequestApplicantSecondary(req: JoinRequestResponse): string {
  const primary = joinRequestApplicantLabel(req);
  if (req.email && req.email !== primary) return req.email;
  return "";
}

// 成员行的主标题：优先展示「空间名」，回退到代表用户名 / 空间 ID。Plan 3
// 之后每一行成员都对应一个空间，UI 必须先于代表用户呈现空间身份，
// 否则用户会误以为这是按"人"加进来的。
const memberPrimaryLabel = (m: OrganizationMember): string => {
  return m.tenant_name || m.username || `tenant#${m.tenant_id}`;
};

// 副标题：主标题展示的是空间名时，副标题展示代表用户名；如果主标题已经
// 是用户名（无 tenant_name 时的回退），副标题留空，避免重复信息。
// 邮箱在空间成员列表里没什么用（不是邀请人需要联系的对象），不展示。
const memberSecondaryLabel = (m: OrganizationMember): string => {
  if (m.tenant_name && m.username) {
    return m.username;
  }
  return "";
};

// Owner identification is workspace-keyed: the org's owner_tenant_id names
// the row in the per-workspace members list that represents the owner.
const isOwnerMember = (member: OrganizationMember): boolean => member.tenant_id === orgInfo.value?.owner_tenant_id;

const inviteLink = computed(() => {
  if (!inviteCode.value) return "";
  return `${window.location.origin}/join?code=${inviteCode.value}`;
});

const inviteValidityOptions = computed(() => [
  { label: t("organization.settings.validity1Day"), value: 1 },
  { label: t("organization.settings.validity7Days"), value: 7 },
  { label: t("organization.settings.validity30Days"), value: 30 },
  { label: t("organization.settings.validityNever"), value: 0 },
]);

const remainingValidityText = computed(() => {
  const at = inviteCodeExpiresAt.value;
  if (!at) return t("organization.settings.remainingValidityNever");
  const exp = new Date(at);
  const now = new Date();
  if (exp.getTime() <= now.getTime()) return t("organization.settings.remainingValidityExpired");
  const days = Math.ceil((exp.getTime() - now.getTime()) / (24 * 60 * 60 * 1000));
  return t("organization.settings.remainingValidity", { n: days });
});

// Methods
const handleClose = () => {
  // Blur before unmount so focus is not left on a node that is about to be detached.
  if (document.activeElement instanceof HTMLElement) {
    document.activeElement.blur();
  }
  emit("update:visible", false);
};

const fetchOrgDetail = async () => {
  if (!props.orgId) return;
  try {
    const res = await getOrganization(props.orgId);
    if (!props.visible) return;
    if (res.success && res.data) {
      orgStore.setCurrentOrganization(res.data);
      const validity = res.data.invite_code_validity_days;
      const memberLimit = res.data.member_limit;
      formData.value = {
        name: res.data.name,
        description: res.data.description || "",
        avatar: res.data.avatar || "",
        require_approval: res.data.require_approval || false,
        searchable: res.data.searchable || false,
        invite_code_validity_days: typeof validity === "number" ? validity : 7,
        member_limit: typeof memberLimit === "number" && memberLimit >= 0 ? memberLimit : 50,
      };
      inviteCode.value = res.data.invite_code || "";
      inviteCodeExpiresAt.value = res.data.invite_code_expires_at ?? null;
      // 初始化是否有待处理的升级申请
      hasPendingUpgrade.value = res.data.has_pending_upgrade || false;
    }
  } catch (error) {
    console.error("Failed to fetch org:", error);
  }
};

const fetchMembers = async () => {
  if (!props.orgId) return;
  membersLoading.value = true;
  try {
    await orgStore.fetchMembers(props.orgId);
  } catch (error) {
    console.error("Failed to fetch members:", error);
  } finally {
    membersLoading.value = false;
  }
};

const fetchSharedKBs = async () => {
  if (!props.orgId) return;
  sharesLoading.value = true;
  try {
    const kbRes = await listOrgShares(props.orgId);
    if (kbRes.success && kbRes.data) {
      sharedKnowledgeBases.value = kbRes.data.shares || [];
    } else {
      sharedKnowledgeBases.value = [];
    }
  } catch (error) {
    console.error("Failed to fetch shared resources:", error);
    sharedKnowledgeBases.value = [];
  } finally {
    sharesLoading.value = false;
  }
};

const orgRoleOptions = [
  { label: t("organization.role.viewer"), value: "viewer" },
  { label: t("organization.role.editor"), value: "editor" },
  { label: t("organization.role.admin"), value: "admin" },
];
const approvePopupRequestId = ref<string | null>(null);
const approveAssignRole = ref<"viewer" | "editor" | "admin">("viewer");

function normalizeJoinRequestRole(role: string): "viewer" | "editor" | "admin" {
  if (role === "admin" || role === "editor" || role === "viewer") return role;
  return "viewer";
}

function openApprovePopup(req: JoinRequestResponse) {
  approvePopupRequestId.value = req.id;
  approveAssignRole.value = normalizeJoinRequestRole(req.requested_role);
}

function closeApprovePopup() {
  approvePopupRequestId.value = null;
}

function handleApprovePopupVisibleChange(visible: boolean, req: JoinRequestResponse) {
  if (visible) {
    openApprovePopup(req);
    return;
  }
  if (approvePopupRequestId.value === req.id) {
    closeApprovePopup();
  }
}

function roleLabel(role: string) {
  if (role === "admin") return t("organization.role.admin");
  if (role === "editor") return t("organization.role.editor");
  return t("organization.role.viewer");
}

const fetchJoinRequests = async () => {
  if (!props.orgId) return;
  joinRequestsLoading.value = true;
  try {
    const res = await listJoinRequests(props.orgId);
    if (res.success && res.data) {
      joinRequests.value = res.data.requests || [];
    } else {
      joinRequests.value = [];
    }
  } catch (error) {
    console.error("Failed to fetch join requests:", error);
    joinRequests.value = [];
  } finally {
    joinRequestsLoading.value = false;
  }
};

/**
 * 审批结果会同时影响设置弹窗、空间卡片和全局侧栏中的待审批数量。
 * 后两处读取的是 organization store，因此必须绕过列表缓存并同步最新计数。
 */
const refreshOrganizationAfterReview = async () => {
  await Promise.all([fetchOrgDetail(), fetchMembers()]);
};

const confirmApproveRequest = async (req: JoinRequestResponse) => {
  const success = await handleApproveRequest(req, approveAssignRole.value);
  if (success) closeApprovePopup();
};

const handleApproveRequest = async (
  req: JoinRequestResponse,
  assignRole: "viewer" | "editor" | "admin",
): Promise<boolean> => {
  if (!props.orgId) return false;
  reviewingRequestId.value = req.id;
  try {
    const res = await orgStore.reviewOrganizationJoinRequest(
      props.orgId,
      req.id,
      { approved: true, role: assignRole },
      { requestType: req.request_type },
    );
    if (res.success) {
      MessagePlugin.success(t("organization.settings.approveSuccess"));
      joinRequests.value = joinRequests.value.filter((r) => r.id !== req.id);
      await refreshOrganizationAfterReview();
      return true;
    }
    MessagePlugin.error(res.message || t("organization.settings.reviewFailed"));
    return false;
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("organization.settings.reviewFailed"));
    return false;
  } finally {
    reviewingRequestId.value = null;
  }
};

const handleRejectRequest = async (req: JoinRequestResponse) => {
  if (!props.orgId) return;
  reviewingRequestId.value = req.id;
  try {
    const res = await orgStore.reviewOrganizationJoinRequest(
      props.orgId,
      req.id,
      { approved: false },
      { requestType: req.request_type },
    );
    if (res.success) {
      MessagePlugin.success(t("organization.settings.rejectSuccess"));
      joinRequests.value = joinRequests.value.filter((r) => r.id !== req.id);
      await refreshOrganizationAfterReview();
    } else {
      MessagePlugin.error(res.message || t("organization.settings.reviewFailed"));
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("organization.settings.reviewFailed"));
  } finally {
    reviewingRequestId.value = null;
  }
};

const handleSave = async () => {
  if (!formData.value.name.trim()) {
    MessagePlugin.warning(t("organization.nameRequired"));
    currentSection.value = "basic";
    return;
  }

  submitting.value = true;
  try {
    if (isCreateMode.value) {
      // 创建模式
      const result = await orgStore.create(
        formData.value.name.trim(),
        formData.value.description.trim(),
        formData.value.avatar || undefined,
      );
      if (result) {
        MessagePlugin.success(t("organization.createSuccess"));
        emit("saved");
        handleClose();
      } else {
        MessagePlugin.error(orgStore.error || t("organization.createFailed"));
      }
    } else {
      // 编辑模式
      if (!props.orgId) return;
      const result = await orgStore.updateOrganization(props.orgId, {
        name: formData.value.name.trim(),
        description: formData.value.description.trim(),
        avatar: formData.value.avatar || undefined,
        require_approval: formData.value.require_approval,
        searchable: formData.value.searchable,
        invite_code_validity_days: formData.value.invite_code_validity_days,
        member_limit: formData.value.member_limit,
      });
      if (result) {
        MessagePlugin.success(t("common.saveSuccess"));
        emit("saved");
        handleClose();
      } else {
        MessagePlugin.error(orgStore.error || t("common.saveFailed"));
      }
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("common.saveFailed"));
  } finally {
    submitting.value = false;
  }
};

const handleRoleChange = async (member: OrganizationMember, newRole: string) => {
  if (!props.orgId) return;
  try {
    const success = await orgStore.changeMemberRole(
      props.orgId,
      member.tenant_id,
      newRole as "admin" | "editor" | "viewer",
    );
    if (success) {
      MessagePlugin.success(t("organization.roleUpdated"));
    } else {
      MessagePlugin.error(orgStore.error || t("organization.roleUpdateFailed"));
      fetchMembers();
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("organization.roleUpdateFailed"));
    fetchMembers();
  }
};

const confirmRemoveMember = async (member: OrganizationMember) => {
  if (!props.orgId) return;

  try {
    const success = await orgStore.kickMember(props.orgId, member.tenant_id);
    if (success) {
      MessagePlugin.success(t("organization.memberRemoved"));
    } else {
      MessagePlugin.error(orgStore.error || t("organization.memberRemoveFailed"));
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("organization.memberRemoveFailed"));
  }
};

watch(upgradePopupVisible, (visible) => {
  if (!visible) return;
  upgradeForm.value = {
    requested_role: (upgradeRoleOptions.value[0]?.value as "editor" | "admin") || "editor",
    message: "",
  };
});

const handleSubmitUpgrade = async () => {
  if (!props.orgId) return;

  upgradeSubmitting.value = true;
  try {
    const res = await orgStore.requestOrganizationRoleUpgrade(props.orgId, {
      requested_role: upgradeForm.value.requested_role,
      message: upgradeForm.value.message,
    });
    if (res.success) {
      MessagePlugin.success(t("organization.upgrade.submitSuccess"));
      hasPendingUpgrade.value = true;
      upgradePopupVisible.value = false;
    } else {
      MessagePlugin.error(res.message || t("organization.upgrade.submitFailed"));
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("organization.upgrade.submitFailed"));
  } finally {
    upgradeSubmitting.value = false;
  }
};

// 添加成员：搜索空间（仅按空间名模糊匹配，按 tenant_id 去重）
let tenantSearchTimer: ReturnType<typeof setTimeout> | null = null;
const handleTenantSearch = (query: string) => {
  if (tenantSearchTimer) {
    clearTimeout(tenantSearchTimer);
  }
  if (!query || query.length < 2) {
    tenantSearchResults.value = [];
    return;
  }
  tenantSearchTimer = setTimeout(async () => {
    if (!props.orgId) return;
    tenantSearchLoading.value = true;
    try {
      const res = await searchTenantsForInvite(props.orgId, query, 10);
      if (res.success && res.data) {
        tenantSearchResults.value = res.data;
      }
    } catch (error) {
      console.error("Failed to search tenants:", error);
    } finally {
      tenantSearchLoading.value = false;
    }
  }, 300);
};

// 添加成员：把选中的空间拉入空间。后端要求 tenant_id；representative_user_id
// 仅做展示/审计用，所以把搜索结果中代表用户也一并带上。
const handleAddMember = async () => {
  if (!props.orgId || selectedTenantId.value == null) return;

  const candidate = tenantSearchResults.value.find((c) => c.tenant_id === selectedTenantId.value);

  addMemberSubmitting.value = true;
  try {
    const res = await orgStore.inviteOrganizationMember(props.orgId, {
      tenant_id: selectedTenantId.value,
      representative_user_id: candidate?.representative_user_id,
      role: addMemberRole.value,
    });
    if (res.success) {
      MessagePlugin.success(t("organization.addMember.success"));
      addMemberPopupVisible.value = false;
      resetAddMemberDialog();
      fetchMembers(); // 刷新成员列表
    } else {
      MessagePlugin.error(res.message || t("organization.addMember.failed"));
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("organization.addMember.failed"));
  } finally {
    addMemberSubmitting.value = false;
  }
};

// The tenant picker's own state: whether its list is open, the query typed
// into it, and the label of the chosen tenant. The label is kept apart from
// the search results so it survives a later search that no longer lists it.
const tenantPickerOpen = ref(false);
const tenantQuery = ref("");
const selectedTenantLabel = ref("");

const selectTenant = (opt: { label: string; value: number }) => {
  selectedTenantId.value = opt.value;
  selectedTenantLabel.value = opt.label;
  tenantPickerOpen.value = false;
};

const clearSelectedTenant = () => {
  selectedTenantId.value = null;
  selectedTenantLabel.value = "";
};

// 重置添加成员弹窗
const resetAddMemberDialog = () => {
  selectedTenantId.value = null;
  selectedTenantLabel.value = "";
  tenantQuery.value = "";
  addMemberRole.value = "viewer";
  tenantSearchResults.value = [];
};

const copyInviteCode = async () => {
  await copyWithToast(inviteCode.value, "common.copied");
};

const copyInviteLink = async () => {
  await copyWithToast(inviteLink.value, "common.copied");
};

const refreshInviteCode = async () => {
  if (!props.orgId) return;
  refreshingCode.value = true;
  try {
    const code = await orgStore.refreshInviteCode(props.orgId);
    if (code) {
      inviteCode.value = code;
      MessagePlugin.success(t("organization.inviteCodeRefreshed"));
      await fetchOrgDetail();
    } else {
      MessagePlugin.error(orgStore.error || t("organization.inviteCodeRefreshFailed"));
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("organization.inviteCodeRefreshFailed"));
  } finally {
    refreshingCode.value = false;
  }
};

// The select speaks strings; the setting is a number of days.
const onValiditySelect = (value: AcceptableValue) => {
  const days = Number(value);
  formData.value.invite_code_validity_days = days;
  void handleValidityChange(days);
};

// The member limit input. An empty field leaves the last number in place
// (a cleared field is mid-edit, not a request for "unlimited", which is 0),
// and leaving the field clamps the value to the range t-input-number enforced.
const onMemberLimitInput = (value: string | number) => {
  if (value === "") return;
  const n = Number(value);
  if (Number.isFinite(n)) formData.value.member_limit = n;
};

const clampMemberLimit = () => {
  const n = Math.round(formData.value.member_limit);
  formData.value.member_limit = Math.min(10000, Math.max(0, Number.isFinite(n) ? n : 0));
};

const handleValidityChange = async (value: number) => {
  if (!props.orgId) return;
  try {
    const result = await orgStore.updateOrganization(props.orgId, {
      invite_code_validity_days: value,
    });
    if (result) {
      MessagePlugin.success(t("common.saveSuccess"));
    } else {
      formData.value.invite_code_validity_days = orgInfo.value?.invite_code_validity_days ?? 7;
      MessagePlugin.error(orgStore.error || t("common.saveFailed"));
    }
  } catch (error: any) {
    formData.value.invite_code_validity_days = orgInfo.value?.invite_code_validity_days ?? 7;
    MessagePlugin.error(error?.message || t("common.saveFailed"));
  }
};

// 切换审核开关时立即保存
const handleApprovalToggle = async (value: boolean) => {
  if (!props.orgId) return;
  try {
    const result = await orgStore.updateOrganization(props.orgId, {
      require_approval: value,
    });
    if (result) {
      MessagePlugin.success(t("common.saveSuccess"));
    } else {
      // 回滚
      formData.value.require_approval = !value;
      MessagePlugin.error(orgStore.error || t("common.saveFailed"));
    }
  } catch (error: any) {
    // 回滚
    formData.value.require_approval = !value;
    MessagePlugin.error(error?.message || t("common.saveFailed"));
  }
};

// 切换开放可被搜索时立即保存
const handleSearchableToggle = async (value: boolean) => {
  if (!props.orgId) return;
  try {
    const result = await orgStore.updateOrganization(props.orgId, {
      searchable: value,
    });
    if (result) {
      MessagePlugin.success(t("common.saveSuccess"));
    } else {
      formData.value.searchable = !value;
      MessagePlugin.error(orgStore.error || t("common.saveFailed"));
    }
  } catch (error: any) {
    formData.value.searchable = !value;
    MessagePlugin.error(error?.message || t("common.saveFailed"));
  }
};

const handleShareClick = (share: KnowledgeBaseShare) => {
  handleClose();
  router.push(`/platform/knowledge-bases/${share.knowledge_base_id}`);
};

const handleRemoveShare = async (share: KnowledgeBaseShare) => {
  if (!props.orgId) return;
  try {
    const res = await orgStore.unshareKnowledgeBase(share.knowledge_base_id, share.id, props.orgId);
    if (res.success) {
      MessagePlugin.success(t("organization.settings.removeShareSuccess"));
      sharedKnowledgeBases.value = sharedKnowledgeBases.value.filter((s) => s.id !== share.id);
    } else {
      MessagePlugin.error(res.message || t("organization.settings.removeShareFailed"));
    }
  } catch (error: any) {
    MessagePlugin.error(error?.message || t("organization.settings.removeShareFailed"));
  }
};

const formatDate = (dateStr: string) => {
  if (!dateStr) return "";
  const date = new Date(dateStr);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
};

// Tag colours. The tone follows the old getRoleTheme / getPermissionTheme
// mapping (admin primary, editor warning, the rest neutral); the variant
// matches the TDesign tag it replaces — "dark" is TDesign's default solid
// fill, "light" its tinted `variant="light"`.
type TagTone = "primary" | "warning" | "default";

const roleTone = (role: string): TagTone => {
  switch (role) {
    case "admin":
      return "primary";
    case "editor":
      return "warning";
    default:
      return "default";
  }
};

const TAG_BASE = "inline-flex h-[22px] shrink-0 items-center rounded-[3px] px-2 text-xs leading-none whitespace-nowrap";

const tagClass = (tone: TagTone, variant: "dark" | "light"): string => {
  if (variant === "dark") {
    if (tone === "primary") return `${TAG_BASE} bg-primary text-primary-foreground`;
    if (tone === "warning") return `${TAG_BASE} bg-warning text-primary-foreground`;
    return `${TAG_BASE} bg-[var(--td-bg-color-component)] text-foreground`;
  }
  if (tone === "primary") return `${TAG_BASE} bg-[var(--td-brand-color-light)] text-primary`;
  if (tone === "warning") return `${TAG_BASE} bg-[var(--td-warning-color-light)] text-warning`;
  return `${TAG_BASE} bg-muted text-foreground`;
};

// Class strings shared by several parts of the template. They are named
// here once rather than repeated, because the three list sections (members,
// join requests, shared knowledge bases) are built from the same pieces.
const sectionTitleClass = "text-foreground m-0 font-(family-name:--app-font-family) text-xl font-semibold";
const sectionDescriptionClass =
  "text-muted-foreground m-0 mt-2 font-(family-name:--app-font-family) text-sm leading-normal";
const sectionHeaderRowClass = "flex w-full min-w-0 flex-wrap items-center justify-between gap-3";
const sectionTitleWrapClass = "inline-flex min-w-0 flex-auto items-center gap-1";
const hintTriggerClass =
  "text-muted-foreground hover:bg-muted hover:text-primary focus-visible:outline-ring inline-flex size-[22px] shrink-0 items-center justify-center rounded-md leading-none transition-colors duration-200 focus-visible:outline-2 focus-visible:outline-offset-1";
// The hint popovers keep the old overlay's frosted look in both themes.
const hintPopoverClass =
  "z-[3050] gap-0 overflow-hidden rounded-xl border-[0.5px] border-border bg-card p-0 ring-0 shadow-[0_0_0_0.5px_rgba(0,0,0,0.03),0_2px_4px_rgba(0,0,0,0.04),0_8px_24px_rgba(0,0,0,0.1)] backdrop-blur-[20px] backdrop-saturate-[180%] dark:border-white/8 dark:bg-[rgba(36,36,36,0.92)] dark:shadow-[0_0_0_0.5px_rgba(255,255,255,0.05),0_2px_4px_rgba(0,0,0,0.12),0_8px_32px_rgba(0,0,0,0.28)]";

const settingRowClass =
  "flex min-w-0 items-start justify-between gap-6 border-b border-border py-4 first:pt-0 last:border-b-0";
const settingInfoClass = "min-w-0 max-w-[42%] flex-[0_0_42%]";
const settingLabelClass = "text-foreground mb-1 block text-[15px] font-medium";
const settingDescClass = "text-muted-foreground m-0 text-[13px] leading-normal";
const settingControlClass = "flex min-w-0 max-w-[58%] flex-[1_1_58%] items-start justify-end";

const inviteHeaderClass = "mb-2.5 flex items-center gap-2";
const inviteIconClass = "text-primary size-4";
const inviteTitleClass = "text-foreground text-[13px] font-semibold";
const inviteDescClass = "text-muted-foreground m-0 mt-1 mb-2.5 text-xs leading-[1.4]";
const inviteDividerClass = "bg-muted my-3 h-px";

const listHeaderClass = "flex flex-wrap items-center justify-between gap-3 px-0.5";
const listTitleWrapClass = "inline-flex min-w-0 items-center gap-2";
const listTitleClass = "text-foreground text-sm font-semibold";
const countBadgeClass =
  "bg-muted text-foreground inline-flex h-5 min-w-[22px] items-center justify-center rounded-[10px] px-[7px] text-xs leading-none font-semibold";
const listActionsClass =
  "inline-flex min-w-0 flex-[0_1_auto] items-center gap-2 max-[560px]:w-full max-[560px]:justify-start";
const listSearchClass = "relative w-56 min-w-0 flex-[0_0_14rem] max-[560px]:w-auto max-[560px]:flex-auto";
const searchIconClass = "text-placeholder pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2";
const searchInputClass = "h-7 pr-7 pl-7 text-[13px] md:text-[13px]";
const clearButtonClass =
  "text-placeholder hover:text-muted-foreground absolute top-1/2 right-2 flex -translate-y-1/2 items-center";

const loadingInlineClass = "flex items-center gap-2 pt-5 pb-2";
const emptyClass = "gap-2 p-0 pt-2 pb-4";
const emptyIconClass = "text-placeholder size-12";
const tableShellClass = "border-border bg-card relative overflow-hidden rounded-[10px] border";
const tableLoadingClass = "bg-card/60 absolute inset-0 z-10 flex items-center justify-center";
const headRowClass = "bg-muted hover:bg-muted";
const headCellClass = "h-auto px-3 py-3 text-[13px] font-semibold";
const bodyRowClass = "even:bg-muted/50 hover:bg-accent";
const bodyCellClass = "overflow-hidden px-3 py-3 text-ellipsis";
const dangerIconButtonClass = "text-destructive hover:text-destructive hover:bg-destructive/10";

// The member / upgrade / approve popovers share one frame.
const actionPopoverClass =
  "z-[3050] max-w-full gap-0 rounded-[10px] border border-border p-4 ring-0 shadow-[var(--td-shadow-2),0_8px_24px_rgba(15,23,42,0.08)]";
const popupTitleClass = "text-foreground m-0 text-[15px] leading-[1.35] font-semibold";
const popupTipClass =
  "bg-muted border-border text-muted-foreground m-0 rounded-lg border px-3 py-2.5 text-[13px] leading-normal";
const popupFieldClass = "flex w-full min-w-0 flex-col gap-2";
const popupFieldLabelClass = "text-foreground m-0 block text-sm leading-[1.4] font-medium";
const popupFooterClass = "border-border mt-4 flex justify-end gap-2 border-t pt-3";

// The two info icons in the section headers open their popover on hover,
// as the old t-popup trigger="hover" did; a click toggles it too, so the
// hint is also reachable without a mouse. The short close delay lets the
// pointer travel from the icon into the popover without it closing.
function useHoverPopover() {
  const open = ref(false);
  let timer: ReturnType<typeof setTimeout> | null = null;
  const cancel = () => {
    if (timer) {
      clearTimeout(timer);
      timer = null;
    }
  };
  const show = () => {
    cancel();
    open.value = true;
  };
  const hide = () => {
    cancel();
    timer = setTimeout(() => {
      open.value = false;
    }, 120);
  };
  return { open, show, hide };
}

// reactive() unwraps `open`, so the template can bind it with v-model directly.
const membersPermHint = reactive(useHoverPopover());
const shareCalcHint = reactive(useHoverPopover());

const scrollContentToTop = async () => {
  await nextTick();
  contentWrapperRef.value?.scrollTo({ top: 0, behavior: "auto" });
};

let previousBodyOverflow = "";
let bodyScrollLocked = false;

const lockBackgroundScroll = () => {
  if (bodyScrollLocked) return;
  previousBodyOverflow = document.body.style.overflow;
  document.body.style.overflow = "hidden";
  bodyScrollLocked = true;
};

const unlockBackgroundScroll = () => {
  if (!bodyScrollLocked) return;
  document.body.style.overflow = previousBodyOverflow;
  bodyScrollLocked = false;
};

// Watch
watch(
  () => props.visible,
  (newVal) => {
    if (newVal) {
      lockBackgroundScroll();
      void scrollContentToTop();
      currentSection.value = "basic";
      memberSearchQuery.value = "";
      joinRequestSearchQuery.value = "";
      approvePopupRequestId.value = null;
      joinRequests.value = [];
      if (props.mode === "create") {
        // 创建模式：重置表单
        formData.value = {
          name: "",
          description: "",
          avatar: "",
          require_approval: false,
          searchable: false,
          invite_code_validity_days: 7,
          member_limit: 50,
        };
        orgStore.clearCurrentOrganizationContext();
        sharedKnowledgeBases.value = [];
        inviteCode.value = "";
        inviteCodeExpiresAt.value = null;
      } else if (props.orgId) {
        // 清空上一个组织的详情上下文，避免在 fetchOrgDetail 返回前短暂显示旧组织信息
        if (orgStore.currentOrganization?.id !== props.orgId) {
          orgStore.clearCurrentOrganizationContext();
          sharedKnowledgeBases.value = [];
        }
        fetchOrgDetail();
        fetchMembers();
        fetchSharedKBs();
      }
    } else {
      unlockBackgroundScroll();
    }
  },
  { immediate: true },
);

watch(
  () => props.orgId,
  () => {
    if (props.visible) {
      void scrollContentToTop();
    }
  },
);

watch(currentSection, (section) => {
  void scrollContentToTop();
  if (section === "joinRequests" && props.orgId) {
    fetchJoinRequests();
  }
});

onBeforeUnmount(() => {
  unlockBackgroundScroll();
});

watch(addMemberPopupVisible, (visible) => {
  if (!visible) {
    resetAddMemberDialog();
  }
});
</script>

<style scoped>
/*
 * What stays CSS here is what utilities cannot express: the section fade-in
 * keyframes (a scoped @keyframes name is rewritten per component, so a
 * utility could not reference it), the <Transition name="modal"> classes,
 * whose leave state also reaches into the child modal, and the scrollbar
 * pseudo-elements of the two scrolling columns.
 */
.section {
  animation: sectionFadeIn 0.25s ease;
}

@keyframes sectionFadeIn {
  from {
    opacity: 0;
    transform: translateY(6px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

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

.settings-nav::-webkit-scrollbar,
.content-wrapper::-webkit-scrollbar {
  width: 6px;
}

.settings-nav::-webkit-scrollbar-track {
  background: var(--td-bg-color-secondarycontainer);
}

.settings-nav::-webkit-scrollbar-thumb,
.content-wrapper::-webkit-scrollbar-thumb {
  background: var(--td-gray-color-5);
  border-radius: 3px;
}

.settings-nav::-webkit-scrollbar-thumb:hover,
.content-wrapper::-webkit-scrollbar-thumb:hover {
  background: var(--td-gray-color-6);
}

.content-wrapper::-webkit-scrollbar-track {
  background: var(--td-bg-color-container);
}
</style>
