import { get, post, put } from "@/utils/request";

import type { PersonRef } from "@/api/findings";

// Stewardship of a knowledge entry: who maintains it — the person knowledge
// health takes its problems to — and when somebody last vouched for it. It is
// not a permission; who may change the entry is decided as before.
//
// A docs page's mirror entry takes its maintainer from the page, so its owner
// is changed on the page (setPageOwner in the docs API), not here.

/** Where an entry's content is maintained. */
export type KnowledgeOrigin = "local" | "docs" | "synced";

export interface KnowledgeStewardship {
  knowledge_id: string;
  origin: KnowledgeOrigin;
  owner: PersonRef | null;
  /** False for a docs mirror, whose owner is its page's. */
  owner_editable: boolean;
  reviewed_at: string | null;
  reviewed_by: PersonRef | null;
  /** The knowledge base's review period in days; 0 when there is none. */
  review_interval_days: number;
  review_due_at: string | null;
  overdue: boolean;
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

const base = (knowledgeId: string) => `/api/v1/knowledge/${encodeURIComponent(knowledgeId)}`;

/** Backend: GET /api/v1/knowledge/:id/stewardship. */
export async function getStewardship(knowledgeId: string): Promise<KnowledgeStewardship> {
  return unwrap<KnowledgeStewardship>(await get(`${base(knowledgeId)}/stewardship`));
}

/** Backend: PUT /api/v1/knowledge/:id/owner. An empty owner leaves the entry without one. */
export async function setKnowledgeOwner(knowledgeId: string, ownerId: string): Promise<KnowledgeStewardship> {
  return unwrap<KnowledgeStewardship>(await put(`${base(knowledgeId)}/owner`, { owner_id: ownerId }));
}

/** Backend: POST /api/v1/knowledge/:id/review — the caller vouches for the entry as it stands. */
export async function confirmKnowledgeReviewed(knowledgeId: string): Promise<KnowledgeStewardship> {
  return unwrap<KnowledgeStewardship>(await post(`${base(knowledgeId)}/review`, {}));
}
