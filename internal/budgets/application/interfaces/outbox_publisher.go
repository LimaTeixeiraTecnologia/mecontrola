package interfaces

import (
	"context"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ExpenseCommittedEnvelope struct {
	UserID             uuid.UUID
	Competence         valueobjects.Competence
	SubcategoryID      uuid.UUID
	RootSlug           valueobjects.RootSlug
	MutationKind       valueobjects.MutationKind
	CommittedAt        time.Time
	CutoffCompetenceBR valueobjects.Competence
	ExpenseID          uuid.UUID
}

type ExpenseCommittedPublisher interface {
	Publish(ctx context.Context, db database.DBTX, env ExpenseCommittedEnvelope) error
}

func NewExpenseCommittedEnvelope(
	expenseID uuid.UUID,
	userID uuid.UUID,
	subcategoryID uuid.UUID,
	rootSlug valueobjects.RootSlug,
	competence valueobjects.Competence,
	mutationKind valueobjects.MutationKind,
	committedAt time.Time,
	cutoffCompetenceBR valueobjects.Competence,
) ExpenseCommittedEnvelope {
	return ExpenseCommittedEnvelope{
		UserID:             userID,
		Competence:         competence,
		SubcategoryID:      subcategoryID,
		RootSlug:           rootSlug,
		MutationKind:       mutationKind,
		CommittedAt:        committedAt,
		CutoffCompetenceBR: cutoffCompetenceBR,
		ExpenseID:          expenseID,
	}
}
