<template>
  <div class="w-full">
    <div class="mb-8">
      <h2 class="text-foreground mt-0 mb-2 text-xl font-semibold">{{ $t("general.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-normal">{{ $t("general.description") }}</p>
    </div>

    <div class="flex flex-col">
      <!-- 语言选择 -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ $t("language.language") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ $t("language.languageDescription") }}</p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <Select v-model="localLanguage" @update:model-value="handleLanguageChange">
            <SelectTrigger class="w-[280px]">
              <SelectValue :placeholder="$t('language.selectLanguage')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="zh-CN">{{ $t("language.zhCN") }}</SelectItem>
              <SelectItem value="en-US">{{ $t("language.enUS") }}</SelectItem>
              <SelectItem value="ru-RU">{{ $t("language.ruRU") }}</SelectItem>
              <SelectItem value="ko-KR">{{ $t("language.koKR") }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <!-- 主题设置 -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ $t("theme.theme") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ $t("theme.themeDescription") }}</p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <Select v-model="localTheme" @update:model-value="(v) => handleThemeChange(v as ThemeMode)">
            <SelectTrigger class="w-[280px]">
              <SelectValue :placeholder="$t('theme.selectTheme')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="light">{{ $t("theme.light") }}</SelectItem>
              <SelectItem value="dark">{{ $t("theme.dark") }}</SelectItem>
              <SelectItem value="system">{{ $t("theme.system") }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <!-- 界面字体 -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ $t("font.uiFont") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ $t("font.uiFontDescription") }}</p>
        </div>
        <div class="flex min-w-[280px] shrink-0 flex-col items-end justify-end gap-2">
          <Select v-model="localSansFont" @update:model-value="(v) => handleSansFontChange(v as FontKey)">
            <SelectTrigger class="w-[280px]">
              <SelectValue :placeholder="$t('font.selectFont')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="opt in sansFontOptions" :key="opt.value" :value="opt.value">
                <span :style="{ fontFamily: opt.preview }">{{ opt.label }}</span>
              </SelectItem>
            </SelectContent>
          </Select>
          <div
            class="border-border bg-card text-foreground box-border w-[280px] rounded-md border px-3 py-2 text-left text-sm leading-snug"
            :style="{ fontFamily: currentSansStack }"
          >
            {{ $t("font.sansPreview") }}
          </div>
        </div>
      </div>

      <!-- 代码字体 -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ $t("font.monoFont") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ $t("font.monoFontDescription") }}</p>
        </div>
        <div class="flex min-w-[280px] shrink-0 flex-col items-end justify-end gap-2">
          <Select v-model="localMonoFont" @update:model-value="(v) => handleMonoFontChange(v as MonoFontKey)">
            <SelectTrigger class="w-[280px]">
              <SelectValue :placeholder="$t('font.selectFont')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="opt in monoFontOptions" :key="opt.value" :value="opt.value">
                <span :style="{ fontFamily: opt.preview }">{{ opt.label }}</span>
              </SelectItem>
            </SelectContent>
          </Select>
          <!-- The mono preview never wraps: long monospace samples are cut with an ellipsis. -->
          <div
            class="border-border bg-card text-foreground box-border w-[280px] overflow-hidden rounded-md border px-3 py-2 text-left text-sm leading-snug text-ellipsis whitespace-nowrap"
            :style="{ fontFamily: currentMonoStack }"
          >
            {{ $t("font.monoPreview") }}
          </div>
        </div>
      </div>

      <!-- 字体大小 -->
      <div class="border-border flex items-start justify-between py-5 [&:not(:last-child)]:border-b">
        <div class="max-w-[65%] min-w-0 flex-1 pr-6">
          <label class="text-foreground mb-1 block text-[15px] font-medium">{{ $t("font.fontSize") }}</label>
          <p class="text-muted-foreground m-0 text-[13px] leading-normal">{{ $t("font.fontSizeDescription") }}</p>
        </div>
        <div class="flex min-w-[280px] shrink-0 items-center justify-end">
          <!-- A segmented control stands in for the old radio-button group:
               one button per size, the active one keeping the brand colour. -->
          <div class="border-border inline-flex rounded-md border">
            <Button
              v-for="size in fontSizeOptions"
              :key="size.value"
              type="button"
              variant="ghost"
              size="sm"
              :aria-pressed="localFontSize === size.value"
              class="not-first:border-border rounded-none border-0 not-first:border-l"
              :class="localFontSize === size.value ? 'text-primary bg-secondary' : 'text-muted-foreground'"
              @click="selectFontSize(size.value)"
            >
              {{ size.label }}
            </Button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch } from "vue";
