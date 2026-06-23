package input

import "errors"

var (
	ErrInputInvalidUserID      = errors.New("user_id: UUID inválido")
	ErrInputInvalidCompetence  = errors.New("competence: formato inválido (esperado YYYY-MM)")
	ErrInputInvalidTotalCents  = errors.New("total_cents: deve ser maior que zero")
	ErrInputAllocationsEmpty   = errors.New("allocations: não pode ser vazio")
	ErrInputInvalidSource      = errors.New("source: não pode ser vazio")
	ErrInputInvalidExternalID  = errors.New("external_transaction_id: formato inválido")
	ErrInputInvalidSubcategory = errors.New("subcategory_id: UUID inválido")
	ErrInputAmountCentsInvalid = errors.New("amount_cents: deve ser maior que zero")
	ErrInputMonthsOutOfRange   = errors.New("months: deve estar entre 1 e 12")
	ErrInputExpectedVersion    = errors.New("expected_version: deve ser maior que zero quando fornecido")
	ErrInputInvalidRootSlug    = errors.New("root_slug: não pode ser vazio")
	ErrInputPercentageRange    = errors.New("percentage: deve estar entre 0 e 100")
)
