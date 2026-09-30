import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";

import type { PageView } from "@/api/docs";
import enUS from "@/i18n/locales/en-US";

// Mounted tests for the page's maintainer in its header: named for everyone,
// changeable only by whom the server says, and handed over through the API.

const api = vi.hoisted(() => ({ setPageOwner: vi.fn(), suggestMentions: vi.fn() }));
vi.mock("@/api/docs", () => api);

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("tdesign-vue-next", () => ({ MessagePlugin: toast }));

import PageOwner from "./PageOwner.vue";

const mounted: VueWrapper[] = [];

beforeEach(() => {
  Object.values(api).forEach((fn) => fn.mockReset());
  toast.success.mockReset();
  toast.error.mockReset();
});

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
  document.body.innerHTML = "";
});

const page = (over: Partial<PageView> = {}): PageView =>
  ({
    id: "p1",
    steward_id: "u1",
    steward: { user_id: "u1", username: "Alice" },
    can_change_owner: true,
    ...over,
  }) as PageView;

async function mountOwner(p: PageView) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(PageOwner, { props: { page: p }, global: { plugins: [i18n] }, attachTo: document.body });
  mounted.push(wrapper);
  await flushPromises();
  return wrapper;
}

test("names the maintainer, and only whom the server allows may change it", async () => {
  const reader = await mountOwner(page({ can_change_owner: false }));
  const button = reader.get('[data-testid="page-owner"]');
  assert.match(button.text(), /Owner: Alice/);
  assert.equal(button.attributes("disabled"), "");
});

test("handing over sends the member picked and passes the page on", async () => {
  api.suggestMentions.mockResolvedValue([{ user_id: "u2", username: "Bob" }]);
  const next = page({ steward_id: "u2", steward: { user_id: "u2", username: "Bob" } });
  api.setPageOwner.mockResolvedValue(next);
  const wrapper = await mountOwner(page());

  await wrapper.get('[data-testid="page-owner"]').trigger("click");
  await flushPromises();
  const option = document.body.querySelector('[data-testid="page-owner-option"][data-user-id="u2"]') as HTMLElement;
  assert.ok(option, "the page's readers are offered");
  option.click();
  await flushPromises();
  assert.deepEqual(api.setPageOwner.mock.calls[0], ["p1", "u2"]);
  assert.deepEqual(wrapper.emitted("changed")?.[0], [next]);
  assert.equal(toast.success.mock.calls.length, 1);
});
