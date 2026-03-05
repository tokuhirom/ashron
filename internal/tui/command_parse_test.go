package tui

import (
	"reflect"
	"testing"
)

func TestParseCommandLine(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "simple", input: "/cmd a b", want: []string{"/cmd", "a", "b"}},
		{name: "double quote", input: "/cmd \"a b\" c", want: []string{"/cmd", "a b", "c"}},
		{name: "single quote", input: "/cmd 'a b' c", want: []string{"/cmd", "a b", "c"}},
		{name: "escaped space", input: "/cmd a\\ b c", want: []string{"/cmd", "a b", "c"}},
		{name: "mixed", input: "/cmd \"a \\\"b\\\"\" 'x y'", want: []string{"/cmd", "a \"b\"", "x y"}},
		{name: "empty quoted arg", input: "/cmd \"\" x", want: []string{"/cmd", "", "x"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseCommandLine(tc.input)
			if err != nil {
				t.Fatalf("parseCommandLine() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseCommandLine() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestParseCommandLineErrors(t *testing.T) {
	t.Parallel()

	cases := []string{
		"/cmd \"unterminated",
		"/cmd 'unterminated",
		"/cmd trailing\\",
	}

	for _, input := range cases {
		if _, err := parseCommandLine(input); err == nil {
			t.Fatalf("parseCommandLine(%q) expected error", input)
		}
	}
}
