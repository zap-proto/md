package md

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const maxHeading = 6

// emit writes the document root. The root carries no title, so a root object or
// array renders its body directly; nested objects/arrays acquire headings.
func emit(buf *bytes.Buffer, v any) {
	switch t := v.(type) {
	case *omap:
		emitObject(buf, t, 2)
	case []any:
		emitArray(buf, t, 2)
	default:
		buf.WriteString(scalarInline(t))
		buf.WriteByte('\n')
	}
}

// emitArray renders a table for a uniform array of scalar objects, otherwise a
// section (objects/arrays) or bullet (scalars) per element.
func emitArray(buf *bytes.Buffer, arr []any, level int) {
	if len(arr) == 0 {
		buf.WriteString("_(empty)_\n")
		return
	}
	if tableable(arr) {
		emitTable(buf, arr)
		return
	}
	for i, el := range arr {
		switch e := el.(type) {
		case *omap:
			writeHeading(buf, level, fmt.Sprintf("Item %d", i+1))
			emitObject(buf, e, level+1)
		case []any:
			writeHeading(buf, level, fmt.Sprintf("Item %d", i+1))
			emitArray(buf, e, level+1)
		default:
			buf.WriteString("- ")
			buf.WriteString(scalarInline(e))
			buf.WriteByte('\n')
		}
	}
}

// emitObject renders an object as a key-ordered list, promoting nested
// objects/object-arrays to sub-sections and code-ish strings to fenced blocks.
func emitObject(buf *bytes.Buffer, m *omap, level int) {
	lang := langHint(m)
	for i, k := range m.keys {
		switch val := m.vals[i].(type) {
		case *omap:
			writeHeading(buf, level, k)
			emitObject(buf, val, level+1)
		case []any:
			switch {
			case tableable(val):
				writeHeading(buf, level, k)
				emitTable(buf, val)
			case isScalarArray(val):
				buf.WriteString("- **")
				buf.WriteString(k)
				buf.WriteString("**: ")
				buf.WriteString(joinScalars(val))
				buf.WriteByte('\n')
			default:
				writeHeading(buf, level, k)
				emitArray(buf, val, level+1)
			}
		default:
			if s, ok := val.(string); ok && isCodeField(k, s) {
				emitCodeField(buf, k, s, codeLang(k, lang))
				continue
			}
			buf.WriteString("- **")
			buf.WriteString(k)
			buf.WriteString("**: ")
			buf.WriteString(scalarInline(val))
			buf.WriteByte('\n')
		}
	}
}

// emitTable writes the array as a GitHub-flavored table: header once, then one
// row per object. This is the token win over JSON's per-element key repetition.
func emitTable(buf *bytes.Buffer, arr []any) {
	cols := tableColumns(arr)
	buf.WriteByte('|')
	for _, c := range cols {
		buf.WriteByte(' ')
		buf.WriteString(cellEscape(c))
		buf.WriteString(" |")
	}
	buf.WriteByte('\n')
	buf.WriteByte('|')
	for range cols {
		buf.WriteString(" --- |")
	}
	buf.WriteByte('\n')
	for _, el := range arr {
		m := el.(*omap)
		buf.WriteByte('|')
		for _, c := range cols {
			buf.WriteByte(' ')
			if v, ok := m.get(c); ok {
				buf.WriteString(cellEscape(scalarInline(v)))
			}
			buf.WriteString(" |")
		}
		buf.WriteByte('\n')
	}
}

func writeHeading(buf *bytes.Buffer, level int, title string) {
	if level > maxHeading {
		level = maxHeading
	}
	if buf.Len() > 0 {
		buf.WriteByte('\n')
	}
	buf.WriteString(strings.Repeat("#", level))
	buf.WriteByte(' ')
	buf.WriteString(title)
	buf.WriteString("\n\n")
}

func emitCodeField(buf *bytes.Buffer, key, content, lang string) {
	if buf.Len() > 0 {
		buf.WriteByte('\n')
	}
	buf.WriteString("**")
	buf.WriteString(key)
	buf.WriteString("**:\n\n")
	fence := fenceFor(content)
	buf.WriteString(fence)
	buf.WriteString(lang)
	buf.WriteByte('\n')
	buf.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		buf.WriteByte('\n')
	}
	buf.WriteString(fence)
	buf.WriteByte('\n')
}

// tableColumns is the union of object keys across the array in first-seen order.
func tableColumns(arr []any) []string {
	var cols []string
	seen := map[string]bool{}
	for _, el := range arr {
		m, ok := el.(*omap)
		if !ok {
			continue
		}
		for _, k := range m.keys {
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	return cols
}

// tableable reports whether arr is a non-empty array of objects whose every
// value is a scalar — the only shape that renders as a clean table.
func tableable(arr []any) bool {
	if len(arr) == 0 {
		return false
	}
	for _, el := range arr {
		m, ok := el.(*omap)
		if !ok {
			return false
		}
		for _, v := range m.vals {
			if !isScalar(v) {
				return false
			}
		}
	}
	return true
}

func isScalar(v any) bool {
	switch v.(type) {
	case *omap, []any:
		return false
	default:
		return true
	}
}

func isScalarArray(arr []any) bool {
	for _, v := range arr {
		if !isScalar(v) {
			return false
		}
	}
	return true
}

func joinScalars(arr []any) string {
	parts := make([]string, len(arr))
	for i, v := range arr {
		parts[i] = scalarInline(v)
	}
	return strings.Join(parts, ", ")
}

func scalarInline(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// cellEscape makes a scalar safe inside a table cell. Pipes are escaped and
// newlines become <br>; ParseTable reverses both.
func cellEscape(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "<br>")
	return s
}

var codeKeys = map[string]bool{
	"diff": true, "patch": true, "code": true, "snippet": true, "source": true,
	"stdout": true, "stderr": true, "traceback": true, "stacktrace": true,
}

// isCodeField reports whether a string value should render as a fenced block:
// any multi-line string, or a value under a code-ish key.
func isCodeField(key, val string) bool {
	if strings.Contains(val, "\n") {
		return true
	}
	return codeKeys[strings.ToLower(key)]
}

var langKeys = map[string]bool{
	"language": true, "lang": true, "filetype": true, "ext": true,
}

// langHint returns the first sibling language field's value, used as the fence
// language for code fields in the same object.
func langHint(m *omap) string {
	for i, k := range m.keys {
		if langKeys[strings.ToLower(k)] {
			if s, ok := m.vals[i].(string); ok {
				return s
			}
		}
	}
	return ""
}

func codeLang(key, hint string) string {
	switch strings.ToLower(key) {
	case "diff", "patch":
		return "diff"
	}
	return hint
}

// fenceFor returns a backtick fence longer than any backtick run in content, so
// content is never able to break out of its own fence.
func fenceFor(content string) string {
	longest, run := 0, 0
	for _, r := range content {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	n := 3
	if longest >= 3 {
		n = longest + 1
	}
	return strings.Repeat("`", n)
}
