package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Default   DefaultConfig
	Providers map[string]ProviderConfig
	Tools     ToolsConfig
	Context   ContextConfig
}

type DefaultConfig struct {
	Provider string
	Model    string
}

type ProviderConfig struct {
	Type    string
	BaseURL string
	APIKey  string
	Timeout time.Duration
	Models  map[string]ModelConfig
}

type ModelConfig struct {
	Model       string
	Temperature float32
}

type ToolsConfig struct {
	AutoApproveTools    []string
	AutoApproveCommands []string
	MaxOutputSize       int
	CommandTimeout      time.Duration
	SandboxMode         string
	Yolo                bool
}

type ContextConfig struct {
	MaxMessages     int
	MaxTokens       int
	CompactionRatio float32
	AutoCompact     bool
}

// rawConfig mirrors Config but uses string for Duration fields to support YAML parsing.
type rawConfig struct {
	Default   rawDefaultConfig             `yaml:"default"`
	Providers map[string]rawProviderConfig `yaml:"providers"`
	Tools     rawToolsConfig               `yaml:"tools"`
	Context   rawContextConfig             `yaml:"context"`
}

type rawDefaultConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

type rawProviderConfig struct {
	Type    string                    `yaml:"type"`
	BaseURL string                    `yaml:"base_url"`
	APIKey  string                    `yaml:"api_key"`
	Timeout string                    `yaml:"timeout"`
	Models  map[string]rawModelConfig `yaml:"models"`
}

type rawModelConfig struct {
	Model       string  `yaml:"model"`
	Temperature float32 `yaml:"temperature"`
}

type rawToolsConfig struct {
	AutoApproveTools    []string `yaml:"auto_approve_tools"`
	AutoApproveCommands []string `yaml:"auto_approve_commands"`
	MaxOutputSize       int      `yaml:"max_output_size"`
	CommandTimeout      string   `yaml:"command_timeout"`
	SandboxMode         string   `yaml:"sandbox_mode"`
}

type rawContextConfig struct {
	MaxMessages     int     `yaml:"max_messages"`
	MaxTokens       int     `yaml:"max_tokens"`
	CompactionRatio float32 `yaml:"compaction_ratio"`
	AutoCompact     *bool   `yaml:"auto_compact"`
}

func Load() (*Config, error) {
	cfgPath := configFilePath()

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := createDefaultConfig(cfgPath); err != nil {
				return nil, fmt.Errorf("failed to create default config: %w", err)
			}
			return nil, fmt.Errorf("config file not found. Created default config at %s. Please set your API key and run again", cfgPath)
		}
		return nil, fmt.Errorf("failed to read config file '%s': %w", cfgPath, err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse config file '%s': %w", cfgPath, err)
	}

	applyDefaults(&raw)

	// Apply OPENAI_API_KEY env var to openai provider if api_key is unset.
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		if prov, ok := raw.Providers["openai"]; ok && prov.APIKey == "" {
			prov.APIKey = apiKey
			raw.Providers["openai"] = prov
		}
	}

	return convertConfig(raw)
}

func configFilePath() string {
	return filepath.Join(xdgConfigDir(), "ashron", "ashron.yaml")
}

// xdgConfigDir returns $XDG_CONFIG_HOME if set, else falls back to os.UserConfigDir().
func xdgConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return dir
	}
	return filepath.Join(os.Getenv("HOME"), ".config")
}

func applyDefaults(raw *rawConfig) {
	if raw.Default.Provider == "" {
		raw.Default.Provider = "openai"
	}
	if raw.Default.Model == "" {
		raw.Default.Model = "gpt4"
	}
	if len(raw.Tools.AutoApproveTools) == 0 {
		raw.Tools.AutoApproveTools = []string{"read_file", "list_directory", "list_tools", "git_ls_files", "git_grep"}
	}
	if raw.Tools.MaxOutputSize == 0 {
		raw.Tools.MaxOutputSize = 50000
	}
	if raw.Tools.CommandTimeout == "" {
		raw.Tools.CommandTimeout = "10m"
	}
	if raw.Tools.SandboxMode == "" {
		raw.Tools.SandboxMode = "auto"
	}
	if raw.Context.MaxMessages == 0 {
		raw.Context.MaxMessages = 50
	}
	if raw.Context.MaxTokens == 0 {
		raw.Context.MaxTokens = 65535
	}
	if raw.Context.CompactionRatio == 0 {
		raw.Context.CompactionRatio = 0.5
	}
}

func convertConfig(raw rawConfig) (*Config, error) {
	cmdTimeout, err := parseDuration(raw.Tools.CommandTimeout, 10*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("invalid tools.command_timeout: %w", err)
	}

	autoCompact := true
	if raw.Context.AutoCompact != nil {
		autoCompact = *raw.Context.AutoCompact
	}

	providers := make(map[string]ProviderConfig, len(raw.Providers))
	for name, rp := range raw.Providers {
		timeout, err := parseDuration(rp.Timeout, 5*time.Minute)
		if err != nil {
			return nil, fmt.Errorf("invalid providers.%s.timeout: %w", name, err)
		}
		models := make(map[string]ModelConfig, len(rp.Models))
		for mname, rm := range rp.Models {
			models[mname] = ModelConfig(rm)
		}
		providers[name] = ProviderConfig{
			Type:    rp.Type,
			BaseURL: rp.BaseURL,
			APIKey:  rp.APIKey,
			Timeout: timeout,
			Models:  models,
		}
	}

	return &Config{
		Default: DefaultConfig{
			Provider: raw.Default.Provider,
			Model:    raw.Default.Model,
		},
		Providers: providers,
		Tools: ToolsConfig{
			AutoApproveTools:    raw.Tools.AutoApproveTools,
			AutoApproveCommands: raw.Tools.AutoApproveCommands,
			MaxOutputSize:       raw.Tools.MaxOutputSize,
			CommandTimeout:      cmdTimeout,
			SandboxMode:         raw.Tools.SandboxMode,
		},
		Context: ContextConfig{
			MaxMessages:     raw.Context.MaxMessages,
			MaxTokens:       raw.Context.MaxTokens,
			CompactionRatio: raw.Context.CompactionRatio,
			AutoCompact:     autoCompact,
		},
	}, nil
}

func parseDuration(s string, defaultVal time.Duration) (time.Duration, error) {
	if s == "" {
		return defaultVal, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	return d, nil
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

func createDefaultConfig(cfgPath string) error {
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
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
  sandbox_mode: auto

# Context Management
context:
  max_messages: 50
  max_tokens: 65535
  compaction_ratio: 0.5
  auto_compact: true
`

	if err := os.WriteFile(cfgPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
