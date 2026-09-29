<template>
  <Teleport to="body">
    <!--
      The resize handle is teleported separately from the sliding panel, so
      it travels in along the same path (setting-drawer-resize-handle-in)
      instead of flashing at the panel's final left edge while the drawer is
      still entering from the right. Being outside the panel, it has to opt
      back into pointer events (a modal drawer switches them off for the rest
      of the page), and a press on it must not count as a press outside the
      drawer — see onPointerDownOutside.
    -->
    <div
      v-if="drawerVisible && resizable"
      class="setting-drawer-resize-handle group pointer-events-auto fixed top-0 bottom-0 z-[2501] -ml-1.5 flex w-3 cursor-col-resize items-center justify-center"
      :class="{ 'setting-drawer-resize-handle--active': drawerResizing }"
      :style="{ right: `${drawerWidthPx}px`, '--setting-drawer-travel': `${drawerWidthPx}px` }"
      role="separator"
      aria-orientation="vertical"
      @mousedown.prevent="onResizeStart"
    >
      <div
        class="bg-border group-hover:bg-primary h-12 w-0.5 rounded-full opacity-55 transition-[opacity,background-color] duration-150 group-hover:opacity-100"
        :class="{ 'bg-primary opacity-100': drawerResizing }"
      />
    </div>
  </Teleport>

  <Drawer :open="visible" swipe-direction="right" @update:open="onOpenChange">
    <!--
      Attributes the consumer puts on <SettingDrawer> (a class such as
      `storage-engine-drawer--minio`, data attributes, listeners) land on the
      panel itself. Several settings screens colour the header badge through
      global rules keyed on that class, and DataSourceEditorDialog reaches the
      body through it, so the panel is the element they have to be on.

      z-[2500] is the layer the TDesign drawer used. The settings modal these
      drawers open from sits at z-[1100], so the default z-50 would slide
      the panel in behind it. Popovers, selects and tooltips rendered inside
      the drawer are portalled to <body> and sit on the popup layer (5500)
      by default, so they open above it.
    -->
    <DrawerContent
      v-bind="passthroughAttrs"
      :class="
        cn('setting-drawer z-[2500] max-w-none rounded-none border-0 sm:max-w-none', attrClass, {
          'setting-drawer--resizing': drawerResizing,
        })
      "
      :style="{ width: effectiveWidth, transition: drawerResizing ? 'none' : undefined }"
      @pointer-down-outside="onPointerDownOutside"
    >
      <DrawerTitle class="sr-only">{{ title }}</DrawerTitle>

      <!--
        The header carries a leading icon badge and an optional subtitle next
        to the title. There is no close button: the overlay click, Escape and
        the footer's cancel button already close the drawer.
      -->
      <header class="border-border flex flex-col gap-2 border-b px-[18px] py-3.5">
        <div class="setting-drawer__header flex min-w-0 flex-1 items-center gap-2.5 py-0.5">
          <!--
            `setting-drawer__header-icon` is a hook class: Vector store, web
            search, storage and parser settings recolour this badge per
            provider from their own global style blocks. The 16px font size
            sizes a legacy TDesign icon font glyph passed through the headerIcon slot.
          -->
          <div
            v-if="$slots.headerIcon || resolvedIcon"
            class="setting-drawer__header-icon bg-primary/10 text-primary flex size-8 shrink-0 items-center justify-center rounded-[9px] text-base transition-colors duration-200"
          >
            <slot name="headerIcon">
              <component :is="resolvedIcon" class="size-4" />
            </slot>
          </div>
          <div class="flex min-w-0 flex-col gap-px">
            <div class="setting-drawer__title text-foreground truncate text-[15px] leading-[1.4] font-semibold">
              {{ title }}
            </div>
            <div
              v-if="description || $slots.subtitle"
              class="setting-drawer__subtitle text-muted-foreground text-xs leading-[1.45]"
            >
              <slot name="subtitle">{{ description }}</slot>
            </div>
          </div>
        </div>
        <div v-if="$slots['header-extra']" class="min-w-0">
          <slot name="header-extra" />
        </div>
      </header>

      <!--
        `setting-drawer__body` and `data-setting-drawer-body` are hooks:
        RuntimeQueues finds the scroll container with closest(), and
        DataSourceEditorDialog styles it from outside.
      -->
      <div
        data-setting-drawer-body
        class="setting-drawer__body motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-y-1 flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto px-[18px] py-4 motion-safe:duration-300"
      >
        <slot />
      </div>

      <footer
        v-if="!hideFooter"
        class="border-border flex w-full items-center justify-between gap-3 border-t px-[18px] py-2.5 shadow-[0_-2px_8px_rgba(15,23,42,0.04)]"
      >
        <div class="flex min-w-0 flex-1 items-center gap-2">
          <slot name="footer-left" />
        </div>
        <div class="flex shrink-0 items-center gap-3">
          <slot name="footer-right">
            <Button variant="outline" @click="handleCancel">
              {{ cancelText || t("common.cancel") }}
            </Button>
            <Button :disabled="confirmDisabled || confirmLoading" @click="handleConfirm">
              <Loader2Icon v-if="confirmLoading" class="animate-spin" />
              {{ confirmText || t("common.save") }}
            </Button>
          </slot>
        </div>
      </footer>
    </DrawerContent>
  </Drawer>
