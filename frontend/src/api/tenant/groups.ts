import { del, get, patch, post, put } from "@/utils/request";

// Workspace groups API (backend: internal/router/routes_groups.go). A group
// is a named set of workspace members that permissions can be granted to as
// one principal; the "everyone" default group contains every member and is
// managed by the server. Every function unwraps Yuheng's `{ success, data }`
// envelope and rejects with the server message when success is false.

export interface TenantGroup {
  id: string;
  tenant_id: number;
  name: string;
  description: string;
  is_default: boolean;
  /** Where membership is managed: "manual" here, "oidc" / "ldap" mirrored from an identity provider. */
  source: string;
  created_at: string;
  updated_at: string;
  member_count: number;
}

export interface GroupMemberRow {
  user_id: string;
  username?: string;
  email?: string;
  avatar?: string;
}

export interface GroupMemberPage {
  members: GroupMemberRow[];
  total: number;
  page: number;
  page_size: number;
}

export interface CreateGroupRequest {
  name: string;
  description?: string;
  member_ids?: string[];
}

export interface UpdateGroupRequest {
  name?: string;
  description?: string;
}

export interface ListGroupMembersParams {
  q?: string;
  page?: number;
  page_size?: number;
}

interface Envelope<T> {
  success?: boolean;
  data?: T;
  message?: string;
  error?: { message?: string } | string;
}

function unwrap<T>(res: unknown): T {
  const env = (res ?? {}) as Envelope<T>;
  if (env.success === false) {
    const detail = typeof env.error === "string" ? env.error : env.error?.message;
    throw new Error(env.message || detail || "request failed");
  }
  return env.data as T;
}

const base = "/api/v1/groups";

/** Backend: GET /api/v1/groups (Viewer+). The default group comes first. */
export async function listGroups(): Promise<TenantGroup[]> {
  return unwrap<TenantGroup[] | null>(await get(base)) ?? [];
}

/** Backend: POST /api/v1/groups (Admin+). */
export async function createGroup(body: CreateGroupRequest): Promise<TenantGroup> {
  return unwrap<TenantGroup>(await post(base, body));
}

/** Backend: PATCH /api/v1/groups/:gid (Admin+). */
export async function updateGroup(id: string, body: UpdateGroupRequest): Promise<TenantGroup> {
  return unwrap<TenantGroup>(await patch(`${base}/${encodeURIComponent(id)}`, body));
}

/** Backend: DELETE /api/v1/groups/:gid (Admin+). Removes every permission granted to the group. */
export async function deleteGroup(id: string): Promise<void> {
  await del(`${base}/${encodeURIComponent(id)}`);
}

/** Backend: GET /api/v1/groups/:gid/members (Viewer+). */
export async function listGroupMembers(id: string, params: ListGroupMembersParams = {}): Promise<GroupMemberPage> {
  const qs = new URLSearchParams();
  if (params.q?.trim()) qs.set("q", params.q.trim());
  if (params.page && params.page > 0) qs.set("page", String(params.page));
  if (params.page_size && params.page_size > 0) qs.set("page_size", String(params.page_size));
  const suffix = qs.toString() ? `?${qs.toString()}` : "";
  const page = unwrap<GroupMemberPage | null>(await get(`${base}/${encodeURIComponent(id)}/members${suffix}`));
  return page ?? { members: [], total: 0, page: params.page ?? 1, page_size: params.page_size ?? 20 };
}

/** Backend: PUT /api/v1/groups/:gid/members (Admin+). Existing members are skipped. */
export async function addGroupMembers(id: string, userIds: string[]): Promise<TenantGroup> {
  return unwrap<TenantGroup>(await put(`${base}/${encodeURIComponent(id)}/members`, { user_ids: userIds }));
}

/** Backend: DELETE /api/v1/groups/:gid/members/:uid (Admin+). */
export async function removeGroupMember(id: string, userId: string): Promise<void> {
  await del(`${base}/${encodeURIComponent(id)}/members/${encodeURIComponent(userId)}`);
}
