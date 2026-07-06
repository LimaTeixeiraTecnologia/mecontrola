package interfaces

import (
	"context"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type CategoryWriteGate interface {
	Approve(ctx context.Context, in CategoryWriteGateInput) (valueobjects.CategoryWriteEvidence, error)
}

type CategoryWriteGateInput struct {
	Direction       string
	RootCategoryID  uuid.UUID
	SubcategoryID   uuid.UUID
	Source          valueobjects.CategoryDecisionSource
	Outcome         string
	Score           float64
	Confidence      string
	Quality         string
	SignalType      string
	MatchedTerm     string
	MatchReason     string
	ExpectedVersion int64
	Surface         string
}
