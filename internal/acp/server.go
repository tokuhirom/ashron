package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
	"github.com/tokuhirom/ashron/internal/tools"
)

const protocolVersion = 1

type session struct {
	id       string
	cwd      string
	messages []api.Message
	cancel   context.CancelFunc
}

type pendingCall struct {
	resultCh chan json.RawMessage
	errCh    chan *RPCError
}

// Server implements the ACP (Agent Client Protocol) over stdin/stdout JSON-RPC 2.0.
type Server struct {
	cfg       *config.Config
	apiClient *api.Client
	toolExec  *tools.Executor
	version   string

	mu       sync.Mutex
	sessions map[string]*session

	writerMu sync.Mutex
	encoder  *json.Encoder

	pendingMu sync.Mutex
	pending   map[int64]*pendingCall

	nextSessionID atomic.Int64
	nextReqID     atomic.Int64
}

// NewServer creates a new ACP server. Tools run in yolo mode (auto-approve all);
// dangerous tools are gated via session/request_permission sent to the client.
func NewServer(cfg *config.Config, apiClient *api.Client, version string) *Server {
	// ACP server approves tools itself via session/request_permission;
	// set Yolo so the executor never blocks waiting for TUI approval.
	toolsCfg := cfg.Tools
	toolsCfg.Yolo = true
	return &Server{
		cfg:       cfg,
		apiClient: apiClient,
		toolExec:  tools.NewExecutor(&toolsCfg),
		version:   version,
		sessions:  make(map[string]*session),
		encoder:   json.NewEncoder(os.Stdout),
		pending:   make(map[int64]*pendingCall),
	}
}

// Run reads newline-delimited JSON-RPC messages from stdin until EOF.
func (s *Server) Run() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		s.handleLine(line)
	}
	return scanner.Err()
}

func (s *Server) handleLine(line string) {
	// Peek at the raw JSON to decide if it is a request, notification, or response.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		slog.Error("ACP: failed to parse JSON", "error", err)
		return
	}

	_, hasMethod := raw["method"]
	_, hasID := raw["id"]

	if !hasMethod && hasID {
		// Response to one of our outgoing requests.
		s.handleClientResponse(raw)
		return
	}

	var req Request
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		slog.Error("ACP: failed to unmarshal request", "error", err)
		return
	}

	if req.ID == nil {
		// Notification – no response expected.
		s.handleNotification(req)
		return
	}

	// Dispatch requests. session/prompt runs in its own goroutine so that the
	// main scanner loop stays unblocked for cancel notifications.
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "session/new":
		s.handleSessionNew(req)
	case "session/prompt":
		go s.handleSessionPrompt(req)
	default:
		s.sendError(req.ID, -32601, "method not found: "+req.Method)
	}
}

func (s *Server) handleClientResponse(raw map[string]json.RawMessage) {
	var id int64
	if err := json.Unmarshal(raw["id"], &id); err != nil {
		slog.Error("ACP: failed to parse client response id", "error", err)
		return
	}

	s.pendingMu.Lock()
	call, ok := s.pending[id]
	delete(s.pending, id)
	s.pendingMu.Unlock()

	if !ok {
		slog.Warn("ACP: received response for unknown id", "id", id)
		return
	}

	if errJSON, hasErr := raw["error"]; hasErr {
		var rpcErr RPCError
		_ = json.Unmarshal(errJSON, &rpcErr)
		call.errCh <- &rpcErr
		return
	}

	result := raw["result"]
	call.resultCh <- result
}

func (s *Server) handleNotification(req Request) {
	switch req.Method {
	case "session/cancel":
		var params SessionCancelParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return
		}
		s.mu.Lock()
		sess, ok := s.sessions[params.SessionID]
		s.mu.Unlock()
		if ok && sess.cancel != nil {
			sess.cancel()
		}
	}
}

func (s *Server) handleInitialize(req Request) {
	result := InitializeResult{
		ProtocolVersion: protocolVersion,
		AgentInfo: AgentInfo{
			Name:    "ashron",
			Title:   "Ashron",
			Version: s.version,
		},
		AgentCapabilities: AgentCapabilities{},
		AuthMethods:       []string{},
	}
	s.sendResult(req.ID, result)
}

func (s *Server) handleSessionNew(req Request) {
	var params SessionNewParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, -32602, "invalid params: "+err.Error())
		return
	}

	cwd := params.CWD
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	id := fmt.Sprintf("sess_%d", s.nextSessionID.Add(1))

	s.mu.Lock()
	s.sessions[id] = &session{id: id, cwd: cwd}
	s.mu.Unlock()

	s.sendResult(req.ID, SessionNewResult{SessionID: id})
}

