package services

import (
	"fmt"
	"time"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type MarkPaidOutcome uint8

const (
	MarkPaidOutcomeTransitioned MarkPaidOutcome = iota + 1
	MarkPaidOutcomeNoChange
)

type MarkPaidCommand struct {
	SubscriptionID     string
	CustomerMobileE164 string
	CustomerEmail      string
	ExternalSaleID     string
	PaidAt             time.Time
}

type MarkPaidDecision struct {
	Token   entities.MagicToken
	Outcome MarkPaidOutcome
}

type ConsumeCommand struct {
	UserID         string
	FromE164       string
	ActivationPath valueobjects.ActivationPath
	EventID        string
}

type ConsumeDecision struct {
	Token entities.MagicToken
	Event entities.SubscriptionBound
}

type MagicTokenWorkflow struct{}

func NewMagicTokenWorkflow() MagicTokenWorkflow {
	return MagicTokenWorkflow{}
}

func (w MagicTokenWorkflow) DecideMarkPaid(
	current entities.MagicToken,
	cmd MarkPaidCommand,
) (MarkPaidDecision, error) {
	if current.Status() != valueobjects.TokenStatusPending {
		return MarkPaidDecision{Token: current, Outcome: MarkPaidOutcomeNoChange}, nil
	}
	if cmd.SubscriptionID == "" {
		return MarkPaidDecision{}, fmt.Errorf("onboarding: subscription id is required")
	}
	updated, err := current.MarkPaid(cmd.SubscriptionID, cmd.CustomerMobileE164, cmd.CustomerEmail, cmd.ExternalSaleID, cmd.PaidAt)
	if err != nil {
		return MarkPaidDecision{}, fmt.Errorf("onboarding: decide mark paid: %w", err)
	}
	return MarkPaidDecision{Token: updated, Outcome: MarkPaidOutcomeTransitioned}, nil
}

func (w MagicTokenWorkflow) DecideConsume(
	current entities.MagicToken,
	cmd ConsumeCommand,
	now time.Time,
) (ConsumeDecision, error) {
	if current.Status() != valueobjects.TokenStatusPaid {
		return ConsumeDecision{}, domain.ErrTransitionNotAllowed
	}
	if cmd.UserID == "" {
		return ConsumeDecision{}, fmt.Errorf("onboarding: consume: user id is required")
	}
	if cmd.EventID == "" {
		return ConsumeDecision{}, fmt.Errorf("onboarding: consume: event id is required")
	}
	consumed, err := current.MarkConsumed(cmd.UserID, cmd.FromE164, cmd.ActivationPath, now)
	if err != nil {
		return ConsumeDecision{}, fmt.Errorf("onboarding: decide consume: %w", err)
	}
	evt := entities.SubscriptionBound{
		EventID:         cmd.EventID,
		TokenID:         consumed.ID(),
		UserID:          cmd.UserID,
		SubscriptionID:  consumed.SubscriptionID(),
		TokenHashPrefix: valueobjects.TokenHashPrefix(consumed.TokenHash()),
		ActivationPath:  cmd.ActivationPath,
		BoundAt:         now,
	}
	return ConsumeDecision{Token: consumed, Event: evt}, nil
}
