package acp

import "encoding/json"

// JSON-RPC 2.0 base types

// Request is an incoming JSON-RPC request or notification from the client.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is sent back to the client for a request.
type Response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      *int64    `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

// Notification is a one-way message with no id.
type Notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// OutgoingRequest is a request sent from the agent to the client (bidirectional RPC).
type OutgoingRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// ClientResponse is a response from the client to one of our outgoing requests.
type ClientResponse struct {
	ID     *int64          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *RPCError       `json:"error,omitempty"`
}

// RPCError follows the JSON-RPC 2.0 error format.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ACP protocol types

type AgentInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Version string `json:"version"`
}

type AgentCapabilities struct {
	LoadSession bool `json:"loadSession,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentInfo         AgentInfo         `json:"agentInfo"`
	AgentCapabilities AgentCapabilities `json:"agentCapabilities"`
	AuthMethods       []string          `json:"authMethods"`
}

type SessionNewParams struct {
	CWD string `json:"cwd"`
}

type SessionNewResult struct {
	SessionID string `json:"sessionId"`
}

type SessionPromptParams struct {
	SessionID string `json:"sessionId"`
	Prompt    string `json:"prompt"`
}

type SessionPromptResult struct {
	StopReason string `json:"stopReason"` // "completed", "cancelled", "error"
}

type SessionCancelParams struct {
	SessionID string `json:"sessionId"`
}

// session/update notification types

type SessionUpdateParams struct {
	SessionID string        `json:"sessionId"`
	Update    SessionUpdate `json:"update"`
}

type SessionUpdate struct {
	SessionUpdate string `json:"sessionUpdate"`
	// agent_message_chunk
	Chunk string `json:"chunk,omitempty"`
	// tool_call / tool_call_update
	ToolCallID string `json:"toolCallId,omitempty"`
	Title      string `json:"title,omitempty"`
	Status     string `json:"status,omitempty"` // "pending", "in_progress", "completed", "failed"
}

// session/request_permission types

type PermissionOption struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type PermissionToolCall struct {
	ToolCallID  string `json:"toolCallId"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

type RequestPermissionParams struct {
	SessionID string             `json:"sessionId"`
	ToolCall  PermissionToolCall `json:"toolCall"`
	Options   []PermissionOption `json:"options"`
}

type PermissionOutcomeSelected struct {
	Outcome  string `json:"outcome"`
	OptionID string `json:"optionId"`
}

type PermissionOutcomeCancelled struct {
	Outcome string `json:"outcome"`
}
