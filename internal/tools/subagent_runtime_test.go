package tools

import "testing"

func TestCancelAllRunningSubagentsWithoutManager(t *testing.T) {
	subagentMu.Lock()
	orig := subagentManager
	subagentManager = nil
	subagentMu.Unlock()
	defer func() {
		subagentMu.Lock()
		subagentManager = orig
		subagentMu.Unlock()
	}()

	if got := CancelAllRunningSubagents(); got != 0 {
		t.Fatalf("CancelAllRunningSubagents() = %d, want 0", got)
	}
}
