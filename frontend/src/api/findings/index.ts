import { get, patch, post, put } from "@/utils/request";

// Knowledge health ("findings") API. When content in a knowledge base changes,
// the backend runs detectors over it and records what they report as findings;
// the open-source detector compares the documents of a knowledge base and
// reports word-for-word copies ("duplicate") and near-copies that differ
// ("divergent"). Each finding is routed to the person who should act on it. Every function here unwraps Yuheng's
// `{ success, data }` envelope and rejects with the server message when
// success is false, the same contract as the docs API module, so callers only
// deal with data.

/** Finding types this build has a label for. The type field stays a plain
 * string because other editions add detectors (e.g. "contradiction") that an
 * older frontend must still be able to list, under their raw name. */
export type KnownFindingType = "duplicate" | "divergent";
export type FindingSeverity = "info" | "warning" | "error";
export type FindingStatus = "open" | "dismissed" | "resolved";
/** What the list can be filtered by: one status, or every status at once. */
export type FindingStatusFilter = FindingStatus | "all";
/** Why a person dismissed a finding. */
export type FindingDismissReason = "distinct_scope" | "intentional";
/** Why a closed finding was closed: a dismissal's reason, or "cleared" when
 * the content changed and the detectors stopped reporting it. */
export type FindingResolution = FindingDismissReason | "cleared";

/** A person named in a response. */
export interface PersonRef {
  id: string;
  username: string;
  avatar?: string;
  /** False for somebody who left the workspace or whose account is off. */
  active: boolean;
}

export interface FindingEvidence {
  subject_chunk_id: string;
  subject_excerpt: string;
  related_chunk_id: string;
  related_excerpt: string;
  /** Similarity of this pair of passages, 0..1. */
  score: number;
  /** The passages differ inside the text they share; the excerpts are then
   * centred on the first difference. */
  differs?: boolean;
}

export interface FindingDocumentRef {
  knowledge_id: string;
  title: string;
}

export interface Finding {
  id: string;
  knowledge_base_id: string;
  type: string;
  detector: string;
  severity: FindingSeverity;
  status: FindingStatus;
  /** Overall similarity, 0..1. */
  score: number;
  /** Share of the subject document covered by the overlap, 0..1. */
  overlap_ratio: number;
  subject: FindingDocumentRef;
  /** The other document, for findings that are about a pair. */
  related: FindingDocumentRef | null;
  evidence: FindingEvidence[];
  created_at: string;
  updated_at: string;
  resolved_at: string | null;
  resolved_by: string | null;
  resolution?: FindingResolution | string | null;
  /** Who the finding is taken to; null when nobody could be found. */
  assignee?: PersonRef | null;
  /** A person chose the assignee; checks leave the choice alone. */
  assigned_manually?: boolean;
  /** Set in listings that span knowledge bases. */
  knowledge_base_name?: string;
}

export interface FindingPage {
  items: Finding[];
  total: number;
  page: number;
  page_size: number;
}

export interface FindingsSummary {
  open_total: number;
  open_by_type: Record<string, number>;
  last_scan_at: string | null;
  /** Whether health checks run for this knowledge base at all. */
  enabled: boolean;
  /** Whether the knowledge base's retrieval engine can answer similarity
   * lookups, which duplicate detection is built on. */
  supported: boolean;
}

export interface ListFindingsParams {
  status?: FindingStatusFilter;
  type?: string;
  knowledge_id?: string;
  /** Only the findings assigned to the caller. */
  mine?: boolean;
  page?: number;
  page_size?: number;
}

export interface ScanResult {
  /** How many documents were queued to be checked again. */
  queued: number;
}

/** A page elsewhere in the space's knowledge base that this page overlaps. */
export interface RelatedDocsPage {
  id: string;
  short_id: string;
  title: string;
  space_slug: string;
}

export interface PageFinding {
  id: string;
  type: string;
  severity: FindingSeverity;
  score: number;
  overlap_ratio: number;
  related_page: RelatedDocsPage;
  evidence: FindingEvidence[];
  assignee?: PersonRef | null;
}

