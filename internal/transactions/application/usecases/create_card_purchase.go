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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
)

type CreateCardPurchase struct {
	factory           interfaces.RepositoryFactory
	cardLookup        interfaces.CardLookup
	categoryValidator interfaces.CategoryValidator
	workflow          *services.CardPurchaseWorkflow
	publisher         interfaces.CardPurchaseEventPublisher
	uow               uow.UnitOfWork
	idGen             id.Generator
	o11y              observability.Observability
}

func NewCreateCardPurchase(
	factory interfaces.RepositoryFactory,
	cardLookup interfaces.CardLookup,
	categoryValidator interfaces.CategoryValidator,
	workflow *services.CardPurchaseWorkflow,
	publisher interfaces.CardPurchaseEventPublisher,
	u uow.UnitOfWork,
	idGen id.Generator,
	o11y observability.Observability,
) *CreateCardPurchase {
	return &CreateCardPurchase{
		factory:           factory,
		cardLookup:        cardLookup,
		categoryValidator: categoryValidator,
		workflow:          workflow,
		publisher:         publisher,
		uow:               u,
		idGen:             idGen,
		o11y:              o11y,
	}
}

func (uc *CreateCardPurchase) Execute(ctx context.Context, raw input.RawCreateCardPurchase) (output.CardPurchase, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.create_card_purchase")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok || principal.IsZero() {
		return output.CardPurchase{}, ErrUsecaseUnauthorized
	}

	if err := raw.Validate(); err != nil {
		return output.CardPurchase{}, err
	}

	cmd, err := uc.buildCommand(raw, principal.UserID)
	if err != nil {
		return output.CardPurchase{}, err
	}

	snapshot, err := uc.cardLookup.GetForUser(ctx, cmd.CardID.UUID(), cmd.UserID.UUID())
	if err != nil {
		return output.CardPurchase{}, fmt.Errorf("transactions/create_card_purchase: lookup cartão: %w", err)
	}

	var subPtr *uuid.UUID
	if sub, ok2 := cmd.SubcategoryID.Get(); ok2 {
		v := sub.UUID()
		subPtr = &v
	}
	catSnapshot, err := uc.categoryValidator.Validate(ctx, cmd.CategoryID.UUID(), subPtr)
	if err != nil {
		return output.CardPurchase{}, fmt.Errorf("transactions/create_card_purchase: validar categoria: %w", err)
	}

	eventID, _ := uuid.Parse(uc.idGen.NewID())
	purchaseID, _ := uuid.Parse(uc.idGen.NewID())

	decision := uc.workflow.DecideCreate(cmd, snapshot, purchaseID, eventID, time.Now().UTC())

	purchase := entities.NewCardPurchase(
		purchaseID,
		cmd.UserID,
		cmd.CardID,
		cmd.TotalAmount,
		cmd.Installments,
		cmd.Description,
		cmd.CategoryID,
		cmd.SubcategoryID,
		catSnapshot.Name,
		catSnapshot.ParentName,
		cmd.PurchasedAt,
		snapshot,
		time.Now().UTC(),
	)

	if raw.OriginWamid != "" {
		purchase.SetOrigin(raw.OriginWamid, raw.OriginItemSeq, raw.OriginOperation)
	}

	resolver := services.BillingCycleResolver{}
	_, closings, dues := resolver.Resolve(cmd.PurchasedAt, snapshot, cmd.Installments)

	var created bool
	result, err := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (entities.CardPurchase, error) {
		p, c, e := uc.executeInTx(ctx, db, &purchase, decision, cmd, closings, dues)
		created = c
		return p, e
	})
	if err != nil {
		span.RecordError(err)
		return output.CardPurchase{}, err
	}

	if !created {
		out := output.CardPurchaseFrom(&result, nil, nil)
		out.Reconciled = true
		return out, nil
	}

	return uc.successOutput(&purchase, decision), nil
}

func (uc *CreateCardPurchase) successOutput(purchase *entities.CardPurchase, decision services.CardPurchaseDecision) output.CardPurchase {
	refMonthsStr := make([]string, len(decision.Items))
	for i, item := range decision.Items {
		refMonthsStr[i] = item.RefMonth().String()
	}

	out := output.CardPurchaseFrom(purchase, nil, refMonthsStr)
	out.Reconciled = false
	return out
}

