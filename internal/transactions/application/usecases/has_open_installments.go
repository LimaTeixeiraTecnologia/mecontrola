package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
)

type HasOpenInstallments struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork
	o11y    observability.Observability
}

func NewHasOpenInstallments(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *HasOpenInstallments {
	return &HasOpenInstallments{
		factory: factory,
		uow:     u,
		o11y:    o11y,
	}
}

func (uc *HasOpenInstallments) Execute(ctx context.Context, cardID, userID uuid.UUID) (bool, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.has_open_installments")
	defer span.End()

	result, err := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (bool, error) {
		repo := uc.factory.TransactionRepository(db)
		exists, checkErr := repo.ExistsActiveCreditByCard(ctx, cardID, userID)
		if checkErr != nil {
			return false, fmt.Errorf("transactions/has_open_installments: verificar parcelas: %w", checkErr)
		}
		return exists, nil
	})
	if err != nil {
		span.RecordError(err)
		return false, err
	}
	return result, nil
}
