<template>
  <div
    class="relative flex min-h-full w-full overflow-hidden bg-[linear-gradient(225deg,#022c22_0%,#064e3b_15%,#065f46_25%,#047857_38%,#059669_50%,#07c05f_65%,#10b981_78%,#34d399_90%,#6ee7b7_100%)] max-[768px]:flex-col dark:bg-[linear-gradient(225deg,#011a14_0%,#032e22_15%,#043a2c_25%,#05503d_38%,#046647_50%,#038a56_65%,#049b60_78%,#06a06a_90%,#07b074_100%)]"
  >
    <div
      class="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_20%_50%,rgba(255,255,255,0.06)_0%,transparent_50%),radial-gradient(circle_at_80%_50%,rgba(255,255,255,0.04)_0%,transparent_50%)]"
    />

    <!-- Animated knowledge-graph background -->
    <div
      class="pointer-events-none absolute inset-0 z-[1] overflow-hidden [contain:strict] motion-reduce:hidden max-[480px]:hidden"
    >
      <div
        v-for="node in KNOWLEDGE_NODES"
        :key="node.n"
        class="absolute flex size-10 animate-[login-node-pulse_5s_ease-in-out_infinite] items-center justify-center rounded-full border-2 border-white/30 bg-white/15 shadow-[0_0_15px_rgba(255,255,255,0.35),0_0_30px_rgba(16,185,129,0.2),inset_0_0_8px_rgba(255,255,255,0.1)] will-change-[transform,opacity] motion-reduce:animate-none motion-reduce:opacity-[0.65] dark:border-white/20 dark:bg-white/10 dark:shadow-[0_0_8px_rgba(255,255,255,0.15)]"
        :class="node.responsiveHide ? 'max-[768px]:hidden' : ''"
        :style="node.style"
      >
        <svg class="size-5 text-white/90" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <template v-if="node.icon === 'book'">
            <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
            <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
          </template>
          <template v-else-if="node.icon === 'folder'">
            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
          </template>
          <template v-else-if="node.icon === 'layers'">
            <path d="M12 2L2 7l10 5 10-5-10-5z" />
            <path d="M2 17l10 5 10-5" />
            <path d="M2 12l10 5 10-5" />
          </template>
          <template v-else-if="node.icon === 'database'">
            <ellipse cx="12" cy="5" rx="9" ry="3" />
            <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3" />
            <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5" />
          </template>
          <template v-else-if="node.icon === 'search'">
            <circle cx="11" cy="11" r="8" />
            <path d="m21 21-4.35-4.35" />
          </template>
          <template v-else-if="node.icon === 'box'">
            <path
              d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"
            />
            <polyline points="3.27 6.96 12 12.01 20.73 6.96" />
            <line x1="12" y1="22.08" x2="12" y2="12" />
          </template>
          <template v-else-if="node.icon === 'file'">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
            <polyline points="14 2 14 8 20 8" />
          </template>
          <template v-else-if="node.icon === 'users'">
            <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
            <circle cx="9" cy="7" r="4" />
            <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
            <path d="M16 3.13a4 4 0 0 1 0 7.75" />
          </template>
          <template v-else-if="node.icon === 'message'">
            <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
          </template>
          <template v-else-if="node.icon === 'settings'">
            <circle cx="12" cy="12" r="3" />
            <path
              d="M12 1v6m0 6v6M5.64 5.64l4.24 4.24m4.24 4.24l4.24 4.24M1 12h6m6 0h6M5.64 18.36l4.24-4.24m4.24-4.24l4.24-4.24"
            />
          </template>
          <template v-else-if="node.icon === 'check'">
            <path d="M9 11l3 3L22 4" />
            <path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" />
          </template>
          <template v-else>
            <polygon
              points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"
            />
          </template>
        </svg>
      </div>

      <svg class="absolute inset-0 h-full w-full opacity-[0.35]" viewBox="0 0 100 100" preserveAspectRatio="none">
        <line
          v-for="line in KNOWLEDGE_LINES"
          :key="line.n"
          :x1="line.x1"
          :y1="line.y1"
          :x2="line.x2"
          :y2="line.y2"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-dasharray="6 3"
          class="animate-[login-line-flow_10s_linear_infinite] stroke-white/50 will-change-[stroke-dashoffset] motion-reduce:animate-none dark:stroke-white/25"
          :class="line.responsiveHide ? 'max-[768px]:hidden' : ''"
          :style="{ animationDelay: line.delay }"
        />
      </svg>
    </div>

    <!-- Logo - Top Left -->
    <div
      class="fixed top-8 left-[50px] z-[100] flex cursor-pointer items-center gap-2.5 max-[1024px]:top-[26px] max-[1024px]:left-10 max-[768px]:top-[22px] max-[768px]:left-[30px] max-[480px]:top-[18px] max-[480px]:left-5"
    >
      <img :src="yuhengMark" alt="" class="size-8 shrink-0 rounded-[8px] max-[768px]:size-7" draggable="false" />
      <span
        class="text-foreground inline-block text-[22px] leading-[1.2] font-bold tracking-[-0.01em] whitespace-nowrap select-none max-[1024px]:text-[19px] max-[768px]:text-[17px] max-[480px]:text-[15px] dark:[filter:invert(1)_hue-rotate(180deg)_brightness(1.1)]"
        >Yuheng</span
      >
    </div>

    <!-- Header Links - Top Right -->
    <div
      class="fixed top-7 right-7 z-[100] flex items-center gap-2.5 max-[1024px]:top-[22px] max-[1024px]:right-[22px] max-[1024px]:gap-2 max-[768px]:top-[18px] max-[768px]:right-[18px] max-[480px]:top-3.5 max-[480px]:right-3.5 max-[480px]:flex-wrap max-[480px]:gap-1.5"
    >
      <div ref="languageSwitchRef" class="relative">
        <button
          type="button"
          data-slot="language-switch"
          class="relative flex cursor-pointer items-center gap-[7px] rounded-[20px] border border-white/25 bg-white/20 px-[15px] py-[9px] text-[13px] font-semibold tracking-[0.2px] text-white hover:border-white/40 hover:bg-white/30 max-[1024px]:gap-0 max-[1024px]:p-2.5 max-[768px]:px-3 max-[768px]:py-2 max-[768px]:text-xs max-[480px]:px-2.5 max-[480px]:py-[7px] max-[480px]:text-[11px] dark:border-white/[0.15] dark:bg-white/[0.12] dark:hover:bg-white/20"
          :title="currentLangOption?.label"
          @click="toggleLanguageMenu"
        >
          <span class="shrink-0 text-base leading-none">{{ currentLangOption?.flag }}</span>
          <span class="leading-none max-[1024px]:hidden max-[768px]:inline">{{ currentLangOption?.shortLabel }}</span>
          <ChevronDownIcon class="ml-0.5 size-3 shrink-0" stroke-width="2.5" />
        </button>

        <!-- Language Dropdown -->
        <div
          v-if="showLanguageMenu"
          class="border-border absolute top-[calc(100%+8px)] right-0 z-[1000] min-w-40 overflow-hidden rounded-lg border bg-white/[0.97] shadow-[0_4px_16px_rgba(0,0,0,0.12)] dark:bg-[rgba(36,36,36,0.97)] dark:shadow-[0_4px_16px_rgba(0,0,0,0.4)]"
        >
          <div
            v-for="lang in languageOptions"
            :key="lang.value"
            class="flex cursor-pointer items-center gap-2.5 px-3.5 py-2.5 text-[13px]"
            :class="
              currentLanguage === lang.value
                ? 'bg-[var(--td-success-color-light)] text-[var(--td-brand-color-active)]'
                : 'text-foreground hover:bg-secondary'
            "
            @click="selectLanguage(lang.value)"
          >
            <span class="shrink-0 text-base">{{ lang.flag }}</span>
            <span class="flex-1">{{ lang.label }}</span>
            <span v-if="currentLanguage === lang.value" class="text-success shrink-0 text-sm font-bold">✓</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Left Showcase Section -->
    <div
      class="relative box-border flex flex-[0_0_52%] items-end pt-[100px] pr-[30px] pb-[100px] pl-[50px] max-[768px]:min-h-[50vh] max-[768px]:flex-none max-[768px]:items-end max-[768px]:p-10 max-[768px]:px-6 max-[480px]:items-end max-[480px]:p-8 max-[480px]:px-5"
    >
      <div class="relative z-[2] mb-[60px] flex w-full max-w-[600px] flex-col max-[768px]:max-w-full">
        <p
          class="m-0 mb-2 text-[22px] leading-[1.4] font-medium text-white/95 max-[1024px]:text-lg max-[768px]:mb-6 max-[768px]:text-base max-[480px]:text-sm"
        >
          {{ $t("platform.subtitle") }}
        </p>
        <p class="m-0 mb-7 text-[15px] leading-[1.5] text-white/80">{{ $t("platform.description") }}</p>

        <div class="mb-10 flex flex-wrap gap-3 max-[768px]:mb-6">
          <span
            class="inline-block rounded-[20px] bg-white/20 px-5 py-2 text-sm font-medium text-white max-[480px]:px-4 max-[480px]:py-1.5 max-[480px]:text-xs dark:bg-white/[0.12]"
          >
            {{ $t("platform.rag") }}
          </span>
          <span
            class="inline-block rounded-[20px] bg-white/20 px-5 py-2 text-sm font-medium text-white max-[480px]:px-4 max-[480px]:py-1.5 max-[480px]:text-xs dark:bg-white/[0.12]"
          >
            {{ $t("platform.wiki") }}
          </span>
          <span
            class="inline-block rounded-[20px] bg-white/20 px-5 py-2 text-sm font-medium text-white max-[480px]:px-4 max-[480px]:py-1.5 max-[480px]:text-xs dark:bg-white/[0.12]"
          >
            {{ $t("platform.hybridSearch") }}
          </span>
        </div>

        <!-- Swiper Carousel -->
        <div class="mt-12 w-full max-[768px]:mt-6">
          <swiper
            :modules="modules"
            :slides-per-view="1"
            :loop="true"
            :autoplay="{
              delay: 4000,
              disableOnInteraction: false,
            }"
            :effect="'fade'"
            :fade-effect="{ crossFade: true }"
            :pagination="{ clickable: true, dynamicBullets: false }"
            :speed="800"
            class="screenshot-swiper w-full overflow-hidden rounded-2xl shadow-[0_20px_60px_rgba(0,0,0,0.3)]"
          >
            <swiper-slide v-for="(slide, index) in slides" :key="index">
              <div class="bg-card flex h-full w-full items-center justify-center overflow-hidden rounded-2xl">
                <img :src="slide.image" :alt="slide.title" class="block h-full w-full object-contain" />
              </div>
            </swiper-slide>
          </swiper>
        </div>
      </div>
    </div>

    <!-- Right Form Section -->
    <div
      class="relative box-border flex flex-[0_0_48%] items-end justify-center pt-10 pr-[50px] pb-[100px] pl-[30px] max-[768px]:flex-none max-[768px]:items-end max-[768px]:p-6 max-[480px]:p-5"
    >
      <div class="relative z-[2] mb-[60px] w-full max-w-[480px]">
        <!-- Login Card -->
        <div
          v-if="!isRegisterMode"
          class="box-border w-full rounded-2xl border-0 bg-white/[0.97] p-10 shadow-[0_10px_40px_rgba(0,0,0,0.15)] max-[768px]:p-[32px_24px] max-[480px]:p-[28px_20px] dark:bg-[rgba(36,36,36,0.97)] dark:shadow-[0_10px_40px_rgba(0,0,0,0.4)]"
        >
          <!-- invite_only 模式下共享链接停在登录卡，同样需要邀请上下文。 -->
          <div
            v-if="inviteLookup"
            class="border-border bg-accent text-foreground mb-5 flex items-start gap-2.5 rounded-[10px] border px-3.5 py-3"
          >
            <LinkIcon class="text-muted-foreground mt-0.5 size-[18px] shrink-0" />
            <div class="flex min-w-0 flex-col gap-0.5">
              <div class="text-foreground text-sm leading-[1.4] font-semibold">
                {{ $t("inviteRegister.bannerTitle", { tenant: inviteLookup.tenant_name || "" }) }}
              </div>
              <div class="text-muted-foreground text-xs leading-[1.5]">
                {{ $t("inviteRegister.bannerHintLogin") }}
              </div>
            </div>
          </div>
          <div
            v-else-if="inviteLookupError"
            class="text-destructive mb-5 flex items-start gap-2.5 rounded-[10px] border border-[var(--td-error-color-3)] bg-[var(--td-error-color-1)] px-3.5 py-3 text-[13px]"
          >
            {{ inviteLookupError }}
          </div>
          <div class="mb-8 text-center max-[480px]:mb-6">
            <h2 class="text-foreground m-0 mb-1.5 text-2xl font-semibold max-[768px]:text-[22px]">
              {{ $t("auth.login") }}
            </h2>
            <p class="text-muted-foreground m-0 text-[13px]">{{ $t("auth.subtitle") }}</p>
            <p
              v-if="registrationEnabled"
              class="m-0 mt-2.5 rounded-lg bg-[var(--td-success-color-light)] px-3 py-2 text-[12.5px] leading-[1.5] text-[var(--td-brand-color-active)]"
            >
              {{ $t("auth.loginHint") }}
            </p>
          </div>

          <div>
            <form @submit.prevent="handleLogin">
              <div class="mb-6">
                <Label for="login-email" :class="FIELD_LABEL_CLASS">{{ $t("auth.email") }}</Label>
                <div class="relative">
                  <Input
                    id="login-email"
                    v-model="formData.email"
                    :placeholder="$t('auth.emailPlaceholder')"
                    type="text"
                    autocomplete="email"
                    :disabled="loading"
                    :aria-invalid="loginErrors.email ? true : undefined"
                    :class="FIELD_INPUT_CLASS"
                    @update:model-value="(v) => validateField(loginErrors, 'email', v)"
                  />
                  <p v-if="loginErrors.email" :class="FIELD_ERROR_CLASS">{{ $t(loginErrors.email) }}</p>
                </div>
              </div>
              <div :class="registrationEnabled || oidcEnabled ? 'mb-6' : 'mb-0'">
                <Label for="login-password" :class="FIELD_LABEL_CLASS">{{ $t("auth.password") }}</Label>
                <div class="relative">
                  <Input
                    id="login-password"
                    v-model="formData.password"
                    :placeholder="$t('auth.passwordPlaceholder')"
                    type="password"
                    autocomplete="current-password"
                    :disabled="loading"
                    :aria-invalid="loginErrors.password ? true : undefined"
                    :class="FIELD_INPUT_CLASS"
                    @update:model-value="(v) => validateField(loginErrors, 'password', v)"
                    @keydown.enter.prevent="handleLogin"
                  />
                  <p v-if="loginErrors.password" :class="FIELD_ERROR_CLASS">{{ $t(loginErrors.password) }}</p>
                </div>
              </div>

              <Button
                type="submit"
                size="lg"
                :disabled="loading"
                class="my-5 mb-4 h-[46px] w-full rounded-lg text-base font-medium"
              >
                <Loader2Icon v-if="loading" class="animate-spin" />
                {{ loading ? $t("auth.loggingIn") : $t("auth.login") }}
              </Button>

              <div v-if="registrationEnabled" class="mt-2">
                <div
                  class="text-muted-foreground before:border-border relative my-1 mb-3.5 text-center text-[13px] before:absolute before:inset-x-0 before:top-1/2 before:border-t"
                >
                  <span class="relative z-[1] bg-white/[0.97] px-3 dark:bg-[rgba(36,36,36,0.97)]">
                    {{ $t("auth.firstTime") }}
                  </span>
                </div>
                <Button
                  type="button"
                  variant="outline"
                  size="lg"
                  :disabled="loading"
                  class="border-primary text-primary dark:border-primary h-[46px] w-full rounded-lg bg-transparent text-[15px] font-medium hover:border-[var(--td-brand-color-active)] hover:bg-[var(--td-success-color-light)] hover:text-[var(--td-brand-color-active)] dark:bg-transparent dark:hover:bg-[var(--td-success-color-light)]"
                  @click="toggleMode"
                >
                  {{ $t("auth.createAccount") }}
                </Button>
              </div>

              <div
                v-if="oidcEnabled"
                class="text-placeholder before:border-border relative my-1 mb-1.5 text-center text-xs before:absolute before:inset-x-0 before:top-1/2 before:border-t"
              >
                <span class="relative z-[1] bg-white/[0.95] px-3 dark:bg-[rgba(36,36,36,0.97)]">
                  {{ $t("auth.orContinueWith") }}
                </span>
              </div>

              <Button
                v-if="oidcEnabled"
                type="button"
                variant="secondary"
                size="lg"
                :disabled="loading || oidcLoading"
                class="text-foreground h-[46px] w-full rounded-lg border-[var(--td-bg-color-component)] bg-[var(--td-bg-color-component)] text-[15px] font-medium hover:border-[var(--td-bg-color-component-hover)] hover:bg-[var(--td-bg-color-component-hover)] disabled:border-[var(--td-bg-color-component-disabled)] disabled:bg-[var(--td-bg-color-component-disabled)] disabled:opacity-100"
                @click="handleOIDCLogin"
              >
                <Loader2Icon v-if="oidcLoading" class="animate-spin" />
                {{ oidcLoading ? $t("auth.redirectingToOIDC") : oidcLoginText }}
              </Button>
            </form>

            <!-- Features list -->
            <div class="mt-5 p-0">
              <div class="text-muted-foreground mb-3 flex items-center text-[13px] last:mb-0">
                <span
                  class="mr-2.5 flex size-5 shrink-0 items-center justify-center rounded-full bg-[var(--td-success-color-light)] text-xs font-bold text-[var(--td-brand-color-active)] dark:bg-[rgba(6,176,77,0.15)]"
                  >✓</span
                >
                <span class="leading-[1.4]">{{ $t("platform.multimodalParsing") }}</span>
              </div>
              <div class="text-muted-foreground mb-3 flex items-center text-[13px] last:mb-0">
                <span
                  class="mr-2.5 flex size-5 shrink-0 items-center justify-center rounded-full bg-[var(--td-success-color-light)] text-xs font-bold text-[var(--td-brand-color-active)] dark:bg-[rgba(6,176,77,0.15)]"
                  >✓</span
                >
                <span class="leading-[1.4]">{{ $t("platform.hybridSearchEngine") }}</span>
              </div>
              <div class="text-muted-foreground mb-3 flex items-center text-[13px] last:mb-0">
                <span
                  class="mr-2.5 flex size-5 shrink-0 items-center justify-center rounded-full bg-[var(--td-success-color-light)] text-xs font-bold text-[var(--td-brand-color-active)] dark:bg-[rgba(6,176,77,0.15)]"
                  >✓</span
                >
                <span class="leading-[1.4]">{{ $t("platform.ragQandA") }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Register Card. Renders when the user is in register mode
             AND either self-service registration is enabled OR they
             arrived with a valid share-link token (which bypasses the
             invite_only gate). -->
        <div
          v-if="isRegisterMode && (registrationEnabled || inviteLookup)"
          class="box-border w-full rounded-2xl border-0 bg-white/[0.97] p-10 shadow-[0_10px_40px_rgba(0,0,0,0.15)] max-[768px]:p-[32px_24px] max-[480px]:p-[28px_20px] dark:bg-[rgba(36,36,36,0.97)] dark:shadow-[0_10px_40px_rgba(0,0,0,0.4)]"
        >
          <!-- Share-link banner: shown only when ?token= resolved to a
               real invitation row. Sits above the form header so the
               invitee instantly sees who invited them and into which
               workspace, without bumping the existing register UX. -->
          <div
            v-if="inviteLookup"
            class="border-border bg-accent text-foreground mb-5 flex items-start gap-2.5 rounded-[10px] border px-3.5 py-3"
          >
            <LinkIcon class="text-muted-foreground mt-0.5 size-[18px] shrink-0" />
            <div class="flex min-w-0 flex-col gap-0.5">
              <div class="text-foreground text-sm leading-[1.4] font-semibold">
                {{ $t("inviteRegister.bannerTitle", { tenant: inviteLookup.tenant_name || "" }) }}
              </div>
              <div class="text-muted-foreground text-xs leading-[1.5]">{{ $t("inviteRegister.bannerHint") }}</div>
            </div>
          </div>
          <div
            v-else-if="inviteLookupError"
            class="text-destructive mb-5 flex items-start gap-2.5 rounded-[10px] border border-[var(--td-error-color-3)] bg-[var(--td-error-color-1)] px-3.5 py-3 text-[13px]"
          >
            {{ inviteLookupError }}
          </div>
          <div class="mb-8 text-center max-[480px]:mb-6">
            <h2 class="text-foreground m-0 mb-1.5 text-2xl font-semibold max-[768px]:text-[22px]">
              {{ $t("auth.createAccount") }}
            </h2>
            <p class="text-muted-foreground m-0 text-[13px]">{{ $t("auth.registerSubtitle") }}</p>
          </div>

          <div>
            <form @submit.prevent="handleRegister">
              <div class="mb-6">
                <Label for="register-username" :class="FIELD_LABEL_CLASS">{{ $t("auth.username") }}</Label>
                <div class="relative">
                  <Input
                    id="register-username"
                    v-model="registerData.username"
                    :placeholder="$t('auth.usernamePlaceholder')"
                    :disabled="loading"
                    :aria-invalid="registerErrors.username ? true : undefined"
                    :class="FIELD_INPUT_CLASS"
                    @update:model-value="(v) => validateField(registerErrors, 'username', v)"
                  />
                  <p v-if="registerErrors.username" :class="FIELD_ERROR_CLASS">{{ $t(registerErrors.username) }}</p>
                </div>
              </div>
              <div class="mb-6">
                <Label for="register-email" :class="FIELD_LABEL_CLASS">{{ $t("auth.email") }}</Label>
                <div class="relative">
                  <Input
                    id="register-email"
                    v-model="registerData.email"
                    :placeholder="$t('auth.emailPlaceholder')"
                    type="text"
                    autocomplete="email"
                    :disabled="loading"
                    :aria-invalid="registerErrors.email ? true : undefined"
                    :class="FIELD_INPUT_CLASS"
                    @update:model-value="(v) => validateField(registerErrors, 'email', v)"
                  />
                  <p v-if="registerErrors.email" :class="FIELD_ERROR_CLASS">{{ $t(registerErrors.email) }}</p>
                </div>
              </div>
              <div class="mb-6">
                <Label for="register-password" :class="FIELD_LABEL_CLASS">{{ $t("auth.password") }}</Label>
                <div class="relative">
                  <Input
                    id="register-password"
                    v-model="registerData.password"
                    :placeholder="$t('auth.passwordPlaceholder')"
                    type="password"
                    autocomplete="new-password"
                    :disabled="loading"
                    :aria-invalid="registerErrors.password ? true : undefined"
                    :class="FIELD_INPUT_CLASS"
                    @update:model-value="(v) => validateField(registerErrors, 'password', v)"
                  />
                  <p v-if="registerErrors.password" :class="FIELD_ERROR_CLASS">{{ $t(registerErrors.password) }}</p>
                </div>
              </div>
              <div class="mb-0">
                <Label for="register-confirmPassword" :class="FIELD_LABEL_CLASS">{{
                  $t("auth.confirmPassword")
                }}</Label>
                <div class="relative">
                  <Input
                    id="register-confirmPassword"
                    v-model="registerData.confirmPassword"
                    :placeholder="$t('auth.confirmPasswordPlaceholder')"
                    type="password"
                    autocomplete="new-password"
                    :disabled="loading"
                    :aria-invalid="registerErrors.confirmPassword ? true : undefined"
                    :class="FIELD_INPUT_CLASS"
                    @update:model-value="(v) => validateField(registerErrors, 'confirmPassword', v)"
                    @keydown.enter.prevent="handleRegister"
                  />
                  <p v-if="registerErrors.confirmPassword" :class="FIELD_ERROR_CLASS">
                    {{ $t(registerErrors.confirmPassword) }}
                  </p>
                </div>
              </div>
              <!-- The first account of a deployment also creates its default
                   workspace, so only that registrant gets to name it. Every
                   later account joins workspaces through an administrator, and
                   the server ignores the field for them anyway. -->
              <div v-if="isFirstUser" class="mt-6 mb-0">
                <Label for="register-workspace-name" :class="FIELD_LABEL_CLASS">{{ $t("auth.workspaceName") }}</Label>
                <div class="relative">
                  <Input
                    id="register-workspace-name"
                    v-model="registerData.workspaceName"
                    :placeholder="$t('auth.workspaceNamePlaceholder')"
                    maxlength="128"
                    :disabled="loading"
                    :class="FIELD_INPUT_CLASS"
                    @keydown.enter.prevent="handleRegister"
                  />
                  <p class="text-muted-foreground mt-1.5 mb-0 text-xs leading-[1.5]">
                    {{ $t("auth.workspaceNameHint") }}
                  </p>
                </div>
              </div>

              <Button
                type="submit"
                size="lg"
                :disabled="loading"
                class="my-5 mb-4 h-[46px] w-full rounded-lg text-base font-medium"
              >
                <Loader2Icon v-if="loading" class="animate-spin" />
                {{ loading ? $t("auth.registering") : $t("auth.register") }}
              </Button>
            </form>

            <div class="border-border text-muted-foreground mt-4 border-b pb-4 text-center text-sm">
              <span>{{ $t("auth.haveAccount") }}</span>
              <a
                href="#"
                class="text-primary ml-1 font-medium no-underline transition-all duration-200 hover:underline"
                @click.prevent="toggleMode"
              >
                {{ $t("auth.backToLogin") }}
              </a>
            </div>

            <!-- Features list for register -->
            <div class="mt-5 p-0">
              <div class="text-muted-foreground mb-3 flex items-center text-[13px] last:mb-0">
                <span
                  class="mr-2.5 flex size-5 shrink-0 items-center justify-center rounded-full bg-[var(--td-success-color-light)] text-xs font-bold text-[var(--td-brand-color-active)] dark:bg-[rgba(6,176,77,0.15)]"
                  >✓</span
                >
                <span class="leading-[1.4]">{{ $t("platform.independentTenant") }}</span>
              </div>
              <div class="text-muted-foreground mb-3 flex items-center text-[13px] last:mb-0">
                <span
                  class="mr-2.5 flex size-5 shrink-0 items-center justify-center rounded-full bg-[var(--td-success-color-light)] text-xs font-bold text-[var(--td-brand-color-active)] dark:bg-[rgba(6,176,77,0.15)]"
                  >✓</span
                >
                <span class="leading-[1.4]">{{ $t("platform.fullApiAccess") }}</span>
              </div>
              <div class="text-muted-foreground mb-3 flex items-center text-[13px] last:mb-0">
                <span
                  class="mr-2.5 flex size-5 shrink-0 items-center justify-center rounded-full bg-[var(--td-success-color-light)] text-xs font-bold text-[var(--td-brand-color-active)] dark:bg-[rgba(6,176,77,0.15)]"
                  >✓</span
                >
                <span class="leading-[1.4]">{{ $t("platform.knowledgeBaseManagement") }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, nextTick, onMounted, onBeforeUnmount, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import yuhengMark from "@/assets/img/yuheng-mark.svg";
import { MessagePlugin } from "tdesign-vue-next";
import { ChevronDownIcon, LinkIcon, Loader2Icon } from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useRoleLabel } from "@/composables/useRoleLabel";
import { notifyLoginSuccess } from "@/utils/loginNotify";
import { Swiper, SwiperSlide } from "swiper/vue";
import { Autoplay, EffectFade, Pagination } from "swiper/modules";
import "swiper/css";
import "swiper/css/effect-fade";
import "swiper/css/pagination";
import {
  login,
  register,
  getOIDCAuthorizationURL,
  getOIDCConfig,
  getAuthConfig,
  userInfoFromApi,
  getInvitationByToken,
  registerByInvite,
  type InviteLookup,
} from "@/api/auth";
import { useAuthStore } from "@/stores/auth";
import { useI18n } from "vue-i18n";

// Import screenshot images
import screenshot1 from "@/assets/img/screenshot-1.svg";
import screenshot2 from "@/assets/img/screenshot-2.svg";
import screenshot3 from "@/assets/img/screenshot-3.svg";

const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();
const { t, tm, locale } = useI18n();
const { formatRole, roleIcon } = useRoleLabel();

// Swiper modules
const modules = [Autoplay, EffectFade, Pagination];

// Carousel slides data
const slides = [
  {
    image: screenshot2,
    title: t("platform.carousel.hybridSearchTitle"),
    description: t("platform.carousel.hybridSearchDesc"),
  },
  {
    image: screenshot3,
    title: t("platform.carousel.wikiTitle"),
    description: t("platform.carousel.wikiDesc"),
  },
  {
    image: screenshot1,
    title: t("platform.carousel.smartDocRetrievalTitle"),
    description: t("platform.carousel.smartDocRetrievalDesc"),
  },
];

// The animated knowledge-graph backdrop. Each node carries its own position
// and pulse offset; positions are percentages of the backdrop so the graph
// scales with the window. The last four nodes and lines drop out below 768px,
// where the showcase shrinks to half the viewport and a full graph crowds it.
type NodeIcon =
  | "book"
  | "folder"
  | "layers"
  | "database"
  | "search"
  | "box"
  | "file"
  | "users"
  | "message"
  | "settings"
  | "check"
  | "star";

const KNOWLEDGE_NODES: {
  n: number;
  icon: NodeIcon;
  style: Record<string, string>;
  responsiveHide: boolean;
}[] = [
  { n: 1, icon: "book", style: { top: "15%", left: "20%", animationDelay: "0s" }, responsiveHide: false },
  { n: 2, icon: "folder", style: { top: "25%", left: "35%", animationDelay: "0.5s" }, responsiveHide: false },
  { n: 3, icon: "layers", style: { top: "20%", left: "55%", animationDelay: "1s" }, responsiveHide: false },
  { n: 4, icon: "database", style: { top: "30%", left: "75%", animationDelay: "1.5s" }, responsiveHide: false },
  { n: 5, icon: "search", style: { top: "45%", left: "25%", animationDelay: "2s" }, responsiveHide: false },
  { n: 6, icon: "box", style: { top: "50%", left: "45%", animationDelay: "2.5s" }, responsiveHide: false },
  { n: 7, icon: "file", style: { top: "48%", left: "65%", animationDelay: "3s" }, responsiveHide: false },
  { n: 8, icon: "users", style: { top: "60%", left: "20%", animationDelay: "0.3s" }, responsiveHide: false },
  { n: 9, icon: "message", style: { top: "12%", right: "15%", animationDelay: "1.8s" }, responsiveHide: true },
  { n: 10, icon: "settings", style: { top: "38%", right: "10%", animationDelay: "2.3s" }, responsiveHide: true },
  { n: 11, icon: "check", style: { top: "70%", left: "40%", animationDelay: "0.8s" }, responsiveHide: true },
  { n: 12, icon: "star", style: { top: "65%", left: "80%", animationDelay: "1.3s" }, responsiveHide: true },
];

const KNOWLEDGE_LINES = [
  { n: 1, x1: 20, y1: 15, x2: 35, y2: 25, delay: "0s", responsiveHide: false },
  { n: 2, x1: 35, y1: 25, x2: 55, y2: 20, delay: "0.5s", responsiveHide: false },
  { n: 3, x1: 55, y1: 20, x2: 85, y2: 12, delay: "1s", responsiveHide: false },
  { n: 4, x1: 8, y1: 35, x2: 25, y2: 45, delay: "0.3s", responsiveHide: false },
  { n: 5, x1: 25, y1: 45, x2: 65, y2: 48, delay: "0.8s", responsiveHide: false },
  { n: 6, x1: 20, y1: 60, x2: 60, y2: 75, delay: "1.3s", responsiveHide: false },
  { n: 7, x1: 20, y1: 15, x2: 20, y2: 60, delay: "1.8s", responsiveHide: false },
  { n: 8, x1: 55, y1: 20, x2: 45, y2: 50, delay: "2.3s", responsiveHide: false },
  { n: 9, x1: 65, y1: 48, x2: 90, y2: 38, delay: "0.2s", responsiveHide: true },
  { n: 10, x1: 40, y1: 70, x2: 75, y2: 80, delay: "0.7s", responsiveHide: true },
  { n: 11, x1: 35, y1: 25, x2: 25, y2: 45, delay: "0.9s", responsiveHide: true },
  { n: 12, x1: 75, y1: 30, x2: 65, y2: 48, delay: "1.5s", responsiveHide: true },
];

const languageSwitchRef = ref<HTMLElement | null>(null);

// State management
const loading = ref(false);
const oidcLoading = ref(false);
const isRegisterMode = ref(false);
const showLanguageMenu = ref(false);
const oidcEnabled = ref(false);
const oidcProviderName = ref("");
// registrationEnabled defaults to true so that on first paint the Register
// link is visible; the actual mode is fetched from /auth/config in onMounted.
// In invite_only mode the link/card are hidden.
const registrationEnabled = ref(true);
// isFirstUser is /auth/config's first_user: the deployment has no account yet,
// so the registrant becomes the administrator and creates the default
// workspace. It defaults to false so the workspace-name field never flashes
// for an ordinary registrant before the config arrives.
const isFirstUser = ref(false);

// invite-link state. When the URL carries ?token=xxx we resolve it to
// the originating tenant + role and switch the form into a "register
// via invitation" mode. The token bypasses the normal invite_only
// gate — possessing it IS the authorisation. Submitting the register
// form with this set hits /auth/register-by-invite (auto-login on
// success) instead of /auth/register.
const inviteToken = ref("");
const inviteLookup = ref<InviteLookup | null>(null);
const inviteLookupError = ref("");
const inviteLookupLoading = ref(false);

// Language options
const languageOptions = [
  { value: "zh-CN", label: "简体中文", shortLabel: "中文", flag: "🇨🇳" },
  { value: "en-US", label: "English", shortLabel: "EN", flag: "🇺🇸" },
  { value: "ru-RU", label: "Русский", shortLabel: "RU", flag: "🇷🇺" },
  { value: "ko-KR", label: "한국어", shortLabel: "한국어", flag: "🇰🇷" },
];

const currentLanguage = computed(() => locale.value);
const oidcLoginText = computed(() => {
  if (oidcProviderName.value) {
    return t("auth.oidcLoginWithProvider", { provider: oidcProviderName.value });
  }
  return t("auth.oidcLogin");
});
const currentLangOption = computed(() => languageOptions.find((l) => l.value === currentLanguage.value));

// Login form data
const formData = reactive<{ [key: string]: any }>({
  email: "",
  password: "",
});

// Register form data
const registerData = reactive<{ [key: string]: any }>({
  username: "",
  email: "",
  password: "",
  confirmPassword: "",
  // Bootstrap only (see isFirstUser); empty means the server's default name.
  workspaceName: "",
});

// Field errors, shown under each input. The forms used to be TDesign forms
// driven by rule tables; the checks below are the same rules, run in the same
// order, and the first one that fails is the message shown. Each error is
// kept as its i18n key rather than as translated text, so a message already
// on screen follows a language switch the way TDesign's computed rules did.
type FieldErrors<K extends string> = Record<K, string>;
type LoginField = "email" | "password";
type RegisterField = "username" | "email" | "password" | "confirmPassword";

const loginErrors = reactive<FieldErrors<LoginField>>({ email: "", password: "" });
const registerErrors = reactive<FieldErrors<RegisterField>>({
  username: "",
  email: "",
  password: "",
  confirmPassword: "",
});

// The look TDesign gave these fields: a top label on a 32px line, a large
// (40px) input with the page's own border, hover and focus colours, and the
// error line hung absolutely below the input so it never pushes the form
// down. `md:text-[15px]` is needed because the Input's own `md:text-sm`
// would otherwise win from 768px up.
const FIELD_LABEL_CLASS = "text-foreground block min-h-8 text-sm leading-8 font-normal";
const FIELD_INPUT_CLASS =
  "border-border bg-card h-10 rounded-lg px-3 text-[15px] transition-all duration-200 md:text-[15px] " +
  "hover:border-primary focus-visible:border-primary focus-visible:ring-primary/10 " +
  "dark:bg-background dark:border-white/10 dark:hover:border-primary dark:focus-visible:border-primary " +
  "aria-invalid:ring-0 aria-invalid:focus-visible:ring-3";
const FIELD_ERROR_CLASS = "text-destructive absolute top-full left-0 m-0 max-w-full truncate text-xs leading-5";

// TDesign's `email: true` rule is validator.js's isEmail. This is a close
// equivalent without the dependency: no whitespace, one @, and a domain of
// dot-separated labels ending in a top-level domain of two letters or more.
const EMAIL_PATTERN = /^[^\s@]+@(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z\u00a1-\uffff]{2,}$/i;

// TDesign's `min`/`max` rules measure strings with getCharacterLength, which
// counts every non-ASCII character as two. A Chinese username of one
// character therefore passes `min: 2`, and ten is the most `max: 20` allows.
const characterLength = (value: string): number => {
  let length = 0;
  for (let i = 0; i < value.length; i++) length += value.charCodeAt(i) > 127 ? 2 : 1;
  return length;
};

const emailError = (value: string): string => {
  if (!value) return "auth.emailRequired";
  if (!EMAIL_PATTERN.test(value)) return "auth.emailInvalid";
  return "";
};

const passwordError = (value: string): string => {
  if (!value) return "auth.passwordRequired";
  if (characterLength(value) < 8) return "auth.passwordMinLength";
  if (characterLength(value) > 32) return "auth.passwordMaxLength";
  if (!/[a-zA-Z]/.test(value)) return "auth.passwordMustContainLetter";
  if (!/\d/.test(value)) return "auth.passwordMustContainNumber";
  return "";
};

const usernameError = (value: string): string => {
  if (!value) return "auth.usernameRequired";
  if (characterLength(value) < 2) return "auth.usernameMinLength";
  if (characterLength(value) > 20) return "auth.usernameMaxLength";
  if (!/^[a-zA-Z0-9_\u4e00-\u9fa5]+$/.test(value)) return "auth.usernameInvalid";
  return "";
};

// Only the confirmation's own changes re-check it, as in TDesign: editing the
// password afterwards leaves an existing mismatch message where it is.
const confirmPasswordError = (value: string): string => {
  if (!value) return "auth.confirmPasswordRequired";
  if (value !== registerData.password) return "auth.passwordMismatch";
  return "";
};

const FIELD_CHECKS: Record<LoginField | RegisterField, (value: string) => string> = {
  username: usernameError,
  email: emailError,
  password: passwordError,
  confirmPassword: confirmPasswordError,
};

// TDesign re-validated a field on every change of its value (the rules'
// default `change` trigger), so an error appears and clears as the user types.
const validateField = (errors: Record<string, string>, field: LoginField | RegisterField, value: string | number) => {
  errors[field] = FIELD_CHECKS[field](String(value ?? ""));
};

const validateLogin = (): boolean => {
  loginErrors.email = emailError(formData.email);
  loginErrors.password = passwordError(formData.password);
  return !loginErrors.email && !loginErrors.password;
};

const validateRegister = (): boolean => {
  registerErrors.username = usernameError(registerData.username);
  registerErrors.email = emailError(registerData.email);
  registerErrors.password = passwordError(registerData.password);
  registerErrors.confirmPassword = confirmPasswordError(registerData.confirmPassword);
  return Object.values(registerErrors).every((message) => !message);
};

const clearErrors = (errors: Record<string, string>) => {
  Object.keys(errors).forEach((key) => {
    errors[key] = "";
  });
};

// Toggle login/register mode
const toggleMode = () => {
  isRegisterMode.value = !isRegisterMode.value;

  Object.keys(registerData).forEach((key) => {
    (registerData as any)[key] = "";
  });
  clearErrors(loginErrors);
  clearErrors(registerErrors);
};

// Toggle language menu
const toggleLanguageMenu = () => {
  showLanguageMenu.value = !showLanguageMenu.value;
};

// Select language
const selectLanguage = (lang: string) => {
  locale.value = lang;
  localStorage.setItem("locale", lang);
  showLanguageMenu.value = false;
  MessagePlugin.success(t("language.languageSaved"));
};

// Close language menu when clicking outside
const handleClickOutside = (event: MouseEvent) => {
  if (!languageSwitchRef.value?.contains(event.target as Node)) {
    showLanguageMenu.value = false;
  }
};

// Add click outside listener
onMounted(() => {
  document.addEventListener("click", handleClickOutside);
});

onBeforeUnmount(() => {
  document.removeEventListener("click", handleClickOutside);
});

const persistLoginResponse = async (response: any, skipRedirect = false) => {
  // `active_tenant` is the workspace whose ID is encoded in the JWT: the one
  // the server remembered for the user, else their earliest membership, or
  // absent when they belong to no workspace yet.
  const activeTenant = response.active_tenant;
  if (response.user && response.token) {
    authStore.setUser(userInfoFromApi(response.user));
    authStore.setToken(response.token);
    if (response.refresh_token) {
      authStore.setRefreshToken(response.refresh_token);
    }
    if (activeTenant) {
      authStore.setTenant({
        id: String(activeTenant.id) || "",
        name: activeTenant.name || "",
        owner_id: response.user.id || "",
        created_at: activeTenant.created_at || new Date().toISOString(),
        updated_at: activeTenant.updated_at || new Date().toISOString(),
      });
    } else {
      authStore.setTenant(null);
    }
    if (Array.isArray(response.memberships)) {
      authStore.setMemberships(response.memberships);
    }
    // The token is already scoped to the workspace the server chose, so
    // any X-Tenant-ID override left in localStorage by a previous session
    // (possibly another account) is stale and would fight the token.
    authStore.setSelectedTenant(null, null);
  }

  // Pull runtime capabilities (including whether ordinary users may create
  // workspaces) before entering the main UI so create actions never flash
  // briefly when the deployment is invitation-only.
  await authStore.refreshFromAuthMe();
  await nextTick();
  if (skipRedirect) return;
  router.replace(authStore.hasValidTenant ? "/platform/knowledge-bases" : "/onboarding/workspace");
};

const getBackendOIDCRedirectURI = () => `${window.location.origin}/api/v1/auth/oidc/callback`;

const loadOIDCConfig = async () => {
  try {
    const response = await getOIDCConfig();
    oidcEnabled.value = !!response.success && !!response.enabled;
    oidcProviderName.value = response.provider_display_name || "";
  } catch {
    oidcEnabled.value = false;
    oidcProviderName.value = "";
  }
};

// loadAuthConfig fetches /auth/config and caches whether self-service
// registration is allowed. Failures fall back to "enabled" so a transient
// network glitch doesn't lock new users out of an open deployment.
const loadAuthConfig = async () => {
  try {
    const response = await getAuthConfig();
    registrationEnabled.value = response.registration_mode !== "invite_only";
    isFirstUser.value = response.first_user === true;
  } catch {
    registrationEnabled.value = true;
    isFirstUser.value = false;
  }
};

const handleOIDCLogin = async () => {
  try {
    oidcLoading.value = true;
    const response = await getOIDCAuthorizationURL(getBackendOIDCRedirectURI());
    const authorizationURL = response.authorization_url;

    if (!response.success || !authorizationURL) {
      MessagePlugin.error(response.message || t("auth.oidcLoginFailed"));
      return;
    }

    // 跳转 IdP 会丢失 URL 中的 token，暂存到 sessionStorage，回调后由 App.vue 兑换。
    if (inviteToken.value) {
      sessionStorage.setItem("yuheng_pending_invite_token", inviteToken.value);
    }
    window.location.href = authorizationURL;
  } catch (error: any) {
    console.error("OIDC 登录跳转失败:", error);
    MessagePlugin.error(error.message || t("auth.oidcLoginFailed"));
  } finally {
    oidcLoading.value = false;
  }
};

// 用 token 加入空间并进入应用。会话此时已有效，故即便 token 失效也照常进入（避免困在登录页）。
const acceptAndEnter = async (token: string) => {
  loading.value = true;
  try {
    const result = await authStore.acceptInvitationByTokenAndRefresh(token);
    if (result.ok) {
      MessagePlugin.success(t("inviteRegister.joined"));
    } else {
      MessagePlugin.warning(t("inviteRegister.invalidBody"));
    }
  } catch {
    MessagePlugin.warning(t("inviteRegister.invalidBody"));
  } finally {
    loading.value = false;
    await nextTick();
    router.replace("/platform/knowledge-bases");
  }
};

// Handle login
const handleLogin = async () => {
  try {
    if (!validateLogin()) return;

    loading.value = true;

    const response = await login({
      email: formData.email,
      password: formData.password,
    });

    if (response.success) {
      if (inviteToken.value) {
        // 从邀请链接登录：持久化会话后兑换 token 并进入对应空间。
        await persistLoginResponse(response, true);
        await acceptAndEnter(inviteToken.value);
        return;
      }
      await persistLoginResponse(response);
      notifyLoginSuccess(response, t, tm, formatRole, roleIcon);
    } else {
      MessagePlugin.error(response.message || t("auth.loginError"));
    }
  } catch (error: any) {
    console.error("登录错误:", error);
    MessagePlugin.error(error.message || t("auth.loginErrorRetry"));
  } finally {
    loading.value = false;
  }
};

// Handle registration. Dispatches based on whether the user arrived
// with a share-link token: with token -> register-by-invite (auto-
// login on success); without -> the normal self-service register
// (drops back to the login form for the user to sign in).
const handleRegister = async () => {
  try {
    if (!validateRegister()) return;

    loading.value = true;

    if (inviteToken.value) {
      const response = await registerByInvite({
        token: inviteToken.value,
        username: registerData.username,
        email: registerData.email,
        password: registerData.password,
      });
      if (!response.success) {
        MessagePlugin.error(response.message || t("auth.registerFailed"));
        return;
      }
      MessagePlugin.success(t("auth.registerSuccess"));
      // register-by-invite returns the same shape as login (token +
      // active_tenant + memberships), so reuse the login persistence
      // path — same store writes, same redirect target.
      await persistLoginResponse(response);
      return;
    }

    const workspaceName = String(registerData.workspaceName || "").trim();
    const response = await register({
      username: registerData.username,
      email: registerData.email,
      password: registerData.password,
      // Only the bootstrap registrant names a workspace; omit the field
      // otherwise so the request says exactly what the form showed.
      ...(isFirstUser.value && workspaceName ? { workspace_name: workspaceName } : {}),
    });

    if (response.success) {
      MessagePlugin.success(t("auth.registerSuccess"));

      // Switch to login mode and fill in email
      isRegisterMode.value = false;
      formData.email = registerData.email;

      // Clear register form
      Object.keys(registerData).forEach((key) => {
        (registerData as any)[key] = "";
      });
    } else {
      MessagePlugin.error(response.message || t("auth.registerFailed"));
    }
  } catch (error: any) {
    console.error("注册错误:", error);
    MessagePlugin.error(error.message || t("auth.registerError"));
  } finally {
    loading.value = false;
  }
};

// Check if already logged in; logged-in users go straight to the workspace.
onMounted(async () => {
  // Share-link landing: ?token=xxx switches the form into invite-
  // register mode before any other auto-flow (logged-in redirect /
  // OIDC) gets a chance to redirect. Resolution failure
  // surfaces inline; the user can still log in normally if they
  // already have an account. We check this BEFORE the isLoggedIn
  // redirect so an existing session doesn't bounce the user to
  // /platform (and possibly back to /login if the session is stale),
  // dropping the invite token along the way.
  const tokenFromQuery = String(route.query.token || "").trim();
  if (tokenFromQuery) {
    inviteToken.value = tokenFromQuery;
    inviteLookupLoading.value = true;
    // 1. 先校验 token：无效/过期则停在登录页报错，不进注册模式。
    try {
      const resp = await getInvitationByToken(tokenFromQuery);
      if (resp.success && resp.data) {
        inviteLookup.value = resp.data;
      } else {
        inviteLookupError.value = resp.message || t("inviteRegister.invalidBody");
        loadOIDCConfig();
        loadAuthConfig();
        return;
      }
    } catch {
      inviteLookupError.value = t("inviteRegister.invalidBody");
      loadOIDCConfig();
      loadAuthConfig();
      return;
    } finally {
      inviteLookupLoading.value = false;
    }

    // 2. 已登录则直接兑换 token 进入空间（两种模式通用）。
    if (authStore.isLoggedIn && (await authStore.refreshFromAuthMe())) {
      await acceptAndEnter(tokenFromQuery);
      return;
    }

    // 3. 未登录：按注册模式决定界面。invite_only 停在登录页、登录后再兑换；self_serve 保持注册流程。
    const cfg = await getAuthConfig();
    const inviteOnly = cfg.registration_mode === "invite_only";
    registrationEnabled.value = !inviteOnly;
    isRegisterMode.value = !inviteOnly;
    loadOIDCConfig();
    return;
  }

  if (authStore.isLoggedIn) {
    router.replace("/platform/knowledge-bases");
    return;
  }

  loadOIDCConfig();
  loadAuthConfig();
});
</script>

<style scoped>
/*
 * Kept as CSS: the carousel's pagination bullets are Swiper's own markup,
 * which this template cannot put classes on.
 */
/*
 * Swiper's own stylesheet is unlayered and sets `.swiper { padding: 0 }`,
 * which beats any (layered) utility; this scoped rule is more specific.
 */
.screenshot-swiper {
  padding-bottom: 40px;
}

.screenshot-swiper :deep(.swiper-wrapper) {
  transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
}

.screenshot-swiper :deep(.swiper-pagination) {
  bottom: 15px !important;
  z-index: 10;
}

.screenshot-swiper :deep(.swiper-pagination-bullet) {
  width: 10px;
  height: 10px;
  margin: 0 6px !important;
  background: rgba(255, 255, 255, 0.5);
  opacity: 1;
  transition: all 0.3s ease;
}

.screenshot-swiper :deep(.swiper-pagination-bullet-active) {
  width: 28px;
  border-radius: 5px;
  background: var(--td-bg-color-container);
}

/*
 * Not `:global(html[…]) .screenshot-swiper`: Vue replaces a whole selector
 * that contains :global() with the :global argument alone, which painted the
 * entire <html> in the bullet colour in dark mode. A plain ancestor selector
 * keeps the scope attribute on `.screenshot-swiper`.
 */
html[theme-mode="dark"] .screenshot-swiper :deep(.swiper-pagination-bullet-active) {
  background: rgba(255, 255, 255, 0.9);
}
</style>

<style>
/*
 * The backdrop's keyframes, unscoped on purpose: a scoped block renames its
 * keyframes with a hash and only rewrites `animation` declarations in that
 * same block, so the `animate-[…]` utilities in the template would name a
 * keyframe that does not exist and the backdrop would never move. The
 * `login-` prefix keeps the global names from colliding.
 */
@keyframes login-node-pulse {
  0%,
  100% {
    transform: scale(1);
    opacity: 0.65;
  }
  50% {
    transform: scale(1.08);
    opacity: 0.9;
  }
}

@keyframes login-line-flow {
  from {
    stroke-dashoffset: 0;
  }
  to {
    stroke-dashoffset: 18;
  }
}
</style>
