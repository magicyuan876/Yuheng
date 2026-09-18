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
