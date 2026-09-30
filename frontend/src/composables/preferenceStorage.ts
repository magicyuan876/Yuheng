/**
 * Shared localStorage utilities for per-user UI preferences (theme, fonts).
 *
 * Storage layout: Yuheng_${userId}_${suffix}, where userId is the active
 * user's id or "anon" before login. Read paths are intentionally narrow —
 * no cross-namespace fallbacks — so one user's preferences cannot bleed
 * into another user's session.
 *
 * At login the "anon" namespace (written while nobody is logged in, e.g. a
 * theme picked on the login page) is adopted into the user's namespace and
 * cleared, so the choice carries over and the next user to log in cannot
 * inherit it.
 */

const PREFERENCE_SUFFIXES = ["theme", "font_sans", "font_mono", "font_size"] as const;

export function readUserId(): string {
  try {
    const raw = localStorage.getItem("yuheng_user");
    if (!raw) return "anon";
    const parsed = JSON.parse(raw);
    return parsed?.id ? String(parsed.id) : "anon";
  } catch {
    return "anon";
  }
}

export function safeGetItem(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

export function safeSetItem(key: string, value: string): void {
  try {
    localStorage.setItem(key, value);
  } catch (err) {
    // Quota exceeded, disabled storage, private mode — surface in DevTools
    // so the issue is at least diagnosable, but don't break the UI.
    console.warn(`[Yuheng] failed to persist preference "${key}":`, err);
  }
}

export function safeRemoveItem(key: string): void {
  try {
    localStorage.removeItem(key);
  } catch {
    // Same conditions as setItem; silent best-effort.
  }
}

export function userKey(suffix: string): string {
  return `Yuheng_${readUserId()}_${suffix}`;
}

export function loadPreference(suffix: string): string | null {
  return safeGetItem(userKey(suffix));
}

export function savePreference(suffix: string, value: string): void {
  safeSetItem(userKey(suffix), value);
}

let adoptedForUser: string | null = null;

/**
 * Adopt the anon preferences into the current user's namespace, then remove
 * the anon keys. A value the user already has wins over the anon one.
 * Idempotent per session per user — repeat calls for the same userId are
 * no-ops. Safe to call before the user is logged in (it returns early when
 * userId === "anon").
 */
export function adoptAnonPreferences(): void {
  const userId = readUserId();
  if (userId === "anon") return;
  if (adoptedForUser === userId) return;
  adoptedForUser = userId;

  for (const suffix of PREFERENCE_SUFFIXES) {
    const target = `Yuheng_${userId}_${suffix}`;
    const anonKey = `Yuheng_anon_${suffix}`;
    const anonValue = safeGetItem(anonKey);
    if (anonValue !== null && safeGetItem(target) === null) {
      safeSetItem(target, anonValue);
    }
    // Always clear the anon key so a later user cannot inherit it.
    safeRemoveItem(anonKey);
  }
}

/** Resets the per-session adoption latch (used when the active user changes). */
export function resetAdoptionLatch(): void {
  adoptedForUser = null;
}

// Adopt once at module load so the composables that read from storage see
// the adopted values when initialising their refs.
adoptAnonPreferences();
