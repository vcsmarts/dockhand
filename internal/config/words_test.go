package config

import (
	"reflect"
	"testing"
)

func TestSplitWords(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{`docker ps -a`, []string{"docker", "ps", "-a"}},
		{`  spaced   out  `, []string{"spaced", "out"}},
		{`tabs	between	words`, []string{"tabs", "between", "words"}},
		{`docker ps --format "table {{.Names}}\t{{.Image}}"`, []string{"docker", "ps", "--format", `table {{.Names}}\t{{.Image}}`}},
		{`echo 'single "quoted"'`, []string{"echo", `single "quoted"`}},
		{`echo "escaped \" quote and \\ backslash"`, []string{"echo", `escaped " quote and \ backslash`}},
		{`a\ b c`, []string{"a b", "c"}},
		{`mixed"quote"ing'works'`, []string{"mixedquoteingworks"}},
		{`""`, []string{""}},
		{``, nil},
	} {
		got, err := splitWords(tc.in)
		if err != nil {
			t.Errorf("%q: unexpected error %v", tc.in, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q:\n got  %q\n want %q", tc.in, got, tc.want)
		}
	}
}

func TestSplitWordsErrors(t *testing.T) {
	for _, in := range []string{`unterminated "quote`, `unterminated 'quote`, `trailing \`} {
		if _, err := splitWords(in); err == nil {
			t.Errorf("%q: want error", in)
		}
	}
}
