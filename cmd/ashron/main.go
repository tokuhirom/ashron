package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/alecthomas/kingpin/v2"
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
	app := kingpin.New("ashron", "AI Coding Assistant\n\nAn interactive AI-powered coding assistant that helps with software engineering tasks.")
	app.Version(fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date))
	app.HelpFlag.Short('h')
	app.Author("tokuhirom")

	apiKey := app.Flag("api-key", "OpenAI API key (overrides config)").Envar("OPENAI_API_KEY").String()
	model := app.Flag("model", "Model to use (overrides config)").String()
	baseURL := app.Flag("base-url", "API base URL (overrides config)").String()
	logFile := app.Flag("log", "Path to log file for debugging").String()

	kingpin.MustParse(app.Parse(os.Args[1:]))

	// Load configuration
	cfg, err := loadConfig()
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

	// Setup logging
	if err := logger.Setup(*logFile); err != nil {
		log.Fatalf("Failed to setup logging: %v", err)
	}
	defer logger.Close()

	slog.Info("Starting Ashron", "version", version, "commit", commit)

	// Create the simple TUI model (streaming mode)
	tuiModel, err := tui.NewSimpleModel(cfg)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}

	// Create the program without alt screen for streaming mode
	p := tea.NewProgram(tuiModel,
		tea.WithAltScreen())

	// Run the program
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running application: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig() (*config.Config, error) {
	return config.Load()
}
