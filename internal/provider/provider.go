package provider

import (
	"fmt"

	"github.com/delta-code/cli/pkg/models"
)

type Provider interface {
	Name() string
	Type() models.ProviderType
	Chat(req models.ChatRequest) (*models.ChatResponse, error)
	ChatStream(req models.ChatRequest) (<-chan models.StreamChunk, error)
	ListModels() ([]models.Model, error)
}

func NewProvider(cfg models.ProviderConfig) (Provider, error) {
	switch cfg.Type {
	case models.ProviderOpenAI, models.ProviderDeepSeek, models.ProviderCustom:
		if cfg.BaseURL == "" {
			cfg.BaseURL = "https://api.openai.com/v1"
		}
		return NewOpenAICompatible(cfg), nil
	case models.ProviderAnthropic:
		return NewAnthropic(cfg), nil
	case models.ProviderGoogle:
		return NewGoogle(cfg), nil
	case models.ProviderOllama:
		return NewOllama(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Type)
	}
}

func DefaultProviders() []models.ProviderConfig {
	return []models.ProviderConfig{
		{
			Name:    "openai",
			Type:    models.ProviderOpenAI,
			BaseURL: "https://api.openai.com/v1",
			Models:  []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"},
		},
		{
			Name:    "anthropic",
			Type:    models.ProviderAnthropic,
			BaseURL: "https://api.anthropic.com/v1",
			Models:  []string{"claude-sonnet-4-20250514", "claude-haiku-3-5-20241022"},
		},
		{
			Name:    "google",
			Type:    models.ProviderGoogle,
			BaseURL: "https://generativelanguage.googleapis.com/v1beta",
			Models:  []string{"gemini-2.0-flash", "gemini-1.5-pro", "gemini-1.5-flash"},
		},
		{
			Name:    "deepseek",
			Type:    models.ProviderDeepSeek,
			BaseURL: "https://api.deepseek.com/v1",
			Models:  []string{"deepseek-chat", "deepseek-coder"},
		},
		{
			Name:    "ollama",
			Type:    models.ProviderOllama,
			BaseURL: "http://localhost:11434",
			Models:  []string{},
		},
	}
}
