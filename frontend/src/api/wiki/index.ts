import { get, post, put, del } from "../../utils/request";

// encodeSlugPath encodes each segment of a hierarchical wiki slug (e.g.
// "foo/bar baz?") so the URL is safe while preserving the "/" separators
// between segments. Using encodeURIComponent on the whole slug would also
// escape the "/" and break hierarchical routing on the backend.
function encodeSlugPath(slug: string): string {
  return slug.split("/").map(encodeURIComponent).join("/");
}

// Wiki Page Types
export interface WikiPage {
  id: string;
  tenant_id: number;
  knowledge_base_id: string;
  slug: string;
  title: string;
  page_type: string;
  status: string;
  content: string;
  summary: string;
  aliases: string[];
  parent_slug?: string;
  category_path?: string[];
  wiki_path?: string;
  depth?: number;
  sort_order?: number;
  source_refs: string[];
  in_links: string[];
  out_links: string[];
  page_metadata: Record<string, any>;
  version: number;
  // Author kind of the current version: 'pipeline' | 'agent' | 'user' |
  // 'revert'. Only the last three get a badge in the reader.
  last_edit_source?: string;
  last_editor_id?: string;
  created_at: string;
  updated_at: string;
}

export interface WikiPageListResponse {
  pages: WikiPage[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface WikiFolder {
  id: string;
  tenant_id: number;
  knowledge_base_id: string;
  parent_id: string;
  name: string;
  path: string;
  depth: number;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export interface WikiFolderNode extends WikiFolder {
  page_count: number;
  has_children: boolean;
}

export interface WikiFolderListResponse {
  parent_id: string;
  folders: WikiFolderNode[];
}

export interface WikiGraphMeta {
  mode: "overview" | "ego" | string;
  total: number;
  returned: number;
  truncated: boolean;
  center?: string;
  depth?: number;
  familiar_count?: number;
}

export interface WikiGraphData {
  nodes: { slug: string; title: string; page_type: string; link_count: number; familiar?: boolean }[];
  edges: { source: string; target: string }[];
  meta: WikiGraphMeta;
}

export interface WikiStats {
  total_pages: number;
  pages_by_type: Record<string, number>;
  total_links: number;
  orphan_count: number;
  recent_updates: WikiPage[];
  pending_tasks: number;
  is_active: boolean;
}

/** One structural problem the wiki lint found (internal/application/service/wiki_lint.go). */
export interface WikiLintIssue {
  type: "orphan_page" | "broken_link" | "stale_ref" | "missing_cross_ref" | "empty_content" | "duplicate_slug";
  severity: "info" | "warning" | "error";
  page_slug: string;
  /** The other page involved: a broken link's target, a missing cross-reference's entity. */
  target_slug?: string;
  description: string;
  auto_fixable: boolean;
}

/**
 * The wiki lint report. Computed from the pages on every request and stored
 * nowhere, so it is always current and costs a walk of the whole wiki: fetch
 * it when somebody asks for it, not on every page view.
 */
export interface WikiLintReport {
  knowledge_base_id: string;
  issues: WikiLintIssue[] | null;
  /** 0–100. */
  health_score: number;
  summary: string;
}

// Wiki API Functions. The wiki handlers answer with the resource itself, not
// the { success, data } envelope most other endpoints use, so every function
// here resolves to the bare response shape.
export function listWikiPages(
  kbId: string,
  params?: {
    page_type?: string;
    status?: string;
    query?: string;
    category_path?: string;
    category_depth?: number;
    page?: number;
    page_size?: number;
    sort_by?: string;
    sort_order?: string;
  },
) {
  const query = new URLSearchParams();
  if (params) {
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== "") {
        query.set(key, String(value));
      }
    });
  }
  const qs = query.toString();
  return get<WikiPageListResponse>(`/api/v1/knowledgebase/${kbId}/wiki/pages${qs ? "?" + qs : ""}`);
}

// listWikiFolders returns the direct child folders of parentId ("" = root),
// each enriched with a recursive page_count and a has_children flag so the tree
// can render expand affordances and empty folders without a second request.
// pageTypes scopes the view to a sidebar tab: only folders whose subtree holds
// a page of those types (or are entirely empty) come back, and page_count is
// counted within those types.
export function listWikiFolders(kbId: string, parentId = "", pageTypes = "") {
  const query = new URLSearchParams();
  if (parentId) query.set("parent_id", parentId);
  if (pageTypes) query.set("page_types", pageTypes);
  const qs = query.toString();
  return get<WikiFolderListResponse>(`/api/v1/knowledgebase/${kbId}/wiki/folders${qs ? "?" + qs : ""}`);
}

