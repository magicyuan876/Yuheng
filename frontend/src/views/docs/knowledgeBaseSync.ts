import { EMBEDDING_MODEL_REQUIRED, type KnowledgeBaseChoice } from "@/api/docs";
import { listKnowledgeBases } from "@/api/knowledge-base";
import { listModels } from "@/api/model";
import { selectInitialModelId } from "@/utils/modelDefaults";

// Which knowledge bases a space may sync into, and whether a new one can be
// made for it. The server decides both (internal/docs/service/spacekb.go);
// this mirrors its rules so the form offers only what will be accepted, and
// explains up front what it cannot offer instead of failing on submit.

/** The fields of a knowledge base this module reads. */
export interface KnowledgeBaseSummary {
  id: string;
  name: string;
  type?: string;
  is_temporary?: boolean;
  creator_id?: string;
}

export interface SyncOptions {
  /** Every knowledge base of the workspace the caller can see, to name the bound one. */
  all: KnowledgeBaseSummary[];
  /** Those the caller may bind: see bindableKnowledgeBases. */
  bindable: KnowledgeBaseSummary[];
  /** Whether the workspace has an embedding model, without which none can be made. */
  canCreate: boolean;
}

/**
 * The knowledge bases a caller may bind. Only document knowledge bases hold
 * pages (an FAQ base indexes question-answer entries; a temporary one is
 * internal), and binding fills the knowledge base with the space's pages, so
 * it follows the knowledge base's own rule for adding documents: its creator,
 * or a workspace administrator.
 */
export function bindableKnowledgeBases(
  kbs: readonly KnowledgeBaseSummary[],
  caller: { userId: string; isAdmin: boolean },
): KnowledgeBaseSummary[] {
  return kbs.filter(
    (kb) =>
      (!kb.type || kb.type === "document") &&
      !kb.is_temporary &&
      (caller.isAdmin || (!!kb.creator_id && kb.creator_id === caller.userId)),
  );
}

function asSummaries(res: unknown): KnowledgeBaseSummary[] {
  const data = (res as { data?: unknown } | null)?.data;
  if (!Array.isArray(data)) return [];
  return data.filter(
    (kb): kb is KnowledgeBaseSummary =>
      typeof kb === "object" && kb !== null && typeof (kb as { id?: unknown }).id === "string",
  );
}

/**
 * Loads what the knowledge-base field needs. A failure of either request
 * degrades rather than throws: without the list nothing can be bound, without
 * the models nothing can be created, and the form says so.
 */
export async function loadSyncOptions(caller: { userId: string; isAdmin: boolean }): Promise<SyncOptions> {
  const [kbs, models] = await Promise.all([
    listKnowledgeBases().then(asSummaries, () => []),
    listModels("Embedding").catch(() => []),
  ]);
  return {
    all: kbs,
    bindable: bindableKnowledgeBases(kbs, caller),
    // The rule the server applies when it picks the model for the new
    // knowledge base: an active embedding model must exist.
    canCreate: selectInitialModelId(models, "Embedding") !== null,
  };
}

/** True when the server refused because the workspace has no embedding model. */
export function isEmbeddingModelRequired(err: unknown): boolean {
  return (err as { error?: { code?: unknown } } | null)?.error?.code === EMBEDDING_MODEL_REQUIRED;
}

/** Two choices bind the same thing. */
export function sameChoice(a: KnowledgeBaseChoice, b: KnowledgeBaseChoice): boolean {
  if (a.mode !== b.mode) return false;
  return a.mode !== "existing" || (a.id ?? "") === (b.id ?? "");
}

/** The choice that describes a space's current binding. */
export function currentChoice(knowledgeBaseId: string | null | undefined): KnowledgeBaseChoice {
  return knowledgeBaseId ? { mode: "existing", id: knowledgeBaseId } : { mode: "none" };
}

/** A choice the server accepts: an id only with "existing", and none without one. */
export function choiceRequest(choice: KnowledgeBaseChoice): KnowledgeBaseChoice {
  return choice.mode === "existing" ? { mode: "existing", id: choice.id ?? "" } : { mode: choice.mode };
}

/** A choice is complete when "existing" names a knowledge base. */
export function choiceComplete(choice: KnowledgeBaseChoice): boolean {
  return choice.mode !== "existing" || !!choice.id;
}
