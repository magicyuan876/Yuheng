import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import { defineComponent, h } from "vue";
import enUS from "@/i18n/locales/en-US";
import { TooltipProvider } from "@/components/ui/tooltip";

// The file preview used by the document drawer and the chat attachment
// drawer. These tests mount it with the preview API mocked, and pin the
// states it shows and the layout ladder (default, fill-height, fullscreen)
// that replaced its Less block.

const previewKnowledgeFile = vi.fn<(id: string) => Promise<Blob>>();

vi.mock("@/api/knowledge-base/index", () => ({
  previewKnowledgeFile: (id: string) => previewKnowledgeFile(id),
}));
vi.mock("@/api/chat/temporary-attachments", () => ({
  previewTemporaryAttachment: vi.fn(),
}));

const { default: DocumentPreview } = await import("./document-preview.vue");

const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });

let wrapper: VueWrapper | undefined;

interface PreviewProps {
  knowledgeId: string;
  fileType: string;
  fileName: string;
  active: boolean;
  fillHeight?: boolean;
}

function mountPreview(props: PreviewProps) {
  const Host = defineComponent({
    setup() {
      return () => h(TooltipProvider, () => h(DocumentPreview, props));
    },
  });
  wrapper = mount(Host, { attachTo: document.body, global: { plugins: [i18n] } });
  return wrapper;
}

function root(w: VueWrapper) {
  return w.get(".document-preview");
}

beforeEach(() => {
  // happy-dom has no blob URLs; the component only needs a string back.
  URL.createObjectURL = vi.fn(() => "blob:preview");
  URL.revokeObjectURL = vi.fn();
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  previewKnowledgeFile.mockReset();
  document.body.style.overflow = "";
});

test("an unsupported type says so without fetching or showing the toolbar", async () => {
  const w = mountPreview({ knowledgeId: "k1", fileType: "exe", fileName: "setup.exe", active: true });
  await flushPromises();

  assert.equal(previewKnowledgeFile.mock.calls.length, 0);
  assert.match(w.text(), new RegExp(enUS.preview.unsupported));
  assert.equal(w.find("button").exists(), false);
});

test("a failed fetch shows the error and retries on demand", async () => {
  previewKnowledgeFile.mockRejectedValueOnce(new Error("boom"));
  const w = mountPreview({ knowledgeId: "k1", fileType: "png", fileName: "a.png", active: true });
  await flushPromises();

  assert.match(w.text(), /boom/);
  previewKnowledgeFile.mockResolvedValueOnce(new Blob(["x"], { type: "image/png" }));
  const retry = w.findAll("button").find((b) => b.text() === enUS.preview.retry);
  assert.ok(retry, "the retry button is shown");
  await retry.trigger("click");
  await flushPromises();

  assert.equal(previewKnowledgeFile.mock.calls.length, 2);
  assert.equal(w.find("img").attributes("src"), "blob:preview");
});

test("the fullscreen button pins the preview over the page and back", async () => {
  previewKnowledgeFile.mockResolvedValue(new Blob(["x"], { type: "image/png" }));
  const w = mountPreview({ knowledgeId: "k1", fileType: "png", fileName: "a.png", active: true });
  await flushPromises();

  assert.ok(root(w).classes().includes("relative"));
  assert.ok(root(w).classes().includes("min-h-[200px]"));
  const toggle = w.get(`button[aria-label="${enUS.preview.fullscreen}"]`);
  await toggle.trigger("click");

  assert.ok(root(w).classes().includes("fixed"));
  assert.ok(root(w).classes().includes("inset-0"));
  assert.equal(document.body.style.overflow, "hidden");
  // In fullscreen the image may take the container height, less the toolbar room.
  assert.ok(w.get("img").classes().includes("max-h-[calc(100%-80px)]"));

  await w.get(`button[aria-label="${enUS.preview.exitFullscreen}"]`).trigger("click");
  assert.ok(root(w).classes().includes("relative"));
  assert.equal(document.body.style.overflow, "");
});

test("fill-height makes the preview a flex column that its pane fills", async () => {
  previewKnowledgeFile.mockResolvedValue(new Blob(["x"], { type: "image/png" }));
  const w = mountPreview({ knowledgeId: "k1", fileType: "png", fileName: "a.png", active: true, fillHeight: true });
  await flushPromises();

  const classes = root(w).classes();
  for (const c of ["flex", "h-full", "min-h-0", "flex-col"]) assert.ok(classes.includes(c), c);
  assert.ok(!classes.includes("min-h-[200px]"));
  const pane = w.get("img").element.parentElement?.parentElement;
  assert.ok(pane?.classList.contains("flex-1"));
  // Fill-height outranks fullscreen: the image stays capped at the pane.
  await w.get(`button[aria-label="${enUS.preview.fullscreen}"]`).trigger("click");
  assert.ok(w.get("img").classes().includes("max-h-full"));
});
