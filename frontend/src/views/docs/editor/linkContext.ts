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

export const DOCS_TITLE_CACHE: InjectionKey<TitleCacheHandle> = Symbol('docs.titleCache')
export const DOCS_DIRECTORY: InjectionKey<DirectoryHandle> = Symbol('docs.directory')
