package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseDefaults(t *testing.T) {
	cfg, err := Parse(Defaults())
	if err != nil {
		t.Fatalf("embedded defaults must parse: %v", err)
	}
	if len(cfg.Aliases) == 0 || len(cfg.Pickers) == 0 {
		t.Fatal("embedded defaults define no aliases or no pickers")
	}
	wantAliases := map[string]struct{ picker, command string }{
		"dl":    {"container-all", "docker logs"},
		"dexec": {"container", "docker exec -it {}"},
		"dcl":   {"service", "docker compose logs"},
		"dps":   {PickerNone, "docker ps"},
		"kctx":  {"context", "kubectl config use-context {}"},
		"kl":    {"pod", "kubectl logs"},
		"kexec": {"pod", "kubectl exec -it {} --"},
	}
	for name, w := range wantAliases {
		a, ok := cfg.FindAlias(name)
		if !ok {
			t.Errorf("default alias %s missing", name)
			continue
		}
		if a.Picker != w.picker || strings.Join(a.Command, " ") != w.command {
			t.Errorf("%s = %+v, want picker %s running %q", name, a, w.picker, w.command)
		}
	}
	for _, name := range []string{"dexec", "kexec"} {
		a, _ := cfg.FindAlias(name)
		if strings.Join(a.DefaultArgs, " ") != "sh" {
			t.Errorf("%s default args = %q, want sh", name, a.DefaultArgs)
		}
	}

	pod, ok := cfg.FindPicker("pod")
	if !ok || !pod.Header || pod.Multi || pod.Column != "" {
		t.Errorf("pod picker = %+v, want header, single, first column", pod)
	}
	if want := []string{"-n", "--namespace", "--context", "--kubeconfig"}; !reflect.DeepEqual(pod.Forward, want) {
		t.Errorf("pod forward = %q, want %q", pod.Forward, want)
	}
	c, _ := cfg.FindPicker("container")
	if want := []string{"docker", "ps", "--format", `table {{.Names}}\t{{.Image}}\t{{.Status}}`}; !reflect.DeepEqual(c.Command, want) {
		t.Errorf("container command = %q, want %q (quotes must group the format)", c.Command, want)
	}
	if c.Column != "NAMES" || !c.Header {
		t.Errorf("container picker = %+v, want header col=NAMES", c)
	}
	if s, _ := cfg.FindPicker("service"); !s.Multi {
		t.Errorf("service picker must be multi")
	}
}

