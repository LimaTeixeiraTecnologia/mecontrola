package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type DeleteTransaction struct {
	factory   interfaces.RepositoryFactory
	uow       uow.UnitOfWork
	publisher interfaces.TransactionEventPublisher
	o11y      observability.Observability
}

func NewDeleteTransaction(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	publisher interfaces.TransactionEventPublisher,
	o11y observability.Observability,
) *DeleteTransaction {
	return &DeleteTransaction{
		factory:   factory,
		uow:       u,
		publisher: publisher,
		o11y:      o11y,
	}
}

func (uc *DeleteTransaction) Execute(ctx context.Context, txID string, version int64) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.delete_transaction")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return ErrUsecaseUnauthorized
	}

	parsedID, err := uuid.Parse(txID)
	if err != nil {
		return fmt.Errorf("transactions/delete_transaction: transaction_id inválido: %w", err)
	}

	userID := valueobjects.UserIDFromUUID(principal.UserID)
	now := time.Now().UTC()
	eventID := uuid.New()

	_, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (struct{}, error) {
		repo := uc.factory.TransactionRepository(db)

		current, getErr := repo.GetByID(ctx, parsedID, userID.UUID())
		if getErr != nil {
			return struct{}{}, fmt.Errorf("transactions/delete_transaction: buscar lançamento: %w", getErr)
		}

		if softDelErr := repo.SoftDelete(ctx, parsedID, userID.UUID(), version, now); softDelErr != nil {
			return struct{}{}, fmt.Errorf("transactions/delete_transaction: soft-delete: %w", softDelErr)
		}

		evt := entities.TransactionDeleted{
			EventID:           eventID,
			AggregateID:       parsedID,
			UserID:            userID.UUID(),
			OccurredAt:        now,
			RefMonth:          current.RefMonth(),
			RefMonthsAffected: []valueobjects.RefMonth{current.RefMonth()},
		}

		if publishErr := uc.publisher.PublishDeleted(ctx, db, evt); publishErr != nil {
			return struct{}{}, fmt.Errorf("transactions/delete_transaction: publicar evento: %w", publishErr)
		}

		return struct{}{}, nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return execErr
	}

	return nil
}
