<template>
  <div class="w-full">
    <!-- Section header. The (i) permission speed-look popover lives
         next to the title so it reads as meta-info about *this
         section*. The audit-log entry sits on the right of the header
         row — secondary navigation that opens the audit drawer; gated
         to Admin+ so non-managers don't see a button they can't use. -->
    <div class="mb-5">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="inline-flex min-w-0 items-center gap-0.5">
          <h2 class="text-foreground m-0 text-[20px] leading-[1.25] font-semibold tracking-[-0.02em]">
            {{ $t("tenantMember.title") }}
          </h2>
          <!--
            t-popup opened this on hover; Reka's popover opens on click only,
            so hover is wired by hand with a short grace period that lets the
            pointer travel from the icon into the panel.
          -->
          <Popover :open="permissionsPopupOpen" @update:open="onPermissionsPopupOpenChange">
            <PopoverTrigger as-child>
              <button
                type="button"
                data-slot="permissions-trigger"
                class="text-muted-foreground hover:bg-secondary hover:text-primary focus-visible:outline-ring inline-flex size-[22px] shrink-0 items-center justify-center rounded-md leading-none transition-colors duration-200 focus-visible:outline-2 focus-visible:outline-offset-1"
                :aria-label="$t('tenantMember.permissions.title')"
                :title="$t('tenantMember.permissions.iconHint')"
                @mouseenter="openPermissionsPopup"
                @mouseleave="schedulePermissionsPopupClose"
              >
                <InfoIcon class="size-4" />
              </button>
            </PopoverTrigger>
            <!-- z-index above the settings modal (1100), as the old overlay had. -->
            <PopoverContent
              align="start"
              :class="[
                popupSurfaceClass,
                'max-h-[min(400px,65vh)] w-[min(520px,calc(100vw-24px))] max-w-[min(520px,calc(100vw-24px))] overflow-hidden p-0',
              ]"
              @mouseenter="openPermissionsPopup"
              @mouseleave="schedulePermissionsPopupClose"
              @open-auto-focus.prevent
            >
              <div class="m-0 max-h-[min(392px,calc(65vh-8px))] overflow-x-hidden overflow-y-auto px-3.5 py-3">
                <div class="mb-2.5 flex flex-col gap-0.5">
                  <span class="text-foreground text-[13px] font-semibold">{{
                    $t("tenantMember.permissions.title")
                  }}</span>
                  <span class="text-muted-foreground text-[12px] leading-[1.45]">{{
                    $t("tenantMember.permissions.desc")
                  }}</span>
                </div>
                <div class="grid grid-cols-2 gap-2 max-[480px]:grid-cols-1">
                  <div
                    v-for="r in roleMatrixOrder"
                    :key="r"
                    class="rounded-md border px-2.5 py-2"
                    :class="
                      currentRole === r ? 'border-primary bg-[var(--td-brand-color-light)]' : 'border-border bg-card'
                    "
                  >
                    <div class="text-foreground mb-1.5 flex items-center gap-1 text-[12px] font-semibold">
                      <component :is="roleMatrixIcon(r)" class="size-3" />
                      <span>{{ $t("tenantMember.role." + r) }}</span>
                      <span
                        v-if="currentRole === r"
                        class="text-primary ml-auto rounded-[4px] bg-[var(--td-brand-color-light)] px-[5px] py-px text-[10px] font-medium"
                        >{{ $t("common.me") }}</span
                      >
                    </div>
                    <div class="flex flex-col gap-[3px]">
                      <span
                        v-for="(perm, i) in roleMatrix[r]"
                        :key="i"
                        class="flex items-start gap-1 text-[11px] leading-[1.35]"
                        :class="perm.has ? 'text-muted-foreground' : 'text-[var(--td-text-color-disabled)]'"
                      >
                        <CheckIcon v-if="perm.has" class="text-primary mt-px size-3 shrink-0" />
                        <XIcon v-else class="mt-px size-3 shrink-0" />
                        {{ $t("tenantMember.permissions." + perm.key) }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </PopoverContent>
          </Popover>
          <!-- Audit log entry sits inline with the title: title (i)
               [审计日志]. Keeping all section-level affordances on the
               left edge avoids the "lonely right-aligned button"
               pattern in narrow settings panels. -->
          <Button v-if="canViewAudit" variant="ghost" size="sm" class="shrink-0" @click="openAuditDrawer">
            <HistoryIcon />
            {{ $t("tenantMember.audit.tabLabel") }}
          </Button>
        </div>
      </div>
      <p class="text-muted-foreground mt-2 mb-0 max-w-[52rem] text-[13px] leading-[1.55]">
        {{ $t("tenantMember.sectionDescription") }}
        <a
          class="doc-link"
          v-if="docsUrl('03-features/01-tenant-auth')"
          :href="docsUrl('03-features/01-tenant-auth')"
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ $t("tenantMember.learnRbacGuide") }}
          <LinkIcon class="link-icon size-[1em]" />
        </a>
      </p>
    </div>

    <div class="flex flex-col">
      <!-- Toolbar 已被并入「空间成员」列表头：搜索框紧贴列表头右
           侧，邀请按钮再往右一个图标位，所有「针对这张列表」的控
           件聚到同一行，独立 toolbar 不复存在。 -->

      <!-- Pending invitations. Shown only to managers because the
               viewer/contributor roles don't have an action surface
               here. Even when empty we still render the header so
               operators get a stable "is there anything pending?"
               affordance after they hit "Send invitation". -->
      <div v-if="canManage" class="mb-6 flex flex-col gap-2.5">
        <div class="flex flex-col gap-1">
          <div class="flex items-center gap-2">
            <span class="text-foreground text-[14px] font-semibold">
              {{ $t("tenantInvitation.pendingSectionTitle") }}
            </span>
            <!-- Same count-badge style as the «空间成员» list header
                 so the two list titles read at parity. -->
            <span :class="countBadgeClass">{{ invitationsTotal }}</span>
          </div>
          <span class="text-muted-foreground text-[12px]">
            {{ $t("tenantInvitation.pendingSectionDesc", { days: INVITATION_TTL_DAYS }) }}
          </span>
        </div>
        <div v-if="invitationsLoading" class="flex items-center gap-2 pt-5 pb-2">
          <Loader2Icon class="text-primary size-4 animate-spin" />
          <span>{{ $t("tenantMember.loading") }}</span>
        </div>
        <div v-else-if="invitationsError" class="flex items-center gap-2 pt-5 pb-2">
          <Alert variant="destructive">
            <CircleAlertIcon />
            <AlertDescription>{{ invitationsError }}</AlertDescription>
            <AlertAction>
              <Button size="sm" variant="outline" @click="loadInvitations">{{ $t("tenantMember.retry") }}</Button>
            </AlertAction>
          </Alert>
        </div>
        <div
          v-else-if="invitationsTotal === 0"
          class="border-border bg-card text-muted-foreground rounded-lg border border-dashed px-3 py-2.5 text-[13px]"
        >
          {{ $t("tenantInvitation.pendingEmpty") }}
        </div>
        <!-- Visually distinguish from members table so the eye doesn't fuse
             "pending" rows with "actual member" rows. -->
        <div v-else :class="[tableShellClass, 'bg-secondary']">
          <Table class="min-w-[790px] table-fixed">
            <TableHeader>
              <TableRow :class="tableHeadRowClass">
                <TableHead :class="[tableHeadClass, 'min-w-[160px]']">
                  {{ $t("tenantInvitation.columns.invitee") }}
                </TableHead>
                <TableHead :class="[tableHeadClass, 'w-[110px]']">{{ $t("tenantInvitation.columns.role") }}</TableHead>
                <TableHead :class="[tableHeadClass, 'min-w-[140px]']">
                  {{ $t("tenantInvitation.columns.inviter") }}
                </TableHead>
                <TableHead :class="[tableHeadClass, 'w-[160px]']">
                  {{ $t("tenantInvitation.columns.expiresAt") }}
                </TableHead>
                <TableHead :class="[tableHeadClass, 'w-[100px]']">
                  {{ $t("tenantInvitation.columns.status") }}
                </TableHead>
                <TableHead v-if="canManage" :class="[tableHeadClass, 'w-[120px]']">
                  {{ $t("tenantInvitation.columns.operations") }}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="row in invitations" :key="row.id" class="hover:bg-accent">
                <TableCell :class="tableCellClass">
                  <div :class="memberCellClass">
                    <template v-if="row.is_share_link">
                      <span :class="[memberNameClass, 'text-primary inline-flex items-center gap-1']">
                        <LinkIcon class="size-3.5 shrink-0" />
                        {{ $t("tenantInvitation.shareLink.cellTitle") }}
                      </span>
                      <span :class="memberEmailClass">
                        {{
                          (row.accepted_count ?? 0) > 0
                            ? $t("tenantInvitation.shareLink.cellAccepted", { count: row.accepted_count })
                            : $t("tenantInvitation.shareLink.cellEmpty")
                        }}
                      </span>
                    </template>
                    <template v-else>
                      <span :class="[memberNameClass, 'text-foreground']">{{ inviteePrimary(row) }}</span>
                      <span v-if="row.invitee_email && row.invitee_name" :class="memberEmailClass">{{
                        row.invitee_email
                      }}</span>
                    </template>
                  </div>
                </TableCell>
                <TableCell :class="tableCellClass">
                  <span :class="[tagBaseClass, roleTagClass(row.role)]">
                    {{ $t("tenantMember.role." + row.role) }}
                  </span>
                </TableCell>
                <TableCell :class="[tableCellClass, 'truncate']">{{ inviterPrimary(row) }}</TableCell>
                <TableCell :class="tableCellClass">{{ formatDate(row.expires_at) }}</TableCell>
                <TableCell :class="tableCellClass">
                  <span :class="[tagBaseClass, invitationStatusClass(row.status)]">
                    {{
                      row.is_share_link && row.status === "pending"
                        ? $t("tenantInvitation.status.shareLinkActive")
                        : $t("tenantInvitation.status." + row.status)
                    }}
                  </span>
                </TableCell>
                <TableCell v-if="canManage" :class="tableCellClass">
                  <div class="flex items-center">
                    <!-- Per-row "copy link" for active share-link rows.
                         Icon-only with tooltip so two actions ("copy" +
                         "revoke") fit inside the actions column without
                         clipping; the full label was too wide. -->
                    <Tooltip v-if="row.status === 'pending' && row.invite_url">
                      <TooltipTrigger as-child>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          :aria-label="$t('tenantInvitation.copyLink')"
                          @click="copyText(absoluteInviteURL(row.invite_url))"
                        >
                          <CopyIcon />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent side="top" :class="tooltipLayerClass">{{
                        $t("tenantInvitation.copyLink")
                      }}</TooltipContent>
                    </Tooltip>
                    <!-- Inline popconfirm anchored to the revoke button.
                           Avoids spawning a top-level modal for a simple
                           yes/no decision; the popover stays inside the
                           table cell so the user keeps spatial context. -->
                    <Popover v-if="row.status === 'pending'">
                      <Tooltip>
                        <TooltipTrigger as-child>
                          <PopoverTrigger as-child>
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              :class="dangerIconButtonClass"
                              :aria-label="$t('tenantInvitation.revoke.button')"
                            >
                              <XIcon />
                            </Button>
                          </PopoverTrigger>
                        </TooltipTrigger>
                        <TooltipContent side="top" :class="tooltipLayerClass">{{
                          $t("tenantInvitation.revoke.button")
                        }}</TooltipContent>
                      </Tooltip>
                      <PopoverContent side="left" :class="popconfirmClass">
                        <div class="flex items-start gap-2">
                          <CircleAlertIcon class="text-warning mt-0.5 size-4 shrink-0" />
                          <p class="text-foreground m-0 text-[14px] leading-[1.5]">
                            {{
                              row.is_share_link
                                ? $t("tenantInvitation.shareLink.revokeConfirm")
                                : $t("tenantInvitation.revoke.confirmBody", {
                                    email: row.invitee_email || row.invitee_user_id,
                                  })
                            }}
                          </p>
                        </div>
                        <div class="flex justify-end gap-2">
                          <PopoverClose as-child>
                            <Button variant="outline" size="sm">{{ $t("common.cancel") }}</Button>
                          </PopoverClose>
                          <PopoverClose as-child>
                            <Button size="sm" :class="dangerSolidButtonClass" @click="doRevokeInvitation(row)">
                              {{ $t("tenantInvitation.revoke.confirm") }}
                            </Button>
                          </PopoverClose>
                        </div>
                      </PopoverContent>
                    </Popover>
                  </div>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
          <!-- 待接受区块整体为浅底色，分页条与表格区同色阶、仅靠顶部分割线与表格区分 -->
          <div v-if="invitationsTotal > 0" :class="pagerBarClass">
            <TenantMembersPager
              v-model:page="invitationsPage"
              v-model:page-size="invitationsPageSize"
              :total="invitationsTotal"
              :page-size-options="INVITATIONS_PAGE_SIZE_OPTIONS"
              @change="onInvitationsPageChange"
            />
          </div>
        </div>
      </div>

      <!-- Member list. 列表头（标题 / 计数 / 搜索框 / 邀请按钮）始终
           渲染，loading / error / empty / 表格作为下方的内容状态切换。
           这样搜索时输入框不会被卸载，避免焦点丢失与页面抖动。 -->
      <div class="flex flex-col gap-2.5">
        <!-- 列表上方的标题行：「空间成员 [N]」 左侧；右侧
             是「搜索 + 邀请按钮」一组。视觉级别与「待接受邀请」一致。 -->
        <div class="flex flex-wrap items-center justify-between gap-3 px-0.5">
          <div class="inline-flex min-w-0 items-center gap-2">
            <span class="text-foreground text-[14px] font-semibold">{{ $t("tenantMember.listTitle") }}</span>
            <span :class="countBadgeClass">{{ membersTotal }}</span>
          </div>
          <div
            class="inline-flex min-w-0 flex-[0_1_auto] items-center gap-2 max-[560px]:w-full max-[560px]:justify-start"
          >
            <!-- An explicit width keeps the row from shifting when the clear
                 button appears; on narrow screens the box takes the row. -->
            <div class="relative w-56 min-w-0 flex-[0_0_14rem] max-[560px]:w-auto max-[560px]:flex-[1_1_auto]">
              <SearchIcon
                class="text-placeholder pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2"
              />
              <Input
                v-model="searchQuery"
                :placeholder="$t('tenantMember.searchPlaceholder')"
                class="h-7 pr-7 pl-7 text-[13px] md:text-[13px]"
              />
              <button
                v-if="searchQuery"
                type="button"
                data-slot="search-clear"
                class="text-placeholder hover:text-muted-foreground absolute top-1/2 right-2 flex -translate-y-1/2"
                :aria-label="$t('common.clear')"
                @click="searchQuery = ''"
              >
                <CircleXIcon class="size-3.5" />
              </button>
            </div>
            <Popover v-if="canManage" v-model:open="invitePopupVisible">
              <PopoverTrigger as-child>
                <!-- outline + primary: an icon button with enough weight to read as the list's main action. -->
                <Button
                  variant="outline"
                  size="icon-sm"
                  class="border-primary text-primary hover:text-primary shrink-0"
                  :title="$t('tenantMember.add.button')"
                  :aria-label="$t('tenantMember.add.button')"
                >
                  <UserPlusIcon />
                </Button>
              </PopoverTrigger>
              <PopoverContent align="end" :class="[popupSurfaceClass, invitePopupClass]">
                <div class="max-w-full" @click.stop>
                  <div class="text-foreground m-0 mb-3 text-[15px] leading-[1.35] font-semibold">
                    {{
                      addDialogStep === "form"
                        ? $t("tenantMember.add.dialogTitle")
                        : $t("tenantInvitation.confirmInviteTitle")
                    }}
                  </div>
                  <form v-if="addDialogStep === 'form'" class="flex flex-col" @submit.prevent="submitAdd">
                    <div :class="formItemClass">
                      <Label for="tenant-member-invite-email" :class="formLabelClass">
                        {{ $t("tenantMember.add.emailLabel") }}
                      </Label>
                      <div class="min-w-0">
                        <Input
                          id="tenant-member-invite-email"
                          v-model="addForm.email"
                          :placeholder="$t('tenantMember.add.emailPlaceholder')"
                          :aria-invalid="addFormErrors.email ? true : undefined"
                          @blur="validateAddEmail"
                        />
                        <p v-if="addFormErrors.email" class="text-destructive m-0 mt-1 text-xs">
                          {{ addFormErrors.email }}
                        </p>
                      </div>
                    </div>
                    <div :class="[formItemClass, 'mb-1']">
                      <Label :class="formLabelClass">{{ $t("tenantMember.add.roleLabel") }}</Label>
                      <div class="min-w-0">
                        <Select
                          :model-value="addForm.role"
                          @update:model-value="(v) => onAddRoleChange(String(v ?? ''))"
                        >
                          <SelectTrigger class="w-full" :aria-invalid="addFormErrors.role ? true : undefined">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent position="popper" :class="selectLayerClass">
                            <SelectItem v-for="opt in roleOptions" :key="opt.value" :value="opt.value">
                              {{ opt.label }}
                            </SelectItem>
                          </SelectContent>
                        </Select>
                        <p v-if="addFormErrors.role" class="text-destructive m-0 mt-1 text-xs">
                          {{ addFormErrors.role }}
                        </p>
                      </div>
                    </div>
                  </form>
                  <div v-else :class="inviteBodyClass">
                    {{
                      $t("tenantInvitation.confirmInviteBody", {
                        email: addConfirmEmail,
                        role: addConfirmRoleLabel,
                      })
                    }}
                  </div>
                  <div class="mt-4 flex justify-end gap-2">
                    <Button
                      v-if="addDialogStep === 'form'"
                      variant="outline"
                      :disabled="adding"
                      @click="invitePopupVisible = false"
                    >
                      {{ $t("common.cancel") }}
                    </Button>
                    <Button v-else variant="outline" :disabled="adding" @click="goBackToForm">
                      {{ $t("common.back") }}
                    </Button>
                    <Button :disabled="adding" @click="submitAdd">
                      <Loader2Icon v-if="adding" class="animate-spin" />
                      {{ dialogConfirmLabel }}
                    </Button>
                  </div>
                </div>
              </PopoverContent>
            </Popover>
            <!-- Share-link generator. Sits next to the invite-by-email
                 popup so the two flows live side-by-side: "I know who"
                 (email input) vs "I don't" (one link, group chat). -->
            <Popover v-if="canManage" v-model:open="shareLinkPopupVisible">
              <PopoverTrigger as-child>
                <Button
                  variant="outline"
                  size="icon-sm"
                  class="shrink-0"
                  :title="$t('tenantInvitation.shareLink.button')"
                  :aria-label="$t('tenantInvitation.shareLink.button')"
                >
                  <LinkIcon />
                </Button>
              </PopoverTrigger>
              <PopoverContent align="end" :class="[popupSurfaceClass, invitePopupClass]">
                <div class="max-w-full" @click.stop>
                  <div class="text-foreground m-0 mb-3 text-[15px] leading-[1.35] font-semibold">
                    {{
                      shareLinkResult
                        ? $t("tenantInvitation.shareLink.resultTitle")
                        : $t("tenantInvitation.shareLink.dialogTitle")
                    }}
                  </div>
                  <div v-if="!shareLinkResult" class="flex flex-col">
                    <p :class="[inviteBodyClass, 'm-0']">
                      {{ $t("tenantInvitation.shareLink.description", { days: INVITATION_TTL_DAYS }) }}
                    </p>
                    <div :class="[formItemClass, 'mb-1']">
                      <Label :class="formLabelClass">{{ $t("tenantMember.add.roleLabel") }}</Label>
                      <Select
                        :model-value="shareLinkForm.role"
                        @update:model-value="(v) => (shareLinkForm.role = String(v ?? '') as TenantRole)"
                      >
                        <SelectTrigger class="w-full">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent position="popper" :class="selectLayerClass">
                          <SelectItem v-for="opt in roleOptions" :key="opt.value" :value="opt.value">
                            {{ opt.label }}
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                  <!-- Share-link result panel — single-row layout: input field stretches,
                       copy button stays fixed-width. -->
                  <div v-else class="pt-1">
                    <p :class="[inviteBodyClass, 'm-0']">
                      {{ $t("tenantInvitation.shareLink.resultBody") }}
                    </p>
                    <div class="mt-2 flex items-center gap-2">
                      <input
                        data-slot="share-link-input"
                        class="border-border bg-background text-foreground focus:border-primary min-w-0 flex-auto rounded-md border px-2.5 py-[7px] font-mono text-[12px] outline-none"
                        :value="absoluteInviteURL(shareLinkResult.invite_url || '')"
                        readonly
                        @click="($event.target as HTMLInputElement).select()"
                      />
                      <Button
                        size="sm"
                        variant="outline"
                        class="border-primary text-primary hover:text-primary"
                        @click="copyText(absoluteInviteURL(shareLinkResult.invite_url || ''))"
                      >
                        <CopyIcon />
                        {{ $t("tenantInvitation.copyLink") }}
                      </Button>
                    </div>
                  </div>
                  <div class="mt-4 flex justify-end gap-2">
                    <Button
                      v-if="!shareLinkResult"
                      variant="outline"
                      :disabled="creatingShareLink"
                      @click="shareLinkPopupVisible = false"
                    >
                      {{ $t("common.cancel") }}
                    </Button>
                    <Button v-else variant="outline" @click="shareLinkPopupVisible = false">
                      {{ $t("common.close") }}
                    </Button>
                    <Button v-if="!shareLinkResult" :disabled="creatingShareLink" @click="submitShareLink">
                      <Loader2Icon v-if="creatingShareLink" class="animate-spin" />
                      {{ $t("tenantInvitation.shareLink.generate") }}
                    </Button>
                  </div>
                </div>
              </PopoverContent>
            </Popover>
          </div>
        </div>
        <div v-if="loading && members.length === 0" class="flex items-center gap-2 pt-5 pb-2">
          <Loader2Icon class="text-primary size-4 animate-spin" />
          <span>{{ $t("tenantMember.loading") }}</span>
        </div>
        <div v-else-if="error" class="flex items-center gap-2 pt-5 pb-2">
          <Alert variant="destructive">
            <CircleAlertIcon />
            <AlertDescription>{{ error }}</AlertDescription>
            <AlertAction>
              <Button size="sm" variant="outline" @click="loadMembers">{{ $t("tenantMember.retry") }}</Button>
            </AlertAction>
          </Alert>
        </div>
        <div v-else-if="membersTotal === 0" class="flex justify-center pt-10 pb-4">
          <Empty class="p-0">
            <EmptyDescription>{{
              searchQuery.trim() ? $t("tenantMember.emptySearch", { q: searchQuery }) : $t("tenantMember.empty")
            }}</EmptyDescription>
          </Empty>
        </div>
        <div v-else :class="[tableShellClass, 'bg-card']">
          <div class="relative">
            <Table class="min-w-[502px] table-fixed">
              <TableHeader>
                <TableRow :class="tableHeadRowClass">
                  <TableHead :class="[tableHeadClass, 'min-w-[132px]']">
                    {{ $t("tenantMember.columns.member") }}
                  </TableHead>
                  <TableHead :class="[tableHeadClass, 'w-[128px]']">{{ $t("tenantMember.columns.role") }}</TableHead>
                  <TableHead :class="[tableHeadClass, 'w-[154px]']">
                    {{ $t("tenantMember.columns.joinedAt") }}
                  </TableHead>
                  <TableHead :class="[tableHeadClass, 'w-[88px]']">
                    {{ $t("tenantMember.columns.operations") }}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-for="row in members" :key="row.user_id" class="even:bg-secondary hover:bg-accent">
                  <TableCell :class="tableCellClass">
                    <div :class="memberCellClass">
                      <span :class="[memberNameClass, 'text-foreground']">{{ memberPrimary(row) }}</span>
                      <span v-if="memberSecondary(row)" :class="memberEmailClass">{{ memberSecondary(row) }}</span>
                    </div>
                  </TableCell>
                  <TableCell :class="tableCellClass">
                    <!-- 角色列：下拉收缩到格宽以内，与其他列对齐。 -->
                    <div class="box-border flex min-w-0 items-center">
                      <Select
                        v-if="canManage && row.user_id !== currentUserId"
                        :model-value="row.role"
                        @update:model-value="(v) => onRoleChange(row, String(v ?? ''))"
                      >
                        <SelectTrigger size="sm" class="w-full text-[13px]">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent position="popper" :class="selectLayerClass">
                          <SelectItem v-for="opt in roleOptions" :key="opt.value" :value="opt.value">
                            <span class="inline-flex items-center gap-2">
                              <component :is="roleIcon(opt.value)" class="text-muted-foreground size-3.5" />
                              <span>{{ opt.label }}</span>
                            </span>
                          </SelectItem>
                        </SelectContent>
                      </Select>
                      <span v-else :class="[tagBaseClass, roleTagClass(row.role)]">
                        {{ $t("tenantMember.role." + row.role) }}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell :class="tableCellClass">{{ formatDate(row.joined_at) }}</TableCell>
                  <TableCell :class="tableCellClass">
                    <Popover v-if="canManage && row.user_id !== currentUserId">
                      <Tooltip>
                        <TooltipTrigger as-child>
                          <PopoverTrigger as-child>
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              :class="dangerIconButtonClass"
                              :aria-label="$t('tenantMember.remove.button')"
                              @click.stop
                            >
                              <UserXIcon />
                            </Button>
                          </PopoverTrigger>
                        </TooltipTrigger>
                        <TooltipContent side="top" :class="tooltipLayerClass">{{
                          $t("tenantMember.remove.button")
                        }}</TooltipContent>
                      </Tooltip>
                      <PopoverContent side="left" :class="popconfirmClass">
                        <div class="flex items-start gap-2">
                          <InfoIcon class="text-primary mt-0.5 size-4 shrink-0" />
                          <p class="text-foreground m-0 text-[14px] leading-[1.5]">
                            {{ $t("tenantMember.remove.confirmBody", { name: row.username || row.email }) }}
                          </p>
                        </div>
                        <div class="flex justify-end gap-2">
                          <PopoverClose as-child>
                            <Button variant="outline" size="sm">{{ $t("common.cancel") }}</Button>
                          </PopoverClose>
                          <PopoverClose as-child>
                            <Button size="sm" :class="dangerSolidButtonClass" @click="removeRow(row)">
                              {{ $t("tenantMember.remove.confirm") }}
                            </Button>
                          </PopoverClose>
                        </div>
                      </PopoverContent>
                    </Popover>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
            <!-- A reload over rows already on screen dims them instead of blanking the table. -->
            <div v-if="loading" class="bg-card/60 absolute inset-0 flex items-center justify-center">
              <Loader2Icon class="text-primary size-5 animate-spin" />
            </div>
          </div>
          <div v-if="membersTotal > 0" :class="[pagerBarClass, 'bg-card']">
            <TenantMembersPager
              v-model:page="membersPage"
              v-model:page-size="membersPageSize"
              :total="membersTotal"
              :page-size-options="MEMBERS_PAGE_SIZE_OPTIONS"
              @change="onMembersPageChange"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- Audit log drawer. Only rendered for Admin+ because the backend
         route is g.Admin()-gated; rendering it for lower roles would
         just produce an unhelpful 403. Lazy-loaded on first open. -->
    <Drawer
      v-if="canViewAudit"
      :open="auditDrawerVisible"
      swipe-direction="right"
      @update:open="(v: boolean) => (auditDrawerVisible = v)"
    >
      <!--
        z-[2500] lifts the panel over the settings modal (z 1100) that hosts
        this page; the ui drawer's own z-50 would sit underneath it. The width is set through the same data-[swipe-direction=right] variants
        the ui drawer uses for its default 3/4 width, so that they replace it
        rather than lose to its higher specificity.
      -->
      <DrawerContent
        class="z-[2500] data-[swipe-direction=right]:w-[880px] data-[swipe-direction=right]:max-w-[100vw] data-[swipe-direction=right]:rounded-l-none data-[swipe-direction=right]:sm:max-w-[100vw]"
      >
        <header class="border-border flex items-center gap-2.5 border-b px-[18px] py-3.5">
          <div class="bg-primary/10 text-primary flex size-8 shrink-0 items-center justify-center rounded-[9px]">
            <HistoryIcon class="size-4" />
          </div>
          <DrawerTitle class="text-foreground m-0 min-w-0 flex-1 truncate text-[15px] leading-[1.4] font-semibold">
            {{ $t("tenantMember.audit.tabLabel") }}
          </DrawerTitle>
          <DrawerClose as-child>
            <Button variant="ghost" size="icon-sm" :aria-label="$t('common.close')">
              <XIcon />
            </Button>
          </DrawerClose>
        </header>

        <div class="box-border flex min-h-0 w-full flex-auto flex-col gap-3.5 px-[18px] py-4">
          <div class="bg-secondary flex items-center justify-between gap-3 rounded-lg px-4 py-3">
            <span class="text-muted-foreground min-w-0 flex-1 text-[13px]">{{
              $t("tenantMember.audit.description")
            }}</span>
            <Button variant="ghost" size="sm" class="shrink-0" :disabled="auditLoading" @click="reloadAuditLog">
              <Loader2Icon v-if="auditLoading" class="animate-spin" />
              <RefreshCwIcon v-else />
              {{ $t("tenantMember.audit.refresh") }}
            </Button>
          </div>

          <div class="flex min-h-0 flex-auto flex-col">
            <div v-if="auditError" class="flex min-h-0 flex-auto flex-col justify-center">
              <div class="flex w-full items-center gap-2 pt-5 pb-2">
                <Alert variant="destructive">
                  <CircleAlertIcon />
                  <AlertDescription>{{ auditError }}</AlertDescription>
                  <AlertAction>
                    <Button size="sm" variant="outline" @click="reloadAuditLog">
                      {{ $t("tenantMember.retry") }}
                    </Button>
                  </AlertAction>
                </Alert>
              </div>
            </div>

            <div
              v-else-if="!auditLoading && auditEntries.length === 0"
              class="flex min-h-0 flex-auto flex-col items-center justify-center px-3 py-6"
            >
              <Empty class="p-0">
                <EmptyDescription>{{ $t("tenantMember.audit.empty") }}</EmptyDescription>
              </Empty>
            </div>

            <!--
              This area is the vertical scroller: the IntersectionObserver
              uses it as root, and the sticky header pins to it. The table
              is a bare <table> rather than ui/Table because that wrapper is
              a scroll container of its own, which would capture the sticky
              header.
            -->
            <div v-else ref="auditScrollRoot" class="flex min-h-0 flex-auto flex-col overflow-auto">
              <div :class="[tableShellClass, 'bg-card shrink-0 overflow-clip']">
                <table data-slot="table" class="w-full min-w-[906px] table-fixed caption-bottom text-sm">
                  <TableHeader>
                    <TableRow class="hover:bg-transparent">
                      <TableHead :class="[tableHeadClass, auditStickyHeadClass, 'w-9']" />
                      <TableHead :class="[tableHeadClass, auditStickyHeadClass, 'w-[120px]']">
                        {{ $t("tenantMember.audit.columns.time") }}
                      </TableHead>
                      <TableHead :class="[tableHeadClass, auditStickyHeadClass, 'w-[180px]']">
                        {{ $t("tenantMember.audit.columns.actor") }}
                      </TableHead>
                      <TableHead :class="[tableHeadClass, auditStickyHeadClass, 'w-[130px]']">
                        {{ $t("tenantMember.audit.columns.action") }}
                      </TableHead>
                      <TableHead :class="[tableHeadClass, auditStickyHeadClass, 'min-w-[200px]']">
                        {{ $t("tenantMember.audit.columns.target") }}
                      </TableHead>
                      <TableHead :class="[tableHeadClass, auditStickyHeadClass, 'min-w-[160px]']">
                        {{ $t("tenantMember.audit.columns.path") }}
                      </TableHead>
                      <TableHead :class="[tableHeadClass, auditStickyHeadClass, 'w-[80px] text-center']">
                        {{ $t("tenantMember.audit.columns.outcome") }}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <template v-for="row in auditEntries" :key="row.id">
                      <!-- The whole row toggles its detail panel, as expand-on-row-click did. -->
                      <TableRow
                        class="hover:bg-accent cursor-pointer"
                        :aria-expanded="auditExpandedRowKeys.includes(row.id)"
                        @click="toggleAuditRow(row.id)"
                      >
                        <TableCell :class="[auditCellClass, 'px-2 text-center']">
                          <ChevronRightIcon
                            class="text-muted-foreground inline size-4 transition-transform duration-200"
                            :class="{ 'rotate-90': auditExpandedRowKeys.includes(row.id) }"
                          />
                        </TableCell>
                        <TableCell :class="auditCellClass">
                          <!-- Stacked "date / time": fifty events in one minute stay distinguishable. -->
                          <div class="flex flex-col gap-0.5 leading-[1.3]">
                            <span class="text-muted-foreground text-[12px]">{{
                              formatAuditDatePart(row.created_at)
                            }}</span>
                            <span class="text-foreground text-[13px] font-medium tabular-nums">{{
                              formatAuditTimePart(row.created_at)
                            }}</span>
                          </div>
                        </TableCell>
                        <TableCell :class="auditCellClass">
                          <div class="flex min-w-0 flex-col gap-0.5 leading-[1.3]">
                            <span class="text-foreground truncate text-[13px] font-medium">
                              {{
                                row.actor_user_id
                                  ? actorDisplayName(row.actor_user_id)
                                  : $t("tenantMember.audit.systemActor")
                              }}
                            </span>
                            <span v-if="row.actor_role" class="text-muted-foreground text-[12px]">
                              {{ $t("tenantMember.role." + row.actor_role) }}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell :class="auditCellClass">
                          <span :class="[tagBaseClass, 'border', auditActionClass(row.action)]">
                            {{ formatAuditAction(row.action) }}
                          </span>
                        </TableCell>
                        <TableCell :class="[auditCellClass, 'whitespace-normal']">
                          <div class="flex min-w-0 flex-col gap-1 py-0.5 leading-[1.35]">
                            <span v-if="auditTargetSubject(row)" class="text-foreground text-[13px] break-all">{{
                              auditTargetSubject(row)
                            }}</span>
                            <span
                              v-if="auditTargetDiff(row)"
                              class="text-muted-foreground font-[family-name:var(--td-font-family-mono,monospace)] text-[12px] leading-[1.4] break-all"
                              >{{ auditTargetDiff(row) }}</span
                            >
                            <span v-else-if="!auditTargetSubject(row)" class="text-placeholder">—</span>
                          </div>
                        </TableCell>
                        <TableCell :class="[auditCellClass, 'whitespace-normal']">
                          <span
                            v-if="row.request_path"
                            class="text-muted-foreground font-[family-name:var(--td-font-family-mono,monospace)] text-[12px] break-all"
                          >
                            <span v-if="row.request_method" class="text-foreground mr-1 inline-block font-semibold">{{
                              row.request_method
                            }}</span>
                            {{ row.request_path }}
                          </span>
                          <span v-else class="text-placeholder">—</span>
                        </TableCell>
                        <TableCell :class="[auditCellClass, 'text-center']">
                          <span :class="[tagBaseClass, auditOutcomeClass(row.outcome)]">
                            {{ $t("tenantMember.audit.outcome." + row.outcome) }}
                          </span>
                        </TableCell>
                      </TableRow>
                      <!-- Expandable row body. Background steps off-card so the nested
                           context is clearly distinct from the row strip above it. -->
                      <TableRow v-if="auditExpandedRowKeys.includes(row.id)" class="hover:bg-transparent">
                        <TableCell colspan="7" class="p-0 whitespace-normal">
                          <div class="bg-accent flex flex-col gap-3 px-4 py-3">
                            <div class="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-x-[18px] gap-y-2.5">
                              <div class="flex min-w-0 flex-col gap-0.5">
                                <span :class="auditExpandedLabelClass">{{
                                  $t("tenantMember.audit.expanded.actorId")
                                }}</span>
                                <span :class="auditExpandedValueClass">{{ row.actor_user_id || "—" }}</span>
                              </div>
                              <div v-if="row.target_user_id" class="flex min-w-0 flex-col gap-0.5">
                                <span :class="auditExpandedLabelClass">{{
                                  $t("tenantMember.audit.expanded.targetUserId")
                                }}</span>
                                <span :class="auditExpandedValueClass">{{ row.target_user_id }}</span>
                              </div>
                              <div v-if="row.target_type" class="flex min-w-0 flex-col gap-0.5">
                                <span :class="auditExpandedLabelClass">{{
                                  $t("tenantMember.audit.expanded.targetType")
                                }}</span>
                                <span :class="auditExpandedValueClass">{{ row.target_type }}</span>
                              </div>
                              <div v-if="row.target_id" class="flex min-w-0 flex-col gap-0.5">
                                <span :class="auditExpandedLabelClass">{{
                                  $t("tenantMember.audit.expanded.targetId")
                                }}</span>
                                <span :class="auditExpandedValueClass">{{ row.target_id }}</span>
                              </div>
                            </div>
                            <div class="flex flex-col gap-1">
                              <span :class="auditExpandedLabelClass">{{
                                $t("tenantMember.audit.expanded.details")
                              }}</span>
                              <pre
                                class="border-border bg-card text-foreground m-0 max-h-[280px] overflow-auto rounded-md border px-3 py-2.5 font-[family-name:var(--td-font-family-mono,ui-monospace,SFMono-Regular,Menlo,Consolas,monospace)] text-[12px] leading-[1.55] break-all whitespace-pre-wrap"
                                >{{ auditDetailsJSON(row) }}</pre>
                            </div>
                          </div>
                        </TableCell>
                      </TableRow>
                    </template>
                  </TableBody>
                </table>
              </div>

              <!-- 触底 sentinel：IntersectionObserver root 指向滚动区 -->
              <div ref="auditLoadSentinelEl" class="pointer-events-none h-px w-full shrink-0" aria-hidden="true" />

              <div
                v-if="auditLoading && auditEntries.length > 0"
                class="text-muted-foreground flex items-center justify-center gap-2.5 p-3 text-[12px]"
              >
                <Loader2Icon class="text-primary size-4 animate-spin" />
                <span>{{ $t("tenantMember.loading") }}</span>
              </div>

              <p
                v-if="!auditHasMore && auditEntries.length > 0 && !auditLoading"
                class="m-0 pt-2 pb-3.5 text-center text-[12px] text-[var(--td-text-color-disabled)]"
              >
                {{ $t("tenantMember.audit.end") }}
              </p>
            </div>
          </div>
        </div>
      </DrawerContent>
    </Drawer>
  </div>
