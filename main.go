// dockhand is a single binary providing short docker/compose commands with
// built-in fuzzy container/service selection.
//
// Every alias from aliases.conf is a subcommand: `dockhand dl -f` runs
// `docker logs -f <fuzzy-picked container>`. `dockhand list` shows them all.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Innovative-Digitale-Medizin-IDM/dockhand/internal/config"
	"github.com/Innovative-Digitale-Medizin-IDM/dockhand/internal/picker"
	"github.com/Innovative-Digitale-Medizin-IDM/dockhand/internal/runner"
)

func main() {
	if err := run(); err != nil {
		if errors.Is(err, picker.ErrAborted) {
			fmt.Fprintln(os.Stderr, "dockhand: no selection, aborting.")
		} else {
			fmt.Fprintf(os.Stderr, "dockhand: %v\n", err)
		}
		os.Exit(1)
	}
}

func run() error {
	aliases, source, err := config.Load()
	if err != nil {
		return err
	}

	if len(os.Args) < 2 {
		usage(aliases, source)
		return nil
	}

	switch cmd := os.Args[1]; cmd {
	case "setup":
		return setup(os.Args[2:])
	case "list":
		list(aliases, source)
		return nil
	case "init-config":
		return initConfig()
	case "help", "-h", "--help":
		usage(aliases, source)
		return nil
	default:
		alias, ok := config.Find(aliases, cmd)
		if !ok {
			return fmt.Errorf("unknown command or alias %q (see 'dockhand list')", cmd)
		}
		return runner.Run(alias, os.Args[2:])
	}
}

func usage(aliases []config.Alias, source string) {
	fmt.Printf(`dockhand — short docker/compose commands with fuzzy target selection

Usage:
  dockhand <alias> [args...]     run an alias (see below)
  dockhand list                  show configured aliases
  dockhand init-config           write the default config to your user config dir
  dockhand setup [--bin DIR]     copy this binary to DIR (default: ~/.local/bin)
                                 and add DIR to PATH in your shell rc if needed

Aliases (from %s):
`, source)
	list(aliases, "")
}

func list(aliases []config.Alias, source string) {
	if source != "" && source != "builtin" {
		fmt.Printf("# from %s\n", source)
	}
	for _, a := range aliases {
		fmt.Printf("  %-12s %-10s %s\n", a.Name, a.Picker, describe(a))
	}
}

// describe renders an alias command the way it is written in aliases.conf.
func describe(a config.Alias) string {
	cmd := strings.Join(a.Command, " ")
	if len(a.DefaultArgs) > 0 {
		cmd += " " + config.DefaultArgsSeparator + " " + strings.Join(a.DefaultArgs, " ")
	}
	return cmd
}

// setup installs the running binary as DIR/dockhand and makes sure DIR is on
// PATH, appending an export line to the shell rc file when it is not.
func setup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	binDir := fs.String("bin", "", "directory to install dockhand into (default: ~/.local/bin)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := *binDir
	if dir == "" {
		dir = filepath.Join(home, ".local", "bin")
	}
	if dir, err = filepath.Abs(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	dest := filepath.Join(dir, "dockhand")
	copied, err := installBinary(dest)
	if err != nil {
		return err
	}
	if copied {
		fmt.Printf("Installed %s\n", dest)
	} else {
		fmt.Printf("%s is already this binary\n", dest)
	}

	if onPath(dir) {
		fmt.Printf("%s is on your PATH.\n", dir)
		return nil
	}
	rc := shellRC(home)
	if rc == "" {
		fmt.Printf("\nNOTE: %s is not on your PATH and your shell (%s) is not recognised.\n"+
			"Add this to your shell startup file:\n  export PATH=\"%s:$PATH\"\n", dir, os.Getenv("SHELL"), dir)
		return nil
	}
	added, err := ensurePathLine(rc, dir)
	if err != nil {
		return err
	}
	if added {
		fmt.Printf("Added %s to PATH in %s. Restart your shell or run:\n  source %s\n", dir, rc, rc)
	} else {
		fmt.Printf("%s already adds %s to PATH; restart your shell to pick it up.\n", rc, dir)
	}
	return nil
}

// installBinary copies the running executable to dest atomically. It returns
// false when dest already is the running executable.
func installBinary(dest string) (bool, error) {
	self, err := os.Executable()
	if err != nil {
		return false, err
	}
	if self, err = filepath.EvalSymlinks(self); err != nil {
		return false, err
	}
	if resolved, err := filepath.EvalSymlinks(dest); err == nil && resolved == self {
		return false, nil
	}
	data, err := os.ReadFile(self)
	if err != nil {
		return false, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".dockhand-*")
	if err != nil {
		return false, err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return false, err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return false, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return false, err
	}
	// Rename replaces the directory entry; an already-running old binary at
	// dest keeps its inode and is unaffected.
	if err := os.Rename(tmpName, dest); err != nil {
		os.Remove(tmpName)
		return false, err
	}
	return true, nil
}

// shellRC returns the startup file for the user's login shell, or "" if the
// shell is not one we know how to edit.
func shellRC(home string) string {
	switch filepath.Base(os.Getenv("SHELL")) {
	case "bash":
		return filepath.Join(home, ".bashrc")
	case "zsh":
		return filepath.Join(home, ".zshrc")
	default:
		return ""
	}
}

// ensurePathLine appends `export PATH="<dir>:$PATH"` to rc unless a line
// already adds dir to PATH. It returns whether a line was added.
func ensurePathLine(rc, dir string) (bool, error) {
	existing, err := os.ReadFile(rc)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if pathLineMentions(string(existing), dir) {
		return false, nil
	}
	f, err := os.OpenFile(rc, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}
	defer f.Close()
	prefix := "\n"
	if len(existing) == 0 || existing[len(existing)-1] == '\n' {
		prefix = ""
	}
	line := fmt.Sprintf("%s\n# added by dockhand setup\nexport PATH=\"%s:$PATH\"\n", prefix, dir)
	if _, err := f.WriteString(line); err != nil {
		return false, err
	}
	return true, nil
}

// pathLineMentions reports whether content has an uncommented line that sets
// PATH and mentions dir, either literally or via $HOME/~ for the home part.
func pathLineMentions(content, dir string) bool {
	variants := []string{dir}
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(dir, home+string(filepath.Separator)) {
		rest := dir[len(home):]
		variants = append(variants, "$HOME"+rest, "${HOME}"+rest, "~"+rest)
	}
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "PATH") {
			continue
		}
		for _, v := range variants {
			if strings.Contains(line, v) {
				return true
			}
		}
	}
	return false
}

func initConfig() error {
	path, err := config.UserConfigPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists, not overwriting", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(config.Defaults()), 0o644); err != nil {
		return err
	}
	fmt.Printf("Wrote default config to %s\nEdit it and the new aliases are available immediately.\n", path)
	return nil
}

func onPath(dir string) bool {
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if abs, err := filepath.Abs(p); err == nil && abs == dir {
			return true
		}
	}
	return false
}
