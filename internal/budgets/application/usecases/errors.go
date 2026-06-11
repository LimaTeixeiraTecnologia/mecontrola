package usecases

import (
	"errors"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
)

var ErrRecurrenceSourceInvalid = errors.New("budgets: source_competence inválida para recorrência")

var (
	ErrBudgetInvalidUserID                    = commands.ErrCommandInvalidUserID
	ErrBudgetInvalidCompetence                = commands.ErrCommandInvalidCompetence
	ErrBudgetInvalidTotalCents                = commands.ErrCommandInvalidTotalCents
	ErrBudgetInvalidAllocationRootSlug        = commands.ErrCommandInvalidAllocation
	ErrBudgetAllocationBasisPointsInvalid     = commands.ErrCommandInvalidAllocation
	ErrBudgetAllocationSumExceeds10000        = commands.ErrCommandInvalidAllocation
	ErrRecurrenceInvalidMonths                = commands.ErrCommandInvalidMonths
	ErrRecurrenceSourceAutoDraftWithoutAllocs = services.ErrRecurrenceSourceAutoDraftWithoutAllocs
	ErrRecurrenceSourceDraftWithoutFullAllocs = services.ErrRecurrenceSourceDraftWithoutFullAllocs
	ErrRecurrenceSourceNegativeTotal          = services.ErrRecurrenceSourceNegativeTotal
)

var (
	ErrApplyPendingEventInvalidEventID = commands.ErrCommandInvalidEventID

	ErrDeleteExpenseInvalidExternalID = commands.ErrCommandInvalidExternalID
	ErrDeleteExpenseInvalidSource     = commands.ErrCommandInvalidSource
	ErrDeleteExpenseInvalidUserID     = commands.ErrCommandInvalidUserID

	ErrUpsertExpenseExplicitVersion    = commands.ErrCommandExplicitVersion
	ErrUpsertExpenseInvalidAmount      = commands.ErrCommandInvalidAmount
	ErrUpsertExpenseInvalidCompetence  = commands.ErrCommandInvalidCompetence
	ErrUpsertExpenseInvalidExternalID  = commands.ErrCommandInvalidExternalID
	ErrUpsertExpenseInvalidSource      = commands.ErrCommandInvalidSource
	ErrUpsertExpenseInvalidSubcategory = commands.ErrCommandInvalidSubcategory
	ErrUpsertExpenseInvalidUserID      = commands.ErrCommandInvalidUserID
	ErrUpsertExpenseVersionRequired    = commands.ErrCommandVersionRequired
)

var (
	ErrListAlertsInvalidUserID     = commands.ErrCommandInvalidUserID
	ErrGetSummaryInvalidUserID     = commands.ErrCommandInvalidUserID
	ErrGetSummaryInvalidCompetence = commands.ErrCommandInvalidCompetence
)