</template>

<script setup lang="ts">
import { docsUrl } from "@/config/externalLinks";
import { computed, nextTick, onUnmounted, reactive, ref, watch, type Component } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import { copyWithToast } from "@/utils/clipboard";
import { useAuthStore } from "@/stores/auth";
import { AUDIT_ACTION_I18N_ROOTS } from "@/i18n/auditActionRegistry";
import { auditActionLabel } from "@/i18n/auditActionLabel";
import { listMembers, updateMemberRole, removeMember, type TenantMember, type TenantRole } from "@/api/tenant/members";
import {
  listTenantInvitations,
  createInvitation,
  createInviteLink,
  revokeInvitation,
  type TenantInvitation,
} from "@/api/tenant/invitations";
import { listAuditLog, type AuditLog, type AuditAction, type AuditOutcome } from "@/api/tenant/audit-log";
import {
  CheckIcon,
  ChevronRightIcon,
  CircleAlertIcon,
  CircleXIcon,
  CopyIcon,
  CrownIcon,
  EyeIcon,
  HistoryIcon,
  InfoIcon,
  LinkIcon,
  Loader2Icon,
  PencilIcon,
  RefreshCwIcon,
  SearchIcon,
  ShieldCheckIcon,
  UserIcon,
  UserPlusIcon,
  UserXIcon,
  XIcon,
} from "@lucide/vue";
import { PopoverClose } from "reka-ui";
import { Alert, AlertAction, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Drawer, DrawerClose, DrawerContent, DrawerTitle } from "@/components/ui/drawer";
import { Empty, EmptyDescription } from "@/components/ui/empty";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { TableBody, TableCell, TableHead, TableHeader, TableRow, Table } from "@/components/ui/table";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import TenantMembersPager from "./TenantMembersPager.vue";

