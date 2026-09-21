// Turns a paste, a drop or a file picker into an attachment and a node.
//
// Everything that can be decided without a browser lives in attachments.ts and
// uploadPlaceholder.ts and is tested there; this file is the wiring: it talks
// to the API, drives the editor, and shows what is happening.
import { Extension } from "@tiptap/core";
// The core Editor rather than the Vue one: the Vue editor is a subclass, so
// typing against the base accepts both, and this module never touches
// anything Vue-specific.
import type { Editor } from "@tiptap/core";
import { ref, type Ref } from "vue";

import { uploadAttachment, type DocsAttachment } from "@/api/docs";

import { nodeForAttachment, uploadableFiles, UploadQueue, type UploadTask } from "./attachments";
import { addPlaceholder, placeholderPos, removePlaceholder, uploadPlaceholderPlugin } from "./uploadPlaceholder";

export interface DocUploadsOptions {
  spaceId: Ref<string>;
  pageId: Ref<string>;
  /** False while the page is read-only; dropping a file then does nothing. */
  canEdit: Ref<boolean>;
  /** Shown when an upload is refused, so the reason reaches the person. */
  onError: (message: string) => void;
  /** Localised label for the placeholder, e.g. "Uploading photo.png…". */
  placeholderLabel: (task: UploadTask) => string;
}

export interface DocUploadsHandle {
  /** In-flight uploads, for the status strip. */
  tasks: Ref<UploadTask[]>;
  /** The Tiptap extension carrying the placeholder plugin. */
  extension: Extension;
  /** Uploads files and inserts them at pos (default: the selection). */
  insert: (editor: Editor, files: readonly File[], pos?: number) => void;
  /** Wires paste and drop; pass the result as the editor's editorProps. */
  editorProps: Record<string, unknown>;
  /** Hands over the editor once it exists, so paste and drop can reach it. */
  bind: (editor: Editor | null) => void;
}

export function useDocUploads(opts: DocUploadsOptions): DocUploadsHandle {
  const tasks = ref<UploadTask[]>([]);
  const queue = new UploadQueue((list) => {
    tasks.value = list;
  });

  /** Builds the DOM shown while a file is on its way. */
  const renderPlaceholder = (key: string): HTMLElement => {
    const el = document.createElement("span");
    el.className = "docs-upload-placeholder";
    const task = queue.get(key);
    el.textContent = task ? opts.placeholderLabel(task) : "";
    return el;
  };

  const extension = Extension.create({
    name: "yuhengUploadPlaceholder",
    addProseMirrorPlugins: () => [uploadPlaceholderPlugin(renderPlaceholder)],
  });

  async function one(editor: Editor, file: File, at: number): Promise<void> {
    const task = queue.start(file);
    editor.view.dispatch(addPlaceholder(editor.state.tr, task.key, at));
    try {
      const uploaded: DocsAttachment = await uploadAttachment(opts.spaceId.value, file, {
        pageId: opts.pageId.value,
        onProgress: (percent) => queue.progress(task.key, percent),
      });
      insertNode(editor, task.key, uploaded);
      queue.finish(task.key);
    } catch (err) {
      // The placeholder goes either way: leaving it would suggest the file is
      // still coming. The message is what tells the person it is not.
      editor.view.dispatch(removePlaceholder(editor.state.tr, task.key));
      queue.finish(task.key);
      opts.onError(messageOf(err, file.name));
    }
  }

  /** Puts the finished node where its placeholder ended up. */
  function insertNode(editor: Editor, key: string, uploaded: DocsAttachment): void {
    const at = placeholderPos(editor.state, key);
    const { type, attrs } = nodeForAttachment(uploaded);
    let tr = removePlaceholder(editor.state.tr, key);
    if (at === null) {
      // The text the file was dropped into is gone; append rather than lose
      // the upload, which is stored and charged for either way.
      tr = tr.insert(tr.doc.content.size, editor.schema.nodes[type]!.create(attrs));
    } else {
      tr = tr.insert(at, editor.schema.nodes[type]!.create(attrs));
    }
    editor.view.dispatch(tr.scrollIntoView());
  }

  function insert(editor: Editor, files: readonly File[], pos?: number): void {
    if (!opts.canEdit.value || !opts.spaceId.value) return;
    const accepted = uploadableFiles(files);
    if (accepted.length === 0) return;
    const at = pos ?? editor.state.selection.from;
    for (const file of accepted) void one(editor, file, at);
  }

  // Paste and drop are handled through the editor the component hands over
  // once it exists, so there is exactly one insertion path for the toolbar
  // button, a paste and a drop.
  let bound: Editor | null = null;
  const bind = (editor: Editor | null) => {
    bound = editor;
  };

  const editorProps = {
    handlePaste: (_view: unknown, event: ClipboardEvent): boolean => {
      const files = [...(event.clipboardData?.files ?? [])];
      if (!bound || uploadableFiles(files).length === 0) return false;
      // A copied image arrives as a file and as a fragment of HTML; taking
      // the file means the picture is stored here rather than hot-linked from
      // wherever it came from, and stays readable when that page does not.
      event.preventDefault();
      insert(bound, files);
      return true;
    },
    handleDrop: (view: Editor["view"], event: DragEvent, _slice: unknown, moved: boolean): boolean => {
      if (moved) return false; // a node being dragged around inside the document
      const files = [...(event.dataTransfer?.files ?? [])];
      if (!bound || uploadableFiles(files).length === 0) return false;
      event.preventDefault();
      const at = view.posAtCoords({ left: event.clientX, top: event.clientY })?.pos;
      insert(bound, files, at);
      return true;
    },
  };

  return { tasks, extension, insert, editorProps, bind };
}

/** The server's message when there is one, so a quota refusal reads as one. */
function messageOf(err: unknown, fileName: string): string {
  const message = (err as { message?: string })?.message;
  return message ? `${fileName}: ${message}` : fileName;
}
