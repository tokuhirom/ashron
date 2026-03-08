package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

// Scratchpad is a session-scoped key-value store for the agent to track
// progress, decisions, and other notes that should survive context compaction.
// Unlike persistent memory, scratchpad data is not written to disk and is
// discarded when the session ends.
type Scratchpad struct {
	mu      sync.RWMutex
	entries map[string]string
}

// NewScratchpad creates an empty scratchpad.
func NewScratchpad() *Scratchpad {
	return &Scratchpad{entries: make(map[string]string)}
}

// Set stores or updates a key-value pair.
func (s *Scratchpad) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = value
}

// Get retrieves a value by key. Returns empty string and false if not found.
func (s *Scratchpad) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.entries[key]
	return v, ok
}

// Delete removes a key from the scratchpad.
func (s *Scratchpad) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, key)
}

// maxSnapshotBytes is the maximum size of the scratchpad snapshot injected
// into context after compaction. Entries are included in sorted key order
// until the limit is reached; remaining entries are listed as truncated.
const maxSnapshotBytes = 8000

// Snapshot returns a sorted copy of all entries for injection into context.
// Returns empty string if the scratchpad is empty. The output is capped at
// maxSnapshotBytes to prevent scratchpad from consuming too much context.
func (s *Scratchpad) Snapshot() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.entries) == 0 {
		return ""
	}
	keys := make([]string, 0, len(s.entries))
	for k := range s.entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	omitted := 0
	for _, k := range keys {
		entry := fmt.Sprintf("## %s\n%s\n\n", k, s.entries[k])
		if sb.Len()+len(entry) > maxSnapshotBytes {
			omitted++
			continue
		}
		sb.WriteString(entry)
	}
	result := strings.TrimSpace(sb.String())
	if omitted > 0 {
		result += fmt.Sprintf("\n\n[%d scratchpad entries omitted due to size limit]", omitted)
	}
	return result
}

// scratchpad is the package-level instance, set by ConfigureScratchpad.
var (
	scratchpadMu sync.RWMutex
	scratchpad   *Scratchpad
)

// ConfigureScratchpad sets the package-level scratchpad instance.
func ConfigureScratchpad(sp *Scratchpad) {
	scratchpadMu.Lock()
	defer scratchpadMu.Unlock()
	scratchpad = sp
}

func getScratchpad() *Scratchpad {
	scratchpadMu.RLock()
	defer scratchpadMu.RUnlock()
	return scratchpad
}

// ScratchpadWrite handles the scratchpad_write tool call.
func ScratchpadWrite(_ *config.ToolsConfig, toolCallID string, argsJSON string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}
	var args struct {
		Key     string `json:"key"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		result.Error = err
		result.Output = "Error: " + err.Error()
		return result
	}
	if args.Key == "" {
		result.Error = fmt.Errorf("key is required")
		result.Output = "Error: key is required"
		return result
	}
	sp := getScratchpad()
	if sp == nil {
		result.Error = fmt.Errorf("scratchpad not available")
		result.Output = "Error: scratchpad not available"
		return result
	}
	sp.Set(args.Key, args.Content)
	result.Output = fmt.Sprintf("Scratchpad key %q updated.", args.Key)
	return result
}

// ScratchpadRead handles the scratchpad_read tool call.
func ScratchpadRead(_ *config.ToolsConfig, toolCallID string, argsJSON string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}
	var args struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		result.Error = err
		result.Output = "Error: " + err.Error()
		return result
	}
	sp := getScratchpad()
	if sp == nil {
		result.Error = fmt.Errorf("scratchpad not available")
		result.Output = "Error: scratchpad not available"
		return result
	}
	// If no key specified, return all entries.
	if args.Key == "" {
		snapshot := sp.Snapshot()
		if snapshot == "" {
			result.Output = "(scratchpad is empty)"
		} else {
			result.Output = snapshot
		}
		return result
	}
	v, ok := sp.Get(args.Key)
	if !ok {
		result.Output = fmt.Sprintf("No scratchpad entry for key %q", args.Key)
		return result
	}
	result.Output = v
	return result
}
