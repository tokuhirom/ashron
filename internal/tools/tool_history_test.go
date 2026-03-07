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

func TestCompactSearchResult_ShortOutput(t *testing.T) {
	t.Parallel()
	in := "file1.go:10:match\nfile2.go:20:match"
	got := CompactToolResultForHistory("search_files", in)
	if got != in {
		t.Fatalf("short search output should be unchanged, got: %q", got)
	}
}

func TestCompactSearchResult_TruncatesLong(t *testing.T) {
	t.Parallel()
	// Generate many search result lines
	var lines []string
	for i := 0; i < 500; i++ {
		lines = append(lines, "file.go:"+strings.Repeat("x", 20)+":match line content here")
	}
	in := strings.Join(lines, "\n")
	got := CompactToolResultForHistory("search_files", in)
	if len(got) >= len(in) {
		t.Fatalf("expected shorter output, got %d bytes (original %d)", len(got), len(in))
	}
	if !strings.Contains(got, "[truncated: showing") {
		t.Fatalf("expected truncation summary, got: %q", got[:200])
	}
	if !strings.Contains(got, "lines") {
		t.Fatalf("expected 'lines' in summary, got: %q", got[len(got)-100:])
	}
}

func TestCompactSearchResult_VerySmallLimit(t *testing.T) {
	t.Parallel()
	// Edge case: output smaller than the 100-byte reserved space
	in := "short"
	got := compactSearchResult(in, 3000)
	// Should be unchanged since it's under limit
	if got != in {
		t.Fatalf("expected unchanged output, got: %q", got)
	}
}

func TestCompactCommandResult_ShortOutput(t *testing.T) {
	t.Parallel()
	in := "command output"
	got := CompactToolResultForHistory("execute_command", in)
	if got != in {
		t.Fatalf("short command output should be unchanged, got: %q", got)
	}
}

func TestCompactCommandResult_TruncatesWithEqualSplit(t *testing.T) {
	t.Parallel()
	// Head part + tail part (where errors typically appear)
	head := strings.Repeat("a", 7000)
	tail := "ERROR: something failed\n"
	in := head + tail
	got := CompactToolResultForHistory("execute_command", in)
	if len(got) >= len(in) {
		t.Fatalf("expected shorter output, got %d bytes (original %d)", len(got), len(in))
	}
	// The tail should be preserved (50/50 split keeps more tail than 75/25)
	if !strings.Contains(got, "ERROR: something failed") {
		t.Fatalf("expected tail with error to be preserved, got: %q", got[len(got)-200:])
	}
	if !strings.Contains(got, "[truncated: kept first") {
		t.Fatalf("expected truncation marker")
	}
}

func TestCompactToolResultForHistory_GrepFilesUsesSearchStrategy(t *testing.T) {
	t.Parallel()
	var lines []string
	for i := 0; i < 500; i++ {
		lines = append(lines, "result"+strings.Repeat("y", 20))
	}
	in := strings.Join(lines, "\n")
	got := CompactToolResultForHistory("grep_files", in)
	if !strings.Contains(got, "[truncated: showing") {
		t.Fatalf("grep_files should use search compaction strategy, got: %q", got[:200])
	}
}
