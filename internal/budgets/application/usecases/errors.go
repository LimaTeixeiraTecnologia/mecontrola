package usecases

import "errors"

var ErrBudgetInvalidUserID = errors.New("budgets: user_id inválido")

var ErrBudgetInvalidCompetence = errors.New("budgets: competence inválida")

var ErrBudgetInvalidTotalCents = errors.New("budgets: total_cents deve ser maior que zero")

var ErrBudgetInvalidAllocationRootSlug = errors.New("budgets: root_slug de alocação inválido")

var ErrBudgetAllocationBasisPointsInvalid = errors.New("budgets: basis_points inválido")

var ErrBudgetAllocationSumExceeds10000 = errors.New("budgets: soma dos basis points não pode exceder 10000")

var ErrRecurrenceInvalidMonths = errors.New("budgets: meses de recorrência deve ser entre 1 e 12")

var ErrRecurrenceSourceInvalid = errors.New("budgets: source_competence inválida para recorrência")

var ErrRecurrenceSourceAutoDraftWithoutAllocs = errors.New("budgets: rascunho automático sem alocações não é fonte válida para recorrência")

var ErrRecurrenceSourceDraftWithoutFullAllocs = errors.New("budgets: rascunho manual com soma diferente de 100% não é fonte válida")

var ErrRecurrenceSourceNegativeTotal = errors.New("budgets: competência sem valor total positivo não é fonte válida")
