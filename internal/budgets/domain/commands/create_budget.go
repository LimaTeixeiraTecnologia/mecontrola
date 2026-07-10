package commands

import (
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type AllocationCommandInput struct {
	RootSlug    string
	BasisPoints int
}

type AllocationCommandItem struct {
	RootSlug    valueobjects.RootSlug
	BasisPoints valueobjects.BasisPoints
}

type CreateBudgetCommand struct {
	UserID      uuid.UUID
	Competence  valueobjects.Competence
	TotalCents  int64
	Allocations []AllocationCommandItem
}

func NewCreateBudgetCommand(
	userID string,
	competence string,
	totalCents int64,
	allocations []AllocationCommandInput,
) (CreateBudgetCommand, error) {
	var errs []error

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		errs = append(errs, ErrCommandInvalidUserID)
	}

	parsedCompetence, err := valueobjects.NewCompetence(competence)
	if err != nil {
		errs = append(errs, ErrCommandInvalidCompetence)
	}

	if totalCents <= 0 {
		errs = append(errs, ErrCommandInvalidTotalCents)
	}

	parsedAllocations := make([]AllocationCommandItem, 0, len(allocations))
	sumBP := 0
	for _, a := range allocations {
		slug, slugErr := valueobjects.ParseRootSlug(a.RootSlug)
		if slugErr != nil {
			errs = append(errs, ErrCommandInvalidAllocation)
			continue
		}
		bp, bpErr := valueobjects.NewBasisPoints(a.BasisPoints)
		if bpErr != nil {
			errs = append(errs, ErrCommandInvalidAllocation)
			continue
		}
		sumBP += a.BasisPoints
		parsedAllocations = append(parsedAllocations, AllocationCommandItem{
			RootSlug:    slug,
			BasisPoints: bp,
		})
	}
	if sumBP != 10000 {
		errs = append(errs, ErrCommandInvalidAllocation)
	}

	if len(errs) > 0 {
		return CreateBudgetCommand{}, errors.Join(errs...)
	}

	return CreateBudgetCommand{
		UserID:      parsedUserID,
		Competence:  parsedCompetence,
		TotalCents:  totalCents,
		Allocations: parsedAllocations,
	}, nil
}
