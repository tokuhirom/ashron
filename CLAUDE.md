# CLAUDE.md

## Project Overview

ashron is a TUI-based AI coding assistant built in Go with Bubble Tea. It uses OpenAI-compatible APIs.

## Build & Test

```bash
# Build
go build -o ashron ./cmd/ashron

# Run all unit tests (fast, no external deps)
go test ./... -short

# Run integration tests (requires local Ollama)
go test ./internal/api/ -run TestIntegration -v -timeout 10m

# Lint
golangci-lint run
```

## Integration Test Setup (Local LLM)

Integration tests use Ollama with a small model for CPU-only inference.

```bash
# Install & start Ollama (managed via mise)
mise install
mise exec -- ollama serve &
mise exec -- ollama pull qwen3:1.7b

# Run integration tests
go test ./internal/api/ -run TestIntegration -v -timeout 10m
```

- Default URL: `http://localhost:11434/v1`
- Default model: `qwen3:1.7b` (fast enough for CPU, ~20-80s per test)
- Override with env vars: `ASHRON_TEST_LLM_URL`, `ASHRON_TEST_LLM_MODEL`
- Tests skip automatically with `-short` or when Ollama is not reachable
- No NVIDIA GPU on this machine; Ollama runs on CPU only

## Code Conventions

- Use bubbletea for TUI; never use `fmt.Printf`/`Println`/`log.Print` directly
- Use `slog` for structured logging
- OpenAI-compatible API via `internal/api` package
- Tools defined in `internal/tools/tool_info.go`
- Read-only tool subset in `internal/tools/tool_selection.go`
- Config in YAML: `~/.config/ashron/ashron.yaml`

## Key Architecture

- `internal/api/` — API client, types, streaming
- `internal/tui/` — Bubble Tea TUI (simple_app.go, stream_simple.go)
- `internal/tools/` — Tool definitions and execution
- `internal/context/` — Context management with staged compaction
- `internal/config/` — YAML config loading
- `internal/subagent/` — Subagent spawning
- `internal/mcp/` — MCP client support
