package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tokuhirom/ashron/internal/api"
	"github.com/tokuhirom/ashron/internal/config"
)

// GetDiagnosticsArgs holds arguments for the get_diagnostics tool.
type GetDiagnosticsArgs struct {
	Path string `json:"path"`
}

// GetDiagnostics starts the appropriate language server for the given file,
// opens the document, waits for publishDiagnostics, and returns formatted output.
func GetDiagnostics(_ *config.ToolsConfig, toolCallID string, argsJSON string) api.ToolResult {
	result := api.ToolResult{ToolCallID: toolCallID}

	var args GetDiagnosticsArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		result.Error = err
		result.Output = "Error: " + err.Error()
		return result
	}

	path, err := filepath.Abs(filepath.Clean(args.Path))
	if err != nil {
		result.Error = err
		result.Output = "Error resolving path: " + err.Error()
		return result
	}

	if _, err := os.Stat(path); err != nil {
		result.Error = err
		result.Output = "Error: file not found: " + path
		return result
	}

	lsCmd, lsArgs, err := detectLanguageServer(path)
	if err != nil {
		result.Error = err
		result.Output = err.Error()
		return result
	}

	slog.Info("Starting language server", "cmd", lsCmd, "args", lsArgs, "file", path)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := newLSPClient(ctx, lsCmd, lsArgs...)
	if err != nil {
		result.Error = err
		result.Output = fmt.Sprintf("Failed to start %q: %v", lsCmd, err)
		return result
	}
	defer client.shutdown()

	rootURI := "file://" + findProjectRoot(path)
	fileURI := "file://" + path

	// Initialize handshake
	if _, err := client.request(ctx, "initialize", map[string]any{
		"processId": os.Getpid(),
		"rootUri":   rootURI,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"publishDiagnostics": map[string]any{
					"relatedInformation": false,
				},
			},
		},
		"initializationOptions": map[string]any{},
	}); err != nil {
		result.Error = err
		result.Output = "LSP initialize failed: " + err.Error()
		return result
	}
	_ = client.notify("initialized", map[string]any{})

	content, err := os.ReadFile(path)
	if err != nil {
		result.Error = err
		result.Output = "Error reading file: " + err.Error()
		return result
	}

	langID := extensionToLanguageID(strings.ToLower(filepath.Ext(path)))
	_ = client.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri":        fileURI,
			"languageId": langID,
			"version":    1,
			"text":       string(content),
		},
	})

	// Wait for publishDiagnostics for our file (10s timeout).
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()

	for {
		select {
		case uri := <-client.diagCh:
			if uri == fileURI {
				client.mu.Lock()
				diags := client.diagnostics[fileURI]
				client.mu.Unlock()
				result.Output = formatDiagnostics(path, diags)
				return result
			}
		case <-deadline.C:
			// Timeout — return whatever we have (may be empty).
			client.mu.Lock()
			diags := client.diagnostics[fileURI]
			client.mu.Unlock()
			if len(diags) == 0 {
				result.Output = fmt.Sprintf(
					"No diagnostics received within timeout for %s.\nThe language server may still be initializing or the file has no issues.",
					path,
				)
			} else {
				result.Output = formatDiagnostics(path, diags)
			}
			return result
		case <-ctx.Done():
			result.Output = "Timed out waiting for diagnostics"
			return result
		}
	}
}

// --- LSP client ---------------------------------------------------------------

