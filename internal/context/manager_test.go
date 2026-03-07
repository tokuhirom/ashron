package context

import (
	"strings"
	"testing"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

func makeMessages(roles ...string) []api.Message {
	msgs := make([]api.Message, len(roles))
	for i, role := range roles {
		msgs[i] = api.Message{Role: role, Content: strings.Repeat("x", 100)}
	}
	return msgs
}

func TestCompactionLevel_None(t *testing.T) {
	t.Parallel()
	mgr := NewManager(&config.ContextConfig{
		MaxTokens:   10000,
		MaxMessages: 100,
		AutoCompact: true,
	})
	// 3 messages × 100 chars = 300 chars ≈ 75 tokens → well below 80%
	msgs := makeMessages("user", "assistant", "user")
	if lvl := mgr.CompactionLevel(msgs); lvl != CompactionNone {
		t.Fatalf("expected CompactionNone, got %d", lvl)
	}
}

func TestCompactionLevel_PruneAt80Percent(t *testing.T) {
	t.Parallel()
	mgr := NewManager(&config.ContextConfig{
		MaxTokens:   1000,
		MaxMessages: 1000,
		AutoCompact: true,
	})
	// 1000 tokens * 4 chars = 4000 chars needed. 80% = 800 tokens = 3200 chars.
	// Create messages totaling ~3400 chars → ~850 tokens (above 80%, below 90%).
	msgs := []api.Message{{Role: "user", Content: strings.Repeat("x", 3400)}}
	lvl := mgr.CompactionLevel(msgs)
	if lvl != CompactionPrune {
		t.Fatalf("expected CompactionPrune, got %d", lvl)
	}
}

func TestCompactionLevel_SummarizeAt90Percent(t *testing.T) {
	t.Parallel()
	mgr := NewManager(&config.ContextConfig{
		MaxTokens:   1000,
		MaxMessages: 1000,
		AutoCompact: true,
	})
	// 90% = 900 tokens = 3600 chars. Create ~3800 chars → ~950 tokens.
	msgs := []api.Message{{Role: "user", Content: strings.Repeat("x", 3800)}}
	lvl := mgr.CompactionLevel(msgs)
	if lvl != CompactionSummarize {
		t.Fatalf("expected CompactionSummarize, got %d", lvl)
	}
}

func TestCompactionLevel_MaxMessagesTriggersCompaction(t *testing.T) {
	t.Parallel()
	mgr := NewManager(&config.ContextConfig{
		MaxTokens:   100000,
		MaxMessages: 5,
		AutoCompact: true,
	})
	msgs := makeMessages("user", "assistant", "user", "assistant", "user", "assistant")
	lvl := mgr.CompactionLevel(msgs)
	if lvl != CompactionSummarize {
		t.Fatalf("expected CompactionSummarize for max messages, got %d", lvl)
	}
}

func TestCompactionLevel_DisabledAutoCompact(t *testing.T) {
	t.Parallel()
	mgr := NewManager(&config.ContextConfig{
		MaxTokens:   100,
		MaxMessages: 1,
		AutoCompact: false,
	})
	msgs := []api.Message{{Role: "user", Content: strings.Repeat("x", 10000)}}
	if lvl := mgr.CompactionLevel(msgs); lvl != CompactionNone {
		t.Fatalf("expected CompactionNone when auto compact disabled, got %d", lvl)
	}
}

func TestCompactionLevel_CustomCompactionRatioOverrides(t *testing.T) {
	t.Parallel()
	mgr := NewManager(&config.ContextConfig{
		MaxTokens:       1000,
		MaxMessages:     1000,
		CompactionRatio: 0.5, // lower than default 0.9
		AutoCompact:     true,
	})
	// 50% = 500 tokens = 2000 chars. ~2200 chars → ~550 tokens.
	msgs := []api.Message{{Role: "user", Content: strings.Repeat("x", 2200)}}
	lvl := mgr.CompactionLevel(msgs)
	if lvl != CompactionSummarize {
		t.Fatalf("expected CompactionSummarize with custom ratio, got %d", lvl)
	}
}

func TestNeedsCompaction_BackwardCompat(t *testing.T) {
	t.Parallel()
	mgr := NewManager(&config.ContextConfig{
		MaxTokens:   1000,
		MaxMessages: 1000,
		AutoCompact: true,
	})
	// Above 90% → NeedsCompaction should return true
	msgs := []api.Message{{Role: "user", Content: strings.Repeat("x", 3800)}}
	if !mgr.NeedsCompaction(msgs) {
		t.Fatal("expected NeedsCompaction to return true above 90%")
	}
	// Between 80% and 90% → NeedsCompaction should return false (only prune)
	msgs = []api.Message{{Role: "user", Content: strings.Repeat("x", 3400)}}
	if mgr.NeedsCompaction(msgs) {
		t.Fatal("expected NeedsCompaction to return false between 80% and 90%")
	}
}

func TestCompactionLevel_ZeroCompactionRatioUsesDefault(t *testing.T) {
	t.Parallel()
	mgr := NewManager(&config.ContextConfig{
		MaxTokens:       1000,
		MaxMessages:     1000,
		CompactionRatio: 0, // zero value — should fall back to summarizeRatio (0.90)
		AutoCompact:     true,
	})
	// 85% = 850 tokens = 3400 chars → should be Prune (below 90%)
	msgs := []api.Message{{Role: "user", Content: strings.Repeat("x", 3400)}}
	lvl := mgr.CompactionLevel(msgs)
	if lvl != CompactionPrune {
		t.Fatalf("expected CompactionPrune with zero CompactionRatio, got %d", lvl)
	}
	// 95% = 950 tokens = 3800 chars → should be Summarize
	msgs = []api.Message{{Role: "user", Content: strings.Repeat("x", 3800)}}
	lvl = mgr.CompactionLevel(msgs)
	if lvl != CompactionSummarize {
		t.Fatalf("expected CompactionSummarize with zero CompactionRatio, got %d", lvl)
	}
}

func TestPrune_TruncatesOldToolOutput(t *testing.T) {
	t.Parallel()
	mgr := NewManager(&config.ContextConfig{MaxTokens: 10000})
	msgs := make([]api.Message, 25)
	msgs[0] = api.Message{Role: "user", Content: "hello"}
	msgs[1] = api.Message{Role: "assistant", Content: "hi"}
	msgs[2] = api.Message{Role: "tool", Content: strings.Repeat("x", 500), ToolCallID: "tc1"}
	for i := 3; i < 25; i++ {
		if i%2 == 0 {
			msgs[i] = api.Message{Role: "user", Content: "msg"}
		} else {
			msgs[i] = api.Message{Role: "assistant", Content: "resp"}
		}
	}
	pruned := mgr.Prune(msgs)
	if len(pruned[2].Content) >= 500 {
		t.Fatalf("expected tool output to be truncated, got %d bytes", len(pruned[2].Content))
	}
	if !strings.Contains(pruned[2].Content, "[truncated]") {
		t.Fatal("expected truncation marker")
	}
}

func TestBuildCompacted_PreservesSystemMessages(t *testing.T) {
	t.Parallel()
	mgr := NewManager(&config.ContextConfig{MaxTokens: 10000})
	msgs := []api.Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	result := mgr.BuildCompacted("summary of conversation", msgs)
	if result[0].Role != "system" || result[0].Content != "system prompt" {
		t.Fatal("first message should be original system prompt")
	}
	if result[1].Role != "system" || !strings.Contains(result[1].Content, "summary of conversation") {
		t.Fatal("second message should be the summary")
	}
}
