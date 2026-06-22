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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type UpdateCardPurchase struct {
	factory           interfaces.RepositoryFactory
	categoryValidator interfaces.CategoryValidator
	workflow          *services.CardPurchaseWorkflow
	publisher         interfaces.CardPurchaseEventPublisher
	uow               uow.UnitOfWork
	idGen             id.Generator
	o11y              observability.Observability
}

func NewUpdateCardPurchase(
	factory interfaces.RepositoryFactory,
	categoryValidator interfaces.CategoryValidator,
	workflow *services.CardPurchaseWorkflow,
	publisher interfaces.CardPurchaseEventPublisher,
	u uow.UnitOfWork,
	idGen id.Generator,
	o11y observability.Observability,
) *UpdateCardPurchase {
	return &UpdateCardPurchase{
		factory:           factory,
		categoryValidator: categoryValidator,
		workflow:          workflow,
		publisher:         publisher,
		uow:               u,
		idGen:             idGen,
		o11y:              o11y,
	}
}

func (uc *UpdateCardPurchase) Execute(ctx context.Context, purchaseID uuid.UUID, raw input.RawUpdateCardPurchase) (output.CardPurchase, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.update_card_purchase")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok || principal.IsZero() {
		return output.CardPurchase{}, ErrUsecaseUnauthorized
	}

	if err := raw.Validate(); err != nil {
		return output.CardPurchase{}, err
	}

	purchasedAt, parseErr := parseISO8601(raw.PurchasedAt)
	if parseErr != nil {
		return output.CardPurchase{}, fmt.Errorf("transactions/update_card_purchase: purchased_at inválido: %w", parseErr)
	}

	rawCmd := commands.RawUpdateCardPurchase{
		PurchaseID:        purchaseID.String(),
		TotalAmountCents:  raw.TotalAmountCents,
		InstallmentsTotal: raw.InstallmentsTotal,
		Description:       raw.Description,
		CategoryID:        raw.CategoryID.String(),
		PurchasedAt:       purchasedAt,
		Version:           raw.Version,
	}
	if raw.SubcategoryID != nil {
		rawCmd.SubcategoryID = raw.SubcategoryID.String()
	}

	cmd, err := commands.NewUpdateCardPurchase(rawCmd, principal.UserID)
	if err != nil {
		return output.CardPurchase{}, err
	}

	var subPtr *uuid.UUID
	if sub, ok2 := cmd.SubcategoryID.Get(); ok2 {
		v := sub.UUID()
		subPtr = &v
	}
	catSnapshot, err := uc.categoryValidator.Validate(ctx, cmd.CategoryID.UUID(), subPtr)
	if err != nil {
		return output.CardPurchase{}, fmt.Errorf("transactions/update_card_purchase: validar categoria: %w", err)
	}

	eventID, _ := uuid.Parse(uc.idGen.NewID())

	var result entities.CardPurchase
	var refMonthsAffected []string

	_, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (entities.CardPurchase, error) {
		p, affected, txErr := uc.updateInTx(ctx, db, cmd, catSnapshot, eventID, principal.UserID)
		if txErr != nil {
			return entities.CardPurchase{}, txErr
		}
		result = p
		refMonthsAffected = affected
		return p, nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return output.CardPurchase{}, execErr
	}

	return output.CardPurchaseFrom(&result, nil, refMonthsAffected), nil
}

