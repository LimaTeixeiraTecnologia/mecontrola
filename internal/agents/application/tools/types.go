package tools

import (
	"context"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
)

type idempotentWriter interface {
	Execute(ctx context.Context, userID uuid.UUID, wamid string, itemSeq int, operation, resourceKind string, write usecases.WriteFn) (usecases.IdempotentWriteResult, error)
}

type entryRegistrar interface {
	RegisterExpense(ctx context.Context, cmd usecases.RegisterExpenseCommand) (usecases.RegisterResult, error)
	RegisterIncome(ctx context.Context, cmd usecases.RegisterIncomeCommand) (usecases.RegisterResult, error)
}
