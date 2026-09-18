/**
 * @yuheng/docs-schema — the TypeScript side of the Yuheng document model.
 *
 * `schema.json` is the single definition of every node, mark and attribute a
 * document may contain. The Go server embeds a copy and validates incoming
 * documents against it; the editor must build its Tiptap extension set to the
 * same shape. This module gives the editor two tools for that:
 *
 *   - `toProseMirrorSpec()` turns the definition into a prosemirror-model
 *     `SchemaSpec`, so tests can construct a real `Schema` and let ProseMirror
 *     itself prove every content expression is well-formed;
 *   - `diffEditorSchema()` / `assertEditorSchema()` compare a live editor
 *     schema (node names, content expressions, attribute names, mark names)
 *     against the definition and report every divergence.
 *
 * Nothing here depends on Tiptap; the editor package wires the assertion into
 * its startup path.
 */

import definition from "../schema.json" with { type: "json" };

export type AttrType = "string" | "int" | "number" | "bool" | "enum" | "list";

export interface AttrSpec {
  type: AttrType;
  values?: string[];
  default?: unknown;
  nullable?: boolean;
  required?: boolean;
  min?: number;
  max?: number;
  maxLength?: number;
  items?: AttrType;
  format?: string;
}

export interface NodeDefinition {
  group?: string;
  content?: string;
  inline?: boolean;
  atom?: boolean;
  text?: boolean;
  /** `undefined` = all marks, `""` = none, otherwise space-separated names. */
  marks?: string;
  use?: string[];
  attrs?: Record<string, AttrSpec>;
  requireOneOf?: string[][];
}

export interface MarkDefinition {
  excludes?: string;
  attrs?: Record<string, AttrSpec>;
}

export interface SchemaDefinition {
  version: number;
  topNode: string;
  limits: { maxDepth: number; maxNodes: number; maxTextBytes: number; maxAttrStringBytes: number };
  formats: Record<string, string>;
  attrSets: Record<string, Record<string, AttrSpec>>;
  nodes: Record<string, NodeDefinition>;
  marks: Record<string, MarkDefinition>;
}

export const schema: SchemaDefinition = definition as unknown as SchemaDefinition;

/** Every node type name, sorted. */
export const nodeNames: readonly string[] = Object.keys(schema.nodes).sort();
/** Every mark type name, sorted. */
export const markNames: readonly string[] = Object.keys(schema.marks).sort();

/** Resolved attributes of a node: its attrSets first, then its own attrs. */
export function nodeAttrs(name: string): Record<string, AttrSpec> {
  const node = schema.nodes[name];
  if (!node) throw new Error(`docs-schema: unknown node "${name}"`);
  const out: Record<string, AttrSpec> = {};
  for (const setName of node.use ?? []) {
    const set = schema.attrSets[setName];
    if (!set) throw new Error(`docs-schema: node "${name}" uses unknown attrSet "${setName}"`);
    for (const [attr, spec] of Object.entries(set)) {
      if (attr in out) throw new Error(`docs-schema: node "${name}" declares "${attr}" twice`);
      out[attr] = spec;
    }
  }
  for (const [attr, spec] of Object.entries(node.attrs ?? {})) {
    if (attr in out) throw new Error(`docs-schema: node "${name}" overrides attrSet attribute "${attr}"`);
    out[attr] = spec;
  }
  return out;
}

/** Attributes of a mark. */
export function markAttrs(name: string): Record<string, AttrSpec> {
  const mark = schema.marks[name];
  if (!mark) throw new Error(`docs-schema: unknown mark "${name}"`);
  return { ...(mark.attrs ?? {}) };
}

/** Node names belonging to a group, sorted. */
export function group(name: string): string[] {
  return nodeNames.filter((n) => (schema.nodes[n]?.group ?? "").split(/\s+/).includes(name));
}

/** A compiled regular expression for a named value format. */
export function format(name: string): RegExp {
  const pattern = schema.formats[name];
  if (!pattern) throw new Error(`docs-schema: unknown format "${name}"`);
  return new RegExp(pattern);
}

// ---- ProseMirror spec ------------------------------------------------------

/** Minimal structural type for prosemirror-model's AttributeSpec. */
interface PMAttributeSpec {
  default?: unknown;
  /** ProseMirror >= 1.23 runs this from `Node.check()` / `createChecked()`. */
  validate?: string | ((value: unknown) => void);
}

/**
 * Build a value validator for one attribute, mirroring the Go validator's
 * rules (type, nullability, enum, range, length, named format). The editor
 * attaches it as the ProseMirror `validate` option so `node.check()` enforces
 * the same contract the server does.
 */
