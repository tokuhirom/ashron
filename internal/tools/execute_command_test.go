package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/tokuhirom/ashron/internal/config"
)

func TestBuildShellCommandSandboxOff(t *testing.T) {
	t.Parallel()

	cmd, backend, err := buildShellCommand(
		context.Background(),
		&config.ToolsConfig{SandboxMode: "off"},
		ExecuteCommandArgs{},
		"echo hello",
		"",
	)
	if err != nil {
		t.Fatalf("buildShellCommand() returned error: %v", err)
	}

	if backend != "none" {
		t.Fatalf("expected backend 'none', got %q", backend)
	}
	if !strings.HasSuffix(cmd.Path, "/sh") && cmd.Path != "sh" {
		t.Fatalf("expected command path ending with '/sh', got %q", cmd.Path)
	}
}

func TestEffectiveSandboxMode(t *testing.T) {
	t.Parallel()

	cfg := &config.ToolsConfig{SandboxMode: "auto"}

	if got := EffectiveSandboxMode(cfg, ExecuteCommandArgs{}); got != "auto" {
		t.Fatalf("expected auto, got %q", got)
	}
	if got := EffectiveSandboxMode(cfg, ExecuteCommandArgs{SandboxMode: "off"}); got != "off" {
		t.Fatalf("expected off, got %q", got)
	}
	if got := EffectiveSandboxMode(cfg, ExecuteCommandArgs{SandboxMode: "AUTO"}); got != "auto" {
		t.Fatalf("expected auto, got %q", got)
	}
	if got := EffectiveSandboxMode(cfg, ExecuteCommandArgs{SandboxMode: "invalid"}); got != "auto" {
		t.Fatalf("expected fallback to auto, got %q", got)
	}

	yoloCfg := &config.ToolsConfig{SandboxMode: "auto", Yolo: true}
	if got := EffectiveSandboxMode(yoloCfg, ExecuteCommandArgs{SandboxMode: "auto"}); got != "off" {
		t.Fatalf("expected yolo to force off, got %q", got)
	}
}

func TestBuildDarwinSandboxProfile(t *testing.T) {
	t.Parallel()

	profile := buildDarwinSandboxProfile(`/tmp/ab"c\def`)

	if !strings.Contains(profile, `(deny default)`) {
		t.Fatalf("sandbox profile should deny default")
	}
	if !strings.Contains(profile, `(subpath "/tmp/ab\"c\\def")`) {
		t.Fatalf("working directory path should be escaped in profile: %s", profile)
	}
}
