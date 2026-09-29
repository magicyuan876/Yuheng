<template>
  <div class="w-full">
    <div v-if="!embedded" class="mb-5">
      <h2 class="text-foreground mt-0 mb-1.5 text-xl font-semibold">{{ $t("kbSettings.parser.title") }}</h2>
      <p class="text-muted-foreground m-0 text-sm leading-normal">{{ $t("kbSettings.parser.description") }}</p>
    </div>

    <div v-if="loading" class="flex items-center gap-2 py-4">
      <Loader2Icon class="size-4 animate-spin" />
      <span>{{ $t("kbSettings.parser.loading") }}</span>
    </div>

    <div v-else-if="fileTypeGroups.length === 0" class="text-muted-foreground py-6">
      <p class="m-0">{{ $t("kbSettings.parser.noEngineAvailable") }}</p>
    </div>

    <div
      v-else
      class="flex flex-col"
      :class="embedded ? 'border-border bg-muted overflow-hidden rounded-lg border' : ''"
    >
      <div
        v-for="group in fileTypeGroups"
        :key="group.key"
        class="border-border flex justify-between [&:not(:last-child)]:border-b"
        :class="embedded ? 'bg-card items-center gap-4 px-3.5 py-2.5' : 'items-start py-4'"
      >
        <div class="shrink-0" :class="embedded ? 'block max-w-[168px] basis-[168px]' : 'max-w-[40%] basis-2/5 pr-6'">
          <label
            class="text-foreground mb-1 flex items-center gap-1.5 font-medium"
            :class="embedded ? 'text-[13px]' : 'text-[15px]'"
          >
            <component :is="iconFor(group.icon)" v-if="!embedded" class="text-muted-foreground size-[18px] shrink-0" />
            {{ group.label }}
          </label>
          <div class="flex flex-wrap" :class="embedded ? 'mt-0 gap-1' : 'mt-1.5 gap-1.5'">
            <span
              v-for="ext in group.extensions"
              :key="ext"
              class="bg-muted text-muted-foreground inline-block rounded [font-family:var(--app-font-family-mono)] leading-none"
              :class="embedded ? 'px-1.5 py-0.5 text-[11px]' : 'px-2 py-[3px] text-xs'"
              >.{{ ext }}</span
            >
          </div>
        </div>
        <div
          class="flex flex-col"
          :class="embedded ? 'min-w-0 flex-1 items-stretch' : 'max-w-[55%] shrink-0 basis-[55%] items-end'"
        >
          <div class="flex flex-col items-stretch gap-2.5" :class="embedded ? 'w-full' : 'w-[280px]'">
            <Select
              :model-value="getEngineForGroup(group.extensions) || undefined"
              @update:model-value="(val) => handleEngineChange(group.extensions, String(val))"
            >
              <SelectTrigger
                class="w-full"
                :class="[embedded ? '' : 'w-[280px]', !hasAvailableEngine(group.extensions) ? 'border-warning' : '']"
              >
                <SelectValue :placeholder="$t('kbSettings.parser.noEngine')" />
              </SelectTrigger>
              <!-- The old popup capped its list at 240px. -->
              <SelectContent class="max-h-[240px]">
                <SelectItem v-for="opt in getEngineOptions(group.extensions)" :key="opt.value" :value="opt.value">
                  {{ opt.selectLabel }}
                </SelectItem>
              </SelectContent>
            </Select>
            <!-- A <label>, so the text toggles the box as t-checkbox's label did. -->
            <label
              v-if="group.extensions.includes('xlsx') && getEngineForGroup(group.extensions) === 'builtin'"
              class="flex cursor-pointer items-start gap-2"
            >
              <Checkbox
                class="mt-0.5"
                :model-value="getXLSXFirstRowAsHeader(group.extensions)"
                @update:model-value="(checked) => handleXLSXFirstRowAsHeaderChange(group.extensions, checked === true)"
              />
              <span class="text-foreground text-left text-xs leading-normal">{{
                $t("kbSettings.parser.xlsxFirstRowAsHeader")
              }}</span>
            </label>
            <div
              v-if="!hasAvailableEngine(group.extensions)"
              class="text-warning mt-2 flex items-center gap-1 text-xs leading-[1.4]"
            >
              <a
                v-if="canManageParser"
                class="text-primary cursor-pointer whitespace-nowrap no-underline hover:underline"
                @click.prevent="goToParserSettings"
                >{{ $t("kbSettings.parser.goConfig") }}</a
              >
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { storeToRefs } from "pinia";
import {
  FileAudioIcon,
  FileCodeIcon,
  FileIcon,
  FileSpreadsheetIcon,
  FileTextIcon,
  ImageIcon,
  Loader2Icon,
  PresentationIcon,
  type LucideIcon,
} from "@lucide/vue";
import { type ParserEngineInfo } from "@/api/system";
import { useEditorResourcesStore } from "@/stores/editorResources";
import { useUIStore } from "@/stores/ui";
import { usePlatformInfraAccess } from "@/composables/usePlatformInfraAccess";
import { Checkbox } from "@/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

