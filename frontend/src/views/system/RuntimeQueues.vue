<template>
  <div class="text-foreground">
    <header class="mb-6 flex items-start justify-between gap-6 max-[860px]:flex-col">
      <div>
        <h2 class="text-foreground m-0 mb-2 text-xl leading-[1.3] font-semibold tracking-[-0.01em]">
          {{ t("system.globalSettings.runtime.title") }}
        </h2>
        <p class="text-muted-foreground m-0 max-w-[560px] text-sm leading-[1.6] text-pretty">
          {{ t("system.globalSettings.runtime.description") }}
        </p>
      </div>
      <div class="flex shrink-0 items-center gap-2.5 max-[860px]:w-full max-[860px]:justify-between">
        <label
          class="text-muted-foreground flex min-h-8 cursor-pointer items-center gap-[7px] text-[13px] whitespace-nowrap"
        >
          <span
            class="size-1.5 rounded-full transition-[background-color,box-shadow] duration-200"
            :class="autoRefresh ? 'bg-success shadow-[0_0_0_3px_var(--td-success-color-1)]' : 'bg-placeholder'"
          />
          <span>{{ t("system.globalSettings.runtime.autoRefresh") }}</span>
          <Switch v-model="autoRefresh" size="sm" :aria-label="t('system.globalSettings.runtime.autoRefresh')" />
        </label>
        <button
          type="button"
          data-slot="icon-button"
          class="text-placeholder enabled:hover:bg-secondary enabled:hover:text-primary enabled:active:bg-secondary inline-flex size-5 shrink-0 cursor-pointer items-center justify-center rounded-md transition-colors duration-200 ease-[cubic-bezier(0.16,1,0.3,1)] disabled:cursor-default disabled:opacity-70"
          :disabled="loading"
          :title="t('system.globalSettings.runtime.refresh')"
          :aria-label="t('system.globalSettings.runtime.refresh')"
          @click="reload"
        >
          <Loader2Icon v-if="loading" class="size-3 animate-spin" />
          <RefreshCwIcon v-else class="size-3" />
        </button>
      </div>
    </header>

    <div v-if="loading && !loadedOnce" class="grid gap-[22px] pt-1" aria-live="polite">
      <div
        class="border-border bg-border grid grid-cols-4 gap-px overflow-hidden rounded-[10px] border max-[860px]:grid-cols-2 max-[620px]:grid-cols-1"
      >
        <div v-for="n in 4" :key="n" class="bg-card flex flex-col gap-2 p-[18px]">
          <Skeleton class="h-7 w-[42%]" />
          <Skeleton class="h-3.5 w-[66%]" />
        </div>
      </div>
      <div class="flex flex-col gap-2">
        <Skeleton class="h-[42px] w-full" />
        <Skeleton class="h-12 w-full" />
        <Skeleton class="h-12 w-full" />
        <Skeleton class="h-12 w-full" />
      </div>
    </div>

    <div
      v-else-if="error"
      class="border-border bg-secondary grid min-h-28 grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3.5 rounded-[10px] border px-[22px] py-5 max-[620px]:grid-cols-[auto_minmax(0,1fr)]"
      role="alert"
    >
      <div class="text-destructive grid size-11 place-items-center rounded-lg bg-[var(--td-error-color-1)]">
        <CircleAlertIcon class="size-6" />
      </div>
      <div class="flex flex-col gap-[5px]">
        <strong class="text-sm font-semibold">{{ t("system.globalSettings.runtime.errors.generic") }}</strong>
        <span class="text-muted-foreground max-w-[560px] text-sm leading-[1.55]">{{ error }}</span>
      </div>
      <Button
        size="sm"
        variant="outline"
        class="max-[620px]:col-start-2 max-[620px]:justify-self-start"
        @click="reload"
      >
        {{ t("system.globalSettings.runtime.retry") }}
      </Button>
    </div>

    <div
      v-else-if="!available && !modelLimiterAvailable"
      class="border-border bg-secondary grid min-h-28 grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3.5 rounded-[10px] border px-[22px] py-5"
    >
      <div class="bg-card text-muted-foreground grid size-11 place-items-center rounded-lg">
        <InfoIcon class="size-6" />
      </div>
      <div class="flex flex-col gap-[5px]">
        <strong class="text-sm font-semibold">{{ t("system.globalSettings.runtime.unavailableTitle") }}</strong>
        <span class="text-muted-foreground max-w-[560px] text-sm leading-[1.55]">
          {{ t("system.globalSettings.runtime.unavailable") }}
        </span>
      </div>
    </div>

    <template v-else>
      <template v-if="available">
        <section
          class="bg-secondary mb-[30px] flex min-h-16 items-center gap-7 rounded-lg px-4 py-[13px] max-[860px]:flex-wrap max-[860px]:items-start"
          :aria-label="t('system.globalSettings.runtime.summary.title')"
        >
          <div
            class="text-foreground inline-flex min-w-28 items-center gap-[9px] text-sm font-semibold whitespace-nowrap"
          >
            <span class="bg-card text-primary grid size-7 place-items-center rounded-md text-[15px]">
              <ChartLineIcon class="size-4" />
            </span>
            <span>{{ t("system.globalSettings.runtime.summary.title") }}</span>
          </div>
          <div
            class="grid flex-1 grid-cols-[repeat(4,minmax(64px,1fr))] items-stretch gap-6 max-[860px]:w-full max-[620px]:grid-cols-2 max-[620px]:gap-4"
          >
            <div class="flex min-w-0 flex-col items-start justify-center gap-1">
              <span class="text-muted-foreground text-[13px] leading-[1.35] whitespace-nowrap">
                {{ t("system.globalSettings.runtime.summary.active") }}
              </span>
              <strong class="text-primary text-xl leading-[1.1] font-semibold tracking-[-0.02em] tabular-nums">
                {{ totalActive }}
              </strong>
            </div>
            <div class="flex min-w-0 flex-col items-start justify-center gap-1">
              <span class="text-muted-foreground text-[13px] leading-[1.35] whitespace-nowrap">
                {{ t("system.globalSettings.runtime.summary.pending") }}
              </span>
              <strong class="text-foreground text-xl leading-[1.1] font-semibold tracking-[-0.02em] tabular-nums">
                {{ totalPending }}
              </strong>
            </div>
            <div class="flex min-w-0 flex-col items-start justify-center gap-1">
              <span class="text-muted-foreground text-[13px] leading-[1.35] whitespace-nowrap">
                {{ t("system.globalSettings.runtime.summary.retry") }}
              </span>
              <strong
                class="text-xl leading-[1.1] font-semibold tracking-[-0.02em] tabular-nums"
                :class="totalRetry > 0 ? 'text-warning' : 'text-foreground'"
              >
                {{ totalRetry }}
              </strong>
            </div>
            <div class="flex min-w-0 flex-col items-start justify-center gap-1">
              <span class="text-muted-foreground text-[13px] leading-[1.35] whitespace-nowrap">
                {{ t("system.globalSettings.runtime.summary.archived") }}
              </span>
              <strong
                class="text-xl leading-[1.1] font-semibold tracking-[-0.02em] tabular-nums"
                :class="totalArchived > 0 ? 'text-destructive' : 'text-foreground'"
              >
                {{ totalArchived }}
              </strong>
            </div>
          </div>
        </section>
        <section class="mb-[30px]">
          <div class="mb-3.5 flex items-start justify-between gap-5 max-[860px]:flex-col">
            <div>
              <h3
                class="text-foreground before:bg-primary m-0 mb-1.5 flex items-center gap-2 text-[15px] leading-[1.35] font-semibold select-none before:h-[15px] before:w-[3px] before:shrink-0 before:rounded-[2px] before:content-['']"
              >
                {{ t("system.globalSettings.runtime.poolsTitle") }}
              </h3>
              <p class="text-muted-foreground m-0 text-[13px] leading-[1.55]">
                {{ t("system.globalSettings.runtime.poolsDescription") }}
              </p>
            </div>
            <span
              class="bg-secondary text-muted-foreground mt-0.5 shrink-0 rounded-full px-2.5 py-1 text-xs leading-[1.4] whitespace-nowrap"
            >
              {{ t("system.globalSettings.runtime.perInstance") }}
            </span>
          </div>
          <div class="grid grid-cols-4 gap-3 max-[860px]:grid-cols-2 max-[620px]:grid-cols-1">
            <div
              v-for="pool in pools"
              :key="pool.name"
              class="border-border bg-card flex min-w-0 flex-col gap-2 rounded-[10px] border px-[18px] py-4"
            >
              <div class="flex items-center justify-between gap-3">
                <span class="text-foreground text-sm leading-[1.35] font-medium">{{ poolLabel(pool.name) }}</span>
                <strong class="text-primary text-[22px] leading-none font-semibold tracking-[-0.02em] tabular-nums">
                  {{ pool.instances > 0 ? `${pool.active}/${pool.cluster_capacity}` : pool.concurrency }}
                </strong>
              </div>
              <p class="text-muted-foreground m-0 text-[13px] leading-[1.55] text-pretty">
                {{ poolDescription(pool.name) }}
                <span class="text-placeholder mt-1 block text-xs leading-[1.4]">
                  {{ t("system.globalSettings.runtime.poolConfigured", { value: pool.concurrency }) }}
                  <template v-if="pool.instances > 0">
                    · {{ t("system.globalSettings.runtime.poolInstances", { value: pool.instances }) }} ·
                    {{ t("system.globalSettings.runtime.poolUtilization", { value: poolUtilization(pool) }) }}
                  </template>
                  · {{ t("system.globalSettings.runtime.queueCount", { value: pool.queue_count }) }}
                </span>
              </p>
            </div>
          </div>
        </section>

        <section class="mt-0.5">
          <div class="mb-3.5 flex items-start justify-between gap-5 max-[860px]:flex-col">
            <div>
              <h3
                class="text-foreground before:bg-primary m-0 mb-1.5 flex items-center gap-2 text-[15px] leading-[1.35] font-semibold select-none before:h-[15px] before:w-[3px] before:shrink-0 before:rounded-[2px] before:content-['']"
              >
                {{ t("system.globalSettings.runtime.detailsTitle") }}
              </h3>
              <p class="text-muted-foreground m-0 text-[13px] leading-[1.55]">
                {{ t("system.globalSettings.runtime.detailsDescription") }}
              </p>
            </div>
            <span
              v-if="updatedAt"
              class="text-placeholder inline-flex shrink-0 items-center gap-[5px] text-xs tabular-nums"
            >
              <ClockIcon class="size-3.5" />
              {{ t("system.globalSettings.runtime.updatedAt", { value: updatedAt }) }}
            </span>
          </div>

          <div
            v-if="totalArchived > 0"
            class="border-border bg-card mb-3.5 flex items-start gap-3 rounded-[10px] border px-3.5 py-3"
            role="status"
          >
            <span
              class="bg-destructive/10 text-destructive grid size-7 shrink-0 place-items-center rounded-lg text-[16px]"
              aria-hidden="true"
            >
              <CircleAlertIcon class="size-4" />
            </span>
            <div class="flex min-w-0 flex-1 flex-col gap-0.5">
              <p class="text-foreground m-0 text-[13px] leading-[1.45] font-semibold tabular-nums">
                {{ t("system.globalSettings.runtime.failedNotice.title", { count: totalArchived }) }}
              </p>
              <p class="text-muted-foreground m-0 text-xs leading-[1.55]">
                {{ t("system.globalSettings.runtime.failedNotice.description") }}
              </p>
            </div>
          </div>

          <div
            v-if="queues.length === 0"
            class="border-border text-placeholder flex min-h-[180px] flex-col items-center justify-center gap-2.5 rounded-[10px] border border-dashed text-[13px]"
          >
            <ListTodoIcon class="size-7" />
            <span>{{ t("system.globalSettings.runtime.empty") }}</span>
          </div>

          <div v-else class="border-border bg-card overflow-x-auto rounded-[10px] border">
            <Table>
              <TableHeader>
                <TableRow class="hover:bg-transparent">
                  <TableHead
                    class="bg-secondary text-muted-foreground h-10 min-w-[188px] px-4 text-xs font-medium tracking-[0.01em] whitespace-nowrap"
                  >
                    {{ t("system.globalSettings.runtime.columns.queue") }}
                  </TableHead>
                  <TableHead
                    class="bg-secondary text-muted-foreground h-10 w-[74px] px-4 text-center text-xs font-medium tracking-[0.01em] whitespace-nowrap"
                  >
                    {{ t("system.globalSettings.runtime.columns.active") }}
                  </TableHead>
                  <TableHead
                    class="bg-secondary text-muted-foreground h-10 w-[84px] px-4 text-center text-xs font-medium tracking-[0.01em] whitespace-nowrap"
                  >
                    {{ t("system.globalSettings.runtime.columns.pending") }}
                  </TableHead>
                  <TableHead
                    class="bg-secondary text-muted-foreground h-10 w-[68px] px-4 text-center text-xs font-medium tracking-[0.01em] whitespace-nowrap"
                  >
                    {{ t("system.globalSettings.runtime.columns.retry") }}
                  </TableHead>
                  <TableHead
                    class="bg-secondary text-muted-foreground h-10 w-24 px-4 text-center text-xs font-medium tracking-[0.01em] whitespace-nowrap"
                  >
                    {{ t("system.globalSettings.runtime.columns.archived") }}
                  </TableHead>
                  <TableHead
                    class="bg-secondary text-muted-foreground h-10 w-[84px] px-4 text-center text-xs font-medium tracking-[0.01em] whitespace-nowrap"
                  >
                    {{ t("system.globalSettings.runtime.columns.completed") }}
                  </TableHead>
                  <TableHead
                    class="bg-secondary text-muted-foreground h-10 w-[104px] px-4 text-center text-xs font-medium tracking-[0.01em] whitespace-nowrap"
                  >
                    {{ t("system.globalSettings.runtime.columns.latency") }}
                  </TableHead>
                  <TableHead
                    class="bg-secondary text-muted-foreground h-10 w-24 px-4 text-xs font-medium tracking-[0.01em] whitespace-nowrap"
                  >
                    {{ t("system.globalSettings.runtime.columns.status") }}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-for="row in queues" :key="row.name" class="last:border-b-0">
                  <TableCell class="h-14 px-4 py-2.5 align-middle text-sm tabular-nums">
                    <div class="flex min-w-0 flex-col gap-[3px]">
                      <span
                        class="text-foreground overflow-hidden text-sm leading-[1.35] font-medium text-ellipsis whitespace-nowrap"
                      >
                        {{ queueLabel(row.name) }}
                      </span>
                      <span
                        class="text-placeholder [display:-webkit-box] overflow-hidden text-xs leading-[1.45] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]"
                      >
                        {{ queueMeta(row) }}
                      </span>
                    </div>
                  </TableCell>
                  <TableCell class="h-14 px-4 py-2.5 text-center align-middle text-sm tabular-nums">
                    <button
                      v-if="row.active > 0"
                      type="button"
                      class="text-primary hover:bg-accent inline-flex h-7 cursor-pointer items-center gap-0.5 rounded-[3px] px-0.5 text-xs font-semibold tabular-nums transition-colors"
                      @click="openRuntimeTasks(row, 'active')"
                    >
                      {{ row.active }}
                      <ChevronRightIcon class="size-3.5" />
                    </button>
                    <span v-else class="text-muted-foreground tabular-nums">0</span>
                  </TableCell>
                  <TableCell class="h-14 px-4 py-2.5 text-center align-middle text-sm tabular-nums">
                    <div class="flex flex-col items-center gap-px">
                      <button
                        v-if="row.pending > 0"
                        type="button"
                        class="text-foreground hover:bg-accent inline-flex h-7 cursor-pointer items-center gap-0.5 rounded-[3px] px-0.5 text-xs font-semibold tabular-nums transition-colors"
                        @click="openRuntimeTasks(row, 'pending')"
                      >
                        {{ row.pending }}
                        <ChevronRightIcon class="size-3.5" />
                      </button>
                      <span v-else class="text-muted-foreground tabular-nums">0</span>
                      <button
                        v-if="row.scheduled > 0"
                        type="button"
                        class="text-placeholder hover:bg-accent inline-flex h-[18px] cursor-pointer items-center rounded-[3px] px-0 text-[11px] transition-colors"
                        @click="openRuntimeTasks(row, 'scheduled')"
                      >
                        +{{ row.scheduled }} {{ t("system.globalSettings.runtime.columns.scheduled") }}
                      </button>
                    </div>
                  </TableCell>
                  <TableCell class="h-14 px-4 py-2.5 text-center align-middle text-sm tabular-nums">
                    <button
                      v-if="row.retry > 0"
                      type="button"
                      class="text-warning hover:bg-accent inline-flex h-7 cursor-pointer items-center gap-0.5 rounded-[3px] px-0.5 text-xs font-semibold tabular-nums transition-colors"
                      @click="openRuntimeTasks(row, 'retry')"
                    >
                      {{ row.retry }}
                      <ChevronRightIcon class="size-3.5" />
                    </button>
                    <span v-else class="text-muted-foreground tabular-nums">0</span>
                  </TableCell>
                  <TableCell class="h-14 px-4 py-2.5 text-center align-middle text-sm tabular-nums">
                    <button
                      v-if="row.archived > 0"
                      type="button"
                      class="text-destructive hover:bg-accent inline-flex h-7 cursor-pointer items-center gap-0.5 rounded-[3px] px-0.5 text-xs font-semibold tabular-nums transition-colors"
                      :aria-label="
                        t('system.globalSettings.runtime.tasks.openAria', {
                          state: taskStateLabel('archived'),
                          queue: queueLabel(row.name),
                          count: row.archived,
                        })
                      "
                      @click="openRuntimeTasks(row, 'archived')"
                    >
                      {{ row.archived }}
                      <ChevronRightIcon class="size-3.5" />
                    </button>
                    <span v-else class="text-muted-foreground tabular-nums">0</span>
                  </TableCell>
                  <TableCell class="h-14 px-4 py-2.5 text-center align-middle text-sm tabular-nums">
                    <button
                      v-if="row.completed > 0"
                      type="button"
                      class="text-success hover:bg-accent inline-flex h-7 cursor-pointer items-center gap-0.5 rounded-[3px] px-0.5 text-xs font-semibold tabular-nums transition-colors"
                      @click="openRuntimeTasks(row, 'completed')"
                    >
                      {{ row.completed }}
                      <ChevronRightIcon class="size-3.5" />
                    </button>
                    <span v-else class="text-muted-foreground tabular-nums">0</span>
                  </TableCell>
                  <TableCell class="h-14 px-4 py-2.5 text-center align-middle text-sm tabular-nums">
                    <span class="text-muted-foreground tabular-nums">{{ formatLatency(row.latency_ms) }}</span>
                  </TableCell>
                  <TableCell class="h-14 px-4 py-2.5 align-middle text-sm tabular-nums">
                    <span
                      class="inline-flex items-center gap-1.5 text-xs whitespace-nowrap"
                      :class="queueState(row).textClass"
                    >
                      <span class="size-1.5 rounded-full" :class="queueState(row).dotClass" />
                      {{ queueState(row).label }}
                    </span>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </div>
        </section>
      </template>

      <section class="mt-10 pt-8">
        <div class="mb-3.5 flex items-start justify-between gap-5 max-[860px]:flex-col">
          <div>
            <h3
              class="text-foreground before:bg-primary m-0 mb-1.5 flex items-center gap-2 text-[15px] leading-[1.35] font-semibold select-none before:h-[15px] before:w-[3px] before:shrink-0 before:rounded-[2px] before:content-['']"
            >
              {{ t("system.globalSettings.runtime.models.title") }}
            </h3>
            <p class="text-muted-foreground m-0 text-[13px] leading-[1.55]">
              {{ t("system.globalSettings.runtime.models.description") }}
            </p>
          </div>
          <span
            class="bg-secondary text-muted-foreground mt-0.5 shrink-0 rounded-full px-2.5 py-1 text-xs leading-[1.4] whitespace-nowrap"
          >
            {{ t("system.globalSettings.runtime.models.scope") }}
          </span>
        </div>
        <div
          v-if="!modelLimiterAvailable"
          class="border-border text-placeholder flex min-h-[180px] flex-col items-center justify-center gap-2.5 rounded-[10px] border border-dashed text-[13px]"
        >
          <InfoIcon class="size-7" />
          <span>{{ t("system.globalSettings.runtime.models.disabled") }}</span>
        </div>
        <div
          v-else-if="models.length === 0"
          class="border-border text-placeholder flex min-h-[180px] flex-col items-center justify-center gap-2.5 rounded-[10px] border border-dashed text-[13px]"
        >
          <ServerIcon class="size-7" />
          <span>{{ t("system.globalSettings.runtime.models.empty") }}</span>
        </div>
        <div v-else class="border-border bg-card overflow-x-auto rounded-[10px] border">
          <Table>
            <TableHeader>
              <TableRow class="hover:bg-transparent">
                <TableHead
                  class="bg-secondary text-muted-foreground h-10 min-w-[240px] px-4 text-xs font-medium tracking-[0.01em] whitespace-nowrap"
                >
                  {{ t("system.globalSettings.runtime.models.columns.model") }}
                </TableHead>
                <TableHead
                  class="bg-secondary text-muted-foreground h-10 w-[86px] px-4 text-center text-xs font-medium tracking-[0.01em] whitespace-nowrap"
                >
                  {{ t("system.globalSettings.runtime.models.columns.active") }}
                </TableHead>
                <TableHead
                  class="bg-secondary text-muted-foreground h-10 w-[86px] px-4 text-center text-xs font-medium tracking-[0.01em] whitespace-nowrap"
                >
                  {{ t("system.globalSettings.runtime.models.columns.waiting") }}
                </TableHead>
                <TableHead
                  class="bg-secondary text-muted-foreground h-10 w-[190px] px-4 text-xs font-medium tracking-[0.01em] whitespace-nowrap"
                >
                  {{ t("system.globalSettings.runtime.models.columns.usage") }}
                </TableHead>
                <TableHead
                  class="bg-secondary text-muted-foreground h-10 w-24 px-4 text-xs font-medium tracking-[0.01em] whitespace-nowrap"
                >
                  {{ t("system.globalSettings.runtime.columns.status") }}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="row in models" :key="row.model_id" class="last:border-b-0">
                <TableCell class="h-14 px-4 py-2.5 align-middle text-sm tabular-nums">
                  <div class="flex min-w-0 flex-col gap-[3px]">
                    <span
                      class="text-foreground overflow-hidden text-sm leading-[1.35] font-medium text-ellipsis whitespace-nowrap"
                    >
                      {{ row.name || row.model_id }}
                    </span>
                    <span
                      class="text-placeholder [display:-webkit-box] overflow-hidden text-xs leading-[1.45] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]"
                    >
                      {{ row.name ? row.model_id : t("system.globalSettings.runtime.models.backgroundOnly") }}
                    </span>
                  </div>
                </TableCell>
                <TableCell class="h-14 px-4 py-2.5 text-center align-middle text-sm tabular-nums">
                  <span
                    class="tabular-nums"
                    :class="row.active > 0 ? 'text-primary font-semibold' : 'text-muted-foreground'"
                  >
                    {{ row.active }}
                  </span>
                </TableCell>
                <TableCell class="h-14 px-4 py-2.5 text-center align-middle text-sm tabular-nums">
                  <span
                    class="tabular-nums"
                    :class="row.waiting > 0 ? 'text-warning font-semibold' : 'text-muted-foreground'"
                  >
                    {{ row.waiting }}
                  </span>
                </TableCell>
                <TableCell class="h-14 px-4 py-2.5 align-middle text-sm tabular-nums">
                  <div
                    class="text-muted-foreground grid grid-cols-[minmax(72px,1fr)_auto] items-center gap-2.5 text-xs tabular-nums"
                  >
                    <div class="h-1 overflow-hidden rounded-full bg-[var(--td-bg-color-component)]">
                      <div
                        class="bg-primary h-full rounded-full transition-[width] duration-200"
                        :style="{ width: `${modelUsage(row)}%` }"
                      />
                    </div>
                    <span>{{ row.active }} / {{ row.limit }}</span>
                  </div>
                </TableCell>
                <TableCell class="h-14 px-4 py-2.5 align-middle text-sm tabular-nums">
                  <span
                    class="inline-flex items-center gap-1.5 text-xs whitespace-nowrap"
                    :class="modelState(row).textClass"
                  >
                    <span class="size-1.5 rounded-full" :class="modelState(row).dotClass" />
                    {{ modelState(row).label }}
                  </span>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </section>

      <p class="text-placeholder m-0 mt-3 text-xs leading-[1.55]">
        {{ t("system.globalSettings.runtime.footnote") }}
      </p>
    </template>

    <SettingDrawer
      v-model:visible="taskDrawerVisible"
      :title="t('system.globalSettings.runtime.tasks.title', { queue: taskQueueLabel })"
      :description="t('system.globalSettings.runtime.tasks.description')"
      :icon="ListTodoIcon"
      width="720px"
      :min-width="520"
      :max-width="1040"
      storage-key="setting-drawer:width:runtime-tasks"
      hide-footer
    >
      <section class="border-border flex flex-col gap-3.5 border-b py-3 pb-4 first:pt-0 last:border-b-0 last:pb-0">
        <div
          class="border-border max-[620px]:no-scrollbar mb-3 flex border-b max-[620px]:overflow-x-auto"
          role="tablist"
          :aria-label="t('system.globalSettings.runtime.tasks.stateFilter')"
        >
          <button
            v-for="state in taskStates"
            :key="state"
            type="button"
            role="tab"
            class="focus-visible:outline-primary relative inline-flex min-w-0 flex-1 cursor-pointer items-center justify-center gap-1 border-0 bg-transparent px-1.5 py-2.5 text-[13px] whitespace-nowrap transition-colors duration-150 outline-none focus-visible:outline-2 focus-visible:-outline-offset-2 max-[620px]:min-w-[72px] max-[620px]:flex-none max-[620px]:px-2.5"
            :class="
              taskState === state
                ? 'text-primary after:bg-primary font-semibold after:absolute after:right-2 after:-bottom-px after:left-2 after:h-0.5 after:rounded-t-[2px]'
                : 'text-muted-foreground hover:text-foreground'
            "
            :aria-selected="taskState === state"
            @click="selectTaskState(state)"
          >
            <span class="overflow-hidden text-ellipsis">{{ taskStateLabel(state) }}</span>
            <span
              class="text-[11px] leading-none font-medium tabular-nums"
              :class="
                taskState === state
                  ? 'text-primary font-semibold'
                  : taskStateCount(taskQueue, state) > 0
                    ? 'text-muted-foreground'
                    : 'text-placeholder'
              "
            >
              {{ taskStateCount(taskQueue, state) }}
            </span>
          </button>
        </div>
        <p class="text-muted-foreground m-0 text-[13px] leading-[1.6]">{{ taskStateGuide }}</p>
      </section>

      <section class="border-border flex flex-col gap-3.5 border-b py-3 pb-4 first:pt-0 last:border-b-0 last:pb-0">
        <div class="mb-1 flex items-center justify-between gap-3">
          <h4
            class="text-foreground before:bg-primary m-0 flex items-center gap-2 text-[13px] font-semibold select-none before:h-3.5 before:w-[3px] before:shrink-0 before:rounded-[2px] before:content-['']"
          >
            {{ t("system.globalSettings.runtime.tasks.listTitle", { state: taskStateLabel(taskState) }) }}
          </h4>
          <div class="inline-flex shrink-0 items-center gap-0.5">
            <Popover v-if="taskState === 'archived' && tasks.length > 0">
              <PopoverTrigger as-child>
                <Button
                  variant="ghost"
                  size="sm"
                  class="text-destructive hover:text-destructive"
                  :disabled="purging || Boolean(taskActionID)"
                >
                  <Loader2Icon v-if="purging" class="animate-spin" />
                  <EraserIcon v-else />
                  {{ t("system.globalSettings.runtime.tasks.purgeArchived") }}
                </Button>
              </PopoverTrigger>
              <PopoverContent class="w-80">
                <p class="mb-3 text-[13px] leading-[1.5]">
                  {{
                    t("system.globalSettings.runtime.tasks.purgeArchivedConfirm", {
                      count: taskStateCount(taskQueue, "archived"),
                    })
                  }}
                </p>
                <div class="flex justify-end gap-2">
                  <PopoverClose as-child>
                    <Button size="sm" variant="outline">{{ t("common.cancel") }}</Button>
                  </PopoverClose>
                  <PopoverClose as-child>
                    <Button
                      size="sm"
                      variant="destructive"
                      class="bg-destructive text-primary-foreground hover:bg-destructive/80"
                      :disabled="purging"
                      @click="purgeArchivedTasks"
                    >
                      {{ t("common.confirm") }}
                    </Button>
                  </PopoverClose>
                </div>
              </PopoverContent>
            </Popover>
            <Button variant="ghost" size="sm" :disabled="tasksLoading && !tasksLoadingMore" @click="reloadRuntimeTasks">
              <Loader2Icon v-if="tasksLoading && !tasksLoadingMore" class="animate-spin" />
              <RefreshCwIcon v-else />
              {{ t("system.globalSettings.runtime.refresh") }}
            </Button>
          </div>
        </div>

        <div
          v-if="tasksLoading && tasks.length === 0"
          class="text-muted-foreground flex min-h-[180px] items-center justify-center gap-2.5 text-[13px]"
        >
          <Loader2Icon class="size-4 animate-spin" />
          <span>{{ t("system.globalSettings.runtime.loading") }}</span>
        </div>
        <div
          v-else-if="tasksError"
          class="border-border bg-card text-muted-foreground flex items-center justify-between gap-3 rounded-[10px] border px-3.5 py-3 text-[13px] leading-[1.55]"
        >
          <span>{{ tasksError }}</span>
          <Button size="sm" variant="outline" @click="reloadRuntimeTasks">
            {{ t("system.globalSettings.runtime.retry") }}
          </Button>
        </div>
        <Empty v-else-if="tasks.length === 0">
          <EmptyDescription>
            {{ t("system.globalSettings.runtime.tasks.empty", { state: taskStateLabel(taskState) }) }}
          </EmptyDescription>
        </Empty>
        <div v-else class="border-border bg-card overflow-hidden rounded-[10px] border">
          <article
            v-for="task in tasks"
            :key="task.id"
            class="border-border flex items-start gap-2 border-b py-3 pr-3 pl-4 last:border-b-0 max-[620px]:pl-3"
          >
            <div class="flex min-w-0 flex-1 flex-col gap-1">
              <div
                class="text-muted-foreground flex min-w-0 flex-wrap items-center gap-1.5 text-xs leading-[1.45] max-[620px]:gap-1"
              >
                <span class="text-foreground text-[13px] font-semibold">{{ runtimeTaskTypeLabel(task.type) }}</span>
                <span class="text-placeholder" aria-hidden="true">·</span>
                <span class="rounded-full px-1.5 py-px text-[11px]" :class="taskStatePillClass(task.state)">
                  {{ taskStateLabel(task.state) }}
                </span>
                <span class="text-placeholder" aria-hidden="true">·</span>
                <span class="tabular-nums">
                  {{
                    t("system.globalSettings.runtime.tasks.attempts", {
                      current: task.retried + 1,
                      max: task.max_retry + 1,
                    })
                  }}
                </span>
              </div>
              <dl v-if="runtimeTaskMeta(task).length > 0" class="m-0 mt-0.5 flex flex-col gap-1">
                <div
                  v-for="ref in runtimeTaskMeta(task)"
                  :key="ref.key"
                  class="m-0 grid grid-cols-[72px_minmax(0,1fr)] items-baseline gap-2"
                >
                  <dt class="text-placeholder m-0 text-xs leading-[1.45] whitespace-nowrap">{{ ref.label }}</dt>
                  <dd
                    class="text-muted-foreground m-0 overflow-hidden font-mono text-[11px] leading-[1.45] text-ellipsis whitespace-nowrap"
                    :title="ref.value"
                  >
                    {{ ref.value }}
                  </dd>
                </div>
              </dl>
              <p v-else class="text-placeholder m-0 mt-0.5 text-xs leading-[1.45]">
                {{ t("system.globalSettings.runtime.tasks.unknownTarget") }}
              </p>
              <p
                v-if="task.last_error"
                class="bg-secondary text-foreground m-0 mt-2 rounded-md px-2.5 py-2 font-mono text-[11px] leading-[1.55] wrap-anywhere whitespace-pre-wrap"
              >
                {{ task.last_error }}
              </p>
            </div>

            <div v-if="task.allowed_actions.length > 0" class="-mt-0.5 flex shrink-0 items-center">
              <Popover v-if="task.allowed_actions.includes('cancel')">
                <PopoverTrigger as-child>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    class="text-destructive hover:text-destructive"
                    :title="t('system.globalSettings.runtime.tasks.cancel')"
                    :aria-label="t('system.globalSettings.runtime.tasks.cancel')"
                    :disabled="Boolean(taskActionID)"
                  >
                    <Loader2Icon v-if="taskActionID === task.id && taskAction === 'cancel'" class="animate-spin" />
                    <XCircleIcon v-else />
                  </Button>
                </PopoverTrigger>
                <PopoverContent class="w-72">
                  <p class="mb-3 text-[13px] leading-[1.5]">
                    {{ t("system.globalSettings.runtime.tasks.cancelConfirm") }}
                  </p>
                  <div class="flex justify-end gap-2">
                    <PopoverClose as-child>
                      <Button size="sm" variant="outline">{{ t("common.cancel") }}</Button>
                    </PopoverClose>
                    <PopoverClose as-child>
                      <Button
                        size="sm"
                        variant="destructive"
                        class="bg-destructive text-primary-foreground hover:bg-destructive/80"
                        @click="runTaskAction(task, 'cancel')"
                      >
                        {{ t("common.confirm") }}
                      </Button>
                    </PopoverClose>
                  </div>
                </PopoverContent>
              </Popover>
              <Popover v-if="task.allowed_actions.includes('run_now')">
                <PopoverTrigger as-child>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    :title="t('system.globalSettings.runtime.tasks.runNow')"
                    :aria-label="t('system.globalSettings.runtime.tasks.runNow')"
                    :disabled="Boolean(taskActionID)"
                  >
                    <Loader2Icon v-if="taskActionID === task.id && taskAction === 'run_now'" class="animate-spin" />
                    <RotateCwIcon v-else />
                  </Button>
                </PopoverTrigger>
                <PopoverContent class="w-72">
                  <p class="mb-3 text-[13px] leading-[1.5]">
                    {{ t("system.globalSettings.runtime.tasks.runNowConfirm") }}
                  </p>
                  <div class="flex justify-end gap-2">
                    <PopoverClose as-child>
                      <Button size="sm" variant="outline">{{ t("common.cancel") }}</Button>
                    </PopoverClose>
                    <PopoverClose as-child>
                      <Button size="sm" @click="runTaskAction(task, 'run_now')">
                        {{ t("common.confirm") }}
                      </Button>
                    </PopoverClose>
                  </div>
                </PopoverContent>
              </Popover>
              <Popover v-if="task.allowed_actions.includes('delete')">
                <PopoverTrigger as-child>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    class="text-destructive hover:text-destructive"
                    :title="t('system.globalSettings.runtime.tasks.deleteRecord')"
                    :aria-label="t('system.globalSettings.runtime.tasks.deleteRecord')"
                    :disabled="Boolean(taskActionID)"
                  >
                    <Loader2Icon v-if="taskActionID === task.id && taskAction === 'delete'" class="animate-spin" />
                    <Trash2Icon v-else />
                  </Button>
                </PopoverTrigger>
                <PopoverContent class="w-72">
                  <p class="mb-3 text-[13px] leading-[1.5]">
                    {{ t("system.globalSettings.runtime.tasks.deleteConfirm") }}
                  </p>
                  <div class="flex justify-end gap-2">
                    <PopoverClose as-child>
                      <Button size="sm" variant="outline">{{ t("common.cancel") }}</Button>
                    </PopoverClose>
                    <PopoverClose as-child>
                      <Button
                        size="sm"
                        variant="destructive"
                        class="bg-destructive text-primary-foreground hover:bg-destructive/80"
                        @click="runTaskAction(task, 'delete')"
                      >
                        {{ t("common.confirm") }}
                      </Button>
                    </PopoverClose>
                  </div>
                </PopoverContent>
              </Popover>
            </div>
          </article>

          <div ref="tasksSentinelRef" class="h-px" aria-hidden="true" />

          <div class="border-border bg-secondary flex flex-col gap-2.5 border-t px-4 pt-3 pb-3.5">
            <span class="text-placeholder text-center text-xs leading-[1.5] tabular-nums">
              <template v-if="tasksLoadingMore">
                {{ t("system.globalSettings.runtime.tasks.loadingMore") }}
              </template>
              <template v-else-if="!tasksHasMore">
                {{ t("system.globalSettings.runtime.tasks.loadedAll", { count: tasks.length }) }}
              </template>
              <template v-else>
                {{ t("system.globalSettings.runtime.tasks.loadedSummary", { count: tasks.length }) }}
              </template>
            </span>
            <Button v-if="tasksHasMore" variant="outline" :disabled="tasksLoadingMore" @click="loadMoreRuntimeTasks">
              <Loader2Icon v-if="tasksLoadingMore" class="animate-spin" />
              {{ t("system.globalSettings.runtime.tasks.loadMore") }}
            </Button>
          </div>
        </div>
      </section>
    </SettingDrawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import SettingDrawer from "@/components/settings/SettingDrawer.vue";
