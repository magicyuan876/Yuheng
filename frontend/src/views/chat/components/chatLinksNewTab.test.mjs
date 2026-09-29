// @vitest-environment node
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "vitest";

const referenceDrawer = readFileSync(new URL("../../../components/ChatReferencesDrawer.vue", import.meta.url), "utf8");

test("reference document links open in a new tab", () => {
  assert.match(
    referenceDrawer,
    /:href="getDocumentHref\(item\)"[\s\S]*?target="_blank"[\s\S]*?rel="noopener noreferrer"/,
  );
});

test("citation highlighting waits for drawer entry and only scrolls its own body", () => {
  assert.match(referenceDrawer, /@after-enter="handlePanelAfterEnter"/);
  assert.match(referenceDrawer, /if \(!panelEntered\.value\) return/);
  assert.match(referenceDrawer, /container\.scrollTo\(\{ top: Math\.max\(0, nextTop\), behavior: ["']smooth["'] \}\)/);
  assert.doesNotMatch(referenceDrawer, /el\.scrollIntoView\(/);
});

// The case "references drawer smoothly shifts the chat area while opening"
// used to live here and regex-matched the chat view's Less. It is replaced by
// chatLayout.test.ts, which checks the classes the chat view actually gets
// with the panel open and closed.
