package picker

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// cellSplit separates columns in output that has no header to take
// positions from: a tab, or a run of two or more spaces.
var cellSplit = regexp.MustCompile(`\t|\s{2,}`)

// ParseTable splits command output into rows of cells.
//
// With header, the first non-empty line is the header. If it contains tabs,
// every line is split on tabs. Otherwise columns are cut where the header's
// titles start, which is how `docker ps`, `kubectl get` and most other
// column-aligned tools lay out their output; that keeps empty cells and cells
// containing single spaces ("Exited (0) 2 minutes ago") intact. titles is
// nil when header is false.
//
// Without header, each line is split on tabs or runs of two or more spaces.
func ParseTable(out string, header bool) (titles []string, rows [][]string) {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimRight(l, " \t\r"); strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return nil, nil
	}
	if !header {
		for _, l := range lines {
			rows = append(rows, cellSplit.Split(strings.TrimSpace(l), -1))
		}
		return nil, rows
	}

	head, body := lines[0], lines[1:]
	if strings.Contains(head, "\t") {
		titles = trimAll(strings.Split(head, "\t"))
		for _, l := range body {
			rows = append(rows, trimAll(strings.Split(l, "\t")))
		}
		return titles, rows
	}

	titles = cellSplit.Split(strings.TrimSpace(head), -1)
	offsets := make([]int, len(titles))
	headRunes := []rune(head)
	pos := 0
	for i, t := range titles {
		idx := strings.Index(string(headRunes[pos:]), t)
		offsets[i] = pos + utf8.RuneCountInString(string(headRunes[pos:])[:idx])
		pos = offsets[i] + utf8.RuneCountInString(t)
	}
	for _, l := range body {
		rows = append(rows, cutAt([]rune(l), offsets))
	}
	return titles, rows
}

// cutAt slices line at the given rune offsets, trimming each cell. Cells
// beyond the end of a short line are empty.
func cutAt(line []rune, offsets []int) []string {
	cells := make([]string, len(offsets))
	for i, start := range offsets {
		end := len(line)
		if i+1 < len(offsets) {
			end = offsets[i+1]
		}
		if start > len(line) {
			start = len(line)
		}
		if end > len(line) {
			end = len(line)
		}
		cells[i] = strings.TrimSpace(string(line[start:end]))
	}
	return cells
}

func trimAll(cells []string) []string {
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

// table aligns rows into "v1 | v2 | v3" lines, padding every column to the
// widest value in it (titles included) so the picker reads like the original
// command's tabular output.
type table struct {
	widths []int
	titles []string
}

func newTable(titles []string, rows [][]string) *table {
	t := &table{titles: titles}
	t.measure(titles)
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
	return "  " + t.Line(t.titles)
}
