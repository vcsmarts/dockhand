package config

import (
	"strings"
	"testing"
)

func TestParseDefaults(t *testing.T) {
	aliases, err := Parse(Defaults())
	if err != nil {
		t.Fatalf("embedded defaults must parse: %v", err)
	}
	if len(aliases) != 10 {
		t.Fatalf("want 10 default aliases, got %d", len(aliases))
	}
	dl, ok := Find(aliases, "dl")
	if !ok {
		t.Fatal("default alias dl missing")
	}
	if dl.Picker != PickerDocker || strings.Join(dl.Command, " ") != "docker logs" {
		t.Errorf("dl = %+v, want docker picker running 'docker logs'", dl)
	}
}

func TestParseErrors(t *testing.T) {
	for name, content := range map[string]string{
		"too few fields": "dl docker",
		"bad picker":     "dl magic docker logs",
		"duplicate":      "dl docker docker logs\ndl none docker ps",
	} {
		if _, err := Parse(content); err == nil {
			t.Errorf("%s: want error, got none", name)
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
