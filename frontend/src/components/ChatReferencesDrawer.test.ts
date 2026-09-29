import assert from "node:assert/strict";
import { afterEach, test, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { createI18n } from "vue-i18n";
import { defineComponent, h } from "vue";
import enUS from "@/i18n/locales/en-US";
import { provideChatReferencesDrawer, type ChatReferencesDrawerContext } from "@/composables/useChatReferencesDrawer";
import ChatReferencesDrawer from "./ChatReferencesDrawer.vue";

// The citations side panel of the chat view. These tests mount it inside a
// host that provides the drawer context, and replace the source-text checks
// that used to match its Less block.

vi.mock("vue-router", () => ({
  useRouter: () => ({
    resolve: ({ path, query }: { path: string; query: Record<string, string> }) => ({
      href: `${path}?${new URLSearchParams(query).toString()}`,
    }),
  }),
}));

// The panel slides in through a named <Transition>. The stub renders the panel
// at once and keeps the after-enter hook, so a test decides when the entry
// animation "finishes".
let finishEntry: (() => void) | undefined;
const TransitionStub = defineComponent({
  inheritAttrs: false,
  setup(_, { slots, attrs }) {
    if (typeof attrs.onAfterEnter === "function") finishEntry = attrs.onAfterEnter as () => void;
    return () => slots.default?.();
  },
});

const i18n = createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });

let wrapper: VueWrapper | undefined;
let drawer: ChatReferencesDrawerContext | undefined;

function mountDrawer() {
  const Host = defineComponent({
    setup() {
      drawer = provideChatReferencesDrawer();
      return () => h(ChatReferencesDrawer);
    },
  });
  wrapper = mount(Host, {
    attachTo: document.body,
    global: { plugins: [i18n], stubs: { transition: TransitionStub } },
  });
  return wrapper;
}

const documentReference = {
  id: "chunk-1",
  knowledge_id: "k1",
  knowledge_title: "Handbook",
  knowledge_base_id: "kb1",
  content: "The handbook explains the onboarding process.",
};

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  drawer = undefined;
  finishEntry = undefined;
  vi.restoreAllMocks();
});

test("the panel is a fixed column on the right edge of the viewport", async () => {
  const w = mountDrawer();
  drawer!.open({ references: [documentReference] });
  await flushPromises();

  const panel = w.find("aside");
  assert.ok(panel.exists());
  const classes = panel.classes();
  for (const name of ["fixed", "top-0", "right-0", "bottom-0"]) {
    assert.ok(classes.includes(name), `panel is missing ${name}`);
  }
  assert.equal(panel.attributes("role"), "complementary");
});

test("a document reference links to its knowledge base in a new tab", async () => {
  const w = mountDrawer();
  drawer!.open({ references: [documentReference] });
  await flushPromises();

  const link = w.find('a[href^="/platform/knowledge-bases/kb1"]');
  assert.ok(link.exists());
  assert.equal(link.attributes("href"), "/platform/knowledge-bases/kb1?knowledge_id=k1");
  assert.equal(link.attributes("target"), "_blank");
  assert.equal(link.attributes("rel"), "noopener noreferrer");
});

test("the close button closes the drawer", async () => {
  const w = mountDrawer();
  drawer!.open({ references: [documentReference] });
  await flushPromises();

  await w.get(`button[aria-label="${enUS.common.close}"]`).trigger("click");
  assert.equal(drawer!.visible.value, false);
  assert.equal(w.find("aside").exists(), false);
});

test("citation highlighting waits for the entry animation and only scrolls the panel body", async () => {
  const scrollTo = vi.fn();
  const scrollIntoView = vi.fn();
  vi.spyOn(HTMLElement.prototype, "scrollTo").mockImplementation(scrollTo);
  HTMLElement.prototype.scrollIntoView = scrollIntoView;
  // The highlighted card sits below the visible part of the list.
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
    const isCard = this.tagName === "ARTICLE";
    return {
      top: isCard ? 900 : 0,
      bottom: isCard ? 1000 : 500,
      left: 0,
      right: 400,
      width: 400,
      height: isCard ? 100 : 500,
      x: 0,
      y: isCard ? 900 : 0,
      toJSON: () => ({}),
    } as DOMRect;
  });

  mountDrawer();
  drawer!.open({ references: [documentReference], highlight: { chunkId: "chunk-1" } });
  await flushPromises();
  assert.equal(scrollTo.mock.calls.length, 0, "no scrolling before the panel has entered");

  assert.ok(finishEntry, "the panel transition carries an after-enter hook");
  finishEntry!();
  await flushPromises();

  assert.equal(scrollTo.mock.calls.length, 1);
  assert.deepEqual(scrollTo.mock.calls[0][0], { top: 508, behavior: "smooth" });
  assert.equal(scrollIntoView.mock.calls.length, 0);
});
