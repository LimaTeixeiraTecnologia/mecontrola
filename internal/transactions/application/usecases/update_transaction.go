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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
)

type UpdateTransaction struct {
	factory           interfaces.RepositoryFactory
	uow               uow.UnitOfWork
	categoryValidator interfaces.CategoryValidator
	workflow          services.TransactionWorkflow
	publisher         interfaces.TransactionEventPublisher
	o11y              observability.Observability
}

func NewUpdateTransaction(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	categoryValidator interfaces.CategoryValidator,
	workflow services.TransactionWorkflow,
	publisher interfaces.TransactionEventPublisher,
	o11y observability.Observability,
) *UpdateTransaction {
	return &UpdateTransaction{
		factory:           factory,
		uow:               u,
		categoryValidator: categoryValidator,
		workflow:          workflow,
		publisher:         publisher,
		o11y:              o11y,
	}
}

func (uc *UpdateTransaction) Execute(ctx context.Context, txID string, raw input.RawUpdateTransaction) (output.Transaction, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.update_transaction")
	defer span.End()

	if err := raw.Validate(); err != nil {
		return output.Transaction{}, err
	}

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return output.Transaction{}, ErrUsecaseUnauthorized
	}

	cmd, err := commands.NewUpdateTransaction(toCommandRawUpdate(raw, txID), principal.UserID)
	if err != nil {
		span.RecordError(err)
		return output.Transaction{}, fmt.Errorf("transactions/update_transaction: comando: %w", err)
	}

	if err := guardSubcategoryRequired(cmd.Direction, cmd.SubcategoryID.IsPresent()); err != nil {
		return output.Transaction{}, err
	}

	catSubID := optSubcategoryUUID(cmd.SubcategoryID)
	catSnap, err := uc.categoryValidator.Validate(ctx, cmd.CategoryID.UUID(), catSubID)
	if err != nil {
		span.RecordError(err)
		return output.Transaction{}, fmt.Errorf("transactions/update_transaction: validar categoria: %w", err)
	}

	if err := guardCategoryKindDirection(cmd.Direction, catSnap.Kind); err != nil {
		return output.Transaction{}, err
	}

	eventID := uuid.New()
	now := time.Now().UTC()

	tx, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (entities.Transaction, error) {
		return uc.persist(ctx, span, db, cmd, catSnap, catSubID, eventID, now)
	})
	if execErr != nil {
		span.RecordError(execErr)
		return output.Transaction{}, execErr
	}

	return output.TransactionFrom(&tx), nil
}

func (uc *UpdateTransaction) persist(
	ctx context.Context,
	span observability.Span,
	db database.DBTX,
	cmd commands.UpdateTransaction,
	catSnap interfaces.CategorySnapshot,
	catSubID *uuid.UUID,
	eventID uuid.UUID,
	now time.Time,
) (entities.Transaction, error) {
	repo := uc.factory.TransactionRepository(db)

	current, getErr := repo.GetByID(ctx, cmd.TransactionID, cmd.UserID.UUID())
	if getErr != nil {
		return entities.Transaction{}, fmt.Errorf("transactions/update_transaction: buscar lançamento: %w", getErr)
	}

	if guardErr := guardPaymentMethodMigration(current.PaymentMethod(), cmd.PaymentMethod); guardErr != nil {
		return entities.Transaction{}, guardErr
	}

	itemPtrs, itemsErr := repo.GetItemsByTransactionID(ctx, cmd.TransactionID)
	if itemsErr != nil {
		return entities.Transaction{}, fmt.Errorf("transactions/update_transaction: buscar itens: %w", itemsErr)
	}
	currentItems := derefInvoiceItems(itemPtrs)

	itemIDs := newInvoiceItemIDs(cmd.PaymentMethod, cmd.Installments)
	decision := uc.workflow.DecideUpdate(*current, currentItems, cmd, eventID, itemIDs, now)
	span.SetAttributes(
		observability.Int("installments_total", len(decision.Items)),
		observability.Int("ref_months_affected_count", refMonthsAffectedCount(decision.Event)),
	)
	decision.Transaction.SetCategorySnapshots(catSnap.Name, snapSubName(catSubID, catSnap))

	if updateErr := repo.UpdateWithVersion(ctx, &decision.Transaction, cmd.Version); updateErr != nil {
		return entities.Transaction{}, fmt.Errorf("transactions/update_transaction: atualizar: %w", updateErr)
	}

	if len(decision.Items) > 0 {
		if invErr := uc.recomposeInvoices(ctx, db, repo, cmd, &decision); invErr != nil {
			return entities.Transaction{}, invErr
		}
	}

	if updated, ok := decision.Event.(entities.TransactionUpdated); ok {
		if publishErr := uc.publisher.PublishUpdated(ctx, db, updated); publishErr != nil {
			return entities.Transaction{}, fmt.Errorf("transactions/update_transaction: publicar evento: %w", publishErr)
		}
	}

	return decision.Transaction, nil
}

func (uc *UpdateTransaction) recomposeInvoices(
	ctx context.Context,
	db database.DBTX,
	repo interfaces.TransactionRepository,
	cmd commands.UpdateTransaction,
	decision *services.TransactionDecision,
) error {
	invoiceRepo := uc.factory.CardInvoiceRepository(db)
	count := installmentCountOrSingle(cmd.Installments)
	snapshot, _ := decision.Transaction.BillingSnapshot().Get()
	resolver := services.BillingCycleResolver{}
	_, closings, dues := resolver.Resolve(cmd.OccurredAt, snapshot, count)
	cardID, _ := decision.Transaction.CardID().Get()
	items, buildErr := rebuildInvoiceItems(ctx, invoiceRepo, cmd.UserID.UUID(), cardID.UUID(), decision.Transaction.ID(), decision.Items, closings, dues)
	if buildErr != nil {
		return buildErr
	}
	if replaceErr := repo.ReplaceItems(ctx, decision.Transaction.ID(), items); replaceErr != nil {
		return fmt.Errorf("transactions/update_transaction: replace items: %w", replaceErr)
	}
	if deltaErr := applyInvoiceDeltasByMonth(ctx, invoiceRepo, cmd.UserID.UUID(), cardID.UUID(), decision.InvoiceDeltas); deltaErr != nil {
		return deltaErr
	}
	return nil
}
