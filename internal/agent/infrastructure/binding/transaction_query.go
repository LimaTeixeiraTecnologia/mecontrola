package binding

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	transactionsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	transactionsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	transactionsusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

func withWhatsAppPrincipal(ctx context.Context, userID uuid.UUID) context.Context {
	if _, ok := auth.FromContext(ctx); !ok {
		return auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
	}
	return ctx
}

type CardPurchaseLoggerAdapter struct {
	uc *usecases.RecordCardPurchaseFromAgent
}

func NewCardPurchaseLoggerAdapter(uc *usecases.RecordCardPurchaseFromAgent) *CardPurchaseLoggerAdapter {
	return &CardPurchaseLoggerAdapter{uc: uc}
}

func (a *CardPurchaseLoggerAdapter) Execute(ctx context.Context, in tools.CardPurchaseLoggerInput) (tools.CardPurchaseLoggerResult, error) {
	result, err := a.uc.Execute(ctx, usecases.RecordCardPurchaseFromAgentInput{
		UserID:        in.UserID,
		Intent:        in.Intent,
		ForceCategory: in.ForceCategory,
		AmountCents:   in.AmountCents,
		Merchant:      in.Merchant,
		PaymentMethod: in.PaymentMethod,
		CardHint:      in.CardHint,
		Installments:  in.Installments,
	})
	if err != nil {
		return tools.CardPurchaseLoggerResult{}, translateCategoryError(err)
	}
	return tools.CardPurchaseLoggerResult{
		Persisted:    result.Persisted,
		CardFound:    result.CardFound,
		CardName:     result.CardName,
		AmountCents:  result.AmountCents,
		Installments: result.Installments,
		CategoryPath: result.CategoryPath,
	}, nil
}

type cardListUseCase interface {
	Execute(ctx context.Context, in cardinput.ListCards) (cardoutput.CardList, error)
}

type cardPurchaseCreateUseCase interface {
	Execute(ctx context.Context, raw transactionsinput.RawCreateCardPurchase) (transactionsoutput.CardPurchase, error)
}

type CardPurchaseCreatorAdapter struct {
	cardLister cardListUseCase
	createUC   cardPurchaseCreateUseCase
}

func NewCardPurchaseCreatorAdapter(cardLister cardListUseCase, createUC cardPurchaseCreateUseCase) *CardPurchaseCreatorAdapter {
	return &CardPurchaseCreatorAdapter{cardLister: cardLister, createUC: createUC}
}

func (a *CardPurchaseCreatorAdapter) Execute(ctx context.Context, in usecases.CreateCardPurchaseCommand) (usecases.CreateCardPurchaseResult, error) {
	userID, err := uuid.Parse(strings.TrimSpace(in.UserID))
	if err != nil {
		return usecases.CreateCardPurchaseResult{}, fmt.Errorf("agent: card purchase creator: user id: %w", err)
	}
	rootID, err := uuid.Parse(strings.TrimSpace(in.RootCategoryID))
	if err != nil {
		return usecases.CreateCardPurchaseResult{}, fmt.Errorf("agent: card purchase creator: category id: %w", err)
	}
	var subID *uuid.UUID
	if trimmed := strings.TrimSpace(in.SubcategoryID); trimmed != "" {
		parsed, parseErr := uuid.Parse(trimmed)
		if parseErr != nil {
			return usecases.CreateCardPurchaseResult{}, fmt.Errorf("agent: card purchase creator: subcategory id: %w", parseErr)
		}
		subID = &parsed
	}

	ctx = withWhatsAppPrincipal(ctx, userID)

	cards, err := a.cardLister.Execute(ctx, cardinput.ListCards{UserID: userID, Limit: defaultListCardsLimit})
	if err != nil {
		return usecases.CreateCardPurchaseResult{}, fmt.Errorf("agent: card purchase creator: listar cartões: %w", err)
	}
	resolved, ok := resolveCardByName(cards, in.CardHint)
	if !ok {
		return usecases.CreateCardPurchaseResult{CardFound: false}, nil
	}
	cardID, err := uuid.Parse(resolved.ID)
	if err != nil {
		return usecases.CreateCardPurchaseResult{}, fmt.Errorf("agent: card purchase creator: card id: %w", err)
	}

	_, err = a.createUC.Execute(ctx, transactionsinput.RawCreateCardPurchase{
		CardID:            cardID,
		TotalAmountCents:  in.AmountCents,
		InstallmentsTotal: in.Installments,
		Description:       in.Description,
		CategoryID:        rootID,
		SubcategoryID:     subID,
		PurchasedAt:       time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return usecases.CreateCardPurchaseResult{}, fmt.Errorf("agent: card purchase creator: criar: %w", err)
	}

	name := strings.TrimSpace(resolved.Nickname)
	if name == "" {
		name = strings.TrimSpace(resolved.Name)
	}
	return usecases.CreateCardPurchaseResult{CardFound: true, CardName: name}, nil
}

const defaultListCardsLimit = 200

type listTransactionsUseCase interface {
	Execute(ctx context.Context, refMonthStr, cursor string, limit int) (transactionsusecases.TransactionPage, error)
}

type TransactionListerAdapter struct {
	uc    listTransactionsUseCase
	limit int
}

func NewTransactionListerAdapter(uc listTransactionsUseCase) *TransactionListerAdapter {
	return &TransactionListerAdapter{uc: uc, limit: 200}
}

func (a *TransactionListerAdapter) Execute(ctx context.Context, in tools.TransactionListInput) (tools.TransactionListResult, error) {
	userID, err := uuid.Parse(strings.TrimSpace(in.UserID))
	if err != nil {
		return tools.TransactionListResult{}, fmt.Errorf("agent: transaction lister: user id: %w", err)
	}
	ctx = withWhatsAppPrincipal(ctx, userID)

	refMonth := in.RefMonth
	if refMonth == "" {
		refMonth = time.Now().UTC().Format("2006-01")
	}

	page, err := a.uc.Execute(ctx, refMonth, "", a.limit)
	if err != nil {
		return tools.TransactionListResult{}, fmt.Errorf("agent: transaction lister: %w", err)
	}
	views := make([]tools.TransactionView, 0, len(page.Transactions))
	for _, t := range page.Transactions {
		views = append(views, transactionViewFrom(t))
	}
	return tools.TransactionListResult{RefMonth: refMonth, Transactions: views}, nil
}

func transactionViewFrom(t transactionsoutput.Transaction) tools.TransactionView {
	return tools.TransactionView{
		ID:          t.ID.String(),
		Direction:   t.Direction,
		AmountCents: t.AmountCents,
		Description: t.Description,
		OccurredAt:  t.OccurredAt,
		CreatedAt:   t.CreatedAt,
		Version:     t.Version,
	}
}

type deleteTransactionUseCase interface {
	Execute(ctx context.Context, txID string, version int64) error
}

type LastTransactionDeleterAdapter struct {
	uc deleteTransactionUseCase
}

func NewLastTransactionDeleterAdapter(uc deleteTransactionUseCase) *LastTransactionDeleterAdapter {
	return &LastTransactionDeleterAdapter{uc: uc}
}

func (a *LastTransactionDeleterAdapter) Execute(ctx context.Context, userID, txID string, version int64) error {
	parsed, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return fmt.Errorf("agent: last transaction deleter: user id: %w", err)
	}
	ctx = withWhatsAppPrincipal(ctx, parsed)
	if err := a.uc.Execute(ctx, txID, version); err != nil {
		return fmt.Errorf("agent: last transaction deleter: %w", err)
	}
	return nil
}