import { mergeRuntimeTaskPage } from "./runtimeTaskPagination";
import {
  getRuntimeTasks,
  getRuntimeQueues,
  mutateRuntimeTask,
  purgeArchivedRuntimeTasks,
  type ModelRuntimeStat,
  type QueueStat,
  type RuntimeTask,
  type RuntimeTaskAction,
  type RuntimeTaskState,
  type RuntimeWorkerPool,
} from "@/api/system";

import { Button } from "@/components/ui/button";
import { Empty, EmptyDescription } from "@/components/ui/empty";
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
  ChartLineIcon,
  ChevronRightIcon,
  CircleAlertIcon,
  ClockIcon,
  EraserIcon,
  InfoIcon,
  ListTodoIcon,
  Loader2Icon,
  RefreshCwIcon,
  RotateCwIcon,
  ServerIcon,
  Trash2Icon,
  XCircleIcon,
} from "@lucide/vue";

const { t, te, locale } = useI18n();

const POLL_INTERVAL_MS = 5000;

const queues = ref<QueueStat[]>([]);
const pools = ref<RuntimeWorkerPool[]>([]);
const models = ref<ModelRuntimeStat[]>([]);
const modelLimiterAvailable = ref(false);
const available = ref(true);
const loading = ref(false);
const loadedOnce = ref(false);
const error = ref("");
const autoRefresh = ref(true);
const updatedAt = ref("");
const taskDrawerVisible = ref(false);
const taskQueue = ref<QueueStat | null>(null);
const taskState = ref<RuntimeTaskState>("archived");
const tasks = ref<RuntimeTask[]>([]);
const tasksLoading = ref(false);
const tasksLoadingMore = ref(false);
const tasksError = ref("");
const tasksCursor = ref("");
const tasksHasMore = ref(false);
const tasksSentinelRef = ref<HTMLElement | null>(null);
const taskActionID = ref("");
const taskAction = ref<RuntimeTaskAction | "">("");
const purging = ref(false);

