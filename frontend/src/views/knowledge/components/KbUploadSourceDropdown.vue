<template>
  <div>
    <input
      ref="fileInputRef"
      type="file"
      class="pointer-events-none absolute h-0 w-0 opacity-0"
      multiple
      :accept="acceptFileTypes || undefined"
      @change="(e) => handleFilesChange(e, false)"
    />
    <input
      ref="folderInputRef"
      type="file"
      class="pointer-events-none absolute h-0 w-0 opacity-0"
      webkitdirectory
      multiple
      @change="(e) => handleFilesChange(e, true)"
    />

    <!--
      The menu is the outer component so that the tooltip's trigger and the
      menu's trigger can both land on the one button (as-child all the way down).
    -->
    <DropdownMenu>
      <Tooltip>
        <TooltipTrigger as-child>
          <DropdownMenuTrigger as-child>
            <Button
              variant="ghost"
              size="icon-xs"
              :class="cn('text-muted-foreground hover:text-primary', triggerClass)"
              :data-guide="dataGuide || undefined"
              :aria-label="tooltipText"
            >
              <component :is="triggerIconComponent" class="size-4" />
            </Button>
          </DropdownMenuTrigger>
        </TooltipTrigger>
        <TooltipContent side="top">{{ tooltipText }}</TooltipContent>
      </Tooltip>
      <DropdownMenuContent :side="dropdownSide" :align="dropdownAlign">
        <DropdownMenuItem @select="handleActionSelect('upload')">
          <UploadIcon class="size-4" />
          {{ t("upload.uploadDocument") }}
        </DropdownMenuItem>
        <DropdownMenuItem @select="handleActionSelect('uploadFolder')">
          <FolderPlusIcon class="size-4" />
          {{ t("upload.uploadFolder") }}
        </DropdownMenuItem>
        <DropdownMenuItem @select="handleActionSelect('importURL')">
          <LinkIcon class="size-4" />
          {{ t("knowledgeBase.importURL") }}
        </DropdownMenuItem>
        <DropdownMenuItem v-if="includeManual" @select="handleActionSelect('manualCreate')">
          <SquarePenIcon class="size-4" />
          {{ t("upload.onlineEdit") }}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>

    <!--
      z-[3100]: this menu also sits inside UploadConfirmDialog, whose overlay is
      z-[3000], above the dialog layer (2500); at the default layer the URL
      dialog opened from "continue adding" would open behind it.
    -->
    <Dialog v-model:open="urlDialogVisible">
      <DialogContent class="z-[3100] sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{{ t("knowledgeBase.importURLTitle") }}</DialogTitle>
        </DialogHeader>
        <div>
          <div class="text-foreground mb-2 text-sm font-medium">{{ t("knowledgeBase.urlLabel") }}</div>
          <div class="relative">
            <Input
              v-model="urlInputValue"
              :placeholder="t('knowledgeBase.urlPlaceholder')"
              class="pr-8"
              autofocus
              @keydown.enter="handleUrlDialogConfirm"
            />
            <button
              v-if="urlInputValue"
              type="button"
              data-slot="input-clear"
              class="text-placeholder hover:text-muted-foreground absolute top-1/2 right-2 -translate-y-1/2"
              :aria-label="t('common.clear')"
              @click="urlInputValue = ''"
            >
              <CircleXIcon class="size-4" aria-hidden="true" />
            </button>
          </div>
          <div class="text-placeholder mt-2 text-xs leading-normal">{{ t("knowledgeBase.urlTip") }}</div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="handleUrlDialogCancel">{{ t("common.cancel") }}</Button>
          <Button @click="handleUrlDialogConfirm">{{ t("common.confirm") }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import {
  CircleXIcon,
  FilePlusIcon,
  FolderPlusIcon,
  LinkIcon,
  SquarePenIcon,
  UploadIcon,
  type LucideIcon,
} from "@lucide/vue";
import { filterUploadFiles } from "../utils/uploadSources";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

const props = withDefaults(
  defineProps<{
    acceptFileTypes?: string;
    supportedFileTypes?: string[];
    includeManual?: boolean;
    triggerIcon?: string;
    triggerClass?: string;
    dataGuide?: string;
    tooltip?: string;
    placement?: "top" | "bottom" | "bottom-right" | "bottom-left";
  }>(),
  {
    acceptFileTypes: "",
    supportedFileTypes: () => [],
    includeManual: false,
    triggerIcon: "file-add",
    triggerClass: "",
    dataGuide: "",
    tooltip: "",
    placement: "bottom-right",
  },
);

const emit = defineEmits<{
  files: [files: File[]];
  url: [url: string];
  manual: [];
}>();

const { t } = useI18n();

const fileInputRef = ref<HTMLInputElement | null>(null);
const folderInputRef = ref<HTMLInputElement | null>(null);
const urlDialogVisible = ref(false);
const urlInputValue = ref("");

const tooltipText = computed(() => props.tooltip || t("knowledgeBase.addDocument"));

// The prop still takes the TDesign icon names the callers pass.
const triggerIconMap: Record<string, LucideIcon> = {
  "file-add": FilePlusIcon,
  upload: UploadIcon,
};

const triggerIconComponent = computed(() => triggerIconMap[props.triggerIcon] ?? UploadIcon);

const dropdownSide = computed(() => (props.placement === "top" ? "top" : "bottom"));
const dropdownAlign = computed(() => {
  if (props.placement === "bottom-left") return "start";
  if (props.placement === "bottom-right") return "end";
  return "center";
});

const handleActionSelect = (value: string) => {
  switch (value) {
    case "upload":
      fileInputRef.value?.click();
      break;
    case "uploadFolder":
      folderInputRef.value?.click();
      break;
    case "importURL":
      urlInputValue.value = "";
      urlDialogVisible.value = true;
      break;
    case "manualCreate":
      emit("manual");
      break;
    default:
      break;
  }
};

const notifyFilterResult = (result: ReturnType<typeof filterUploadFiles>, emptyAllSkippedKey: string) => {
  const { validFiles, skippedCount } = result;
  if (validFiles.length === 0) {
    if (skippedCount > 0) {
      MessagePlugin.warning(t(emptyAllSkippedKey));
    }
    return false;
  }
  if (skippedCount > 0) {
    MessagePlugin.warning(t("knowledgeBase.filesSkippedNoEngine", { count: skippedCount }));
  }
  return true;
};

const handleFilesChange = (event: Event, fromFolder: boolean) => {
  const input = event.target as HTMLInputElement;
  const files = input.files;
  if (!files || files.length === 0) return;

  const result = filterUploadFiles(files, {
    supportedFileTypes: props.supportedFileTypes,
    fromFolder,
    multiFile: files.length > 1,
  });

  if (!notifyFilterResult(result, "knowledgeBase.allFilesSkippedNoEngine")) {
    input.value = "";
    return;
  }

  emit("files", result.validFiles);
  input.value = "";
};

const handleUrlDialogConfirm = () => {
  const url = urlInputValue.value.trim();
  if (!url) {
    MessagePlugin.warning(t("knowledgeBase.urlRequired"));
    return;
  }
  try {
    new URL(url);
  } catch {
    MessagePlugin.warning(t("knowledgeBase.invalidURL"));
    return;
  }
  urlDialogVisible.value = false;
  urlInputValue.value = "";
  emit("url", url);
};

const handleUrlDialogCancel = () => {
  urlDialogVisible.value = false;
  urlInputValue.value = "";
};

const openUrlDialog = () => {
  urlInputValue.value = "";
  urlDialogVisible.value = true;
};

defineExpose({ openUrlDialog });
</script>
