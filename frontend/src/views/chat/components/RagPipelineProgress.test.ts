import assert from "node:assert/strict";
import { afterEach, test, vi } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import { defineComponent, h, nextTick } from "vue";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import { provideChatReferencesDrawer, type ChatReferencesDrawerContext } from "@/composables/useChatReferencesDrawer";
import { RAG_WAIT_REVEAL_DELAY_MS } from "@/utils/rag-pipeline-state";
import RagPipelineProgress from "./RagPipelineProgress.vue";

// These tests replace an older file that regex-matched the component's
// source — its class names, its Less rules, even the spelling of a ternary.
// They mount the timeline instead and check what the reader sees at each
// stage of a quick-answer turn, and what a click does.

type Session = InstanceType<typeof RagPipelineProgress>["$props"]["session"];

const mounted: VueWrapper[] = [];

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
  vi.useRealTimers();
});

function mountTimeline(session: Session) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  let drawer: ChatReferencesDrawerContext | undefined;
  // The chat view provides the references drawer; a small host does the same
  // here so a click on a search step has somewhere to go.
  const Host = defineComponent({
    props: { session: { type: Object, default: undefined } },
    setup(hostProps) {
      drawer = provideChatReferencesDrawer();
      return () => h(RagPipelineProgress, { session: hostProps.session as Session });
    },
  });
  const wrapper = mount(Host, { props: { session }, global: { plugins: [i18n] } });
  mounted.push(wrapper);
  assert.ok(drawer);
  return { wrapper, drawer };
}

const searchDone = {
  type: "tool_call",
  tool_name: "knowledge_search",
  tool_call_id: "call-1",
  pending: false,
  success: true,
  tool_data: { count: 2, results: [{}, {}] },
};

const searchRunning = { ...searchDone, pending: true, tool_data: undefined };

const references = [
  { knowledge_id: "k1", knowledge_title: "Handbook.pdf" },
  { knowledge_id: "k2", knowledge_title: "Policy.docx" },
];

function liveRegions(wrapper: VueWrapper) {
  return wrapper.findAll("[aria-live]");
}

test("shows a single preparing step, announced from one live region, before anything streams", () => {
  const { wrapper } = mountTimeline({ id: "m1", agentEventStream: [] });
  assert.ok(wrapper.text().includes(enUS.chat.preparingAnswer));
  const regions = liveRegions(wrapper);
  assert.equal(regions.length, 1);
  assert.equal(regions[0].text(), enUS.chat.preparingAnswer);
  // The pending title opts into the shared shimmer.
  assert.ok(wrapper.find(".action-card.action-pending .action-name").exists());
});

test("marks a running search step so its title shimmers", () => {
  const { wrapper } = mountTimeline({ id: "m1", agentEventStream: [searchRunning] });
  const names = wrapper.findAll(".action-name");
  assert.equal(names.length, 1);
  assert.ok(names[0].classes().includes("is-running"));
  assert.ok(wrapper.find(".tree-child svg").exists(), "the step carries its tool icon");
});

test("shows the model-wait step once retrieval has finished and the answer has not started", async () => {
  vi.useFakeTimers();
  const { wrapper } = mountTimeline({ id: "m1", agentEventStream: [searchDone] });
  assert.equal(wrapper.find(".rag-model-wait-step").exists(), false, "the wait row is held back briefly");

  vi.advanceTimersByTime(RAG_WAIT_REVEAL_DELAY_MS);
  await nextTick();
  const wait = wrapper.find(".rag-model-wait-step");
  assert.ok(wait.exists());
  assert.ok(wait.text().includes(enUS.chat.connectingModelAndGeneratingAnswer));
  assert.ok(wait.find(".action-card").classes().includes("action-pending"));
  assert.equal(liveRegions(wrapper)[0].text(), enUS.chat.connectingModelAndGeneratingAnswer);
});

test("renders streamed thinking before the done row, and folds it from its header", async () => {
  const session: Session = {
    id: "m1",
    agentEventStream: [searchDone, { type: "thinking", content: "Weighing the two documents." }],
  };
  const { wrapper } = mountTimeline(session);
  const thinking = wrapper.find(".rag-thinking-step");
  assert.ok(thinking.exists());
  assert.ok(thinking.text().includes(enUS.agent.think));
  assert.ok(thinking.text().includes("Weighing the two documents."));
  assert.equal(wrapper.find(".agent-step-done").exists(), false, "no done row while the turn is still running");

  await thinking.find(".action-header").trigger("click");
  assert.equal(wrapper.find(".thinking-detail-content").exists(), false);
  await wrapper.find(".rag-thinking-step .action-header").trigger("click");
  assert.ok(wrapper.find(".thinking-detail-content").exists());
});

test("collapses to a one-line summary after the answer, and expands from that line", async () => {
  const { wrapper } = mountTimeline({
    id: "m1",
    content: "The answer.",
    is_completed: true,
    knowledge_references: references,
    agentEventStream: [searchDone, { type: "thinking", content: "Some thought." }],
  });
  const toggle = wrapper.find("button.tree-root-expand");
  assert.ok(toggle.exists());
  assert.equal(toggle.attributes("aria-expanded"), "false");
  assert.ok(toggle.text().includes(enUS.agentStream.ragPipeline.searchDone));
  assert.ok(toggle.text().includes("2"), "the reference count is part of the summary");
  assert.equal(wrapper.findAll(".tree-root-expand__icon").length, 1);
  assert.equal(wrapper.find(".tree-child").exists(), false);

  await toggle.trigger("click");
  assert.equal(toggle.attributes("aria-expanded"), "true");
  const rows = wrapper.findAll(".tree-child");
  assert.ok(rows.length >= 3, "search, thinking and done rows");
  const thinkingIndex = rows.findIndex((row) => row.classes().includes("rag-thinking-step"));
  const doneIndex = rows.findIndex((row) => row.classes().includes("agent-step-done"));
  assert.ok(thinkingIndex > -1 && doneIndex > thinkingIndex, "thinking sits before the done row");
  assert.ok(rows[doneIndex].text().includes(enUS.common.finish));

  await toggle.trigger("click");
  assert.equal(wrapper.find(".tree-child").exists(), false);
});

test("a finished search step with references opens the references drawer", async () => {
  const { wrapper, drawer } = mountTimeline({
    id: "m1",
    knowledge_references: references,
    agentEventStream: [searchDone, { type: "thinking", content: "" }],
  });
  const trigger = wrapper.find(".has-reference-trigger");
  assert.ok(trigger.exists());
  assert.equal(trigger.attributes("role"), "button");
  assert.equal(trigger.attributes("tabindex"), "0");

  await trigger.trigger("click");
  assert.equal(drawer.visible.value, true);
  assert.equal(drawer.references.value.length, 2);

  // The same step toggles the drawer shut again, from the keyboard too.
  await trigger.trigger("keydown", { key: "Enter" });
  assert.equal(drawer.visible.value, false);
});

test("a search step without references is not clickable", async () => {
  const { wrapper, drawer } = mountTimeline({ id: "m1", agentEventStream: [searchDone] });
  assert.equal(wrapper.find(".has-reference-trigger").exists(), false);
  await wrapper.find(".action-card").trigger("click");
  assert.equal(drawer.visible.value, false);
});

test("puts attachment preparation on the timeline", () => {
  const { wrapper } = mountTimeline({
    id: "m1",
    agentEventStream: [
      { type: "tool_call", tool_name: "attachment_parsing", tool_call_id: "att-1", pending: true },
      searchRunning,
    ],
  });
  assert.equal(wrapper.findAll(".tree-child").length, 2);
});