func (uc *CreateCardPurchase) buildCommand(raw input.RawCreateCardPurchase, userID uuid.UUID) (commands.CreateCardPurchase, error) {
	purchasedAt, parseErr := parseISO8601(raw.PurchasedAt)
	if parseErr != nil {
		return commands.CreateCardPurchase{}, fmt.Errorf("transactions/create_card_purchase: purchased_at inválido: %w", parseErr)
	}

	rawCmd := commands.RawCreateCardPurchase{
		CardID:            raw.CardID.String(),
		TotalAmountCents:  raw.TotalAmountCents,
		InstallmentsTotal: raw.InstallmentsTotal,
		Description:       raw.Description,
		CategoryID:        raw.CategoryID.String(),
		PurchasedAt:       purchasedAt,
	}
	if raw.SubcategoryID != nil {
		rawCmd.SubcategoryID = raw.SubcategoryID.String()
	}

	return commands.NewCreateCardPurchase(rawCmd, userID)
}

func (uc *CreateCardPurchase) executeInTx(
	ctx context.Context,
	db database.DBTX,
	purchase *entities.CardPurchase,
	decision services.CardPurchaseDecision,
	cmd commands.CreateCardPurchase,
	closings, dues []time.Time,
) (entities.CardPurchase, bool, error) {
	purchasesRepo := uc.factory.CardPurchaseRepository(db)
	invoicesRepo := uc.factory.CardInvoiceRepository(db)

	canonicalID, created, createErr := purchasesRepo.Create(ctx, purchase)
	if createErr != nil {
		return entities.CardPurchase{}, false, fmt.Errorf("transactions/create_card_purchase: criar compra: %w", createErr)
	}
	if !created {
		existing, getErr := purchasesRepo.GetByID(ctx, canonicalID, cmd.UserID.UUID())
		if getErr != nil {
			return entities.CardPurchase{}, false, fmt.Errorf("transactions/create_card_purchase: reconciliar compra: %w", getErr)
		}
		return *existing, false, nil
	}

	items, buildErr := buildItemsWithInvoices(ctx, invoicesRepo, cmd, decision.Items, closings, dues)
	if buildErr != nil {
		return entities.CardPurchase{}, false, buildErr
	}

	if replaceErr := purchasesRepo.ReplaceItems(ctx, purchase.ID(), items); replaceErr != nil {
		return entities.CardPurchase{}, false, fmt.Errorf("transactions/create_card_purchase: replace items: %w", replaceErr)
	}

	evt, evtOk := decision.Event.(entities.CardPurchaseCreated)
	if !evtOk {
		return entities.CardPurchase{}, false, fmt.Errorf("transactions/create_card_purchase: tipo de evento inesperado")
	}
	if pubErr := uc.publisher.PublishCreated(ctx, db, evt); pubErr != nil {
		return entities.CardPurchase{}, false, fmt.Errorf("transactions/create_card_purchase: publicar evento: %w", pubErr)
	}
	return *purchase, true, nil
}

func buildItemsWithInvoices(
	ctx context.Context,
	invoicesRepo interfaces.CardInvoiceRepository,
	cmd commands.CreateCardPurchase,
	decisionItems []entities.CardInvoiceItem,
	closings, dues []time.Time,
) ([]*entities.CardInvoiceItem, error) {
	items := make([]*entities.CardInvoiceItem, len(decisionItems))
	for i := range decisionItems {
		item := decisionItems[i]
		inv, upsertErr := invoicesRepo.UpsertByMonth(ctx, cmd.UserID.UUID(), cmd.CardID.UUID(), item.RefMonth(), closings[i], dues[i])
		if upsertErr != nil {
			return nil, fmt.Errorf("transactions/create_card_purchase: upsert fatura [%d]: %w", i, upsertErr)
		}
		itemWithInvoice := entities.NewCardInvoiceItem(
			item.ID(), inv.ID(), item.PurchaseID(), item.UserID(),
			item.RefMonth(), item.InstallmentIndex(), item.Amount(), item.CreatedAt(),
		)
		items[i] = &itemWithInvoice
		if deltaErr := invoicesRepo.ApplyDelta(ctx, inv.ID(), item.Amount().Cents(), inv.Version()); deltaErr != nil {
			return nil, fmt.Errorf("transactions/create_card_purchase: apply delta [%d]: %w", i, deltaErr)
		}
	}
	return items, nil
}
