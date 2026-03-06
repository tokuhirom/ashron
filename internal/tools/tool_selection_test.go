package tools

import "testing"

func TestLikelyNeedsExtendedToolset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  bool
	}{
		{input: "read README and explain", want: false},
		{input: "please edit this file", want: true},
		{input: "テストを実行して", want: true},
		{input: "", want: false},
	}
	for _, tt := range tests {
		if got := LikelyNeedsExtendedToolset(tt.input); got != tt.want {
			t.Fatalf("LikelyNeedsExtendedToolset(%q)=%v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSelectBuiltinTools_MinimalForReadOnlyPrompt(t *testing.T) {
	t.Parallel()
	tools := SelectBuiltinTools("read docs and summarize")
	seen := map[string]bool{}
	for _, t := range tools {
		seen[t.Function.Name] = true
	}
	if !seen["read_file"] {
		t.Fatalf("read_file must exist in minimal toolset")
	}
	if seen["write_file"] {
		t.Fatalf("write_file must not exist in minimal toolset")
	}
	if seen["execute_command"] {
		t.Fatalf("execute_command must not exist in minimal toolset")
	}
}

func TestSelectBuiltinTools_FullForEditPrompt(t *testing.T) {
	t.Parallel()
	tools := SelectBuiltinTools("fix bug and run tests")
	seen := map[string]bool{}
	for _, t := range tools {
		seen[t.Function.Name] = true
	}
	if !seen["write_file"] {
		t.Fatalf("write_file must exist in full toolset")
	}
	if !seen["execute_command"] {
		t.Fatalf("execute_command must exist in full toolset")
	}
}
