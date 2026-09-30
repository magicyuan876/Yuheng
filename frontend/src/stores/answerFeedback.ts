import { defineStore } from "pinia";
import { ref } from "vue";

import { listSessionFeedback, setAnswerFeedback, type AnswerFeedback, type SetAnswerFeedback } from "@/api/feedback";

// The caller's feedback on the answers of the session on screen. Every answer
// of a conversation renders its own feedback buttons; this lets them share one
// request for the whole session instead of each asking for its own.
export const useAnswerFeedbackStore = defineStore("answerFeedback", () => {
  const sessionId = ref("");
  const byMessage = ref<Record<string, AnswerFeedback>>({});
  let loading: Promise<void> | null = null;

  /** Loads the session's feedback once; later calls for the same session
   * wait for the same request. */
  function ensure(id: string): Promise<void> {
    if (!id) return Promise.resolve();
    if (id === sessionId.value && loading) return loading;
    sessionId.value = id;
    byMessage.value = {};
    loading = listSessionFeedback(id)
      .then((map) => {
        if (sessionId.value === id) byMessage.value = map;
      })
      .catch((err) => {
        // The buttons still work; they just start unpressed.
        console.debug("answer feedback unavailable", err);
      });
    return loading;
  }

  async function set(id: string, messageId: string, body: SetAnswerFeedback): Promise<AnswerFeedback | null> {
    const saved = await setAnswerFeedback(id, messageId, body);
    if (sessionId.value === id) {
      const next = { ...byMessage.value };
      if (saved) next[messageId] = saved;
      else delete next[messageId];
      byMessage.value = next;
    }
    return saved;
  }

  const get = (messageId: string): AnswerFeedback | undefined => byMessage.value[messageId];

  return { ensure, set, get };
});
