// What a page link and a mention need from outside themselves.
//
// A node view is created by ProseMirror, not by a parent component, so it
// cannot be handed props. It gets these through provide/inject instead: one
// title lookup shared by every link on the page, and one directory shared by
// every mention, both owned by the editor component and torn down with it.
//
// `revision` is a counter the caches bump whenever an answer arrives. A node
// view reads it inside its computed properties, which is what makes a link
// that rendered as "loading" re-render as a title a moment later — the caches
// themselves are plain objects, deliberately, so they can be tested without
// Vue.
import type { InjectionKey, Ref } from 'vue'

import type { ResolvedBlockRef } from './blockRefCache'
import type { ResolvedPage } from './titleCache'

export interface TitleCacheHandle {
  get: (pageId: string) => ResolvedPage | undefined
  revision: Ref<number>
}

export interface DirectoryPerson {
  userId: string
  username?: string
  email?: string
  avatar?: string
}

export interface DirectoryHandle {
  get: (userId: string) => DirectoryPerson | undefined
  revision: Ref<number>
}

/** What an embed node was told about its address. */
export interface ResolvedEmbed {
  /** The address to frame, or empty when the deployment refuses it. */
  embedUrl: string
  title?: string
}

export interface EmbedResolverHandle {
  /** What to frame; undefined while the answer is still being fetched. */
  get: (provider: string, url: string) => ResolvedEmbed | undefined
  revision: Ref<number>
}

/** What a diagram node needs to open its editor. */
export interface DiagramHost {
  /** The self-hosted draw.io address, or empty when diagrams are read-only. */
  drawioURL: Ref<string>
  /** Stores an edited diagram and returns the two attachment ids. */
  save: (kind: 'drawio' | 'excalidraw', source: string, preview: string, sourceName: string)
    => Promise<{ attachmentId: string; previewAttachmentId: string }>
  /** Reads a stored diagram's source back. */
  load: (attachmentId: string) => Promise<string>
}

/** What a block reference needs to draw the block it points at. */
export interface BlockRefHandle {
  get: (ref: { sourcePageId: string; sourceBlockId: string }) => ResolvedBlockRef | undefined
  revision: Ref<number>
}

export const DOCS_TITLE_CACHE: InjectionKey<TitleCacheHandle> = Symbol('docs.titleCache')
export const DOCS_BLOCK_REFS: InjectionKey<BlockRefHandle> = Symbol('docs.blockRefs')
export const DOCS_EMBEDS: InjectionKey<EmbedResolverHandle> = Symbol('docs.embeds')
export const DOCS_DIAGRAMS: InjectionKey<DiagramHost> = Symbol('docs.diagrams')
export const DOCS_DIRECTORY: InjectionKey<DirectoryHandle> = Symbol('docs.directory')
