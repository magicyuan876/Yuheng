import assert from "node:assert/strict";
import { afterEach, test } from "vitest";

import { getEnterpriseInfoUrl, isEnterprisePromotionHidden } from "./enterprisePromotion";

afterEach(() => {
  delete window.__RUNTIME_CONFIG__;
});

test("with no runtime config there is no link and promotion is on", () => {
  assert.equal(getEnterpriseInfoUrl(), "");
  assert.equal(isEnterprisePromotionHidden(), false);
});

test("an http(s) info URL is passed through and surrounding space is ignored", () => {
  window.__RUNTIME_CONFIG__ = { ENTERPRISE_INFO_URL: "  https://example.com/enterprise  " };
  assert.equal(getEnterpriseInfoUrl(), "https://example.com/enterprise");
  window.__RUNTIME_CONFIG__ = { ENTERPRISE_INFO_URL: "http://intranet.local/x" };
  assert.equal(getEnterpriseInfoUrl(), "http://intranet.local/x");
});

test("anything that is not an absolute http(s) URL yields no link", () => {
  for (const value of ["", "   ", "example.com", "/relative", "javascript:alert(1)", "data:text/html,x", "ftp://a.b"]) {
    window.__RUNTIME_CONFIG__ = { ENTERPRISE_INFO_URL: value };
    assert.equal(getEnterpriseInfoUrl(), "", value);
  }
});

test("only an explicit true hides the promotion", () => {
  window.__RUNTIME_CONFIG__ = { HIDE_ENTERPRISE_PROMOTION: true };
  assert.equal(isEnterprisePromotionHidden(), true);
  window.__RUNTIME_CONFIG__ = { HIDE_ENTERPRISE_PROMOTION: false };
  assert.equal(isEnterprisePromotionHidden(), false);
  window.__RUNTIME_CONFIG__ = {};
  assert.equal(isEnterprisePromotionHidden(), false);
});
