package tui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestParseAtPathQuery(t *testing.T) {
	t.Parallel()

	q, ok := parseAtPathQuery("review @internal/t")
	if !ok {
		t.Fatalf("parseAtPathQuery should detect @ token")
	}
	if q.InsertPrefix != "review " {
		t.Fatalf("InsertPrefix = %q, want %q", q.InsertPrefix, "review ")
	}
	if q.PathPrefix != "internal/t" {
		t.Fatalf("PathPrefix = %q, want %q", q.PathPrefix, "internal/t")
	}
}

func TestParseAtPathQueryNoToken(t *testing.T) {
	t.Parallel()

	if _, ok := parseAtPathQuery("review internal/t"); ok {
		t.Fatalf("parseAtPathQuery should not detect without @ prefix")
	}
}

func TestAtPathCompletionItems(t *testing.T) {
	tmp := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prev)
	})
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("os.Chdir(%q) error = %v", tmp, err)
	}

	if err := os.WriteFile("README.md", []byte("x"), 0o600); err != nil {
		t.Fatalf("os.WriteFile README.md error = %v", err)
	}
	if err := os.WriteFile("space name.txt", []byte("x"), 0o600); err != nil {
		t.Fatalf("os.WriteFile space name.txt error = %v", err)
	}
	if err := os.WriteFile(".hidden", []byte("x"), 0o600); err != nil {
		t.Fatalf("os.WriteFile .hidden error = %v", err)
	}
	if err := os.Mkdir("src", 0o755); err != nil {
		t.Fatalf("os.Mkdir src error = %v", err)
	}
	if err := os.WriteFile(filepath.Join("src", "main.go"), []byte("package main"), 0o600); err != nil {
		t.Fatalf("os.WriteFile src/main.go error = %v", err)
	}

	items := atPathCompletionItems("")
	if !slices.Contains(items, "@README.md") {
		t.Fatalf("items should contain @README.md, got %#v", items)
	}
	if !slices.Contains(items, "@src/") {
		t.Fatalf("items should contain @src/, got %#v", items)
	}
	if slices.Contains(items, "@.hidden") {
		t.Fatalf("items should not contain hidden file without dot prefix, got %#v", items)
	}

	spaceItems := atPathCompletionItems("space")
	if !slices.Contains(spaceItems, "@space\\ name.txt") {
		t.Fatalf("spaceItems should contain escaped path, got %#v", spaceItems)
	}

	nestedItems := atPathCompletionItems("src/m")
	if !slices.Contains(nestedItems, "@src/main.go") {
		t.Fatalf("nestedItems should contain nested match, got %#v", nestedItems)
	}

	hiddenItems := atPathCompletionItems(".")
	if !slices.Contains(hiddenItems, "@.hidden") {
		t.Fatalf("hiddenItems should contain hidden file when dot-prefixed, got %#v", hiddenItems)
	}
}
