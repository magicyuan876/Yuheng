import assert from "node:assert/strict";
import { afterEach, test } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import { h, nextTick } from "vue";
import { createI18n } from "vue-i18n";
import { SparklesIcon } from "@lucide/vue";
import enUS from "@/i18n/locales/en-US";
import SettingDrawer from "./SettingDrawer.vue";

// SettingDrawer carries two APIs, merged when its new-stack twin was folded
// into it: the one the older settings screens were written against (a
// TDesign icon name, a class that reaches the panel, the section styling
// contract) and the twin's (a lucide icon component, closeOnOverlayClick).
// These tests mount it and check both from the outside.

const mounted: VueWrapper[] = [];

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
  document.body.innerHTML = "";
});

async function mountDrawer(props: Record<string, unknown> = {}, attrs: Record<string, unknown> = {}, slots = {}) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(SettingDrawer, {
    props: { visible: true, title: "Edit model", storageKey: "", ...props },
    attrs,
    slots,
    global: { plugins: [i18n] },
    // The panel is teleported to <body>.
    attachTo: document.body,
  });
  mounted.push(wrapper);
  await nextTick();
  await nextTick();
  return wrapper;
}

function panel(): HTMLElement {
  const el = document.body.querySelector<HTMLElement>('[data-slot="drawer-content"]');
  assert.ok(el, "the drawer panel should be rendered while visible");
  return el;
}

function bodyButton(label: string): HTMLButtonElement | undefined {
  return Array.from(document.body.querySelectorAll("button")).find((b) => b.textContent?.includes(label));
}

test("a class on the drawer lands on the panel, next to the setting-drawer hook class", async () => {
  await mountDrawer({}, { class: "storage-engine-drawer storage-engine-drawer--minio", "data-test": "x" });
  const el = panel();
  assert.ok(el.classList.contains("setting-drawer"));
  assert.ok(el.classList.contains("storage-engine-drawer--minio"));
  assert.equal(el.getAttribute("data-test"), "x");
});

test("the overlay directly precedes the panel, which the z-index rule for it relies on", async () => {
  await mountDrawer();
  const overlay = document.body.querySelector('[data-slot="drawer-overlay"]');
  assert.ok(overlay, "the overlay should be rendered");
  assert.equal(overlay.nextElementSibling, panel());
});

test("title, description and the header-icon hook class render", async () => {
  await mountDrawer({ description: "Pick a provider", icon: "edit-1" });
  const el = panel();
  assert.match(el.textContent ?? "", /Edit model/);
  assert.match(el.textContent ?? "", /Pick a provider/);
  const badge = el.querySelector(".setting-drawer__header-icon");
  assert.ok(badge, "a legacy TDesign icon name should still produce the badge");
  assert.ok(badge.querySelector("svg"), "the name should map onto a lucide icon");
});

test("a lucide icon component renders in the badge", async () => {
  await mountDrawer({ icon: SparklesIcon });
  assert.ok(panel().querySelector(".setting-drawer__header-icon svg"));
});

test("an unknown icon name renders no badge instead of an empty one", async () => {
  await mountDrawer({ icon: "no-such-icon" });
  assert.equal(panel().querySelector(".setting-drawer__header-icon"), null);
});

test("the headerIcon and subtitle slots replace the defaults", async () => {
  await mountDrawer({}, {}, { headerIcon: () => h("img", { class: "header-icon__img" }), subtitle: () => "MinIO" });
  const el = panel();
  assert.ok(el.querySelector(".setting-drawer__header-icon .header-icon__img"));
  assert.match(el.textContent ?? "", /MinIO/);
});

test("the default footer confirms, and cancels by closing", async () => {
  const wrapper = await mountDrawer({ confirmText: "Run", cancelText: "Close" });
  bodyButton("Run")?.click();
  assert.equal(wrapper.emitted("confirm")?.length, 1);
  bodyButton("Close")?.click();
  assert.equal(wrapper.emitted("cancel")?.length, 1);
  assert.deepEqual(wrapper.emitted("update:visible"), [[false]]);
});

test("confirmLoading and confirmDisabled both disable the confirm button", async () => {
  await mountDrawer({ confirmText: "Run", confirmDisabled: true });
  assert.equal(bodyButton("Run")?.disabled, true);
  mounted.pop()?.unmount();
  document.body.innerHTML = "";
  await mountDrawer({ confirmText: "Run", confirmLoading: true });
  assert.equal(bodyButton("Run")?.disabled, true);
});

test("hideFooter removes the footer, and footer slots replace its halves", async () => {
  await mountDrawer({ hideFooter: true });
  assert.equal(panel().querySelector("footer"), null);
  mounted.pop()?.unmount();
  document.body.innerHTML = "";
  await mountDrawer({}, {}, { "footer-left": () => "Test connection", "footer-right": () => "Custom actions" });
  const footer = panel().querySelector("footer");
  assert.match(footer?.textContent ?? "", /Test connection/);
  assert.match(footer?.textContent ?? "", /Custom actions/);
  assert.equal(bodyButton("Save"), undefined);
});

test("the body keeps the hooks other screens reach it by", async () => {
  await mountDrawer({}, {}, { default: () => h("section", { class: "setting-drawer__section" }, "content") });
  const body = panel().querySelector("[data-setting-drawer-body]");
  assert.ok(body?.classList.contains("setting-drawer__body"));
  assert.ok(body?.querySelector(".setting-drawer__section"));
});

test("the resize handle shows only while resizable", async () => {
  await mountDrawer();
  assert.ok(document.body.querySelector('[role="separator"]'));
  mounted.pop()?.unmount();
  document.body.innerHTML = "";
  await mountDrawer({ resizable: false });
  assert.equal(document.body.querySelector('[role="separator"]'), null);
});
