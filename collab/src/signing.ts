// Request signing between the collaboration service and the Go server.
//
// Both directions carry two headers:
//
//   X-Collab-Timestamp: Unix milliseconds when the request was signed
//   X-Collab-Signature: hex HMAC-SHA256 over
//       "<timestamp>\n<METHOD>\n<path?query>\n<sha256-hex of body>"
//
// The timestamp must be within five minutes of the receiver's clock. The Go
// implementation in internal/docs/collab/sign.go produces byte-identical
// strings; a change on either side must be mirrored on the other.

import { createHash, createHmac, timingSafeEqual } from 'node:crypto'

export const TIMESTAMP_HEADER = 'x-collab-timestamp'
export const SIGNATURE_HEADER = 'x-collab-signature'
export const MAX_SKEW_MS = 5 * 60_000

export function sha256Hex(body: Uint8Array | string): string {
  return createHash('sha256').update(body).digest('hex')
}

/** The string both sides sign. */
export function canonical(method: string, pathWithQuery: string, timestamp: number | string, body: Uint8Array | string): string {
  return `${timestamp}\n${method.toUpperCase()}\n${pathWithQuery}\n${sha256Hex(body)}`
}

export function sign(secret: string, method: string, pathWithQuery: string, timestamp: number | string,
  body: Uint8Array | string): string {
  return createHmac('sha256', secret).update(canonical(method, pathWithQuery, timestamp, body)).digest('hex')
}

/** Headers to attach to an outgoing request. */
export function signedHeaders(secret: string, method: string, pathWithQuery: string, body: Uint8Array | string,
  now = Date.now()): Record<string, string> {
  return {
    [TIMESTAMP_HEADER]: String(now),
    [SIGNATURE_HEADER]: sign(secret, method, pathWithQuery, now, body),
  }
}

export type VerifyResult = { ok: true } | { ok: false; reason: string }

/** Checks the signature and freshness of an incoming request. */
export function verify(secret: string, method: string, pathWithQuery: string, timestampHeader: string | undefined,
  signatureHeader: string | undefined, body: Uint8Array | string, now = Date.now()): VerifyResult {
  if (!timestampHeader || !signatureHeader) return { ok: false, reason: 'missing signature headers' }
  const ts = Number(timestampHeader)
  if (!Number.isFinite(ts)) return { ok: false, reason: 'malformed timestamp' }
  if (Math.abs(now - ts) > MAX_SKEW_MS) return { ok: false, reason: 'timestamp outside the allowed window' }
  const expected = Buffer.from(sign(secret, method, pathWithQuery, timestampHeader, body), 'hex')
  let given: Buffer
  try {
    given = Buffer.from(signatureHeader, 'hex')
  } catch {
    return { ok: false, reason: 'malformed signature' }
  }
  if (given.length !== expected.length || !timingSafeEqual(given, expected)) {
    return { ok: false, reason: 'signature mismatch' }
  }
  return { ok: true }
}
