package input

import "errors"

var (
	ErrInputAmountCentsRequired    = errors.New("amount_cents: deve ser maior que zero")
	ErrInputDescriptionRequired    = errors.New("description: não pode ser vazio")
	ErrInputDirectionRequired      = errors.New("direction: não pode ser vazio")
	ErrInputPaymentMethodRequired  = errors.New("payment_method: não pode ser vazio")
	ErrInputOccurredAtRequired     = errors.New("occurred_at: não pode ser vazio")
	ErrInputStartedAtRequired      = errors.New("started_at: não pode ser vazio")
	ErrInputFrequencyRequired      = errors.New("frequency: não pode ser vazio")
	ErrInputDayOfMonthOutOfRange   = errors.New("day_of_month: deve estar entre 1 e 28")
	ErrInputInstallmentsOutOfRange = errors.New("installments: deve estar entre 1 e 24")
	ErrInputVersionRequired        = errors.New("version: deve ser maior que zero")
	ErrInputCategoryIDRequired     = errors.New("category_id: obrigatório")
	ErrInputCardIDRequired         = errors.New("card_id: obrigatório")
)
