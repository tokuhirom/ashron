package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tokuhirom/ashron/internal/config"
)

func TestMCPCallUnknownServer(t *testing.T) {
	args := `{"server":"missing","tool":"echo","arguments":{"message":"hi"}}`
	res := MCPCall(&config.ToolsConfig{}, "tc1", args)
	if res.Error == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(res.Output, "unknown mcp server") {
		t.Fatalf("unexpected output: %s", res.Output)
	}
}

func TestMCPCallSuccess(t *testing.T) {
	toolsCfg := &config.ToolsConfig{
		MCPServers: map[string]config.MCPServerConfig{
			"helper": {
				Command:        os.Args[0],
				Args:           []string{"-test.run=TestHelperMCPServerProcess", "--"},
				Env:            map[string]string{"GO_WANT_HELPER_MCP_SERVER": "1"},
				StartupTimeout: time.Second,
				CallTimeout:    3 * time.Second,
			},
		},
	}
	args := `{"server":"helper","tool":"echo","arguments":{"message":"hello"}}`
	res := MCPCall(toolsCfg, "tc2", args)
	if res.Error != nil {
		t.Fatalf("unexpected error: %v output=%s", res.Error, res.Output)
	}
	if !strings.Contains(res.Output, "echo: hello") {
		t.Fatalf("unexpected output: %s", res.Output)
	}
}

func TestHelperMCPServerProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_MCP_SERVER") != "1" {
		return
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		payload, err := readFrame(reader)
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
			_ = writeFrame(os.Stdout, map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]any{
					"protocolVersion": "2025-06-18",
					"capabilities":    map[string]any{},
					"serverInfo":      map[string]any{"name": "helper", "version": "1.0.0"},
				},
			})
		case "tools/call":
			params, _ := req["params"].(map[string]any)
			callArgs, _ := params["arguments"].(map[string]any)
			msg, _ := callArgs["message"].(string)
			if hasID {
				_ = writeFrame(os.Stdout, map[string]any{
					"jsonrpc": "2.0",
					"id":      id,
					"result": map[string]any{
						"content": []map[string]any{{"type": "text", "text": fmt.Sprintf("echo: %s", msg)}},
					},
				})
			}
		case "notifications/initialized":
		default:
			if hasID {
				_ = writeFrame(os.Stdout, map[string]any{
					"jsonrpc": "2.0",
					"id":      id,
					"error":   map[string]any{"code": -32601, "message": "method not found"},
				})
			}
		}
	}
}

func writeFrame(w io.Writer, msg any) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}

func readFrame(r *bufio.Reader) ([]byte, error) {
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
		if strings.EqualFold(strings.TrimSpace(parts[0]), "content-length") {
			n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, err
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

var _ = exec.ErrNotFound
