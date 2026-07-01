package binding

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	txinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	txoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	txusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

type transactionsLedgerAdapter struct {
	createTx       *txusecases.CreateTransaction
	createCP       *txusecases.CreateCardPurchase
	updateTx       *txusecases.UpdateTransaction
	deleteTx       *txusecases.DeleteTransaction
	updateCP       *txusecases.UpdateCardPurchase
	deleteCP       *txusecases.DeleteCardPurchase
	listMonthlyE   *txusecases.ListMonthlyEntries
	getMonthlySumm *txusecases.GetMonthlySummary
	o11y           observability.Observability
}

func NewTransactionsLedgerAdapter(
	createTx *txusecases.CreateTransaction,
	createCP *txusecases.CreateCardPurchase,
	updateTx *txusecases.UpdateTransaction,
	deleteTx *txusecases.DeleteTransaction,
	updateCP *txusecases.UpdateCardPurchase,
	deleteCP *txusecases.DeleteCardPurchase,
	listMonthlyE *txusecases.ListMonthlyEntries,
	getMonthlySumm *txusecases.GetMonthlySummary,
	o11y observability.Observability,
) agentsifaces.TransactionsLedger {
	return &transactionsLedgerAdapter{
		createTx:       createTx,
		createCP:       createCP,
		updateTx:       updateTx,
		deleteTx:       deleteTx,
		updateCP:       updateCP,
		deleteCP:       deleteCP,
		listMonthlyE:   listMonthlyE,
		getMonthlySumm: getMonthlySumm,
		o11y:           o11y,
	}
}

func (a *transactionsLedgerAdapter) CreateTransaction(ctx context.Context, in agentsifaces.RawTransaction) (agentsifaces.EntryRef, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.create_transaction")
	defer span.End()

	out, err := a.createTx.Execute(ctx, txinput.RawCreateTransaction{
		Direction:       in.Direction,
		PaymentMethod:   in.PaymentMethod,
		AmountCents:     in.AmountCents,
		Description:     in.Description,
		CategoryID:      in.CategoryID,
		SubcategoryID:   in.SubcategoryID,
		OccurredAt:      in.OccurredAt,
		OriginWamid:     in.OriginWamid,
		OriginItemSeq:   in.OriginItemSeq,
		OriginOperation: in.OriginOperation,
	})
	if err != nil {
		span.RecordError(err)
		return agentsifaces.EntryRef{}, fmt.Errorf("agents/binding/transactions_ledger: criar transação: %w", err)
	}
	return agentsifaces.EntryRef{ID: out.ID, Kind: "transaction", Reconciled: out.Reconciled}, nil
}

func (a *transactionsLedgerAdapter) CreateCardPurchase(ctx context.Context, in agentsifaces.RawCardPurchase) (agentsifaces.EntryRef, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.create_card_purchase")
	defer span.End()

	out, err := a.createCP.Execute(ctx, txinput.RawCreateCardPurchase{
		CardID:            in.CardID,
		TotalAmountCents:  in.TotalAmountCents,
		InstallmentsTotal: in.InstallmentsTotal,
		Description:       in.Description,
		CategoryID:        in.CategoryID,
		SubcategoryID:     in.SubcategoryID,
		PurchasedAt:       in.PurchasedAt,
		OriginWamid:       in.OriginWamid,
		OriginItemSeq:     in.OriginItemSeq,
		OriginOperation:   in.OriginOperation,
	})
	if err != nil {
		span.RecordError(err)
		return agentsifaces.EntryRef{}, fmt.Errorf("agents/binding/transactions_ledger: criar compra cartão: %w", err)
	}
	return agentsifaces.EntryRef{ID: out.ID, Kind: "card_purchase", Reconciled: out.Reconciled}, nil
}