const TASK_PAGE_SIZE = 20;
const taskStates: RuntimeTaskState[] = ["active", "pending", "scheduled", "retry", "archived", "completed"];
const runtimeTaskTypeKeys: Record<string, string> = {
  "document:process": "documentProcess",
  "manual:process": "manualProcess",
  "temporary_document:process": "temporaryDocumentProcess",
  "knowledge:post_process": "postProcess",
  "summary:generation": "summary",
  "datatable:summary": "tableSummary",
  "question:generation": "question",
  "image:multimodal": "multimodal",
  "chunk:extract": "graph",
  "datasource:sync": "sync",
  "faq:import": "faqImport",
  "knowledge:list_reparse": "batchReparse",
  "knowledge:list_delete": "batchDelete",
  "knowledge:move": "move",
  "index:delete": "indexDelete",
  "kb:clone": "kbClone",
  "kb:delete": "kbDelete",
  "wiki:ingest": "wikiIngest",
  "wiki:finalize": "wikiFinalize",
};

let pollTimer: ReturnType<typeof setInterval> | null = null;
let tasksScrollObserver: IntersectionObserver | null = null;
let tasksRequestID = 0;

function modelUsage(row: ModelRuntimeStat): number {
  return row.limit > 0 ? Math.min(100, Math.round((row.active / row.limit) * 100)) : 0;
}

