package interfaces

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

type LLMRequest struct {
	SystemPrompt string
	UserMessage  string
}

type LLMResponse struct {
	Provider         valueobjects.ModelSlug
	RawJSON          []byte
	PromptTokens     int
	CompletionTokens int
}

type LLMProvider interface {
	Slug() valueobjects.ModelSlug
	Interpret(ctx context.Context, req LLMRequest) (LLMResponse, error)
}
