# dockhand

Short commands for the docker, docker-compose and kubectl commands you type
all day, with **built-in fuzzy container/service/pod selection** — a single Go
binary, no `fzf` or shell-script generation required.

`dockhand` is the Go successor of the bash `aliases` project: same declarative
config, same alias names, but every alias is a subcommand of one binary, so
there is exactly one file to keep track of.

## Install

```bash
go build -o dockhand . && ./dockhand setup
```

`setup` copies the binary to `~/.local/bin/dockhand` (or `--bin DIR`) and, if
that directory is not on your `PATH`, appends an `export PATH` line to your
`~/.bashrc` or `~/.zshrc`. Restart your shell afterwards. `make install` does
the same.

The only runtime dependencies are the `docker` and `kubectl` CLIs, each needed
only for the aliases that use it. dockhand runs on Linux and macOS (it replaces
itself with the real command via `execve`).

## Usage

```bash
dockhand dl -f --tail 0   # fuzzy-pick a container, then: docker logs -f --tail 0 <picked>
dockhand dcl -f           # fuzzy-pick service(s),   then: docker compose logs -f <picked>
dockhand dexec            # fuzzy-pick a container,  then: docker exec -it <picked> sh
dockhand dexec bash       # fuzzy-pick a container,  then: docker exec -it <picked> bash
dockhand dps              # docker ps
dockhand kctx             # fuzzy-pick a context,    then: kubectl config use-context <picked>
dockhand kl -n web -f     # fuzzy-pick a pod in web, then: kubectl logs -n web -f <picked>
dockhand kexec            # fuzzy-pick a pod,        then: kubectl exec -it <picked> -- sh
dockhand kpf 8080:80      # fuzzy-pick a pod,        then: kubectl port-forward <picked> 8080:80
dockhand list             # show every alias, its picker and its command
```

By default your extra args are inserted **before** the picked target, so flags
work as expected. Commands that need the target *before* your args, like
`docker exec CONTAINER COMMAND`, put a `{}` placeholder in the alias command;
the target replaces it and your args go last. A trailing `[group]` in the alias
command is used only when you pass no args at all, which is how bare
`dockhand dexec` falls back to `sh`. For `compose` pickers, use `TAB` to select
multiple services.

For `kube-pod` and `kube-namespace` pickers, `-n`/`--namespace`, `--context`
and `--kubeconfig` in your args are also applied to the picker, so the list you
choose from matches the command that runs. `-A` is not forwarded, since a pod
name alone is ambiguous across namespaces.

The picker needs an interactive terminal; aliases with a picker cannot be used
in pipes or scripts.

## Default aliases

| alias       | picker     | command                    |
|-------------|------------|----------------------------|
| `dl`        | docker-all | `docker logs`              |
| `dcl`       | compose    | `docker compose logs`      |
| `dexec`     | docker     | `docker exec -it {} [sh]`  |
| `dstop`     | docker     | `docker stop`              |
| `dps`       | none       | `docker ps`                |
| `dcps`      | none       | `docker compose ps`        |
| `dcdown`    | none       | `docker compose down`      |
| `dcup`      | none       | `docker compose up`        |
| `dcupd`     | none       | `docker compose up -d`     |
| `dcrestart` | compose    | `docker compose restart`   |
| `kctx`      | kube-context   | `kubectl config use-context {}` |
| `kns`       | kube-namespace | `kubectl config set-context --current --namespace {}` |
| `kgp`       | none       | `kubectl get pods`         |
| `kl`        | kube-pod   | `kubectl logs`             |
| `kd`        | kube-pod   | `kubectl describe pod`     |
| `kexec`     | kube-pod   | `kubectl exec -it {} -- [sh]` |
| `kpf`       | kube-pod   | `kubectl port-forward {}`  |
| `kdel`      | kube-pod   | `kubectl delete pod`       |

## Customizing aliases

```bash
dockhand init-config        # writes defaults to ~/.config/dockhand/aliases.conf
$EDITOR ~/.config/dockhand/aliases.conf
```

Changes take effect immediately; there is nothing to reinstall. One line per
alias:

```
#  name        picker    command
   dtop        docker    docker top
```

`picker` is one of:

| picker       | behaviour                                                 |
|--------------|-----------------------------------------------------------|
| `none`       | no target; runs the command as-is                         |
| `docker`     | fuzzy-pick one running container (`docker ps`)            |
| `docker-all` | fuzzy-pick one container, running or not (`docker ps -a`) |
| `compose`    | fuzzy-pick one+ services of the project (multi-select)    |
| `kube-context`   | fuzzy-pick one kubectl context                        |
| `kube-namespace` | fuzzy-pick one namespace                              |
| `kube-pod`       | fuzzy-pick one pod, scoped by `-n`/`--context` in your args |

Alias names must be plain file names (no `/`) and cannot be one of dockhand's
own subcommands (`setup`, `list`, `init-config`, `help`).

## How it works

- `dockhand <alias>` looks the alias up in the config, resolves the target with
  an interactive fuzzy finder
  ([go-fuzzyfinder](https://github.com/ktr0731/go-fuzzyfinder)) and then
  **replaces itself** with the real command via `execve`, so signals, TTY and
  exit codes behave exactly as if you had typed the docker command yourself.
- Aliases come from `~/.config/dockhand/aliases.conf` if present, otherwise
  from defaults embedded in the binary.

## Development

```bash
make build    # build ./dockhand
make test     # go vet + go test ./...
make install  # build and run ./dockhand setup
```
