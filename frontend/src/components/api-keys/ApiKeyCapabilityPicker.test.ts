import assert from "node:assert/strict";
import { afterEach, test } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import type { TenantAPIKeyCapability } from "@/api/tenant";
import {
  PLATFORM_API_KEY_CAPABILITY_GROUPS,
  TENANT_API_KEY_CAPABILITY_GROUPS,
  capabilitiesOf,
} from "@/config/apiKeyCapabilities";
import ApiKeyCapabilityPicker from "./ApiKeyCapabilityPicker.vue";
import { maskApiKey } from "./apiKeyDisplay";

const mounted: VueWrapper[] = [];

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount());
});

function mountPicker(modelValue: TenantAPIKeyCapability[], groups = TENANT_API_KEY_CAPABILITY_GROUPS) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(ApiKeyCapabilityPicker, {
    props: {
      modelValue,
      groups,
      "onUpdate:modelValue": (value: TenantAPIKeyCapability[]) => wrapper.setProps({ modelValue: value }),
    },
    global: { plugins: [i18n] },
  });
  mounted.push(wrapper);
  return wrapper;
}

const model = (wrapper: ReturnType<typeof mountPicker>) => wrapper.props().modelValue;

test("every capability of every group has a label and a hint in each group's section", () => {
  const wrapper = mountPicker([]);
  for (const capability of capabilitiesOf(TENANT_API_KEY_CAPABILITY_GROUPS)) {
    const box = wrapper.find(`#api-key-capability-${capability}`);
    assert.ok(box.exists(), `${capability} has a checkbox`);
    assert.ok(wrapper.text().includes(enUS.apiKeys.capabilities[capability].label));
  }
  assert.ok(wrapper.text().includes(enUS.apiKeys.capabilityGroups.dataSources));
});

test("ticking a box adds it in picker order; unticking removes it", async () => {
  const wrapper = mountPicker(["chat"]);
  await wrapper.find("#api-key-capability-retrieve").trigger("click");
  await flushPromises();
  assert.deepEqual(model(wrapper), ["retrieve", "chat"]);

  await wrapper.find("#api-key-capability-chat").trigger("click");
  await flushPromises();
  assert.deepEqual(model(wrapper), ["retrieve"]);
});

test("the group button selects the whole group, then clears it", async () => {
  const wrapper = mountPicker([]);
  const knowledgeGroupButton = () =>
    wrapper
      .findAll("button")
      .find((b) => b.text() === enUS.apiKeys.selectGroup || b.text() === enUS.apiKeys.clearGroup);
  // The first group is "knowledge".
  await knowledgeGroupButton()!.trigger("click");
  await flushPromises();
  assert.deepEqual(model(wrapper), ["retrieve", "chat", "ingest", "manage_kbs", "message_history"]);

  await knowledgeGroupButton()!.trigger("click");
  await flushPromises();
  assert.deepEqual(model(wrapper), []);
});

test("grants outside the groups shown are kept, not dropped", async () => {
  const onlyKnowledge = TENANT_API_KEY_CAPABILITY_GROUPS.filter((group) => group.key === "knowledge");
  const wrapper = mountPicker(["docs_read"], onlyKnowledge);
  await wrapper.find("#api-key-capability-retrieve").trigger("click");
  await flushPromises();
  assert.deepEqual(model(wrapper), ["retrieve", "docs_read"]);
});

test("the platform picker offers the docs capabilities too, so they reach the request", () => {
  // They used to be listed but missing from the flat list the form submitted.
  const all = capabilitiesOf(PLATFORM_API_KEY_CAPABILITY_GROUPS);
  for (const capability of ["docs_read", "docs_write", "docs_admin", "system_audit_read"] as const) {
    assert.ok(all.includes(capability), capability);
  }
});

test("maskApiKey never shows the middle of a key, and is idempotent", () => {
  const masked = maskApiKey("sk-live-0123456789abcdefghijWXYZ");
  assert.equal(masked, "sk-live...WXYZ");
  assert.equal(maskApiKey(masked), masked);
  assert.equal(maskApiKey("short"), "***");
  assert.equal(maskApiKey(undefined), "***");
});
