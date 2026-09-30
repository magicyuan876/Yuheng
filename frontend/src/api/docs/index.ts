import {
  del,
  get,
  getDownload,
  patch,
  post,
  postDownload,
  postUpload,
  put,
  saveBlob,
  type DownloadedFile,
} from "@/utils/request";

// Online documents API (backend: internal/router/routes_docs.go). Every
// function unwraps Yuheng's `{ success, data }` envelope and rejects with the
// server message when success is false, so callers only deal with data.

export type SpaceVisibility = "private" | "open" | "public";
export type SpaceRole = "none" | "reader" | "writer" | "admin";
export type PrincipalType = "user" | "group";

export interface DocsSpace {
  id: string;
  tenant_id: number;
  slug: string;
  name: string;
  description: string;
  icon?: string | null;
  visibility: SpaceVisibility;
  default_role: SpaceRole;
  knowledge_base_id?: string | null;
  storage_backend_id?: string | null;
  settings?: Record<string, unknown>;
  creator_id?: string | null;
  created_at: string;
  updated_at: string;
  /** The caller's effective role in this space. */
  role: SpaceRole;
  /** Direct memberships (users and groups). */
  member_count: number;
  page_count: number;
}

export interface SpaceMember {
  principal_type: PrincipalType;
  principal_id: string;
  role: SpaceRole;
  name: string;
  email?: string;
  avatar?: string;
  is_default_group?: boolean;
  group_member_count?: number;
  added_by?: string;
  created_at: string;
}

