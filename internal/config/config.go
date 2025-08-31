package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	API      APIConfig     `mapstructure:"api"`
	Tools    ToolsConfig   `mapstructure:"tools"`
	Context  ContextConfig `mapstructure:"context"`
	Provider string        `mapstructure:"provider"`
}

type APIConfig struct {
	BaseURL     string        `mapstructure:"base_url"`
	APIKey      string        `mapstructure:"api_key"`
	Model       string        `mapstructure:"model"`
	MaxTokens   int           `mapstructure:"max_tokens"`
	Temperature float32       `mapstructure:"temperature"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

type ToolsConfig struct {
	AutoApproveTools []string      `mapstructure:"auto_approve_tools"`
	MaxOutputSize    int           `mapstructure:"max_output_size"`
	CommandTimeout   time.Duration `mapstructure:"command_timeout"`
}

type ContextConfig struct {
	MaxMessages     int     `mapstructure:"max_messages"`
	MaxTokens       int     `mapstructure:"max_tokens"`
	CompactionRatio float32 `mapstructure:"compaction_ratio"`
	AutoCompact     bool    `mapstructure:"auto_compact"`
}

func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Set config name and paths
	v.SetConfigName("ashron")
	v.SetConfigType("yaml")

	// Add config paths
	if configDir := os.Getenv("XDG_CONFIG_HOME"); configDir != "" {
		v.AddConfigPath(filepath.Join(configDir, "ashron"))
	}
	v.AddConfigPath(filepath.Join(os.Getenv("HOME"), ".config", "ashron"))

	// Read environment variables
	v.SetEnvPrefix("ASHRON")
	v.AutomaticEnv()

	// Override API key from env if present
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		v.Set("api.api_key", apiKey)
	}

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			// Config file not found, create default config
			if err := createDefaultConfig(); err != nil {
				return nil, fmt.Errorf("failed to create default config: %w", err)
			}
			return nil, fmt.Errorf("config file not found. Created default config at ~/.config/ashron/ashron.yaml. Please set your API key and run again")
		}
		return nil, fmt.Errorf("failed to read config file '%s': %w", v.ConfigFileUsed(), err)
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config from '%s': %w", v.ConfigFileUsed(), err)
	}

	return &config, nil
}

func setDefaults(v *viper.Viper) {
	// API defaults
	v.SetDefault("provider", "openai")
	v.SetDefault("api.base_url", "https://api.openai.com/v1")
	v.SetDefault("api.model", "gpt-4-turbo-preview")
	v.SetDefault("api.max_tokens", 4096)
	v.SetDefault("api.temperature", 0.7)
	v.SetDefault("api.timeout", 60) // 60 seconds default

	// Tools defaults
	v.SetDefault("tools.auto_approve_tools", []string{"read_file", "list_directory", "list_tools", "git_ls_files", "git_grep"})
	v.SetDefault("tools.max_output_size", 50000)
	v.SetDefault("tools.command_timeout", 10*time.Minute)

	// Context defaults
	v.SetDefault("context.max_messages", 50)
	v.SetDefault("context.max_tokens", 100000)
	v.SetDefault("context.compaction_ratio", 0.5)
	v.SetDefault("context.auto_compact", true)
}

func (c *Config) Validate() error {
	if c.API.APIKey == "" {
		return ErrMissingAPIKey
	}
	if c.API.Model == "" {
		return ErrMissingModel
	}
	return nil
}

var (
	ErrMissingAPIKey = &ConfigError{"API key is required"}
	ErrMissingModel  = &ConfigError{"Model name is required"}
)

type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}

func createDefaultConfig() error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "ashron")
	configFile := filepath.Join(configDir, "ashron.yaml")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Default configuration content
	defaultConfig := `# Ashron Configuration File

# Provider settings (openai, anthropic, custom)
provider: openai

# API Configuration
api:
  base_url: https://api.openai.com/v1
  # Set your API key here or use OPENAI_API_KEY environment variable
  # api_key: YOUR_API_KEY_HERE
  model: gpt-4-turbo-preview
  max_tokens: 4096
  temperature: 0.7
  timeout: 60  # API request timeout in seconds

# Tools Configuration
tools:
  # Commands that don't require approval
  auto_approve_tools:
    - read_file
    - list_directory
    - list_tools
    - git_ls_files
    - git_grep
  max_output_size: 50000  # Maximum bytes for command output
  command_timeout: 10m

# Context Management
context:
  max_messages: 50
  max_tokens: 100000
  compaction_ratio: 0.5  # Compact when context uses more than 50 percent of max tokens
  auto_compact: true
`

	// Write the default config file
	if err := os.WriteFile(configFile, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
