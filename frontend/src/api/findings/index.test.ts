import assert from "node:assert/strict";
import { beforeEach, test, vi } from "vitest";

// The transport is mocked so these tests pin down what the client asks for —
// the path, the query, the body — and how it reads the envelope back, which
// is the whole of the contract with the backend.
const request = vi.hoisted(() => ({ get: vi.fn(), patch: vi.fn(), post: vi.fn(), put: vi.fn() }));
vi.mock("@/utils/request", () => request);

import {
  assignFinding,
  buildFindingsQuery,
  countAssignedFindings,
  getFindingsSummary,
  listAssignedFindings,
  listFindings,
  listPageFindings,
  scanFindings,
  updateFindingStatus,
} from "./index";

beforeEach(() => {
  request.get.mockReset();
  request.patch.mockReset();
  request.post.mockReset();
  request.put.mockReset();
});

test("an empty query leaves every parameter to the server's defaults", () => {
  assert.equal(buildFindingsQuery(), "");
  assert.equal(buildFindingsQuery({ type: "", knowledge_id: "", page: 0 }), "");
});

test("the query carries each chosen filter once", () => {
  const qs = buildFindingsQuery({ status: "all", type: "duplicate", knowledge_id: "k 1", page: 2, page_size: 50 });
  const parsed = new URLSearchParams(qs.slice(1));
  assert.equal(qs[0], "?");
  assert.equal(parsed.get("status"), "all");
  assert.equal(parsed.get("type"), "duplicate");
  assert.equal(parsed.get("knowledge_id"), "k 1");
  assert.equal(parsed.get("page"), "2");
  assert.equal(parsed.get("page_size"), "50");
});

test("listing findings reads the page out of the envelope", async () => {
  request.get.mockResolvedValue({ success: true, data: { items: [{ id: "f1" }], total: 1, page: 1, page_size: 20 } });
  const page = await listFindings("kb/1", { status: "dismissed", page: 1 });
  assert.equal(request.get.mock.calls[0][0], "/api/v1/knowledge-bases/kb%2F1/findings?status=dismissed&page=1");
  assert.equal(page.total, 1);
  assert.equal(page.items[0].id, "f1");
});

test("a null page reads as an empty one", async () => {
  request.get.mockResolvedValue({ success: true, data: null });
  const page = await listFindings("kb1", { page: 3, page_size: 10 });
  assert.deepEqual(page, { items: [], total: 0, page: 3, page_size: 10 });
});

test("an unsuccessful envelope rejects with the server's message", async () => {
  request.get.mockResolvedValue({ success: false, error: { message: "nope" } });
  await assert.rejects(listFindings("kb1"), /nope/);
});

test("the summary has its own path and fills in what the server left out", async () => {
  request.get.mockResolvedValue({ success: true, data: { open_total: 4, enabled: true, supported: true } });
  const s = await getFindingsSummary("kb1");
  assert.equal(request.get.mock.calls[0][0], "/api/v1/knowledge-bases/kb1/findings/summary");
  assert.deepEqual(s, { open_total: 4, open_by_type: {}, last_scan_at: null, enabled: true, supported: true });
});

test("dismissing patches the finding with its new status and the reason", async () => {
  request.patch.mockResolvedValue({ success: true, data: { id: "f1", status: "dismissed" } });
  const f = await updateFindingStatus("kb1", "f1", "dismissed", "distinct_scope");
  assert.deepEqual(request.patch.mock.calls[0], [
    "/api/v1/knowledge-bases/kb1/findings/f1",
    { status: "dismissed", reason: "distinct_scope" },
  ]);
  assert.equal(f.status, "dismissed");

  await updateFindingStatus("kb1", "f1", "open");
  assert.deepEqual(request.patch.mock.calls[1][1], { status: "open" }, "reopening carries no reason");
});

test("assigning puts the assignee, and the caller's own findings have their own path", async () => {
  request.put.mockResolvedValue({ success: true, data: { id: "f1" } });
  await assignFinding("kb1", "f1", "u2");
  assert.deepEqual(request.put.mock.calls[0], [
    "/api/v1/knowledge-bases/kb1/findings/f1/assignee",
    { assignee_id: "u2" },
  ]);

  request.get.mockResolvedValue({ success: true, data: { open_total: 4 } });
  assert.equal(await countAssignedFindings(), 4);
  assert.equal(request.get.mock.calls.at(-1)?.[0], "/api/v1/findings/assigned/count");

  request.get.mockResolvedValue({ success: true, data: null });
  const page = await listAssignedFindings({ status: "open", page: 2 });
  assert.equal(request.get.mock.calls.at(-1)?.[0], "/api/v1/findings/assigned?status=open&page=2");
  assert.deepEqual(page, { items: [], total: 0, page: 2, page_size: 20 });
});

test("only mine is sent as assignee=me", () => {
  assert.equal(buildFindingsQuery({ mine: true }), "?assignee=me");
  assert.equal(buildFindingsQuery({ mine: false }), "");
});

test("a scan posts to the scan path and returns how many were queued", async () => {
  request.post.mockResolvedValue({ success: true, data: { queued: 7 } });
  assert.deepEqual(await scanFindings("kb1"), { queued: 7 });
  assert.equal(request.post.mock.calls[0][0], "/api/v1/knowledge-bases/kb1/findings/scan");
});

test("a docs page's findings come from the docs path", async () => {
  request.get.mockResolvedValue({ success: true, data: { items: null, other_count: 2 } });
  const res = await listPageFindings("p1");
  assert.equal(request.get.mock.calls[0][0], "/api/v1/docs/pages/p1/findings");
  assert.deepEqual(res, { items: [], other_count: 2 });
});
