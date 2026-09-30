import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createPinia } from "pinia";
import { createI18n } from "vue-i18n";

import type { Finding, FindingsSummary } from "@/api/findings";
import enUS from "@/i18n/locales/en-US";

// Mounted tests for the knowledge-health view. The API module is mocked, so
// what is checked is the view's side of the contract: which states it shows
// for which summary, that a dismissal disappears at once and comes back when
// the server refuses it, and that only administrators get the re-check.

const api = vi.hoisted(() => ({
  getFindingsSummary: vi.fn(),
  listFindings: vi.fn(),
  scanFindings: vi.fn(),
  updateFindingStatus: vi.fn(),
  assignFinding: vi.fn(),
  supersedeFinding: vi.fn(),
}));
vi.mock("@/api/findings", () => api);

const stewardship = vi.hoisted(() => ({ confirmKnowledgeReviewed: vi.fn() }));
vi.mock("@/api/stewardship", () => stewardship);

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("tdesign-vue-next", () => ({ MessagePlugin: toast }));

import FindingItem from "./FindingItem.vue";
import KnowledgeHealthView from "./KnowledgeHealthView.vue";

const mounted: VueWrapper[] = [];

beforeEach(() => {
  Object.values(api).forEach((fn) => fn.mockReset());
  stewardship.confirmKnowledgeReviewed.mockReset();
  toast.success.mockReset();
  toast.error.mockReset();
});

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
  document.body.innerHTML = "";
});

const summary = (over: Partial<FindingsSummary> = {}): FindingsSummary => ({
  open_total: 2,
  open_by_type: { duplicate: 2 },
  last_scan_at: "2026-09-01T10:00:00Z",
  enabled: true,
  supported: true,
  ...over,
});

const finding = (id: string, over: Partial<Finding> = {}): Finding => ({
  id,
  knowledge_base_id: "kb1",
  type: "duplicate",
  detector: "vector-duplicate",
  severity: "warning",
  status: "open",
  score: 0.934,
  overlap_ratio: 0.5,
  subject: { knowledge_id: `${id}-a`, title: `Doc ${id} A` },
  related: { knowledge_id: `${id}-b`, title: `Doc ${id} B` },
  evidence: [
    {
      subject_chunk_id: "c1",
      subject_excerpt: "the same words",
      related_chunk_id: "c2",
      related_excerpt: "the same words again",
      score: 0.97,
    },
  ],
  created_at: "2026-09-01T10:00:00Z",
  updated_at: "2026-09-01T10:00:00Z",
  resolved_at: null,
  resolved_by: null,
  ...over,
});

async function mountView(props: Record<string, unknown> = {}) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(KnowledgeHealthView, {
    props: { kbId: "kb1", canEdit: true, ...props },
    global: { plugins: [i18n, createPinia()] },
    attachTo: document.body,
  });
  mounted.push(wrapper);
  await flushPromises();
  return wrapper;
}

/** Dismisses the first finding for the reason "intentional", as picking it
 * from the card's menu does. */
async function dismissFirst(wrapper: VueWrapper) {
  const item = wrapper.findAllComponents(FindingItem)[0];
  item.vm.$emit("dismiss", item.props("finding"), "intentional");
  await wrapper.vm.$nextTick();
}

test("lists the open findings with both documents, similarity and overlap", async () => {
  api.getFindingsSummary.mockResolvedValue(summary());
  api.listFindings.mockResolvedValue({ items: [finding("f1"), finding("f2")], total: 2, page: 1, page_size: 20 });

  const wrapper = await mountView();

  assert.deepEqual(api.listFindings.mock.calls[0], ["kb1", { status: "open", mine: false, page: 1, page_size: 20 }]);
  const rows = wrapper.findAll('[data-testid="finding-item"]');
  assert.equal(rows.length, 2);
  assert.match(rows[0].text(), /Doc f1 A/);
  assert.match(rows[0].text(), /Doc f1 B/);
  assert.match(rows[0].text(), /Duplicate content/);
  assert.match(rows[0].get('[data-testid="finding-score"]').text(), /93%/);
  assert.match(rows[0].text(), /Overlap 50%/);
  assert.match(wrapper.get('[data-testid="health-summary"]').text(), /2 open/);
});

