package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/mappers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type UpsertExpense struct {
	factory    interfaces.RepositoryFactory
	categories interfaces.CategoriesReader
	publisher  interfaces.ExpenseCommittedPublisher
	autoDraft  *CreateOrAutoDraftForExpense
	uow        uow.UnitOfWork
	o11y       observability.Observability
	loc        *time.Location
}

func NewUpsertExpense(
	factory interfaces.RepositoryFactory,
	categories interfaces.CategoriesReader,
	publisher interfaces.ExpenseCommittedPublisher,
	autoDraft *CreateOrAutoDraftForExpense,
	u uow.UnitOfWork,
	o11y observability.Observability,
	loc *time.Location,
) *UpsertExpense {
	return &UpsertExpense{
		factory:    factory,
		categories: categories,
		publisher:  publisher,
		autoDraft:  autoDraft,
		uow:        u,
		o11y:       o11y,
		loc:        loc,
	}
}

func (uc *UpsertExpense) Execute(ctx context.Context, in input.UpsertExpenseInput) (output.ExpenseOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.upsert_expense")
	defer span.End()

	if err := in.Validate(); err != nil {
		return output.ExpenseOutput{}, err
	}

	cmd, err := commands.NewUpsertExpenseCommand(
		in.UserID, in.SubcategoryID, in.Source, in.ExternalTransactionID,
		in.Competence, in.AmountCents, in.OccurredAt, in.ExpectedVersion, in.Reconcile,
	)
	if err != nil {
		return output.ExpenseOutput{}, err
	}

	rootSlug, err := uc.resolveRootSlug(ctx, cmd.SubcategoryID)
	if err != nil {
		return output.ExpenseOutput{}, err
	}

	expense, err := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (entities.Expense, error) {
		return uc.executeInTx(ctx, tx, cmd, rootSlug)
	})
	if err != nil {
		uc.logFailure(ctx, span, in, err)
		return output.ExpenseOutput{}, err
	}

	return mappers.M.Expense(expense), nil
}

func (uc *UpsertExpense) ExecuteWithTx(ctx context.Context, tx database.DBTX, in input.UpsertExpenseInput) (output.ExpenseOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.upsert_expense.with_tx")
	defer span.End()

	cmd, err := commands.NewUpsertExpenseCommand(
		in.UserID, in.SubcategoryID, in.Source, in.ExternalTransactionID,
		in.Competence, in.AmountCents, in.OccurredAt, in.ExpectedVersion, in.Reconcile,
	)
	if err != nil {
		return output.ExpenseOutput{}, err
	}

	rootSlug, err := uc.resolveRootSlug(ctx, cmd.SubcategoryID)
	if err != nil {
		return output.ExpenseOutput{}, err
	}

	expense, err := uc.executeInTx(ctx, tx, cmd, rootSlug)
	if err != nil {
		uc.logFailure(ctx, span, in, err)
		return output.ExpenseOutput{}, err
	}

	return mappers.M.Expense(expense), nil
}

func (uc *UpsertExpense) resolveRootSlug(ctx context.Context, subcategoryID uuid.UUID) (valueobjects.RootSlug, error) {
	rootSlugStr, _, catErr := uc.categories.ValidateExpenseSubcategory(ctx, subcategoryID)
	if catErr != nil {
		return 0, fmt.Errorf("budgets.usecase.upsert_expense: validar subcategoria: %w", catErr)
	}
	rootSlug, err := valueobjects.ParseRootSlug(rootSlugStr)
	if err != nil {
		return 0, fmt.Errorf("budgets.usecase.upsert_expense: root slug inválido: %w", err)
	}
	return rootSlug, nil
}

func (uc *UpsertExpense) executeInTx(ctx context.Context, tx database.DBTX, cmd commands.UpsertExpenseCommand, rootSlug valueobjects.RootSlug) (entities.Expense, error) {
	expenses := uc.factory.ExpenseRepository(tx)
	identity := entities.ExpenseIdentity{
		UserID:                cmd.UserID,
		Source:                cmd.Source,
		ExternalTransactionID: cmd.ExtID,
	}

	existing, tombstone, getErr := expenses.GetByIdentity(ctx, identity)

	if tombstone.IsPresent() {
		return entities.Expense{}, interfaces.ErrExpenseTombstoneConflict
	}

	now := time.Now().UTC()
	cutoff := valueobjects.CompetenceFromTime(now, uc.loc)

	if getErr != nil {
		return uc.createExpense(ctx, tx, expenses, cmd, rootSlug, getErr, now, cutoff)
	}

	if existing.IsDeleted() {
		return entities.Expense{}, interfaces.ErrExpenseTombstoneConflict
	}

	if cmd.Reconcile {
		currentVersion := existing.Version()
		cmd.ExpectedVersion = &currentVersion
	}

	if cmd.ExpectedVersion == nil {
		return existing, nil
	}

	return uc.updateExpense(ctx, tx, expenses, existing, cmd, rootSlug, now, cutoff)
}

