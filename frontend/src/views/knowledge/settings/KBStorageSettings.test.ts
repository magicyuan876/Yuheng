import assert from "node:assert/strict";
import { afterEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import KBStorageSettings from "./KBStorageSettings.vue";

// The storage binding decides only where new files go, so a knowledge base
// with files may change it; the panel explains that instead of locking the
// choice.

vi.mock("@/api/storage-backend", () => ({
  listStorageBackends: async () => ({
    success: true,
    default_storage_backend_id: "env",
    data: [
      { id: "env", name: "Deployment storage", provider: "local", source: "env", status: "active", config: {} },
      { id: "team", name: "Team S3", provider: "s3", source: "user", status: "active", config: {} },
    ],
  }),
}));
vi.mock("@/stores/ui", () => ({ useUIStore: () => ({}) }));
vi.mock("@/composables/usePlatformInfraAccess", () => ({ usePlatformInfraAccess: () => false }));

const mounted: VueWrapper[] = [];
afterEach(() => mounted.splice(0).forEach((w) => w.unmount()));

async function mountPanel(props: Record<string, unknown>) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(KBStorageSettings, { props, global: { plugins: [i18n] } });
  mounted.push(wrapper);
  await flushPromises();
  return wrapper;
}

const { rebindHint } = enUS.kbSettings.storage;

test("a knowledge base with files can be rebound, with a note on what that means", async () => {
  const wrapper = await mountPanel({ storageBackendId: "env", boundStorageBackendId: "env", hasFiles: true });

  const trigger = wrapper.find("button");
  assert.ok(trigger.exists(), "the backend picker is rendered");
  assert.equal(trigger.attributes("disabled"), undefined, "the picker is not locked");
  assert.ok(!wrapper.text().includes(rebindHint), "no note while the binding is unchanged");

  await wrapper.setProps({ storageBackendId: "team" });
  assert.ok(wrapper.text().includes(rebindHint), "choosing another backend explains the rebind");
});

test("a new knowledge base starts on the workspace default", async () => {
  const wrapper = await mountPanel({ storageBackendId: "", hasFiles: false });
  assert.deepEqual(wrapper.emitted("update:storageBackendId"), [["env"]]);
  assert.ok(wrapper.text().includes("Deployment storage"));
  assert.ok(!wrapper.text().includes(rebindHint));
});
