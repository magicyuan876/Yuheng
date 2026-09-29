import assert from "node:assert/strict";
import { afterEach, test } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import { nextTick } from "vue";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import zhCN from "@/i18n/locales/zh-CN";
import koKR from "@/i18n/locales/ko-KR";
import ruRU from "@/i18n/locales/ru-RU";
import FAQBatchBar from "./FAQBatchBar.vue";

// These tests replace an older file that regex-matched the TDesign source of
// FAQBatchBar.vue. They mount the bar instead and check what a user sees and
// what the parent receives, so a markup change that keeps the behaviour no
// longer fails them, and one that breaks it does.

type BarProps = {
  count: number;
  enabledCount: number;
  disabledCount: number;
  canEdit: boolean;
  canManage: boolean;
  tagLoading?: boolean;
  statusAction?: "enable" | "disable" | null;
  deleteLoading?: boolean;
};

const baseProps: BarProps = {
  count: 3,
  enabledCount: 2,
  disabledCount: 1,
  canEdit: true,
  canManage: true,
};

const mounted: VueWrapper[] = [];

function mountBar(overrides: Partial<BarProps> = {}) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(FAQBatchBar, {
    props: { ...baseProps, ...overrides },
    global: { plugins: [i18n] },
    // The confirmation dialog is teleported to <body>; attaching the bar to
    // the document keeps the portal target and the bar in the same tree.
    attachTo: document.body,
  });
  mounted.push(wrapper);
  return wrapper;
}

// Buttons are found by their visible label, which is what the user reads,
// rather than by a class name the markup is free to change.
function findButton(wrapper: VueWrapper, label: string) {
  return wrapper.findAll("button").find((b) => b.text().includes(label));
}

function bodyButton(label: string): HTMLButtonElement | undefined {
  return Array.from(document.body.querySelectorAll("button")).find((b) => b.textContent?.includes(label));
}

afterEach(() => {
  while (mounted.length) mounted.pop()!.unmount();
  document.body.innerHTML = "";
});

const faq = enUS.knowledgeEditor.faq;

test("the bar stays hidden without a selection or without any permission", () => {
  assert.equal(mountBar({ count: 0 }).find('[role="region"]').exists(), false);
  assert.equal(mountBar({ canEdit: false, canManage: false }).find('[role="region"]').exists(), false);
  assert.equal(mountBar().find('[role="region"]').exists(), true);
});

test("enable and disable are offered only when they would change something", () => {
  const onlyDisabled = mountBar({ enabledCount: 0, disabledCount: 3 });
  assert.ok(findButton(onlyDisabled, faq.batchEnable));
  assert.equal(findButton(onlyDisabled, faq.batchDisable), undefined);

  const onlyEnabled = mountBar({ enabledCount: 3, disabledCount: 0 });
  assert.equal(findButton(onlyEnabled, faq.batchEnable), undefined);
  assert.ok(findButton(onlyEnabled, faq.batchDisable));
});

test("edit actions need canEdit and delete needs canManage", () => {
  const manageOnly = mountBar({ canEdit: false, canManage: true });
  assert.equal(findButton(manageOnly, faq.batchUpdateTag), undefined);
  assert.equal(findButton(manageOnly, faq.batchEnable), undefined);
  assert.equal(findButton(manageOnly, faq.batchDisable), undefined);
  assert.ok(findButton(manageOnly, faq.batchDelete));

  const editOnly = mountBar({ canEdit: true, canManage: false });
  assert.ok(findButton(editOnly, faq.batchUpdateTag));
  assert.equal(findButton(editOnly, faq.batchDelete), undefined);
});

test("each action is reported to the parent as its own event", async () => {
  const wrapper = mountBar();
  await findButton(wrapper, enUS.knowledgeBase.clearSelection)!.trigger("click");
  await findButton(wrapper, faq.batchUpdateTag)!.trigger("click");
  await findButton(wrapper, faq.batchEnable)!.trigger("click");
  await findButton(wrapper, faq.batchDisable)!.trigger("click");
  assert.equal(wrapper.emitted("cancel")?.length, 1);
  assert.equal(wrapper.emitted("batchTag")?.length, 1);
  assert.equal(wrapper.emitted("enable")?.length, 1);
  assert.equal(wrapper.emitted("disable")?.length, 1);
});

test("delete asks for confirmation first and emits only once confirmed", async () => {
  const wrapper = mountBar();
  await findButton(wrapper, faq.batchDelete)!.trigger("click");
  await nextTick();
  assert.equal(wrapper.emitted("delete"), undefined);

  const confirm = bodyButton(enUS.knowledgeBase.confirmDelete);
  assert.ok(confirm, "the confirmation dialog shows a confirm button");
  confirm.click();
  await nextTick();
  assert.equal(wrapper.emitted("delete")?.length, 1);
});

test("while any batch request runs, every action is disabled against double submits", () => {
  for (const loading of [{ tagLoading: true }, { statusAction: "enable" as const }, { deleteLoading: true }]) {
    const wrapper = mountBar(loading);
    for (const label of [
      enUS.knowledgeBase.clearSelection,
      faq.batchUpdateTag,
      faq.batchEnable,
      faq.batchDisable,
      faq.batchDelete,
    ]) {
      const button = findButton(wrapper, label);
      assert.ok(button, `${label} is rendered`);
      assert.equal(button.attributes("disabled"), "", `${label} is disabled while ${JSON.stringify(loading)}`);
    }
  }
});

test("every supported locale carries the FAQ batch action strings", () => {
  for (const locale of [zhCN, enUS, koKR, ruRU]) {
    const strings = locale.knowledgeEditor.faq;
    for (const key of ["batchEnable", "batchDisable", "batchDelete", "confirmBatchDelete", "batchDeleteSuccess"]) {
      assert.equal(typeof strings[key as keyof typeof strings], "string", key);
    }
  }
});
