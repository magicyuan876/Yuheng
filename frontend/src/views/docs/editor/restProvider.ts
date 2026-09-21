// The editing transport for deployments with no collaboration service (the
// Lite edition, and any deployment that leaves YUHENG_COLLAB_URL empty).
//
// It is deliberately shaped like the Hocuspocus provider it replaces: it owns
// the same Y.Doc the editor is bound to, reports the same three connection
// states, and says whether this client may write. Everything above it —
// the editor, the extensions, the banner logic — is identical in both modes,
// which is what keeps one frontend bundle serving both editions.
//
// What differs is underneath. Nothing merges concurrent edits here, so one
// client at a time holds a short lease on the page and posts the whole Yjs
// state over REST; the others poll, follow along read-only, and pick the page
// up when the lease lapses.
//
// There is no Vue, DOM or network client in this file: the transport is
// injected, so the whole state machine is exercised directly by its tests.
import { prosemirrorJSONToYXmlFragment, yDocToProsemirrorJSON } from "@tiptap/y-tiptap";
import { getSchema } from "@tiptap/core";
import type * as Y from "yjs";
import { encodeStateAsUpdate, applyUpdate } from "yjs";

import { officialExtensions } from "./extensions";

/** The Yjs fragment Tiptap's Collaboration extension binds to by default,
 * and the one the collaboration service and the Go renderer agree on. */
export const FRAGMENT = "default";

export type RestStatus = "connecting" | "connected" | "disconnected";
export type RestAccess = "read-write" | "readonly" | "unknown";

/** What the save loop is doing, for the "saving…/saved" indicator. */
export type SaveState = "idle" | "saving" | "saved" | "failed" | "superseded";

/** The holder of a page's lease, as the banner needs it. */
export interface LeaseHolder {
  userId: string;
  name: string;
}

export interface RestProviderState {
  status: RestStatus;
  access: RestAccess;
  /** Who is editing; null when the page is free or we hold it ourselves. */
  otherHolder: LeaseHolder | null;
  save: SaveState;
  /** The stored version this client's document is based on. */
  version: number;
  /** True when the server says this deployment does not do exclusive editing
   * at all, which means the client was told the wrong mode. */
  unsupported: boolean;
}

/** The server calls this provider makes, injected so tests need no network. */
export interface RestTransport {
  loadYDoc(pageId: string): Promise<{ ydoc?: string; content?: unknown; ydoc_version: number }>;
  getLease(pageId: string): Promise<LeaseResponse>;
  acquireLease(pageId: string, sessionId: string): Promise<LeaseResponse>;
  releaseLease(pageId: string, sessionId: string): Promise<void>;
  saveYDoc(
    pageId: string,
    body: {
      session_id: string;
      base_version: number;
      ydoc: string;
      content: unknown;
    },
  ): Promise<{ ydoc_version: number; lease: LeaseResponse }>;
}

export interface LeaseResponse {
  held: boolean;
  held_by_me: boolean;
  holder?: { user_id: string; username?: string; email?: string };
  ydoc_version: number;
}

export interface RestProviderOptions {
  pageId: string;
  ydoc: Y.Doc;
  /** The page's own permission answer; a reader never asks for the lease. */
  canEdit: boolean;
  /** Distinguishes this tab from every other one, including other tabs of
   * the same person. Two tabs must not both believe they hold the page. */
  sessionId: string;
  transport: RestTransport;
  onChange: (state: RestProviderState) => void;
  /** Milliseconds between saves while somebody is typing. */
  saveIntervalMs?: number;
  /** Milliseconds between lease renewals and remote-change polls. */
  pollIntervalMs?: number;
  /** Save immediately once this many document updates are unsaved, so a
   * paste or a find-and-replace is not held back by the timer. */
  maxPendingUpdates?: number;
}

/** At most one save every two seconds while typing (技术方案 §1). */
const DEFAULT_SAVE_INTERVAL_MS = 2000;
/** Renew and poll well inside the server's five-minute lease. */
const DEFAULT_POLL_INTERVAL_MS = 10_000;
const DEFAULT_MAX_PENDING_UPDATES = 200;

/** The origin tag on transactions this provider applies, so its own writes
 * are not mistaken for the user typing. */
const REMOTE_ORIGIN = "yuheng-rest-provider";

export function toBase64(bytes: Uint8Array): string {
  let binary = "";
  for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]);
  return btoa(binary);
}

export function fromBase64(value: string): Uint8Array {
  const binary = atob(value);
  const out = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) out[i] = binary.charCodeAt(i);
  return out;
}

