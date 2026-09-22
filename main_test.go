package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsurePathLine(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".local", "bin")
	rc := filepath.Join(home, ".bashrc")

	added, err := ensurePathLine(rc, dir)
	if err != nil || !added {
		t.Fatalf("first call on missing rc: added=%v err=%v", added, err)
	}
	content, _ := os.ReadFile(rc)
	if !strings.Contains(string(content), `export PATH="`+dir+`:$PATH"`) {
		t.Fatalf("rc does not contain export line:\n%s", content)
	}

	added, err = ensurePathLine(rc, dir)
	if err != nil || added {
		t.Fatalf("second call must be a no-op: added=%v err=%v", added, err)
	}
	if n := strings.Count(string(content), "export PATH"); n != 1 {
		t.Fatalf("export line duplicated %d times", n)
	}
}

func TestEnsurePathLineAppendsAfterMissingNewline(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	rc := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(rc, []byte("alias ll='ls -l'"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ensurePathLine(rc, "/opt/bin"); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(rc)
	if !strings.HasPrefix(string(content), "alias ll='ls -l'\n\n# added by dockhand setup\n") {
		t.Fatalf("existing last line was corrupted:\n%s", content)
	}
}

func TestPathLineMentions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".local", "bin")
	for content, want := range map[string]bool{
		`export PATH="` + dir + `:$PATH"`:        true,
		`export PATH="$HOME/.local/bin:$PATH"`:   true,
		`export PATH="${HOME}/.local/bin:$PATH"`: true,
		`PATH=~/.local/bin:$PATH`:                true,
		`# export PATH="$HOME/.local/bin:$PATH"`: false,
		`export PATH="$HOME/bin:$PATH"`:          false,
		`echo ` + dir:                            false,
		"":                                       false,
		"alias x=y\n  export PATH=\"$HOME/.local/bin:$PATH\"\n": true,
	} {
		if got := pathLineMentions(content, dir); got != want {
			t.Errorf("%q: got %v, want %v", content, got, want)
		}
	}
}