func (s *Server) handleSessionPrompt(req Request) {
	var params SessionPromptParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, -32602, "invalid params: "+err.Error())
		return
	}

	s.mu.Lock()
	sess, ok := s.sessions[params.SessionID]
	s.mu.Unlock()
	if !ok {
		s.sendError(req.ID, -32602, "session not found: "+params.SessionID)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	sess.cancel = cancel
	s.mu.Unlock()
	defer cancel()

	// Switch to the session's working directory.
	if sess.cwd != "" {
		if err := os.Chdir(sess.cwd); err != nil {
			slog.Warn("ACP: failed to chdir", "cwd", sess.cwd, "error", err)
		}
	}

	sess.messages = append(sess.messages, api.NewUserMessage(params.Prompt))
	builtinTools := tools.SelectBuiltinTools(params.Prompt)

	// Agentic loop: stream → execute tools → stream again until no tool calls.
	for {
		if ctx.Err() != nil {
			s.sendResult(req.ID, SessionPromptResult{StopReason: "cancelled"})
			return
		}

		stream, err := s.apiClient.StreamChatCompletionWithTools(ctx, sess.messages, builtinTools)
		if err != nil {
			if ctx.Err() != nil {
				s.sendResult(req.ID, SessionPromptResult{StopReason: "cancelled"})
				return
			}
			s.sendError(req.ID, -32000, "API error: "+err.Error())
			return
		}

		var fullContent strings.Builder
		var toolCalls []api.ToolCall
		toolCallArgs := make(map[int]*strings.Builder)
		toolCallsByIndex := make(map[int]*api.ToolCall)

		for event := range stream {
			if ctx.Err() != nil {
				s.sendResult(req.ID, SessionPromptResult{StopReason: "cancelled"})
				return
			}
			if event.Error != nil {
				s.sendError(req.ID, -32000, "stream error: "+event.Error.Error())
				return
			}
			if event.Data == nil || len(event.Data.Choices) == 0 {
				continue
			}

			choice := event.Data.Choices[0]

			if choice.Delta.Content != "" {
				fullContent.WriteString(choice.Delta.Content)
				s.sendNotification("session/update", SessionUpdateParams{
					SessionID: params.SessionID,
					Update: SessionUpdate{
						SessionUpdate: "agent_message_chunk",
						Chunk:         choice.Delta.Content,
					},
				})
			}

			for _, dtc := range choice.Delta.ToolCalls {
				idx := dtc.Index
				if dtc.ID != "" {
					tc := &api.ToolCall{
						ID:   dtc.ID,
						Type: dtc.Type,
						Function: api.FunctionCall{
							Name:      dtc.Function.Name,
							Arguments: dtc.Function.Arguments,
						},
					}
					toolCallsByIndex[idx] = tc
					toolCallArgs[idx] = &strings.Builder{}
				} else if dtc.Function.Arguments != "" {
					if _, exists := toolCallsByIndex[idx]; exists {
						toolCallArgs[idx].WriteString(dtc.Function.Arguments)
					}
				}
			}

			if choice.FinishReason == "stop" || choice.FinishReason == "tool_calls" {
				for idx, tc := range toolCallsByIndex {
					if toolCallArgs[idx] != nil && toolCallArgs[idx].Len() > 0 {
						tc.Function.Arguments = toolCallArgs[idx].String()
					}
					if tc.Function.Arguments == "" {
						tc.Function.Arguments = "{}"
					}
					toolCalls = append(toolCalls, *tc)
				}
				sess.messages = append(sess.messages, api.Message{
					Role:      "assistant",
					Content:   fullContent.String(),
					ToolCalls: toolCalls,
				})
				break
			}
		}

		if len(toolCalls) == 0 {
			break
		}

		// Execute tools one by one.
		for _, tc := range toolCalls {
			if ctx.Err() != nil {
				s.sendResult(req.ID, SessionPromptResult{StopReason: "cancelled"})
				return
			}

			// Ask the client for permission if the tool is not auto-approved.
			if !s.isAutoApproved(tc) {
				approved, err := s.requestPermission(ctx, params.SessionID, tc)
				if err != nil || !approved {
					slog.Info("ACP: tool denied or permission error",
						"tool", tc.Function.Name, "error", err)
					sess.messages = append(sess.messages,
						api.NewToolMessage(tc.ID, "Tool execution was denied by the user."))
					s.sendNotification("session/update", SessionUpdateParams{
						SessionID: params.SessionID,
						Update: SessionUpdate{
							SessionUpdate: "tool_call_update",
							ToolCallID:    tc.ID,
							Status:        "failed",
						},
					})
					continue
				}
			}

			s.sendNotification("session/update", SessionUpdateParams{
				SessionID: params.SessionID,
				Update: SessionUpdate{
					SessionUpdate: "tool_call",
					ToolCallID:    tc.ID,
					Title:         tc.Function.Name,
					Status:        "in_progress",
				},
			})

			result := s.toolExec.Execute(tc)
			historyOutput := tools.CompactToolResultForHistory(tc.Function.Name, result.Output)
			sess.messages = append(sess.messages, api.NewToolMessage(tc.ID, historyOutput))

			status := "completed"
			if result.Error != nil {
				status = "failed"
			}

			s.sendNotification("session/update", SessionUpdateParams{
				SessionID: params.SessionID,
				Update: SessionUpdate{
					SessionUpdate: "tool_call_update",
					ToolCallID:    tc.ID,
					Status:        status,
				},
			})
		}
	}

	s.sendResult(req.ID, SessionPromptResult{StopReason: "completed"})
}