/** True when the server refused because this deployment merges edits instead
 * of leasing them. Recognised by status alone would be too broad — a stale
 * base version is a 409 too — so the message is checked as well. */
export function isUnsupported(err: unknown): boolean {
  const status =
    (err as { status?: number; response?: { status?: number } })?.status ??
    (err as { response?: { status?: number } })?.response?.status;
  if (status !== 409) return false;
  const message = String((err as { message?: string })?.message ?? "");
  return message.includes("collaboration service");
}

/** True when the server refused a save because the page moved on. */
export function isVersionConflict(err: unknown): boolean {
  const status =
    (err as { status?: number; response?: { status?: number } })?.status ??
    (err as { response?: { status?: number } })?.response?.status;
  return status === 409;
}

function holderOf(lease: LeaseResponse | null): LeaseHolder | null {
  if (!lease?.held || lease.held_by_me || !lease.holder) return null;
  const h = lease.holder;
  return { userId: h.user_id, name: h.username?.trim() || h.email?.trim() || h.user_id };
}

export class RestProvider {
  private readonly opts: Required<
    Pick<RestProviderOptions, "saveIntervalMs" | "pollIntervalMs" | "maxPendingUpdates">
  > &
    RestProviderOptions;

  private state: RestProviderState = {
    status: "connecting",
    access: "unknown",
    otherHolder: null,
    save: "idle",
    version: 0,
    unsupported: false,
  };

  private lease: LeaseResponse | null = null;
  private pendingUpdates = 0;
  private saveTimer: ReturnType<typeof setTimeout> | null = null;
  private pollTimer: ReturnType<typeof setInterval> | null = null;
  private applying = false;
  private stopped = false;
  private inFlight: Promise<void> = Promise.resolve();
  private onUpdate = (_update: Uint8Array, origin: unknown) => {
    if (this.applying || origin === REMOTE_ORIGIN) return;
    this.pendingUpdates++;
    this.scheduleSave();
  };

  constructor(options: RestProviderOptions) {
    this.opts = {
      ...options,
      saveIntervalMs: options.saveIntervalMs ?? DEFAULT_SAVE_INTERVAL_MS,
      pollIntervalMs: options.pollIntervalMs ?? DEFAULT_POLL_INTERVAL_MS,
      maxPendingUpdates: options.maxPendingUpdates ?? DEFAULT_MAX_PENDING_UPDATES,
    };
  }

  current(): RestProviderState {
    return { ...this.state };
  }

  /** Loads the document, takes the lease when allowed, and starts the poll.
   * Never rejects: a failure becomes the 'disconnected' state and a banner,
   * exactly as a dropped WebSocket does in the collaborative mode. */
  async start(): Promise<void> {
    try {
      await this.load();
      await this.claim();
      this.patch({ status: "connected" });
    } catch (err) {
      if (isUnsupported(err)) {
        this.patch({ status: "disconnected", access: "readonly", unsupported: true });
        return;
      }
      this.patch({ status: "disconnected", access: "readonly" });
      return;
    }
    this.opts.ydoc.on("update", this.onUpdate);
    this.pollTimer = setInterval(() => void this.poll(), this.opts.pollIntervalMs);
  }

  /** Writes any unsaved changes now. Safe to call when there is nothing to
   * save, and never rejects. */
  async flush(): Promise<void> {
    this.clearSaveTimer();
    if (this.pendingUpdates === 0 || !this.holdsLease()) return;
    await this.save();
  }

  /** Stops the provider: last save, timers cleared, lease handed back. */
  async stop(): Promise<void> {
    if (this.stopped) return;
    this.stopped = true;
    this.opts.ydoc.off("update", this.onUpdate);
    this.clearSaveTimer();
    if (this.pollTimer) {
      clearInterval(this.pollTimer);
      this.pollTimer = null;
    }
    await this.inFlight;
    if (this.pendingUpdates > 0 && this.holdsLease()) {
      await this.save();
    }
    if (this.holdsLease()) {
      try {
        await this.opts.transport.releaseLease(this.opts.pageId, this.opts.sessionId);
      } catch {
        // The lease expires on its own; failing to hand it back early is not
        // worth surfacing while the view is being torn down.
      }
    }
    this.lease = null;
    this.patch({ status: "disconnected", access: "unknown", otherHolder: null });
  }

  // ---- internals ----------------------------------------------------------

  private holdsLease(): boolean {
    return this.lease?.held === true && this.lease.held_by_me === true;
  }

  private patch(next: Partial<RestProviderState>): void {
    this.state = { ...this.state, ...next };
    this.opts.onChange(this.current());
  }

