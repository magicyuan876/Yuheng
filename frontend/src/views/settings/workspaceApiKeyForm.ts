import type { TenantAPIKey, TenantAPIKeyCapability, UpdateTenantAPIKeyPayload } from "@/api/tenant";
import { formatApiKeyDate } from "@/components/api-keys/apiKeyDisplay";
import { KB_SCOPED_API_KEY_CAPABILITIES } from "@/config/apiKeyCapabilities";

/**
 * Form model and request mapping for the workspace API-key drawer, kept out of
 * the component so the rules the backend relies on are testable on their own.
 *
 * Backend contract (internal/handler/tenant.go, CreateAPIKey / UpdateAPIKey):
 *  - create and update take the same body; update REPLACES every field, so a
 *    missing expires_at_unix clears the expiry and a missing
 *    knowledge_base_ids widens the key to every knowledge base;
 *  - a scoped key needs at least one capability; a full-access key has its
 *    capabilities and knowledge-base allow-list cleared server-side;
 *  - create refuses an expiry that is not in the future; update does not.
 */

/** Preset lifetimes offered in the form, in days. */
export const EXPIRY_PRESET_DAYS = [30, 90, 180, 365] as const;

export type ExpiryChoice = "never" | "date" | `${(typeof EXPIRY_PRESET_DAYS)[number]}`;

export interface WorkspaceApiKeyForm {
  name: string;
  fullAccess: boolean;
  capabilities: TenantAPIKeyCapability[];
  knowledgeBaseIds: string[];
  expiry: ExpiryChoice;
  /** Local YYYY-MM-DD, used when expiry is "date". */
  expiryDate: string;
  /**
   * The key's current expiry when editing. Re-saving without touching the
   * date sends this instant back unchanged; the date field alone would move
   * it to the end of that day.
   */
  originalExpiresAt: Date | null;
}

export type WorkspaceApiKeyFormError = "nameRequired" | "capabilityRequired" | "expiryInFuture";

const DAY_MS = 24 * 60 * 60 * 1000;

/** A new key starts with the two capabilities nearly every integration needs, and a 90-day lifetime. */
export function emptyWorkspaceApiKeyForm(): WorkspaceApiKeyForm {
  return {
    name: "",
    fullAccess: false,
    capabilities: ["retrieve", "chat"],
    knowledgeBaseIds: [],
    expiry: "90",
    expiryDate: "",
    originalExpiresAt: null,
  };
}

export function workspaceApiKeyFormFromKey(key: TenantAPIKey): WorkspaceApiKeyForm {
  const expiresAt = key.expires_at ? new Date(key.expires_at) : null;
  const validExpiry = expiresAt && !Number.isNaN(expiresAt.getTime()) ? expiresAt : null;
  return {
    name: key.name,
    fullAccess: key.full_access,
    capabilities: [...(key.capabilities ?? [])],
    knowledgeBaseIds: [...(key.knowledge_base_ids ?? [])],
    expiry: validExpiry ? "date" : "never",
    expiryDate: validExpiry ? formatApiKeyDate(validExpiry) : "",
    originalExpiresAt: validExpiry,
  };
}

/** Whether the knowledge-base allow-list constrains anything the form grants. */
export function usesKnowledgeBaseScope(form: Pick<WorkspaceApiKeyForm, "fullAccess" | "capabilities">): boolean {
  return !form.fullAccess && form.capabilities.some((capability) => KB_SCOPED_API_KEY_CAPABILITIES.has(capability));
}

/**
 * The expiry the form asks for, or null for none. A picked date means the
 * end of that local day, so a key set to expire "on the 30th" still works on
 * the 30th.
 */
export function resolveExpiry(form: WorkspaceApiKeyForm, now: Date): Date | null {
  if (form.expiry === "never") return null;
  if (form.expiry === "date") {
    if (form.originalExpiresAt && form.expiryDate === formatApiKeyDate(form.originalExpiresAt)) {
      return form.originalExpiresAt;
    }
    const [year, month, day] = form.expiryDate.split("-").map(Number);
    if (!year || !month || !day) return null;
    return new Date(year, month - 1, day, 23, 59, 59);
  }
  return new Date(now.getTime() + Number(form.expiry) * DAY_MS);
}

export function validateWorkspaceApiKeyForm(form: WorkspaceApiKeyForm, now: Date): WorkspaceApiKeyFormError | null {
  if (!form.name.trim()) return "nameRequired";
  if (!form.fullAccess && form.capabilities.length === 0) return "capabilityRequired";
  if (form.expiry === "date") {
    const expiry = resolveExpiry(form, now);
    // Keeping an existing (even past) expiry is the user's call; a newly
    // picked date has to be in the future, which create enforces anyway.
    const unchanged = expiry !== null && expiry === form.originalExpiresAt;
    if (!expiry || (!unchanged && expiry.getTime() <= now.getTime())) return "expiryInFuture";
  }
  return null;
}

/** The request body for both create and update (see the contract above). */
export function workspaceApiKeyPayload(form: WorkspaceApiKeyForm, now: Date): UpdateTenantAPIKeyPayload {
  const expiry = resolveExpiry(form, now);
  const payload: UpdateTenantAPIKeyPayload = {
    name: form.name.trim(),
    full_access: form.fullAccess,
    capabilities: form.fullAccess ? [] : [...form.capabilities],
    // An allow-list nothing granted would consult is dropped rather than kept
    // dormant, where it would silently start to apply the day a KB-bound
    // capability is added.
    knowledge_base_ids: usesKnowledgeBaseScope(form) ? [...form.knowledgeBaseIds] : [],
  };
  if (expiry) payload.expires_at_unix = Math.floor(expiry.getTime() / 1000);
  return payload;
}

export function isApiKeyExpired(key: Pick<TenantAPIKey, "expires_at">, now: Date): boolean {
  if (!key.expires_at) return false;
  const expiresAt = new Date(key.expires_at).getTime();
  return !Number.isNaN(expiresAt) && expiresAt <= now.getTime();
}
