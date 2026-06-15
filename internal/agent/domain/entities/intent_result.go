package entities

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

var ErrIntentResultEmptyHint = errors.New("agent.llm: response hint is empty")

var ErrIntentResultHintTooLong = errors.New("agent.llm: response hint exceeds maximum length")

const maxResponseHintLength = 280

type RawIntent struct {
	Module       string          `json:"module"`
	Action       string          `json:"action"`
	Filters      json.RawMessage `json:"filters,omitempty"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	ResponseHint string          `json:"response_hint"`
	Error        string          `json:"error,omitempty"`
	Message      string          `json:"message,omitempty"`
}

type IntentError struct {
	Code    string
	Message string
}

func (e IntentError) IsZero() bool { return e.Code == "" && e.Message == "" }

type IntentResult struct {
	module       valueobjects.IntentModule
	action       valueobjects.IntentAction
	rawFilters   json.RawMessage
	rawPayload   json.RawMessage
	responseHint string
	intentError  IntentError
}

func NewIntentResult(
	module valueobjects.IntentModule,
	action valueobjects.IntentAction,
	filters json.RawMessage,
	payload json.RawMessage,
	responseHint string,
) (IntentResult, error) {
	if module.IsZero() {
		return IntentResult{}, fmt.Errorf("agent.llm: intent result: %w", valueobjects.ErrIntentModuleEmpty)
	}
	if action.IsZero() {
		return IntentResult{}, fmt.Errorf("agent.llm: intent result: %w", valueobjects.ErrIntentActionEmpty)
	}
	trimmedHint := strings.TrimSpace(responseHint)
	if trimmedHint == "" {
		return IntentResult{}, ErrIntentResultEmptyHint
	}
	if len([]rune(trimmedHint)) > maxResponseHintLength {
		return IntentResult{}, ErrIntentResultHintTooLong
	}
	return IntentResult{
		module:       module,
		action:       action,
		rawFilters:   filters,
		rawPayload:   payload,
		responseHint: trimmedHint,
	}, nil
}

func NewIntentResultFromError(intentError IntentError) IntentResult {
	return IntentResult{intentError: intentError}
}

func (r IntentResult) IsError() bool { return !r.intentError.IsZero() }

func (r IntentResult) Module() valueobjects.IntentModule { return r.module }
func (r IntentResult) Action() valueobjects.IntentAction { return r.action }
func (r IntentResult) Filters() json.RawMessage          { return r.rawFilters }
func (r IntentResult) Payload() json.RawMessage          { return r.rawPayload }
func (r IntentResult) ResponseHint() string              { return r.responseHint }
func (r IntentResult) Error() IntentError                { return r.intentError }