test("clicking a document title asks to open it, and the evidence expands side by side", async () => {
  api.getFindingsSummary.mockResolvedValue(summary());
  api.listFindings.mockResolvedValue({ items: [finding("f1")], total: 1, page: 1, page_size: 20 });
  const wrapper = await mountView();

  await wrapper.get('[data-testid="finding-related"]').trigger("click");
  assert.deepEqual(wrapper.emitted("open-knowledge"), [["f1-b"]]);

  assert.equal(wrapper.find('[data-testid="finding-evidence"]').exists(), false);
  await wrapper.get('[data-testid="finding-evidence-toggle"]').trigger("click");
  const evidence = wrapper.get('[data-testid="finding-evidence"]');
  assert.match(evidence.text(), /the same words/);
  assert.match(evidence.text(), /the same words again/);
  assert.match(evidence.text(), /97%/);
});

test("an unknown finding type is shown under its raw name", async () => {
  api.getFindingsSummary.mockResolvedValue(summary({ open_by_type: { contradiction: 1 } }));
  api.listFindings.mockResolvedValue({
    items: [finding("f1", { type: "contradiction", severity: "error" })],
    total: 1,
    page: 1,
    page_size: 20,
  });
  const wrapper = await mountView();
  assert.match(wrapper.get('[data-testid="finding-item"]').text(), /contradiction/);
  assert.match(wrapper.get('[data-testid="health-summary"]').text(), /contradiction · 1/);
});

test("dismissing removes the finding at once and keeps it gone when the server agrees", async () => {
  api.getFindingsSummary.mockResolvedValue(summary());
  api.listFindings.mockResolvedValue({ items: [finding("f1"), finding("f2")], total: 2, page: 1, page_size: 20 });
  let answer: (f: Finding) => void = () => {};
  api.updateFindingStatus.mockReturnValue(new Promise<Finding>((resolve) => (answer = resolve)));
  const wrapper = await mountView();

  await dismissFirst(wrapper);
  // Before the server has answered.
  assert.equal(wrapper.findAll('[data-testid="finding-item"]').length, 1);
  assert.match(wrapper.get('[data-testid="health-summary"]').text(), /1 open/);

  answer(finding("f1", { status: "dismissed" }));
  await flushPromises();
  assert.deepEqual(api.updateFindingStatus.mock.calls[0], ["kb1", "f1", "dismissed", "intentional"]);
  assert.equal(wrapper.findAll('[data-testid="finding-item"]').length, 1);
  assert.equal(toast.success.mock.calls.length, 1);
  const lastSummary = wrapper.emitted("summary-change")?.at(-1)?.[0] as FindingsSummary;
  assert.equal(lastSummary.open_total, 1);
});

test("a refused dismissal puts the finding back where it was and says so", async () => {
  api.getFindingsSummary.mockResolvedValue(summary());
  api.listFindings.mockResolvedValue({ items: [finding("f1"), finding("f2")], total: 2, page: 1, page_size: 20 });
  api.updateFindingStatus.mockRejectedValue(new Error("forbidden"));
  const wrapper = await mountView();

  await dismissFirst(wrapper);
  await flushPromises();

  const rows = wrapper.findAll('[data-testid="finding-item"]');
  assert.deepEqual(
    rows.map((r) => r.attributes("data-finding-id")),
    ["f1", "f2"],
  );
  assert.match(wrapper.get('[data-testid="health-summary"]').text(), /2 open/);
  assert.equal(toast.error.mock.calls.length, 1);
  assert.match(String(toast.error.mock.calls[0][0]), /forbidden/);
});