// createWikiFolder creates a new empty folder under parentId ("" = root).
export function createWikiFolder(kbId: string, parentId: string, name: string) {
  return post<WikiFolder>(`/api/v1/knowledgebase/${kbId}/wiki/folders`, { parent_id: parentId, name });
}

// updateWikiFolder renames and/or reparents a folder. Pass move_parent: true
// (and parent_id) to reparent; omit it for a pure rename.
export function updateWikiFolder(
  kbId: string,
  folderId: string,
  data: { name?: string; parent_id?: string; move_parent?: boolean },
) {
  return put<WikiFolder>(`/api/v1/knowledgebase/${kbId}/wiki/folders/${folderId}`, data);
}

// deleteWikiFolder removes an empty folder (no pages, no sub-folders).
export function deleteWikiFolder(kbId: string, folderId: string) {
  return del(`/api/v1/knowledgebase/${kbId}/wiki/folders/${folderId}`);
}

// moveWikiPage relocates a page into folderId ("" = root). The slug is sent in
// the body because wiki slugs are hierarchical.
export function moveWikiPage(kbId: string, slug: string, folderId: string) {
  return put<WikiPage>(`/api/v1/knowledgebase/${kbId}/wiki/move-page`, { slug, folder_id: folderId });
}

export function createWikiPage(kbId: string, data: Partial<WikiPage>) {
  return post<WikiPage>(`/api/v1/knowledgebase/${kbId}/wiki/pages`, data);
}

export function getWikiPage(kbId: string, slug: string) {
  return get<WikiPage>(`/api/v1/knowledgebase/${kbId}/wiki/pages/${encodeSlugPath(slug)}`);
}

// WikiPageUpdatePayload is a partial update: absent fields keep their stored
// value. `version` is the optimistic-lock guard — send the version the page
// had when the user started editing; the backend answers 409 (with
// `current_version` in the body) when someone else edited in between.
export interface WikiPageUpdatePayload {
  title?: string;
  content?: string;
  summary?: string;
  page_type?: string;
  status?: string;
  aliases?: string[];
  version?: number;
}

export function updateWikiPage(kbId: string, slug: string, data: WikiPageUpdatePayload) {
  return put<WikiPage>(`/api/v1/knowledgebase/${kbId}/wiki/pages/${encodeSlugPath(slug)}`, data);
}

export function deleteWikiPage(kbId: string, slug: string) {
  return del(`/api/v1/knowledgebase/${kbId}/wiki/pages/${encodeSlugPath(slug)}`);
}

// WikiPageRevision is one immutable snapshot of a superseded page version.
// `content` is only populated when fetching a single revision.
export interface WikiPageRevision {
  id: string;
  tenant_id: number;
  knowledge_base_id: string;
  page_id: string;
  slug: string;
  version: number;
  title: string;
  page_type: string;
  status: string;
  content?: string;
  summary: string;
  aliases: string[];
  edit_source: string;
  editor_id: string;
  edited_at: string;
  created_at: string;
}

export interface WikiRevisionListResponse {
  revisions: WikiPageRevision[];
  total: number;
  current_version: number;
}

// listWikiRevisions returns the page's historical snapshots newest-first
// (content omitted) plus the current version number. The current version has
// no revision row — it is the page itself.
export function listWikiRevisions(kbId: string, slug: string, params?: { limit?: number; offset?: number }) {
  const query = new URLSearchParams();
  if (params?.limit !== undefined) query.set("limit", String(params.limit));
  if (params?.offset !== undefined) query.set("offset", String(params.offset));
  const qs = query.toString();
  return get<WikiRevisionListResponse>(
    `/api/v1/knowledgebase/${kbId}/wiki/revisions/${encodeSlugPath(slug)}${qs ? "?" + qs : ""}`,
  );
}

// getWikiRevision returns one snapshot with full content.
export function getWikiRevision(kbId: string, slug: string, version: number) {
  return get<WikiPageRevision>(
    `/api/v1/knowledgebase/${kbId}/wiki/revisions/${encodeSlugPath(slug)}?version=${version}`,
  );
}

