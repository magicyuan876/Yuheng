// @ts-nocheck
<script setup lang="ts">
import { ref, shallowRef, watch, onUnmounted, nextTick, defineAsyncComponent, computed } from "vue";
import {
  CircleAlertIcon,
  FileQuestionMarkIcon,
  Loader2Icon,
  MaximizeIcon,
  MinimizeIcon,
  Volume2Icon,
} from "@lucide/vue";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { previewKnowledgeFile } from "@/api/knowledge-base/index";
import { previewTemporaryAttachment } from "@/api/chat/temporary-attachments";
import hljs from "highlight.js";
import "highlight.js/styles/github.css";
import markedKatex from "marked-katex-extension";
import "katex/dist/katex.min.css";
import { useI18n } from "vue-i18n";
import { sanitizeHTML, safeMarkdownToHTML } from "@/utils/security";

const VueOfficePptx = defineAsyncComponent(() => import("@vue-office/pptx"));

const { t } = useI18n();

const props = defineProps<{
  knowledgeId?: string;
  sessionId?: string;
  attachmentId?: string;
  fileType: string;
  fileName: string;
  active: boolean;
  fillHeight?: boolean;
}>();

const loading = ref(false);
const error = ref("");
const previewType = ref<
  "pdf" | "docx" | "image" | "excel" | "text" | "markdown" | "pptx" | "audio" | "video" | "unsupported"
>("unsupported");
const blobUrl = ref("");
const textContent = ref("");
const highlightedCode = ref("");
const markdownHtml = ref("");
const excelHtml = ref("");
const pptxData = shallowRef<ArrayBuffer | null>(null);
const docxContainer = ref<HTMLElement | null>(null);
const imageNaturalWidth = ref(0);
const imageNaturalHeight = ref(0);
let loadedForId = "";

const isFullscreen = ref(false);

function toggleFullscreen() {
  isFullscreen.value = !isFullscreen.value;
  if (isFullscreen.value) {
    document.body.style.overflow = "hidden";
  } else {
    document.body.style.overflow = "";
  }
}

const fileTypeMap: Record<string, typeof previewType.value> = {};
["pdf"].forEach((t) => (fileTypeMap[t] = "pdf"));
["docx"].forEach((t) => (fileTypeMap[t] = "docx"));
["pptx", "ppt"].forEach((t) => (fileTypeMap[t] = "pptx"));
["jpg", "jpeg", "png", "gif", "bmp", "webp", "tiff", "svg"].forEach((t) => (fileTypeMap[t] = "image"));
["xlsx", "xls", "csv"].forEach((t) => (fileTypeMap[t] = "excel"));
["md", "markdown"].forEach((t) => (fileTypeMap[t] = "markdown"));
[
  "txt",
  "json",
  "xml",
  "html",
  "css",
  "js",
  "ts",
  "py",
  "java",
  "go",
  "cpp",
  "c",
  "h",
  "sh",
  "yaml",
  "yml",
  "ini",
  "conf",
  "log",
  "sql",
  "rs",
  "rb",
  "php",
  "swift",
  "kt",
  "scala",
  "r",
  "lua",
  "pl",
  "toml",
].forEach((t) => (fileTypeMap[t] = "text"));
["mp3", "wav", "m4a", "flac", "ogg"].forEach((t) => (fileTypeMap[t] = "audio"));
["mp4", "mov", "avi", "mkv", "webm", "wmv", "flv", "m4v"].forEach((t) => (fileTypeMap[t] = "video"));

