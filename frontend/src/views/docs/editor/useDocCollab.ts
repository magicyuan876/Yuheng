// Wires one page's Y.Doc to whichever editing transport this deployment has,
// and exposes the reactive state DocEditor.vue needs.
//
// Two transports, one interface. A deployment with a collaboration service
// gets a HocuspocusProvider over a WebSocket, with live cursors and merged
// concurrent edits. A deployment without one (the Lite edition) gets the REST
// provider, where one person at a time holds a lease on the page. Which one is
// in use is decided solely by the docs_collab_url deployment capability, so
// both editions ship the same bundle and the editor above this file does not
// know the difference.
//
// All the actual decisions (is it editable right now, what banner to show, how
// to shape a collaborator) live in session.ts and restProvider.ts and are
// exercised by their own tests; this file is wiring, and is exercised by using
// the real editor, same as useDocsEvents.ts's SSE connection.
import { HocuspocusProvider } from "@hocuspocus/provider";
import { computed, onBeforeUnmount, ref, shallowRef, watch, type Ref } from "vue";
import { IndexeddbPersistence } from "y-indexeddb";
import * as Y from "yjs";

import { acquirePageLease, getPageLease, getPageYDoc, releasePageLease, savePageYDoc } from "@/api/docs";

import { RestProvider, type RestTransport, type SaveState } from "./restProvider";
import {
  awarenessUser,
  dedupeOnlineUsers,
  deriveBanner,
  isEditable,
  type AwarenessUser,
  type Banner,
  type CollabAccess,
  type ConnectionStatus,
  type UserLike,
} from "./session";

export interface DocCollabHandle {
  ydoc: Ref<Y.Doc>;
  /** The WebSocket provider, or null in exclusive-edit mode. Only the live
   * cursor extension needs it, and only the collaborative mode has one. */
  provider: Ref<HocuspocusProvider | null>;
  status: Ref<ConnectionStatus>;
  collabAccess: Ref<CollabAccess>;
  editable: Ref<boolean>;
  banner: Ref<Banner>;
  onlineUsers: Ref<AwarenessUser[]>;
  /** True when this page is edited one person at a time (no collaboration
   * service). The editor uses it to show a save indicator, which a merged
   * collaborative session does not need. */
  exclusive: Ref<boolean>;
  /** What the exclusive-edit save loop is doing; always 'idle' otherwise. */
  saveState: Ref<SaveState>;
  /** True once the very first sync (server state, or the local
   * y-indexeddb cache while still connecting) has been applied, so the
   * editor can wait to mount until there is something real to show. */
  ready: Ref<boolean>;
}

export interface DocCollabOptions {
  /** Reactive: switching pages tears down the old session and opens a new one. */
  pageId: Ref<string | undefined>;
  tenantId: Ref<string | number | undefined>;
  /** The page's own ACL answer (REST `can_edit`); independent of, and
   * checked before, whatever the editing transport confirms. */
  canEditPage: Ref<boolean>;
  /** The docs_collab_url deployment capability; empty means no collaboration
   * service, which selects the exclusive-edit transport. */
  collabUrl: Ref<string>;
  currentUser: Ref<UserLike | null>;
  /** Reads the current bearer token; called fresh on every (re)connect so a
   * token refresh during a long session is picked up automatically. */
  getToken: () => string | null;
}

/** Bumped into the DB name so a schema-breaking change does not try to
 * resurrect updates written under an incompatible document model. */
const INDEXEDDB_SCHEMA_VERSION = 1;

/** How often to re-derive the online-users list as a safety net alongside
 * the awareness `change` listener (covers any missed event ordering). */
const AWARENESS_POLL_MS = 5000;

/** Identifies this tab to the lease endpoints. Two tabs of one person must
 * never both believe they hold a page, so it is per composable instance
 * rather than per user or per browser. */
