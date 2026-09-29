import assert from "node:assert/strict";
import { afterEach, beforeEach, test } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import EnterpriseGate from "./EnterpriseGate.vue";
import { useDeploymentCapabilitiesStore } from "@/stores/deploymentCapabilities";

const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
const KEY = "enterprise.saml";

beforeEach(() => {
  setActivePinia(createPinia());
});
afterEach(() => {
  delete window.__RUNTIME_CONFIG__;
});

function render(extensions: Record<string, { supported: boolean; reason?: string }>) {
  useDeploymentCapabilitiesStore().extensions = extensions;
  return mount(EnterpriseGate, {
    props: { feature: KEY },
    slots: { default: '<p data-testid="real">the real feature</p>' },
    global: { plugins: [i18n] },
  });
}

test("enabled: the slot, and no teaser", () => {
  const w = render({ [KEY]: { supported: true } });
  assert.ok(w.find("[data-testid='real']").exists());
  assert.equal(w.find("article").exists(), false);
});

test("unavailable and locked: the teaser, never the slot", () => {
  for (const extensions of [
    {} as Record<string, { supported: boolean }>,
    { [KEY]: { supported: false, reason: "license_required" } },
  ]) {
    const w = render(extensions);
    assert.equal(w.find("[data-testid='real']").exists(), false);
    assert.equal(w.get("article").attributes("data-feature"), KEY);
  }
});

test("a locked but planned feature is still labelled planned", () => {
  const w = render({ [KEY]: { supported: false, reason: "license_required" } });
  // A planned feature is not sold even when locked; it just is not enabled.
  assert.equal(w.get("[data-testid='stage']").text(), "Planned");
});

test("promotion off: teasers render nothing, enabled features still render", () => {
  window.__RUNTIME_CONFIG__ = { HIDE_ENTERPRISE_PROMOTION: true };
  assert.equal(render({}).find("article").exists(), false);
  assert.equal(render({ [KEY]: { supported: false } }).html(), "<!--v-if-->");
  const enabled = render({ [KEY]: { supported: true } });
  assert.ok(enabled.find("[data-testid='real']").exists());
});

test("an unknown feature key renders no teaser", () => {
  useDeploymentCapabilitiesStore().extensions = {};
  const w = mount(EnterpriseGate, { props: { feature: "enterprise.nope" }, global: { plugins: [i18n] } });
  assert.equal(w.find("article").exists(), false);
});
