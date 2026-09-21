// Turns an attachment id into an object URL the browser's own media elements
// can load. The attachment endpoint requires a bearer token, which a bare
// <img>/<video>/<audio>/<iframe> cannot send, so the bytes are fetched with
// the token and handed to the element as a blob URL instead. This mirrors the
// diagram-source loading in useDocMedia.ts and the knowledge-base previews in
// doc-content.vue.

import { onBeforeUnmount, ref, watch, type Ref } from "vue";

import { authHeaders } from "./useDocMedia";

/**
 * Resolves an attachment id to a blob URL. While the fetch is in flight, and
 * when the id is null or the fetch fails, the ref stays null so the node view
 * shows its empty state. The URL is revoked when the id changes or the view
 * unmounts.
 */
export function useAttachmentUrl(id: Ref<string | null>): Ref<string | null> {
  const url = ref<string | null>(null);
  let objectUrl: string | null = null;
  let seq = 0;

  const revoke = () => {
    if (objectUrl) {
      URL.revokeObjectURL(objectUrl);
      objectUrl = null;
    }
    url.value = null;
  };

  watch(
    id,
    async (value) => {
      const mySeq = ++seq;
      revoke();
      if (!value) return;
      try {
        const res = await fetch(`/api/v1/docs/attachments/${encodeURIComponent(value)}`, {
          headers: authHeaders(),
        });
        if (!res.ok) throw new Error(`attachment ${value} could not be read`);
        const blob = await res.blob();
        if (mySeq !== seq) return;
        objectUrl = URL.createObjectURL(blob);
        url.value = objectUrl;
      } catch {
        // A reader sees the node's empty state; a 404 here means the attachment
        // was deleted, and a network error is retried by re-rendering the page.
      }
    },
    { immediate: true },
  );

  onBeforeUnmount(() => {
    seq += 1;
    revoke();
  });

  return url;
}