type lspDiagnostic struct {
	Range    lspRange `json:"range"`
	Severity int      `json:"severity"` // 1=Error 2=Warning 3=Info 4=Hint
	Message  string   `json:"message"`
	Source   string   `json:"source,omitempty"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
}

type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type publishDiagnosticsParams struct {
	URI         string          `json:"uri"`
	Diagnostics []lspDiagnostic `json:"diagnostics"`
}

type lspMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type lspClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader

	mu          sync.Mutex
	pending     map[int64]chan *lspMsg
	diagnostics map[string][]lspDiagnostic
	diagCh      chan string

	nextID atomic.Int64
}

func newLSPClient(ctx context.Context, command string, args ...string) (*lspClient, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = nil // discard stderr noise

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", command, err)
	}

	c := &lspClient{
		cmd:         cmd,
		stdin:       stdin,
		stdout:      bufio.NewReader(stdout),
		pending:     make(map[int64]chan *lspMsg),
		diagnostics: make(map[string][]lspDiagnostic),
		diagCh:      make(chan string, 32),
	}
	go c.readLoop()
	return c, nil
}

func (c *lspClient) readLoop() {
	for {
		msg, err := c.readMessage()
		if err != nil {
			return
		}
		c.dispatch(msg)
	}
}

func (c *lspClient) readMessage() (*lspMsg, error) {
	contentLength := -1
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length: ") {
			n, err := strconv.Atoi(strings.TrimPrefix(line, "Content-Length: "))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %w", err)
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.stdout, body); err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var msg lspMsg
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &msg, nil
}

func (c *lspClient) dispatch(msg *lspMsg) {
	// Server-initiated notification
	if msg.Method != "" && msg.ID == nil {
		if msg.Method == "textDocument/publishDiagnostics" {
			var params publishDiagnosticsParams
			if err := json.Unmarshal(msg.Params, &params); err == nil {
				c.mu.Lock()
				c.diagnostics[params.URI] = params.Diagnostics
				c.mu.Unlock()
				select {
				case c.diagCh <- params.URI:
				default:
				}
			}
		}
		return
	}

	// Response to our request
	if msg.ID != nil {
		c.mu.Lock()
		ch, ok := c.pending[*msg.ID]
		if ok {
			delete(c.pending, *msg.ID)
		}
		c.mu.Unlock()
		if ok {
			ch <- msg
		}
	}
}

func (c *lspClient) send(v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := io.WriteString(c.stdin, header); err != nil {
		return err
	}
	_, err = c.stdin.Write(body)
	return err
}

func (c *lspClient) request(ctx context.Context, method string, params any) (*lspMsg, error) {
	id := c.nextID.Add(1)
	ch := make(chan *lspMsg, 1)

	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	if err := c.send(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-ch:
		return msg, nil
	}
}

func (c *lspClient) notify(method string, params any) error {
	return c.send(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
}

func (c *lspClient) shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _ = c.request(ctx, "shutdown", nil)
	_ = c.notify("exit", nil)
	_ = c.stdin.Close()
	_ = c.cmd.Wait()
}

// --- Helpers ------------------------------------------------------------------

// detectLanguageServer returns the command and args for the language server
// appropriate for the given file, or an error if none is available.
func detectLanguageServer(path string) (string, []string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		if _, err := exec.LookPath("gopls"); err != nil {
			return "", nil, fmt.Errorf("gopls not found in PATH.\nInstall: go install golang.org/x/tools/gopls@latest")
		}
		return "gopls", nil, nil
	case ".py":
		if _, err := exec.LookPath("pyright-langserver"); err == nil {
			return "pyright-langserver", []string{"--stdio"}, nil
		}
		if _, err := exec.LookPath("pylsp"); err == nil {
			return "pylsp", nil, nil
		}
		return "", nil, fmt.Errorf("no Python language server found.\nInstall pyright: npm install -g pyright")
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		if _, err := exec.LookPath("typescript-language-server"); err != nil {
			return "", nil, fmt.Errorf("typescript-language-server not found.\nInstall: npm install -g typescript-language-server typescript")
		}
		return "typescript-language-server", []string{"--stdio"}, nil
	case ".rs":
		if _, err := exec.LookPath("rust-analyzer"); err != nil {
			return "", nil, fmt.Errorf("rust-analyzer not found.\nSee: https://rust-analyzer.github.io/")
		}
		return "rust-analyzer", nil, nil
	case ".c", ".cpp", ".cc", ".cxx", ".h", ".hpp":
		if _, err := exec.LookPath("clangd"); err != nil {
			return "", nil, fmt.Errorf("clangd not found.\nInstall clang tools for your OS")
		}
		return "clangd", nil, nil
	case ".rb":
		if _, err := exec.LookPath("solargraph"); err != nil {
			return "", nil, fmt.Errorf("solargraph not found.\nInstall: gem install solargraph")
		}
		return "solargraph", []string{"stdio"}, nil
	default:
		return "", nil, fmt.Errorf("no language server configured for %q files", ext)
	}
}

// extensionToLanguageID maps file extensions to LSP language identifiers.
func extensionToLanguageID(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".rs":
		return "rust"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".h", ".hpp":
		return "cpp"
	case ".rb":
		return "ruby"
	default:
		return "plaintext"
	}
}

// findProjectRoot walks up from the file's directory looking for a project root
// marker (go.mod, package.json, Cargo.toml, pyproject.toml, .git).
// Falls back to the file's directory.
func findProjectRoot(filePath string) string {
	dir := filepath.Dir(filePath)
	markers := []string{"go.mod", "package.json", "Cargo.toml", "pyproject.toml", "setup.py", ".git"}
	for {
		for _, m := range markers {
			if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Dir(filePath)
}

// formatDiagnostics formats LSP diagnostics for display.
func formatDiagnostics(path string, diags []lspDiagnostic) string {
	if len(diags) == 0 {
		return fmt.Sprintf("No diagnostics: %s looks clean.", path)
	}

	severity := map[int]string{1: "ERROR", 2: "WARNING", 3: "INFO", 4: "HINT"}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d diagnostic(s) in %s:\n\n", len(diags), path)
	for _, d := range diags {
		sev := severity[d.Severity]
		if sev == "" {
			sev = "UNKNOWN"
		}
		line := d.Range.Start.Line + 1
		col := d.Range.Start.Character + 1
		if d.Source != "" {
			fmt.Fprintf(&sb, "[%s] %d:%d (%s): %s\n", sev, line, col, d.Source, d.Message)
		} else {
			fmt.Fprintf(&sb, "[%s] %d:%d: %s\n", sev, line, col, d.Message)
		}
	}
	return sb.String()
}
