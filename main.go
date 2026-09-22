// dockhand is a single binary providing short docker/compose commands with
// built-in fuzzy container/service selection.
//
// It dispatches on the name it was invoked as: `dockhand install` creates one
// symlink per alias (dl, dcl, dexec, ...) pointing at the dockhand binary, so
// typing `dl -f` runs `docker logs -f <fuzzy-picked container>`.
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

	// Invoked via an alias symlink (dl, dexec, ...)? Run that alias. Any
	// other name (dockhand, dockhand-linux-amd64, ...) is the main binary.
	if alias, ok := config.Find(aliases, filepath.Base(os.Args[0])); ok {
		return runner.Run(alias, os.Args[1:])
	}

	if len(os.Args) < 2 {
		usage(aliases, source)
		return nil
	}

	switch cmd := os.Args[1]; cmd {
	case "install":
		return install(aliases, os.Args[2:])
	case "list":
		list(aliases, source)
		return nil
	case "init-config":
		return initConfig()
	case "help", "-h", "--help":
		usage(aliases, source)
		return nil
	default:
		// `dockhand <alias> [args...]` also works without symlinks.
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
  dockhand install [--bin DIR]   create one symlink per alias (default: ~/.local/bin)
  dockhand list                  show configured aliases
  dockhand init-config           write the default config to your user config dir
  dockhand <alias> [args...]     run an alias without its symlink

Aliases (from %s):
`, source)
	list(aliases, "")
}

func list(aliases []config.Alias, source string) {
	if source != "" && source != "builtin" {
		fmt.Printf("# from %s\n", source)
	}
	for _, a := range aliases {
		cmd := strings.Join(a.Command, " ")
		if len(a.DefaultArgs) > 0 {
			cmd += " " + config.DefaultArgsSeparator + " " + strings.Join(a.DefaultArgs, " ")
		}
		fmt.Printf("  %-12s %-10s %s\n", a.Name, a.Picker, cmd)
	}
}

func install(aliases []config.Alias, args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	binDir := fs.String("bin", "", "directory for alias symlinks (default: ~/.local/bin)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	dir := *binDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dir = filepath.Join(home, ".local", "bin")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	self, err := os.Executable()
	if err != nil {
		return err
	}
	if self, err = filepath.EvalSymlinks(self); err != nil {
		return err
	}

	installed := 0
	for _, a := range aliases {
		link := filepath.Join(dir, a.Name)
		// Only replace symlinks we own — never clobber a real file or
		// someone else's symlink that happens to share an alias name.
		if info, statErr := os.Lstat(link); statErr == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				fmt.Printf("  %-12s SKIPPED: %s exists and is not a symlink\n", a.Name, link)
				continue
			}
			if !ownsLink(link, self) {
				target, _ := os.Readlink(link)
				fmt.Printf("  %-12s SKIPPED: %s is a symlink to %s, not to dockhand\n", a.Name, link, target)
				continue
			}
			if err := os.Remove(link); err != nil {
				return err
			}
		}
		if err := os.Symlink(self, link); err != nil {
			return err
		}
		installed++
		fmt.Printf("  %-12s -> %s\n", a.Name, strings.Join(a.Command, " "))
	}

	removed, err := removeStaleLinks(dir, self, aliases)
	if err != nil {
		return err
	}

	fmt.Printf("\nInstalled %d of %d aliases in %s", installed, len(aliases), dir)
	if removed > 0 {
		fmt.Printf(", removed %d stale symlink(s)", removed)
	}
	fmt.Println()
	if !onPath(dir) {
		fmt.Printf("\nNOTE: %s is not on your PATH. Add to ~/.bashrc:\n  export PATH=\"%s:$PATH\"\n", dir, dir)
	}
	return nil
}

// ownsLink reports whether the symlink at link belongs to dockhand: it
// resolves to self, points at a binary named dockhand (e.g. a previous
// location), or is dangling.
func ownsLink(link, self string) bool {
	target, err := os.Readlink(link)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(link), target)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return true // dangling
	}
	return resolved == self || filepath.Base(target) == "dockhand"
}

// removeStaleLinks deletes symlinks in dir that point at dockhand but whose
// name is no longer a configured alias (e.g. after editing aliases.conf).
func removeStaleLinks(dir, self string, aliases []config.Alias) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, e := range entries {
		name := e.Name()
		if e.Type()&os.ModeSymlink == 0 || name == "dockhand" {
			continue
		}
		if _, isAlias := config.Find(aliases, name); isAlias {
			continue
		}
		link := filepath.Join(dir, name)
		if !ownsLink(link, self) {
			continue
		}
		if err := os.Remove(link); err != nil {
			return removed, err
		}
		removed++
		fmt.Printf("  %-12s removed (no longer configured)\n", name)
	}
	return removed, nil
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
	fmt.Printf("Wrote default config to %s\nEdit it, then re-run 'dockhand install'.\n", path)
	return nil
}

func onPath(dir string) bool {
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if p == dir {
			return true
		}
	}
	return false
}
