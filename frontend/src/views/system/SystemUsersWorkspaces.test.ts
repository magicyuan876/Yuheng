import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import SystemUsersWorkspaces from "./SystemUsersWorkspaces.vue";

// The system administrator's Users & workspaces page, mounted: the
// workspace catalog with member counts, the members drawer for a
// workspace the administrator is not in, and the create-user dialog that
// can place the account straight into a workspace.

const api = vi.hoisted(() => ({
  listAllTenants: vi.fn(),
  listWorkspaceMembers: vi.fn(),
  addWorkspaceMember: vi.fn(),
  updateWorkspaceMemberRole: vi.fn(),
  removeWorkspaceMember: vi.fn(),
  createSystemUser: vi.fn(),
  listSystemAdmins: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
}));

vi.mock("@/api/tenant", () => ({ listAllTenants: api.listAllTenants, createTenant: vi.fn() }));
vi.mock("@/api/system", () => ({
  listWorkspaceMembers: api.listWorkspaceMembers,
  addWorkspaceMember: api.addWorkspaceMember,
  updateWorkspaceMemberRole: api.updateWorkspaceMemberRole,
  removeWorkspaceMember: api.removeWorkspaceMember,
  createSystemUser: api.createSystemUser,
  listSystemAdmins: api.listSystemAdmins,
  promoteUserToSystemAdmin: vi.fn(),
  revokeSystemAdmin: vi.fn(),
  resetUserPassword: vi.fn(),
}));
vi.mock("@/stores/auth", () => ({ useAuthStore: () => ({ currentUserId: "admin" }) }));
vi.mock("@/utils/clipboard", () => ({ copyWithToast: vi.fn().mockResolvedValue(true) }));
vi.mock("tdesign-vue-next", () => ({ MessagePlugin: { success: api.success, error: api.error } }));

const mounted: VueWrapper[] = [];

beforeEach(() => {
  Object.values(api).forEach((fn) => fn.mockReset());
  api.listAllTenants.mockResolvedValue({
    success: true,
    data: {
      items: [
        { id: 1, name: "Acme", description: "Default", member_count: 3, created_at: "2026-09-01T08:00:00Z" },
        { id: 2, name: "Field Office", member_count: 1, created_at: "2026-09-02T08:00:00Z" },
      ],
    },
  });
  api.listSystemAdmins.mockResolvedValue({ total: 1, admins: [{ id: "admin", email: "admin@example.com" }] });
  api.listWorkspaceMembers.mockResolvedValue({
    success: true,
    data: {
      members: [
        {
          user_id: "u-lead",
          email: "lead@example.com",
          username: "lead",
          role: "owner",
          status: "active",
          joined_at: "2026-09-02T08:00:00Z",
        },
      ],
      total: 1,
    },
  });
});

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount());
  document.body.innerHTML = "";
});

async function mountPage() {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(SystemUsersWorkspaces, { global: { plugins: [i18n] }, attachTo: document.body });
  mounted.push(wrapper);
  await flushPromises();
  return wrapper;
}

async function click(el: Element | null | undefined) {
  assert.ok(el instanceof HTMLElement, "the element to click exists");
  el.click();
  await flushPromises();
}

async function type(el: Element | null, value: string) {
  assert.ok(el instanceof HTMLInputElement, "the input exists");
  el.value = value;
  el.dispatchEvent(new Event("input"));
  await flushPromises();
}

test("the catalog lists every workspace with its member count", async () => {
  const wrapper = await mountPage();
  const rows = wrapper.findAll('[data-slot="workspace-row"]');
  assert.equal(rows.length, 2);
  assert.ok(rows[0].text().includes("Acme"));
  assert.ok(rows[0].text().includes("Default"));
  assert.ok(rows[0].text().includes("3"));
  assert.ok(rows[1].text().includes("Field Office"));
});