type getTransactionUseCase interface {
	Execute(ctx context.Context, txID string) (transactionsoutput.Transaction, error)
}

type updateTransactionUseCase interface {
	Execute(ctx context.Context, txID string, raw transactionsinput.RawUpdateTransaction) (transactionsoutput.Transaction, error)
}

type LastTransactionEditorAdapter struct {
	getUC    getTransactionUseCase
	updateUC updateTransactionUseCase
}

func NewLastTransactionEditorAdapter(getUC getTransactionUseCase, updateUC updateTransactionUseCase) *LastTransactionEditorAdapter {
	return &LastTransactionEditorAdapter{getUC: getUC, updateUC: updateUC}
}

func (a *LastTransactionEditorAdapter) Execute(ctx context.Context, in tools.EditTransactionInput) (tools.EditTransactionResult, error) {
	userID, err := uuid.Parse(strings.TrimSpace(in.UserID))
	if err != nil {
		return tools.EditTransactionResult{}, fmt.Errorf("agent: last transaction editor: user id: %w", err)
	}
	ctx = withWhatsAppPrincipal(ctx, userID)

	current, err := a.getUC.Execute(ctx, in.Current.ID)
	if err != nil {
		return tools.EditTransactionResult{}, fmt.Errorf("agent: last transaction editor: buscar atual: %w", err)
	}

	updated, err := a.updateUC.Execute(ctx, current.ID.String(), transactionsinput.RawUpdateTransaction{
		Direction:     current.Direction,
		PaymentMethod: current.PaymentMethod,
		AmountCents:   in.NewAmount,
		Description:   current.Description,
		CategoryID:    current.CategoryID,
		SubcategoryID: current.SubcategoryID,
		OccurredAt:    current.OccurredAt.UTC().Format(time.RFC3339),
		Version:       current.Version,
	})
	if err != nil {
		return tools.EditTransactionResult{}, fmt.Errorf("agent: last transaction editor: atualizar: %w", err)
	}

	return tools.EditTransactionResult{
		Persisted:   true,
		OldAmount:   current.AmountCents,
		NewAmount:   updated.AmountCents,
		Description: updated.Description,
	}, nil
}

func resolveCardByName(list cardoutput.CardList, name string) (cardoutput.Card, bool) {
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return cardoutput.Card{}, false
	}
	for _, item := range list.Items {
		if strings.EqualFold(strings.TrimSpace(item.Name), name) {
			return item, true
		}
		if strings.EqualFold(strings.TrimSpace(item.Nickname), name) {
			return item, true
		}
	}
	for _, item := range list.Items {
		if strings.Contains(strings.ToLower(item.Name), target) {
			return item, true
		}
		if strings.Contains(strings.ToLower(item.Nickname), target) {
			return item, true
		}
	}
	return cardoutput.Card{}, false
}