const mimeTypeMap: Record<string, string> = {
  pdf: "application/pdf",
  docx: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  doc: "application/msword",
  pptx: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
  ppt: "application/vnd.ms-powerpoint",
  xlsx: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  xls: "application/vnd.ms-excel",
  csv: "text/csv",
  jpg: "image/jpeg",
  jpeg: "image/jpeg",
  png: "image/png",
  gif: "image/gif",
  bmp: "image/bmp",
  webp: "image/webp",
  tiff: "image/tiff",
  svg: "image/svg+xml",
  txt: "text/plain",
  md: "text/markdown",
  markdown: "text/markdown",
  json: "application/json",
  xml: "application/xml",
  html: "text/html",
  css: "text/css",
  js: "text/javascript",
  ts: "text/typescript",
  py: "text/x-python",
  java: "text/x-java",
  go: "text/x-go",
  mp3: "audio/mpeg",
  wav: "audio/wav",
  m4a: "audio/mp4",
  flac: "audio/flac",
  ogg: "audio/ogg",
  mp4: "video/mp4",
  m4v: "video/mp4",
  mov: "video/quicktime",
  webm: "video/webm",
  mkv: "video/x-matroska",
  avi: "video/x-msvideo",
  wmv: "video/x-ms-wmv",
  flv: "video/x-flv",
};

function getMimeType(ft: string): string {
  return mimeTypeMap[ft?.toLowerCase()] || "application/octet-stream";
}

function ensureBlobType(blob: Blob, ft: string): Blob {
  const expected = getMimeType(ft);
  if (blob.type === expected) return blob;
  return new Blob([blob], { type: expected });
}

const langMap: Record<string, string> = {
  js: "javascript",
  ts: "typescript",
  py: "python",
  rb: "ruby",
  sh: "bash",
  yml: "yaml",
  md: "markdown",
  rs: "rust",
  kt: "kotlin",
  pl: "perl",
  conf: "ini",
  log: "plaintext",
};

function resolvePreviewType(ft: string): typeof previewType.value {
  return fileTypeMap[ft?.toLowerCase()] || "unsupported";
}

function getHighlightLang(ft: string): string {
  const lower = ft?.toLowerCase() || "";
  return langMap[lower] || lower;
}

const preprocessMathDelimiters = (rawText: string): string => {
  if (!rawText || typeof rawText !== "string") {
    return "";
  }
  return rawText.replace(/\\\[([\s\S]*?)\\\]/g, "$$$$$1$$$$").replace(/\\\(([\s\S]*?)\\\)/g, "$$$1$$");
};

async function renderDocx(blob: Blob) {
  const { renderAsync } = await import("docx-preview");
  if (docxContainer.value) {
    docxContainer.value.innerHTML = "";
    await renderAsync(blob, docxContainer.value, undefined, {
      className: "docx-preview-wrapper",
      inWrapper: true,
      ignoreWidth: false,
      ignoreHeight: false,
      ignoreFonts: false,
      breakPages: true,
      ignoreLastRenderedPageBreak: true,
      experimental: false,
      trimXmlDeclaration: true,
      useBase64URL: true,
    });
  }
}

function isValidUTF8(bytes: Uint8Array): boolean {
  for (let i = 0; i < bytes.length;) {
    const b = bytes[i];
    let remaining = 0;
    if (b <= 0x7f) {
      remaining = 0;
    } else if ((b & 0xe0) === 0xc0) {
      remaining = 1;
    } else if ((b & 0xf0) === 0xe0) {
      remaining = 2;
    } else if ((b & 0xf8) === 0xf0) {
      remaining = 3;
    } else {
      return false;
    }
    if (i + remaining >= bytes.length) return false;
    for (let j = 1; j <= remaining; j++) {
      if ((bytes[i + j] & 0xc0) !== 0x80) return false;
    }
    i += 1 + remaining;
  }
  return true;
}

function decodeCSVBlob(arrayBuffer: ArrayBuffer): string {
  const bytes = new Uint8Array(arrayBuffer);
  if (bytes[0] === 0xef && bytes[1] === 0xbb && bytes[2] === 0xbf) {
    return new TextDecoder("utf-8").decode(bytes);
  }
  if (isValidUTF8(bytes)) {
    return new TextDecoder("utf-8").decode(bytes);
  }
  return new TextDecoder("gbk").decode(bytes);
}

