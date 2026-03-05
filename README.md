# Ashron - AI Coding Assistant

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Ashron is a TUI-based AI coding assistant for developers. It provides an interactive terminal interface for AI-assisted programming with OpenAI APIs.

## Features

- **Beautiful TUI Interface** - Built with Bubble Tea framework for a smooth terminal experience
- **Streaming Chat** - Real-time streaming responses for natural conversation flow
- **Tool System** - Execute commands, read/write files directly from the chat
- **Context Management** - Smart context compaction with `/compact` command
- **Flexible Configuration** - YAML-based configuration with environment variable support
- **Safety First** - Tool approval system with configurable auto-approval
- **AGENTS.md** - [AGENTS.md](https://agents.md) supported.

## Installation

### From Source

```bash
go install github.com/tokuhirom/ashron/cmd/ashron@latest
```

### Using Go

```bash
git clone https://github.com/tokuhirom/ashron.git
cd ashron
go build -o ashron ./cmd/ashron
```

### Pre-built Binaries

(TODO: not released yet)
Download from [Releases](https://github.com/tokuhirom/ashron/releases)

## Quick Start

1. Set your OpenAI API key:
```bash
export OPENAI_API_KEY=your_api_key_here
```

2. Run Ashron:
```bash
ashron
```

3. Start chatting! Try:
```
> Help me write a function to calculate fibonacci numbers
> Read the contents of main.go
> Execute ls -la
```

## Configuration

Ashron loads configuration from the platform-standard config directory:

- **Linux**: `$XDG_CONFIG_HOME/ashron/ashron.yaml` (defaults to `~/.config/ashron/ashron.yaml`)
- **macOS**: `~/Library/Application Support/ashron/ashron.yaml`

If no config file is found, a default one is created automatically.

### Example Configuration

```yaml
# Active provider and model
default:
  provider: openai
  model: gpt4

# Provider definitions
providers:
  openai:
    type: openai-compat
    base_url: https://api.openai.com/v1
    # api_key: YOUR_API_KEY_HERE  (or use OPENAI_API_KEY env var)
    timeout: 5m
    models:
      gpt4:
        model: gpt-4-turbo-preview
        temperature: 0.7
      gpt-4.1:
        model: gpt-4.1
        temperature: 0.7

# Tools Configuration
tools:
  auto_approve_tools:
    - read_file
    - list_directory
    - list_tools
    - git_ls_files
    - git_grep
  auto_approve_commands:
    - /^git add .*$/
  max_output_size: 50000
  command_timeout: 10m
  sandbox_mode: auto # auto|off

# Context Management
context:
  max_messages: 50
  max_tokens: 65535
  compaction_ratio: 0.5
  auto_compact: true
```

## Commands

### In-App Commands

- `/help` - Show available commands
- `/clear` - Clear screen
- `/compact` - Manually compact conversation context
- `/config` - Display current configuration
- `/skills` - List locally available skills (`$XDG_CONFIG_HOME/ashron/skills`, `~/.config/ashron/skills`)
- `/model [name]` - Show available models or switch to a different model
- `/commit` - Generate and commit a git commit message
- `/init` - Generate AGENTS.md for the current project
- `/quit`, `/exit` - Exit application

### Keyboard Shortcuts

- `Enter` - Send message
- `Ctrl+J` - Insert new line in input
- `Ctrl+C` - Cancel current operation or exit
- `Ctrl+P` / `Ctrl+N` - Scroll up / down
- `y` / `n` - Approve / cancel pending tool calls
- `Tab` / `Up` / `Down` - Navigate command completion
- `Esc` - Close command completion

## Available Tools

### File Operations
- **read_file** - Read contents of a file
- **write_file** - Write content to a file
- **list_directory** - List files in a directory

### Command Execution
- **execute_command** - Execute shell commands with timeout protection and OS sandboxing (`sandbox-exec` on macOS, `bwrap` on Linux)

### Subagent
- **spawn_subagent** - Start a background subagent with an initial prompt
- **send_subagent_input** - Send additional input to an existing subagent
- **wait_subagent** - Wait for subagent completion and retrieve current result
- **list_subagents** - List subagents and their status
- **close_subagent** - Close a subagent and release resources

## Skills

Ashron can discover local skills from:

- `$XDG_CONFIG_HOME/ashron/skills`
- `~/.config/ashron/skills`

Each skill must be placed in its own directory and include `SKILL.md` with YAML frontmatter:

```md
---
name: my-skill
description: Short plain-text description of when this skill should be used.
---
```

`name` must be lowercase kebab-case (`[a-z0-9-]`, max 64 chars). Invalid skills are ignored.

At startup, discovered skill `name`/`description` metadata is injected into the system prompt.

When running `/init`, generated `AGENTS.md` now includes a `Skills` section listing discovered skills and their `SKILL.md` paths.

## Sandboxing

`execute_command` uses an OS-specific sandbox in `tools.sandbox_mode: auto`.

### Quick Mode Guide

| Mode | Sandbox | Tool approval | Intended use |
| --- | --- | --- | --- |
| Default (`sandbox_mode: auto`) | ON | Required (or auto-approve rules only) | Safe day-to-day use |
| Global Off (`sandbox_mode: off`) | OFF | Required | Debugging sandbox-related failures |
| Per-command Off (`execute_command.sandbox_mode: off`) | OFF (that command only) | Always required | One-shot unsandboxed command |
| YOLO (`--yolo`) | OFF | Not required | Fully trusted local use only |

### Priority Rules

1. `--yolo` has highest priority:
   - sandbox is always disabled
   - all tools are auto-approved
2. If YOLO is off, command-level `execute_command.sandbox_mode` is used when provided.
3. If command-level mode is omitted, global `tools.sandbox_mode` is used.

- `macOS`:
  - Backend: `sandbox-exec`
  - Network: shared with host (network access is allowed)
  - Filesystem:
    - Read: allowed
    - Write: limited to `working_dir`, `/tmp`, `/private/tmp`, `/var/tmp`
- `Linux`:
  - Backend: `bwrap` (bubblewrap)
  - Network: shared with host (no network namespace isolation)
  - Filesystem:
    - `/` is mounted read-only inside sandbox
    - `working_dir` is bind-mounted writable
    - `/tmp` and `/var/tmp` are isolated tmpfs

Behavior and configuration:

- `tools.sandbox_mode: auto` (default): use OS sandbox (`sandbox-exec` on macOS, `bwrap` on Linux)
- `tools.sandbox_mode: off`: run commands without sandbox
- Per-command override: `execute_command` accepts `sandbox_mode` (`auto` or `off`)
- If required backend command is missing in `auto` mode, command execution fails with an explicit error.
- Commands with `sandbox_mode: off` are never auto-approved and always require explicit approval.
- `--yolo`: disables sandbox and auto-approves all tools for that run (dangerous)

Examples:

```yaml
# Recommended default
tools:
  sandbox_mode: auto
```

```yaml
# Disable sandbox globally, but still keep manual approval
tools:
  sandbox_mode: off
```

```bash
# YOLO for this run only
ashron --yolo
```

Prerequisites:

- macOS: `sandbox-exec` available in `PATH`
- Linux: `bwrap` available in `PATH`

## Command Line Options

```bash
ashron [options]

Options:
  --api-key string   OpenAI API key (overrides config) [$OPENAI_API_KEY]
  --model string     Model to use (overrides config)
  --base-url string  API base URL (overrides config)
  --log string       Path to log file for debugging
  --yolo             Disable sandbox and require no tool approvals (dangerous)
  --version          Show version information
  --help             Show help message
```

## Environment Variables

- `OPENAI_API_KEY` - OpenAI API key (also configurable via `--api-key`)
- `XDG_CONFIG_HOME` - Override the base config directory (Linux)

## Development

### Prerequisites

- Go 1.24 or higher
- [mise](https://mise.jdx.dev/) (recommended, for pinned tool versions)
- lefthook (for git hooks)

### Setup

```bash
# Clone the repository
git clone https://github.com/tokuhirom/ashron.git
cd ashron

# Install pinned tool versions (Go, golangci-lint)
mise install

# Install dependencies
go mod download

# Setup git hooks
lefthook install

# Run tests
go test -v ./...

# Run linter
mise exec -- golangci-lint run

# Build
go build -o ashron ./cmd/ashron
```

### Project Structure

```
ashron/
├── cmd/ashron/          # Application entry point
├── internal/
│   ├── api/            # OpenAI API client
│   ├── config/         # Configuration management
│   ├── context/        # Context management & compaction
│   ├── tools/          # Tool execution system
│   └── tui/            # Terminal UI components
├── configs/            # Default configuration
└── .github/workflows/  # CI/CD configuration
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - The amazing TUI framework
- [Claude Code](https://claude.ai/code) - Inspiration for the interface and workflow
- [OpenAI](https://openai.com) - For the powerful API

## Roadmap

- [ ] Multiple conversation sessions
- [ ] Conversation history persistence
- [ ] MCP support
- [ ] Theme customization
- [ ] Image/file upload support

## Support

- Issues: [GitHub Issues](https://github.com/tokuhirom/ashron/issues)
- Discussions: [GitHub Discussions](https://github.com/tokuhirom/ashron/discussions)

## See also

- [600行から始める自作Coding Agent](https://zenn.dev/reiwatravel/articles/f4223888af33be)
