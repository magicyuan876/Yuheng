import { del, get, patch, post, put } from '@/utils/request'

// Online documents API (backend: internal/router/routes_docs.go). Every
// function unwraps Yuheng's `{ success, data }` envelope and rejects with the
// server message when success is false, so callers only deal with data.

export type SpaceVisibility = 'private' | 'open' | 'public'
export type SpaceRole = 'none' | 'reader' | 'writer' | 'admin'
export type PrincipalType = 'user' | 'group'

export interface DocsSpace {
  id: string
  tenant_id: number
  slug: string
  name: string
  description: string
  icon?: string | null
  visibility: SpaceVisibility
  default_role: SpaceRole
  knowledge_base_id?: string | null
  storage_backend_id?: string | null
  settings?: Record<string, unknown>
  creator_id?: string | null
  created_at: string
  updated_at: string
  /** The caller's effective role in this space. */
  role: SpaceRole
  /** Direct memberships (users and groups). */
  member_count: number
  page_count: number
}

export interface SpaceMember {
  principal_type: PrincipalType
  principal_id: string
  role: SpaceRole
  name: string
  email?: string
  avatar?: string
  is_default_group?: boolean
  group_member_count?: number
  added_by?: string
  created_at: string
}

export interface TenantGroup {
  id: string
  tenant_id: number
  name: string
  description: string
  is_default: boolean
  source: string
  created_at: string
  updated_at: string
  member_count: number
}

export interface GroupMemberRow {
  user_id: string
  username?: string
  email?: string
  avatar?: string
}

export interface GroupMemberPage {
  members: GroupMemberRow[]
  total: number
  page: number
  page_size: number
}

export interface CreateSpaceRequest {
  name: string
  slug?: string
  description?: string
  icon?: string | null
  visibility?: SpaceVisibility
  default_role?: SpaceRole
  knowledge_base_id?: string | null
  storage_backend_id?: string | null
  settings?: Record<string, unknown>
}

export interface UpdateSpaceRequest {
  name?: string
  slug?: string
  description?: string
  icon?: string
  visibility?: SpaceVisibility
  default_role?: SpaceRole
  settings?: Record<string, unknown>
}

export interface SpaceMemberInput {
  principal_type: PrincipalType
  principal_id: string
  role: SpaceRole
}

export interface BindKnowledgeBaseRequest {
  /** Omit to leave unchanged; empty string clears the binding. */
  knowledge_base_id?: string
  storage_backend_id?: string
}

export interface CreateGroupRequest {
  name: string
  description?: string
  member_ids?: string[]
}

export interface UpdateGroupRequest {
  name?: string
  description?: string
}

export interface ListGroupMembersParams {
  q?: string
  page?: number
  page_size?: number
}

interface Envelope<T> {
  success?: boolean
  data?: T
  message?: string
  error?: { message?: string } | string
}

function unwrap<T>(res: unknown): T {
  const env = (res ?? {}) as Envelope<T>
  if (env.success === false) {
    const detail = typeof env.error === 'string' ? env.error : env.error?.message
    throw new Error(env.message || detail || 'request failed')
  }
  return env.data as T
}

const base = '/api/v1/docs'

// ---- spaces ---------------------------------------------------------------

/** Backend: GET /api/v1/docs/spaces (Viewer+). Spaces the caller can read, with their role. */
export async function listSpaces(): Promise<DocsSpace[]> {
  return unwrap<DocsSpace[] | null>(await get(`${base}/spaces`)) ?? []
}

/** Backend: POST /api/v1/docs/spaces (Contributor+). The creator becomes the first admin. */
export async function createSpace(body: CreateSpaceRequest): Promise<DocsSpace> {
  return unwrap<DocsSpace>(await post(`${base}/spaces`, body))
}

/** Backend: GET /api/v1/docs/spaces/:sid (space reader). */
export async function getSpace(id: string): Promise<DocsSpace> {
  return unwrap<DocsSpace>(await get(`${base}/spaces/${encodeURIComponent(id)}`))
}

/** Backend: GET /api/v1/docs/spaces/by-slug/:slug (space reader). */
export async function getSpaceBySlug(slug: string): Promise<DocsSpace> {
  return unwrap<DocsSpace>(await get(`${base}/spaces/by-slug/${encodeURIComponent(slug)}`))
}

