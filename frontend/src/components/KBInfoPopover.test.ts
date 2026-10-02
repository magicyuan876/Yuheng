import assert from "node:assert/strict";
import { afterEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { defineComponent, h } from "vue";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import { TooltipProvider } from "@/components/ui/tooltip";
import KBInfoPopover from "./KBInfoPopover.vue";

// Mounted tests for the knowledge-base info popover after its move off
// TDesign: the trigger's warning state, and the card's sections opening on
// click with the role badge tinted by the viewer's access.

// The viewer is a Contributor who created knowledge base "kb1" and nothing
// else: on their own card the badge reads "Owner", on a colleague's card it
// reads their workspace role and the hint says they can only read it.
vi.mock("@/stores/auth", () => ({
  useAuthStore: () => ({ user: { id: "u1" }, currentTenantRole: "contributor", hasRole: () => false }),
}));

const mounted: VueWrapper[] = [];

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
  document.body.innerHTML = "";
});

function mountPopover(kbInfo: Record<string, unknown>) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  // Reka's tooltips need a provider, which the app mounts once at its root.
  const Host = defineComponent({
    setup: () => () => h(TooltipProvider, null, () => h(KBInfoPopover, { kbInfo })),
  });
  const wrapper = mount(Host, {
    global: { plugins: [i18n], stubs: { VectorStoreBadge: true } },
    // The card is teleported to <body>.
    attachTo: document.body,
  });
  mounted.push(wrapper);
  return wrapper;
}

test("the trigger turns red when the bound vector store is unavailable", () => {
  const ok = mountPopover({ id: "kb1", type: "document" });
  assert.ok(ok.get("button").classes().includes("text-placeholder"));

  const down = mountPopover({ id: "kb2", type: "document", vector_store_status: "unavailable" });
  assert.ok(down.get("button").classes().includes("text-destructive"));
});

test("clicking the trigger opens the card; the creator's role badge is the success tint", async () => {
  const wrapper = mountPopover({
    id: "kb1",
    type: "document",
    creator_id: "u1",
    knowledge_count: 12,
    vlm_config: { enabled: true },
  });
  await wrapper.get("button").trigger("click");
  await flushPromises();

  const card = document.body.querySelector('[data-slot="popover-content"]');
  assert.ok(card, "the popover content is rendered");
  assert.match(card.textContent ?? "", new RegExp(enUS.knowledgeBase.infoCard.title));
  assert.match(card.textContent ?? "", /12/);

  const badges = Array.from(card.querySelectorAll("span")).filter((el) =>
    (el.textContent ?? "").includes(enUS.knowledgeBase.accessInfo.roleOwner),
  );
  assert.ok(badges.some((el) => el.classList.contains("bg-success")));
  // The value cell wraps the badge and reads "VLM" too, so look for any match.
  const vlm = Array.from(card.querySelectorAll("span")).filter((el) => el.textContent?.trim() === "VLM");
  assert.ok(vlm.some((el) => el.classList.contains("text-primary")));
});

test("on a colleague's knowledge base the badge shows the viewer's workspace role, untinted", async () => {
  const wrapper = mountPopover({ id: "kb2", type: "document", creator_id: "u2" });
  await wrapper.get("button").trigger("click");
  await flushPromises();

  const card = document.body.querySelector('[data-slot="popover-content"]');
  assert.ok(card, "the popover content is rendered");
  const badges = Array.from(card.querySelectorAll("span")).filter(
    (el) => (el.textContent ?? "").trim() === enUS.tenantMember.role.contributor,
  );
  assert.equal(badges.length, 1);
  assert.ok(!badges[0].classList.contains("bg-success"));
  assert.ok(!badges[0].classList.contains("bg-primary"));
  assert.match(card.textContent ?? "", new RegExp(enUS.knowledgeBase.accessInfo.permissionViewer));
});
