package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDefaults(t *testing.T) {
	aliases, err := Parse(Defaults())
	if err != nil {
		t.Fatalf("embedded defaults must parse: %v", err)
	}
	if len(aliases) == 0 {
		t.Fatal("embedded defaults define no aliases")
	}
	want := map[string]struct {
		picker  Picker
		command string
	}{
		"dl":    {PickerDockerAll, "docker logs"},
		"dexec": {PickerDocker, "docker exec -it {}"},
		"dcl":   {PickerCompose, "docker compose logs"},
		"dps":   {PickerNone, "docker ps"},
	}
	for name, w := range want {
		a, ok := Find(aliases, name)
		if !ok {
			t.Errorf("default alias %s missing", name)
			continue
		}
		if a.Picker != w.picker || strings.Join(a.Command, " ") != w.command {
			t.Errorf("%s = %+v, want %s picker running %q", name, a, w.picker, w.command)
		}
	}
	dexec, _ := Find(aliases, "dexec")
	if strings.Join(dexec.DefaultArgs, " ") != "sh" {
		t.Errorf("dexec default args = %q, want sh so bare dexec opens a shell", dexec.DefaultArgs)
	}
}

func TestParseDefaultArgs(t *testing.T) {
	aliases, err := Parse("dexec docker docker exec -it {} -- sh -l\ndps none docker ps\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(aliases[0].Command, " "); got != "docker exec -it {}" {
		t.Errorf("command = %q", got)
	}
	if got := strings.Join(aliases[0].DefaultArgs, " "); got != "sh -l" {
		t.Errorf("default args = %q, want \"sh -l\"", got)
	}
	if aliases[1].DefaultArgs != nil {
		t.Errorf("dps default args = %q, want none", aliases[1].DefaultArgs)
	}
}

func TestParseErrors(t *testing.T) {
	for _, tc := range []struct{ name, content, wantErr string }{
		{"too few fields", "dl docker", "want '<name> <picker> <command...>'"},
		{"bad picker", "dl magic docker logs", "unknown picker"},
		{"duplicate", "dl docker docker logs\ndl none docker ps", "duplicate alias"},
		{"reserved name", "list none docker ps", "reserved"},
		{"path separator", "../evil none docker ps", "invalid alias name"},
		{"dash prefix", "-x none docker ps", "invalid alias name"},
		{"dotdot", ".. none docker ps", "invalid alias name"},
		{"placeholder without picker", "dps none docker ps {}", "needs a picker"},
		{"two placeholders", "dx docker docker exec {} {}", "at most once"},
		{"placeholder first", "dx docker {} exec", "cannot start with"},
		{"empty default args", "dx docker docker exec -it {} --", "nothing after --"},
		{"empty command before --", "dx docker -- sh", "command is empty"},
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
	aliases, err := Parse("# comment\n\n   dtop   docker   docker top\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(aliases) != 1 || aliases[0].Name != "dtop" {
		t.Fatalf("got %+v, want single dtop alias", aliases)
	}
}

func TestParseAcceptsPlaceholder(t *testing.T) {
	aliases, err := Parse("dexec docker docker exec -it {}\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(aliases[0].Command, " "); got != "docker exec -it {}" {
		t.Fatalf("command = %q", got)
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
	aliases, source, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if source != "builtin" || len(aliases) == 0 {
		t.Fatalf("source=%q, %d aliases; want builtin defaults", source, len(aliases))
	}
}

func TestLoadUsesUserConfig(t *testing.T) {
	path := setUserConfig(t)
	if err := os.WriteFile(path, []byte("dtop docker docker top\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	aliases, source, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if source != path || len(aliases) != 1 || aliases[0].Name != "dtop" {
		t.Fatalf("source=%q aliases=%+v; want user config with dtop", source, aliases)
	}
}

func TestLoadReportsUnreadableUserConfig(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can read mode-000 files")
	}
	path := setUserConfig(t)
	if err := os.WriteFile(path, []byte("dtop docker docker top\n"), 0o000); err != nil {
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
