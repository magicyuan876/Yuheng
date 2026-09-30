// Pure logic for the sidebar session list: which folder a session belongs to,
// and the date buckets the web folder is grouped by.

/** Mirrors backend types.SessionOwnerAPITenantKeyPrefix (sessions.user_id owner). */
export const API_SESSION_OWNER_PREFIX = "api_tenant_key:";

/** Mirrors backend types.SessionOwnerAPIExternalUserPrefix. */
export const API_EXTERNAL_USER_SESSION_OWNER_PREFIX = "api_external_user:";

export interface SessionForGrouping {
  id: string;
  title?: string;
  is_pinned?: boolean;
  created_at?: string;
  updated_at?: string;
  user_id?: string;
  originalIndex?: number;
}

export interface SessionGroup<T extends SessionForGrouping = SessionForGrouping> {
  key: string;
  label: string;
  items: T[];
}

/**
 * Where a session came from: the user's own Web-console chats, or the
 * admin-only folder of sessions created through API keys. (Upstream also had
 * IM and embed channels; both were removed, and the backend files their
 * legacy rows under neither web nor api.)
 */
export type SessionOrigin = { kind: "web" } | { kind: "api" };

export function resolveSessionOrigin(session: SessionForGrouping): SessionOrigin {
  const ownerId = session.user_id || "";
  if (ownerId.startsWith(API_SESSION_OWNER_PREFIX) || ownerId.startsWith(API_EXTERNAL_USER_SESSION_OWNER_PREFIX)) {
    return { kind: "api" };
  }
  return { kind: "web" };
}

/** The sidebar bucket key for an origin; matches sessionSidebarBuckets' keys. */
export function originGroupKey(origin: SessionOrigin): string {
  return origin.kind;
}

export function classifyDateBucket(dateStr: string | undefined): DateBucketKey {
  if (!dateStr) return "earlier";

  const date = new Date(dateStr);
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterday = new Date(today.getTime() - 24 * 60 * 60 * 1000);
  const sevenDaysAgo = new Date(today.getTime() - 7 * 24 * 60 * 60 * 1000);
  const thirtyDaysAgo = new Date(today.getTime() - 30 * 24 * 60 * 60 * 1000);
  const oneYearAgo = new Date(today.getTime() - 365 * 24 * 60 * 60 * 1000);
  const sessionDate = new Date(date.getFullYear(), date.getMonth(), date.getDate());

  if (sessionDate.getTime() >= today.getTime()) return "today";
  if (sessionDate.getTime() >= yesterday.getTime()) return "yesterday";
  if (date.getTime() >= sevenDaysAgo.getTime()) return "last7Days";
  if (date.getTime() >= thirtyDaysAgo.getTime()) return "last30Days";
  if (date.getTime() >= oneYearAgo.getTime()) return "lastYear";
  return "earlier";
}

const DATE_BUCKET_ORDER = ["pinned", "today", "yesterday", "last7Days", "last30Days", "lastYear", "earlier"] as const;

export type DateBucketKey = (typeof DATE_BUCKET_ORDER)[number];

export function groupSessionsByDate<T extends SessionForGrouping>(
  sessions: T[],
  bucketLabels: Record<DateBucketKey, string>,
  categorize: (session: T) => DateBucketKey,
): SessionGroup<T>[] {
  const buckets = new Map<DateBucketKey, T[]>();
  for (const key of DATE_BUCKET_ORDER) buckets.set(key, []);

  for (const session of sessions) {
    const bucket: DateBucketKey = session.is_pinned ? "pinned" : categorize(session);
    buckets.get(bucket)!.push(session);
  }

  return DATE_BUCKET_ORDER.filter((key) => (buckets.get(key)?.length ?? 0) > 0).map((key) => ({
    key,
    label: bucketLabels[key],
    items: buckets.get(key)!,
  }));
}
