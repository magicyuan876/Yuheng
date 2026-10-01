import assert from "node:assert/strict";
import { test } from "vitest";

import { protectStoredImageSrcInHTML, isValidURL } from "./security.ts";

test("protectStoredImageSrcInHTML uses a placeholder src for resource references", () => {
  const html = '<p><img alt="preview" src="resource://AbCdEfGhIjKlMnOpQrStUv"></p>';
  const sanitized = protectStoredImageSrcInHTML(html);
  const renderedSrc = sanitized.match(/<img[^>]*\ssrc="([^"]+)"/)?.[1];

  assert.match(renderedSrc || "", /^data:image\/gif;base64,/);
  assert.match(sanitized, /data-protected-src="resource:\/\/AbCdEfGhIjKlMnOpQrStUv"/);
});

// A storage locator is never stored content: the server refuses it on every
// file proxy, so it is neither protected nor treated as a usable image URL.
test("storage locators are not protected images", () => {
  for (const locator of [
    "local://10000/exports/a.jpg",
    "s3://bucket/yuheng/10000/exports/a.jpg",
    "storage://c0d93536-702c-4977-aa5e-fe670073c3cb/local://10000/exports/a.png",
  ]) {
    const html = `<p><img alt="preview" src="${locator}"></p>`;
    assert.equal(protectStoredImageSrcInHTML(html), html, locator);
    assert.equal(isValidURL(locator), false, locator);
  }
  assert.equal(isValidURL("resource://AbCdEfGhIjKlMnOpQrStUv"), true);
});
