<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import { CircleCheckIcon, Loader2Icon, MinusCircleIcon, TagsIcon, Trash2Icon } from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";

const props = defineProps<{
  count: number;
  enabledCount: number;
  disabledCount: number;
  canEdit: boolean;
  canManage: boolean;
  tagLoading?: boolean;
  statusAction?: "enable" | "disable" | null;
  deleteLoading?: boolean;
}>();

const emit = defineEmits<{
  (event: "cancel"): void;
  (event: "batchTag"): void;
  (event: "enable"): void;
  (event: "disable"): void;
  (event: "delete"): void;
}>();

const { t } = useI18n();

const actionLoading = computed(() => props.tagLoading || props.statusAction != null || props.deleteLoading);
const deleteConfirmOpen = ref(false);
</script>

<template>
  <transition name="faq-batch-bar-fade">
    <div
      v-if="count > 0 && (canEdit || canManage)"
      class="box-border w-full max-w-[760px] px-1"
      role="region"
      :aria-label="t('knowledgeBase.selectedCount', { count })"
    >
      <div
        class="bg-card flex items-center justify-between gap-3 rounded-[8px] border border-[var(--td-component-stroke)] px-3 py-2 shadow-[0_6px_16px_rgba(0,0,0,0.08)] max-[760px]:flex-col max-[760px]:items-stretch"
      >
        <div class="flex min-w-0 shrink-0 items-center gap-1">
          <span class="text-muted-foreground text-[13px] font-medium whitespace-nowrap">{{
            t("knowledgeBase.selectedCount", { count })
          }}</span>
          <Button variant="ghost" size="xs" class="font-normal" :disabled="actionLoading" @click="emit('cancel')">
            {{ t("knowledgeBase.clearSelection") }}
          </Button>
        </div>

        <div class="flex flex-wrap items-center justify-end gap-2 max-[760px]:justify-start">
          <Button v-if="canEdit" variant="outline" size="xs" :disabled="actionLoading" @click="emit('batchTag')">
            <Loader2Icon v-if="tagLoading" class="size-3.5 animate-spin" />
            <TagsIcon v-else class="size-3.5" />
            {{ t("knowledgeEditor.faq.batchUpdateTag") }}
          </Button>

          <Button
            v-if="canEdit && disabledCount > 0"
            variant="outline"
            size="xs"
            :disabled="actionLoading"
            @click="emit('enable')"
          >
            <Loader2Icon v-if="statusAction === 'enable'" class="size-3.5 animate-spin" />
            <CircleCheckIcon v-else class="size-3.5" />
            {{ t("knowledgeEditor.faq.batchEnable") }}
          </Button>

          <Button
            v-if="canEdit && enabledCount > 0"
            variant="outline"
            size="xs"
            :disabled="actionLoading"
            @click="emit('disable')"
          >
            <Loader2Icon v-if="statusAction === 'disable'" class="size-3.5 animate-spin" />
            <MinusCircleIcon v-else class="size-3.5" />
            {{ t("knowledgeEditor.faq.batchDisable") }}
          </Button>

          <template v-if="canManage">
            <!-- TDesign's danger outline: red text and border on the bar's own surface. -->
            <Button
              variant="outline"
              size="xs"
              class="border-destructive text-destructive hover:text-destructive dark:border-destructive hover:bg-[var(--td-error-color-1)]"
              :disabled="actionLoading"
              @click.stop="deleteConfirmOpen = true"
            >
              <Loader2Icon v-if="deleteLoading" class="size-3.5 animate-spin" />
              <Trash2Icon v-else class="size-3.5" />
              {{ t("knowledgeEditor.faq.batchDelete") }}
            </Button>
            <Dialog v-model:open="deleteConfirmOpen">
              <DialogContent class="sm:max-w-[420px]">
                <DialogHeader>
                  <DialogTitle>{{ t("knowledgeEditor.faq.confirmBatchDelete", { count }) }}</DialogTitle>
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
        </div>
      </div>
    </div>
  </transition>
</template>

<!-- Vue <transition> hooks have no Tailwind equivalent; this is the only
     style block in the file and exists solely to animate the bar in/out. -->
<style scoped>
.faq-batch-bar-fade-enter-active,
.faq-batch-bar-fade-leave-active {
  transition:
    transform 0.2s ease,
    opacity 0.2s ease;
}

.faq-batch-bar-fade-enter-from,
.faq-batch-bar-fade-leave-to {
  opacity: 0;
  transform: translateY(6px);
}
</style>
