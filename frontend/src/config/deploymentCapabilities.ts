export const DEPLOYMENT_CAPABILITY_KEYS = [
  'organizations',
  'settings.websearch',
  'settings.vectorstore',
  'settings.storage',
  'docs',
  'docs.public_sharing',
] as const

export type DeploymentCapabilityKey = typeof DEPLOYMENT_CAPABILITY_KEYS[number]

export interface DeploymentCapability {
  supported: boolean
  reason?: string
}

export type DeploymentCapabilityMap = Partial<Record<DeploymentCapabilityKey, DeploymentCapability>>

/**
 * 能力接口失败或旧版后端没有返回某个键时保持可见，避免一次探测失败把整个菜单清空。
 * 只有后端明确返回 supported: false 时才隐藏入口。
 */
export function isDeploymentCapabilitySupported(
  capabilities: DeploymentCapabilityMap,
  key?: DeploymentCapabilityKey,
): boolean {
  if (!key) return true
  return capabilities[key]?.supported !== false
}

export const SETTINGS_SECTION_CAPABILITY: Partial<Record<string, DeploymentCapabilityKey>> = {
  websearch: 'settings.websearch',
  vectorstore: 'settings.vectorstore',
  storage: 'settings.storage',
  // Tenant user groups are part of the docs module's permission model, so
  // the section follows that capability rather than one of its own.
  groups: 'docs',
}
