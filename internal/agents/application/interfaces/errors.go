package interfaces

import "errors"

var (
	ErrTransactionsLedgerUnavailable = errors.New("agents: transactions ledger indisponível")
	ErrBudgetPlannerUnavailable      = errors.New("agents: budget planner indisponível")
	ErrCardManagerUnavailable        = errors.New("agents: card manager indisponível")
	ErrCategoriesReaderUnavailable   = errors.New("agents: categories reader indisponível")
	ErrBudgetNotFound                = errors.New("agents: orçamento não encontrado")
	ErrBudgetAlreadyActive           = errors.New("agents: orçamento já está ativo")
	ErrCardNotFound                  = errors.New("agents: cartão não encontrado")
)
