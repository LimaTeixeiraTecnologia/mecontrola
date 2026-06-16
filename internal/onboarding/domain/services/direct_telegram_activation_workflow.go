package services

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

type DirectActivationOutcome uint8

const (
	OutcomeRequiresWhatsAppActivation DirectActivationOutcome = iota + 1
	OutcomeDirectAllowed
	OutcomeDirectBlocked
)

func (o DirectActivationOutcome) String() string {
	switch o {
	case OutcomeRequiresWhatsAppActivation:
		return "requires_whatsapp_activation"
	case OutcomeDirectAllowed:
		return "direct_allowed"
	case OutcomeDirectBlocked:
		return "direct_blocked"
	default:
		return "unknown"
	}
}

type DirectActivationDecision struct {
	Outcome            DirectActivationOutcome
	CustomerMobileE164 string
	CustomerEmail      string
}

type DirectTelegramActivationWorkflow struct{}

func NewDirectTelegramActivationWorkflow() DirectTelegramActivationWorkflow {
	return DirectTelegramActivationWorkflow{}
}

func (w DirectTelegramActivationWorkflow) Decide(token entities.MagicToken, flagEnabled bool) DirectActivationDecision {
	if !flagEnabled {
		return DirectActivationDecision{Outcome: OutcomeRequiresWhatsAppActivation}
	}
	mobile := token.CustomerMobileE164()
	email := token.CustomerEmail()
	if mobile == "" || email == "" {
		return DirectActivationDecision{Outcome: OutcomeDirectBlocked}
	}
	return DirectActivationDecision{
		Outcome:            OutcomeDirectAllowed,
		CustomerMobileE164: mobile,
		CustomerEmail:      email,
	}
}
