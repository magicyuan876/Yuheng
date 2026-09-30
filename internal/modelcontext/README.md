# Model context boundary

`modelcontext.Registry` is the only application-facing boundary for values
that are shortened before an LLM call and restored afterwards. The whole
codec lives in this one package:

| File | Role |
| --- | --- |
| `registry.go` | `Registry` facade — the only type request lifecycles use |
| `handle_table.go` | `handleTable[M]`, the generic bidirectional primitive behind every handle space |
| `sources.go` | source handles (`cN`/`dN`/`bN`/`wN`) and history compaction |
| `citations.go` | protocol prompts, `<ref/>` → `<kb/>`/`<web/>` expansion, citation stream expander |
| `model_output.go` | compact source-centric renderings of retrieved knowledge and web results |
| `resources.go` | durable-resource handles (`res://NNNN`) |
| `stream.go` | `streamHold` suffix-hold primitive and every streaming decoder |
| `handles.go` | exported `HandleTable` for invocation-local spaces (`ref-N`, `c000`) |

## Identity rules

- Durable application identities: UUIDs, Wiki slugs, URLs, and
  `resource://...` references.
- Request-local model handles: `cN`, `dN`, `bN`, `wN`, and `res://NNNN`.
- Wiki-ingest call handles: `cNNN` and `ref-N`, allocated with `HandleTable`.
- Temporary handles are decoded before persistence or UI delivery. They are
  never accepted as durable authorization or routing data.

## Request lifecycle

1. Create one `Registry` for the request.
2. Register structured source identities as they enter the context, and render
   retrieved results with `ModelToolResult`.
3. Call `EncodeMessages` immediately before the model call. Canonical citations
   in replayed assistant turns are compacted back into this request's handles.
4. Decode complete responses with `DecodeResponse`, or streaming text with one
   `StreamDecoder` per response channel.

The registry owns codec ordering. Resource references are encoded before
source IDs so a Wiki slug such as `summary/<knowledge-id>` cannot be corrupted
into `summary/d1`.

## Observability

Langfuse generation observations contain the exact encoded payload sent to and
returned by the model. Langfuse is an observer of this boundary; it must not
implement another handle mapping.
