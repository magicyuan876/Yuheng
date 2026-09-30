import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import { defineComponent, h } from "vue";
import enUS from "@/i18n/locales/en-US";
import { TooltipProvider } from "@/components/ui/tooltip";
import WikiBrowser from "./WikiBrowser.vue";

// The wiki browser mounted against a mocked wiki API. These tests cover the
// parts of the screen that the move off TDesign rewrote by hand rather than
// swapped one component for another: the sidebar search (its clear button
// used to come with the TDesign input), the reader header a search hit
// opens, the delete confirmation that replaced the popconfirm, and the lint
// report with its automatic fix.

const page = {
  id: "p1",
  slug: "concept/raft",
  title: "Raft consensus",
  summary: "Leader-based replication.",
  content: "# Raft consensus\n\nA [[concept/paxos]] alternative.",
  page_type: "concept",
  version: 3,
  updated_at: "2026-09-01T10:00:00Z",
  in_links: [],
  source_refs: [],
  aliases: ["raft"],
};

const api = vi.hoisted(() => ({
  searchWikiPages: vi.fn(),
  getWikiPage: vi.fn(),
  getWikiStats: vi.fn(),
  lintWiki: vi.fn(),
  autoFixWiki: vi.fn(),
  deleteWikiPage: vi.fn(),
}));

vi.mock("@/api/wiki", () => {
  const empty = async () => ({ data: {} });
  return {
    listWikiPages: vi.fn(async () => ({ data: { pages: [], total: 0 } })),
    listWikiFolders: vi.fn(async () => ({ data: [] })),
    createWikiFolder: vi.fn(empty),
    updateWikiFolder: vi.fn(empty),
    deleteWikiFolder: vi.fn(empty),
    moveWikiPage: vi.fn(empty),
    createWikiPage: vi.fn(empty),
    updateWikiPage: vi.fn(empty),
    deleteWikiPage: api.deleteWikiPage,
    getWikiPage: api.getWikiPage,
    getWikiIndex: vi.fn(async () => {
      throw new Error("no index");
    }),
    getWikiGraph: vi.fn(empty),
    getWikiStats: api.getWikiStats,
    searchWikiPages: api.searchWikiPages,
    lintWiki: api.lintWiki,
    autoFixWiki: api.autoFixWiki,
  };
});

vi.mock("@/api/knowledge-base", () => ({ getKnowledgeDetails: vi.fn(async () => ({ data: {} })) }));
vi.mock("vue-router", () => ({ useRoute: () => ({ query: {} }) }));
vi.mock("tdesign-vue-next", () => ({ MessagePlugin: { success: vi.fn(), error: vi.fn(), warning: vi.fn() } }));

let wrapper: VueWrapper | null = null;

async function mountBrowser() {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  wrapper = mount(
    defineComponent({
      render: () =>
        h(TooltipProvider, () => h(WikiBrowser, { knowledgeBaseId: "kb1", view: "browser", canEdit: true })),
    }),
    { attachTo: document.body, global: { plugins: [i18n] } },
  );
  await flushPromises();
  return wrapper;
}

function searchInput(): HTMLInputElement {
  const input = document.querySelector<HTMLInputElement>(
    `input[placeholder="${enUS.knowledgeEditor.wikiBrowser.searchPlaceholder}"]`,
  );
  assert.ok(input, "the sidebar search input renders");
  return input;
}

async function searchAndOpenHit() {
  const input = searchInput();
  input.value = "raft";
  input.dispatchEvent(new Event("input"));
  // The Input component emits its model from a watcher, so the query only
  // reaches the page after a tick, as it would between two keystrokes.
  await flushPromises();
  input.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter" }));
  await flushPromises();
  const hit = [...document.querySelectorAll("div")].find((el) => el.textContent?.trim() === page.title);
  assert.ok(hit, "the search hit is listed");
  hit.click();
  await flushPromises();
}

beforeEach(() => {
  api.searchWikiPages.mockResolvedValue({ data: { pages: [page] } });
  api.getWikiPage.mockResolvedValue({ data: page });
  api.getWikiStats.mockResolvedValue({ data: { pending_tasks: 0, is_active: false } });
  api.deleteWikiPage.mockResolvedValue({ data: {} });
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  document.body.innerHTML = "";
  vi.clearAllMocks();
});