test("the dismissed filter lists dismissed findings, which can be reopened", async () => {
  api.getFindingsSummary.mockResolvedValue(summary());
  api.listFindings.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 });
  const wrapper = await mountView();

  api.listFindings.mockResolvedValue({
    items: [finding("f9", { status: "dismissed" })],
    total: 1,
    page: 1,
    page_size: 20,
  });
  await wrapper.get('[data-testid="health-filter-dismissed"]').trigger("click");
  await flushPromises();
  assert.equal(api.listFindings.mock.calls.at(-1)?.[1].status, "dismissed");

  api.updateFindingStatus.mockResolvedValue(finding("f9", { status: "open" }));
  await wrapper.get('[data-testid="finding-reopen"]').trigger("click");
  await flushPromises();
  assert.deepEqual(api.updateFindingStatus.mock.calls[0], ["kb1", "f9", "open", undefined]);
  assert.equal(wrapper.findAll('[data-testid="finding-item"]').length, 0);
});

test("nothing open reads as a clean bill of health", async () => {
  api.getFindingsSummary.mockResolvedValue(summary({ open_total: 0, open_by_type: {} }));
  api.listFindings.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 });
  const wrapper = await mountView();
  assert.match(wrapper.get('[data-testid="health-empty"]').text(), /No problems found/);
});

test("checks switched off say so and do not list anything", async () => {
  api.getFindingsSummary.mockResolvedValue(summary({ enabled: false }));
  const wrapper = await mountView({ canRescan: true });
  assert.ok(wrapper.find('[data-testid="health-disabled"]').exists());
  assert.equal(api.listFindings.mock.calls.length, 0);
  assert.equal(wrapper.get('[data-testid="health-recheck"]').attributes("disabled"), "");
});

test("an engine without similarity search is explained, ahead of the switched-off state", async () => {
  api.getFindingsSummary.mockResolvedValue(summary({ supported: false, enabled: false }));
  const wrapper = await mountView();
  assert.ok(wrapper.find('[data-testid="health-unsupported"]').exists());
  assert.equal(wrapper.find('[data-testid="health-disabled"]').exists(), false);
  assert.equal(api.listFindings.mock.calls.length, 0);
});

test("only administrators see the re-check, which reports how many were queued", async () => {
  api.getFindingsSummary.mockResolvedValue(summary());
  api.listFindings.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 });

  const reader = await mountView({ canRescan: false });
  assert.equal(reader.find('[data-testid="health-recheck"]').exists(), false);

  api.scanFindings.mockResolvedValue({ queued: 12 });
  const admin = await mountView({ canRescan: true });
  await admin.get('[data-testid="health-recheck"]').trigger("click");
  await flushPromises();
  assert.deepEqual(api.scanFindings.mock.calls[0], ["kb1"]);
  assert.match(admin.get('[data-testid="health-queued"]').text(), /12 document\(s\) queued/);
});

test("more than one page of findings gets a pager that asks for the next page", async () => {
  api.getFindingsSummary.mockResolvedValue(summary({ open_total: 45 }));
  api.listFindings.mockResolvedValue({ items: [finding("f1")], total: 45, page: 1, page_size: 20 });
  const wrapper = await mountView();

  const pager = wrapper.get('[data-testid="health-pager"]');
  assert.match(pager.text(), /Page 1 of 3/);
  await pager.get(`[aria-label="${enUS.knowledgeHealth.next}"]`).trigger("click");
  await flushPromises();
  assert.equal(api.listFindings.mock.calls.at(-1)?.[1].page, 2);
});

test("a wiki knowledge base links to the wiki's own issue list", async () => {
  api.getFindingsSummary.mockResolvedValue(summary());
  api.listFindings.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 });
  const plain = await mountView();
  assert.equal(plain.find('[data-testid="health-wiki"]').exists(), false);

  const wiki = await mountView({ isWiki: true, wikiPendingIssues: 3 });
  const section = wiki.get('[data-testid="health-wiki"]');
  assert.match(section.text(), /3 wiki issue/);
  await section.get("button").trigger("click");
  assert.equal(wiki.emitted("open-wiki-issues")?.length, 1);
});

