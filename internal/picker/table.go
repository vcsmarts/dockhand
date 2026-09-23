package picker

import (
	"strings"
	"unicode/utf8"
)

// table aligns rows into "v1 | v2 | v3" lines, padding every column to the
// widest value in it (header included) so the picker reads like the original
// command's tabular output. Header returns the column titles laid out the
// same way, for use as the picker header.
type table struct {
	widths []int
	header []string
}

func newTable(header []string, rows [][]string) *table {
	t := &table{header: header, widths: make([]int, len(header))}
	t.measure(header)
	for _, r := range rows {
		t.measure(r)
	}
	return t
}

func (t *table) measure(row []string) {
	for i, v := range row {
		if i >= len(t.widths) {
			t.widths = append(t.widths, 0)
		}
		if w := utf8.RuneCountInString(v); w > t.widths[i] {
			t.widths[i] = w
		}
	}
}

// Line renders one row. The last column is not padded, so lines carry no
// trailing whitespace.
func (t *table) Line(row []string) string {
	var b strings.Builder
	for i, v := range row {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(v)
		if i < len(row)-1 {
			b.WriteString(strings.Repeat(" ", t.widths[i]-utf8.RuneCountInString(v)))
		}
	}
	return b.String()
}

// Header renders the column titles, indented to sit above the rows, which the
// fuzzy finder prefixes with a two-character cursor.
func (t *table) Header() string {
	return "  " + t.Line(t.header)
}
