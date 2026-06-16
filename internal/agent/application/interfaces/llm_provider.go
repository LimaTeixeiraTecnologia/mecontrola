package interfaces

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

type JSONSchemaSpec struct {
	Name   string
	Strict bool
	Schema map[string]any
}

type LLMRequest struct {
	SystemPrompt string
	UserMessage  string
	JSONSchema   *JSONSchemaSpec
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
