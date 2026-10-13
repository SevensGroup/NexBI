package model

import (
	"context"

	einomodel "github.com/cloudwego/eino/components/model"
)

// Provider constructs an Eino chat model from runtime configuration.
// Implementations must not be HTTP-aware; they only talk to LLM endpoints.
type Provider interface {
	NewChatModel(ctx context.Context) (einomodel.BaseChatModel, error)
}

var _ Provider = (*OpenAIProvider)(nil)
