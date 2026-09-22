package runner

import (
	"strings"
	"testing"

	"github.com/Innovative-Digitale-Medizin-IDM/dockhand/internal/config"
)

func TestBuildArgv(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		command, userArgs, targets []string
		want                       string
	}{
		{
			name:    "no picker, no args",
			command: []string{"docker", "ps"},
			want:    "docker ps",
		},
		{
			name:     "no picker, user args appended",
			command:  []string{"docker", "compose", "up"},
			userArgs: []string{"-d", "--build"},
			want:     "docker compose up -d --build",
		},
		{
			name:     "target after user args (flags before container)",
			command:  []string{"docker", "logs"},
			userArgs: []string{"-f", "--tail", "0"},
			targets:  []string{"web"},
			want:     "docker logs -f --tail 0 web",
		},
		{
			name:     "multiple compose targets",
			command:  []string{"docker", "compose", "logs"},
			userArgs: []string{"-f"},
			targets:  []string{"web", "db"},
			want:     "docker compose logs -f web db",
		},
		{
			name:     "placeholder puts target before user args (docker exec)",
			command:  []string{"docker", "exec", "-it", "{}"},
			userArgs: []string{"sh", "-c", "id"},
			targets:  []string{"web"},
			want:     "docker exec -it web sh -c id",
		},
		{
			name:    "placeholder with no user args",
			command: []string{"docker", "exec", "-it", "{}"},
			targets: []string{"web"},
			want:    "docker exec -it web",
		},
		{
			name:     "placeholder mid-command expands multiple targets",
			command:  []string{"docker", "compose", "logs", "{}"},
			userArgs: []string{"-f"},
			targets:  []string{"web", "db"},
			want:     "docker compose logs web db -f",
		},
	} {
		got := strings.Join(BuildArgv(tc.command, tc.userArgs, tc.targets), " ")
		if got != tc.want {
			t.Errorf("%s:\n got  %q\n want %q", tc.name, got, tc.want)
		}
	}
}

func TestBuildArgvDoesNotAliasInputs(t *testing.T) {
	command := []string{"docker", "logs"}
	argv := BuildArgv(command, []string{"-f"}, []string{"web"})
	argv[0] = "changed"
	if command[0] != "docker" {
		t.Fatal("BuildArgv returned a slice sharing memory with the alias command")
	}
}

func TestEffectiveArgs(t *testing.T) {
	dexec := config.Alias{Command: []string{"docker", "exec", "-it", "{}"}, DefaultArgs: []string{"sh"}}
	if got := strings.Join(effectiveArgs(dexec, nil), " "); got != "sh" {
		t.Errorf("no user args: got %q, want default sh", got)
	}
	if got := strings.Join(effectiveArgs(dexec, []string{"bash", "-l"}), " "); got != "bash -l" {
		t.Errorf("user args must win: got %q", got)
	}
	plain := config.Alias{Command: []string{"docker", "ps"}}
	if got := effectiveArgs(plain, nil); len(got) != 0 {
		t.Errorf("alias without defaults: got %q, want none", got)
	}
}
