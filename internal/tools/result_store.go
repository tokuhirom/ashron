package tools

import "sync"

// ResultStore holds full tool outputs keyed by tool call ID.
// Older tool messages in the conversation history are replaced with lightweight
// stubs when sending to the API; the AI can use get_tool_result to fetch the
// full content on demand.
type ResultStore struct {
	mu      sync.RWMutex
	results map[string]string
}

// NewResultStore creates an empty ResultStore.
func NewResultStore() *ResultStore {
	return &ResultStore{results: make(map[string]string)}
}

// Store saves the full output for the given tool call ID.
func (s *ResultStore) Store(id, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[id] = content
}

// Get retrieves the stored output for a tool call ID.
func (s *ResultStore) Get(id string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.results[id]
	return v, ok
}
