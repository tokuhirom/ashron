package api

import "testing"

func TestTruncateForLog(t *testing.T) {
	got := truncateForLog("abcdefghij", 4)
	want := "abcd...(truncated 6 bytes)"
	if got != want {
		t.Fatalf("unexpected truncated string: got %q want %q", got, want)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	got := firstNonEmpty("", " ", "x-request-id")
	if got != "x-request-id" {
		t.Fatalf("unexpected value: %q", got)
	}
}