// revertWikiPage rolls the page back to a stored revision. Applied as a
// regular edit: the pre-revert state is snapshotted and version advances,
// so a revert is itself revertable.
export function revertWikiPage(kbId: string, slug: string, version: number) {
  return post<WikiPage>(`/api/v1/knowledgebase/${kbId}/wiki/revert`, { slug, version });
}

export interface WikiIndexEntryDTO {
  slug: string;
  title: string;
  summary: string;
  parent_slug?: string;
  category_path?: string[];
  wiki_path?: string;
  depth?: number;
  sort_order?: number;
}

export interface WikiIndexGroup {
  type: string;
  total: number;
  items: WikiIndexEntryDTO[];
  next_cursor?: string;
}

export interface WikiIndexResponse {
  intro: string;
  version: number;
  groups: WikiIndexGroup[];
}

// getWikiIndex fetches the structured index view for a wiki KB as
// { intro, groups }, so a 40k-page wiki does not round-trip multiple
// megabytes on every index open. Pass `types` to restrict which
// page_type buckets come back; `limit` bounds the per-group window;
// `cursor` resumes from a previous response.
export function getWikiIndex(kbId: string, params?: { types?: string[]; limit?: number; cursor?: string }) {
  const query = new URLSearchParams();
  if (params) {
    if (params.types && params.types.length > 0) query.set("types", params.types.join(","));
    if (params.limit !== undefined) query.set("limit", String(params.limit));
    if (params.cursor) query.set("cursor", params.cursor);
  }
  const qs = query.toString();
  const suffix = qs ? `?${qs}` : "";
  return get<WikiIndexResponse>(`/api/v1/knowledgebase/${kbId}/wiki/index${suffix}`);
}

export interface WikiGraphQueryParams {
  mode?: "overview" | "ego";
  center?: string;
  depth?: number;
  types?: string[];
  limit?: number;
}

// getWikiGraph fetches a slice of the wiki link graph. Without params the
// backend returns the top-500 most-connected pages (overview mode). Pass
// `mode: 'ego', center: <slug>` to drill into a specific page's neighborhood.
// For knowledge bases with tens of thousands of pages the overview cap is
// what prevents the browser from choking on a 30MB payload / 100k SVG nodes.
export function getWikiGraph(kbId: string, params?: WikiGraphQueryParams) {
  const query = new URLSearchParams();
  if (params) {
    if (params.mode) query.set("mode", params.mode);
    if (params.center) query.set("center", params.center);
    if (params.depth !== undefined) query.set("depth", String(params.depth));
    if (params.limit !== undefined) query.set("limit", String(params.limit));
    if (params.types && params.types.length > 0) {
      query.set("types", params.types.join(","));
    }
  }
  const qs = query.toString();
  return get<WikiGraphData>(`/api/v1/knowledgebase/${kbId}/wiki/graph${qs ? "?" + qs : ""}`);
}

export function getWikiStats(kbId: string) {
  return get<WikiStats>(`/api/v1/knowledgebase/${kbId}/wiki/stats`);
}

export function searchWikiPages(kbId: string, q: string, limit?: number) {
  const params = new URLSearchParams({ q });
  if (limit) params.set("limit", String(limit));
  return get<{ pages: WikiPage[] }>(`/api/v1/knowledgebase/${kbId}/wiki/search?${params.toString()}`);
}

/** Backend: GET /knowledgebase/:kb_id/wiki/lint (reader). Answers the report itself, not an envelope. */
export function lintWiki(kbId: string): Promise<WikiLintReport> {
  return get<WikiLintReport>(`/api/v1/knowledgebase/${kbId}/wiki/lint`);
}

/** Backend: POST /knowledgebase/:kb_id/wiki/auto-fix (KB editor). Fixes the auto-fixable lint issues. */
export function autoFixWiki(kbId: string): Promise<{ fixed: number }> {
  return post<{ fixed: number }>(`/api/v1/knowledgebase/${kbId}/wiki/auto-fix`, {});
}

export function rebuildWikiLinks(kbId: string) {
  return post(`/api/v1/knowledgebase/${kbId}/wiki/rebuild-links`, {});
}