func (a *transactionsLedgerAdapter) UpdateTransaction(ctx context.Context, in agentsifaces.RawUpdateTransaction) (agentsifaces.EntryRef, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.update_transaction")
	defer span.End()

	out, err := a.updateTx.Execute(ctx, in.ID.String(), txinput.RawUpdateTransaction{
		Direction:     in.Direction,
		PaymentMethod: in.PaymentMethod,
		AmountCents:   in.AmountCents,
		Description:   in.Description,
		CategoryID:    in.CategoryID,
		SubcategoryID: in.SubcategoryID,
		OccurredAt:    in.OccurredAt,
		Version:       in.Version,
	})
	if err != nil {
		span.RecordError(err)
		return agentsifaces.EntryRef{}, fmt.Errorf("agents/binding/transactions_ledger: atualizar transação: %w", err)
	}
	return agentsifaces.EntryRef{ID: out.ID, Kind: "transaction"}, nil
}

func (a *transactionsLedgerAdapter) DeleteTransaction(ctx context.Context, ref agentsifaces.EntryRef, version int64) error {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.delete_transaction")
	defer span.End()

	if err := a.deleteTx.Execute(ctx, ref.ID.String(), version); err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents/binding/transactions_ledger: deletar transação: %w", err)
	}
	return nil
}

func (a *transactionsLedgerAdapter) UpdateCardPurchase(ctx context.Context, in agentsifaces.RawUpdateCardPurchase) (agentsifaces.EntryRef, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.update_card_purchase")
	defer span.End()

	out, err := a.updateCP.Execute(ctx, in.ID, txinput.RawUpdateCardPurchase{
		TotalAmountCents:  in.TotalAmountCents,
		InstallmentsTotal: in.InstallmentsTotal,
		Description:       in.Description,
		CategoryID:        in.CategoryID,
		SubcategoryID:     in.SubcategoryID,
		PurchasedAt:       in.PurchasedAt,
		Version:           in.Version,
	})
	if err != nil {
		span.RecordError(err)
		return agentsifaces.EntryRef{}, fmt.Errorf("agents/binding/transactions_ledger: atualizar compra cartão: %w", err)
	}
	return agentsifaces.EntryRef{ID: out.ID, Kind: "card_purchase"}, nil
}

func (a *transactionsLedgerAdapter) DeleteCardPurchase(ctx context.Context, ref agentsifaces.EntryRef, version int64) error {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.delete_card_purchase")
	defer span.End()

	if err := a.deleteCP.Execute(ctx, ref.ID, version); err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents/binding/transactions_ledger: deletar compra cartão: %w", err)
	}
	return nil
}

func (a *transactionsLedgerAdapter) ListMonthlyEntries(ctx context.Context, _ uuid.UUID, refMonth string, cursor string, limit int) ([]agentsifaces.MonthlyEntry, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.list_monthly_entries")
	defer span.End()

	page, err := a.listMonthlyE.Execute(ctx, refMonth, cursor, limit)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("agents/binding/transactions_ledger: listar entradas mensais: %w", err)
	}

	entries := make([]agentsifaces.MonthlyEntry, 0, len(page.Items))
	for _, item := range page.Items {
		e, ok := item.(txoutput.MonthlyEntry)
		if !ok {
			continue
		}
		entries = append(entries, agentsifaces.MonthlyEntry{
			Kind:        e.Kind,
			ID:          e.ID,
			RefMonth:    e.RefMonth,
			AmountCents: e.AmountCents,
			Direction:   e.Direction,
			Description: e.Description,
			CreatedAt:   e.CreatedAt,
		})
	}
	return entries, nil
}

func (a *transactionsLedgerAdapter) GetMonthlySummary(ctx context.Context, _ uuid.UUID, refMonth string) (agentsifaces.MonthlySummary, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.get_monthly_summary")
	defer span.End()

	out, err := a.getMonthlySumm.Execute(ctx, refMonth)
	if err != nil {
		span.RecordError(err)
		return agentsifaces.MonthlySummary{}, fmt.Errorf("agents/binding/transactions_ledger: resumo mensal: %w", err)
	}
	return agentsifaces.MonthlySummary{
		RefMonth:     out.RefMonth,
		IncomeCents:  out.IncomeCents,
		OutcomeCents: out.OutcomeCents,
		TotalCents:   out.TotalCents,
	}, nil
}
