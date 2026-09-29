import assert from "node:assert/strict";
import { afterEach, test } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { nextTick } from "vue";
import { createI18n } from "vue-i18n";
import { TooltipProvider } from "@/components/ui/tooltip";
import enUS from "@/i18n/locales/en-US";
import type { KnowledgeFolderTree } from "@/api/knowledge-base/index";
import KbFolderTree from "./KbFolderTree.vue";
import DocumentBatchBar from "./DocumentBatchBar.vue";

// The source files the remaining text checks read. Resolved through a file
// path rather than `new URL(..., import.meta.url)`, which happy-dom turns into
// a non-file URL.
const here = (name: string) => fileURLToPath(new URL(name, `file://${__filename}`));
const cardView = readFileSync(here("./DocumentCardView.vue"), "utf8");
const listView = readFileSync(here("./DocumentListView.vue"), "utf8");

const mounted: VueWrapper[] = [];

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount());
  document.body.innerHTML = "";
});

const i18n = () => createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });

const tree: KnowledgeFolderTree = {
  root_document_count: 1,
  total_document_count: 4,
  folders: [
    {
      path: "docs",
      name: "docs",
      document_count: 1,
      total_count: 3,
      children: [{ path: "docs/spec", name: "spec", document_count: 2, total_count: 2 }],
    },
  ],
};

function mountTree(props: Record<string, unknown> = {}) {
  // The tree's buttons carry tooltips; the application mounts one provider at
  // its root, so the test supplies the same.
  const wrapper = mount(
    {
      components: { TooltipProvider, KbFolderTree },
      props: ["p"],
      template: '<TooltipProvider><KbFolderTree v-bind="p" /></TooltipProvider>',
    },
    { props: { p: { tree, selectedPath: "", ...props } }, global: { plugins: [i18n()] }, attachTo: document.body },
  );
  mounted.push(wrapper);
  return wrapper;
}

function rowByLabel(wrapper: VueWrapper, label: string) {
  return wrapper.findAll('[role="button"][tabindex="0"]').find((r) => r.text().includes(label));
}

// The root folder's own path IS the empty string, so a falsy "nothing is being
// renamed" sentinel matches it and leaves a stray rename input on the root row.
test("the rename sentinel cannot collide with the root folder path", async () => {
  const wrapper = mountTree({ canEdit: true });
  await flushPromises();
  assert.equal(wrapper.findAll("input").length, 0);
  const root = rowByLabel(wrapper, enUS.knowledgeBase.folderTree.rootRow);
  assert.ok(root);
  assert.equal(root.find("input").exists(), false);
});

test("only real folders expose a rename affordance", async () => {
  const readOnly = mountTree();
  await flushPromises();
  assert.equal(readOnly.findAll(`button[aria-label="${enUS.docs.tree.moreActions}"]`).length, 0);
  mounted.splice(0).forEach((w) => w.unmount());

  const wrapper = mountTree({ canEdit: true });
  await flushPromises();
  const root = rowByLabel(wrapper, enUS.knowledgeBase.folderTree.rootRow);
  assert.equal(root?.find(`button[aria-label="${enUS.docs.tree.moreActions}"]`).exists(), false);
  // Both folders are visible, because the top level opens on first load and
  // every folder row carries its own menu.
  assert.equal(wrapper.findAll(`button[aria-label="${enUS.docs.tree.moreActions}"]`).length, 2);
});

test("renaming a folder from its menu edits only the last segment", async () => {
  const wrapper = mountTree({ canEdit: true });
  await flushPromises();
  const spec = rowByLabel(wrapper, "spec");
  assert.ok(spec);
  await spec.find(`button[aria-label="${enUS.docs.tree.moreActions}"]`).trigger("click");
  await flushPromises();
  const renameItem = Array.from(document.body.querySelectorAll<HTMLElement>("[data-menu-item='rename']"))[0];
  assert.ok(renameItem, "the folder menu opens with a rename item");
  renameItem.click();
  await flushPromises();

  const input = wrapper.find("input");
  assert.ok(input.exists());
  assert.equal((input.element as HTMLInputElement).value, "spec");
  await input.setValue("design");
  await input.trigger("keydown", { key: "Enter" });
  const tree = wrapper.findComponent(KbFolderTree);
  assert.deepEqual(tree.emitted("rename")?.[0], [{ from: "docs/spec", to: "docs/design" }]);
  assert.equal(wrapper.find("input").exists(), false);
});

test("selecting a row and collapsing the panel are reported to the parent", async () => {
  const wrapper = mountTree();
  await flushPromises();
  await rowByLabel(wrapper, "docs")?.trigger("click");
  const tree = wrapper.findComponent(KbFolderTree);
  assert.deepEqual(tree.emitted("select")?.[0], ["docs"]);
  await wrapper.find(`button[aria-label="${enUS.knowledgeBase.folderTree.collapse}"]`).trigger("click");
  assert.deepEqual(tree.emitted("update:collapsed")?.[0], [true]);
});

// Picking a folder is a small, reversible action, so it stays a popup: in the row
// menu it is another level of the menu that is already open, and in the batch bar
// it hangs off the button. Neither should escalate to a modal dialog.
test("the folder picker is a popup rather than a modal", async () => {
  const wrapper = mount(DocumentBatchBar, {
    props: { count: 2, showMoveToFolder: true, folderOptions: [{ path: "docs", name: "docs", depth: 0 }] },
    global: { plugins: [i18n()] },
    attachTo: document.body,
  });
  mounted.push(wrapper);
  const move = wrapper.findAll("button").find((b) => b.text().includes(enUS.knowledgeBase.moveToFolder.action));
  assert.ok(move);
  await move.trigger("click");
  await flushPromises();
  // A popover's content carries role="dialog" too, so the check is on the
  // component that renders it: a popover, and no modal dialog anywhere.
  assert.equal(document.body.querySelector('[data-slot="dialog-content"]'), null);
  const folder = document.body.querySelector<HTMLElement>('[data-slot="popover-content"] [data-folder-path="docs"]');
  assert.ok(folder, "the picker lists the folders inside a popover");
  folder.click();
  await nextTick();
  assert.deepEqual(wrapper.emitted("moveToFolder")?.[0], ["docs"]);
  for (const source of [cardView, listView]) {
    assert.match(source, /folderPickerItemId === item\.id/);
    assert.doesNotMatch(source, /MoveToFolderDialog/);
  }
});

// The folder picker must render ahead of the normal action menu; otherwise
// moveMenuMode === 'normal' keeps the menu visible and clicks look dead.
test("the folder picker wins over the normal action menu", () => {
  for (const source of [cardView, listView]) {
    const pickerIdx = source.indexOf('v-if="folderPickerItemId === item.id"');
    const normalIdx = source.indexOf("v-else-if=\"moveMenuMode === 'normal'\"");
    assert.ok(pickerIdx >= 0);
    assert.ok(normalIdx >= 0);
    assert.ok(pickerIdx < normalIdx);
  }
});

test("folder navigation cancels stale card hover popovers", () => {
  assert.match(cardView, /if \(!cardElement\.isConnected/);
  assert.match(cardView, /watch\(\(\) => props\.items, dismissCardPopover\)/);
  assert.match(
    cardView,
    /const onOpenFolder = \(path: string\)[\s\S]*?dismissCardPopover\(\)[\s\S]*?emit\(["']open-folder["'], path\)/,
  );
  assert.match(cardView, /@click="onOpenFolder\(folder\.path\)"/);
});
