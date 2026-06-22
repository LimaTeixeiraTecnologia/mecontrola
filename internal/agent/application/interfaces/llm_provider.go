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

type ToolSpec struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type ToolCall struct {
	ID            string
	FunctionName  string
	ArgumentsJSON map[string]any
}

type ConversationMessage struct {
	Role    string
	Content string
}

type LLMRequest struct {
	SystemPrompt string
	UserMessage  string
	Messages     []ConversationMessage
	JSONSchema   *JSONSchemaSpec
	FreeText     bool
	MaxTokens    int
	Tools        []ToolSpec
	ToolChoice   string
}

type LLMResponse struct {
	Provider         valueobjects.ModelSlug
	RawJSON          []byte
	PromptTokens     int
	CompletionTokens int
	ToolCalls        []ToolCall
}

type LLMProvider interface {
	Slug() valueobjects.ModelSlug
	Interpret(ctx context.Context, req LLMRequest) (LLMResponse, error)
}
