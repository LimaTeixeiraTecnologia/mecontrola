package services

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type TransactionDecision struct {
	Transaction   entities.Transaction
	Items         []entities.CardInvoiceItem
	InvoiceDeltas map[string]int64
	Event         any
}

type TransactionWorkflow struct {
	cards BillingCycleResolver
}

func NewTransactionWorkflow() TransactionWorkflow {
	return TransactionWorkflow{cards: BillingCycleResolver{}}
}

func (w TransactionWorkflow) DecideCreate(
	cmd commands.CreateTransaction,
	snap option.Option[valueobjects.CardBillingSnapshot],
	txID uuid.UUID,
	eventID uuid.UUID,
	itemIDs []uuid.UUID,
	now time.Time,
) TransactionDecision {
	refMonth := valueobjects.RefMonthFromTime(cmd.OccurredAt, time.UTC)

	subcategoryID := uuid.Nil
	if sub, ok := cmd.SubcategoryID.Get(); ok {
		subcategoryID = sub.UUID()
	}

	tx := entities.NewTransaction(
		txID,
		cmd.UserID,
		cmd.Direction,
		cmd.PaymentMethod,
		cmd.Amount,
		cmd.Description,
		cmd.CategoryID,
		cmd.SubcategoryID,
		"",
		"",
		refMonth,
		cmd.OccurredAt,
		now,
	)

	snapshot, isCard := snap.Get()
	if !isCard || cmd.PaymentMethod != valueobjects.PaymentMethodCreditCard {
		evt := entities.TransactionCreated{
			EventID:           eventID,
			AggregateID:       txID,
			UserID:            cmd.UserID.UUID(),
			OccurredAt:        now,
			Direction:         cmd.Direction,
			PaymentMethod:     cmd.PaymentMethod,
			AmountCents:       cmd.Amount.Cents(),
			RefMonth:          refMonth,
			CategoryID:        cmd.CategoryID.UUID(),
			SubcategoryID:     subcategoryID,
			RefMonthsAffected: []valueobjects.RefMonth{refMonth},
		}
		return TransactionDecision{Transaction: tx, Event: evt}
	}

	installments := installmentsOrSingle(cmd.Installments)
	tx.SetCardBilling(cardIDOrNil(cmd.CardID), installments, snapshot)

	amounts := InstallmentSplitter{}.Split(cmd.Amount, installments)
	refMonths, _, _ := w.cards.Resolve(cmd.OccurredAt, snapshot, installments)

	items := make([]entities.CardInvoiceItem, len(amounts))
	deltas := make(map[string]int64, len(amounts))
	for i, amt := range amounts {
		items[i] = entities.NewCardInvoiceItem(
			itemIDs[i],
			uuid.Nil,
			txID,
			cmd.UserID,
			refMonths[i],
			i+1,
			amt,
			now,
		)
		deltas[refMonths[i].String()] += amt.Cents()
	}

	installmentsEvt := make([]entities.CardPurchaseInstallment, len(items))
	for i, it := range items {
		installmentsEvt[i] = entities.CardPurchaseInstallment{
			ItemID:      it.ID(),
			RefMonth:    it.RefMonth(),
			AmountCents: it.Amount().Cents(),
			Index:       it.InstallmentIndex(),
		}
	}

	affected := dedupeRefMonths(refMonths)
	sortRefMonths(affected)

	evt := entities.TransactionCreated{
		EventID:           eventID,
		AggregateID:       txID,
		UserID:            cmd.UserID.UUID(),
		OccurredAt:        now,
		Direction:         cmd.Direction,
		PaymentMethod:     cmd.PaymentMethod,
		AmountCents:       cmd.Amount.Cents(),
		RefMonth:          refMonth,
		CategoryID:        cmd.CategoryID.UUID(),
		SubcategoryID:     subcategoryID,
		RefMonthsAffected: affected,
		Installments:      installmentsEvt,
	}

	return TransactionDecision{Transaction: tx, Items: items, InvoiceDeltas: deltas, Event: evt}
}