/** Backend: PATCH /api/v1/docs/spaces/:sid (space admin). */
export async function updateSpace(id: string, body: UpdateSpaceRequest): Promise<DocsSpace> {
  return unwrap<DocsSpace>(await patch(`${base}/spaces/${encodeURIComponent(id)}`, body))
}

/** Backend: DELETE /api/v1/docs/spaces/:sid (space admin). Moves the space to the trash. */
export async function deleteSpace(id: string): Promise<void> {
  await del(`${base}/spaces/${encodeURIComponent(id)}`)
}

/** Backend: GET /api/v1/docs/spaces/:sid/members (space reader). */
export async function listSpaceMembers(id: string): Promise<SpaceMember[]> {
  return unwrap<SpaceMember[] | null>(await get(`${base}/spaces/${encodeURIComponent(id)}/members`)) ?? []
}

/**
 * Backend: PUT /api/v1/docs/spaces/:sid/members (space admin). Adds members
 * or changes roles; returns the full member list afterwards.
 */
export async function setSpaceMembers(id: string, members: SpaceMemberInput[]): Promise<SpaceMember[]> {
  return unwrap<SpaceMember[] | null>(
    await put(`${base}/spaces/${encodeURIComponent(id)}/members`, { members }),
  ) ?? []
}

/** Backend: DELETE /api/v1/docs/spaces/:sid/members/:ptype/:pid (space admin). */
export async function removeSpaceMember(id: string, type: PrincipalType, principalId: string): Promise<void> {
  await del(`${base}/spaces/${encodeURIComponent(id)}/members/${type}/${encodeURIComponent(principalId)}`)
}

/** Backend: PUT /api/v1/docs/spaces/:sid/knowledge-base (space admin). */
export async function bindSpaceKnowledgeBase(id: string, body: BindKnowledgeBaseRequest): Promise<DocsSpace> {
  return unwrap<DocsSpace>(await put(`${base}/spaces/${encodeURIComponent(id)}/knowledge-base`, body))
}

// ---- tenant groups ---------------------------------------------------------

const groupsBase = '/api/v1/groups'

/** Backend: GET /api/v1/groups (Viewer+). The default group comes first. */
export async function listGroups(): Promise<TenantGroup[]> {
  return unwrap<TenantGroup[] | null>(await get(groupsBase)) ?? []
}

/** Backend: POST /api/v1/groups (Admin+). */
export async function createGroup(body: CreateGroupRequest): Promise<TenantGroup> {
  return unwrap<TenantGroup>(await post(groupsBase, body))
}

/** Backend: PATCH /api/v1/groups/:gid (Admin+). */
export async function updateGroup(id: string, body: UpdateGroupRequest): Promise<TenantGroup> {
  return unwrap<TenantGroup>(await patch(`${groupsBase}/${encodeURIComponent(id)}`, body))
}

/** Backend: DELETE /api/v1/groups/:gid (Admin+). Removes the group's grants everywhere. */
export async function deleteGroup(id: string): Promise<void> {
  await del(`${groupsBase}/${encodeURIComponent(id)}`)
}

/** Backend: GET /api/v1/groups/:gid/members (Viewer+). */
export async function listGroupMembers(id: string, params: ListGroupMembersParams = {}): Promise<GroupMemberPage> {
  const qs = new URLSearchParams()
  if (params.q?.trim()) qs.set('q', params.q.trim())
  if (params.page && params.page > 0) qs.set('page', String(params.page))
  if (params.page_size && params.page_size > 0) qs.set('page_size', String(params.page_size))
  const suffix = qs.toString() ? `?${qs.toString()}` : ''
  const page = unwrap<GroupMemberPage | null>(await get(`${groupsBase}/${encodeURIComponent(id)}/members${suffix}`))
  return page ?? { members: [], total: 0, page: params.page ?? 1, page_size: params.page_size ?? 20 }
}

/** Backend: PUT /api/v1/groups/:gid/members (Admin+). Existing members are skipped. */
export async function addGroupMembers(id: string, userIds: string[]): Promise<TenantGroup> {
  return unwrap<TenantGroup>(await put(`${groupsBase}/${encodeURIComponent(id)}/members`, { user_ids: userIds }))
}

/** Backend: DELETE /api/v1/groups/:gid/members/:uid (Admin+). */
export async function removeGroupMember(id: string, userId: string): Promise<void> {
  await del(`${groupsBase}/${encodeURIComponent(id)}/members/${encodeURIComponent(userId)}`)
}

