package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func installmentCountOrSingle(opt option.Option[valueobjects.InstallmentCount]) valueobjects.InstallmentCount {
	if ic, ok := opt.Get(); ok {
		return ic
	}
	single, _ := valueobjects.NewInstallmentCount(1)
	return single
}

func newInvoiceItemIDs(pm valueobjects.PaymentMethod, installments option.Option[valueobjects.InstallmentCount]) []uuid.UUID {
	if pm != valueobjects.PaymentMethodCreditCard {
		return nil
	}
	count := installmentCountOrSingle(installments).Value()
	ids := make([]uuid.UUID, count)
	for i := range ids {
		ids[i] = uuid.New()
	}
	return ids
}

func refMonthsAffectedCount(evt any) int {
	switch e := evt.(type) {
	case entities.TransactionCreated:
		return len(e.RefMonthsAffected)
	case entities.TransactionUpdated:
		return len(e.RefMonthsAffected)
	case entities.TransactionDeleted:
		return len(e.RefMonthsAffected)
	default:
		return 0
	}
}

func createInvoiceItems(
	ctx context.Context,
	invoiceRepo interfaces.CardInvoiceRepository,
	userID, cardID, txID uuid.UUID,
	decisionItems []entities.CardInvoiceItem,
	closings, dues []time.Time,
) ([]*entities.CardInvoiceItem, error) {
	items := make([]*entities.CardInvoiceItem, len(decisionItems))
	for i := range decisionItems {
		item := decisionItems[i]
		inv, upsertErr := invoiceRepo.UpsertByMonth(ctx, userID, cardID, item.RefMonth(), closings[i], dues[i])
		if upsertErr != nil {
			return nil, fmt.Errorf("transactions/card_invoice: upsert fatura [%d]: %w", i, upsertErr)
		}
		withInvoice := entities.NewCardInvoiceItem(
			item.ID(), inv.ID(), txID, item.UserID(),
			item.RefMonth(), item.InstallmentIndex(), item.Amount(), item.CreatedAt(),
		)
		items[i] = &withInvoice
		if deltaErr := invoiceRepo.ApplyDelta(ctx, inv.ID(), item.Amount().Cents(), inv.Version()); deltaErr != nil {
			return nil, fmt.Errorf("transactions/card_invoice: apply delta [%d]: %w", i, deltaErr)
		}
	}
	return items, nil
}

func rebuildInvoiceItems(
	ctx context.Context,
	invoiceRepo interfaces.CardInvoiceRepository,
	userID, cardID, txID uuid.UUID,
	decisionItems []entities.CardInvoiceItem,
	closings, dues []time.Time,
) ([]*entities.CardInvoiceItem, error) {
	items := make([]*entities.CardInvoiceItem, len(decisionItems))
	for i := range decisionItems {
		item := decisionItems[i]
		inv, upsertErr := invoiceRepo.UpsertByMonth(ctx, userID, cardID, item.RefMonth(), closings[i], dues[i])
		if upsertErr != nil {
			return nil, fmt.Errorf("transactions/card_invoice: upsert fatura [%d]: %w", i, upsertErr)
		}
		withInvoice := entities.NewCardInvoiceItem(
			item.ID(), inv.ID(), txID, item.UserID(),
			item.RefMonth(), item.InstallmentIndex(), item.Amount(), item.CreatedAt(),
		)
		items[i] = &withInvoice
	}
	return items, nil
}

func applyInvoiceDeltasByMonth(
	ctx context.Context,
	invoiceRepo interfaces.CardInvoiceRepository,
	userID, cardID uuid.UUID,
	deltas map[string]int64,
) error {
	for refMonthStr, delta := range deltas {
		if delta == 0 {
			continue
		}
		rm, rmErr := valueobjects.NewRefMonth(refMonthStr)
		if rmErr != nil {
			return fmt.Errorf("transactions/card_invoice: ref_month inválido [%s]: %w", refMonthStr, rmErr)
		}
		inv, _, invErr := invoiceRepo.GetByMonth(ctx, userID, cardID, rm)
		if invErr != nil {
			return fmt.Errorf("transactions/card_invoice: obter fatura para delta [%s]: %w", refMonthStr, invErr)
		}
		if applyErr := invoiceRepo.ApplyDelta(ctx, inv.ID(), delta, inv.Version()); applyErr != nil {
			return fmt.Errorf("transactions/card_invoice: apply delta [%s]: %w", refMonthStr, applyErr)
		}
	}
	return nil
}

func derefInvoiceItems(items []*entities.CardInvoiceItem) []entities.CardInvoiceItem {
	result := make([]entities.CardInvoiceItem, len(items))
	for i, it := range items {
		result[i] = *it
	}
	return result
}
