import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createI18n } from "vue-i18n";

import enUS from "@/i18n/locales/en-US";

// Mounted tests for the helpful / not helpful buttons of an answer: one
// request for the session's feedback, a toggle for "helpful", a form for "not
// helpful" that says who sees what, and taking feedback back.

const api = vi.hoisted(() => ({ listSessionFeedback: vi.fn(), setAnswerFeedback: vi.fn() }));
vi.mock("@/api/feedback", () => api);

const toast = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("tdesign-vue-next", () => ({ MessagePlugin: toast }));

import { Textarea } from "@/components/ui/textarea";

import AnswerFeedback from "./AnswerFeedback.vue";

const mounted: VueWrapper[] = [];

beforeEach(() => {
  setActivePinia(createPinia());
  Object.values(api).forEach((fn) => fn.mockReset());
  api.listSessionFeedback.mockResolvedValue({});
});

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
  document.body.innerHTML = "";
});

async function mountButtons(messageId = "m1") {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(AnswerFeedback, {
    props: { sessionId: "s1", messageId },
    global: { plugins: [i18n] },
    attachTo: document.body,
  });
  mounted.push(wrapper);
  await flushPromises();
  return wrapper;
}

test("answers of one session share one request, and show what was given", async () => {
  api.listSessionFeedback.mockResolvedValue({
    m1: { message_id: "m1", rating: "up", comment: "", share_question: false, updated_at: "" },
  });
  const first = await mountButtons("m1");
  await mountButtons("m2");
  assert.equal(api.listSessionFeedback.mock.calls.length, 1);
  assert.equal(first.get('[data-testid="answer-feedback-up"]').attributes("aria-pressed"), "true");
});

test("helpful toggles, and pressing it again takes it back", async () => {
  const wrapper = await mountButtons();
  api.setAnswerFeedback.mockResolvedValue({ message_id: "m1", rating: "up", comment: "", share_question: false });
  await wrapper.get('[data-testid="answer-feedback-up"]').trigger("click");
  await flushPromises();
  assert.deepEqual(api.setAnswerFeedback.mock.calls[0], ["s1", "m1", { rating: "up" }]);
  assert.equal(wrapper.get('[data-testid="answer-feedback-up"]').attributes("aria-pressed"), "true");

  api.setAnswerFeedback.mockResolvedValue(null);
  await wrapper.get('[data-testid="answer-feedback-up"]').trigger("click");
  await flushPromises();
  assert.deepEqual(api.setAnswerFeedback.mock.calls[1], ["s1", "m1", { rating: "" }]);
  assert.equal(wrapper.get('[data-testid="answer-feedback-up"]').attributes("aria-pressed"), "false");
});

test("not helpful asks why, says who sees it, and attaches the question only when asked to", async () => {
  const wrapper = await mountButtons();
  await wrapper.get('[data-testid="answer-feedback-down"]').trigger("click");
  await flushPromises();
  const form = document.body.querySelector('[data-testid="answer-feedback-form"]');
  assert.ok(form?.textContent?.includes("attached only if you tick the box"), form?.textContent ?? "");
  assert.equal(api.setAnswerFeedback.mock.calls.length, 0, "nothing is sent before the form");

  await wrapper.getComponent(Textarea).setValue("  Leave is ten days now  ");
  api.setAnswerFeedback.mockResolvedValue({ message_id: "m1", rating: "down", comment: "x", share_question: false });
  (document.body.querySelector('[data-testid="answer-feedback-send"]') as HTMLElement).click();
  await flushPromises();
  assert.deepEqual(api.setAnswerFeedback.mock.calls[0], [
    "s1",
    "m1",
    { rating: "down", comment: "Leave is ten days now", share_question: false },
  ]);
  assert.equal(toast.success.mock.calls.length, 1);

  // Pressed already, "not helpful" takes the feedback back.
  api.setAnswerFeedback.mockResolvedValue(null);
  await wrapper.get('[data-testid="answer-feedback-down"]').trigger("click");
  await flushPromises();
  assert.deepEqual(api.setAnswerFeedback.mock.calls[1], ["s1", "m1", { rating: "" }]);
});
