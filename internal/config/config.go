// Package config loads alias and picker definitions, either from the user's
// config file (~/.config/dockhand/aliases.conf) or from the embedded
// defaults.
package config

import (
	"bufio"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed defaults.conf
var defaultsConf string

// PickerNone is the picker name meaning "no target, run the command as-is".
const PickerNone = "none"

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

// pickerKeyword starts a picker definition line.
const pickerKeyword = "picker"

// Reserved lists names that cannot be used as aliases: dockhand's own
// subcommands, the binary itself, and the picker line keyword.
var Reserved = []string{"dockhand", "setup", "list", "init-config", "help", pickerKeyword}

// Picker describes how to list candidates and which column is the target.
// dockhand knows nothing about docker or kubectl; a picker is just a command
// whose tabular output is offered in a fuzzy finder.
type Picker struct {
	Name    string
	Command []string // list command; rows of its stdout are the candidates
	Header  bool     // first output line is a header (column titles)
	Multi   bool     // allow selecting several rows (TAB)
	Column  string   // target column: "" = first, a 1-based number, or a header title
	Forward []string // flags copied from the user's args to the list command (e.g. -n)
}

// Alias is one runnable subcommand.
type Alias struct {
	Name        string
	Picker      string // PickerNone or the name of a Picker
	Command     []string
	DefaultArgs []string // used in place of the user's args when they pass none
}

// NeedsTarget reports whether the alias's picker produces target(s).
func (a Alias) NeedsTarget() bool { return a.Picker != PickerNone }

// Config is the parsed aliases.conf: pickers and aliases in file order.
type Config struct {
	Pickers []Picker
	Aliases []Alias
}

// FindAlias returns the alias with the given name, if any.
func (c *Config) FindAlias(name string) (Alias, bool) {
	for _, a := range c.Aliases {
		if a.Name == name {
			return a, true
		}
	}
	return Alias{}, false
}

// FindPicker returns the picker with the given name, if any.
func (c *Config) FindPicker(name string) (Picker, bool) {
	for _, p := range c.Pickers {
		if p.Name == name {
			return p, true
		}
	}
	return Picker{}, false
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

// Load returns the config from the user file if present, otherwise the
// embedded defaults. The second return value is the source path ("builtin"
// for the embedded defaults).
//
// Only a missing user file falls back to the defaults; any other read error
// (permissions, I/O) is reported so a broken config never silently
// disappears.
func Load() (*Config, string, error) {
	path, err := UserConfigPath()
	if err == nil {
		data, readErr := os.ReadFile(path)
		switch {
		case readErr == nil:
			cfg, parseErr := Parse(string(data))
			if parseErr != nil {
				return nil, path, fmt.Errorf("%s: %w", path, parseErr)
			}
			return cfg, path, nil
		case !errors.Is(readErr, fs.ErrNotExist):
			return nil, path, readErr
		}
	}
	cfg, parseErr := Parse(defaultsConf)
	if parseErr != nil {
		return nil, "builtin", parseErr
	}
	return cfg, "builtin", nil
}

// Defaults returns the embedded default config text, e.g. for seeding the
// user's own config file.
func Defaults() string {
	return defaultsConf
}

// Parse reads aliases.conf content. Lines are shell-style words; blank
// lines and lines starting with # are ignored. Two line kinds:
//
//	picker <name> [header] [multi] [col=<n|TITLE>] [forward=<flag,...>] <command...>
//	<name> <picker|none> <command...> [<default args>]
func Parse(content string) (*Config, error) {
	cfg := &Config{}
	seenAlias, seenPicker := map[string]bool{}, map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		words, err := splitWords(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", lineNo, err)
		}
		if words[0] == pickerKeyword {
			p, err := parsePicker(words[1:])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNo, err)
			}
			if seenPicker[p.Name] {
				return nil, fmt.Errorf("line %d: duplicate picker %q", lineNo, p.Name)
			}
			seenPicker[p.Name] = true
			cfg.Pickers = append(cfg.Pickers, p)
			continue
		}
		a, err := parseAlias(words)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if seenAlias[a.Name] {
			return nil, fmt.Errorf("line %d: duplicate alias %q", lineNo, a.Name)
		}
		seenAlias[a.Name] = true
		cfg.Aliases = append(cfg.Aliases, a)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	// Pickers may be defined anywhere in the file, so resolve references last.
	for _, a := range cfg.Aliases {
		if a.NeedsTarget() && !seenPicker[a.Picker] {
			return nil, fmt.Errorf("alias %q uses undefined picker %q (define it with a 'picker %s ...' line)", a.Name, a.Picker, a.Picker)
		}
	}
	return cfg, nil
}

