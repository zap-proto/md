package md

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

// Table is the parsed form of a rendered markdown table: the column headers and
// the string cells of each row, in order.
type Table struct {
	Columns []string
	Rows    [][]string
}

// ParseTable recovers a Table from markdown produced by Render for a uniform
// array of objects. It is the inverse of the headline table case and gives a
// concrete round-trip; cells come back as strings (markdown is a display
// encoding — value types are not carried). The first table found is returned.
func ParseTable(src []byte) (*Table, error) {
	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	var header []string
	sawSeparator := false
	tbl := &Table{}
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "|") {
			if header != nil {
				break // table ended
			}
			continue // still scanning for a table
		}
		cells := splitRow(line)
		switch {
		case header == nil:
			header = cells
		case !sawSeparator:
			if !isSeparatorRow(cells) {
				return nil, fmt.Errorf("md: expected table separator, got %q", line)
			}
			sawSeparator = true
			tbl.Columns = header
		default:
			tbl.Rows = append(tbl.Rows, cells)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if !sawSeparator {
		return nil, fmt.Errorf("md: no table found")
	}
	return tbl, nil
}

// splitRow splits a "| a | b |" line into trimmed, unescaped cells.
func splitRow(line string) []string {
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	raw := strings.Split(line, "|")
	cells := make([]string, len(raw))
	for i, c := range raw {
		cells[i] = cellUnescape(strings.TrimSpace(c))
	}
	return cells
}

func isSeparatorRow(cells []string) bool {
	for _, c := range cells {
		c = strings.TrimSpace(c)
		if c == "" || strings.Trim(c, "-:") != "" {
			return false
		}
	}
	return len(cells) > 0
}

func cellUnescape(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "\\|", "|")
	return s
}