function newSessionId(): string {
  const uuid = globalThis.crypto?.randomUUID?.();
  if (uuid) return uuid;
  return `s-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

export function useDocCollab(opts: DocCollabOptions): DocCollabHandle {
  const ydoc = shallowRef(new Y.Doc());
  const provider = shallowRef<HocuspocusProvider | null>(null);
  const status = ref<ConnectionStatus>("connecting");
  const collabAccess = ref<CollabAccess>("unknown");
  const onlineUsers = ref<AwarenessUser[]>([]);
  const ready = ref(false);
  const exclusive = ref(false);
  const saveState = ref<SaveState>("idle");
  const leaseHolder = ref("");
  const superseded = ref(false);
  const unavailable = ref(false);

  let indexeddb: IndexeddbPersistence | null = null;
  let awarenessPollTimer: ReturnType<typeof setInterval> | null = null;
  let rest: RestProvider | null = null;

  const editable = computed(() =>
    isEditable({
      canEditPage: opts.canEditPage.value,
      connectionStatus: status.value,
      collabAccess: collabAccess.value,
    }),
  );
  const banner = computed<Banner>(() =>
    deriveBanner({
      canEditPage: opts.canEditPage.value,
      connectionStatus: status.value,
      collabAccess: collabAccess.value,
      leaseHolder: leaseHolder.value,
      superseded: superseded.value,
      unavailable: unavailable.value,
    }),
  );

  function teardown() {
    if (awarenessPollTimer) {
      clearInterval(awarenessPollTimer);
      awarenessPollTimer = null;
    }
    provider.value?.destroy();
    provider.value = null;
    if (rest) {
      // Flushes the last edit and hands the page back. The view is going away
      // either way, so a failure here must not block it.
      void rest.stop();
      rest = null;
    }
    indexeddb?.destroy().catch(() => undefined);
    indexeddb = null;
    ydoc.value.destroy();
    status.value = "connecting";
    collabAccess.value = "unknown";
    onlineUsers.value = [];
    ready.value = false;
    exclusive.value = false;
    saveState.value = "idle";
    leaseHolder.value = "";
    superseded.value = false;
    unavailable.value = false;
  }

  /** The lease endpoints, as the REST provider consumes them. */
  const transport: RestTransport = {
    loadYDoc: (pageId) => getPageYDoc(pageId),
    getLease: (pageId) => getPageLease(pageId),
    acquireLease: (pageId, sessionId) => acquirePageLease(pageId, sessionId),
    releaseLease: (pageId, sessionId) => releasePageLease(pageId, sessionId),
    saveYDoc: (pageId, b) => savePageYDoc(pageId, b),
  };

  function connectExclusive(pageId: string, doc: Y.Doc) {
    exclusive.value = true;
    const p = new RestProvider({
      pageId,
      ydoc: doc,
      canEdit: opts.canEditPage.value,
      sessionId: newSessionId(),
      transport,
      onChange: (state) => {
        // A free page reports no holder at all, and this client's own lease
        // is not a holder either, so only somebody else's name ever reaches
        // the banner.
        leaseHolder.value = state.otherHolder?.name ?? "";
        superseded.value = state.save === "superseded";
        unavailable.value = state.unsupported;
        saveState.value = state.save;
        status.value = state.status;
        collabAccess.value = state.access;
        // Anything but 'connecting' is a final answer for this attempt, so
        // the editor stops showing the loading skeleton either way.
        if (state.status !== "connecting") ready.value = true;
      },
    });
    rest = p;
    void p.start();
  }

  function connectCollaborative(pageId: string, tenantId: string, doc: Y.Doc) {
    const url = `${opts.collabUrl.value}?tenant=${encodeURIComponent(tenantId)}`;
    const p = new HocuspocusProvider({
      url,
      name: pageId,
      document: doc,
      token: () => opts.getToken() ?? "",
      onStatus: ({ status: s }) => {
        status.value = s;
      },
      onAuthenticated: ({ scope }) => {
        collabAccess.value = scope;
      },
      onAuthenticationFailed: () => {
        collabAccess.value = "readonly";
      },
      onSynced: () => {
        ready.value = true;
      },
      onStateless: ({ payload }) => {
        try {
          const msg = JSON.parse(payload) as { type?: string; access?: string };
          if (msg.type === "yuheng.access" && msg.access === "readonly") {
            collabAccess.value = "readonly";
          }
        } catch {
          // Not one of ours; ignore.
        }
      },
    });

    const user = opts.currentUser.value;
    if (user) p.setAwarenessField("user", awarenessUser(user));

    const refreshOnlineUsers = () => {
      const states = [...p.awareness!.getStates().entries()].map(([clientId, state]) => ({
        clientId,
        user: state.user as Partial<AwarenessUser> | undefined,
      }));
      onlineUsers.value = dedupeOnlineUsers(states);
    };
    p.awareness?.on("change", refreshOnlineUsers);
    // The 'change' listener can miss peers already online at the moment it
    // is attached; a light poll catches that without reasoning about event
    // ordering across the reconnect.
    awarenessPollTimer = setInterval(refreshOnlineUsers, AWARENESS_POLL_MS);
    refreshOnlineUsers();

    provider.value = p;
  }

  function connect(pageId: string, tenantId: string) {
    teardown();
    const doc = new Y.Doc();
    ydoc.value = doc;

    if (opts.collabUrl.value) {
      // The offline cache belongs to the collaborative transport only. There,
      // the stored Yjs state and the cached one are the same document, so
      // merging them is what lets an edit made offline survive. In
      // exclusive-edit mode the server rebuilds the document from JSON
      // whenever a replace happens, and merging a cache of the previous
      // document into the new one would duplicate its content rather than
      // recover anything.
      indexeddb = new IndexeddbPersistence(`yuheng-docs-v${INDEXEDDB_SCHEMA_VERSION}-${pageId}`, doc);
      indexeddb.whenSynced
        .then(() => {
          ready.value = true;
        })
        .catch(() => {
          ready.value = true; // the local cache is a nice-to-have, not a blocker
        });
      connectCollaborative(pageId, tenantId, doc);
    } else {
      connectExclusive(pageId, doc);
    }
  }

  // collabUrl is watched too: GET /system/capabilities resolves after mount,
  // so the first evaluation usually sees an empty URL and must reconnect once
  // the real address arrives. An empty URL is itself a mode rather than a
  // missing value, so the reconnect also switches transports correctly.
  watch(
    [opts.pageId, opts.tenantId, opts.collabUrl],
    ([pageId, tenantId]) => {
      if (!pageId || tenantId === undefined || tenantId === "") {
        teardown();
        return;
      }
      connect(pageId, String(tenantId));
    },
    { immediate: true },
  );

  onBeforeUnmount(teardown);

  return {
    ydoc,
    provider,
    status,
    collabAccess,
    editable,
    banner,
    onlineUsers,
    exclusive,
    saveState,
    ready,
  };
}
