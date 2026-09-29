<template>
  <div class="w-full">
    <!--
      Sticks to the top of the scrollable section so the title stays visible
      over a long form. The negative top and margins cancel the host
      content-wrapper's 24px 32px padding, so the band spans the full width.
    -->
    <div
      v-if="!embedded"
      class="border-border bg-card sticky top-[-24px] z-5 -mx-8 -mt-6 mb-4 flex items-start justify-between gap-4 border-b px-8 pt-6 pb-3"
    >
      <div class="min-w-0 flex-1">
        <h2 class="text-foreground mt-0 mb-1.5 text-xl font-semibold">{{ $t("knowledgeEditor.chunking.title") }}</h2>
        <p class="text-muted-foreground m-0 text-sm leading-normal">{{ $t("knowledgeEditor.chunking.description") }}</p>
      </div>
    </div>

    <div class="flex flex-col">
      <!-- Strategy -->
      <div :class="rowClass">
        <div :class="infoClass">
          <label :class="labelClass">{{ $t("knowledgeEditor.chunking.strategyLabel") }}</label>
          <p :class="descClass">{{ $t("knowledgeEditor.chunking.strategyDescription") }}</p>
        </div>
        <!-- The picker sits above the test trigger, both right-aligned in the right column. -->
        <div
          class="flex flex-col"
          :class="embedded ? 'w-full items-stretch' : 'max-w-[55%] shrink-0 basis-[55%] items-end gap-1.5'"
        >
          <div class="group/strategy relative" :class="embedded ? 'w-full' : 'w-[280px]'">
            <Select
              :model-value="localStrategy || undefined"
              @update:model-value="
                (val) => {
                  localStrategy = String(val ?? '');
                  handleStrategyChange();
                }
              "
            >
              <SelectTrigger class="w-full">
                <SelectValue :placeholder="$t('knowledgeEditor.chunking.strategyPlaceholder')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="opt in strategyOptions" :key="opt.value" :value="opt.value">
                  {{ opt.label }}
                </SelectItem>
              </SelectContent>
            </Select>
            <!-- t-select's `clearable`: an empty strategy means "server default". -->
            <button
              v-if="localStrategy"
              type="button"
              data-slot="icon-button"
              class="bg-card text-placeholder hover:text-muted-foreground absolute top-1/2 right-2 hidden -translate-y-1/2 group-hover/strategy:inline-flex"
              :aria-label="$t('common.clear')"
              @click="
                localStrategy = '';
                handleStrategyChange();
              "
            >
              <CircleXIcon class="size-4" />
            </button>
          </div>
          <!-- Test trigger sits right next to the strategy picker so users
               discover it exactly when they're deciding which strategy to
               use on their content. -->
          <KBChunkingDebug v-if="!embedded" :config="debugConfig" />
        </div>
      </div>

      <!-- Strategy explanation panel -->
      <div
        v-if="currentStrategyInfo"
        class="bg-accent text-muted-foreground border-l-primary rounded-r border-l-[3px] leading-normal break-words"
        :class="embedded ? '-mt-1 mb-2.5 px-3 py-2 text-xs' : 'mb-4 px-3.5 py-2.5 text-[13px]'"
      >
        <p class="m-0">
          <strong class="text-foreground">{{ currentStrategyInfo.label }}:</strong>
          {{ currentStrategyInfo.tooltip }}
        </p>
      </div>

      <!-- Chunk Size -->
      <div :class="rowClass">
        <div :class="infoClass">
          <label :class="labelClass">{{ $t("knowledgeEditor.chunking.sizeLabel") }}</label>
          <p :class="descClass">{{ $t("knowledgeEditor.chunking.sizeDescription") }}</p>
        </div>
        <div :class="controlClass">
          <ChunkSlider
            v-model="localChunkSize"
            :min="100"
            :max="4000"
            :step="50"
            :marks="embedded ? undefined : chunkSizeMarks"
            :embedded="embedded"
            :unit="$t('knowledgeEditor.chunking.characters')"
            @update:model-value="handleChunkSizeChange"
          />
        </div>
      </div>

      <!-- Chunk Overlap -->
      <div :class="rowClass">
        <div :class="infoClass">
          <label :class="labelClass">{{ $t("knowledgeEditor.chunking.overlapLabel") }}</label>
          <p :class="descClass">{{ $t("knowledgeEditor.chunking.overlapDescription") }}</p>
          <p v-if="overlapTooHigh" class="text-warning mt-1 mb-0 text-xs leading-[1.4]">
            {{ $t("knowledgeEditor.chunking.overlapWarning") }}
          </p>
        </div>
        <div :class="controlClass">
          <ChunkSlider
            v-model="localChunkOverlap"
            :min="0"
            :max="500"
            :step="20"
            :marks="embedded ? undefined : chunkOverlapMarks"
            :embedded="embedded"
            :unit="$t('knowledgeEditor.chunking.characters')"
            @update:model-value="handleChunkOverlapChange"
          />
        </div>
      </div>

      <!-- Separators -->
      <div
        class="border-border flex justify-between [&:not(:last-child)]:border-b"
        :class="embedded ? 'flex-col items-stretch gap-2.5 pt-3.5 pb-[18px]' : 'items-start py-4'"
      >
        <div :class="infoClass">
          <label :class="labelClass">{{ $t("knowledgeEditor.chunking.separatorsLabel") }}</label>
          <p :class="descClass">{{ $t("knowledgeEditor.chunking.separatorsDescription") }}</p>
        </div>
        <div :class="controlClass">
          <!--
            t-select multiple + creatable: a Popover rather than a DropdownMenu,
            because a menu's typeahead would swallow the keys typed into the
            "add your own" input, and a menu closes on every toggle.
          -->
          <Popover>
            <PopoverTrigger as-child>
              <Button variant="outline" class="justify-between font-normal" :class="embedded ? 'w-full' : 'w-[280px]'">
                <span class="truncate" :class="localSeparators.length ? '' : 'text-placeholder'">
                  {{
                    localSeparators.length
                      ? localSeparators.map((s) => separatorLabel(s)).join(", ")
                      : $t("knowledgeEditor.chunking.separatorsPlaceholder")
                  }}
                </span>
                <ChevronDownIcon class="size-4 opacity-50" />
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" class="w-(--reka-popover-trigger-width) min-w-[240px] gap-1 p-1">
              <div class="flex max-h-[240px] flex-col gap-px overflow-y-auto">
                <label
                  v-for="opt in separatorChoices"
                  :key="opt.value"
                  class="hover:bg-accent flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm"
                >
                  <Checkbox
                    :model-value="localSeparators.includes(opt.value)"
                    @update:model-value="(checked) => toggleSeparator(opt.value, checked === true)"
                  />
                  <span class="truncate">{{ opt.label }}</span>
                </label>
              </div>
              <div class="border-border flex items-center gap-1.5 border-t p-1.5">
                <Input
                  v-model="customSeparator"
                  class="h-7 flex-1 text-xs md:text-xs"
                  :placeholder="$t('knowledgeEditor.chunking.separatorsPlaceholder')"
                  @keydown.enter.prevent="addCustomSeparator"
                />
                <Button
                  size="icon-xs"
                  variant="ghost"
                  :disabled="!customSeparator"
                  :aria-label="$t('common.add')"
                  @click="addCustomSeparator"
                >
                  <PlusIcon />
                </Button>
              </div>
            </PopoverContent>
          </Popover>
        </div>
      </div>

      <!-- Parent-Child Chunking: embedded, a switch row stays a row. -->
      <div
        class="border-border flex justify-between [&:not(:last-child)]:border-b"
        :class="embedded ? 'items-center gap-4 py-3.5' : 'items-start py-4'"
      >
        <div :class="embedded ? 'min-w-0 flex-1' : 'max-w-[40%] shrink-0 basis-2/5 pr-6'">
          <label :class="labelClass">{{ $t("knowledgeEditor.chunking.parentChildLabel") }}</label>
          <p :class="descClass">{{ $t("knowledgeEditor.chunking.parentChildDescription") }}</p>
        </div>
        <div
          :class="
            embedded
              ? 'flex w-auto flex-none justify-end'
              : 'flex max-w-[55%] shrink-0 basis-[55%] items-center justify-end'
          "
        >
          <Switch
            :model-value="localEnableParentChild"
            @update:model-value="
              (val: boolean) => {
                localEnableParentChild = val;
                handleParentChildChange();
              }
            "
          />
        </div>
      </div>

      <!-- Parent Chunk Size -->
      <div v-if="localEnableParentChild" :class="rowClass">
        <div :class="infoClass">
          <label :class="labelClass">{{ $t("knowledgeEditor.chunking.parentChunkSizeLabel") }}</label>
          <p :class="descClass">{{ $t("knowledgeEditor.chunking.parentChunkSizeDescription") }}</p>
        </div>
        <div :class="controlClass">
          <ChunkSlider
            v-model="localParentChunkSize"
            :min="512"
            :max="8192"
            :step="64"
            :marks="embedded ? undefined : parentChunkSizeMarks"
            :embedded="embedded"
            :unit="$t('knowledgeEditor.chunking.characters')"
            @update:model-value="handleParentChunkSizeChange"
          />
        </div>
      </div>

      <!-- Child Chunk Size -->
      <div v-if="localEnableParentChild" :class="rowClass">
        <div :class="infoClass">
          <label :class="labelClass">{{ $t("knowledgeEditor.chunking.childChunkSizeLabel") }}</label>
          <p :class="descClass">{{ $t("knowledgeEditor.chunking.childChunkSizeDescription") }}</p>
        </div>
        <div :class="controlClass">
          <ChunkSlider
            v-model="localChildChunkSize"
            :min="64"
            :max="2048"
            :step="32"
            :marks="embedded ? undefined : childChunkSizeMarks"
            :embedded="embedded"
            :unit="$t('knowledgeEditor.chunking.characters')"
            @update:model-value="handleChildChunkSizeChange"
          />
        </div>
      </div>

      <!-- Advanced section toggle -->
      <button
        type="button"
        data-slot="advanced-toggle"
        class="text-muted-foreground hover:text-foreground focus-visible:outline-ring inline-flex items-center gap-1.5 self-start pb-2 text-sm font-medium select-none focus-visible:rounded focus-visible:outline-2 focus-visible:outline-offset-2"
        :class="embedded ? 'pt-2.5' : 'pt-4'"
        @click="advancedOpen = !advancedOpen"
      >
        <ChevronRightIcon class="size-4 transition-transform duration-150" :class="advancedOpen ? 'rotate-90' : ''" />
        <span>{{ $t("knowledgeEditor.chunking.advancedLabel") }}</span>
      </button>

      <div v-if="advancedOpen" class="mt-1">
        <!-- Token Limit -->
        <div :class="[rowClass, advancedDisabled ? 'opacity-50' : '']">
          <div :class="infoClass">
            <label :class="labelClass">{{ $t("knowledgeEditor.chunking.tokenLimitLabel") }}</label>
            <p :class="descClass">{{ $t("knowledgeEditor.chunking.tokenLimitDescription") }}</p>
          </div>
          <div :class="controlClass">
            <Input
              v-model.number="localTokenLimit"
              type="number"
              :min="0"
              :max="8192"
              :step="64"
              :disabled="advancedDisabled"
              :class="embedded ? 'w-full' : 'w-[200px]'"
              @change="handleTokenLimitChange"
            />
          </div>
        </div>

        <!-- Languages -->
        <div :class="[rowClass, advancedDisabled ? 'opacity-50' : '']">
          <div :class="infoClass">
            <label :class="labelClass">{{ $t("knowledgeEditor.chunking.languagesLabel") }}</label>
            <p :class="descClass">{{ $t("knowledgeEditor.chunking.languagesDescription") }}</p>
          </div>
          <div :class="controlClass">
            <!-- t-select multiple: a checkbox list that stays open while toggling. -->
            <Popover>
              <PopoverTrigger as-child>
                <Button
                  variant="outline"
                  class="justify-between font-normal"
                  :class="embedded ? 'w-full' : 'w-[280px]'"
                  :disabled="advancedDisabled"
                >
                  <span class="truncate" :class="localLanguages.length ? '' : 'text-placeholder'">
                    {{
                      localLanguages.length
                        ? localLanguages.map((l) => languageLabel(l)).join(", ")
                        : $t("knowledgeEditor.chunking.languagesPlaceholder")
                    }}
                  </span>
                  <ChevronDownIcon class="size-4 opacity-50" />
                </Button>
              </PopoverTrigger>
              <PopoverContent align="end" class="w-(--reka-popover-trigger-width) min-w-[200px] gap-px p-1">
                <label
                  v-for="opt in languageOptions"
                  :key="opt.value"
                  class="hover:bg-accent flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm"
                >
                  <Checkbox
                    :model-value="localLanguages.includes(opt.value)"
                    @update:model-value="(checked) => toggleLanguage(opt.value, checked === true)"
                  />
                  <span>{{ opt.label }}</span>
                </label>
              </PopoverContent>
            </Popover>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed } from "vue";
