import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import { h, reactive } from "vue";
import enUS from "@/i18n/locales/en-US";
import { TooltipProvider } from "@/components/ui/tooltip";
import UserMenu from "./UserMenu.vue";

// The avatar menu at the foot of the sidebar. These tests mount it and check
// what the user can reach from it; the logout entry in particular was
// invisible for a while (it sat in a directive-less <template>, which Vue 3
// renders as an inert native element), and nothing noticed.

const logoutApi = vi.fn();
vi.mock("@/api/auth", () => ({
  getCurrentUser: vi.fn(async () => ({
    success: true,
    data: { user: { id: 1, username: "Ada", email: "ada@example.com", avatar: "" } },
  })),
  logout: (...args: unknown[]) => logoutApi(...args),
  userInfoFromApi: (user: unknown) => user,
}));

const push = vi.fn();
vi.mock("vue-router", () => ({
  useRouter: () => ({ push }),
}));

const authStore = reactive({
  selectedTenantName: "",
  tenant: { name: "Ada's space" },
  currentTenantRole: "owner",
  canAccessAllTenants: false,
  isSystemAdmin: false,
  canCreateTenant: false,
  memberships: [] as { tenant_id: number; tenant_name?: string; role: string }[],
  effectiveTenantId: 1,
  hasRole: () => true,
  setUser: vi.fn(),
  setTenant: vi.fn(),
  setMemberships: vi.fn(),
  setCanCreateTenant: vi.fn(),
  refreshFromAuthMe: vi.fn(),
  logout: vi.fn(),
});
vi.mock("@/stores/auth", () => ({ useAuthStore: () => authStore }));

const uiStore = reactive({ sidebarCollapsed: false, openSettings: vi.fn() });
vi.mock("@/stores/ui", () => ({ useUIStore: () => uiStore }));

vi.mock("@/composables/useRoleLabel", () => ({
  useRoleLabel: () => ({ formatRole: (role: string) => role, roleIcon: () => null }),
  useHomeTenant: () => ({ homeTenantId: { value: 1 }, isHomeTenant: () => false }),
}));

vi.mock("@/config/contextualGuides", () => ({ openNewUserGuide: vi.fn() }));

vi.mock("tdesign-vue-next", () => ({
  MessagePlugin: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}));

// The create-workspace dialog is its own component with its own tests' worth
// of behaviour; here it only has to exist.
vi.mock("@/components/CreateTenantDialog.vue", async () => {
  const { defineComponent, h } = await import("vue");
  return { default: defineComponent({ setup: () => () => h("div") }) };
});

const mounted: VueWrapper[] = [];

beforeEach(() => {
  logoutApi.mockReset();
  push.mockReset();
  authStore.logout.mockReset();
  uiStore.sidebarCollapsed = false;
});

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount());
  document.body.innerHTML = "";
});

async function mountMenu() {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(TooltipProvider, {
    slots: { default: () => h(UserMenu) },
    global: { plugins: [i18n] },
    attachTo: document.body,
  });
  mounted.push(wrapper);
  await flushPromises();
  return wrapper;
}

function entryByText(wrapper: VueWrapper, text: string) {
  const entry = wrapper.findAll("div").find((d) => d.element.children.length <= 3 && d.text() === text);
  assert.ok(entry, `no menu entry reading "${text}"`);
  return entry;
}

test("the user button is the onboarding tour's target and opens the menu", async () => {
  const wrapper = await mountMenu();
  const button = wrapper.find('[data-guide="user-menu"]');
  assert.ok(button.exists());
  assert.match(button.text(), /Ada/);
  assert.equal(wrapper.text().includes(enUS.auth.logout), false);

  await button.trigger("click");
  assert.ok(wrapper.text().includes(enUS.general.allSettings));
});

test("the menu offers logout, which ends the session and goes to the login page", async () => {
  logoutApi.mockResolvedValue({ success: true });
  const wrapper = await mountMenu();
  await wrapper.find('[data-guide="user-menu"]').trigger("click");

  await entryByText(wrapper, enUS.auth.logout).trigger("click");
  await flushPromises();

  assert.equal(logoutApi.mock.calls.length, 1);
  assert.equal(authStore.logout.mock.calls.length, 1);
  assert.deepEqual(push.mock.calls.at(-1), ["/login"]);
});

test("a settings entry opens the settings at its section", async () => {
  const wrapper = await mountMenu();
  await wrapper.find('[data-guide="user-menu"]').trigger("click");

  await entryByText(wrapper, enUS.general.personalSettings).trigger("click");

  assert.equal(uiStore.openSettings.mock.calls.length > 0, true);
  assert.deepEqual(push.mock.calls.at(-1), [{ path: "/platform/settings", query: { section: "general" } }]);
});
