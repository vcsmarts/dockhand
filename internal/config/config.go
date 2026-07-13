// Package config loads alias definitions, either from the user's config file
// (~/.config/dockhand/aliases.conf) or from the embedded defaults.
package config

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed defaults.conf
var defaultsConf string

// Picker determines how an alias resolves its target argument(s).
type Picker string

const (
	PickerNone    Picker = "none"    // run the command as-is
	PickerDocker  Picker = "docker"  // fuzzy-pick one running container
	PickerCompose Picker = "compose" // fuzzy-pick one or more compose services
)

// Alias is one entry from aliases.conf.
type Alias struct {
	Name    string
	Picker  Picker
	Command []string
}

// UserConfigPath returns the location of the user's override config,
// whether or not it exists.
func UserConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dockhand", "aliases.conf"), nil
}

// Load returns the aliases from the user config file if present, otherwise
// the embedded defaults. The second return value is the source path
// ("builtin" for the embedded defaults), and the slice preserves file order.
func Load() ([]Alias, string, error) {
	path, err := UserConfigPath()
	if err == nil {
		if data, readErr := os.ReadFile(path); readErr == nil {
			aliases, parseErr := Parse(string(data))
			if parseErr != nil {
				return nil, path, fmt.Errorf("%s: %w", path, parseErr)
			}
			return aliases, path, nil
		}
	}
	aliases, parseErr := Parse(defaultsConf)
	if parseErr != nil {
		return nil, "builtin", parseErr
	}
	return aliases, "builtin", nil
}

// Defaults returns the embedded default config text, e.g. for seeding the
// user's own config file.
func Defaults() string {
	return defaultsConf
}

// Parse reads aliases.conf content. Each non-comment line is
// "<name> <picker> <command...>".
func Parse(content string) ([]Alias, error) {
	var aliases []Alias
	seen := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, fmt.Errorf("line %d: want '<name> <picker> <command...>', got %q", lineNo, line)
		}
		name, picker, command := fields[0], Picker(fields[1]), fields[2:]
		switch picker {
		case PickerNone, PickerDocker, PickerCompose:
		default:
			return nil, fmt.Errorf("line %d: unknown picker %q (want none, docker, or compose)", lineNo, picker)
		}
		if seen[name] {
			return nil, fmt.Errorf("line %d: duplicate alias %q", lineNo, name)
		}
		seen[name] = true
		aliases = append(aliases, Alias{Name: name, Picker: picker, Command: command})
	}
	return aliases, scanner.Err()
}

// Find returns the alias with the given name, if any.
func Find(aliases []Alias, name string) (Alias, bool) {
	for _, a := range aliases {
		if a.Name == name {
			return a, true
		}
	}
	return Alias{}, false
}
