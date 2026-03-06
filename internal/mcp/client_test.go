package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/tokuhirom/ashron/internal/config"
)

func TestFrameReadWrite(t *testing.T) {
	var buf bytes.Buffer
	msg := map[string]any{"jsonrpc": "2.0", "method": "ping"}
	if err := writeFramedJSON(&buf, msg); err != nil {
		t.Fatalf("writeFramedJSON: %v", err)
	}
	payload, err := readFramedJSON(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("readFramedJSON: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got["method"] != "ping" {
		t.Fatalf("unexpected payload: %#v", got)
	}
}

func TestCallToolWithHelperProcess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := config.MCPServerConfig{
		Command:        os.Args[0],
		Args:           []string{"-test.run=TestHelperMCPProcess", "--"},
		Env:            map[string]string{"GO_WANT_HELPER_MCP": "1"},
		StartupTimeout: time.Second,
		CallTimeout:    3 * time.Second,
	}
	out, err := CallTool(ctx, cfg, "echo", json.RawMessage(`{"message":"hello"}`))
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if !strings.Contains(out, "echo: hello") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestHelperMCPProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_MCP") != "1" {
		return
	}

	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		payload, err := readFramedJSON(reader)
		if err != nil {
			os.Exit(0)
		}
		var req map[string]any
		if err := json.Unmarshal(payload, &req); err != nil {
			continue
		}
		method, _ := req["method"].(string)
		id, hasID := req["id"]

		switch method {
		case "initialize":
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]any{
					"protocolVersion": "2025-06-18",
					"capabilities":    map[string]any{},
					"serverInfo": map[string]any{
						"name":    "helper",
						"version": "1.0.0",
					},
				},
			}
			_ = writeFramedJSON(writer, resp)
		case "tools/call":
			params, _ := req["params"].(map[string]any)
			args, _ := params["arguments"].(map[string]any)
			msg, _ := args["message"].(string)
			if hasID {
				resp := map[string]any{
					"jsonrpc": "2.0",
					"id":      id,
					"result": map[string]any{
						"content": []map[string]any{
							{"type": "text", "text": fmt.Sprintf("echo: %s", msg)},
						},
					},
				}
				_ = writeFramedJSON(writer, resp)
			}
		case "notifications/initialized":
			// no-op
		default:
			if hasID {
				resp := map[string]any{
					"jsonrpc": "2.0",
					"id":      id,
					"error": map[string]any{
						"code":    -32601,
						"message": "method not found",
					},
				}
				_ = writeFramedJSON(writer, resp)
			}
		}
	}
}

func TestCallToolMissingServerCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := CallTool(ctx, config.MCPServerConfig{}, "echo", json.RawMessage(`{}`))
	if err == nil {
		t.Fatalf("expected error for empty command")
	}
}

var _ = exec.ErrNotFound
