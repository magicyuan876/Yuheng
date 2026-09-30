<template>
  <!--
    SystemSettings — platform-wide tunables (system_settings table) for
    SystemAdmin. Gated server-side by RequireSystemAdmin middleware;
    the route also has meta.requiresSystemAdmin so non-admins never
    reach this component (see frontend/src/router/index.ts).

    UI principle: every control auto-persists, no Save button. The
    commit signal differs by control type so the user isn't surprised
    by writes while they're still composing:

      - Switch / Select (single-pick)         → commit on pick.
      - Input (int / string)                  → commit on blur.
      - SSRF whitelist (string_list)          → controlled tags input +
                                                 per-tag inline confirm.
      - System admins                         → tags input with
                                                 inline popconfirm per delta.

    auth.registration_mode triggers an
    inline confirm (same as Reset / bulk-apply) before persisting;
    cancelling rolls the in-progress edit back to the canonical value.
  -->
  <div class="w-full">
    <div class="mb-6">
      <h2 class="text-foreground m-0 mb-2 text-xl font-semibold">{{ t("system.globalSettings.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-[1.5]">
        {{ t("system.globalSettings.description") }}
      </p>
    </div>

    <div
      v-if="loading && settings.length === 0"
      class="text-placeholder flex items-center justify-center gap-2 py-[60px] text-[13px]"
    >
      <Loader2Icon class="size-4 animate-spin" />
      <span>{{ t("system.globalSettings.loading") }}</span>
    </div>

    <div
      v-else-if="settings.length === 0"
      class="text-placeholder flex items-center justify-center gap-2 py-[60px] text-[13px]"
    >
      <InfoIcon class="size-6" />
      <span>{{ t("system.globalSettings.empty") }}</span>
    </div>

    <template v-else>
      <div class="border-border bg-secondary mb-[18px] rounded-md border px-3.5 py-3">
        <div class="text-muted-foreground mb-2 flex items-center gap-[7px] text-[13px] font-medium">
          <InfoIcon class="text-primary size-4" />
          <span>{{ t("system.globalSettings.priorityHint.disclosure") }}</span>
        </div>
        <ul class="text-foreground m-0 list-disc pl-5 text-[13px] leading-[1.65] [&>li+li]:mt-1">
          <li>{{ t("system.globalSettings.priorityHint.tier1") }}</li>
          <li>{{ t("system.globalSettings.priorityHint.tier2") }}</li>
          <li>{{ t("system.globalSettings.priorityHint.tier3") }}</li>
        </ul>
      </div>

      <Tabs v-model="activeSettingsSection" class="w-full">
        <TabsList
          variant="line"
          class="bg-card sticky top-0 z-[3] mb-[18px] w-full justify-start gap-0 p-0 shadow-[0_1px_0_var(--td-component-stroke)] group-data-horizontal/tabs:h-12"
        >
          <TabsTrigger
            v-for="section in visibleSections"
            :key="section"
            :value="section"
            class="text-muted-foreground data-active:text-primary dark:data-active:text-primary after:bg-primary h-full flex-none rounded-none px-4 group-data-horizontal/tabs:after:bottom-0"
          >
            {{ sectionTabLabel(section) }}
          </TabsTrigger>
        </TabsList>
      </Tabs>

      <section class="min-w-0" :aria-labelledby="`settings-section-${activeSettingsSection}`">
        <div
          class="flex items-start justify-between gap-4 pb-3 max-[860px]:flex-col"
          :class="activeSettingsSection === 'runtime' ? 'border-b-0' : 'border-border border-b'"
        >
          <div>
            <h3
              :id="`settings-section-${activeSettingsSection}`"
              class="text-foreground m-0 mb-1 text-base leading-[1.4] font-bold"
            >
              {{ activeSectionTitle }}
            </h3>
            <p class="text-muted-foreground m-0 text-[13px] leading-[1.5]">{{ activeSectionDescription }}</p>
          </div>
          <Badge v-if="activeSettingsSection === 'runtime'" class="shrink-0" :class="settingTagClass('warning')">
            {{ t("system.globalSettings.sections.runtime.restartHint") }}
          </Badge>
        </div>

        <div
          v-if="activeSettingsSection === 'runtime'"
          class="border-border bg-secondary text-muted-foreground grid grid-cols-[minmax(0,1fr)_280px] gap-6 rounded-t-lg border border-b-0 px-4 py-2.5 text-xs font-medium max-[860px]:hidden"
          aria-hidden="true"
        >
          <span>{{ t("system.globalSettings.runtimeTable.setting") }}</span>
          <span class="text-right">{{ t("system.globalSettings.runtimeTable.value") }}</span>
        </div>

        <div
          class="flex flex-col"
          :class="
            activeSettingsSection === 'runtime'
              ? 'border-border overflow-hidden rounded-b-lg border max-[860px]:rounded-lg'
              : ''
          "
        >
          <!--
        System-admins management. Visually identical to SSRF whitelist
        (a tags input with one entry per email). NOT a system_setting
        row — it's backed by the user table via promote/revoke APIs.
        We sit it at the top because changing who can edit this page
        is structurally more important than tweaking any value below.
        Self-edit safety: the current user is excluded from the visible
        tags (they can't revoke themselves anyway, and showing a tag
        that can't be removed is worse than not showing it).
      -->
          <div
            v-if="activeSettingsSection === 'access'"
            class="border-border flex items-start justify-between border-b py-5 last:border-b-0 max-[860px]:flex-col max-[860px]:gap-3"
          >
            <div class="max-w-[65%] flex-1 pr-6 max-[860px]:w-full max-[860px]:max-w-none max-[860px]:pr-0">
              <div
                class="text-foreground mb-1 flex flex-wrap items-center gap-1.5 text-[15px] leading-[1.4] font-medium"
              >
                <span>{{ t("system.globalSettings.admins.label") }}</span>
                <Badge :class="settingTagClass('danger')">{{ t("system.globalSettings.badgeHighRisk") }}</Badge>
              </div>
              <p class="text-muted-foreground m-0 max-w-[480px] text-[13px] leading-[1.5]">
                {{ t("system.globalSettings.admins.description") }}
              </p>
            </div>
            <div
              class="flex min-w-[280px] shrink-0 flex-col items-end gap-1.5 max-[860px]:w-full max-[860px]:items-start"
            >
              <div class="flex items-center justify-end gap-2 max-[860px]:w-full max-[860px]:justify-start">
                <Popover :open="adminPopconfirm.visible" @update:open="onAdminPopoverOpen">
                  <PopoverAnchor as-child>
                    <div class="min-w-0 flex-1">
                      <TagsFieldInput
                        v-model="adminEmails"
                        class="w-[320px] max-[860px]:w-full"
                        :placeholder="t('system.globalSettings.admins.placeholder')"
                        :aria-label="t('system.globalSettings.admins.label')"
                        :disabled="adminBusy"
                        @change="onAdminsChange"
                      />
                    </div>
                  </PopoverAnchor>
                  <PopoverContent side="left" class="w-72">
                    <p class="mb-3 text-[13px] leading-[1.5]">{{ adminPopconfirm.content }}</p>
                    <div class="flex justify-end gap-2">
                      <Button size="sm" variant="outline" @click="adminPopconfirm.finish(false)">
                        {{ t("system.globalSettings.confirm.cancelBtn") }}
                      </Button>
                      <Button
                        size="sm"
                        :class="confirmBtnClass(adminPopconfirm.confirmBtn.theme)"
                        @click="adminPopconfirm.finish(true)"
                      >
                        {{ adminPopconfirm.confirmBtn.content }}
                      </Button>
                    </div>
                  </PopoverContent>
                </Popover>
                <div
                  v-if="adminBusy"
                  class="text-muted-foreground inline-flex min-w-[52px] shrink-0 items-center gap-[5px] text-xs"
                  role="status"
                >
                  <Loader2Icon class="size-3.5 animate-spin" />
                  <span>{{ t("system.globalSettings.saving") }}</span>
                </div>
              </div>
            </div>
          </div>

          <div
            v-if="activeSettingsSection === 'access'"
            class="border-border flex items-start justify-between border-b py-5 last:border-b-0 max-[860px]:flex-col max-[860px]:gap-3"
          >
            <div class="max-w-[65%] flex-1 pr-6 max-[860px]:w-full max-[860px]:max-w-none max-[860px]:pr-0">
              <div
                class="text-foreground mb-1 flex flex-wrap items-center gap-1.5 text-[15px] leading-[1.4] font-medium"
              >
                <span>{{ t("system.globalSettings.passwordReset.label") }}</span>
                <Badge :class="settingTagClass('danger')">{{ t("system.globalSettings.badgeHighRisk") }}</Badge>
              </div>
              <p class="text-muted-foreground m-0 max-w-[480px] text-[13px] leading-[1.5]">
                {{ t("system.globalSettings.passwordReset.description") }}
              </p>
            </div>
            <div
              class="flex min-w-[280px] shrink-0 flex-col items-end gap-1.5 max-[860px]:w-full max-[860px]:items-start"
            >
              <Button
                variant="destructive"
                class="hover:border-destructive/40 min-w-28 rounded-md px-3"
                @click="openPasswordResetDialog"
              >
                <LockIcon />
                {{ t("system.globalSettings.passwordReset.action") }}
              </Button>
            </div>
          </div>

          <div
            v-for="item in activeSectionSettings"
            :key="item.key"
            class="border-border items-start justify-between border-b last:border-b-0"
            :class="
              activeSettingsSection === 'runtime'
                ? 'grid grid-cols-[minmax(0,1fr)_280px] gap-6 px-4 py-3.5 max-[860px]:flex max-[860px]:flex-col max-[860px]:gap-3'
                : 'flex py-5 max-[860px]:flex-col max-[860px]:gap-3'
            "
          >
            <div
              :class="
                activeSettingsSection === 'runtime'
                  ? 'max-w-none pr-0 max-[860px]:w-full'
                  : 'max-w-[65%] flex-1 pr-6 max-[860px]:w-full max-[860px]:max-w-none max-[860px]:pr-0'
              "
            >
              <div
                class="text-foreground flex flex-wrap items-center gap-1.5 font-medium"
                :class="
                  activeSettingsSection === 'runtime' ? 'mb-1 text-sm leading-[1.4]' : 'mb-1 text-[15px] leading-[1.4]'
                "
              >
                <span>{{ keyLabel(item.key) }}</span>
                <Badge v-if="item.requires_restart" :class="settingTagClass('warning')">
                  {{ t("system.globalSettings.badgeRequiresRestart") }}
                </Badge>
                <Badge v-if="item.is_secret" :class="settingTagClass('primary')">
                  {{ t("system.globalSettings.badgeSecret") }}
                </Badge>
                <Badge v-if="isHighImpactKey(item.key)" :class="settingTagClass('danger')">
                  {{ t("system.globalSettings.badgeHighRisk") }}
                </Badge>
                <Badge
                  v-if="hasOverride(item)"
                  :class="settingTagClass('success')"
                  :title="t('system.globalSettings.badgeOverrideTooltip')"
                >
                  {{ t("system.globalSettings.badgeOverride") }}
                </Badge>
              </div>
              <p
                v-if="settingDescription(item)"
                class="text-muted-foreground m-0 leading-[1.5] max-[860px]:max-w-none"
                :class="activeSettingsSection === 'runtime' ? 'max-w-[620px] text-xs' : 'max-w-[480px] text-[13px]'"
              >
                {{ settingDescription(item) }}
              </p>
              <div v-if="modifiedMeta(item)" class="text-placeholder mt-1.5 text-xs">
                {{ t("system.globalSettings.modifiedAt", { value: modifiedMeta(item) }) }}
              </div>
            </div>

            <div
              class="flex shrink-0 flex-col gap-1.5"
              :class="
                activeSettingsSection === 'runtime'
                  ? 'min-w-0 items-end max-[860px]:w-full max-[860px]:items-start'
                  : 'min-w-[280px] items-end max-[860px]:w-full max-[860px]:items-start'
              "
            >
              <!--
            Two-row layout: input + spinner on top, secondary actions
            (currently just Reset) on a second row below, right-aligned
            under the input.
          -->
              <div class="flex items-center justify-end gap-2 max-[860px]:w-full max-[860px]:justify-start">
                <Popover
                  v-if="hasEnum(item) && isHighRiskKey(item.key)"
                  :open="highRiskPopconfirm.visible"
                  @update:open="onHighRiskPopoverOpen"
                >
                  <PopoverAnchor as-child>
                    <div class="min-w-0">
                      <Select
                        :model-value="editValues[item.key] as string"
                        :disabled="savingKey === item.key"
                        @update:model-value="onHighRiskSelectInput(item, $event)"
                      >
                        <SelectTrigger class="w-[240px] max-[860px]:w-full" :aria-label="keyLabel(item.key)">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem v-for="opt in enumOptions(item)" :key="opt.value" :value="opt.value">
                            {{ opt.label }}
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </PopoverAnchor>
                  <PopoverContent side="left" class="w-72">
                    <p class="mb-3 text-[13px] leading-[1.5]">{{ highRiskPopconfirm.content }}</p>
                    <div class="flex justify-end gap-2">
                      <Button size="sm" variant="outline" @click="highRiskPopconfirm.finish(false)">
                        {{ t("system.globalSettings.confirm.cancelBtn") }}
                      </Button>
                      <Button
                        size="sm"
                        :class="confirmBtnClass(highRiskPopconfirm.confirmBtn.theme)"
                        @click="highRiskPopconfirm.finish(true)"
                      >
                        {{ highRiskPopconfirm.confirmBtn.content }}
                      </Button>
                    </div>
                  </PopoverContent>
                </Popover>

                <Select
                  v-if="hasEnum(item) && !isHighRiskKey(item.key)"
                  :model-value="editValues[item.key] as string"
                  :disabled="savingKey === item.key"
                  @update:model-value="onSelectInput(item, $event)"
                >
                  <SelectTrigger
                    class="max-[860px]:w-full"
                    :class="activeSettingsSection === 'runtime' ? 'w-[210px] max-[860px]:flex-1' : 'w-[240px]'"
                    :aria-label="keyLabel(item.key)"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="opt in enumOptions(item)" :key="opt.value" :value="opt.value">
                      {{ opt.label }}
                    </SelectItem>
                  </SelectContent>
                </Select>

                <Switch
                  v-else-if="item.value_type === 'bool'"
                  :model-value="editValues[item.key] as boolean"
                  :disabled="savingKey === item.key"
                  :aria-label="keyLabel(item.key)"
                  @update:model-value="onSwitchInput(item, $event)"
                />

                <Input
                  v-else-if="item.value_type === 'int'"
                  type="number"
                  :min="minimumFor(item)"
                  :step="1"
                  :model-value="intModel(item)"
                  :placeholder="placeholderFor(item)"
                  :aria-label="keyLabel(item.key)"
                  :disabled="savingKey === item.key"
                  class="max-[860px]:w-full"
                  :class="activeSettingsSection === 'runtime' ? 'w-[210px] max-[860px]:flex-1' : 'w-[240px]'"
                  @update:model-value="setIntModel(item, $event)"
                  @blur="onChange(item)"
                />

                <Popover
                  v-if="item.value_type === 'string_list' && item.key === 'ssrf.whitelist'"
                  :open="ssrfPopconfirm.visible"
                  @update:open="onSsrfPopoverOpen"
                >
                  <PopoverAnchor as-child>
                    <div class="min-w-0">
                      <TagsFieldInput
                        :key="`ssrf-tag-${ssrfTagInputKey()}`"
                        :model-value="ssrfWhitelistModelValue()"
                        class="w-[320px] max-[860px]:w-full"
                        :placeholder="emptyListPlaceholder"
                        :aria-label="keyLabel(item.key)"
                        :disabled="savingKey === item.key"
                        @update:model-value="onSsrfWhitelistModelUpdate"
                      />
                    </div>
                  </PopoverAnchor>
                  <PopoverContent side="left" class="w-72">
                    <p class="mb-3 text-[13px] leading-[1.5]">{{ ssrfPopconfirm.content }}</p>
                    <div class="flex justify-end gap-2">
                      <Button size="sm" variant="outline" @click="ssrfPopconfirm.finish(false)">
                        {{ t("system.globalSettings.confirm.cancelBtn") }}
                      </Button>
                      <Button
                        size="sm"
                        :class="confirmBtnClass(ssrfPopconfirm.confirmBtn.theme)"
                        @click="ssrfPopconfirm.finish(true)"
                      >
                        {{ ssrfPopconfirm.confirmBtn.content }}
                      </Button>
                    </div>
                  </PopoverContent>
                </Popover>

                <div
                  v-if="
                    item.value_type !== 'bool' &&
                    item.value_type !== 'int' &&
                    !hasEnum(item) &&
                    !(item.value_type === 'string_list' && item.key === 'ssrf.whitelist')
                  "
                  class="relative"
                  :class="
                    activeSettingsSection === 'runtime' ? 'max-[860px]:flex-1' : 'max-[860px]:w-full max-[860px]:flex-1'
                  "
                >
                  <Input
                    :model-value="editValues[item.key] as string"
                    :placeholder="placeholderFor(item)"
                    :aria-label="keyLabel(item.key)"
                    :disabled="savingKey === item.key"
                    class="pr-7"
                    :class="
                      activeSettingsSection === 'runtime'
                        ? 'w-[210px] max-[860px]:w-full'
                        : 'w-[240px] max-[860px]:w-full'
                    "
                    @update:model-value="(v) => (editValues[item.key] = v)"
                    @blur="onChange(item)"
                  />
                  <button
                    v-if="String(editValues[item.key] ?? '') !== '' && savingKey !== item.key"
                    type="button"
                    data-slot="input-clear"
                    class="text-muted-foreground hover:text-foreground absolute top-1/2 right-1.5 -translate-y-1/2 cursor-pointer"
                    :aria-label="t('common.clear')"
                    @click="editValues[item.key] = ''"
                  >
                    <XIcon class="size-3.5" />
                  </button>
                </div>

                <!--
            Per-row saving spinner. Appears next to the control while
            a PUT is in flight; the controls stay disabled (see
            :disabled bindings above) so concurrent edits can't race.
          -->
                <div
                  v-if="savingKey === item.key"
                  class="text-muted-foreground inline-flex min-w-[52px] shrink-0 items-center gap-[5px] text-xs"
                  role="status"
                >
                  <Loader2Icon class="size-3.5 animate-spin" />
                  <span>{{ t("system.globalSettings.saving") }}</span>
                </div>
                <div
                  v-else-if="savedKey === item.key"
                  class="text-success inline-flex min-w-[52px] shrink-0 items-center gap-[5px] text-xs"
                  role="status"
                >
                  <CircleCheckIcon class="size-3.5" />
                  <span>{{ t("system.globalSettings.saved") }}</span>
                </div>
              </div>

              <!--
            Reset-to-default lives on the row below the input, right-
            aligned under it. Hidden entirely for virtual (ENV / default)
            rows so the layout collapses to a single row in the common
            case.
          -->
              <div
                v-if="hasOverride(item) || hasBulkAction(item)"
                class="flex justify-end max-[860px]:w-full max-[860px]:justify-start"
              >
                <Popover v-if="hasBulkAction(item)">
                  <PopoverTrigger as-child>
                    <Button
                      variant="ghost"
                      size="xs"
                      class="shrink-0"
                      :disabled="savingKey === item.key || isDirty(item)"
                      :title="t('system.globalSettings.bulkApply.tooltip')"
                    >
                      <UsersIcon />
                      {{ t("system.globalSettings.bulkApply.label") }}
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent side="left" class="w-80">
                    <p class="mb-3 text-[13px] leading-[1.5]">{{ bulkActionConfirmBody(item) }}</p>
                    <div class="flex justify-end gap-2">
                      <PopoverClose as-child>
                        <Button size="sm" variant="outline">{{ t("system.globalSettings.confirm.cancelBtn") }}</Button>
                      </PopoverClose>
                      <PopoverClose as-child>
                        <Button size="sm" @click="runBulkAction(item)">
                          {{ t("system.globalSettings.bulkApply.confirmBtn") }}
                        </Button>
                      </PopoverClose>
                    </div>
                  </PopoverContent>
                </Popover>

                <Popover v-if="hasOverride(item)">
                  <PopoverTrigger as-child>
                    <Button
                      variant="ghost"
                      size="xs"
                      class="shrink-0"
                      :disabled="savingKey === item.key"
                      :title="t('system.globalSettings.reset.tooltip')"
                    >
                      <RotateCwIcon />
                      {{ t("system.globalSettings.reset.label") }}
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent side="left" class="w-72">
                    <p class="mb-3 text-[13px] leading-[1.5]">
                      {{ t("system.globalSettings.reset.confirmBody", { label: keyLabel(item.key) }) }}
                    </p>
                    <div class="flex justify-end gap-2">
                      <PopoverClose as-child>
                        <Button size="sm" variant="outline">{{ t("system.globalSettings.confirm.cancelBtn") }}</Button>
                      </PopoverClose>
                      <PopoverClose as-child>
                        <Button size="sm" :class="confirmBtnClass('warning')" @click="resetSetting(item)">
                          {{ t("system.globalSettings.reset.confirmBtn") }}
                        </Button>
                      </PopoverClose>
                    </div>
                  </PopoverContent>
                </Popover>
              </div>
            </div>
          </div>
        </div>
      </section>
      <div class="sr-only" role="status" aria-live="polite">{{ saveAnnouncement }}</div>
    </template>

    <!--
      The shell reproduces the old password-reset-dialog overrides: a 64px
      header and a footer, both ruled off, 24px side padding, 12px radius,
      and a tighter layout under 480px.
    -->
    <Dialog :open="passwordResetVisible" @update:open="onPasswordResetOpenChange">
      <DialogContent
        :show-close-button="false"
        class="border-border gap-0 overflow-hidden rounded-xl border p-0 shadow-[0_12px_32px_rgba(15,23,42,0.12),0_2px_8px_rgba(15,23,42,0.08)] ring-0 max-[480px]:max-w-[calc(100vw-24px)] sm:max-w-[440px]"
        @interact-outside="onPasswordResetInteractOutside"
      >
        <DialogHeader
          class="border-border min-h-16 flex-row items-center justify-between gap-3 border-b px-6 max-[480px]:min-h-14 max-[480px]:px-5"
        >
          <DialogTitle class="text-lg leading-[26px] font-semibold max-[480px]:text-[17px]">
            {{ t("system.globalSettings.passwordReset.dialogTitle") }}
          </DialogTitle>
          <DialogClose v-if="!passwordResetSubmitting" as-child>
            <Button variant="ghost" size="icon-sm" class="rounded-md" :aria-label="t('common.close')">
              <XIcon />
            </Button>
          </DialogClose>
        </DialogHeader>
        <div class="px-6 pt-5 pb-1 max-[480px]:px-5 max-[480px]:pt-4">
          <Alert class="mb-5 rounded-lg border-0 bg-[var(--td-warning-color-focus)] px-3.5 py-3">
            <CircleAlertIcon class="text-warning" />
            <AlertTitle class="text-foreground text-[13px] leading-5 font-normal">
              {{ t("system.globalSettings.passwordReset.warning") }}
            </AlertTitle>
          </Alert>
          <div class="mb-4 grid">
            <Label class="min-h-7 text-sm leading-[22px]" for="password-reset-email">
              {{ t("system.globalSettings.passwordReset.emailLabel") }}
            </Label>
            <div class="relative">
              <Input
                id="password-reset-email"
                v-model="passwordResetForm.email"
                type="text"
                autocomplete="off"
                class="rounded-md pr-8"
                :aria-invalid="passwordErrors.email ? true : undefined"
                :disabled="passwordResetSubmitting"
                :placeholder="t('system.globalSettings.passwordReset.emailPlaceholder')"
                @input="passwordErrors.email = ''"
                @blur="validateEmailField"
              />
              <button
                v-if="passwordResetForm.email && !passwordResetSubmitting"
                type="button"
                data-slot="input-clear"
                class="text-muted-foreground hover:text-foreground absolute top-1/2 right-2.5 -translate-y-1/2 cursor-pointer"
                :aria-label="t('common.clear')"
                @mousedown.prevent
                @click="passwordResetForm.email = ''"
              >
                <XIcon class="size-3.5" />
              </button>
            </div>
            <p v-if="passwordErrors.email" class="text-destructive m-0 mt-1 text-xs">{{ passwordErrors.email }}</p>
          </div>
          <div class="mb-4 grid">
            <Label class="min-h-7 text-sm leading-[22px]" for="password-reset-new">
              {{ t("system.globalSettings.passwordReset.newPasswordLabel") }}
            </Label>
            <div class="relative">
              <LockIcon class="text-placeholder absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
              <Input
                id="password-reset-new"
                v-model="passwordResetForm.newPassword"
                type="password"
                autocomplete="new-password"
                :aria-invalid="passwordErrors.newPassword ? true : undefined"
                :disabled="passwordResetSubmitting"
                :placeholder="t('system.globalSettings.passwordReset.newPasswordPlaceholder')"
                class="rounded-md pl-8"
                @input="passwordErrors.newPassword = ''"
                @blur="validateNewPasswordField"
              />
            </div>
            <p v-if="passwordErrors.newPassword" class="text-destructive m-0 mt-1 text-xs">
              {{ passwordErrors.newPassword }}
            </p>
          </div>
          <div class="mb-4 grid">
            <Label class="min-h-7 text-sm leading-[22px]" for="password-reset-confirm">
              {{ t("system.globalSettings.passwordReset.confirmPasswordLabel") }}
            </Label>
            <div class="relative">
              <LockIcon class="text-placeholder absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
              <Input
                id="password-reset-confirm"
                v-model="passwordResetForm.confirmPassword"
                type="password"
                autocomplete="new-password"
                :aria-invalid="passwordErrors.confirmPassword ? true : undefined"
                :disabled="passwordResetSubmitting"
                :placeholder="t('system.globalSettings.passwordReset.confirmPasswordPlaceholder')"
                class="rounded-md pl-8"
                @keydown.enter="submitPasswordReset"
                @input="passwordErrors.confirmPassword = ''"
                @blur="validateConfirmPasswordField"
              />
            </div>
            <p v-if="passwordErrors.confirmPassword" class="text-destructive m-0 mt-1 text-xs">
              {{ passwordErrors.confirmPassword }}
            </p>
          </div>
        </div>
        <DialogFooter
          class="border-border m-0 rounded-none bg-transparent px-6 pt-4 pb-5 max-[480px]:px-5 max-[480px]:pt-3.5 max-[480px]:pb-[18px]"
        >
          <Button
            variant="outline"
            class="min-w-[88px] rounded-md"
            :disabled="passwordResetSubmitting"
            @click="onPasswordResetOpenChange(false)"
          >
            {{ t("system.globalSettings.confirm.cancelBtn") }}
          </Button>
          <Button
            class="bg-destructive text-primary-foreground hover:bg-destructive/80 min-w-[88px] rounded-md"
            :disabled="passwordResetSubmitting"
            @click="submitPasswordReset"
          >
            <Loader2Icon v-if="passwordResetSubmitting" class="animate-spin" />
            {{ t("system.globalSettings.passwordReset.confirmBtn") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, computed, watch, nextTick } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import {
  listSystemSettings,
  updateSystemSetting,
  resetSystemSetting,
  applyDefaultStorageQuotaToAllTenants,
  listSystemAdmins,
  promoteUserToSystemAdmin,
  revokeSystemAdmin,
  resetUserPassword,
  type SystemSettingItem,
} from "@/api/system";
import { useAuthStore } from "@/stores/auth";
import TagsFieldInput from "@/components/settings/TagsFieldInput.vue";

import { Alert, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Popover, PopoverAnchor, PopoverClose, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  CircleAlertIcon,
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  LockIcon,
  RotateCwIcon,
  UsersIcon,
  XIcon,
} from "@lucide/vue";

const authStore = useAuthStore();
const currentUserId = computed(() => authStore.currentUserId);

const { t, te } = useI18n();

// Friendly labels per key live in i18n (system.globalSettings.keyLabels.*).
// Adding a new entry there must accompany every new key registered in
// service/system_setting.go on the backend; locales without an entry
// fall back to the raw key so a misconfigured deploy still renders.
function keyLabel(k: string): string {
  const path = `system.globalSettings.keyLabels.${k}`;
  return te(path) ? (t(path) as string) : k;
}

// Descriptions are registered in Chinese on the backend for operator docs;
// user-facing copy lives in i18n (system.globalSettings.keyDescriptions.*).
function settingDescription(item: { key: string; description?: string }): string {
  const path = `system.globalSettings.keyDescriptions.${item.key}`;
  if (te(path)) return t(path) as string;
  return item.description ?? "";
}

// Enum keys whose change triggers a whole-value inline confirm before
// PUT. ssrf.whitelist is not here — it uses per-tag confirm instead.
const HIGH_RISK_KEYS = new Set<string>(["auth.registration_mode"]);

const HIGH_IMPACT_KEYS = new Set<string>([
  "auth.registration_mode",
  "ssrf.whitelist",
  // Flipping this relocates every infrastructure settings page between the
  // workspace and the platform, so every workspace admin sees their nav
  // change on the next load. Worth an explicit acknowledgement.
  "governance.centralized_infra",
]);

function isHighRiskKey(key: string): boolean {
  return HIGH_RISK_KEYS.has(key);
}

function isHighImpactKey(key: string): boolean {
  return HIGH_IMPACT_KEYS.has(key);
}

type PopconfirmBtn = { content: string; theme?: "primary" | "danger" | "warning" };

// Shared inline confirm controller (anchored to the control row,
// same interaction model as Reset / bulk-apply). Replaces modal dialogs.
// State must be reactive (not nested refs) so template bindings unwrap.
function createInlinePopconfirm() {
  const state = reactive({
    visible: false,
    content: "",
    theme: "warning" as "default" | "warning" | "danger",
    confirmBtn: { content: "", theme: "primary" } as PopconfirmBtn,
  });
  let resolver: ((ok: boolean) => void) | null = null;
  let settled = false;

  function ask(opts: {
    content: string;
    theme?: "default" | "warning" | "danger";
    confirmBtn: PopconfirmBtn;
  }): Promise<boolean> {
    state.content = opts.content;
    state.theme = opts.theme ?? "warning";
    state.confirmBtn = opts.confirmBtn;
    settled = false;
    return new Promise((resolve) => {
      resolver = resolve;
      state.visible = true;
    });
  }

  function finish(ok: boolean) {
    if (settled) return;
    settled = true;
    state.visible = false;
    const r = resolver;
    resolver = null;
    r?.(ok);
  }

  function onVisibleChange(v: boolean) {
    if (!v && resolver) finish(false);
  }

  return Object.assign(state, { ask, finish, onVisibleChange });
}

const ssrfPopconfirm = createInlinePopconfirm();
const adminPopconfirm = createInlinePopconfirm();
const highRiskPopconfirm = createInlinePopconfirm();

function onSsrfPopoverOpen(v: boolean) {
  ssrfPopconfirm.onVisibleChange(v);
}
function onAdminPopoverOpen(v: boolean) {
  adminPopconfirm.onVisibleChange(v);
}
function onHighRiskPopoverOpen(v: boolean) {
  highRiskPopconfirm.onVisibleChange(v);
}

// Friendly labels for enum options live in i18n
// (system.globalSettings.enumLabels.<key>.<value>). Falls back to the
// raw enum value when no translation exists.
function enumLabel(itemKey: string, optionValue: string): string {
  const path = `system.globalSettings.enumLabels.${itemKey}.${optionValue}`;
  return te(path) ? (t(path) as string) : optionValue;
}

const emptyListPlaceholder = computed(() => t("system.globalSettings.tagInputPlaceholder"));

const settings = ref<SystemSettingItem[]>([]);
const loading = ref(false);
const savingKey = ref<string | null>(null);
const savedKey = ref<string | null>(null);
const saveAnnouncement = ref("");
let savedKeyTimer: ReturnType<typeof setTimeout> | null = null;

type SettingsSection = "access" | "tenant" | "file" | "runtime" | "security" | "other";

// Product-oriented order, rather than the registry's alphabetical key order.
// Unknown/out-of-band rows remain visible in a conditional "Other" tab so the
// backend's diagnostic contract is preserved when a deployment contains an
// unexpected key.
const SETTINGS_SECTION_KEYS: Record<Exclude<SettingsSection, "other">, readonly string[]> = {
  access: [
    "governance.centralized_infra",
    "auth.registration_mode",
    "auth.default_tenant_mode",
    "tenant.self_service_creation_enabled",
    "tenant.max_owned_per_user",
  ],
  tenant: ["tenant.default_storage_quota_gb"],
  file: ["file.max_size_mb", "file.video_max_size_mb"],
  runtime: [
    "asynq.core_concurrency",
    "asynq.enrichment_concurrency",
    "asynq.postprocess_concurrency",
    "asynq.maintenance_concurrency",
    "asynq.shared_concurrency",
    "asynq.wiki_concurrency",
    "model.max_concurrency",
  ],
  security: ["ssrf.whitelist"],
};

const activeSettingsSection = ref<SettingsSection>("access");
const knownSettingKeys = new Set(Object.values(SETTINGS_SECTION_KEYS).flat());
const settingsByKey = computed(() => new Map(settings.value.map((item) => [item.key, item])));
const unknownSettings = computed(() => settings.value.filter((item) => !knownSettingKeys.has(item.key)));
const hasUnknownSettings = computed(() => unknownSettings.value.length > 0);
const visibleSections = computed<SettingsSection[]>(() =>
  hasUnknownSettings.value
    ? ["access", "tenant", "file", "runtime", "security", "other"]
    : ["access", "tenant", "file", "runtime", "security"],
);

watch(hasUnknownSettings, (hasUnknown) => {
  if (!hasUnknown && activeSettingsSection.value === "other") {
    activeSettingsSection.value = "access";
  }
});

const activeSectionSettings = computed(() => {
  if (activeSettingsSection.value === "other") return unknownSettings.value;
  return SETTINGS_SECTION_KEYS[activeSettingsSection.value]
    .map((key) => settingsByKey.value.get(key))
    .filter((item): item is SystemSettingItem => Boolean(item));
});

const activeSectionTitle = computed(() => t(`system.globalSettings.sections.${activeSettingsSection.value}.title`));
const activeSectionDescription = computed(() =>
  t(`system.globalSettings.sections.${activeSettingsSection.value}.description`),
);

function sectionTabLabel(section: SettingsSection): string {
  const count =
    section === "other"
      ? unknownSettings.value.length
      : SETTINGS_SECTION_KEYS[section].filter((key) => settingsByKey.value.has(key)).length +
        (section === "access" ? 2 : 0);
  return t(`system.globalSettings.sections.${section}.tab`, { count });
}

function markSettingSaved(item: SystemSettingItem) {
  savedKey.value = item.key;
  saveAnnouncement.value = t("system.globalSettings.saveAnnouncement", {
    label: keyLabel(item.key),
  });
  if (savedKeyTimer) clearTimeout(savedKeyTimer);
  savedKeyTimer = setTimeout(() => {
    if (savedKey.value === item.key) savedKey.value = null;
    savedKeyTimer = null;
  }, 2000);
}

// Admin management state. We keep two parallel structures:
//   - adminEmails: the v-model bound to the tags input (excludes
//     current user; that's the visible source of truth).
//   - adminEmailToId: email → user UUID, populated from the list
//     endpoint. Needed because revoke takes a UUID, not an email.
// Both reset on every reload to avoid stale entries persisting after
// a peer SystemAdmin makes a change. adminBusy disables the input and
// shows the row spinner only while promote/revoke API calls are in
// flight — not while the inline confirm is waiting for a click.
const adminEmails = ref<string[]>([]);
const adminEmailToId = ref<Record<string, string>>({});
const adminBusy = ref(false);

const passwordResetVisible = ref(false);
const passwordResetSubmitting = ref(false);
const passwordResetForm = reactive({
  email: "",
  newPassword: "",
  confirmPassword: "",
});
const passwordErrors = reactive({ email: "", newPassword: "", confirmPassword: "" });

function clearPasswordErrors() {
  passwordErrors.email = "";
  passwordErrors.newPassword = "";
  passwordErrors.confirmPassword = "";
}

// One validator per field, ported from the old t-form rules. The rules
// fired on blur as well as on submit, so each field's @blur runs its own
// validator and submit runs all three; the first failing rule's message
// wins, as it did in TDesign.
function validateEmailField(): boolean {
  const email = passwordResetForm.email.trim();
  if (!email) {
    passwordErrors.email = t("system.globalSettings.passwordReset.validation.emailRequired");
  } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    passwordErrors.email = t("system.globalSettings.passwordReset.validation.emailInvalid");
  } else {
    passwordErrors.email = "";
  }
  return !passwordErrors.email;
}

function validateNewPasswordField(): boolean {
  const pwd = passwordResetForm.newPassword;
  if (!pwd) {
    passwordErrors.newPassword = t("system.globalSettings.passwordReset.validation.passwordRequired");
  } else if (pwd.length < 8 || pwd.length > 32) {
    passwordErrors.newPassword = t("system.globalSettings.passwordReset.validation.passwordLength");
  } else if (!/[a-zA-Z]/.test(pwd)) {
    passwordErrors.newPassword = t("system.globalSettings.passwordReset.validation.passwordLetter");
  } else if (!/\d/.test(pwd)) {
    passwordErrors.newPassword = t("system.globalSettings.passwordReset.validation.passwordNumber");
  } else {
    passwordErrors.newPassword = "";
  }
  return !passwordErrors.newPassword;
}

function validateConfirmPasswordField(): boolean {
  if (!passwordResetForm.confirmPassword) {
    passwordErrors.confirmPassword = t("system.globalSettings.passwordReset.validation.confirmRequired");
  } else if (passwordResetForm.confirmPassword !== passwordResetForm.newPassword) {
    passwordErrors.confirmPassword = t("system.globalSettings.passwordReset.validation.passwordMismatch");
  } else {
    passwordErrors.confirmPassword = "";
  }
  return !passwordErrors.confirmPassword;
}

function validatePasswordResetForm(): boolean {
  // Run every validator (no short-circuit) so all failing fields show
  // their message at once, as t-form's validate() did.
  const results = [validateEmailField(), validateNewPasswordField(), validateConfirmPasswordField()];
  return results.every(Boolean);
}

// The dialog could not be dismissed by the overlay or the close button
// while the reset request was in flight (close-on-overlay-click /
// close-btn were bound to !passwordResetSubmitting); Esc still closed it.
function onPasswordResetOpenChange(open: boolean) {
  passwordResetVisible.value = open;
  if (!open) resetPasswordResetForm();
}

function onPasswordResetInteractOutside(event: Event) {
  if (passwordResetSubmitting.value) event.preventDefault();
}

// Tag classes matching the old small light t-tag: 20px high, 4px side
// padding, 3px radius, regular weight, the theme's "-light" tint.
function settingTagClass(theme: "warning" | "primary" | "danger" | "success"): string {
  const base = "h-5 rounded-[3px] px-1 font-normal";
  switch (theme) {
    case "warning":
      return `${base} bg-[var(--td-warning-color-light)] text-warning`;
    case "primary":
      return `${base} bg-[var(--td-brand-color-light)] text-primary`;
    case "danger":
      return `${base} bg-[var(--td-error-color-light)] text-destructive`;
    default:
      return `${base} bg-[var(--td-success-color-light)] text-success`;
  }
}

// Solid confirm button colours for the inline confirms: TDesign's
// confirm-btn theme painted a filled danger / warning / primary button.
function confirmBtnClass(theme: PopconfirmBtn["theme"]): string {
  if (theme === "danger") return "bg-destructive text-primary-foreground hover:bg-destructive/80";
  if (theme === "warning") return "bg-warning text-primary-foreground hover:bg-warning/80";
  return "";
}

function resetPasswordResetForm() {
  passwordResetForm.email = "";
  passwordResetForm.newPassword = "";
  passwordResetForm.confirmPassword = "";
  clearPasswordErrors();
}

async function openPasswordResetDialog() {
  resetPasswordResetForm();
  passwordResetVisible.value = true;
}

async function submitPasswordReset() {
  if (passwordResetSubmitting.value) return;
  if (!validatePasswordResetForm()) return;

  passwordResetSubmitting.value = true;
  try {
    await resetUserPassword({
      email: passwordResetForm.email.trim(),
      new_password: passwordResetForm.newPassword,
    });
    saveAnnouncement.value = t("system.globalSettings.passwordReset.success");
    MessagePlugin.success(t("system.globalSettings.passwordReset.success"));
    passwordResetVisible.value = false;
  } catch (err: any) {
    const msg = err?.message || t("system.globalSettings.passwordReset.failed");
    saveAnnouncement.value = msg;
    MessagePlugin.error(msg);
  } finally {
    passwordResetSubmitting.value = false;
  }
}

// Guards ssrf.whitelist while an async confirm roundtrip is in flight.
const listConfirmBusyKey = ref<string | null>(null);

// Bumped when the SSRF tags input is snapped back to the saved list so
// Vue remounts the control and clears its internal draft state.
const ssrfTagInputKeys = reactive<Record<string, number>>({});

// Briefly blocks model updates while the SSRF tags input remount settles.
const ssrfSnapLocked = ref(false);

// Reactive map of in-progress edits, keyed by setting key. We don't
// mutate the canonical `settings` array directly so a failed save
// leaves the original value visible until the user retries or refreshes.
// Initialised lazily in loadSettings; setting.value is the JSON-decoded
// form (number / boolean / string / string[]).
const editValues = reactive<Record<string, unknown>>({});

function hasEnum(item: SystemSettingItem): boolean {
  return Array.isArray(item.enum) && item.enum.length > 0;
}

function enumOptions(item: SystemSettingItem): { label: string; value: string }[] {
  const opts = item.enum ?? [];
  return opts.map((v) => ({ label: enumLabel(item.key, v), value: v }));
}

function onSelectInput(item: SystemSettingItem, value: unknown) {
  editValues[item.key] = value;
  onChange(item);
}

function onHighRiskSelectInput(item: SystemSettingItem, value: unknown) {
  editValues[item.key] = value;
  onHighRiskSelectChange(item);
}

function onSwitchInput(item: SystemSettingItem, value: boolean) {
  editValues[item.key] = value;
  onChange(item);
}

// t-input-number equivalent: commit numeric edits into editValues on
// input (the actual save still happens on blur via onChange).
function intModel(item: SystemSettingItem): string {
  const v = editValues[item.key];
  return typeof v === "number" ? String(v) : "";
}

function setIntModel(item: SystemSettingItem, value: string | number) {
  const s = String(value);
  if (s === "") return;
  const n = Number(s);
  if (Number.isFinite(n)) editValues[item.key] = n;
}

// hasOverride reports whether the row carries a real DB override (vs a
// virtual row backed by ENV/default). Distinguishing these is what
// `last_modified_by` was made for: empty string means the value came
// from registry/ENV. Drives the "已覆盖" badge.
function hasOverride(item: SystemSettingItem): boolean {
  return Boolean(item.last_modified_by && item.last_modified_by.trim() !== "");
}

// modifiedMeta returns a humane "上次修改" line for rows that have been
// persisted (last_modified_by non-empty AND updated_at not the Go zero
// value). Returns '' for virtual rows so the meta line collapses
// entirely instead of rendering "1/1/1 08:05:43" garbage.
function modifiedMeta(item: SystemSettingItem): string {
  if (!hasOverride(item)) return "";
  const ts = item.updated_at;
  if (!ts || ts.startsWith("0001-")) return "";
  const formatted = formatDate(ts);
  // Prefer the resolved username/email the server enriches via
  // last_modified_by_name. Fall back to the UUID's first 8 chars when
  // the user can't be resolved (deleted account, transient lookup
  // failure) — the full ID is still in the audit log.
  const actor =
    item.last_modified_by_name && item.last_modified_by_name.trim() !== ""
      ? item.last_modified_by_name
      : (item.last_modified_by || "").slice(0, 8);
  return `${formatted} · ${actor}`;
}

const SSRF_WHITELIST_KEY = "ssrf.whitelist";

function ssrfWhitelistModelValue(): string[] {
  const v = editValues[SSRF_WHITELIST_KEY];
  return Array.isArray(v) ? (v as string[]) : [];
}

function ssrfTagInputKey(): number {
  return ssrfTagInputKeys[SSRF_WHITELIST_KEY] ?? 0;
}

function resetSsrfTagInput() {
  ssrfTagInputKeys[SSRF_WHITELIST_KEY] = (ssrfTagInputKeys[SSRF_WHITELIST_KEY] ?? 0) + 1;
}

function globalSettingsText(path: string, params?: Record<string, string>): string {
  if (!te(path)) return path;
  const msg = params ? t(path, params) : t(path);
  return typeof msg === "string" ? msg : path;
}

// Controlled SSRF tags input: we commit editValues so a declined delta
// can be rolled back without the component re-applying a removal.
function onSsrfWhitelistModelUpdate(next: string[]) {
  if (listConfirmBusyKey.value === SSRF_WHITELIST_KEY || ssrfSnapLocked.value) return;
  editValues[SSRF_WHITELIST_KEY] = next;
  void onSsrfWhitelistChange();
}

async function onSsrfWhitelistChange() {
  const item = settings.value.find((s) => s.key === SSRF_WHITELIST_KEY);
  if (!item || !isDirty(item)) return;
  if (listConfirmBusyKey.value === SSRF_WHITELIST_KEY) return;
  await handleSSRFListChange(item);
}

async function snapSsrfWhitelistToSaved(item: SystemSettingItem) {
  const saved = Array.isArray(item.value) ? (item.value as string[]) : [];
  editValues[SSRF_WHITELIST_KEY] = [...saved];
  resetSsrfTagInput();
  ssrfSnapLocked.value = true;
  await nextTick();
  await nextTick();
  ssrfSnapLocked.value = false;
}

function isDirty(item: SystemSettingItem): boolean {
  const cur = editValues[item.key];
  const orig = item.value;
  if (Array.isArray(cur) && Array.isArray(orig)) {
    if (cur.length !== orig.length) return true;
    for (let i = 0; i < cur.length; i++) {
      if (cur[i] !== orig[i]) return true;
    }
    return false;
  }
  return cur !== orig;
}

function formatDate(isoString: string): string {
  try {
    const d = new Date(isoString);
    return d.toLocaleString("zh-CN", { hour12: false });
  } catch {
    return isoString;
  }
}

// placeholderFor renders the current effective value (DB / ENV / default)
// as a placeholder hint inside the edit control. For string_list it's
// joined with comma; for booleans we show nothing (the switch already
// reflects the value).
function placeholderFor(item: SystemSettingItem): string {
  const v = item.value;
  if (v === null || v === undefined) return "";
  if (Array.isArray(v)) return v.join(", ");
  return String(v);
}

function minimumFor(item: SystemSettingItem): number {
  if (item.key.startsWith("asynq.") && item.key.endsWith("_concurrency")) return 1;
  return 0;
}

async function loadSettings() {
  loading.value = true;
  try {
    const list = await listSystemSettings();
    settings.value = list;
    // Reset edit values to the canonical state on every load — no
    // partial drafts survive a refresh, which avoids the "I came back
    // and my unsaved edits look saved" trap.
    for (const item of list) {
      // Defensive copy for arrays so the tags input doesn't mutate
      // the canonical settings entry through the v-model binding.
      editValues[item.key] = Array.isArray(item.value) ? [...(item.value as unknown[])] : item.value;
    }
  } catch (err: any) {
    const msg = err?.message || t("system.globalSettings.messages.loadFailed");
    MessagePlugin.error(msg);
  } finally {
    loading.value = false;
  }
}

// onChange persists non-SSRF settings. SSRF whitelist and system admins
// have dedicated handlers with inline confirm.
async function onChange(item: SystemSettingItem) {
  if (!isDirty(item)) return;

  // SSRF whitelist gets the per-entry confirm flow — same shape as the
  // admin tags input above. Adding or removing each host/CIDR is its
  // own privileged change (a single bad CIDR can punch a hole through
  // the egress firewall), so we ask once per delta instead of once
  // per "save". This matches the operator's mental model: every tag
  // they touch is acknowledged on its own.
  await persistSetting(item);
}

async function onHighRiskSelectChange(item: SystemSettingItem) {
  const newValue = editValues[item.key];
  if (newValue === item.value) return;

  // Revert the select immediately so cancel leaves the saved value
  // visible; re-apply only after the inline confirm is confirmed.
  editValues[item.key] = item.value;

  const ok = await highRiskPopconfirm.ask({
    content: highRiskConfirmBody(item, newValue),
    theme: "danger",
    confirmBtn: {
      content: t("system.globalSettings.confirm.confirmBtn"),
      theme: "danger",
    },
  });
  if (!ok) return;

  editValues[item.key] = newValue;
  await persistSetting(item);
}

function confirmSsrfListEntryChange(action: "add" | "remove", entry: string): Promise<boolean> {
  const base = `system.globalSettings.listConfirm.${SSRF_WHITELIST_KEY}.${action}`;
  return ssrfPopconfirm.ask({
    content: globalSettingsText(`${base}.body`, { entry }),
    theme: action === "add" ? "danger" : "warning",
    confirmBtn: {
      content: globalSettingsText(`${base}.confirmBtn`),
      theme: action === "add" ? "danger" : "primary",
    },
  });
}

// handleSSRFListChange reconciles the current edit against the saved
// list one entry at a time. The strategy is "confirmed deltas only":
// we start from the saved value, then walk the user's added/removed
// sets and apply each entry the operator individually approves. If
// every prompt is declined we end up identical to the saved value
// and short-circuit before hitting the API. Otherwise we save the
// merged result in a single PUT so the audit log and pubsub get one
// coherent post-image (instead of N noisy events).
async function handleSSRFListChange(item: SystemSettingItem) {
  listConfirmBusyKey.value = item.key;
  try {
    const oldArr = Array.isArray(item.value) ? (item.value as string[]) : [];
    const nextArr = Array.isArray(editValues[item.key]) ? (editValues[item.key] as string[]) : [];

    const oldSet = new Set(oldArr);
    const nextSet = new Set(nextArr);

    const added: string[] = [];
    for (const v of nextSet) if (!oldSet.has(v)) added.push(v);
    const removed: string[] = [];
    for (const v of oldSet) if (!nextSet.has(v)) removed.push(v);

    if (added.length === 0 && removed.length === 0) return;

    // Build the candidate value from approved deltas only. We keep
    // insertion order roughly aligned with the operator's intent:
    // start from the saved list (so unchanged entries keep their
    // position), drop approved removals, append approved additions.
    const finalSet = new Set(oldArr);
    for (const entry of added) {
      const ok = await confirmSsrfListEntryChange("add", entry);
      if (ok) {
        finalSet.add(entry);
      } else {
        await snapSsrfWhitelistToSaved(item);
        return;
      }
    }
    for (const entry of removed) {
      const ok = await confirmSsrfListEntryChange("remove", entry);
      if (ok) {
        finalSet.delete(entry);
      } else {
        await snapSsrfWhitelistToSaved(item);
        return;
      }
    }

    const finalArr = Array.from(finalSet);
    // Compare against saved value, not against `editValues`. If every
    // delta was declined, the saved list still wins; we just need to
    // snap the input back to it.
    const sameAsSaved = finalArr.length === oldArr.length && finalArr.every((v, i) => v === oldArr[i]);
    if (sameAsSaved) {
      await snapSsrfWhitelistToSaved(item);
      return;
    }

    editValues[item.key] = finalArr;
    await persistSetting(item);
  } finally {
    await nextTick();
    listConfirmBusyKey.value = null;
  }
}

function highRiskConfirmBody(item: SystemSettingItem, value: unknown): string {
  const renderedValue = Array.isArray(value)
    ? value.length === 0
      ? t("system.globalSettings.confirm.emptyValue")
      : value.join(", ")
    : String(value);
  return t("system.globalSettings.confirm.bodyAuthRegistrationMode", {
    label: keyLabel(item.key),
    value: renderedValue,
  });
}

// hasBulkAction tells the template whether the current row carries an
// extra "apply to existing data" action beyond plain save/reset.
// Currently only `tenant.default_storage_quota_gb` does — saving the
// setting only affects future tenants, so the bulk button is the
// escape hatch for "rewrite all current tenants too".
function hasBulkAction(item: SystemSettingItem): boolean {
  return item.key === "tenant.default_storage_quota_gb";
}

function bulkActionConfirmBody(item: SystemSettingItem): string {
  // Use the canonical (saved) value, not the in-progress edit, so the
  // operator sees exactly what will be written. The button is disabled
  // when the row is dirty (see template), so item.value is the value
  // that's currently in effect for new tenants.
  const v = item.value;
  const valueText = v === null || v === undefined ? "" : String(v);
  return t("system.globalSettings.bulkApply.confirmBody", { value: valueText });
}

async function runBulkAction(item: SystemSettingItem) {
  if (!hasBulkAction(item)) return;
  savedKey.value = null;
  savingKey.value = item.key;
  try {
    const result = await applyDefaultStorageQuotaToAllTenants();
    MessagePlugin.success(
      t("system.globalSettings.bulkApply.success", {
        count: result.affected,
        gb: result.quota_gb,
      }),
    );
    markSettingSaved(item);
  } catch (err: any) {
    const msg = err?.message || t("system.globalSettings.bulkApply.failed");
    saveAnnouncement.value = msg;
    MessagePlugin.error(msg);
  } finally {
    savingKey.value = null;
  }
}

// resetSetting drops the DB override and reloads the row so the UI
// reflects the resolved fallback (ENV value if set, otherwise the
// in-code default). We refetch the whole list rather than the single
// row because the list endpoint is what populates the canonical
// settings array and re-running it keeps the modified-by enrichment
// consistent for every row in the table.
async function resetSetting(item: SystemSettingItem) {
  savedKey.value = null;
  savingKey.value = item.key;
  try {
    await resetSystemSetting(item.key);
    await loadSettings();
    markSettingSaved(item);
    MessagePlugin.success(t("system.globalSettings.reset.success"));
  } catch (err: any) {
    const msg = err?.message || t("system.globalSettings.reset.failed");
    saveAnnouncement.value = msg;
    MessagePlugin.error(msg);
  } finally {
    savingKey.value = null;
  }
}

async function persistSetting(item: SystemSettingItem) {
  const newValue = editValues[item.key];
  savedKey.value = null;
  savingKey.value = item.key;
  try {
    const updated = await updateSystemSetting(item.key, newValue);
    // Replace the row in-place so the table stays at scroll position
    // and other rows' edit state isn't disturbed.
    const idx = settings.value.findIndex((s) => s.key === item.key);
    if (idx >= 0) {
      settings.value[idx] = updated;
    }
    editValues[item.key] = Array.isArray(updated.value) ? [...(updated.value as unknown[])] : updated.value;
    markSettingSaved(updated);
    MessagePlugin.success(t("system.globalSettings.messages.saveSuccess"));
  } catch (err: any) {
    const msg = err?.message || t("system.globalSettings.messages.saveFailed");
    saveAnnouncement.value = msg;
    MessagePlugin.error(msg);
    // Roll the input back to the canonical value on failure. Without
    // this an invalid edit (e.g. SSRF whitelist with a malformed CIDR
    // that the backend 400'd) would stay rendered as if accepted, and
    // the user couldn't tell whether the rejection actually stuck.
    const failed = settings.value.find((s) => s.key === item.key);
    if (failed) {
      editValues[item.key] = Array.isArray(failed.value) ? [...(failed.value as unknown[])] : failed.value;
    }
  } finally {
    savingKey.value = null;
  }
}

// loadAdmins refreshes the admin tags list + the email→id lookup
// table. We exclude the current user from the visible list so the
// "you can't revoke yourself" rule has nothing to enforce in the UI
// (the backend rejects it too, but hiding the tag is friendlier).
async function loadAdmins() {
  try {
    const resp = await listSystemAdmins({ limit: 200 });
    const map: Record<string, string> = {};
    const emails: string[] = [];
    for (const u of resp.admins ?? []) {
      // Empty emails would collapse to a single tag "" that can't be
      // round-tripped to a user_id; skip them. Same defensive stance
      // as resolveMaxOwnedTenantsPerUser on the backend.
      if (!u.email) continue;
      map[u.email] = u.id;
      if (u.id !== currentUserId.value) {
        emails.push(u.email);
      }
    }
    adminEmailToId.value = map;
    adminEmails.value = emails;
  } catch (err: any) {
    const msg = err?.message || t("system.globalSettings.admins.loadFailed");
    MessagePlugin.error(msg);
  }
}

function confirmAdminChange(action: "promote" | "revoke", email: string): Promise<boolean> {
  const base = `system.globalSettings.admins.confirm.${action}`;
  return adminPopconfirm.ask({
    content: globalSettingsText(`${base}.body`, { email }),
    theme: action === "revoke" ? "danger" : "warning",
    confirmBtn: {
      content: globalSettingsText(`${base}.confirmBtn`),
      theme: action === "revoke" ? "danger" : "primary",
    },
  });
}

// onAdminsChange diffs the new tag list against the canonical state
// and dispatches one promote / revoke per delta. Failures roll back
// the whole tag list to the server-side truth — this is simpler than
// trying to undo individual ops, and the network/error case for batch
// edits is rare enough that a full reload doesn't surprise anyone.
async function onAdminsChange(next: string[]) {
  if (adminBusy.value) return;

  // Snapshot of what's currently authoritative — the email→id map's
  // keys, minus the current user. Anything in `next` that's not here
  // is an addition; anything here that's not in `next` is a removal.
  const authoritative = new Set<string>();
  for (const email of Object.keys(adminEmailToId.value)) {
    if (adminEmailToId.value[email] !== currentUserId.value) {
      authoritative.add(email);
    }
  }
  const nextSet = new Set(next.map((e) => e.trim()).filter(Boolean));

  // Drop the user-typed entry to canonical lowercase/trim before we
  // diff. We don't lowercase server-returned emails because the
  // backend stores the original case; matching against the map's keys
  // happens with the as-typed value, which is what the user sees.
  const added: string[] = [];
  for (const email of nextSet) {
    if (!authoritative.has(email)) added.push(email);
  }
  const removed: string[] = [];
  for (const email of authoritative) {
    if (!nextSet.has(email)) removed.push(email);
  }

  if (added.length === 0 && removed.length === 0) return;

  // Confirm before any privilege change (no loading spinner yet — the
  // inline confirm is the only UI; adminBusy is reserved for API roundtrips).
  for (const email of added) {
    const ok = await confirmAdminChange("promote", email);
    if (!ok) {
      await loadAdmins();
      return;
    }
  }
  for (const email of removed) {
    const userId = adminEmailToId.value[email];
    if (!userId) continue;
    const ok = await confirmAdminChange("revoke", email);
    if (!ok) {
      await loadAdmins();
      return;
    }
  }

  adminBusy.value = true;
  let applied = 0;
  try {
    for (const email of added) {
      await promoteUserToSystemAdmin({ email });
      applied++;
    }
    for (const email of removed) {
      const userId = adminEmailToId.value[email];
      if (!userId) continue;
      await revokeSystemAdmin(userId);
      applied++;
    }
    await loadAdmins();
    if (applied > 0) {
      saveAnnouncement.value = t("system.globalSettings.admins.saveSuccess");
      MessagePlugin.success(t("system.globalSettings.admins.saveSuccess"));
    }
  } catch (err: any) {
    const msg = err?.message || t("system.globalSettings.admins.saveFailed");
    saveAnnouncement.value = msg;
    MessagePlugin.error(msg);
    await loadAdmins();
  } finally {
    adminBusy.value = false;
  }
}

onMounted(() => {
  loadSettings();
  loadAdmins();
});

onUnmounted(() => {
  if (savedKeyTimer) clearTimeout(savedKeyTimer);
});
</script>
