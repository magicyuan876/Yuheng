import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import { reactive } from "vue";
import enUS from "@/i18n/locales/en-US";
import WorkspaceOnboarding from "./WorkspaceOnboarding.vue";

// The page a user without a workspace lands on. Ordinary users are told to
// ask to be invited or added and get their invitation inbox; only a system
// administrator (can_create_tenant from /auth/me) is offered the create
// button. The copy was the "self-service creation is disabled" wording
// before; nothing is disabled now, it is simply not theirs to do.

const replace = vi.fn();
vi.mock("vue-router", () => ({ useRouter: () => ({ replace }) }));
vi.mock("@/api/auth", () => ({ logout: vi.fn() }));
vi.mock("@/components/CreateTenantDialog.vue", async () => {
  const { defineComponent, h } = await import("vue");
  return { default: defineComponent({ setup: () => () => h("div") }) };
});
vi.mock("@/components/MyInvitationsDialog.vue", async () => {
  const { defineComponent, h } = await import("vue");
  return { default: defineComponent({ setup: () => () => h("div") }) };
});

const authStore = reactive({
  canCreateTenant: false,
  hasValidTenant: false,
  pendingInvitationCount: 0,
  refreshFromAuthMe: vi.fn(async () => true),
  fetchPendingInvitationCount: vi.fn(async () => undefined),
  setSelectedTenant: vi.fn(),
  logout: vi.fn(),
});
vi.mock("@/stores/auth", () => ({ useAuthStore: () => authStore }));

const mounted: VueWrapper[] = [];

beforeEach(() => {
  authStore.canCreateTenant = false;
  authStore.pendingInvitationCount = 0;
});

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount());
});

async function mountPage() {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(WorkspaceOnboarding, { global: { plugins: [i18n] } });
  mounted.push(wrapper);
  await flushPromises();
  return wrapper;
}

test("an ordinary user is told to ask for an invitation and sees no create button", async () => {
  authStore.pendingInvitationCount = 2;
  const wrapper = await mountPage();

  assert.ok(wrapper.text().includes(enUS.auth.workspaceOnboarding.title));
  assert.equal(wrapper.find('[data-slot="onboarding-description"]').text(), enUS.auth.workspaceOnboarding.description);
  assert.equal(wrapper.find('[data-slot="onboarding-help"]').text(), enUS.auth.workspaceOnboarding.help);
  assert.equal(wrapper.find('[data-slot="onboarding-create"]').exists(), false);
  assert.match(wrapper.find('[data-slot="onboarding-invitations"]').text(), /View invitations.*\(2\)/s);
});

test("a system administrator is offered workspace creation", async () => {
  authStore.canCreateTenant = true;
  const wrapper = await mountPage();

  assert.equal(
    wrapper.find('[data-slot="onboarding-description"]').text(),
    enUS.auth.workspaceOnboarding.adminDescription,
  );
  assert.ok(wrapper.find('[data-slot="onboarding-create"]').exists());
  assert.equal(wrapper.find('[data-slot="onboarding-help"]').text(), enUS.auth.workspaceOnboarding.adminHelp);
});
