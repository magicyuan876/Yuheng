import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { defineComponent, h } from "vue";
import { createI18n } from "vue-i18n";

import type { DocsSpace } from "@/api/docs";
import { TooltipProvider } from "@/components/ui/tooltip";
import enUS from "@/i18n/locales/en-US";

// Mounted tests for a space's knowledge base in its settings: named and linked
// rather than shown as an id, and rebound or unbound by its administrators
// with the consequence for the pages already synced stated before saving.

const api = vi.hoisted(() => ({
  getSpaceBySlug: vi.fn(),
  listSpaceMembers: vi.fn(),
  bindSpaceKnowledgeBase: vi.fn(),
  getSpaceUsage: vi.fn(),
  listKnowledgeBases: vi.fn(),
  listModels: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
}));
vi.mock("@/api/docs", () => ({
  bindSpaceKnowledgeBase: api.bindSpaceKnowledgeBase,
  deleteSpace: vi.fn(),
  getSpaceBySlug: api.getSpaceBySlug,
  listSpaceMembers: api.listSpaceMembers,
  removeSpaceMember: vi.fn(),
  setSpaceMembers: vi.fn(),
  updateSpace: vi.fn(),
  downloadExport: vi.fn(),
  getImportJob: vi.fn(),
  getExportJob: vi.fn(),
  startImport: vi.fn(),
  startSpaceExport: vi.fn(),
  getSpaceUsage: api.getSpaceUsage,
  setSpaceQuota: vi.fn(),
  EMBEDDING_MODEL_REQUIRED: 2300,
}));
vi.mock("@/api/tenant/groups", () => ({ listGroups: vi.fn().mockResolvedValue([]) }));
vi.mock("@/api/tenant/members", () => ({ listMembers: vi.fn().mockResolvedValue({ items: [] }) }));
vi.mock("@/api/knowledge-base", () => ({ listKnowledgeBases: api.listKnowledgeBases }));
vi.mock("@/api/model", () => ({ listModels: api.listModels }));
vi.mock("@/stores/auth", () => ({
  useAuthStore: () => ({ currentUserId: "alice", hasRole: () => true }),
}));
vi.mock("@/stores/ui", () => ({ useUIStore: () => ({ openSettings: vi.fn() }) }));
vi.mock("vue-router", () => ({
  useRoute: () => ({ params: { slug: "handbook" } }),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  RouterLink: defineComponent({
    props: { to: { type: Object, required: true } },
    setup:
      (props, { slots }) =>
      () =>
        h("a", { "data-to": JSON.stringify(props.to) }, slots.default?.()),
  }),
}));
vi.mock("tdesign-vue-next", () => ({
  MessagePlugin: { success: api.success, error: api.error, warning: vi.fn(), info: vi.fn() },
}));

import SpaceSettings from "./SpaceSettings.vue";

const mounted: VueWrapper[] = [];

const space = (over: Partial<DocsSpace> = {}): DocsSpace => ({
  id: "s1",
  tenant_id: 1,
  slug: "handbook",
  name: "Handbook",
  description: "",
  visibility: "private",
  default_role: "none",
  knowledge_base_id: "kb-1",
  storage_backend_id: "sb",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
  role: "admin",
  member_count: 1,
  page_count: 3,
  ...over,
});

beforeEach(() => {
  Object.values(api).forEach((fn) => fn.mockReset());
  api.getSpaceBySlug.mockResolvedValue(space());
  api.listSpaceMembers.mockResolvedValue([]);
  api.getSpaceUsage.mockResolvedValue({ used_bytes: 0, quota_bytes: 0 });
  api.listKnowledgeBases.mockResolvedValue({
    success: true,
    data: [
      { id: "kb-1", name: "Handbook KB", type: "document", creator_id: "alice" },
      { id: "kb-2", name: "Team KB", type: "document", creator_id: "alice" },
    ],
  });
  api.listModels.mockResolvedValue([{ id: "m1", name: "e", type: "Embedding", source: "remote", parameters: {} }]);
});

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
  document.body.innerHTML = "";
});

async function mountSettings() {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const Root = defineComponent({ setup: () => () => h(TooltipProvider, () => h(SpaceSettings)) });
  const wrapper = mount(Root, { global: { plugins: [i18n] }, attachTo: document.body });
  mounted.push(wrapper);
  await flushPromises();
  return wrapper;
}

