import assert from "node:assert/strict";
import { afterEach, beforeEach, test, vi } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { defineComponent, h, nextTick } from "vue";
import { createI18n } from "vue-i18n";
import enUS from "@/i18n/locales/en-US";
import type { UploadConfirmResult } from "@/stores/uploadConfirm";
import UploadConfirmDialog from "./UploadConfirmDialog.vue";
import FolderPickerMenu from "./FolderPickerMenu.vue";

// The dialog cases below replace older ones that regex-matched the TDesign
// source of UploadConfirmDialog.vue and FolderPickerMenu.vue. They mount the
// dialog and check what the user sees and what the parent receives. The
// checks on the host, the knowledge base page and the platform shell stay as
// they were: they are about how those files wire the dialog in.

const listKnowledgeTags = vi.fn();
vi.mock("@/api/knowledge-base", () => ({
  listKnowledgeTags: (...args: unknown[]) => listKnowledgeTags(...args),
}));
vi.mock("@/stores/chatResources", () => ({
  useChatResourcesStore: () => ({ ensureModels: async () => {}, allModels: [] }),
}));
vi.mock("@/stores/editorResources", () => ({
  useEditorResourcesStore: () => ({ ensureSystemInfo: async () => {}, systemInfo: {} }),
}));
vi.mock("@/stores/ui", () => ({
  useUIStore: () => ({ openSettings: () => {} }),
}));

// Resolved through a file path: happy-dom turns `new URL(..., import.meta.url)`
// into a non-file URL that readFileSync refuses.
const read = (name: string) => readFileSync(fileURLToPath(new URL(name, `file://${__filename}`)), "utf8");
const host = read("../../../components/UploadConfirmHost.vue");
const knowledgeBase = read("../KnowledgeBase.vue");
const platform = read("../../platform/index.vue");

const u = enUS.uploadConfirm;

// Heavy children that talk to stores of their own; the dialog only passes them props.
const stub = (name: string) => defineComponent({ name, render: () => h("div", { "data-stub": name }) });
const stubs = {
  ModelSelector: stub("ModelSelector"),
  KBParserSettings: stub("KBParserSettings"),
  GraphSettings: stub("GraphSettings"),
  KbUploadSourceDropdown: stub("KbUploadSourceDropdown"),
};

const mounted: VueWrapper[] = [];

beforeEach(() => {
  listKnowledgeTags.mockReset();
  listKnowledgeTags.mockResolvedValue({
    data: {
      data: [
        { id: 1, name: "Alpha" },
        { id: 2, name: "Beta" },
        { id: 3, name: "Gamma" },
      ],
    },
  });
});

afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount());
  document.body.innerHTML = "";
});

const i18n = () => createI18n({ legacy: false, locale: "en-US", messages: { "en-US": enUS } });

async function openDialog(props: Record<string, unknown> = {}) {
  const wrapper = mount(UploadConfirmDialog, {
    props: {
      visible: false,
      kbInfo: { id: "kb1" },
      files: [new File(["hello"], "notes.txt")],
      ...props,
    },
    global: { plugins: [i18n()], stubs },
    attachTo: document.body,
  });
  mounted.push(wrapper);
  // The dialog seeds its state when it opens, as it does in the app.
  await wrapper.setProps({ visible: true });
  await flushPromises();
  return wrapper;
}

function modal(): HTMLElement {
  const el = document.body.querySelector<HTMLElement>(`[role="dialog"][aria-label="${u.title}"]`);
  assert.ok(el, "the upload dialog is open");
  return el;
}

function buttonIn(root: ParentNode, text: string): HTMLButtonElement | undefined {
  return Array.from(root.querySelectorAll("button")).find((b) => b.textContent?.trim() === text);
}

function confirmed(wrapper: VueWrapper): UploadConfirmResult {
  const payload = wrapper.emitted("confirm")?.[0]?.[0] as UploadConfirmResult | undefined;
  assert.ok(payload, "the dialog confirmed");
  return payload;
}

function popover(): HTMLElement {
  const el = document.body.querySelector<HTMLElement>('[data-slot="popover-content"]');
  assert.ok(el, "a popover is open");
  return el;
}

