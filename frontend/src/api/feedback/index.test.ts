import assert from "node:assert/strict";
import { beforeEach, test, vi } from "vitest";

// The transport is mocked; what is pinned is the path and body the client
// sends, and how it reads the envelope back.
const request = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }));
vi.mock("@/utils/request", () => request);

import { listSessionFeedback, setAnswerFeedback } from "./index";

beforeEach(() => {
  request.get.mockReset();
  request.put.mockReset();
});

test("the session's feedback is read from its own path, and an empty answer is an empty map", async () => {
  request.get.mockResolvedValue({ success: true, data: null });
  assert.deepEqual(await listSessionFeedback("s 1"), {});
  assert.equal(request.get.mock.calls[0][0], "/api/v1/sessions/s%201/feedback");
});

test("feedback is put on the answer, and taking it back resolves to null", async () => {
  request.put.mockResolvedValue({ success: true, data: null });
  const body = { rating: "down" as const, comment: "wrong", share_question: true };
  assert.equal(await setAnswerFeedback("s1", "m1", body), null);
  assert.deepEqual(request.put.mock.calls[0], ["/api/v1/sessions/s1/messages/m1/feedback", body]);

  request.put.mockResolvedValue({ success: false, message: "forbidden" });
  await assert.rejects(setAnswerFeedback("s1", "m1", { rating: "up" }), /forbidden/);
});