// isAutoApproved returns true if a tool call should be executed without prompting the user.
func (s *Server) isAutoApproved(tc api.ToolCall) bool {
	if s.cfg.Tools.Yolo {
		return true
	}
	for _, name := range s.cfg.Tools.AutoApproveTools {
		if name == tc.Function.Name {
			return true
		}
	}
	if tc.Function.Name == "execute_command" {
		var args tools.ExecuteCommandArgs
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return false
		}
		if strings.EqualFold(tools.EffectiveSandboxMode(&s.cfg.Tools, args), "off") {
			return false
		}
		for _, cmd := range s.cfg.Tools.AutoApproveCommands {
			if strings.HasPrefix(cmd, "/") && strings.HasSuffix(cmd, "/") {
				pattern := strings.TrimPrefix(strings.TrimSuffix(cmd, "/"), "/")
				matched, err := regexp.MatchString(pattern, args.Command)
				if err == nil && matched {
					return true
				}
			} else if cmd == args.Command {
				return true
			}
		}
	}
	return false
}

// requestPermission sends a session/request_permission call to the client and
// waits for the response. Returns (true, nil) if approved.
func (s *Server) requestPermission(ctx context.Context, sessionID string, tc api.ToolCall) (bool, error) {
	id := s.nextReqID.Add(1)

	call := &pendingCall{
		resultCh: make(chan json.RawMessage, 1),
		errCh:    make(chan *RPCError, 1),
	}

	s.pendingMu.Lock()
	s.pending[id] = call
	s.pendingMu.Unlock()

	description := tc.Function.Name
	if tc.Function.Arguments != "" && tc.Function.Arguments != "{}" {
		if len(tc.Function.Arguments) < 200 {
			description = tc.Function.Name + " " + tc.Function.Arguments
		}
	}

	req := OutgoingRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "session/request_permission",
		Params: RequestPermissionParams{
			SessionID: sessionID,
			ToolCall: PermissionToolCall{
				ToolCallID:  tc.ID,
				Title:       tc.Function.Name,
				Description: description,
			},
			Options: []PermissionOption{
				{ID: "approve", Title: "Approve"},
				{ID: "deny", Title: "Deny"},
			},
		},
	}

	s.writerMu.Lock()
	err := s.encoder.Encode(req)
	s.writerMu.Unlock()
	if err != nil {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return false, fmt.Errorf("write permission request: %w", err)
	}

	select {
	case <-ctx.Done():
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return false, ctx.Err()
	case rpcErr := <-call.errCh:
		return false, fmt.Errorf("permission error %d: %s", rpcErr.Code, rpcErr.Message)
	case result := <-call.resultCh:
		var outcome struct {
			Outcome  string `json:"outcome"`
			OptionID string `json:"optionId"`
		}
		if err := json.Unmarshal(result, &outcome); err != nil {
			return false, fmt.Errorf("parse permission response: %w", err)
		}
		return outcome.Outcome == "selected" && outcome.OptionID == "approve", nil
	}
}

func (s *Server) sendResult(id *int64, result interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	s.writerMu.Lock()
	defer s.writerMu.Unlock()
	if err := s.encoder.Encode(resp); err != nil {
		slog.Error("ACP: failed to write response", "error", err)
	}
}

func (s *Server) sendError(id *int64, code int, message string) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message},
	}
	s.writerMu.Lock()
	defer s.writerMu.Unlock()
	if err := s.encoder.Encode(resp); err != nil {
		slog.Error("ACP: failed to write error response", "error", err)
	}
}

func (s *Server) sendNotification(method string, params interface{}) {
	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	s.writerMu.Lock()
	defer s.writerMu.Unlock()
	if err := s.encoder.Encode(notif); err != nil {
		slog.Error("ACP: failed to write notification", "error", err)
	}
}
