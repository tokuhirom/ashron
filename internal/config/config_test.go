package config

import "testing"

func TestActiveContextUsesDefaultWhenModelOverrideMissing(t *testing.T) {
	cfg := &Config{
		Default: DefaultConfig{Provider: "openai", Model: "gpt4"},
		Providers: map[string]ProviderConfig{
			"openai": {
				Models: map[string]ModelConfig{
					"gpt4": {Model: "gpt-4.1", Temperature: 0.7},
				},
			},
		},
		DefaultContext: ContextConfig{
			MaxMessages:     50,
			MaxTokens:       65535,
			CompactionRatio: 0.5,
			AutoCompact:     true,
		},
	}

	ctx, err := cfg.ActiveContext()
	if err != nil {
		t.Fatalf("ActiveContext returned error: %v", err)
	}
	if *ctx != cfg.DefaultContext {
		t.Fatalf("unexpected context: got %+v want %+v", *ctx, cfg.DefaultContext)
	}
}

func TestConvertConfigMergesModelContextOverride(t *testing.T) {
	maxTokens := 32768
	autoCompact := false

	cfg, err := convertConfig(rawConfig{
		Default: rawDefaultConfig{Provider: "openai", Model: "gpt4"},
		Providers: map[string]rawProviderConfig{
			"openai": {
				Type:    "openai-compat",
				BaseURL: "https://api.openai.com/v1",
				Timeout: "5m",
				Models: map[string]rawModelConfig{
					"gpt4": {
						Model:       "gpt-4.1",
						Temperature: 0.7,
						Context: &rawContextOverrideConfig{
							MaxTokens:   &maxTokens,
							AutoCompact: &autoCompact,
						},
					},
				},
			},
		},
		Tools: rawToolsConfig{
			CommandTimeout: "10m",
			SandboxMode:    "auto",
		},
		DefaultContext: rawContextConfig{
			MaxMessages:     50,
			MaxTokens:       65535,
			CompactionRatio: 0.5,
		},
	})
	if err != nil {
		t.Fatalf("convertConfig returned error: %v", err)
	}

	if cfg.DefaultContext.MaxTokens != 65535 {
		t.Fatalf("default context should be unchanged, got %d", cfg.DefaultContext.MaxTokens)
	}
	if cfg.DefaultContext.AutoCompact != true {
		t.Fatalf("default auto_compact should default to true")
	}

	_, model, err := cfg.ActiveModel()
	if err != nil {
		t.Fatalf("ActiveModel returned error: %v", err)
	}
	if model.Context == nil {
		t.Fatalf("expected model context override to be present")
	}
	if model.Context.MaxMessages != 50 {
		t.Fatalf("max_messages should inherit default, got %d", model.Context.MaxMessages)
	}
	if model.Context.MaxTokens != 32768 {
		t.Fatalf("max_tokens override not applied, got %d", model.Context.MaxTokens)
	}
	if model.Context.CompactionRatio != 0.5 {
		t.Fatalf("compaction_ratio should inherit default, got %f", model.Context.CompactionRatio)
	}
	if model.Context.AutoCompact != false {
		t.Fatalf("auto_compact override not applied, got %v", model.Context.AutoCompact)
	}

	activeCtx, err := cfg.ActiveContext()
	if err != nil {
		t.Fatalf("ActiveContext returned error: %v", err)
	}
	if activeCtx.MaxTokens != 32768 {
		t.Fatalf("active context should use model override, got %d", activeCtx.MaxTokens)
	}
}
