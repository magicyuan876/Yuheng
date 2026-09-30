import assert from "node:assert/strict";
import { test } from "vitest";

import {
  SETTINGS_SECTION_CAPABILITY,
  extensionState,
  extensionUnavailableReason,
  isDeploymentCapabilitySupported,
  isExtensionEnabled,
  type DeploymentCapabilityMap,
  type ExtensionCapabilityMap,
} from "./deploymentCapabilities";

test("capability filtering is fail-open unless backend explicitly disables a feature", () => {
  assert.equal(isDeploymentCapabilitySupported({}, "organizations"), true);

  const capabilities: DeploymentCapabilityMap = {
    organizations: { supported: false, reason: "route_not_registered" },
    "settings.storage": { supported: true },
  };
  assert.equal(isDeploymentCapabilitySupported(capabilities, "organizations"), false);
  assert.equal(isDeploymentCapabilitySupported(capabilities, "settings.storage"), true);
});

test("only route-backed settings sections require deployment capabilities", () => {
  assert.equal(SETTINGS_SECTION_CAPABILITY.websearch, "settings.websearch");
  assert.equal(SETTINGS_SECTION_CAPABILITY.storage, "settings.storage");
  assert.equal(SETTINGS_SECTION_CAPABILITY.parser, undefined);
  assert.equal(SETTINGS_SECTION_CAPABILITY["runtime-queues"], undefined);
});

test("extension features are fail-closed: they exist only when the backend says supported", () => {
  const extensions: ExtensionCapabilityMap = {
    "acme.on": { supported: true },
    "acme.off": { supported: false, reason: "needs_something" },
  };

  assert.equal(isExtensionEnabled(extensions, "acme.on"), true);
  assert.equal(isExtensionEnabled(extensions, "acme.off"), false);
  assert.equal(isExtensionEnabled(extensions, "acme.unknown"), false, "a key nobody reported does not exist");
  assert.equal(isExtensionEnabled({}, "acme.on"), false);
});

test("an extension feature explains why it is unavailable only when the backend gave a reason", () => {
  const extensions: ExtensionCapabilityMap = {
    "acme.on": { supported: true, reason: "ignored while supported" },
    "acme.off": { supported: false, reason: "needs_something" },
    "acme.bare": { supported: false },
  };

  assert.equal(extensionUnavailableReason(extensions, "acme.off"), "needs_something");
  assert.equal(extensionUnavailableReason(extensions, "acme.on"), undefined);
  assert.equal(extensionUnavailableReason(extensions, "acme.bare"), undefined);
  assert.equal(extensionUnavailableReason(extensions, "acme.unknown"), undefined);
});

test("an extension feature is enabled, locked or unavailable", () => {
  const extensions: ExtensionCapabilityMap = {
    "acme.on": { supported: true },
    "acme.locked": { supported: false, reason: "license_required" },
    "acme.locked_no_reason": { supported: false },
  };
  assert.equal(extensionState(extensions, "acme.on"), "enabled");
  assert.equal(extensionState(extensions, "acme.locked"), "locked");
  assert.equal(extensionState(extensions, "acme.locked_no_reason"), "locked");
  // A community build and a failed probe both look like this.
  assert.equal(extensionState(extensions, "acme.absent"), "unavailable");
  assert.equal(extensionState({}, "acme.on"), "unavailable");
  // The fail-closed check is untouched: only "enabled" passes.
  assert.equal(isExtensionEnabled(extensions, "acme.locked"), false);
});
