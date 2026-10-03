export type SettingsRoleKey = "viewer" | "contributor" | "admin" | "owner";

/**
 * Workspace-scoped settings access policy.
 *
 * Keep this as the single frontend source of truth for both the complete
 * Settings navigation and any shortcuts that lead into it. Backend route
 * guards remain authoritative.
 */
export const SETTINGS_SECTION_MIN_ROLE: Record<string, SettingsRoleKey> = {
  general: "viewer",
  ollama: "admin",
  models: "viewer",
  websearch: "admin",
  chathistory: "admin",
  vectorstore: "admin",
  parser: "admin",
  storage: "admin",
  system: "viewer",
  userprofile: "viewer",
  tenant: "viewer",
  members: "viewer",
  groups: "viewer",
  // GET/POST/PUT/DELETE /tenants/:id/api-keys are g.Owner(); a lower role
  // could open the page but not even list the keys.
  apikeys: "owner",
  // A read-only description of the Enterprise edition; nothing on it can change state.
  enterprise: "viewer",
};

/**
 * A management-labelled avatar shortcut has a stricter threshold than the
 * corresponding read-only Settings page.
 */
export const SETTINGS_MANAGEMENT_SHORTCUT_MIN_ROLE = {
  members: "owner",
  models: "admin",
} as const satisfies Record<string, SettingsRoleKey>;

export const SYSTEM_ADMIN_SETTINGS_SECTIONS = new Set([
  "users-workspaces",
  "system-global",
  "runtime-queues",
  "platform-api-keys",
  "system-audit-log",
]);

/**
 * Sections whose ownership moves to the platform when the
 * governance.centralized_infra system setting is on.
 *
 * These are the shared-infrastructure pages: their contents are selected at
 * point of use (the knowledge-base editor lists models, vector stores,
 * storage backends and parser engines through their own Viewer+ read
 * endpoints), so hiding the settings entry costs a non-admin nothing — they
 * never needed to configure the backing service, only to pick one.
 *
 * Deliberately NOT in this set:
 *  - chathistory / tenant / members — workspace dimension.
 *  - general / userprofile — personal dimension.
 *
 * Raising SETTINGS_SECTION_MIN_ROLE instead would achieve nothing: every
 * self-registered user is Owner of their own personal workspace, so no role
 * threshold can express "only the platform operator". Backend route guards
 * (middleware.RequirePlatformManaged) remain authoritative; this table only
 * decides whether the navigation entry is rendered.
 */
export const PLATFORM_MANAGED_SETTINGS_SECTIONS = new Set([
  "models",
  "ollama",
  "websearch",
  "vectorstore",
  "parser",
  "storage",
]);

/**
 * Whether a settings section should be hidden from this caller.
 *
 * `centralizedInfra` comes from the governance store; it is false while the
 * probe is in flight or has failed, which fails open to today's behaviour.
 */
export function isPlatformManagedSection(key: string, centralizedInfra: boolean): boolean {
  return centralizedInfra && PLATFORM_MANAGED_SETTINGS_SECTIONS.has(key);
}

/**
 * Sections that exist only to promote something. The operator can switch them
 * off (HIDE_ENTERPRISE_PROMOTION), and unlike every other section they are
 * then hidden for everybody, whatever their role.
 */
export const PROMOTION_SETTINGS_SECTIONS = new Set(["enterprise"]);

export function isPromotionSection(key: string, promotionHidden: boolean): boolean {
  return promotionHidden && PROMOTION_SETTINGS_SECTIONS.has(key);
}
