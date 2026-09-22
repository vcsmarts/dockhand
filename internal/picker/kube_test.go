package picker

import (
	"reflect"
	"strings"
	"testing"
)

func TestKubeScope(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"-f --tail 10", ""},
		{"-n kube-system -f", "-n kube-system"},
		{"-f -n kube-system", "-n kube-system"},
		{"--namespace=monitoring", "--namespace=monitoring"},
		{"--namespace monitoring --tail 5", "--namespace monitoring"},
		{"--context prod -n web --kubeconfig /tmp/kc", "--context prod -n web --kubeconfig /tmp/kc"},
		{"--context=prod", "--context=prod"},
		{"-n", ""}, // dangling flag: nothing to forward
		{"-A", ""}, // all-namespaces is not forwarded; pod names would be ambiguous
	} {
		in := strings.Fields(tc.in)
		got := strings.Join(KubeScope(in), " ")
		if got != tc.want {
			t.Errorf("KubeScope(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParsePods(t *testing.T) {
	out := "web-7d9f8b6c5-abcde   1/1     Running     0          3d2h\n" +
		"job-xyz              0/1     Completed   0          45m\n" +
		"api-5c6d7e8f9-fghij  2/2     Running     3 (2m ago) 12h\n" +
		"No resources found in default namespace.\n"
	got := ParsePods(out)
	want := []PodInfo{
		{Name: "web-7d9f8b6c5-abcde", Ready: "1/1", Status: "Running", Restarts: "0", Age: "3d2h"},
		{Name: "job-xyz", Ready: "0/1", Status: "Completed", Restarts: "0", Age: "45m"},
		{Name: "api-5c6d7e8f9-fghij", Ready: "2/2", Status: "Running", Restarts: "3 (2m ago)", Age: "12h"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}
