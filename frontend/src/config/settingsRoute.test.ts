import assert from "node:assert/strict";
import { test } from "vitest";

import { buildSettingsRouteQuery, settingsQueryUnchanged } from "./settingsRoute";

test("a settings nav item writes its section and keeps unrelated query keys", () => {
  assert.deepEqual(buildSettingsRouteQuery("models", { section: "system-global" }), { section: "models" });
  assert.deepEqual(buildSettingsRouteQuery("general"), { section: "general" });
  assert.deepEqual(buildSettingsRouteQuery("runtime-queues", { section: "system-global", q: "x" }), {
    section: "runtime-queues",
    q: "x",
  });
});

test("canonical settings query skips a redundant replace", () => {
  assert.equal(settingsQueryUnchanged({ section: "models" }, { section: "models" }), true);
  assert.equal(settingsQueryUnchanged({ section: "models" }, { section: "general" }), false);
  assert.equal(settingsQueryUnchanged({}, { section: "general" }), false);
});
