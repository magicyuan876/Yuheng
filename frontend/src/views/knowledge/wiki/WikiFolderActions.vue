<template>
  <!-- Anchored in-place popup for folder actions. The trigger is icon-only to
       keep the directory row compact; everything (menu, name input, delete
       confirm) happens inside this one popup so no full-page dialog is spawned.
       click.stop on the trigger and content prevents the surrounding directory
       row from treating the interaction as an expand/collapse toggle. -->
  <Popover v-model:open="open" @update:open="onVisibleChange">
    <PopoverTrigger as-child>
      <!--
        Hidden until the directory row is hovered (the row is
        group/wiki-dir in WikiBrowser) or this menu is open.
      -->
      <span
        class="text-placeholder hover:text-primary inline-flex shrink-0 cursor-pointer items-center transition-[color,opacity] duration-150 group-hover/wiki-dir:opacity-100 focus-visible:opacity-100"
        :class="open ? 'opacity-100' : 'opacity-0'"
        :title="t('knowledgeEditor.wikiBrowser.folderActions')"
        :aria-label="t('knowledgeEditor.wikiBrowser.folderActions')"
        role="button"
        @click.stop
        @dragstart.prevent.stop
      >
        <EllipsisIcon class="size-[15px]" />
      </span>
    </PopoverTrigger>
    <!--
      One popover, two looks: the compact menu, and the roomier anchored form
      (name input, delete confirmation) it turns into.
    -->
    <PopoverContent
      align="start"
      :class="
        mode === 'menu'
          ? 'w-auto min-w-[148px] gap-0 rounded-[10px] p-1'
          : 'w-auto max-w-[min(392px,calc(100vw-24px))] min-w-[300px] gap-0 rounded-xl px-4 py-3.5'
      "
      @click.stop
    >
      <div v-if="mode === 'menu'" class="flex min-w-[188px] flex-col gap-px">
        <div :class="itemClass" @click="enterMode('create')">
          <FolderPlusIcon :class="iconClass" />
          <span>{{ t("knowledgeEditor.wikiBrowser.newSubfolder") }}</span>
        </div>
        <div :class="itemClass" @click="emitRename">
          <PenLineIcon :class="iconClass" />
          <span>{{ t("knowledgeEditor.wikiBrowser.renameFolder") }}</span>
        </div>
        <div :class="dangerItemClass" @click="enterMode('delete')">
          <Trash2Icon class="size-4 shrink-0" />
          <span>{{ t("knowledgeEditor.wikiBrowser.deleteFolder") }}</span>
        </div>
      </div>

      <div v-else class="max-w-full">
        <div v-if="mode === 'create'">
          <div class="text-foreground mb-3 text-[15px] leading-[1.35] font-semibold">
            {{ t("knowledgeEditor.wikiBrowser.newSubfolder") }}
          </div>
          <Input
            ref="inputRef"
            v-model="nameInput"
            :placeholder="t('knowledgeEditor.wikiBrowser.folderNamePlaceholder')"
            @keydown.enter="submitName"
          />
        </div>
        <div v-else>
          <div class="text-foreground mb-3 text-[15px] leading-[1.35] font-semibold">
            {{ t("knowledgeEditor.wikiBrowser.deleteFolder") }}
          </div>
          <div class="text-foreground pb-1 text-sm leading-[1.6] break-words">
            {{
              deletable
                ? t("knowledgeEditor.wikiBrowser.deleteFolderConfirm", { name })
                : t("knowledgeEditor.wikiBrowser.deleteFolderNotEmpty")
            }}
          </div>
        </div>
        <div class="mt-4 flex justify-end gap-2">
          <Button variant="outline" @click="open = false">
            {{ mode === "delete" && !deletable ? t("common.confirm") : t("common.cancel") }}
          </Button>
          <Button v-if="mode === 'create'" :disabled="!nameInput.trim()" @click="submitName">
            {{ t("common.confirm") }}
          </Button>
          <Button v-else-if="deletable" variant="destructive" @click="submitDelete">
            {{ t("common.confirm") }}
          </Button>
        </div>
      </div>
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { ref, computed, nextTick } from "vue";
import { useI18n } from "vue-i18n";
import { EllipsisIcon, FolderPlusIcon, PenLineIcon, Trash2Icon } from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";

const props = withDefaults(
  defineProps<{
    name?: string;
    pageCount?: number;
    hasChildren?: boolean;
  }>(),
  {
    name: "",
    pageCount: 0,
    hasChildren: false,
  },
);

const emit = defineEmits<{
  (e: "create", name: string): void;
  (e: "rename"): void;
  (e: "delete"): void;
}>();

const { t } = useI18n();

const open = ref(false);
const mode = ref<"menu" | "create" | "delete">("menu");
const nameInput = ref("");
const inputRef = ref<InstanceType<typeof Input> | null>(null);

const deletable = computed(() => props.pageCount === 0 && !props.hasChildren);

// The menu rows of the shared popup menu (formerly .popup-menu-item in
// dropdown-menu.css): the icon darkens with its row, and the destructive row
// sits under a hairline divider.
const itemClass =
  "group flex cursor-pointer items-center gap-2.5 rounded-md px-3 py-2 text-sm leading-5 text-foreground transition-all hover:bg-accent active:bg-[var(--td-bg-color-container-active)] active:scale-[0.98]";
const iconClass = "text-muted-foreground group-hover:text-foreground size-4 shrink-0 transition-colors";
// Merged through cn() so the red text and hover win over itemClass's own.
const dangerItemClass = cn(
  itemClass,
  "text-destructive relative mt-1 before:absolute before:-top-[3px] before:right-2 before:left-2 before:h-px before:bg-border before:content-[''] hover:bg-[var(--td-error-color-1)] active:bg-[var(--td-error-color-2)]",
);

function onVisibleChange(visible: boolean) {
  mode.value = "menu";
  if (!visible) {
    nameInput.value = "";
  }
}

function enterMode(next: "create" | "delete") {
  mode.value = next;
  if (next === "create") {
    nameInput.value = "";
    nextTick(() => {
      const el = inputRef.value?.$el;
      if (el instanceof HTMLElement) el.focus();
    });
  }
}

function emitRename() {
  emit("rename");
  open.value = false;
}

function submitName() {
  const value = nameInput.value.trim();
  if (!value) return;
  emit("create", value);
  open.value = false;
}

function submitDelete() {
  emit("delete");
  open.value = false;
}
</script>
