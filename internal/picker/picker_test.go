package picker

import (
	"reflect"
	"testing"
)

func TestParseContainers(t *testing.T) {
	out := "web-1\tnginx:1.27\tUp 3 hours\n" +
		"db\tpostgres:16\tExited (0) 2 minutes ago\n" +
		"malformed line without tabs\n" +
		"\n" +
		"  spaced  \tbusybox\tUp 1 second  \n"
	got := ParseContainers(out)
	want := []ContainerInfo{
		{Name: "web-1", Image: "nginx:1.27", Status: "Up 3 hours"},
		{Name: "db", Image: "postgres:16", Status: "Exited (0) 2 minutes ago"},
		{Name: "spaced", Image: "busybox", Status: "Up 1 second"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

func TestParseContainersEmpty(t *testing.T) {
	if got := ParseContainers(""); len(got) != 0 {
		t.Fatalf("got %+v, want none", got)
	}
}
