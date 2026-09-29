import assert from "node:assert/strict";
import { afterEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import CredentialResource, { type CredentialResourceApi } from "./CredentialResource.vue";

// Mounted tests for the credential card's three states — configured,
// unconfigured, editing — and the two-step remove. They drive the card the
// way a user does, by its visible labels, and check what reaches the api.

vi.mock("tdesign-vue-next", () => ({ MessagePlugin: { success: vi.fn(), error: vi.fn() } }));

type Field = "api_key";

const mounted: VueWrapper[] = [];

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
});

function mountCard(configured: boolean, api: Partial<CredentialResourceApi<Field>> = {}) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const fullApi: CredentialResourceApi<Field> = {
    save: vi.fn(async () => ({ api_key: { configured: true } })),
    remove: vi.fn(async () => {}),
    ...api,
  };
  const wrapper = mount(CredentialResource<Field>, {
    props: { fields: [{ key: "api_key", label: "API key" }], api: fullApi, meta: { api_key: { configured } } },
    global: { plugins: [i18n] },
  });
  mounted.push(wrapper);
  return { wrapper, api: fullApi };
}

function button(wrapper: VueWrapper, label: string) {
  const found = wrapper.findAll("button").find((b) => b.text().includes(label));
  assert.ok(found, `expected a "${label}" button`);
  return found;
}

test("a configured credential shows its state and the update/remove actions", () => {
  const { wrapper } = mountCard(true);
  assert.match(wrapper.text(), new RegExp(enUS.credential.configured));
  button(wrapper, enUS.credential.update);
  button(wrapper, enUS.credential.remove);
  assert.equal(wrapper.find("input").exists(), false);
});

test("remove asks first, and only the confirmation calls the api", async () => {
  const { wrapper, api } = mountCard(true);
  await button(wrapper, enUS.credential.remove).trigger("click");
  assert.match(wrapper.text(), new RegExp(enUS.credential.confirmRemovePrompt.replace(/[?？]/g, ".")));
  assert.equal((api.remove as ReturnType<typeof vi.fn>).mock.calls.length, 0);
  await button(wrapper, enUS.credential.confirmRemove).trigger("click");
  await flushPromises();
  assert.deepEqual((api.remove as ReturnType<typeof vi.fn>).mock.calls, [["api_key"]]);
  assert.equal(wrapper.emitted("changed")?.length, 1);
  assert.match(wrapper.text(), new RegExp(enUS.credential.removedToast));
});

test("configuring saves the typed secret, Save staying disabled while the field is empty", async () => {
  const { wrapper, api } = mountCard(false);
  await button(wrapper, enUS.credential.configure).trigger("click");
  const input = wrapper.get('input[type="password"]');
  assert.equal(button(wrapper, enUS.common.save).attributes("disabled"), "");
  await input.setValue("sk-123");
  await button(wrapper, enUS.common.save).trigger("click");
  await flushPromises();
  assert.deepEqual((api.save as ReturnType<typeof vi.fn>).mock.calls, [[{ api_key: "sk-123" }]]);
  assert.match(wrapper.text(), new RegExp(enUS.credential.configured));
  assert.equal(wrapper.emitted("changed")?.length, 1);
});

test("cancelling an edit returns to the state the parent reported", async () => {
  const { wrapper } = mountCard(true);
  await button(wrapper, enUS.credential.update).trigger("click");
  assert.ok(wrapper.find('input[type="password"]').exists());
  await button(wrapper, enUS.common.cancel).trigger("click");
  assert.equal(wrapper.find("input").exists(), false);
  assert.match(wrapper.text(), new RegExp(enUS.credential.configured));
});
