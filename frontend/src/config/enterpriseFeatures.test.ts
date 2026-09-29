import assert from "node:assert/strict";
import { test } from "vitest";

import {
  ENTERPRISE_FEATURES,
  ENTERPRISE_LICENSE_FEATURE_KEY,
  ENTERPRISE_LICENSE_I18N,
  findEnterpriseFeature,
} from "./enterpriseFeatures";
import enUS from "@/i18n/locales/en-US";
import koKR from "@/i18n/locales/ko-KR";
import ruRU from "@/i18n/locales/ru-RU";
import zhCN from "@/i18n/locales/zh-CN";

function lookup(messages: unknown, path: string): unknown {
  return path
    .split(".")
    .reduce<unknown>((node, part) => (node as Record<string, unknown> | undefined)?.[part], messages);
}

test("catalog keys are unique, namespaced, and exclude the license page", () => {
  const keys = ENTERPRISE_FEATURES.map((f) => f.key);
  assert.equal(new Set(keys).size, keys.length);
  for (const key of keys) assert.match(key, /^enterprise\.[a-z_]+$/);
  assert.equal(keys.includes(ENTERPRISE_LICENSE_FEATURE_KEY), false);
  assert.deepEqual(keys, ["enterprise.acl_retrieval", "enterprise.saml", "enterprise.audit_export"]);
});

test("nothing is claimed to be available before it exists", () => {
  // Flipping a stage is a deliberate act in the catalog; this pin makes it show up in review.
  for (const feature of ENTERPRISE_FEATURES) assert.equal(feature.stage, "planned", feature.key);
});

test("every feature has an icon and text in all four locales", () => {
  const keys = [
    ...ENTERPRISE_FEATURES.flatMap((f) => [f.titleKey, f.descriptionKey]),
    ENTERPRISE_LICENSE_I18N.titleKey,
    ENTERPRISE_LICENSE_I18N.descriptionKey,
    "enterprise.stage.planned",
    "enterprise.stage.available",
    "enterprise.stage.enabled",
    "enterprise.reason.generic",
  ];
  for (const feature of ENTERPRISE_FEATURES) {
    assert.ok(feature.icon, `${feature.key} has no icon`);
  }
  for (const [name, messages] of Object.entries({ enUS, zhCN, koKR, ruRU })) {
    for (const key of keys) {
      const value = lookup(messages, key);
      assert.equal(typeof value, "string", `${name} is missing ${key}`);
      assert.ok((value as string).length > 0, `${name} has an empty ${key}`);
    }
  }
});

test("findEnterpriseFeature resolves catalog keys only", () => {
  assert.equal(findEnterpriseFeature("enterprise.saml")?.key, "enterprise.saml");
  assert.equal(findEnterpriseFeature("nope"), undefined);
});
