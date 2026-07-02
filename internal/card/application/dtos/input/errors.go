package input

import "errors"

var (
	ErrCardUserIDRequired = errors.New("user_id: obrigatório")
	ErrCardIDRequired     = errors.New("id: obrigatório")
	ErrCardBankRequired   = errors.New("bank: obrigatório")
	ErrCardDueDayInvalid  = errors.New("due_day: deve estar entre 1 e 31")
)