import { useI18n } from "vue-i18n";
import { ChevronDownIcon, ChevronRightIcon, CircleXIcon, PlusIcon } from "@lucide/vue";
import ChunkSlider from "./ChunkSlider.vue";
import KBChunkingDebug from "./KBChunkingDebug.vue";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";

interface ParserEngineRule {
  file_types: string[];
  engine: string;
  xlsx_first_row_as_header?: boolean;
}

// Slider ranges defined in this file (min/max props on the sliders) mirror
// the validated bounds in the backend splitter:
//   ChunkSize:      100–4000  (default 512). 100 = too fragmented to be
//                   useful; 4000 = approaches the 7500-char absoluteMaxSize
//                   that the splitter hard-caps to anyway.
//   ChunkOverlap:   0–500     (default 80). Backend caps to ChunkSize/2
//                   when set higher than that.
//   ParentChunkSize: 512–8192 (default 4096 ≈ 1000 EN tokens).
//   ChildChunkSize:  64–2048  (default 384 ≈ 80 EN tokens, sweet spot for
//                   sentence-transformer / BGE embedders).
//   TokenLimit:      0–8192   (default 0 = off, char-based budget only).
//                   Set to 200 for MiniLM (256-tok limit), 400 for BGE/
//                   Cohere (512-tok), leave at 0 for OpenAI/Voyage/Jina-v3.
interface ChunkingConfig {
  chunkSize: number;
  chunkOverlap: number;
  separators: string[];
  parserEngineRules?: ParserEngineRule[];
  enableParentChild: boolean;
  parentChunkSize: number;
  childChunkSize: number;
  // Adaptive chunking strategy. Empty string = legacy / not set.
  strategy?: string;
  // Cap chunk size in approx tokens. 0 = char-based budget only.
  tokenLimit?: number;
  // Language hints for heuristic patterns (de/en/zh).
  languages?: string[];
}