func (uc *UpsertExpense) createExpense(
	ctx context.Context,
	tx database.DBTX,
	expenses interfaces.ExpenseRepository,
	cmd commands.UpsertExpenseCommand,
	rootSlug valueobjects.RootSlug,
	getErr error,
	now time.Time,
	cutoff valueobjects.Competence,
) (entities.Expense, error) {
	if !errors.Is(getErr, interfaces.ErrExpenseNotFound) {
		return entities.Expense{}, fmt.Errorf("budgets.usecase.upsert_expense: ler despesa: %w", getErr)
	}
	if cmd.ExpectedVersion != nil {
		return entities.Expense{}, ErrUpsertExpenseExplicitVersion
	}

	occurredAt := cmd.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = now
	}

	expense, err := entities.NewExpense(cmd.UserID, cmd.Source, cmd.ExtID, cmd.SubcategoryID, rootSlug, cmd.Competence, cmd.AmountCents, occurredAt, now)
	if err != nil {
		return entities.Expense{}, err
	}
	if err := uc.autoDraft.EnsureExists(ctx, tx, cmd.UserID, cmd.Competence, now); err != nil {
		return entities.Expense{}, fmt.Errorf("budgets.usecase.upsert_expense: auto draft: %w", err)
	}
	if err := expenses.Insert(ctx, expense); err != nil {
		return entities.Expense{}, fmt.Errorf("budgets.usecase.upsert_expense: inserir despesa: %w", err)
	}
	if err := uc.publishCommitted(ctx, tx, expense.ID(), cmd, rootSlug, valueobjects.MutationKindCreate, now, cutoff); err != nil {
		return entities.Expense{}, err
	}
	return expense, nil
}

func (uc *UpsertExpense) updateExpense(
	ctx context.Context,
	tx database.DBTX,
	expenses interfaces.ExpenseRepository,
	existing entities.Expense,
	cmd commands.UpsertExpenseCommand,
	rootSlug valueobjects.RootSlug,
	now time.Time,
	cutoff valueobjects.Competence,
) (entities.Expense, error) {
	occurredAt := cmd.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = existing.OccurredAt()
	}
	if err := existing.Edit(cmd.SubcategoryID, rootSlug, cmd.Competence, cmd.AmountCents, occurredAt, *cmd.ExpectedVersion, now); err != nil {
		if errors.Is(err, entities.ErrExpenseVersionMismatch) {
			return entities.Expense{}, interfaces.ErrExpenseConflict
		}
		return entities.Expense{}, fmt.Errorf("budgets.usecase.upsert_expense: editar despesa: %w", err)
	}
	if err := expenses.Update(ctx, existing, *cmd.ExpectedVersion); err != nil {
		return entities.Expense{}, fmt.Errorf("budgets.usecase.upsert_expense: atualizar despesa: %w", err)
	}
	if err := uc.publishCommitted(ctx, tx, existing.ID(), cmd, rootSlug, valueobjects.MutationKindUpdate, now, cutoff); err != nil {
		return entities.Expense{}, err
	}
	return existing, nil
}

func (uc *UpsertExpense) publishCommitted(
	ctx context.Context,
	tx database.DBTX,
	expenseID uuid.UUID,
	cmd commands.UpsertExpenseCommand,
	rootSlug valueobjects.RootSlug,
	mutationKind valueobjects.MutationKind,
	committedAt time.Time,
	cutoff valueobjects.Competence,
) error {
	evt, err := events.NewExpenseCommitted(
		expenseID, cmd.UserID, cmd.SubcategoryID, rootSlug, cmd.Competence,
		mutationKind, committedAt, cutoff,
	)
	if err != nil {
		return fmt.Errorf("budgets.usecase.upsert_expense: construir evento: %w", err)
	}
	if err := uc.publisher.Publish(ctx, tx, evt); err != nil {
		return fmt.Errorf("budgets.usecase.upsert_expense: publicar evento: %w", err)
	}
	return nil
}

func (uc *UpsertExpense) logFailure(ctx context.Context, span observability.Span, in input.UpsertExpenseInput, err error) {
	span.RecordError(err)
	uc.o11y.Logger().Warn(ctx, "budgets.usecase.upsert_expense.failed",
		observability.String("user_id", in.UserID),
		observability.String("source", in.Source),
		observability.String("external_transaction_id", in.ExternalTransactionID),
		observability.Error(err),
	)
}
