import { safeRemoveItem } from "@/composables/preferenceStorage";

export const SETTINGS_STORAGE_KEY = "Yuheng_settings";

function isSettingsRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

/**
 * Load the settings from localStorage over a deep copy of the defaults.
 *
 * Stored fields win; a top-level field the stored copy lacks takes its
 * default, so adding a setting needs no per-field guard anywhere else. A
 * stored value that is not a settings object at all (corrupt JSON, `null`, an
 * array) is dropped and the defaults are used.
 */
export function loadSettings<T extends object>(defaultSettings: T): T {
  const defaults: T = JSON.parse(JSON.stringify(defaultSettings));
  try {
    const raw = localStorage.getItem(SETTINGS_STORAGE_KEY);
    if (!raw) return defaults;
    const parsed: unknown = JSON.parse(raw);
    if (isSettingsRecord(parsed)) return { ...defaults, ...parsed };
    console.error("[settings] Stored Yuheng_settings is not a settings object, resetting to defaults");
  } catch (e) {
    console.error("[settings] Failed to parse Yuheng_settings from localStorage, resetting to defaults:", e);
  }
  safeRemoveItem(SETTINGS_STORAGE_KEY);
  return defaults;
}
