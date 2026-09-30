import assert from "node:assert/strict";
import { test } from "vitest";

import { shouldRejectKnowledgeFileType } from "./fileTypeVerification.ts";

test("shouldRejectKnowledgeFileType accepts HTML in the fallback whitelist", () => {
  assert.equal(shouldRejectKnowledgeFileType("page.html"), false);
  assert.equal(shouldRejectKnowledgeFileType("legacy.HTM"), false);
  assert.equal(shouldRejectKnowledgeFileType("payload.exe"), true);
});

test("shouldRejectKnowledgeFileType preserves dynamic whitelist behavior", () => {
  assert.equal(shouldRejectKnowledgeFileType("custom.xyz", ["xyz"]), false);
  assert.equal(shouldRejectKnowledgeFileType("page.html", ["pdf"]), true);
  assert.equal(shouldRejectKnowledgeFileType("page.html", []), false);
});

test("the fallback whitelist accepts what the backend import paths accept", () => {
  for (const name of ["mind.xmind", "photo.webp", "anim.gif", "notes.markdown", "data.json"]) {
    assert.equal(shouldRejectKnowledgeFileType(name), false, name);
  }
  // Parsed by some engines but not carried end to end, so the backend refuses them too.
  assert.equal(shouldRejectKnowledgeFileType("scan.tiff"), true);
  assert.equal(shouldRejectKnowledgeFileType("scan.bmp"), true);
});