// ---- pages ----------------------------------------------------------------

export type PageStatus = 'draft' | 'published'

/** A page row as the tree and page endpoints return it (content excluded). */
export interface DocsPage {
  id: string
  short_id: string
  tenant_id: number
  space_id: string
  parent_id: string | null
  position: string
  title: string
  icon?: string | null
  cover?: string | null
  ydoc_version: number
  status: PageStatus
  is_locked: boolean
  template_id?: string | null
  source_refs: string[]
  contributor_ids: string[]
  creator_id?: string | null
  last_editor_id?: string | null
  deleted_by?: string | null
  word_count: number
  attachment_bytes: number
  created_at: string
  updated_at: string
  content_updated_at?: string | null
  deleted_at?: string | null
}

export interface PageView extends DocsPage {
  role: SpaceRole
  can_edit: boolean
  has_children: boolean
  restricted: boolean
}

export interface TreeNode extends DocsPage {
  has_children: boolean
  can_edit: boolean
  restricted: boolean
}

export interface TreePage {
  items: TreeNode[]
  next_cursor?: string
}

export interface TrashEntry extends DocsPage {
  deleted_by_user?: { user_id: string; username?: string; email?: string; avatar?: string }
  can_restore: boolean
}

export interface PageContent {
  page_id: string
  ydoc_version: number
  content: unknown
  html?: string
}

export interface CreatePageRequest {
  space_id: string
  parent_id?: string | null
  title?: string
  icon?: string | null
  /** ProseMirror JSON document; exclusive with markdown. */
  content?: unknown
  markdown?: string
}

export interface UpdatePageRequest {
  title?: string
  /** Empty string clears the icon. */
  icon?: string
  cover?: string
}

export interface MovePageRequest {
  /** null or absent: the space root. */
  parent_id?: string | null
  /** Another space to move into (with the subtree). */
  space_id?: string
  /** Sibling to follow; null puts the page first; absent appends. */
  after_id?: string | null
  /** Explicit order key (API clients that keep their own tree). */
  position?: string
}

export interface MoveResult {
  page: PageView
  orphaned?: string[]
  rebalanced: boolean
}

export interface DuplicatePageRequest {
  space_id?: string
  parent_id?: string | null
  title?: string
}

export interface DuplicateResult {
  page: PageView
  child_ids: string[]
  count: number
}

/** The body of a 410 answer for a page in the trash. */
export interface GonePage {
  page_id: string
  short_id: string
  space_id: string
  title: string
  deleted_at: string
  deleted_by?: string | null
  restorable: boolean
}

/** Recognises the 410 the backend answers for trashed pages. */
export function gonePageFrom(err: unknown): GonePage | null {
  const e = err as { status?: number; data?: GonePage } | null
  if (!e || e.status !== 410 || !e.data || typeof e.data !== 'object') return null
  return e.data
}

export function requestStatus(err: unknown): number | undefined {
  return (err as { status?: number } | null)?.status
}

/** Backend: POST /api/v1/docs/pages (writer on the parent or space). */
export async function createPage(body: CreatePageRequest): Promise<PageView> {
  return unwrap<PageView>(await post(`${base}/pages`, body))
}

/** Backend: GET /api/v1/docs/pages/:pid (page reader). Rejects with status 410 for trashed pages. */
export async function getPage(id: string): Promise<PageView> {
  return unwrap<PageView>(await get(`${base}/pages/${encodeURIComponent(id)}`))
}

/** Backend: GET /api/v1/docs/pages/by-short-id/:short (page reader). */
export async function getPageByShortId(shortId: string): Promise<PageView> {
  return unwrap<PageView>(await get(`${base}/pages/by-short-id/${encodeURIComponent(shortId)}`))
}

/** Backend: GET /api/v1/docs/pages/:pid/content (page reader). */
export async function getPageContent(id: string, format: 'json' | 'html' = 'json'): Promise<PageContent> {
  return unwrap<PageContent>(await get(`${base}/pages/${encodeURIComponent(id)}/content?format=${format}`))
}

/** Backend: PATCH /api/v1/docs/pages/:pid (page writer). */
export async function updatePage(id: string, body: UpdatePageRequest): Promise<PageView> {
  return unwrap<PageView>(await patch(`${base}/pages/${encodeURIComponent(id)}`, body))
}

