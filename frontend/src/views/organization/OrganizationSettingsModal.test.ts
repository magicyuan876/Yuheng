import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import { defineComponent, h, reactive } from "vue";
import enUS from "@/i18n/locales/en-US";
import { TooltipProvider } from "@/components/ui/tooltip";
import OrganizationSettingsModal from "./OrganizationSettingsModal.vue";

// These tests replace an older one that regex-matched the component's source
// (its TDesign markup and Less included). They mount the settings modal and
// check the behaviour that test stood for: reviews go through the store and
// refresh the modal's data, the single content scroller is reset on
// navigation, and the page behind the modal is locked while it is open.

const getOrganization = vi.fn();
const listOrgShares = vi.fn();
const listJoinRequests = vi.fn();
vi.mock("@/api/organization", () => ({
  getOrganization: (...args: unknown[]) => getOrganization(...args),
  listOrgShares: (...args: unknown[]) => listOrgShares(...args),
  listJoinRequests: (...args: unknown[]) => listJoinRequests(...args),
  searchTenantsForInvite: vi.fn(async () => ({ success: true, data: [] })),
}));

vi.mock("vue-router", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

vi.mock("tdesign-vue-next", () => ({
  MessagePlugin: { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() },
}));

const org = {
  id: "org1",
  name: "Research",
  description: "",
  avatar: "",
  my_role: "admin",
  is_owner: true,
  owner_id: "u1",
  owner_tenant_id: 1,
  member_count: 1,
  pending_join_request_count: 1,
  invite_code: "ABC123",
  invite_code_validity_days: 7,
  member_limit: 50,
  require_approval: false,
  searchable: false,
};

const orgStore = reactive({
  currentOrganization: null as typeof org | null,
  currentMembers: [] as unknown[],
  organizations: [] as unknown[],
  error: "",
  setCurrentOrganization: vi.fn((o: typeof org) => {
    orgStore.currentOrganization = o;
  }),
  clearCurrentOrganizationContext: vi.fn(),
  fetchMembers: vi.fn(async () => {}),
  reviewOrganizationJoinRequest: vi.fn(async () => ({ success: true })),
  updateOrganization: vi.fn(async () => true),
  create: vi.fn(),
  changeMemberRole: vi.fn(),
  kickMember: vi.fn(),
  requestOrganizationRoleUpgrade: vi.fn(),
  inviteOrganizationMember: vi.fn(),
  refreshInviteCode: vi.fn(),
  unshareKnowledgeBase: vi.fn(),
});
vi.mock("@/stores/organization", () => ({ useOrganizationStore: () => orgStore }));

const authStore = reactive({
  currentUserId: "u1",
  canAccessAllTenants: false,
  hasRole: () => true,
});
vi.mock("@/stores/auth", () => ({ useAuthStore: () => authStore }));

const joinRequest = {
  id: "jr1",
  user_id: "u2",
  username: "Grace",
  email: "grace@example.com",
  message: "Let me in",
  request_type: "join",
  requested_role: "editor",
  status: "pending",
  created_at: "2026-01-02T00:00:00Z",
};

const mounted: VueWrapper[] = [];

beforeEach(() => {
  getOrganization.mockReset().mockResolvedValue({ success: true, data: { ...org } });
  listOrgShares.mockReset().mockResolvedValue({ success: true, data: { shares: [] } });
  listJoinRequests.mockReset().mockResolvedValue({ success: true, data: { requests: [joinRequest], total: 1 } });
  orgStore.currentOrganization = null;
  orgStore.fetchMembers.mockClear();
  orgStore.reviewOrganizationJoinRequest.mockClear();
  document.body.style.overflow = "";
});

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount());
  document.body.innerHTML = "";
});

// The modal is mounted inside the app's single TooltipProvider, as it is at
// runtime; `props` is reactive so a test can open and close it.
async function mountModal(initial: { visible: boolean; orgId?: string; mode?: "view" | "edit" | "create" }) {
  const props = reactive({ ...initial });
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const Host = defineComponent({
    setup: () => () => h(TooltipProvider, () => h(OrganizationSettingsModal, { ...props })),
  });
  const wrapper = mount(Host, { global: { plugins: [i18n] }, attachTo: document.body });
  mounted.push(wrapper);
  await flushPromises();
  return { wrapper, props };
}

function navItem(label: string): HTMLElement {
  const item = Array.from(document.body.querySelectorAll<HTMLElement>(".settings-nav > div")).find(
    (el) => el.textContent?.trim().startsWith(label) && el.classList.contains("cursor-pointer"),
  );
  assert.ok(item, `no navigation entry reading "${label}"`);
  return item;
}