const { t } = useI18n();
const editorResources = useEditorResourcesStore();

function getEngineDisplayName(engineName: string): string {
  const key = `kbSettings.parser.engines.${engineName}.name`;
  const translated = t(key);
  return translated !== key ? translated : engineName;
}

export interface ParserEngineRule {
  file_types: string[];
  engine: string;
  xlsx_first_row_as_header?: boolean;
}

interface EngineOption {
  value: string;
  selectLabel: string;
  isDefault: boolean;
}

function buildOptionLabel(name: string, isDefault: boolean): string {
  const label = getEngineDisplayName(name);
  return isDefault ? `${label} (${t("kbSettings.parser.default")})` : label;
}

interface Props {
  parserEngineRules?: ParserEngineRule[];
  /** Compact layout for upload-confirm dialog */
  embedded?: boolean;
  /** When set, only show file-type groups matching these extensions */
  relevantExtensions?: string[];
}

const props = withDefaults(defineProps<Props>(), {
  parserEngineRules: () => [],
  embedded: false,
  relevantExtensions: () => [],
});

const emit = defineEmits<{
  "update:parserEngineRules": [value: ParserEngineRule[]];
}>();

const uiStore = useUIStore();
const canManageParser = usePlatformInfraAccess("parser");
const localEngineRules = ref<ParserEngineRule[]>([...props.parserEngineRules]);
const parserEngines = ref<ParserEngineInfo[]>([]);
const loading = ref(true);

const iconComponents: Record<string, LucideIcon> = {
  "file-pdf": FileTextIcon,
  "file-word": FileTextIcon,
  "file-powerpoint": PresentationIcon,
  "file-excel": FileSpreadsheetIcon,
  file: FileIcon,
  "file-code": FileCodeIcon,
  image: ImageIcon,
  sound: FileAudioIcon,
};

function iconFor(name: string): LucideIcon {
  return iconComponents[name] ?? FileIcon;
}

const allFileTypes = computed(() => {
  const s = new Set<string>();
  for (const engine of parserEngines.value) {
    for (const ft of engine.FileTypes || []) {
      s.add(ft);
    }
  }
  return s;
});

