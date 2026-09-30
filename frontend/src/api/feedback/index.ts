import { get, put } from "@/utils/request";

// Feedback on answers. A "not helpful" on an answer that cites documents of
// the workspace shows up in those documents' knowledge health, for their
// owners: with the comment and the start of the answer, and with the question
// only when the person attaches it.

export type AnswerRating = "up" | "down";

export interface AnswerFeedback {
  message_id: string;
  rating: AnswerRating;
  comment: string;
  share_question: boolean;
  updated_at: string;
}

export interface SetAnswerFeedback {
  /** Empty takes the feedback back. */
  rating: AnswerRating | "";
  comment?: string;
  share_question?: boolean;
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

const sessionBase = (sessionId: string) => `/api/v1/sessions/${encodeURIComponent(sessionId)}`;

/** Backend: GET /api/v1/sessions/:id/feedback — the caller's feedback, keyed by message ID. */
export async function listSessionFeedback(sessionId: string): Promise<Record<string, AnswerFeedback>> {
  return unwrap<Record<string, AnswerFeedback> | null>(await get(`${sessionBase(sessionId)}/feedback`)) ?? {};
}

/** Backend: PUT /api/v1/sessions/:id/messages/:message_id/feedback. Resolves to null when the
 * feedback was taken back. */
export async function setAnswerFeedback(
  sessionId: string,
  messageId: string,
  body: SetAnswerFeedback,
): Promise<AnswerFeedback | null> {
  return (
    unwrap<AnswerFeedback | null>(
      await put(`${sessionBase(sessionId)}/messages/${encodeURIComponent(messageId)}/feedback`, body),
    ) ?? null
  );
}
