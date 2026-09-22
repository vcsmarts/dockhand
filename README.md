# dockhand

Short commands for the docker / docker-compose commands you type all day, with
**built-in fuzzy container/service selection** — a single Go binary, no `fzf`
or shell-script generation required.

`dockhand` is the Go successor of the bash `aliases` project: same declarative
config, same alias names, but one static binary that dispatches on the name it
is invoked as (busybox-style symlinks).

## Install

```bash
go build -o ~/.local/bin/dockhand .
dockhand install            # creates dl, dcl, dexec, ... symlinks in ~/.local/bin
```

Make sure `~/.local/bin` is on your `PATH`. The only runtime dependency is the
`docker` CLI itself. dockhand runs on Linux and macOS (it replaces itself with
the docker command via `execve`).

## Usage

```bash
dl -f --tail 0      # fuzzy-pick a container, then: docker logs -f --tail 0 <picked>
dcl -f              # fuzzy-pick service(s),   then: docker compose logs -f <picked>
dexec               # fuzzy-pick a container,  then: docker exec -it <picked> sh
dexec bash          # fuzzy-pick a container,  then: docker exec -it <picked> bash
dps                 # docker ps
dcdown              # docker compose down
```

By default your extra args are inserted **before** the picked target, so flags
work as expected. Commands that need the target *before* your args, like
`docker exec CONTAINER COMMAND`, put a `{}` placeholder in the alias command;
the target replaces it and your args go last. Anything after `--` in the alias
command is used only when you pass no args at all, which is how bare `dexec`
falls back to `sh`. For `compose` pickers, use `TAB` to select multiple
services.

The picker needs an interactive terminal; aliases with a picker cannot be used
in pipes or scripts.

Every alias also works without its symlink: `dockhand dl -f`.

## Default aliases

| alias       | picker     | command                |
|-------------|------------|------------------------|
| `dl`        | docker-all | `docker logs`          |
| `dcl`       | compose | `docker compose logs`     |
| `dexec`     | docker  | `docker exec -it {} -- sh` |
| `dstop`     | docker  | `docker stop`             |
| `dps`       | none    | `docker ps`               |
| `dcps`      | none    | `docker compose ps`       |
| `dcdown`    | none    | `docker compose down`     |
| `dcup`      | none    | `docker compose up`       |
| `dcupd`     | none    | `docker compose up -d`    |
| `dcrestart` | compose | `docker compose restart`  |

## Customizing aliases

```bash
dockhand init-config        # writes defaults to ~/.config/dockhand/aliases.conf
$EDITOR ~/.config/dockhand/aliases.conf
dockhand install            # re-create symlinks for the new set
```

`dockhand install` only touches symlinks that point at dockhand: it never
replaces a real file or someone else's symlink with an alias name, and it
removes symlinks to dockhand whose alias you deleted from the config.

One line per alias:

```
#  name        picker    command
   dtop        docker    docker top
```

`picker` is one of:

| picker       | behaviour                                              |
|--------------|--------------------------------------------------------|
| `none`       | no target; runs the command as-is                      |
| `docker`     | fuzzy-pick one running container (`docker ps`)         |
| `docker-all` | fuzzy-pick one container, running or not (`docker ps -a`) |
| `compose`    | fuzzy-pick one+ services of the project (multi-select) |

Alias names must be plain file names (no `/`) and cannot be one of dockhand's
own subcommands (`install`, `list`, `init-config`, `help`).

## How it works

- `dockhand install` symlinks each alias name to the `dockhand` binary.
- When invoked, dockhand looks at `argv[0]`: if it matches an alias, it
  resolves the target with an interactive fuzzy finder
  ([go-fuzzyfinder](https://github.com/ktr0731/go-fuzzyfinder)) and then
  **replaces itself** with the real command via `execve`, so signals, TTY and
  exit codes behave exactly as if you had typed the docker command yourself.
- Aliases come from `~/.config/dockhand/aliases.conf` if present, otherwise
  from defaults embedded in the binary.

## Development

```bash
make build    # build ./dockhand
make test     # go vet + go test ./...
make install  # build to ~/.local/bin and create symlinks
```
