package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Default        DefaultConfig
	Providers      map[string]ProviderConfig
	Tools          ToolsConfig
	DefaultContext ContextConfig
	MCPServers     map[string]MCPServerConfig
	Debug          bool
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
	Model            string
	Temperature      float32
	TopP             float32
	MinP             float32
	TopK             int
	FrequencyPenalty float32
	PresencePenalty  float32
	Stop             []string
	Context          *ContextConfig
}

type ToolsConfig struct {
	AutoApproveTools    []string
	AutoApproveCommands []string
	MaxOutputSize       int
	CommandTimeout      time.Duration
	SandboxMode         string
	Yolo                bool
	MCPServers          map[string]MCPServerConfig
}

type ContextConfig struct {
	MaxMessages     int
	MaxTokens       int
	CompactionRatio float32
	AutoCompact     bool
}

type MCPServerConfig struct {
	Command        string
	Args           []string
	Env            map[string]string
	WorkingDir     string
	StartupTimeout time.Duration
	CallTimeout    time.Duration
}

// rawConfig mirrors Config but uses string for Duration fields to support YAML parsing.
type rawConfig struct {
	Default        rawDefaultConfig              `yaml:"default"`
	Providers      map[string]rawProviderConfig  `yaml:"providers"`
	Tools          rawToolsConfig                `yaml:"tools"`
	DefaultContext rawContextConfig              `yaml:"default_context"`
	MCPServers     map[string]rawMCPServerConfig `yaml:"mcp_servers"`
	Debug          bool                          `yaml:"debug"`
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
	Model            string                    `yaml:"model"`
	Temperature      float32                   `yaml:"temperature"`
	TopP             float32                   `yaml:"top_p"`
	MinP             float32                   `yaml:"min_p"`
	TopK             int                       `yaml:"top_k"`
	FrequencyPenalty float32                   `yaml:"frequency_penalty"`
	PresencePenalty  float32                   `yaml:"presence_penalty"`
	Stop             []string                  `yaml:"stop"`
	Context          *rawContextOverrideConfig `yaml:"context"`
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

type rawContextOverrideConfig struct {
	MaxMessages     *int     `yaml:"max_messages"`
	MaxTokens       *int     `yaml:"max_tokens"`
	CompactionRatio *float32 `yaml:"compaction_ratio"`
	AutoCompact     *bool    `yaml:"auto_compact"`
}

type rawMCPServerConfig struct {
	Command        string            `yaml:"command"`
	Args           []string          `yaml:"args"`
	Env            map[string]string `yaml:"env"`
	WorkingDir     string            `yaml:"working_dir"`
	StartupTimeout string            `yaml:"startup_timeout"`
	CallTimeout    string            `yaml:"call_timeout"`
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

// xdgConfigDir returns $XDG_CONFIG_HOME if set, else $HOME/.config per XDG spec.
func xdgConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".config")
}