const fileTypeGroups = computed(() => {
  const ft = allFileTypes.value;
  const groups: { key: string; label: string; icon: string; extensions: string[] }[] = [];

  const pdfExts = ["pdf"].filter((e) => ft.has(e));
  const officeExts = ["docx", "doc"].filter((e) => ft.has(e));
  const pptExts = ["pptx", "ppt"].filter((e) => ft.has(e));
  const excelExts = ["xlsx", "xls"].filter((e) => ft.has(e));
  const ebookExts = ["epub"].filter((e) => ft.has(e));
  const webArchiveExts = ["mhtml"].filter((e) => ft.has(e));
  const csvExts = ["csv"].filter((e) => ft.has(e));
  const mdExts = ["md", "markdown"].filter((e) => ft.has(e));
  const txtExts = ["txt"].filter((e) => ft.has(e));
  const jsonExts = ["json"].filter((e) => ft.has(e));
  const imageExts = ["jpg", "jpeg", "png", "gif", "bmp", "tiff", "webp"].filter((e) => ft.has(e));
  const audioExts = ["mp3", "wav", "m4a", "flac", "ogg"].filter((e) => ft.has(e));
  const audiovisualExts = [...audioExts];

  if (pdfExts.length)
    groups.push({ key: "pdf", label: t("kbSettings.parser.fileTypePdf"), icon: "file-pdf", extensions: pdfExts });
  if (officeExts.length)
    groups.push({
      key: "office",
      label: t("kbSettings.parser.fileTypeWord"),
      icon: "file-word",
      extensions: officeExts,
    });
  if (pptExts.length)
    groups.push({
      key: "ppt",
      label: t("kbSettings.parser.fileTypePpt"),
      icon: "file-powerpoint",
      extensions: pptExts,
    });
  if (excelExts.length)
    groups.push({
      key: "excel",
      label: t("kbSettings.parser.fileTypeExcel"),
      icon: "file-excel",
      extensions: excelExts,
    });
  if (ebookExts.length)
    groups.push({ key: "ebook", label: t("kbSettings.parser.fileTypeEbook"), icon: "file", extensions: ebookExts });
  if (webArchiveExts.length)
    groups.push({
      key: "webarchive",
      label: t("kbSettings.parser.fileTypeWebArchive"),
      icon: "file",
      extensions: webArchiveExts,
    });
  if (csvExts.length)
    groups.push({ key: "csv", label: t("kbSettings.parser.fileTypeCsv"), icon: "file-excel", extensions: csvExts });
  if (mdExts.length) groups.push({ key: "markdown", label: "Markdown", icon: "file-code", extensions: mdExts });
  if (txtExts.length)
    groups.push({ key: "text", label: t("kbSettings.parser.fileTypeText"), icon: "file", extensions: txtExts });
  if (jsonExts.length)
    groups.push({ key: "json", label: t("kbSettings.parser.fileTypeJson"), icon: "file-code", extensions: jsonExts });
  if (imageExts.length)
    groups.push({ key: "image", label: t("kbSettings.parser.fileTypeImage"), icon: "image", extensions: imageExts });
  if (audiovisualExts.length) {
    groups.push({
      key: "audiovisual",
      label: t("kbSettings.parser.fileTypeAudiovisual"),
      icon: "sound",
      extensions: audiovisualExts,
    });
  }

  // Keep the UI driven by the backend registry. New parser plugins can expose
  // file types without requiring another frontend release; known families get
  // friendly labels above and everything else gets a compact dynamic row.
  const grouped = new Set(groups.flatMap((group) => group.extensions));
  for (const ext of [...ft].filter((ext) => !grouped.has(ext) && ext !== "url").sort()) {
    groups.push({ key: `dynamic-${ext}`, label: ext.toUpperCase(), icon: "file-code", extensions: [ext] });
  }

  const rel = props.relevantExtensions;
  if (!rel?.length) return groups;
  const relSet = new Set(rel);
  const filtered = groups.filter((g) => g.extensions.some((e) => relSet.has(e)));
  return filtered.length > 0 ? filtered : groups;
});

function getEngineOptions(extensions: string[]): EngineOption[] {
  const raw: { name: string; desc: string; fileTypes: string[]; available: boolean; reason: string }[] = [];
  for (const engine of parserEngines.value) {
    const supports = extensions.some((ext) => (engine.FileTypes || []).includes(ext));
    if (supports) {
      raw.push({
        name: engine.Name,
        desc: engine.Description || engine.Name,
        fileTypes: engine.FileTypes || [],
        available: engine.Available !== false,
        reason: engine.UnavailableReason || "",
      });
    }
  }
  const defaultName = raw.find((e) => e.available)?.name ?? "";
  return raw
    .filter((e) => e.available)
    .map((e) => ({
      value: e.name,
      selectLabel: buildOptionLabel(e.name, defaultName !== "" && e.name === defaultName),
      isDefault: defaultName !== "" && e.name === defaultName,
    }));
}