const totalActive = computed(() => queues.value.reduce((s, q) => s + q.active, 0));
const totalPending = computed(() => queues.value.reduce((s, q) => s + q.pending, 0));
const totalRetry = computed(() => queues.value.reduce((s, q) => s + q.retry, 0));
const totalArchived = computed(() => queues.value.reduce((s, q) => s + q.archived, 0));
const taskQueueLabel = computed(() => (taskQueue.value ? queueLabel(taskQueue.value.name) : ""));
const taskStateGuide = computed(() => t(`system.globalSettings.runtime.tasks.guides.${taskState.value}`));

// Friendly per-queue label lives in i18n; falls back to the raw queue
// name so a queue added on the backend still renders before translations
// catch up.
function queueLabel(name: string): string {
  const path = `system.globalSettings.runtime.queueNames.${name}`;
  return te(path) ? (t(path) as string) : name;
}

function queueDescription(name: string): string {
  const path = `system.globalSettings.runtime.queueDescriptions.${name}`;
  return te(path) ? (t(path) as string) : name;
}

function queueMeta(row: QueueStat): string {
  const scope = queueDescription(row.name);
  if (poolQueueCount(row.pool) > 1) {
    return `${scope} · ${t("system.globalSettings.runtime.weightShort", { value: row.weight })}`;
  }
  return scope;
}