async function renderExcel(blob: Blob, fileType?: string) {
  const XLSX = await import("xlsx");
  const arrayBuffer = await blob.arrayBuffer();

  let workbook;
  if (fileType?.toLowerCase() === "csv") {
    const csvText = decodeCSVBlob(arrayBuffer);
    workbook = XLSX.read(csvText, { type: "string" });
  } else {
    workbook = XLSX.read(arrayBuffer, { type: "array" });
  }

  let html = "";
  workbook.SheetNames.forEach((name, sheetIdx) => {
    const sheet = workbook.Sheets[name];
    const sheetHtml = XLSX.utils.sheet_to_html(sheet, { id: `sheet-${sheetIdx}` });
    html += `<div class="excel-sheet">`;
    if (workbook.SheetNames.length > 1) {
      html += `<div class="excel-sheet-name">${name}</div>`;
    }
    html += sheetHtml;
    html += `</div>`;
  });
  excelHtml.value = sanitizeHTML(html);
}

async function renderText(blob: Blob, fileType: string) {
  const text = await blob.text();
  textContent.value = text;

  const lang = getHighlightLang(fileType);
  if (lang && hljs.getLanguage(lang)) {
    try {
      highlightedCode.value = hljs.highlight(text, { language: lang }).value;
      return;
    } catch {
      /* fallthrough */
    }
  }
  const auto = hljs.highlightAuto(text);
  highlightedCode.value = auto.value;
}

async function renderMarkdown(blob: Blob) {
  const { marked } = await import("marked");
  const text = await blob.text();

  // 校验文本内容是否有效
  if (!text || typeof text !== "string") {
    markdownHtml.value =
      '<p style="color: var(--td-text-color-disabled); text-align: center; padding: 20px;">文档内容为空</p>';
    return;
  }

  marked.use({
    breaks: true,
    gfm: true,
  });
  marked.use(markedKatex({ throwOnError: false, nonStandard: true }));
  const renderer = new marked.Renderer();
  renderer.code = function ({ text, lang }) {
    // 空值校验：防止 text 为 undefined 或 null
    if (!text || typeof text !== "string") {
      text = "";
    }

    let highlighted = "";
    if (lang && hljs.getLanguage(lang)) {
      try {
        highlighted = hljs.highlight(text, { language: lang }).value;
      } catch {
        highlighted = hljs.highlightAuto(text).value;
      }
    } else {
      highlighted = hljs.highlightAuto(text).value;
    }
    return `<pre><code class="hljs">${highlighted}</code></pre>`;
  };
  const mathSafeText = preprocessMathDelimiters(text);
  const safeText = safeMarkdownToHTML(mathSafeText);
  // Keep this renderer local. `marked.use` mutates a shared singleton and
  // would otherwise inherit renderers installed by the chunk-content view.
  const rawHtml = marked.parse(safeText, { renderer }) as string;
  markdownHtml.value = sanitizeHTML(rawHtml);
}

function onImageLoad(e: Event) {
  const img = e.target as HTMLImageElement;
  imageNaturalWidth.value = img.naturalWidth;
  imageNaturalHeight.value = img.naturalHeight;
}

function getPreviewSourceKey(): string {
  if (props.knowledgeId) return `knowledge:${props.knowledgeId}`;
  if (props.sessionId && props.attachmentId) return `attachment:${props.sessionId}:${props.attachmentId}`;
  return "";
}

async function fetchPreviewBlob(): Promise<Blob> {
  if (props.knowledgeId) {
    return previewKnowledgeFile(props.knowledgeId);
  }
  if (props.sessionId && props.attachmentId) {
    return previewTemporaryAttachment(props.sessionId, props.attachmentId);
  }
  throw new Error("Missing preview source");
}