test("a search hit opens in the reader with its title, badges and rendered body", async () => {
  await mountBrowser();
  await searchAndOpenHit();

  assert.equal(api.searchWikiPages.mock.calls[0]?.[1], "raft");
  const title = document.querySelector("h2");
  assert.equal(title?.textContent?.trim(), page.title);
  const header = title?.parentElement?.textContent ?? "";
  assert.match(header, /Concept/);
  assert.match(header, /raft/);
  // [[wiki-links]] in the Markdown become the clickable link class the
  // content click handler looks for.
  assert.ok(document.querySelector(".wiki-reader-body a.wiki-content-link[data-slug='concept/paxos']"));
});

test("the clear button empties the search and returns to the bucketed view", async () => {
  await mountBrowser();
  await searchAndOpenHit();

  const clear = document.querySelector<HTMLButtonElement>(`button[aria-label="${enUS.common.clear}"]`);
  assert.ok(clear, "a non-empty search shows a clear button");
  clear.click();
  await flushPromises();

  assert.equal(searchInput().value, "");
  assert.equal(document.querySelector(`button[aria-label="${enUS.common.clear}"]`), null);
});

test("deleting a page asks first, and only the confirm button deletes it", async () => {
  await mountBrowser();
  await searchAndOpenHit();

  const del = document.querySelector<HTMLButtonElement>(
    `button[aria-label="${enUS.knowledgeEditor.wikiBrowser.deletePageBtn}"]`,
  );
  assert.ok(del, "an editor sees the delete button");
  del.click();
  await flushPromises();
  assert.equal(api.deleteWikiPage.mock.calls.length, 0);

  const confirm = [...document.querySelectorAll("button")].find((el) => el.textContent?.trim() === enUS.common.confirm);
  assert.ok(confirm, "the confirmation offers a confirm button");
  confirm.click();
  await flushPromises();
  assert.deepEqual(api.deleteWikiPage.mock.calls[0], ["kb1", page.slug]);
});

function lintButton(): HTMLButtonElement {
  const btn = [...document.querySelectorAll("button")].find(
    (el) => el.textContent?.trim() === enUS.knowledgeEditor.wikiBrowser.lintOpen,
  );
  assert.ok(btn, "the wiki offers its structural check");
  return btn as HTMLButtonElement;
}

const report = {
  knowledge_base_id: "kb1",
  health_score: 80,
  summary: "",
  issues: [
    {
      type: "broken_link",
      severity: "warning",
      page_slug: page.slug,
      target_slug: "concept/gone",
      description: "Page 'Raft consensus' links to [[concept/gone]] which does not exist",
      auto_fixable: true,
    },
    {
      type: "orphan_page",
      severity: "info",
      page_slug: "entity/lonely",
      description: "Page 'Lonely' has no inbound links",
      auto_fixable: false,
    },
  ],
};

test("the lint report is run when opened and lists what it found", async () => {
  api.lintWiki.mockResolvedValue(report);
  await mountBrowser();
  assert.equal(api.lintWiki.mock.calls.length, 0, "the whole-wiki check is not run on every visit");

  lintButton().click();
  await flushPromises();
  assert.deepEqual(api.lintWiki.mock.calls[0], ["kb1"]);
  const drawer = document.querySelector('[data-testid="wiki-lint"]');
  assert.ok(drawer, "the report opens");
  const text = drawer.textContent ?? "";
  assert.match(text, /80/);
  assert.ok(text.includes(enUS.knowledgeEditor.wikiBrowser.lintBrokenLink));
  assert.ok(text.includes(enUS.knowledgeEditor.wikiBrowser.lintOrphan));
  assert.ok(text.includes(report.issues[0].description));
});

test("automatic fixing fixes what can be fixed and runs the check again", async () => {
  api.lintWiki.mockResolvedValueOnce(report).mockResolvedValueOnce({ ...report, issues: [report.issues[1]] });
  api.autoFixWiki.mockResolvedValue({ fixed: 1 });
  await mountBrowser();
  lintButton().click();
  await flushPromises();

  const label = enUS.knowledgeEditor.wikiBrowser.lintAutoFix.replace("{count}", "1");
  const fix = [...document.querySelectorAll("button")].find((el) => el.textContent?.trim() === label);
  assert.ok(fix, "an editor is offered the fix for the one fixable problem");
  fix.click();
  await flushPromises();

  assert.deepEqual(api.autoFixWiki.mock.calls[0], ["kb1"]);
  assert.equal(api.lintWiki.mock.calls.length, 2, "the report follows the fix");
  assert.equal(
    [...document.querySelectorAll("button")].find((el) => el.textContent?.trim() === label),
    undefined,
    "nothing fixable is left to offer",
  );
});