const { t, tm, locale } = useI18n();
const authStore = useAuthStore();

// ---- Shared class strings ------------------------------------------------
// The two tables, their pagers and the floating panels repeat the same
// look several times over; naming each once keeps the template readable
// and the copies from drifting apart.

/**
 * The floating panels (permission matrix, invite, share link) sit above the
 * settings modal (z 1100) and its full-screen overlays, as the old overlay
 * classes did with z-index 3050. The glassy card surface is theirs too, with
 * a darker translucent ground in dark mode.
 */
const popupSurfaceClass =
  "z-[3050] rounded-xl bg-card ring-0 border-[0.5px] border-border backdrop-blur-[20px] backdrop-saturate-[180%] " +
  "shadow-[0_0_0_0.5px_rgba(0,0,0,0.03),0_2px_4px_rgba(0,0,0,0.04),0_8px_24px_rgba(0,0,0,0.1)] " +
  "dark:border-white/8 dark:bg-[rgba(36,36,36,0.92)] " +
  "dark:shadow-[0_0_0_0.5px_rgba(255,255,255,0.05),0_2px_4px_rgba(0,0,0,0.12),0_8px_32px_rgba(0,0,0,0.28)]";
const invitePopupClass = "w-auto min-w-[300px] max-w-[min(392px,calc(100vw-24px))] px-4 py-3.5";
/** Inline confirmations anchored to a row action (the old t-popconfirm). */
const popconfirmClass = "z-[3050] w-[min(320px,calc(100vw-24px))] gap-3 p-3";
/** Select menus open from inside the invite popovers, so they must clear those too. */
const selectLayerClass = "z-[6200]";
const tooltipLayerClass = "z-[3100]";

