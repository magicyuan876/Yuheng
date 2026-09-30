// Where two excerpts of a "divergent" finding differ, for highlighting.
//
// The server decides whether two passages differ (internal/application/
// service/findings/divergence.go) and centres both excerpts on the first
// difference; this marks the differences inside the excerpts so the eye goes
// straight to "15" against "10". It follows the server's rule about edges: two
// documents cut into passages at different places share a stretch of text and
// differ where one passage starts earlier or ends later, and that is not a
// difference worth pointing at. So only the changes between two anchors are
// marked — long shared runs, or the places where both excerpts begin or end
// together.

export interface DiffSegment {
  text: string;
  /** This text is not in the other excerpt. */
  changed: boolean;
}

export interface ExcerptDiff {
  subject: DiffSegment[];
  related: DiffSegment[];
}

/** The shortest shared run that anchors the alignment; the server uses the same. */
const ANCHOR_CHARS = 8;
/** Larger inputs are shown unmarked rather than compared: excerpts are about
 * 300 characters each, so this only ever stops something malformed. */
const MAX_CELLS = 250_000;

type Op = { kind: "=" | "-" | "+"; a: string[]; b: string[]; aStart: number; bStart: number };

/**
 * Splits two excerpts into segments, marking what differs between them inside
 * the text they share. Excerpts that cannot be aligned (nothing long in common,
 * or too large) come back as one unmarked segment each.
 */
export function diffExcerpts(subject: string, related: string): ExcerptDiff {
  const a = Array.from(subject);
  const b = Array.from(related);
  const plain = (): ExcerptDiff => ({
    subject: subject ? [{ text: subject, changed: false }] : [],
    related: related ? [{ text: related, changed: false }] : [],
  });
  if (subject === related || a.length * b.length > MAX_CELLS) return plain();

  const ops = align(a, b);
  let first = -1;
  let last = -1;
  let long = false;
  ops.forEach((op, i) => {
    if (op.kind !== "=") return;
    const isLong = op.a.length >= ANCHOR_CHARS;
    const atEdge =
      (op.aStart === 0 && op.bStart === 0) ||
      (op.aStart + op.a.length === a.length && op.bStart + op.b.length === b.length);
    if (!isLong && !atEdge) return;
    long = long || isLong;
    if (first < 0) first = i;
    last = i;
  });
  if (!long) return plain();

  const out: ExcerptDiff = { subject: [], related: [] };
  ops.forEach((op, i) => {
    const interior = i > first && i < last;
    if (op.a.length) push(out.subject, op.a.join(""), op.kind !== "=" && interior);
    if (op.b.length) push(out.related, op.b.join(""), op.kind !== "=" && interior);
  });
  return out;
}

function push(segments: DiffSegment[], text: string, changed: boolean) {
  const prev = segments[segments.length - 1];
  if (prev && prev.changed === changed) prev.text += text;
  else segments.push({ text, changed });
}

/** A longest-common-subsequence alignment of two character arrays, as runs of
 * shared, subject-only ("-") and related-only ("+") characters. */
function align(a: string[], b: string[]): Op[] {
  const n = a.length;
  const m = b.length;
  const width = m + 1;
  // lcs[i * width + j]: the length of the longest common subsequence of
  // a[i..] and b[j..].
  const lcs = new Uint16Array((n + 1) * width);
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      lcs[i * width + j] =
        a[i] === b[j] ? lcs[(i + 1) * width + j + 1] + 1 : Math.max(lcs[(i + 1) * width + j], lcs[i * width + j + 1]);
    }
  }
  const ops: Op[] = [];
  const emit = (kind: Op["kind"], ca: string | null, cb: string | null, i: number, j: number) => {
    const prev = ops[ops.length - 1];
    if (prev && prev.kind === kind) {
      if (ca !== null) prev.a.push(ca);
      if (cb !== null) prev.b.push(cb);
      return;
    }
    ops.push({ kind, a: ca !== null ? [ca] : [], b: cb !== null ? [cb] : [], aStart: i, bStart: j });
  };
  let i = 0;
  let j = 0;
  while (i < n || j < m) {
    if (i < n && j < m && a[i] === b[j]) {
      emit("=", a[i], b[j], i, j);
      i++;
      j++;
    } else if (j >= m || (i < n && lcs[(i + 1) * width + j] >= lcs[i * width + j + 1])) {
      emit("-", a[i], null, i, j);
      i++;
    } else {
      emit("+", null, b[j], i, j);
      j++;
    }
  }
  return ops;
}
