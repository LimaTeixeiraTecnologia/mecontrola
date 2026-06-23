package commands

import (
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type EditCategoryPercentageCommand struct {
	UserID      uuid.UUID
	Competence  valueobjects.Competence
	RootSlug    valueobjects.RootSlug
	BasisPoints valueobjects.BasisPoints
}

func NewEditCategoryPercentageCommand(userID string, competence string, rootSlug string, percentage int) (EditCategoryPercentageCommand, error) {
	var errs []error

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		errs = append(errs, ErrCommandInvalidUserID)
	}

	parsedCompetence, err := valueobjects.NewCompetence(competence)
	if err != nil {
		errs = append(errs, ErrCommandInvalidCompetence)
	}

	parsedSlug, err := valueobjects.ParseRootSlug(rootSlug)
	if err != nil {
		errs = append(errs, ErrCommandInvalidAllocation)
	}

	basisPoints, err := valueobjects.NewBasisPoints(percentage * 100)
	if err != nil {
		errs = append(errs, ErrCommandInvalidAllocation)
	}

	if len(errs) > 0 {
		return EditCategoryPercentageCommand{}, errors.Join(errs...)
	}

	return EditCategoryPercentageCommand{
		UserID:      parsedUserID,
		Competence:  parsedCompetence,
		RootSlug:    parsedSlug,
		BasisPoints: basisPoints,
	}, nil
}
