import { test } from "vitest";
import assert from "node:assert/strict";

import { formatBytes, parseBytes, usageFraction, usageLevel } from "./formatBytes.ts";

test("bytes below a kilobyte are shown as bytes", () => {
  assert.equal(formatBytes(0), "0 B");
  assert.equal(formatBytes(1), "1 B");
  assert.equal(formatBytes(1023), "1023 B");
});

test("larger sizes step up through binary units", () => {
  assert.equal(formatBytes(1024), "1 KB");
  assert.equal(formatBytes(1024 * 1024), "1 MB");
  assert.equal(formatBytes(1024 ** 3), "1 GB");
  assert.equal(formatBytes(1024 ** 4), "1 TB");
});

// "5 MB" rather than "5.0 MB"; 9.4 GB is useful and 943.2 GB is noise.
test("one decimal below ten and none above", () => {
  assert.equal(formatBytes(Math.round(1.5 * 1024 ** 3)), "1.5 GB");
  assert.equal(formatBytes(5 * 1024 * 1024), "5 MB");
  assert.equal(formatBytes(Math.round(943.2 * 1024 ** 3)), "943 GB");
});

// An empty cell reads as missing data, which is a different claim.
test("nonsense renders as a dash rather than nothing", () => {
  assert.equal(formatBytes(Number.NaN), "—");
  assert.equal(formatBytes(-1), "—");
  assert.equal(formatBytes(Number.POSITIVE_INFINITY), "—");
});

// A bare number is bytes: it is the only reading that cannot silently
// multiply what somebody meant by a thousand.
test("a bare number is bytes", () => {
  assert.equal(parseBytes("500"), 500);
  assert.equal(parseBytes("0"), 0);
});

test("a suffix is honoured, in any case and spacing", () => {
  assert.equal(parseBytes("1KB"), 1024);
  assert.equal(parseBytes("1 kb"), 1024);
  assert.equal(parseBytes("  2 MB "), 2 * 1024 * 1024);
  assert.equal(parseBytes("1.5gb"), Math.round(1.5 * 1024 ** 3));
});

test("the binary spellings mean the same thing", () => {
  assert.equal(parseBytes("1KiB"), parseBytes("1KB"));
  assert.equal(parseBytes("2 TiB"), parseBytes("2TB"));
});

test("what is not a size is not guessed at", () => {
  for (const input of ["", "  ", "lots", "1 elephant", "-5", "1.2.3", "MB", "1 MB extra"]) {
    assert.equal(parseBytes(input), null, `input ${JSON.stringify(input)}`);
  }
});

test("a size too large to be exact is refused rather than rounded wrong", () => {
  assert.equal(parseBytes("9999999 PB"), null);
});

test("what is rendered can be parsed back", () => {
  for (const bytes of [0, 1023, 1024, 5 * 1024 * 1024, 1024 ** 3]) {
    const parsed = parseBytes(formatBytes(bytes).replace(" ", ""));
    assert.equal(parsed, bytes, `round trip of ${bytes}`);
  }
});

test("an unlimited space has no fraction to show", () => {
  assert.equal(usageFraction(500, 0), null);
  assert.equal(usageLevel(500, 0), "ok");
});

test("the fraction tracks usage", () => {
  assert.equal(usageFraction(0, 1000), 0);
  assert.equal(usageFraction(500, 1000), 0.5);
  assert.equal(usageFraction(1000, 1000), 1);
});

// A space over its quota (because the limit was lowered) shows a full bar
// rather than one that overflows its track.
test("the fraction is clamped at full", () => {
  assert.equal(usageFraction(5000, 1000), 1);
  assert.equal(usageLevel(5000, 1000), "full");
});

test("the level warns before it is too late", () => {
  assert.equal(usageLevel(500, 1000), "ok");
  assert.equal(usageLevel(899, 1000), "ok");
  assert.equal(usageLevel(900, 1000), "warning");
  assert.equal(usageLevel(1000, 1000), "full");
});
