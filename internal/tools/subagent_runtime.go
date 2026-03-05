package tools

import (
	"sync"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
	"github.com/tokuhirom/ashron/internal/subagent"
)

var (
	subagentMu      sync.RWMutex
	subagentManager *subagent.Manager
)

func ConfigureSubagentRuntime(client *api.Client, ctxCfg *config.ContextConfig) {
	subagentMu.Lock()
	defer subagentMu.Unlock()
	subagentManager = subagent.NewManager(client, ctxCfg)
}

func getSubagentManager() *subagent.Manager {
	subagentMu.RLock()
	defer subagentMu.RUnlock()
	return subagentManager
}

// SubagentSummary is a lightweight view of a running subagent for the TUI.
type SubagentSummary struct {
	ID       string
	LastLine string
}

// GetSubagentsSummary returns summaries of all currently running subagents.
// Safe to call from any goroutine; intended for TUI polling.
func GetSubagentsSummary() []SubagentSummary {
	mgr := getSubagentManager()
	if mgr == nil {
		return nil
	}
	raw := mgr.GetRunningSummary()
	out := make([]SubagentSummary, len(raw))
	for i, s := range raw {
		out[i] = SubagentSummary{ID: s.ID, LastLine: s.LastLine}
	}
	return out
}

// GetSubagentLog returns the full accumulated log for the given subagent.
func GetSubagentLog(id string) (string, error) {
	mgr := getSubagentManager()
	if mgr == nil {
		return "", nil
	}
	return mgr.GetLog(id)
}

// CancelAllRunningSubagents cancels and closes all currently running subagents.
// Returns how many subagents were cancelled.
func CancelAllRunningSubagents() int {
	mgr := getSubagentManager()
	if mgr == nil {
		return 0
	}
	snaps := mgr.List()
	cancelled := 0
	for _, s := range snaps {
		if s.Status != subagent.AgentStatusRunning {
			continue
		}
		if err := mgr.Close(s.ID); err == nil {
			cancelled++
		}
	}
	return cancelled
}
