import { ref } from "vue";
import { defineStore } from "pinia";
import { getDeploymentCapabilities } from "@/api/system";
import {
  extensionUnavailableReason,
  isDeploymentCapabilitySupported,
  isExtensionEnabled,
  type DeploymentCapabilityKey,
  type DeploymentCapabilityMap,
  type ExtensionCapabilityMap,
} from "@/config/deploymentCapabilities";

export const useDeploymentCapabilitiesStore = defineStore("deploymentCapabilities", () => {
  const capabilities = ref<DeploymentCapabilityMap>({});
  const extensions = ref<ExtensionCapabilityMap>({});
  const docsCollabUrl = ref("");
  const loaded = ref(false);
  const loadError = ref("");
  let loadingPromise: Promise<void> | null = null;

  const ensureLoaded = async (force = false): Promise<void> => {
    if (loaded.value && !force) return;
    if (loadingPromise) return loadingPromise;

    loadingPromise = (async () => {
      try {
        const response = await getDeploymentCapabilities();
        capabilities.value = response.data?.capabilities || {};
        extensions.value = response.data?.extensions || {};
        docsCollabUrl.value = response.data?.docs_collab_url || "";
        loadError.value = "";
      } catch (error) {
        // 能力探测失败时保持 fail-open；权限仍由后端路由最终校验。
        capabilities.value = {};
        // Unlike the built-ins, a failed probe hides extension features: with no
        // answer there is nothing to say they exist.
        extensions.value = {};
        docsCollabUrl.value = "";
        loadError.value = error instanceof Error ? error.message : String(error);
      } finally {
        loaded.value = true;
        loadingPromise = null;
      }
    })();

    return loadingPromise;
  };

  const isSupported = (key?: DeploymentCapabilityKey) => {
    return isDeploymentCapabilitySupported(capabilities.value, key);
  };

  const isExtensionSupported = (key: string) => isExtensionEnabled(extensions.value, key);
  const extensionReason = (key: string) => extensionUnavailableReason(extensions.value, key);

  return {
    capabilities,
    extensions,
    docsCollabUrl,
    loaded,
    loadError,
    ensureLoaded,
    isSupported,
    isExtensionSupported,
    extensionReason,
  };
});
