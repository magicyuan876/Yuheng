import assert from "node:assert/strict";
import { afterEach, test, vi } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import { computed, nextTick } from "vue";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import type { ModelConfig } from "@/api/model";
import ModelSelector from "./ModelSelector.vue";

// These tests replace the part of modelSelectorClearability.test.ts that
// regex-matched ModelSelector's TDesign source. They mount the selector and
// check what the parent receives: clearing sends an empty string, the
// selector is not clearable unless asked, and the "add model" entry raises
// its own event instead of becoming the selected value.

// The add-model entry is gated on a store-backed permission; the tests pin
// it open so they need no Pinia store.
vi.mock("@/composables/usePlatformInfraAccess", () => ({
  usePlatformInfraAccess: () => computed(() => true),
}));

const models = [
  { id: "m1", name: "qwen-plus", display_name: "Qwen Plus", type: "KnowledgeQA", source: "remote" },
  { id: "m2", name: "deepseek-chat", type: "KnowledgeQA", source: "remote" },
] as unknown as ModelConfig[];

const mounted: VueWrapper[] = [];

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
  document.body.innerHTML = "";
});

function mountSelector(props: Record<string, unknown> = {}) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(ModelSelector, {
    props: { modelType: "KnowledgeQA", allModels: models, ...props },
    global: { plugins: [i18n] },
    // The option list is teleported to <body>.
    attachTo: document.body,
  });
  mounted.push(wrapper);
  return wrapper;
}

async function openList(wrapper: VueWrapper) {
  await wrapper.get('[data-slot="model-selector-trigger"]').trigger("click");
  await nextTick();
  await nextTick();
}

function bodyOption(label: string): HTMLElement | undefined {
  return Array.from(document.body.querySelectorAll<HTMLElement>('[role="option"]')).find((el) =>
    el.textContent?.includes(label),
  );
}

test("the selector is not clearable by default", () => {
  const wrapper = mountSelector({ selectedModelId: "m1" });
  assert.equal(wrapper.find('[data-slot="model-selector-clear"]').exists(), false);
});

test("clearing a clearable selector sends an empty string to the parent", async () => {
  const wrapper = mountSelector({ selectedModelId: "m1", clearable: true });
  await wrapper.get('[data-slot="model-selector-clear"]').trigger("click");
  assert.deepEqual(wrapper.emitted("update:selectedModelId"), [[""]]);
});

test("the trigger shows the display name of the selected model", () => {
  const wrapper = mountSelector({ selectedModelId: "m1" });
  assert.match(wrapper.get('[data-slot="model-selector-trigger"]').text(), /Qwen Plus/);
});

test("choosing a model emits its id", async () => {
  const wrapper = mountSelector();
  await openList(wrapper);
  const option = bodyOption("deepseek-chat");
  assert.ok(option, "the option list should render the models");
  option.click();
  await nextTick();
  assert.deepEqual(wrapper.emitted("update:selectedModelId"), [["m2"]]);
});

test("the add-model entry raises add-model and leaves the selection alone", async () => {
  const wrapper = mountSelector();
  await openList(wrapper);
  const option = bodyOption(enUS.model.addModelInSettings);
  assert.ok(option, "the add-model entry should be listed");
  option.click();
  await nextTick();
  assert.equal(wrapper.emitted("add-model")?.length, 1);
  assert.equal(wrapper.emitted("update:selectedModelId"), undefined);
});
