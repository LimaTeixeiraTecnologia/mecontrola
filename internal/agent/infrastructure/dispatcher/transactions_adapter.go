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

type GetTransactionUseCase interface {
	Execute(ctx context.Context, txID string) (transactionsoutput.Transaction, error)
}

type CreateTransactionUseCase interface {
	Execute(ctx context.Context, raw transactionsinput.RawCreateTransaction) (transactionsoutput.Transaction, error)
}

type DeleteTransactionUseCase interface {
	Execute(ctx context.Context, txID string, version int64) error
}

type CreateCardPurchaseUseCase interface {
	Execute(ctx context.Context, raw transactionsinput.RawCreateCardPurchase) (transactionsoutput.CardPurchase, error)
}

type CreateRecurringTemplateUseCase interface {
	Execute(ctx context.Context, raw transactionsinput.RawCreateRecurringTemplate) (transactionsoutput.RecurringTemplate, error)
}

type ListRecurringTemplatesUseCase interface {
	Execute(ctx context.Context, activeOnly bool, cursor string, limit int) (transactionsusecases.RecurringTemplatePage, error)
}

type TransactionsAdapter struct {
	listUseCase          ListTransactionsUseCase
	createUseCase        CreateTransactionUseCase
	deleteUseCase        DeleteTransactionUseCase
	getUseCase           GetTransactionUseCase
	createCardPurchaseUC CreateCardPurchaseUseCase
	createRecurringUC    CreateRecurringTemplateUseCase
	listRecurringUC      ListRecurringTemplatesUseCase
	defaultLimit         int
}

func NewTransactionsAdapterFull(
	listUseCase ListTransactionsUseCase,
	createUseCase CreateTransactionUseCase,
	deleteUseCase DeleteTransactionUseCase,
	getUseCase GetTransactionUseCase,
	createCardPurchaseUC CreateCardPurchaseUseCase,
	createRecurringUC CreateRecurringTemplateUseCase,
	listRecurringUC ListRecurringTemplatesUseCase,
) *TransactionsAdapter {
	return &TransactionsAdapter{
		listUseCase:          listUseCase,
		createUseCase:        createUseCase,
		deleteUseCase:        deleteUseCase,
		getUseCase:           getUseCase,
		createCardPurchaseUC: createCardPurchaseUC,
		createRecurringUC:    createRecurringUC,
		listRecurringUC:      listRecurringUC,
		defaultLimit:         10,
	}
}

