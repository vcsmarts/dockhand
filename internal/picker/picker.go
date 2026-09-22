// Package picker provides interactive fuzzy selection of docker containers,
// compose services and kubernetes contexts/namespaces/pods, replacing the fzf
// dependency of the original bash implementation.
package picker

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ktr0731/go-fuzzyfinder"
	"golang.org/x/term"
)

// ErrAborted is returned when the user cancels the picker (ESC / ctrl-c).
var ErrAborted = errors.New("no selection")

// ErrNoTerminal is returned when the picker cannot run because stdin or
// stdout is not an interactive terminal.
var ErrNoTerminal = errors.New("the fuzzy picker needs an interactive terminal (stdin and stdout must be a TTY)")

// ContainerInfo holds the columns of one `docker ps` row.
type ContainerInfo struct {
	Name   string
	Image  string
	Status string
}

// Container fuzzy-picks exactly one container and returns its name. With all
// set, stopped containers are offered too (docker ps -a).
func Container(all bool) (string, error) {
	if err := requireTerminal(); err != nil {
		return "", err
	}
	args := []string{"ps", "--format", "{{.Names}}\t{{.Image}}\t{{.Status}}"}
	if all {
		args = append(args, "--all")
	}
	out, err := dockerOutput(args...)
	if err != nil {
		return "", err
	}
	containers := ParseContainers(out)
	if len(containers) == 0 {
		if all {
			return "", errors.New("no containers")
		}
		return "", errors.New("no running containers")
	}

	idx, err := fuzzyfinder.Find(
		containers,
		func(i int) string {
			return fmt.Sprintf("%s  (%s, %s)", containers[i].Name, containers[i].Image, containers[i].Status)
		},
		fuzzyfinder.WithHeader("pick a container"),
	)
	if err != nil {
		return "", pickErr(err)
	}
	return containers[idx].Name, nil
}

// ComposeServices fuzzy-picks one or more compose services (TAB to
// multi-select) and returns their names. All services of the project are
// offered, including stopped ones, since logs/restart/up apply to those too.
func ComposeServices() ([]string, error) {
	if err := requireTerminal(); err != nil {
		return nil, err
	}
	out, err := dockerOutput("compose", "ps", "--all", "--services")
	if err != nil {
		return nil, err
	}
	services := splitLines(out)
	if len(services) == 0 {
		return nil, errors.New("no compose services found (are you in a compose project directory?)")
	}

	idxs, err := fuzzyfinder.FindMulti(
		services,
		func(i int) string { return services[i] },
		fuzzyfinder.WithHeader("TAB to select multiple, ENTER to confirm"),
	)
	if err != nil {
		return nil, pickErr(err)
	}
	picked := make([]string, len(idxs))
	for i, idx := range idxs {
		picked[i] = services[idx]
	}
	return picked, nil
}

// ParseContainers turns the tab-separated output of
// `docker ps --format '{{.Names}}\t{{.Image}}\t{{.Status}}'` into ContainerInfos.
// Malformed lines are skipped.
func ParseContainers(out string) []ContainerInfo {
	var containers []ContainerInfo
	for _, line := range splitLines(out) {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		containers = append(containers, ContainerInfo{
			Name:   strings.TrimSpace(parts[0]),
			Image:  strings.TrimSpace(parts[1]),
			Status: strings.TrimSpace(parts[2]),
		})
	}
	return containers
}

func requireTerminal() error {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return ErrNoTerminal
	}
	return nil
}

func dockerOutput(args ...string) (string, error) {
	return commandOutput("docker", args...)
}

// commandOutput runs bin with args and returns stdout, surfacing the tool's
// own stderr message on failure.
func commandOutput(bin string, args ...string) (string, error) {
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("%s %s: %s", bin, strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("%s %s: %w", bin, strings.Join(args, " "), err)
	}
	return string(out), nil
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func pickErr(err error) error {
	if errors.Is(err, fuzzyfinder.ErrAbort) {
		return ErrAborted
	}
	return err
}
