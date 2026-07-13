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

// Run executes the alias with the user's extra args inserted before the
// picked target(s), matching the original bash behavior:
//
//	dl -f --tail 0  ->  docker logs -f --tail 0 <picked-container>
//
// On success it does not return: the process is replaced via execve.
func Run(alias config.Alias, userArgs []string) error {
	argv := append([]string{}, alias.Command...)
	argv = append(argv, userArgs...)

	switch alias.Picker {
	case config.PickerDocker:
		name, err := picker.Container()
		if err != nil {
			return err
		}
		argv = append(argv, name)
	case config.PickerCompose:
		services, err := picker.ComposeServices()
		if err != nil {
			return err
		}
		argv = append(argv, services...)
	case config.PickerNone:
		// run as-is
	}

	path, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("%s: command not found", argv[0])
	}
	return syscall.Exec(path, argv, os.Environ())
}