func parsePicker(words []string) (Picker, error) {
	if len(words) < 2 {
		return Picker{}, fmt.Errorf("want 'picker <name> [options] <command...>'")
	}
	p := Picker{Name: words[0]}
	if p.Name == PickerNone {
		return Picker{}, fmt.Errorf("picker name %q is reserved", PickerNone)
	}
	if err := validName(p.Name); err != nil {
		return Picker{}, fmt.Errorf("picker: %w", err)
	}
	i := 1
opts:
	for ; i < len(words); i++ {
		w := words[i]
		key, value, hasValue := strings.Cut(w, "=")
		switch {
		case w == "header":
			p.Header = true
		case w == "multi":
			p.Multi = true
		case key == "col" && hasValue:
			if value == "" {
				return Picker{}, fmt.Errorf("col= needs a column number or header title")
			}
			p.Column = value
		case key == "forward" && hasValue:
			for _, f := range strings.Split(value, ",") {
				if !strings.HasPrefix(f, "-") {
					return Picker{}, fmt.Errorf("forward=%s: %q is not a flag", value, f)
				}
				p.Forward = append(p.Forward, f)
			}
		default:
			break opts
		}
	}
	p.Command = words[i:]
	if len(p.Command) == 0 {
		return Picker{}, fmt.Errorf("picker %q has no list command", p.Name)
	}
	if n, err := strconv.Atoi(p.Column); p.Column != "" && err == nil && n < 1 {
		return Picker{}, fmt.Errorf("col=%s: columns are numbered from 1", p.Column)
	}
	if _, err := strconv.Atoi(p.Column); p.Column != "" && err != nil && !p.Header {
		return Picker{}, fmt.Errorf("col=%s: naming a column by title needs the 'header' option", p.Column)
	}
	return p, nil
}

func parseAlias(words []string) (Alias, error) {
	if len(words) < 3 {
		return Alias{}, fmt.Errorf("want '<name> <picker> <command...>', got %q", strings.Join(words, " "))
	}
	a := Alias{Name: words[0], Picker: words[1]}
	if err := validName(a.Name); err != nil {
		return Alias{}, fmt.Errorf("alias: %w", err)
	}
	for _, r := range Reserved {
		if a.Name == r {
			return Alias{}, fmt.Errorf("alias name %q is reserved", a.Name)
		}
	}
	var err error
	if a.Command, a.DefaultArgs, err = splitDefaultArgs(words[2:]); err != nil {
		return Alias{}, err
	}
	if len(a.Command) == 0 {
		return Alias{}, fmt.Errorf("command is empty (nothing before the %s...%s group)", DefaultArgsOpen, DefaultArgsClose)
	}
	placeholders := 0
	for _, c := range a.Command {
		if c == TargetPlaceholder {
			placeholders++
		}
	}
	switch {
	case placeholders > 1:
		return Alias{}, fmt.Errorf("command may contain %s at most once", TargetPlaceholder)
	case placeholders == 1 && !a.NeedsTarget():
		return Alias{}, fmt.Errorf("%s placeholder needs a picker other than %s", TargetPlaceholder, PickerNone)
	case a.Command[0] == TargetPlaceholder:
		return Alias{}, fmt.Errorf("command cannot start with %s", TargetPlaceholder)
	}
	return a, nil
}

func validName(name string) error {
	if name == "." || name == ".." || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, "-") || strings.Contains(name, "=") {
		return fmt.Errorf("invalid name %q: must be a plain word not starting with '-'", name)
	}
	return nil
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