const countBadgeClass =
  "inline-flex h-5 min-w-[22px] items-center justify-center rounded-[10px] bg-secondary px-[7px] text-[12px] leading-none font-semibold text-foreground";
const tableShellClass = "flex flex-col overflow-hidden rounded-[10px] border border-border";
const tableHeadRowClass = "bg-secondary hover:bg-secondary";
const tableHeadClass = "px-4 py-3 text-[13px] font-semibold text-muted-foreground";
const tableCellClass = "px-4 py-3 text-[14px] text-foreground";
const auditCellClass = "px-4 py-3.5 align-middle text-[14px] text-foreground";
// Sticky thead so the column labels survive long scrolls; the inset
// shadow replaces the row separator that would otherwise scroll away.
const auditStickyHeadClass = "sticky top-0 z-[2] bg-secondary shadow-[inset_0_-1px_0_var(--td-component-stroke)]";
const pagerBarClass =
  "flex shrink-0 flex-wrap items-center justify-end gap-x-3 gap-y-2 border-t border-border px-3.5 py-2.5";
const memberCellClass = "flex min-w-0 flex-col gap-0.5 py-0.5";
const memberNameClass = "truncate text-[14px] font-medium";
const memberEmailClass = "truncate text-[12px] leading-[1.35] text-muted-foreground";
const auditExpandedLabelClass = "text-[11px] font-semibold tracking-[0.04em] text-muted-foreground uppercase";
const auditExpandedValueClass =
  "font-[family-name:var(--td-font-family-mono,ui-monospace,SFMono-Regular,Menlo,Consolas,monospace)] text-[12px] break-all text-foreground";
