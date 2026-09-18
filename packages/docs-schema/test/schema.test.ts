import { test } from "node:test";
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { Schema, type NodeSpec, type MarkSpec } from "prosemirror-model";

import {
  schema as definition,
  nodeNames,
  markNames,
  nodeAttrs,
  group,
  format,
  toProseMirrorSpec,
  diffEditorSchema,
  assertEditorSchema,
} from "../src/index.ts";

const here = dirname(fileURLToPath(import.meta.url));
const goldenDir = join(here, "..", "golden");

function buildSchema(): Schema {
  const spec = toProseMirrorSpec();
  return new Schema({
    topNode: spec.topNode,
    nodes: spec.nodes as Record<string, NodeSpec>,
    marks: spec.marks as Record<string, MarkSpec>,
  });
}

test("definition is internally consistent", () => {
  assert.equal(definition.version, 1);
  assert.equal(definition.topNode, "doc");
  assert.ok(nodeNames.includes("text"), "a text node is mandatory");
  for (const name of nodeNames) {
    const def = definition.nodes[name]!;
    if (def.inline) assert.ok(def.group, `inline node "${name}" must declare a group`);
    for (const [attr, spec] of Object.entries(nodeAttrs(name))) {
      assert.ok(
        spec.required || "default" in spec,
        `node "${name}" attribute "${attr}" must be required or have a default`,
      );
      if (spec.format) format(spec.format); // throws on unknown format
    }
    for (const set of def.requireOneOf ?? []) {
      for (const attr of set) assert.ok(attr in nodeAttrs(name), `${name}.requireOneOf names unknown attr ${attr}`);
    }
  }
  for (const name of markNames) {
    for (const other of (definition.marks[name]!.excludes ?? "").split(/\s+/).filter(Boolean)) {
      if (other !== "_") assert.ok(markNames.includes(other), `mark "${name}" excludes unknown mark "${other}"`);
    }
  }
  assert.deepEqual(group("inline").sort(), nodeNames.filter((n) => definition.nodes[n]!.group?.includes("inline")).sort());
});

test("prosemirror-model accepts every content expression", () => {
  const pm = buildSchema();
  assert.equal(pm.topNodeType.name, "doc");
  assert.deepEqual(Object.keys(pm.nodes).sort(), [...nodeNames]);
  assert.deepEqual(Object.keys(pm.marks).sort(), [...markNames]);
});

test("a real ProseMirror schema built from the definition passes the editor assertion", () => {
  const pm = buildSchema();
  assert.deepEqual(diffEditorSchema(pm), []);
  assertEditorSchema(pm);
});

test("the editor assertion reports drift precisely", () => {
  const spec = toProseMirrorSpec();
  delete spec.nodes["callout"];
  spec.nodes["heading"]!.attrs = { level: {} }; // drops the blockId/textBlock attrs
  spec.marks["glow"] = {};
  const pm = new Schema({
    topNode: spec.topNode,
    nodes: spec.nodes as Record<string, NodeSpec>,
    marks: spec.marks as Record<string, MarkSpec>,
  });
  const problems = diffEditorSchema(pm);
  assert.ok(problems.some((p) => p.includes('node "callout" is missing')), problems.join("\n"));
  assert.ok(problems.some((p) => p.startsWith('node "heading" attrs')), problems.join("\n"));
  assert.ok(problems.some((p) => p.includes('mark "glow" exists in the editor')), problems.join("\n"));
  assert.throws(() => assertEditorSchema(pm), /diverges/);
});

test("golden valid documents are accepted by ProseMirror", () => {
  const pm = buildSchema();
  const dir = join(goldenDir, "valid");
  const files = readdirSync(dir).filter((f) => f.endsWith(".json"));
  assert.ok(files.length > 0, "golden/valid is empty");
  for (const file of files) {
    const doc = JSON.parse(readFileSync(join(dir, file), "utf8"));
    const node = pm.nodeFromJSON(doc);
    assert.doesNotThrow(() => node.check(), `${file} should satisfy the ProseMirror schema`);
  }
});

test("golden invalid documents are rejected by ProseMirror when the rule is structural", () => {
  const pm = buildSchema();
  const dir = join(goldenDir, "invalid");
  const files = readdirSync(dir).filter((f) => f.endsWith(".json"));
  assert.ok(files.length > 0, "golden/invalid is empty");
  // ProseMirror knows structure (node types, content expressions, mark
  // exclusion) and, through the `validate` option we attach to every
  // attribute, the per-attribute value rules. Cross-attribute rules
  // (require_one_of) and document limits are the Go validator's alone.
  const structural = new Set([
    "unknown_node", "unknown_mark", "content", "leaf_content", "missing_attr", "attr_type", "attr_value",
    "mark_conflict", "root_type", "mark_not_allowed", "unknown_attr", "text_node",
  ]);
  for (const file of files) {
    const { expect, doc } = JSON.parse(readFileSync(join(dir, file), "utf8"));
    if (!structural.has(expect)) continue;
    assert.throws(
      () => {
        const node = pm.nodeFromJSON(doc);
        node.check();
        // ProseMirror's nodeFromJSON silently drops unknown attrs, so mirror
        // the Go rule explicitly for that code.
        if (expect === "unknown_attr") throw new Error("unknown_attr");
        // root_type is a Yuheng rule (documents must start at topNode).
        if (expect === "root_type" && node.type.name !== pm.topNodeType.name) throw new Error("root_type");
      },
      `${file} (expect ${expect}) should be rejected`,
    );
  }
});
