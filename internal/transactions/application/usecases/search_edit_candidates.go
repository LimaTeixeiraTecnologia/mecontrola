package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

const defaultEditCandidatesLimit = 5
const maxEditCandidatesLimit = 5

type SearchEditCandidates struct {
	factory   interfaces.RepositoryFactory
	uow       uow.UnitOfWork
	brazilLoc *time.Location
	o11y      observability.Observability
}

func NewSearchEditCandidates(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	brazilLoc *time.Location,
	o11y observability.Observability,
) *SearchEditCandidates {
	return &SearchEditCandidates{
		factory:   factory,
		uow:       u,
		brazilLoc: brazilLoc,
		o11y:      o11y,
	}
}

func (uc *SearchEditCandidates) Execute(ctx context.Context, amountCents int64, term, refMonth string, limit int) ([]output.Transaction, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.search_edit_candidates")
	defer span.End()

	in := input.SearchEditCandidates{AmountCents: amountCents, Term: term, RefMonth: refMonth, Limit: limit}
	if err := in.Validate(); err != nil {
		return nil, err
	}

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return nil, ErrUsecaseUnauthorized
	}

	effectiveRefMonth := valueobjects.RefMonth{}
	if in.RefMonth != "" {
		rm, rmErr := valueobjects.NewRefMonth(in.RefMonth)
		if rmErr != nil {
			return nil, fmt.Errorf("transactions/search_edit_candidates: ref_month inválido: %w", rmErr)
		}
		effectiveRefMonth = rm
	} else {
		effectiveRefMonth = valueobjects.RefMonthFromTime(time.Now().UTC(), uc.brazilLoc)
	}

	effectiveLimit := in.Limit
	if effectiveLimit <= 0 {
		effectiveLimit = defaultEditCandidatesLimit
	}
	if effectiveLimit > maxEditCandidatesLimit {
		effectiveLimit = maxEditCandidatesLimit
	}

	result, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) ([]output.Transaction, error) {
		repo := uc.factory.TransactionRepository(db)
		txs, searchErr := repo.SearchEditCandidates(ctx, principal.UserID, in.AmountCents, in.Term, effectiveRefMonth, effectiveLimit)
		if searchErr != nil {
			return nil, fmt.Errorf("transactions/search_edit_candidates: buscar candidatos de edição: %w", searchErr)
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