interface Props {
  config: ChunkingConfig;
  embedded?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  embedded: false,
});

const emit = defineEmits<{
  "update:config": [value: ChunkingConfig];
}>();

const { t } = useI18n();

const localChunkSize = ref(props.config.chunkSize);
const localChunkOverlap = ref(props.config.chunkOverlap);
const localSeparators = ref([...props.config.separators]);
const localEnableParentChild = ref(props.config.enableParentChild ?? false);
const localParentChunkSize = ref(props.config.parentChunkSize || 4096);
const localChildChunkSize = ref(props.config.childChunkSize || 384);
const localStrategy = ref(props.config.strategy ?? "");
const localTokenLimit = ref(props.config.tokenLimit ?? 0);
const localLanguages = ref<string[]>([...(props.config.languages ?? [])]);
const advancedOpen = ref(false);
const customSeparator = ref("");

// The row layout, shared by every row but the switch row and the separators
// row. Embedded (the upload dialog), rows stack with the control full width
// and slightly smaller type.
const rowClass = computed(() =>
  props.embedded
    ? "border-border flex flex-col items-stretch justify-between gap-2.5 py-3.5 [&:not(:last-child)]:border-b"
    : "border-border flex items-start justify-between py-4 [&:not(:last-child)]:border-b",
);
const infoClass = computed(() => (props.embedded ? "max-w-none" : "max-w-[40%] shrink-0 basis-2/5 pr-6"));
const labelClass = computed(
  () => `text-foreground mb-1 block font-medium ${props.embedded ? "text-sm" : "text-[15px]"}`,
);
const descClass = computed(
  () => `text-muted-foreground m-0 leading-normal ${props.embedded ? "text-xs" : "text-[13px]"}`,
);
const controlClass = computed(() =>
  props.embedded
    ? "flex w-full max-w-none items-stretch justify-start"
    : "flex max-w-[55%] shrink-0 basis-[55%] items-center justify-end",
);

