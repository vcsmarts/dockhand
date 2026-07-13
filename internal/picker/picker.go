// Package picker provides interactive fuzzy selection of docker containers
// and compose services, replacing the fzf dependency of the original bash
// implementation.
package picker

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ktr0731/go-fuzzyfinder"
)

// ErrAborted is returned when the user cancels the picker (ESC / ctrl-c).
var ErrAborted = errors.New("no selection")

type container struct {
	Name   string
	Image  string
	Status string
}

// Container fuzzy-picks exactly one running container and returns its name.
func Container() (string, error) {
	out, err := dockerOutput("ps", "--format", "{{.Names}}\t{{.Image}}\t{{.Status}}")
	if err != nil {
		return "", err
	}
	var containers []container
	for _, line := range splitLines(out) {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		containers = append(containers, container{Name: parts[0], Image: parts[1], Status: parts[2]})
	}
	if len(containers) == 0 {
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
// multi-select) and returns their names.
func ComposeServices() ([]string, error) {
	out, err := dockerOutput("compose", "ps", "--services")
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

func dockerOutput(args ...string) (string, error) {
	out, err := exec.Command("docker", args...).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("docker %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("docker %s: %w", strings.Join(args, " "), err)
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
