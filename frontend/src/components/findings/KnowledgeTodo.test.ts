import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createPinia } from "pinia";
import { createI18n } from "vue-i18n";

import type { Finding } from "@/api/findings";
import enUS from "@/i18n/locales/en-US";

// Mounted tests for the knowledge to-do in the page corner: it appears only
// with something to do, lists the caller's findings when opened, and sends
// them to the knowledge base's health tab to act.

const api = vi.hoisted(() => ({ countAssignedFindings: vi.fn(), listAssignedFindings: vi.fn() }));
vi.mock("@/api/findings", () => api);

const router = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("vue-router", () => ({ useRouter: () => router }));

import KnowledgeTodo from "./KnowledgeTodo.vue";

const mounted: VueWrapper[] = [];

beforeEach(() => {
  Object.values(api).forEach((fn) => fn.mockReset());
  router.push.mockReset();
});

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
  document.body.innerHTML = "";
});

const finding = (id: string): Finding => ({
  id,
  knowledge_base_id: "kb7",
  knowledge_base_name: "Handbook",
  type: "divergent",
  detector: "duplicate",
  severity: "warning",
  status: "open",
  score: 0.97,
  overlap_ratio: 0.5,
  subject: { knowledge_id: "a", title: "Leave 2024" },
  related: { knowledge_id: "b", title: "Leave 2026" },
  evidence: [],
  created_at: "2026-09-01T10:00:00Z",
  updated_at: "2026-09-01T10:00:00Z",
  resolved_at: null,
  resolved_by: null,
});

async function mountTodo() {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(KnowledgeTodo, { global: { plugins: [i18n, createPinia()] }, attachTo: document.body });
  mounted.push(wrapper);
  await flushPromises();
  return wrapper;
}

test("nothing to do, nothing in the corner", async () => {
  api.countAssignedFindings.mockResolvedValue(0);
  const wrapper = await mountTodo();
  assert.equal(wrapper.find('[data-testid="knowledge-todo"]').exists(), false);
});

test("opens the caller's findings and sends one to its knowledge base", async () => {
  api.countAssignedFindings.mockResolvedValue(3);
  api.listAssignedFindings.mockResolvedValue({ items: [finding("f1")], total: 3, page: 1, page_size: 20 });
  const wrapper = await mountTodo();
  assert.match(wrapper.get('[data-testid="knowledge-todo-count"]').text(), /3/);

  await wrapper.get('[data-testid="knowledge-todo"]').trigger("click");
  await flushPromises();
  assert.deepEqual(api.listAssignedFindings.mock.calls[0], [{ status: "open", page: 1, page_size: 20 }]);
  const list = document.body.querySelector('[data-testid="knowledge-todo-list"]');
  assert.ok(list?.textContent?.includes("Handbook"), "each finding names its knowledge base");
  assert.ok(list?.textContent?.includes("Load more (2 left)"));

  (document.body.querySelector('[data-testid="knowledge-todo-open"]') as HTMLElement).click();
  await flushPromises();
  assert.deepEqual(router.push.mock.calls[0], [
    { name: "knowledgeBaseDetail", params: { kbId: "kb7" }, query: { tab: "health" } },
  ]);
});
