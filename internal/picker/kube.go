package picker

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ktr0731/go-fuzzyfinder"
)

// KubeScope extracts the kubectl flags that change which cluster/namespace a
// picker should list from: -n/--namespace, --context and --kubeconfig, in
// both "--flag value" and "--flag=value" forms. Everything else is ignored.
// The result is passed through to the kubectl list command so that
// `dockhand kl -n kube-system` picks from kube-system pods.
func KubeScope(userArgs []string) []string {
	var scope []string
	for i := 0; i < len(userArgs); i++ {
		arg := userArgs[i]
		name, _, hasValue := strings.Cut(arg, "=")
		switch name {
		case "-n", "--namespace", "--context", "--kubeconfig":
		default:
			continue
		}
		if hasValue {
			scope = append(scope, arg)
			continue
		}
		if i+1 < len(userArgs) {
			scope = append(scope, name, userArgs[i+1])
			i++
		}
	}
	return scope
}

// KubeContext fuzzy-picks one kubectl context and returns its name.
func KubeContext() (string, error) {
	if err := requireTerminal(); err != nil {
		return "", err
	}
	out, err := commandOutput("kubectl", "config", "get-contexts", "-o", "name")
	if err != nil {
		return "", err
	}
	contexts := splitLines(out)
	if len(contexts) == 0 {
		return "", errors.New("no kubectl contexts configured")
	}
	header := "pick a context"
	if cur, err := commandOutput("kubectl", "config", "current-context"); err == nil {
		header += fmt.Sprintf(" (current: %s)", strings.TrimSpace(cur))
	}
	idx, err := fuzzyfinder.Find(contexts, func(i int) string { return contexts[i] }, fuzzyfinder.WithHeader(header))
	if err != nil {
		return "", pickErr(err)
	}
	return contexts[idx], nil
}

// KubeNamespace fuzzy-picks one namespace and returns its name. scope is the
// output of KubeScope, so --context/--kubeconfig in the user's args apply.
func KubeNamespace(scope []string) (string, error) {
	if err := requireTerminal(); err != nil {
		return "", err
	}
	args := append([]string{"get", "namespaces", "-o", "name"}, scope...)
	out, err := commandOutput("kubectl", args...)
	if err != nil {
		return "", err
	}
	var namespaces []string
	for _, line := range splitLines(out) {
		namespaces = append(namespaces, strings.TrimPrefix(line, "namespace/"))
	}
	if len(namespaces) == 0 {
		return "", errors.New("no namespaces found")
	}
	idx, err := fuzzyfinder.Find(namespaces, func(i int) string { return namespaces[i] }, fuzzyfinder.WithHeader("pick a namespace"))
	if err != nil {
		return "", pickErr(err)
	}
	return namespaces[idx], nil
}

// PodInfo holds the columns of one `kubectl get pods --no-headers` row.
type PodInfo struct {
	Name, Ready, Status, Restarts, Age string
}

// KubePod fuzzy-picks one pod and returns its name. scope is the output of
// KubeScope, so -n/--context/--kubeconfig in the user's args apply.
func KubePod(scope []string) (string, error) {
	if err := requireTerminal(); err != nil {
		return "", err
	}
	args := append([]string{"get", "pods", "--no-headers"}, scope...)
	out, err := commandOutput("kubectl", args...)
	if err != nil {
		return "", err
	}
	pods := ParsePods(out)
	if len(pods) == 0 {
		return "", errors.New("no pods found in this namespace")
	}
	idx, err := fuzzyfinder.Find(
		pods,
		func(i int) string {
			p := pods[i]
			return fmt.Sprintf("%s  (%s, %s, %s)", p.Name, p.Status, p.Ready, p.Age)
		},
		fuzzyfinder.WithHeader("pick a pod"),
	)
	if err != nil {
		return "", pickErr(err)
	}
	return pods[idx].Name, nil
}

// ParsePods turns `kubectl get pods --no-headers` output (columns NAME READY
// STATUS RESTARTS AGE, whitespace-separated) into PodInfos. Lines that do not
// look like a pod row are skipped. RESTARTS may contain spaces, e.g.
// "3 (2m ago)", so AGE is taken from the end and RESTARTS is what is left.
func ParsePods(out string) []PodInfo {
	var pods []PodInfo
	for _, line := range splitLines(out) {
		f := strings.Fields(line)
		if len(f) < 5 || !strings.Contains(f[1], "/") { // READY is always "n/m"
			continue
		}
		pods = append(pods, PodInfo{
			Name:     f[0],
			Ready:    f[1],
			Status:   f[2],
			Restarts: strings.Join(f[3:len(f)-1], " "),
			Age:      f[len(f)-1],
		})
	}
	return pods
}
