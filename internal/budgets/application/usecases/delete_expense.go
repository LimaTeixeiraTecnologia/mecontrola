package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type DeleteExpense struct {
	factory   interfaces.RepositoryFactory
	publisher interfaces.ExpenseCommittedPublisher
	uow       uow.UnitOfWork[struct{}]
	o11y      observability.Observability
	loc       *time.Location
}

func NewDeleteExpense(
	factory interfaces.RepositoryFactory,
	publisher interfaces.ExpenseCommittedPublisher,
	u uow.UnitOfWork[struct{}],
	o11y observability.Observability,
	loc *time.Location,
) *DeleteExpense {
	return &DeleteExpense{
		factory:   factory,
		publisher: publisher,
		uow:       u,
		o11y:      o11y,
		loc:       loc,
	}
}

func (uc *DeleteExpense) Execute(ctx context.Context, in input.DeleteExpenseInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.delete_expense")
	defer span.End()

	cmd, err := commands.NewDeleteExpenseCommand(in.UserID, in.Source, in.ExternalTransactionID, in.ExpectedVersion)
	if err != nil {
		return err
	}

	_, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		return struct{}{}, uc.executeInTx(ctx, tx, cmd)
	})
	if execErr != nil {
		uc.logFailure(ctx, span, in, execErr)
		return execErr
	}

	return nil
}

func (uc *DeleteExpense) ExecuteWithTx(ctx context.Context, tx database.DBTX, in input.DeleteExpenseInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.delete_expense.with_tx")
	defer span.End()

	cmd, err := commands.NewDeleteExpenseCommand(in.UserID, in.Source, in.ExternalTransactionID, in.ExpectedVersion)
	if err != nil {
		return err
	}

	if execErr := uc.executeInTx(ctx, tx, cmd); execErr != nil {
		uc.logFailure(ctx, span, in, execErr)
		return execErr
	}

	return nil
}

func (uc *DeleteExpense) ExecuteByExternalID(ctx context.Context, userIDStr, source, externalTransactionID string) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.delete_expense.by_external_id")
	defer span.End()

	userID, parseErr := uuid.Parse(userIDStr)
	if parseErr != nil {
		span.RecordError(parseErr)
		return ErrDeleteExpenseInvalidUserID
	}

	prodSource, srcErr := valueobjects.NewProducerSource(source)
	if srcErr != nil {
		span.RecordError(srcErr)
		return ErrDeleteExpenseInvalidSource
	}

	extID, extErr := valueobjects.NewExternalTransactionID(externalTransactionID)
	if extErr != nil {
		span.RecordError(extErr)
		return fmt.Errorf("budgets.usecase.delete_expense.by_external_id: extID inválido: %w", extErr)
	}

	_, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		return struct{}{}, uc.deleteByIdentityInTx(ctx, tx, userID, prodSource, extID)
	})
	if execErr != nil {
		span.RecordError(execErr)
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.delete_expense.by_external_id.failed",
			observability.String("user_id", userIDStr),
			observability.String("source", source),
			observability.String("external_transaction_id", externalTransactionID),
			observability.Error(execErr),
		)
		return execErr
	}

	return nil
}

