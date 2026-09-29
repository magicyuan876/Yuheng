import assert from "node:assert/strict";
import { beforeEach, test, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";

const getDeploymentCapabilities = vi.hoisted(() => vi.fn());
vi.mock("@/api/system", () => ({ getDeploymentCapabilities }));

import { useDeploymentCapabilitiesStore } from "./deploymentCapabilities";

beforeEach(() => {
  setActivePinia(createPinia());
  getDeploymentCapabilities.mockReset();
});

test("a backend with extensions makes them available by name", async () => {
  getDeploymentCapabilities.mockResolvedValue({
    data: {
      capabilities: { docs: { supported: true } },
      extensions: {
        "acme.on": { supported: true },
        "acme.off": { supported: false, reason: "needs_something" },
      },
    },
  });
  const store = useDeploymentCapabilitiesStore();

  await store.ensureLoaded();

  assert.equal(store.isExtensionSupported("acme.on"), true);
  assert.equal(store.isExtensionSupported("acme.off"), false);
  assert.equal(store.extensionReason("acme.off"), "needs_something");
  assert.equal(store.isExtensionSupported("acme.unknown"), false);
});

test("a backend without extensions has none", async () => {
  getDeploymentCapabilities.mockResolvedValue({ data: { capabilities: { docs: { supported: true } } } });
  const store = useDeploymentCapabilitiesStore();

  await store.ensureLoaded();

  assert.deepEqual(store.extensions, {});
  assert.equal(store.isExtensionSupported("acme.on"), false);
});

// The two kinds fail in opposite directions on purpose: the built-in entries
// stay visible when the probe fails (the backend refuses them anyway), while an
// extension feature has nothing to show without an answer that says it exists.
test("when the probe fails, built-ins stay visible and extension features are hidden", async () => {
  getDeploymentCapabilities.mockRejectedValue(new Error("network down"));
  const store = useDeploymentCapabilitiesStore();

  await store.ensureLoaded();

  assert.equal(store.loaded, true);
  assert.equal(store.loadError, "network down");
  assert.equal(store.isSupported("docs"), true);
  assert.equal(store.isExtensionSupported("acme.on"), false);
});

test("a refresh replaces what the previous answer said, so a switched-off feature disappears", async () => {
  const store = useDeploymentCapabilitiesStore();
  getDeploymentCapabilities.mockResolvedValueOnce({
    data: { capabilities: {}, extensions: { "acme.on": { supported: true } } },
  });
  await store.ensureLoaded();
  assert.equal(store.isExtensionSupported("acme.on"), true);

  getDeploymentCapabilities.mockResolvedValueOnce({
    data: { capabilities: {}, extensions: { "acme.on": { supported: false, reason: "switched_off" } } },
  });
  await store.ensureLoaded(true);

  assert.equal(store.isExtensionSupported("acme.on"), false);
  assert.equal(store.extensionReason("acme.on"), "switched_off");
});

test("the store reports enabled, locked and unavailable states, and unavailable after a failed probe", async () => {
  getDeploymentCapabilities.mockResolvedValueOnce({
    data: {
      extensions: {
        "acme.on": { supported: true },
        "acme.off": { supported: false, reason: "license_expired_for_build" },
      },
    },
  });
  const store = useDeploymentCapabilitiesStore();
  await store.ensureLoaded();
  assert.equal(store.extensionState("acme.on"), "enabled");
  assert.equal(store.extensionState("acme.off"), "locked");
  assert.equal(store.extensionReason("acme.off"), "license_expired_for_build");
  assert.equal(store.extensionState("acme.other"), "unavailable");

  getDeploymentCapabilities.mockRejectedValueOnce(new Error("down"));
  await store.ensureLoaded(true);
  assert.equal(store.extensionState("acme.on"), "unavailable");
});
