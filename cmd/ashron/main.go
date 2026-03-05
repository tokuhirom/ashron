package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/alecthomas/kong"
	"github.com/tokuhirom/ashron/internal/config"
	"github.com/tokuhirom/ashron/internal/logger"
	"github.com/tokuhirom/ashron/internal/session"
	"github.com/tokuhirom/ashron/internal/tui"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

var cli struct {
	APIKey  string `help:"OpenAI API key (overrides config)" env:"OPENAI_API_KEY" name:"api-key"`
	Model   string `help:"Model to use (overrides config)"`
	BaseURL string `help:"API base URL (overrides config)" name:"base-url"`
	Log     string `help:"Path to log file for debugging"`
	Yolo    bool   `help:"Disable sandbox and require no tool approvals (dangerous)"`
	Resume  string `help:"Resume a previous session by ID" name:"resume" optional:""`

	Version kong.VersionFlag `help:"Show version and exit"`
}

func main() {
	ctx := kong.Parse(&cli,
		kong.Name("ashron"),
		kong.Description("AI Coding Assistant\n\nAn interactive AI-powered coding assistant that helps with software engineering tasks."),
		kong.Vars{"version": fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date)},
	)
	_ = ctx

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override with command-line flags
	provName := cfg.Default.Provider
	if cli.Model != "" {
		foundProv, _, _, err := cfg.FindModel(cli.Model)
		if err != nil {
			log.Fatalf("Model not found: %v", err)
		}
		cfg.Default.Provider = foundProv
		cfg.Default.Model = cli.Model
		provName = foundProv
	}
	if cli.APIKey != "" {
		if prov, ok := cfg.Providers[provName]; ok {
			prov.APIKey = cli.APIKey
			cfg.Providers[provName] = prov
		}
	}
	if cli.BaseURL != "" {
		if prov, ok := cfg.Providers[provName]; ok {
			prov.BaseURL = cli.BaseURL
			cfg.Providers[provName] = prov
		}
	}
	if cli.Yolo {
		cfg.Tools.Yolo = true
		cfg.Tools.SandboxMode = "off"
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Setup logging
	if err := logger.Setup(cli.Log); err != nil {
		log.Fatalf("Failed to setup logging: %v\n", err)
	}
	defer logger.Close()

	slog.Info("Starting Ashron", "version", version, "commit", commit)
	tui.SetBuildInfo(version, commit, date)

	// Load session for resume if requested
	var sess *session.Session
	if cli.Resume != "" {
		var loadErr error
		sess, loadErr = session.Load(cli.Resume)
		if loadErr != nil {
			log.Fatalf("Failed to load session: %v", loadErr)
		}
	} else {
		summaries, listErr := session.ListSummaries(30)
		if listErr != nil {
			slog.Warn("Failed to list sessions", "error", listErr)
		} else if len(summaries) > 0 {
			pick, pickErr := tui.SelectSessionInteractive(summaries)
			if pickErr != nil {
				slog.Warn("Session picker failed", "error", pickErr)
			} else if pick.Cancelled {
				os.Exit(0)
			} else if pick.SessionID != "" {
				var loadErr error
				sess, loadErr = session.Load(pick.SessionID)
				if loadErr != nil {
					log.Fatalf("Failed to load selected session: %v", loadErr)
				}
			}
		}
	}

	// Create the simple TUI model (streaming mode)
	tuiModel, err := tui.NewSimpleModel(cfg, sess)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}

	p := tea.NewProgram(tuiModel)

	// Run the program
	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error running application: %v\n", err)
		os.Exit(1)
	}

	// Print resume hint after exit
	if sm, ok := finalModel.(*tui.SimpleModel); ok {
		if id := sm.SessionID(); id != "" {
			fmt.Printf("\nTo resume this session: ashron --resume %s\n", id)
		}
	}
}

func loadConfig() (*config.Config, error) {
	return config.Load()
}
