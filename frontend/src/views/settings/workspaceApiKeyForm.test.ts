import assert from "node:assert/strict";
import { test } from "vitest";
import type { TenantAPIKey } from "@/api/tenant";
import {
  emptyWorkspaceApiKeyForm,
  isApiKeyExpired,
  validateWorkspaceApiKeyForm,
  workspaceApiKeyFormFromKey,
  workspaceApiKeyPayload,
  type WorkspaceApiKeyForm,
} from "./workspaceApiKeyForm";

// Built from local parts so the assertions hold in any time zone.
const NOW = new Date(2026, 8, 30, 10, 0, 0);
const unix = (date: Date) => Math.floor(date.getTime() / 1000);

function key(overrides: Partial<TenantAPIKey> = {}): TenantAPIKey {
  return {
    id: 1,
    name: "bot",
    api_key: "sk-abcdefghijklmnopqrstuvwxyz",
    full_access: false,
    knowledge_base_ids: ["kb-1"],
    capabilities: ["retrieve"],
    created_at: "2026-09-01T00:00:00Z",
    ...overrides,
  };
}

test("a new key defaults to retrieve + chat for 90 days", () => {
  const payload = workspaceApiKeyPayload({ ...emptyWorkspaceApiKeyForm(), name: "  bot  " }, NOW);
  assert.deepEqual(payload, {
    name: "bot",
    full_access: false,
    capabilities: ["retrieve", "chat"],
    knowledge_base_ids: [],
    expires_at_unix: unix(NOW) + 90 * 24 * 60 * 60,
  });
});

test("a full-access key sends no capabilities and no allow-list", () => {
  const form = { ...emptyWorkspaceApiKeyForm(), name: "ops", fullAccess: true, knowledgeBaseIds: ["kb-1"] };
  const payload = workspaceApiKeyPayload(form, NOW);
  assert.equal(payload.full_access, true);
  assert.deepEqual(payload.capabilities, []);
  assert.deepEqual(payload.knowledge_base_ids, []);
});

test("an allow-list is dropped when no granted capability is bounded by knowledge base", () => {
  const form: WorkspaceApiKeyForm = {
    ...emptyWorkspaceApiKeyForm(),
    name: "members",
    capabilities: ["manage_members"],
    knowledgeBaseIds: ["kb-1"],
  };
  assert.deepEqual(workspaceApiKeyPayload(form, NOW).knowledge_base_ids, []);
  form.capabilities = ["manage_members", "ingest"];
  assert.deepEqual(workspaceApiKeyPayload(form, NOW).knowledge_base_ids, ["kb-1"]);
});

test("'never' sends no expiry, which on update clears it", () => {
  const payload = workspaceApiKeyPayload({ ...emptyWorkspaceApiKeyForm(), name: "x", expiry: "never" }, NOW);
  assert.equal("expires_at_unix" in payload, false);
});

test("a picked date expires at the end of that local day", () => {
  const form = { ...emptyWorkspaceApiKeyForm(), name: "x", expiry: "date" as const, expiryDate: "2026-10-31" };
  assert.equal(workspaceApiKeyPayload(form, NOW).expires_at_unix, unix(new Date(2026, 9, 31, 23, 59, 59)));
});

test("editing keeps the stored expiry instant, and every scope field, unless changed", () => {
  const expiresAt = new Date(2026, 11, 1, 14, 32, 5);
  const form = workspaceApiKeyFormFromKey(key({ expires_at: expiresAt.toISOString() }));
  assert.equal(form.expiry, "date");
  assert.equal(form.expiryDate, "2026-12-01");
  assert.deepEqual(workspaceApiKeyPayload(form, NOW), {
    name: "bot",
    full_access: false,
    capabilities: ["retrieve"],
    knowledge_base_ids: ["kb-1"],
    expires_at_unix: unix(expiresAt),
  });
  form.expiryDate = "2026-12-02";
  assert.equal(workspaceApiKeyPayload(form, NOW).expires_at_unix, unix(new Date(2026, 11, 2, 23, 59, 59)));
});

test("a key without an expiry edits as 'never'", () => {
  assert.equal(workspaceApiKeyFormFromKey(key()).expiry, "never");
});

test("validation: name, at least one capability for a scoped key, a future date", () => {
  const base = { ...emptyWorkspaceApiKeyForm(), name: "x" };
  assert.equal(validateWorkspaceApiKeyForm({ ...base, name: "   " }, NOW), "nameRequired");
  assert.equal(validateWorkspaceApiKeyForm({ ...base, capabilities: [] }, NOW), "capabilityRequired");
  assert.equal(validateWorkspaceApiKeyForm({ ...base, capabilities: [], fullAccess: true }, NOW), null);
  assert.equal(validateWorkspaceApiKeyForm({ ...base, expiry: "date", expiryDate: "" }, NOW), "expiryInFuture");
  assert.equal(
    validateWorkspaceApiKeyForm({ ...base, expiry: "date", expiryDate: "2026-09-29" }, NOW),
    "expiryInFuture",
  );
  // Today still counts: the key lives until the end of the day.
  assert.equal(validateWorkspaceApiKeyForm({ ...base, expiry: "date", expiryDate: "2026-09-30" }, NOW), null);
});

test("an already-expired key can be edited without being forced to a new date", () => {
  const form = workspaceApiKeyFormFromKey(key({ expires_at: new Date(2026, 0, 1).toISOString() }));
  assert.equal(validateWorkspaceApiKeyForm(form, NOW), null);
});

test("isApiKeyExpired", () => {
  assert.equal(isApiKeyExpired({ expires_at: undefined }, NOW), false);
  assert.equal(isApiKeyExpired({ expires_at: new Date(2026, 8, 30, 9).toISOString() }, NOW), true);
  assert.equal(isApiKeyExpired({ expires_at: new Date(2026, 8, 30, 11).toISOString() }, NOW), false);
});
