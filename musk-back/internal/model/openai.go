package model

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"

	"autonomous-agent/internal/config"
)

// OpenAIProvider talks to any OpenAI-compatible Chat Completions API.
type OpenAIProvider struct {
	cfg config.LLMConfig
}

// NewOpenAIProvider builds a provider from LLM configuration.
func NewOpenAIProvider(cfg config.LLMConfig) *OpenAIProvider {
	return &OpenAIProvider{cfg: cfg}
}

// NewChatModel creates an Eino OpenAI chat model pointed at the configured base URL.
func (p *OpenAIProvider) NewChatModel(ctx context.Context) (einomodel.BaseChatModel, error) {
	cm, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: p.cfg.BaseURL,
		APIKey:  p.cfg.APIKey,
		Model:   p.cfg.Model,
		Timeout: p.cfg.Timeout.Duration(),
	})
	if err != nil {
		return nil, fmt.Errorf("create openai-compatible chat model: %w", err)
	}
	return cm, nil
}
