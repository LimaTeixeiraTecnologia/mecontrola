package dispatcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	transactionsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	transactionsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	transactionsusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

var ErrTransactionsCreateInvalidPayload = errors.New("agent.llm.dispatcher.transactions.create: payload invalido")

var ErrTransactionsDeleteMissingID = errors.New("agent.llm.dispatcher.transactions.delete: id ausente no payload")

type ListTransactionsUseCase interface {
	Execute(ctx context.Context, refMonthStr, cursor string, limit int) (transactionsusecases.TransactionPage, error)
}

type CreateTransactionUseCase interface {
	Execute(ctx context.Context, raw transactionsinput.RawCreateTransaction) (transactionsoutput.Transaction, error)
}

type DeleteTransactionUseCase interface {
	Execute(ctx context.Context, txID string, version int64) error
}

type GetTransactionUseCase interface {
	Execute(ctx context.Context, txID string) (transactionsoutput.Transaction, error)
}

type TransactionsAdapter struct {
	listUseCase   ListTransactionsUseCase
	createUseCase CreateTransactionUseCase
	deleteUseCase DeleteTransactionUseCase
	getUseCase    GetTransactionUseCase
	defaultLimit  int
}

func NewTransactionsAdapterFull(
	listUseCase ListTransactionsUseCase,
	createUseCase CreateTransactionUseCase,
	deleteUseCase DeleteTransactionUseCase,
	getUseCase GetTransactionUseCase,
) *TransactionsAdapter {
	return &TransactionsAdapter{
		listUseCase:   listUseCase,
		createUseCase: createUseCase,
		deleteUseCase: deleteUseCase,
		getUseCase:    getUseCase,
		defaultLimit:  10,
	}
}

type listFilters struct {
	Month string `json:"month"`
}

func (a *TransactionsAdapter) List(ctx context.Context, userID uuid.UUID, rawFilters json.RawMessage) (string, error) {
	month := defaultMonth()
	if len(rawFilters) > 0 {
		var parsed listFilters
		if err := json.Unmarshal(rawFilters, &parsed); err == nil && parsed.Month != "" {
			month = parsed.Month
		}
	}

	if _, ok := auth.FromContext(ctx); !ok {
		ctx = auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
	}

	page, err := a.listUseCase.Execute(ctx, month, "", a.defaultLimit)
	if err != nil {
		return "", fmt.Errorf("transactions.list: %w", err)
	}
	if len(page.Transactions) == 0 {
		return fmt.Sprintf("Nenhum lancamento encontrado para %s.", month), nil
	}

	totalIn := int64(0)
	totalOut := int64(0)
	for _, t := range page.Transactions {
		switch t.Direction {
		case "income":
			totalIn += t.AmountCents
		case "expense":
			totalOut += t.AmountCents
		}
	}
	return fmt.Sprintf("Em %s voce tem %d lancamentos: entradas R$ %s e saidas R$ %s.",
		month, len(page.Transactions),
		formatCents(totalIn), formatCents(totalOut),
	), nil
}

type createPayload struct {
	Amount        json.Number `json:"amount"`
	AmountCents   *int64      `json:"amount_cents,omitempty"`
	Type          string      `json:"type"`
	Direction     string      `json:"direction"`
	PaymentMethod string      `json:"payment_method"`
	Description   string      `json:"description"`
	CategoryID    string      `json:"category_id"`
	SubcategoryID string      `json:"subcategory_id,omitempty"`
	OccurredAt    string      `json:"occurred_at"`
}

func (a *TransactionsAdapter) Create(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error) {
	if a.createUseCase == nil {
		return "", fmt.Errorf("agent.llm.dispatcher.transactions.create: %w", ErrIntentUnsupported)
	}
	raw, err := a.buildCreateTransaction(rawPayload)
	if err != nil {
		return "", err
	}

	if _, ok := auth.FromContext(ctx); !ok {
		ctx = auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
	}

	created, err := a.createUseCase.Execute(ctx, raw)
	if err != nil {
		return "", fmt.Errorf("transactions.create: %w", err)
	}

	verb := "saida"
	if created.Direction == "income" {
		verb = "entrada"
	}
	desc := strings.TrimSpace(created.Description)
	if desc == "" {
		desc = "sem descricao"
	}
	return fmt.Sprintf("Anotado: %s de R$ %s em %s (%s).",
		verb, formatCents(created.AmountCents), created.OccurredAt.Format("02/01"),
		strings.ToLower(desc),
	), nil
}

func (a *TransactionsAdapter) buildCreateTransaction(rawPayload json.RawMessage) (transactionsinput.RawCreateTransaction, error) {
	payload, err := parseCreatePayload(rawPayload)
	if err != nil {
		return transactionsinput.RawCreateTransaction{}, err
	}

	cents, err := resolveAmountCents(payload)
	if err != nil {
		return transactionsinput.RawCreateTransaction{}, err
	}
	direction, err := resolveDirection(payload)
	if err != nil {
		return transactionsinput.RawCreateTransaction{}, err
	}
	categoryID, err := uuid.Parse(strings.TrimSpace(payload.CategoryID))
	if err != nil {
		return transactionsinput.RawCreateTransaction{}, fmt.Errorf("agent.llm.dispatcher.transactions.create: category_id invalido: %w", ErrTransactionsCreateInvalidPayload)
	}
	subcategoryID, err := parseOptionalSubcategoryID(payload.SubcategoryID)
	if err != nil {
		return transactionsinput.RawCreateTransaction{}, err
	}

	paymentMethod := strings.TrimSpace(payload.PaymentMethod)
	if paymentMethod == "" {
		paymentMethod = "other"
	}

	return transactionsinput.RawCreateTransaction{
		Direction:     direction,
		PaymentMethod: paymentMethod,
		AmountCents:   cents,
		Description:   strings.TrimSpace(payload.Description),
		CategoryID:    categoryID,
		SubcategoryID: subcategoryID,
		OccurredAt:    resolveOccurredAt(payload.OccurredAt),
	}, nil
}

