package subagent

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

type AgentStatus string

const (
	AgentStatusIdle      AgentStatus = "idle"
	AgentStatusRunning   AgentStatus = "running"
	AgentStatusCompleted AgentStatus = "completed"
	AgentStatusFailed    AgentStatus = "failed"
	AgentStatusCanceled  AgentStatus = "canceled"
)

type AgentSnapshot struct {
	ID         string      `json:"id"`
	Status     AgentStatus `json:"status"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	LastOutput string      `json:"last_output,omitempty"`
	LastError  string      `json:"last_error,omitempty"`
}

type managerAgent struct {
	id        string
	status    AgentStatus
	messages  []api.Message
	createdAt time.Time
	updatedAt time.Time
	output    string
	err       string
	cancel    context.CancelFunc
	done      chan struct{}
}

type Manager struct {
	mu        sync.RWMutex
	apiClient *api.Client
	ctxConfig *config.ContextConfig
	agents    map[string]*managerAgent
	nextID    uint64
}

func NewManager(client *api.Client, ctxConfig *config.ContextConfig) *Manager {
	return &Manager{
		apiClient: client,
		ctxConfig: ctxConfig,
		agents:    make(map[string]*managerAgent),
	}
}

func (m *Manager) Spawn(prompt string) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("prompt is required")
	}

	id := fmt.Sprintf("subagent-%d", atomic.AddUint64(&m.nextID, 1))
	now := time.Now()
	ag := &managerAgent{
		id:        id,
		status:    AgentStatusIdle,
		messages:  []api.Message{api.NewUserMessage(prompt)},
		createdAt: now,
		updatedAt: now,
		done:      make(chan struct{}),
	}

	m.mu.Lock()
	m.agents[id] = ag
	m.mu.Unlock()

	if err := m.startRun(id); err != nil {
		return "", err
	}
	return id, nil
}

func (m *Manager) SendInput(id, input string) error {
	if strings.TrimSpace(input) == "" {
		return fmt.Errorf("input is required")
	}

	m.mu.Lock()
	ag, ok := m.agents[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("subagent not found: %s", id)
	}
	if ag.status == AgentStatusRunning {
		m.mu.Unlock()
		return fmt.Errorf("subagent is still running: %s", id)
	}
	ag.messages = append(ag.messages, api.NewUserMessage(input))
	ag.done = make(chan struct{})
	ag.output = ""
	ag.err = ""
	ag.updatedAt = time.Now()
	m.mu.Unlock()

	return m.startRun(id)
}

func (m *Manager) Wait(id string, timeout time.Duration) (AgentSnapshot, bool, error) {
	m.mu.RLock()
	ag, ok := m.agents[id]
	if !ok {
		m.mu.RUnlock()
		return AgentSnapshot{}, false, fmt.Errorf("subagent not found: %s", id)
	}
	done := ag.done
	m.mu.RUnlock()

	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	select {
	case <-done:
		s, err := m.Snapshot(id)
		return s, false, err
	case <-time.After(timeout):
		s, err := m.Snapshot(id)
		return s, true, err
	}
}

func (m *Manager) Snapshot(id string) (AgentSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ag, ok := m.agents[id]
	if !ok {
		return AgentSnapshot{}, fmt.Errorf("subagent not found: %s", id)
	}
	return AgentSnapshot{
		ID:         ag.id,
		Status:     ag.status,
		CreatedAt:  ag.createdAt,
		UpdatedAt:  ag.updatedAt,
		LastOutput: ag.output,
		LastError:  ag.err,
	}, nil
}

func (m *Manager) List() []AgentSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]AgentSnapshot, 0, len(m.agents))
	for _, ag := range m.agents {
		out = append(out, AgentSnapshot{
			ID:         ag.id,
			Status:     ag.status,
			CreatedAt:  ag.createdAt,
			UpdatedAt:  ag.updatedAt,
			LastOutput: ag.output,
			LastError:  ag.err,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (m *Manager) Close(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ag, ok := m.agents[id]
	if !ok {
		return fmt.Errorf("subagent not found: %s", id)
	}
	if ag.cancel != nil {
		ag.cancel()
	}
	delete(m.agents, id)
	return nil
}

func (m *Manager) startRun(id string) error {
	m.mu.Lock()
	ag, ok := m.agents[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("subagent not found: %s", id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	ag.cancel = cancel
	ag.status = AgentStatusRunning
	ag.updatedAt = time.Now()
	done := ag.done
	messages := make([]api.Message, len(ag.messages))
	copy(messages, ag.messages)
	m.mu.Unlock()

	stream, err := m.apiClient.StreamChatCompletionWithTools(ctx, messages, nil)
	if err != nil {
		cancel()
		m.mu.Lock()
		ag.status = AgentStatusFailed
		ag.err = err.Error()
		ag.updatedAt = time.Now()
		close(done)
		m.mu.Unlock()
		return nil
	}

	go func() {
		defer cancel()
		var sb strings.Builder
		status := AgentStatusCompleted
		errText := ""

		for ev := range stream {
			if ev.Error != nil {
				status = AgentStatusFailed
				errText = ev.Error.Error()
				break
			}
			if ev.Data == nil {
				continue
			}
			for _, ch := range ev.Data.Choices {
				if ch.Delta.Content != "" {
					sb.WriteString(ch.Delta.Content)
				}
			}
		}

		if ctx.Err() == context.DeadlineExceeded {
			status = AgentStatusFailed
			errText = "subagent timed out"
		} else if ctx.Err() == context.Canceled && status == AgentStatusCompleted {
			status = AgentStatusCanceled
		}

		output := strings.TrimSpace(sb.String())
		m.mu.Lock()
		defer m.mu.Unlock()
		cur, ok := m.agents[id]
		if !ok {
			return
		}
		cur.status = status
		cur.output = output
		cur.err = errText
		cur.updatedAt = time.Now()
		if output != "" {
			cur.messages = append(cur.messages, api.Message{Role: "assistant", Content: output})
		}
		close(done)
	}()

	return nil
}