// Tick labels under the sliders in the full layout (t-slider's `marks`).
const chunkSizeMarks = [100, 1000, 2000, 4000];
const chunkOverlapMarks = [0, 250, 500];
const parentChunkSizeMarks = [512, 2048, 4096, 8192];
const childChunkSizeMarks = [64, 384, 1024, 2048];

const strategyOptions = computed(() => [
  {
    label: t("knowledgeEditor.chunking.strategies.auto.label"),
    value: "auto",
    tooltip: t("knowledgeEditor.chunking.strategies.auto.tooltip"),
  },
  {
    label: t("knowledgeEditor.chunking.strategies.heading.label"),
    value: "heading",
    tooltip: t("knowledgeEditor.chunking.strategies.heading.tooltip"),
  },
  {
    label: t("knowledgeEditor.chunking.strategies.heuristic.label"),
    value: "heuristic",
    tooltip: t("knowledgeEditor.chunking.strategies.heuristic.tooltip"),
  },
  {
    label: t("knowledgeEditor.chunking.strategies.legacy.label"),
    value: "legacy",
    tooltip: t("knowledgeEditor.chunking.strategies.legacy.tooltip"),
  },
]);

const currentStrategyInfo = computed(() => {
  if (!localStrategy.value) {
    return null;
  }
  return strategyOptions.value.find((o) => o.value === localStrategy.value) ?? null;
});

