import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import CreateTenantDialog from "./CreateTenantDialog.vue";

// The create-workspace dialog lost its TDesign form in the migration, and
// with it the form rule that refused a blank name. These tests check the
// hand-written replacement: a name of only spaces is refused with the old
// message, a valid one is trimmed and sent, and the parent hears about it.

const createTenant = vi.fn();
vi.mock("@/api/tenant", () => ({
  createTenant: (...args: unknown[]) => createTenant(...args),
}));

vi.mock("tdesign-vue-next", () => ({
  MessagePlugin: { success: vi.fn(), error: vi.fn() },
}));

const mounted: VueWrapper[] = [];

beforeEach(() => {
  createTenant.mockReset();
});

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount());
  document.body.innerHTML = "";
});

async function mountDialog() {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(CreateTenantDialog, {
    props: { visible: false },
    global: { plugins: [i18n] },
    // The dialog is teleported to <body>.
    attachTo: document.body,
  });
  mounted.push(wrapper);
  // The form is reset when the dialog opens, as it is in the app.
  await wrapper.setProps({ visible: true });
  await flushPromises();
  return wrapper;
}

function dialog(): HTMLElement {
  const el = document.body.querySelector<HTMLElement>('[role="dialog"]');
  assert.ok(el, "the dialog is open");
  return el;
}

function submitButton(): HTMLButtonElement {
  const button = Array.from(dialog().querySelectorAll("button")).find(
    (b) => b.textContent?.trim() === enUS.tenant.create.submit,
  );
  assert.ok(button, "the submit button is shown");
  return button;
}

async function type(el: HTMLInputElement | HTMLTextAreaElement, value: string) {
  el.value = value;
  el.dispatchEvent(new Event("input"));
  await flushPromises();
}

test("a blank or whitespace-only name is refused with the required message", async () => {
  await mountDialog();
  const name = dialog().querySelector<HTMLInputElement>("#create-tenant-name");
  assert.ok(name);
  await type(name, "   ");

  submitButton().click();
  await flushPromises();

  assert.equal(createTenant.mock.calls.length, 0);
  assert.ok(dialog().textContent?.includes(enUS.tenant.create.nameRequired));
});

test("a valid name is trimmed and sent, and the parent receives the new workspace and a close", async () => {
  const tenant = { id: 7, name: "Lab" };
  createTenant.mockResolvedValue({ success: true, data: tenant });
  const wrapper = await mountDialog();
  const name = dialog().querySelector<HTMLInputElement>("#create-tenant-name");
  const description = dialog().querySelector<HTMLTextAreaElement>("#create-tenant-description");
  assert.ok(name && description);
  await type(name, "  Lab  ");
  await type(description, " ");

  submitButton().click();
  await flushPromises();

  assert.deepEqual(createTenant.mock.calls[0], [{ name: "Lab", description: undefined }]);
  assert.deepEqual(wrapper.emitted("created"), [[tenant]]);
  assert.deepEqual(wrapper.emitted("update:visible")?.at(-1), [false]);
});
