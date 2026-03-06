package tools

import (
	"strings"
	"testing"
)

func TestCompactToolResultForHistory_NoChangeWhenShort(t *testing.T) {
	t.Parallel()
	in := "short output"
	if got := CompactToolResultForHistory("read_file", in); got != in {
		t.Fatalf("unexpected change for short output: %q", got)
	}
}

func TestCompactToolResultForHistory_TruncatesLong(t *testing.T) {
	t.Parallel()
	in := strings.Repeat("a", defaultToolHistoryLimit+1000)
	got := CompactToolResultForHistory("unknown_tool", in)
	if !strings.Contains(got, "[truncated for history:") {
		t.Fatalf("expected truncation marker, got: %q", got)
	}
	if len(got) >= len(in) {
		t.Fatalf("expected shorter output after truncation")
	}
}

func TestCompactToolResultForHistory_ReadFileUsesLargerLimit(t *testing.T) {
	t.Parallel()
	in := strings.Repeat("a", defaultToolHistoryLimit+100)
	got := CompactToolResultForHistory("read_file", in)
	if got != in {
		t.Fatalf("read_file output should not truncate at default limit")
	}
}
