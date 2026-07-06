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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type CreateTransaction struct {
	factory           interfaces.RepositoryFactory
	uow               uow.UnitOfWork
	cardLookup        interfaces.CardLookup
	categoryValidator interfaces.CategoryValidator
	categoryGate      interfaces.CategoryWriteGate
	workflow          services.TransactionWorkflow
	publisher         interfaces.TransactionEventPublisher
	o11y              observability.Observability
}

func NewCreateTransaction(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	cardLookup interfaces.CardLookup,
	categoryValidator interfaces.CategoryValidator,
	categoryGate interfaces.CategoryWriteGate,
	workflow services.TransactionWorkflow,
	publisher interfaces.TransactionEventPublisher,
	o11y observability.Observability,
) *CreateTransaction {
	return &CreateTransaction{
		factory:           factory,
		uow:               u,
		cardLookup:        cardLookup,
		categoryValidator: categoryValidator,
		categoryGate:      categoryGate,
		workflow:          workflow,
		publisher:         publisher,
		o11y:              o11y,
	}
}

func (uc *CreateTransaction) Execute(ctx context.Context, raw input.RawCreateTransaction) (output.Transaction, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.create_transaction")
	defer span.End()

	if err := raw.Validate(); err != nil {
		return output.Transaction{}, err
	}

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return output.Transaction{}, ErrUsecaseUnauthorized
	}

	cmd, err := commands.NewCreateTransaction(toCommandRawCreate(raw), principal.UserID)
	if err != nil {
		span.RecordError(err)
		return output.Transaction{}, fmt.Errorf("transactions/create_transaction: comando: %w", err)
	}
	if err := guardSubcategoryRequired(cmd.Direction, cmd.SubcategoryID.IsPresent()); err != nil {
		return output.Transaction{}, err
	}

	catSubID := optSubcategoryUUID(cmd.SubcategoryID)
	catSnap, err := uc.categoryValidator.Validate(ctx, cmd.CategoryID.UUID(), catSubID)
	if err != nil {
		span.RecordError(err)
		return output.Transaction{}, fmt.Errorf("transactions/create_transaction: validar categoria: %w", err)
	}

	if err := guardCategoryKindDirection(cmd.Direction, catSnap.Kind); err != nil {
		return output.Transaction{}, err
	}

	decision, billing, cardUUID, prepErr := uc.prepareDecision(ctx, raw, cmd, catSnap, catSubID, principal.UserID)
	if prepErr != nil {
		span.RecordError(prepErr)
		return output.Transaction{}, prepErr
	}
	span.SetAttributes(
		observability.Int("installments_total", installmentCountOrSingle(cmd.Installments).Value()),
		observability.Int("ref_months_affected_count", refMonthsAffectedCount(decision.Event)),
	)

	var created bool
	tx, err := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (entities.Transaction, error) {
		persisted, c, persistErr := uc.persist(ctx, db, cmd, principal.UserID, billing, cardUUID, &decision)
		created = c
		return persisted, persistErr
	})
	if err != nil {
		span.RecordError(err)
		return output.Transaction{}, err
	}

	out := output.TransactionFrom(&tx)
	out.Reconciled = !created
	return out, nil
}

func (uc *CreateTransaction) prepareDecision(
	ctx context.Context,
	raw input.RawCreateTransaction,
	cmd commands.CreateTransaction,
	catSnap interfaces.CategorySnapshot,
	catSubID *uuid.UUID,
	userID uuid.UUID,
) (services.TransactionDecision, valueobjects.CardBillingSnapshot, uuid.UUID, error) {
	snap, billing, cardUUID, lookupErr := uc.resolveBilling(ctx, cmd, userID)
	if lookupErr != nil {
		return services.TransactionDecision{}, valueobjects.CardBillingSnapshot{}, uuid.Nil, lookupErr
	}

	evidence, gateErr := uc.approveCategory(ctx, raw, cmd.Direction.String(), cmd.CategoryID.UUID(), catSubID)
	if gateErr != nil {
		return services.TransactionDecision{}, valueobjects.CardBillingSnapshot{}, uuid.Nil, gateErr
	}

	txID := uuid.New()
	eventID := uuid.New()
	now := time.Now().UTC()

	itemIDs := newInvoiceItemIDs(cmd.PaymentMethod, cmd.Installments)
	decision := uc.workflow.DecideCreate(cmd, snap, evidence, txID, eventID, itemIDs, now)
	decision.Transaction.SetCategorySnapshots(catSnap.Name, snapSubName(catSubID, catSnap))
	if raw.OriginWamid != "" {
		decision.Transaction.SetOrigin(raw.OriginWamid, raw.OriginItemSeq, raw.OriginOperation)
	}

	return decision, billing, cardUUID, nil
}

