package picker

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseTableHeaderPositions(t *testing.T) {
	// kubectl config get-contexts: empty first cell on non-current rows,
	// header title with no space before it, trailing spaces.
	out := "CURRENT   NAME         CLUSTER      AUTHINFO     NAMESPACE\n" +
		"*         argo-clstr   argo-clstr   argo-user    \n" +
		"          other        c2           u2           team-a\n"
	titles, rows := ParseTable(out, true)
	if want := []string{"CURRENT", "NAME", "CLUSTER", "AUTHINFO", "NAMESPACE"}; !reflect.DeepEqual(titles, want) {
		t.Errorf("titles = %q, want %q", titles, want)
	}
	want := [][]string{
		{"*", "argo-clstr", "argo-clstr", "argo-user", ""},
		{"", "other", "c2", "u2", "team-a"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %q\nwant  %q", rows, want)
	}
}

func TestParseTableKeepsSingleSpacesInCells(t *testing.T) {
	out := "NAMES         IMAGE         STATUS\n" +
		"web           nginx:1.27    Up 3 hours\n" +
		"db            postgres:16   Exited (0) 2 minutes ago\n"
	_, rows := ParseTable(out, true)
	if got := rows[1][2]; got != "Exited (0) 2 minutes ago" {
		t.Errorf("status = %q", got)
	}
	// kubectl pods: RESTARTS "3 (2m ago)" and a two-word header title
	out = "NAME    READY   STATUS    RESTARTS      AGE   NOMINATED NODE\n" +
		"api     2/2     Running   3 (2m ago)    12h   <none>\n"
	titles, rows := ParseTable(out, true)
	if titles[5] != "NOMINATED NODE" || rows[0][3] != "3 (2m ago)" || rows[0][5] != "<none>" {
		t.Errorf("titles=%q rows=%q", titles, rows)
	}
}

func TestParseTableTabs(t *testing.T) {
	titles, rows := ParseTable("NAME\tIMAGE\nweb\tnginx\n", true)
	if !reflect.DeepEqual(titles, []string{"NAME", "IMAGE"}) || !reflect.DeepEqual(rows, [][]string{{"web", "nginx"}}) {
		t.Errorf("titles=%q rows=%q", titles, rows)
	}
	_, rows = ParseTable("web\tnginx\tUp 3 hours\n", false)
	if !reflect.DeepEqual(rows, [][]string{{"web", "nginx", "Up 3 hours"}}) {
		t.Errorf("rows=%q", rows)
	}
}

func TestParseTableNoHeader(t *testing.T) {
	titles, rows := ParseTable("web\n\ndb\n  api  \n", false)
	if titles != nil {
		t.Errorf("titles must be nil without header, got %q", titles)
	}
	if want := [][]string{{"web"}, {"db"}, {"api"}}; !reflect.DeepEqual(rows, want) {
		t.Errorf("rows=%q want %q", rows, want)
	}
	_, rows = ParseTable("a  b   c d\n", false)
	if want := [][]string{{"a", "b", "c d"}}; !reflect.DeepEqual(rows, want) {
		t.Errorf("two-space split: rows=%q want %q", rows, want)
	}
}

func TestParseTableEmpty(t *testing.T) {
	if titles, rows := ParseTable("", true); titles != nil || rows != nil {
		t.Errorf("got %q %q", titles, rows)
	}
	if _, rows := ParseTable("NAME\n", true); rows != nil {
		t.Errorf("header only must yield no rows, got %q", rows)
	}
}

func TestResolveColumn(t *testing.T) {
	titles := []string{"CURRENT", "NAME", "AGE"}
	for _, tc := range []struct {
		col  string
		want int
		err  bool
	}{
		{"", 0, false}, {"1", 0, false}, {"3", 2, false}, {"NAME", 1, false}, {"name", 1, false}, {"NOPE", 0, true},
	} {
		got, err := resolveColumn(tc.col, titles)
		if (err != nil) != tc.err || got != tc.want {
			t.Errorf("resolveColumn(%q) = %d, %v; want %d, err=%v", tc.col, got, err, tc.want, tc.err)
		}
	}
}

func TestForward(t *testing.T) {
	flags := []string{"-n", "--namespace", "--context", "--kubeconfig"}
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"-f --tail 10", ""},
		{"-n kube-system -f", "-n kube-system"},
		{"-f -n kube-system", "-n kube-system"},
		{"--namespace=monitoring", "--namespace=monitoring"},
		{"--namespace monitoring --tail 5", "--namespace monitoring"},
		{"--context prod -n web --kubeconfig /tmp/kc", "--context prod -n web --kubeconfig /tmp/kc"},
		{"--context=prod", "--context=prod"},
		{"-n", ""},
		{"-A", ""},
	} {
		got := strings.Join(Forward(flags, strings.Fields(tc.in)), " ")
		if got != tc.want {
			t.Errorf("Forward(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if got := Forward(nil, []string{"-n", "x"}); got != nil {
		t.Errorf("no flags configured: got %q, want nil", got)
	}
}

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

func TestTableRaggedRows(t *testing.T) {
	tb := newTable(nil, [][]string{{"a"}, {"bb", "c"}})
	if got := tb.Line([]string{"a"}); got != "a" {
		t.Errorf("got %q", got)
	}
	if got := tb.Line([]string{"bb", "c"}); got != "bb | c" {
		t.Errorf("got %q", got)
	}
}
