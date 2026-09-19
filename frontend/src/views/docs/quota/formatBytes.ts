// Byte sizes a person can read, and the parsing that lets them type one back.
//
// Separated from the component so both directions can be tested without a
// browser, because a storage figure that is wrong by a factor of 1024 is the
// kind of bug that survives review and then decides somebody's quota.

/** Units, ascending. Binary multiples: a "GB" here is 1024³, matching what
 * every file manager the reader has ever used calls a GB. */
const UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'] as const

const STEP = 1024

/**
 * Renders a byte count for a person.
 *
 * Whole numbers below the first decimal point are shown without one: "5 MB"
 * rather than "5.0 MB". Zero renders as "0 B" rather than an empty string,
 * because an empty cell reads as missing data.
 */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return '—'
  if (bytes < STEP) return `${Math.round(bytes)} B`

  let value = bytes
  let unit = 0
  while (value >= STEP && unit < UNITS.length - 1) {
    value /= STEP
    unit++
  }
  // One decimal below 10, none above: 9.4 GB is useful, 943.2 GB is noise.
  const rounded = value < 10 ? Math.round(value * 10) / 10 : Math.round(value)
  return `${rounded} ${UNITS[unit]}`
}

/**
 * Parses what somebody typed into a byte count, or null if it is not a size.
 *
 * Accepts "500", "500 MB", "1.5gb", "2 TiB". A bare number is bytes, which is
 * the only reading that cannot silently multiply somebody's intent by a
 * thousand.
 */
export function parseBytes(input: string): number | null {
  const text = input.trim().toLowerCase().replace(/\s+/g, '')
  if (!text) return null

  const match = /^(\d+(?:\.\d+)?)(b|kb|mb|gb|tb|pb|kib|mib|gib|tib|pib)?$/.exec(text)
  if (!match) return null

  const value = Number.parseFloat(match[1])
  if (!Number.isFinite(value) || value < 0) return null

  const suffix = (match[2] ?? 'b').replace('i', '') // KiB and KB mean the same here
  const power = ['b', 'kb', 'mb', 'gb', 'tb', 'pb'].indexOf(suffix)
  if (power < 0) return null

  const bytes = Math.round(value * STEP ** power)
  return Number.isSafeInteger(bytes) ? bytes : null
}

/**
 * How full a space is, 0–1, or null when it has no limit.
 *
 * Clamped at 1: a space that is over its quota (because the limit was
 * lowered) should show a full bar rather than one that overflows its track.
 */
export function usageFraction(used: number, quota: number): number | null {
  if (quota <= 0) return null
  if (used <= 0) return 0
  return Math.min(1, used / quota)
}

/** Whether a usage level deserves attention. */
export function usageLevel(used: number, quota: number): 'ok' | 'warning' | 'full' {
  const fraction = usageFraction(used, quota)
  if (fraction === null) return 'ok'
  if (fraction >= 1) return 'full'
  if (fraction >= 0.9) return 'warning'
  return 'ok'
}
