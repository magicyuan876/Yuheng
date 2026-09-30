import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { h } from "vue";
import { createI18n } from "vue-i18n";
import { TooltipProvider } from "@/components/ui/tooltip";
import enUS from "@/i18n/locales/en-US";
import { useWebSearchToggle } from "@/composables/useWebSearchToggle";
import { useSettingsStore } from "@/stores/settings";
import WebSearchToggle from "./WebSearchToggle.vue";

// The chat input's web-search switch: it exists only where a chat turn can
// actually search the web, it remembers the user's choice, and a request
// carries the flag only while web search is available.

const api = vi.hoisted(() => ({
  listWebSearchProviders: vi.fn(),
  getDeploymentCapabilities: vi.fn(),
}));

vi.mock("@/api/web-search-provider", () => ({ listWebSearchProviders: api.listWebSearchProviders }));
vi.mock("@/api/system", () => ({ getDeploymentCapabilities: api.getDeploymentCapabilities }));

const mounted: VueWrapper[] = [];

beforeEach(() => {
  localStorage.clear();
  setActivePinia(createPinia());
  api.listWebSearchProviders.mockReset();
  api.getDeploymentCapabilities.mockReset();
  api.getDeploymentCapabilities.mockResolvedValue({ data: { capabilities: {} } });
});

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount());
  document.body.innerHTML = "";
});

async function mountToggle() {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  // The app provides the tooltip context at its root (AppProviders).
  const wrapper = mount(TooltipProvider, {
    slots: { default: () => h(WebSearchToggle) },
    global: { plugins: [i18n] },
    attachTo: document.body,
  });
  mounted.push(wrapper);
  await flushPromises();
  return wrapper;
}

const toggle = (wrapper: VueWrapper) => wrapper.find('[data-slot="web-search-toggle"]');

test("no default provider, no switch", async () => {
  api.listWebSearchProviders.mockResolvedValue({ data: [{ id: "p1", name: "Bing", is_default: false }] });
  const wrapper = await mountToggle();
  assert.equal(toggle(wrapper).exists(), false);
});

test("a deployment without web search shows no switch and does not ask for providers", async () => {
  api.getDeploymentCapabilities.mockResolvedValue({
    data: { capabilities: { "settings.websearch": { supported: false } } },
  });
  const wrapper = await mountToggle();
  assert.equal(toggle(wrapper).exists(), false);
  assert.equal(api.listWebSearchProviders.mock.calls.length, 0);
});

test("with a default provider the switch appears, toggles and persists the choice", async () => {
  api.listWebSearchProviders.mockResolvedValue({ data: [{ id: "p1", name: "Bing", is_default: true }] });
  const wrapper = await mountToggle();

  assert.ok(toggle(wrapper).exists());
  assert.equal(toggle(wrapper).attributes("aria-pressed"), "false");

  await toggle(wrapper).trigger("click");
  assert.equal(toggle(wrapper).attributes("aria-pressed"), "true");
  assert.equal(useSettingsStore().isWebSearchEnabled, true);
  assert.equal(JSON.parse(localStorage.getItem("Yuheng_settings") || "{}").webSearchEnabled, true);
  assert.equal(useWebSearchToggle().requested.value, true);
});

test("a remembered 'on' is not sent where web search is unavailable", async () => {
  api.listWebSearchProviders.mockResolvedValue({ data: [] });
  useSettingsStore().setWebSearchEnabled(true);
  const webSearch = useWebSearchToggle();
  await webSearch.ensureLoaded();

  assert.equal(webSearch.enabled.value, true);
  assert.equal(webSearch.available.value, false);
  assert.equal(webSearch.requested.value, false);
});

test("opening an old session restores the switch it was sent with", () => {
  const settings = useSettingsStore();
  settings.setWebSearchEnabled(false);
  settings.snapshotAsDefaultsIfNeeded();
  settings.applyLastRequestState({ web_search_enabled: true });
  assert.equal(settings.isWebSearchEnabled, true);

  settings.restoreDefaultsIfSnapshotted();
  assert.equal(settings.isWebSearchEnabled, false, "leaving the session brings the user's default back");
});
