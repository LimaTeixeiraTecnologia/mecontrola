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
