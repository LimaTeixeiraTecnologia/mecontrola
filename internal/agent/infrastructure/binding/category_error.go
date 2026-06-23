package binding

import (
	"errors"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	transactionsvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func translateCategoryError(err error) error {
	if err == nil {
		return nil
	}
	var ambiguous *usecases.CategoryAmbiguousError
	if errors.As(err, &ambiguous) {
		return &tools.CategoryAmbiguousError{Hint: ambiguous.Hint, Candidates: ambiguous.Candidates}
	}
	var needsConfirmation *usecases.CategoryNeedsConfirmationError
	if errors.As(err, &needsConfirmation) {
		return &tools.CategoryNeedsConfirmationError{Hint: needsConfirmation.Hint, Candidates: needsConfirmation.Candidates}
	}
	if errors.Is(err, usecases.ErrLogTransactionCategoryNotFound) {
		return errors.Join(tools.ErrCategoryNotFound, err)
	}
	if errors.Is(err, usecases.ErrLogTransactionNoCategoryHint) {
		return errors.Join(tools.ErrCategoryHintMissing, err)
	}
	return err
}

func translateRecurringError(err error) error {
	if err == nil {
		return nil
	}
	if isCategoryError(err) {
		return translateCategoryError(err)
	}
	if errors.Is(err, transactionsvo.ErrDayOfMonthOutOfRange) {
		return errors.Join(tools.ErrRecurringInvalidDay, err)
	}
	return err
}

func isCategoryError(err error) bool {
	var ambiguous *usecases.CategoryAmbiguousError
	if errors.As(err, &ambiguous) {
		return true
	}
	var needsConfirmation *usecases.CategoryNeedsConfirmationError
	if errors.As(err, &needsConfirmation) {
		return true
	}
	return errors.Is(err, usecases.ErrLogTransactionCategoryNotFound) ||
		errors.Is(err, usecases.ErrLogTransactionNoCategoryHint)
}
