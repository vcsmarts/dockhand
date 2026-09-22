package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShadowedBy(t *testing.T) {
	root := t.TempDir()
	first, ours := filepath.Join(root, "first"), filepath.Join(root, "ours")
	for _, d := range []string{first, ours} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(ours, "dexec")
	if err := os.WriteFile(link, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", strings.Join([]string{first, ours}, string(os.PathListSeparator)))
	if got := shadowedBy("dexec", link); got != "" {
		t.Errorf("nothing earlier on PATH: got %q, want none", got)
	}

	other := filepath.Join(first, "dexec")
	if err := os.WriteFile(other, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := shadowedBy("dexec", link); got != other {
		t.Errorf("earlier executable: got %q, want %q", got, other)
	}

	if err := os.Chmod(other, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := shadowedBy("dexec", link); got != "" {
		t.Errorf("non-executable file must not count: got %q", got)
	}

	if got := shadowedBy("missing", filepath.Join(ours, "missing")); got != "" {
		t.Errorf("name not on PATH: got %q", got)
	}
}
