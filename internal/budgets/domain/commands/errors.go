package commands

import "errors"

var ErrCommandInvalidUserID = errors.New("budgets.command: user_id inválido")

var ErrCommandInvalidSubcategory = errors.New("budgets.command: subcategory_id inválido")

var ErrCommandInvalidCompetence = errors.New("budgets.command: competence inválida")

var ErrCommandInvalidSource = errors.New("budgets.command: source inválido")

var ErrCommandInvalidExternalID = errors.New("budgets.command: external_transaction_id inválido")

var ErrCommandInvalidAmount = errors.New("budgets.command: amount_cents deve ser maior que zero")

var ErrCommandExplicitVersion = errors.New("budgets.command: version não deve ser fornecido na criação")

var ErrCommandVersionRequired = errors.New("budgets.command: expected_version obrigatório para edição")

var ErrCommandInvalidEventID = errors.New("budgets.command: event_id inválido")

var ErrCommandInvalidMutationKind = errors.New("budgets.command: mutation_kind inválido")

var ErrCommandInvalidTotalCents = errors.New("budgets.command: total_cents deve ser maior que zero")

var ErrCommandInvalidAllocation = errors.New("budgets.command: alocação inválida")

var ErrCommandInvalidCursor = errors.New("budgets.command: cursor inválido")

var ErrCommandInvalidLimit = errors.New("budgets.command: limit inválido")

var ErrCommandInvalidMonths = errors.New("budgets.command: months fora do intervalo permitido")

var ErrCommandInvalidOccurredAt = errors.New("budgets.command: occurred_at obrigatório")