const dangerIconButtonClass = "text-destructive hover:bg-destructive/10 hover:text-destructive";
const dangerSolidButtonClass = "bg-destructive text-primary-foreground hover:bg-destructive/90";
const inviteBodyClass = "pt-1 pb-2 text-[14px] leading-[1.6] text-foreground";
// The invite form used a fixed 80px label column beside each control.
const formItemClass = "mb-3.5 grid grid-cols-[80px_minmax(0,1fr)] items-start gap-x-3";
const formLabelClass = "min-h-8 text-[14px] leading-8 font-normal text-foreground";
/** The small TDesign tag every chip in these tables was drawn with. */
const tagBaseClass =
  "inline-flex h-5 items-center rounded-[var(--td-radius-small)] px-1.5 text-[12px] leading-none whitespace-nowrap";

// The permission matrix opened on hover under TDesign. Reka's popover is
// click-driven, so the hover is added here: entering the icon or the panel
// opens it, and leaving either closes it after a short grace period long
// enough for the pointer to cross the gap between the two.
const permissionsPopupOpen = ref(false);
let permissionsPopupCloseTimer: number | undefined;

// Whether the pointer is over the icon or the panel. While it is, a close
// request from the popover itself is ignored: that is a click on the icon,
// which toggles a Reka popover but never closed the hover-driven t-popup.
let permissionsPopupHovered = false;

function openPermissionsPopup() {
  permissionsPopupHovered = true;
  window.clearTimeout(permissionsPopupCloseTimer);
  permissionsPopupOpen.value = true;
}

function schedulePermissionsPopupClose() {
  permissionsPopupHovered = false;
  window.clearTimeout(permissionsPopupCloseTimer);
  permissionsPopupCloseTimer = window.setTimeout(() => {
    permissionsPopupOpen.value = false;
  }, 150);
}

function onPermissionsPopupOpenChange(open: boolean) {
  if (!open && permissionsPopupHovered) return;
  permissionsPopupOpen.value = open;
}

// State
const members = ref<TenantMember[]>([]);
const loading = ref(false);
const error = ref("");
const adding = ref(false);
/** 邀请流程：锚在列表头「+」按钮旁的弹出层（非居中模态）。 */
const invitePopupVisible = ref(false);
// share-link generator state (separate popup next to the email
// invite). shareLinkResult is non-null after a successful create —
// the popup then switches into "here's your link, copy it" mode.
const shareLinkPopupVisible = ref(false);
const shareLinkForm = reactive<{ role: TenantRole }>({ role: "contributor" });
const creatingShareLink = ref(false);
const shareLinkResult = ref<TenantInvitation | null>(null);
// Two-step invite inside the popup: 'form' renders the email/role inputs;
// 'confirm' swaps the body for an in-place summary; primary CTA toggles label.
const addDialogStep = ref<"form" | "confirm">("form");
const searchQuery = ref("");
/** 已应用到服务端筛选的检索词（相对输入框防抖） */
const memberSearchQ = ref("");
let memberSearchDebounceTimer: number | undefined;

const membersTotal = ref(0);
const membersPage = ref(1);
const membersPageSize = ref(20);

const invitationsTotal = ref(0);
const invitationsPage = ref(1);
const invitationsPageSize = ref(20);

/** 历次分页载荷里见过的成员展示字段，补齐审计表里不在当前页的 user id */
const memberDisplayByUserId = reactive<Record<string, { username?: string; email?: string }>>({});

// Pending invitations live alongside members but in a distinct section
// at the top of the Members tab — they're "people we've asked to
// join but haven't yet accepted", and conflating them with the
// authoritative roster would mislead an Owner trying to see who
// actually has access. The load happens on the same trigger as the
// members fetch so the screen renders both at once.
const invitations = ref<TenantInvitation[]>([]);
const invitationsLoading = ref(false);
const invitationsError = ref("");
// Invitation TTL is mirrored from the backend constant
// (defaultInvitationTTL in tenant_invitation.go). Kept as a UI string
// for the section description; the authoritative number comes from
// the server's expires_at on each row.
const INVITATION_TTL_DAYS = 7;

const MEMBERS_PAGE_SIZE_OPTIONS = [10, 20, 50, 100];
const INVITATIONS_PAGE_SIZE_OPTIONS = [10, 20, 50, 100];