export function makeAttrValidator(name: string, spec: AttrSpec): (value: unknown) => void {
  const fmt = spec.format ? format(spec.format) : null;
  const fail = (msg: string): never => {
    throw new RangeError(`attribute "${name}": ${msg}`);
  };
  const checkScalar = (v: unknown, type: AttrType): void => {
    switch (type) {
      case "string":
        if (typeof v !== "string") fail(`expected string, got ${typeof v}`);
        break;
      case "enum":
        if (typeof v !== "string") fail(`expected string, got ${typeof v}`);
        if (!spec.values?.includes(v as string)) fail(`"${v}" is not one of ${spec.values?.join(", ")}`);
        break;
      case "int":
        if (typeof v !== "number" || !Number.isInteger(v)) fail(`expected integer, got ${JSON.stringify(v)}`);
        break;
      case "number":
        if (typeof v !== "number" || !Number.isFinite(v)) fail(`expected number, got ${JSON.stringify(v)}`);
        break;
      case "bool":
        if (typeof v !== "boolean") fail(`expected boolean, got ${typeof v}`);
        break;
      default:
        fail(`unsupported type ${type}`);
    }
    if (typeof v === "number") {
      if (spec.min !== undefined && v < spec.min) fail(`${v} is below minimum ${spec.min}`);
      if (spec.max !== undefined && v > spec.max) fail(`${v} is above maximum ${spec.max}`);
    }
    if (typeof v === "string" && type !== "enum") {
      if (spec.maxLength && [...v].length > spec.maxLength) fail(`length exceeds ${spec.maxLength}`);
      if (fmt && !fmt.test(v)) fail(`"${v.slice(0, 64)}" does not match format ${spec.format}`);
    }
  };
  return (value: unknown): void => {
    if (value === null || value === undefined) {
      if (spec.required) fail("is required");
      if (!spec.nullable) fail("is not nullable");
      return;
    }
    if (spec.type === "list") {
      if (!Array.isArray(value)) fail(`expected array, got ${typeof value}`);
      if (spec.maxLength && (value as unknown[]).length > spec.maxLength) fail(`more than ${spec.maxLength} items`);
      for (const item of value as unknown[]) {
        if (item === null) continue; // e.g. colwidth uses null for unsized columns
        checkScalar(item, spec.items ?? "string");
      }
      return;
    }
    checkScalar(value, spec.type);
  };
}
interface PMNodeSpec {
  content?: string;
  group?: string;
  inline?: boolean;
  atom?: boolean;
  marks?: string;
  attrs?: Record<string, PMAttributeSpec>;
}
interface PMMarkSpec {
  excludes?: string;
  attrs?: Record<string, PMAttributeSpec>;
}
export interface PMSchemaSpec {
  topNode: string;
  nodes: Record<string, PMNodeSpec>;
  marks: Record<string, PMMarkSpec>;
}

function toPMAttrs(attrs: Record<string, AttrSpec>): Record<string, PMAttributeSpec> | undefined {
  const names = Object.keys(attrs);
  if (names.length === 0) return undefined;
  const out: Record<string, PMAttributeSpec> = {};
  for (const name of names.sort()) {
    const spec = attrs[name]!;
    // A required attribute has no default in ProseMirror terms. ProseMirror only
    // enforces attribute rules through `validate` (run by `node.check()`), so
    // every attribute carries the validator derived from its spec.
    const validate = makeAttrValidator(name, spec);
    out[name] = spec.required ? { validate } : { default: spec.default ?? null, validate };
  }
  return out;
}

/**
 * Convert the definition into a prosemirror-model SchemaSpec. The result is
 * used by tests (and may be used by tooling); the editor assembles Tiptap
 * extensions instead, then asserts against the definition.
 */
export function toProseMirrorSpec(): PMSchemaSpec {
  const nodes: Record<string, PMNodeSpec> = {};
  // ProseMirror requires the top node first and "text" to exist; ordering
  // otherwise does not matter but a stable order keeps diffs readable.
  const ordered = [schema.topNode, ...nodeNames.filter((n) => n !== schema.topNode)];
  for (const name of ordered) {
    const def = schema.nodes[name]!;
    const spec: PMNodeSpec = {};
    if (def.content) spec.content = def.content;
    if (def.group) spec.group = def.group;
    if (def.inline) spec.inline = true;
    if (def.atom) spec.atom = true;
    if (def.marks !== undefined) spec.marks = def.marks;
    const attrs = toPMAttrs(nodeAttrs(name));
    if (attrs) spec.attrs = attrs;
    nodes[name] = spec;
  }
  const marks: Record<string, PMMarkSpec> = {};
  for (const name of markNames) {
    const def = schema.marks[name]!;
    const spec: PMMarkSpec = {};
    if (def.excludes !== undefined) spec.excludes = def.excludes;
    const attrs = toPMAttrs(markAttrs(name));
    if (attrs) spec.attrs = attrs;
    marks[name] = spec;
  }
  return { topNode: schema.topNode, nodes, marks };
}

// ---- editor assertion ------------------------------------------------------

