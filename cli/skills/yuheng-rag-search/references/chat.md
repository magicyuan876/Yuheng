# `chat` — RAG answers and raw streams

`chat` returns one buffered JSON envelope with answer events by default.
`--reference` adds lookup-index references; `--verbose` adds execution events.
Under `--format ndjson` you get the raw SSE event stream instead.

## Commands & flags

```
yuheng chat "<query>" --kb <name-or-id> [--session <id>]
```

- `--kb` is required name-or-id.
- `--session <id>` continues an existing conversation; omit to start a new one.
- `--format json` returns one `{ok,data:{events:[...]}}` envelope; `--format
  text` streams the same projection as readable text; `--format ndjson` emits
  the raw event stream.
- `--reference` adds bounded reference indexes to JSON/text.
- `--verbose` adds thinking, reflection, metadata, and lifecycle events.
- Combine them when both provenance and execution detail are needed.

## Event stream (`--format ndjson`)

Under `--format ndjson`, the CLI emits an `init` line first, then passes SDK
events through verbatim:

```jsonc
{"type":"init","session_id":"sess_abc","kb_id":"…","profile":"prod"}
{"response_type":"thinking","content":"…"}
{"response_type":"references","knowledge_references":[…]}
{"response_type":"answer","content":"partial text…"}   // streamed in pieces
{"response_type":"complete","done":true}
```

- Accumulate `response_type:"answer"` `content` pieces for the final answer.
- `knowledge_references` carry the grounding chunks (source attribution).
- **Keep `init.session_id`** to continue the chat (`--session`). The
  `assistant_message_id` needed for `session stop` / `session resume`
  rides on the SDK's `agent_query` frame, not on `init` — scan for it.
- On failure mid-stream you get `response_type:"error"`; a transport/HTTP error
  surfaces as the normal error envelope on stderr with a typed code.

## Recovery

- **Stop server-side generation:** `yuheng session stop <session-id> --message
  <message-id>`. Ctrl-C only closes your local connection — the server keeps
  generating (and billing) until told to stop.
- **Re-attach after a dropped connection:** `yuheng session resume
  <session-id> --message <message-id>`. The server replays the event log from
  index 0 then tails new events, so **dedupe by message_id** if you already
  consumed some events. Buffer TTL is ~1h (redis) or process-lifetime (memory).
