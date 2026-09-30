import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createPinia } from "pinia";
import { createI18n } from "vue-i18n";

import type { KnowledgeStewardship as Stewardship } from "@/api/stewardship";
import enUS from "@/i18n/locales/en-US";

// Mounted tests for the stewardship panel of the document drawer: what it
// shows, that an editor can confirm and transfer, and that a docs mirror's
// owner is left to its page.

const api = vi.hoisted(() => ({
  getStewardship: vi.fn(),
  setKnowledgeOwner: vi.fn(),
  confirmKnowledgeReviewed: vi.fn(),
}));
vi.mock("@/api/stewardship", () => api);

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("tdesign-vue-next", () => ({ MessagePlugin: toast }));

import MemberPicker from "./MemberPicker.vue";
import KnowledgeStewardship from "./KnowledgeStewardship.vue";

const mounted: VueWrapper[] = [];

beforeEach(() => {
  Object.values(api).forEach((fn) => fn.mockReset());
  toast.success.mockReset();
  toast.error.mockReset();
});

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
});

const stewardship = (over: Partial<Stewardship> = {}): Stewardship => ({
  knowledge_id: "k1",
  origin: "local",
  owner: { id: "u1", username: "Alice", active: true },
  owner_editable: true,
  reviewed_at: null,
  reviewed_by: null,
  review_interval_days: 90,
  review_due_at: "2026-12-01T00:00:00Z",
  overdue: true,
  ...over,
});

async function mountPanel(props: Record<string, unknown> = {}) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(KnowledgeStewardship, {
    props: { knowledgeId: "k1", canEdit: true, ...props },
    global: { plugins: [i18n, createPinia()] },
  });
  mounted.push(wrapper);
  await flushPromises();
  return wrapper;
}

test("shows the owner, the last confirmation and an overdue review", async () => {
  api.getStewardship.mockResolvedValue(stewardship());
  const wrapper = await mountPanel();
  assert.match(wrapper.get('[data-testid="stewardship-owner"]').text(), /Alice/);
  assert.match(wrapper.get('[data-testid="stewardship-reviewed"]').text(), /Never confirmed/);
  assert.match(wrapper.text(), /Overdue/);
});

test("confirming records the review and shows the new state", async () => {
  api.getStewardship.mockResolvedValue(stewardship());
  api.confirmKnowledgeReviewed.mockResolvedValue(
    stewardship({
      reviewed_at: "2026-09-30T00:00:00Z",
      reviewed_by: { id: "u2", username: "Bob", active: true },
      overdue: false,
    }),
  );
  const wrapper = await mountPanel();
  await wrapper.get('[data-testid="stewardship-confirm"]').trigger("click");
  await flushPromises();
  assert.deepEqual(api.confirmKnowledgeReviewed.mock.calls[0], ["k1"]);
  assert.match(wrapper.get('[data-testid="stewardship-reviewed"]').text(), /Bob/);
  assert.doesNotMatch(wrapper.text(), /Overdue/);
  assert.equal(toast.success.mock.calls.length, 1);
});

test("transferring hands the entry to the member picked", async () => {
  api.getStewardship.mockResolvedValue(stewardship());
  api.setKnowledgeOwner.mockResolvedValue(stewardship({ owner: { id: "u2", username: "Bob", active: true } }));
  const wrapper = await mountPanel();
  wrapper.getComponent(MemberPicker).vm.$emit("select", "u2");
  await flushPromises();
  assert.deepEqual(api.setKnowledgeOwner.mock.calls[0], ["k1", "u2"]);
  assert.match(wrapper.get('[data-testid="stewardship-owner"]').text(), /Bob/);
});

test("a refused transfer says why", async () => {
  api.getStewardship.mockResolvedValue(stewardship());
  api.setKnowledgeOwner.mockRejectedValue(new Error("the new owner must be able to edit"));
  const wrapper = await mountPanel();
  wrapper.getComponent(MemberPicker).vm.$emit("select", "u9");
  await flushPromises();
  assert.match(String(toast.error.mock.calls[0][0]), /must be able to edit/);
  assert.match(wrapper.get('[data-testid="stewardship-owner"]').text(), /Alice/);
});

test("a docs mirror's owner is changed on the page, and a reader changes nothing", async () => {
  api.getStewardship.mockResolvedValue(stewardship({ origin: "docs", owner_editable: false }));
  let wrapper = await mountPanel();
  assert.equal(wrapper.find('[data-testid="stewardship-transfer"]').exists(), false);
  assert.match(wrapper.text(), /Changed on the docs page/);

  api.getStewardship.mockResolvedValue(stewardship({ owner: null, owner_editable: true }));
  wrapper = await mountPanel({ canEdit: false });
  assert.match(wrapper.get('[data-testid="stewardship-owner"]').text(), /Not set/);
  assert.equal(wrapper.find('[data-testid="stewardship-transfer"]').exists(), false);
  assert.equal(wrapper.find('[data-testid="stewardship-confirm"]').exists(), false);
});