// Audit log moved out of t-tabs into a right-side drawer; this flag
// controls its visibility. Default closed — most operators come here
// to manage members, not to audit. Lazy-load logic is wired through
// openAuditDrawer() rather than a watch() on the visibility flag so a
// re-open of the drawer doesn't re-trigger a fetch the user didn't ask
// for (they have an explicit "refresh" button inside the drawer).
const auditDrawerVisible = ref(false);

// Audit-log state. Backend cursor-paged by descending id (`after_id`);
// frontend appends rows when the sentinel scrolls into view. When
// `next_cursor` is 0, `auditHasMore` becomes false and loading stops.
const auditEntries = ref<AuditLog[]>([]);
const auditLoading = ref(false);
const auditError = ref("");
const auditCursor = ref<number>(0); // 0 = "from the top"
const auditHasMore = ref(true);
const auditLoadedOnce = ref(false);
const AUDIT_PAGE_SIZE = 50;

/** 抽屉内滚动根与触底 sentinel，用于游标分页自动加载下一页（见 attachAuditInfiniteScroll） */
const auditScrollRoot = ref<HTMLElement | null>(null);
const auditLoadSentinelEl = ref<HTMLElement | null>(null);
let auditScrollObserver: IntersectionObserver | null = null;

// Add dialog model — reset on each open. Default role is contributor:
// inviting a fresh member with viewer is too restrictive for the
// expected "let them collaborate on KBs" use case, and admin/owner
// should be a deliberate promote step after the user accepts.
const addForm = reactive<{ email: string; role: TenantRole }>({
  email: "",
  role: "contributor",
});

// Role-aware gates. The server enforces every mutation; UI gates here
// are presentational only, matching the security note in stores/auth.ts.
const currentRole = computed<TenantRole | "">(() => (authStore.currentTenantRole || "") as TenantRole | "");
// Cross-tenant superusers (org-level operators) bypass the Owner gate
// on the server (see middleware/rbac.go RequireRole). The UI must
// mirror that or the buttons would be invisible to the exact admins
// who actually need them. Local Owners of their own tenant come in via
// the role branch.
const canManage = computed(() => currentRole.value === "owner" || authStore.canAccessAllTenants === true);
// Admin+ (and cross-tenant superusers) can view the audit log. Mirrors
// the server's g.Admin() guard on /tenants/:id/audit-log so we don't
// render a tab that would just 403.
const canViewAudit = computed(
  () => currentRole.value === "owner" || currentRole.value === "admin" || authStore.canAccessAllTenants === true,
);
const currentUserId = computed(() => authStore.user?.id ?? "");

// Use the active tenant id from the auth store; the route only allows
// :id == active tenant (auth middleware enforces membership), so we
// don't expose a tenant picker here.
const activeTenantId = computed(() => Number(authStore.currentTenantId ?? 0));

const roleOptions = computed(() => [
  { label: t("tenantMember.role.owner"), value: "owner" },
  { label: t("tenantMember.role.admin"), value: "admin" },
  { label: t("tenantMember.role.contributor"), value: "contributor" },
  { label: t("tenantMember.role.viewer"), value: "viewer" },
]);

// Static role-permissions matrix. The keys reference i18n strings under
// `tenantMember.permissions.*` so each locale can rephrase per culture.
// Keep this aligned with the design-doc §4.3 matrix and the actual
// PR 2 enforcement; if a permission moves between roles, update both
// sides in the same PR.
type RolePerm = { key: string; has: boolean };
const roleMatrixOrder: TenantRole[] = ["owner", "admin", "contributor", "viewer"];
const roleMatrix: Record<TenantRole, RolePerm[]> = {
  owner: [
    { key: "manageMembers", has: true },
    { key: "manageTenantConfig", has: true },
    { key: "manageInfra", has: true },
    { key: "createOwnKB", has: true },
    { key: "readAll", has: true },
  ],
  admin: [
    { key: "manageMembers", has: false },
    { key: "manageTenantConfig", has: false },
    { key: "manageInfra", has: true },
    { key: "createOwnKB", has: true },
    { key: "readAll", has: true },
  ],
  contributor: [
    { key: "manageMembers", has: false },
    { key: "manageTenantConfig", has: false },
    { key: "manageInfra", has: false },
    { key: "createOwnKB", has: true },
    { key: "readAll", has: true },
  ],
  viewer: [
    { key: "manageMembers", has: false },
    { key: "manageTenantConfig", has: false },
    { key: "manageInfra", has: false },
    { key: "createOwnKB", has: false },
    { key: "readAll", has: true },
  ],
};

function roleMatrixIcon(role: TenantRole): Component {
  switch (role) {
    case "owner":
      return CrownIcon;
    case "admin":
      return ShieldCheckIcon;
    case "contributor":
      return PencilIcon;
    default:
      return EyeIcon;
  }
}

function memberPrimary(row: { username?: string; email?: string }) {
  return row.username?.trim() || row.email?.trim() || "—";
}

function memberSecondary(row: { username?: string; email?: string }) {
  const name = row.username?.trim();
  const mail = row.email?.trim();
  if (name && mail) return mail;
  return "";
}

// The invite form's validation, ported from the t-form rules: the email is
// required and must look like an address (checked on blur and on submit),
// and a role must be chosen (checked on change and on submit). The messages
// are the ones the rules carried.
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const addFormErrors = reactive<{ email: string; role: string }>({ email: "", role: "" });

function validateAddEmail(): boolean {
  const value = addForm.email.trim();
  if (!value) addFormErrors.email = t("tenantMember.errors.emailRequired");
  else if (!EMAIL_PATTERN.test(value)) addFormErrors.email = t("tenantMember.errors.emailFormat");
  else addFormErrors.email = "";
  return !addFormErrors.email;
}

function validateAddRole(): boolean {
  addFormErrors.role = addForm.role ? "" : t("tenantMember.errors.roleRequired");
  return !addFormErrors.role;
}

function validateAddForm(): boolean {
  // Both run so that both messages show at once, as t-form's validate() did.
  const emailOk = validateAddEmail();
  const roleOk = validateAddRole();
  return emailOk && roleOk;
}

function onAddRoleChange(value: string) {
  addForm.role = value as TenantRole;
  validateAddRole();
}

// Pretty role tag colour: Owner stands out, Admin is warning, the rest
// stay neutral so the table doesn't become a confetti cannon. These are
// TDesign's solid ("dark") small tags, the variant t-tag defaulted to.
// utils/workspaceNotifyContent.ts mirrors this owner/admin/contributor map.
function roleTagClass(role: TenantRole): string {
  switch (role) {
    case "owner":
      return "bg-primary text-primary-foreground";
    case "admin":
      return "bg-warning text-primary-foreground";
    case "contributor":
      return "bg-success text-primary-foreground";
    default:
      return "bg-[var(--td-bg-color-component)] text-foreground";
  }
}

/** 成员表/下拉与权限矩阵共用图标。 */
function roleIcon(role: TenantRole | string): Component {
  if (role === "owner" || role === "admin" || role === "contributor" || role === "viewer") {
    return roleMatrixIcon(role as TenantRole);
  }
  return UserIcon;
}

