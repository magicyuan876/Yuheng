import assert from "node:assert/strict";
import { test } from "vitest";

import {
  calloutIcon,
  calloutKind,
  columnWidths,
  mermaidId,
  renderMath,
  statusColor,
  CALLOUT_KINDS,
  STATUS_COLORS,
} from "./figures";

test("a formula renders to HTML with no DOM", () => {
  const out = renderMath("a^2 + b^2 = c^2", false);
  assert.equal(out.error, "");
  assert.match(out.html, /class="katex"/);
  assert.ok(out.html.includes("<math"), "the accessible MathML form is part of the output");
});

test("display mode is what separates a block formula from an inline one", () => {
  const inline = renderMath("x", false);
  const block = renderMath("x", true);
  assert.notEqual(inline.html, block.html);
  assert.match(block.html, /katex-display/);
  assert.doesNotMatch(inline.html, /katex-display/);
});

// Acceptance (T2.1): a broken figure must not take the rest of the page with
// it. A formula someone is halfway through typing is the common case.
test("an unparseable formula reports an error instead of throwing", () => {
  const out = renderMath("\\frac{", false);
  assert.equal(out.html, "");
  assert.ok(out.error.length > 0, "the author is told what is wrong");
  // And nothing escaped as an exception.
  assert.doesNotThrow(() => renderMath("\\begin{matrix}", true));
  assert.doesNotThrow(() => renderMath("}{", false));
});

test("an empty formula is blank rather than an error", () => {
  for (const source of ["", "   ", "\n"]) {
    const out = renderMath(source, false);
    assert.equal(out.html, "");
    assert.equal(out.error, "");
  }
});

test("a formula cannot reach outside itself", () => {
  // trust:false is what disarms the commands that would load or link to
  // something. KaTeX does not reject them; it renders their text as inert
  // glyphs, so what matters is that no link or image element comes out.
  const out = renderMath(String.raw`\href{javascript:alert(1)}{click}`, false);
  assert.doesNotMatch(out.html, /<a/, "no anchor element");
  assert.doesNotMatch(out.html, /href=/, "and nothing that could be navigated to");

  const img = renderMath(String.raw`\includegraphics{https://evil.test/x.png}`, false);
  assert.doesNotMatch(img.html, /<img/);
});

test("callout kinds and status colours stay inside the schema vocabulary", () => {
  for (const kind of CALLOUT_KINDS) assert.equal(calloutKind(kind), kind);
  assert.equal(calloutKind("nonsense"), "info");
  assert.equal(calloutKind(undefined), "info");
  assert.equal(calloutKind(7), "info");

  for (const color of STATUS_COLORS) assert.equal(statusColor(color), color);
  assert.equal(statusColor("chartreuse"), "gray");
  assert.equal(statusColor(null), "gray");
});

test("a callout falls back to its kind’s icon and honours a chosen one", () => {
  assert.equal(calloutIcon("warning", null), "⚠️");
  assert.equal(calloutIcon("danger", "  "), "⛔");
  assert.equal(calloutIcon("info", "🔧"), "🔧");
  assert.equal(calloutIcon("nonsense", null), "ℹ️", "an unknown kind still draws something");
});

test("columns with no declared width share the row evenly", () => {
  assert.deepEqual(columnWidths([null, null]), [50, 50]);
  assert.deepEqual(columnWidths([null, null, null, null]), [25, 25, 25, 25]);
  const thirds = columnWidths([null, null, null]);
  assert.equal(thirds.length, 3);
  assert.equal(sum(thirds), 100);
});

test("a sized column keeps its width and the rest share what is left", () => {
  assert.deepEqual(columnWidths([70, null]), [70, 30]);
  const mixed = columnWidths([50, null, null]);
  assert.equal(mixed[0], 50);
  assert.equal(sum(mixed), 100);
});

test("widths that do not add up are scaled rather than rejected", () => {
  // Dragging a divider leaves the row briefly inconsistent; it must still
  // render as a full row.
  assert.equal(sum(columnWidths([30, 30])), 100);
  assert.equal(sum(columnWidths([80, 80])), 100);
});

test("no column can be squeezed out of existence", () => {
  const widths = columnWidths([99, 99, null]);
  assert.equal(widths.length, 3);
  for (const w of widths) assert.ok(w >= 5, `every column keeps at least the schema minimum: ${w}`);
});

test("an empty row has no widths at all", () => {
  assert.deepEqual(columnWidths([]), []);
});

test("every diagram gets its own render id, stable across re-renders", () => {
  assert.equal(mermaidId("abc123", 1), "docs-mermaid-abc123");
  assert.equal(mermaidId("abc123", 1), mermaidId("abc123", 2), "the same block renders under the same id");
  assert.notEqual(mermaidId("abc", 1), mermaidId("def", 1));
  // A block with no id yet still needs one that cannot collide.
  assert.equal(mermaidId(null, 7), "docs-mermaid-7");
  // Anything an id could smuggle into the document's id space is stripped.
  assert.equal(mermaidId('a b"><script>', 1), "docs-mermaid-abscript");
});

function sum(values: number[]): number {
  return Math.round(values.reduce((a, b) => a + b, 0));
}
