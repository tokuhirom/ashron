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

Ashron looks for configuration files in:
1. `$XDG_CONFIG_HOME/ashron/ashron.yaml`
2. `~/.config/ashron/ashron.yaml`

### Example Configuration

```yaml
# Provider settings (openai, anthropic, custom)
provider: openai

# API Configuration
api:
  base_url: https://api.openai.com/v1
  model: gpt-4-turbo-preview
  max_tokens: 4096
  temperature: 0.7

# Tools Configuration
tools:
  auto_approve:
    - read_file
    - list_directory
  max_output_size: 50000
  command_timeout: 10m

# Context Management
context:
  max_messages: 50
  max_tokens: 100000
  compaction_ratio: 0.5
  auto_compact: true
```

## Commands

### In-App Commands

- `/help` - Show available commands
- `/clear` - Clear chat history
- `/compact` - Manually compact conversation context
- `/config` - Display current configuration
- `/exit` - Exit application

### Keyboard Shortcuts

- `Enter` - Send message  
- `Ctrl+J` - Insert new line in input
- `Ctrl+C` - Cancel current operation or exit
- `Ctrl+L` - Clear chat
- `Tab` - Approve pending tool calls
- `Esc` - Cancel pending tool calls
- `Enter` - New line in input

## Available Tools

### File Operations
- **read_file** - Read contents of a file
- **write_file** - Write content to a file
- **list_directory** - List files in a directory

### Command Execution
- **execute_command** - Execute shell commands with timeout protection

## Command Line Options

```bash
ashron [options]

Options:
  -config string    Path to configuration file
  -api-key string   OpenAI API key (overrides config)
  -model string     Model to use (overrides config)
  -base-url string  API base URL (overrides config)
  -version         Show version information
  -help            Show help message
```

## Environment Variables

- `OPENAI_API_KEY` - OpenAI API key

## Development

### Prerequisites

- Go 1.22 or higher
- golangci-lint (for linting)
- lefthook (for git hooks)

### Setup

```bash
# Clone the repository
git clone https://github.com/tokuhirom/ashron.git
cd ashron

# Install dependencies
go mod download

# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/evilmartians/lefthook@latest

# Setup git hooks
lefthook install

# Run tests
go test -v ./...

# Run linter
golangci-lint run

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

