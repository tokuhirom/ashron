package tui

import (
	"strings"
	"testing"
)

func TestBuildHeaderLinesIncludesVersionInfo(t *testing.T) {
	oldVersion, oldCommit, oldDate := buildVersion, buildCommit, buildDate
	t.Cleanup(func() {
		buildVersion, buildCommit, buildDate = oldVersion, oldCommit, oldDate
	})

	SetBuildInfo("1.2.3", "abc123", "2026-03-05")
	lines := buildHeaderLines(false)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "v1.2.3") {
		t.Fatalf("version string not found in header: %q", joined)
	}
	if !strings.Contains(joined, "abc123") {
		t.Fatalf("commit string not found in header: %q", joined)
	}
}
