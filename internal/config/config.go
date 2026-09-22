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
	PickerNone          Picker = "none"           // run the command as-is
	PickerDocker        Picker = "docker"         // fuzzy-pick one running container
	PickerDockerAll     Picker = "docker-all"     // fuzzy-pick one container, running or not
	PickerCompose       Picker = "compose"        // fuzzy-pick one or more compose services
	PickerKubeContext   Picker = "kube-context"   // fuzzy-pick one kubectl context
	PickerKubeNamespace Picker = "kube-namespace" // fuzzy-pick one namespace
	PickerKubePod       Picker = "kube-pod"       // fuzzy-pick one pod (honours -n/--context in your args)
)

// Pickers lists every valid picker, in the order shown in error messages.
var Pickers = []Picker{
	PickerNone, PickerDocker, PickerDockerAll, PickerCompose,
	PickerKubeContext, PickerKubeNamespace, PickerKubePod,
}

// TargetPlaceholder marks where the picked target(s) go in an alias command.
// Without it, targets are appended after the user's extra args.
const TargetPlaceholder = "{}"

// Default args are written as a trailing bracket group and are used only
// when the user passes no args: "kubectl exec -it {} -- [sh]". Brackets are
// used because "--" is itself a meaningful argument to kubectl and docker.
const (
	DefaultArgsOpen  = "["
	DefaultArgsClose = "]"
)

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
		a := Alias{Name: fields[0], Picker: Picker(fields[1])}
		var err error
		if a.Command, a.DefaultArgs, err = splitDefaultArgs(fields[2:]); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
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

// splitDefaultArgs separates a trailing "[a b c]" group from the command.
// defaults is nil when there is no group.
func splitDefaultArgs(tokens []string) (command, defaults []string, err error) {
	open := -1
	for i, t := range tokens {
		if strings.HasPrefix(t, DefaultArgsOpen) {
			if open >= 0 {
				return nil, nil, fmt.Errorf("only one %s...%s default-args group is allowed", DefaultArgsOpen, DefaultArgsClose)
			}
			open = i
		}
	}
	if open < 0 {
		return tokens, nil, nil
	}
	last := tokens[len(tokens)-1]
	if !strings.HasSuffix(last, DefaultArgsClose) {
		return nil, nil, fmt.Errorf("%s default-args group must close with %s at the end of the line", DefaultArgsOpen, DefaultArgsClose)
	}
	group := append([]string{}, tokens[open:]...)
	group[0] = strings.TrimPrefix(group[0], DefaultArgsOpen)
	group[len(group)-1] = strings.TrimSuffix(group[len(group)-1], DefaultArgsClose)
	defaults = make([]string, 0, len(group))
	for _, g := range group {
		if g != "" {
			defaults = append(defaults, g)
		}
	}
	if len(defaults) == 0 {
		return nil, nil, fmt.Errorf("empty %s%s default-args group", DefaultArgsOpen, DefaultArgsClose)
	}
	return tokens[:open], defaults, nil
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
	valid := false
	for _, p := range Pickers {
		if a.Picker == p {
			valid = true
			break
		}
	}
	if !valid {
		names := make([]string, len(Pickers))
		for i, p := range Pickers {
			names[i] = string(p)
		}
		return fmt.Errorf("unknown picker %q (want one of %s)", a.Picker, strings.Join(names, ", "))
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
		return fmt.Errorf("command is empty (nothing before the %s...%s group)", DefaultArgsOpen, DefaultArgsClose)
	}
	if a.Command[0] == TargetPlaceholder {
		return fmt.Errorf("command cannot start with %s", TargetPlaceholder)
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
