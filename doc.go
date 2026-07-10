// Package md is the markdown response adapter for the ZAP protocol ecosystem:
// ONE structured data model, MANY serialization adapters (json · mcp · md)
// chosen by content negotiation. A handler PRODUCES structured data; it never
// hand-writes markdown — this package serializes it, exactly as the json codec
// would, just to a form that costs an LLM fewer tokens to read.
//
// # Why markdown
//
// For an array of uniform objects, JSON repeats every key, quote and brace on
// every element. Markdown writes the column headers once and one row per
// object. The result is materially fewer bytes (and tokens) and is
// human-readable, which is why it is the preferred wire form for agent-facing
// endpoints.
//
// # Rendering rules (the stable, reparseable schema)
//
//   - Array of uniform objects (every element an object whose every value is a
//     scalar) → a GitHub-flavored markdown table. Columns are the union of
//     keys in first-seen order; one row per object; missing keys are empty
//     cells. This is the headline token win and the round-trippable form
//     (see ParseTable).
//   - Object → a compact "- **key**: value" list, in source key order. Nested
//     objects and object-arrays become sub-sections under an ATX heading whose
//     depth grows with nesting (capped at h6).
//   - Scalar array → an inline comma-separated list on the key's line.
//   - Scalar → rendered inline (json.Number verbatim, so 76 and 0.98 keep
//     their exact text; bool as true/false; null as empty).
//   - A string that is multi-line, or whose key names code (diff, patch, code,
//     snippet, source, stdout, stderr, traceback, stacktrace) → a fenced code
//     block. The fence language is taken from a sibling language/lang/ext/
//     filetype field when present; diff and patch force the "diff" language.
//     The fence length grows past any backtick run inside the content.
//
// Output is deterministic: the same input bytes always render to the same
// output bytes. Native Go values (structs, maps, slices) are normalized through
// encoding/json first — struct field order and json tags are honored, and map
// keys are sorted — so the markdown mirrors the JSON codec exactly.
//
// # Inverse
//
// ParseTable recovers the columns and string cells of a rendered table, giving
// a concrete round-trip for the headline case. A full markdown→structured
// parser is deferred; the rules above are the documented, stable schema a
// consumer can rely on.
package md