/** Backend: POST /api/v1/docs/pages/:pid/move (page writer, plus writer on the target). */
export async function movePage(id: string, body: MovePageRequest): Promise<MoveResult> {
  return unwrap<MoveResult>(await post(`${base}/pages/${encodeURIComponent(id)}/move`, body))
}

/** Backend: POST /api/v1/docs/pages/:pid/duplicate (page reader + writer in the target space). */
export async function duplicatePage(id: string, body: DuplicatePageRequest = {}): Promise<DuplicateResult> {
  return unwrap<DuplicateResult>(await post(`${base}/pages/${encodeURIComponent(id)}/duplicate`, body))
}

/** Backend: DELETE /api/v1/docs/pages/:pid (page writer). Moves the subtree to the trash. */
export async function deletePage(id: string): Promise<{ page_id: string; deleted: number }> {
  return unwrap<{ page_id: string; deleted: number }>(await del(`${base}/pages/${encodeURIComponent(id)}`))
}

/** Backend: POST /api/v1/docs/pages/:pid/restore (page writer). */
export async function restorePage(id: string): Promise<PageView> {
  return unwrap<PageView>(await post(`${base}/pages/${encodeURIComponent(id)}/restore`, {}))
}

/** Backend: GET /api/v1/docs/pages/:pid/ancestors (page reader). Root first. */
export async function getPageAncestors(id: string): Promise<DocsPage[]> {
  return unwrap<DocsPage[] | null>(await get(`${base}/pages/${encodeURIComponent(id)}/ancestors`)) ?? []
}

function treeQuery(params: { parent?: string | null; cursor?: string; limit?: number }): string {
  const qs = new URLSearchParams()
  if (params.parent) qs.set('parent', params.parent)
  if (params.cursor) qs.set('cursor', params.cursor)
  if (params.limit && params.limit > 0) qs.set('limit', String(params.limit))
  const s = qs.toString()
  return s ? `?${s}` : ''
}

/** Backend: GET /api/v1/docs/spaces/:sid/tree (space reader). The children of one parent, cursor-paged. */
export async function getSpaceTree(
  spaceId: string,
  params: { parent?: string | null; cursor?: string; limit?: number } = {},
): Promise<TreePage> {
  const page = unwrap<TreePage | null>(
    await get(`${base}/spaces/${encodeURIComponent(spaceId)}/tree${treeQuery(params)}`),
  )
  return page ?? { items: [] }
}

/** Backend: GET /api/v1/docs/pages/:pid/children (page reader). */
export async function getPageChildren(id: string, params: { cursor?: string; limit?: number } = {}): Promise<TreePage> {
  const page = unwrap<TreePage | null>(
    await get(`${base}/pages/${encodeURIComponent(id)}/children${treeQuery(params)}`),
  )
  return page ?? { items: [] }
}

/** Loads every child of a parent by following the cursor. */
export async function loadAllChildren(spaceId: string, parent: string | null): Promise<TreeNode[]> {
  const out: TreeNode[] = []
  let cursor: string | undefined
  do {
    const page = await getSpaceTree(spaceId, { parent, cursor, limit: 500 })
    out.push(...page.items)
    cursor = page.next_cursor
  } while (cursor)
  return out
}

/** Backend: GET /api/v1/docs/spaces/:sid/trash (space reader). */
export async function listTrash(spaceId: string): Promise<TrashEntry[]> {
  return unwrap<TrashEntry[] | null>(await get(`${base}/spaces/${encodeURIComponent(spaceId)}/trash`)) ?? []
}

/** Backend: DELETE /api/v1/docs/spaces/:sid/trash/:pid (space admin). Permanent. */
export async function purgeTrashPage(spaceId: string, pageId: string): Promise<{ purged: string[] }> {
  return unwrap<{ purged: string[] }>(
    await del(`${base}/spaces/${encodeURIComponent(spaceId)}/trash/${encodeURIComponent(pageId)}`),
  )
}

/** Backend: DELETE /api/v1/docs/spaces/:sid/trash (space admin). Permanent. */
export async function emptyTrash(spaceId: string): Promise<{ purged: number }> {
  return unwrap<{ purged: number }>(await del(`${base}/spaces/${encodeURIComponent(spaceId)}/trash`))
}
