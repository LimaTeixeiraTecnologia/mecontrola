package tools

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
)

type entryRegistrar interface {
	RegisterExpense(ctx context.Context, cmd usecases.RegisterExpenseCommand) (usecases.RegisterResult, error)
	RegisterIncome(ctx context.Context, cmd usecases.RegisterIncomeCommand) (usecases.RegisterResult, error)
}

type recurrenceRegistrar interface {
	CreateRecurrence(ctx context.Context, cmd usecases.CreateRecurrenceCommand) (usecases.RegisterResult, error)
}

type entryEditor interface {
	EditEntry(ctx context.Context, cmd usecases.EditEntryCommand) (usecases.RegisterResult, error)
}