func (uc *CreateTransaction) approveCategory(ctx context.Context, raw input.RawCreateTransaction, direction string, rootID uuid.UUID, subID *uuid.UUID) (valueobjects.CategoryWriteEvidence, error) {
	evidence, err := approveUpdateCategory(ctx, uc.categoryGate, evidenceFromRawCreate(raw), direction, "create_transaction", rootID, subID)
	if err != nil {
		return valueobjects.CategoryWriteEvidence{}, fmt.Errorf("transactions/create_transaction: gate de categoria: %w", err)
	}
	return evidence, nil
}

func (uc *CreateTransaction) resolveBilling(
	ctx context.Context,
	cmd commands.CreateTransaction,
	userID uuid.UUID,
) (option.Option[valueobjects.CardBillingSnapshot], valueobjects.CardBillingSnapshot, uuid.UUID, error) {
	if cmd.PaymentMethod != valueobjects.PaymentMethodCreditCard {
		return option.None[valueobjects.CardBillingSnapshot](), valueobjects.CardBillingSnapshot{}, uuid.Nil, nil
	}
	cid, _ := cmd.CardID.Get()
	cardUUID := cid.UUID()
	resolved, lookupErr := uc.cardLookup.GetForUser(ctx, cardUUID, userID)
	if lookupErr != nil {
		return option.None[valueobjects.CardBillingSnapshot](), valueobjects.CardBillingSnapshot{}, uuid.Nil, fmt.Errorf("transactions/create_transaction: lookup cartão: %w", lookupErr)
	}
	return option.Some(resolved), resolved, cardUUID, nil
}

func (uc *CreateTransaction) persist(
	ctx context.Context,
	db database.DBTX,
	cmd commands.CreateTransaction,
	userID uuid.UUID,
	billing valueobjects.CardBillingSnapshot,
	cardUUID uuid.UUID,
	decision *services.TransactionDecision,
) (entities.Transaction, bool, error) {
	repo := uc.factory.TransactionRepository(db)
	canonicalID, created, createErr := repo.Create(ctx, &decision.Transaction)
	if createErr != nil {
		return entities.Transaction{}, false, fmt.Errorf("transactions/create_transaction: persistir: %w", createErr)
	}
	if !created {
		existing, getErr := repo.GetByID(ctx, canonicalID, userID)
		if getErr != nil {
			return entities.Transaction{}, false, fmt.Errorf("transactions/create_transaction: reconciliar: %w", getErr)
		}
		return *existing, false, nil
	}

	if len(decision.Items) > 0 {
		if invErr := uc.applyInvoices(ctx, db, repo, cmd, userID, billing, cardUUID, decision); invErr != nil {
			return entities.Transaction{}, false, invErr
		}
	}

	if createdEvt, ok := decision.Event.(entities.TransactionCreated); ok {
		if publishErr := uc.publisher.PublishCreated(ctx, db, createdEvt); publishErr != nil {
			return entities.Transaction{}, false, fmt.Errorf("transactions/create_transaction: publicar evento: %w", publishErr)
		}
	}
	return decision.Transaction, true, nil
}

func (uc *CreateTransaction) applyInvoices(
	ctx context.Context,
	db database.DBTX,
	repo interfaces.TransactionRepository,
	cmd commands.CreateTransaction,
	userID uuid.UUID,
	billing valueobjects.CardBillingSnapshot,
	cardUUID uuid.UUID,
	decision *services.TransactionDecision,
) error {
	invoiceRepo := uc.factory.CardInvoiceRepository(db)
	count := installmentCountOrSingle(cmd.Installments)
	resolver := services.BillingCycleResolver{}
	_, closings, dues := resolver.Resolve(cmd.OccurredAt, billing, count)
	items, buildErr := createInvoiceItems(ctx, invoiceRepo, userID, cardUUID, decision.Transaction.ID(), decision.Items, closings, dues)
	if buildErr != nil {
		return buildErr
	}
	if replaceErr := repo.ReplaceItems(ctx, decision.Transaction.ID(), items); replaceErr != nil {
		return fmt.Errorf("transactions/create_transaction: replace items: %w", replaceErr)
	}
	return nil
}