func parseCreatePayload(rawPayload json.RawMessage) (createPayload, error) {
	if len(rawPayload) == 0 {
		return createPayload{}, ErrTransactionsCreateInvalidPayload
	}
	dec := json.NewDecoder(strings.NewReader(string(rawPayload)))
	dec.UseNumber()
	var payload createPayload
	if err := dec.Decode(&payload); err != nil {
		return createPayload{}, fmt.Errorf("agent.llm.dispatcher.transactions.create: %w", ErrTransactionsCreateInvalidPayload)
	}
	return payload, nil
}

func parseOptionalSubcategoryID(raw string) (*uuid.UUID, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	subID, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("agent.llm.dispatcher.transactions.create: subcategory_id invalido: %w", ErrTransactionsCreateInvalidPayload)
	}
	return &subID, nil
}

type deletePayload struct {
	ID string `json:"id"`
}

func (a *TransactionsAdapter) Delete(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error) {
	if a.deleteUseCase == nil || a.getUseCase == nil {
		return "", fmt.Errorf("agent.llm.dispatcher.transactions.delete: %w", ErrIntentUnsupported)
	}
	if len(rawPayload) == 0 {
		return "", ErrTransactionsDeleteMissingID
	}
	var p deletePayload
	if err := json.Unmarshal(rawPayload, &p); err != nil || strings.TrimSpace(p.ID) == "" {
		return "", ErrTransactionsDeleteMissingID
	}
	txID := strings.TrimSpace(p.ID)
	if _, err := uuid.Parse(txID); err != nil {
		return "", fmt.Errorf("agent.llm.dispatcher.transactions.delete: id invalido: %w", err)
	}

	if _, ok := auth.FromContext(ctx); !ok {
		ctx = auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
	}

	current, err := a.getUseCase.Execute(ctx, txID)
	if err != nil {
		return "", fmt.Errorf("transactions.delete: get current: %w", err)
	}
	if err := a.deleteUseCase.Execute(ctx, txID, current.Version); err != nil {
		return "", fmt.Errorf("transactions.delete: %w", err)
	}
	return fmt.Sprintf("Lancamento removido: R$ %s em %s.",
		formatCents(current.AmountCents), current.OccurredAt.Format("02/01"),
	), nil
}

func resolveAmountCents(p createPayload) (int64, error) {
	if p.AmountCents != nil && *p.AmountCents > 0 {
		return *p.AmountCents, nil
	}
	raw := strings.TrimSpace(string(p.Amount))
	if raw == "" {
		return 0, fmt.Errorf("agent.llm.dispatcher.transactions.create: amount ausente: %w", ErrTransactionsCreateInvalidPayload)
	}
	amount, err := p.Amount.Float64()
	if err != nil {
		return 0, fmt.Errorf("agent.llm.dispatcher.transactions.create: amount invalido: %w", ErrTransactionsCreateInvalidPayload)
	}
	if amount <= 0 {
		return 0, fmt.Errorf("agent.llm.dispatcher.transactions.create: amount nao pode ser <= 0: %w", ErrTransactionsCreateInvalidPayload)
	}
	cents := int64(math.Round(amount * 100))
	if cents <= 0 {
		return 0, fmt.Errorf("agent.llm.dispatcher.transactions.create: amount muito pequeno: %w", ErrTransactionsCreateInvalidPayload)
	}
	return cents, nil
}

func resolveDirection(p createPayload) (string, error) {
	candidate := strings.ToLower(strings.TrimSpace(p.Direction))
	if candidate == "" {
		switch strings.ToLower(strings.TrimSpace(p.Type)) {
		case "expense", "outcome":
			candidate = "outcome"
		case "income":
			candidate = "income"
		}
	}
	if candidate == "expense" {
		candidate = "outcome"
	}
	if candidate != "income" && candidate != "outcome" {
		return "", fmt.Errorf("agent.llm.dispatcher.transactions.create: direction invalido: %w", ErrTransactionsCreateInvalidPayload)
	}
	return candidate, nil
}

func resolveOccurredAt(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Now().UTC().Format(time.RFC3339)
	}
	if _, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return trimmed
	}
	if parsed, err := time.Parse("2006-01-02", trimmed); err == nil {
		return parsed.UTC().Format(time.RFC3339)
	}
	return time.Now().UTC().Format(time.RFC3339)
}

func defaultMonth() string {
	return time.Now().UTC().Format("2006-01")
}

func formatCents(cents int64) string {
	if cents < 0 {
		cents = -cents
	}
	reais := cents / 100
	centavos := cents % 100
	return fmt.Sprintf("%d,%02d", reais, centavos)
}
