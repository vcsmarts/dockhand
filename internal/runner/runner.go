// Package runner resolves an alias's target via its picker and executes the
// final command, replacing the dockhand process (like `exec` in shell).
package runner

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/Innovative-Digitale-Medizin-IDM/dockhand/internal/config"
	"github.com/Innovative-Digitale-Medizin-IDM/dockhand/internal/picker"
)

// Run resolves the alias's target(s), builds the final argv (see BuildArgv)
// and replaces the current process with it via execve. On success it does
// not return.
func Run(alias config.Alias, userArgs []string) error {
	targets, err := pick(alias.Picker)
	if err != nil {
		return err
	}
	argv := BuildArgv(alias.Command, effectiveArgs(alias, userArgs), targets)

	path, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("%s: command not found", argv[0])
	}
	return syscall.Exec(path, argv, os.Environ())
}

// BuildArgv assembles the command line for an alias.
//
// Without a placeholder, the user's extra args come first and the picked
// target(s) last, so flags work as expected:
//
//	dl -f --tail 0  ->  docker logs -f --tail 0 <picked-container>
//
// With a {} placeholder in the command, the target(s) replace it and the
// user's extra args go at the very end, which is what commands like
// `docker exec CONTAINER COMMAND` need:
//
//	dexec sh  ->  docker exec -it <picked-container> sh
func BuildArgv(command, userArgs, targets []string) []string {
	argv := make([]string, 0, len(command)+len(userArgs)+len(targets))
	placed := false
	for _, c := range command {
		if c == config.TargetPlaceholder && !placed {
			argv = append(argv, targets...)
			placed = true
			continue
		}
		argv = append(argv, c)
	}
	argv = append(argv, userArgs...)
	if !placed {
		argv = append(argv, targets...)
	}
	return argv
}

// effectiveArgs returns the user's args, or the alias's DefaultArgs when the
// user passed none, so `dexec` alone can fall back to a shell.
func effectiveArgs(alias config.Alias, userArgs []string) []string {
	if len(userArgs) == 0 {
		return alias.DefaultArgs
	}
	return userArgs
}

func pick(p config.Picker) ([]string, error) {
	switch p {
	case config.PickerDocker, config.PickerDockerAll:
		name, err := picker.Container(p == config.PickerDockerAll)
		if err != nil {
			return nil, err
		}
		return []string{name}, nil
	case config.PickerCompose:
		return picker.ComposeServices()
	default:
		return nil, nil
	}
}
