<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import {
  ArrowLeftRightIcon,
  ChartColumnIcon,
  DownloadIcon,
  FolderIcon,
  ListChecksIcon,
  PenLineIcon,
  RefreshCwIcon,
  Trash2Icon,
  XCircleIcon,
} from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { cn } from "@/lib/utils";

interface KnowledgeItem {
  id: string;
  file_name?: string;
  title?: string;
  type?: string;
  parse_status?: string;
}

const props = defineProps<{
  item: KnowledgeItem;
  canDownload: boolean;
  canMutateKnowledge: boolean;
  traceVisible: boolean;
  /** Whether the knowledge base has a folder structure to file documents into. */
  foldersAvailable?: boolean;
}>();

const emit = defineEmits<{
  (e: "download"): void;
  (e: "edit"): void;
  (e: "view-trace"): void;
  (e: "reparse"): void;
  (e: "cancel-parse"): void;
  (e: "move"): void;
  (e: "move-folder"): void;
  (e: "batch-manage"): void;
  (e: "delete"): void;
}>();

const { t } = useI18n();

const CANCELABLE_PARSE_STATUSES = new Set(["pending", "processing", "finalizing"]);

const isParseInFlight = computed(() => CANCELABLE_PARSE_STATUSES.has(String(props.item.parse_status ?? "")));

const fileName = computed(() => props.item.file_name || props.item.title || props.item.id);

const itemClass =
  "group flex cursor-pointer items-center gap-2.5 rounded-md px-3 py-2 text-sm leading-5 text-foreground transition-all hover:bg-accent active:bg-[var(--td-bg-color-container-active)] active:scale-[0.98]";
// The icon darkens with its row, as the old `.doc-action-menu-item:hover .icon` did.
const iconClass = "text-muted-foreground group-hover:text-foreground size-4 shrink-0 transition-colors";
// Merged through cn() rather than listed next to itemClass: both set a text
// colour and a hover background, and two conflicting utilities on one element
// resolve by stylesheet order, not by which was written last.
const dangerItemClass = cn(
  itemClass,
  "text-destructive relative mt-1 before:absolute before:-top-[3px] before:right-2 before:left-2 before:h-px before:bg-border before:content-[''] hover:bg-[var(--td-error-color-1)] active:bg-[var(--td-error-color-2)]",
);

const confirmState = ref<{ title: string; confirmText: string; event: "reparse" | "cancel-parse" | "delete" } | null>(
  null,
);

function openConfirm(event: "reparse" | "cancel-parse" | "delete") {
  if (event === "reparse") {
    confirmState.value = {
      title: t("knowledgeBase.rebuildConfirm", { fileName: fileName.value }),
      confirmText: t("common.confirm"),
      event,
    };
  } else if (event === "cancel-parse") {
    confirmState.value = {
      title: t("knowledgeBase.cancelParseConfirmBody", { title: fileName.value }),
      confirmText: t("knowledgeBase.cancelParse"),
      event,
    };
  } else {
    confirmState.value = {
      title: t("knowledgeBase.confirmDeleteDocument", { fileName: fileName.value }),
      confirmText: t("knowledgeBase.confirmDelete"),
      event,
    };
  }
}

function runConfirm() {
  if (!confirmState.value) return;
  const { event } = confirmState.value;
  confirmState.value = null;
  // One call per literal: the typed emit has an overload per event name and
  // will not take the union.
  if (event === "reparse") emit("reparse");
  else if (event === "cancel-parse") emit("cancel-parse");
  else emit("delete");
}
</script>

<template>
  <!-- 下载原始文档 -->
  <div
    v-if="canDownload && (item.type === 'file' || item.type === 'manual')"
    :class="itemClass"
    @click.stop="emit('download')"
  >
    <DownloadIcon :class="iconClass" />
    <span>{{ $t("common.download") }}</span>
  </div>

  <!-- 编辑文档 -->
  <div v-if="item.type === 'manual'" :class="itemClass" @click.stop="emit('edit')">
    <PenLineIcon :class="iconClass" />
    <span>{{ $t("knowledgeBase.editDocument") }}</span>
  </div>

  <!-- 查看处理过程 -->
  <div v-if="traceVisible" :class="itemClass" @click.stop="emit('view-trace')">
    <ChartColumnIcon :class="iconClass" />
    <span>{{ $t("knowledgeStages.viewTrace") }}</span>
  </div>

  <!-- 重建知识 (in-flight: no confirm, just emits) -->
  <div v-if="isParseInFlight" :class="itemClass" @click.stop="emit('reparse')">
    <RefreshCwIcon :class="iconClass" />
    <span>{{ $t("knowledgeBase.rebuildDocument") }}</span>
  </div>

  <!-- 重建知识 (normal: with confirm) -->
  <div v-else :class="itemClass" @click.stop="openConfirm('reparse')">
    <RefreshCwIcon :class="iconClass" />
    <span>{{ $t("knowledgeBase.rebuildDocument") }}</span>
  </div>

  <!-- 取消解析 -->
  <div v-if="isParseInFlight" :class="dangerItemClass" @click.stop="openConfirm('cancel-parse')">
    <XCircleIcon class="size-4 shrink-0" />
    <span>{{ $t("knowledgeBase.cancelParse") }}</span>
  </div>

  <!-- 移动到目录 -->
  <div v-if="canMutateKnowledge" :class="itemClass" @click.stop="emit('move-folder')">
    <FolderIcon :class="iconClass" />
    <span>{{ $t("knowledgeBase.moveToFolder.action") }}</span>
  </div>

  <!-- 移动到其他知识库 -->
  <div v-if="canMutateKnowledge" :class="itemClass" @click.stop="emit('move')">
    <ArrowLeftRightIcon :class="iconClass" />
    <span>{{ $t("knowledgeBase.moveDocument") }}</span>
  </div>

  <!-- 批量管理 -->
  <div v-if="canMutateKnowledge" :class="itemClass" @click.stop="emit('batch-manage')">
    <ListChecksIcon :class="iconClass" />
    <span>{{ $t("menu.batchManage") }}</span>
  </div>

  <!-- 删除文档 -->
  <div :class="dangerItemClass" @click.stop="openConfirm('delete')">
    <Trash2Icon class="size-4 shrink-0" />
    <span>{{ $t("knowledgeBase.deleteDocument") }}</span>
  </div>

  <Dialog :open="confirmState !== null" @update:open="(v: boolean) => !v && (confirmState = null)">
    <DialogContent class="sm:max-w-[420px]">
      <DialogHeader>
        <DialogTitle>{{ confirmState?.title }}</DialogTitle>
      </DialogHeader>
      <DialogFooter>
        <Button variant="outline" @click="confirmState = null">{{ $t("common.cancel") }}</Button>
        <Button :variant="confirmState?.event === 'reparse' ? 'default' : 'destructive'" @click="runConfirm">
          {{ confirmState?.confirmText }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
