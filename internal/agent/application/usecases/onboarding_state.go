package usecases

import (
	"context"

	"github.com/google/uuid"
)

type OnboardingSnapshotCard struct {
	Name       string
	ClosingDay int
}

type OnboardingSnapshotSplit struct {
	Slug    string
	Percent int
}

type OnboardingSnapshot struct {
	InProgress      bool
	Phase           string
	IncomeCents     int64
	Objective       string
	Cards           []OnboardingSnapshotCard
	Splits          []OnboardingSnapshotSplit
	FirstTxRecorded bool
	WelcomeSent     bool
}

type OnboardingStateReader interface {
	Load(ctx context.Context, userID uuid.UUID) (OnboardingSnapshot, error)
}