type listFilters struct {
	Month string `json:"month"`
	ID    string `json:"id"`
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

func (a *TransactionsAdapter) Get(ctx context.Context, userID uuid.UUID, rawFilters json.RawMessage) (string, error) {
	if a.getUseCase == nil {
		return "", fmt.Errorf("transactions.get: %w", ErrIntentUnsupported)
	}
	var filters listFilters
	if err := json.Unmarshal(rawFilters, &filters); err != nil || strings.TrimSpace(filters.ID) == "" {
		return "", fmt.Errorf("transactions.get: id ausente")
	}
	if _, ok := auth.FromContext(ctx); !ok {
		ctx = auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
	}
	out, err := a.getUseCase.Execute(ctx, strings.TrimSpace(filters.ID))
	if err != nil {
		return "", fmt.Errorf("transactions.get: %w", err)
	}
	return fmt.Sprintf("Lancamento de R$ %s em %s: %s.",
		formatCents(out.AmountCents), out.OccurredAt.Format("02/01"),
		strings.ToLower(strings.TrimSpace(out.Description)),
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

type cardPurchasePayload struct {
	AmountCents   int64  `json:"amount_cents"`
	CardID        string `json:"card_id"`
	Installments  int    `json:"installments"`
	Description   string `json:"description"`
	CategoryID    string `json:"category_id"`
	SubcategoryID string `json:"subcategory_id,omitempty"`
	OccurredAt    string `json:"occurred_at"`
}

func (a *TransactionsAdapter) CreateCardPurchase(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error) {
	if a.createCardPurchaseUC == nil {
		return "", ErrIntentUnsupported
	}
	if len(rawPayload) == 0 {
		return "", fmt.Errorf("agent.llm.dispatcher.transactions.create_card_purchase: %w", ErrTransactionsCreateInvalidPayload)
	}
	var p cardPurchasePayload
	if err := json.Unmarshal(rawPayload, &p); err != nil {
		return "", fmt.Errorf("agent.llm.dispatcher.transactions.create_card_purchase: %w", ErrTransactionsCreateInvalidPayload)
	}
	if p.AmountCents <= 0 {
		return "", fmt.Errorf("agent.llm.dispatcher.transactions.create_card_purchase: amount_cents invalido: %w", ErrTransactionsCreateInvalidPayload)
	}
	cardID, err := uuid.Parse(strings.TrimSpace(p.CardID))
	if err != nil {
		return "", fmt.Errorf("agent.llm.dispatcher.transactions.create_card_purchase: card_id invalido: %w", ErrTransactionsCreateInvalidPayload)
	}
	if p.Installments < 2 {
		return "", fmt.Errorf("agent.llm.dispatcher.transactions.create_card_purchase: installments deve ser >= 2: %w", ErrTransactionsCreateInvalidPayload)
	}
	categoryID, err := uuid.Parse(strings.TrimSpace(p.CategoryID))
	if err != nil {
		return "", fmt.Errorf("agent.llm.dispatcher.transactions.create_card_purchase: category_id invalido: %w", ErrTransactionsCreateInvalidPayload)
	}
	subcategoryID, err := parseOptionalSubcategoryID(p.SubcategoryID)
	if err != nil {
		return "", fmt.Errorf("agent.llm.dispatcher.transactions.create_card_purchase: subcategory_id invalido: %w", ErrTransactionsCreateInvalidPayload)
	}
	purchasedAt := resolveOccurredAt(p.OccurredAt)

	if _, ok := auth.FromContext(ctx); !ok {
		ctx = auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
	}

	out, err := a.createCardPurchaseUC.Execute(ctx, transactionsinput.RawCreateCardPurchase{
		CardID:            cardID,
		TotalAmountCents:  p.AmountCents,
		InstallmentsTotal: p.Installments,
		Description:       strings.TrimSpace(p.Description),
		CategoryID:        categoryID,
		SubcategoryID:     subcategoryID,
		PurchasedAt:       purchasedAt,
	})
	if err != nil {
		return "", fmt.Errorf("transactions.create_card_purchase: %w", err)
	}
	return fmt.Sprintf("Compra de R$ %s parcelada em %dx anotada nas suas faturas.",
		formatCents(out.TotalAmountCents), out.InstallmentsTotal,
	), nil
}

type recurringPayload struct {
	AmountCents   int64  `json:"amount_cents"`
	Direction     string `json:"direction"`
	Frequency     string `json:"frequency"`
	DayOfMonth    int    `json:"day_of_month"`
	Description   string `json:"description"`
	CategoryID    string `json:"category_id"`
	PaymentMethod string `json:"payment_method"`
}

type validatedRecurring struct {
	amountCents   int64
	direction     string
	frequency     string
	dayOfMonth    int
	paymentMethod string
	categoryID    uuid.UUID
	description   string
}

func parseRecurringPayload(raw json.RawMessage) (validatedRecurring, error) {
	const prefix = "agent.llm.dispatcher.transactions.create_recurring"
	if len(raw) == 0 {
		return validatedRecurring{}, fmt.Errorf("%s: %w", prefix, ErrTransactionsCreateInvalidPayload)
	}
	var p recurringPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return validatedRecurring{}, fmt.Errorf("%s: %w", prefix, ErrTransactionsCreateInvalidPayload)
	}
	if p.AmountCents <= 0 {
		return validatedRecurring{}, fmt.Errorf("%s: amount_cents invalido: %w", prefix, ErrTransactionsCreateInvalidPayload)
	}
	direction := strings.ToLower(strings.TrimSpace(p.Direction))
	if direction != "income" && direction != "outcome" {
		return validatedRecurring{}, fmt.Errorf("%s: direction invalido: %w", prefix, ErrTransactionsCreateInvalidPayload)
	}
	frequency := strings.ToLower(strings.TrimSpace(p.Frequency))
	if frequency == "" {
		frequency = "monthly"
	}
	if frequency != "monthly" && frequency != "yearly" {
		return validatedRecurring{}, fmt.Errorf("%s: frequency invalido: %w", prefix, ErrTransactionsCreateInvalidPayload)
	}
	categoryID, err := uuid.Parse(strings.TrimSpace(p.CategoryID))
	if err != nil {
		return validatedRecurring{}, fmt.Errorf("%s: category_id invalido: %w", prefix, ErrTransactionsCreateInvalidPayload)
	}
	dayOfMonth := p.DayOfMonth
	if dayOfMonth == 0 {
		dayOfMonth = 1
	}
	if dayOfMonth < 1 || dayOfMonth > 31 {
		return validatedRecurring{}, fmt.Errorf("%s: day_of_month invalido: %w", prefix, ErrTransactionsCreateInvalidPayload)
	}
	paymentMethod := strings.TrimSpace(p.PaymentMethod)
	if paymentMethod == "" {
		paymentMethod = "other"
	}
	return validatedRecurring{
		amountCents:   p.AmountCents,
		direction:     direction,
		frequency:     frequency,
		dayOfMonth:    dayOfMonth,
		paymentMethod: paymentMethod,
		categoryID:    categoryID,
		description:   strings.TrimSpace(p.Description),
	}, nil
}

func (a *TransactionsAdapter) CreateRecurring(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error) {
	if a.createRecurringUC == nil {
		return "", ErrIntentUnsupported
	}
	v, err := parseRecurringPayload(rawPayload)
	if err != nil {
		return "", err
	}
	if _, ok := auth.FromContext(ctx); !ok {
		ctx = auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
	}
	out, err := a.createRecurringUC.Execute(ctx, transactionsinput.RawCreateRecurringTemplate{
		Direction:     v.direction,
		PaymentMethod: v.paymentMethod,
		AmountCents:   v.amountCents,
		Description:   v.description,
		CategoryID:    v.categoryID,
		Frequency:     v.frequency,
		DayOfMonth:    v.dayOfMonth,
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return "", fmt.Errorf("transactions.create_recurring: %w", err)
	}
	verb := "saida"
	if out.Direction == "income" {
		verb = "entrada"
	}
	return fmt.Sprintf("Recorrencia criada: %s de R$ %s (%s) com frequencia %s.",
		verb, formatCents(out.AmountCents), strings.ToLower(strings.TrimSpace(out.Description)), recurringFrequencyLabel(out.Frequency),
	), nil
}

func recurringFrequencyLabel(f string) string {
	switch f {
	case "monthly":
		return "mensal"
	case "yearly":
		return "anual"
	default:
		return f
	}
}

func (a *TransactionsAdapter) ListRecurring(ctx context.Context, userID uuid.UUID, _ json.RawMessage) (string, error) {
	if a.listRecurringUC == nil {
		return "", ErrIntentUnsupported
	}
	if _, ok := auth.FromContext(ctx); !ok {
		ctx = auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
	}
	page, err := a.listRecurringUC.Execute(ctx, true, "", 20)
	if err != nil {
		return "", fmt.Errorf("transactions.list_recurring: %w", err)
	}
	if len(page.Templates) == 0 {
		return "Nenhuma recorrencia cadastrada.", nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Voce tem %d recorrencia(s):\n", len(page.Templates))
	for _, t := range page.Templates {
		verb := "saida"
		if t.Direction == "income" {
			verb = "entrada"
		}
		fmt.Fprintf(&sb, "- %s de R$ %s (%s) - %s\n",
			verb, formatCents(t.AmountCents), recurringFrequencyLabel(t.Frequency), strings.ToLower(strings.TrimSpace(t.Description)),
		)
	}
	if page.NextCursor != "" {
		sb.WriteString("\n...e mais. Pergunte sobre uma recorrencia especifica para ver detalhes.")
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}
