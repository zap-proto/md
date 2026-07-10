# zap-md

Markdown response adapter for the ZAP protocol ecosystem — token-efficient, agent-ready serialization.

[**zap-proto.io**](https://zap-proto.io) · [Spec](https://github.com/zap-proto/spec) · [MCP adapter](https://github.com/zap-proto/mcp) · [Discord](https://zap-proto.io/discord)

ONE data model (structured / ZAP), MANY serialization adapters — `json` · [`mcp`](https://github.com/zap-proto/mcp) · **`md`** — chosen by content negotiation. A handler PRODUCES structured data; it never hand-writes markdown. `zap-md` serializes that data to canonical markdown, exactly as the JSON codec would, only denser: no repeated keys, quotes or braces, so an LLM spends fewer tokens (cheaper) reading it — and a human can read it too.

## Status

**v0.1 — Go MVP.** This repo ships a deterministic Go renderer (`Render`, `RenderJSON`) plus the table inverse (`ParseTable`). Multi-language adapters (rust, py, ts, …), matching the rest of `zap-proto`, land next; a full markdown→structured parser follows.

## Install

```sh
go get github.com/zap-proto/md
```

## Use

```go
import "github.com/zap-proto/md"

out, err := md.Render(searchResults) // struct / map / slice / json.RawMessage
// or re-serialize an already-encoded JSON body, preserving key order:
out, err = md.RenderJSON(jsonBytes)
```

An array of uniform objects becomes a table — the big token win:

```md
| repository | path | line | score |
| --- | --- | --- | --- |
| hanzoai/cloud | serve.go | 76 | 0.98 |
| hanzoai/iam | auth.go | 142 | 0.91 |
```

versus the equivalent JSON, which repeats every key on every row. On an 8-row
search result set that is **~33% fewer bytes and ~32% fewer tokens** (see
`TestByteAndTokenSavings`).

## Rendering rules (stable, reparseable schema)

| Input shape | Markdown |
|---|---|
| Array of uniform objects (all scalar values) | GFM table — columns = first-seen key union, one row per object |
| Object | `- **key**: value` list in source order |
| Nested object / object-array | sub-section under an ATX heading (depth grows with nesting, capped at h6) |
| Scalar array | inline comma-separated list on the key's line |
| Scalar | inline — `json.Number` verbatim, `bool` as true/false, null as empty |
| Multi-line string, or a code-ish key (`diff`, `patch`, `code`, `snippet`, `source`, `stdout`, `stderr`, `traceback`, `stacktrace`) | fenced code block; language from a sibling `language`/`lang`/`ext`/`filetype` field (`diff`/`patch` force `diff`) |

Output is deterministic — the same input bytes always render to the same output
bytes. Native Go values are normalized through `encoding/json` first (struct
field order and tags honored, map keys sorted), so the markdown mirrors the JSON
codec. `ParseTable` recovers a rendered table's columns and string cells,
closing the round-trip on the headline case.

## Why an adapter, not a formatter

Decomplected: verification and policy stay out of it; this package does one
thing — turn a generic structured value into canonical markdown. Because the
data model is shared, the same handler serves `json` to a machine and `md` to an
agent with no second copy of any business logic — the wire form is a
content-negotiation choice, not a code path.

## Sub-protocol family

- [`zap-http`](https://github.com/zap-proto/http) — HTTP request/response over ZAP
- [`zap-ws`](https://github.com/zap-proto/ws) — multi-stream pubsub
- [`zap-mcp`](https://github.com/zap-proto/mcp) — Model Context Protocol over ZAP
- [`zap-acp`](https://github.com/zap-proto/acp) — Agent Communication Protocol
- [`zap-a2a`](https://github.com/zap-proto/a2a) — Google Agent2Agent over ZAP
- [`zap-md`](https://github.com/zap-proto/md) — markdown response adapter

## License

MIT OR Apache-2.0