test("only mine asks the server for the caller's findings", async () => {
  api.getFindingsSummary.mockResolvedValue(summary());
  api.listFindings.mockResolvedValue({ items: [finding("f1")], total: 1, page: 1, page_size: 20 });
  const wrapper = await mountView();

  await wrapper.get('[data-testid="health-filter-mine"]').trigger("click");
  await flushPromises();
  assert.equal(api.listFindings.mock.calls.at(-1)?.[1].mine, true);
  assert.equal(wrapper.get('[data-testid="health-filter-mine"]').attributes("aria-pressed"), "true");
});

test("assigning waits for the server and shows who it named", async () => {
  api.getFindingsSummary.mockResolvedValue(summary());
  api.listFindings.mockResolvedValue({ items: [finding("f1")], total: 1, page: 1, page_size: 20 });
  api.assignFinding.mockResolvedValue(
    finding("f1", { assignee: { id: "u2", username: "Bob", active: true }, assigned_manually: true }),
  );
  const wrapper = await mountView();
  assert.match(wrapper.get('[data-testid="finding-assignee"]').text(), /Unassigned/);

  const item = wrapper.findAllComponents(FindingItem)[0];
  item.vm.$emit("assign", item.props("finding"), "u2");
  await flushPromises();
  assert.deepEqual(api.assignFinding.mock.calls[0], ["kb1", "f1", "u2"]);
  assert.match(wrapper.get('[data-testid="finding-assignee"]').text(), /Bob/);
  assert.match(wrapper.get('[data-testid="finding-assignee"]').text(), /assigned by hand/);
});

test("a reader sees who deals with a finding and cannot act on it", async () => {
  api.getFindingsSummary.mockResolvedValue(summary());
  api.listFindings.mockResolvedValue({
    items: [finding("f1", { assignee: { id: "u1", username: "Alice", active: false } })],
    total: 1,
    page: 1,
    page_size: 20,
  });
  const wrapper = await mountView({ canEdit: false });
  assert.match(wrapper.get('[data-testid="finding-assignee"]').text(), /Alice/);
  assert.match(wrapper.get('[data-testid="finding-assignee"]').text(), /left/);
  assert.equal(wrapper.find('[data-testid="finding-dismiss"]').exists(), false);
  assert.equal(wrapper.find('[data-testid="finding-assign"]').exists(), false);
});

test("a divergent finding explains itself and marks where the passages differ", async () => {
  api.getFindingsSummary.mockResolvedValue(summary({ open_by_type: { divergent: 1 } }));
  api.listFindings.mockResolvedValue({
    items: [
      finding("f1", {
        type: "divergent",
        evidence: [
          {
            subject_chunk_id: "c1",
            subject_excerpt: "Every employee has fifteen days of paid leave a year.",
            related_chunk_id: "c2",
            related_excerpt: "Every employee has ten days of paid leave a year.",
            score: 0.97,
            differs: true,
          },
        ],
      }),
    ],
    total: 1,
    page: 1,
    page_size: 20,
  });
  const wrapper = await mountView();
  const row = wrapper.get('[data-testid="finding-item"]');
  assert.match(row.text(), /Conflicting details/);
  assert.match(row.get('[data-testid="finding-hint"]').text(), /out of date/);

  await wrapper.get('[data-testid="finding-evidence-toggle"]').trigger("click");
  // "fifteen" against "ten": the characters "ten" shares with it are not a
  // difference, the rest of the word is.
  const marks = wrapper.findAll('[data-testid="finding-diff-mark"]').map((m) => m.text());
  assert.ok(marks.join("").includes("fif"), `marked: ${JSON.stringify(marks)}`);
  assert.equal(wrapper.find('[data-differs="true"]').exists(), true);
});

