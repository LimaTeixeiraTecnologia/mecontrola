package domain

import "errors"

var (
	ErrCardNotFound        = errors.New("card: not found")
	ErrNicknameConflict    = errors.New("card: nickname already in use")
	ErrInvalidClosingDay   = errors.New("card: closing_day must be between 1 and 31")
	ErrInvalidDueDay       = errors.New("card: due_day must be between 1 and 31")
	ErrInvalidCardName     = errors.New("card: name must be between 1 and 64 characters")
	ErrInvalidNickname     = errors.New("card: nickname must be between 1 and 32 characters")
	ErrInvalidPurchaseDate = errors.New("card: purchase date is zero or invalid")
	ErrInvalidCursor       = errors.New("card: invalid pagination cursor")
	ErrCardLimitNegative   = errors.New("card: limit_cents must be greater than or equal to zero")
	ErrCardLimitTooLarge   = errors.New("card: limit_cents exceeds maximum allowed (R$ 1.000.000,00)")
	ErrCardLimitConflict   = errors.New("card: limit update version conflict")
)