func TestParsePicker(t *testing.T) {
	cfg, err := Parse("picker thing header multi col=3 forward=-n,--ctx mytool list --all\nx thing mytool show\n")
	if err != nil {
		t.Fatal(err)
	}
	want := Picker{Name: "thing", Header: true, Multi: true, Column: "3", Forward: []string{"-n", "--ctx"}, Command: []string{"mytool", "list", "--all"}}
	if got := cfg.Pickers[0]; !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestParsePickerDefinedAfterUse(t *testing.T) {
	if _, err := Parse("x thing mytool show\npicker thing mytool list\n"); err != nil {
		t.Fatalf("picker defined after the alias using it must be fine: %v", err)
	}
}

func TestParseErrors(t *testing.T) {
	for _, tc := range []struct{ name, content, wantErr string }{
		{"too few fields", "dl container", "want '<name> <picker> <command...>'"},
		{"undefined picker", "dl magic docker logs", "undefined picker"},
		{"duplicate alias", "dl none docker logs\ndl none docker ps", "duplicate alias"},
		{"duplicate picker", "picker c docker ps\npicker c docker ps", "duplicate picker"},
		{"reserved name", "setup none docker ps", "reserved"},
		{"alias named picker", "picker", "want 'picker <name>"},
		{"path separator", "../evil none docker ps", "invalid name"},
		{"dash prefix", "-x none docker ps", "invalid name"},
		{"dotdot", ".. none docker ps", "invalid name"},
		{"placeholder without picker", "dps none docker ps {}", "needs a picker"},
		{"two placeholders", "picker c docker ps\ndx c docker exec {} {}", "at most once"},
		{"placeholder first", "picker c docker ps\ndx c {} exec", "cannot start with"},
		{"empty default args", "picker c docker ps\ndx c docker exec -it {} []", "empty [] default-args group"},
		{"empty command before group", "picker c docker ps\ndx c [sh]", "command is empty"},
		{"unclosed group", "picker c docker ps\ndx c docker exec -it {} [sh -l", "must close with ]"},
		{"group not at end", "picker c docker ps\ndx c docker exec [sh] -it {}", "must close with ]"},
		{"two groups", "picker c docker ps\ndx c docker exec {} [a] [b]", "only one"},
		{"unterminated quote", `dps none docker ps --format "oops`, "unterminated quote"},
		{"picker named none", "picker none docker ps", "reserved"},
		{"picker without command", "picker c header multi", "no list command"},
		{"picker col zero", "picker c col=0 docker ps", "numbered from 1"},
		{"picker col title without header", "picker c col=NAMES docker ps", "needs the 'header' option"},
		{"picker bad forward", "picker c forward=n docker ps", "is not a flag"},
	} {
		_, err := Parse(tc.content)
		if err == nil {
			t.Errorf("%s: want error, got none", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: error %q does not mention %q", tc.name, err, tc.wantErr)
		}
	}
}

func TestParseIgnoresCommentsAndBlanks(t *testing.T) {
	cfg, err := Parse("# comment\n\n   dtop   none   docker top\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Aliases) != 1 || cfg.Aliases[0].Name != "dtop" {
		t.Fatalf("got %+v, want single dtop alias", cfg.Aliases)
	}
}

func TestParseDefaultArgsKeepsLiteralDashDash(t *testing.T) {
	cfg, err := Parse("picker pod kubectl get pods\nkexec pod kubectl exec -it {} -- [sh]\n")
	if err != nil {
		t.Fatal(err)
	}
	a := cfg.Aliases[0]
	if got := strings.Join(a.Command, " "); got != "kubectl exec -it {} --" {
		t.Errorf("command = %q, literal -- must survive", got)
	}
	if got := strings.Join(a.DefaultArgs, " "); got != "sh" {
		t.Errorf("default args = %q", got)
	}
}

func TestParseDefaultArgs(t *testing.T) {
	cfg, err := Parse("picker c docker ps\ndexec c docker exec -it {} [sh -l]\ndps none docker ps\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(cfg.Aliases[0].DefaultArgs, " "); got != "sh -l" {
		t.Errorf("default args = %q, want \"sh -l\"", got)
	}
	if cfg.Aliases[1].DefaultArgs != nil {
		t.Errorf("dps default args = %q, want none", cfg.Aliases[1].DefaultArgs)
	}
}

// setUserConfig points UserConfigPath at a temp dir and returns the path the
// user config would have there.
func setUserConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	path, err := UserConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, dir) {
		t.Skipf("UserConfigPath %q ignores XDG_CONFIG_HOME on this platform", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadFallsBackToBuiltinWhenMissing(t *testing.T) {
	setUserConfig(t)
	cfg, source, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if source != "builtin" || len(cfg.Aliases) == 0 {
		t.Fatalf("source=%q, %d aliases; want builtin defaults", source, len(cfg.Aliases))
	}
}

func TestLoadUsesUserConfig(t *testing.T) {
	path := setUserConfig(t)
	if err := os.WriteFile(path, []byte("dtop none docker top\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, source, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if source != path || len(cfg.Aliases) != 1 || cfg.Aliases[0].Name != "dtop" {
		t.Fatalf("source=%q aliases=%+v; want user config with dtop", source, cfg.Aliases)
	}
}

func TestLoadReportsUnreadableUserConfig(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can read mode-000 files")
	}
	path := setUserConfig(t)
	if err := os.WriteFile(path, []byte("dtop none docker top\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(); err == nil {
		t.Fatal("unreadable user config silently fell back to builtin defaults")
	}
}

func TestLoadReportsBrokenUserConfig(t *testing.T) {
	path := setUserConfig(t)
	if err := os.WriteFile(path, []byte("dl magic docker logs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := Load()
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("err = %v; want parse error naming %s", err, path)
	}
}