async function loadPreview() {
  const sourceKey = getPreviewSourceKey();
  const ft = props.fileType;
  if (!sourceKey || !ft) return;
  if (loadedForId === sourceKey) return;

  cleanup();
  loading.value = true;
  error.value = "";
  previewType.value = resolvePreviewType(ft);

  if (previewType.value === "unsupported") {
    loading.value = false;
    return;
  }

  try {
    const rawBlob = await fetchPreviewBlob();
    const blob = ensureBlobType(rawBlob, ft);
    loadedForId = sourceKey;

    loading.value = false;
    await nextTick();

    switch (previewType.value) {
      case "pdf": {
        blobUrl.value = URL.createObjectURL(blob);
        break;
      }
      case "image": {
        blobUrl.value = URL.createObjectURL(blob);
        break;
      }
      case "docx": {
        await renderDocx(blob);
        break;
      }
      case "excel": {
        await renderExcel(blob, ft);
        break;
      }
      case "text": {
        await renderText(blob, ft);
        break;
      }
      case "markdown": {
        await renderMarkdown(blob);
        break;
      }
      case "pptx": {
        pptxData.value = await blob.arrayBuffer();
        break;
      }
      case "audio": {
        blobUrl.value = URL.createObjectURL(blob);
        break;
      }
      case "video": {
        blobUrl.value = URL.createObjectURL(blob);
        break;
      }
    }
  } catch (err: any) {
    console.error("Document preview failed:", err);
    error.value = err?.message || t("preview.loadFailed");
  } finally {
    loading.value = false;
  }
}

function cleanup() {
  if (blobUrl.value) {
    URL.revokeObjectURL(blobUrl.value);
    blobUrl.value = "";
  }
  textContent.value = "";
  highlightedCode.value = "";
  markdownHtml.value = "";
  excelHtml.value = "";
  pptxData.value = null;
  imageNaturalWidth.value = 0;
  imageNaturalHeight.value = 0;
  loadedForId = "";
  if (docxContainer.value) {
    docxContainer.value.innerHTML = "";
  }
}

watch(
  () => [props.active, props.knowledgeId, props.sessionId, props.attachmentId],
  ([active]) => {
    if (active && getPreviewSourceKey()) {
      loadPreview();
    }
  },
  { immediate: true },
);

// ── Layout classes ──
//
// Each preview pane has three layouts: the default one, the one it takes
// when the parent asks it to fill its height (the chat attachment drawer),
// and the fullscreen one. The last two can be on at once. In the Less this
// replaced, the fill-height rules carried the more specific selectors, so
// where both states set the same property the fill-height value won, and
// the fullscreen value beat the default. Tailwind resolves conflicting
// utilities by its own stylesheet order rather than by class order, so the
// precedence is written out here: every property is chosen once, through
// the same fill > fullscreen > default ladder.

// The capped height every bordered pane shares. <html> carries a `zoom`
// multiplier for font-size control, so 100vh is evaluated against the
// unscaled viewport and the resulting max-height may exceed the real
// viewport by the zoom factor (at most 12.5% at "large"). That produces an
// extra bit of scroll inside the non-fullscreen preview, which is acceptable
// for document reading and not worth inverse-scaling for.
const PANE_MAX_H = "max-h-[calc(100vh-200px)]";

// The bordered, scrolling card the markdown, docx, excel and code panes sit
// in, minus the max-height, which each of them picks from the ladder.
const PANE_BOX = "overflow-auto rounded-[6px] border border-solid border-[var(--td-component-stroke)] bg-card";

// What fill-height does to every pane: take the remaining column height.
const FILL_PANE = "min-h-0 max-h-none flex-1";

// The max-height of a bordered pane: uncapped when filling, the full
// container in fullscreen, and the viewport-derived cap otherwise.
const paneMaxH = computed(() => {
  if (props.fillHeight) return "max-h-none";
  if (isFullscreen.value) return "max-h-full";
  return PANE_MAX_H;
});

// A pane wrapper that only needs fill-height's flex sizing.
const fillPane = computed(() => (props.fillHeight ? FILL_PANE : ""));

