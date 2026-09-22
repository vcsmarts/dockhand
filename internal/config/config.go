// Package config loads alias definitions, either from the user's config file
// (~/.config/dockhand/aliases.conf) or from the embedded defaults.
package config

import (
	"bufio"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed defaults.conf
var defaultsConf string

// Picker determines how an alias resolves its target argument(s).
type Picker string

const (
	PickerNone      Picker = "none"       // run the command as-is
	PickerDocker    Picker = "docker"     // fuzzy-pick one running container
	PickerDockerAll Picker = "docker-all" // fuzzy-pick one container, running or not
	PickerCompose   Picker = "compose"    // fuzzy-pick one or more compose services
)

// TargetPlaceholder marks where the picked target(s) go in an alias command.
// Without it, targets are appended after the user's extra args.
const TargetPlaceholder = "{}"

// DefaultArgsSeparator separates the command from arguments that are used
// only when the user passes none: "docker exec -it {} -- sh".
const DefaultArgsSeparator = "--"

// Reserved lists names that cannot be used as aliases because `dockhand <name>`
// would be interpreted as a subcommand, or because it is the binary itself.
var Reserved = []string{"dockhand", "setup", "list", "init-config", "help"}

// Alias is one entry from aliases.conf.
type Alias struct {
	Name        string
	Picker      Picker
	Command     []string
	DefaultArgs []string // appended in place of the user's args when they pass none
}

// NeedsTarget reports whether the alias's picker produces target(s).
func (a Alias) NeedsTarget() bool { return a.Picker != PickerNone }

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
//
// Only a missing user file falls back to the defaults; any other read error
// (permissions, I/O) is reported so a broken config never silently
// disappears.
func Load() ([]Alias, string, error) {
	path, err := UserConfigPath()
	if err == nil {
		data, readErr := os.ReadFile(path)
		switch {
		case readErr == nil:
			aliases, parseErr := Parse(string(data))
			if parseErr != nil {
				return nil, path, fmt.Errorf("%s: %w", path, parseErr)
			}
			return aliases, path, nil
		case !errors.Is(readErr, fs.ErrNotExist):
			return nil, path, readErr
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
		a := Alias{Name: fields[0], Picker: Picker(fields[1]), Command: fields[2:]}
		for i, f := range a.Command {
			if f == DefaultArgsSeparator {
				a.Command, a.DefaultArgs = a.Command[:i], a.Command[i+1:]
				break
			}
		}
		if err := a.validate(); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if seen[a.Name] {
			return nil, fmt.Errorf("line %d: duplicate alias %q", lineNo, a.Name)
		}
		seen[a.Name] = true
		aliases = append(aliases, a)
	}
	return aliases, scanner.Err()
}

func (a Alias) validate() error {
	switch {
	case a.Name == "." || a.Name == "..",
		strings.ContainsAny(a.Name, `/\`),
		strings.HasPrefix(a.Name, "-"):
		return fmt.Errorf("invalid alias name %q: must be a plain file name not starting with '-'", a.Name)
	}
	for _, r := range Reserved {
		if a.Name == r {
			return fmt.Errorf("alias name %q is reserved (dockhand subcommand)", a.Name)
		}
	}
	switch a.Picker {
	case PickerNone, PickerDocker, PickerDockerAll, PickerCompose:
	default:
		return fmt.Errorf("unknown picker %q (want none, docker, docker-all, or compose)", a.Picker)
	}
	placeholders := 0
	for _, c := range a.Command {
		if c == TargetPlaceholder {
			placeholders++
		}
	}
	switch {
	case placeholders > 1:
		return fmt.Errorf("command may contain %s at most once", TargetPlaceholder)
	case placeholders == 1 && !a.NeedsTarget():
		return fmt.Errorf("%s placeholder needs a picker other than none", TargetPlaceholder)
	}
	if len(a.Command) == 0 {
		return fmt.Errorf("command is empty (nothing before %s)", DefaultArgsSeparator)
	}
	if a.Command[0] == TargetPlaceholder {
		return fmt.Errorf("command cannot start with %s", TargetPlaceholder)
	}
	if a.DefaultArgs != nil && len(a.DefaultArgs) == 0 {
		return fmt.Errorf("nothing after %s (default args)", DefaultArgsSeparator)
	}
	return nil
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
