package binding

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	txinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	txoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	txifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
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
	getTx          *txusecases.GetTransaction
	getCP          *txusecases.GetCardPurchase
	listCP         *txusecases.ListCardPurchases
	getCardInvoice *txusecases.GetCardInvoice
	searchTx       *txusecases.SearchTransactions
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
	getTx *txusecases.GetTransaction,
	getCP *txusecases.GetCardPurchase,
	listCP *txusecases.ListCardPurchases,
	getCardInvoice *txusecases.GetCardInvoice,
	searchTx *txusecases.SearchTransactions,
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
		getTx:          getTx,
		getCP:          getCP,
		listCP:         listCP,
		getCardInvoice: getCardInvoice,
		searchTx:       searchTx,
		o11y:           o11y,
	}
}

func (a *transactionsLedgerAdapter) principalCtx(ctx context.Context) (context.Context, error) {
	if _, ok := auth.FromContext(ctx); ok {
		return ctx, nil
	}
	resourceID, _, _, ok := agent.InboundIdentityFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("agents/binding/transactions_ledger: identidade inbound ausente")
	}
	userID, err := uuid.Parse(resourceID)
	if err != nil {
		return nil, fmt.Errorf("agents/binding/transactions_ledger: userId inválido: %w", err)
	}
	return auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp}), nil
}

func (a *transactionsLedgerAdapter) CreateTransaction(ctx context.Context, in agentsifaces.RawTransaction) (agentsifaces.EntryRef, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.create_transaction")
	defer span.End()

	ctx, err := a.principalCtx(ctx)
	if err != nil {
		span.RecordError(err)
		return agentsifaces.EntryRef{}, err
	}

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

	ctx, err := a.principalCtx(ctx)
	if err != nil {
		span.RecordError(err)
		return agentsifaces.EntryRef{}, err
	}

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

	ctx, err := a.principalCtx(ctx)
	if err != nil {
		span.RecordError(err)
		return agentsifaces.EntryRef{}, err
	}

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

	ctx, err := a.principalCtx(ctx)
	if err != nil {
		span.RecordError(err)
		return err
	}

	if err := a.deleteTx.Execute(ctx, ref.ID.String(), version); err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents/binding/transactions_ledger: deletar transação: %w", err)
	}
	return nil
}

func (a *transactionsLedgerAdapter) UpdateCardPurchase(ctx context.Context, in agentsifaces.RawUpdateCardPurchase) (agentsifaces.EntryRef, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.update_card_purchase")
	defer span.End()

	ctx, err := a.principalCtx(ctx)
	if err != nil {
		span.RecordError(err)
		return agentsifaces.EntryRef{}, err
	}

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

	ctx, err := a.principalCtx(ctx)
	if err != nil {
		span.RecordError(err)
		return err
	}

	if err := a.deleteCP.Execute(ctx, ref.ID, version); err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents/binding/transactions_ledger: deletar compra cartão: %w", err)
	}
	return nil
}