  private async load(): Promise<void> {
    const state = await this.opts.transport.loadYDoc(this.opts.pageId);
    this.applying = true;
    try {
      if (state.ydoc) {
        applyUpdate(this.opts.ydoc, fromBase64(state.ydoc), REMOTE_ORIGIN);
      } else if (state.content) {
        // A page that has never been edited has no Yjs state yet; build one
        // from the stored body so the first save produces a real document.
        prosemirrorJSONToYXmlFragment(
          getSchema(officialExtensions()),
          state.content,
          this.opts.ydoc.getXmlFragment(FRAGMENT),
        );
      }
    } finally {
      this.applying = false;
    }
    this.patch({ version: state.ydoc_version });
  }

  /** Takes the lease if this client may write, and otherwise just finds out
   * who holds it so the read-only banner can name them. */
  private async claim(): Promise<void> {
    const lease = this.opts.canEdit
      ? await this.opts.transport.acquireLease(this.opts.pageId, this.opts.sessionId)
      : await this.opts.transport.getLease(this.opts.pageId);
    this.applyLease(lease);
  }

  private applyLease(lease: LeaseResponse): void {
    this.lease = lease;
    this.patch({
      access: lease.held_by_me ? "read-write" : "readonly",
      otherHolder: holderOf(lease),
    });
  }

  private scheduleSave(): void {
    if (!this.holdsLease() || this.stopped) return;
    if (this.pendingUpdates >= this.opts.maxPendingUpdates) {
      this.clearSaveTimer();
      void this.save();
      return;
    }
    // A fixed window rather than a resetting debounce: continuous typing must
    // still reach the server every saveIntervalMs, not only when it stops.
    if (this.saveTimer !== null) return;
    this.saveTimer = setTimeout(() => {
      this.saveTimer = null;
      void this.save();
    }, this.opts.saveIntervalMs);
  }

  private clearSaveTimer(): void {
    if (this.saveTimer !== null) {
      clearTimeout(this.saveTimer);
      this.saveTimer = null;
    }
  }

  /** One save. Serialised through inFlight so two triggers cannot post the
   * same version twice and turn the second into a spurious conflict. */
  private save(): Promise<void> {
    const run = this.inFlight.then(async () => {
      if (this.pendingUpdates === 0 || !this.holdsLease()) return;
      const sending = this.pendingUpdates;
      this.patch({ save: "saving" });
      try {
        const res = await this.opts.transport.saveYDoc(this.opts.pageId, {
          session_id: this.opts.sessionId,
          base_version: this.state.version,
          ydoc: toBase64(encodeStateAsUpdate(this.opts.ydoc)),
          content: yDocToProsemirrorJSON(this.opts.ydoc, FRAGMENT),
        });
        this.pendingUpdates = Math.max(0, this.pendingUpdates - sending);
        this.applyLease(res.lease);
        this.patch({ version: res.ydoc_version, save: "saved" });
      } catch (err) {
        if (isUnsupported(err)) {
          this.patch({ save: "failed", access: "readonly", unsupported: true });
          return;
        }
        if (isVersionConflict(err)) {
          // Somebody took the page over while this client was typing. The
          // safe answer is to stop writing and say so: the local document is
          // no longer a continuation of what the server holds.
          this.lease = null;
          this.patch({ save: "superseded", access: "readonly" });
          return;
        }
        this.patch({ save: "failed" });
      }
    });
    this.inFlight = run;
    return run;
  }

  /** Renews the lease, picks the page up when it frees, and follows the
   * current holder's saves while this client is read-only.
   *
   * One call covers all three because acquiring is idempotent for the holder:
   * the same request renews our own lease, takes an expired one, and
   * otherwise reports who has it.
   */
  private async poll(): Promise<void> {
    // After a takeover the local document is no longer a continuation of the
    // server's, so neither writing nor merging into it would be honest. The
    // banner asks for a reload instead.
    if (this.stopped || this.state.save === "superseded") return;
    try {
      const heldBefore = this.holdsLease();
      const lease = this.opts.canEdit
        ? await this.opts.transport.acquireLease(this.opts.pageId, this.opts.sessionId)
        : await this.opts.transport.getLease(this.opts.pageId);
      this.applyLease(lease);
      // Catch up on saves made while somebody else held the page. A client
      // that held it throughout is the only writer and cannot be behind.
      const behind = lease.ydoc_version > this.state.version;
      if (behind && !(heldBefore && this.holdsLease())) {
        await this.load();
      }
      this.patch({ status: "connected" });
    } catch (err) {
      if (isUnsupported(err)) {
        this.patch({ status: "disconnected", access: "readonly", unsupported: true });
        return;
      }
      this.patch({ status: "disconnected" });
    }
  }
}
