package md

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderTable(t *testing.T) {
	in := json.RawMessage(`[{"name":"alpha","status":"ok"},{"name":"beta","status":"fail"}]`)
	want := "| name | status |\n| --- | --- |\n| alpha | ok |\n| beta | fail |\n"
	got, err := Render(in)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("table mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestRenderTableUnionColumns(t *testing.T) {
	// Second row introduces a new key; columns are the first-seen union and
	// missing cells are empty.
	in := json.RawMessage(`[{"a":1},{"a":2,"b":3}]`)
	want := "| a | b |\n| --- | --- |\n| 1 |  |\n| 2 | 3 |\n"
	got, _ := Render(in)
	if string(got) != want {
		t.Fatalf("union columns\n got: %q\nwant: %q", got, want)
	}
}

func TestScalarArrayInline(t *testing.T) {
	in := json.RawMessage(`{"tags":["a","b","c"],"count":3}`)
	want := "- **tags**: a, b, c\n- **count**: 3\n"
	got, _ := Render(in)
	if string(got) != want {
		t.Fatalf("scalar array\n got: %q\nwant: %q", got, want)
	}
}

func TestNestedObject(t *testing.T) {
	in := json.RawMessage(`{"id":"u1","profile":{"name":"Ann","age":30}}`)
	want := "- **id**: u1\n\n## profile\n\n- **name**: Ann\n- **age**: 30\n"
	got, _ := Render(in)
	if string(got) != want {
		t.Fatalf("nested object\n got: %q\nwant: %q", got, want)
	}
}

func TestDiffCodeBlock(t *testing.T) {
	in := json.RawMessage(`{"file":"a.go","diff":"- old\n+ new"}`)
	want := "- **file**: a.go\n\n**diff**:\n\n```diff\n- old\n+ new\n```\n"
	got, _ := Render(in)
	if string(got) != want {
		t.Fatalf("diff block\n got: %q\nwant: %q", got, want)
	}
}

func TestCodeBlockLangFromSibling(t *testing.T) {
	// A sibling language field sets the fence language for a code-ish key.
	in := json.RawMessage(`{"language":"go","source":"package main"}`)
	got, _ := Render(in)
	if !strings.Contains(string(got), "```go\npackage main\n```") {
		t.Fatalf("expected go-fenced source, got: %q", got)
	}
}

func TestCodeFenceEscapesBackticks(t *testing.T) {
	// Content containing a triple backtick must be wrapped in a longer fence.
	// (Interpreted string literal: backticks are literal, \n is a real newline.)
	in := json.RawMessage("{\"code\":\"``` inner\\nmore\"}")
	got, _ := Render(in)
	if !strings.Contains(string(got), "````\n") {
		t.Fatalf("expected 4-backtick fence, got: %q", got)
	}
}

func TestStructInputFieldOrder(t *testing.T) {
	// Native structs flow through encoding/json: field order + tags honored.
	type row struct {
		Name  string `json:"name"`
		Score int    `json:"score"`
	}
	got, err := Render([]row{{"alpha", 9}, {"beta", 3}})
	if err != nil {
		t.Fatal(err)
	}
	want := "| name | score |\n| --- | --- |\n| alpha | 9 |\n| beta | 3 |\n"
	if string(got) != want {
		t.Fatalf("struct table\n got: %q\nwant: %q", got, want)
	}
}

func TestDeterministicMapKeysSorted(t *testing.T) {
	m := map[string]any{"b": 2, "a": 1, "c": 3}
	first, _ := Render(m)
	for i := 0; i < 50; i++ {
		again, _ := Render(m)
		if string(again) != string(first) {
			t.Fatalf("non-deterministic map render:\n%q\nvs\n%q", first, again)
		}
	}
	if want := "- **a**: 1\n- **b**: 2\n- **c**: 3\n"; string(first) != want {
		t.Fatalf("sorted map\n got: %q\nwant: %q", first, want)
	}
}

func TestRoundTripTable(t *testing.T) {
	in := json.RawMessage(`[{"name":"alpha","status":"ok","score":9},{"name":"beta","status":"fail","score":3}]`)
	rendered, err := Render(in)
	if err != nil {
		t.Fatal(err)
	}
	tbl, err := ParseTable(rendered)
	if err != nil {
		t.Fatal(err)
	}
	wantCols := []string{"name", "status", "score"}
	if strings.Join(tbl.Columns, ",") != strings.Join(wantCols, ",") {
		t.Fatalf("columns: got %v want %v", tbl.Columns, wantCols)
	}
	wantRows := [][]string{{"alpha", "ok", "9"}, {"beta", "fail", "3"}}
	if len(tbl.Rows) != len(wantRows) {
		t.Fatalf("rows: got %d want %d", len(tbl.Rows), len(wantRows))
	}
	for i, want := range wantRows {
		if strings.Join(tbl.Rows[i], ",") != strings.Join(want, ",") {
			t.Fatalf("row %d: got %v want %v", i, tbl.Rows[i], want)
		}
	}
}

func TestFailSafeInvalidJSON(t *testing.T) {
	if _, err := RenderJSON([]byte(`{not json`)); err == nil {
		t.Fatal("expected error on invalid JSON so a middleware can fall back to JSON")
	}
	if _, err := RenderJSON([]byte(``)); err == nil {
		t.Fatal("expected error on empty input")
	}
}

// estimateTokens is the common ~4-bytes-per-token heuristic; used only to
// report an approximate LLM-token figure alongside the exact byte count.
func estimateTokens(b []byte) int { return (len(b) + 3) / 4 }

func TestByteAndTokenSavings(t *testing.T) {
	// A representative /v1/code/search result set: 8 uniform rows.
	searchJSON := `[` +
		`{"repository":"hanzoai/cloud","path":"serve.go","line":76,"score":0.98},` +
		`{"repository":"hanzoai/iam","path":"auth.go","line":142,"score":0.91},` +
		`{"repository":"hanzoai/zip","path":"ctx.go","line":122,"score":0.88},` +
		`{"repository":"hanzoai/kms","path":"seal.go","line":54,"score":0.83},` +
		`{"repository":"hanzoai/gateway","path":"edge.go","line":211,"score":0.79},` +
		`{"repository":"hanzoai/base","path":"store.go","line":33,"score":0.77},` +
		`{"repository":"hanzoai/o11y","path":"trace.go","line":8,"score":0.74},` +
		`{"repository":"hanzoai/commerce","path":"debit.go","line":90,"score":0.71}]`

	// Compact, canonical JSON — the honest wire size to compare against.
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(searchJSON)); err != nil {
		t.Fatal(err)
	}
	jsonBytes := buf.Bytes()

	mdBytes, err := RenderJSON(jsonBytes)
	if err != nil {
		t.Fatal(err)
	}

	if len(mdBytes) >= len(jsonBytes) {
		t.Fatalf("markdown (%d B) not smaller than json (%d B)", len(mdBytes), len(jsonBytes))
	}
	savedPct := 100.0 * float64(len(jsonBytes)-len(mdBytes)) / float64(len(jsonBytes))
	if savedPct < 15.0 {
		t.Fatalf("savings %.1f%% below 15%% floor", savedPct)
	}
	tokPct := 100.0 * float64(estimateTokens(jsonBytes)-estimateTokens(mdBytes)) / float64(estimateTokens(jsonBytes))
	t.Logf("json=%d B (~%d tok)  md=%d B (~%d tok)  saved=%.1f%% bytes, ~%.1f%% tokens",
		len(jsonBytes), estimateTokens(jsonBytes), len(mdBytes), estimateTokens(mdBytes), savedPct, tokPct)
	t.Logf("markdown:\n%s", mdBytes)
}
