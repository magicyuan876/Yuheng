import type { TenantAPIKeyCapability } from "@/api/tenant";

/**
 * The capability vocabulary of API keys, grouped the way the pickers show it.
 *
 * Mirrors types.APIKeyCapability in internal/types/tenant_api_key.go. The
 * groups are the single source: the flat capability lists below are derived
 * from them, so a capability added to a group is automatically part of what a
 * form submits. (It used to be a separate hand-kept list, which is how the
 * three docs_* capabilities were shown on the platform screen but silently
 * dropped from the request.)
 *
 * Labels live under `apiKeys.capabilities.<value>` and group titles under
 * `apiKeys.capabilityGroups.<key>` in every locale; use the helpers below
 * rather than spelling the keys out.
 */
export type ApiKeyCapabilityGroupKey = "knowledge" | "dataSources" | "collaboration" | "docs" | "tenant" | "system";

export interface ApiKeyCapabilityGroup {
  key: ApiKeyCapabilityGroupKey;
  capabilities: TenantAPIKeyCapability[];
}

/** What a workspace key can be granted. Platform keys get these as well, applied to the X-Tenant-ID target. */
export const TENANT_API_KEY_CAPABILITY_GROUPS: readonly ApiKeyCapabilityGroup[] = [
  { key: "knowledge", capabilities: ["retrieve", "chat", "ingest", "manage_kbs", "message_history"] },
  { key: "dataSources", capabilities: ["manage_datasources"] },
  { key: "collaboration", capabilities: ["manage_members"] },
  { key: "docs", capabilities: ["docs_read", "docs_write", "docs_admin"] },
  {
    key: "tenant",
    capabilities: [
      "manage_models",
      "manage_vector_stores",
      "manage_storage_backends",
      "manage_web_search",
      "run_evaluations",
      "manage_tenant_settings",
    ],
  },
];

/** The platform control plane. The backend honours these on platform keys only. */
export const SYSTEM_API_KEY_CAPABILITY_GROUP: ApiKeyCapabilityGroup = {
  key: "system",
  capabilities: [
    "system_tenants_read",
    "system_tenants_manage",
    "system_settings_read",
    "system_settings_manage",
    "system_runtime_read",
    "system_runtime_manage",
    "system_audit_read",
  ],
};

export const PLATFORM_API_KEY_CAPABILITY_GROUPS: readonly ApiKeyCapabilityGroup[] = [
  SYSTEM_API_KEY_CAPABILITY_GROUP,
  ...TENANT_API_KEY_CAPABILITY_GROUPS,
];

/**
 * Capabilities whose routes are bounded by a key's knowledge_base_ids
 * allow-list. For every other capability the allow-list has no effect, so the
 * workspace form only asks for one when at least one of these is granted.
 */
export const KB_SCOPED_API_KEY_CAPABILITIES: ReadonlySet<TenantAPIKeyCapability> = new Set([
  "retrieve",
  "chat",
  "ingest",
  "manage_kbs",
  "manage_datasources",
]);

/** Every capability in the given groups, in display order. */
export function capabilitiesOf(groups: readonly ApiKeyCapabilityGroup[]): TenantAPIKeyCapability[] {
  return groups.flatMap((group) => group.capabilities);
}

export function capabilityLabelKey(capability: TenantAPIKeyCapability): string {
  return `apiKeys.capabilities.${capability}.label`;
}

export function capabilityHintKey(capability: TenantAPIKeyCapability): string {
  return `apiKeys.capabilities.${capability}.hint`;
}

export function capabilityGroupLabelKey(group: ApiKeyCapabilityGroupKey): string {
  return `apiKeys.capabilityGroups.${group}`;
}
