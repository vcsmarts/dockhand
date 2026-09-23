// Package picker offers the rows of a command's tabular output in an
// interactive fuzzy finder and returns the chosen value(s). It is
// tool-agnostic: what to run and which column is the target come from the
// picker definition in the config.
package picker

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ktr0731/go-fuzzyfinder"
	"golang.org/x/term"

	"github.com/Innovative-Digitale-Medizin-IDM/dockhand/internal/config"
)

// ErrAborted is returned when the user cancels the picker (ESC / ctrl-c).
var ErrAborted = errors.New("no selection")

// ErrNoTerminal is returned when the picker cannot run because stdin or
// stdout is not an interactive terminal.
var ErrNoTerminal = errors.New("the fuzzy picker needs an interactive terminal (stdin and stdout must be a TTY)")

// Pick runs the picker's list command (plus any forwarded flags from
// userArgs), shows its rows aligned as a table and returns the target column
// of the selected row(s).
func Pick(p config.Picker, userArgs []string) ([]string, error) {
	if err := requireTerminal(); err != nil {
		return nil, err
	}
	args := append(append([]string{}, p.Command[1:]...), Forward(p.Forward, userArgs)...)
	out, err := commandOutput(p.Command[0], args...)
	if err != nil {
		return nil, err
	}
	titles, rows := ParseTable(out, p.Header)
	if len(rows) == 0 {
		return nil, fmt.Errorf("picker %s: '%s' listed nothing", p.Name, strings.Join(append([]string{p.Command[0]}, args...), " "))
	}
	col, err := resolveColumn(p.Column, titles)
	if err != nil {
		return nil, fmt.Errorf("picker %s: %w", p.Name, err)
	}

	// The finder shows the header on one line, so with titles the column
	// header itself is the header line, aligned above the rows.
	tb := newTable(titles, rows)
	header := fmt.Sprintf("pick %s", p.Name)
	if titles != nil {
		header = tb.Header()
	}
	if p.Multi {
		header += "   (TAB selects several, ENTER confirms)"
	}
	render := func(i int) string { return tb.Line(rows[i]) }
	value := func(i int) (string, error) {
		if col >= len(rows[i]) || rows[i][col] == "" {
			return "", fmt.Errorf("picker %s: selected row has no value in column %d: %q", p.Name, col+1, tb.Line(rows[i]))
		}
		return rows[i][col], nil
	}

	if !p.Multi {
		idx, err := fuzzyfinder.Find(rows, render, fuzzyfinder.WithHeader(header))
		if err != nil {
			return nil, pickErr(err)
		}
		v, err := value(idx)
		if err != nil {
			return nil, err
		}
		return []string{v}, nil
	}
	idxs, err := fuzzyfinder.FindMulti(rows, render, fuzzyfinder.WithHeader(header))
	if err != nil {
		return nil, pickErr(err)
	}
	picked := make([]string, 0, len(idxs))
	for _, idx := range idxs {
		v, err := value(idx)
		if err != nil {
			return nil, err
		}
		picked = append(picked, v)
	}
	return picked, nil
}

// Forward returns the subset of userArgs that the picker wants copied to its
// list command: each flag in flags, in both "--flag value" and "--flag=value"
// forms. A flag at the very end with no value is dropped.
func Forward(flags, userArgs []string) []string {
	if len(flags) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, f := range flags {
		want[f] = true
	}
	var out []string
	for i := 0; i < len(userArgs); i++ {
		arg := userArgs[i]
		name, _, hasValue := strings.Cut(arg, "=")
		if !want[name] {
			continue
		}
		if hasValue {
			out = append(out, arg)
		} else if i+1 < len(userArgs) {
			out = append(out, name, userArgs[i+1])
			i++
		}
	}
	return out
}

// resolveColumn turns the picker's col= setting into a 0-based index.
func resolveColumn(col string, titles []string) (int, error) {
	if col == "" {
		return 0, nil
	}
	if n, err := strconv.Atoi(col); err == nil {
		return n - 1, nil
	}
	for i, t := range titles {
		if strings.EqualFold(t, col) {
			return i, nil
		}
	}
	return 0, fmt.Errorf("no column titled %q (header has: %s)", col, strings.Join(titles, ", "))
}

func requireTerminal() error {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return ErrNoTerminal
	}
	return nil
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

func pickErr(err error) error {
	if errors.Is(err, fuzzyfinder.ErrAbort) {
		return ErrAborted
	}
	return err
}
