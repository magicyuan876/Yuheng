import assert from "node:assert/strict";
import { afterEach, test, vi } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import { defineComponent, h, nextTick } from "vue";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import { TooltipProvider } from "@/components/ui/tooltip";
import type { MentionItem } from "@/types/mention";
import MentionSelector from "./MentionSelector.vue";

// Mounted tests for the @-mention menu after its move off TDesign. They check
// the behaviour Input-field relies on: the two-level group navigation, the
// keyboard API it calls through the component ref, and that every row keeps
// the .mention-item hook scrollToItem() queries.

// The detail cards navigate and fetch; neither is exercised here.
vi.mock("vue-router", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("@/api/knowledge-base", () => ({
  getKnowledgeBaseById: vi.fn(() => new Promise(() => {})),
  getKnowledgeDetails: vi.fn(() => new Promise(() => {})),
}));

const items: MentionItem[] = [
  { id: "kb1", name: "Handbook", type: "kb", kbType: "document", count: 3 },
  { id: "kb2", name: "FAQ", type: "kb", kbType: "faq", count: 7 },
  { id: "f1", name: "report.pdf", type: "file" },
];

const mounted: VueWrapper[] = [];

afterEach(() => {
  while (mounted.length) mounted.pop()?.unmount();
  document.body.innerHTML = "";
});

type SelectorApi = { moveActive(delta: number): void; confirmActive(): void; leaveGroup(): boolean };

function mountSelector(props: Record<string, unknown> = {}) {
  const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });
  const events: Array<[string, unknown[]]> = [];
  let api: SelectorApi | null = null;
  // Reka's tooltips need a provider, which the app mounts once at its root.
  const Host = defineComponent({
    setup() {
      return () =>
        h(TooltipProvider, null, () =>
          h(MentionSelector, {
            visible: true,
            style: {},
            items,
            activeIndex: 0,
            ...props,
            ref: (el: unknown) => {
              api = el as SelectorApi | null;
            },
            onSelect: (...args: unknown[]) => events.push(["select", args]),
            "onUpdate:activeIndex": (...args: unknown[]) => events.push(["update:activeIndex", args]),
          }),
        );
    },
  });
  const wrapper = mount(Host, { global: { plugins: [i18n] }, attachTo: document.body });
  mounted.push(wrapper);
  return { wrapper, events, api: () => api as unknown as SelectorApi };
}

test("the first level lists one entry per non-empty group, with its count", () => {
  const { wrapper } = mountSelector();
  const entries = wrapper.findAll("button");
  assert.equal(entries.length, 2);
  assert.match(entries[0].text(), /Knowledge Base.*2/);
  assert.match(entries[1].text(), /File.*1/);
  assert.equal(wrapper.findAll(".mention-item").length, 0);
});

test("confirming a group enters it and moves the active index to its first item", async () => {
  const { wrapper, events, api } = mountSelector();
  api().moveActive(1);
  api().confirmActive();
  await nextTick();
  // The file group starts after the two knowledge bases.
  assert.deepEqual(events.at(-1), ["update:activeIndex", [2]]);
  const rows = wrapper.findAll(".mention-item");
  assert.equal(rows.length, 1);
  assert.match(rows[0].text(), /report\.pdf/);
  // Esc (leaveGroup) goes back to the group list rather than closing.
  assert.equal(api().leaveGroup(), true);
  await nextTick();
  assert.equal(wrapper.findAll(".mention-item").length, 0);
});

test("with a query the items are listed flat and a click selects one", async () => {
  const { wrapper, events } = mountSelector({ query: "a" });
  const rows = wrapper.findAll(".mention-item");
  assert.equal(rows.length, 3);
  await rows[1].trigger("click");
  assert.deepEqual(events.at(-1), ["select", [items[1]]]);
});

test("the active row is the one tinted", () => {
  const { wrapper } = mountSelector({ query: "a", activeIndex: 2 });
  const rows = wrapper.findAll(".mention-item");
  assert.deepEqual(
    rows.map((row) => row.classes().includes("bg-secondary")),
    [false, false, true],
  );
});