export interface PageFindings {
  items: PageFinding[];
  /** Overlapping documents in the knowledge base that are not pages the
   * caller may open (uploaded files, pages in restricted spaces). They are
   * counted and never named. */
  other_count: number;
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

const kbBase = (kbId: string) => `/api/v1/knowledge-bases/${encodeURIComponent(kbId)}/findings`;

/**
 * The query string for a findings listing. Empty values are left out rather
 * than sent blank, so the server's defaults (status=open, page 1, 20 a page)
 * apply to whatever the caller did not choose.
 */
export function buildFindingsQuery(params: ListFindingsParams = {}): string {
  const query = new URLSearchParams();
  if (params.status) query.set("status", params.status);
  if (params.type) query.set("type", params.type);
  if (params.knowledge_id) query.set("knowledge_id", params.knowledge_id);
  if (params.mine) query.set("assignee", "me");
  if (params.page && params.page > 0) query.set("page", String(params.page));
  if (params.page_size && params.page_size > 0) query.set("page_size", String(params.page_size));
  const qs = query.toString();
  return qs ? `?${qs}` : "";
}

/** Backend: GET /api/v1/knowledge-bases/:id/findings. */
export async function listFindings(kbId: string, params: ListFindingsParams = {}): Promise<FindingPage> {
  const page = unwrap<FindingPage | null>(await get(`${kbBase(kbId)}${buildFindingsQuery(params)}`));
  return {
    items: page?.items ?? [],
    total: page?.total ?? 0,
    page: page?.page ?? params.page ?? 1,
    page_size: page?.page_size ?? params.page_size ?? 20,
  };
}

/** Backend: GET /api/v1/knowledge-bases/:id/findings/summary. */
export async function getFindingsSummary(kbId: string): Promise<FindingsSummary> {
  const s = unwrap<FindingsSummary | null>(await get(`${kbBase(kbId)}/summary`));
  return {
    open_total: s?.open_total ?? 0,
    open_by_type: s?.open_by_type ?? {},
    last_scan_at: s?.last_scan_at ?? null,
    enabled: s?.enabled ?? false,
    supported: s?.supported ?? false,
  };
}

/** Backend: PATCH /api/v1/knowledge-bases/:id/findings/:finding_id. Dismissing (which needs a
 * reason) and reopening are the only transitions a person makes; "resolved" is the detector's. */
export async function updateFindingStatus(
  kbId: string,
  findingId: string,
  status: "dismissed" | "open",
  reason?: FindingDismissReason,
): Promise<Finding> {
  const body = status === "dismissed" ? { status, reason } : { status };
  return unwrap<Finding>(await patch(`${kbBase(kbId)}/${encodeURIComponent(findingId)}`, body));
}

/** Backend: PUT /api/v1/knowledge-bases/:id/findings/:finding_id/assignee. An empty assignee hands
 * the finding back to the automatic routing. */
export async function assignFinding(kbId: string, findingId: string, assigneeId: string): Promise<Finding> {
  return unwrap<Finding>(
    await put(`${kbBase(kbId)}/${encodeURIComponent(findingId)}/assignee`, { assignee_id: assigneeId }),
  );
}

/** How a superseded document left its knowledge base: an upload is deleted; a docs page is excluded
 * from the knowledge base and marked superseded, and stays readable. */
export type RetiredHow = "deleted" | "excluded";

export interface SupersedeResult {
  retired_knowledge_id: string;
  how: RetiredHow;
}

/** Backend: POST /api/v1/knowledge-bases/:id/findings/:finding_id/supersede. Keeps one of the
 * finding's two documents and takes the other out of the knowledge base. */
export async function supersedeFinding(
  kbId: string,
  findingId: string,
  keepKnowledgeId: string,
): Promise<SupersedeResult> {
  return unwrap<SupersedeResult>(
    await post(`${kbBase(kbId)}/${encodeURIComponent(findingId)}/supersede`, { keep_knowledge_id: keepKnowledgeId }),
  );
}

/** Backend: POST /api/v1/docs/pages/:pid/supersede — the page stays, the other page of the
 * finding leaves the knowledge base. Needs write access to both pages. */
export async function supersedeFromPage(pageId: string, findingId: string): Promise<SupersedeResult> {
  return unwrap<SupersedeResult>(
    await post(`/api/v1/docs/pages/${encodeURIComponent(pageId)}/supersede`, { finding_id: findingId }),
  );
}

/** Backend: GET /api/v1/findings/assigned — the caller's findings across the workspace. */
export async function listAssignedFindings(
  params: Pick<ListFindingsParams, "status" | "page" | "page_size"> = {},
): Promise<FindingPage> {
  const page = unwrap<FindingPage | null>(await get(`/api/v1/findings/assigned${buildFindingsQuery(params)}`));
  return {
    items: page?.items ?? [],
    total: page?.total ?? 0,
    page: page?.page ?? params.page ?? 1,
    page_size: page?.page_size ?? params.page_size ?? 20,
  };
}

/** Backend: GET /api/v1/findings/assigned/count — the caller's open findings. */
export async function countAssignedFindings(): Promise<number> {
  const res = unwrap<{ open_total?: number } | null>(await get("/api/v1/findings/assigned/count"));
  return res?.open_total ?? 0;
}

/** Backend: POST /api/v1/knowledge-bases/:id/findings/scan (knowledge-base administrators). */
export async function scanFindings(kbId: string): Promise<ScanResult> {
  const res = unwrap<ScanResult | null>(await post(`${kbBase(kbId)}/scan`, {}));
  return { queued: res?.queued ?? 0 };
}

/** Backend: GET /api/v1/docs/pages/:pid/findings (page reader). */
export async function listPageFindings(pageId: string): Promise<PageFindings> {
  const res = unwrap<PageFindings | null>(await get(`/api/v1/docs/pages/${encodeURIComponent(pageId)}/findings`));
  return { items: res?.items ?? [], other_count: res?.other_count ?? 0 };
}
