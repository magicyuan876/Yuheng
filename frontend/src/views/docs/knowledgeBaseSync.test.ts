import assert from "node:assert/strict";
import { test } from "vitest";

import {
  bindableKnowledgeBases,
  choiceComplete,
  choiceRequest,
  currentChoice,
  isEmbeddingModelRequired,
  sameChoice,
} from "./knowledgeBaseSync";

const kbs = [
  { id: "mine", name: "Mine", type: "document", creator_id: "alice" },
  { id: "theirs", name: "Theirs", type: "document", creator_id: "bob" },
  { id: "unowned", name: "Unowned", type: "document" },
  { id: "faq", name: "FAQ", type: "faq", creator_id: "alice" },
  { id: "hidden", name: "Hidden", type: "document", creator_id: "alice", is_temporary: true },
];

test("bindable knowledge bases follow the server's rule for filling one", () => {
  const ids = (isAdmin: boolean) => bindableKnowledgeBases(kbs, { userId: "alice", isAdmin }).map((kb) => kb.id);
  assert.deepEqual(ids(false), ["mine"], "a member binds only what they created");
  assert.deepEqual(ids(true), ["mine", "theirs", "unowned"], "an administrator binds any document base");
});

test("choices compare, complete and serialise the way the server reads them", () => {
  assert.deepEqual(currentChoice("kb-1"), { mode: "existing", id: "kb-1" });
  assert.deepEqual(currentChoice(null), { mode: "none" });
  assert.ok(sameChoice({ mode: "existing", id: "a" }, { mode: "existing", id: "a" }));
  assert.ok(!sameChoice({ mode: "existing", id: "a" }, { mode: "existing", id: "b" }));
  assert.ok(sameChoice({ mode: "create", id: "stale" }, { mode: "create" }), "an id means nothing outside existing");
  assert.ok(!choiceComplete({ mode: "existing" }));
  assert.deepEqual(choiceRequest({ mode: "none", id: "stale" }), { mode: "none" });
  assert.ok(isEmbeddingModelRequired({ error: { code: 2300 } }));
  assert.ok(!isEmbeddingModelRequired(new Error("x")));
});
