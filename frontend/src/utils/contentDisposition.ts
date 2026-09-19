// Reading the file name a download was given.
//
// Its own module, with no imports, so it can be unit-tested without dragging
// axios and the whole request stack into the test runner.
//
// A Content-Disposition header can carry the name twice. The plain
// `filename=` parameter may only hold ASCII, so a page called 存储配额说明
// arrives there as underscores; the RFC 5987 `filename*=UTF-8''...` form
// carries the real name. Prefer the encoded one and fall back, which is the
// same order every browser uses.

/** The name to save a download as, or the fallback when the header says nothing usable. */
export function fileNameFromDisposition(header: string | undefined, fallback: string): string {
  if (!header) return fallback;
  const encoded = /filename\*\s*=\s*UTF-8''([^;]+)/i.exec(header);
  if (encoded) {
    try {
      return decodeURIComponent(encoded[1].trim());
    } catch {
      // A malformed header is not worth failing a download over; fall through
      // to the ASCII parameter, and then to the fallback.
    }
  }
  const plain = /filename\s*=\s*"?([^";]+)"?/i.exec(header);
  if (plain) {
    const name = plain[1].trim();
    if (name) return name;
  }
  return fallback;
}
