import assert from "node:assert/strict";
import { test } from "vitest";

import {
  API_EXTERNAL_USER_SESSION_OWNER_PREFIX,
  classifyDateBucket,
  groupSessionsByDate,
  originGroupKey,
  resolveSessionOrigin,
} from "./sessionGrouping.ts";

test("resolveSessionOrigin files API-key sessions apart from the user's own chats", () => {
  assert.deepEqual(resolveSessionOrigin({ id: "1" }), { kind: "web" });
  assert.deepEqual(resolveSessionOrigin({ id: "2", user_id: "user-7" }), { kind: "web" });
  assert.deepEqual(resolveSessionOrigin({ id: "3", user_id: "api_tenant_key:1:10" }), { kind: "api" });
  assert.deepEqual(resolveSessionOrigin({ id: "4", user_id: `${API_EXTERNAL_USER_SESSION_OWNER_PREFIX}1:alice` }), {
    kind: "api",
  });
  assert.equal(originGroupKey({ kind: "api" }), "api");
  assert.equal(originGroupKey({ kind: "web" }), "web");
});

test("groupSessionsByDate keeps pinned sessions in their own bucket", () => {
  const now = new Date().toISOString();
  const groups = groupSessionsByDate(
    [
      { id: "p", is_pinned: true, updated_at: now },
      { id: "t", updated_at: now },
    ],
    {
      pinned: "Pinned",
      today: "Today",
      yesterday: "Yesterday",
      last7Days: "7d",
      last30Days: "30d",
      lastYear: "Year",
      earlier: "Earlier",
    },
    () => "today",
  );
  assert.deepEqual(
    groups.map((g) => g.key),
    ["pinned", "today"],
  );
});

test("classifyDateBucket buckets recent sessions as today", () => {
  assert.equal(classifyDateBucket(new Date().toISOString()), "today");
});