function formatDate(s: string | undefined): string {
  if (!s) return "-";
  try {
    const d = new Date(s);
    return new Intl.DateTimeFormat(locale.value || "zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    }).format(d);
  } catch {
    return s;
  }
}

function rememberMembersForAudit(rows: TenantMember[]) {
  for (const m of rows) {
    memberDisplayByUserId[m.user_id] = { username: m.username, email: m.email };
  }
}

async function loadMembers() {
  if (!activeTenantId.value) {
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    const resp = await listMembers(activeTenantId.value, {
      page: membersPage.value,
      page_size: membersPageSize.value,
      q: memberSearchQ.value || undefined,
    });
    if (resp.success && resp.data) {
      const total = resp.data.total ?? 0;
      const ps = resp.data.page_size ?? membersPageSize.value;
      const safePs = Math.max(1, ps);
      const maxPage = Math.max(1, Math.ceil(total / safePs));
      if (membersPage.value > maxPage) {
        membersPage.value = maxPage;
        loading.value = false;
        await loadMembers();
        return;
      }
      members.value = resp.data.members ?? [];
      membersTotal.value = total;
      if (typeof resp.data.page === "number" && resp.data.page > 0) {
        membersPage.value = resp.data.page;
      }
      if (typeof resp.data.page_size === "number" && resp.data.page_size > 0) {
        membersPageSize.value = resp.data.page_size;
      }
      rememberMembersForAudit(members.value);
    } else {
      error.value = resp.message || t("tenantMember.errors.generic");
    }
  } catch (err: any) {
    error.value = err?.message || t("tenantMember.errors.generic");
  } finally {
    loading.value = false;
  }
}

function onMembersPageChange() {
  void loadMembers();
}

watch(searchQuery, () => {
  if (!activeTenantId.value) return;
  window.clearTimeout(memberSearchDebounceTimer);
  memberSearchDebounceTimer = window.setTimeout(() => {
    memberSearchQ.value = searchQuery.value.trim();
    membersPage.value = 1;
    loadMembers();
  }, 320);
});

// ---- Pending invitations ------------------------------------------------

// Status chips, solid like the role tags: pending is the brand colour,
// accepted green, declined/revoked orange, expired red.
function invitationStatusClass(s: TenantInvitation["status"]): string {
  switch (s) {
    case "pending":
      return "bg-primary text-primary-foreground";
    case "accepted":
      return "bg-success text-primary-foreground";
    case "declined":
    case "revoked":
      return "bg-warning text-primary-foreground";
    case "expired":
      return "bg-destructive text-primary-foreground";
    default:
      return "bg-[var(--td-bg-color-component)] text-foreground";
  }
}

function inviteePrimary(row: TenantInvitation): string {
  return row.invitee_name?.trim() || row.invitee_email?.trim() || row.invitee_user_id;
}

function inviterPrimary(row: TenantInvitation): string {
  return row.inviter_name?.trim() || row.inviter_email?.trim() || row.invited_by || "—";
}

// loadInvitations is called from the same trigger as loadMembers so
// the Members tab can render the pending section without an extra
// round-trip latency budget. canManage gates the fetch — viewers /
// contributors / admins-without-management see no pending section at
// all (the route returns 200 but listing pending invites to those
// roles would be UX noise; the route layer is Viewer+ for the read
// itself).
async function loadInvitations() {
  if (!activeTenantId.value || !canManage.value) {
    invitations.value = [];
    invitationsTotal.value = 0;
    return;
  }
  invitationsLoading.value = true;
  invitationsError.value = "";
  try {
    const resp = await listTenantInvitations(activeTenantId.value, {
      page: invitationsPage.value,
      page_size: invitationsPageSize.value,
    });
    if (resp.success && resp.data) {
      const total = resp.data.total ?? 0;
      const ps = resp.data.page_size ?? invitationsPageSize.value;
      const safePs = Math.max(1, ps);
      const maxPage = Math.max(1, Math.ceil(total / safePs));
      if (invitationsPage.value > maxPage) {
        invitationsPage.value = maxPage;
        invitationsLoading.value = false;
        await loadInvitations();
        return;
      }
      invitations.value = resp.data.invitations ?? [];
      invitationsTotal.value = total;
      if (typeof resp.data.page === "number" && resp.data.page > 0) {
        invitationsPage.value = resp.data.page;
      }
      if (typeof resp.data.page_size === "number" && resp.data.page_size > 0) {
        invitationsPageSize.value = resp.data.page_size;
      }
    } else {
      invitationsError.value = resp.message || t("tenantInvitation.errors.generic");
    }
  } catch (err: any) {
    invitationsError.value = err?.message || t("tenantInvitation.errors.generic");
  } finally {
    invitationsLoading.value = false;
  }
}

function onInvitationsPageChange() {
  void loadInvitations();
}

// doRevokeInvitation is wired to the confirm button of the inline
// confirmation popover in the table template. The popconfirm itself owns the yes/no surface,
// so this function is the post-confirmation action only — no nested
// modal. Errors surface as toasts and the row stays in place for retry.
async function doRevokeInvitation(row: TenantInvitation) {
  try {
    const resp = await revokeInvitation(activeTenantId.value, row.id);
    if (resp.success) {
      await loadInvitations();
      MessagePlugin.success(t("tenantInvitation.revoke.success"));
    } else {
      MessagePlugin.error(resp.message || t("tenantInvitation.errors.generic"));
    }
  } catch (err: any) {
    const status = err?.status;
    if (status === 404) {
      MessagePlugin.error(t("tenantInvitation.errors.notFound"));
    } else if (status === 409) {
      MessagePlugin.error(err?.message || t("tenantInvitation.errors.notPending"));
    } else {
      MessagePlugin.error(err?.message || t("tenantInvitation.errors.generic"));
    }
  }
}

// ---- Audit-log helpers --------------------------------------------------

// Stacked "date / time" cell — mirrors SystemSettings audit table. When
// 50 events fall in the same minute, ellipsing a flat string makes them
// indistinguishable; splitting on two lines keeps the seconds visible
// without eating horizontal budget the diff column needs.
//
// The target column has no fixed width and no ellipsis: this is where the
// role-diff and denied-action context live. Wrap rather than clip — losing
// the "Owner → Admin" half of a role change defeats the point.

function formatAuditDatePart(s: string | undefined): string {
  if (!s) return "-";
  try {
    return new Intl.DateTimeFormat(locale.value || "zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
    }).format(new Date(s));
  } catch {
    return s;
  }
}

function formatAuditTimePart(s: string | undefined): string {
  if (!s) return "";
  try {
    return new Intl.DateTimeFormat(locale.value || "zh-CN", {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    }).format(new Date(s));
  } catch {
    return "";
  }
}

// Action chip colour: rejection events are loud (danger) so an
// operator can scan a chronological feed and immediately spot abuse;
// member adds are reassuring green; removals/role changes warning
// orange because they're worth a second look but aren't intrinsically
// suspicious.
// (Drawn as TDesign's light-outline tags: tinted ground, coloured border.)
function auditActionClass(action: AuditAction): string {
  switch (action) {
    case "rbac.access_denied":
      return "border-destructive/40 bg-[var(--td-error-color-light)] text-destructive";
    case "rbac.member_added":
      return "border-success/40 bg-[var(--td-success-color-light)] text-success";
    case "rbac.member_removed":
    case "rbac.member_left":
    case "rbac.member_role_changed":
      return "border-warning/40 bg-[var(--td-warning-color-light)] text-warning";
    default:
      return "border-border bg-secondary text-foreground";
  }
}

// Outcome chips are TDesign's light tags: tinted ground, no border.
function auditOutcomeClass(o: AuditOutcome): string {
  if (o === "denied") return "bg-[var(--td-error-color-light)] text-destructive";
  if (o === "success") return "bg-[var(--td-success-color-light)] text-success";
  return "bg-secondary text-foreground";
}

// i18n 键名含点号（rbac.member_added）。用 t(path) 会按路径拆开解析，
// 无法命中 tenantMember.audit.action['rbac.*'] — 必须用 tm + 字面量键。
function formatAuditAction(action: AuditAction): string {
  return auditActionLabel({ tm }, AUDIT_ACTION_I18N_ROOTS.tenantMember, action);
}

// Resolve a user id to a display label: prefer current页的 members，
// 再退到历次分页积累的 memberDisplayByUserId，最后是原始 id。
function actorDisplayName(userId: string): string {
  const cur = members.value.find((x) => x.user_id === userId);
  if (cur?.username?.trim()) return cur.username.trim();
  if (cur?.email?.trim()) return cur.email.trim();
  const memo = memberDisplayByUserId[userId];
  if (memo?.username?.trim()) return memo.username!.trim();
  if (memo?.email?.trim()) return memo.email!.trim();
  return userId;
}

// Split target rendering into a "subject" (who was acted on) and a
// "diff" (what changed). The cell template stacks them with the
// subject on the first line, diff on the second. Returns '' when a
// piece is unavailable so the v-if branches drop the wrapper cleanly.

function auditDetailsObject(row: AuditLog): Record<string, unknown> | null {
  if (row.details && typeof row.details === "object") {
    return row.details as Record<string, unknown>;
  }
  return null;
}

function auditTargetSubject(row: AuditLog): string {
  if (row.target_user_id) return actorDisplayName(row.target_user_id);
  if (row.target_id) {
    return row.target_type ? `${row.target_type}:${row.target_id}` : row.target_id;
  }
  return "";
}

function auditTargetDiff(row: AuditLog): string {
  const d = auditDetailsObject(row);
  if (!d) return "";
  if (row.action === "rbac.member_role_changed") {
    if (d.old_role && d.new_role) return `${d.old_role} → ${d.new_role}`;
  }
  if (row.action === "rbac.access_denied") {
    if (typeof d.required_role === "string") {
      return t("tenantMember.audit.requiredRole", { role: d.required_role });
    }
  }
  if (row.action === "rbac.invitation_sent" || row.action === "rbac.invitation_revoked") {
    if (typeof d.role === "string") return String(d.role);
  }
  return "";
}

// Expanded row state — local set of ids the user has opened. We keep
// it ephemeral (not persisted) so reopening the drawer always starts
// in the collapsed view.
const auditExpandedRowKeys = ref<number[]>([]);

function onAuditExpandChange(value: (string | number)[]) {
  auditExpandedRowKeys.value = value
    .map((v) => (typeof v === "number" ? v : Number(v)))
    .filter((v) => Number.isFinite(v));
}

// A click anywhere on an audit row opens or closes its detail panel.
function toggleAuditRow(id: number) {
  const open = auditExpandedRowKeys.value.includes(id);
  onAuditExpandChange(open ? auditExpandedRowKeys.value.filter((k) => k !== id) : [...auditExpandedRowKeys.value, id]);
}

function auditDetailsJSON(row: AuditLog): string {
  if (row.details === null || row.details === undefined) return "{}";
  if (typeof row.details === "string") return row.details;
  try {
    return JSON.stringify(row.details, null, 2);
  } catch {
    return String(row.details);
  }
}

// loadAuditLog fetches a page. `reset=true` discards the current
// list and starts from cursor=0. Used by the refresh button and the
// initial tab-switch trigger.
async function loadAuditLog(reset: boolean) {
  if (!activeTenantId.value || !canViewAudit.value) return;
  if (auditLoading.value) return;
  if (!reset && !auditHasMore.value) return;

  auditLoading.value = true;
  auditError.value = "";
  try {
    const resp = await listAuditLog(activeTenantId.value, {
      after_id: reset ? undefined : auditCursor.value || undefined,
      limit: AUDIT_PAGE_SIZE,
    });
    if (resp.success) {
      const rows = resp.data || [];
      if (reset) {
        auditEntries.value = rows;
      } else {
        auditEntries.value = [...auditEntries.value, ...rows];
      }
      auditCursor.value = resp.next_cursor || 0;
      // The server returns next_cursor=0 when the page is empty OR
      // when the last row is the smallest possible id. Both mean
      // "stop paginating".
      auditHasMore.value = !!resp.next_cursor && rows.length > 0;
      auditLoadedOnce.value = true;
    } else {
      auditError.value = resp.message || t("tenantMember.errors.generic");
    }
  } catch (err: any) {
    const status = err?.status;
    if (status === 403) {
      auditError.value = t("tenantMember.audit.forbidden");
    } else {
      auditError.value = err?.message || t("tenantMember.errors.generic");
    }
  } finally {
    auditLoading.value = false;
  }
}

function detachAuditInfiniteScroll() {
  auditScrollObserver?.disconnect();
  auditScrollObserver = null;
}

