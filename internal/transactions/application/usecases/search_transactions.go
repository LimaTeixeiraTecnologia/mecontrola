package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

const defaultSearchLimit = 10
const maxSearchLimit = 10

type SearchTransactions struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork
	o11y    observability.Observability
}

func NewSearchTransactions(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *SearchTransactions {
	return &SearchTransactions{
		factory: factory,
		uow:     u,
		o11y:    o11y,
	}
}

func (uc *SearchTransactions) Execute(ctx context.Context, query, refMonth string, limit int) ([]output.Transaction, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.search_transactions")
	defer span.End()

	in := input.SearchTransactions{Query: query, RefMonth: refMonth, Limit: limit}
	if err := in.Validate(); err != nil {
		return nil, err
	}

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return nil, ErrUsecaseUnauthorized
	}

	searchQuery, err := valueobjects.NewSearchQuery(in.Query)
	if err != nil {
		return nil, fmt.Errorf("transactions/search_transactions: query inválida: %w", err)
	}

	refMonthOpt := option.None[valueobjects.RefMonth]()
	if in.RefMonth != "" {
		rm, rmErr := valueobjects.NewRefMonth(in.RefMonth)
		if rmErr != nil {
			return nil, fmt.Errorf("transactions/search_transactions: ref_month inválido: %w", rmErr)
		}
		refMonthOpt = option.Some(rm)
	}

	effectiveLimit := in.Limit
	if effectiveLimit <= 0 {
		effectiveLimit = defaultSearchLimit
	}
	if effectiveLimit > maxSearchLimit {
		effectiveLimit = maxSearchLimit
	}

	result, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) ([]output.Transaction, error) {
		repo := uc.factory.TransactionRepository(db)
		txs, searchErr := repo.SearchByDescription(ctx, principal.UserID, searchQuery, refMonthOpt, effectiveLimit)
		if searchErr != nil {
			return nil, fmt.Errorf("transactions/search_transactions: buscar lançamentos: %w", searchErr)
		}
		items := make([]output.Transaction, 0, len(txs))
		for _, tx := range txs {
			items = append(items, output.TransactionFrom(tx))
		}
		return items, nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return nil, execErr
	}

	return result, nil
}
