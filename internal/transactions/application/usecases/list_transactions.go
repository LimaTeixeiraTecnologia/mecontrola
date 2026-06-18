package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

const defaultListLimit = 50
const maxListLimit = 200

type TransactionPage struct {
	Transactions []output.Transaction
	NextCursor   string
}

type ListTransactions struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork
	o11y    observability.Observability
}

func NewListTransactions(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *ListTransactions {
	return &ListTransactions{
		factory: factory,
		uow:     u,
		o11y:    o11y,
	}
}

func (uc *ListTransactions) Execute(ctx context.Context, refMonthStr, cursor string, limit int) (TransactionPage, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.list_transactions")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return TransactionPage{}, ErrUsecaseUnauthorized
	}

	refMonth, err := valueobjects.NewRefMonth(refMonthStr)
	if err != nil {
		return TransactionPage{}, fmt.Errorf("transactions/list_transactions: ref_month inválido: %w", err)
	}

	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	result, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (TransactionPage, error) {
		repo := uc.factory.TransactionRepository(db)
		txs, nextCursor, listErr := repo.ListByMonth(ctx, principal.UserID, refMonth, interfaces.Cursor{Value: cursor}, limit)
		if listErr != nil {
			return TransactionPage{}, fmt.Errorf("transactions/list_transactions: listar lançamentos: %w", listErr)
		}

		items := make([]output.Transaction, 0, len(txs))
		for _, tx := range txs {
			items = append(items, output.TransactionFrom(tx))
		}

		return TransactionPage{
			Transactions: items,
			NextCursor:   nextCursor.Value,
		}, nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return TransactionPage{}, execErr
	}

	return result, nil
}
