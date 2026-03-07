package tools

import (
	"fmt"
	"strings"
	"testing"
)

func TestScratchpad_SetAndGet(t *testing.T) {
	t.Parallel()
	sp := NewScratchpad()
	sp.Set("key1", "value1")
	v, ok := sp.Get("key1")
	if !ok || v != "value1" {
		t.Fatalf("expected value1, got %q (ok=%v)", v, ok)
	}
}

func TestScratchpad_GetMissing(t *testing.T) {
	t.Parallel()
	sp := NewScratchpad()
	_, ok := sp.Get("missing")
	if ok {
		t.Fatal("expected missing key to return false")
	}
}

func TestScratchpad_Overwrite(t *testing.T) {
	t.Parallel()
	sp := NewScratchpad()
	sp.Set("key", "old")
	sp.Set("key", "new")
	v, _ := sp.Get("key")
	if v != "new" {
		t.Fatalf("expected new, got %q", v)
	}
}

func TestScratchpad_Delete(t *testing.T) {
	t.Parallel()
	sp := NewScratchpad()
	sp.Set("key", "val")
	sp.Delete("key")
	_, ok := sp.Get("key")
	if ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestScratchpad_SnapshotEmpty(t *testing.T) {
	t.Parallel()
	sp := NewScratchpad()
	if s := sp.Snapshot(); s != "" {
		t.Fatalf("expected empty snapshot, got %q", s)
	}
}

func TestScratchpad_SnapshotSorted(t *testing.T) {
	t.Parallel()
	sp := NewScratchpad()
	sp.Set("zebra", "z-content")
	sp.Set("alpha", "a-content")
	s := sp.Snapshot()
	alphaIdx := strings.Index(s, "alpha")
	zebraIdx := strings.Index(s, "zebra")
	if alphaIdx < 0 || zebraIdx < 0 {
		t.Fatalf("expected both keys in snapshot: %q", s)
	}
	if alphaIdx > zebraIdx {
		t.Fatalf("expected alpha before zebra in snapshot: %q", s)
	}
}

func TestScratchpad_SnapshotSizeLimit(t *testing.T) {
	t.Parallel()
	sp := NewScratchpad()
	// Write entries that exceed maxSnapshotBytes
	for i := 0; i < 20; i++ {
		sp.Set(fmt.Sprintf("key%02d", i), strings.Repeat("x", 1000))
	}
	s := sp.Snapshot()
	if len(s) > maxSnapshotBytes+200 { // allow some room for the truncation message
		t.Fatalf("snapshot too large: %d bytes (limit %d)", len(s), maxSnapshotBytes)
	}
	if !strings.Contains(s, "omitted due to size limit") {
		t.Fatal("expected truncation message in snapshot")
	}
}

func TestScratchpadWrite_Tool(t *testing.T) {
	t.Parallel()
	sp := NewScratchpad()
	ConfigureScratchpad(sp)
	defer ConfigureScratchpad(nil)

	result := ScratchpadWrite(nil, "tc1", `{"key":"progress","content":"step 1 done"}`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	v, ok := sp.Get("progress")
	if !ok || v != "step 1 done" {
		t.Fatalf("expected 'step 1 done', got %q", v)
	}
}

func TestScratchpadRead_AllEntries(t *testing.T) {
	t.Parallel()
	sp := NewScratchpad()
	ConfigureScratchpad(sp)
	defer ConfigureScratchpad(nil)

	sp.Set("a", "alpha")
	sp.Set("b", "beta")

	result := ScratchpadRead(nil, "tc1", `{}`)
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "alpha") || !strings.Contains(result.Output, "beta") {
		t.Fatalf("expected both entries, got: %q", result.Output)
	}
}

func TestScratchpadRead_SingleKey(t *testing.T) {
	t.Parallel()
	sp := NewScratchpad()
	ConfigureScratchpad(sp)
	defer ConfigureScratchpad(nil)

	sp.Set("progress", "done")
	result := ScratchpadRead(nil, "tc1", `{"key":"progress"}`)
	if result.Output != "done" {
		t.Fatalf("expected 'done', got %q", result.Output)
	}
}

func TestScratchpadWrite_MissingKey(t *testing.T) {
	t.Parallel()
	sp := NewScratchpad()
	ConfigureScratchpad(sp)
	defer ConfigureScratchpad(nil)

	result := ScratchpadWrite(nil, "tc1", `{"key":"","content":"x"}`)
	if result.Error == nil {
		t.Fatal("expected error for empty key")
	}
}