func (uc *DeleteExpense) deleteByIdentityInTx(
	ctx context.Context, tx database.DBTX,
	userID uuid.UUID, source valueobjects.ProducerSource, extID valueobjects.ExternalTransactionID,
) error {
	expenses := uc.factory.ExpenseRepository(tx)
	identity := entities.ExpenseIdentity{UserID: userID, Source: source, ExternalTransactionID: extID}

	existing, tombstone, getErr := expenses.GetByIdentity(ctx, identity)
	if getErr != nil {
		if errors.Is(getErr, interfaces.ErrExpenseNotFound) {
			return nil
		}
		return fmt.Errorf("budgets.usecase.delete_expense.by_external_id: ler despesa: %w", getErr)
	}
	if tombstone.IsPresent() || existing.IsDeleted() {
		return nil
	}

	now := time.Now().UTC()
	cutoff := valueobjects.CompetenceFromTime(now, uc.loc)
	currentVersion := existing.Version()

	if _, entityErr := existing.SoftDelete(currentVersion, now); entityErr != nil {
		return fmt.Errorf("budgets.usecase.delete_expense.by_external_id: entity soft delete: %w", entityErr)
	}

	if _, repoErr := expenses.SoftDelete(ctx, existing, currentVersion); repoErr != nil {
		if errors.Is(repoErr, interfaces.ErrExpenseConflict) {
			return interfaces.ErrExpenseConflict
		}
		return fmt.Errorf("budgets.usecase.delete_expense.by_external_id: repo soft delete: %w", repoErr)
	}

	evt, evtErr := events.NewExpenseCommitted(
		existing.ID(), existing.UserID(), existing.SubcategoryID(), existing.RootSlug(), existing.Competence(),
		valueobjects.MutationKindDelete, now, cutoff,
	)
	if evtErr != nil {
		return fmt.Errorf("budgets.usecase.delete_expense.by_external_id: construir evento: %w", evtErr)
	}
	if pubErr := uc.publisher.Publish(ctx, tx, evt); pubErr != nil {
		return fmt.Errorf("budgets.usecase.delete_expense.by_external_id: publicar evento: %w", pubErr)
	}

	return nil
}

func (uc *DeleteExpense) executeInTx(ctx context.Context, tx database.DBTX, cmd commands.DeleteExpenseCommand) error {
	expenses := uc.factory.ExpenseRepository(tx)
	identity := entities.ExpenseIdentity{
		UserID:                cmd.UserID,
		Source:                cmd.Source,
		ExternalTransactionID: cmd.ExtID,
	}

	existing, tombstone, getErr := expenses.GetByIdentity(ctx, identity)
	if getErr != nil {
		if errors.Is(getErr, interfaces.ErrExpenseNotFound) {
			return interfaces.ErrExpenseNotFound
		}
		return fmt.Errorf("budgets.usecase.delete_expense: ler despesa: %w", getErr)
	}

	if tombstone.IsPresent() || existing.IsDeleted() {
		return nil
	}

	now := time.Now().UTC()
	cutoff := valueobjects.CompetenceFromTime(now, uc.loc)

	if _, entityErr := existing.SoftDelete(cmd.ExpectedVersion, now); entityErr != nil {
		if errors.Is(entityErr, entities.ErrExpenseVersionMismatch) {
			return interfaces.ErrExpenseConflict
		}
		return fmt.Errorf("budgets.usecase.delete_expense: entity soft delete: %w", entityErr)
	}

	if _, softDeleteErr := expenses.SoftDelete(ctx, existing, cmd.ExpectedVersion); softDeleteErr != nil {
		if errors.Is(softDeleteErr, interfaces.ErrExpenseConflict) {
			return interfaces.ErrExpenseConflict
		}
		return fmt.Errorf("budgets.usecase.delete_expense: soft delete: %w", softDeleteErr)
	}

	evt, evtErr := events.NewExpenseCommitted(
		existing.ID(), cmd.UserID, existing.SubcategoryID(), existing.RootSlug(), existing.Competence(),
		valueobjects.MutationKindDelete, now, cutoff,
	)
	if evtErr != nil {
		return fmt.Errorf("budgets.usecase.delete_expense: construir evento: %w", evtErr)
	}
	if pubErr := uc.publisher.Publish(ctx, tx, evt); pubErr != nil {
		return fmt.Errorf("budgets.usecase.delete_expense: publicar evento: %w", pubErr)
	}

	return nil
}

func (uc *DeleteExpense) logFailure(ctx context.Context, span observability.Span, in input.DeleteExpenseInput, err error) {
	span.RecordError(err)
	uc.o11y.Logger().Warn(ctx, "budgets.usecase.delete_expense.failed",
		observability.String("user_id", in.UserID),
		observability.String("source", in.Source),
		observability.String("external_transaction_id", in.ExternalTransactionID),
		observability.Error(err),
	)
}