/** Opens the settings tab; Reka's tabs switch on mousedown, not click. */
async function openSettingsTab() {
  const trigger = Array.from(document.body.querySelectorAll('[role="tab"]')).find(
    (tab) => tab.textContent?.trim() === enUS.docs.spaces.tabs.settings,
  );
  assert.ok(trigger, "an administrator has a settings tab");
  trigger.dispatchEvent(new MouseEvent("mousedown", { bubbles: true, button: 0 }));
  await flushPromises();
  const section = document.body.querySelector('[data-testid="kb-sync-settings"]');
  assert.ok(section, "the knowledge-base section is shown");
  return section;
}

const pick = async (section: Element, mode: string) => {
  (section.querySelector(`[data-testid="kb-sync-${mode}"]`) as HTMLButtonElement).click();
  await flushPromises();
};

const save = async (section: Element) => {
  (section.querySelector('[data-testid="kb-sync-save"]') as HTMLButtonElement).click();
  await flushPromises();
};

test("the bound knowledge base is named and linked, not shown as an id", async () => {
  await mountSettings();
  const link = document.body.querySelector('[data-testid="kb-bound-link"]');
  assert.ok(link);
  assert.match(link.textContent ?? "", /Handbook KB/);
  assert.deepEqual(JSON.parse(link.getAttribute("data-to") ?? "{}"), {
    name: "knowledgeBaseDetail",
    params: { kbId: "kb-1" },
  });
});

test("a knowledge base the reader cannot find is reported as missing", async () => {
  api.getSpaceBySlug.mockResolvedValue(space({ knowledge_base_id: "kb-gone" }));
  await mountSettings();
  const missing = document.body.querySelector('[data-testid="kb-bound-missing"]');
  assert.match(missing?.textContent ?? "", /kb-gone/);
  assert.match(missing?.textContent ?? "", new RegExp(enUS.docs.spaces.kbSync.missing));
});

test("unbinding says the synced pages leave the knowledge base, then unbinds", async () => {
  api.bindSpaceKnowledgeBase.mockResolvedValue(space({ knowledge_base_id: null }));
  await mountSettings();
  const section = await openSettingsTab();
  const saveButton = section.querySelector('[data-testid="kb-sync-save"]') as HTMLButtonElement;
  assert.ok(saveButton.disabled, "nothing to save before a change");

  await pick(section, "none");
  const effect = section.querySelector('[data-testid="kb-sync-effect"]')?.textContent ?? "";
  assert.equal(effect.trim(), enUS.docs.spaces.kbSync.unbindEffect.replace("{name}", "Handbook KB"));

  await save(section);
  assert.deepEqual(api.bindSpaceKnowledgeBase.mock.calls[0], ["s1", { knowledge_base: { mode: "none" } }]);
  assert.deepEqual(api.success.mock.calls[0], [enUS.docs.spaces.kbSync.saveSuccess]);
});

test("rebinding to a new knowledge base says the pages move, then creates it", async () => {
  api.bindSpaceKnowledgeBase.mockResolvedValue(space({ knowledge_base_id: "kb-new" }));
  await mountSettings();
  const section = await openSettingsTab();

  await pick(section, "create");
  const effect = section.querySelector('[data-testid="kb-sync-effect"]')?.textContent ?? "";
  assert.equal(effect.trim(), enUS.docs.spaces.kbSync.rebindEffect.replace("{name}", "Handbook KB"));

  await save(section);
  assert.deepEqual(api.bindSpaceKnowledgeBase.mock.calls[0], ["s1", { knowledge_base: { mode: "create" } }]);
  assert.equal(api.listKnowledgeBases.mock.calls.length, 2, "the list is reloaded to name the new knowledge base");
});

test("a space that syncs nowhere can be bound to an existing knowledge base", async () => {
  api.getSpaceBySlug.mockResolvedValue(space({ knowledge_base_id: null }));
  api.bindSpaceKnowledgeBase.mockResolvedValue(space({ knowledge_base_id: "kb-2" }));
  await mountSettings();
  const section = await openSettingsTab();

  await pick(section, "existing");
  const trigger = section.querySelector('[data-testid="kb-sync-select"]');
  trigger?.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
  await flushPromises();
  const option = Array.from(document.body.querySelectorAll('[role="option"]')).find((o) =>
    o.textContent?.includes("Team KB"),
  );
  assert.ok(option, "the knowledge bases are offered");
  option.dispatchEvent(new Event("pointerup", { bubbles: true }));
  await flushPromises();
  assert.equal(
    section.querySelector('[data-testid="kb-sync-effect"]')?.textContent?.trim(),
    enUS.docs.spaces.kbSync.bindEffect,
  );

  await save(section);
  assert.deepEqual(api.bindSpaceKnowledgeBase.mock.calls[0], [
    "s1",
    { knowledge_base: { mode: "existing", id: "kb-2" } },
  ]);
});
