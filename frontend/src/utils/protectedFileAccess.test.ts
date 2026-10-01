import assert from "node:assert/strict";
import { test } from "vitest";

import {
  buildProtectedFileRequest,
  isProtectedFileProxyPath,
  isResourceRef,
  resolveProtectedFileAccess,
  type ProtectedFileAccessContext,
} from "./protectedFileAccess.ts";

const RESOURCE = "resource://AbCdEfGhIjKlMnOpQrStUv";

test("tenant access routes through the tenant-scoped proxy", () => {
  const request = buildProtectedFileRequest(RESOURCE, { mode: "tenant" });

  assert.equal(request?.url, `/files?file_path=${encodeURIComponent(RESOURCE)}`);
});

test("knowledge-base access routes through the KB-scoped proxy", () => {
  const request = buildProtectedFileRequest(RESOURCE, { mode: "knowledgeBase", kbId: "kb 1" });

  assert.equal(request?.url, `/api/v1/knowledge-bases/kb%201/files?file_path=${encodeURIComponent(RESOURCE)}`);
});

test("message access routes through the session-message-scoped proxy", () => {
  const request = buildProtectedFileRequest(RESOURCE, {
    mode: "message",
    sessionId: "session 1",
    messageId: "message/1",
  });

  assert.equal(
    request?.url,
    `/api/v1/sessions/session%201/messages/message%2F1/files?file_path=${encodeURIComponent(RESOURCE)}`,
  );
});

test("only resource references are proxied", () => {
  assert.equal(buildProtectedFileRequest("https://example.com/a.png", { mode: "tenant" }), null);
  assert.equal(isResourceRef("https://example.com/a.png"), false);
  // Storage locators never leave the server; the proxies refuse them.
  assert.equal(isResourceRef("storage://backend-1/local://42/a.png"), false);
  assert.equal(isResourceRef("local://42/a.png"), false);
  assert.equal(buildProtectedFileRequest("local://42/a.png", { mode: "tenant" }), null);
  assert.equal(isResourceRef(RESOURCE), true);
});

test("a component scope refines the tenant default, and an empty one falls back to it", () => {
  const override: ProtectedFileAccessContext = { mode: "knowledgeBase", kbId: "kb-1" };

  assert.deepEqual(resolveProtectedFileAccess(override), override);
  assert.deepEqual(resolveProtectedFileAccess(), { mode: "tenant" });
  assert.deepEqual(resolveProtectedFileAccess({ mode: "knowledgeBase", kbId: "  " }), { mode: "tenant" });
  assert.deepEqual(resolveProtectedFileAccess({ mode: "message", sessionId: "session-1", messageId: "  " }), {
    mode: "tenant",
  });
});

test("all file proxies are recognized as protected proxy paths", () => {
  assert.equal(isProtectedFileProxyPath("/files"), true);
  assert.equal(isProtectedFileProxyPath("/api/v1/knowledge-bases/kb-1/files"), true);
  assert.equal(isProtectedFileProxyPath("/api/v1/sessions/session-1/messages/message-1/files"), true);
  assert.equal(isProtectedFileProxyPath("/api/v1/knowledge-bases/kb-1/config"), false);
});