const advancedDisabled = computed(() => localStrategy.value === "legacy");

const overlapTooHigh = computed(
  () => localChunkOverlap.value > 0 && localChunkOverlap.value >= localChunkSize.value / 2,
);

// Live config snapshot for the debug panel — uses current local form values
// so the panel reflects edits immediately without waiting for save.
const debugConfig = computed(() => ({
  chunkSize: localChunkSize.value,
  chunkOverlap: localChunkOverlap.value,
  separators: localSeparators.value,
  enableParentChild: localEnableParentChild.value,
  parentChunkSize: localParentChunkSize.value,
  childChunkSize: localChildChunkSize.value,
  strategy: localStrategy.value,
  tokenLimit: localTokenLimit.value,
  languages: localLanguages.value,
}));

const languageOptions = computed(() => [
  { label: t("knowledgeEditor.chunking.languageOptions.de"), value: "de" },
  { label: t("knowledgeEditor.chunking.languageOptions.en"), value: "en" },
  { label: t("knowledgeEditor.chunking.languageOptions.zh"), value: "zh" },
]);

function languageLabel(value: string): string {
  return languageOptions.value.find((o) => o.value === value)?.label ?? value;
}

const separatorOptions = computed(() => [
  { label: t("knowledgeEditor.chunking.separators.doubleNewline"), value: "\n\n" },
  { label: t("knowledgeEditor.chunking.separators.singleNewline"), value: "\n" },
  { label: t("knowledgeEditor.chunking.separators.periodCn"), value: "。" },
  { label: t("knowledgeEditor.chunking.separators.exclamationCn"), value: "！" },
  { label: t("knowledgeEditor.chunking.separators.questionCn"), value: "？" },
  { label: t("knowledgeEditor.chunking.separators.semicolonCn"), value: "；" },
  { label: t("knowledgeEditor.chunking.separators.semicolonEn"), value: ";" },
  { label: t("knowledgeEditor.chunking.separators.space"), value: " " },
]);

