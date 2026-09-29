import assert from "node:assert/strict";
import { afterEach, test } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import TagsFieldInput from "./TagsFieldInput.vue";

// TagsFieldInput replaces t-tag-input in the system settings, where every
// change asks for a confirmation and then calls the API. These tests pin the
// t-tag-input behaviour those consumers were written against.

const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });

let wrapper: VueWrapper | undefined;

function mountInput(modelValue: string[] = []) {
  wrapper = mount(TagsFieldInput, { props: { modelValue }, global: { plugins: [i18n] } });
  return wrapper;
}

function changes(w: VueWrapper): string[][] {
  return (w.emitted("change") ?? []).map((args) => args[0] as string[]);
}

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
});

test("Enter adds the trimmed text as a tag and empties the field", async () => {
  const w = mountInput(["a@example.com"]);
  const input = w.get("input");
  await input.setValue("  b@example.com ");
  await input.trigger("keydown", { key: "Enter" });

  assert.deepEqual(changes(w), [["a@example.com", "b@example.com"]]);
  assert.deepEqual(w.emitted("update:modelValue")?.[0]?.[0], ["a@example.com", "b@example.com"]);
  assert.equal((input.element as HTMLInputElement).value, "");
});

test("Enter on blank text adds nothing", async () => {
  const w = mountInput();
  const input = w.get("input");
  await input.setValue("   ");
  await input.trigger("keydown", { key: "Enter" });
  assert.deepEqual(changes(w), []);
});

test("leaving the field discards the typed text instead of adding it", async () => {
  const w = mountInput();
  const input = w.get("input");
  await input.setValue("c@example.com");
  await input.trigger("blur");

  assert.deepEqual(changes(w), []);
  assert.equal((input.element as HTMLInputElement).value, "");
});

test("a comma is ordinary text", async () => {
  const w = mountInput();
  const input = w.get("input");
  await input.setValue("a,b");
  await input.trigger("keydown", { key: "," });
  assert.deepEqual(changes(w), []);
});

test("the Enter that confirms an IME composition adds no tag", async () => {
  const w = mountInput();
  const input = w.get("input");
  await input.setValue("张三");
  await input.trigger("keydown", { key: "Enter", isComposing: true });
  await input.trigger("keydown", { key: "Enter", keyCode: 229 });
  assert.deepEqual(changes(w), []);
});

test("Backspace in an empty field removes the last tag", async () => {
  const w = mountInput(["a", "b"]);
  await w.get("input").trigger("keydown", { key: "Backspace" });
  assert.deepEqual(changes(w), [["a"]]);
});

test("a tag's remove button removes that tag only, even when its text repeats", async () => {
  const w = mountInput(["a", "b", "a"]);
  const removeButtons = w.findAll(`button[aria-label^="${enUS.common.delete}:"]`);
  assert.equal(removeButtons.length, 3);
  await removeButtons[2].trigger("click");
  assert.deepEqual(changes(w), [["a", "b"]]);
});

test("the clear button empties the list", async () => {
  const w = mountInput(["a", "b"]);
  await w.get(`button[aria-label="${enUS.common.clear}"]`).trigger("click");
  assert.deepEqual(changes(w), [[]]);
});

test("a disabled field offers no remove or clear buttons", () => {
  wrapper = mount(TagsFieldInput, { props: { modelValue: ["a"], disabled: true }, global: { plugins: [i18n] } });
  assert.equal(wrapper.findAll("button").length, 0);
  assert.equal(wrapper.get("input").attributes("disabled"), "");
});
