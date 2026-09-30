import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import type { TenantAPIKey } from "@/api/tenant";
import WorkspaceAPIKeys from "./WorkspaceAPIKeys.vue";

// The workspace API-key screen, mounted: what it lists (never a live
// secret), what it sends on create / edit / revoke, and that the secret is
// shown exactly once.

const api = vi.hoisted(() => ({
  listTenantAPIKeys: vi.fn(),
  createTenantAPIKey: vi.fn(),
  updateTenantAPIKey: vi.fn(),
  deleteTenantAPIKey: vi.fn(),
  listKnowledgeBases: vi.fn(),
  copyWithToast: vi.fn(),
  warning: vi.fn(),
}));

vi.mock("@/api/tenant", () => ({
  listTenantAPIKeys: api.listTenantAPIKeys,
  createTenantAPIKey: api.createTenantAPIKey,
  updateTenantAPIKey: api.updateTenantAPIKey,
  deleteTenantAPIKey: api.deleteTenantAPIKey,
}));
vi.mock("@/api/knowledge-base", () => ({ listKnowledgeBases: api.listKnowledgeBases }));
vi.mock("@/api/system", () => ({ getDeploymentCapabilities: vi.fn().mockResolvedValue({ data: {} }) }));
vi.mock("@/utils/clipboard", () => ({ copyWithToast: api.copyWithToast }));
vi.mock("@/stores/auth", () => ({ useAuthStore: () => ({ effectiveTenantId: 7 }) }));
vi.mock("tdesign-vue-next", () => ({
  MessagePlugin: { success: vi.fn(), error: vi.fn(), warning: api.warning },
}));

const SECRET = "sk-live-0123456789abcdefghijWXYZ";

function key(overrides: Partial<TenantAPIKey> = {}): TenantAPIKey {
  return {
    id: 11,
    name: "Support bot",
    api_key: SECRET,
    full_access: false,
    knowledge_base_ids: ["kb-1"],
    capabilities: ["retrieve", "chat"],
    created_at: "2026-09-01T08:00:00Z",
    ...overrides,
  };
}

const mounted: VueWrapper[] = [];

beforeEach(() => {
  setActivePinia(createPinia());
  Object.values(api).forEach((fn) => fn.mockReset());
  api.listTenantAPIKeys.mockResolvedValue({ success: true, data: [key()] });
  api.listKnowledgeBases.mockResolvedValue({ data: [{ id: "kb-1", name: "Handbook" }] });
  api.copyWithToast.mockResolvedValue(true);
});

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount());
  document.body.innerHTML = "";
});

async function mountScreen() {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(WorkspaceAPIKeys, { global: { plugins: [i18n] }, attachTo: document.body });
  mounted.push(wrapper);
  await flushPromises();
  return wrapper;
}

function button(label: string, root: ParentNode = document.body): HTMLButtonElement {
  const found = Array.from(root.querySelectorAll("button")).find((b) => b.textContent?.trim() === label);
  assert.ok(found, `a "${label}" button is shown`);
  return found;
}

async function click(el: HTMLElement) {
  el.click();
  await flushPromises();
}

test("the list shows a masked key, the grants and the knowledge-base scope", async () => {
  const wrapper = await mountScreen();

  assert.deepEqual(api.listTenantAPIKeys.mock.calls[0], [7]);
  const row = wrapper.find('[data-slot="api-key-row"]');
  assert.ok(row.exists());
  assert.ok(row.text().includes("sk-live...WXYZ"));
  assert.ok(!document.body.innerHTML.includes(SECRET), "the full key never reaches the DOM");
  assert.ok(row.text().includes(enUS.apiKeys.capabilities.retrieve.label));
  assert.ok(row.text().includes("Handbook"));
  assert.ok(row.text().includes(enUS.workspaceApiKeys.expiryNever));
});

test("full-access and expired keys say so", async () => {
  api.listTenantAPIKeys.mockResolvedValue({
    success: true,
    data: [key({ full_access: true, capabilities: [], knowledge_base_ids: [], expires_at: "2020-01-01T00:00:00Z" })],
  });
  const wrapper = await mountScreen();
  const row = wrapper.find('[data-slot="api-key-row"]');
  assert.ok(row.text().includes(enUS.workspaceApiKeys.fullAccess));
  assert.ok(row.text().includes(enUS.workspaceApiKeys.allKnowledgeBases));
  assert.ok(row.text().includes(enUS.workspaceApiKeys.expired));
});

