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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
)

const materializeBatchSize = 200

type transactionCreator interface {
	Execute(ctx context.Context, raw input.RawCreateTransaction) (output.Transaction, error)
}

type cardPurchaseCreator interface {
	Execute(ctx context.Context, raw input.RawCreateCardPurchase) (output.CardPurchase, error)
}

type MaterializeRecurringForDay struct {
	db                 database.DBTX
	factory            interfaces.RepositoryFactory
	uow                uow.UnitOfWork
	workflow           services.RecurringWorkflow
	createTransaction  transactionCreator
	createCardPurchase cardPurchaseCreator
	brazilLoc          *time.Location
	o11y               observability.Observability
}

func NewMaterializeRecurringForDay(
	db database.DBTX,
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	workflow services.RecurringWorkflow,
	createTransaction transactionCreator,
	createCardPurchase cardPurchaseCreator,
	brazilLoc *time.Location,
	o11y observability.Observability,
) *MaterializeRecurringForDay {
	return &MaterializeRecurringForDay{
		db:                 db,
		factory:            factory,
		uow:                u,
		workflow:           workflow,
		createTransaction:  createTransaction,
		createCardPurchase: createCardPurchase,
		brazilLoc:          brazilLoc,
		o11y:               o11y,
	}
}

func (uc *MaterializeRecurringForDay) Execute(ctx context.Context, today time.Time) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.materialize_recurring_for_day")
	defer span.End()

	todayInBrazil := today.In(uc.brazilLoc)
	dayOfMonth := todayInBrazil.Day()
	cursor := interfaces.Cursor{}

	for {
		templateRepo := uc.factory.RecurringTemplateRepository(uc.db)

		templates, nextCursor, err := templateRepo.FindActiveByDayOfMonth(ctx, dayOfMonth, todayInBrazil, cursor, materializeBatchSize)
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("transactions/materialize_recurring_for_day: buscar templates: %w", err)
		}

		for _, template := range templates {
			decision := uc.workflow.DecideMaterializeForDay(*template, todayInBrazil, uc.brazilLoc)
			if !decision.ShouldMaterialize {
				continue
			}

			if materializeErr := uc.materializeOne(ctx, template, decision, todayInBrazil); materializeErr != nil {
				span.RecordError(materializeErr)
				return fmt.Errorf("transactions/materialize_recurring_for_day: template %s: %w", template.ID(), materializeErr)
			}
		}

		if nextCursor.Value == "" {
			break
		}
		cursor = nextCursor
	}

	return nil
}

func (uc *MaterializeRecurringForDay) materializeOne(
	ctx context.Context,
	template *entities.RecurringTemplate,
	decision services.MaterializeDecision,
	todayInBrazil time.Time,
) error {
	occurredAt := time.Date(
		todayInBrazil.Year(), todayInBrazil.Month(), todayInBrazil.Day(),
		12, 0, 0, 0, uc.brazilLoc,
	)
	userCtx := auth.WithPrincipal(ctx, auth.Principal{UserID: template.UserID().UUID()})

	shouldProceed := false

	_, lockCheckErr := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (struct{}, error) {
		materializationRepo := uc.factory.RecurringMaterializationRepository(db)

		acquired, _, lockErr := materializationRepo.TryAdvisoryLock(ctx, decision.TemplateID, decision.RefMonth)
		if lockErr != nil {
			return struct{}{}, fmt.Errorf("transactions/materialize: advisory lock: %w", lockErr)
		}
		if !acquired {
			return struct{}{}, nil
		}

		now := time.Now().UTC()
		inserted, insertErr := materializationRepo.InsertIfAbsent(ctx, decision.TemplateID, decision.RefMonth, nil, nil, now)
		if insertErr != nil {
			return struct{}{}, fmt.Errorf("transactions/materialize: insert if absent: %w", insertErr)
		}
		if inserted {
			shouldProceed = true
			return struct{}{}, nil
		}

		completed, completedErr := materializationRepo.IsCompleted(ctx, decision.TemplateID, decision.RefMonth)
		if completedErr != nil {
			return struct{}{}, fmt.Errorf("transactions/materialize: verificar conclusão: %w", completedErr)
		}
		if !completed {
			shouldProceed = true
		}
		return struct{}{}, nil
	})
	if lockCheckErr != nil {
		return lockCheckErr
	}
	if !shouldProceed {
		return nil
	}

	if decision.AsTransaction {
		entityID, err := uc.materializeAsTransaction(userCtx, template, occurredAt)
		if err != nil {
			return err
		}
		materializationRepo := uc.factory.RecurringMaterializationRepository(uc.db)
		return materializationRepo.MarkCompleted(ctx, decision.TemplateID, decision.RefMonth, &entityID, nil)
	}

	entityID, err := uc.materializeAsCardPurchase(userCtx, template, occurredAt)
	if err != nil {
		return err
	}
	materializationRepo := uc.factory.RecurringMaterializationRepository(uc.db)
	return materializationRepo.MarkCompleted(ctx, decision.TemplateID, decision.RefMonth, nil, &entityID)
}

func (uc *MaterializeRecurringForDay) materializeAsTransaction(
	ctx context.Context,
	template *entities.RecurringTemplate,
	occurredAt time.Time,
) (uuid.UUID, error) {
	raw := input.RawCreateTransaction{
		Direction:     template.Direction().String(),
		PaymentMethod: template.PaymentMethod().String(),
		AmountCents:   template.Amount().Cents(),
		Description:   template.Description().String(),
		CategoryID:    template.CategoryID().UUID(),
		OccurredAt:    occurredAt.Format(time.RFC3339),
	}
	if sub, ok := template.SubcategoryID().Get(); ok {
		u := sub.UUID()
		raw.SubcategoryID = &u
	}

	result, err := uc.createTransaction.Execute(ctx, raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("transactions/materialize: criar transação: %w", err)
	}
	return result.ID, nil
}

func (uc *MaterializeRecurringForDay) materializeAsCardPurchase(
	ctx context.Context,
	template *entities.RecurringTemplate,
	occurredAt time.Time,
) (uuid.UUID, error) {
	cardID, ok := template.CardID().Get()
	if !ok {
		return uuid.Nil, fmt.Errorf("transactions/materialize: template de crédito sem card_id: %s", template.ID())
	}

	raw := input.RawCreateCardPurchase{
		CardID:            cardID.UUID(),
		TotalAmountCents:  template.Amount().Cents(),
		InstallmentsTotal: template.InstallmentsTotal().Value(),
		Description:       template.Description().String(),
		CategoryID:        template.CategoryID().UUID(),
		PurchasedAt:       occurredAt.Format(time.RFC3339),
	}
	if sub, ok := template.SubcategoryID().Get(); ok {
		u := sub.UUID()
		raw.SubcategoryID = &u
	}

	result, err := uc.createCardPurchase.Execute(ctx, raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("transactions/materialize: criar card purchase: %w", err)
	}
	return result.ID, nil
}
