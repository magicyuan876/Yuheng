import { computed } from "vue";
import { useChatResourcesStore } from "@/stores/chatResources";
import { useDeploymentCapabilitiesStore } from "@/stores/deploymentCapabilities";
import { useSettingsStore } from "@/stores/settings";

/**
 * The chat input's web-search switch.
 *
 * Three facts are kept apart:
 *  - `available`: can a chat turn search the web at all? The deployment must
 *    ship web search (the `settings.websearch` capability) and the workspace
 *    must resolve a default provider — without one the backend logs
 *    `web_config_missing` and quietly answers without searching.
 *  - `enabled`: the user's preference, persisted with the other chat-input
 *    settings and restored per session from last_request_state.
 *  - `requested`: what a request sends as web_search_enabled — the
 *    preference, but only while web search is available.
 */
export function useWebSearchToggle() {
  const settings = useSettingsStore();
  const chatResources = useChatResourcesStore();
  const deploymentCapabilities = useDeploymentCapabilitiesStore();

  const available = computed(
    () => deploymentCapabilities.isSupported("settings.websearch") && chatResources.hasDefaultWebSearchProvider,
  );

  const enabled = computed({
    get: () => settings.isWebSearchEnabled,
    set: (value: boolean) => settings.setWebSearchEnabled(value),
  });

  const requested = computed(() => available.value && enabled.value);

  /** Loads what `available` depends on; cached like the other chat-input resources. */
  async function ensureLoaded(force = false): Promise<void> {
    await deploymentCapabilities.ensureLoaded();
    if (!deploymentCapabilities.isSupported("settings.websearch")) return;
    await chatResources.ensureWebSearchProviders(force);
  }

  return { available, enabled, requested, ensureLoaded };
}
