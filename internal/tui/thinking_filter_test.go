package tui

import (
	"strings"
	"testing"
)

// helper: feed all chunks through the filter and collect totals
func runFilter(chunks []string) (display, history string) {
	var f thinkingFilter
	var d, h strings.Builder
	for _, c := range chunks {
		dp, hp := f.Feed(c)
		d.WriteString(dp)
		h.WriteString(hp)
	}
	dp, hp := f.Flush()
	d.WriteString(dp)
	h.WriteString(hp)
	return d.String(), h.String()
}

// TestThinkingFilter_NoThinkBlock verifies that plain content (no think tags)
// passes through unchanged to both display and history.
func TestThinkingFilter_NoThinkBlock(t *testing.T) {
	d, h := runFilter([]string{"hello world"})
	if d != "hello world" {
		t.Errorf("display = %q, want %q", d, "hello world")
	}
	if h != "hello world" {
		t.Errorf("history = %q, want %q", h, "hello world")
	}
}

// TestThinkingFilter_SingleChunk verifies that a complete <think>...</think>
// block in a single chunk appears in display but not in history.
func TestThinkingFilter_SingleChunk(t *testing.T) {
	input := "<think>reasoning</think>answer"
	d, h := runFilter([]string{input})
	if d != input {
		t.Errorf("display = %q, want %q", d, input)
	}
	if h != "answer" {
		t.Errorf("history = %q, want %q", h, "answer")
	}
}

// TestThinkingFilter_SplitOpenTag verifies that <think> split across two chunks
// (e.g. "<thi" / "nk>content</think>answer") is handled correctly.
func TestThinkingFilter_SplitOpenTag(t *testing.T) {
	d, h := runFilter([]string{"<thi", "nk>inner</think>after"})
	wantDisplay := "<think>inner</think>after"
	wantHistory := "after"
	if d != wantDisplay {
		t.Errorf("display = %q, want %q", d, wantDisplay)
	}
	if h != wantHistory {
		t.Errorf("history = %q, want %q", h, wantHistory)
	}
}

// TestThinkingFilter_SplitCloseTag verifies that </think> split across chunks
// (e.g. "</thi" / "nk>answer") is handled correctly.
func TestThinkingFilter_SplitCloseTag(t *testing.T) {
	d, h := runFilter([]string{"<think>inner</thi", "nk>after"})
	wantDisplay := "<think>inner</think>after"
	wantHistory := "after"
	if d != wantDisplay {
		t.Errorf("display = %q, want %q", d, wantDisplay)
	}
	if h != wantHistory {
		t.Errorf("history = %q, want %q", h, wantHistory)
	}
}

// TestThinkingFilter_ContentBeforeThink verifies that content before the think
// block appears in both display and history.
func TestThinkingFilter_ContentBeforeThink(t *testing.T) {
	d, h := runFilter([]string{"preamble<think>thought</think>answer"})
	wantDisplay := "preamble<think>thought</think>answer"
	// history should contain preamble + answer (think block stripped)
	if d != wantDisplay {
		t.Errorf("display = %q, want %q", d, wantDisplay)
	}
	if h != "preambleanswer" {
		t.Errorf("history = %q, want %q", h, "preambleanswer")
	}
}

// TestThinkingFilter_ManyChunks simulates a realistic streaming scenario where
// tags and content arrive one character at a time.
func TestThinkingFilter_ManyChunks(t *testing.T) {
	full := "<think>step 1\nstep 2\n</think>final answer"
	// Split into single-character chunks to stress-test boundary handling.
	chunks := make([]string, len(full))
	for i, c := range full {
		chunks[i] = string(c)
	}
	d, h := runFilter(chunks)
	if d != full {
		t.Errorf("display = %q, want %q", d, full)
	}
	if h != "final answer" {
		t.Errorf("history = %q, want %q", h, "final answer")
	}
}

// TestThinkingFilter_UnclosedThink verifies that content in an unclosed <think>
// block at EOF appears in display but not in history.
func TestThinkingFilter_UnclosedThink(t *testing.T) {
	d, h := runFilter([]string{"<think>unfinished"})
	if d != "<think>unfinished" {
		t.Errorf("display = %q, want %q", d, "<think>unfinished")
	}
	if h != "" {
		t.Errorf("history = %q, want empty", h)
	}
}

// TestThinkingFilter_MultipleBlocks verifies that multiple <think> blocks in
// one response are all stripped from history.
func TestThinkingFilter_MultipleBlocks(t *testing.T) {
	input := "<think>a</think>mid<think>b</think>end"
	d, h := runFilter([]string{input})
	if d != input {
		t.Errorf("display = %q, want %q", d, input)
	}
	if h != "midend" {
		t.Errorf("history = %q, want %q", h, "midend")
	}
}
