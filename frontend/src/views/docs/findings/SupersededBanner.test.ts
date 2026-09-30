import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";

import type { PageView } from "@/api/docs";
import enUS from "@/i18n/locales/en-US";

// Mounted tests for the banner on a superseded page: it says what replaced the
// page, finds the replacement's address only when asked, and lets a writer
// bring the page back.

const api = vi.hoisted(() => ({ getPage: vi.fn(), getSpace: vi.fn() }));
vi.mock("@/api/docs", () => api);

const router = vi.hoisted(() => ({ push: vi.fn() }));
vi.mock("vue-router", () => ({ useRouter: () => router }));

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn(), warning: vi.fn() }));
vi.mock("tdesign-vue-next", () => ({ MessagePlugin: toast }));

import SupersededBanner from "./SupersededBanner.vue";

const mounted: VueWrapper[] = [];

beforeEach(() => {
  Object.values(api).forEach((fn) => fn.mockReset());
  router.push.mockReset();
  toast.warning.mockReset();
});

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
});

const page = (over: Partial<PageView> = {}): PageView =>
  ({
    id: "p-old",
    can_edit: true,
    superseded_by: { knowledge_id: "k-new", title: "Leave policy 2026", page_id: "p-new", at: "2026-09-30T08:00:00Z" },
    ...over,
  }) as PageView;

function mountBanner(p: PageView) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(SupersededBanner, { props: { page: p }, global: { plugins: [i18n] } });
  mounted.push(wrapper);
  return wrapper;
}

test("a page nobody superseded shows nothing", () => {
  assert.equal(
    mountBanner(page({ superseded_by: null }))
      .find('[data-testid="superseded-banner"]')
      .exists(),
    false,
  );
});

test("names the replacement and opens it where it now lives", async () => {
  api.getPage.mockResolvedValue({ id: "p-new", space_id: "s1", title: "Leave policy 2026", short_id: "abc123" });
  api.getSpace.mockResolvedValue({ id: "s1", slug: "hr" });
  const wrapper = mountBanner(page());
  assert.match(wrapper.get('[data-testid="superseded-banner"]').text(), /Superseded by “Leave policy 2026”/);

  await wrapper.get('[data-testid="superseded-open"]').trigger("click");
  await flushPromises();
  assert.deepEqual(api.getPage.mock.calls[0], ["p-new"]);
  const target = router.push.mock.calls[0][0] as { name: string; params: { slug: string; pageSlug: string } };
  assert.equal(target.name, "docsSpace");
  assert.equal(target.params.slug, "hr");
  assert.match(target.params.pageSlug, /abc123/);
});

test("a replacement the reader cannot open stays a title", async () => {
  api.getPage.mockRejectedValue(new Error("not found"));
  const wrapper = mountBanner(page());
  await wrapper.get('[data-testid="superseded-open"]').trigger("click");
  await flushPromises();
  assert.equal(router.push.mock.calls.length, 0);
  assert.equal(toast.warning.mock.calls.length, 1);
});

test("a writer may let the page back in, a reader may not", async () => {
  const writer = mountBanner(page());
  await writer.get('[data-testid="superseded-restore"]').trigger("click");
  assert.equal(writer.emitted("restore")?.length, 1);
  const reader = mountBanner(page({ can_edit: false }));
  assert.equal(reader.find('[data-testid="superseded-restore"]').exists(), false);
});
