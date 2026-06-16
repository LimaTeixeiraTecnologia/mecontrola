package usecases

import (
	"errors"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

func classifyCardOutcome(err error) string {
	switch {
	case errors.Is(err, domain.ErrCardNotFound):
		return "not_found"
	case errors.Is(err, domain.ErrNicknameConflict):
		return "conflict"
	case errors.Is(err, domain.ErrCardLimitConflict):
		return "conflict"
	case isCardValidationError(err):
		return "invalid"
	default:
		return "internal_error"
	}
}

func isCardValidationError(err error) bool {
	return errors.Is(err, domain.ErrInvalidCardName) ||
		errors.Is(err, domain.ErrInvalidNickname) ||
		errors.Is(err, domain.ErrInvalidClosingDay) ||
		errors.Is(err, domain.ErrInvalidDueDay) ||
		errors.Is(err, domain.ErrInvalidPurchaseDate) ||
		errors.Is(err, domain.ErrInvalidCursor) ||
		errors.Is(err, domain.ErrCardLimitNegative) ||
		errors.Is(err, domain.ErrCardLimitTooLarge)
}