func (a *transactionsLedgerAdapter) ListMonthlyEntries(ctx context.Context, _ uuid.UUID, refMonth string, cursor string, limit int) ([]agentsifaces.MonthlyEntry, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.list_monthly_entries")
	defer span.End()

	ctx, err := a.principalCtx(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

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

func (a *transactionsLedgerAdapter) GetTransaction(ctx context.Context, txID string) (agentsifaces.Entry, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.get_transaction")
	defer span.End()

	ctx, err := a.principalCtx(ctx)
	if err != nil {
		span.RecordError(err)
		return agentsifaces.Entry{}, err
	}

	out, err := a.getTx.Execute(ctx, txID)
	if err != nil {
		span.RecordError(err)
		return agentsifaces.Entry{}, fmt.Errorf("agents/binding/transactions_ledger: obter transação: %w", err)
	}
	var sub *string
	if out.SubcategoryID != nil {
		s := out.SubcategoryID.String()
		sub = &s
	}
	return agentsifaces.Entry{
		Kind:                    "transaction",
		ID:                      out.ID.String(),
		UserID:                  out.UserID.String(),
		Direction:               out.Direction,
		PaymentMethod:           out.PaymentMethod,
		AmountCents:             out.AmountCents,
		Description:             out.Description,
		CategoryID:              out.CategoryID.String(),
		SubcategoryID:           sub,
		CategoryNameSnapshot:    out.CategoryNameSnapshot,
		SubcategoryNameSnapshot: out.SubcategoryNameSnapshot,
		RefMonth:                out.RefMonth,
		OccurredAt:              out.OccurredAt,
		Version:                 out.Version,
		CreatedAt:               out.CreatedAt,
		UpdatedAt:               out.UpdatedAt,
	}, nil
}

func (a *transactionsLedgerAdapter) GetCardPurchase(ctx context.Context, purchaseID uuid.UUID) (agentsifaces.Entry, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.get_card_purchase")
	defer span.End()

	ctx, err := a.principalCtx(ctx)
	if err != nil {
		span.RecordError(err)
		return agentsifaces.Entry{}, err
	}

	out, err := a.getCP.Execute(ctx, purchaseID)
	if err != nil {
		span.RecordError(err)
		return agentsifaces.Entry{}, fmt.Errorf("agents/binding/transactions_ledger: obter compra cartão: %w", err)
	}
	var sub *string
	if out.SubcategoryID != nil {
		s := out.SubcategoryID.String()
		sub = &s
	}
	return agentsifaces.Entry{
		Kind:                    "card_purchase",
		ID:                      out.ID.String(),
		UserID:                  out.UserID.String(),
		AmountCents:             out.TotalAmountCents,
		Description:             out.Description,
		CategoryID:              out.CategoryID.String(),
		SubcategoryID:           sub,
		CategoryNameSnapshot:    out.CategoryNameSnapshot,
		SubcategoryNameSnapshot: out.SubcategoryNameSnapshot,
		Version:                 out.Version,
		CreatedAt:               out.CreatedAt,
		UpdatedAt:               out.UpdatedAt,
	}, nil
}

func (a *transactionsLedgerAdapter) ListCardPurchases(ctx context.Context, cardID uuid.UUID, refMonth, cursor string, limit int) ([]agentsifaces.Entry, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.list_card_purchases")
	defer span.End()

	ctx, err := a.principalCtx(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	page, err := a.listCP.Execute(ctx, txusecases.ListCardPurchasesInput{
		CardID: cardID,
		Cursor: txifaces.Cursor{Value: cursor},
		Limit:  limit,
	})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("agents/binding/transactions_ledger: listar compras cartão: %w", err)
	}

	entries := make([]agentsifaces.Entry, 0, len(page.Items))
	for _, cp := range page.Items {
		var sub *string
		if cp.SubcategoryID != nil {
			s := cp.SubcategoryID.String()
			sub = &s
		}
		entries = append(entries, agentsifaces.Entry{
			Kind:                    "card_purchase",
			ID:                      cp.ID.String(),
			UserID:                  cp.UserID.String(),
			AmountCents:             cp.TotalAmountCents,
			Description:             cp.Description,
			CategoryID:              cp.CategoryID.String(),
			SubcategoryID:           sub,
			CategoryNameSnapshot:    cp.CategoryNameSnapshot,
			SubcategoryNameSnapshot: cp.SubcategoryNameSnapshot,
			Version:                 cp.Version,
			CreatedAt:               cp.CreatedAt,
			UpdatedAt:               cp.UpdatedAt,
		})
	}
	return entries, nil
}

func (a *transactionsLedgerAdapter) GetCardInvoice(ctx context.Context, cardID uuid.UUID, refMonth string) (agentsifaces.CardInvoice, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.get_card_invoice")
	defer span.End()

	ctx, err := a.principalCtx(ctx)
	if err != nil {
		span.RecordError(err)
		return agentsifaces.CardInvoice{}, err
	}

	out, err := a.getCardInvoice.Execute(ctx, cardID, refMonth)
	if err != nil {
		span.RecordError(err)
		return agentsifaces.CardInvoice{}, fmt.Errorf("agents/binding/transactions_ledger: obter fatura cartão: %w", err)
	}

	items := make([]agentsifaces.CardInvoiceItem, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, agentsifaces.CardInvoiceItem{
			ID:               item.ID,
			InvoiceID:        item.InvoiceID,
			RefMonth:         item.RefMonth,
			InstallmentIndex: item.InstallmentIndex,
			AmountCents:      item.AmountCents,
		})
	}
	return agentsifaces.CardInvoice{
		ID:              out.ID,
		UserID:          out.UserID,
		CardID:          out.CardID,
		RefMonth:        out.RefMonth,
		ClosingAt:       out.ClosingAt,
		DueAt:           out.DueAt,
		ItemsTotalCents: out.ItemsTotalCents,
		Version:         out.Version,
		Items:           items,
		CreatedAt:       out.CreatedAt,
		UpdatedAt:       out.UpdatedAt,
	}, nil
}

func (a *transactionsLedgerAdapter) SearchTransactions(ctx context.Context, _ uuid.UUID, query, refMonth string, limit int) ([]agentsifaces.Entry, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.search_transactions")
	defer span.End()

	ctx, err := a.principalCtx(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	results, err := a.searchTx.Execute(ctx, query, refMonth, limit)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("agents/binding/transactions_ledger: buscar transações: %w", err)
	}

	entries := make([]agentsifaces.Entry, 0, len(results))
	for _, tx := range results {
		var sub *string
		if tx.SubcategoryID != nil {
			s := tx.SubcategoryID.String()
			sub = &s
		}
		entries = append(entries, agentsifaces.Entry{
			Kind:                    "transaction",
			ID:                      tx.ID.String(),
			UserID:                  tx.UserID.String(),
			Direction:               tx.Direction,
			PaymentMethod:           tx.PaymentMethod,
			AmountCents:             tx.AmountCents,
			Description:             tx.Description,
			CategoryID:              tx.CategoryID.String(),
			SubcategoryID:           sub,
			CategoryNameSnapshot:    tx.CategoryNameSnapshot,
			SubcategoryNameSnapshot: tx.SubcategoryNameSnapshot,
			RefMonth:                tx.RefMonth,
			OccurredAt:              tx.OccurredAt,
			Version:                 tx.Version,
			CreatedAt:               tx.CreatedAt,
			UpdatedAt:               tx.UpdatedAt,
		})
	}
	return entries, nil
}

func (a *transactionsLedgerAdapter) GetMonthlySummary(ctx context.Context, _ uuid.UUID, refMonth string) (agentsifaces.MonthlySummary, error) {
	ctx, span := a.o11y.Tracer().Start(ctx, "agents.binding.transactions_ledger.get_monthly_summary")
	defer span.End()

	ctx, err := a.principalCtx(ctx)
	if err != nil {
		span.RecordError(err)
		return agentsifaces.MonthlySummary{}, err
	}

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