test("selects multiple document tags and returns them with the confirmation result", async () => {
  assert.match(host, /:tag-ids="uploadConfirmStore\.tagIds"/);

  const wrapper = await openDialog();
  assert.deepEqual(listKnowledgeTags.mock.calls[0], ["kb1", { page: 1, page_size: 1000 }]);

  const trigger = modal().querySelector<HTMLButtonElement>(`button[aria-label="${u.tagsPlaceholder}"]`);
  assert.ok(trigger);
  trigger.click();
  await flushPromises();

  // Picking one tag must not close the picker: several are picked in a row.
  for (const name of ["Alpha", "Gamma"]) {
    const option = Array.from(popover().querySelectorAll('[role="option"]')).find((o) => o.textContent?.includes(name));
    option?.querySelector<HTMLButtonElement>('[role="checkbox"]')?.click();
    await flushPromises();
  }
  assert.ok(trigger.textContent?.includes("Alpha, Gamma"));

  // The filter narrows the list.
  const filter = popover().querySelector<HTMLInputElement>("input");
  assert.ok(filter);
  filter.value = "bet";
  filter.dispatchEvent(new Event("input"));
  await flushPromises();
  assert.deepEqual(
    Array.from(popover().querySelectorAll('[role="option"]')).map((o) => o.textContent?.trim()),
    ["Beta"],
  );

  buttonIn(modal(), u.confirm)?.click();
  await nextTick();
  const result = confirmed(wrapper);
  assert.equal(result.mode, "file");
  assert.deepEqual(result.tagIds, ["1", "3"]);
  assert.equal(result.files?.length, 1);
});

test("the tag selection can be cleared in one step", async () => {
  const wrapper = await openDialog({ tagIds: ["2"] });
  const clear = modal().querySelector<HTMLButtonElement>(`button[aria-label="${enUS.common.clear}"]`);
  assert.ok(clear, "a preselected tag offers the clear button");
  clear.click();
  await nextTick();
  buttonIn(modal(), u.confirm)?.click();
  await nextTick();
  assert.deepEqual(confirmed(wrapper).tagIds, []);
});

// Browsing a folder pre-fills the upload destination, so the dialog must show it
// and the batch must use the folder the user confirmed there — never the sidebar
// selection as it stands when the uploads actually start.
test("takes the upload destination folder from the confirmation result", async () => {
  assert.match(host, /:target-folder="uploadConfirmStore\.targetFolder"/);
  assert.match(host, /:folder-options="uploadConfirmStore\.folderOptions"/);
  assert.match(knowledgeBase, /targetFolder: result\.targetFolder \|\| ROOT_FOLDER_PATH/);
  assert.match(knowledgeBase, /targetFolder: selectedFolderPath\.value/);
  assert.match(knowledgeBase, /folderOptions: folderOptions\.value/);

  const wrapper = await openDialog({
    targetFolder: "docs",
    folderOptions: [
      { path: "docs", name: "docs", depth: 0 },
      { path: "docs/specs", name: "specs", depth: 1 },
    ],
  });
  const crumb = modal().querySelector<HTMLButtonElement>(`button[aria-label="${u.destinationChange}"]`);
  assert.ok(crumb);
  assert.ok(crumb.textContent?.includes(`${enUS.knowledgeBase.folderTree.rootRow} / docs`));

  crumb.click();
  await flushPromises();
  popover().querySelector<HTMLElement>('[data-folder-path="docs/specs"]')?.click();
  await flushPromises();
  assert.ok(crumb.textContent?.includes(`${enUS.knowledgeBase.folderTree.rootRow} / docs / specs`));

  buttonIn(modal(), u.confirm)?.click();
  await nextTick();
  assert.equal(confirmed(wrapper).targetFolder, "docs/specs");
});

