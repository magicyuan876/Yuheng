import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import Login from "./Login.vue";

// The login page's forms used to be TDesign forms with rule tables; they are
// now plain forms with explicit validators. These tests mount the page and
// check what the user sees and what reaches the API, so the rules keep the
// behaviour TDesign gave them whatever the markup looks like.

const login = vi.fn();
const register = vi.fn();
vi.mock("@/api/auth", () => ({
  login: (...args: unknown[]) => login(...args),
  register: (...args: unknown[]) => register(...args),
  getOIDCAuthorizationURL: vi.fn(),
  getOIDCConfig: vi.fn(async () => ({ success: true, enabled: true, provider_display_name: "SSO" })),
  getAuthConfig: vi.fn(async () => ({ registration_mode: "self_serve" })),
  userInfoFromApi: vi.fn(),
  getInvitationByToken: vi.fn(),
  registerByInvite: vi.fn(),
}));

const replace = vi.fn();
vi.mock("vue-router", () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace }),
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: () => ({ isLoggedIn: false }),
}));

vi.mock("@/composables/useRoleLabel", () => ({
  useRoleLabel: () => ({ formatRole: vi.fn(), roleIcon: vi.fn() }),
}));

vi.mock("@/utils/loginNotify", () => ({ notifyLoginSuccess: vi.fn() }));

vi.mock("tdesign-vue-next", () => ({
  MessagePlugin: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));

// The carousel is irrelevant here and needs a real layout engine.
vi.mock("swiper/vue", async () => {
  const { defineComponent, h } = await import("vue");
  const Passthrough = defineComponent({
    setup:
      (_, { slots }) =>
      () =>
        h("div", slots.default?.()),
  });
  return { Swiper: Passthrough, SwiperSlide: Passthrough };
});
vi.mock("swiper/modules", () => ({ Autoplay: {}, EffectFade: {}, Pagination: {} }));

const mounted: VueWrapper[] = [];

beforeEach(() => {
  login.mockReset();
  register.mockReset();
  login.mockResolvedValue({ success: false, message: "nope" });
});

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount());
  document.body.innerHTML = "";
});

async function mountLogin() {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(Login, { global: { plugins: [i18n] }, attachTo: document.body });
  mounted.push(wrapper);
  await flushPromises();
  return wrapper;
}

function buttonByText(wrapper: VueWrapper, text: string) {
  const button = wrapper.findAll("button").find((b) => b.text().trim() === text);
  assert.ok(button, `a button reads "${text}"`);
  return button;
}

test("submitting an empty login form shows the required messages and calls no API", async () => {
  const wrapper = await mountLogin();
  await wrapper.find("form").trigger("submit");
  await flushPromises();
  const text = wrapper.text();
  assert.ok(text.includes(enUS.auth.emailRequired));
  assert.ok(text.includes(enUS.auth.passwordRequired));
  assert.equal(login.mock.calls.length, 0);
});

test("a field is re-checked as the user types, first failing rule first", async () => {
  const wrapper = await mountLogin();
  const password = wrapper.find("#login-password");
  await password.setValue("abc");
  assert.ok(wrapper.text().includes(enUS.auth.passwordMinLength));
  await password.setValue("abcdefgh");
  assert.ok(wrapper.text().includes(enUS.auth.passwordMustContainNumber));
  await password.setValue("abcdefg1");
  assert.ok(!wrapper.text().includes(enUS.auth.passwordMustContainNumber));
  assert.equal(password.attributes("aria-invalid"), undefined);
});

test("a valid login form reaches the API", async () => {
  const wrapper = await mountLogin();
  await wrapper.find("#login-email").setValue("user@example.com");
  await wrapper.find("#login-password").setValue("secret123");
  await wrapper.find("form").trigger("submit");
  await flushPromises();
  assert.deepEqual(login.mock.calls[0]?.[0], { email: "user@example.com", password: "secret123" });
});

test("the create-account and SSO buttons do not submit the login form", async () => {
  const wrapper = await mountLogin();
  assert.equal(buttonByText(wrapper, enUS.auth.createAccount).attributes("type"), "button");
  const sso = wrapper.findAll("button").find((b) => b.text().includes("SSO"));
  assert.ok(sso, "the SSO button is shown");
  assert.equal(sso.attributes("type"), "button");
  // A button with no type inside a form is a submit button; clicking it
  // must not run the login.
  await sso.trigger("click");
  await flushPromises();
  assert.equal(login.mock.calls.length, 0);
  assert.equal(wrapper.findAll('form button[type="submit"]').length, 1);
});

test("registration counts a Chinese character as two, as TDesign's length rules did", async () => {
  const wrapper = await mountLogin();
  await buttonByText(wrapper, enUS.auth.createAccount).trigger("click");
  const username = wrapper.find("#register-username");
  await username.setValue("王");
  assert.ok(!wrapper.text().includes(enUS.auth.usernameMinLength));
  await username.setValue("王".repeat(11));
  assert.ok(wrapper.text().includes(enUS.auth.usernameMaxLength));
});

test("the confirmation must match the password", async () => {
  const wrapper = await mountLogin();
  await buttonByText(wrapper, enUS.auth.createAccount).trigger("click");
  await wrapper.find("#register-password").setValue("secret123");
  await wrapper.find("#register-confirmPassword").setValue("secret124");
  assert.ok(wrapper.text().includes(enUS.auth.passwordMismatch));
  await wrapper.find("#register-confirmPassword").setValue("secret123");
  assert.ok(!wrapper.text().includes(enUS.auth.passwordMismatch));
});
