# dockhand

Short commands for the docker, docker-compose, kubectl and whatever-else
commands you type all day, with **built-in fuzzy selection** of the thing to act
on — a single Go binary, no `fzf` or shell-script generation required.

dockhand is tool-agnostic. It knows nothing about docker or kubectl: every
alias is a command, and every *picker* is a command whose tabular output is
offered in a fuzzy finder. The defaults cover docker, compose and kubectl; add
podman, helm, systemctl or your own scripts with a line of config.

## Install

```bash
go build -o dockhand . && ./dockhand setup
```

`setup` copies the binary to `~/.local/bin/dockhand` (or `--bin DIR`) and, if
that directory is not on your `PATH`, appends an `export PATH` line to your
`~/.bashrc` or `~/.zshrc`. Restart your shell afterwards. `make install` does
the same.

dockhand runs on Linux and macOS (it replaces itself with the real command via
`execve`). Each alias needs only the tool it runs.

## Usage

```bash
dockhand dl -f --tail 0   # pick a container, then: docker logs -f --tail 0 <picked>
dockhand dcl -f           # pick service(s),   then: docker compose logs -f <picked>
dockhand dexec            # pick a container,  then: docker exec -it <picked> sh
dockhand dexec bash       # pick a container,  then: docker exec -it <picked> bash
dockhand kctx             # pick a context,    then: kubectl config use-context <picked>
dockhand kl -n web -f     # pick a pod in web, then: kubectl logs -n web -f <picked>
dockhand kexec            # pick a pod,        then: kubectl exec -it <picked> -- sh
dockhand kpf 8080:80      # pick a pod,        then: kubectl port-forward <picked> 8080:80
dockhand list             # show every alias and picker
```

The picker shows the list command's columns aligned as a table, with its
header, and fuzzy-matches on the whole row. `TAB` selects several rows in
multi pickers. The picker needs an interactive terminal; aliases with a picker
cannot be used in pipes or scripts.

## Default aliases

| alias       | picker          | command                                               |
|-------------|-----------------|-------------------------------------------------------|
| `dl`        | container-all   | `docker logs`                                         |
| `dcl`       | service         | `docker compose logs`                                 |
| `dexec`     | container       | `docker exec -it {} [sh]`                             |
| `dstop`     | container       | `docker stop`                                         |
| `dps`       | none            | `docker ps`                                           |
| `dcps`      | none            | `docker compose ps`                                   |
| `dcdown`    | none            | `docker compose down`                                 |
| `dcup`      | none            | `docker compose up`                                   |
| `dcupd`     | none            | `docker compose up -d`                                |
| `dcrestart` | service         | `docker compose restart`                              |
| `kctx`      | context         | `kubectl config use-context {}`                       |
| `kns`       | namespace       | `kubectl config set-context --current --namespace {}` |
| `kgp`       | none            | `kubectl get pods`                                    |
| `kl`        | pod             | `kubectl logs`                                        |
| `kd`        | pod             | `kubectl describe pod`                                |
| `kexec`     | pod             | `kubectl exec -it {} -- [sh]`                         |
| `kpf`       | pod             | `kubectl port-forward {}`                             |
| `kdel`      | pod             | `kubectl delete pod`                                  |

## Default pickers

| picker          | options                                                | list command                                                     |
|-----------------|--------------------------------------------------------|------------------------------------------------------------------|
| `container`     | header col=NAMES                                       | `docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"`    |
| `container-all` | header col=NAMES                                       | `docker ps -a --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"` |
| `service`       | multi                                                  | `docker compose ps -a --services`                                |
| `context`       | header col=NAME                                        | `kubectl config get-contexts`                                    |
| `namespace`     | header col=NAME forward=--context,--kubeconfig         | `kubectl get namespaces`                                         |
| `pod`           | header forward=-n,--namespace,--context,--kubeconfig   | `kubectl get pods`                                               |

## Configuration

```bash
dockhand init-config        # writes the defaults to ~/.config/dockhand/aliases.conf
$EDITOR ~/.config/dockhand/aliases.conf
```

Changes take effect immediately; there is nothing to reinstall. The file
replaces the defaults entirely, so keep the pickers you use. Lines are split
into words like a shell (quotes work); blank lines and `#` comments are
ignored.

### Pickers

```
picker <name> [header] [multi] [col=<n|TITLE>] [forward=<flag,...>] <command...>
```

| option            | meaning                                                                                                   |
|-------------------|-----------------------------------------------------------------------------------------------------------|
| `header`          | the first output line is a header. Columns are cut at the header's positions and shown as titles.       |
| `multi`           | `TAB` selects several rows; all are passed as targets.                                                    |
| `col=<n>`         | the target is column *n* (1-based). Default: the first column.                                            |
| `col=<TITLE>`     | the target is the column with that header title (needs `header`), e.g. `col=NAMES`.                      |
| `forward=<flags>` | flags that, when present in your args, are also passed to the list command, so the list matches the run. |

Without `header`, rows are split on tabs or runs of two or more spaces. With
it, columns are cut where the header's titles start, which is how `docker ps`,
`kubectl get` and most column-aligned tools print, so empty cells and cells
with single spaces (`Exited (0) 2 minutes ago`) survive.

Some more examples:

```
picker  unit       header col=UNIT                       systemctl list-units --type=service --no-pager --plain
picker  release    header forward=-n,--namespace         helm list
picker  branch                                           git branch --format=%(refname:short)
picker  pcontainer header col=NAMES                      podman ps -a --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"

   sr      unit      systemctl restart
   hun     release   helm uninstall
   gco     branch    git checkout
   plog    pcontainer podman logs
```

### Aliases

```
<name> <picker|none> <command...> [<default args>]
```

By default your extra args are inserted **before** the picked target, so flags
work as expected. Commands that need the target *before* your args, like
`docker exec CONTAINER COMMAND`, put a `{}` placeholder in the command; the
target replaces it and your args go last. A trailing `[group]` is used only
when you pass no args at all, which is how bare `dockhand dexec` falls back to
`sh`. Brackets rather than `--` because `--` is a real argument to `kubectl
exec` and friends.

Alias and picker names must be plain words, and an alias cannot be one of
dockhand's own subcommands (`setup`, `list`, `init-config`, `help`) or
`picker`.

## How it works

- `dockhand <alias>` looks the alias up, runs the picker's list command (plus
  any forwarded flags from your args), parses the output into rows, and shows
  them in a fuzzy finder ([go-fuzzyfinder](https://github.com/ktr0731/go-fuzzyfinder)).
- The chosen row's target column is spliced into the alias command, and
  dockhand **replaces itself** with it via `execve`, so signals, TTY and exit
  codes behave exactly as if you had typed the command yourself.
- Config comes from `~/.config/dockhand/aliases.conf` if present, otherwise
  from defaults embedded in the binary.

## Development

```bash
make build    # build ./dockhand
make test     # go vet + go test ./...
make install  # build and run ./dockhand setup
```