const rootClass = computed(() => [
  // `document-preview` is a hook class: ChatAttachmentPreviewDrawer targets
  // it to disable pointer events while the drawer is being resized.
  "document-preview",
  props.fillHeight ? "flex h-full min-h-0 flex-col" : "min-h-[200px]",
  // Children use height: 100% rather than 100vh in fullscreen, because the
  // `zoom` on <html> would scale a 100vh past the screen; the container is
  // inset 0 on all sides, so 100% resolves to the true viewport height.
  isFullscreen.value ? "fixed inset-0 z-[2001] overflow-y-auto bg-card p-0" : "relative",
]);

const toolbarClass = computed(() =>
  isFullscreen.value ? "fixed top-3 right-8 z-[2002]" : "absolute top-2 right-6 z-10",
);

const pdfClass = computed(() => {
  let height = "h-[calc(100vh-200px)]";
  if (props.fillHeight) height = "h-auto";
  else if (isFullscreen.value) height = "h-full";
  return ["w-full", height, props.fillHeight ? FILL_PANE : "min-h-[500px]"];
});

const imagePaneClass = computed(() => [
  "flex justify-center py-5",
  isFullscreen.value ? "items-center" : "",
  props.fillHeight ? FILL_PANE : isFullscreen.value ? "min-h-full" : "",
]);

const imageClass = computed(() => {
  if (props.fillHeight) return "max-h-full";
  if (isFullscreen.value) return "max-h-[calc(100%-80px)]";
  return "max-h-[calc(100vh-280px)]";
});

const docxPaneClass = computed(() => [
  props.fillHeight ? FILL_PANE : "",
  isFullscreen.value ? "h-full" : "",
  props.fillHeight || isFullscreen.value ? "flex flex-col" : "",
]);

const docxContainerClass = computed(() => {
  if (props.fillHeight) return "h-auto min-h-0 max-h-none flex-1";
  if (isFullscreen.value) return "h-full max-h-full flex-1";
  return PANE_MAX_H;
});

const pptxPaneClass = computed(() => {
  let minH = "min-h-[500px]";
  if (props.fillHeight) minH = "min-h-0";
  else if (isFullscreen.value) minH = "min-h-full";
  return [
    "rounded-[6px] bg-card",
    props.fillHeight ? "max-h-none flex-1" : PANE_MAX_H,
    minH,
    // Fill-height keeps the pane scrolling even in fullscreen; fullscreen
    // alone lets the slides run the full length of the page.
    !props.fillHeight && isFullscreen.value ? "overflow-visible" : "overflow-auto",
    isFullscreen.value ? "h-auto border-0" : "border border-solid border-[var(--td-component-stroke)]",
  ];
});

onUnmounted(() => {
  document.body.style.overflow = "";
  cleanup();
});
</script>