test("superseding is confirmed with both titles, then the lists are fetched again", async () => {
  api.getFindingsSummary.mockResolvedValue(summary());
  api.listFindings.mockResolvedValue({ items: [finding("f1")], total: 1, page: 1, page_size: 20 });
  api.supersedeFinding.mockResolvedValue({ retired_knowledge_id: "f1-a", how: "excluded" });
  const wrapper = await mountView();

  const item = wrapper.findAllComponents(FindingItem)[0];
  item.vm.$emit("supersede", item.props("finding"), "f1-b");
  await flushPromises();
  const dialog = document.body.querySelector('[data-testid="supersede-confirm"]');
  assert.ok(dialog?.textContent?.includes("Keep “Doc f1 B” and take “Doc f1 A” out"), dialog?.textContent ?? "");
  assert.equal(api.supersedeFinding.mock.calls.length, 0, "nothing goes before the confirmation");

  api.listFindings.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 });
  (document.body.querySelector('[data-testid="supersede-confirm-ok"]') as HTMLElement).click();
  await flushPromises();
  assert.deepEqual(api.supersedeFinding.mock.calls[0], ["kb1", "f1", "f1-b"]);
  assert.match(String(toast.success.mock.calls[0][0]), /Doc f1 A.*marked superseded/);
  assert.equal(api.getFindingsSummary.mock.calls.length, 2, "the summary is fetched again");
  assert.equal(wrapper.findAll('[data-testid="finding-item"]').length, 0);
});

test("a document due for review is confirmed from its finding, which then leaves the open list", async () => {
  api.getFindingsSummary.mockResolvedValue(summary({ open_total: 1, open_by_type: { stale: 1 } }));
  api.listFindings.mockResolvedValue({
    items: [finding("f1", { type: "stale", severity: "info", related: null, evidence: [] })],
    total: 1,
    page: 1,
    page_size: 20,
  });
  stewardship.confirmKnowledgeReviewed.mockResolvedValue({});
  const wrapper = await mountView();
  const row = wrapper.get('[data-testid="finding-item"]');
  assert.match(row.text(), /Review due/);
  assert.equal(wrapper.find('[data-testid="finding-supersede"]').exists(), false, "one document: nothing to supersede");

  await wrapper.get('[data-testid="finding-confirm"]').trigger("click");
  await flushPromises();
  assert.deepEqual(stewardship.confirmKnowledgeReviewed.mock.calls[0], ["f1-a"]);
  assert.equal(wrapper.findAll('[data-testid="finding-item"]').length, 0);
  assert.match(wrapper.get('[data-testid="health-summary"]').text(), /0 open/);
});

test("a disputed document shows what people said and can be confirmed, without a similarity", async () => {
  api.getFindingsSummary.mockResolvedValue(summary({ open_total: 1, open_by_type: { disputed: 1 } }));
  api.listFindings.mockResolvedValue({
    items: [
      finding("f1", {
        type: "disputed",
        related: null,
        evidence: [],
        score: 2,
        extra: {
          count: 2,
          reports: [
            {
              feedback_id: "fb1",
              question: "",
              answer: "Fifteen days.",
              comment: "It is ten now",
              at: "2026-09-30T08:00:00Z",
            },
            {
              feedback_id: "fb2",
              question: "Leave for interns?",
              answer: "Fifteen.",
              comment: "",
              at: "2026-09-29T08:00:00Z",
            },
          ],
        },
      }),
    ],
    total: 1,
    page: 1,
    page_size: 20,
  });
  const wrapper = await mountView();
  const row = wrapper.get('[data-testid="finding-item"]');
  assert.match(row.text(), /Answers disputed/);
  assert.equal(wrapper.find('[data-testid="finding-score"]').exists(), false, "no similarity for one document");
  assert.ok(wrapper.find('[data-testid="finding-confirm"]').exists());

  await wrapper.get('[data-testid="finding-reports-toggle"]').trigger("click");
  const reports = wrapper.get('[data-testid="finding-reports"]').text();
  assert.match(reports, /It is ten now/);
  assert.match(reports, /Question: Leave for interns\?/);
  assert.match(reports, /\(no comment\)/);
});