function runtimeTaskTypeLabel(type: string): string {
  const key = runtimeTaskTypeKeys[type];
  if (!key) return type;
  const path = `system.globalSettings.runtime.tasks.taskTypes.${key}`;
  return te(path) ? (t(path) as string) : type;
}

interface RuntimeTaskMeta {
  key: string;
  label: string;
  value: string;
}

function taskStateLabel(state: RuntimeTaskState): string {
  return t(`system.globalSettings.runtime.tasks.states.${state}`);
}

function taskStateCount(row: QueueStat | null, state: RuntimeTaskState): number {
  if (!row) return 0;
  return row[state] ?? 0;
}

function formatTaskTime(value?: string): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return date.toLocaleString(locale.value, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

function runtimeTaskMeta(task: RuntimeTask): RuntimeTaskMeta[] {
  const refs: RuntimeTaskMeta[] = [];
  if (task.knowledge_base_id) {
    refs.push({
      key: "kb",
      label: t("system.globalSettings.runtime.tasks.knowledgeBaseLabel"),
      value: task.knowledge_base_id,
    });
  }
  if (task.knowledge_id) {
    refs.push({
      key: "knowledge",
      label: t("system.globalSettings.runtime.tasks.knowledgeLabel"),
      value: task.knowledge_id,
    });
  }
  if (task.task_id) {
    refs.push({
      key: "task",
      label: t("system.globalSettings.runtime.tasks.taskIDLabel"),
      value: task.task_id,
    });
  }
  if (task.source_id)
    refs.push({ key: "source", label: t("system.globalSettings.runtime.tasks.sourceLabel"), value: task.source_id });
  if (task.target_id)
    refs.push({ key: "target", label: t("system.globalSettings.runtime.tasks.targetLabel"), value: task.target_id });
  if (task.source_kb_id)
    refs.push({
      key: "source-kb",
      label: t("system.globalSettings.runtime.tasks.sourceKBLabel"),
      value: task.source_kb_id,
    });
  if (task.target_kb_id)
    refs.push({
      key: "target-kb",
      label: t("system.globalSettings.runtime.tasks.targetKBLabel"),
      value: task.target_kb_id,
    });
  if (task.data_source_id)
    refs.push({
      key: "datasource",
      label: t("system.globalSettings.runtime.tasks.dataSourceLabel"),
      value: task.data_source_id,
    });
  if (task.sync_log_id)
    refs.push({
      key: "sync-log",
      label: t("system.globalSettings.runtime.tasks.syncLogLabel"),
      value: task.sync_log_id,
    });
  if (task.knowledge_count) {
    refs.push({
      key: "knowledge-count",
      label: t("system.globalSettings.runtime.tasks.knowledgeCountLabel"),
      value: String(task.knowledge_count),
    });
  }
  if (task.tenant_id) {
    refs.push({
      key: "tenant",
      label: t("system.globalSettings.runtime.tasks.tenantLabel"),
      value: String(task.tenant_id),
    });
  }
  if (task.enqueued_at)
    refs.push({
      key: "enqueued",
      label: t("system.globalSettings.runtime.tasks.enqueuedAt"),
      value: formatTaskTime(task.enqueued_at),
    });
  if (task.started_at)
    refs.push({
      key: "started",
      label: t("system.globalSettings.runtime.tasks.startedAt"),
      value: formatTaskTime(task.started_at),
    });
  if (task.next_process_at)
    refs.push({
      key: "next",
      label: t("system.globalSettings.runtime.tasks.nextProcessAt"),
      value: formatTaskTime(task.next_process_at),
    });
  if (task.last_failed_at)
    refs.push({
      key: "failed",
      label: t("system.globalSettings.runtime.tasks.lastFailedAt"),
      value: formatTaskTime(task.last_failed_at),
    });
  if (task.completed_at)
    refs.push({
      key: "completed",
      label: t("system.globalSettings.runtime.tasks.completedAt"),
      value: formatTaskTime(task.completed_at),
    });
  if (task.deadline)
    refs.push({
      key: "deadline",
      label: t("system.globalSettings.runtime.tasks.deadline"),
      value: formatTaskTime(task.deadline),
    });
  if (task.worker)
    refs.push({ key: "worker", label: t("system.globalSettings.runtime.tasks.worker"), value: task.worker });
  if (task.is_orphaned)
    refs.push({
      key: "orphaned",
      label: t("system.globalSettings.runtime.tasks.health"),
      value: t("system.globalSettings.runtime.tasks.orphaned"),
    });
  return refs;
}

function poolLabel(pool: string): string {
  const path = `system.globalSettings.runtime.pools.${pool}`;
  return te(path) ? (t(path) as string) : pool;
}

function poolDescription(pool: string): string {
  const path = `system.globalSettings.runtime.poolDescriptions.${pool}`;
  return te(path) ? (t(path) as string) : pool;
}

function poolQueueCount(pool: string): number {
  return pools.value.find((item) => item.name === pool)?.queue_count ?? 0;
}

function poolUtilization(pool: RuntimeWorkerPool): number {
  return Math.round(Math.max(0, Math.min(1, pool.utilization || 0)) * 100);
}

function formatLatency(ms: number): string {
  if (!ms || ms <= 0) return "—";
  if (ms < 1000) return `${ms} ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(1)} s`;
  const m = Math.floor(s / 60);
  const rem = Math.round(s % 60);
  return `${m}m ${rem}s`;
}

/** Status dot + text colours for the queue / model status cells. */
function statusClasses(tone: string): { dotClass: string; textClass: string } {
  switch (tone) {
    case "working":
      return { dotClass: "bg-primary", textClass: "text-muted-foreground" };
    case "attention":
    case "paused":
      return { dotClass: "bg-warning", textClass: "text-warning" };
    case "danger":
      return { dotClass: "bg-destructive", textClass: "text-destructive" };
    case "waiting":
      return { dotClass: "bg-warning", textClass: "text-muted-foreground" };
    default:
      return { dotClass: "bg-placeholder", textClass: "text-muted-foreground" };
  }
}

function queueState(row: QueueStat): { label: string; dotClass: string; textClass: string } {
  let tone: string;
  let label: string;
  if (row.paused) {
    tone = "paused";
    label = t("system.globalSettings.runtime.status.paused");
  } else if (row.archived > 0) {
    tone = "danger";
    label = t("system.globalSettings.runtime.status.actionRequired");
  } else if (row.retry > 0) {
    tone = "attention";
    label = t("system.globalSettings.runtime.status.retrying");
  } else if (row.active > 0) {
    tone = "working";
    label = t("system.globalSettings.runtime.status.working");
  } else if (row.pending > 0 || row.scheduled > 0) {
    tone = "waiting";
    label = t("system.globalSettings.runtime.status.waiting");
  } else {
    tone = "idle";
    label = t("system.globalSettings.runtime.status.idle");
  }
  return { label, ...statusClasses(tone) };
}

function modelState(row: ModelRuntimeStat): { label: string; dotClass: string; textClass: string } {
  let tone: string;
  let label: string;
  if (row.waiting > 0) {
    tone = "attention";
    label = t("system.globalSettings.runtime.models.status.queued");
  } else if (row.active >= row.limit) {
    tone = "waiting";
    label = t("system.globalSettings.runtime.models.status.full");
  } else if (row.active > 0) {
    tone = "working";
    label = t("system.globalSettings.runtime.status.working");
  } else {
    tone = "idle";
    label = t("system.globalSettings.runtime.status.idle");
  }
  return { label, ...statusClasses(tone) };
}

/** Tinted pill for a task's state, matching the old t-tag light themes. */
function taskStatePillClass(state: RuntimeTaskState): string {
  switch (state) {
    case "active":
      return "bg-[var(--td-brand-color-light)] text-primary";
    case "retry":
    case "scheduled":
      return "bg-[var(--td-warning-color-1)] text-warning";
    case "archived":
      return "bg-[var(--td-error-color-1)] text-destructive";
    case "completed":
      return "bg-[var(--td-success-color-1)] text-success";
    default:
      return "bg-secondary text-muted-foreground";
  }
}

async function fetchRuntimeTasks(reset: boolean) {
  const queue = taskQueue.value?.name;
  if (!queue) return;
  if (!reset && (tasksLoadingMore.value || !tasksHasMore.value)) return;

  const requestedState = taskState.value;
  const requestID = ++tasksRequestID;
  const cursor = reset ? "" : tasksCursor.value;
  if (reset) {
    tasksCursor.value = "";
    tasksHasMore.value = false;
    tasks.value = [];
    tasksLoading.value = true;
  } else {
    tasksLoadingMore.value = true;
  }
  tasksError.value = "";
  try {
    const response = await getRuntimeTasks(queue, requestedState, cursor, TASK_PAGE_SIZE);
    if (requestID !== tasksRequestID || taskQueue.value?.name !== queue || taskState.value !== requestedState) return;
    if (!response.available) {
      tasksError.value = t("system.globalSettings.runtime.tasks.unavailable");
      return;
    }
    tasks.value = reset ? response.tasks : mergeRuntimeTaskPage(tasks.value, response.tasks);
    tasksCursor.value = response.next_cursor || "";
    tasksHasMore.value = response.has_more && Boolean(response.next_cursor);
  } catch (err: any) {
    if (requestID !== tasksRequestID) return;
    if (!reset && err?.code === "runtime_task_cursor_expired") {
      tasksLoadingMore.value = false;
      await fetchRuntimeTasks(true);
      return;
    }
    tasksError.value = err?.message || t("system.globalSettings.runtime.tasks.loadError");
  } finally {
    if (requestID !== tasksRequestID) return;
    if (reset) {
      tasksLoading.value = false;
    } else {
      tasksLoadingMore.value = false;
    }
    await nextTick();
    attachTasksScrollObserver();
  }
}

function detachTasksScrollObserver() {
  tasksScrollObserver?.disconnect();
  tasksScrollObserver = null;
}

function attachTasksScrollObserver() {
  detachTasksScrollObserver();
  const sentinel = tasksSentinelRef.value;
  if (!sentinel || !taskDrawerVisible.value || !tasksHasMore.value) return;
  const root = sentinel.closest("[data-setting-drawer-body]") as HTMLElement | null;
  if (!root) return;
  tasksScrollObserver = new IntersectionObserver(
    (entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        void loadMoreRuntimeTasks();
      }
    },
    { root, rootMargin: "96px 0px", threshold: 0 },
  );
  tasksScrollObserver.observe(sentinel);
}