import { MessagePlugin } from "tdesign-vue-next";
import { useI18n } from "vue-i18n";
import { useTheme, type ThemeMode } from "@/composables/useTheme";
import {
  useFont,
  SANS_STACKS,
  MONO_STACKS,
  visibleSansKeys,
  visibleMonoKeys,
  type FontKey,
  type MonoFontKey,
  type FontSizeKey,
} from "@/composables/useFont";

import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

const { t, locale } = useI18n();
const { currentTheme, setTheme } = useTheme();
const { currentSans, currentMono, currentSize, setSansFont, setMonoFont, setFontSize } = useFont();

// 本地状态
const localLanguage = ref("zh-CN");
const localTheme = ref<ThemeMode>(currentTheme.value);
const localSansFont = ref<FontKey>(currentSans.value);
const localMonoFont = ref<MonoFontKey>(currentMono.value);
const localFontSize = ref<FontSizeKey>(currentSize.value);

// Keep the form in sync if preferences change externally (e.g. on user switch).
watch(currentTheme, (val) => {
  localTheme.value = val;
});
watch(currentSans, (val) => {
  localSansFont.value = val;
});
watch(currentMono, (val) => {
  localMonoFont.value = val;
});
watch(currentSize, (val) => {
  localFontSize.value = val;
});

const sansFontOptions = computed<{ value: FontKey; label: string; preview: string }[]>(() =>
  visibleSansKeys().map((key) => ({
    value: key,
    label: t(`font.sans.${key}`),
    preview: SANS_STACKS[key],
  })),
);

const monoFontOptions = computed<{ value: MonoFontKey; label: string; preview: string }[]>(() =>
  visibleMonoKeys().map((key) => ({
    value: key,
    label: t(`font.mono.${key}`),
    preview: MONO_STACKS[key],
  })),
);

const fontSizeOptions = computed<{ value: FontSizeKey; label: string }[]>(() => [
  { value: "small", label: t("font.size.small") },
  { value: "normal", label: t("font.size.normal") },
  { value: "large", label: t("font.size.large") },
]);

// Live preview stacks, driven by the local form refs so the preview row
// updates immediately on selection — even before handleSansFontChange
// commits the choice to the global store and writes the CSS variable.
const currentSansStack = computed(() => SANS_STACKS[localSansFont.value] ?? SANS_STACKS.system);
const currentMonoStack = computed(() => MONO_STACKS[localMonoFont.value] ?? MONO_STACKS.system);

// 初始化加载
onMounted(() => {
  // 从 localStorage 加载语言设置
  const savedLocale = localStorage.getItem("locale");
  if (savedLocale) {
    localLanguage.value = savedLocale;
    locale.value = savedLocale;
  } else {
    localLanguage.value = locale.value;
  }
});

// 处理语言变化
const handleLanguageChange = () => {
  locale.value = localLanguage.value;
  localStorage.setItem("locale", localLanguage.value);
  MessagePlugin.success(t("language.languageSaved"));
};

// 处理主题变化
const handleThemeChange = (val: ThemeMode) => {
  if (!setTheme(val)) {
    // Setter rejected the value (validation guard); roll the form back to
    // the canonical state so the UI doesn't drift.
    localTheme.value = currentTheme.value;
    return;
  }
  MessagePlugin.success(t("common.success"));
};

// 处理字体变化
const handleSansFontChange = (val: FontKey) => {
  if (!setSansFont(val)) {
    localSansFont.value = currentSans.value;
    return;
  }
  MessagePlugin.success(t("common.success"));
};

const handleMonoFontChange = (val: MonoFontKey) => {
  if (!setMonoFont(val)) {
    localMonoFont.value = currentMono.value;
    return;
  }
  MessagePlugin.success(t("common.success"));
};

// The old radio group fired its change event only when the value actually
// changed; clicking the size that is already active must stay a no-op here
// too, rather than re-applying it and showing a second success toast.
const selectFontSize = (val: FontSizeKey) => {
  if (localFontSize.value === val) return;
  localFontSize.value = val;
  handleFontSizeChange(val);
};

const handleFontSizeChange = (val: FontSizeKey) => {
  if (!setFontSize(val)) {
    localFontSize.value = currentSize.value;
    return;
  }
  MessagePlugin.success(t("common.success"));
};
</script>
