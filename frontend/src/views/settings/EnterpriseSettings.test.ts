import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import zhCN from "@/i18n/locales/zh-CN";

const getDeploymentCapabilities = vi.hoisted(() => vi.fn());
vi.mock("@/api/system", () => ({ getDeploymentCapabilities }));

import EnterpriseSettings from "./EnterpriseSettings.vue";
import { ENTERPRISE_FEATURES } from "@/config/enterpriseFeatures";

beforeEach(() => {
  setActivePinia(createPinia());
  getDeploymentCapabilities.mockReset();
  getDeploymentCapabilities.mockResolvedValue({ data: { capabilities: {}, extensions: {} } });
});
afterEach(() => {
  delete window.__RUNTIME_CONFIG__;
});

async function render(locale: "en-US" | "zh-CN" = "en-US") {
  const i18n = createI18n({ legacy: false, locale, messages: { "en-US": enUS, "zh-CN": zhCN } });
  const w = mount(EnterpriseSettings, { global: { plugins: [i18n] } });
  await flushPromises();
  return w;
}

test("lists every catalog feature as a card, honestly marked as planned", async () => {
  const w = await render();
  assert.equal(w.get("h2").text(), "Enterprise");
  const cards = w.findAll("article");
  assert.deepEqual(
    cards.map((c) => c.attributes("data-feature")),
    ENTERPRISE_FEATURES.map((f) => f.key),
  );
  for (const card of cards) {
    assert.equal(card.get("[data-testid='stage']").text(), "Planned");
    assert.equal(card.find("[data-cta]").exists(), false);
  }
  assert.match(w.text(), /free and open source under the MIT license/);
});

test("the edition link appears only when a URL is configured", async () => {
  assert.equal((await render()).find("[data-testid='edition-link']").exists(), false);
  window.__RUNTIME_CONFIG__ = { ENTERPRISE_INFO_URL: "https://example.com/e" };
  const w = await render();
  const link = w.get("[data-testid='edition-link']");
  assert.equal(link.attributes("href"), "https://example.com/e");
  assert.equal(link.attributes("rel"), "noopener noreferrer");
  // Planned cards get the neutral "learn more" link once a URL exists.
  assert.equal(w.findAll("a[data-cta='learn-more']").length, ENTERPRISE_FEATURES.length);
});

test("an enabled feature is shown as enabled, and the page renders in Chinese", async () => {
  getDeploymentCapabilities.mockResolvedValue({
    data: { extensions: { "enterprise.saml": { supported: true } } },
  });
  const w = await render("zh-CN");
  assert.equal(w.get("h2").text(), "企业版");
  const saml = w.get("article[data-feature='enterprise.saml']");
  assert.equal(saml.get("[data-testid='stage']").text(), "已启用");
});
