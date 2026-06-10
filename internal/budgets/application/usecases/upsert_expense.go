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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrUpsertExpenseExplicitVersion = errors.New("budgets: version não deve ser fornecido na criação de despesa")

var ErrUpsertExpenseVersionRequired = errors.New("budgets: expected_version obrigatório para edição de despesa")

var ErrUpsertExpenseInvalidUserID = errors.New("budgets: user_id inválido na despesa")

var ErrUpsertExpenseInvalidSubcategory = errors.New("budgets: subcategory_id inválido")

var ErrUpsertExpenseInvalidCompetence = errors.New("budgets: competence inválida na despesa")

var ErrUpsertExpenseInvalidSource = errors.New("budgets: source inválido na despesa")

var ErrUpsertExpenseInvalidExternalID = errors.New("budgets: external_transaction_id inválido")

var ErrUpsertExpenseInvalidAmount = errors.New("budgets: amount_cents deve ser maior que zero")

type UpsertExpense struct {
	expenses   interfaces.ExpenseRepository
	budgets    interfaces.BudgetRepository
	categories interfaces.CategoriesReader
	publisher  interfaces.ExpenseCommittedPublisher
	autoDraft  *CreateOrAutoDraftForExpense
	uow        uow.UnitOfWork[entities.Expense]
	o11y       observability.Observability
	loc        *time.Location
}

func NewUpsertExpense(
	expenses interfaces.ExpenseRepository,
	budgets interfaces.BudgetRepository,
	categories interfaces.CategoriesReader,
	publisher interfaces.ExpenseCommittedPublisher,
	autoDraft *CreateOrAutoDraftForExpense,
	u uow.UnitOfWork[entities.Expense],
	o11y observability.Observability,
	loc *time.Location,
) *UpsertExpense {
	return &UpsertExpense{
		expenses:   expenses,
		budgets:    budgets,
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

	resolved, resErr := uc.resolveInput(ctx, in)
	if resErr != nil {
		return output.ExpenseOutput{}, resErr
	}

	expense, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.Expense, error) {
		return uc.ExecuteInTx(ctx, tx, resolved)
	})

	if execErr != nil {
		span.RecordError(execErr)
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.upsert_expense.failed",
			observability.String("user_id", in.UserID),
			observability.String("source", in.Source),
			observability.String("external_transaction_id", in.ExternalTransactionID),
			observability.Error(execErr),
		)
		return output.ExpenseOutput{}, execErr
	}

	return mapExpenseOutput(expense), nil
}

func (uc *UpsertExpense) ExecuteWithTx(ctx context.Context, tx database.DBTX, in input.UpsertExpenseInput) (output.ExpenseOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.upsert_expense.with_tx")
	defer span.End()

	resolved, resErr := uc.resolveInput(ctx, in)
	if resErr != nil {
		return output.ExpenseOutput{}, resErr
	}

	expense, execErr := uc.ExecuteInTx(ctx, tx, resolved)
	if execErr != nil {
		span.RecordError(execErr)
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.upsert_expense.failed",
			observability.String("user_id", in.UserID),
			observability.String("source", in.Source),
			observability.String("external_transaction_id", in.ExternalTransactionID),
			observability.Error(execErr),
		)
		return output.ExpenseOutput{}, execErr
	}

	return mapExpenseOutput(expense), nil
}

func (uc *UpsertExpense) resolveInput(ctx context.Context, in input.UpsertExpenseInput) (upsertExpenseResolved, error) {
	userID, err := uuid.Parse(in.UserID)
	if err != nil {
		return upsertExpenseResolved{}, ErrUpsertExpenseInvalidUserID
	}

	subcategoryID, err := uuid.Parse(in.SubcategoryID)
	if err != nil {
		return upsertExpenseResolved{}, ErrUpsertExpenseInvalidSubcategory
	}

	source, err := valueobjects.NewProducerSource(in.Source)
	if err != nil {
		return upsertExpenseResolved{}, ErrUpsertExpenseInvalidSource
	}

	extID, err := valueobjects.NewExternalTransactionID(in.ExternalTransactionID)
	if err != nil {
		return upsertExpenseResolved{}, ErrUpsertExpenseInvalidExternalID
	}

	competence, err := valueobjects.NewCompetence(in.Competence)
	if err != nil {
		return upsertExpenseResolved{}, ErrUpsertExpenseInvalidCompetence
	}

	if in.AmountCents <= 0 {
		return upsertExpenseResolved{}, ErrUpsertExpenseInvalidAmount
	}

	rootSlugStr, _, catErr := uc.categories.ValidateExpenseSubcategory(ctx, subcategoryID)
	if catErr != nil {
		return upsertExpenseResolved{}, fmt.Errorf("budgets.usecase.upsert_expense: validar subcategoria: %w", catErr)
	}

	rootSlug, err := valueobjects.ParseRootSlug(rootSlugStr)
	if err != nil {
		return upsertExpenseResolved{}, fmt.Errorf("budgets.usecase.upsert_expense: root slug inválido: %w", err)
	}

	return upsertExpenseResolved{
		userID:        userID,
		source:        source,
		extID:         extID,
		subcategoryID: subcategoryID,
		rootSlug:      rootSlug,
		competence:    competence,
		amountCents:   in.AmountCents,
		occurredAt:    in.OccurredAt,
		expectedVer:   in.ExpectedVersion,
	}, nil
}

