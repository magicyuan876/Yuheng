// What the media, embed and diagram nodes need from the server: the
// deployment's embed policy, the derived frame address for each embed, and a
// way to store an edited diagram as two attachments.
//
// It exists as one composable rather than three because all three are owned by
// the editor component and torn down with it, and because a node view reaches
// them through provide/inject rather than through props.
import { ref, shallowRef, type Ref } from 'vue'

import {
  getEmbedPolicy, resolveEmbed, uploadAttachment,
  type EmbedPolicyView,
} from '@/api/docs'

import type { DiagramHost, EmbedResolverHandle, ResolvedEmbed } from './linkContext'

export interface DocMediaOptions {
  spaceId: Ref<string>
  pageId: Ref<string>
}

export interface DocMediaHandle {
  /** What this deployment allows, for the insert menus to offer. */
  policy: Ref<EmbedPolicyView>
  embeds: EmbedResolverHandle
  diagrams: DiagramHost
  /** Loads the policy once; safe to call repeatedly. */
  load: () => Promise<void>
  dispose: () => void
}

export function useDocMedia(opts: DocMediaOptions): DocMediaHandle {
  const policy = shallowRef<EmbedPolicyView>({ providers: [], drawio_url: '' })
  const drawioURL = ref('')
  const revision = ref(0)

  // One answer per address. An embed node cannot resolve its own frame
  // address — the allow-list lives on the server — so each one asks once and
  // every node showing the same address shares the answer.
  const known = new Map<string, ResolvedEmbed>()
  const asking = new Set<string>()
  let disposed = false

  // A separator that cannot occur in either half, so two different pairs
  // cannot collide into one cached answer.
  function keyOf(provider: string, url: string): string {
    return provider + String.fromCharCode(0) + url
  }

  async function ask(provider: string, url: string): Promise<void> {
    const key = keyOf(provider, url)
    if (asking.has(key)) return
    asking.add(key)
    try {
      const answer = await resolveEmbed(url)
      if (disposed) return
      // A stored node claiming a provider the address does not belong to is
      // refused here as well as on save, so a document written by an older or
      // a hostile client still cannot get an unexpected page into a frame.
      known.set(key, answer.provider === provider
        ? { embedUrl: answer.embed_url, title: answer.title }
        : { embedUrl: '' })
    } catch {
      // Refused, or unreachable. Either way there is nothing to frame, and
      // the node shows why rather than an empty box.
      if (!disposed) known.set(key, { embedUrl: '' })
    } finally {
      asking.delete(key)
      if (!disposed) revision.value++
    }
  }

  const embeds: EmbedResolverHandle = {
    get: (provider, url) => {
      if (!url) return { embedUrl: '' }
      const key = keyOf(provider, url)
      const have = known.get(key)
      if (have) return have
      void ask(provider, url)
      return undefined
    },
    revision,
  }

  const diagrams: DiagramHost = {
    drawioURL,
    /**
     * Stores an edited diagram as two attachments: the editable source and the
     * rendering every reader sees.
     *
     * The preview is uploaded first. If the second upload fails, what is left
     * behind is an unreferenced file the maintenance sweep collects — whereas
     * storing the source first and failing on the preview would leave a
     * diagram that every reader sees as broken.
     */
    save: async (kind, source, preview, name) => {
      const previewFile = new File([preview], `${name}.svg`, { type: 'image/svg+xml' })
      const previewUpload = await uploadAttachment(opts.spaceId.value, previewFile, {
        pageId: opts.pageId.value,
      })
      const sourceFile = kind === 'drawio'
        ? new File([source], `${name}.drawio`, { type: 'application/xml' })
        : new File([source], `${name}.excalidraw`, { type: 'application/json' })
      const sourceUpload = await uploadAttachment(opts.spaceId.value, sourceFile, {
        pageId: opts.pageId.value,
      })
      return { attachmentId: sourceUpload.id, previewAttachmentId: previewUpload.id }
    },
    load: async (attachmentId) => {
      const res = await fetch(`/api/v1/docs/attachments/${encodeURIComponent(attachmentId)}`, {
        credentials: 'same-origin',
        headers: authHeaders(),
      })
      if (!res.ok) throw new Error(`attachment ${attachmentId} could not be read`)
      return res.text()
    },
  }

  async function load(): Promise<void> {
    try {
      const answer = await getEmbedPolicy()
      if (disposed) return
      policy.value = answer
      drawioURL.value = answer.drawio_url ?? ''
    } catch {
      // A deployment that will not say what it allows is treated as allowing
      // nothing, which matches what its save path will do anyway.
      if (disposed) return
      policy.value = { providers: [], drawio_url: '' }
      drawioURL.value = ''
    }
  }

  return {
    policy, embeds, diagrams, load,
    dispose: () => {
      disposed = true
      known.clear()
      asking.clear()
    },
  }
}

/** The bearer token, for the fetches that do not go through the client. */
export function authHeaders(): Record<string, string> {
  try {
    const token = localStorage.getItem('yuheng_token')
    return token ? { Authorization: `Bearer ${token}` } : {}
  } catch {
    return {}
  }
}