function hasAvailableEngine(extensions: string[]): boolean {
  return getEngineOptions(extensions).length > 0;
}

function getDefaultEngine(extensions: string[]): string {
  const opts = getEngineOptions(extensions);
  return opts.find((o) => o.isDefault)?.value ?? "";
}

function getEngineForGroup(extensions: string[]): string {
  for (const rule of localEngineRules.value) {
    if (rule.file_types.some((ft) => extensions.includes(ft))) {
      return rule.engine;
    }
  }
  return getDefaultEngine(extensions);
}

function handleEngineChange(extensions: string[], engine: string) {
  const currentRule = getRuleForGroup(extensions);
  const otherRules = localEngineRules.value.filter((r) => !r.file_types.some((ft) => extensions.includes(ft)));
  if (engine) {
    otherRules.push({
      file_types: [...extensions],
      engine,
      ...(currentRule?.xlsx_first_row_as_header !== undefined
        ? { xlsx_first_row_as_header: currentRule.xlsx_first_row_as_header }
        : {}),
    });
  }
  localEngineRules.value = otherRules;
  emit("update:parserEngineRules", buildCompleteRules());
}

function getRuleForGroup(extensions: string[]): ParserEngineRule | undefined {
  return localEngineRules.value.find((rule) => rule.file_types.some((fileType) => extensions.includes(fileType)));
}

function getXLSXFirstRowAsHeader(extensions: string[]): boolean {
  return getRuleForGroup(extensions)?.xlsx_first_row_as_header === true;
}

function handleXLSXFirstRowAsHeaderChange(extensions: string[], checked: boolean) {
  const rules = buildCompleteRules();
  const rule = rules.find((item) => item.file_types.some((fileType) => extensions.includes(fileType)));
  if (!rule) return;

  rule.xlsx_first_row_as_header = checked;
  localEngineRules.value = rules;
  emit("update:parserEngineRules", rules);
}

function buildCompleteRules(): ParserEngineRule[] {
  const rules: ParserEngineRule[] = [];
  for (const group of fileTypeGroups.value) {
    const engine = getEngineForGroup(group.extensions);
    if (engine) {
      const currentRule = getRuleForGroup(group.extensions);
      rules.push({
        file_types: [...group.extensions],
        engine,
        ...(currentRule?.xlsx_first_row_as_header !== undefined
          ? { xlsx_first_row_as_header: currentRule.xlsx_first_row_as_header }
          : {}),
      });
    }
  }
  return rules;
}

function goToParserSettings() {
  uiStore.openSettings("parser");
}

async function loadEngines(force = false) {
  loading.value = true;
  try {
    await editorResources.ensureParserEngines(force);
    parserEngines.value = editorResources.parserEngines as ParserEngineInfo[];
  } catch {
    parserEngines.value = [];
  } finally {
    loading.value = false;
    ensureCompleteRules();
  }
}

function ensureCompleteRules() {
  if (!parserEngines.value.length) return;
  const complete = buildCompleteRules();
  if (complete.length && complete.length > localEngineRules.value.length) {
    localEngineRules.value = complete;
    emit("update:parserEngineRules", complete);
  }
}

onMounted(loadEngines);

const { showSettingsModal } = storeToRefs(uiStore);
watch(showSettingsModal, (open, wasOpen) => {
  if (wasOpen && !open) {
    loadEngines(true);
  }
});

watch(
  () => props.parserEngineRules,
  (v) => {
    localEngineRules.value = v?.length ? [...v] : [];
  },
  { deep: true },
);
</script>
