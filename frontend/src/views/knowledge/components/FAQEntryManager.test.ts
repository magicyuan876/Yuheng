import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { defineComponent, h } from "vue";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import { TooltipProvider } from "@/components/ui/tooltip";

// A mounted test of the FAQ manager's batch wiring: selecting cards, the
// batch bar's actions reaching the API with the selected ids, and the editor
// refusing an empty entry. It replaces the part of the old regex test that
// asserted on the manager's source text.

const api = vi.hoisted(() => ({
  listFAQEntries: vi.fn(),
  upsertFAQEntries: vi.fn(),
  createFAQEntry: vi.fn(),
  updateFAQEntry: vi.fn(),
  updateFAQEntryFieldsBatch: vi.fn(),
  deleteFAQEntries: vi.fn(),
  searchFAQEntries: vi.fn(),
  exportFAQEntries: vi.fn(),
  listKnowledgeTags: vi.fn(),
  updateFAQEntryTagBatch: vi.fn(),
  getKnowledgeBaseById: vi.fn(),
  listKnowledgeBases: vi.fn(),
  getFAQImportProgress: vi.fn(),
  updateFAQImportResultDisplayStatus: vi.fn(),
}));

vi.mock("@/api/knowledge-base", () => api);
vi.mock("vue-router", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("tdesign-vue-next", () => ({
  MessagePlugin: { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() },
}));
// The stores are replaced by the few members the manager reads, with a user
// who created the knowledge base and so may edit and manage it.
vi.mock("@/stores/auth", () => ({ useAuthStore: () => ({ user: { id: "u1" }, hasRole: () => false }) }));
vi.mock("@/stores/ui", () => ({
  useUIStore: () => ({ clearSelectedTagIds: vi.fn(), toggleSelectedTagId: vi.fn(), openKBSettings: vi.fn() }),
}));

const { default: FAQEntryManager } = await import("./FAQEntryManager.vue");

const entries = [
  { id: 1, standard_question: "First question", answers: ["A1"], similar_questions: [], negative_questions: [] },
  { id: 2, standard_question: "Second question", answers: ["A2"], similar_questions: [], negative_questions: [] },
].map((e) => ({ ...e, chunk_id: "", knowledge_id: "", knowledge_base_id: "kb1", is_enabled: true, updated_at: "" }));

beforeEach(() => {
  for (const fn of Object.values(api)) fn.mockReset();
  api.getKnowledgeBaseById.mockResolvedValue({ data: { id: "kb1", name: "KB", type: "faq", creator_id: "u1" } });
  api.listKnowledgeBases.mockResolvedValue({ data: [] });
  api.listKnowledgeTags.mockResolvedValue({ data: { data: [], total: 0 } });
  api.listFAQEntries.mockResolvedValue({ data: { data: entries, total: entries.length } });
  api.updateFAQEntryFieldsBatch.mockResolvedValue({});
  api.deleteFAQEntries.mockResolvedValue({});
  api.getFAQImportProgress.mockResolvedValue({});
});

const mounted: VueWrapper[] = [];
afterEach(() => {
  while (mounted.length) mounted.pop()!.unmount();
  document.body.innerHTML = "";
});

async function mountManager() {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  // The tooltips need the provider AppProviders mounts at the root.
  const Host = defineComponent(() => () => h(TooltipProvider, () => h(FAQEntryManager, { kbId: "kb1" })));
  const wrapper = mount(Host, {
    global: {
      plugins: [i18n],
      stubs: { KBInfoPopover: true, KBSwitcherDropdown: true, KbTagManageDrawer: true },
    },
    attachTo: document.body,
  });
  mounted.push(wrapper);
  await flushPromises();
  return wrapper;
}

function cards(wrapper: VueWrapper) {
  return wrapper.findAll(".faq-card");
}

function buttonByText(wrapper: VueWrapper, label: string) {
  return wrapper.findAll("button").find((b) => b.text().includes(label));
}

const faq = enUS.knowledgeEditor.faq;

test("loaded entries render as cards, and clicking one selects it for the batch bar", async () => {
  const wrapper = await mountManager();
  assert.equal(cards(wrapper).length, 2);
  assert.ok(cards(wrapper)[0].text().includes("First question"));
  assert.equal(wrapper.find('[role="region"]').exists(), false, "no batch bar before a selection");

  await cards(wrapper)[0].trigger("click");
  assert.ok(wrapper.find('[role="region"]').text().includes(enUS.knowledgeBase.selectedCount.replace("{count}", "1")));

  await cards(wrapper)[0].trigger("click");
  assert.equal(wrapper.find('[role="region"]').exists(), false, "a second click deselects");
});

test("batch disable sends every selected id to the API", async () => {
  const wrapper = await mountManager();
  await cards(wrapper)[0].trigger("click");
  await cards(wrapper)[1].trigger("click");
  await buttonByText(wrapper, faq.batchDisable)!.trigger("click");
  await flushPromises();
  assert.equal(api.updateFAQEntryFieldsBatch.mock.calls.length, 1);
  assert.deepEqual(api.updateFAQEntryFieldsBatch.mock.calls[0], [
    "kb1",
    { by_id: { 1: { is_enabled: false }, 2: { is_enabled: false } } },
  ]);
});

test("batch delete goes through the confirmation and deletes the selected ids", async () => {
  const wrapper = await mountManager();
  await cards(wrapper)[1].trigger("click");
  await buttonByText(wrapper, faq.batchDelete)!.trigger("click");
  await flushPromises();
  assert.equal(api.deleteFAQEntries.mock.calls.length, 0, "nothing is deleted before confirming");

  const confirm = Array.from(document.body.querySelectorAll("button")).find((b) =>
    b.textContent?.includes(enUS.knowledgeBase.confirmDelete),
  );
  assert.ok(confirm);
  confirm.click();
  await flushPromises();
  assert.deepEqual(api.deleteFAQEntries.mock.calls[0], ["kb1", [2]]);
});

test("the editor refuses an entry without a question or an answer, and says why under the fields", async () => {
  const wrapper = await mountManager();
  const create = wrapper.find(`button[aria-label="${faq.createGroup}"]`);
  assert.ok(create.exists());
  // Reka opens a menu on pointerdown or from the keyboard, not on click; the
  // keyboard path is the one happy-dom dispatches faithfully.
  await create.trigger("keydown", { key: "Enter" });
  await flushPromises();
  const item = Array.from(document.body.querySelectorAll('[role="menuitem"]')).find((el) =>
    el.textContent?.includes(faq.editorCreate),
  ) as HTMLElement | undefined;
  assert.ok(item, "the create menu offers a new entry");
  item.click();
  await flushPromises();

  const submit = Array.from(document.body.querySelectorAll("button")).filter((b) =>
    b.textContent?.includes(faq.editorCreate),
  );
  // The drawer title and the submit button share the label; the button is the last one.
  submit[submit.length - 1].click();
  await flushPromises();
  assert.equal(api.createFAQEntry.mock.calls.length, 0);
  assert.ok(document.body.textContent?.includes(enUS.knowledgeEditor.messages.nameRequired));
  assert.ok(document.body.textContent?.includes(faq.answerRequired));
});
