package services

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

type IntentOutcomeKind int

const (
	IntentOutcomeRouted IntentOutcomeKind = iota + 1
	IntentOutcomeStructuredError
	IntentOutcomeProviderExhausted
	IntentOutcomeUnsupportedAction
)

func (k IntentOutcomeKind) String() string {
	switch k {
	case IntentOutcomeRouted:
		return "routed"
	case IntentOutcomeStructuredError:
		return "structured_error"
	case IntentOutcomeProviderExhausted:
		return "provider_exhausted"
	case IntentOutcomeUnsupportedAction:
		return "unsupported_action"
	default:
		return "invalid"
	}
}

type IntentOutcome struct {
	Kind         IntentOutcomeKind
	Intent       entities.IntentResult
	Provider     valueobjects.ModelSlug
	Reason       string
	ResponseHint string
	EventID      uuid.UUID
	OccurredAt   time.Time
}

type IntentWorkflow struct {
	supportedActions map[string]map[string]struct{}
}

func NewIntentWorkflow() IntentWorkflow {
	supported := map[string]map[string]struct{}{
		"categories":   {"list": {}, "get": {}},
		"cards":        {"list": {}, "get": {}, "create": {}},
		"budgets":      {"list": {}, "get": {}, "create": {}},
		"transactions": {"list": {}, "get": {}, "create": {}, "delete": {}},
	}
	return IntentWorkflow{supportedActions: supported}
}

func (w IntentWorkflow) DecideRoute(
	intent entities.IntentResult,
	provider valueobjects.ModelSlug,
	eventID uuid.UUID,
	now time.Time,
) IntentOutcome {
	base := IntentOutcome{
		Intent:     intent,
		Provider:   provider,
		EventID:    eventID,
		OccurredAt: now,
	}

	if intent.IsError() {
		base.Kind = IntentOutcomeStructuredError
		base.Reason = intent.Error().Code
		base.ResponseHint = intent.Error().Message
		return base
	}

	moduleSupported, ok := w.supportedActions[intent.Module().String()]
	if !ok {
		base.Kind = IntentOutcomeUnsupportedAction
		base.Reason = "module_not_supported"
		base.ResponseHint = intent.ResponseHint()
		return base
	}
	if _, allowed := moduleSupported[intent.Action().String()]; !allowed {
		base.Kind = IntentOutcomeUnsupportedAction
		base.Reason = "action_not_supported_for_module"
		base.ResponseHint = intent.ResponseHint()
		return base
	}

	base.Kind = IntentOutcomeRouted
	base.ResponseHint = intent.ResponseHint()
	return base
}

func (w IntentWorkflow) DecideExhausted(
	lastReason string,
	eventID uuid.UUID,
	now time.Time,
) IntentOutcome {
	return IntentOutcome{
		Kind:         IntentOutcomeProviderExhausted,
		Reason:       lastReason,
		ResponseHint: "Estou com instabilidade momentanea. Tente novamente em instantes.",
		EventID:      eventID,
		OccurredAt:   now,
	}
}