<template>
  <div :class="rootClass">
    <!-- Toolbar -->
    <div
      v-if="!loading && !error && previewType !== 'unsupported'"
      :class="toolbarClass"
      class="border-border bg-card flex items-center gap-2 rounded-[var(--td-radius-default)] border border-solid p-1 opacity-60 shadow-[var(--td-shadow-1)] transition-opacity duration-200 hover:opacity-100"
    >
      <Tooltip>
        <TooltipTrigger as-child>
          <Button
            variant="ghost"
            size="icon"
            :aria-label="isFullscreen ? $t('preview.exitFullscreen') : $t('preview.fullscreen')"
            @click="toggleFullscreen"
          >
            <MinimizeIcon v-if="isFullscreen" />
            <MaximizeIcon v-else />
          </Button>
        </TooltipTrigger>
        <!-- Above the fullscreen layer (z 2001/2002), which a default z-50 would sit under. -->
        <TooltipContent side="bottom" class="z-[2003]">
          {{ isFullscreen ? $t("preview.exitFullscreen") : $t("preview.fullscreen") }}
        </TooltipContent>
      </Tooltip>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="flex flex-col items-center justify-center gap-4 px-5 py-[60px]" :class="fillPane">
      <Loader2Icon class="text-primary size-5 animate-spin" />
      <span class="text-placeholder text-sm">{{ $t("preview.loading") }}</span>
    </div>

    <!-- Error -->
    <div
      v-else-if="error"
      class="text-destructive flex flex-col items-center justify-center gap-3 px-5 py-[60px]"
      :class="fillPane"
    >
      <CircleAlertIcon class="size-12" />
      <p class="text-muted-foreground m-0 text-sm">{{ error }}</p>
      <Button
        size="sm"
        @click="
          loadedForId = '';
          loadPreview();
        "
      >
        {{ $t("preview.retry") }}
      </Button>
    </div>

    <!-- Unsupported -->
    <div
      v-else-if="previewType === 'unsupported'"
      class="flex flex-col items-center justify-center gap-3 px-5 py-[60px] text-[var(--td-text-color-disabled)]"
      :class="fillPane"
    >
      <FileQuestionMarkIcon class="size-12" />
      <p class="text-muted-foreground m-0 text-sm">{{ $t("preview.unsupported") }}</p>
      <p class="text-placeholder m-0 text-xs">{{ $t("preview.unsupportedHint") }}</p>
    </div>

    <!-- PDF -->
    <div v-else-if="previewType === 'pdf' && blobUrl" :class="pdfClass">
      <iframe :src="blobUrl" class="size-full rounded-[6px] border-0" />
    </div>

    <!-- Image -->
    <div v-else-if="previewType === 'image' && blobUrl" :class="imagePaneClass">
      <div class="flex flex-col items-center gap-2">
        <img
          :src="blobUrl"
          :alt="fileName"
          class="max-w-full rounded-[6px] object-contain shadow-[0_2px_12px_rgba(7,192,95,0.08)]"
          :class="imageClass"
          @load="onImageLoad"
        />
        <div v-if="imageNaturalWidth" class="text-placeholder text-xs">
          {{ imageNaturalWidth }} × {{ imageNaturalHeight }} px
        </div>
      </div>
    </div>

    <!-- DOCX -->
    <div v-else-if="previewType === 'docx'" :class="docxPaneClass">
      <div ref="docxContainer" :class="[PANE_BOX, docxContainerClass]" />
    </div>

    <!-- PPTX -->
    <div v-else-if="previewType === 'pptx' && pptxData" class="preview-pptx" :class="pptxPaneClass">
      <vue-office-pptx
        :src="pptxData"
        @rendered="() => {}"
        @error="
          (e: any) => {
            error = e?.message || $t('preview.loadFailed');
          }
        "
      />
    </div>

    <!-- Excel -->
    <div v-else-if="previewType === 'excel' && excelHtml" :class="fillPane">
      <div :class="[PANE_BOX, paneMaxH]" v-html="excelHtml" />
    </div>

    <!-- Markdown -->
    <div
      v-else-if="previewType === 'markdown' && markdownHtml"
      class="px-6 py-5"
      :class="[PANE_BOX, paneMaxH, fillPane]"
    >
      <div class="markdown-body" v-html="markdownHtml" />
    </div>

    <!-- Text / Code -->
    <div v-else-if="previewType === 'text' && highlightedCode" :class="fillPane">
      <pre
        class="code-preview m-0 p-4 text-[13px] leading-[1.6]"
        :class="[PANE_BOX, paneMaxH]"
      ><code class="hljs" v-html="highlightedCode"></code></pre>
    </div>

    <!-- Audio -->
    <div v-else-if="previewType === 'audio' && blobUrl" class="flex justify-center px-5 py-10" :class="fillPane">
      <div class="text-muted-foreground flex flex-col items-center gap-4">
        <Volume2Icon class="size-12" />
        <p class="text-foreground m-0 text-sm">{{ fileName }}</p>
        <audio controls :src="blobUrl" class="w-full max-w-[480px]">
          {{ $t("preview.audioNotSupported") }}
        </audio>
      </div>
    </div>

    <!-- Video -->
    <div
      v-else-if="previewType === 'video' && blobUrl"
      class="bg-muted flex items-center justify-center rounded-[6px] p-4"
      :class="fillPane"
    >
      <video controls :src="blobUrl" class="max-h-[70vh] w-full rounded-[6px] bg-black">
        {{ $t("preview.videoNotSupported") }}
      </video>
    </div>
  </div>
