package valueobjects

import (
	"errors"
	"fmt"
	"strings"
)

var ErrModelSlugEmpty = errors.New("agent.llm: model slug is empty")

var ErrModelSlugUnknown = errors.New("agent.llm: model slug is not in the OpenRouter allowlist")

type ModelSlug struct {
	value string
}

const (
	modelGeminiFlashLite = "google/gemini-2.5-flash-lite"
	modelGPT5Nano        = "openai/gpt-5-nano"
	modelMistralSmall    = "mistralai/mistral-small-3.2-24b-instruct"
	modelClaudeHaiku45   = "anthropic/claude-haiku-4.5"
)

func NewModelSlug(raw string) (ModelSlug, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ModelSlug{}, ErrModelSlugEmpty
	}
	switch trimmed {
	case modelGeminiFlashLite, modelGPT5Nano, modelMistralSmall, modelClaudeHaiku45:
		return ModelSlug{value: trimmed}, nil
	default:
		return ModelSlug{}, fmt.Errorf("agent.llm: %q: %w", raw, ErrModelSlugUnknown)
	}
}

func ModelSlugGeminiFlashLite() ModelSlug { return ModelSlug{value: modelGeminiFlashLite} }
func ModelSlugGPT5Nano() ModelSlug        { return ModelSlug{value: modelGPT5Nano} }

func (m ModelSlug) String() string { return m.value }
func (m ModelSlug) IsZero() bool   { return m.value == "" }

func (m ModelSlug) Equal(o ModelSlug) bool { return m.value == o.value }
