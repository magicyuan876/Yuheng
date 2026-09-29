import assert from "node:assert/strict";
import { afterEach, test } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import { nextTick } from "vue";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import type { MessageSuggestionSet } from "@/api/message-suggestion";
import FollowUpSuggestions from "./FollowUpSuggestions.vue";

// These tests replace an older file that regex-matched the component's
// source (the TDesign icon names, the Less transition rules). They mount the
// card instead and check what the reader sees and what the chat view is told.

const mounted: VueWrapper[] = [];

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
});

function makeSet(overrides: Partial<MessageSuggestionSet> = {}): MessageSuggestionSet {
  return {
    id: "set-1",
    session_id: "session-1",
    assistant_message_id: "message-1",
    status: "ready",
    allow_regenerate: true,
    questions: [
      { id: "q1", text: "What changed in the last release?", source: "model" },
      { id: "q2", text: "Who owns the billing service?", source: "model" },
    ],
    ...overrides,
  };
}

function mountCard(props: {
  suggestionSet?: MessageSuggestionSet | null;
  loading?: boolean;
  allowRegenerate?: boolean;
}) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(FollowUpSuggestions, { props, global: { plugins: [i18n] } });
  mounted.push(wrapper);
  return wrapper;
}

function button(wrapper: VueWrapper, label: string) {
  return wrapper.findAll("button").find((b) => b.text().includes(label) || b.attributes("aria-label") === label);
}

test("does not insert a separate status row while the first suggestions are loading", () => {
  const wrapper = mountCard({ suggestionSet: makeSet({ status: "generating" }) });
  assert.equal(wrapper.find(".follow-ups").exists(), false);
  assert.equal(wrapper.text(), "");
});

test("lists every question under the lightbulb title and forwards a click", async () => {
  const set = makeSet();
  const wrapper = mountCard({ suggestionSet: set });
  const title = wrapper.find(".follow-ups__title");
  assert.ok(title.text().includes(enUS.chat.followUpQuestions));
  assert.ok(title.find("svg").exists(), "the title carries its lightbulb icon");

  const items = wrapper.findAll(".follow-ups__list button");
  assert.deepEqual(
    items.map((b) => b.text()),
    set.questions.map((q) => q.text),
  );
  await items[1].trigger("click");
  assert.deepEqual(wrapper.emitted("select"), [[set.questions[1]]]);
});

test("keeps the existing follow-up card visible while regenerating", async () => {
  const wrapper = mountCard({ suggestionSet: makeSet(), allowRegenerate: true, loading: false });
  const refreshLabel = enUS.chat.refreshSuggestedQuestions;
  const refresh = button(wrapper, refreshLabel);
  assert.ok(refresh);
  assert.equal(refresh.attributes("disabled"), undefined);
  assert.equal(refresh.find(".animate-spin").exists(), false);
  await refresh.trigger("click");
  assert.equal(wrapper.emitted("regenerate")?.length, 1);

  await wrapper.setProps({ loading: true });
  const busy = button(wrapper, refreshLabel);
  assert.ok(busy);
  assert.notEqual(busy.attributes("disabled"), undefined);
  assert.ok(busy.find(".animate-spin").exists(), "a spinner replaces the refresh glyph");
  assert.equal(wrapper.findAll(".follow-ups__list button").length, 2, "the old questions stay on screen");
});

test("hides the regenerate button unless the parent allows it", () => {
  const wrapper = mountCard({ suggestionSet: makeSet() });
  assert.equal(button(wrapper, enUS.chat.refreshSuggestedQuestions), undefined);
});

test("reports one impression per set and the dismissal of the shown set", async () => {
  const set = makeSet();
  const wrapper = mountCard({ suggestionSet: set });
  assert.deepEqual(wrapper.emitted("impression"), [[set]]);

  // The same set arriving again (a re-render of the message) is not a new impression.
  await wrapper.setProps({ suggestionSet: { ...set } });
  await nextTick();
  assert.equal(wrapper.emitted("impression")?.length, 1);

  const close = button(wrapper, enUS.common.close);
  assert.ok(close);
  await close.trigger("click");
  assert.equal(wrapper.emitted("dismiss")?.length, 1);
});
