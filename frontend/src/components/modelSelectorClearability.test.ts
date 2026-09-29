// @vitest-environment node
import assert from "node:assert/strict";
import { test } from "vitest";
import { readFileSync } from "node:fs";

const kbModelConfig = readFileSync(new URL("../views/knowledge/settings/KBModelConfig.vue", import.meta.url), "utf8");
const kbEditor = readFileSync(new URL("../views/knowledge/KnowledgeBaseEditorModal.vue", import.meta.url), "utf8");
const uploadConfirm = readFileSync(
  new URL("../views/knowledge/components/UploadConfirmDialog.vue", import.meta.url),
  "utf8",
);

function modelSelectorTag(source: string, selectedModelBinding: string): string {
  const tag = source
    .match(/<ModelSelector\b[\s\S]*?\/>/g)
    ?.find((candidate) => candidate.includes(selectedModelBinding));
  assert.ok(tag, `找不到模型选择器：${selectedModelBinding}`);
  return tag;
}

function assertClearable(tag: string): void {
  assert.match(tag, /(?:\s|:)clearable(?:\s|=|\/>)/);
}

function assertNotClearable(tag: string): void {
  assert.doesNotMatch(tag, /(?:\s|:)clearable(?:\s|=|\/>)/);
}

// ModelSelector's own clearing behaviour is covered by the mounted tests in
// ModelSelector.test.ts. What stays here is which call sites opt in.

test("知识库仅在模型确实可选时允许恢复为空", () => {
  const embedding = modelSelectorTag(kbModelConfig, "config.embeddingModelId");
  assert.match(embedding, /:clearable="ragEnabled === false && wikiEnabled"/);
  assertClearable(modelSelectorTag(kbModelConfig, "config.wikiSynthesisModelId"));
});

test("必填模型继续保持不可清空", () => {
  assertNotClearable(modelSelectorTag(kbModelConfig, "config.llmModelId"));
  assertNotClearable(modelSelectorTag(kbEditor, "formData.multimodalConfig.vllmModelId"));
  assertNotClearable(modelSelectorTag(kbEditor, "formData.asrConfig.modelId"));
  assertNotClearable(modelSelectorTag(uploadConfirm, "uiState.multimodalConfig.vllmModelId"));
  assertNotClearable(modelSelectorTag(uploadConfirm, "uiState.asrConfig.modelId"));
});