func applyDefaults(raw *rawConfig) {
	if raw.Default.Provider == "" {
		raw.Default.Provider = "openai"
	}
	if raw.Default.Model == "" {
		raw.Default.Model = "gpt4"
	}
	if len(raw.Tools.AutoApproveTools) == 0 {
		raw.Tools.AutoApproveTools = []string{"read_file", "read_skill", "list_directory", "list_tools", "get_diagnostics", "memory_list"}
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
	if raw.DefaultContext.MaxMessages == 0 {
		raw.DefaultContext.MaxMessages = 50
	}
	if raw.DefaultContext.MaxTokens == 0 {
		raw.DefaultContext.MaxTokens = 65535
	}
	if raw.DefaultContext.CompactionRatio == 0 {
		raw.DefaultContext.CompactionRatio = 0.5
	}
}

func convertConfig(raw rawConfig) (*Config, error) {
	cmdTimeout, err := parseDuration(raw.Tools.CommandTimeout, 10*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("invalid tools.command_timeout: %w", err)
	}
	mcpServers, err := convertMCPServers(raw.MCPServers)
	if err != nil {
		return nil, err
	}

	autoCompact := true
	if raw.DefaultContext.AutoCompact != nil {
		autoCompact = *raw.DefaultContext.AutoCompact
	}
	defaultContext := ContextConfig{
		MaxMessages:     raw.DefaultContext.MaxMessages,
		MaxTokens:       raw.DefaultContext.MaxTokens,
		CompactionRatio: raw.DefaultContext.CompactionRatio,
		AutoCompact:     autoCompact,
	}

	providers := make(map[string]ProviderConfig, len(raw.Providers))
	for name, rp := range raw.Providers {
		timeout, err := parseDuration(rp.Timeout, 5*time.Minute)
		if err != nil {
			return nil, fmt.Errorf("invalid providers.%s.timeout: %w", name, err)
		}
		models := make(map[string]ModelConfig, len(rp.Models))
		for mname, rm := range rp.Models {
			modelCfg := ModelConfig{
				Model:            rm.Model,
				Temperature:      rm.Temperature,
				TopP:             rm.TopP,
				MinP:             rm.MinP,
				TopK:             rm.TopK,
				FrequencyPenalty: rm.FrequencyPenalty,
				PresencePenalty:  rm.PresencePenalty,
				Stop:             rm.Stop,
			}
			if rm.Context != nil {
				ctx := mergeContext(defaultContext, rm.Context)
				modelCfg.Context = &ctx
			}
			models[mname] = modelCfg
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
		Default:   DefaultConfig{Provider: raw.Default.Provider, Model: raw.Default.Model},
		Providers: providers,
		Tools: ToolsConfig{
			AutoApproveTools:    raw.Tools.AutoApproveTools,
			AutoApproveCommands: raw.Tools.AutoApproveCommands,
			MaxOutputSize:       raw.Tools.MaxOutputSize,
			CommandTimeout:      cmdTimeout,
			SandboxMode:         raw.Tools.SandboxMode,
			MCPServers:          mcpServers,
		},
		DefaultContext: defaultContext,
		MCPServers:     mcpServers,
		Debug:          raw.Debug,
	}, nil
}

func convertMCPServers(raw map[string]rawMCPServerConfig) (map[string]MCPServerConfig, error) {
	out := make(map[string]MCPServerConfig, len(raw))
	for name, cfg := range raw {
		if strings.TrimSpace(cfg.Command) == "" {
			return nil, fmt.Errorf("mcp_servers.%s.command is required", name)
		}
		startupTimeout, err := parseDuration(cfg.StartupTimeout, 15*time.Second)
		if err != nil {
			return nil, fmt.Errorf("invalid mcp_servers.%s.startup_timeout: %w", name, err)
		}
		callTimeout, err := parseDuration(cfg.CallTimeout, 30*time.Second)
		if err != nil {
			return nil, fmt.Errorf("invalid mcp_servers.%s.call_timeout: %w", name, err)
		}
		env := make(map[string]string, len(cfg.Env))
		for k, v := range cfg.Env {
			env[k] = v
		}
		out[name] = MCPServerConfig{
			Command:        cfg.Command,
			Args:           append([]string(nil), cfg.Args...),
			Env:            env,
			WorkingDir:     cfg.WorkingDir,
			StartupTimeout: startupTimeout,
			CallTimeout:    callTimeout,
		}
	}
	return out, nil
}

func mergeContext(base ContextConfig, override *rawContextOverrideConfig) ContextConfig {
	cfg := base
	if override.MaxMessages != nil {
		cfg.MaxMessages = *override.MaxMessages
	}
	if override.MaxTokens != nil {
		cfg.MaxTokens = *override.MaxTokens
	}
	if override.CompactionRatio != nil {
		cfg.CompactionRatio = *override.CompactionRatio
	}
	if override.AutoCompact != nil {
		cfg.AutoCompact = *override.AutoCompact
	}
	return cfg
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

// ActiveContext returns context settings for the active model.
func (c *Config) ActiveContext() (*ContextConfig, error) {
	_, model, err := c.ActiveModel()
	if err != nil {
		return nil, err
	}
	if model.Context != nil {
		return model.Context, nil
	}
	return &c.DefaultContext, nil
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
    - read_skill
    - list_directory
    - list_tools
  auto_approve_commands:
    - /^git add .*$/
  max_output_size: 50000
  command_timeout: 10m
  sandbox_mode: auto

# Default Context Management
default_context:
  max_messages: 50
  max_tokens: 65535
  compaction_ratio: 0.5
  auto_compact: true

# Enable debug logging (writes logs under $XDG_DATA_HOME/ashron/logs)
debug: false
`

	if err := os.WriteFile(cfgPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