function openRuntimeTasks(row: QueueStat, state: RuntimeTaskState) {
  taskQueue.value = row;
  taskState.value = state;
  taskDrawerVisible.value = true;
  void fetchRuntimeTasks(true);
}

function selectTaskState(state: RuntimeTaskState) {
  if (taskState.value === state) return;
  taskState.value = state;
  void fetchRuntimeTasks(true);
}

function reloadRuntimeTasks() {
  return fetchRuntimeTasks(true);
}

function loadMoreRuntimeTasks() {
  if (tasksLoading.value || tasksLoadingMore.value || !tasksHasMore.value) return;
  return fetchRuntimeTasks(false);
}

async function runTaskAction(task: RuntimeTask, action: RuntimeTaskAction) {
  const queue = taskQueue.value?.name;
  if (!queue) return;
  taskActionID.value = task.id;
  taskAction.value = action;
  try {
    await mutateRuntimeTask(queue, task.id, action);
    MessagePlugin.success(t(`system.globalSettings.runtime.tasks.actionSuccess.${action}`));
    await Promise.all([reloadRuntimeTasks(), load(false)]);
    taskQueue.value = queues.value.find((item) => item.name === queue) ?? taskQueue.value;
  } catch (err: any) {
    MessagePlugin.error(err?.message || t(`system.globalSettings.runtime.tasks.actionError.${action}`));
  } finally {
    taskActionID.value = "";
    taskAction.value = "";
  }
}

