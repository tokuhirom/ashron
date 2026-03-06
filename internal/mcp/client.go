package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tokuhirom/ashron/internal/config"
)

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// CallTool invokes a tool on an external MCP server configured in ashron.yaml.
func CallTool(ctx context.Context, serverCfg config.MCPServerConfig, toolName string, arguments json.RawMessage) (string, error) {
	callCtx := ctx
	var cancel context.CancelFunc
	if serverCfg.CallTimeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, serverCfg.CallTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(callCtx, serverCfg.Command, serverCfg.Args...)
	if serverCfg.WorkingDir != "" {
		cmd.Dir = serverCfg.WorkingDir
	}
	cmd.Env = os.Environ()
	for k, v := range serverCfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("mcp stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("mcp stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("mcp stderr pipe: %w", err)
	}

	var stderrBuf bytes.Buffer
	go func() {
		_, _ = io.Copy(&stderrBuf, stderr)
	}()

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start mcp server: %w", err)
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	client := newClient(stdin, stdout)

	startupCtx := callCtx
	if serverCfg.StartupTimeout > 0 {
		startupCtx, cancel = context.WithTimeout(callCtx, serverCfg.StartupTimeout)
		defer cancel()
	}
	if err := client.initialize(startupCtx); err != nil {
		return "", fmt.Errorf("initialize mcp server: %w; stderr: %s", err, strings.TrimSpace(stderrBuf.String()))
	}

	output, err := client.callTool(callCtx, toolName, arguments)
	if err != nil {
		return "", fmt.Errorf("tools/call %s failed: %w; stderr: %s", toolName, err, strings.TrimSpace(stderrBuf.String()))
	}
	return output, nil
}

type client struct {
	writer io.Writer
	reader *bufio.Reader

	nextID atomic.Int64
	mu     sync.Mutex
}

func newClient(writer io.Writer, reader io.Reader) *client {
	return &client{
		writer: writer,
		reader: bufio.NewReader(reader),
	}
}

func (c *client) initialize(ctx context.Context) error {
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID.Add(1),
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "ashron",
				"version": "dev",
			},
		},
	}
	if _, err := c.sendRequest(ctx, req); err != nil {
		return err
	}
	return c.sendNotification(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params":  map[string]any{},
	})
}

func (c *client) callTool(ctx context.Context, name string, arguments json.RawMessage) (string, error) {
	args := map[string]any{}
	if len(arguments) > 0 && string(arguments) != "null" {
		if err := json.Unmarshal(arguments, &args); err != nil {
			return "", fmt.Errorf("arguments must be a JSON object: %w", err)
		}
	}

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID.Add(1),
		Method:  "tools/call",
		Params: map[string]any{
			"name":      name,
			"arguments": args,
		},
	}
	resultRaw, err := c.sendRequest(ctx, req)
	if err != nil {
		return "", err
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError,omitempty"`
	}
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		return "", fmt.Errorf("parse tools/call result: %w", err)
	}
	var parts []string
	for _, item := range result.Content {
		if item.Type == "text" {
			parts = append(parts, item.Text)
		}
	}
	output := strings.Join(parts, "\n")
	if result.IsError {
		return output, fmt.Errorf("mcp tool returned error")
	}
	return output, nil
}

func (c *client) sendRequest(ctx context.Context, req rpcRequest) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := writeFramedJSON(c.writer, req); err != nil {
		return nil, err
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		payload, err := readFramedJSON(c.reader)
		if err != nil {
			return nil, err
		}
		var resp rpcResponse
		if err := json.Unmarshal(payload, &resp); err != nil {
			continue
		}
		if resp.ID != req.ID {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (c *client) sendNotification(notif map[string]any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return writeFramedJSON(c.writer, notif)
}

func writeFramedJSON(w io.Writer, msg any) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(payload))
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}

func readFramedJSON(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])
		if name == "content-length" {
			n, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid content-length: %w", err)
			}
			length = n
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("missing content-length")
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}