</template>

<style scoped>
/*
 * What stays CSS here is the markup this component does not write: the
 * HTML that marked, docx-preview, SheetJS and vue-office inject, reached
 * through :deep(). One rule targets our own <code> element, because
 * highlight.js's github.css (unlayered) styles `.hljs` and would outrank a
 * utility class, which lives in a cascade layer.
 */
/*
 * Inside a drawer panel (a shadcn part, data-slot) the element resets in
 * tailwind.css reach the injected markup too and turn its images and SVGs
 * into blocks; the rendered documents want them inline, as the browser draws
 * them. The rule sits in the reset's own base layer, so every other rule
 * here and the libraries' own stylesheets still override it.
 */
@layer base {
  :deep(.markdown-body) :where(img, svg, video),
  :deep(.docx-preview-wrapper) :where(img, svg, video),
  :deep(.excel-sheet) :where(img, svg, video) {
    display: revert-layer;
    vertical-align: revert-layer;
  }
}

.code-preview code {
  white-space: pre;
  word-wrap: normal;
  display: block;
  background: transparent;
}

.preview-pptx :deep(.pptx-preview-wrapper) {
  height: auto !important;
  overflow-y: visible !important;
}

:deep(.markdown-body) {
  font-size: 14px;
  line-height: 1.7;
  color: var(--td-text-color-primary);
  word-break: break-word;

  h1,
  h2,
  h3,
  h4,
  h5,
  h6 {
    margin-top: 20px;
    margin-bottom: 10px;
    font-weight: 600;
    line-height: 1.4;
  }
  h1 {
    font-size: 24px;
    border-bottom: 1px solid var(--td-component-stroke);
    padding-bottom: 8px;
  }
  h2 {
    font-size: 20px;
    border-bottom: 1px solid var(--td-component-stroke);
    padding-bottom: 6px;
  }
  h3 {
    font-size: 17px;
  }

  p {
    margin: 8px 0;
  }
  blockquote {
    margin: 12px 0;
    padding: 8px 16px;
    border-left: 4px solid var(--td-brand-color);
    background: var(--td-bg-color-container);
    color: var(--td-text-color-secondary);
  }
  ul,
  ol {
    padding-left: 24px;
    margin: 8px 0;
  }
  li {
    margin: 4px 0;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
    margin: 12px 0;
  }
  th,
  td {
    border: 1px solid var(--td-component-stroke);
    padding: 6px 12px;
    text-align: left;
  }
  th {
    background: var(--td-success-color-light);
    font-weight: 600;
    color: var(--td-text-color-primary);
  }
  tr:hover td {
    background: var(--td-success-color-light);
    transition: all 0.2s ease;
  }

  pre {
    margin: 12px 0;
    padding: 14px;
    background: var(--td-bg-color-container);
    border-radius: 6px;
    overflow: auto;
    font-size: 13px;
    line-height: 1.5;

    code {
      background: transparent;
      padding: 0;
    }
  }
  code {
    background: var(--td-bg-color-secondarycontainer);
    padding: 2px 6px;
    border-radius: 3px;
    font-size: 0.9em;
  }
  img {
    max-width: 100%;
    border-radius: 4px;
  }
  hr {
    border: none;
    border-top: 1px solid var(--td-component-stroke);
    margin: 20px 0;
  }
  a {
    color: var(--td-brand-color);
    text-decoration: none;

    &:hover {
      color: var(--td-brand-color-active);
      text-decoration: underline;
    }
  }
  strong {
    font-weight: 600;
  }
}

:deep(.docx-preview-wrapper) {
  padding: 20px;
  max-width: 100%;
  width: 100%;
  box-sizing: border-box;
  /* 如果内容过宽，允许水平滚动而不是溢出 */
  overflow-x: auto;
}