// The preset separators plus any the user typed in: t-select showed created
// values as tags, so they must stay visible here to be removable.
const separatorChoices = computed(() => [
  ...separatorOptions.value,
  ...localSeparators.value
    .filter((value) => !separatorOptions.value.some((o) => o.value === value))
    .map((value) => ({ label: value, value })),
]);

function separatorLabel(value: string): string {
  return separatorOptions.value.find((o) => o.value === value)?.label ?? value;
}

function toggleSeparator(value: string, checked: boolean) {
  if (checked) {
    if (!localSeparators.value.includes(value)) localSeparators.value = [...localSeparators.value, value];
  } else {
    localSeparators.value = localSeparators.value.filter((s) => s !== value);
  }
  handleSeparatorsChange();
}

function addCustomSeparator() {
  const value = customSeparator.value;
  if (!value) return;
  if (!localSeparators.value.includes(value)) {
    localSeparators.value = [...localSeparators.value, value];
    handleSeparatorsChange();
  }
  customSeparator.value = "";
}

function toggleLanguage(value: string, checked: boolean) {
  if (checked) {
    if (!localLanguages.value.includes(value)) localLanguages.value = [...localLanguages.value, value];
  } else {
    localLanguages.value = localLanguages.value.filter((l) => l !== value);
  }
  handleLanguagesChange();
}

watch(
  () => props.config,
  (newConfig) => {
    localChunkSize.value = newConfig.chunkSize;
    localChunkOverlap.value = newConfig.chunkOverlap;
    localSeparators.value = [...newConfig.separators];
    localEnableParentChild.value = newConfig.enableParentChild ?? false;
    localParentChunkSize.value = newConfig.parentChunkSize || 4096;
    localChildChunkSize.value = newConfig.childChunkSize || 384;
    localStrategy.value = newConfig.strategy ?? "";
    localTokenLimit.value = newConfig.tokenLimit ?? 0;
    localLanguages.value = [...(newConfig.languages ?? [])];
  },
  { deep: true },
);

const handleChunkSizeChange = () => {
  emitUpdate();
};
const handleChunkOverlapChange = () => {
  emitUpdate();
};
const handleSeparatorsChange = () => {
  emitUpdate();
};
const handleParentChildChange = () => {
  emitUpdate();
};
const handleParentChunkSizeChange = () => {
  emitUpdate();
};
const handleChildChunkSizeChange = () => {
  emitUpdate();
};
const handleStrategyChange = () => {
  emitUpdate();
};
const handleTokenLimitChange = () => {
  emitUpdate();
};
const handleLanguagesChange = () => {
  emitUpdate();
};

const emitUpdate = () => {
  // Spread arrays so the parent gets its own copy. Mutating the emitted
  // arrays from outside must not leak back into our reactive state and
  // cause two-way ref drift between the form and the editor model.
  emit("update:config", {
    chunkSize: localChunkSize.value,
    chunkOverlap: localChunkOverlap.value,
    separators: [...localSeparators.value],
    parserEngineRules: props.config.parserEngineRules,
    enableParentChild: localEnableParentChild.value,
    parentChunkSize: localParentChunkSize.value,
    childChunkSize: localChildChunkSize.value,
    strategy: localStrategy.value,
    tokenLimit: localTokenLimit.value,
    languages: [...localLanguages.value],
  });
};
</script>
