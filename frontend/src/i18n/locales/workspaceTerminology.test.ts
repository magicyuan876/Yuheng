import assert from "node:assert/strict";
import { test } from "vitest";

import enUS from "./en-US.ts";
import koKR from "./ko-KR.ts";
import ruRU from "./ru-RU.ts";
import zhCN from "./zh-CN.ts";

type LocaleValue = string | Record<string, unknown> | unknown[];

function collectStrings(value: LocaleValue, path = ""): Array<{ path: string; value: string }> {
  if (typeof value === "string") return [{ path, value }];
  if (Array.isArray(value)) {
    return value.flatMap((item, index) => collectStrings(item as LocaleValue, `${path}[${index}]`));
  }
  if (value && typeof value === "object") {
    return Object.entries(value).flatMap(([key, item]) =>
      collectStrings(item as LocaleValue, path ? `${path}.${key}` : key),
    );
  }
  return [];
}

function withoutTechnicalTenantTokens(value: string): string {
  return value
    .replace(/\{tenant(?:Id)?\}/g, "")
    .replace(/\btenant_id\b/gi, "")
    .replace(/\btenantless\b/gi, "")
    .replace(/\bX-Tenant-ID\b/g, "");
}

const localeChecks = [
  { name: "zh-CN", locale: zhCN, forbidden: /租户/ },
  { name: "en-US", locale: enUS, forbidden: /\btenants?\b/i },
  { name: "ko-KR", locale: koKR, forbidden: /테넌트/ },
  { name: "ru-RU", locale: ruRU, forbidden: /(?:тенант|арендатор)/i },
];

test("user-facing locale values use workspace terminology", () => {
  for (const check of localeChecks) {
    const legacyValues = collectStrings(check.locale)
      .filter(({ value }) => check.forbidden.test(withoutTechnicalTenantTokens(value)))
      .map(({ path, value }) => `${check.name}:${path}=${value}`);

    assert.deepEqual(legacyValues, [], `${check.name} still contains user-facing tenant terminology`);
  }
});

// Chinese has two words that English keeps apart as "Workspace" and "Space": the tenant layer
// is 「工作区」, and only the docs module's content containers are 「空间」. The two drifted
// together once (the tenant layer was called 空间 in most of the UI while the docs module used
// the same word for its own containers), so outside the namespaces where 空间 is the right word
// a bare 空间 is treated as the tenant layer misnamed. Compounds that are a different word
// altogether — 存储空间 (storage), 命名空间 (a GitLab namespace) — or that name a docs space
// explicitly (文档空间) are not a bare 空间.
const zhSpaceNamespaces = [
  // The docs module itself: its containers are spaces.
  "docs.",
  // Third-party data sources: Feishu wiki spaces, cloud drives and the like are their names.
  "datasource.",
];
const zhNotBareSpace = /(?:文档|存储|命名)空间/g;

test("zh-CN calls the tenant layer 工作区 and keeps 空间 for docs spaces", () => {
  const misnamed = collectStrings(zhCN)
    .filter(({ path }) => !zhSpaceNamespaces.some((prefix) => path.startsWith(prefix)))
    .filter(({ value }) => value.replace(zhNotBareSpace, "").includes("空间"))
    .map(({ path, value }) => `${path}=${value}`);

  assert.deepEqual(misnamed, [], "outside the docs module the tenant layer is 工作区, a docs space is 文档空间");
});
