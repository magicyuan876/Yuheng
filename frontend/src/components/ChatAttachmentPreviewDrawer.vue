<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";

import { useI18n } from "vue-i18n";

import { FileIcon, XIcon } from "@lucide/vue";

import { Drawer, DrawerClose, DrawerContent, DrawerHeader, DrawerTitle } from "@/components/ui/drawer";
import DocumentPreview from "@/components/document-preview.vue";
import { useChatAttachmentPreviewDrawer } from "@/composables/useChatAttachmentPreviewDrawer";

const drawer = useChatAttachmentPreviewDrawer();
const { t } = useI18n();

const MAIN_DRAWER_WIDTH_KEY = "yuheng-chat-attachment-drawer-width";
const MAIN_DRAWER_DEFAULT_WIDTH = 654;
const MAIN_DRAWER_MIN_WIDTH = 480;

const mainDrawerWidth = ref(MAIN_DRAWER_DEFAULT_WIDTH);
const mainDrawerResizing = ref(false);

let mainResizeStartX = 0;
let mainResizeStartWidth = 0;

const visible = computed(() => drawer?.visible.value ?? false);
const target = computed(() => drawer?.target.value ?? null);

function mainDrawerMaxWidth() {
  return Math.min(1600, Math.max(MAIN_DRAWER_MIN_WIDTH, Math.floor(window.innerWidth * 0.95)));
}

function clampMainDrawerWidth(width: number) {
  return Math.max(MAIN_DRAWER_MIN_WIDTH, Math.min(mainDrawerMaxWidth(), width));
}

function loadMainDrawerWidth() {
  try {
    const raw = localStorage.getItem(MAIN_DRAWER_WIDTH_KEY);
    const parsed = raw ? parseInt(raw, 10) : NaN;
    if (!Number.isNaN(parsed)) {
      mainDrawerWidth.value = clampMainDrawerWidth(parsed);
    }
  } catch {
    /* ignore */
  }
}

function onMainDrawerResizeStart(e: MouseEvent) {
  mainDrawerResizing.value = true;
  mainResizeStartX = e.clientX;
  mainResizeStartWidth = mainDrawerWidth.value;
  document.addEventListener("mousemove", onMainDrawerResizeMove);
  document.addEventListener("mouseup", onMainDrawerResizeEnd);
  document.body.style.cursor = "col-resize";
  document.body.style.userSelect = "none";
}

function onMainDrawerResizeMove(e: MouseEvent) {
  const delta = mainResizeStartX - e.clientX;
  mainDrawerWidth.value = clampMainDrawerWidth(mainResizeStartWidth + delta);
}

function onMainDrawerResizeEnd() {
  document.removeEventListener("mousemove", onMainDrawerResizeMove);
  document.removeEventListener("mouseup", onMainDrawerResizeEnd);
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
  mainDrawerResizing.value = false;
  try {
    localStorage.setItem(MAIN_DRAWER_WIDTH_KEY, String(mainDrawerWidth.value));
  } catch {
    /* ignore */
  }
}

function cleanupMainDrawerResize() {
  document.removeEventListener("mousemove", onMainDrawerResizeMove);
  document.removeEventListener("mouseup", onMainDrawerResizeEnd);
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
  mainDrawerResizing.value = false;
}

function onWindowResize() {
  mainDrawerWidth.value = clampMainDrawerWidth(mainDrawerWidth.value);
}

function close() {
  drawer?.close();
}

// A press on the resize handle (or its drag overlay) is not a press outside the
// drawer: both are teleported next to the panel, not into it, and a modal drawer
// would otherwise close the moment the user grabbed its edge.
function onDrawerPointerDownOutside(event: CustomEvent<{ originalEvent: PointerEvent }>) {
  const target = event.detail?.originalEvent?.target;
  if (
    target instanceof Element &&
    target.closest(".chat-attachment-drawer-resize-handle, .chat-attachment-drawer-resize-overlay")
  ) {
    event.preventDefault();
  }
}

onMounted(() => {
  loadMainDrawerWidth();
  window.addEventListener("resize", onWindowResize);
});

onUnmounted(() => {
  window.removeEventListener("resize", onWindowResize);
  cleanupMainDrawerResize();
});
</script>

<template>
  <teleport to="body">
    <!-- The handle and its drag overlay sit outside the drawer panel, and a modal Reka drawer turns
         pointer events off on everything outside its panel, so both opt back in; the panel also
         treats a press on the handle as inside (onDrawerPointerDownOutside). -->
    <div
      v-if="mainDrawerResizing"
      class="chat-attachment-drawer-resize-overlay pointer-events-auto fixed inset-0 z-[2001] cursor-col-resize"
      aria-hidden="true"
    />
    <div
      v-if="visible"
      class="chat-attachment-drawer-resize-handle group pointer-events-auto fixed top-0 bottom-0 z-[2002] -ml-1.5 flex w-3 cursor-col-resize items-center justify-center"
      :style="{ right: `${mainDrawerWidth}px` }"
      role="separator"
      aria-orientation="vertical"
      @mousedown.prevent="onMainDrawerResizeStart"
    >
      <div
        class="h-12 w-0.5 rounded-full transition-[opacity,background-color] duration-150"
        :class="
          mainDrawerResizing
            ? 'bg-primary opacity-100'
            : 'bg-border group-hover:bg-primary opacity-55 group-hover:opacity-100'
        "
      />
    </div>
  </teleport>

  <Drawer :open="visible" swipe-direction="right" @update:open="(v: boolean) => !v && close()">
    <!-- z-[2000] is the old t-drawer's z-index: above page chrome such as the invitation bell,
         below the resize handle and its overlay. -->
    <DrawerContent
      class="z-[2000] max-w-none rounded-none border-0 sm:max-w-none"
      :class="
        mainDrawerResizing
          ? '[&_*]:select-none [&_.document-preview]:pointer-events-none [&_iframe]:pointer-events-none'
          : ''
      "
      :style="{ width: `${mainDrawerWidth}px`, maxWidth: '95vw', transition: mainDrawerResizing ? 'none' : undefined }"
      @pointer-down-outside="onDrawerPointerDownOutside"
    >
      <DrawerHeader
        class="border-border relative flex shrink-0 flex-row items-center gap-2.5 border-b px-[18px] py-3.5"
      >
        <div
          class="bg-primary/10 text-primary flex h-8 w-8 shrink-0 items-center justify-center rounded-[9px] text-base"
        >
          <FileIcon class="size-4" />
        </div>
        <div class="min-w-0 flex-1 pr-8">
          <DrawerTitle class="text-foreground truncate text-[15px] leading-[1.4] font-semibold">
            {{ target?.fileName || "" }}
          </DrawerTitle>
        </div>
        <DrawerClose
          class="text-muted-foreground hover:bg-accent hover:text-foreground absolute top-4 right-4 inline-flex size-6 cursor-pointer items-center justify-center rounded-md"
          :aria-label="t('common.close')"
        >
          <XIcon class="size-4" />
        </DrawerClose>
      </DrawerHeader>

      <section v-if="target" class="flex min-h-0 flex-1 flex-col overflow-hidden p-[12px_16px_16px]">
        <DocumentPreview
          :session-id="target.sessionId"
          :attachment-id="target.attachmentId"
          :file-type="target.fileType"
          :file-name="target.fileName"
          :active="visible"
          fill-height
        />
      </section>
    </DrawerContent>
  </Drawer>
</template>
