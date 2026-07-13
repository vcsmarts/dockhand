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

	// Invoked via an alias symlink (dl, dexec, ...)? Run that alias.
	invoked := filepath.Base(os.Args[0])
	if invoked != "dockhand" {
		alias, ok := config.Find(aliases, invoked)
		if !ok {
			return fmt.Errorf("invoked as %q but no such alias is defined in %s", invoked, source)
		}
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
		fmt.Printf("  %-12s %-8s %s\n", a.Name, a.Picker, strings.Join(a.Command, " "))
	}
}

func install(aliases []config.Alias, args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	binDir := fs.String("bin", "", "directory for alias symlinks (default: ~/.local/bin)")
	if err := fs.Parse(args); err != nil {
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

	for _, a := range aliases {
		link := filepath.Join(dir, a.Name)
		// Only replace things that are already symlinks — never clobber a
		// real file that happens to share an alias name.
		if info, statErr := os.Lstat(link); statErr == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				fmt.Printf("  %-12s SKIPPED: %s exists and is not a symlink\n", a.Name, link)
				continue
			}
			if err := os.Remove(link); err != nil {
				return err
			}
		}
		if err := os.Symlink(self, link); err != nil {
			return err
		}
		fmt.Printf("  %-12s -> %s\n", a.Name, strings.Join(a.Command, " "))
	}

	fmt.Printf("\nInstalled %d aliases in %s\n", len(aliases), dir)
	if !onPath(dir) {
		fmt.Printf("\nNOTE: %s is not on your PATH. Add to ~/.bashrc:\n  export PATH=\"%s:$PATH\"\n", dir, dir)
	}
	return nil
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
