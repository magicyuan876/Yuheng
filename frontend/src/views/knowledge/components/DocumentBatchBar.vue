<script setup lang="ts">
import { ref } from "vue";
import { useI18n } from "vue-i18n";
import { FolderIcon, Loader2Icon, RefreshCwIcon, TagsIcon, Trash2Icon } from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import FolderPickerMenu, { type FolderOption } from "./FolderPickerMenu.vue";

defineProps<{
  count: number;
  deleteLoading?: boolean;
  reparseLoading?: boolean;
  tagLoading?: boolean;
  // When true the bar stays visible even with 0 selections, so users can exit
  // batch mode from here without selecting anything first.
  visible?: boolean;
  /** Hidden when the knowledge base has no folder structure to file into. */
  showMoveToFolder?: boolean;
  folderOptions?: FolderOption[];
}>();

const emit = defineEmits<{
  (e: "cancel"): void;
  (e: "delete"): void;
  (e: "reparse"): void;
  (e: "batchTag"): void;
  (e: "moveToFolder", folderPath: string): void;
}>();

const { t } = useI18n();

const folderPickerVisible = ref(false);
const reparseConfirmOpen = ref(false);
const deleteConfirmOpen = ref(false);
</script>

<template>
  <transition name="batch-bar-fade">
    <div
      v-if="visible || count > 0"
      class="relative z-5 mx-auto box-border w-full max-w-[560px] px-1"
      role="region"
      :aria-label="t('knowledgeBase.selectedCount', { count })"
    >
      <div
        class="bg-card flex items-center justify-between gap-3 rounded-[8px] border border-[var(--td-component-stroke)] px-3 py-2 shadow-[0_6px_16px_rgba(0,0,0,0.08)]"
      >
        <div class="flex min-w-0 flex-1 items-center gap-1">
          <span class="text-muted-foreground text-[13px] font-medium whitespace-nowrap">{{
            t("knowledgeBase.selectedCount", { count })
          }}</span>
          <Button
            variant="ghost"
            size="sm"
            class="text-muted-foreground hover:text-primary h-7 shrink-0 px-1.5 text-xs font-normal hover:bg-transparent dark:hover:bg-transparent"
            @click="emit('cancel')"
          >
            {{ t("knowledgeBase.clearSelection") }}
          </Button>
        </div>
        <div class="flex shrink-0 flex-wrap items-center justify-end gap-2">
          <Button
            variant="outline"
            size="xs"
            :disabled="count === 0 || deleteLoading || reparseLoading || tagLoading"
            @click.stop="reparseConfirmOpen = true"
          >
            <Loader2Icon v-if="reparseLoading" class="size-3.5 animate-spin" />
            <RefreshCwIcon v-else class="size-3.5" />
            {{ t("knowledgeBase.rebuildDocument") }}
          </Button>

          <Button
            variant="outline"
            size="xs"
            :disabled="count === 0 || deleteLoading || reparseLoading || tagLoading"
            @click="emit('batchTag')"
          >
            <Loader2Icon v-if="tagLoading" class="size-3.5 animate-spin" />
            <TagsIcon v-else class="size-3.5" />
            {{ t("knowledgeBase.batchTag") }}
          </Button>

          <Popover v-if="showMoveToFolder" v-model:open="folderPickerVisible">
            <PopoverTrigger as-child>
              <Button
                variant="outline"
                size="xs"
                :disabled="count === 0 || deleteLoading || reparseLoading || tagLoading"
              >
                <FolderIcon class="size-3.5" />
                {{ t("knowledgeBase.moveToFolder.action") }}
              </Button>
            </PopoverTrigger>
            <PopoverContent side="top" class="w-auto min-w-[148px] gap-0 rounded-[10px] p-1">
              <div class="flex min-w-[140px] flex-col gap-px">
                <FolderPickerMenu
                  :options="folderOptions || []"
                  @confirm="
                    (path: string) => {
                      folderPickerVisible = false;
                      emit('moveToFolder', path);
                    }
                  "
                />
              </div>
            </PopoverContent>
          </Popover>

          <!-- TDesign's danger outline: red text and border on the bar's own surface. -->
          <Button
            variant="outline"
            size="xs"
            class="border-destructive text-destructive hover:text-destructive dark:border-destructive hover:bg-[var(--td-error-color-1)]"
            :disabled="count === 0 || deleteLoading || reparseLoading || tagLoading"
            @click.stop="deleteConfirmOpen = true"
          >
            <Loader2Icon v-if="deleteLoading" class="size-3.5 animate-spin" />
            <Trash2Icon v-else class="size-3.5" />
            {{ t("knowledgeBase.batchDelete") }}
          </Button>
        </div>
      </div>
    </div>
  </transition>

  <Dialog v-model:open="reparseConfirmOpen">
    <DialogContent class="sm:max-w-[420px]">
      <DialogHeader>
        <DialogTitle>{{ t("knowledgeBase.confirmBatchReparseDocument", { count }) }}</DialogTitle>
      </DialogHeader>
      <DialogFooter>
        <Button variant="outline" @click="reparseConfirmOpen = false">{{ t("common.cancel") }}</Button>
        <Button
          class="bg-warning/15 text-warning hover:bg-warning/25"
          :disabled="reparseLoading"
          @click="
            emit('reparse');
            reparseConfirmOpen = false;
          "
        >
          {{ t("knowledgeBase.confirmBatchReparse") }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <Dialog v-model:open="deleteConfirmOpen">
    <DialogContent class="sm:max-w-[420px]">
      <DialogHeader>
        <DialogTitle>{{ t("knowledgeBase.confirmBatchDeleteDocument", { count }) }}</DialogTitle>
      </DialogHeader>
      <DialogFooter>
        <Button variant="outline" @click="deleteConfirmOpen = false">{{ t("common.cancel") }}</Button>
        <Button
          variant="destructive"
          :disabled="deleteLoading"
          @click="
            emit('delete');
            deleteConfirmOpen = false;
          "
        >
          {{ t("knowledgeBase.confirmDelete") }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<!-- Vue <transition> hooks have no Tailwind equivalent; this is the only
     style block in the file and exists solely to animate the bar in/out. -->
<style scoped>
.batch-bar-fade-enter-active,
.batch-bar-fade-leave-active {
  transition:
    transform 0.2s ease,
    opacity 0.2s ease;
}

.batch-bar-fade-enter-from,
.batch-bar-fade-leave-to {
  opacity: 0;
  transform: translateY(6px);
}
</style>
