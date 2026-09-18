type QueryValue = string | number | null | undefined | Array<string | number | null>
export type SettingsRouteQuery = Record<string, QueryValue>

// Legacy "发布集成" (integrations) settings sections were removed together with
// the agent publishing surfaces (IM / embed channels, agent API playground).
// Old bookmarks that name an integration section now land on the settings home.
const LEGACY_INTEGRATION_SECTIONS = new Set([
  'integrations',
  'integration-im',
  'integration-embed',
  'integration-api',
  'integration-chrome',
  'integration-claw',
  'im',
  'embed',
  'api',
  'chrome',
  'claw',
])

/**
 * Map URL `section` (and a leftover `tab` from old bookmarks) onto the
 * settings nav key. Removed integration sections fall back to 'general'.
 */
export function normalizeSettingsSection(section: string, tab?: string | null): string {
  if (LEGACY_INTEGRATION_SECTIONS.has(section)) {
    return 'general'
  }
  if (tab && LEGACY_INTEGRATION_SECTIONS.has(tab)) {
    return 'general'
  }
  return section
}

/**
 * Settings left-nav → URL. Every page is `?section=<navKey>`; `tab` is dropped.
 */
export function buildSettingsRouteQuery(
  sectionKey: string,
  currentQuery: object = {},
): SettingsRouteQuery {
  const query: SettingsRouteQuery = { ...(currentQuery as SettingsRouteQuery) }
  delete query.tab
  delete query.agentId
  query.section = sectionKey
  return query
}

export function settingsQueryUnchanged(
  currentQuery: object,
  nextQuery: SettingsRouteQuery,
): boolean {
  const current = currentQuery as SettingsRouteQuery
  return String(current.section ?? '') === String(nextQuery.section ?? '')
    && current.tab == null
    && nextQuery.tab == null
    && String(current.agentId ?? '') === String(nextQuery.agentId ?? '')
}
