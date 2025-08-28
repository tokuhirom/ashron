package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tokuhirom/ashron/internal/config"
	"github.com/tokuhirom/ashron/internal/logger"
	"github.com/tokuhirom/ashron/internal/tui"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	var (
		configFile  = flag.String("config", "", "Path to configuration file")
		showVersion = flag.Bool("version", false, "Show version information")
		apiKey      = flag.String("api-key", "", "OpenAI API key (overrides config)")
		model       = flag.String("model", "", "Model to use (overrides config)")
		baseURL     = flag.String("base-url", "", "API base URL (overrides config)")
		logFile     = flag.String("log", "", "Path to log file for debugging")
		help        = flag.Bool("help", false, "Show help")
	)

	flag.Parse()

	if *help {
		printHelp()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("Ashron %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	// Setup logging
	if err := logger.Setup(*logFile); err != nil {
		log.Fatalf("Failed to setup logging: %v", err)
	}
	defer logger.Close()

	slog.Info("Starting Ashron", "version", version, "commit", commit)

	// Load configuration
	cfg, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override with command-line flags
	if *apiKey != "" {
		cfg.API.APIKey = *apiKey
	}
	if *model != "" {
		cfg.API.Model = *model
	}
	if *baseURL != "" {
		cfg.API.BaseURL = *baseURL
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create the simple TUI model (streaming mode)
	tuiModel, err := tui.NewSimpleModel(cfg)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}

	// Create the program without alt screen for streaming mode
	p := tea.NewProgram(tuiModel)

	// Run the program
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running application: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig(configFile string) (*config.Config, error) {
	return config.Load()
}

func printHelp() {
	fmt.Println(`Ashron - AI Coding Assistant

Usage:
  ashron [options]

Options:
  -config string
        Path to configuration file
  -api-key string
        OpenAI API key (overrides config)
  -model string
        Model to use (overrides config)
  -base-url string
        API base URL (overrides config)
  -log string
        Path to log file for debugging
  -version
        Show version information
  -help
        Show this help message

Environment Variables:
  OPENAI_API_KEY        OpenAI API key
  ASHRON_CONFIG_FILE    Path to configuration file
  ASHRON_API_BASE_URL   API base URL
  ASHRON_API_MODEL      Model to use

Commands (in application):
  /help                 Show available commands
  /clear                Clear chat history
  /compact              Compact conversation context
  /config               Show current configuration
  /exit                 Exit application

Keyboard Shortcuts:
  Ctrl+J               Send message
  Alt+Enter            Send message (alternative)
  Ctrl+C               Cancel operation or exit
  Ctrl+L               Clear chat
  Tab                  Approve pending tool calls
  Esc                  Cancel pending tool calls
  Enter                New line in input

Configuration:
  Ashron looks for configuration in the following locations:
  1. $XDG_CONFIG_HOME/ashron/ashron.yaml
  2. ~/.config/ashron/ashron.yaml
  
  If no config file exists, Ashron will create a default one at ~/.config/ashron/ashron.yaml

For more information, visit: https://github.com/tokuhirom/ashron`)
}
