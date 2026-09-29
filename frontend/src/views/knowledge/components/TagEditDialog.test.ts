import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { nextTick } from "vue";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import zhCN from "@/i18n/locales/zh-CN";
import koKR from "@/i18n/locales/ko-KR";
import ruRU from "@/i18n/locales/ru-RU";
import TagEditDialog from "./TagEditDialog.vue";

// These tests replace an older one that regex-matched the TDesign source of
// the dialog. They mount it and check what a user sees and what the parent
// receives, so a markup change that keeps the behaviour does not fail them.

const createKnowledgeBaseTag = vi.fn();
vi.mock("@/api/knowledge-base", () => ({
  createKnowledgeBaseTag: (...args: unknown[]) => createKnowledgeBaseTag(...args),
}));

const tagList = [
  { id: "t1", name: "Alpha", knowledge_count: 2 },
  { id: "t2", name: "Beta" },
  { id: "t3", name: "Gamma" },
];

const mounted: VueWrapper[] = [];

beforeEach(() => {
  createKnowledgeBaseTag.mockReset();
});

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount());
  document.body.innerHTML = "";
});

async function mountDialog(props: Record<string, unknown> = {}) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(TagEditDialog, {
    props: { visible: false, knowledgeName: "Report.pdf", kbId: "kb1", tagList, selectedTags: [], ...props },
    global: { plugins: [i18n] },
    // The dialog is teleported to <body>.
    attachTo: document.body,
  });
  mounted.push(wrapper);
  // The selection is seeded when the dialog opens, as it is in the app.
  await wrapper.setProps({ visible: true });
  await flushPromises();
  return wrapper;
}

function dialog(): HTMLElement {
  const el = document.body.querySelector<HTMLElement>('[role="dialog"]');
  assert.ok(el, "the dialog is open");
  return el;
}

function buttonByText(text: string): HTMLButtonElement | undefined {
  return Array.from(dialog().querySelectorAll("button")).find((b) => b.textContent?.trim() === text);
}

function chip(name: string): HTMLButtonElement | undefined {
  return Array.from(dialog().querySelectorAll<HTMLButtonElement>('button[data-slot="tag-chip"]')).find(
    (b) => b.textContent?.trim() === name,
  );
}

test("shows the heading, the document name and the document's tags in their own section", async () => {
  await mountDialog({ selectedTags: [tagList[1]] });
  const text = dialog().textContent ?? "";
  assert.ok(text.includes(enUS.knowledgeBase.tagEditDialogHeading));
  assert.ok(text.includes("Report.pdf"));
  assert.ok(text.includes(enUS.knowledgeBase.tagEditSelectedSection));
  assert.ok(text.includes(enUS.knowledgeBase.tagEditAvailableSection));
  // The selected tag appears once, not also among the available ones.
  const chips = Array.from(dialog().querySelectorAll('button[data-slot="tag-chip"]')).map((b) => b.textContent?.trim());
  assert.deepEqual(chips, ["Beta", "Alpha", "Gamma"]);
  assert.equal(chip("Alpha")?.title, "Alpha (2)");
});

test("toggling chips and confirming emits the selected tag ids and closes the dialog", async () => {
  const wrapper = await mountDialog({ selectedTags: [tagList[1]] });
  chip("Alpha")?.click();
  await nextTick();
  chip("Beta")?.click();
  await nextTick();
  buttonByText(enUS.common.confirm)?.click();
  await nextTick();
  assert.deepEqual(wrapper.emitted("confirm")?.[0], [["t1"]]);
  assert.deepEqual(wrapper.emitted("update:visible")?.at(-1), [false]);
});

test("the clear action empties the selection", async () => {
  const wrapper = await mountDialog({ selectedTags: [tagList[0], tagList[1]] });
  buttonByText(enUS.knowledgeBase.tagClearAction)?.click();
  await nextTick();
  assert.ok(dialog().textContent?.includes(enUS.knowledgeBase.tagEditNoSelected));
  buttonByText(enUS.common.confirm)?.click();
  await nextTick();
  assert.deepEqual(wrapper.emitted("confirm")?.[0], [[]]);
});

test("cancel closes the dialog without confirming", async () => {
  const wrapper = await mountDialog();
  buttonByText(enUS.common.cancel)?.click();
  await nextTick();
  assert.deepEqual(wrapper.emitted("update:visible")?.[0], [false]);
  assert.equal(wrapper.emitted("confirm"), undefined);
});

test("the manage link is offered only to managers, and closes the dialog before opening the manager", async () => {
  await mountDialog();
  assert.equal(buttonByText(enUS.knowledgeBase.tagManageLink), undefined);
  mounted.splice(0).forEach((w) => w.unmount());

  const wrapper = await mountDialog({ canManage: true });
  buttonByText(enUS.knowledgeBase.tagManageLink)?.click();
  await nextTick();
  assert.deepEqual(wrapper.emitted("update:visible")?.[0], [false]);
  assert.equal(wrapper.emitted("open-manage")?.length, 1);
});

test("searching filters the available tags and offers to create a missing one", async () => {
  createKnowledgeBaseTag.mockResolvedValue({ data: { id: "t9", name: "Delta" } });
  const wrapper = await mountDialog();
  const search = dialog().querySelector<HTMLInputElement>(`input[placeholder="${enUS.knowledgeBase.tagEditSearch}"]`);
  assert.ok(search);
  search.value = "gam";
  search.dispatchEvent(new Event("input"));
  await nextTick();
  assert.equal(chip("Alpha"), undefined);
  assert.ok(chip("Gamma"));

  search.value = "Delta";
  search.dispatchEvent(new Event("input"));
  await nextTick();
  assert.ok(dialog().textContent?.includes(enUS.knowledgeBase.tagEmptyResult));
  const create = Array.from(dialog().querySelectorAll("button")).find((b) =>
    b.textContent?.includes(enUS.knowledgeBase.tagCreateAction),
  );
  create?.click();
  await flushPromises();
  assert.deepEqual(createKnowledgeBaseTag.mock.calls[0], ["kb1", { name: "Delta" }]);
  assert.equal(wrapper.emitted("tag-created")?.length, 1);
  assert.equal(search.value, "");
});

test("Enter in the new-tag field selects an existing tag of that name without creating one", async () => {
  const wrapper = await mountDialog();
  const input = dialog().querySelector<HTMLInputElement>(
    `input[placeholder="${enUS.knowledgeBase.tagNewPlaceholder}"]`,
  );
  assert.ok(input);
  assert.equal(input.maxLength, 40);
  input.value = "Gamma";
  input.dispatchEvent(new Event("input"));
  // The Input component relays its value to the parent on the next tick.
  await nextTick();
  input.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter" }));
  await flushPromises();
  assert.equal(createKnowledgeBaseTag.mock.calls.length, 0);
  buttonByText(enUS.common.confirm)?.click();
  await nextTick();
  assert.deepEqual(wrapper.emitted("confirm")?.[0], [["t3"]]);
});

test("defines the short dialog heading in every supported locale", () => {
  for (const locale of [zhCN, enUS, koKR, ruRU]) {
    for (const key of ["tagEditDialogHeading", "tagEditSelectedSection", "tagEditAvailableSection"] as const) {
      assert.ok(locale.knowledgeBase[key], key);
    }
  }
});
