package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
)

type GetTransaction struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork
	o11y    observability.Observability
}

func NewGetTransaction(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *GetTransaction {
	return &GetTransaction{
		factory: factory,
		uow:     u,
		o11y:    o11y,
	}
}

func (uc *GetTransaction) Execute(ctx context.Context, txID string) (output.Transaction, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.get_transaction")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return output.Transaction{}, ErrUsecaseUnauthorized
	}

	parsedID, err := uuid.Parse(txID)
	if err != nil {
		return output.Transaction{}, fmt.Errorf("transactions/get_transaction: transaction_id inválido: %w", err)
	}

	result, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (output.Transaction, error) {
		repo := uc.factory.TransactionRepository(db)
		tx, getErr := repo.GetByID(ctx, parsedID, principal.UserID)
		if getErr != nil {
			return output.Transaction{}, fmt.Errorf("transactions/get_transaction: buscar lançamento: %w", getErr)
		}
		return output.TransactionFrom(tx), nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return output.Transaction{}, execErr
	}

	return result, nil
}
