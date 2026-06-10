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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrDeleteExpenseInvalidUserID = errors.New("budgets: user_id inválido para exclusão de despesa")

var ErrDeleteExpenseInvalidSource = errors.New("budgets: source inválido para exclusão de despesa")

var ErrDeleteExpenseInvalidExternalID = errors.New("budgets: external_transaction_id inválido para exclusão")

type DeleteExpense struct {
	expenses  interfaces.ExpenseRepository
	publisher interfaces.ExpenseCommittedPublisher
	uow       uow.UnitOfWork[struct{}]
	o11y      observability.Observability
	loc       *time.Location
}

func NewDeleteExpense(
	expenses interfaces.ExpenseRepository,
	publisher interfaces.ExpenseCommittedPublisher,
	u uow.UnitOfWork[struct{}],
	o11y observability.Observability,
	loc *time.Location,
) *DeleteExpense {
	return &DeleteExpense{
		expenses:  expenses,
		publisher: publisher,
		uow:       u,
		o11y:      o11y,
		loc:       loc,
	}
}

func (uc *DeleteExpense) Execute(ctx context.Context, in input.DeleteExpenseInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.delete_expense")
	defer span.End()

	resolved, resErr := uc.resolveInput(in)
	if resErr != nil {
		return resErr
	}

	_, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		return struct{}{}, uc.ExecuteInTx(ctx, tx, resolved)
	})

	if execErr != nil {
		span.RecordError(execErr)
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.delete_expense.failed",
			observability.String("user_id", in.UserID),
			observability.String("source", in.Source),
			observability.String("external_transaction_id", in.ExternalTransactionID),
			observability.Error(execErr),
		)
		return execErr
	}

	return nil
}

func (uc *DeleteExpense) ExecuteWithTx(ctx context.Context, tx database.DBTX, in input.DeleteExpenseInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.delete_expense.with_tx")
	defer span.End()

	resolved, resErr := uc.resolveInput(in)
	if resErr != nil {
		return resErr
	}

	if execErr := uc.ExecuteInTx(ctx, tx, resolved); execErr != nil {
		span.RecordError(execErr)
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.delete_expense.failed",
			observability.String("user_id", in.UserID),
			observability.String("source", in.Source),
			observability.String("external_transaction_id", in.ExternalTransactionID),
			observability.Error(execErr),
		)
		return execErr
	}

	return nil
}

type deleteExpenseResolved struct {
	userID          uuid.UUID
	source          valueobjects.ProducerSource
	extID           valueobjects.ExternalTransactionID
	expectedVersion int64
}

func (uc *DeleteExpense) resolveInput(in input.DeleteExpenseInput) (deleteExpenseResolved, error) {
	userID, err := uuid.Parse(in.UserID)
	if err != nil {
		return deleteExpenseResolved{}, ErrDeleteExpenseInvalidUserID
	}

	source, err := valueobjects.NewProducerSource(in.Source)
	if err != nil {
		return deleteExpenseResolved{}, ErrDeleteExpenseInvalidSource
	}

	extID, err := valueobjects.NewExternalTransactionID(in.ExternalTransactionID)
	if err != nil {
		return deleteExpenseResolved{}, ErrDeleteExpenseInvalidExternalID
	}

	return deleteExpenseResolved{
		userID:          userID,
		source:          source,
		extID:           extID,
		expectedVersion: in.ExpectedVersion,
	}, nil
}

func (uc *DeleteExpense) ExecuteInTx(ctx context.Context, tx database.DBTX, r deleteExpenseResolved) error {
	identity := entities.ExpenseIdentity{
		UserID:                r.userID,
		Source:                r.source,
		ExternalTransactionID: r.extID,
	}

	existing, tombstone, getErr := uc.expenses.GetByIdentity(ctx, tx, identity)
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
	committedAt := now
	cutoff := valueobjects.CompetenceFromTime(now, uc.loc)

	if _, softDeleteErr := uc.expenses.SoftDelete(ctx, tx, existing, r.expectedVersion); softDeleteErr != nil {
		if errors.Is(softDeleteErr, interfaces.ErrExpenseConflict) {
			return interfaces.ErrExpenseConflict
		}
		return fmt.Errorf("budgets.usecase.delete_expense: soft delete: %w", softDeleteErr)
	}

	env := interfaces.ExpenseCommittedEnvelope{
		UserID:             r.userID,
		Competence:         existing.Competence(),
		SubcategoryID:      existing.SubcategoryID(),
		RootSlug:           existing.RootSlug(),
		MutationKind:       valueobjects.MutationKindDelete,
		CommittedAt:        committedAt,
		CutoffCompetenceBR: cutoff,
		ExpenseID:          existing.ID(),
	}
	if pubErr := uc.publisher.Publish(ctx, tx, env); pubErr != nil {
		return fmt.Errorf("budgets.usecase.delete_expense: publicar evento: %w", pubErr)
	}

	return nil
}
