import assert from "node:assert/strict";
import { describe, test } from "vitest";
import { mount } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import EnterpriseFeatureCard from "./EnterpriseFeatureCard.vue";
import { ENTERPRISE_FEATURES, type EnterpriseFeature, type EnterpriseFeatureStage } from "@/config/enterpriseFeatures";
import type { ExtensionState } from "@/config/deploymentCapabilities";

const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
const INFO = "https://example.com/enterprise";
const RouterLinkStub = { props: ["to"], template: '<a :href="to"><slot /></a>' };

function render(props: {
  stage: EnterpriseFeatureStage;
  state: ExtensionState;
  reason?: string;
  isAdmin?: boolean;
  licenseRoute?: string;
  infoUrl?: string;
}) {
  const feature: EnterpriseFeature = { ...ENTERPRISE_FEATURES[0], stage: props.stage };
  return mount(EnterpriseFeatureCard, {
    props: {
      feature,
      state: props.state,
      reason: props.reason,
      isAdmin: props.isAdmin,
      licenseRoute: props.licenseRoute,
      infoUrl: props.infoUrl,
    },
    global: { plugins: [i18n], stubs: { RouterLink: RouterLinkStub } },
  });
}

const kinds = (w: ReturnType<typeof render>) => w.findAll("[data-cta]").map((el) => el.attributes("data-cta"));

describe("planned features are described, never sold", () => {
  for (const state of ["unavailable", "locked", "enabled"] as const) {
    test(`state ${state}`, () => {
      const w = render({ stage: "planned", state, infoUrl: INFO, isAdmin: true, licenseRoute: "/license" });
      assert.ok(!w.text().includes("Learn about the Enterprise edition"));
      assert.equal(w.find("[data-cta='locked']").exists(), false);
      assert.equal(w.find("[data-testid='license-link']").exists(), false);
      assert.deepEqual(kinds(w), state === "enabled" ? [] : ["learn-more"]);
      assert.ok(w.find("[data-testid='stage']").text() === (state === "enabled" ? "Enabled" : "Planned"));
    });
  }

  test("without an info URL there is no call to action at all", () => {
    assert.deepEqual(kinds(render({ stage: "planned", state: "unavailable" })), []);
  });
});

describe("available features", () => {
  test("unavailable: a link to the edition page that opens safely in a new tab", () => {
    const w = render({ stage: "available", state: "unavailable", infoUrl: INFO });
    const a = w.get("a[data-cta='learn-edition']");
    assert.equal(a.text(), "Learn about the Enterprise edition");
    assert.equal(a.attributes("href"), INFO);
    assert.equal(a.attributes("target"), "_blank");
    assert.equal(a.attributes("rel"), "noopener noreferrer");
    assert.equal(w.get("[data-testid='stage']").text(), "Available");
  });

  test("unavailable without an info URL shows no link", () => {
    assert.deepEqual(kinds(render({ stage: "available", state: "unavailable" })), []);
  });

  test("locked: known reason, and admins get the license button when a route exists", () => {
    const w = render({
      stage: "available",
      state: "locked",
      reason: "license_required",
      isAdmin: true,
      licenseRoute: "/license",
      infoUrl: INFO,
    });
    assert.equal(w.get("[data-testid='reason']").text(), "This feature requires a valid Enterprise license.");
    assert.equal(w.get("[data-testid='license-link']").attributes("href"), "/license");
    assert.equal(w.find("[data-testid='ask-admin']").exists(), false);
    assert.equal(w.find("[data-cta='learn-edition']").exists(), false);
  });

  test("locked: admins see only the text while there is no license route", () => {
    const w = render({ stage: "available", state: "locked", reason: "license_expired_for_build", isAdmin: true });
    assert.match(w.get("[data-testid='reason']").text(), /expired/);
    assert.equal(w.find("[data-testid='license-link']").exists(), false);
    assert.equal(w.find("[data-testid='ask-admin']").exists(), false);
  });

  test("locked: non-admins are told to ask their administrator, and get no button", () => {
    const w = render({
      stage: "available",
      state: "locked",
      reason: "license_required",
      isAdmin: false,
      licenseRoute: "/license",
    });
    assert.equal(w.get("[data-testid='ask-admin']").text(), "Ask your administrator");
    assert.equal(w.find("[data-testid='license-link']").exists(), false);
  });

  test("locked: unknown or missing reasons fall back to the generic text", () => {
    for (const reason of ["something_new", undefined]) {
      const w = render({ stage: "available", state: "locked", reason });
      assert.equal(w.get("[data-testid='reason']").text(), "This feature is not available in this deployment.");
    }
  });

  test("enabled: no call to action", () => {
    assert.deepEqual(kinds(render({ stage: "available", state: "enabled", infoUrl: INFO })), []);
  });
});

test("the card is a labelled article with a heading", () => {
  const w = render({ stage: "planned", state: "unavailable" });
  const article = w.get("article");
  const heading = w.get("h3");
  assert.equal(article.attributes("aria-labelledby"), heading.attributes("id"));
  assert.equal(heading.text(), "Permission-aware retrieval");
});
