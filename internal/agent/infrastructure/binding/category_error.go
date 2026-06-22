package binding

import (
	"errors"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	transactionsvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func translateCategoryError(err error) error {
	if err == nil {
		return nil
	}
	var ambiguous *usecases.CategoryAmbiguousError
	if errors.As(err, &ambiguous) {
		return &appservices.CategoryAmbiguousError{Hint: ambiguous.Hint, Candidates: ambiguous.Candidates}
	}
	var needsConfirmation *usecases.CategoryNeedsConfirmationError
	if errors.As(err, &needsConfirmation) {
		return &appservices.CategoryNeedsConfirmationError{Hint: needsConfirmation.Hint, Candidates: needsConfirmation.Candidates}
	}
	if errors.Is(err, usecases.ErrLogTransactionCategoryNotFound) {
		return errors.Join(appservices.ErrCategoryNotFound, err)
	}
	if errors.Is(err, usecases.ErrLogTransactionNoCategoryHint) {
		return errors.Join(appservices.ErrCategoryHintMissing, err)
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
		return errors.Join(appservices.ErrRecurringInvalidDay, err)
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
