type QueryValue = string | number | null | undefined | Array<string | number | null>;
export type SettingsRouteQuery = Record<string, QueryValue>;

/**
 * Settings left-nav → URL. Every settings page is addressed as
 * `?section=<navKey>`; any other query keys the current route carries are
 * left alone so unrelated state survives a section switch.
 */
export function buildSettingsRouteQuery(sectionKey: string, currentQuery: object = {}): SettingsRouteQuery {
  return { ...(currentQuery as SettingsRouteQuery), section: sectionKey };
}

/** True when the route already shows `nextQuery`'s section, so a replace would be a no-op. */
export function settingsQueryUnchanged(currentQuery: object, nextQuery: SettingsRouteQuery): boolean {
  return String((currentQuery as SettingsRouteQuery).section ?? "") === String(nextQuery.section ?? "");
}