/**
 * The parts of a live prosemirror-model Schema the assertion reads. Typed
 * structurally so this package needs no runtime dependency on ProseMirror.
 */
export interface LiveSchemaLike {
  topNodeType: { name: string };
  nodes: Record<string, { name: string; spec: PMNodeSpec & { group?: string } }>;
  marks: Record<string, { name: string; spec: PMMarkSpec }>;
}

/**
 * Compare a live editor schema with the definition. Returns a list of
 * human-readable divergences; empty means the editor matches the model.
 *
 * Group membership is compared as sets, content expressions as normalised
 * strings, attributes by name (types cannot be observed on a ProseMirror
 * spec; the Go validator enforces them server-side).
 */
export function diffEditorSchema(live: LiveSchemaLike): string[] {
  const out: string[] = [];
  if (live.topNodeType.name !== schema.topNode) {
    out.push(`topNode: editor has "${live.topNodeType.name}", schema has "${schema.topNode}"`);
  }
  const liveNodes = Object.keys(live.nodes).sort();
  for (const name of nodeNames) {
    if (!(name in live.nodes)) out.push(`node "${name}" is missing from the editor`);
  }
  for (const name of liveNodes) {
    if (!(name in schema.nodes)) out.push(`node "${name}" exists in the editor but not in the schema`);
  }
  for (const name of nodeNames) {
    const liveNode = live.nodes[name];
    const def = schema.nodes[name]!;
    if (!liveNode) continue;
    const liveContent = normaliseExpr(liveNode.spec.content ?? "");
    const defContent = normaliseExpr(def.content ?? "");
    if (liveContent !== defContent) {
      out.push(`node "${name}" content: editor "${liveContent}", schema "${defContent}"`);
    }
    const liveGroups = new Set((liveNode.spec.group ?? "").split(/\s+/).filter(Boolean));
    const defGroups = new Set((def.group ?? "").split(/\s+/).filter(Boolean));
    if (!sameSet(liveGroups, defGroups)) {
      out.push(`node "${name}" group: editor [${[...liveGroups]}], schema [${[...defGroups]}]`);
    }
    if (Boolean(liveNode.spec.inline) !== Boolean(def.inline)) {
      out.push(`node "${name}" inline: editor ${Boolean(liveNode.spec.inline)}, schema ${Boolean(def.inline)}`);
    }
    const liveMarks = liveNode.spec.marks;
    const defMarks = def.marks;
    if ((liveMarks ?? "_") !== (defMarks ?? "_") && !(liveMarks === "_" && defMarks === undefined)) {
      out.push(`node "${name}" marks: editor "${liveMarks ?? "_"}", schema "${defMarks ?? "_"}"`);
    }
    const liveAttrs = new Set(Object.keys(liveNode.spec.attrs ?? {}));
    const defAttrs = new Set(Object.keys(nodeAttrs(name)));
    if (!sameSet(liveAttrs, defAttrs)) {
      out.push(`node "${name}" attrs: editor [${[...liveAttrs].sort()}], schema [${[...defAttrs].sort()}]`);
    }
  }
  const liveMarkNames = Object.keys(live.marks).sort();
  for (const name of markNames) {
    if (!(name in live.marks)) out.push(`mark "${name}" is missing from the editor`);
  }
  for (const name of liveMarkNames) {
    if (!(name in schema.marks)) out.push(`mark "${name}" exists in the editor but not in the schema`);
  }
  for (const name of markNames) {
    const liveMark = live.marks[name];
    if (!liveMark) continue;
    const liveAttrs = new Set(Object.keys(liveMark.spec.attrs ?? {}));
    const defAttrs = new Set(Object.keys(markAttrs(name)));
    if (!sameSet(liveAttrs, defAttrs)) {
      out.push(`mark "${name}" attrs: editor [${[...liveAttrs].sort()}], schema [${[...defAttrs].sort()}]`);
    }
    const liveEx = liveMark.spec.excludes ?? "";
    const defEx = schema.marks[name]!.excludes ?? "";
    if (liveEx !== defEx) out.push(`mark "${name}" excludes: editor "${liveEx}", schema "${defEx}"`);
  }
  return out;
}

/** Throw with every divergence listed when the editor drifts from the model. */
export function assertEditorSchema(live: LiveSchemaLike): void {
  const problems = diffEditorSchema(live);
  if (problems.length > 0) {
    throw new Error(`docs-schema: editor schema diverges from schema.json:\n  - ${problems.join("\n  - ")}`);
  }
}

function normaliseExpr(expr: string): string {
  return expr.replace(/\s+/g, " ").replace(/\s*\|\s*/g, " | ").replace(/\(\s+/g, "(").replace(/\s+\)/g, ")").trim();
}

function sameSet(a: Set<string>, b: Set<string>): boolean {
  if (a.size !== b.size) return false;
  for (const v of a) if (!b.has(v)) return false;
  return true;
}