func (w TransactionWorkflow) DecideUpdate(
	current entities.Transaction,
	currentItems []entities.CardInvoiceItem,
	cmd commands.UpdateTransaction,
	eventID uuid.UUID,
	itemIDs []uuid.UUID,
	now time.Time,
) TransactionDecision {
	newRefMonth := valueobjects.RefMonthFromTime(cmd.OccurredAt, time.UTC)
	snapshot, isCard := current.BillingSnapshot().Get()

	if !isCard || cmd.PaymentMethod != valueobjects.PaymentMethodCreditCard {
		oldRefMonth := current.RefMonth()
		current.Update(
			cmd.Direction,
			cmd.PaymentMethod,
			cmd.Amount,
			cmd.Description,
			cmd.CategoryID,
			cmd.SubcategoryID,
			"",
			"",
			newRefMonth,
			cmd.OccurredAt,
			now,
		)
		affected := dedupeRefMonths([]valueobjects.RefMonth{oldRefMonth, newRefMonth})
		evt := entities.TransactionUpdated{
			EventID:           eventID,
			AggregateID:       current.ID(),
			UserID:            cmd.UserID.UUID(),
			OccurredAt:        now,
			Direction:         cmd.Direction,
			PaymentMethod:     cmd.PaymentMethod,
			AmountCents:       cmd.Amount.Cents(),
			RefMonth:          newRefMonth,
			RefMonthsAffected: affected,
		}
		return TransactionDecision{Transaction: current, Event: evt}
	}

	installments := installmentsOrSingle(cmd.Installments)
	newAmounts := InstallmentSplitter{}.Split(cmd.Amount, installments)
	newRefMonths, _, _ := w.cards.Resolve(cmd.OccurredAt, snapshot, installments)

	oldRefMonths := make([]valueobjects.RefMonth, len(currentItems))
	oldByRef := make(map[string]int64, len(currentItems))
	for i, item := range currentItems {
		oldRefMonths[i] = item.RefMonth()
		oldByRef[item.RefMonth().String()] += item.Amount().Cents()
	}

	newByRef := make(map[string]int64, len(newRefMonths))
	for i, ref := range newRefMonths {
		newByRef[ref.String()] += newAmounts[i].Cents()
	}

	affected := dedupeRefMonths(append(oldRefMonths, newRefMonths...))
	sortRefMonths(affected)

	deltas := make(map[string]int64, len(affected))
	for _, ref := range affected {
		deltas[ref.String()] = newByRef[ref.String()] - oldByRef[ref.String()]
	}

	cardID := cardIDOrNil(cmd.CardID)
	if !cmd.CardID.IsPresent() {
		if cur, ok := current.CardID().Get(); ok {
			cardID = cur
		}
	}

	current.Update(
		cmd.Direction,
		cmd.PaymentMethod,
		cmd.Amount,
		cmd.Description,
		cmd.CategoryID,
		cmd.SubcategoryID,
		"",
		"",
		newRefMonth,
		cmd.OccurredAt,
		now,
	)
	current.SetCardBilling(cardID, installments, snapshot)

	newItems := make([]entities.CardInvoiceItem, len(newAmounts))
	for i, amt := range newAmounts {
		newItems[i] = entities.NewCardInvoiceItem(
			itemIDs[i],
			uuid.Nil,
			current.ID(),
			cmd.UserID,
			newRefMonths[i],
			i+1,
			amt,
			now,
		)
	}

	evt := entities.TransactionUpdated{
		EventID:           eventID,
		AggregateID:       current.ID(),
		UserID:            cmd.UserID.UUID(),
		OccurredAt:        now,
		Direction:         cmd.Direction,
		PaymentMethod:     cmd.PaymentMethod,
		AmountCents:       cmd.Amount.Cents(),
		RefMonth:          newRefMonth,
		RefMonthsAffected: affected,
	}

	return TransactionDecision{Transaction: current, Items: newItems, InvoiceDeltas: deltas, Event: evt}
}

func (w TransactionWorkflow) DecideDelete(
	current entities.Transaction,
	currentItems []entities.CardInvoiceItem,
	eventID uuid.UUID,
	now time.Time,
) (TransactionDecision, error) {
	if err := current.SoftDelete(now); err != nil {
		return TransactionDecision{}, fmt.Errorf("transactions/transaction_workflow: %w", err)
	}

	if len(currentItems) == 0 {
		refMonth := current.RefMonth()
		evt := entities.TransactionDeleted{
			EventID:           eventID,
			AggregateID:       current.ID(),
			UserID:            current.UserID().UUID(),
			OccurredAt:        now,
			RefMonth:          refMonth,
			RefMonthsAffected: []valueobjects.RefMonth{refMonth},
		}
		return TransactionDecision{Transaction: current, Event: evt}, nil
	}

	refMonths := make([]valueobjects.RefMonth, len(currentItems))
	deltas := make(map[string]int64, len(currentItems))
	for i, item := range currentItems {
		refMonths[i] = item.RefMonth()
		deltas[item.RefMonth().String()] -= item.Amount().Cents()
	}

	affected := dedupeRefMonths(refMonths)
	sortRefMonths(affected)

	evt := entities.TransactionDeleted{
		EventID:           eventID,
		AggregateID:       current.ID(),
		UserID:            current.UserID().UUID(),
		OccurredAt:        now,
		RefMonth:          current.RefMonth(),
		RefMonthsAffected: affected,
	}

	return TransactionDecision{Transaction: current, InvoiceDeltas: deltas, Event: evt}, nil
}

func installmentsOrSingle(opt option.Option[valueobjects.InstallmentCount]) valueobjects.InstallmentCount {
	if ic, ok := opt.Get(); ok {
		return ic
	}
	single, _ := valueobjects.NewInstallmentCount(1)
	return single
}

func cardIDOrNil(opt option.Option[valueobjects.CardID]) valueobjects.CardID {
	if cid, ok := opt.Get(); ok {
		return cid
	}
	return valueobjects.CardIDFromUUID(uuid.Nil)
}

func dedupeRefMonths(months []valueobjects.RefMonth) []valueobjects.RefMonth {
	seen := make(map[string]struct{}, len(months))
	result := make([]valueobjects.RefMonth, 0, len(months))
	for _, m := range months {
		if _, ok := seen[m.String()]; !ok {
			seen[m.String()] = struct{}{}
			result = append(result, m)
		}
	}
	return result
}

func sortRefMonths(months []valueobjects.RefMonth) {
	slices.SortFunc(months, func(a, b valueobjects.RefMonth) int {
		return strings.Compare(a.String(), b.String())
	})
}
