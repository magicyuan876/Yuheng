import assert from "node:assert/strict";
import { test } from "vitest";

import {
  PLATFORM_MANAGED_SETTINGS_SECTIONS,
  SETTINGS_MANAGEMENT_SHORTCUT_MIN_ROLE,
  SETTINGS_SECTION_MIN_ROLE,
  SYSTEM_ADMIN_SETTINGS_SECTIONS,
  isPlatformManagedSection,
  isPromotionSection,
} from "./settingsAccess";

test("management shortcuts are stricter than read-only settings pages", () => {
  assert.equal(SETTINGS_SECTION_MIN_ROLE.members, "viewer");
  assert.equal(SETTINGS_MANAGEMENT_SHORTCUT_MIN_ROLE.members, "owner");
  assert.equal(SETTINGS_SECTION_MIN_ROLE.models, "viewer");
  assert.equal(SETTINGS_MANAGEMENT_SHORTCUT_MIN_ROLE.models, "admin");
});

test("system administration settings stay explicitly system-admin-only", () => {
  assert.deepEqual(
    [...SYSTEM_ADMIN_SETTINGS_SECTIONS],
    ["system-global", "runtime-queues", "platform-api-keys", "system-audit-log"],
  );
});

test("centralised mode only relocates shared-infrastructure sections", () => {
  // Pinned rather than spot-checked: this list is the contract with
  // internal/router/routes_infra.go, where the matching write routes carry
  // g.PlatformManaged(). Adding a key here without moving the route (or the
  // reverse) produces an entry the user can open but not save, or one they
  // can save through an endpoint the nav claims is gone.
  assert.deepEqual([...PLATFORM_MANAGED_SETTINGS_SECTIONS].sort(), [
    "models",
    "ollama",
    "parser",
    "storage",
    "vectorstore",
    "websearch",
  ]);
});

test("workspace and personal sections are never relocated to the platform", () => {
  // Integrations stay with the workspace: each team publishes its own bot,
  // widget and API keys, which is workspace business rather than shared
  // infrastructure. The rest are plainly workspace/personal dimension.
  for (const key of [
    "general",
    "userprofile",
    "mymemory",
    "tenant",
    "members",
    "chathistory",
    "memory",
    "system",
    "integrations-im",
    "integrations-embed",
    "integrations-api",
  ]) {
    assert.equal(
      isPlatformManagedSection(key, true),
      false,
      `${key} must stay with the workspace under centralised mode`,
    );
  }
});

test("centralised mode is off by default, so nothing is hidden", () => {
  for (const key of PLATFORM_MANAGED_SETTINGS_SECTIONS) {
    assert.equal(
      isPlatformManagedSection(key, false),
      false,
      "with the switch off every section keeps its existing role gate",
    );
  }
});

test("the two access sets are disjoint", () => {
  // A section in both would be checked twice with the same outcome, which is
  // harmless but signals someone mis-classified a platform-admin page as
  // shared infrastructure (or vice versa).
  for (const key of PLATFORM_MANAGED_SETTINGS_SECTIONS) {
    assert.equal(SYSTEM_ADMIN_SETTINGS_SECTIONS.has(key), false, `${key} is classified in both access sets`);
  }
});

test("the Enterprise section is readable by everyone and disappears with the promotion switch", () => {
  assert.equal(SETTINGS_SECTION_MIN_ROLE.enterprise, "viewer");
  assert.equal(isPromotionSection("enterprise", false), false);
  assert.equal(isPromotionSection("enterprise", true), true);
  // Only promotion sections are affected.
  assert.equal(isPromotionSection("general", true), false);
});