</template>

<script setup lang="ts">
import type { DrawerOpenChangeDetails } from "reka-ui";
import type { Component } from "vue";
import { computed, onMounted, onUnmounted, ref, useAttrs } from "vue";
import { useI18n } from "vue-i18n";

import { Button } from "@/components/ui/button";
import { Drawer, DrawerContent, DrawerTitle } from "@/components/ui/drawer";
import { cn } from "@/lib/utils";
import { CirclePlayIcon, Loader2Icon, SquarePenIcon } from "@lucide/vue";

interface Props {
  visible: boolean;
  title: string;
  description?: string;
  /**
   * Leading icon badge in the header: a lucide icon component, or — for
   * callers not yet migrated — one of the TDesign icon names listed in
   * `legacyIcons` below. An unknown name renders no badge.
   */
  icon?: Component | string;
  /**
   * Initial width when the user has no persisted preference. Accepts any
   * CSS length string (e.g. "560px", "40%").
   */
  width?: string;
  /**
   * Whether the drawer can be horizontally resized by dragging the visible
   * handle on its left edge.
   */
  resizable?: boolean;
  /** Min/max bounds for the drag-resize, in px. */
  minWidth?: number;
  maxWidth?: number;
  /**
   * localStorage key used to remember the user's chosen width. Set to '' to
   * disable persistence. Default key is namespaced per-consumer using the
   * drawer title.
   */
  storageKey?: string;
  confirmLoading?: boolean;
  confirmDisabled?: boolean;
  confirmText?: string;
  cancelText?: string;
  hideFooter?: boolean;
  /** When false, clicking the overlay does not dismiss the drawer. */
  closeOnOverlayClick?: boolean;
}

// Attributes are forwarded to the panel by hand (see the template): the
// component has two roots, the teleported handle and the drawer.
defineOptions({ inheritAttrs: false });

const props = withDefaults(defineProps<Props>(), {
  description: "",
  icon: undefined,
  width: "560px",
  resizable: true,
  minWidth: 480,
  maxWidth: 1200,
  storageKey: "",
  confirmLoading: false,
  confirmDisabled: false,
  confirmText: "",
  cancelText: "",
  hideFooter: false,
  closeOnOverlayClick: true,
});

const emit = defineEmits<{
  (e: "update:visible", value: boolean): void;
  (e: "confirm"): void;
  (e: "cancel"): void;
}>();

const { t } = useI18n();
const attrs = useAttrs();

// The consumer's class is merged through cn() rather than left to $attrs,
// so a width or border utility passed from outside wins over the defaults
// here instead of competing with them.
const attrClass = computed(() => attrs.class as string | string[] | Record<string, boolean> | undefined);
const passthroughAttrs = computed(() => {
  const { class: _class, ...rest } = attrs;
  return rest;
});

// The TDesign icon names the legacy callers pass, mapped onto lucide. The
// list only has to cover names actually in use; new callers pass a
// component.
const legacyIcons: Record<string, Component> = {
  "edit-1": SquarePenIcon,
  "play-circle-stroke": CirclePlayIcon,
};

const resolvedIcon = computed<Component | undefined>(() => {
  if (!props.icon) return undefined;
  if (typeof props.icon === "string") return legacyIcons[props.icon];
  return props.icon;
});

const drawerVisible = computed(() => props.visible);

// ---------- width state ----------
// Storage key derives from the drawer title so different drawers (model
// editor vs MCP service vs web search provider) get independent widths.
// Callers can override via the `storageKey` prop when titles collide.
const resolvedStorageKey = computed(() => props.storageKey || `setting-drawer:width:${props.title || "default"}`);

const clampWidth = (n: number) => Math.max(props.minWidth, Math.min(props.maxWidth, Math.round(n)));

const parseWidthToPx = (width: string) => {
  const n = parseInt(width, 10);
  return Number.isFinite(n) ? n : 560;
};

const loadStoredWidth = (): number | null => {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.localStorage.getItem(resolvedStorageKey.value);
    if (!raw) return null;
    const n = Number(raw);
    if (!Number.isFinite(n)) return null;
    return clampWidth(n);
  } catch {
    return null;
  }
};

// User's persisted width (px) wins over the prop default.
const userWidthPx = ref<number | null>(loadStoredWidth());

const effectiveWidth = computed(() => (userWidthPx.value != null ? `${userWidthPx.value}px` : props.width));

const drawerWidthPx = computed(() => userWidthPx.value ?? parseWidthToPx(props.width));

const persistWidth = (width: number) => {
  const next = clampWidth(width);
  userWidthPx.value = next;
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(resolvedStorageKey.value, String(next));
  } catch {
    // localStorage can throw in private mode / quota errors.
  }
};