test("the members drawer lists a workspace's roster and adds an account by email", async () => {
  api.addWorkspaceMember.mockResolvedValue({ success: true });
  const wrapper = await mountPage();
  await click(wrapper.findAll('[data-slot="workspace-members"]')[1].element);

  assert.deepEqual(api.listWorkspaceMembers.mock.calls[0], [2, { page: 1, page_size: 20 }]);
  const drawer = document.body.querySelector('[data-slot="drawer-content"]');
  assert.ok(drawer, "the drawer is open");
  assert.ok(drawer.textContent?.includes("Members of Field Office"));
  const row = drawer.querySelector('[data-slot="admin-member-row"]');
  assert.ok(row?.textContent?.includes("lead@example.com"));

  await type(drawer.querySelector("#admin-member-email"), " lonely@example.com ");
  await click(drawer.querySelector('[data-slot="admin-member-add"]'));

  assert.deepEqual(api.addWorkspaceMember.mock.calls[0], [2, { email: "lonely@example.com", role: "viewer" }]);
  // The roster and the catalog's member counts are both refreshed.
  assert.equal(api.listWorkspaceMembers.mock.calls.length, 2);
  assert.equal(api.listAllTenants.mock.calls.length, 2);
});

test("an unknown email is explained rather than reported as a generic failure", async () => {
  api.addWorkspaceMember.mockRejectedValue({ status: 404, message: "user with this email is not registered" });
  const wrapper = await mountPage();
  await click(wrapper.findAll('[data-slot="workspace-members"]')[0].element);
  const drawer = document.body.querySelector('[data-slot="drawer-content"]');
  assert.ok(drawer);

  await type(drawer.querySelector("#admin-member-email"), "ghost@example.com");
  await click(drawer.querySelector('[data-slot="admin-member-add"]'));

  assert.ok(drawer.textContent?.includes(enUS.system.usersWorkspaces.members.notRegistered));
});

test("creating a user can place the account into a workspace", async () => {
  api.createSystemUser.mockResolvedValue({
    user: { id: "u9", username: "dana", email: "dana@example.com" },
    membership: { user_id: "u9", email: "dana@example.com", username: "dana", role: "contributor", status: "active" },
  });
  const wrapper = await mountPage();
  await click(wrapper.find('[data-slot="user-create"]').element);
  const dialog = document.body.querySelector('[role="dialog"]');
  assert.ok(dialog, "the create-user dialog is open");

  await type(dialog.querySelector("#create-user-username"), "dana");
  await type(dialog.querySelector("#create-user-email"), "dana@example.com");
  await type(dialog.querySelector("#create-user-password"), "PlainPass9");
  // Pick a workspace. Reka's Select opens on pointerdown or from the
  // keyboard, not on click, and an item selects on pointerup; happy-dom
  // dispatches the keyboard path and plain events faithfully.
  const trigger = dialog.querySelector("#create-user-workspace");
  assert.ok(trigger);
  trigger.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true }));
  await flushPromises();
  const option = Array.from(document.body.querySelectorAll('[role="option"]')).find((o) =>
    o.textContent?.includes("Field Office"),
  );
  assert.ok(option, "the workspace list is open");
  option.dispatchEvent(new Event("pointerup", { bubbles: true }));
  await flushPromises();

  const submit = Array.from(dialog.querySelectorAll("button")).find(
    (b) => b.textContent?.trim() === enUS.system.usersWorkspaces.users.submit,
  );
  await click(submit);

  assert.deepEqual(api.createSystemUser.mock.calls[0], [
    { username: "dana", email: "dana@example.com", password: "PlainPass9", tenant_id: 2, role: "viewer" },
  ]);
  // The new member changes a workspace's count, so the catalog reloads.
  assert.equal(api.listAllTenants.mock.calls.length, 2);
  assert.equal(document.body.querySelector('[role="dialog"]'), null, "the dialog closes on success");
});

test("a generated password is shown once instead of closing the dialog", async () => {
  api.createSystemUser.mockResolvedValue({
    user: { id: "u9", username: "dana", email: "dana@example.com" },
    generated_password: "G3n3r4t3dP4ssw0rd",
  });
  const wrapper = await mountPage();
  await click(wrapper.find('[data-slot="user-create"]').element);
  const dialog = document.body.querySelector('[role="dialog"]');
  assert.ok(dialog);
  await type(dialog.querySelector("#create-user-username"), "dana");
  await type(dialog.querySelector("#create-user-email"), "dana@example.com");
  const submit = Array.from(dialog.querySelectorAll("button")).find(
    (b) => b.textContent?.trim() === enUS.system.usersWorkspaces.users.submit,
  );
  await click(submit);

  assert.deepEqual(api.createSystemUser.mock.calls[0], [{ username: "dana", email: "dana@example.com" }]);
  const shown = document.body.querySelector<HTMLTextAreaElement>('[data-slot="generated-password"]');
  assert.ok(shown, "the one-time password view is open");
  assert.equal(shown.value, "G3n3r4t3dP4ssw0rd");
});
