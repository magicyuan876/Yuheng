import assert from "node:assert/strict";
import { mount, type VueWrapper } from "@vue/test-utils";
import { afterEach, test } from "vitest";
import { createI18n } from "vue-i18n";

import type { CommentView } from "@/api/docs";
import enUS from "@/i18n/locales/en-US";

import CommentsPanel from "./CommentsPanel.vue";

// The panel moved from a native checkbox and hand-styled classes to the
// shadcn Checkbox and utilities. These tests mount it and check what the page
// relies on: the resolved toggle still reports a plain boolean, a click on a
// thread still selects it, and the reply button still opens a composer.
// The comment and composer components are stubbed: they render stored
// documents through the editor's extensions, which is not what is under test.

const mounted: VueWrapper[] = [];

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
});

const thread = {
  id: "c1",
  body: null,
  created_at: "2026-01-01T00:00:00Z",
  can_resolve: true,
  replies: [],
} as unknown as CommentView;

function mountPanel(props: Partial<InstanceType<typeof CommentsPanel>["$props"]> = {}) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(CommentsPanel, {
    props: {
      threads: [thread],
      grouped: { inline: [thread], page: [], orphaned: [] },
      open: 1,
      total: 1,
      loading: false,
      activeId: "",
      showResolved: false,
      ...props,
    },
    global: {
      plugins: [i18n],
      stubs: { CommentItem: true, CommentComposer: { template: '<div class="composer-stub" />' } },
    },
  });
  mounted.push(wrapper);
  return wrapper;
}

test("the resolved toggle emits a boolean", async () => {
  const wrapper = mountPanel();
  await wrapper.get('[data-slot="checkbox"]').trigger("click");
  assert.deepEqual(wrapper.emitted("update:showResolved"), [[true]]);
});

test("the resolved toggle reflects the prop", () => {
  const wrapper = mountPanel({ showResolved: true });
  assert.equal(wrapper.get('[data-slot="checkbox"]').attributes("aria-checked"), "true");
});

test("clicking a thread selects it", async () => {
  const wrapper = mountPanel();
  await wrapper.get("article").trigger("click");
  assert.deepEqual(wrapper.emitted("select"), [["c1"]]);
});

test("reply opens a composer without selecting the thread", async () => {
  const wrapper = mountPanel();
  const reply = wrapper.findAll("button").find((b) => b.text() === enUS.docs.comments.reply);
  assert.ok(reply);
  await reply.trigger("click");
  assert.ok(wrapper.find(".composer-stub").exists());
  assert.equal(wrapper.emitted("select"), undefined);
});

test("resolve emits the new state", async () => {
  const wrapper = mountPanel();
  const resolve = wrapper.findAll("button").find((b) => b.text() === enUS.docs.comments.resolve);
  assert.ok(resolve);
  await resolve.trigger("click");
  assert.deepEqual(wrapper.emitted("resolve"), [["c1", true]]);
});
