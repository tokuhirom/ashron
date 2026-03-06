package tools

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetDiagnosticsCleanFile(t *testing.T) {
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("gopls not found in PATH, skipping LSP integration test")
	}

	args, _ := json.Marshal(GetDiagnosticsArgs{Path: "lsp.go"})
	result := GetDiagnostics(nil, "test-1", string(args))

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	t.Logf("output:\n%s", result.Output)

	if result.Output == "" {
		t.Fatal("expected non-empty output")
	}
	// Clean file should have no errors
	if strings.Contains(result.Output, "[ERROR]") {
		t.Fatalf("expected no errors, got:\n%s", result.Output)
	}
}

func TestGetDiagnosticsErrorFile(t *testing.T) {
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("gopls not found in PATH, skipping LSP integration test")
	}

	// Write a broken Go file inside a temp module so gopls can analyze it.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testlsp\n\ngo 1.21\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	brokenPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(brokenPath, []byte("package main\n\nfunc main() {\n\t_ = undefined_variable\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	args, _ := json.Marshal(GetDiagnosticsArgs{Path: brokenPath})
	result := GetDiagnostics(nil, "test-err", string(args))

	t.Logf("output:\n%s", result.Output)

	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.Contains(result.Output, "[ERROR]") {
		t.Fatalf("expected ERROR diagnostic, got:\n%s", result.Output)
	}
}

func TestGetDiagnosticsUnknownExtension(t *testing.T) {
	args, _ := json.Marshal(GetDiagnosticsArgs{Path: "somefile.xyz"})
	result := GetDiagnostics(nil, "test-2", string(args))

	if result.Error == nil {
		t.Fatal("expected error for unknown extension")
	}
}

func TestGetDiagnosticsNotFound(t *testing.T) {
	args, _ := json.Marshal(GetDiagnosticsArgs{Path: "/nonexistent/file.go"})
	result := GetDiagnostics(nil, "test-3", string(args))

	if result.Error == nil {
		t.Fatal("expected error for missing file")
	}
}
