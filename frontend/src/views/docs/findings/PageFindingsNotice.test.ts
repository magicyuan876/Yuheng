import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, RouterLinkStub, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";

import type { PageFinding, PageFindings } from "@/api/findings";
import enUS from "@/i18n/locales/en-US";

// Mounted tests for the overlap notice on a docs page. The API module is
// mocked; what is checked is when the notice asks at all (only for a page in a
// space with a knowledge base, and not kept out of it), what it says, and
// where its links lead.

const api = vi.hoisted(() => ({ listPageFindings: vi.fn() }));
vi.mock("@/api/findings", () => api);

import PageFindingsNotice from "./PageFindingsNotice.vue";

const mounted: VueWrapper[] = [];

beforeEach(() => {
  api.listPageFindings.mockReset();
  sessionStorage.clear();
});

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
  document.body.innerHTML = "";
});

const item = (id: string, title: string): PageFinding => ({
  id,
  type: "duplicate",
  severity: "warning",
  score: 0.9,
  overlap_ratio: 0.4,
  related_page: { id: `p-${id}`, short_id: `s${id}`, title, space_slug: "eng" },
  evidence: [
    { subject_chunk_id: "a", subject_excerpt: "here", related_chunk_id: "b", related_excerpt: "there", score: 0.95 },
  ],
});

/** The load waits for the browser to be idle, with a bounded delay behind it. */
async function settle() {
  await new Promise((resolve) => setTimeout(resolve, 450));
  await flushPromises();
}

async function mountNotice(props: Record<string, unknown>, answer?: PageFindings) {
  if (answer) api.listPageFindings.mockResolvedValue(answer);
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(PageFindingsNotice, {
    props: { pageId: "p1", pageTitle: "Mine", knowledgeBaseId: "kb1", excluded: false, ...props },
    global: { plugins: [i18n], stubs: { RouterLink: RouterLinkStub } },
    attachTo: document.body,
  });
  mounted.push(wrapper);
  await settle();
  return wrapper;
}

test("a page with overlaps shows how many, and how many more the knowledge base holds", async () => {
  const wrapper = await mountNotice({}, { items: [item("1", "Alpha"), item("2", "Beta")], other_count: 3 });
  assert.deepEqual(api.listPageFindings.mock.calls[0], ["p1"]);
  assert.match(wrapper.get('[data-testid="page-findings-count"]').text(), /overlaps with 2 other page/);
  assert.match(wrapper.get('[data-testid="page-findings-other"]').text(), /and 3 more in the knowledge base/);
});

test("the details list links each related page by its slug", async () => {
  const wrapper = await mountNotice({}, { items: [item("1", "Alpha")], other_count: 0 });
  assert.equal(wrapper.find('[data-testid="page-findings-other"]').exists(), false);

  await wrapper.get('[data-testid="page-findings-details"]').trigger("click");
  await flushPromises();
  const links = wrapper.findAllComponents(RouterLinkStub);
  assert.equal(links.length, 1);
  assert.deepEqual(links[0].props("to"), { name: "docsSpace", params: { slug: "eng", pageSlug: "alpha-s1" } });
  assert.match(document.body.textContent ?? "", /there/);
});

test("no overlaps, no notice", async () => {
  const wrapper = await mountNotice({}, { items: [], other_count: 4 });
  assert.equal(wrapper.find('[data-testid="page-findings-notice"]').exists(), false);
});

test("a space without a knowledge base never asks", async () => {
  const wrapper = await mountNotice({ knowledgeBaseId: null }, { items: [item("1", "Alpha")], other_count: 0 });
  assert.equal(api.listPageFindings.mock.calls.length, 0);
  assert.equal(wrapper.find('[data-testid="page-findings-notice"]').exists(), false);
});

test("a page kept out of the knowledge base never asks", async () => {
  const wrapper = await mountNotice({ excluded: true }, { items: [item("1", "Alpha")], other_count: 0 });
  assert.equal(api.listPageFindings.mock.calls.length, 0);
  assert.equal(wrapper.find('[data-testid="page-findings-notice"]').exists(), false);
});

test("a failed request leaves the page alone", async () => {
  api.listPageFindings.mockRejectedValue(new Error("404"));
  const debug = vi.spyOn(console, "debug").mockImplementation(() => {});
  const wrapper = await mountNotice({});
  assert.equal(wrapper.find('[data-testid="page-findings-notice"]').exists(), false);
  debug.mockRestore();
});

test("putting the notice away keeps it away for the session", async () => {
  const first = await mountNotice({}, { items: [item("1", "Alpha")], other_count: 0 });
  await first.get('[data-testid="page-findings-dismiss"]').trigger("click");
  assert.equal(first.find('[data-testid="page-findings-notice"]').exists(), false);

  api.listPageFindings.mockClear();
  const second = await mountNotice({});
  assert.equal(second.find('[data-testid="page-findings-notice"]').exists(), false);
  assert.equal(api.listPageFindings.mock.calls.length, 0);
});