func (uc *UpdateCardPurchase) updateInTx(
	ctx context.Context,
	db database.DBTX,
	cmd commands.UpdateCardPurchase,
	catSnapshot interfaces.CategorySnapshot,
	eventID uuid.UUID,
	userID uuid.UUID,
) (entities.CardPurchase, []string, error) {
	purchasesRepo := uc.factory.CardPurchaseRepository(db)
	invoicesRepo := uc.factory.CardInvoiceRepository(db)

	current, getErr := purchasesRepo.GetByID(ctx, cmd.PurchaseID, userID)
	if getErr != nil {
		return entities.CardPurchase{}, nil, fmt.Errorf("transactions/update_card_purchase: obter compra: %w", getErr)
	}

	currentItems := loadCurrentItemsForPurchase(ctx, invoicesRepo, current, userID)
	decision := uc.workflow.DecideUpdate(*current, currentItems, cmd, eventID, time.Now().UTC())
	decision.Purchase.UpdateNameSnapshots(catSnapshot.Name, catSnapshot.ParentName)

	if updateErr := purchasesRepo.UpdateWithVersion(ctx, &decision.Purchase, cmd.Version); updateErr != nil {
		return entities.CardPurchase{}, nil, fmt.Errorf("transactions/update_card_purchase: atualizar compra: %w", updateErr)
	}

	resolver := services.BillingCycleResolver{}
	_, closings, dues := resolver.Resolve(cmd.PurchasedAt, current.BillingSnapshot(), cmd.Installments)

	newItems, buildErr := buildNewItemsForUpdate(ctx, invoicesRepo, userID, current.CardID().UUID(), decision.Items, closings, dues)
	if buildErr != nil {
		return entities.CardPurchase{}, nil, buildErr
	}

	if replaceErr := purchasesRepo.ReplaceItems(ctx, decision.Purchase.ID(), newItems); replaceErr != nil {
		return entities.CardPurchase{}, nil, fmt.Errorf("transactions/update_card_purchase: replace items: %w", replaceErr)
	}

	evt, evtOk := decision.Event.(entities.CardPurchaseUpdated)
	if !evtOk {
		return entities.CardPurchase{}, nil, fmt.Errorf("transactions/update_card_purchase: tipo de evento inesperado")
	}

	if deltaErr := applyInvoiceDeltas(ctx, invoicesRepo, userID, current.CardID().UUID(), evt.InvoiceDeltas); deltaErr != nil {
		return entities.CardPurchase{}, nil, deltaErr
	}

	if pubErr := uc.publisher.PublishUpdated(ctx, db, evt); pubErr != nil {
		return entities.CardPurchase{}, nil, fmt.Errorf("transactions/update_card_purchase: publicar evento: %w", pubErr)
	}

	affected := make([]string, len(evt.RefMonthsAffected))
	for i, rm := range evt.RefMonthsAffected {
		affected[i] = rm.String()
	}
	return decision.Purchase, affected, nil
}

func buildNewItemsForUpdate(
	ctx context.Context,
	invoicesRepo interfaces.CardInvoiceRepository,
	userID, cardID uuid.UUID,
	decisionItems []entities.CardInvoiceItem,
	closings, dues []time.Time,
) ([]*entities.CardInvoiceItem, error) {
	newItems := make([]*entities.CardInvoiceItem, len(decisionItems))
	for i := range decisionItems {
		item := decisionItems[i]
		inv, upsertErr := invoicesRepo.UpsertByMonth(ctx, userID, cardID, item.RefMonth(), closings[i], dues[i])
		if upsertErr != nil {
			return nil, fmt.Errorf("transactions/update_card_purchase: upsert fatura [%d]: %w", i, upsertErr)
		}
		itemWithInvoice := entities.NewCardInvoiceItem(
			item.ID(), inv.ID(), item.PurchaseID(), item.UserID(),
			item.RefMonth(), item.InstallmentIndex(), item.Amount(), item.CreatedAt(),
		)
		newItems[i] = &itemWithInvoice
	}
	return newItems, nil
}

func applyInvoiceDeltas(
	ctx context.Context,
	invoicesRepo interfaces.CardInvoiceRepository,
	userID, cardID uuid.UUID,
	invoiceDeltas map[string]int64,
) error {
	for refMonthStr, delta := range invoiceDeltas {
		if delta == 0 {
			continue
		}
		rm, rmErr := valueobjects.NewRefMonth(refMonthStr)
		if rmErr != nil {
			return fmt.Errorf("transactions/update_card_purchase: ref_month inválido: %w", rmErr)
		}
		inv, _, invErr := invoicesRepo.GetByMonth(ctx, userID, cardID, rm)
		if invErr != nil {
			return fmt.Errorf("transactions/update_card_purchase: obter fatura para delta [%s]: %w", refMonthStr, invErr)
		}
		if applyErr := invoicesRepo.ApplyDelta(ctx, inv.ID(), delta, inv.Version()); applyErr != nil {
			return fmt.Errorf("transactions/update_card_purchase: apply delta [%s]: %w", refMonthStr, applyErr)
		}
	}
	return nil
}

func loadCurrentItemsForPurchase(ctx context.Context, repo interfaces.CardInvoiceRepository, purchase *entities.CardPurchase, userID uuid.UUID) []entities.CardInvoiceItem {
	resolver := services.BillingCycleResolver{}
	refMonths, _, _ := resolver.Resolve(purchase.PurchasedAt(), purchase.BillingSnapshot(), purchase.InstallmentsTotal())

	var result []entities.CardInvoiceItem
	for _, rm := range refMonths {
		_, items, err := repo.GetByMonth(ctx, userID, purchase.CardID().UUID(), rm)
		if err != nil {
			continue
		}
		for _, item := range items {
			if item.PurchaseID() == purchase.ID() {
				result = append(result, *item)
			}
		}
	}
	return result
}