async function purgeArchivedTasks() {
  const queue = taskQueue.value?.name;
  if (!queue || purging.value) return;
  purging.value = true;
  try {
    const { deleted } = await purgeArchivedRuntimeTasks(queue);
    MessagePlugin.success(t("system.globalSettings.runtime.tasks.purgeArchivedSuccess", { count: deleted }));
    await Promise.all([reloadRuntimeTasks(), load(false)]);
    taskQueue.value = queues.value.find((item) => item.name === queue) ?? taskQueue.value;
  } catch (err: any) {
    MessagePlugin.error(err?.message || t("system.globalSettings.runtime.tasks.purgeArchivedError"));
  } finally {
    purging.value = false;
  }
}

async function load(showSpinner: boolean) {
  if (showSpinner) loading.value = true;
  try {
    const resp = await getRuntimeQueues();
    available.value = resp.available;
    pools.value = resp.pools || [];
    queues.value = resp.queues || [];
    if (taskQueue.value) {
      taskQueue.value = queues.value.find((item) => item.name === taskQueue.value?.name) ?? taskQueue.value;
    }
    models.value = resp.models || [];
    modelLimiterAvailable.value = Boolean(resp.model_limiter_available);
    updatedAt.value = new Date((resp.timestamp || Date.now() / 1000) * 1000).toLocaleTimeString(locale.value, {
      hour12: false,
    });
    error.value = "";
    loadedOnce.value = true;
  } catch (err: any) {
    error.value = err?.message || t("system.globalSettings.runtime.errors.generic");
  } finally {
    if (showSpinner) loading.value = false;
  }
}

function reload() {
  load(true);
}

function startPolling() {
  stopPolling();
  if (!autoRefresh.value) return;
  pollTimer = setInterval(() => {
    // Silent background refresh — no spinner so the table doesn't flash.
    if (!loading.value) load(false);
  }, POLL_INTERVAL_MS);
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

watch(autoRefresh, (on) => {
  if (on) startPolling();
  else stopPolling();
});

watch(
  taskDrawerVisible,
  async (open) => {
    if (!open) {
      detachTasksScrollObserver();
      return;
    }
    await nextTick();
    attachTasksScrollObserver();
  },
  { flush: "post" },
);

watch(tasksHasMore, async () => {
  if (!taskDrawerVisible.value) return;
  await nextTick();
  attachTasksScrollObserver();
});

onMounted(() => {
  load(true);
  startPolling();
});

onUnmounted(() => {
  stopPolling();
  detachTasksScrollObserver();
});
</script>
