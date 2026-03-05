package tui

import (
	"strings"
	"testing"
)

func TestCompactUserInputForDisplayShortUnchanged(t *testing.T) {
	t.Parallel()
	in := "hello\nworld"
	got := compactUserInputForDisplay(in)
	if got != in {
		t.Fatalf("got %q want %q", got, in)
	}
}

func TestCompactUserInputForDisplayManyLines(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	for i := 0; i < 60; i++ {
		sb.WriteString("line\n")
	}
	in := sb.String()
	got := compactUserInputForDisplay(in)
	if !strings.Contains(got, "[omitted") {
		t.Fatalf("expected omitted marker, got: %q", got)
	}
	if strings.Count(got, "line") >= 60 {
		t.Fatalf("expected compacted output")
	}
}

func TestCompactUserInputForDisplayLongSingleLine(t *testing.T) {
	t.Parallel()
	in := strings.Repeat("a", maxDisplayInputChars+500)
	got := compactUserInputForDisplay(in)
	if !strings.Contains(got, "[omitted") {
		t.Fatalf("expected omitted marker")
	}
	if len(got) >= len(in) {
		t.Fatalf("expected compacted output to be shorter")
	}
}
