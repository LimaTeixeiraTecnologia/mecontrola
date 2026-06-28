package workflow

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

const OnboardingWelcomeSignal = "__onboarding_welcome__"

type OnboardingAwaiting int

const (
	AwaitingNone OnboardingAwaiting = iota + 1
	AwaitingText
	AwaitingConfirm
)

func (a OnboardingAwaiting) String() string {
	switch a {
	case AwaitingNone:
		return "none"
	case AwaitingText:
		return "text"
	case AwaitingConfirm:
		return "confirm"
	default:
		return "unknown"
	}
}

func (a OnboardingAwaiting) IsValid() bool {
	return a >= AwaitingNone && a <= AwaitingConfirm
}

type CorrectionTarget int

const (
	CorrectionTargetNone CorrectionTarget = iota + 1
	CorrectionTargetObjective
	CorrectionTargetBudget
	CorrectionTargetCards
	CorrectionTargetValues
)

func (c CorrectionTarget) String() string {
	switch c {
	case CorrectionTargetNone:
		return "none"
	case CorrectionTargetObjective:
		return "objective"
	case CorrectionTargetBudget:
		return "budget"
	case CorrectionTargetCards:
		return "cards"
	case CorrectionTargetValues:
		return "values"
	default:
		return "unknown"
	}
}

func (c CorrectionTarget) IsValid() bool {
	return c >= CorrectionTargetNone && c <= CorrectionTargetValues
}

type OnboardingCardState struct {
	Name   string `json:"name"`
	DueDay int    `json:"due_day"`
}

type OnboardingState struct {
	UserID              uuid.UUID                    `json:"user_id"`
	Phase               valueobjects.OnboardingPhase `json:"phase"`
	Awaiting            OnboardingAwaiting           `json:"awaiting"`
	Inbound             string                       `json:"inbound"`
	MessageID           string                       `json:"message_id"`
	ProcessedMessageIDs []string                     `json:"processed_message_ids"`
	CardLoop            int                          `json:"card_loop"`
	Values              map[string]int64             `json:"values"`
	Ack                 string                       `json:"ack"`
	Correction          CorrectionTarget             `json:"correction"`
	RepromptCount       int                          `json:"reprompt_count"`
	SuspendedAt         time.Time                    `json:"suspended_at"`
	AbandonedAt         time.Time                    `json:"abandoned_at"`
}
