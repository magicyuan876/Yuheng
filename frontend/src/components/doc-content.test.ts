import assert from "node:assert/strict";
import { afterEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import { defineComponent, h, nextTick, reactive } from "vue";
import enUS from "@/i18n/locales/en-US";
import { TooltipProvider } from "@/components/ui/tooltip";

// The knowledge document drawer. These tests mount it open, with the API,
// the auth store and the heavy renderers mocked, and check the parts its
// move from t-drawer / t-popup / t-pagination onto Reka could break: the
// drawer and its close paths, the metadata editor, a chunk toolbar popover
// and the chunk pager.

vi.mock("@/api/knowledge-base/index", () => ({
  KNOWLEDGE_CHUNK_PAGE_SIZE: 25,
  downKnowledgeDetails: vi.fn(),
  deleteGeneratedQuestion: vi.fn(),
  getChunkByIdOnly: vi.fn(),
  previewKnowledgeFile: vi.fn(),
  updateDocumentChunk: vi.fn(),
  listChunkRevisions: vi.fn(async () => ({ data: [] })),
  revertDocumentChunk: vi.fn(),
  updateKnowledgeMetadata: vi.fn(),
  updateKnowledgeSummary: vi.fn(),
  regenerateKnowledgeSummary: vi.fn(),
  upsertGeneratedQuestion: vi.fn(),
  regenerateGeneratedQuestions: vi.fn(),
  getKnowledgeDetails: vi.fn(async () => ({ data: {} })),
}));
vi.mock("@/stores/auth", () => ({ useAuthStore: () => ({ hasRole: () => true }) }));
vi.mock("mermaid", () => ({ default: { initialize: vi.fn(), render: vi.fn() } }));
vi.mock("@/utils/mermaidViewer", () => ({ openMermaidFullscreen: vi.fn() }));
vi.mock("@/components/knowledge-processing-timeline.vue", () => ({
  default: defineComponent({ name: "KnowledgeProcessingTimeline", render: () => null }),
}));

const { default: DocContent } = await import("./doc-content.vue");

const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });

let wrapper: VueWrapper | undefined;

function makeDetails(total = 2) {
  return reactive({
    id: "k1",
    title: "Handbook",
    type: "manual",
    time: "2026-09-01 10:00",
    total,
    chunkLoading: false,
    custom_metadata: { team: "docs" },
    md: [
      { id: "c1", content: "First chunk", is_enabled: true, start_at: 0, end_at: 11 },
      { id: "c2", content: "Second chunk", is_enabled: false, start_at: 12, end_at: 24 },
    ],
  });
}

function mountDrawer(details = makeDetails()) {
  const events: { closeDoc: unknown[]; getDoc: unknown[] } = { closeDoc: [], getDoc: [] };
  const Host = defineComponent({
    setup() {
      return () =>
        h(TooltipProvider, () =>
          h(DocContent, {
            visible: true,
            details,
            canEditKB: true,
            canDownloadKB: true,
            onCloseDoc: (v: unknown) => events.closeDoc.push(v),
            onGetDoc: (v: unknown) => events.getDoc.push(v),
          }),
        );
    },
  });
  wrapper = mount(Host, { attachTo: document.body, global: { plugins: [i18n] } });
  return { wrapper, events };
}

// The drawer is portalled to <body>, so queries go through the document.
function panel() {
  const el = document.body.querySelector('[data-slot="drawer-content"]');
  assert.ok(el, "the drawer panel is rendered");
  return el as HTMLElement;
}

function buttonByLabel(label: string, root: ParentNode = document.body) {
  const el = root.querySelector(`button[aria-label="${label}"]`);
  assert.ok(el, `button "${label}" exists`);
  return el as HTMLButtonElement;
}

function buttonByText(text: string, root: ParentNode = document.body) {
  const el = [...root.querySelectorAll("button")].find((b) => b.textContent?.trim() === text);
  assert.ok(el, `button "${text}" exists`);
  return el;
}

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  document.body.innerHTML = "";
});

test("the open drawer shows the title, the detail rows and the merged content", async () => {
  mountDrawer();
  await flushPromises();

  const el = panel();
  assert.equal(el.querySelector('[data-slot="drawer-title"]')?.textContent?.trim(), "Handbook");
  assert.match(el.textContent || "", /2026-09-01 10:00/);
  assert.match(el.querySelector(".md-content")?.textContent || "", /First chunk/);
  // Its width is the persisted (here the default) drawer width.
  assert.equal(el.style.width, "654px");
});

test("the close button asks the parent to close the drawer", async () => {
  const { events } = mountDrawer();
  await flushPromises();

  buttonByLabel(enUS.common.close, panel()).click();
  await nextTick();
  assert.deepEqual(events.closeDoc, [false]);
});

test("the chunk view lists each chunk with its enable switch", async () => {
  mountDrawer();
  await flushPromises();

  buttonByText(enUS.knowledgeBase.viewChunks, panel()).click();
  await flushPromises();

  const el = panel();
  assert.match(el.textContent || "", new RegExp(`${enUS.knowledgeBase.segment}\\s+1`));
  assert.match(el.textContent || "", new RegExp(`${enUS.knowledgeBase.segment}\\s+2`));
  const switches = el.querySelectorAll('[role="switch"]');
  assert.equal(switches.length, 2);
  assert.equal(switches[0].getAttribute("data-state"), "checked");
  assert.equal(switches[0].getAttribute("aria-checked"), "true");
  assert.equal(switches[1].getAttribute("data-state"), "unchecked");
  assert.equal(switches[1].getAttribute("aria-checked"), "false");
});

test("the questions popover opens from the chunk toolbar", async () => {
  mountDrawer();
  await flushPromises();
  buttonByText(enUS.knowledgeBase.viewChunks, panel()).click();
  await flushPromises();

  const entry = panel().querySelector("button.chunk-question-entry") as HTMLButtonElement | null;
  assert.ok(entry);
  entry.click();
  await flushPromises();

  const popover = document.body.querySelector('[data-slot="popover-content"]');
  assert.ok(popover, "the popover opened");
  assert.match(popover.textContent || "", new RegExp(enUS.knowledgeBase.noGeneratedQuestions));
  assert.equal(entry.getAttribute("aria-expanded"), "true");
});

test("editing metadata turns the display into rows of inputs, and cancel restores it", async () => {
  mountDrawer();
  await flushPromises();

  const el = panel();
  assert.match(el.textContent || "", /team:?\s*docs/);
  buttonByLabel(enUS.common.edit, el).click();
  await flushPromises();

  const inputs = [...el.querySelectorAll('input[data-slot="input"]')] as HTMLInputElement[];
  assert.equal(inputs[0].value, "team");
  assert.equal(inputs[1].value, "docs");
  buttonByText(enUS.common.cancel, el).click();
  await flushPromises();
  assert.equal(el.querySelectorAll('input[data-slot="input"]').length, 0);
});

test("the pager appears past one page and asks the parent for the page picked", async () => {
  const { events } = mountDrawer(makeDetails(60));
  await flushPromises();

  const el = panel();
  assert.match(el.textContent || "", /Total 60 items/);
  buttonByText("2", el).click();
  await flushPromises();
  assert.deepEqual(events.getDoc, [2]);
});
