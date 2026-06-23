package input

import "errors"

var (
	ErrCardUserIDRequired    = errors.New("user_id: obrigatório")
	ErrCardIDRequired        = errors.New("id: obrigatório")
	ErrCardIDCardRequired    = errors.New("card_id: obrigatório")
	ErrCardNameRequired      = errors.New("name: obrigatório")
	ErrCardClosingDayInvalid = errors.New("closing_day: deve ser maior que zero")
	ErrCardDueDayInvalid     = errors.New("due_day: deve ser maior que zero")
	ErrCardLimitCentsInvalid = errors.New("limit_cents: deve ser maior ou igual a zero")
)
