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
	Default   DefaultConfig             `mapstructure:"default"`
	Providers map[string]ProviderConfig `mapstructure:"providers"`
	Tools     ToolsConfig               `mapstructure:"tools"`
	Context   ContextConfig             `mapstructure:"context"`
}

type DefaultConfig struct {
	Provider string `mapstructure:"provider"`
	Model    string `mapstructure:"model"`
}

type ProviderConfig struct {
	Type    string                 `mapstructure:"type"`
	BaseURL string                 `mapstructure:"base_url"`
	APIKey  string                 `mapstructure:"api_key"`
	Timeout time.Duration          `mapstructure:"timeout"`
	Models  map[string]ModelConfig `mapstructure:"models"`
}

type ModelConfig struct {
	Model       string  `mapstructure:"model"`
	Temperature float32 `mapstructure:"temperature"`
}

type ToolsConfig struct {
	AutoApproveTools    []string      `mapstructure:"auto_approve_tools"`
	AutoApproveCommands []string      `mapstructure:"auto_approve_commands"`
	MaxOutputSize       int           `mapstructure:"max_output_size"`
	CommandTimeout      time.Duration `mapstructure:"command_timeout"`
}

type ContextConfig struct {
	MaxMessages     int     `mapstructure:"max_messages"`
	MaxTokens       int     `mapstructure:"max_tokens"`
	CompactionRatio float32 `mapstructure:"compaction_ratio"`
	AutoCompact     bool    `mapstructure:"auto_compact"`
}

func Load() (*Config, error) {
	v := viper.New()

	setDefaults(v)

	v.SetConfigName("ashron")
	v.SetConfigType("yaml")

	if configDir := os.Getenv("XDG_CONFIG_HOME"); configDir != "" {
		v.AddConfigPath(filepath.Join(configDir, "ashron"))
	}
	v.AddConfigPath(filepath.Join(os.Getenv("HOME"), ".config", "ashron"))

	v.SetEnvPrefix("ASHRON")
	v.AutomaticEnv()

	// Convenience: support OPENAI_API_KEY env var for the openai provider
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		v.Set("providers.openai.api_key", apiKey)
	}

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
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
	v.SetDefault("default.provider", "openai")
	v.SetDefault("default.model", "gpt4")

	v.SetDefault("tools.auto_approve_tools", []string{"read_file", "list_directory", "list_tools", "git_ls_files", "git_grep"})
	v.SetDefault("tools.auto_approve_commands", []string{})
	v.SetDefault("tools.max_output_size", 50000)
	v.SetDefault("tools.command_timeout", 10*time.Minute)

	v.SetDefault("context.max_messages", 50)
	v.SetDefault("context.max_tokens", 65535)
	v.SetDefault("context.compaction_ratio", 0.5)
	v.SetDefault("context.auto_compact", true)
}

// ActiveProvider returns the name and config of the currently active provider.
func (c *Config) ActiveProvider() (string, *ProviderConfig, error) {
	name := c.Default.Provider
	p, ok := c.Providers[name]
	if !ok {
		return "", nil, fmt.Errorf("provider %q not found in config", name)
	}
	return name, &p, nil
}

// ActiveModel returns the name and config of the currently active model.
func (c *Config) ActiveModel() (string, *ModelConfig, error) {
	_, provider, err := c.ActiveProvider()
	if err != nil {
		return "", nil, err
	}
	name := c.Default.Model
	m, ok := provider.Models[name]
	if !ok {
		return "", nil, fmt.Errorf("model %q not found in provider %q", name, c.Default.Provider)
	}
	return name, &m, nil
}

// FindModel searches all providers for a model by name.
// Returns the provider name, provider config, and model config.
func (c *Config) FindModel(modelName string) (string, *ProviderConfig, *ModelConfig, error) {
	for provName, prov := range c.Providers {
		if m, ok := prov.Models[modelName]; ok {
			p := prov
			mc := m
			return provName, &p, &mc, nil
		}
	}
	return "", nil, nil, fmt.Errorf("model %q not found in any provider", modelName)
}

// AllModelNames returns all model names across all providers, with their provider name.
func (c *Config) AllModelNames() []struct{ Provider, Model string } {
	var names []struct{ Provider, Model string }
	for provName, prov := range c.Providers {
		for modelName := range prov.Models {
			names = append(names, struct{ Provider, Model string }{provName, modelName})
		}
	}
	return names
}

func (c *Config) Validate() error {
	if len(c.Providers) == 0 {
		return &ConfigError{"no providers configured"}
	}
	_, provider, err := c.ActiveProvider()
	if err != nil {
		return err
	}
	if provider.APIKey == "" {
		return ErrMissingAPIKey
	}
	_, _, err = c.ActiveModel()
	return err
}

var ErrMissingAPIKey = &ConfigError{"API key is required"}

type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}

func createDefaultConfig() error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "ashron")
	configFile := filepath.Join(configDir, "ashron.yaml")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	defaultConfig := `# Ashron Configuration File

# Active provider and model
default:
  provider: openai
  model: gpt4

# Provider definitions
providers:
  openai:
    type: openai-compat
    base_url: https://api.openai.com/v1
    # Set your API key here or use OPENAI_API_KEY environment variable
    # api_key: YOUR_API_KEY_HERE
    timeout: 5m
    models:
      gpt4:
        model: gpt-4-turbo-preview
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

# Context Management
context:
  max_messages: 50
  max_tokens: 65535
  compaction_ratio: 0.5
  auto_compact: true
`

	if err := os.WriteFile(configFile, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