export interface TenantGroup {
  id: string;
  tenant_id: number;
  name: string;
  description: string;
  is_default: boolean;
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

export interface CreateSpaceRequest {
  name: string;
  slug?: string;
  description?: string;
  icon?: string | null;
  visibility?: SpaceVisibility;
  default_role?: SpaceRole;
  knowledge_base_id?: string | null;
  storage_backend_id?: string | null;
  settings?: Record<string, unknown>;
}

export interface UpdateSpaceRequest {
  name?: string;
  slug?: string;
  description?: string;
  icon?: string;
  visibility?: SpaceVisibility;
  default_role?: SpaceRole;
  settings?: Record<string, unknown>;
}

export interface SpaceMemberInput {
  principal_type: PrincipalType;
  principal_id: string;
  role: SpaceRole;
}

export interface BindKnowledgeBaseRequest {
  /** Omit to leave unchanged; empty string clears the binding. */
  knowledge_base_id?: string;
  storage_backend_id?: string;
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

const base = "/api/v1/docs";

// ---- spaces ---------------------------------------------------------------

/** Backend: GET /api/v1/docs/spaces (Viewer+). Spaces the caller can read, with their role. */
export async function listSpaces(): Promise<DocsSpace[]> {
  return unwrap<DocsSpace[] | null>(await get(`${base}/spaces`)) ?? [];
}

/** Backend: POST /api/v1/docs/spaces (Contributor+). The creator becomes the first admin. */
export async function createSpace(body: CreateSpaceRequest): Promise<DocsSpace> {
  return unwrap<DocsSpace>(await post(`${base}/spaces`, body));
}

/** Backend: GET /api/v1/docs/spaces/:sid (space reader). */
export async function getSpace(id: string): Promise<DocsSpace> {
  return unwrap<DocsSpace>(await get(`${base}/spaces/${encodeURIComponent(id)}`));
}

/** Backend: GET /api/v1/docs/spaces/by-slug/:slug (space reader). */
export async function getSpaceBySlug(slug: string): Promise<DocsSpace> {
  return unwrap<DocsSpace>(await get(`${base}/spaces/by-slug/${encodeURIComponent(slug)}`));
}

/** Backend: PATCH /api/v1/docs/spaces/:sid (space admin). */
export async function updateSpace(id: string, body: UpdateSpaceRequest): Promise<DocsSpace> {
  return unwrap<DocsSpace>(await patch(`${base}/spaces/${encodeURIComponent(id)}`, body));
}

/** Backend: DELETE /api/v1/docs/spaces/:sid (space admin). Moves the space to the trash. */
export async function deleteSpace(id: string): Promise<void> {
  await del(`${base}/spaces/${encodeURIComponent(id)}`);
}

/** Backend: GET /api/v1/docs/spaces/:sid/members (space reader). */
export async function listSpaceMembers(id: string): Promise<SpaceMember[]> {
  return unwrap<SpaceMember[] | null>(await get(`${base}/spaces/${encodeURIComponent(id)}/members`)) ?? [];
}

/**
 * Backend: PUT /api/v1/docs/spaces/:sid/members (space admin). Adds members
 * or changes roles; returns the full member list afterwards.
 */
export async function setSpaceMembers(id: string, members: SpaceMemberInput[]): Promise<SpaceMember[]> {
  return unwrap<SpaceMember[] | null>(await put(`${base}/spaces/${encodeURIComponent(id)}/members`, { members })) ?? [];
}

/** Backend: DELETE /api/v1/docs/spaces/:sid/members/:ptype/:pid (space admin). */
export async function removeSpaceMember(id: string, type: PrincipalType, principalId: string): Promise<void> {
  await del(`${base}/spaces/${encodeURIComponent(id)}/members/${type}/${encodeURIComponent(principalId)}`);
}

/** Backend: PUT /api/v1/docs/spaces/:sid/knowledge-base (space admin). */
export async function bindSpaceKnowledgeBase(id: string, body: BindKnowledgeBaseRequest): Promise<DocsSpace> {
  return unwrap<DocsSpace>(await put(`${base}/spaces/${encodeURIComponent(id)}/knowledge-base`, body));
}

// ---- tenant groups ---------------------------------------------------------

const groupsBase = "/api/v1/groups";

/** Backend: GET /api/v1/groups (Viewer+). The default group comes first. */
export async function listGroups(): Promise<TenantGroup[]> {
  return unwrap<TenantGroup[] | null>(await get(groupsBase)) ?? [];
}

/** Backend: POST /api/v1/groups (Admin+). */
export async function createGroup(body: CreateGroupRequest): Promise<TenantGroup> {
  return unwrap<TenantGroup>(await post(groupsBase, body));
}

/** Backend: PATCH /api/v1/groups/:gid (Admin+). */
export async function updateGroup(id: string, body: UpdateGroupRequest): Promise<TenantGroup> {
  return unwrap<TenantGroup>(await patch(`${groupsBase}/${encodeURIComponent(id)}`, body));
}

/** Backend: DELETE /api/v1/groups/:gid (Admin+). Removes the group's grants everywhere. */
export async function deleteGroup(id: string): Promise<void> {
  await del(`${groupsBase}/${encodeURIComponent(id)}`);
}

/** Backend: GET /api/v1/groups/:gid/members (Viewer+). */
export async function listGroupMembers(id: string, params: ListGroupMembersParams = {}): Promise<GroupMemberPage> {
  const qs = new URLSearchParams();
  if (params.q?.trim()) qs.set("q", params.q.trim());
  if (params.page && params.page > 0) qs.set("page", String(params.page));
  if (params.page_size && params.page_size > 0) qs.set("page_size", String(params.page_size));
  const suffix = qs.toString() ? `?${qs.toString()}` : "";
  const page = unwrap<GroupMemberPage | null>(await get(`${groupsBase}/${encodeURIComponent(id)}/members${suffix}`));
  return page ?? { members: [], total: 0, page: params.page ?? 1, page_size: params.page_size ?? 20 };
}

/** Backend: PUT /api/v1/groups/:gid/members (Admin+). Existing members are skipped. */
export async function addGroupMembers(id: string, userIds: string[]): Promise<TenantGroup> {
  return unwrap<TenantGroup>(await put(`${groupsBase}/${encodeURIComponent(id)}/members`, { user_ids: userIds }));
}

/** Backend: DELETE /api/v1/groups/:gid/members/:uid (Admin+). */
export async function removeGroupMember(id: string, userId: string): Promise<void> {
  await del(`${groupsBase}/${encodeURIComponent(id)}/members/${encodeURIComponent(userId)}`);
}

// ---- pages ----------------------------------------------------------------

/** A page row as the tree and page endpoints return it (content excluded). */
export interface DocsPage {
  id: string;
  short_id: string;
  tenant_id: number;
  space_id: string;
  parent_id: string | null;
  position: string;
  title: string;
  icon?: string | null;
  cover?: string | null;
  ydoc_version: number;
  /** Kept out of the space's knowledge base, and so out of AI answers. Not a permission. */
  exclude_from_knowledge: boolean;
  /** Set while the page is excluded because another document superseded it. */
  superseded_by?: SupersededBy | null;
  is_locked: boolean;
  template_id?: string | null;
  source_refs: string[];
  contributor_ids: string[];
  creator_id?: string | null;
  /** Set when the page was handed to another maintainer; see steward_id. */
  owner_id?: string | null;
  last_editor_id?: string | null;
  deleted_by?: string | null;
  word_count: number;
  attachment_bytes: number;
  created_at: string;
  updated_at: string;
  content_updated_at?: string | null;
  deleted_at?: string | null;
}

/** A snapshot of what superseded a page, taken when it happened. */
export interface SupersededBy {
  knowledge_id: string;
  title: string;
  /** Set when the replacement is a page. */
  page_id?: string;
  by?: string;
  at: string;
}

export interface PageView extends DocsPage {
  role: SpaceRole;
  can_edit: boolean;
  has_children: boolean;
  restricted: boolean;
  /** What the page is filed under. Carried on the page read so the chips
   * appear with the title rather than a moment after it. */
  labels?: LabelView[];
  /** Whether the caller starred this page. */
  favourite?: boolean;
  /** The page's maintainer: the person it was handed to, else its creator.
   * Knowledge health takes the problems of the page to them. */
  steward_id?: string;
  /** Whether the caller may hand the page to somebody else. */
  can_change_owner?: boolean;
  /** The maintainer, named. */
  steward?: { user_id: string; username?: string; email?: string; avatar?: string } | null;
}

export interface TreeNode extends DocsPage {
  has_children: boolean;
  can_edit: boolean;
  restricted: boolean;
}

export interface TreePage {
  items: TreeNode[];
  next_cursor?: string;
}

export interface TrashEntry extends DocsPage {
  deleted_by_user?: { user_id: string; username?: string; email?: string; avatar?: string };
  can_restore: boolean;
}

export interface PageContent {
  page_id: string;
  ydoc_version: number;
  content: unknown;
  html?: string;
}

export interface CreatePageRequest {
  space_id: string;
  parent_id?: string | null;
  title?: string;
  icon?: string | null;
  /** ProseMirror JSON document; exclusive with markdown. */
  content?: unknown;
  markdown?: string;
  /** Start the page from a saved body. Mutually exclusive with content and
   * markdown. */
  template_id?: string;
}

export interface UpdatePageRequest {
  title?: string;
  /** Empty string clears the icon. */
  icon?: string;
  cover?: string;
}

export interface MovePageRequest {
  /** null or absent: the space root. */
  parent_id?: string | null;
  /** Another space to move into (with the subtree). */
  space_id?: string;
  /** Sibling to follow; null puts the page first; absent appends. */
  after_id?: string | null;
  /** Explicit order key (API clients that keep their own tree). */
  position?: string;
}

export interface MoveResult {
  page: PageView;
  orphaned?: string[];
  rebalanced: boolean;
}

export interface DuplicatePageRequest {
  space_id?: string;
  parent_id?: string | null;
  title?: string;
}

export interface DuplicateResult {
  page: PageView;
  child_ids: string[];
  count: number;
}

/** The body of a 410 answer for a page in the trash. */
export interface GonePage {
  page_id: string;
  short_id: string;
  space_id: string;
  title: string;
  deleted_at: string;
  deleted_by?: string | null;
  restorable: boolean;
}

/** Recognises the 410 the backend answers for trashed pages. */
export function gonePageFrom(err: unknown): GonePage | null {
  const e = err as { status?: number; data?: GonePage } | null;
  if (!e || e.status !== 410 || !e.data || typeof e.data !== "object") return null;
  return e.data;
}

export function requestStatus(err: unknown): number | undefined {
  return (err as { status?: number } | null)?.status;
}

/** Backend: POST /api/v1/docs/pages (writer on the parent or space). */
export async function createPage(body: CreatePageRequest): Promise<PageView> {
  return unwrap<PageView>(await post(`${base}/pages`, body));
}

/** Backend: GET /api/v1/docs/pages/:pid (page reader). Rejects with status 410 for trashed pages. */
export async function getPage(id: string): Promise<PageView> {
  return unwrap<PageView>(await get(`${base}/pages/${encodeURIComponent(id)}`));
}

/** Backend: GET /api/v1/docs/pages/by-short-id/:short (page reader). */
export async function getPageByShortId(shortId: string): Promise<PageView> {
  return unwrap<PageView>(await get(`${base}/pages/by-short-id/${encodeURIComponent(shortId)}`));
}

/** Backend: GET /api/v1/docs/pages/:pid/content (page reader). */
export async function getPageContent(id: string, format: "json" | "html" = "json"): Promise<PageContent> {
  return unwrap<PageContent>(await get(`${base}/pages/${encodeURIComponent(id)}/content?format=${format}`));
}

/** Backend: PATCH /api/v1/docs/pages/:pid (page writer). */
export async function updatePage(id: string, body: UpdatePageRequest): Promise<PageView> {
  return unwrap<PageView>(await patch(`${base}/pages/${encodeURIComponent(id)}`, body));
}

/** Backend: POST /api/v1/docs/pages/:pid/move (page writer, plus writer on the target). */
export async function movePage(id: string, body: MovePageRequest): Promise<MoveResult> {
  return unwrap<MoveResult>(await post(`${base}/pages/${encodeURIComponent(id)}/move`, body));
}

/** Backend: POST /api/v1/docs/pages/:pid/duplicate (page reader + writer in the target space). */
export async function duplicatePage(id: string, body: DuplicatePageRequest = {}): Promise<DuplicateResult> {
  return unwrap<DuplicateResult>(await post(`${base}/pages/${encodeURIComponent(id)}/duplicate`, body));
}

/** Backend: DELETE /api/v1/docs/pages/:pid (page writer). Moves the subtree to the trash. */
export async function deletePage(id: string): Promise<{ page_id: string; deleted: number }> {
  return unwrap<{ page_id: string; deleted: number }>(await del(`${base}/pages/${encodeURIComponent(id)}`));
}

/** Backend: POST /api/v1/docs/pages/:pid/restore (page writer). */
export async function restorePage(id: string): Promise<PageView> {
  return unwrap<PageView>(await post(`${base}/pages/${encodeURIComponent(id)}/restore`, {}));
}

/** Backend: GET /api/v1/docs/pages/:pid/ancestors (page reader). Root first. */
export async function getPageAncestors(id: string): Promise<DocsPage[]> {
  return unwrap<DocsPage[] | null>(await get(`${base}/pages/${encodeURIComponent(id)}/ancestors`)) ?? [];
}

function treeQuery(params: { parent?: string | null; cursor?: string; limit?: number }): string {
  const qs = new URLSearchParams();
  if (params.parent) qs.set("parent", params.parent);
  if (params.cursor) qs.set("cursor", params.cursor);
  if (params.limit && params.limit > 0) qs.set("limit", String(params.limit));
  const s = qs.toString();
  return s ? `?${s}` : "";
}

/** Backend: GET /api/v1/docs/spaces/:sid/tree (space reader). The children of one parent, cursor-paged. */
export async function getSpaceTree(
  spaceId: string,
  params: { parent?: string | null; cursor?: string; limit?: number } = {},
): Promise<TreePage> {
  const page = unwrap<TreePage | null>(
    await get(`${base}/spaces/${encodeURIComponent(spaceId)}/tree${treeQuery(params)}`),
  );
  return page ?? { items: [] };
}

/** Backend: GET /api/v1/docs/pages/:pid/children (page reader). */
export async function getPageChildren(id: string, params: { cursor?: string; limit?: number } = {}): Promise<TreePage> {
  const page = unwrap<TreePage | null>(
    await get(`${base}/pages/${encodeURIComponent(id)}/children${treeQuery(params)}`),
  );
  return page ?? { items: [] };
}

/** Loads every child of a parent by following the cursor. */
export async function loadAllChildren(spaceId: string, parent: string | null): Promise<TreeNode[]> {
  const out: TreeNode[] = [];
  let cursor: string | undefined;
  do {
    const page = await getSpaceTree(spaceId, { parent, cursor, limit: 500 });
    out.push(...page.items);
    cursor = page.next_cursor;
  } while (cursor);
  return out;
}

/** Backend: GET /api/v1/docs/spaces/:sid/trash (space reader). */
export async function listTrash(spaceId: string): Promise<TrashEntry[]> {
  return unwrap<TrashEntry[] | null>(await get(`${base}/spaces/${encodeURIComponent(spaceId)}/trash`)) ?? [];
}

/** Backend: DELETE /api/v1/docs/spaces/:sid/trash/:pid (space admin). Permanent. */
export async function purgeTrashPage(spaceId: string, pageId: string): Promise<{ purged: string[] }> {
  return unwrap<{ purged: string[] }>(
    await del(`${base}/spaces/${encodeURIComponent(spaceId)}/trash/${encodeURIComponent(pageId)}`),
  );
}

/** Backend: DELETE /api/v1/docs/spaces/:sid/trash (space admin). Permanent. */
export async function emptyTrash(spaceId: string): Promise<{ purged: number }> {
  return unwrap<{ purged: number }>(await del(`${base}/spaces/${encodeURIComponent(spaceId)}/trash`));
}

// ---- exclusive editing (deployments without a collaboration service) ------

export interface EditLease {
  page_id: string;
  /** False when the page is free to take. */
  held: boolean;
  /** True only for the session that asked. */
  held_by_me: boolean;
  holder?: { user_id: string; username?: string; email?: string; avatar?: string };
  expires_at?: string;
  /** How often the holder should renew, in seconds. */
  renew_after_seconds: number;
  ydoc_version: number;
}

export interface YDocState {
  page_id: string;
  /** Base64 Yjs state; absent for a page that has never been edited. */
  ydoc?: string;
  /** The stored body to build a Yjs document from, when there is no state yet. */
  content?: unknown;
  ydoc_version: number;
}

export interface SaveYDocRequest {
  session_id: string;
  base_version: number;
  /** Base64 Yjs state. */
  ydoc: string;
  content: unknown;
}

export interface SaveYDocResult {
  ydoc_version: number;
  lease: EditLease;
}

/** Backend: GET /api/v1/docs/pages/:pid/lease (page reader). 409 where a collaboration service runs. */
export async function getPageLease(id: string): Promise<EditLease> {
  return unwrap<EditLease>(await get(`${base}/pages/${encodeURIComponent(id)}/lease`));
}

/** Backend: POST /api/v1/docs/pages/:pid/lease (page writer). Takes the lease, or renews this session's. */
export async function acquirePageLease(id: string, sessionId: string): Promise<EditLease> {
  return unwrap<EditLease>(await post(`${base}/pages/${encodeURIComponent(id)}/lease`, { session_id: sessionId }));
}

/** Backend: DELETE /api/v1/docs/pages/:pid/lease (page writer). Only the holding session frees the page. */
export async function releasePageLease(id: string, sessionId: string): Promise<void> {
  await del(`${base}/pages/${encodeURIComponent(id)}/lease?session_id=${encodeURIComponent(sessionId)}`);
}

/** Backend: GET /api/v1/docs/pages/:pid/ydoc (page reader). */
export async function getPageYDoc(id: string): Promise<YDocState> {
  return unwrap<YDocState>(await get(`${base}/pages/${encodeURIComponent(id)}/ydoc`));
}

/** Backend: PUT /api/v1/docs/pages/:pid/ydoc (page writer, holding the lease). 409 on a stale base version. */
export async function savePageYDoc(id: string, body: SaveYDocRequest): Promise<SaveYDocResult> {
  return unwrap<SaveYDocResult>(await put(`${base}/pages/${encodeURIComponent(id)}/ydoc`, body));
}

// ---- attachments ----------------------------------------------------------

export type AttachmentKind = "file" | "image" | "video" | "audio" | "diagram";

export interface DocsAttachment {
  id: string;
  space_id: string;
  page_id?: string;
  file_name: string;
  /** The media type the server derived from the bytes, never from the name. */
  mime: string;
  size_bytes: number;
  kind: AttachmentKind;
  width?: number;
  height?: number;
  /** The permission-checked address to read it from. */
  url: string;
  uploader?: { user_id: string; username?: string; email?: string; avatar?: string };
  created_at: string;
  /** Widths this image can also be served at, for a srcset. */
  variants?: number[];
}

/**
 * Backend: POST /api/v1/docs/spaces/:sid/attachments (space writer).
 * The server sniffs the type, sanitises SVG, deduplicates by content digest
 * and charges the workspace quota, so the client sends the file as it is.
 */
export async function uploadAttachment(
  spaceId: string,
  file: File,
  opts: { pageId?: string; onProgress?: (percent: number) => void; signal?: AbortSignal } = {},
): Promise<DocsAttachment> {
  const form = new FormData();
  form.append("file", file);
  if (opts.pageId) form.append("page_id", opts.pageId);
  const res = await postUpload(
    `${base}/spaces/${encodeURIComponent(spaceId)}/attachments`,
    form,
    (event: { loaded?: number; total?: number }) => {
      if (!opts.onProgress || !event?.total) return;
      opts.onProgress(Math.min(100, Math.round(((event.loaded ?? 0) / event.total) * 100)));
    },
    { signal: opts.signal },
  );
  return unwrap<DocsAttachment>(res);
}

/** Backend: GET /api/v1/docs/pages/:pid/attachments (page reader). */
export async function listPageAttachments(pageId: string): Promise<DocsAttachment[]> {
  return unwrap<DocsAttachment[] | null>(await get(`${base}/pages/${encodeURIComponent(pageId)}/attachments`)) ?? [];
}

/** Backend: DELETE /api/v1/docs/attachments/:aid (page or space writer). */
export async function deleteAttachment(id: string): Promise<void> {
  await del(`${base}/attachments/${encodeURIComponent(id)}`);
}

// ---- whole-body writes ----------------------------------------------------

export interface ReplaceContentRequest {
  /** A ProseMirror document; mutually exclusive with markdown. */
  content?: unknown;
  markdown?: string;
}

export interface ReplaceContentResult {
  ydoc_version: number;
  /** 'collab' when the live document was updated in place, 'direct' when the
   * stored body was replaced and the Yjs state left to be rebuilt. */
  applied: "collab" | "direct";
}

/**
 * Backend: PUT /api/v1/docs/pages/:pid/content (page writer).
 * The single entry point for writing a body without typing it. Where a
 * collaboration service runs, the change reaches everyone with the page open
 * as one undoable step.
 */
export async function replacePageContent(id: string, body: ReplaceContentRequest): Promise<ReplaceContentResult> {
  return unwrap<ReplaceContentResult>(await put(`${base}/pages/${encodeURIComponent(id)}/content`, body));
}

// ---- page links, mentions and backlinks -----------------------------------

export interface PageRef {
  page_id: string;
  short_id?: string;
  space_id?: string;
  title: string;
  icon?: string;
  breadcrumb?: string[];
  /** False when the page is gone or the caller may not see it. The server
   * does not distinguish the two, so neither does the editor. */
  resolved: boolean;
}

export interface MentionCandidate {
  user_id: string;
  username?: string;
  email?: string;
  avatar?: string;
}

/** Backend: GET /api/v1/docs/pages/:pid/backlinks (page reader). */
export async function listBacklinks(pageId: string): Promise<PageRef[]> {
  return unwrap<PageRef[] | null>(await get(`${base}/pages/${encodeURIComponent(pageId)}/backlinks`)) ?? [];
}

/**
 * Backend: POST /api/v1/docs/page-links/titles (tenant member).
 * A POST that only reads: the id list is as long as the open page has links.
 */
export async function resolvePageTitles(pageIds: string[]): Promise<PageRef[]> {
  return unwrap<PageRef[] | null>(await post(`${base}/page-links/titles`, { page_ids: pageIds })) ?? [];
}

// ---- search --------------------------------------------------------------------

/**
 * One search result.
 *
 * `kind` says what matched: the page itself, a comment on it, or text the
 * page shows by reference from another page. A transclusion hit carries
 * `source_page_id` — the page the text lives on, and the page whose
 * permissions allowed the result.
 */
export interface SearchHit {
  kind: "page" | "comment" | "transclusion";
  page_id: string;
  short_id: string;
  space_id: string;
  /** The slug a URL is built from. Carried on the hit because results come
   * from any space the caller can read. */
  space_slug: string;
  title: string;
  excerpt: string;
  comment_id?: string;
  source_page_id?: string;
  score: number;
}

export interface SearchResults {
  query: string;
  hits: SearchHit[];
  /** True when more matched than were returned. */
  truncated: boolean;
}

/**
 * Backend: GET /api/v1/docs/search (workspace member).
 *
 * Matches pages, comments and referenced text, all filtered by what the
 * caller may read.
 */
export async function searchDocs(query: string, opts: { space?: string; limit?: number } = {}): Promise<SearchResults> {
  const params = new URLSearchParams({ q: query });
  if (opts.space) params.set("space", opts.space);
  if (opts.limit) params.set("limit", String(opts.limit));
  return unwrap<SearchResults>(await get(`${base}/search?${params.toString()}`));
}

// ---- space storage ------------------------------------------------------------

/** What a space holds and what it may hold. `quota_bytes` 0 is unlimited. */
export interface SpaceUsage {
  space_id: string;
  used_bytes: number;
  quota_bytes: number;
  /** True when the limit came from the deployment rather than this space. */
  from_default: boolean;
  /** Whether the caller may change the quota (workspace administrators only). */
  can_manage: boolean;
}

/** Backend: GET /api/v1/docs/spaces/:sid/usage (space reader). */
export async function getSpaceUsage(spaceId: string): Promise<SpaceUsage> {
  return unwrap<SpaceUsage>(await get(`${base}/spaces/${encodeURIComponent(spaceId)}/usage`));
}

/**
 * Backend: PUT /api/v1/docs/spaces/:sid/quota (workspace administrator).
 *
 * Setting a quota below current usage stops the space growing and deletes
 * nothing.
 */
export async function setSpaceQuota(spaceId: string, quotaBytes: number): Promise<SpaceUsage> {
  return unwrap<SpaceUsage>(
    await put(`${base}/spaces/${encodeURIComponent(spaceId)}/quota`, { quota_bytes: quotaBytes }),
  );
}

// ---- locking and publication state -------------------------------------------

/**
 * Backend: PUT /api/v1/docs/pages/:pid/lock (page admin).
 *
 * A locked page caps everybody except space administrators at reader, and
 * open collaborative sessions are disconnected. Both locking and unlocking
 * need an administrator.
 */
export async function setPageLocked(pageId: string, locked: boolean): Promise<PageView> {
  return unwrap<PageView>(await put(`${base}/pages/${encodeURIComponent(pageId)}/lock`, { locked }));
}

/**
 * Backend: PUT /api/v1/docs/pages/:pid/knowledge (page writer). Excluding a page keeps it out of AI
 * answers; everyone who can read the page still can.
 */
export async function setPageKnowledgeExcluded(pageId: string, excluded: boolean): Promise<PageView> {
  return unwrap<PageView>(await put(`${base}/pages/${encodeURIComponent(pageId)}/knowledge`, { excluded }));
}

/**
 * Backend: PUT /api/v1/docs/pages/:pid/owner. Hands the page to another maintainer, who must be
 * able to edit it; only the current maintainer or an administrator of the page may.
 */
export async function setPageOwner(pageId: string, ownerId: string): Promise<PageView> {
  return unwrap<PageView>(await put(`${base}/pages/${encodeURIComponent(pageId)}/owner`, { owner_id: ownerId }));
}

/**
 * Backend: POST /api/v1/docs/pages/:pid/review (page writer). Vouches for the page as it stands,
 * restarting its review clock in the knowledge base.
 */
export async function confirmPageReviewed(pageId: string): Promise<void> {
  unwrap<unknown>(await post(`${base}/pages/${encodeURIComponent(pageId)}/review`, {}));
}

// ---- templates ---------------------------------------------------------------

/**
 * A reusable page body.
 *
 * `shared` true means the template belongs to the whole workspace and appears
 * in every space; false means it belongs to one space. `content` is present
 * only on a single read, never in a listing.
 */
export interface TemplateView {
  id: string;
  space_id?: string;
  name: string;
  description?: string;
  icon?: string;
  category?: string;
  shared: boolean;
  content?: unknown;
  creator: { user_id: string; username?: string; email?: string; avatar?: string };
  can_edit: boolean;
  created_at: string;
  updated_at: string;
}

/** Backend: GET /api/v1/docs/templates?space=... (workspace member). */
export async function listTemplates(spaceId?: string): Promise<TemplateView[]> {
  const query = spaceId ? `?space=${encodeURIComponent(spaceId)}` : "";
  return unwrap<TemplateView[] | null>(await get(`${base}/templates${query}`)) ?? [];
}

/** Backend: GET /api/v1/docs/templates/:tid — carries the body. */
export async function getTemplate(templateId: string): Promise<TemplateView> {
  return unwrap<TemplateView>(await get(`${base}/templates/${encodeURIComponent(templateId)}`));
}

/**
 * Backend: POST /api/v1/docs/templates.
 *
 * `space_id` empty makes it workspace-wide, which needs an administrator.
 * Give either `content` or `from_page_id`, never both. Whatever cannot
 * travel - page links, block references, mentions, attachments - is removed
 * as it is saved.
 */
export async function createTemplate(body: {
  space_id?: string;
  name: string;
  description?: string;
  icon?: string;
  category?: string;
  content?: unknown;
  from_page_id?: string;
}): Promise<TemplateView> {
  return unwrap<TemplateView>(await post(`${base}/templates`, body));
}

/** Backend: PATCH /api/v1/docs/templates/:tid. Absent fields are left alone. */
export async function updateTemplate(
  templateId: string,
  body: {
    name?: string;
    description?: string;
    icon?: string;
    category?: string;
    content?: unknown;
  },
): Promise<TemplateView> {
  return unwrap<TemplateView>(await patch(`${base}/templates/${encodeURIComponent(templateId)}`, body));
}

/** Backend: DELETE /api/v1/docs/templates/:tid (scope administrator). */
export async function deleteTemplate(templateId: string): Promise<void> {
  await del(`${base}/templates/${encodeURIComponent(templateId)}`);
}

// ---- public share links and public spaces -----------------------------------

/** A public link as its owner sees it. Carries the key: the owner is the one
 * person entitled to it. */
export interface ShareView {
  id: string;
  page_id: string;
  space_id: string;
  key: string;
  include_children: boolean;
  allow_search_index: boolean;
  has_password: boolean;
  expires_at?: string;
  view_count: number;
  creator: { user_id: string; username?: string; email?: string; avatar?: string };
  created_at: string;
  /** False when the link resolves to nothing right now — the page was
   * restricted or trashed after the link was made. */
  live: boolean;
}

/** What a visit to a link produced. Only 'ok' carries a page. */
export type ShareState = "ok" | "password" | "expired" | "revoked" | "gone";

/** One page in a shared subtree. */
export interface SharedRef {
  short_id: string;
  title: string;
  icon?: string;
}

/** What an anonymous visitor gets: a document, and nothing else. */
export interface SharedPage {
  title: string;
  icon?: string;
  html: string;
  short_id: string;
  updated_at: string;
  children: SharedRef[];
  breadcrumb: SharedRef[];
  allow_search_index: boolean;
  space_name: string;
}

export interface ShareResult {
  state: ShareState;
  page?: SharedPage;
  unlock_token?: string;
}

export interface PublicSpaceView {
  id: string;
  name: string;
  slug: string;
  description?: string;
  icon?: string;
  pages: SharedRef[];
}

/** Backend: GET /api/v1/docs/pages/:pid/shares (page reader). */
export async function listShares(pageId: string): Promise<ShareView[]> {
  return unwrap<ShareView[] | null>(await get(`${base}/pages/${encodeURIComponent(pageId)}/shares`)) ?? [];
}

/** Backend: POST /api/v1/docs/pages/:pid/shares (page writer). */
export async function createShare(
  pageId: string,
  body: {
    include_children?: boolean;
    allow_search_index?: boolean;
    password?: string;
    expires_at?: string | null;
  },
): Promise<ShareView> {
  return unwrap<ShareView>(await post(`${base}/pages/${encodeURIComponent(pageId)}/shares`, body));
}

/**
 * Backend: PATCH /api/v1/docs/pages/:pid/shares/:shid (page writer).
 *
 * An absent field is left alone. `password: ''` removes the password;
 * `clear_expiry: true` makes the link permanent.
 */
export async function updateShare(
  pageId: string,
  shareId: string,
  body: {
    include_children?: boolean;
    allow_search_index?: boolean;
    password?: string;
    expires_at?: string;
    clear_expiry?: boolean;
  },
): Promise<ShareView> {
  return unwrap<ShareView>(
    await patch(`${base}/pages/${encodeURIComponent(pageId)}/shares/${encodeURIComponent(shareId)}`, body),
  );
}

/** Backend: DELETE /api/v1/docs/pages/:pid/shares/:shid (page writer). */
export async function revokeShare(pageId: string, shareId: string): Promise<void> {
  await del(`${base}/pages/${encodeURIComponent(pageId)}/shares/${encodeURIComponent(shareId)}`);
}

/** The header an unlock token travels in — never a query parameter, so it
 * stays out of access logs and pasted URLs. */
export const UNLOCK_HEADER = "X-Docs-Share-Unlock";

/**
 * Backend: GET /api/v1/docs/public/:key — no authentication.
 *
 * Always answers 200 with a state; a dead link is a thing to render, not a
 * fetch failure.
 */
export async function visitShare(
  key: string,
  opts: { page?: string; unlockToken?: string } = {},
): Promise<ShareResult> {
  const query = opts.page ? `?page=${encodeURIComponent(opts.page)}` : "";
  return unwrap<ShareResult>(
    await get(
      `${base}/public/${encodeURIComponent(key)}${query}`,
      opts.unlockToken ? { headers: { [UNLOCK_HEADER]: opts.unlockToken } } : undefined,
    ),
  );
}

/** Backend: POST /api/v1/docs/public/:key/unlock — no authentication. */
export async function unlockShare(key: string, password: string): Promise<ShareResult> {
  return unwrap<ShareResult>(await post(`${base}/public/${encodeURIComponent(key)}/unlock`, { password }));
}

/** Backend: GET /api/v1/docs/public-spaces/:sid — no authentication.
 * Addressed by id, not slug: a slug is unique per tenant and a visitor has
 * no tenant. */
export async function visitPublicSpace(spaceId: string): Promise<PublicSpaceView> {
  return unwrap<PublicSpaceView>(await get(`${base}/public-spaces/${encodeURIComponent(spaceId)}`));
}

/** Backend: GET /api/v1/docs/public-spaces/:sid/pages/:short — no authentication. */
export async function visitPublicSpacePage(spaceId: string, shortId: string): Promise<SharedPage> {
  return unwrap<SharedPage>(
    await get(`${base}/public-spaces/${encodeURIComponent(spaceId)}/pages/${encodeURIComponent(shortId)}`),
  );
}

// ---- page-level permissions -------------------------------------------------

/**
 * One row of a page's permission list.
 *
 * `role` is what was granted; `effective` is what it actually does once the
 * principal's space role has been applied. They differ whenever somebody is
 * granted more than the space gives them, because a grant is a ceiling and
 * never a promotion.
 */
export interface GrantView {
  principal_type: "user" | "group";
  principal_id: string;
  role: SpaceRole;
  effective: SpaceRole;
  /** False when the principal has no role in the space, making the grant inert. */
  in_space: boolean;
  name: string;
  email?: string;
  avatar?: string;
  is_default_group?: boolean;
  group_member_count?: number;
  added_by?: string;
  created_at: string;
}

/** A page in the permission chain. `visible` false means the caller may not
 * open it, and its title is then withheld. */
export interface AncestorRef {
  id: string;
  short_id: string;
  title: string;
  visible: boolean;
}

export interface PageAccessView {
  page_id: string;
  /** Whether this page itself cuts inheritance. */
  restricted: boolean;
  /** Restricted ancestors, nearest last: a page can be narrowed by a level
   * above it that this panel does not manage. */
  inherited_from: AncestorRef[];
  grants: GrantView[];
  can_manage: boolean;
  space_default: SpaceRole;
}

/** The short answer to "what may I do here", for drawing controls. */
export interface EffectivePermission {
  page_id: string;
  role: SpaceRole;
  can_edit: boolean;
  can_comment: boolean;
  can_manage_access: boolean;
  restricted: boolean;
  restricted_here: boolean;
}

/** Backend: GET /api/v1/docs/pages/:pid/access (page reader). */
export async function getPageAccess(pageId: string): Promise<PageAccessView> {
  return unwrap<PageAccessView>(await get(`${base}/pages/${encodeURIComponent(pageId)}/access`));
}

/**
 * Backend: PUT /api/v1/docs/pages/:pid/access (page admin).
 *
 * `restricted: false` restores inheritance AND drops every grant on the page.
 */
export async function setPageRestricted(pageId: string, restricted: boolean): Promise<PageAccessView> {
  return unwrap<PageAccessView>(await put(`${base}/pages/${encodeURIComponent(pageId)}/access`, { restricted }));
}

/** Backend: POST /api/v1/docs/pages/:pid/grants (page admin). */
export async function addPageGrant(
  pageId: string,
  body: { principal_type: "user" | "group"; principal_id: string; role: SpaceRole },
): Promise<PageAccessView> {
  return unwrap<PageAccessView>(await post(`${base}/pages/${encodeURIComponent(pageId)}/grants`, body));
}

/** Backend: DELETE /api/v1/docs/pages/:pid/grants/:ptype/:principal (page admin). */
export async function removePageGrant(
  pageId: string,
  principalType: "user" | "group",
  principalId: string,
): Promise<PageAccessView> {
  return unwrap<PageAccessView>(
    await del(
      `${base}/pages/${encodeURIComponent(pageId)}/grants/` +
        `${encodeURIComponent(principalType)}/${encodeURIComponent(principalId)}`,
    ),
  );
}

/** Backend: GET /api/v1/docs/pages/:pid/effective-permission (page reader). */
export async function getEffectivePermission(pageId: string): Promise<EffectivePermission> {
  return unwrap<EffectivePermission>(await get(`${base}/pages/${encodeURIComponent(pageId)}/effective-permission`));
}

// ---- labels, favourites and the space home ----------------------------------

/** A space's label. */
export interface LabelView {
  id: string;
  space_id: string;
  name: string;
  color: string;
  page_count: number;
}

/** What a space's landing page shows. */
export interface SpaceHome {
  recently_edited: TreeNode[];
  labels: LabelView[];
  favourites: TreeNode[];
}

/** The colours a label may take; the server refuses anything else. */
export const LABEL_COLORS = ["gray", "red", "orange", "yellow", "green", "teal", "blue", "purple", "pink"] as const;

export type LabelColor = (typeof LABEL_COLORS)[number];

/** Backend: GET /api/v1/docs/spaces/:sid/labels (space reader). */
export async function listLabels(spaceId: string): Promise<LabelView[]> {
  return unwrap<LabelView[] | null>(await get(`${base}/spaces/${encodeURIComponent(spaceId)}/labels`)) ?? [];
}

/** Backend: POST /api/v1/docs/spaces/:sid/labels (space writer). */
export async function createLabel(spaceId: string, body: { name: string; color?: string }): Promise<LabelView> {
  return unwrap<LabelView>(await post(`${base}/spaces/${encodeURIComponent(spaceId)}/labels`, body));
}

/** Backend: PATCH /api/v1/docs/spaces/:sid/labels/:lid (space writer). */
export async function updateLabel(
  spaceId: string,
  labelId: string,
  body: { name?: string; color?: string },
): Promise<LabelView> {
  return unwrap<LabelView>(
    await patch(`${base}/spaces/${encodeURIComponent(spaceId)}/labels/${encodeURIComponent(labelId)}`, body),
  );
}

/** Backend: DELETE /api/v1/docs/spaces/:sid/labels/:lid (space admin). */
export async function deleteLabel(spaceId: string, labelId: string): Promise<void> {
  await del(`${base}/spaces/${encodeURIComponent(spaceId)}/labels/${encodeURIComponent(labelId)}`);
}

/** Backend: PUT /api/v1/docs/pages/:pid/labels. The list replaces what was there. */
export async function setPageLabels(pageId: string, labelIds: string[]): Promise<LabelView[]> {
  return (
    unwrap<LabelView[] | null>(
      await put(`${base}/pages/${encodeURIComponent(pageId)}/labels`, { label_ids: labelIds }),
    ) ?? []
  );
}

/** Backend: GET /api/v1/docs/spaces/:sid/home (space reader). */
export async function getSpaceHome(spaceId: string): Promise<SpaceHome> {
  return (
    unwrap<SpaceHome | null>(await get(`${base}/spaces/${encodeURIComponent(spaceId)}/home`)) ?? {
      recently_edited: [],
      labels: [],
      favourites: [],
    }
  );
}

/**
 * Backend: GET /api/v1/docs/spaces/:sid/pages-by-label (space reader).
 * Several labels mean pages carrying all of them.
 */
export async function pagesWithLabels(spaceId: string, labelIds: string[], limit?: number): Promise<TreeNode[]> {
  const query = new URLSearchParams({ labels: labelIds.join(",") });
  if (limit) query.set("limit", String(limit));
  return (
    unwrap<TreeNode[] | null>(await get(`${base}/spaces/${encodeURIComponent(spaceId)}/pages-by-label?${query}`)) ?? []
  );
}

/** Backend: PUT /api/v1/docs/pages/:pid/favourite (page reader). */
export async function setFavourite(pageId: string, favourite: boolean): Promise<void> {
  await put(`${base}/pages/${encodeURIComponent(pageId)}/favourite`, { favourite });
}

/** Backend: GET /api/v1/docs/favourites (tenant member). */
export async function listFavourites(): Promise<TreeNode[]> {
  return unwrap<TreeNode[] | null>(await get(`${base}/favourites`)) ?? [];
}

// ---- watching and notifications ---------------------------------------------

/** How somebody is related to a page. */
export interface WatchView {
  page_id: string;
  /** manual | author | comment | mention */
  reason?: string;
  muted: boolean;
  watched: boolean;
}

/** One entry in somebody's inbox. */
export interface NotificationView {
  id: string;
  /** comment | mention | page_updated | access_granted */
  kind: string;
  page_id?: string;
  space_id?: string;
  comment_id?: string;
  actor_id?: string;
  actor?: { user_id: string; username?: string; email?: string; avatar?: string };
  payload: Record<string, unknown>;
  read_at?: string;
  created_at: string;
}

export interface NotificationPage {
  items: NotificationView[];
  unread: number;
  next_cursor?: string;
}

/** Backend: GET /api/v1/docs/pages/:pid/watch (page reader). */
export async function getWatchState(pageId: string): Promise<WatchView> {
  return unwrap<WatchView>(await get(`${base}/pages/${encodeURIComponent(pageId)}/watch`));
}

/** Backend: PUT /api/v1/docs/pages/:pid/watch (page reader). */
export async function setWatch(pageId: string, watching: boolean): Promise<WatchView> {
  return unwrap<WatchView>(await put(`${base}/pages/${encodeURIComponent(pageId)}/watch`, { watching }));
}

/** Backend: PUT /api/v1/docs/pages/:pid/mute (page reader). */
export async function setMuted(pageId: string, muted: boolean): Promise<WatchView> {
  return unwrap<WatchView>(await put(`${base}/pages/${encodeURIComponent(pageId)}/mute`, { muted }));
}

/** Backend: GET /api/v1/docs/notifications (tenant member). */
export async function listNotifications(
  params: { unread?: boolean; cursor?: string; limit?: number } = {},
): Promise<NotificationPage> {
  const query = new URLSearchParams();
  if (params.unread) query.set("unread", "true");
  if (params.cursor) query.set("cursor", params.cursor);
  if (params.limit) query.set("limit", String(params.limit));
  const suffix = query.toString() ? `?${query}` : "";
  return unwrap<NotificationPage | null>(await get(`${base}/notifications${suffix}`)) ?? { items: [], unread: 0 };
}

/** Backend: POST /api/v1/docs/notifications/read. An empty list means all. */
export async function markNotificationsRead(ids: string[] = []): Promise<void> {
  await post(`${base}/notifications/read`, { ids });
}

/** Backend: POST /api/v1/docs/notifications/archive. An empty list means all. */
export async function archiveNotifications(ids: string[] = []): Promise<void> {
  await post(`${base}/notifications/archive`, { ids });
}

// ---- comments ---------------------------------------------------------------

/** One comment as the server returns it. */
export interface CommentView {
  id: string;
  page_id: string;
  parent_id?: string;
  /** A ProseMirror document — a small subset of the page schema. */
  body: unknown;
  /** The editor's Yjs relative position; absent for a page-level comment. */
  anchor?: unknown;
  quoted_text?: string;
  placement: "inline" | "page";
  creator: { user_id: string; username?: string; email?: string; avatar?: string };
  creator_id: string;
  created_at: string;
  edited_at?: string;
  resolved_at?: string;
  resolved_by?: string;
  resolved_user?: { user_id: string; username?: string; email?: string; avatar?: string };
  replies?: CommentView[];
  /** What this caller may do, decided server-side so the client does not
   * reimplement the rules and disagree with them. */
  can_edit: boolean;
  can_delete: boolean;
  can_resolve: boolean;
}

export interface CommentList {
  items: CommentView[];
  open: number;
  total: number;
}

export interface CreateCommentBody {
  body: unknown;
  anchor?: unknown;
  quoted_text?: string;
  parent_id?: string;
}

/** Backend: GET /api/v1/docs/pages/:pid/comments (page reader). */
export async function listComments(pageId: string, includeResolved = false): Promise<CommentList> {
  const suffix = includeResolved ? "?resolved=true" : "";
  return (
    unwrap<CommentList | null>(await get(`${base}/pages/${encodeURIComponent(pageId)}/comments${suffix}`)) ?? {
      items: [],
      open: 0,
      total: 0,
    }
  );
}

/**
 * Backend: POST /api/v1/docs/pages/:pid/comments (page reader).
 * A reader may comment: commenting is not editing.
 */
export async function createComment(pageId: string, body: CreateCommentBody): Promise<CommentView> {
  return unwrap<CommentView>(await post(`${base}/pages/${encodeURIComponent(pageId)}/comments`, body));
}

/** Backend: PATCH /api/v1/docs/pages/:pid/comments/:cid (the author only). */
export async function updateComment(pageId: string, commentId: string, body: unknown): Promise<CommentView> {
  return unwrap<CommentView>(
    await patch(`${base}/pages/${encodeURIComponent(pageId)}/comments/${encodeURIComponent(commentId)}`, { body }),
  );
}

/** Backend: POST /api/v1/docs/pages/:pid/comments/:cid/resolve (page writer or author). */
export async function resolveComment(pageId: string, commentId: string, resolved: boolean): Promise<CommentView> {
  return unwrap<CommentView>(
    await post(`${base}/pages/${encodeURIComponent(pageId)}/comments/${encodeURIComponent(commentId)}/resolve`, {
      resolved,
    }),
  );
}

/** Backend: DELETE /api/v1/docs/pages/:pid/comments/:cid (the author or a space admin). */
export async function deleteComment(pageId: string, commentId: string): Promise<void> {
  await del(`${base}/pages/${encodeURIComponent(pageId)}/comments/${encodeURIComponent(commentId)}`);
}

// ---- page history -----------------------------------------------------------

/** One entry in a page's history. Never carries the body. */
export interface RevisionView {
  id: string;
  page_id: string;
  version: number;
  title: string;
  icon?: string;
  /** interval | publish | restore | import | manual */
  reason: string;
  editor_ids: string[];
  editors?: { user_id: string; username?: string; email?: string; avatar?: string }[];
  created_by?: string;
  created_at: string;
  word_count: number;
}

export interface RevisionPage {
  items: RevisionView[];
  next_cursor?: string;
}

export interface RevisionDetail extends RevisionView {
  content: unknown;
}

/** One line of a text comparison. */
export interface DiffLineView {
  kind: "same" | "added" | "removed";
  text: string;
  old_line?: number;
  new_line?: number;
}

/** One block of a structural comparison. */
export interface DiffBlockView {
  block_id: string;
  kind: "same" | "added" | "removed" | "moved" | "changed";
  type: string;
  text?: string;
  old_index: number;
  new_index: number;
}

export interface DiffSummaryView {
  added: number;
  removed: number;
  changed: number;
  moved: number;
  unchanged: number;
}

export interface DiffView {
  from?: RevisionView;
  /** Absent when the newer side is the page as it stands now. */
  to?: RevisionView;
  lines: DiffLineView[];
  blocks: DiffBlockView[];
  line_summary: DiffSummaryView;
  block_summary: DiffSummaryView;
}

/** Backend: GET /api/v1/docs/pages/:pid/revisions (page reader). */
export async function listRevisions(
  pageId: string,
  params: { cursor?: string; limit?: number } = {},
): Promise<RevisionPage> {
  const query = new URLSearchParams();
  if (params.cursor) query.set("cursor", params.cursor);
  if (params.limit) query.set("limit", String(params.limit));
  const suffix = query.toString() ? `?${query}` : "";
  return (
    unwrap<RevisionPage | null>(await get(`${base}/pages/${encodeURIComponent(pageId)}/revisions${suffix}`)) ?? {
      items: [],
    }
  );
}

/** Backend: GET /api/v1/docs/pages/:pid/revisions/:rid (page reader). */
export async function getRevision(pageId: string, revisionId: string): Promise<RevisionDetail> {
  return unwrap<RevisionDetail>(
    await get(`${base}/pages/${encodeURIComponent(pageId)}/revisions/${encodeURIComponent(revisionId)}`),
  );
}

/**
 * Backend: GET /api/v1/docs/pages/:pid/revisions/:rid/diff (page reader).
 * Omitting `to` compares against the page as it stands now.
 */
export async function getRevisionDiff(pageId: string, revisionId: string, to?: string): Promise<DiffView> {
  const suffix = to ? `?to=${encodeURIComponent(to)}` : "";
  return unwrap<DiffView>(
    await get(`${base}/pages/${encodeURIComponent(pageId)}/revisions/${encodeURIComponent(revisionId)}/diff${suffix}`),
  );
}

/** Backend: POST /api/v1/docs/pages/:pid/revisions/:rid/restore (page writer). */
export async function restoreRevision(pageId: string, revisionId: string): Promise<{ ydoc_version: number }> {
  return unwrap<{ ydoc_version: number }>(
    await post(`${base}/pages/${encodeURIComponent(pageId)}/revisions/${encodeURIComponent(revisionId)}/restore`, {}),
  );
}

/** One block reference, as the server addresses it. */
export interface BlockRefRequest {
  source_page_id: string;
  source_block_id: string;
}

/** One resolved block reference. */
export interface BlockRefView extends BlockRefRequest {
  /** 'ok' carries content; 'missing' covers both deleted and not-permitted;
   * 'pending' means the source page has not been saved since the reference
   * was recorded, so there is nothing to show yet. */
  state: "ok" | "missing" | "pending";
  content?: unknown;
  title?: string;
  icon?: string;
  source_short_id?: string;
}

/**
 * Backend: POST /api/v1/docs/block-refs/resolve (tenant member).
 * A POST that only reads, like the title lookup: the list is as long as the
 * open page has references. Each source page's permissions are checked
 * server-side, under the reader's own identity rather than the author's.
 */
export async function resolveBlockRefs(refs: BlockRefRequest[]): Promise<BlockRefView[]> {
  return unwrap<BlockRefView[] | null>(await post(`${base}/block-refs/resolve`, { refs })) ?? [];
}

/** Backend: GET /api/v1/docs/page-links/suggest (tenant member, filtered by permission). */
export async function suggestPages(params: { q?: string; space?: string; limit?: number } = {}): Promise<PageRef[]> {
  const query = new URLSearchParams();
  if (params.q) query.set("q", params.q);
  if (params.space) query.set("space", params.space);
  if (params.limit) query.set("limit", String(params.limit));
  const suffix = query.toString() ? `?${query}` : "";
  return unwrap<PageRef[] | null>(await get(`${base}/page-links/suggest${suffix}`)) ?? [];
}

/** Backend: GET /api/v1/docs/pages/:pid/mention-candidates (page reader). */
export async function suggestMentions(
  pageId: string,
  params: { q?: string; limit?: number } = {},
): Promise<MentionCandidate[]> {
  const query = new URLSearchParams();
  if (params.q) query.set("q", params.q);
  if (params.limit) query.set("limit", String(params.limit));
  const suffix = query.toString() ? `?${query}` : "";
  return (
    unwrap<MentionCandidate[] | null>(
      await get(`${base}/pages/${encodeURIComponent(pageId)}/mention-candidates${suffix}`),
    ) ?? []
  );
}

// ---- embeds ---------------------------------------------------------------

export interface ResolvedEmbedView {
  provider: string;
  url: string;
  /** What to put in the iframe. Derived by the server on every call, so a
   * narrowed allow-list takes effect without rewriting any document. */
  embed_url: string;
  title?: string;
  author?: string;
  aspect_ratio?: number;
}

export interface EmbedPolicyView {
  providers: string[];
  /** The self-hosted draw.io editor; empty means diagrams are read-only. */
  drawio_url?: string;
}

/** Backend: GET /api/v1/docs/embeds/policy (tenant member). */
export async function getEmbedPolicy(): Promise<EmbedPolicyView> {
  return unwrap<EmbedPolicyView>(await get(`${base}/embeds/policy`));
}

/**
 * Backend: POST /api/v1/docs/embeds/resolve (tenant member).
 * Rejects with 400 for an address this deployment does not allow.
 */
export async function resolveEmbed(url: string): Promise<ResolvedEmbedView> {
  return unwrap<ResolvedEmbedView>(await post(`${base}/embeds/resolve`, { url }));
}

// ---- export ---------------------------------------------------------------

/** How an export is rendered. */
export type ExportFormat = "markdown" | "html";

/** An asynchronous space export, as returned by the job endpoints. */
export interface ExportJobView {
  id: string;
  space_id: string;
  format: string;
  /** pending | running | succeeded | partial | failed */
  status: string;
  file_name?: string;
  /** Pages written into the archive. */
  exported: number;
  /** Pages left out because the requester cannot read them. */
  skipped: number;
  error?: string;
  /** True once the archive can be downloaded. */
  ready: boolean;
  created_at: string;
  finished_at?: string;
  expires_at?: string;
}

/**
 * Backend: POST /api/v1/docs/pages/:pid/export (page reader).
 *
 * Synchronous: the file comes back on this request, and is handed straight to
 * the browser's save dialog.
 */
export async function exportPage(pageId: string, format: ExportFormat = "markdown"): Promise<void> {
  const file = await postDownload(
    `${base}/pages/${encodeURIComponent(pageId)}/export`,
    { format },
    `page.${format === "html" ? "html" : "md"}`,
  );
  saveBlob(file);
}

/**
 * Backend: POST /api/v1/docs/spaces/:sid/export (space reader).
 *
 * Asynchronous: this starts a job. Poll it with getExportJob until `ready`,
 * then call downloadExport. The archive contains only the pages the caller
 * could read, so only they can download it, and it is deleted after a day.
 */
export async function startSpaceExport(spaceId: string, format: ExportFormat = "markdown"): Promise<ExportJobView> {
  return unwrap<ExportJobView>(await post(`${base}/spaces/${encodeURIComponent(spaceId)}/export`, { format }));
}

/** Backend: GET /api/v1/docs/exports/:jid (the person who started it). */
export async function getExportJob(jobId: string): Promise<ExportJobView> {
  return unwrap<ExportJobView>(await get(`${base}/exports/${encodeURIComponent(jobId)}`));
}

/** Backend: GET /api/v1/docs/exports/:jid/download. Saves the archive. */
export async function downloadExport(jobId: string, fallbackName = "export.zip"): Promise<void> {
  const file: DownloadedFile = await getDownload(`${base}/exports/${encodeURIComponent(jobId)}/download`, fallbackName);
  saveBlob(file);
}

// ---- import ---------------------------------------------------------------

/** An asynchronous import, as returned by the job endpoints. */
export interface ImportJobView {
  id: string;
  space_id: string;
  kind: string;
  /** pending | running | succeeded | partial | failed */
  status: string;
  file_name?: string;
  target_parent_id?: string;
  /** Pages that now exist. */
  created: number;
  /** Files stored alongside them. */
  attachments: number;
  /** What was left out, and why. */
  skipped?: string[];
  error?: string;
  /** True once the job has stopped, whatever the outcome. */
  done: boolean;
  created_at: string;
  finished_at?: string;
}

/**
 * Backend: POST /api/v1/docs/spaces/:sid/imports (space writer).
 *
 * Takes a .md file or a .zip of them. Asynchronous: poll with getImportJob
 * until `done`. A file the importer cannot read does not fail the whole
 * import; it is named in `skipped`.
 */
export async function startImport(spaceId: string, file: File, parentId?: string): Promise<ImportJobView> {
  const form = new FormData();
  form.append("file", file);
  if (parentId) form.append("parent_id", parentId);
  return unwrap<ImportJobView>(await postUpload(`${base}/spaces/${encodeURIComponent(spaceId)}/imports`, form));
}

/** Backend: GET /api/v1/docs/imports/:jid (anybody who can read the space). */
export async function getImportJob(jobId: string): Promise<ImportJobView> {
  return unwrap<ImportJobView>(await get(`${base}/imports/${encodeURIComponent(jobId)}`));
}
