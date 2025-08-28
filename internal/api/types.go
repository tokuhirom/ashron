package api

import (
	"encoding/json"
	"time"
)

// ChatCompletionRequest represents a chat completion API request
type ChatCompletionRequest struct {
	Model       string      `json:"model"`
	Messages    []Message   `json:"messages"`
	Temperature float32     `json:"temperature,omitempty"`
	MaxTokens   int         `json:"max_tokens,omitempty"`
	Stream      bool        `json:"stream,omitempty"`
	Tools       []Tool      `json:"tools,omitempty"`
	ToolChoice  interface{} `json:"tool_choice,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Tool represents a function that can be called by the model
type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef defines a function that can be called
type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall represents a function call request from the model
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall contains the function name and arguments
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatCompletionResponse represents the API response
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a response choice
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	Delta        Message `json:"delta,omitempty"`
	FinishReason string  `json:"finish_reason"`
}

// Usage tracks token usage
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamResponse represents a streaming response chunk
type StreamResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

// Error response from the API
type ErrorResponse struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// Tool execution results
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
	Error      error  `json:"error,omitempty"`
}

// Internal tool definitions for file and command operations
var BuiltinTools = []Tool{
	{
		Type: "function",
		Function: FunctionDef{
			Name:        "read_file",
			Description: "Read the contents of a file",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "The file path to read"
					}
				},
				"required": ["path"]
			}`),
		},
	},
	{
		Type: "function",
		Function: FunctionDef{
			Name:        "write_file",
			Description: "Write content to a file",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "The file path to write"
					},
					"content": {
						"type": "string",
						"description": "The content to write"
					}
				},
				"required": ["path", "content"]
			}`),
		},
	},
	{
		Type: "function",
		Function: FunctionDef{
			Name:        "execute_command",
			Description: "Execute a shell command",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"command": {
						"type": "string",
						"description": "The command to execute"
					},
					"working_dir": {
						"type": "string",
						"description": "Working directory for the command"
					}
				},
				"required": ["command"]
			}`),
		},
	},
	{
		Type: "function",
		Function: FunctionDef{
			Name:        "list_directory",
			Description: "List files in a directory",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "The directory path to list"
					}
				},
				"required": ["path"]
			}`),
		},
	},
	{
		Type: "function",
		Function: FunctionDef{
			Name:        "init",
			Description: "Generate an AGENTS.md file for the project",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {},
				"required": []
			}`),
		},
	},
	{
		Type: "function",
		Function: FunctionDef{
			Name:        "list_tools",
			Description: "List all available tools and their descriptions",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"format": {
						"type": "string",
						"description": "Output format: 'text' (default) or 'json'"
					}
				},
				"required": []
			}`),
		},
	},
}

// Helper function to create a user message
func NewUserMessage(content string) Message {
	return Message{
		Role:    "user",
		Content: content,
	}
}

// Helper function to create an assistant message
func NewAssistantMessage(content string) Message {
	return Message{
		Role:    "assistant",
		Content: content,
	}
}

// Helper function to create a system message
func NewSystemMessage(content string) Message {
	return Message{
		Role:    "system",
		Content: content,
	}
}

// Helper function to create a tool message
func NewToolMessage(toolCallID, content string) Message {
	return Message{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	}
}

// ChatSession manages the conversation state
type ChatSession struct {
	Messages     []Message
	TotalTokens  int
	CreatedAt    time.Time
	LastActivity time.Time
}
