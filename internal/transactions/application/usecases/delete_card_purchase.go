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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type DeleteCardPurchase struct {
	factory   interfaces.RepositoryFactory
	workflow  *services.CardPurchaseWorkflow
	publisher interfaces.CardPurchaseEventPublisher
	uow       uow.UnitOfWork
	idGen     id.Generator
	o11y      observability.Observability
}

func NewDeleteCardPurchase(
	factory interfaces.RepositoryFactory,
	workflow *services.CardPurchaseWorkflow,
	publisher interfaces.CardPurchaseEventPublisher,
	u uow.UnitOfWork,
	idGen id.Generator,
	o11y observability.Observability,
) *DeleteCardPurchase {
	return &DeleteCardPurchase{
		factory:   factory,
		workflow:  workflow,
		publisher: publisher,
		uow:       u,
		idGen:     idGen,
		o11y:      o11y,
	}
}

func (uc *DeleteCardPurchase) Execute(ctx context.Context, purchaseID uuid.UUID, version int64) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.delete_card_purchase")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok || principal.IsZero() {
		return ErrUsecaseUnauthorized
	}

	eventID, _ := uuid.Parse(uc.idGen.NewID())

	_, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (entities.CardPurchase, error) {
		purchasesRepo := uc.factory.CardPurchaseRepository(db)
		invoicesRepo := uc.factory.CardInvoiceRepository(db)

		current, getErr := purchasesRepo.GetByID(ctx, purchaseID, principal.UserID)
		if getErr != nil {
			return entities.CardPurchase{}, fmt.Errorf("transactions/delete_card_purchase: obter compra: %w", getErr)
		}

		currentItems := loadCurrentItemsForPurchase(ctx, invoicesRepo, current, principal.UserID)

		decision, decideErr := uc.workflow.DecideDelete(*current, currentItems, eventID, time.Now().UTC())
		if decideErr != nil {
			return entities.CardPurchase{}, fmt.Errorf("transactions/delete_card_purchase: %w", decideErr)
		}

		if softDeleteErr := purchasesRepo.SoftDelete(ctx, purchaseID, principal.UserID, version, time.Now().UTC()); softDeleteErr != nil {
			return entities.CardPurchase{}, fmt.Errorf("transactions/delete_card_purchase: deletar compra: %w", softDeleteErr)
		}

		if replaceErr := purchasesRepo.ReplaceItems(ctx, purchaseID, nil); replaceErr != nil {
			return entities.CardPurchase{}, fmt.Errorf("transactions/delete_card_purchase: remover items: %w", replaceErr)
		}

		evt, evtOk := decision.Event.(entities.CardPurchaseDeleted)
		if !evtOk {
			return entities.CardPurchase{}, fmt.Errorf("transactions/delete_card_purchase: tipo de evento inesperado")
		}

		for refMonthStr, delta := range evt.InvoiceDeltas {
			if delta == 0 {
				continue
			}
			rm, rmErr := valueobjects.NewRefMonth(refMonthStr)
			if rmErr != nil {
				return entities.CardPurchase{}, fmt.Errorf("transactions/delete_card_purchase: ref_month inválido: %w", rmErr)
			}
			inv, _, invErr := invoicesRepo.GetByMonth(ctx, principal.UserID, current.CardID().UUID(), rm)
			if invErr != nil {
				return entities.CardPurchase{}, fmt.Errorf("transactions/delete_card_purchase: obter fatura para delta [%s]: %w", refMonthStr, invErr)
			}
			if applyErr := invoicesRepo.ApplyDelta(ctx, inv.ID(), delta, inv.Version()); applyErr != nil {
				return entities.CardPurchase{}, fmt.Errorf("transactions/delete_card_purchase: apply delta [%s]: %w", refMonthStr, applyErr)
			}
		}

		if pubErr := uc.publisher.PublishDeleted(ctx, db, evt); pubErr != nil {
			return entities.CardPurchase{}, fmt.Errorf("transactions/delete_card_purchase: publicar evento: %w", pubErr)
		}

		return *current, nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return execErr
	}
	return nil
}
