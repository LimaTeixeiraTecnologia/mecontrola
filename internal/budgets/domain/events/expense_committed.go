package events

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrExpenseCommittedInvalid = errors.New("budgets: evento expense_committed inválido")

type ExpenseCommitted struct {
	ExpenseID          uuid.UUID
	UserID             uuid.UUID
	SubcategoryID      uuid.UUID
	RootSlug           valueobjects.RootSlug
	Competence         valueobjects.Competence
	MutationKind       valueobjects.MutationKind
	CommittedAt        time.Time
	CutoffCompetenceBR valueobjects.Competence
}

func NewExpenseCommitted(
	expenseID uuid.UUID,
	userID uuid.UUID,
	subcategoryID uuid.UUID,
	rootSlug valueobjects.RootSlug,
	competence valueobjects.Competence,
	mutationKind valueobjects.MutationKind,
	committedAt time.Time,
	cutoffCompetenceBR valueobjects.Competence,
) (ExpenseCommitted, error) {
	var errs []error
	if expenseID == uuid.Nil {
		errs = append(errs, fmt.Errorf("expense_id: %w", ErrExpenseCommittedInvalid))
	}
	if userID == uuid.Nil {
		errs = append(errs, fmt.Errorf("user_id: %w", ErrExpenseCommittedInvalid))
	}
	if subcategoryID == uuid.Nil {
		errs = append(errs, fmt.Errorf("subcategory_id: %w", ErrExpenseCommittedInvalid))
	}
	if mutationKind.String() == "" {
		errs = append(errs, fmt.Errorf("mutation_kind: %w", ErrExpenseCommittedInvalid))
	}
	if committedAt.IsZero() {
		errs = append(errs, fmt.Errorf("committed_at: %w", ErrExpenseCommittedInvalid))
	}
	if competence.IsZero() {
		errs = append(errs, fmt.Errorf("competence: %w", ErrExpenseCommittedInvalid))
	}
	if cutoffCompetenceBR.IsZero() {
		errs = append(errs, fmt.Errorf("cutoff_competence_br: %w", ErrExpenseCommittedInvalid))
	}
	if len(errs) > 0 {
		return ExpenseCommitted{}, errors.Join(errs...)
	}
	return ExpenseCommitted{
		ExpenseID:          expenseID,
		UserID:             userID,
		SubcategoryID:      subcategoryID,
		RootSlug:           rootSlug,
		Competence:         competence,
		MutationKind:       mutationKind,
		CommittedAt:        committedAt,
		CutoffCompetenceBR: cutoffCompetenceBR,
	}, nil
}