type upsertExpenseResolved struct {
	userID        uuid.UUID
	source        valueobjects.ProducerSource
	extID         valueobjects.ExternalTransactionID
	subcategoryID uuid.UUID
	rootSlug      valueobjects.RootSlug
	competence    valueobjects.Competence
	amountCents   int64
	occurredAt    time.Time
	expectedVer   *int64
}

func (uc *UpsertExpense) ExecuteInTx(ctx context.Context, tx database.DBTX, r upsertExpenseResolved) (entities.Expense, error) {
	identity := entities.ExpenseIdentity{
		UserID:                r.userID,
		Source:                r.source,
		ExternalTransactionID: r.extID,
	}

	existing, tombstone, getErr := uc.expenses.GetByIdentity(ctx, tx, identity)

	if tombstone.IsPresent() {
		return entities.Expense{}, interfaces.ErrExpenseTombstoneConflict
	}

	now := time.Now().UTC()
	committedAt := now
	cutoff := valueobjects.CompetenceFromTime(now, uc.loc)

	if getErr != nil {
		if !errors.Is(getErr, interfaces.ErrExpenseNotFound) {
			return entities.Expense{}, fmt.Errorf("budgets.usecase.upsert_expense: ler despesa: %w", getErr)
		}

		if r.expectedVer != nil {
			return entities.Expense{}, ErrUpsertExpenseExplicitVersion
		}

		occurredAt := r.occurredAt
		if occurredAt.IsZero() {
			occurredAt = now
		}

		newExpense, newErr := entities.NewExpense(r.userID, r.source, r.extID, r.subcategoryID, r.rootSlug, r.competence, r.amountCents, occurredAt, now)
		if newErr != nil {
			return entities.Expense{}, newErr
		}

		if autoDraftErr := uc.autoDraft.EnsureExists(ctx, tx, r.userID, r.competence, now); autoDraftErr != nil {
			return entities.Expense{}, fmt.Errorf("budgets.usecase.upsert_expense: auto draft: %w", autoDraftErr)
		}

		if insertErr := uc.expenses.Insert(ctx, tx, newExpense); insertErr != nil {
			return entities.Expense{}, fmt.Errorf("budgets.usecase.upsert_expense: inserir despesa: %w", insertErr)
		}

		env := interfaces.ExpenseCommittedEnvelope{
			UserID:             r.userID,
			Competence:         r.competence,
			SubcategoryID:      r.subcategoryID,
			RootSlug:           r.rootSlug,
			MutationKind:       valueobjects.MutationKindCreate,
			CommittedAt:        committedAt,
			CutoffCompetenceBR: cutoff,
			ExpenseID:          newExpense.ID(),
		}
		if pubErr := uc.publisher.Publish(ctx, tx, env); pubErr != nil {
			return entities.Expense{}, fmt.Errorf("budgets.usecase.upsert_expense: publicar evento: %w", pubErr)
		}

		return newExpense, nil
	}

	if existing.IsDeleted() {
		return entities.Expense{}, interfaces.ErrExpenseTombstoneConflict
	}

	if r.expectedVer == nil {
		return existing, nil
	}

	occurredAt := r.occurredAt
	if occurredAt.IsZero() {
		occurredAt = existing.OccurredAt()
	}

	if editErr := existing.Edit(r.subcategoryID, r.rootSlug, r.competence, r.amountCents, occurredAt, *r.expectedVer, now); editErr != nil {
		if errors.Is(editErr, entities.ErrExpenseVersionMismatch) {
			return entities.Expense{}, interfaces.ErrExpenseConflict
		}
		return entities.Expense{}, fmt.Errorf("budgets.usecase.upsert_expense: editar despesa: %w", editErr)
	}

	if updateErr := uc.expenses.Update(ctx, tx, existing, *r.expectedVer); updateErr != nil {
		return entities.Expense{}, fmt.Errorf("budgets.usecase.upsert_expense: atualizar despesa: %w", updateErr)
	}

	env := interfaces.ExpenseCommittedEnvelope{
		UserID:             r.userID,
		Competence:         r.competence,
		SubcategoryID:      r.subcategoryID,
		RootSlug:           r.rootSlug,
		MutationKind:       valueobjects.MutationKindUpdate,
		CommittedAt:        committedAt,
		CutoffCompetenceBR: cutoff,
		ExpenseID:          existing.ID(),
	}
	if pubErr := uc.publisher.Publish(ctx, tx, env); pubErr != nil {
		return entities.Expense{}, fmt.Errorf("budgets.usecase.upsert_expense: publicar evento: %w", pubErr)
	}

	return existing, nil
}

func mapExpenseOutput(e entities.Expense) output.ExpenseOutput {
	return output.ExpenseOutput{
		ID:                    e.ID().String(),
		UserID:                e.UserID().String(),
		Source:                e.Source().String(),
		ExternalTransactionID: e.ExternalTransactionID().String(),
		SubcategoryID:         e.SubcategoryID().String(),
		RootSlug:              e.RootSlug().String(),
		Competence:            e.Competence().String(),
		AmountCents:           e.AmountCents(),
		OccurredAt:            e.OccurredAt(),
		Version:               e.Version(),
		TombstoneVersion:      e.TombstoneVersion(),
		DeletedAt:             e.DeletedAt(),
		CreatedAt:             e.CreatedAt(),
		UpdatedAt:             e.UpdatedAt(),
	}
}