test("creating a key sends the form and shows the secret once", async () => {
  api.createTenantAPIKey.mockResolvedValue({ success: true, data: { ...key(), token: SECRET } });
  const wrapper = await mountScreen();

  await click(wrapper.find('[data-slot="api-key-create"]').element as HTMLElement);
  const name = document.body.querySelector<HTMLInputElement>("#workspace-api-key-name");
  assert.ok(name, "the drawer is open");
  name.value = "  Search widget ";
  name.dispatchEvent(new Event("input"));
  await flushPromises();

  await click(button(enUS.workspaceApiKeys.create, document.querySelector('[data-slot="drawer-content"]')!));

  assert.equal(api.createTenantAPIKey.mock.calls.length, 1);
  const [tenantId, payload] = api.createTenantAPIKey.mock.calls[0];
  assert.equal(tenantId, 7);
  assert.equal(payload.name, "Search widget");
  assert.equal(payload.full_access, false);
  assert.deepEqual(payload.capabilities, ["retrieve", "chat"]);
  assert.equal(typeof payload.expires_at_unix, "number");

  const secret = document.body.querySelector<HTMLTextAreaElement>('[data-slot="api-key-secret"]');
  assert.ok(secret, "the secret dialog is open");
  assert.equal(secret.value, SECRET);

  await click(button(enUS.apiKeys.secret.copy));
  assert.deepEqual(api.copyWithToast.mock.calls[0], [SECRET, "apiKeys.secret.copySuccess"]);
  assert.equal(document.body.querySelector('[data-slot="api-key-secret"]'), null, "copying closes the dialog");
});

test("a blank name is refused before anything is sent", async () => {
  const wrapper = await mountScreen();
  await click(wrapper.find('[data-slot="api-key-create"]').element as HTMLElement);

  await click(button(enUS.workspaceApiKeys.create, document.querySelector('[data-slot="drawer-content"]')!));

  assert.equal(api.createTenantAPIKey.mock.calls.length, 0);
  assert.deepEqual(api.warning.mock.calls[0], [enUS.workspaceApiKeys.nameRequired]);
});

test("editing sends the key's scope and expiry back unchanged unless edited", async () => {
  const expiresAt = "2027-03-01T06:07:08Z";
  api.listTenantAPIKeys.mockResolvedValue({ success: true, data: [key({ expires_at: expiresAt })] });
  api.updateTenantAPIKey.mockResolvedValue({ success: true, data: key() });
  const wrapper = await mountScreen();

  await click(wrapper.find('[data-slot="api-key-edit"]').element as HTMLElement);
  await click(button(enUS.common.save, document.querySelector('[data-slot="drawer-content"]')!));

  assert.equal(api.updateTenantAPIKey.mock.calls.length, 1);
  assert.deepEqual(api.updateTenantAPIKey.mock.calls[0], [
    7,
    11,
    {
      name: "Support bot",
      full_access: false,
      capabilities: ["retrieve", "chat"],
      knowledge_base_ids: ["kb-1"],
      expires_at_unix: Math.floor(Date.parse(expiresAt) / 1000),
    },
  ]);
  assert.equal(document.body.querySelector('[data-slot="api-key-secret"]'), null, "an edit shows no secret");
});

test("revoking asks first, then deletes and reloads", async () => {
  api.deleteTenantAPIKey.mockResolvedValue({ success: true });
  await mountScreen();

  const trash = document.querySelector<HTMLElement>(`[aria-label="${enUS.workspaceApiKeys.revoke}"]`);
  assert.ok(trash);
  await click(trash);
  assert.equal(api.deleteTenantAPIKey.mock.calls.length, 0, "the trash button only opens the confirmation");

  await click(document.querySelector<HTMLElement>('[data-slot="api-key-revoke-confirm"]')!);
  assert.deepEqual(api.deleteTenantAPIKey.mock.calls[0], [7, 11]);
  assert.equal(api.listTenantAPIKeys.mock.calls.length, 2);
});