// ---------- open/close ----------
// A press on the resize handle is not a press outside the drawer: the handle
// is teleported next to the panel, not into it, and the modal drawer would
// otherwise close the moment the user grabbed its edge.
function onPointerDownOutside(event: CustomEvent<{ originalEvent: PointerEvent }>) {
  const target = event.detail?.originalEvent?.target;
  if (target instanceof Element && target.closest(".setting-drawer-resize-handle")) event.preventDefault();
}

function onOpenChange(value: boolean, details?: DrawerOpenChangeDetails) {
  if (!value && !props.closeOnOverlayClick && details?.reason === "outside-press") return;
  if (!value) blurActiveElementBeforeClose();
  emit("update:visible", value);
}

// ---------- Custom drag-resize (visible handle) ----------
const drawerResizing = ref(false);

let resizeStartX = 0;
let resizeStartWidth = 0;

function onResizeStart(e: MouseEvent) {
  drawerResizing.value = true;
  resizeStartX = e.clientX;
  resizeStartWidth = drawerWidthPx.value;
  document.addEventListener("mousemove", onResizeMove);
  document.addEventListener("mouseup", onResizeEnd);
  document.body.style.cursor = "col-resize";
  document.body.style.userSelect = "none";
}

function onResizeMove(e: MouseEvent) {
  const delta = resizeStartX - e.clientX;
  userWidthPx.value = clampWidth(resizeStartWidth + delta);
}

function onResizeEnd() {
  document.removeEventListener("mousemove", onResizeMove);
  document.removeEventListener("mouseup", onResizeEnd);
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
  drawerResizing.value = false;
  persistWidth(drawerWidthPx.value);
}

function cleanupResize() {
  document.removeEventListener("mousemove", onResizeMove);
  document.removeEventListener("mouseup", onResizeEnd);
  document.body.style.cursor = "";
  document.body.style.userSelect = "";
  drawerResizing.value = false;
}

function onWindowResize() {
  if (userWidthPx.value != null) {
    userWidthPx.value = clampWidth(userWidthPx.value);
  }
}

onMounted(() => {
  window.addEventListener("resize", onWindowResize, { passive: true });
});

onUnmounted(() => {
  window.removeEventListener("resize", onWindowResize);
  cleanupResize();
});

function blurActiveElementBeforeClose() {
  // Autosizing textareas call getComputedStyle on blur/resize; if the
  // drawer is already tearing down, that node may no longer be an Element
  // and the promise rejects uncaught.
  if (document.activeElement instanceof HTMLElement) {
    document.activeElement.blur();
  }
}

const handleConfirm = () => emit("confirm");
const handleCancel = () => {
  blurActiveElementBeforeClose();
  emit("cancel");
  emit("update:visible", false);
};
</script>

<!--
  This stays CSS for two reasons utilities cannot cover. The section and
  section-title rules are a styling contract for markup the consumers put
  in the default slot (ModelEditorDialog, ModelDebugDrawer, the manual
  editor…), reached with :deep(), including the staggered entry that
  depends on :nth-child. The resize handle's entry animation needs a
  @keyframes rule that reads the travel distance from a custom property.
-->
<style scoped>
.setting-drawer__body :deep(.setting-drawer__section) {
  padding: 12px 0 16px;
  border-bottom: 1px solid var(--td-component-stroke);
  display: flex;
  flex-direction: column;
  gap: 14px;
  animation: setting-drawer-section-in 0.32s ease both;

  &:first-child {
    padding-top: 0;
    animation-delay: 0.04s;
  }

  &:nth-child(2) {
    animation-delay: 0.08s;
  }

  &:nth-child(3) {
    animation-delay: 0.12s;
  }

  &:last-child {
    border-bottom: none;
    padding-bottom: 0;
  }
}

@keyframes setting-drawer-section-in {
  from {
    opacity: 0;
    transform: translateY(6px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.setting-drawer__body :deep(.setting-drawer__section-title) {
  font-size: 13px;
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin: 0 0 4px;
  user-select: none;
  display: flex;
  align-items: center;
  gap: 8px;

  /* A subtle leading bar, instead of all-caps and letter-spacing (which
     mangle Chinese), gives every section title the same visual anchor. */
  &::before {
    content: "";
    width: 3px;
    height: 14px;
    background: var(--td-brand-color);
    border-radius: 2px;
  }
}

.setting-drawer-resize-handle {
  animation: setting-drawer-resize-handle-in 0.28s cubic-bezier(0.38, 0, 0.24, 1) both;
}

@keyframes setting-drawer-resize-handle-in {
  from {
    transform: translateX(var(--setting-drawer-travel));
  }

  to {
    transform: translateX(0);
  }
}
</style>

<!--
  Not scoped, and CSS because the element is not ours to class: the drawer
  overlay is rendered by the ui/drawer component, which takes no class for
  it. It is raised to the panel's layer through the sibling it always
  precedes, so the dim covers the settings modal the drawer opens from
  (z-[1100]) instead of sitting underneath it.
-->
<style>
[data-slot="drawer-overlay"]:has(+ .setting-drawer) {
  z-index: 2500;
}
</style>
