package entities

import (
	"time"

	"github.com/google/uuid"
)

type OnboardingDomainEvent interface {
	EventType() string
}

type IncomeRegistered struct {
	EventID     uuid.UUID
	UserID      uuid.UUID
	Channel     string
	IncomeCents int64
	OccurredAt  time.Time
}

func (IncomeRegistered) EventType() string { return "onboarding.income_registered" }

type CardRegistered struct {
	EventID    uuid.UUID
	UserID     uuid.UUID
	Channel    string
	Name       string
	LimitCents int64
	ClosingDay int
	DueDay     int
	OccurredAt time.Time
}

func (CardRegistered) EventType() string { return "onboarding.card_registered" }

type SplitsCalculated struct {
	EventID     uuid.UUID
	UserID      uuid.UUID
	Channel     string
	IncomeCents int64
	Allocations []SplitsCalculatedEntry
	OccurredAt  time.Time
}

type SplitsCalculatedEntry struct {
	Kind    string
	Percent int
}

func (SplitsCalculated) EventType() string { return "onboarding.splits_calculated" }

type OnboardingCompleted struct {
	EventID    uuid.UUID
	UserID     uuid.UUID
	Channel    string
	OccurredAt time.Time
}

func (OnboardingCompleted) EventType() string { return "onboarding.completed" }
