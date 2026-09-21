// The decisions an upload needs on the client, kept free of Vue, ProseMirror
// and the network so they are exercised directly by their own tests.
//
// None of this is a security boundary. The server sniffs every upload's type,
// sanitises what needs it and enforces the size and quota limits; what is here
// only saves a round trip and tells the person what is happening.

/** An attachment as the editor's nodes and views need it. */
export interface UploadedAttachment {
  id: string;
  file_name: string;
  mime: string;
  size_bytes: number;
  kind: "file" | "image" | "video" | "audio" | "diagram";
  width?: number;
  height?: number;
  url: string;
  variants?: number[];
}

/** The address to read an attachment from, optionally at a smaller width. */
export function attachmentSrc(id: string, width?: number): string {
  const base = `/api/v1/docs/attachments/${encodeURIComponent(id)}`;
  return width ? `${base}?w=${width}` : base;
}

/**
 * A srcset for the widths the server says it can render, so a browser on a
 * dense display picks the right one instead of always downloading the
 * original. Returns an empty string when there is nothing smaller to offer.
 */
export function attachmentSrcSet(id: string, variants?: number[]): string {
  if (!variants?.length) return "";
  return variants.map((w) => `${attachmentSrc(id, w)} ${w}w`).join(", ");
}

/** Human-readable size, for the file card and the error messages. */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "";
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / 1024;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value >= 10 || Number.isInteger(value) ? Math.round(value) : value.toFixed(1)} ${units[unit]}`;
}

/** True when a dropped or pasted file should become an image node. */
export function isImageFile(file: { type?: string; name?: string }): boolean {
  const type = (file.type ?? "").toLowerCase();
  if (type.startsWith("image/")) return true;
  // Some sources hand over a file with no type at all; the extension is the
  // only hint, and guessing wrong only costs the wrong node type — the server
  // decides what the bytes really are.
  return /\.(png|jpe?g|gif|webp|bmp|svg|avif|ico)$/i.test(file.name ?? "");
}

/**
 * The files worth taking from a paste or a drop.
 *
 * A copied image from another web page arrives as both a file and a fragment
 * of HTML, and pasting text that merely mentions a file must not upload
 * anything, so only real entries with content are kept.
 */
export function uploadableFiles(files: readonly File[]): File[] {
  return files.filter((f) => f && f.size > 0);
}

/** One in-flight upload, as the placeholder shows it. */
export interface UploadTask {
  /** Identifies the placeholder in the document until the real id arrives. */
  key: string;
  name: string;
  size: number;
  isImage: boolean;
  /** 0-100; -1 while the total is unknown. */
  progress: number;
  /** Set when the upload failed; the placeholder then offers a retry. */
  error?: string;
}

let uploadSeq = 0;

/** A key unique within this tab, used to find the placeholder again. */
export function newUploadKey(): string {
  uploadSeq += 1;
  return `upload-${Date.now().toString(36)}-${uploadSeq}`;
}

/**
 * Tracks the uploads a document has in flight.
 *
 * It is a plain object rather than a store because a placeholder has to be
 * findable from a ProseMirror plugin, which has no access to Vue's reactivity,
 * and because the whole thing can then be driven from a test.
 */
export class UploadQueue {
  private readonly tasks = new Map<string, UploadTask>();

  constructor(private readonly onChange: (tasks: UploadTask[]) => void = () => {}) {}

  start(file: File): UploadTask {
    const task: UploadTask = {
      key: newUploadKey(),
      name: file.name || "file",
      size: file.size,
      isImage: isImageFile(file),
      progress: 0,
    };
    this.tasks.set(task.key, task);
    this.emit();
    return task;
  }

  progress(key: string, percent: number): void {
    const task = this.tasks.get(key);
    if (!task) return;
    task.progress = Math.max(0, Math.min(100, percent));
    this.emit();
  }

  fail(key: string, message: string): void {
    const task = this.tasks.get(key);
    if (!task) return;
    task.error = message;
    this.emit();
  }

  finish(key: string): void {
    if (this.tasks.delete(key)) this.emit();
  }

  get(key: string): UploadTask | undefined {
    return this.tasks.get(key);
  }

  list(): UploadTask[] {
    return [...this.tasks.values()];
  }

  /** True while anything is still uploading, so a view can warn before it closes. */
  get busy(): boolean {
    return [...this.tasks.values()].some((t) => !t.error);
  }

  private emit(): void {
    this.onChange(this.list());
  }
}

/**
 * The ProseMirror node an uploaded file becomes.
 *
 * The kind the server derived from the bytes decides, not the name the file
 * was uploaded under: a video called .txt is still played, and a script called
 * .mp4 is still a download.
 */
export function nodeForAttachment(a: UploadedAttachment): {
  type: "image" | "video" | "audio" | "pdfEmbed" | "attachment";
  attrs: Record<string, unknown>;
} {
  if (a.kind === "image") {
    return {
      type: "image",
      attrs: {
        attachmentId: a.id,
        src: null,
        alt: a.file_name,
        title: null,
        width: a.width ?? null,
        height: a.height ?? null,
        align: "center",
      },
    };
  }
  if (a.kind === "video") {
    return {
      type: "video",
      attrs: { attachmentId: a.id, align: "center", width: null, height: null },
    };
  }
  if (a.kind === "audio") {
    return { type: "audio", attrs: { attachmentId: a.id } };
  }
  if (a.mime === "application/pdf") {
    return {
      type: "pdfEmbed",
      attrs: { attachmentId: a.id, name: a.file_name, width: null, height: null },
    };
  }
  // Everything else is a file card: a name, a size and a download link, which
  // is all a browser can usefully do with it.
  return {
    type: "attachment",
    attrs: {
      attachmentId: a.id,
      name: a.file_name,
      mime: a.mime,
      size: a.size_bytes,
    },
  };
}