function buttonByLabel(label: string, root: ParentNode = document.body): HTMLButtonElement {
  const button = Array.from(root.querySelectorAll<HTMLButtonElement>("button")).find(
    (b) => b.getAttribute("aria-label") === label,
  );
  assert.ok(button, `no button labelled "${label}"`);
  return button;
}

function buttonByText(text: string, root: ParentNode = document.body): HTMLButtonElement {
  const button = Array.from(root.querySelectorAll<HTMLButtonElement>("button")).find(
    (b) => b.textContent?.trim() === text,
  );
  assert.ok(button, `no button reading "${text}"`);
  return button;
}

function openPopover(): HTMLElement {
  const popover = document.body.querySelector<HTMLElement>('[data-slot="popover-content"]');
  assert.ok(popover, "a popover is open");
  return popover;
}

async function openJoinRequests() {
  navItem(enUS.organization.settings.joinRequests).click();
  await flushPromises();
  assert.ok(document.body.textContent?.includes("Grace"), "the pending request is listed");
}

test("approving a join request goes through the store and refreshes the organization and its members", async () => {
  await mountModal({ visible: true, orgId: "org1", mode: "edit" });
  await openJoinRequests();
  const detailCalls = getOrganization.mock.calls.length;
  const memberCalls = orgStore.fetchMembers.mock.calls.length;

  buttonByLabel(enUS.organization.settings.approve).click();
  await flushPromises();
  // The role the applicant asked for is preselected; approving submits it.
  buttonByText(enUS.organization.settings.approve, openPopover()).click();
  await flushPromises();

  assert.deepEqual(orgStore.reviewOrganizationJoinRequest.mock.calls[0], [
    "org1",
    "jr1",
    { approved: true, role: "editor" },
    { requestType: "join" },
  ]);
  assert.equal(getOrganization.mock.calls.length, detailCalls + 1);
  assert.equal(orgStore.fetchMembers.mock.calls.length, memberCalls + 1);
  assert.equal(document.body.textContent?.includes("Grace"), false, "the reviewed request leaves the list");
});

test("rejecting a join request asks first, then goes through the store and refreshes", async () => {
  await mountModal({ visible: true, orgId: "org1", mode: "edit" });
  await openJoinRequests();
  const detailCalls = getOrganization.mock.calls.length;

  buttonByLabel(enUS.organization.settings.reject).click();
  await flushPromises();
  const popover = openPopover();
  assert.ok(popover.textContent?.includes(enUS.organization.joinRequests.rejectConfirm));
  assert.equal(orgStore.reviewOrganizationJoinRequest.mock.calls.length, 0, "nothing happens before confirming");

  buttonByText(enUS.organization.settings.reject, popover).click();
  await flushPromises();

  assert.deepEqual(orgStore.reviewOrganizationJoinRequest.mock.calls[0], [
    "org1",
    "jr1",
    { approved: false },
    { requestType: "join" },
  ]);
  assert.equal(getOrganization.mock.calls.length, detailCalls + 1);
});

test("the content column is the one scroller, and it returns to the top on every navigation", async () => {
  await mountModal({ visible: true, orgId: "org1", mode: "edit" });
  const scroller = document.body.querySelector<HTMLElement>(".content-wrapper");
  assert.ok(scroller);
  const scrollTo = vi.fn();
  scroller.scrollTo = scrollTo as unknown as HTMLElement["scrollTo"];

  navItem(enUS.organization.manageMembers).click();
  await flushPromises();
  assert.deepEqual(scrollTo.mock.calls.at(-1), [{ top: 0, behavior: "auto" }]);

  navItem(enUS.organization.share.sharedKnowledgeBase).click();
  await flushPromises();
  assert.equal(scrollTo.mock.calls.length, 2);
});

test("the page behind is locked while the modal is open and restored when it closes or unmounts", async () => {
  document.body.style.overflow = "scroll";
  const { wrapper, props } = await mountModal({ visible: false, mode: "create" });
  assert.equal(document.body.style.overflow, "scroll");

  props.visible = true;
  await flushPromises();
  assert.equal(document.body.style.overflow, "hidden");

  props.visible = false;
  await flushPromises();
  assert.equal(document.body.style.overflow, "scroll");

  props.visible = true;
  await flushPromises();
  assert.equal(document.body.style.overflow, "hidden");
  wrapper.unmount();
  mounted.splice(mounted.indexOf(wrapper), 1);
  assert.equal(document.body.style.overflow, "scroll");
});
