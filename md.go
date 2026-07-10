package md

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Render converts a structured value to canonical, token-efficient markdown.
//
// It accepts native Go values (structs, maps, slices, scalars), a
// json.RawMessage, or raw JSON bytes. Anything that is not already JSON is
// normalized through encoding/json, so struct field order and json tags are
// honored and map keys are sorted — the markdown mirrors what the JSON codec
// would emit, only denser. The output is deterministic.
func Render(v any) ([]byte, error) {
	root, err := normalize(v)
	if err != nil {
		return nil, fmt.Errorf("md: %w", err)
	}
	var buf bytes.Buffer
	emit(&buf, root)
	return buf.Bytes(), nil
}

// RenderJSON renders JSON bytes, preserving the source key order. It is the
// entry point a content-negotiation middleware uses to re-serialize a handler's
// already-encoded JSON response as markdown.
func RenderJSON(data []byte) ([]byte, error) {
	return Render(json.RawMessage(data))
}

// normalize reduces any input to the order-preserving intermediate tree
// (*omap / []any / scalar). JSON inputs are parsed directly to keep their key
// order; every other Go value is marshaled once and re-parsed, reusing the
// standard codec for all type/tag handling.
func normalize(v any) (any, error) {
	var data []byte
	switch t := v.(type) {
	case nil:
		return nil, nil
	case string:
		return t, nil
	case json.RawMessage:
		data = t
	case []byte:
		data = t
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		data = b
	}
	return decodeJSON(data)
}

// omap is an order-preserving JSON object: keys and vals are parallel slices in
// source order. A value is one of *omap, []any, string, json.Number, bool, nil.
type omap struct {
	keys []string
	vals []any
}

func (m *omap) get(key string) (any, bool) {
	for i, k := range m.keys {
		if k == key {
			return m.vals[i], true
		}
	}
	return nil, false
}

// decodeJSON parses data into the ordered tree with numbers kept exact.
func decodeJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing data after JSON value")
	}
	return v, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		// string, json.Number, bool, or nil.
		return tok, nil
	}
	switch delim {
	case '{':
		m := &omap{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, ok := kt.(string)
			if !ok {
				return nil, fmt.Errorf("non-string object key %v", kt)
			}
			val, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			m.keys = append(m.keys, key)
			m.vals = append(m.vals, val)
		}
		if _, err := dec.Token(); err != nil { // consume '}'
			return nil, err
		}
		return m, nil
	case '[':
		arr := []any{}
		for dec.More() {
			val, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, val)
		}
		if _, err := dec.Token(); err != nil { // consume ']'
			return nil, err
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("unexpected delimiter %q", delim)
	}
}
