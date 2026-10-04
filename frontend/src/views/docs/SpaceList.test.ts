import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { defineComponent, h } from "vue";
import { createI18n } from "vue-i18n";

import { TooltipProvider } from "@/components/ui/tooltip";

import enUS from "@/i18n/locales/en-US";

// Mounted tests for the create-space dialog's knowledge-base field: a new
// knowledge base by default, the reason when none can be made, and only the
// knowledge bases the caller may fill offered for binding.

const api = vi.hoisted(() => ({
  listSpaces: vi.fn(),
  createSpace: vi.fn(),
  listKnowledgeBases: vi.fn(),
  listModels: vi.fn(),
  push: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
}));
vi.mock("@/api/docs", () => ({
  listSpaces: api.listSpaces,
  createSpace: api.createSpace,
  EMBEDDING_MODEL_REQUIRED: 2300,
}));
vi.mock("@/api/knowledge-base", () => ({ listKnowledgeBases: api.listKnowledgeBases }));
vi.mock("@/api/model", () => ({ listModels: api.listModels }));
vi.mock("@/stores/auth", () => ({
  useAuthStore: () => ({ currentUserId: "alice", hasRole: (r: string) => r === "contributor" }),
}));
vi.mock("@/stores/ui", () => ({ useUIStore: () => ({ openSettings: vi.fn() }) }));
vi.mock("vue-router", () => ({ useRouter: () => ({ push: api.push }) }));
vi.mock("tdesign-vue-next", () => ({ MessagePlugin: { success: api.success, error: api.error } }));

import SpaceList from "./SpaceList.vue";

const mounted: VueWrapper[] = [];

const embedder = { id: "m1", name: "embedder", type: "Embedding", source: "remote", parameters: {} };
const kbs = [
  { id: "kb-alice", name: "Alice's notes", type: "document", creator_id: "alice" },
  { id: "kb-bob", name: "Bob's notes", type: "document", creator_id: "bob" },
  { id: "kb-faq", name: "Alice's FAQ", type: "faq", creator_id: "alice" },
];

beforeEach(() => {
  Object.values(api).forEach((fn) => fn.mockReset());
  api.listSpaces.mockResolvedValue([]);
  api.listKnowledgeBases.mockResolvedValue({ success: true, data: kbs });
  api.listModels.mockResolvedValue([embedder]);
  api.createSpace.mockImplementation(async (body: { name: string }) => ({ id: "s1", slug: "handbook", ...body }));
});

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
  document.body.innerHTML = "";
});

async function openCreateDialog() {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  // The list's create button has a tooltip, which needs the provider App.vue gives the app.
  const Root = defineComponent({ setup: () => () => h(TooltipProvider, () => h(SpaceList)) });
  const wrapper = mount(Root, { global: { plugins: [i18n] }, attachTo: document.body });
  mounted.push(wrapper);
  await flushPromises();
  (document.body.querySelector(`button[aria-label="${enUS.docs.spaces.create}"]`) as HTMLButtonElement).click();
  await flushPromises();
  const dialog = document.body.querySelector('[role="dialog"]');
  assert.ok(dialog, "the create dialog is open");
  const name = dialog.querySelector("#space-form-name") as HTMLInputElement;
  name.value = "Handbook";
  name.dispatchEvent(new Event("input", { bubbles: true }));
  await flushPromises();
  return dialog;
}

const radio = (dialog: Element, mode: string) =>
  dialog.querySelector(`[data-testid="kb-sync-${mode}"]`) as HTMLButtonElement;

async function submit(dialog: Element) {
  const button = Array.from(dialog.querySelectorAll("button")).find(
    (b) => b.textContent?.trim() === enUS.common.create,
  ) as HTMLButtonElement;
  button.click();
  await flushPromises();
}

test("a new space syncs to a knowledge base created with it, by default", async () => {
  const dialog = await openCreateDialog();
  assert.equal(radio(dialog, "create").getAttribute("aria-checked"), "true");

  await submit(dialog);
  assert.deepEqual(api.createSpace.mock.calls[0][0].knowledge_base, { mode: "create" });
});

test("without an embedding model the field says why and the space is made without syncing", async () => {
  api.listModels.mockResolvedValue([]);
  const dialog = await openCreateDialog();

  assert.ok(radio(dialog, "create").hasAttribute("disabled"));
  const reason = dialog.querySelector('[data-testid="kb-sync-no-embedding"]');
  assert.match(reason?.textContent ?? "", new RegExp(enUS.docs.spaces.kbSync.noEmbedding));
  assert.equal(radio(dialog, "none").getAttribute("aria-checked"), "true");

  await submit(dialog);
  assert.deepEqual(api.createSpace.mock.calls[0][0].knowledge_base, { mode: "none" });
});

test("binding offers only the document knowledge bases the caller may fill", async () => {
  const dialog = await openCreateDialog();
  radio(dialog, "existing").click();
  await flushPromises();

  // One candidate (Bob's is not Alice's to fill, the FAQ base holds no pages),
  // so it is chosen already.
  const trigger = dialog.querySelector('[data-testid="kb-sync-select"]');
  assert.match(trigger?.textContent ?? "", /Alice's notes/);
  trigger?.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
  await flushPromises();
  const offered = Array.from(document.body.querySelectorAll('[role="option"]')).map((o) => o.textContent?.trim());
  assert.deepEqual(offered, ["Alice's notes"]);
  document.body.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
  await flushPromises();

  await submit(dialog);
  assert.deepEqual(api.createSpace.mock.calls[0][0].knowledge_base, { mode: "existing", id: "kb-alice" });
});

test("a refusal for want of an embedding model is explained, not shown raw", async () => {
  api.createSpace.mockRejectedValue({ status: 400, error: { code: 2300, message: "no embedding model" } });
  const dialog = await openCreateDialog();
  await submit(dialog);

  assert.deepEqual(api.error.mock.calls[0], [enUS.docs.spaces.kbSync.noEmbedding]);
  assert.equal(api.push.mock.calls.length, 0, "no space to go to");
});
