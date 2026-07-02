package domain

import "errors"

var (
	ErrCardNotFound        = errors.New("card: not found")
	ErrNicknameConflict    = errors.New("card: nickname already in use")
	ErrInvalidClosingDay   = errors.New("card: closing_day must be between 1 and 31")
	ErrInvalidDueDay       = errors.New("card: due_day must be between 1 and 31")
	ErrInvalidNickname     = errors.New("card: nickname must be between 1 and 32 characters")
	ErrInvalidPurchaseDate = errors.New("card: purchase date is zero or invalid")
	ErrInvalidCursor       = errors.New("card: invalid pagination cursor")
	ErrInvalidBank         = errors.New("card: bank must be a non-empty text")
)
