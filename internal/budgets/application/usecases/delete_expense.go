package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
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

	if _, softDeleteErr := expenses.SoftDelete(ctx, existing, cmd.ExpectedVersion); softDeleteErr != nil {
		if errors.Is(softDeleteErr, interfaces.ErrExpenseConflict) {
			return interfaces.ErrExpenseConflict
		}
		return fmt.Errorf("budgets.usecase.delete_expense: soft delete: %w", softDeleteErr)
	}

	envelope := interfaces.NewExpenseCommittedEnvelope(
		existing.ID(), cmd.UserID, existing.SubcategoryID(), existing.RootSlug(), existing.Competence(),
		valueobjects.MutationKindDelete, now, cutoff,
	)
	if pubErr := uc.publisher.Publish(ctx, tx, envelope); pubErr != nil {
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
