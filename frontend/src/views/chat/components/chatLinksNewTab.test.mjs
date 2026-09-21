// @vitest-environment node
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "vitest";

const referenceDrawer = readFileSync(new URL("../../../components/ChatReferencesDrawer.vue", import.meta.url), "utf8");
const chatView = readFileSync(new URL("../index.vue", import.meta.url), "utf8");

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

test("references drawer smoothly shifts the chat area while opening", () => {
  assert.match(referenceDrawer, /\.chat-references-panel \{[\s\S]*?position: fixed;/);
  assert.match(chatView, /["']has-references-panel["']: referencesDrawerVisible/);
  assert.match(chatView, /transition: padding-right 0\.3s cubic-bezier\(0\.22, 0\.61, 0\.36, 1\)/);
  assert.match(chatView, /&\.has-references-panel \{[\s\S]*?padding-right:;?\s*420px/);
});