function attachAuditInfiniteScroll() {
  detachAuditInfiniteScroll();
  const root = auditScrollRoot.value;
  const sentinel = auditLoadSentinelEl.value;
  if (!root || !sentinel) return;

  auditScrollObserver = new IntersectionObserver(
    (entries) => {
      const hitBottom = entries.some((e) => e.isIntersecting);
      if (!hitBottom || !auditHasMore.value || auditLoading.value) return;
      void loadAuditLog(false);
    },
    { root, rootMargin: "100px 0px", threshold: 0 },
  );
  auditScrollObserver.observe(sentinel);
}

function reloadAuditLog() {
  auditCursor.value = 0;
  auditHasMore.value = true;
  loadAuditLog(true);
}

// Lazy-load the audit log the first time the drawer is opened. We
// avoid watching `auditDrawerVisible` so closing+reopening the drawer
// doesn't re-fetch behind the user's back — refresh is an explicit
// action via the drawer's "Refresh" button.
function openAuditDrawer() {
  auditDrawerVisible.value = true;
  if (!auditLoadedOnce.value) {
    loadAuditLog(true);
  }
}

watch(
  auditDrawerVisible,
  async (open) => {
    if (!open) {
      detachAuditInfiniteScroll();
      return;
    }
    await nextTick();
    attachAuditInfiniteScroll();
  },
  { flush: "post" },
);

watch(
  () => auditError.value,
  async () => {
    if (!auditDrawerVisible.value) return;
    await nextTick();
    if (!auditError.value) {
      attachAuditInfiniteScroll();
      return;
    }
    detachAuditInfiniteScroll();
  },
  { flush: "post" },
);

onUnmounted(() => {
  detachAuditInfiniteScroll();
  window.clearTimeout(permissionsPopupCloseTimer);
});

watch(invitePopupVisible, (open) => {
  if (!open) return;
  addForm.email = "";
  addForm.role = "contributor";
  addFormErrors.email = "";
  addFormErrors.role = "";
  addDialogStep.value = "form";
});

// Share-link popup: re-init on every open so the operator never sees
// the previous result on a fresh click.
watch(shareLinkPopupVisible, (open) => {
  if (!open) return;
  shareLinkForm.role = "contributor";
  shareLinkResult.value = null;
});

// absoluteInviteURL turns the backend's potentially-host-relative
// invite_url into a copy-friendly absolute URL. The backend returns
// "/register?token=…" when FRONTEND_BASE_URL is unset (the typical
// case); the SPA is best-positioned to know its own origin.
function absoluteInviteURL(raw: string): string {
  if (!raw) return "";
  if (/^https?:\/\//i.test(raw)) return raw;
  const origin = (typeof window !== "undefined" && window.location && window.location.origin) || "";
  return raw.startsWith("/") ? origin + raw : origin + "/" + raw;
}

async function copyText(text: string) {
  await copyWithToast(text, "tenantInvitation.copied", "tenantInvitation.copyFailed");
}

async function submitShareLink() {
  creatingShareLink.value = true;
  try {
    const resp = await createInviteLink(activeTenantId.value, { role: shareLinkForm.role });
    if (!resp.success || !resp.data) {
      MessagePlugin.error(resp.message || t("tenantInvitation.errors.generic"));
      return;
    }
    shareLinkResult.value = resp.data;
    invitationsPage.value = 1;
    await loadInvitations();
  } catch (err: any) {
    MessagePlugin.error(err?.message || t("tenantInvitation.errors.generic"));
  } finally {
    creatingShareLink.value = false;
  }
}

// Live display strings for the in-place confirm step. Recomputed
// every time the user goes Back, tweaks the form, and re-advances —
// the summary always mirrors the current form state.
const addConfirmEmail = computed(() => addForm.email.trim());
const addConfirmRoleLabel = computed(() => t("tenantMember.role." + addForm.role));

// submitAdd is wired to the popup footer primary CTA. On step='form' it
// validates and swaps to summary; on step='confirm' it fires the API.
// With auto-accept the action is a direct add, so we skip the invitation
// confirm step entirely and fire the API right after validation.
async function submitAdd() {
  if (addDialogStep.value === "form") {
    if (!validateAddForm()) return;
    if (authStore.autoAcceptInvitation) {
      await sendInvitation(addForm.email.trim(), addForm.role);
      return;
    }
    addDialogStep.value = "confirm";
    return;
  }
  await sendInvitation(addForm.email.trim(), addForm.role);
}

// goBackToForm un-advances from confirm to form inside the popup.
function goBackToForm() {
  addDialogStep.value = "form";
}

// dialogConfirmLabel: "Send" on the confirm step, or when auto-accept makes
// the form step a direct add; otherwise "Send invitation".
const dialogConfirmLabel = computed(() =>
  addDialogStep.value === "confirm" || authStore.autoAcceptInvitation
    ? t("tenantInvitation.confirmSend")
    : t("tenantInvitation.inviteSubmit"),
);

// sendInvitation actually fires the create-invitation API call.
async function sendInvitation(email: string, role: TenantRole) {
  adding.value = true;
  try {
    const resp = await createInvitation(activeTenantId.value, { email, role });
    if (resp.success) {
      // auto-accept returns a member (user_id) instead of an invitation (id)
      const autoJoined = !!resp.data && "user_id" in resp.data;
      invitePopupVisible.value = false;
      if (autoJoined) {
        // The invitee is already a member — refresh the roster so they
        // appear immediately. No toast: the new row is the feedback.
        await loadMembers();
      } else {
        invitationsPage.value = 1;
        await loadInvitations();
        MessagePlugin.success(t("tenantInvitation.inviteSuccess"));
      }
    } else {
      MessagePlugin.error(resp.message || t("tenantInvitation.errors.generic"));
    }
  } catch (err: any) {
    const status = err?.status;
    if (status === 404) {
      MessagePlugin.error(t("tenantMember.errors.userNotFound"));
    } else if (status === 409) {
      // Server returns the same 409 for both "already a member" and
      // "already a pending invite". The message body discriminates,
      // but for the toast we show both possibilities folded into one
      // helpful line.
      MessagePlugin.error(
        err?.message || `${t("tenantInvitation.errors.alreadyMember")} / ${t("tenantInvitation.errors.pendingExists")}`,
      );
    } else if (status === 400) {
      MessagePlugin.error(err?.message || t("tenantMember.errors.invalidRole"));
    } else {
      MessagePlugin.error(err?.message || t("tenantInvitation.errors.generic"));
    }
  } finally {
    adding.value = false;
  }
}

async function onRoleChange(row: TenantMember, newRole: string) {
  const prev = row.role;
  const next = newRole as TenantRole;
  if (prev === next) return;

  try {
    const resp = await updateMemberRole(activeTenantId.value, row.user_id, next);
    if (resp.success) {
      // Mutate the row by replacing it in `members.value` instead of
      // assigning `row.role = next` in place. The `row` argument here
      // was once the row object handed in by t-table's slot scope, which
      // in some TDesign versions was a shallow copy that didn't share
      // reactivity with the `members` array. Splicing a fresh object into
      // the source array keeps working now that the rows are rendered
      // straight from `members`, and guarantees the table re-renders.
      const idx = members.value.findIndex((m) => m.user_id === row.user_id);
      if (idx >= 0) {
        const merged = { ...members.value[idx], role: next };
        members.value.splice(idx, 1, merged);
        rememberMembersForAudit([merged]);
      } else {
        row.role = next;
      }
      MessagePlugin.success(t("tenantMember.roleChange.success"));
      return;
    }
    MessagePlugin.error(resp.message || t("tenantMember.errors.generic"));
  } catch (err: any) {
    const status = err?.status;
    if (status === 409) {
      MessagePlugin.error(t("tenantMember.errors.lastOwner"));
    } else if (status === 404) {
      MessagePlugin.error(t("tenantMember.errors.notFound"));
    } else {
      MessagePlugin.error(err?.message || t("tenantMember.errors.generic"));
    }
    // The select is bound via :model-value (one-way), so its rendered
    // value stays at `prev` automatically — no DOM hack needed.
  }
}

// 原地 popconfirm 替代 DialogPlugin 模态确认：与"共享资源删除"等其它列表内
// 的删除入口风格统一，避免一个简单的二次确认打断成员管理表格的浏览节奏。
// 错误分支保持与旧实现一致（409 last-owner / 404 not-found / 兜底）。
async function removeRow(row: TenantMember) {
  try {
    const resp = await removeMember(activeTenantId.value, row.user_id);
    if (resp.success) {
      await loadMembers();
      MessagePlugin.success(t("tenantMember.remove.success"));
    } else {
      MessagePlugin.error(resp.message || t("tenantMember.errors.generic"));
    }
  } catch (err: any) {
    const status = err?.status;
    if (status === 409) {
      MessagePlugin.error(t("tenantMember.errors.lastOwner"));
    } else if (status === 404) {
      MessagePlugin.error(t("tenantMember.errors.notFound"));
    } else {
      MessagePlugin.error(err?.message || t("tenantMember.errors.generic"));
    }
  }
}

// Re-load whenever the active tenant resolves (or changes via the
// tenant switcher). onMounted alone would race with auth-store
// hydration — currentTenantId is often 0 at the moment this component
// mounts on a cold reload.
watch(
  activeTenantId,
  (id) => {
    if (id) {
      searchQuery.value = "";
      memberSearchQ.value = "";
      window.clearTimeout(memberSearchDebounceTimer);
      membersPage.value = 1;
      invitationsPage.value = 1;
      membersPageSize.value = 20;
      invitationsPageSize.value = 20;
      membersTotal.value = 0;
      invitationsTotal.value = 0;
      loadMembers();
      loadInvitations();
    }
  },
  { immediate: true },
);
</script>
