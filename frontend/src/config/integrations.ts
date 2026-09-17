import type { DeploymentCapabilityKey } from './deploymentCapabilities'

export const CHROME_EXTENSION_URL =
  'https://chromewebstore.google.com/detail/jpemjbopikggjlmikmclgbmkhhopjdgd?utm_source=item-share-cb'

export const CLAWHUB_SKILL_URL = 'https://clawhub.ai/lyingbug/yuheng'

export type IntegrationTab = 'im' | 'embed' | 'api' | 'chrome' | 'claw'

/**
 * Route vocabulary — every tab an URL may legally name. Keep 'chrome' and
 * 'claw' here so existing bookmarks still resolve instead of 404-ing; what the
 * Integrations Center actually renders is INTEGRATION_PREVIEW_ITEMS below.
 */
export const INTEGRATION_TABS: IntegrationTab[] = ['im', 'embed', 'api', 'chrome', 'claw']

/**
 * Tabs hidden from the UI. 'chrome' and 'claw' promote upstream-branded
 * third-party distributions (the Chrome Web Store extension and the ClawHub
 * skill). Empty this set to surface them again.
 */
export const HIDDEN_INTEGRATION_TABS = new Set<IntegrationTab>(['chrome', 'claw'])

/** Aligns with Settings.vue SECTION_MIN_ROLE.api and router.go g.Owner() on /api-principal-config. */
export type IntegrationTabRole = 'viewer' | 'contributor' | 'admin' | 'owner'

export const INTEGRATION_TAB_MIN_ROLE: Partial<Record<IntegrationTab, IntegrationTabRole>> = {
  api: 'owner',
}

export const INTEGRATION_TAB_CAPABILITY: Partial<Record<IntegrationTab, DeploymentCapabilityKey>> = {
  im: 'integrations.im',
  embed: 'integrations.embed',
  api: 'integrations.api',
}

export type IntegrationPreviewIcon =
  | { type: 'icon'; name: string }
  | { type: 'emoji'; value: string }

/** Sidebar hover preview + Integrations modal nav — add new entries here. */
export const INTEGRATION_PREVIEW_ITEMS: Array<{
  key: IntegrationTab
  icon: IntegrationPreviewIcon
}> = [
  { key: 'im', icon: { type: 'icon', name: 'chat-message' } },
  { key: 'embed', icon: { type: 'icon', name: 'code' } },
  { key: 'api', icon: { type: 'icon', name: 'secured' } },
]
