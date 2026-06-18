package services

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type CardPurchaseDecision struct {
	Purchase entities.CardPurchase
	Items    []entities.CardInvoiceItem
	Event    any
}

type CardPurchaseWorkflow struct {
	splitter BillingCycleResolver
}

func NewCardPurchaseWorkflow() CardPurchaseWorkflow {
	return CardPurchaseWorkflow{splitter: BillingCycleResolver{}}
}

func (w CardPurchaseWorkflow) DecideCreate(
	cmd commands.CreateCardPurchase,
	snapshot valueobjects.CardBillingSnapshot,
	purchaseID uuid.UUID,
	eventID uuid.UUID,
	now time.Time,
) CardPurchaseDecision {
	amounts := InstallmentSplitter{}.Split(cmd.TotalAmount, cmd.Installments)
	refMonths, _, _ := w.splitter.Resolve(cmd.PurchasedAt, snapshot, cmd.Installments)

	purchase := entities.NewCardPurchase(
		purchaseID,
		cmd.UserID,
		cmd.CardID,
		cmd.TotalAmount,
		cmd.Installments,
		cmd.Description,
		cmd.CategoryID,
		cmd.SubcategoryID,
		"",
		"",
		cmd.PurchasedAt,
		snapshot,
		now,
	)

	items := make([]entities.CardInvoiceItem, len(amounts))
	for i, amt := range amounts {
		items[i] = entities.NewCardInvoiceItem(
			uuid.New(),
			uuid.Nil,
			purchaseID,
			cmd.UserID,
			refMonths[i],
			i+1,
			amt,
			now,
		)
	}

	subcategoryID := uuid.Nil
	if sub, ok := cmd.SubcategoryID.Get(); ok {
		subcategoryID = sub.UUID()
	}

	installments := make([]entities.CardPurchaseInstallment, len(items))
	for i, it := range items {
		installments[i] = entities.CardPurchaseInstallment{
			ItemID:      it.ID(),
			RefMonth:    it.RefMonth(),
			AmountCents: it.Amount().Cents(),
			Index:       it.InstallmentIndex(),
		}
	}

	evt := entities.CardPurchaseCreated{
		EventID:           eventID,
		AggregateID:       purchaseID,
		UserID:            cmd.UserID.UUID(),
		OccurredAt:        now,
		CardID:            cmd.CardID.UUID(),
		SubcategoryID:     subcategoryID,
		TotalAmountCents:  cmd.TotalAmount.Cents(),
		InstallmentsTotal: cmd.Installments.Value(),
		RefMonthsAffected: refMonths,
		Installments:      installments,
	}

	return CardPurchaseDecision{Purchase: purchase, Items: items, Event: evt}
}

func (w CardPurchaseWorkflow) DecideUpdate(
	current entities.CardPurchase,
	currentItems []entities.CardInvoiceItem,
	cmd commands.UpdateCardPurchase,
	eventID uuid.UUID,
	now time.Time,
) CardPurchaseDecision {
	snapshot := current.BillingSnapshot()
	newAmounts := InstallmentSplitter{}.Split(cmd.TotalAmount, cmd.Installments)
	newRefMonths, _, _ := w.splitter.Resolve(cmd.PurchasedAt, snapshot, cmd.Installments)

	oldRefMonths := make([]valueobjects.RefMonth, len(currentItems))
	for i, item := range currentItems {
		oldRefMonths[i] = item.RefMonth()
	}

	allRefMonths := dedupeRefMonths(append(oldRefMonths, newRefMonths...))
	sortRefMonths(allRefMonths)

	oldByRef := make(map[string]int64, len(currentItems))
	for _, item := range currentItems {
		oldByRef[item.RefMonth().String()] += item.Amount().Cents()
	}

	newByRef := make(map[string]int64, len(newRefMonths))
	for i, ref := range newRefMonths {
		newByRef[ref.String()] += newAmounts[i].Cents()
	}

	invoiceDeltas := make(map[string]int64, len(allRefMonths))
	for _, ref := range allRefMonths {
		delta := newByRef[ref.String()] - oldByRef[ref.String()]
		invoiceDeltas[ref.String()] = delta
	}

	current.Update(
		cmd.TotalAmount,
		cmd.Installments,
		cmd.Description,
		cmd.CategoryID,
		cmd.SubcategoryID,
		"",
		"",
		cmd.PurchasedAt,
		now,
	)

	newItems := make([]entities.CardInvoiceItem, len(newAmounts))
	for i, amt := range newAmounts {
		newItems[i] = entities.NewCardInvoiceItem(
			uuid.New(),
			uuid.Nil,
			current.ID(),
			cmd.UserID,
			newRefMonths[i],
			i+1,
			amt,
			now,
		)
	}

	evt := entities.CardPurchaseUpdated{
		EventID:           eventID,
		AggregateID:       current.ID(),
		UserID:            cmd.UserID.UUID(),
		OccurredAt:        now,
		CardID:            current.CardID().UUID(),
		TotalAmountCents:  cmd.TotalAmount.Cents(),
		InstallmentsTotal: cmd.Installments.Value(),
		RefMonthsAffected: allRefMonths,
		InvoiceDeltas:     invoiceDeltas,
	}

	return CardPurchaseDecision{Purchase: current, Items: newItems, Event: evt}
}

func (w CardPurchaseWorkflow) DecideDelete(
	current entities.CardPurchase,
	currentItems []entities.CardInvoiceItem,
	eventID uuid.UUID,
	now time.Time,
) (CardPurchaseDecision, error) {
	if err := current.SoftDelete(now); err != nil {
		return CardPurchaseDecision{}, fmt.Errorf("transactions/card_purchase_workflow: %w", err)
	}

	refMonths := make([]valueobjects.RefMonth, len(currentItems))
	invoiceDeltas := make(map[string]int64, len(currentItems))
	for i, item := range currentItems {
		refMonths[i] = item.RefMonth()
		invoiceDeltas[item.RefMonth().String()] -= item.Amount().Cents()
	}

	unique := dedupeRefMonths(refMonths)
	sortRefMonths(unique)

	evt := entities.CardPurchaseDeleted{
		EventID:           eventID,
		AggregateID:       current.ID(),
		UserID:            current.UserID().UUID(),
		OccurredAt:        now,
		CardID:            current.CardID().UUID(),
		RefMonthsAffected: unique,
		InvoiceDeltas:     invoiceDeltas,
	}

	return CardPurchaseDecision{Purchase: current, Items: nil, Event: evt}, nil
}

func sortRefMonths(months []valueobjects.RefMonth) {
	sort.Slice(months, func(i, j int) bool {
		return months[i].String() < months[j].String()
	})
}
