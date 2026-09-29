import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { defineComponent, reactive } from "vue";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import KnowledgeBaseEditorModal from "./KnowledgeBaseEditorModal.vue";

// Replaces a case of KnowledgeBaseEditorModal.test.mjs that regex-matched the
// footer note's TDesign-era class name. This mounts the editor, creates a
// knowledge base through it and checks what the user then sees, so markup
// changes that keep the behaviour do not fail it.

const createKnowledgeBase = vi.fn();
vi.mock("@/api/knowledge-base", () => ({
  createKnowledgeBase: (...args: unknown[]) => createKnowledgeBase(...args),
  getKnowledgeBaseById: async (id: string) => ({
    data: { id, name: "Handbook", description: "", type: "document", tenant_id: 1, creator_id: "u1" },
  }),
  listKnowledgeFiles: async () => ({ total: 0 }),
  updateKnowledgeBase: vi.fn(),
  rebuildKBIndex: vi.fn(),
}));
vi.mock("@/api/initialization", () => ({ updateKBConfig: vi.fn() }));
vi.mock("tdesign-vue-next", () => ({
  MessagePlugin: { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() },
  DialogPlugin: { confirm: vi.fn() },
}));
vi.mock("@/config/contextualGuides", () => ({
  KB_EDITOR_FOCUS_SECTION_EVENT: "test:kb-editor-focus-section",
  markContextualGuideDone: vi.fn(),
}));

// The stores are replaced by the few members the editor reads, so the test
// needs neither Pinia nor the network behind the real ones.
vi.mock("@/stores/chatResources", () => ({
  useChatResourcesStore: () => ({
    ensureModels: async () => {},
    allModels: [
      { id: "chat-1", type: "KnowledgeQA" },
      { id: "embed-1", type: "Embedding" },
    ],
  }),
}));
vi.mock("@/stores/editorResources", () => ({
  useEditorResourcesStore: () => ({
    ensureStorageEngine: async () => {},
    storageConfig: null,
    resolveUsableStorageProvider: (provider?: string) => provider || "local",
  }),
}));
vi.mock("@/stores/ui", () => ({
  useUIStore: () => reactive({ kbEditorInitialSection: null, showSettingsModal: false }),
}));
vi.mock("@/stores/auth", () => ({
  useAuthStore: () => ({ user: { id: "u1" }, currentTenantId: 1, hasRole: () => true }),
}));

// The settings panels are separate components with their own data needs; the
// behaviour under test is the editor's own frame around them. Their modules
// are replaced outright, so the test does not load (or depend on) them.
vi.mock("./settings/KBModelConfig.vue", () => ({ default: { render: () => null } }));
vi.mock("./settings/KBParserSettings.vue", () => ({ default: { render: () => null } }));
vi.mock("./settings/KBStorageSettings.vue", () => ({ default: { render: () => null } }));
vi.mock("./settings/KBChunkingSettings.vue", () => ({ default: { render: () => null } }));
vi.mock("./settings/KBVectorStoreSettings.vue", () => ({ default: { render: () => null } }));
vi.mock("./settings/KBAdvancedSettings.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/ModelSelector.vue", () => ({ default: { render: () => null } }));
vi.mock("./settings/GraphSettings.vue", () => ({ default: { render: () => null } }));
vi.mock("./settings/KBShareSettings.vue", () => ({ default: { render: () => null } }));
vi.mock("./settings/DataSourceSettings.vue", () => ({ default: { render: () => null } }));
vi.mock("./settings/KnowledgeBaseActivitySettings.vue", () => ({ default: { render: () => null } }));
vi.mock("@/components/KbCreateContextualGuide.vue", () => ({ default: { render: () => null } }));

const Empty = defineComponent({ render: () => null });
const PassThrough = defineComponent({
  setup(_, { slots }) {
    return () => slots.default?.();
  },
});
const stubs = {
  // The copy button's tooltip needs the app-level TooltipProvider.
  Tooltip: PassThrough,
  TooltipTrigger: PassThrough,
  TooltipContent: Empty,
};

const mounted: VueWrapper[] = [];

beforeEach(() => {
  createKnowledgeBase.mockReset();
  createKnowledgeBase.mockResolvedValue({ success: true, data: { id: "kb-new" } });
});

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount());
  document.body.innerHTML = "";
});

async function openCreateEditor() {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(KnowledgeBaseEditorModal, {
    props: { visible: false, mode: "create" },
    global: { plugins: [i18n], stubs },
    // The editor is teleported to <body>.
    attachTo: document.body,
  });
  mounted.push(wrapper);
  // The form is initialised when the editor opens, as it is in the app.
  await wrapper.setProps({ visible: true });
  await flushPromises();
  return wrapper;
}

function buttonByText(text: string): HTMLButtonElement | undefined {
  return Array.from(document.body.querySelectorAll("button")).find((b) => b.textContent?.trim() === text);
}

test("shows a post-create hint after the first successful save", async () => {
  const wrapper = await openCreateEditor();
  const { postCreateHint, buttons } = enUS.knowledgeEditor;

  // Before the first save there is no hint, and the button creates.
  assert.ok(!document.body.textContent?.includes(postCreateHint.title));
  const input = document.body.querySelector<HTMLInputElement>('[data-guide="kb-create-name"] input');
  assert.ok(input, "the name field is rendered");
  input.value = "Handbook";
  input.dispatchEvent(new Event("input"));
  // The Input's v-model is passive: it reports the value on the next tick.
  await flushPromises();

  const create = buttonByText(buttons.create);
  assert.ok(create, "the create button is shown");
  create.click();
  await flushPromises();

  assert.equal(createKnowledgeBase.mock.calls.length, 1);
  assert.deepEqual(wrapper.emitted("success"), [["kb-new"]]);
  // The editor stays open, now in edit mode, with the follow-up hint in the
  // footer and the follow-up description under the new knowledge base's id.
  assert.deepEqual(wrapper.emitted("update:visible"), undefined);
  const text = document.body.textContent ?? "";
  assert.ok(text.includes(postCreateHint.title), "the footer shows the post-create title");
  assert.ok(text.includes(postCreateHint.footer), "the footer explains what to do next");
  assert.ok(text.includes(postCreateHint.followUpDesc), "the id field carries the follow-up description");
  assert.ok(text.includes("kb-new"), "the new knowledge base's id is shown");
  assert.ok(buttonByText(buttons.saveAndClose), "the button now saves and closes");
});
