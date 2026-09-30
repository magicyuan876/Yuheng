/**
 * Display helpers shared by the workspace and platform API-key screens.
 */

/**
 * The shape the backend's maskManagedAPIKey produces: the first seven
 * characters, an ellipsis, the last four.
 *
 * The platform list arrives masked already, but the workspace list
 * (GET /tenants/:id/api-keys) returns `api_key` in full. The screen must not
 * put a live secret into the DOM, so the value is always masked here. Applying
 * it to a value that is already masked is a no-op — seven characters, "...",
 * four characters come out exactly as they went in — so the screens keep
 * working unchanged once the backend masks the workspace list too.
 */
export function maskApiKey(value: string | null | undefined): string {
  const token = (value ?? "").trim();
  if (token.length <= 12) return "***";
  return `${token.slice(0, 7)}...${token.slice(-4)}`;
}

/** Local "YYYY-MM-DD HH:mm", or "" for a missing or unparsable value. */
export function formatApiKeyDateTime(value?: string | null): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return `${formatApiKeyDate(date)} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** Local "YYYY-MM-DD" of a date — also the value format of <input type="date">. */
export function formatApiKeyDate(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

function pad(part: number): string {
  return String(part).padStart(2, "0");
}
