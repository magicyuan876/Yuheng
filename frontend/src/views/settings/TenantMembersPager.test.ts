import assert from "node:assert/strict";
import { afterEach, test } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import TenantMembersPager from "./TenantMembersPager.vue";

// The pager under the tenant member and invitation tables, which replaced
// t-pagination. These tests pin what the tables rely on: the total, the
// numbered pages, and one `change` per user action with the models updated
// before it fires.

const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });

let wrapper: VueWrapper | undefined;

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
});

function mountPager(props: { page: number; pageSize: number; total: number }) {
  wrapper = mount(TenantMembersPager, {
    props: { ...props, pageSizeOptions: [10, 20, 50, 100] },
    global: { plugins: [i18n] },
    attachTo: document.body,
  });
  return wrapper;
}

test("shows the total and one button per page", () => {
  const w = mountPager({ page: 1, pageSize: 20, total: 45 });
  assert.match(w.text(), /Total 45 items/);
  const pages = w.findAll("[data-type='page']").map((b) => b.text());
  assert.deepEqual(pages, ["1", "2", "3"]);
});

test("clicking a page emits the new page, then change", async () => {
  const w = mountPager({ page: 1, pageSize: 20, total: 45 });
  const second = w.findAll("[data-type='page']").find((b) => b.text() === "2");
  assert.ok(second);
  await second.trigger("click");
  assert.deepEqual(w.emitted("update:page"), [[2]]);
  assert.equal(w.emitted("change")?.length, 1);
});

test("the jumper clamps to the last page and ignores the current one", async () => {
  const w = mountPager({ page: 1, pageSize: 20, total: 45 });
  const input = w.find("input[type='number']");
  await input.setValue("99");
  await input.trigger("keydown", { key: "Enter" });
  assert.deepEqual(w.emitted("update:page"), [[3]]);

  await input.setValue("1");
  await input.trigger("keydown", { key: "Enter" });
  assert.equal(w.emitted("update:page")?.length, 1, "jumping to the current page is not a change");
});