/*
 * The rules for the wrapper's descendants are written out rather than nested:
 * Vue's scoped compiler drops a bare `*` nested inside a :deep() rule and
 * emits a rule with no selector, so the width constraint below never applied.
 */

/* 约束所有子元素的宽度 */
:deep(.docx-preview-wrapper *) {
  max-width: 100%;
  box-sizing: border-box;
}

/* 特别处理表格 */
:deep(.docx-preview-wrapper table) {
  width: 100%;
  table-layout: auto;
  word-wrap: break-word;
}

/* 处理图片 */
:deep(.docx-preview-wrapper img) {
  max-width: 100%;
  height: auto;
}

/* 处理可能的固定宽度元素 */
:deep(.docx-preview-wrapper [style*="width"]) {
  max-width: 100% !important;
}

:deep(.vue-office-pptx) {
  width: 100%;
  min-height: 100%;
}

:deep(.vue-office-pptx-main) {
  width: 100%;
  min-height: 100%;
}

:deep(.excel-sheet) {
  padding: 0;

  .excel-sheet-name {
    position: sticky;
    top: 0;
    background: var(--td-success-color-light);
    padding: 8px 16px;
    font-weight: 600;
    font-size: 13px;
    color: var(--td-text-color-primary);
    border-bottom: 1px solid var(--td-component-stroke);
    z-index: 1;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }
  th,
  td {
    border: 1px solid var(--td-component-stroke);
    padding: 6px 12px;
    text-align: left;
    white-space: nowrap;
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  th {
    background: var(--td-success-color-light);
    font-weight: 600;
    color: var(--td-text-color-primary);
  }
  tr:hover td {
    background: var(--td-success-color-light);
    transition: all 0.2s ease;
  }
}
</style>

<!--
  highlight.js github.css is a light theme imported globally; its token
  colors (notably the base #24292e) become unreadable on the dark container
  background used in dark mode. Remap the palette to the github-dark colors
  when dark mode is active. Non-scoped on purpose so it covers every hljs
  block (txt/code preview, markdown code fences), and deliberately outside
  any cascade layer: github.css is unlayered, and a layered rule would lose
  to it whatever its specificity.
-->
<style>
html[theme-mode="dark"] {
  .hljs {
    color: #c9d1d9;
    background: transparent;
  }
  .hljs-doctag,
  .hljs-keyword,
  .hljs-meta .hljs-keyword,
  .hljs-template-tag,
  .hljs-template-variable,
  .hljs-type,
  .hljs-variable.language_ {
    color: #ff7b72;
  }
  .hljs-title,
  .hljs-title.class_,
  .hljs-title.class_.inherited__,
  .hljs-title.function_ {
    color: #d2a8ff;
  }
  .hljs-attr,
  .hljs-attribute,
  .hljs-literal,
  .hljs-meta,
  .hljs-number,
  .hljs-operator,
  .hljs-variable,
  .hljs-selector-attr,
  .hljs-selector-class,
  .hljs-selector-id {
    color: #79c0ff;
  }
  .hljs-regexp,
  .hljs-string,
  .hljs-meta .hljs-string {
    color: #a5d6ff;
  }
  .hljs-built_in,
  .hljs-symbol {
    color: #ffa657;
  }
  .hljs-comment,
  .hljs-code,
  .hljs-formula {
    color: #8b949e;
  }
  .hljs-name,
  .hljs-quote,
  .hljs-selector-tag,
  .hljs-selector-pseudo {
    color: #7ee787;
  }
  .hljs-subst {
    color: #c9d1d9;
  }
  .hljs-section {
    color: #1f6feb;
    font-weight: bold;
  }
  .hljs-bullet {
    color: #f2cc60;
  }
  .hljs-emphasis {
    color: #c9d1d9;
    font-style: italic;
  }
  .hljs-strong {
    color: #c9d1d9;
    font-weight: bold;
  }
  .hljs-addition {
    color: #aff5b4;
    background-color: #033a16;
  }
  .hljs-deletion {
    color: #ffdcd7;
    background-color: #67060c;
  }
}
</style>
