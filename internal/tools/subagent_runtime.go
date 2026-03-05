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
