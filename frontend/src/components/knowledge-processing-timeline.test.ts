import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";

// These tests replace an older file that regex-matched the component's
// source for the two stage-count rules. They mount the full timeline on a
// canned /spans payload instead and read the counts where the reader sees
// them: the "Current stage n/total" or "Main stages n/total" part of the
// header's meta line.

const api = vi.hoisted(() => ({
  getKnowledgeSpans: vi.fn(),
  getKnowledgeDetails: vi.fn(),
  reparseKnowledge: vi.fn(),
  cancelKnowledgeParse: vi.fn(),
}));

vi.mock("@/api/knowledge-base/index", () => api);

import KnowledgeProcessingTimeline from "./knowledge-processing-timeline.vue";

const mounted: VueWrapper[] = [];

beforeEach(() => {
  api.getKnowledgeDetails.mockResolvedValue({ success: false });
});

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
  vi.clearAllMocks();
});

type StageStatus = "done" | "skipped" | "pending" | "running" | "failed";

// A payload whose root span has one child per pipeline stage, in the
// component's stage order, with the given statuses.
function spansPayload(parseStatus: string, statuses: StageStatus[]) {
  const names = ["docreader", "chunking", "embedding", "multimodal", "postprocess"];
  return {
    success: true,
    data: {
      knowledge_id: "k1",
      attempt: 1,
      latest_attempt: 1,
      parse_status: parseStatus,
      trace: {
        span_id: "root",
        name: "knowledge",
        kind: "root",
        status: parseStatus === "completed" ? "done" : "running",
        children: names.map((name, i) => ({ span_id: `s-${name}`, name, kind: "stage", status: statuses[i] })),
      },
    },
  };
}

async function mountTimeline(payload: ReturnType<typeof spansPayload>) {
  api.getKnowledgeSpans.mockResolvedValue(payload);
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const wrapper = mount(KnowledgeProcessingTimeline, {
    props: { knowledgeId: "k1", autoPoll: false },
    global: { plugins: [i18n] },
  });
  mounted.push(wrapper);
  await flushPromises();
  return wrapper;
}

test("counts skipped stages when determining the current stage", async () => {
  // Two stages behind it (one done, one skipped), none running: the
  // current stage is the third.
  const wrapper = await mountTimeline(spansPayload("processing", ["done", "skipped", "pending", "pending", "pending"]));

  assert.match(wrapper.text(), /Current stage 3\/5/);
});

test("counts skipped stages in the completed stage total", async () => {
  const wrapper = await mountTimeline(spansPayload("completed", ["done", "done", "done", "skipped", "done"]));

  assert.match(wrapper.text(), /Main stages 5\/5/);
});
