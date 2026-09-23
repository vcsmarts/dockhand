package picker

import "testing"

func TestTable(t *testing.T) {
	rows := [][]string{
		{"web-7d9f8b6c5-abcde", "1/1", "Running", "0", "3d2h"},
		{"api", "2/2", "CrashLoopBackOff", "39 (3m1s ago)", "12h"},
	}
	tb := newTable([]string{"NAME", "READY", "STATUS", "RESTARTS", "AGE"}, rows)
	want := []string{
		"web-7d9f8b6c5-abcde | 1/1   | Running          | 0             | 3d2h",
		"api                 | 2/2   | CrashLoopBackOff | 39 (3m1s ago) | 12h",
	}
	for i, r := range rows {
		if got := tb.Line(r); got != want[i] {
			t.Errorf("row %d:\n got  %q\n want %q", i, got, want[i])
		}
	}
	wantHeader := "  NAME                | READY | STATUS           | RESTARTS      | AGE"
	if got := tb.Header(); got != wantHeader {
		t.Errorf("header:\n got  %q\n want %q", got, wantHeader)
	}
}

func TestTableHeaderWiderThanValues(t *testing.T) {
	tb := newTable([]string{"NAME", "IMAGE", "STATUS"}, [][]string{{"a", "b", "c"}})
	if got := tb.Line([]string{"a", "b", "c"}); got != "a    | b     | c" {
		t.Errorf("got %q", got)
	}
}
