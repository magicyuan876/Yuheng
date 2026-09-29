import { computed } from "vue";
import { useAuthStore } from "@/stores/auth";
import { useDeploymentCapabilitiesStore } from "@/stores/deploymentCapabilities";

/**
 * What the enterprise components need to know about the deployment: the state
 * and reason per feature key, and whether the viewer administers the tenant
 * (only they are pointed at the license page).
 */
export function useEnterpriseFeatureContext() {
  const capabilities = useDeploymentCapabilitiesStore();
  const auth = useAuthStore();
  const isAdmin = computed(() => auth.canAccessAllTenants || auth.hasRole("admin"));
  return {
    isAdmin,
    stateOf: (key: string) => capabilities.extensionState(key),
    reasonOf: (key: string) => capabilities.extensionReason(key),
  };
}