test("creates sub-folders from per-row actions without changing the selected destination", async () => {
  const wrapper = mount(FolderPickerMenu, {
    props: { options: [{ path: "docs", name: "docs", depth: 0 }], currentPath: "docs" },
    global: { plugins: [i18n()] },
    attachTo: document.body,
  });
  mounted.push(wrapper);
  const addUnder = enUS.knowledgeBase.moveToFolder.newFolderAddUnder.replace("{folder}", "docs");
  await wrapper.find(`button[aria-label="${addUnder}"]`).trigger("click");
  await flushPromises();

  const input = wrapper.find<HTMLInputElement>("input");
  assert.ok(input.exists(), "a name field opens under the folder");
  assert.equal(document.activeElement, input.element, "and takes the focus");
  await input.setValue("drafts");
  await input.trigger("keydown", { key: "Enter" });
  await flushPromises();

  assert.deepEqual(wrapper.emitted("create")?.[0], ["docs/drafts"]);
  assert.equal(wrapper.emitted("confirm"), undefined, "creating a folder does not pick it");
  assert.ok(wrapper.find('[data-folder-path="docs/drafts"]').exists(), "the new folder is listed at once");

  // Inside the dialog the created folder is offered without moving the destination.
  const dialog = await openDialog({ targetFolder: "docs", folderOptions: [{ path: "docs", name: "docs", depth: 0 }] });
  const crumb = modal().querySelector<HTMLButtonElement>(`button[aria-label="${u.destinationChange}"]`);
  crumb?.click();
  await flushPromises();
  dialog.findComponent(FolderPickerMenu).vm.$emit("create", "docs/fresh");
  await flushPromises();
  assert.ok(crumb?.textContent?.includes(`${enUS.knowledgeBase.folderTree.rootRow} / docs`));
  assert.ok(!crumb?.textContent?.includes("fresh"));
  assert.ok(popover().querySelector('[data-folder-path="docs/fresh"]'));
});

test("calls out directory uploads via relative paths in the file list", async () => {
  const file = new File(["x"], "README.md");
  Object.defineProperty(file, "webkitRelativePath", { value: "handbook/intro/README.md" });
  await openDialog({ files: [file] });
  assert.ok(modal().textContent?.includes("handbook/intro"));
});

test("routes global knowledge file drops through the upload confirmation flow", () => {
  assert.match(platform, /yuheng:knowledge-file-drop/);
  assert.match(knowledgeBase, /handleKnowledgeFileDrop/);
  assert.match(knowledgeBase, /handleUploadSourceFiles\(files\)/);
});

test("uses confirmed tags for file and URL imports instead of reading the list filter at upload time", () => {
  assert.match(knowledgeBase, /const tagIds = result\.tagIds \|\| \[\]/);
  assert.match(knowledgeBase, /executeUploadBatch\(files, \{[\s\S]*?\btagIds,[\s\S]*?\}\)/);
  assert.match(knowledgeBase, /executeUrlImport\(url, processConfig, tagIds\)/);
  assert.doesNotMatch(
    knowledgeBase,
    /const tagIdsToUpload = selectedTagIds\.value\.length > 0 \? \[\.\.\.selectedTagIds\.value\] : undefined/,
  );
});

test("uses section navigation with inline chunking controls and advanced options grouped", async () => {
  const wrapper = await openDialog();
  const nav = modal().querySelector("nav");
  assert.ok(nav);
  const navButton = (label: string) =>
    Array.from(nav.querySelectorAll("button")).find((b) => b.textContent?.includes(label));

  // Without a graph database there is no graph section at all.
  assert.equal(modal().querySelector('[data-stub="GraphSettings"]'), null);

  navButton(enUS.knowledgeEditor.chunking.title)?.click();
  await nextTick();
  const chunking = Array.from(modal().querySelectorAll("h2")).find(
    (el) => el.textContent?.trim() === enUS.knowledgeEditor.chunking.title,
  );
  assert.ok(chunking);

  // Chunk size and overlap sit inline; a value out of range is pulled back on change.
  const [size, overlap] = Array.from(modal().querySelectorAll<HTMLInputElement>('input[type="number"]'));
  assert.equal(size.value, "512");
  assert.equal(overlap.value, "80");
  size.value = "99999";
  size.dispatchEvent(new Event("input"));
  await nextTick();
  size.dispatchEvent(new Event("change"));
  await nextTick();
  assert.equal(size.value, "4000");

  // The rarer options wait behind the toggle.
  const toggle = buttonIn(modal(), u.moreOptions);
  assert.ok(toggle);
  assert.equal(toggle.getAttribute("aria-expanded"), "false");
  assert.equal(modal().textContent?.includes(enUS.knowledgeEditor.chunking.separatorsLabel), false);
  toggle.click();
  await nextTick();
  assert.ok(modal().textContent?.includes(enUS.knowledgeEditor.chunking.separatorsLabel));

  // Each nav entry carries a status line, with the full text as its title.
  assert.ok(nav.querySelector("span[title]"));
  assert.ok(modal().querySelector('[data-section="multimodal"]'));

  buttonIn(modal(), u.confirm)?.click();
  await nextTick();
  assert.equal(confirmed(wrapper).processConfig?.chunking_config?.chunk_size, 4000);
});
