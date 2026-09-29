import assert from "node:assert/strict";
import { afterEach, test } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import { LockIcon } from "@lucide/vue";
import enUS from "@/i18n/locales/en-US";
import SettingsInput from "./SettingsInput.vue";
import SettingsSelectClear from "./SettingsSelectClear.vue";

// The settings drawers lost TDesign's `clearable` and `prefix-icon` when
// t-input became the shadcn Input, which has neither — the lock icons had
// silently disappeared and fields could no longer be cleared. These tests
// pin down the two small helpers that bring both back.

const mounted: VueWrapper[] = [];

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount());
});

const i18n = () => createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });

function mountInput(props: Record<string, unknown>, attrs: Record<string, unknown> = {}) {
  const wrapper = mount(SettingsInput, { props, attrs, global: { plugins: [i18n()] } });
  mounted.push(wrapper);
  return wrapper;
}

test("forwards input attributes and reports typing as a string", async () => {
  const wrapper = mountInput({ modelValue: "" }, { type: "password", placeholder: "***" });
  const input = wrapper.get("input");
  assert.equal(input.attributes("type"), "password");
  assert.equal(input.attributes("placeholder"), "***");

  await input.setValue("secret");
  assert.deepEqual(wrapper.emitted("update:modelValue")?.at(-1), ["secret"]);
});

test("renders the prefix icon and pads the input for it", () => {
  const wrapper = mountInput({ modelValue: "", prefixIcon: LockIcon });
  assert.ok(wrapper.find("svg").exists(), "the lock icon is rendered");
  assert.match(wrapper.get("input").classes().join(" "), /\bpl-8\b/);
});

test("offers a clear button only for a non-empty, editable, clearable field", async () => {
  const wrapper = mountInput({ modelValue: "", clearable: true });
  assert.equal(wrapper.find('[data-slot="input-clear"]').exists(), false, "empty: no clear button");

  await wrapper.setProps({ modelValue: "minio.example.com" });
  const clear = wrapper.get('[data-slot="input-clear"]');
  assert.equal(clear.attributes("aria-label"), "Clear");

  await clear.trigger("click");
  assert.deepEqual(wrapper.emitted("update:modelValue")?.at(-1), [""]);
  assert.equal(wrapper.emitted("clear")?.length, 1);

  await wrapper.setProps({ disabled: true });
  assert.equal(wrapper.find('[data-slot="input-clear"]').exists(), false, "disabled: no clear button");
});

test("the select clear button renders only when visible and emits clear", async () => {
  const wrapper = mount(SettingsSelectClear, { props: { visible: false }, global: { plugins: [i18n()] } });
  mounted.push(wrapper);
  assert.equal(wrapper.find("button").exists(), false);

  await wrapper.setProps({ visible: true });
  await wrapper.get("button").trigger("click");
  assert.equal(wrapper.emitted("clear")?.length, 1);
});
